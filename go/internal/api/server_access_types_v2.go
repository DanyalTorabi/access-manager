package api

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/dtorabi/access-manager/internal/access"
	"github.com/dtorabi/access-manager/internal/logger"
	"github.com/dtorabi/access-manager/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const maxAccessTypesPerDomain = 63 // Maximum number of permission bits per domain (0-62)

// accessTypeBodyV2 is the request body for POST /api/v2/.../access-types.
// Unlike the v1 body, Bit is optional: when omitted the server allocates the
// lowest unused bit in the domain automatically.
type accessTypeBodyV2 struct {
	Title string  `json:"title"`
	Bit   *string `json:"bit"` // decimal or 0x hex; omit to auto-allocate
}

// accessTypeResponseV2 is the response body for access-type endpoints in v2.
type accessTypeResponseV2 struct {
	ID       string `json:"id"`
	DomainID string `json:"domain_id"`
	Title    string `json:"title"`
	Bit      uint64 `json:"bit"`
}

// loadDomainAccessTypes returns all access types for domainID. At most
// maxAccessTypesPerDomain rows can exist per domain (one per available bit).
func (s *Server) loadDomainAccessTypes(r *http.Request, domainID string) ([]store.AccessType, error) {
	list, _, err := s.Store.AccessTypeList(r.Context(), domainID, store.ListOpts{Limit: maxAccessTypesPerDomain})
	return list, err
}

func (s *Server) accessTypeCreateV2(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	var b accessTypeBodyV2
	if !readJSON(w, r, &b) {
		return
	}

	// Validate title is non-empty.
	if strings.TrimSpace(b.Title) == "" {
		writeErr(w, r, http.StatusBadRequest, errors.New("title is required"))
		return
	}

	var bit uint64
	autoAlloc := b.Bit == nil
	if b.Bit != nil {
		// Caller supplied an explicit bit value — validate and use it as-is.
		var err error
		bit, err = parseUint64Validated(*b.Bit, maxAccessMask)
		if err != nil {
			writeErr(w, r, http.StatusBadRequest, err)
			return
		}
		// Validate that bit is a non-zero power of two.
		if bit == 0 || (bit&(bit-1)) != 0 {
			writeErr(w, r, http.StatusBadRequest, errors.New("bit must be a non-zero power of two"))
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

	// Try creating the access type. If auto-allocated and we hit a conflict due to
	// a concurrent request also allocating the same bit, retry up to 2 times by
	// re-reading the domain types and re-running allocation.
	const maxRetries = 2
	for attempt := 0; attempt <= maxRetries; attempt++ {
		a := &store.AccessType{ID: uuid.NewString(), DomainID: domainID, Title: b.Title, Bit: bit}
		err := s.Store.AccessTypeCreate(r.Context(), a)
		if err == nil {
			logger.Audit(r.Context(), "access_type_create",
				slog.String("domain_id", domainID),
				slog.String("access_type_id", a.ID),
				slog.Uint64("bit", a.Bit),
				slog.String("api_version", "v2"),
			)
			writeJSON(w, r, http.StatusCreated, &accessTypeResponseV2{
				ID:       a.ID,
				DomainID: a.DomainID,
				Title:    a.Title,
				Bit:      a.Bit,
			})
			return
		}

		// If conflict and auto-allocation, retry; otherwise fail.
		if !autoAlloc || !errors.Is(err, store.ErrConflict) || attempt == maxRetries {
			writeStoreErr(w, r, err)
			return
		}

		// Retry: re-read domain types and re-allocate.
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
}
