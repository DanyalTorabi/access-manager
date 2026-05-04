package mysql

import (
	"errors"
	"strings"
	"testing"

	"github.com/dtorabi/access-manager/internal/store"
	mysql "github.com/go-sql-driver/mysql"
)

// --- wrapConstraintError ---

func TestWrapConstraintError_nil(t *testing.T) {
	if err := wrapConstraintError(nil); err != nil {
		t.Fatalf("nil error wrapped to non-nil: %v", err)
	}
}

func TestWrapConstraintError_dupEntry(t *testing.T) {
	mysqlErr := &mysql.MySQLError{Number: 1062, Message: "Duplicate entry"}
	wrapped := wrapConstraintError(mysqlErr)
	if !errors.Is(wrapped, store.ErrConflict) {
		t.Fatalf("ER_DUP_ENTRY should wrap ErrConflict, got: %v", wrapped)
	}
}

func TestWrapConstraintError_rowIsReferenced(t *testing.T) {
	mysqlErr := &mysql.MySQLError{Number: 1451, Message: "FK reference"}
	wrapped := wrapConstraintError(mysqlErr)
	if !errors.Is(wrapped, store.ErrFKViolation) {
		t.Fatalf("ER_ROW_IS_REFERENCED_2 should wrap ErrFKViolation, got: %v", wrapped)
	}
}

func TestWrapConstraintError_noReferencedRow(t *testing.T) {
	mysqlErr := &mysql.MySQLError{Number: 1452, Message: "FK no ref row"}
	wrapped := wrapConstraintError(mysqlErr)
	if !errors.Is(wrapped, store.ErrFKViolation) {
		t.Fatalf("ER_NO_REFERENCED_ROW_2 should wrap ErrFKViolation, got: %v", wrapped)
	}
}

func TestWrapConstraintError_otherMySQLError(t *testing.T) {
	mysqlErr := &mysql.MySQLError{Number: 1044, Message: "Access denied"}
	wrapped := wrapConstraintError(mysqlErr)
	if errors.Is(wrapped, store.ErrConflict) || errors.Is(wrapped, store.ErrFKViolation) {
		t.Fatalf("non-constraint mysql error should not be mapped, got: %v", wrapped)
	}
}

func TestWrapConstraintError_nonMySQLError(t *testing.T) {
	plain := errors.New("plain error")
	if got := wrapConstraintError(plain); got != plain {
		t.Fatal("non-mysql error should pass through unchanged")
	}
}

// --- maskToSQL ---

func TestMaskToSQL_zero(t *testing.T) {
	v, err := maskToSQL(0)
	if err != nil || v != 0 {
		t.Fatalf("maskToSQL(0) = %d, %v", v, err)
	}
}

func TestMaskToSQL_maxValid(t *testing.T) {
	m := uint64(1<<63 - 1)
	v, err := maskToSQL(m)
	if err != nil {
		t.Fatalf("maskToSQL(maxInt63) unexpected error: %v", err)
	}
	if uint64(v) != m {
		t.Fatalf("round-trip failed: got %d, want %d", v, m)
	}
}

func TestMaskToSQL_overflow(t *testing.T) {
	_, err := maskToSQL(1 << 63)
	if !errors.Is(err, store.ErrInvalidInput) {
		t.Fatalf("bit-63 set should return ErrInvalidInput, got: %v", err)
	}
}

// --- maskFromSQL ---

func TestMaskFromSQL_positive(t *testing.T) {
	s := New(nil)
	if got := s.maskFromSQL(7); got != 7 {
		t.Fatalf("maskFromSQL(7) = %d, want 7", got)
	}
}

func TestMaskFromSQL_zero(t *testing.T) {
	s := New(nil)
	if got := s.maskFromSQL(0); got != 0 {
		t.Fatalf("maskFromSQL(0) = %d, want 0", got)
	}
}

func TestMaskFromSQL_negative_returns_zero(t *testing.T) {
	s := New(nil)
	called := false
	s.SetNegativeMaskHook(func() { called = true })
	if got := s.maskFromSQL(-5); got != 0 {
		t.Fatalf("maskFromSQL(-5) = %d, want 0", got)
	}
	if !called {
		t.Fatal("negative mask hook was not called")
	}
}

// --- SetNegativeMaskHook ---

func TestSetNegativeMaskHook_nil(t *testing.T) {
	s := New(nil)
	s.SetNegativeMaskHook(func() {})
	s.SetNegativeMaskHook(nil)
	// Clearing hook must not panic.
	_ = s.maskFromSQL(-1)
}

// --- escapeLikePattern ---

