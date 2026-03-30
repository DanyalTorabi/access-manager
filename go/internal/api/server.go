package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/dtorabi/access-manager/internal/access"
	"github.com/dtorabi/access-manager/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Server exposes HTTP handlers for the access manager.
type Server struct {
	Store store.Store
	// APIBearerToken, if non-empty, requires Authorization: Bearer <token> on /api/v1/*.
	// /health stays public. Empty means no auth on API (local dev / loopback only — document in README).
	APIBearerToken string
}

func (s *Server) Router() chi.Router {
	r := chi.NewRouter()
	r.Get("/health", s.health)

	r.Route("/api/v1", func(r chi.Router) {
		if tok := strings.TrimSpace(s.APIBearerToken); tok != "" {
			r.Use(BearerAuth(tok))
		}
		r.Post("/domains", s.domainCreate)
		r.Get("/domains", s.domainList)

		r.Route("/domains/{domainID}", func(r chi.Router) {
			r.Get("/users", s.userList)
			r.Post("/users", s.userCreate)
			r.Get("/users/{userID}", s.userGet)

			r.Get("/groups", s.groupList)
			r.Post("/groups", s.groupCreate)
			r.Get("/groups/{groupID}", s.groupGet)
			r.Patch("/groups/{groupID}/parent", s.groupSetParent)

			r.Get("/resources", s.resourceList)
			r.Post("/resources", s.resourceCreate)
			r.Get("/resources/{resourceID}", s.resourceGet)

			r.Get("/access-types", s.accessTypeList)
			r.Post("/access-types", s.accessTypeCreate)

			r.Get("/permissions", s.permissionList)
			r.Post("/permissions", s.permissionCreate)
			r.Get("/permissions/{permissionID}", s.permissionGet)

			r.Post("/users/{userID}/groups/{groupID}", s.addUserToGroup)
			r.Delete("/users/{userID}/groups/{groupID}", s.removeUserFromGroup)

			r.Post("/users/{userID}/permissions/{permissionID}", s.grantUserPermission)
			r.Delete("/users/{userID}/permissions/{permissionID}", s.revokeUserPermission)

			r.Post("/groups/{groupID}/permissions/{permissionID}", s.grantGroupPermission)
			r.Delete("/groups/{groupID}/permissions/{permissionID}", s.revokeGroupPermission)

			r.Get("/authz/check", s.authzCheck)
			r.Get("/authz/masks", s.authzMasks)
		})
	})
	return r
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type titleBody struct {
	Title string `json:"title"`
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

func (s *Server) domainCreate(w http.ResponseWriter, r *http.Request) {
	var b titleBody
	if !readJSON(w, r, &b) {
		return
	}
	d := &store.Domain{ID: uuid.NewString(), Title: b.Title}
	if err := s.Store.DomainCreate(r.Context(), d); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
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

func (s *Server) userCreate(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	var b titleBody
	if !readJSON(w, r, &b) {
		return
	}
	u := &store.User{ID: uuid.NewString(), DomainID: domainID, Title: b.Title}
	if err := s.Store.UserCreate(r.Context(), u); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
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
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
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
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
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
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
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
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
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

func (s *Server) addUserToGroup(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	uid := chi.URLParam(r, "userID")
	gid := chi.URLParam(r, "groupID")
	if err := s.Store.AddUserToGroup(r.Context(), domainID, uid, gid); err != nil {
		writeStoreErr(w, r, err)
		return
	}
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
	mask, err := s.Store.EffectiveMask(r.Context(), domainID, userID, resourceID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
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
	masks, err := s.Store.PermissionMasksForUserResource(r.Context(), domainID, userID, resourceID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
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

func readJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return false
	}
	return true
}

func parseUint64(s string) (uint64, error) {
	return strconv.ParseUint(s, 0, 64)
}
