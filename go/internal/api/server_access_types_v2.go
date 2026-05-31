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

// accessTypeBodyV2 is the request body for POST /api/v2/.../access-types.
// Unlike the v1 body, Bit is optional: when omitted the server allocates the
// lowest unused bit in the domain automatically.
type accessTypeBodyV2 struct {
	Title string  `json:"title"`
	Bit   *string `json:"bit"` // decimal or 0x hex; omit to auto-allocate
}

// loadDomainAccessTypes returns all access types for domainID. At most 63
// rows can exist per domain (one per available bit).
func (s *Server) loadDomainAccessTypes(r *http.Request, domainID string) ([]store.AccessType, error) {
	list, _, err := s.Store.AccessTypeList(r.Context(), domainID, store.ListOpts{Limit: 63})
	return list, err
}

func (s *Server) accessTypeCreateV2(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	var b accessTypeBodyV2
	if !readJSON(w, r, &b) {
		return
	}

	var bit uint64
	if b.Bit != nil {
		// Caller supplied an explicit bit value — validate and use it as-is.
		var err error
		bit, err = parseUint64Validated(*b.Bit, maxAccessMask)
		if err != nil {
			writeErr(w, r, http.StatusBadRequest, err)
			return
		}
	} else {
		// Auto-allocate the lowest unused bit in this domain.
		types, err := s.loadDomainAccessTypes(r, domainID)
		if err != nil {
			writeInternalErr(w, r, err)
			return
		}
		bit, err = access.AllocateNextBit(types)
		if err != nil {
			if errors.Is(err, access.ErrBitsExhausted) {
				writeErr(w, r, http.StatusConflict, errors.New("all 63 permission bits are exhausted for this domain"))
				return
			}
			writeInternalErr(w, r, err)
			return
		}
	}

	a := &store.AccessType{ID: uuid.NewString(), DomainID: domainID, Title: b.Title, Bit: bit}
	if err := s.Store.AccessTypeCreate(r.Context(), a); err != nil {
		writeStoreErr(w, r, err)
		return
	}
	logger.Audit(r.Context(), "access_type_create",
		slog.String("domain_id", domainID),
		slog.String("access_type_id", a.ID),
		slog.Uint64("bit", a.Bit),
		slog.String("api_version", "v2"),
	)
	writeJSON(w, r, http.StatusCreated, a)
}
