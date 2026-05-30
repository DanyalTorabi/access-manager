package api

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/dtorabi/access-manager/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type accessTypeBody struct {
	Title string `json:"title"`
	Bit   string `json:"bit"` // decimal or 0x hex
}

type accessTypePatchBody struct {
	Title *string `json:"title"`
	Bit   *string `json:"bit"`
}

func (s *Server) accessTypeCreate(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	var b accessTypeBody
	if !s.readJSON(w, r, &b) {
		return
	}
	bit, err := parseUint64Validated(b.Bit, maxAccessMask)
	if err != nil {
		s.writeErr(w, r, http.StatusBadRequest, err)
		return
	}
	a := &store.AccessType{ID: uuid.NewString(), DomainID: domainID, Title: b.Title, Bit: bit}
	if err := s.Store.AccessTypeCreate(r.Context(), a); err != nil {
		s.writeStoreErr(w, r, err)
		return
	}
	s.auditLog(r.Context(), "access_type_create",
		slog.String("domain_id", domainID),
		slog.String("access_type_id", a.ID),
		slog.Uint64("bit", a.Bit),
	)
	s.writeJSON(w, r, http.StatusCreated, a)
}

func (s *Server) accessTypeList(w http.ResponseWriter, r *http.Request) {
	opts, err := parseListOpts(r)
	if err != nil {
		s.writeErr(w, r, http.StatusBadRequest, err)
		return
	}
	opts.Sort, opts.Order, err = parseSortOrder(r.URL.Query(), store.AccessTypeSortFields)
	if err != nil {
		s.writeErr(w, r, http.StatusBadRequest, err)
		return
	}
	domainID := chi.URLParam(r, "domainID")
	list, total, err := s.Store.AccessTypeList(r.Context(), domainID, opts)
	if err != nil {
		s.writeInternalErr(w, r, err)
		return
	}
	if list == nil {
		list = []store.AccessType{}
	}
	s.writeList(w, r, list, total, opts)
}

func (s *Server) accessTypeGet(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	id := chi.URLParam(r, "accessTypeID")
	a, err := s.Store.AccessTypeGet(r.Context(), domainID, id)
	if err != nil {
		s.writeStoreErr(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, a)
}

func (s *Server) accessTypePatch(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	id := chi.URLParam(r, "accessTypeID")
	var b accessTypePatchBody
	if !s.readJSON(w, r, &b) {
		return
	}
	if b.Title == nil && b.Bit == nil {
		s.writeErr(w, r, http.StatusBadRequest, errors.New("at least one of title, bit is required"))
		return
	}
	params := store.AccessTypePatchParams{Title: b.Title}
	if b.Bit != nil {
		bit, err := parseUint64Validated(*b.Bit, maxAccessMask)
		if err != nil {
			s.writeErr(w, r, http.StatusBadRequest, err)
			return
		}
		params.Bit = &bit
	}
	a, err := s.Store.AccessTypePatch(r.Context(), domainID, id, params)
	if err != nil {
		s.writeStoreErr(w, r, err)
		return
	}
	s.auditLog(r.Context(), "access_type_patch",
		slog.String("domain_id", domainID),
		slog.String("access_type_id", id),
		slog.Uint64("bit", a.Bit),
	)
	s.writeJSON(w, r, http.StatusOK, a)
}

func (s *Server) accessTypeDelete(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	id := chi.URLParam(r, "accessTypeID")
	if err := s.Store.AccessTypeDelete(r.Context(), domainID, id); err != nil {
		s.writeStoreErr(w, r, err)
		return
	}
	s.auditLog(r.Context(), "access_type_delete", slog.String("domain_id", domainID), slog.String("access_type_id", id))
	w.WriteHeader(http.StatusNoContent)
}
