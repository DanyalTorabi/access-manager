package api

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/dtorabi/access-manager/internal/logger"
	"github.com/dtorabi/access-manager/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type permissionBody struct {
	Title      string `json:"title"`
	ResourceID string `json:"resource_id"`
	AccessMask string `json:"access_mask"` // decimal or 0x hex
}

type permissionPatchBody struct {
	Title      *string `json:"title"`
	ResourceID *string `json:"resource_id"`
	AccessMask *string `json:"access_mask"`
}

func (s *Server) permissionCreate(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	var b permissionBody
	if !readJSON(w, r, &b) {
		return
	}
	mask, err := parseUint64Validated(b.AccessMask, maxAccessMask)
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, err)
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
	writeJSON(w, r, http.StatusCreated, p)
}

func (s *Server) permissionList(w http.ResponseWriter, r *http.Request) {
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
	if list == nil {
		list = []store.Permission{}
	}
	writeList(w, r, list, total, opts.ListOpts)
}

func (s *Server) permissionGet(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	id := chi.URLParam(r, "permissionID")
	p, err := s.Store.PermissionGet(r.Context(), domainID, id)
	if err != nil {
		writeStoreErr(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, p)
}

func (s *Server) permissionPatch(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	id := chi.URLParam(r, "permissionID")
	var b permissionPatchBody
	if !readJSON(w, r, &b) {
		return
	}
	if b.Title == nil && b.ResourceID == nil && b.AccessMask == nil {
		writeErr(w, r, http.StatusBadRequest, errors.New("at least one of title, resource_id, access_mask is required"))
		return
	}
	params := store.PermissionPatchParams{Title: b.Title, ResourceID: b.ResourceID}
	if b.AccessMask != nil {
		mask, err := parseUint64Validated(*b.AccessMask, maxAccessMask)
		if err != nil {
			writeErr(w, r, http.StatusBadRequest, err)
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
	writeJSON(w, r, http.StatusOK, p)
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
