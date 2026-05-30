package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/dtorabi/access-manager/internal/store"
)

func (s *Server) writeJSON(w http.ResponseWriter, r *http.Request, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// The response header is already committed; log with request context for
		// operator visibility so the failing endpoint can be identified.
		attrs := []slog.Attr{slog.String("err", err.Error())}
		if r != nil {
			attrs = append(attrs,
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
			)
		}
		s.serverLogger().LogAttrs(r.Context(), slog.LevelError, "response encode failed", attrs...)
	}
}

func (s *Server) writeErr(w http.ResponseWriter, r *http.Request, status int, err error) {
	s.writeJSON(w, r, status, map[string]string{"error": err.Error()})
}

// writeStoreErr classifies a store-layer error into the correct HTTP status
// and returns a stable, database-agnostic message. The full error is logged
// server-side so operators can correlate support requests with logs.
func (s *Server) writeStoreErr(w http.ResponseWriter, r *http.Request, err error) {
	var status int
	var msg string
	switch {
	case errors.Is(err, store.ErrNotFound):
		status = http.StatusNotFound
		msg = "resource not found"
	case errors.Is(err, store.ErrFKViolation):
		status = http.StatusBadRequest
		msg = "referenced entity does not exist or is still referenced"
	case errors.Is(err, store.ErrInvalidInput):
		status = http.StatusBadRequest
		msg = publicInvalidInputMsg(err)
	case errors.Is(err, store.ErrConflict):
		status = http.StatusConflict
		msg = "resource already exists"
	default:
		status = http.StatusInternalServerError
		msg = "internal server error"
	}
	s.logRequestErr(r, status, err)
	s.writeJSON(w, r, status, map[string]string{"error": msg})
}

// writeInternalErr logs the full error and returns a generic 500 to the client.
// Intended for read/list operations where the store returns only unexpected DB
// errors, not structured store errors (ErrNotFound, ErrConflict, ErrFKViolation,
// etc.). For single-entity operations use writeStoreErr, which maps those errors
// to appropriate HTTP status codes.
//
// Misuse guard: if a known structured store sentinel is passed here by mistake,
// the function logs an additional ERROR-level alert so the incorrect call site
// is immediately visible in production instead of silently producing a 500 for
// errors that should map to 4xx. The client always receives the generic 500.
func (s *Server) writeInternalErr(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrConflict) ||
		errors.Is(err, store.ErrFKViolation) || errors.Is(err, store.ErrInvalidInput) {
		s.serverLogger().LogAttrs(r.Context(), slog.LevelError,
			"writeInternalErr misuse: structured store error must use writeStoreErr",
			slog.String("err", err.Error()),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
		)
	}
	s.logRequestErr(r, http.StatusInternalServerError, err)
	s.writeJSON(w, r, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
}

// logRequestErr logs the error with request context. 5xx errors are logged at
// ERROR level; 4xx at WARN to avoid inflating error-level alerts for expected
// client mistakes.
func (s *Server) logRequestErr(r *http.Request, status int, err error) {
	attrs := []slog.Attr{
		slog.Int("status", status),
		slog.String("err", err.Error()),
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
	}
	if status >= 500 {
		s.serverLogger().LogAttrs(r.Context(), slog.LevelError, "request error", attrs...)
	} else {
		s.serverLogger().LogAttrs(r.Context(), slog.LevelWarn, "request error", attrs...)
	}
}

// publicInvalidInputMsg returns a stable, client-safe message for an
// ErrInvalidInput-classed error. It uses errors.As to extract a
// store.InvalidInputError carrying the Detail set at the validation site,
// so intermediate fmt.Errorf("%w", err) wrapping is safe. The mask-overflow
// case is translated to the API's existing wording for backward
// compatibility with clients that key on the message text.
func publicInvalidInputMsg(err error) string {
	var iie *store.InvalidInputError
	if errors.As(err, &iie) && iie != nil && iie.Detail != "" {
		if iie.Detail == store.InvalidInputDetailMaskOverflow {
			return "mask value must be within signed 64-bit range"
		}
		return iie.Detail
	}
	return "invalid request"
}

const maxRequestBodySize = 1 << 20 // 1 MiB

type listMeta struct {
	Total  int64  `json:"total"`
	Offset int    `json:"offset"`
	Limit  int    `json:"limit"`
	Sort   string `json:"sort"`
	Order  string `json:"order"`
}

type listEnvelope struct {
	Data any      `json:"data"`
	Meta listMeta `json:"meta"`
}

