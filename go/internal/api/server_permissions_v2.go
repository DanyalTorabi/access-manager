package api

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/dtorabi/access-manager/internal/access"
	"github.com/dtorabi/access-manager/internal/logger"
	"github.com/dtorabi/access-manager/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// permissionBodyV2 is the request body for POST /api/v2/.../permissions.
// Instead of a raw access_mask integer, callers supply a slice of
// access-type titles registered for the domain.
type permissionBodyV2 struct {
	Title       string   `json:"title"`
	ResourceID  string   `json:"resource_id"`
	Permissions []string `json:"permissions"`
}

// permissionPatchBodyV2 is the PATCH body for /api/v2/.../permissions/{id}.
type permissionPatchBodyV2 struct {
	Title       *string   `json:"title"`
	ResourceID  *string   `json:"resource_id"`
	Permissions *[]string `json:"permissions"`
}

// permissionResponseV2 is the JSON body returned by v2 permission endpoints.
type permissionResponseV2 struct {
	ID          string   `json:"id"`
	DomainID    string   `json:"domain_id"`
	Title       string   `json:"title"`
	ResourceID  string   `json:"resource_id"`
	Permissions []string `json:"permissions"` // sorted titles
}

// toPermissionResponseV2 converts a store.Permission to its v2 representation
// by translating the access mask into a sorted title slice.
func toPermissionResponseV2(p *store.Permission, types []store.AccessType) permissionResponseV2 {
	return permissionResponseV2{
		ID:          p.ID,
		DomainID:    p.DomainID,
		Title:       p.Title,
		ResourceID:  p.ResourceID,
		Permissions: access.MaskToTitles(p.AccessMask, types),
	}
}

func (s *Server) permissionCreateV2(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	var b permissionBodyV2
	if !readJSON(w, r, &b) {
		return
	}

	types, err := s.loadDomainAccessTypes(r, domainID)
	if err != nil {
		writeInternalErr(w, r, err)
		return
	}

	mask, err := access.TitlesToMask(b.Permissions, types)
	if err != nil {
		var ute *access.UnknownTitleError
		if errors.As(err, &ute) {
			writeErr(w, r, http.StatusBadRequest, err)
			return
		}
		writeInternalErr(w, r, err)
		return
	}

	p := &store.Permission{
		ID:         uuid.NewString(),
		DomainID:   domainID,
		Title:      b.Title,
		ResourceID: b.ResourceID,
		AccessMask: mask,
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
		slog.String("api_version", "v2"),
	)
	writeJSON(w, r, http.StatusCreated, toPermissionResponseV2(p, types))
}

func (s *Server) permissionGetV2(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	id := chi.URLParam(r, "permissionID")

	p, err := s.Store.PermissionGet(r.Context(), domainID, id)
	if err != nil {
		writeStoreErr(w, r, err)
		return
	}

	types, err := s.loadDomainAccessTypes(r, domainID)
	if err != nil {
		writeInternalErr(w, r, err)
		return
	}

	writeJSON(w, r, http.StatusOK, toPermissionResponseV2(p, types))
}

func (s *Server) permissionListV2(w http.ResponseWriter, r *http.Request) {
	opts, err := parsePermissionListOpts(r)
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, err)
		return
	}
	opts.Sort, opts.Order, err = parseSortOrder(r.URL.Query(), store.PermissionSortFields)
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, err)
		return
	}
	domainID := chi.URLParam(r, "domainID")

	list, total, err := s.Store.PermissionList(r.Context(), domainID, opts)
	if err != nil {
		writeInternalErr(w, r, err)
		return
	}

	types, err := s.loadDomainAccessTypes(r, domainID)
	if err != nil {
		writeInternalErr(w, r, err)
		return
	}

	dtos := make([]permissionResponseV2, 0, len(list))
	for i := range list {
		dtos = append(dtos, toPermissionResponseV2(&list[i], types))
	}
	writeList(w, r, dtos, total, opts.ListOpts)
}

func (s *Server) permissionPatchV2(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	id := chi.URLParam(r, "permissionID")
	var b permissionPatchBodyV2
	if !readJSON(w, r, &b) {
		return
	}
	if b.Title == nil && b.ResourceID == nil && b.Permissions == nil {
		writeErr(w, r, http.StatusBadRequest, errors.New("at least one of title, resource_id, permissions is required"))
		return
	}

	params := store.PermissionPatchParams{Title: b.Title, ResourceID: b.ResourceID}

	if b.Permissions != nil {
		types, err := s.loadDomainAccessTypes(r, domainID)
		if err != nil {
			writeInternalErr(w, r, err)
			return
		}
		mask, err := access.TitlesToMask(*b.Permissions, types)
		if err != nil {
			var ute *access.UnknownTitleError
			if errors.As(err, &ute) {
				writeErr(w, r, http.StatusBadRequest, err)
				return
			}
			writeInternalErr(w, r, err)
			return
		}
		params.AccessMask = &mask
	}

	p, err := s.Store.PermissionPatch(r.Context(), domainID, id, params)
	if err != nil {
		writeStoreErr(w, r, err)
		return
	}

	types, err := s.loadDomainAccessTypes(r, domainID)
	if err != nil {
		writeInternalErr(w, r, err)
		return
	}

	logger.Audit(r.Context(), "permission_patch",
		slog.String("domain_id", domainID),
		slog.String("permission_id", id),
		slog.String("resource_id", p.ResourceID),
		slog.Uint64("access_mask", p.AccessMask),
		slog.String("api_version", "v2"),
	)
	writeJSON(w, r, http.StatusOK, toPermissionResponseV2(p, types))
}
