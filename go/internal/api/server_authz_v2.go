package api

import (
	"net/http"

	"github.com/dtorabi/access-manager/internal/access"
	"github.com/dtorabi/access-manager/internal/store"
	"github.com/go-chi/chi/v5"
)

// userAuthzResourceV2 is the JSON response row for
// GET /api/v2/domains/{domainID}/users/{userID}/authz/resources.
type userAuthzResourceV2 struct {
	ResourceID  string   `json:"resource_id"`
	Permissions []string `json:"permissions"` // sorted titles
}

// groupAuthzResourceV2 is the JSON response row for
// GET /api/v2/domains/{domainID}/groups/{groupID}/authz/resources.
type groupAuthzResourceV2 struct {
	ResourceID  string   `json:"resource_id"`
	Permissions []string `json:"permissions"` // sorted titles
}

// resourceAuthzUserV2 is the JSON response row for
// GET /api/v2/domains/{domainID}/resources/{resourceID}/authz/users.
type resourceAuthzUserV2 struct {
	UserID      string   `json:"user_id"`
	Permissions []string `json:"permissions"` // sorted titles
}

// resourceAuthzGroupV2 is the JSON response row for
// GET /api/v2/domains/{domainID}/resources/{resourceID}/authz/groups.
type resourceAuthzGroupV2 struct {
	GroupID     string   `json:"group_id"`
	Permissions []string `json:"permissions"` // sorted titles
}

// userResourcePermissionsV2Response is the body for
// GET /api/v2/domains/{domainID}/users/{userID}/resources/{resourceID}/permissions.
type userResourcePermissionsV2Response struct {
	Permissions []string `json:"permissions"` // sorted effective titles
}

func (s *Server) userAuthzResourcesV2(w http.ResponseWriter, r *http.Request) {
	opts, err := parseOffsetLimitOpts(r)
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, err)
		return
	}
	opts.Sort = userAuthzResourcesSortField
	opts.Order = store.OrderAsc

	domainID := chi.URLParam(r, "domainID")
	uid := chi.URLParam(r, "userID")

	list, total, err := s.Store.UserAuthzResourcesList(r.Context(), domainID, uid, opts)
	if err != nil {
		writeStoreErr(w, r, err)
		return
	}

	types, err := s.loadDomainAccessTypes(r, domainID)
	if err != nil {
		writeInternalErr(w, r, err)
		return
	}

	dtos := make([]userAuthzResourceV2, 0, len(list))
	for _, it := range list {
		// Users report EffectiveMask: the union of their direct permissions plus
		// those inherited through group membership. This is the true set of
		// permissions the user has on the resource.
		dtos = append(dtos, userAuthzResourceV2{
			ResourceID:  it.ResourceID,
			Permissions: access.MaskToTitles(it.EffectiveMask, types),
		})
	}
	writeList(w, r, dtos, total, opts)
}

func (s *Server) groupAuthzResourcesV2(w http.ResponseWriter, r *http.Request) {
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

	types, err := s.loadDomainAccessTypes(r, domainID)
	if err != nil {
		writeInternalErr(w, r, err)
		return
	}

	dtos := make([]groupAuthzResourceV2, 0, len(list))
	for _, it := range list {
		// Groups report Mask: their direct permissions on the resource.
		// This is NOT the union of member permissions (that would be computed
		// per-member and is returned in userAuthzResourcesV2 as EffectiveMask).
		dtos = append(dtos, groupAuthzResourceV2{
			ResourceID:  it.ResourceID,
			Permissions: access.MaskToTitles(it.Mask, types),
		})
	}
	writeList(w, r, dtos, total, opts)
}

func (s *Server) resourceAuthzUsersV2(w http.ResponseWriter, r *http.Request) {
	opts, err := parseOffsetLimitOpts(r)
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, err)
		return
	}
	opts.Sort = resourceAuthzUsersSortField
	opts.Order = store.OrderAsc

	domainID := chi.URLParam(r, "domainID")
	rid := chi.URLParam(r, "resourceID")

	list, total, err := s.Store.ResourceAuthzUsersList(r.Context(), domainID, rid, opts)
	if err != nil {
		writeStoreErr(w, r, err)
		return
	}

	types, err := s.loadDomainAccessTypes(r, domainID)
	if err != nil {
		writeInternalErr(w, r, err)
		return
	}

	dtos := make([]resourceAuthzUserV2, 0, len(list))
	for _, it := range list {
		// Users report EffectiveMask: the union of their direct permissions plus
		// those inherited through group membership. This is the true set of
		// permissions the user has on this resource.
		dtos = append(dtos, resourceAuthzUserV2{
			UserID:      it.UserID,
			Permissions: access.MaskToTitles(it.EffectiveMask, types),
		})
	}
	writeList(w, r, dtos, total, opts)
}

func (s *Server) resourceAuthzGroupsV2(w http.ResponseWriter, r *http.Request) {
	opts, err := parseOffsetLimitOpts(r)
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, err)
		return
	}
	opts.Sort = resourceAuthzGroupsSortField
	opts.Order = store.OrderAsc

	domainID := chi.URLParam(r, "domainID")
	rid := chi.URLParam(r, "resourceID")

	list, total, err := s.Store.ResourceAuthzGroupsList(r.Context(), domainID, rid, opts)
	if err != nil {
		writeStoreErr(w, r, err)
		return
	}

	types, err := s.loadDomainAccessTypes(r, domainID)
	if err != nil {
		writeInternalErr(w, r, err)
		return
	}

	dtos := make([]resourceAuthzGroupV2, 0, len(list))
	for _, it := range list {
		// Groups report Mask: their direct permissions on this resource.
		// This is NOT the union of member permissions.
		dtos = append(dtos, resourceAuthzGroupV2{
			GroupID:     it.GroupID,
			Permissions: access.MaskToTitles(it.Mask, types),
		})
	}
	writeList(w, r, dtos, total, opts)
}

// userResourcePermissionsV2 returns the effective permission titles for a
// specific user on a specific resource. Unlike the v1 authz/masks endpoint
// which returns raw masks, this endpoint returns a sorted title slice.
func (s *Server) userResourcePermissionsV2(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	uid := chi.URLParam(r, "userID")
	rid := chi.URLParam(r, "resourceID")

	mask, err := s.Store.EffectiveMask(r.Context(), domainID, uid, rid)
	if err != nil {
		writeStoreErr(w, r, err)
		return
	}

	types, err := s.loadDomainAccessTypes(r, domainID)
	if err != nil {
		writeInternalErr(w, r, err)
		return
	}

	writeJSON(w, r, http.StatusOK, userResourcePermissionsV2Response{
		Permissions: access.MaskToTitles(mask, types),
	})
}
