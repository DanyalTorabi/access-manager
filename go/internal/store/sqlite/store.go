package sqlite

import (
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"sync/atomic"

	"github.com/dtorabi/access-manager/internal/store"
	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// Store implements store.Store for SQLite.
type Store struct {
	db *sql.DB
	// negativeMaskHook is invoked once per negative mask read by
	// (*Store).maskFromSQL. Callers (typically the API layer) install a
	// callback to bump a Prometheus counter so dashboards can alert on
	// out-of-band or legacy data. The default is a no-op. See T50.
	negativeMaskHook atomic.Pointer[func()]
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

var _ store.Store = (*Store)(nil)

func constraintCode(err error) int {
	var e *sqlite.Error
	if errors.As(err, &e) {
		return e.Code()
	}
	return 0
}

func wrapConstraintError(err error) error {
	if err == nil {
		return nil
	}
	switch constraintCode(err) {
	case sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY:
		return errors.Join(store.ErrFKViolation, err)
	case sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY, sqlite3.SQLITE_CONSTRAINT_UNIQUE:
		return errors.Join(store.ErrConflict, err)
	}
	// database/sql sometimes returns errors that do not unwrap to *sqlite.Error.
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "foreign key constraint failed") {
		return errors.Join(store.ErrFKViolation, err)
	}
	if strings.Contains(msg, "unique constraint failed") || strings.Contains(msg, "primary key constraint") {
		return errors.Join(store.ErrConflict, err)
	}
	return err
}

const maxInt64 = 1<<63 - 1

// maskToSQL converts a uint64 mask into a signed int64 suitable for SQLite
// storage. Returns an error if the value cannot be represented in signed
// 64-bit (i.e. uses bit 63). Callers should validate input and return a
// client-facing validation error when necessary.
func maskToSQL(m uint64) (int64, error) {
	if m > uint64(maxInt64) {
		return 0, store.NewInvalidInput(store.InvalidInputDetailMaskOverflow)
	}
	return int64(m), nil
}

// negativeMaskHook is invoked once per negative mask read by maskFromSQL.
// It is set by callers (typically the API layer) to bump a Prometheus
// counter so dashboards can alert on out-of-band or legacy data. The
// default is a no-op. See T50.

// SetNegativeMaskHook installs a callback invoked whenever this Store
// observes a negative int64 mask value via maskFromSQL. Pass nil to clear.
// Safe for concurrent use. The hook is invoked synchronously inside the
// row-scan path, so implementations must be fast (e.g. a single atomic
// counter increment) and must not block.
func (s *Store) SetNegativeMaskHook(f func()) {
	if f == nil {
		s.negativeMaskHook.Store(nil)
		return
	}
	s.negativeMaskHook.Store(&f)
}

// maskFromSQL converts an int64 value read from SQLite into uint64. If a
// negative value is encountered, log a warning and treat it as zero to avoid
// propagating unexpected large unsigned values.
func (s *Store) maskFromSQL(v int64) uint64 {
	if v < 0 {
		slog.Warn("negative mask value read from DB; treating as 0", "value", v)
		if h := s.negativeMaskHook.Load(); h != nil {
			(*h)()
		}
		return 0
	}
	return uint64(v)
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func escapeLikePattern(s string) string { return likeEscaper.Replace(s) }

// sortColumns builds a field→column map from the store's allowed sort fields.
// By default field name == column name; pass overrides for any allowed fields
// whose column names differ. Override keys not present in fields are ignored.
func sortColumns(fields []string, overrides map[string]string) map[string]string {
	cols := make(map[string]string, len(fields))
	for _, f := range fields {
		cols[f] = f
	}
	for f, col := range overrides {
		if _, ok := cols[f]; ok {
			cols[f] = col
		}
	}
	return cols
}

var (
	domainSortColumns     = sortColumns(store.DomainSortFields, nil)
	userSortColumns       = sortColumns(store.UserSortFields, nil)
	groupSortColumns      = sortColumns(store.GroupSortFields, nil)
	resourceSortColumns   = sortColumns(store.ResourceSortFields, nil)
	accessTypeSortColumns = sortColumns(store.AccessTypeSortFields, nil)
	permissionSortColumns = sortColumns(store.PermissionSortFields, nil)
)

// orderByClause returns a safe " ORDER BY <col> <dir>, id <dir>" clause.
// sort should already be validated against the allow-list by the caller.
// An empty sort falls back to fallbackCol. An unknown non-empty sort also
// falls back to fallbackCol for compatibility, but emits a warning so
// call-site bugs are not silently masked.
// A secondary ", id" tiebreaker is always appended to guarantee
// deterministic pagination when the primary column has duplicates.
func orderByClause(sort string, order store.SortOrder, allowed map[string]string, fallbackCol string) string {
	col := fallbackCol
	if sort != "" {
		if mapped, ok := allowed[sort]; ok {
			col = mapped
		} else {
			slog.Warn("unknown sort field, falling back to default", "sort", sort, "fallback", fallbackCol)
		}
	}
	dir := "ASC"
	if order == store.OrderDesc {
		dir = "DESC"
	}
	clause := " ORDER BY " + col + " " + dir
	if col != "id" {
		clause += ", id " + dir
	}
	return clause
}

func likePattern(search string, st store.SearchType) string {
	escaped := escapeLikePattern(search)
	switch st {
	case store.SearchContains, "":
		return "%" + escaped + "%"
	case store.SearchStartsWith:
		return escaped + "%"
	case store.SearchEndsWith:
		return "%" + escaped
	default:
		// Unknown SearchType — likely a call-site typo. Log and fall back to contains.
		slog.Warn("unknown SearchType, falling back to contains", "search_type", string(st))
		return "%" + escaped + "%"
	}
}
