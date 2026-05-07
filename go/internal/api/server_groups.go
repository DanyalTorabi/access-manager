package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/dtorabi/access-manager/internal/logger"
	"github.com/dtorabi/access-manager/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (s *Server) groupCreate(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	var b struct {
		Title         string  `json:"title"`
		ParentGroupID *string `json:"parent_group_id"`
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
	writeJSON(w, r, http.StatusCreated, g)
}

func (s *Server) groupList(w http.ResponseWriter, r *http.Request) {
	opts, err := parseGroupListOpts(r)
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, err)
		return
	}
	opts.Sort, opts.Order, err = parseSortOrder(r.URL.Query(), store.GroupSortFields)
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, err)
		return
	}
	domainID := chi.URLParam(r, "domainID")
	list, total, err := s.Store.GroupList(r.Context(), domainID, opts)
	if err != nil {
		writeInternalErr(w, r, err)
		return
	}
	if list == nil {
		list = []store.Group{}
	}
	writeList(w, r, list, total, opts.ListOpts)
}

func (s *Server) groupGet(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	id := chi.URLParam(r, "groupID")
	g, err := s.Store.GroupGet(r.Context(), domainID, id)
	if err != nil {
		writeStoreErr(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, g)
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
				writeErr(w, r, http.StatusBadRequest, errors.New("parent_group_id must be a UUID string or null"))
				return
			}
			params.ParentGroupID = &pid
		}
	}
	if params.Title == nil && !params.UpdateParent {
		writeErr(w, r, http.StatusBadRequest, errors.New("at least one of title, parent_group_id is required"))
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
	writeJSON(w, r, http.StatusOK, g)
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