func parseListOpts(r *http.Request) (store.ListOpts, error) {
	opts := store.ListOpts{Offset: 0, Limit: store.DefaultLimit}
	q := r.URL.Query()
	if v := q.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return opts, errors.New("offset must be an integer")
		}
		if n < 0 {
			return opts, errors.New("offset must not be negative")
		}
		opts.Offset = n
	}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return opts, errors.New("limit must be an integer")
		}
		if n < 1 {
			n = 1
		}
		if n > store.MaxLimit {
			n = store.MaxLimit
		}
		opts.Limit = n
	}
	opts.Search = strings.TrimSpace(q.Get("search"))
	if utf8.RuneCountInString(opts.Search) > 255 {
		return opts, errors.New("search must be at most 255 characters")
	}
	opts.SearchType = store.SearchContains
	if opts.Search != "" {
		if v := strings.TrimSpace(q.Get("search_type")); v != "" {
			st := store.SearchType(v)
			switch st {
			case store.SearchContains, store.SearchStartsWith, store.SearchEndsWith:
				opts.SearchType = st
			default:
				return opts, errors.New("search_type must be contains, starts_with, or ends_with")
			}
		}
	}
	return opts, nil
}

func parseOffsetLimitOpts(r *http.Request) (store.ListOpts, error) {
	opts := store.ListOpts{Offset: 0, Limit: store.DefaultLimit}
	q := r.URL.Query()
	if _, ok := q["search"]; ok {
		return opts, errors.New("only limit and offset are supported")
	}
	if _, ok := q["search_type"]; ok {
		return opts, errors.New("only limit and offset are supported")
	}
	if _, ok := q["sort"]; ok {
		return opts, errors.New("only limit and offset are supported")
	}
	if _, ok := q["order"]; ok {
		return opts, errors.New("only limit and offset are supported")
	}
	if v := q.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return opts, errors.New("offset must be an integer")
		}
		if n < 0 {
			return opts, errors.New("offset must not be negative")
		}
		opts.Offset = n
	}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return opts, errors.New("limit must be an integer")
		}
		if n < 1 {
			n = 1
		}
		if n > store.MaxLimit {
			n = store.MaxLimit
		}
		opts.Limit = n
	}
	return opts, nil
}

func parseGroupListOpts(r *http.Request) (store.GroupListOpts, error) {
	base, err := parseListOpts(r)
	if err != nil {
		return store.GroupListOpts{}, err
	}
	out := store.GroupListOpts{ListOpts: base}
	if v := strings.TrimSpace(r.URL.Query().Get("parent_group_id")); v != "" {
		out.ParentGroupID = &v
	}
	return out, nil
}

func parsePermissionListOpts(r *http.Request) (store.PermissionListOpts, error) {
	base, err := parseListOpts(r)
	if err != nil {
		return store.PermissionListOpts{}, err
	}
	out := store.PermissionListOpts{ListOpts: base}
	if v := strings.TrimSpace(r.URL.Query().Get("resource_id")); v != "" {
		out.ResourceID = &v
	}
	return out, nil
}

// parseSortOrder reads sort and order query params, validates them against
// the allowed sort fields, and returns the validated values.
func parseSortOrder(q url.Values, allowed []string) (string, store.SortOrder, error) {
	sortField, err := store.ValidateSort(strings.TrimSpace(q.Get("sort")), allowed)
	if err != nil {
		return "", "", err
	}
	order := store.OrderAsc
	if v := strings.TrimSpace(q.Get("order")); v != "" {
		o := store.SortOrder(v)
		switch o {
		case store.OrderAsc, store.OrderDesc:
			order = o
		default:
			return "", "", errors.New("order must be asc or desc")
		}
	}
	return sortField, order, nil
}

func (s *Server) writeList(w http.ResponseWriter, r *http.Request, data any, total int64, opts store.ListOpts) {
	s.writeJSON(w, r, http.StatusOK, listEnvelope{
		Data: data,
		Meta: listMeta{
			Total:  total,
			Offset: opts.Offset,
			Limit:  opts.Limit,
			Sort:   opts.Sort,
			Order:  string(opts.Order),
		},
	})
}

func (s *Server) readJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			s.logReadJSONErr(r, "body_too_large", "request body too large")
			s.writeErr(w, r, http.StatusRequestEntityTooLarge, errors.New("request body too large"))
			return false
		}
		cls := classifyDecodeErr(err)
		s.logReadJSONErr(r, cls.kind, cls.logMsg)
		s.writeErr(w, r, http.StatusBadRequest, errors.New(cls.clientMsg))
		return false
	}
	// Decode a second value to verify the stream is exhausted. io.EOF means
	// the body was cleanly consumed (trailing whitespace is allowed); any other
	// result — including nil error, meaning a second value decoded successfully
	// — indicates trailing data and is rejected. Using dec.More() is explicitly
	// avoided: its contract is scoped to array/object iteration, not top-level
	// stream exhaustion, and its behaviour outside that scope is undocumented.
	var extra json.RawMessage
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		s.logReadJSONErr(r, "trailing_data", "trailing data after first JSON value")
		s.writeErr(w, r, http.StatusBadRequest, errors.New("request body must contain exactly one JSON value"))
		return false
	}
	return true
}

