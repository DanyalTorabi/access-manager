package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/dtorabi/access-manager/internal/access"
	"github.com/dtorabi/access-manager/internal/logger"
	"github.com/dtorabi/access-manager/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server exposes HTTP handlers for the access manager.
type Server struct {
	Store store.Store
	// APIBearerToken, if non-empty, requires Authorization: Bearer <token> on /api/v1/*.
	// /health stays public. Empty means no auth on API (local dev / loopback only — document in README).
	APIBearerToken string

	metrics *Metrics
}

// NegativeMaskCounter returns the store_negative_mask_observed_total
// Prometheus counter, or nil when metrics are disabled. It is the narrow
// accessor used by cmd/server to wire the SQLite store's negative-mask
// hook without exposing the full Metrics struct.
func (s *Server) NegativeMaskCounter() prometheus.Counter {
	if s.metrics == nil {
		return nil
	}
	return s.metrics.NegativeMaskTotal
}

// Router builds the chi router. reg and gather supply the Prometheus registry
// for metrics middleware and the /metrics endpoint. Pass nil for both to
// disable instrumentation (e.g. in tests that don't care about metrics).
func (s *Server) Router(reg prometheus.Registerer, gather prometheus.Gatherer) chi.Router {
	r := chi.NewRouter()

	if reg != nil {
		s.metrics = NewMetrics(reg)
		r.Use(s.metrics.Middleware)
	} else {
		s.metrics = nil
	}

	r.Get("/health", s.health)
	// /metrics is outside bearer auth so Prometheus can scrape without a token.
	// Bind to loopback or use network policy when exposing beyond localhost.
	if gather != nil {
		r.Handle("/metrics", promhttp.HandlerFor(gather, promhttp.HandlerOpts{}))
	}

	r.Route("/api/v1", func(r chi.Router) {
		if tok := strings.TrimSpace(s.APIBearerToken); tok != "" {
			r.Use(BearerAuth(tok))
		}
		r.Post("/domains", s.domainCreate)
		r.Get("/domains", s.domainList)
		r.Get("/domains/{domainID}", s.domainGet)
		r.Patch("/domains/{domainID}", s.domainPatch)
		r.Delete("/domains/{domainID}", s.domainDelete)

		r.Get("/domains/{domainID}/users", s.userList)
		r.Post("/domains/{domainID}/users", s.userCreate)
		r.Get("/domains/{domainID}/users/{userID}", s.userGet)
		r.Patch("/domains/{domainID}/users/{userID}", s.userPatch)
		r.Delete("/domains/{domainID}/users/{userID}", s.userDelete)

		r.Get("/domains/{domainID}/groups", s.groupList)
		r.Post("/domains/{domainID}/groups", s.groupCreate)
		r.Get("/domains/{domainID}/groups/{groupID}", s.groupGet)
		r.Patch("/domains/{domainID}/groups/{groupID}", s.groupPatch)
		r.Delete("/domains/{domainID}/groups/{groupID}", s.groupDelete)
		r.Patch("/domains/{domainID}/groups/{groupID}/parent", s.groupSetParent)

		r.Get("/domains/{domainID}/resources", s.resourceList)
		r.Post("/domains/{domainID}/resources", s.resourceCreate)
		r.Get("/domains/{domainID}/resources/{resourceID}", s.resourceGet)
		r.Patch("/domains/{domainID}/resources/{resourceID}", s.resourcePatch)
		r.Delete("/domains/{domainID}/resources/{resourceID}", s.resourceDelete)

		r.Get("/domains/{domainID}/access-types", s.accessTypeList)
		r.Post("/domains/{domainID}/access-types", s.accessTypeCreate)
		r.Get("/domains/{domainID}/access-types/{accessTypeID}", s.accessTypeGet)
		r.Patch("/domains/{domainID}/access-types/{accessTypeID}", s.accessTypePatch)
		r.Delete("/domains/{domainID}/access-types/{accessTypeID}", s.accessTypeDelete)

		r.Get("/domains/{domainID}/permissions", s.permissionList)
		r.Post("/domains/{domainID}/permissions", s.permissionCreate)
		r.Get("/domains/{domainID}/permissions/{permissionID}", s.permissionGet)
		r.Patch("/domains/{domainID}/permissions/{permissionID}", s.permissionPatch)
		r.Delete("/domains/{domainID}/permissions/{permissionID}", s.permissionDelete)

		r.Post("/domains/{domainID}/users/{userID}/groups/{groupID}", s.addUserToGroup)
		r.Delete("/domains/{domainID}/users/{userID}/groups/{groupID}", s.removeUserFromGroup)

		r.Post("/domains/{domainID}/users/{userID}/permissions/{permissionID}", s.grantUserPermission)
		r.Delete("/domains/{domainID}/users/{userID}/permissions/{permissionID}", s.revokeUserPermission)
		r.Get("/domains/{domainID}/users/{userID}/authz/resources", s.userAuthzResources)

		r.Post("/domains/{domainID}/groups/{groupID}/permissions/{permissionID}", s.grantGroupPermission)
		r.Delete("/domains/{domainID}/groups/{groupID}/permissions/{permissionID}", s.revokeGroupPermission)
		r.Get("/domains/{domainID}/groups/{groupID}/authz/resources", s.groupAuthzResources)

		r.Get("/domains/{domainID}/resources/{resourceID}/authz/users", s.resourceAuthzUsers)
		r.Get("/domains/{domainID}/resources/{resourceID}/authz/groups", s.resourceAuthzGroups)

		r.Get("/domains/{domainID}/authz/check", s.authzCheck)
		r.Get("/domains/{domainID}/authz/masks", s.authzMasks)
	})
	return r
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type titleBody struct {
	Title string `json:"title"`
}

type patchTitleBody struct {
	Title *string `json:"title"`
}

type groupPatchBody struct {
	Title         *string         `json:"title"`
	ParentGroupID json.RawMessage `json:"parent_group_id"`
}

type accessTypePatchBody struct {
	Title *string `json:"title"`
	Bit   *string `json:"bit"`
}

type permissionPatchBody struct {
	Title      *string `json:"title"`
	ResourceID *string `json:"resource_id"`
	AccessMask *string `json:"access_mask"`
}

type permissionBody struct {
	Title      string `json:"title"`
	ResourceID string `json:"resource_id"`
	AccessMask string `json:"access_mask"` // decimal or 0x hex
}

type accessTypeBody struct {
	Title string `json:"title"`
	Bit   string `json:"bit"` // decimal or 0x hex
}

type parentBody struct {
	ParentGroupID *string `json:"parent_group_id"`
}

// parentGroupAuditAttrs adds parent hierarchy fields for group create vs set-parent.
// When explicitClear is true (PATCH parent), nil ParentGroupID means the parent was cleared.
// When explicitClear is false (create), nil means the new group is a root (no parent).
func parentGroupAuditAttrs(parentID *string, explicitClear bool) []slog.Attr {
	if parentID != nil {
		return []slog.Attr{slog.String("parent_group_id", *parentID)}
	}
	if explicitClear {
		return []slog.Attr{slog.Bool("parent_cleared", true)}
	}
	return []slog.Attr{slog.Bool("parent_root", true)}
}

func (s *Server) domainCreate(w http.ResponseWriter, r *http.Request) {
	var b titleBody
	if !readJSON(w, r, &b) {
		return
	}
	d := &store.Domain{ID: uuid.NewString(), Title: b.Title}
	if err := s.Store.DomainCreate(r.Context(), d); err != nil {
		writeStoreErr(w, r, err)
		return
	}
	logger.Audit(r.Context(), "domain_create", slog.String("domain_id", d.ID))
	writeJSON(w, http.StatusCreated, d)
}

func (s *Server) domainList(w http.ResponseWriter, r *http.Request) {
	opts, err := parseListOpts(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	opts.Sort, opts.Order, err = parseSortOrder(r.URL.Query(), store.DomainSortFields)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	list, total, err := s.Store.DomainList(r.Context(), opts)
	if err != nil {
		writeInternalErr(w, r, err)
		return
	}
	if list == nil {
		list = []store.Domain{}
	}
	writeList(w, list, total, opts)
}

func (s *Server) domainGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "domainID")
	d, err := s.Store.DomainGet(r.Context(), id)
	if err != nil {
		writeStoreErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) domainPatch(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "domainID")
	var b patchTitleBody
	if !readJSON(w, r, &b) {
		return
	}
	if b.Title == nil {
		writeErr(w, http.StatusBadRequest, errors.New("title is required for patch"))
		return
	}
	d, err := s.Store.DomainPatch(r.Context(), id, b.Title)
	if err != nil {
		writeStoreErr(w, r, err)
		return
	}
	logger.Audit(r.Context(), "domain_patch", slog.String("domain_id", id))
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) domainDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "domainID")
	if err := s.Store.DomainDelete(r.Context(), id); err != nil {
		writeStoreErr(w, r, err)
		return
	}
	logger.Audit(r.Context(), "domain_delete", slog.String("domain_id", id))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) userCreate(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	var b titleBody
	if !readJSON(w, r, &b) {
		return
	}
	u := &store.User{ID: uuid.NewString(), DomainID: domainID, Title: b.Title}
	if err := s.Store.UserCreate(r.Context(), u); err != nil {
		writeStoreErr(w, r, err)
		return
	}
	logger.Audit(r.Context(), "user_create", slog.String("domain_id", domainID), slog.String("user_id", u.ID))
	writeJSON(w, http.StatusCreated, u)
}

