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
	writeJSON(w, r, http.StatusCreated, d)
}

func (s *Server) domainList(w http.ResponseWriter, r *http.Request) {
	opts, err := parseListOpts(r)
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, err)
		return
	}
	opts.Sort, opts.Order, err = parseSortOrder(r.URL.Query(), store.DomainSortFields)
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, err)
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
	writeList(w, r, list, total, opts)
}

func (s *Server) domainGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "domainID")
	d, err := s.Store.DomainGet(r.Context(), id)
	if err != nil {
		writeStoreErr(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, d)
}

func (s *Server) domainPatch(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "domainID")
	var b patchTitleBody
	if !readJSON(w, r, &b) {
		return
	}
	if b.Title == nil {
		writeErr(w, r, http.StatusBadRequest, errors.New("title is required for patch"))
		return
	}
	d, err := s.Store.DomainPatch(r.Context(), id, b.Title)
	if err != nil {
		writeStoreErr(w, r, err)
		return
	}
	logger.Audit(r.Context(), "domain_patch", slog.String("domain_id", id))
	writeJSON(w, r, http.StatusOK, d)
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