// decodeClass holds the classification of a JSON decode failure: the
// structured log kind, a sanitized log message (safe to persist), and the
// stable client-facing message.
type decodeClass struct {
	kind      string
	logMsg    string // safe to log (no raw user input)
	clientMsg string
}

// classifyDecodeErr classifies a JSON decode error into a decodeClass.
// Kinds:
//   - empty_body          — io.EOF (no body at all)
//   - json_syntax         — truncated body or syntax error
//   - json_type           — wrong value type for a known field
//   - json_unknown_field  — client sent a field name not in the schema
//   - json_decode         — other decode errors
func classifyDecodeErr(err error) decodeClass {
	var synErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	switch {
	case errors.Is(err, io.EOF):
		return decodeClass{"empty_body", "empty request body", "request body must not be empty"}
	case errors.Is(err, io.ErrUnexpectedEOF), errors.As(err, &synErr):
		return decodeClass{"json_syntax", "malformed JSON in request body", "request body contains malformed JSON"}
	case errors.As(err, &typeErr):
		return decodeClass{"json_type", "invalid field value type in request body", "request body contains an invalid field value"}
	case strings.HasPrefix(err.Error(), "json: unknown field"):
		// Do not log the raw error string: it contains the attacker-controlled
		// field name verbatim (e.g. "json: unknown field \"injected\"").
		return decodeClass{"json_unknown_field", "unknown field in request body", "invalid request body"}
	default:
		return decodeClass{"json_decode", "request body decode error", "invalid request body"}
	}
}

// logReadJSONErr logs a server-side warning for request body parse failures.
// The detail parameter must be a pre-sanitized string — never pass err.Error()
// directly for errors that may contain user-controlled input (e.g. unknown
// field names from DisallowUnknownFields).
func (s *Server) logReadJSONErr(r *http.Request, kind, detail string) {
	s.serverLogger().LogAttrs(r.Context(), slog.LevelWarn, "request body decode failed",
		slog.String("kind", kind),
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
		slog.String("detail", detail),
	)
}

// errInvalidNumericValue is the stable, client-safe error returned when a
// query/body field cannot be parsed as a uint64. It intentionally does not
// echo back the raw input or the underlying strconv message.
var errInvalidNumericValue = errors.New("invalid numeric value")

// errAccessMaskOutOfRange is the stable, client-safe error returned when a
// mask/bit value is parseable but exceeds the API's signed-63 limit (see
// issue #67 / T46). Wording must stay backward-compatible with existing
// clients and tests.
var errAccessMaskOutOfRange = errors.New("mask value must be within signed 64-bit range")

// maxAccessMask is the largest mask value permitted by the API until a v2
// migration stores full uint64 values. Bit 63 (1<<63) is reserved to avoid
// signed-64 overflow when masks are persisted in SQLite. See issue #67 / T46.
const maxAccessMask uint64 = 1<<63 - 1

// maxNumericInputLen caps the length of strings accepted by
// parseUint64Validated. The longest legal input is 18 chars (`0x` + 16 hex
// digits) for full uint64; 32 leaves comfortable headroom while bounding
// CPU on pathological inputs without depending on outer body-size limits.
const maxNumericInputLen = 32

// parseUint64Validated parses s as a uint64 in either base 10 (decimal)
// or base 16 (with a `0x`/`0X` prefix), matching the format documented in
// api/openapi.yaml. Other strconv.ParseUint(base=0) modes (octal `0nnn`,
// binary `0b…`) are intentionally rejected so the wire format stays
// unambiguous. When max > 0 the helper also rejects values greater than
// max. The returned errors are stable, client-safe sentinels and never
// embed the user input.
func parseUint64Validated(s string, max uint64) (uint64, error) {
	if len(s) == 0 || len(s) > maxNumericInputLen {
		return 0, errInvalidNumericValue
	}
	base := 10
	digits := s
	if len(s) > 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X') {
		base = 16
		digits = s[2:]
	}
	n, err := strconv.ParseUint(digits, base, 64)
	if err != nil {
		return 0, errInvalidNumericValue
	}
	if max > 0 && n > max {
		return 0, errAccessMaskOutOfRange
	}
	return n, nil
}
