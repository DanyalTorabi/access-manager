package api

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/dtorabi/access-manager/internal/access"
	"github.com/dtorabi/access-manager/internal/logger"
	"github.com/dtorabi/access-manager/internal/store"
	"github.com/go-chi/chi/v5"
)

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
		writeErr(w, r, http.StatusBadRequest, err)
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
	writeList(w, r, userAuthzResourceDTOs(list), total, opts)
}

const groupAuthzResourcesSortField = "resource_id"

func (s *Server) groupAuthzResources(w http.ResponseWriter, r *http.Request) {
	opts, err := parseOffsetLimitOpts(r)
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, err)
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
	writeList(w, r, groupAuthzResourceDTOs(list), total, opts)
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
		writeErr(w, r, http.StatusBadRequest, err)
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
	writeList(w, r, resourceAuthzUserDTOs(list), total, opts)
}

const resourceAuthzGroupsSortField = "group_id"

func (s *Server) resourceAuthzGroups(w http.ResponseWriter, r *http.Request) {
	opts, err := parseOffsetLimitOpts(r)
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, err)
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
	writeList(w, r, resourceAuthzGroupDTOs(list), total, opts)
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
		writeErr(w, r, http.StatusBadRequest, err)
		return
	}
	mask, err := s.Store.EffectiveMask(r.Context(), domainID, userID, resourceID)
	if err != nil {
		writeStoreErr(w, r, err)
		return
	}
	allowed := access.HasBit(mask, bit)
	result = authzResultOK
	writeJSON(w, r, http.StatusOK, map[string]any{
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
	writeJSON(w, r, http.StatusOK, map[string]any{"masks": masks})
}
