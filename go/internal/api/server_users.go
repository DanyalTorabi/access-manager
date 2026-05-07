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
	writeJSON(w, r, http.StatusCreated, u)
}

func (s *Server) userList(w http.ResponseWriter, r *http.Request) {
	opts, err := parseListOpts(r)
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, err)
		return
	}
	opts.Sort, opts.Order, err = parseSortOrder(r.URL.Query(), store.UserSortFields)
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, err)
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
	writeList(w, r, list, total, opts)
}

func (s *Server) userGet(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	id := chi.URLParam(r, "userID")
	u, err := s.Store.UserGet(r.Context(), domainID, id)
	if err != nil {
		writeStoreErr(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, u)
}

func (s *Server) userPatch(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	id := chi.URLParam(r, "userID")
	var b patchTitleBody
	if !readJSON(w, r, &b) {
		return
	}
	if b.Title == nil {
		writeErr(w, r, http.StatusBadRequest, errors.New("title is required for patch"))
		return
	}
	u, err := s.Store.UserPatch(r.Context(), domainID, id, b.Title)
	if err != nil {
		writeStoreErr(w, r, err)
		return
	}
	logger.Audit(r.Context(), "user_patch", slog.String("domain_id", domainID), slog.String("user_id", id))
	writeJSON(w, r, http.StatusOK, u)
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