func (s *Server) userList(w http.ResponseWriter, r *http.Request) {
	opts, err := parseListOpts(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	opts.Sort, opts.Order, err = parseSortOrder(r.URL.Query(), store.UserSortFields)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	domainID := chi.URLParam(r, "domainID")
	list, total, err := s.Store.UserList(r.Context(), domainID, opts)
	if err != nil {
		writeInternalErr(w, r, err)
		return
	}
	if list == nil {
		list = []store.User{}
	}
	writeList(w, list, total, opts)
}

func (s *Server) userGet(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	id := chi.URLParam(r, "userID")
	u, err := s.Store.UserGet(r.Context(), domainID, id)
	if err != nil {
		writeStoreErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func (s *Server) userPatch(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	id := chi.URLParam(r, "userID")
	var b patchTitleBody
	if !readJSON(w, r, &b) {
		return
	}
	if b.Title == nil {
		writeErr(w, http.StatusBadRequest, errors.New("title is required for patch"))
		return
	}
	u, err := s.Store.UserPatch(r.Context(), domainID, id, b.Title)
	if err != nil {
		writeStoreErr(w, r, err)
		return
	}
	logger.Audit(r.Context(), "user_patch", slog.String("domain_id", domainID), slog.String("user_id", id))
	writeJSON(w, http.StatusOK, u)
}

func (s *Server) userDelete(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	id := chi.URLParam(r, "userID")
	if err := s.Store.UserDelete(r.Context(), domainID, id); err != nil {
		writeStoreErr(w, r, err)
		return
	}
	logger.Audit(r.Context(), "user_delete", slog.String("domain_id", domainID), slog.String("user_id", id))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) groupCreate(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	var b struct {
		Title         string  `json:"title"`
		ParentGroupID *string `json:"parent_group_id"`
	}
	if !readJSON(w, r, &b) {
		return
	}
	g := &store.Group{ID: uuid.NewString(), DomainID: domainID, Title: b.Title, ParentGroupID: b.ParentGroupID}
	if err := s.Store.GroupCreate(r.Context(), g); err != nil {
		writeStoreErr(w, r, err)
		return
	}
	gaudit := []slog.Attr{slog.String("domain_id", domainID), slog.String("group_id", g.ID)}
	gaudit = append(gaudit, parentGroupAuditAttrs(b.ParentGroupID, false)...)
	logger.Audit(r.Context(), "group_create", gaudit...)
	writeJSON(w, http.StatusCreated, g)
}

func (s *Server) groupList(w http.ResponseWriter, r *http.Request) {
	opts, err := parseGroupListOpts(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	opts.Sort, opts.Order, err = parseSortOrder(r.URL.Query(), store.GroupSortFields)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	domainID := chi.URLParam(r, "domainID")
	list, total, err := s.Store.GroupList(r.Context(), domainID, opts)
	if err != nil {
		writeInternalErr(w, r, err)
		return
	}
	if list == nil {
		list = []store.Group{}
	}
	writeList(w, list, total, opts.ListOpts)
}

func (s *Server) groupGet(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	id := chi.URLParam(r, "groupID")
	g, err := s.Store.GroupGet(r.Context(), domainID, id)
	if err != nil {
		writeStoreErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func (s *Server) groupPatch(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	groupID := chi.URLParam(r, "groupID")
	var b groupPatchBody
	if !readJSON(w, r, &b) {
		return
	}
	params := store.GroupPatchParams{Title: b.Title}
	if len(b.ParentGroupID) > 0 {
		params.UpdateParent = true
		trimmed := bytes.TrimSpace(b.ParentGroupID)
		switch {
		case bytes.Equal(trimmed, []byte("null")):
			params.ParentGroupID = nil
		default:
			var pid string
			if err := json.Unmarshal(trimmed, &pid); err != nil {
				writeErr(w, http.StatusBadRequest, errors.New("parent_group_id must be a UUID string or null"))
				return
			}
			params.ParentGroupID = &pid
		}
	}
	if params.Title == nil && !params.UpdateParent {
		writeErr(w, http.StatusBadRequest, errors.New("at least one of title, parent_group_id is required"))
		return
	}
	g, err := s.Store.GroupPatch(r.Context(), domainID, groupID, params)
	if err != nil {
		writeStoreErr(w, r, err)
		return
	}
	gaudit := []slog.Attr{slog.String("domain_id", domainID), slog.String("group_id", groupID)}
	if params.Title != nil {
		gaudit = append(gaudit, slog.String("title", *params.Title))
	}
	if params.UpdateParent {
		gaudit = append(gaudit, parentGroupAuditAttrs(params.ParentGroupID, true)...)
	}
	logger.Audit(r.Context(), "group_patch", gaudit...)
	writeJSON(w, http.StatusOK, g)
}

func (s *Server) groupDelete(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	id := chi.URLParam(r, "groupID")
	if err := s.Store.GroupDelete(r.Context(), domainID, id); err != nil {
		writeStoreErr(w, r, err)
		return
	}
	logger.Audit(r.Context(), "group_delete", slog.String("domain_id", domainID), slog.String("group_id", id))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) groupSetParent(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	groupID := chi.URLParam(r, "groupID")
	var b parentBody
	if !readJSON(w, r, &b) {
		return
	}
	if err := s.Store.GroupSetParent(r.Context(), domainID, groupID, b.ParentGroupID); err != nil {
		writeStoreErr(w, r, err)
		return
	}
	auditAttrs := []slog.Attr{slog.String("domain_id", domainID), slog.String("group_id", groupID)}
	auditAttrs = append(auditAttrs, parentGroupAuditAttrs(b.ParentGroupID, true)...)
	logger.Audit(r.Context(), "group_set_parent", auditAttrs...)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) resourceCreate(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	var b titleBody
	if !readJSON(w, r, &b) {
		return
	}
	res := &store.Resource{ID: uuid.NewString(), DomainID: domainID, Title: b.Title}
	if err := s.Store.ResourceCreate(r.Context(), res); err != nil {
		writeStoreErr(w, r, err)
		return
	}
	logger.Audit(r.Context(), "resource_create", slog.String("domain_id", domainID), slog.String("resource_id", res.ID))
	writeJSON(w, http.StatusCreated, res)
}

func (s *Server) resourceList(w http.ResponseWriter, r *http.Request) {
	opts, err := parseListOpts(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	opts.Sort, opts.Order, err = parseSortOrder(r.URL.Query(), store.ResourceSortFields)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	domainID := chi.URLParam(r, "domainID")
	list, total, err := s.Store.ResourceList(r.Context(), domainID, opts)
	if err != nil {
		writeInternalErr(w, r, err)
		return
	}
	if list == nil {
		list = []store.Resource{}
	}
	writeList(w, list, total, opts)
}

func (s *Server) resourceGet(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	id := chi.URLParam(r, "resourceID")
	res, err := s.Store.ResourceGet(r.Context(), domainID, id)
	if err != nil {
		writeStoreErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) resourcePatch(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	id := chi.URLParam(r, "resourceID")
	var b patchTitleBody
	if !readJSON(w, r, &b) {
		return
	}
	if b.Title == nil {
		writeErr(w, http.StatusBadRequest, errors.New("title is required for patch"))
		return
	}
	res, err := s.Store.ResourcePatch(r.Context(), domainID, id, b.Title)
	if err != nil {
		writeStoreErr(w, r, err)
		return
	}
	logger.Audit(r.Context(), "resource_patch", slog.String("domain_id", domainID), slog.String("resource_id", id))
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) resourceDelete(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	id := chi.URLParam(r, "resourceID")
	if err := s.Store.ResourceDelete(r.Context(), domainID, id); err != nil {
		writeStoreErr(w, r, err)
		return
	}
	logger.Audit(r.Context(), "resource_delete", slog.String("domain_id", domainID), slog.String("resource_id", id))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) accessTypeCreate(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	var b accessTypeBody
	if !readJSON(w, r, &b) {
		return
	}
	bit, err := parseUint64Validated(b.Bit, maxAccessMask)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	a := &store.AccessType{ID: uuid.NewString(), DomainID: domainID, Title: b.Title, Bit: bit}
	if err := s.Store.AccessTypeCreate(r.Context(), a); err != nil {
		writeStoreErr(w, r, err)
		return
	}
	logger.Audit(r.Context(), "access_type_create",
		slog.String("domain_id", domainID),
		slog.String("access_type_id", a.ID),
		slog.Uint64("bit", a.Bit),
	)
	writeJSON(w, http.StatusCreated, a)
}

func (s *Server) accessTypeList(w http.ResponseWriter, r *http.Request) {
	opts, err := parseListOpts(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	opts.Sort, opts.Order, err = parseSortOrder(r.URL.Query(), store.AccessTypeSortFields)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	domainID := chi.URLParam(r, "domainID")
	list, total, err := s.Store.AccessTypeList(r.Context(), domainID, opts)
	if err != nil {
		writeInternalErr(w, r, err)
		return
	}
	if list == nil {
		list = []store.AccessType{}
	}
	writeList(w, list, total, opts)
}

func (s *Server) accessTypeGet(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	id := chi.URLParam(r, "accessTypeID")
	a, err := s.Store.AccessTypeGet(r.Context(), domainID, id)
	if err != nil {
		writeStoreErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func (s *Server) accessTypePatch(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	id := chi.URLParam(r, "accessTypeID")
	var b accessTypePatchBody
	if !readJSON(w, r, &b) {
		return
	}
	if b.Title == nil && b.Bit == nil {
		writeErr(w, http.StatusBadRequest, errors.New("at least one of title, bit is required"))
		return
	}
	params := store.AccessTypePatchParams{Title: b.Title}
	if b.Bit != nil {
		bit, err := parseUint64Validated(*b.Bit, maxAccessMask)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		params.Bit = &bit
	}
	a, err := s.Store.AccessTypePatch(r.Context(), domainID, id, params)
	if err != nil {
		writeStoreErr(w, r, err)
		return
	}
	logger.Audit(r.Context(), "access_type_patch",
		slog.String("domain_id", domainID),
		slog.String("access_type_id", id),
		slog.Uint64("bit", a.Bit),
	)
	writeJSON(w, http.StatusOK, a)
}

func (s *Server) accessTypeDelete(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	id := chi.URLParam(r, "accessTypeID")
	if err := s.Store.AccessTypeDelete(r.Context(), domainID, id); err != nil {
		writeStoreErr(w, r, err)
		return
	}
	logger.Audit(r.Context(), "access_type_delete", slog.String("domain_id", domainID), slog.String("access_type_id", id))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) permissionCreate(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	var b permissionBody
	if !readJSON(w, r, &b) {
		return
	}
	mask, err := parseUint64Validated(b.AccessMask, maxAccessMask)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	p := &store.Permission{
		ID: uuid.NewString(), DomainID: domainID, Title: b.Title,
		ResourceID: b.ResourceID, AccessMask: mask,
	}
	if err := s.Store.PermissionCreate(r.Context(), p); err != nil {
		writeStoreErr(w, r, err)
		return
	}
	logger.Audit(r.Context(), "permission_create",
		slog.String("domain_id", domainID),
		slog.String("permission_id", p.ID),
		slog.String("resource_id", p.ResourceID),
		slog.Uint64("access_mask", p.AccessMask),
	)
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) permissionList(w http.ResponseWriter, r *http.Request) {
	opts, err := parsePermissionListOpts(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	opts.Sort, opts.Order, err = parseSortOrder(r.URL.Query(), store.PermissionSortFields)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	domainID := chi.URLParam(r, "domainID")
	list, total, err := s.Store.PermissionList(r.Context(), domainID, opts)
	if err != nil {
		writeInternalErr(w, r, err)
		return
	}
	if list == nil {
		list = []store.Permission{}
	}
	writeList(w, list, total, opts.ListOpts)
}

func (s *Server) permissionGet(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	id := chi.URLParam(r, "permissionID")
	p, err := s.Store.PermissionGet(r.Context(), domainID, id)
	if err != nil {
		writeStoreErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) permissionPatch(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	id := chi.URLParam(r, "permissionID")
	var b permissionPatchBody
	if !readJSON(w, r, &b) {
		return
	}
	if b.Title == nil && b.ResourceID == nil && b.AccessMask == nil {
		writeErr(w, http.StatusBadRequest, errors.New("at least one of title, resource_id, access_mask is required"))
		return
	}
	params := store.PermissionPatchParams{Title: b.Title, ResourceID: b.ResourceID}
	if b.AccessMask != nil {
		mask, err := parseUint64Validated(*b.AccessMask, maxAccessMask)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		params.AccessMask = &mask
	}
	p, err := s.Store.PermissionPatch(r.Context(), domainID, id, params)
	if err != nil {
		writeStoreErr(w, r, err)
		return
	}
	logger.Audit(r.Context(), "permission_patch",
		slog.String("domain_id", domainID),
		slog.String("permission_id", id),
		slog.String("resource_id", p.ResourceID),
		slog.Uint64("access_mask", p.AccessMask),
	)
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) permissionDelete(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	id := chi.URLParam(r, "permissionID")
	if err := s.Store.PermissionDelete(r.Context(), domainID, id); err != nil {
		writeStoreErr(w, r, err)
		return
	}
	logger.Audit(r.Context(), "permission_delete", slog.String("domain_id", domainID), slog.String("permission_id", id))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) addUserToGroup(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	uid := chi.URLParam(r, "userID")
	gid := chi.URLParam(r, "groupID")
	if err := s.Store.AddUserToGroup(r.Context(), domainID, uid, gid); err != nil {
		writeStoreErr(w, r, err)
		return
	}
	logger.Audit(r.Context(), "add_user_to_group", slog.String("domain_id", domainID), slog.String("user_id", uid), slog.String("group_id", gid))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) removeUserFromGroup(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	uid := chi.URLParam(r, "userID")
	gid := chi.URLParam(r, "groupID")
	if err := s.Store.RemoveUserFromGroup(r.Context(), domainID, uid, gid); err != nil {
		writeStoreErr(w, r, err)
		return
	}
	logger.Audit(r.Context(), "remove_user_from_group", slog.String("domain_id", domainID), slog.String("user_id", uid), slog.String("group_id", gid))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) grantUserPermission(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	uid := chi.URLParam(r, "userID")
	pid := chi.URLParam(r, "permissionID")
	if err := s.Store.GrantUserPermission(r.Context(), domainID, uid, pid); err != nil {
		writeStoreErr(w, r, err)
		return
	}
	logger.Audit(r.Context(), "grant_user_permission", slog.String("domain_id", domainID), slog.String("user_id", uid), slog.String("permission_id", pid))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) revokeUserPermission(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	uid := chi.URLParam(r, "userID")
	pid := chi.URLParam(r, "permissionID")
	if err := s.Store.RevokeUserPermission(r.Context(), domainID, uid, pid); err != nil {
		writeStoreErr(w, r, err)
		return
	}
	logger.Audit(r.Context(), "revoke_user_permission", slog.String("domain_id", domainID), slog.String("user_id", uid), slog.String("permission_id", pid))
	w.WriteHeader(http.StatusNoContent)
}

const userAuthzResourcesSortField = "resource_id"

func (s *Server) userAuthzResources(w http.ResponseWriter, r *http.Request) {
	opts, err := parseOffsetLimitOpts(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	// This endpoint only exposes pagination and uses a fixed stable ordering.
	opts.Sort = userAuthzResourcesSortField
	opts.Order = store.OrderAsc

	domainID := chi.URLParam(r, "domainID")
	uid := chi.URLParam(r, "userID")
	list, total, err := s.Store.UserAuthzResourcesList(r.Context(), domainID, uid, opts)
	if err != nil {
		writeStoreErr(w, r, err)
		return
	}
	writeList(w, userAuthzResourceDTOs(list), total, opts)
}

const groupAuthzResourcesSortField = "resource_id"

func (s *Server) groupAuthzResources(w http.ResponseWriter, r *http.Request) {
	opts, err := parseOffsetLimitOpts(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	opts.Sort = groupAuthzResourcesSortField
	opts.Order = store.OrderAsc

	domainID := chi.URLParam(r, "domainID")
	gid := chi.URLParam(r, "groupID")
	list, total, err := s.Store.GroupAuthzResourcesList(r.Context(), domainID, gid, opts)
	if err != nil {
		writeStoreErr(w, r, err)
		return
	}
	writeList(w, groupAuthzResourceDTOs(list), total, opts)
}

// resourceAuthzUsersSortField is the meta.sort label returned to clients.
// It is the public/JSON name ("user_id") for what the store internally
// orders by (users.id). Both refer to the same column — the store
// ALWAYS uses a fixed ORDER BY users.id ASC for deterministic pagination
// regardless of the opts.Sort/Order values set here; those fields exist
// only so meta.sort/meta.order are populated consistently with other list
// endpoints.
const resourceAuthzUsersSortField = "user_id"

func (s *Server) resourceAuthzUsers(w http.ResponseWriter, r *http.Request) {
	opts, err := parseOffsetLimitOpts(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	// Populated for meta only; the store enforces fixed ORDER BY users.id ASC.
	opts.Sort = resourceAuthzUsersSortField
	opts.Order = store.OrderAsc

	domainID := chi.URLParam(r, "domainID")
	rid := chi.URLParam(r, "resourceID")
	list, total, err := s.Store.ResourceAuthzUsersList(r.Context(), domainID, rid, opts)
	if err != nil {
		writeStoreErr(w, r, err)
		return
	}
	writeList(w, resourceAuthzUserDTOs(list), total, opts)
}

const resourceAuthzGroupsSortField = "group_id"

func (s *Server) resourceAuthzGroups(w http.ResponseWriter, r *http.Request) {
	opts, err := parseOffsetLimitOpts(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	// Populated for meta only; the store enforces fixed ORDER BY group_permissions.group_id ASC.
	opts.Sort = resourceAuthzGroupsSortField
	opts.Order = store.OrderAsc

	domainID := chi.URLParam(r, "domainID")
	rid := chi.URLParam(r, "resourceID")
	list, total, err := s.Store.ResourceAuthzGroupsList(r.Context(), domainID, rid, opts)
	if err != nil {
		writeStoreErr(w, r, err)
		return
	}
	writeList(w, resourceAuthzGroupDTOs(list), total, opts)
}

func (s *Server) grantGroupPermission(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	gid := chi.URLParam(r, "groupID")
	pid := chi.URLParam(r, "permissionID")
	if err := s.Store.GrantGroupPermission(r.Context(), domainID, gid, pid); err != nil {
		writeStoreErr(w, r, err)
		return
	}
	logger.Audit(r.Context(), "grant_group_permission", slog.String("domain_id", domainID), slog.String("group_id", gid), slog.String("permission_id", pid))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) revokeGroupPermission(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	gid := chi.URLParam(r, "groupID")
	pid := chi.URLParam(r, "permissionID")
	if err := s.Store.RevokeGroupPermission(r.Context(), domainID, gid, pid); err != nil {
		writeStoreErr(w, r, err)
		return
	}
	logger.Audit(r.Context(), "revoke_group_permission", slog.String("domain_id", domainID), slog.String("group_id", gid), slog.String("permission_id", pid))
	w.WriteHeader(http.StatusNoContent)
}

// recordAuthz increments AuthzTotal exactly once per request with the
// outcome label (ok/err). Intended to be called from a deferred closure
// that captures the outcome variable. Nil metrics are a no-op so tests
// without a registry still work.
func (s *Server) recordAuthz(result string) {
	if s.metrics == nil {
		return
	}
	s.metrics.AuthzTotal.WithLabelValues(result).Inc()
}

func (s *Server) authzCheck(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	result := authzResultErr
	defer func() { s.recordAuthz(result) }()

	q := r.URL.Query()
	userID := q.Get("user_id")
	resourceID := q.Get("resource_id")
	bitStr := q.Get("access_bit")
	if userID == "" || resourceID == "" || bitStr == "" {
		http.Error(w, "user_id, resource_id, and access_bit are required", http.StatusBadRequest)
		return
	}
	bit, err := parseUint64Validated(bitStr, maxAccessMask)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	mask, err := s.Store.EffectiveMask(r.Context(), domainID, userID, resourceID)
	if err != nil {
		writeStoreErr(w, r, err)
		return
	}
	allowed := access.HasBit(mask, bit)
	result = authzResultOK
	writeJSON(w, http.StatusOK, map[string]any{
		"allowed":        allowed,
		"effective_mask": strconv.FormatUint(mask, 10),
	})
}

func (s *Server) authzMasks(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	result := authzResultErr
	defer func() { s.recordAuthz(result) }()

	q := r.URL.Query()
	userID := q.Get("user_id")
	resourceID := q.Get("resource_id")
	if userID == "" || resourceID == "" {
		http.Error(w, "user_id and resource_id are required", http.StatusBadRequest)
		return
	}
	masks, err := s.Store.PermissionMasksForUserResource(r.Context(), domainID, userID, resourceID)
	if err != nil {
		writeStoreErr(w, r, err)
		return
	}
	result = authzResultOK
	writeJSON(w, http.StatusOK, map[string]any{"masks": masks})
}

// TODO(T54): inject *slog.Logger via Server.Log so tests can capture logs without
// mutating the package-level global (enables t.Parallel()).
// TODO(T55): pass *http.Request to this function so encode-failure logs include
// method and path (depends on T54).
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// The response header is already committed; log for operator visibility.
		logger.Error("response encode failed", slog.String("err", err.Error()))
	}
}

func writeErr(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

// writeStoreErr classifies a store-layer error into the correct HTTP status
// and returns a stable, database-agnostic message. The full error is logged
// server-side so operators can correlate support requests with logs.
func writeStoreErr(w http.ResponseWriter, r *http.Request, err error) {
	var status int
	var msg string
	switch {
	case errors.Is(err, store.ErrNotFound):
		status = http.StatusNotFound
		msg = "resource not found"
	case errors.Is(err, store.ErrFKViolation):
		status = http.StatusBadRequest
		msg = "referenced entity does not exist or is still referenced"
	case errors.Is(err, store.ErrInvalidInput):
		status = http.StatusBadRequest
		msg = publicInvalidInputMsg(err)
	case errors.Is(err, store.ErrConflict):
		status = http.StatusConflict
		msg = "resource already exists"
	default:
		status = http.StatusInternalServerError
		msg = "internal server error"
	}
	logRequestErr(r, status, err)
	writeJSON(w, status, map[string]string{"error": msg})
}

// writeInternalErr logs the full error and returns a generic 500 to the client.
// Intended for read/list operations where the store returns only unexpected DB
// errors, not structured store errors (ErrNotFound, ErrConflict, ErrFKViolation,
// etc.). For single-entity operations use writeStoreErr, which maps those errors
// to appropriate HTTP status codes.
//
// Misuse guard: if a known structured store sentinel is passed here by mistake,
// the function logs an additional ERROR-level alert so the incorrect call site
// is immediately visible in production instead of silently producing a 500 for
// errors that should map to 4xx. The client always receives the generic 500.
func writeInternalErr(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrConflict) ||
		errors.Is(err, store.ErrFKViolation) || errors.Is(err, store.ErrInvalidInput) {
		logger.Error("writeInternalErr misuse: structured store error must use writeStoreErr",
			slog.String("err", err.Error()),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
		)
	}
	logRequestErr(r, http.StatusInternalServerError, err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
}

// logRequestErr logs the error with request context. 5xx errors are logged at
// ERROR level; 4xx at WARN to avoid inflating error-level alerts for expected
// client mistakes.
func logRequestErr(r *http.Request, status int, err error) {
	attrs := []slog.Attr{
		slog.Int("status", status),
		slog.String("err", err.Error()),
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
	}
	if status >= 500 {
		logger.Error("request error", attrs...)
	} else {
		logger.Warn("request error", attrs...)
	}
}

// publicInvalidInputMsg returns a stable, client-safe message for an
// ErrInvalidInput-classed error. It uses errors.As to extract a
// store.InvalidInputError carrying the Detail set at the validation site,
// so intermediate fmt.Errorf("%w", err) wrapping is safe. The mask-overflow
// case is translated to the API's existing wording for backward
// compatibility with clients that key on the message text.
func publicInvalidInputMsg(err error) string {
	var iie *store.InvalidInputError
	if errors.As(err, &iie) && iie != nil && iie.Detail != "" {
		if iie.Detail == store.InvalidInputDetailMaskOverflow {
			return "mask value must be within signed 64-bit range"
		}
		return iie.Detail
	}
	return "invalid request"
}

const maxRequestBodySize = 1 << 20 // 1 MiB

type listMeta struct {
	Total  int64  `json:"total"`
	Offset int    `json:"offset"`
	Limit  int    `json:"limit"`
	Sort   string `json:"sort"`
	Order  string `json:"order"`
}

type listEnvelope struct {
	Data any      `json:"data"`
	Meta listMeta `json:"meta"`
}

func parseListOpts(r *http.Request) (store.ListOpts, error) {
	opts := store.ListOpts{Offset: 0, Limit: store.DefaultLimit}
	q := r.URL.Query()
	if v := q.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return opts, errors.New("offset must be an integer")
		}
		if n < 0 {
			return opts, errors.New("offset must not be negative")
		}
		opts.Offset = n
	}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return opts, errors.New("limit must be an integer")
		}
		if n < 1 {
			n = 1
		}
		if n > store.MaxLimit {
			n = store.MaxLimit
		}
		opts.Limit = n
	}
	opts.Search = strings.TrimSpace(q.Get("search"))
	if utf8.RuneCountInString(opts.Search) > 255 {
		return opts, errors.New("search must be at most 255 characters")
	}
	opts.SearchType = store.SearchContains
	if opts.Search != "" {
		if v := strings.TrimSpace(q.Get("search_type")); v != "" {
			st := store.SearchType(v)
			switch st {
			case store.SearchContains, store.SearchStartsWith, store.SearchEndsWith:
				opts.SearchType = st
			default:
				return opts, errors.New("search_type must be contains, starts_with, or ends_with")
			}
		}
	}
	return opts, nil
}

func parseOffsetLimitOpts(r *http.Request) (store.ListOpts, error) {
	opts := store.ListOpts{Offset: 0, Limit: store.DefaultLimit}
	q := r.URL.Query()
	if _, ok := q["search"]; ok {
		return opts, errors.New("only limit and offset are supported")
	}
	if _, ok := q["search_type"]; ok {
		return opts, errors.New("only limit and offset are supported")
	}
	if _, ok := q["sort"]; ok {
		return opts, errors.New("only limit and offset are supported")
	}
	if _, ok := q["order"]; ok {
		return opts, errors.New("only limit and offset are supported")
	}
	if v := q.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return opts, errors.New("offset must be an integer")
		}
		if n < 0 {
			return opts, errors.New("offset must not be negative")
		}
		opts.Offset = n
	}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return opts, errors.New("limit must be an integer")
		}
		if n < 1 {
			n = 1
		}
		if n > store.MaxLimit {
			n = store.MaxLimit
		}
		opts.Limit = n
	}
	return opts, nil
}

func parseGroupListOpts(r *http.Request) (store.GroupListOpts, error) {
	base, err := parseListOpts(r)
	if err != nil {
		return store.GroupListOpts{}, err
	}
	out := store.GroupListOpts{ListOpts: base}
	if v := strings.TrimSpace(r.URL.Query().Get("parent_group_id")); v != "" {
		out.ParentGroupID = &v
	}
	return out, nil
}

func parsePermissionListOpts(r *http.Request) (store.PermissionListOpts, error) {
	base, err := parseListOpts(r)
	if err != nil {
		return store.PermissionListOpts{}, err
	}
	out := store.PermissionListOpts{ListOpts: base}
	if v := strings.TrimSpace(r.URL.Query().Get("resource_id")); v != "" {
		out.ResourceID = &v
	}
	return out, nil
}

// parseSortOrder reads sort and order query params, validates them against
// the allowed sort fields, and returns the validated values.
func parseSortOrder(q url.Values, allowed []string) (string, store.SortOrder, error) {
	sortField, err := store.ValidateSort(strings.TrimSpace(q.Get("sort")), allowed)
	if err != nil {
		return "", "", err
	}
	order := store.OrderAsc
	if v := strings.TrimSpace(q.Get("order")); v != "" {
		o := store.SortOrder(v)
		switch o {
		case store.OrderAsc, store.OrderDesc:
			order = o
		default:
			return "", "", errors.New("order must be asc or desc")
		}
	}
	return sortField, order, nil
}

func writeList(w http.ResponseWriter, data any, total int64, opts store.ListOpts) {
	writeJSON(w, http.StatusOK, listEnvelope{
		Data: data,
		Meta: listMeta{
			Total:  total,
			Offset: opts.Offset,
			Limit:  opts.Limit,
			Sort:   opts.Sort,
			Order:  string(opts.Order),
		},
	})
}

func readJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			logReadJSONErr(r, "body_too_large", "request body too large")
			writeErr(w, http.StatusRequestEntityTooLarge, errors.New("request body too large"))
			return false
		}
		cls := classifyDecodeErr(err)
		logReadJSONErr(r, cls.kind, cls.logMsg)
		writeErr(w, http.StatusBadRequest, errors.New(cls.clientMsg))
		return false
	}
	// Decode a second value to verify the stream is exhausted. io.EOF means
	// the body was cleanly consumed (trailing whitespace is allowed); any other
	// result — including nil error, meaning a second value decoded successfully
	// — indicates trailing data and is rejected. Using dec.More() is explicitly
	// avoided: its contract is scoped to array/object iteration, not top-level
	// stream exhaustion, and its behaviour outside that scope is undocumented.
	var extra json.RawMessage
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		logReadJSONErr(r, "trailing_data", "trailing data after first JSON value")
		writeErr(w, http.StatusBadRequest, errors.New("request body must contain exactly one JSON value"))
		return false
	}
	return true
}

// decodeClass holds the classification of a JSON decode failure: the
// structured log kind, a sanitized log message (safe to persist), and the
// stable client-facing message.
type decodeClass struct {
	kind      string
	logMsg    string // safe to log (no raw user input)
	clientMsg string
}

// classifyDecodeErr classifies a JSON decode error into a decodeClass.
// Kinds:
//   - empty_body          — io.EOF (no body at all)
//   - json_syntax         — truncated body or syntax error
//   - json_type           — wrong value type for a known field
//   - json_unknown_field  — client sent a field name not in the schema
//   - json_decode         — other decode errors
func classifyDecodeErr(err error) decodeClass {
	var synErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	switch {
	case errors.Is(err, io.EOF):
		return decodeClass{"empty_body", "empty request body", "request body must not be empty"}
	case errors.Is(err, io.ErrUnexpectedEOF), errors.As(err, &synErr):
		return decodeClass{"json_syntax", "malformed JSON in request body", "request body contains malformed JSON"}
	case errors.As(err, &typeErr):
		return decodeClass{"json_type", "invalid field value type in request body", "request body contains an invalid field value"}
	case strings.HasPrefix(err.Error(), "json: unknown field"):
		// Do not log the raw error string: it contains the attacker-controlled
		// field name verbatim (e.g. "json: unknown field \"injected\"").
		return decodeClass{"json_unknown_field", "unknown field in request body", "invalid request body"}
	default:
		return decodeClass{"json_decode", "request body decode error", "invalid request body"}
	}
}

// logReadJSONErr logs a server-side warning for request body parse failures.
// The detail parameter must be a pre-sanitized string — never pass err.Error()
// directly for errors that may contain user-controlled input (e.g. unknown
// field names from DisallowUnknownFields).
func logReadJSONErr(r *http.Request, kind, detail string) {
	logger.Warn("request body decode failed",
		slog.String("kind", kind),
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
		slog.String("detail", detail),
	)
}

// errInvalidNumericValue is the stable, client-safe error returned when a
// query/body field cannot be parsed as a uint64. It intentionally does not
// echo back the raw input or the underlying strconv message.
var errInvalidNumericValue = errors.New("invalid numeric value")

// errAccessMaskOutOfRange is the stable, client-safe error returned when a
// mask/bit value is parseable but exceeds the API's signed-63 limit (see
// issue #67 / T46). Wording must stay backward-compatible with existing
// clients and tests.
var errAccessMaskOutOfRange = errors.New("mask value must be within signed 64-bit range")

// maxAccessMask is the largest mask value permitted by the API until a v2
// migration stores full uint64 values. Bit 63 (1<<63) is reserved to avoid
// signed-64 overflow when masks are persisted in SQLite. See issue #67 / T46.
const maxAccessMask uint64 = 1<<63 - 1

// maxNumericInputLen caps the length of strings accepted by
// parseUint64Validated. The longest legal input is 18 chars (`0x` + 16 hex
// digits) for full uint64; 32 leaves comfortable headroom while bounding
// CPU on pathological inputs without depending on outer body-size limits.
const maxNumericInputLen = 32

// parseUint64Validated parses s as a uint64 in either base 10 (decimal)
// or base 16 (with a `0x`/`0X` prefix), matching the format documented in
// api/openapi.yaml. Other strconv.ParseUint(base=0) modes (octal `0nnn`,
// binary `0b…`) are intentionally rejected so the wire format stays
// unambiguous. When max > 0 the helper also rejects values greater than
// max. The returned errors are stable, client-safe sentinels and never
// embed the user input.
func parseUint64Validated(s string, max uint64) (uint64, error) {
	if len(s) == 0 || len(s) > maxNumericInputLen {
		return 0, errInvalidNumericValue
	}
	base := 10
	digits := s
	if len(s) > 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X') {
		base = 16
		digits = s[2:]
	}
	n, err := strconv.ParseUint(digits, base, 64)
	if err != nil {
		return 0, errInvalidNumericValue
	}
	if max > 0 && n > max {
		return 0, errAccessMaskOutOfRange
	}
	return n, nil
}