func TestEscapeLikePattern(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"hello", "hello"},
		{"100%", `100\%`},
		{"_name", `\_name`},
		{`back\slash`, `back\\slash`},
		{`50%_off\n`, `50\%\_off\\n`},
	}
	for _, tc := range tests {
		got := escapeLikePattern(tc.in)
		if got != tc.want {
			t.Errorf("escapeLikePattern(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// --- sortColumns ---

func TestSortColumns_noOverrides(t *testing.T) {
	cols := sortColumns([]string{"title", "id"}, nil)
	if cols["title"] != "title" || cols["id"] != "id" {
		t.Fatalf("unexpected result: %v", cols)
	}
}

func TestSortColumns_withOverride(t *testing.T) {
	cols := sortColumns([]string{"title", "resource_id"}, map[string]string{"resource_id": "r.id"})
	if cols["resource_id"] != "r.id" {
		t.Fatalf("override not applied: %v", cols)
	}
}

func TestSortColumns_overrideForUnknownFieldIgnored(t *testing.T) {
	cols := sortColumns([]string{"title"}, map[string]string{"nonexistent": "x"})
	if _, ok := cols["nonexistent"]; ok {
		t.Fatal("override for unknown field should be ignored")
	}
}

// --- orderByClause ---

func TestOrderByClause_asc(t *testing.T) {
	cols := map[string]string{"title": "title"}
	clause := orderByClause("title", store.OrderAsc, cols, "title")
	if !strings.Contains(clause, "ASC") {
		t.Fatalf("expected ASC: %q", clause)
	}
}

func TestOrderByClause_desc(t *testing.T) {
	cols := map[string]string{"title": "title"}
	clause := orderByClause("title", store.OrderDesc, cols, "title")
	if !strings.Contains(clause, "DESC") {
		t.Fatalf("expected DESC: %q", clause)
	}
}

func TestOrderByClause_unknownSortFallback(t *testing.T) {
	cols := map[string]string{"title": "title"}
	clause := orderByClause("unknown", store.OrderAsc, cols, "title")
	if strings.Contains(clause, "unknown") {
		t.Fatalf("unknown field should not appear: %q", clause)
	}
}

func TestOrderByClause_idColNoDouble(t *testing.T) {
	cols := map[string]string{"id": "id"}
	clause := orderByClause("id", store.OrderAsc, cols, "id")
	if strings.Count(clause, "id") != 1 {
		t.Fatalf("'id' should appear once: %q", clause)
	}
}

// --- likePattern ---

func TestLikePattern_contains(t *testing.T) {
	p := likePattern("foo", store.SearchContains)
	if p != "%foo%" {
		t.Fatalf("got %q", p)
	}
}

func TestLikePattern_emptyTypeIsContains(t *testing.T) {
	if p := likePattern("x", ""); p != "%x%" {
		t.Fatalf("got %q", p)
	}
}

func TestLikePattern_startsWith(t *testing.T) {
	if p := likePattern("foo", store.SearchStartsWith); p != "foo%" {
		t.Fatalf("got %q", p)
	}
}

func TestLikePattern_endsWith(t *testing.T) {
	if p := likePattern("foo", store.SearchEndsWith); p != "%foo" {
		t.Fatalf("got %q", p)
	}
}

func TestLikePattern_unknownFallback(t *testing.T) {
	if p := likePattern("foo", "exact"); p != "%foo%" {
		t.Fatalf("unknown type should fallback to contains, got %q", p)
	}
}

func TestLikePattern_escapedChars(t *testing.T) {
	p := likePattern("50%", store.SearchContains)
	if !strings.Contains(p, `\%`) {
		t.Fatalf("percent not escaped: %q", p)
	}
}

// --- parseVersion (migrate.go) ---

func TestParseVersion_valid(t *testing.T) {
	v, ok := parseVersion("000001_create_domains.up.sql")
	if !ok || v != 1 {
		t.Fatalf("parseVersion = %d, %v; want 1, true", v, ok)
	}
}

func TestParseVersion_multiDigit(t *testing.T) {
	v, ok := parseVersion("000042_add_table.up.sql")
	if !ok || v != 42 {
		t.Fatalf("parseVersion = %d, %v; want 42, true", v, ok)
	}
}

func TestParseVersion_noUnderscore(t *testing.T) {
	if _, ok := parseVersion("migrations.sql"); ok {
		t.Fatal("file without underscore should return ok=false")
	}
}

func TestParseVersion_nonNumericPrefix(t *testing.T) {
	if _, ok := parseVersion("abc_create.up.sql"); ok {
		t.Fatal("non-numeric prefix should return ok=false")
	}
}
