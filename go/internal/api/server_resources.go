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
	writeJSON(w, r, http.StatusCreated, res)
}

func (s *Server) resourceList(w http.ResponseWriter, r *http.Request) {
	opts, err := parseListOpts(r)
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, err)
		return
	}
	opts.Sort, opts.Order, err = parseSortOrder(r.URL.Query(), store.ResourceSortFields)
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, err)
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
	writeList(w, r, list, total, opts)
}

func (s *Server) resourceGet(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	id := chi.URLParam(r, "resourceID")
	res, err := s.Store.ResourceGet(r.Context(), domainID, id)
	if err != nil {
		writeStoreErr(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, res)
}

func (s *Server) resourcePatch(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	id := chi.URLParam(r, "resourceID")
	var b patchTitleBody
	if !readJSON(w, r, &b) {
		return
	}
	if b.Title == nil {
		writeErr(w, r, http.StatusBadRequest, errors.New("title is required for patch"))
		return
	}
	res, err := s.Store.ResourcePatch(r.Context(), domainID, id, b.Title)
	if err != nil {
		writeStoreErr(w, r, err)
		return
	}
	logger.Audit(r.Context(), "resource_patch", slog.String("domain_id", domainID), slog.String("resource_id", id))
	writeJSON(w, r, http.StatusOK, res)
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
