package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

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

		r.Post("/domains/{domainID}/groups/{groupID}/permissions/{permissionID}", s.grantGroupPermission)
		r.Delete("/domains/{domainID}/groups/{groupID}/permissions/{permissionID}", s.revokeGroupPermission)

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
	Title       *string `json:"title"`
	ResourceID  *string `json:"resource_id"`
	AccessMask  *string `json:"access_mask"`
}

type permissionBody struct {
	Title       string `json:"title"`
	ResourceID  string `json:"resource_id"`
	AccessMask  string `json:"access_mask"` // decimal or 0x hex
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
	list, err := s.Store.DomainList(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if list == nil {
		list = []store.Domain{}
	}
	writeJSON(w, http.StatusOK, list)
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
	domainID := chi.URLParam(r, "domainID")
	list, err := s.Store.UserList(r.Context(), domainID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if list == nil {
		list = []store.User{}
	}
	writeJSON(w, http.StatusOK, list)
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
		Title           string  `json:"title"`
		ParentGroupID   *string `json:"parent_group_id"`
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
	domainID := chi.URLParam(r, "domainID")
	list, err := s.Store.GroupList(r.Context(), domainID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if list == nil {
		list = []store.Group{}
	}
	writeJSON(w, http.StatusOK, list)
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
				writeErr(w, http.StatusBadRequest, err)
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
	domainID := chi.URLParam(r, "domainID")
	list, err := s.Store.ResourceList(r.Context(), domainID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if list == nil {
		list = []store.Resource{}
	}
	writeJSON(w, http.StatusOK, list)
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
	bit, err := parseUint64(b.Bit)
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
	domainID := chi.URLParam(r, "domainID")
	list, err := s.Store.AccessTypeList(r.Context(), domainID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if list == nil {
		list = []store.AccessType{}
	}
	writeJSON(w, http.StatusOK, list)
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
		bit, err := parseUint64(*b.Bit)
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
	mask, err := parseUint64(b.AccessMask)
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
	domainID := chi.URLParam(r, "domainID")
	list, err := s.Store.PermissionList(r.Context(), domainID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if list == nil {
		list = []store.Permission{}
	}
	writeJSON(w, http.StatusOK, list)
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
		mask, err := parseUint64(*b.AccessMask)
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

func (s *Server) authzCheck(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	q := r.URL.Query()
	userID := q.Get("user_id")
	resourceID := q.Get("resource_id")
	bitStr := q.Get("access_bit")
	if userID == "" || resourceID == "" || bitStr == "" {
		http.Error(w, "user_id, resource_id, and access_bit are required", http.StatusBadRequest)
		return
	}
	bit, err := parseUint64(bitStr)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if s.metrics != nil {
		s.metrics.AuthzTotal.WithLabelValues(domainID).Inc()
	}
	mask, err := s.Store.EffectiveMask(r.Context(), domainID, userID, resourceID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if s.metrics != nil {
		s.metrics.AuthzTotal.WithLabelValues(domainID).Inc()
	}
	allowed := access.HasBit(mask, bit)
	writeJSON(w, http.StatusOK, map[string]any{
		"allowed":        allowed,
		"effective_mask": strconv.FormatUint(mask, 10),
	})
}

func (s *Server) authzMasks(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	q := r.URL.Query()
	userID := q.Get("user_id")
	resourceID := q.Get("resource_id")
	if userID == "" || resourceID == "" {
		http.Error(w, "user_id and resource_id are required", http.StatusBadRequest)
		return
	}
	if s.metrics != nil {
		s.metrics.AuthzTotal.WithLabelValues(domainID).Inc()
	}
	masks, err := s.Store.PermissionMasksForUserResource(r.Context(), domainID, userID, resourceID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if s.metrics != nil {
		s.metrics.AuthzTotal.WithLabelValues(domainID).Inc()
	}
	writeJSON(w, http.StatusOK, map[string]any{"masks": masks})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

// writeStoreErr classifies a store-layer error into the correct HTTP status:
// ErrNotFound → 404, ErrFKViolation/ErrInvalidInput → 400, ErrConflict → 409,
// everything else → 500.
func writeStoreErr(w http.ResponseWriter, _ *http.Request, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeErr(w, http.StatusNotFound, err)
	case errors.Is(err, store.ErrFKViolation):
		writeErr(w, http.StatusBadRequest, err)
	case errors.Is(err, store.ErrInvalidInput):
		writeErr(w, http.StatusBadRequest, err)
	case errors.Is(err, store.ErrConflict):
		writeErr(w, http.StatusConflict, err)
	default:
		writeErr(w, http.StatusInternalServerError, err)
	}
}

const maxRequestBodySize = 1 << 20 // 1 MiB

func readJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeErr(w, http.StatusRequestEntityTooLarge, errors.New("request body too large"))
			return false
		}
		writeErr(w, http.StatusBadRequest, err)
		return false
	}
	return true
}

func parseUint64(s string) (uint64, error) {
	return strconv.ParseUint(s, 0, 64)
}
