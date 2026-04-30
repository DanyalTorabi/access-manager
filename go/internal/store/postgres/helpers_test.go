package postgres

import (
	"errors"
	"strings"
	"testing"

	"github.com/dtorabi/access-manager/internal/store"
	"github.com/lib/pq"
)

// --- rebind ---

func TestRebind_noPlaceholders(t *testing.T) {
	got := rebind("SELECT 1")
	if got != "SELECT 1" {
		t.Fatalf("rebind(%q) = %q", "SELECT 1", got)
	}
}

func TestRebind_singlePlaceholder(t *testing.T) {
	got := rebind("SELECT * FROM t WHERE id = ?")
	want := "SELECT * FROM t WHERE id = $1"
	if got != want {
		t.Fatalf("rebind = %q, want %q", got, want)
	}
}

func TestRebind_multiplePlaceholders(t *testing.T) {
	got := rebind("INSERT INTO t (a, b, c) VALUES (?, ?, ?)")
	want := "INSERT INTO t (a, b, c) VALUES ($1, $2, $3)"
	if got != want {
		t.Fatalf("rebind = %q, want %q", got, want)
	}
}

// --- wrapConstraintError ---

func TestWrapConstraintError_nil(t *testing.T) {
	if err := wrapConstraintError(nil); err != nil {
		t.Fatalf("nil error wrapped to non-nil: %v", err)
	}
}

func TestWrapConstraintError_uniqueViolation(t *testing.T) {
	pqErr := &pq.Error{Code: "23505"}
	wrapped := wrapConstraintError(pqErr)
	if !errors.Is(wrapped, store.ErrConflict) {
		t.Fatalf("unique violation should wrap ErrConflict, got: %v", wrapped)
	}
}

func TestWrapConstraintError_foreignKeyViolation(t *testing.T) {
	pqErr := &pq.Error{Code: "23503"}
	wrapped := wrapConstraintError(pqErr)
	if !errors.Is(wrapped, store.ErrFKViolation) {
		t.Fatalf("FK violation should wrap ErrFKViolation, got: %v", wrapped)
	}
}

func TestWrapConstraintError_otherPQError(t *testing.T) {
	pqErr := &pq.Error{Code: "42P01"} // undefined_table
	wrapped := wrapConstraintError(pqErr)
	if errors.Is(wrapped, store.ErrConflict) || errors.Is(wrapped, store.ErrFKViolation) {
		t.Fatalf("non-constraint pq error should not be mapped, got: %v", wrapped)
	}
	if !errors.As(wrapped, new(*pq.Error)) {
		t.Fatalf("non-constraint pq error should be preserved as *pq.Error, got: %T", wrapped)
	}
}

func TestWrapConstraintError_nonPQError(t *testing.T) {
	plain := errors.New("some other error")
	wrapped := wrapConstraintError(plain)
	if wrapped != plain {
		t.Fatalf("non-pq error should pass through unchanged")
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
		t.Fatalf("maskToSQL round-trip failed: got %d, want %d", v, m)
	}
}

func TestMaskToSQL_overflow(t *testing.T) {
	_, err := maskToSQL(1 << 63)
	if !errors.Is(err, store.ErrInvalidInput) {
		t.Fatalf("maskToSQL with bit-63 set should return ErrInvalidInput, got: %v", err)
	}
}

// --- maskFromSQL ---

func TestMaskFromSQL_positive(t *testing.T) {
	s := New(nil)
	if got := s.maskFromSQL(42); got != 42 {
		t.Fatalf("maskFromSQL(42) = %d, want 42", got)
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
	if got := s.maskFromSQL(-1); got != 0 {
		t.Fatalf("maskFromSQL(-1) = %d, want 0", got)
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
	// After clearing, hook must not fire (no panic).
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
		t.Fatalf("unexpected sortColumns result: %v", cols)
	}
}

func TestSortColumns_withOverride(t *testing.T) {
	cols := sortColumns([]string{"title", "resource_id"}, map[string]string{"resource_id": "r.id"})
	if cols["resource_id"] != "r.id" {
		t.Fatalf("override not applied, got: %v", cols)
	}
	if cols["title"] != "title" {
		t.Fatalf("non-overridden field changed: %v", cols)
	}
}

func TestSortColumns_overrideForUnknownFieldIgnored(t *testing.T) {
	cols := sortColumns([]string{"title"}, map[string]string{"nonexistent": "x"})
	if _, ok := cols["nonexistent"]; ok {
		t.Fatal("override for unknown field should be ignored")
	}
}

// --- orderByClause ---

func TestOrderByClause_defaultAsc(t *testing.T) {
	cols := map[string]string{"title": "title"}
	clause := orderByClause("title", store.OrderAsc, cols, "title")
	if !strings.Contains(clause, "ASC") {
		t.Fatalf("expected ASC in clause: %q", clause)
	}
}

func TestOrderByClause_desc(t *testing.T) {
	cols := map[string]string{"title": "title"}
	clause := orderByClause("title", store.OrderDesc, cols, "title")
	if !strings.Contains(clause, "DESC") {
		t.Fatalf("expected DESC in clause: %q", clause)
	}
}

func TestOrderByClause_unknownSortFallback(t *testing.T) {
	cols := map[string]string{"title": "title"}
	clause := orderByClause("unknown_field", store.OrderAsc, cols, "title")
	// Falls back to default col; clause must not contain the unknown field.
	if strings.Contains(clause, "unknown_field") {
		t.Fatalf("unknown field should not appear in clause: %q", clause)
	}
	if !strings.Contains(clause, "title") {
		t.Fatalf("fallback col 'title' should appear in clause: %q", clause)
	}
}

func TestOrderByClause_idColNoDouble(t *testing.T) {
	cols := map[string]string{"id": "id"}
	clause := orderByClause("id", store.OrderAsc, cols, "id")
	// When col == "id", there should be exactly one ORDER BY segment.
	count := strings.Count(clause, "id")
	if count != 1 {
		t.Fatalf("'id' appears %d times in clause %q, expected 1", count, clause)
	}
}

// --- likePattern ---

func TestLikePattern_contains(t *testing.T) {
	p := likePattern("foo", store.SearchContains)
	if p != "%foo%" {
		t.Fatalf("contains: got %q, want %%foo%%", p)
	}
}

func TestLikePattern_empty_type_is_contains(t *testing.T) {
	p := likePattern("foo", "")
	if p != "%foo%" {
		t.Fatalf("empty type: got %q, want %%foo%%", p)
	}
}

func TestLikePattern_startsWith(t *testing.T) {
	p := likePattern("foo", store.SearchStartsWith)
	if p != "foo%" {
		t.Fatalf("startsWith: got %q, want foo%%", p)
	}
}

func TestLikePattern_endsWith(t *testing.T) {
	p := likePattern("foo", store.SearchEndsWith)
	if p != "%foo" {
		t.Fatalf("endsWith: got %q, want %%foo", p)
	}
}

func TestLikePattern_unknownType_fallbackContains(t *testing.T) {
	p := likePattern("foo", "exact")
	if p != "%foo%" {
		t.Fatalf("unknown type should fall back to contains, got %q", p)
	}
}

func TestLikePattern_escapedChars(t *testing.T) {
	p := likePattern("50%", store.SearchContains)
	if !strings.Contains(p, `\%`) {
		t.Fatalf("percent not escaped in: %q", p)
	}
}

// --- inPlaceholders ---

func TestInPlaceholders_single(t *testing.T) {
	got, err := inPlaceholders(1)
	if err != nil {
		t.Fatal(err)
	}
	if got != "?" {
		t.Fatalf("inPlaceholders(1) = %q", got)
	}
}

func TestInPlaceholders_multiple(t *testing.T) {
	got, err := inPlaceholders(3)
	if err != nil {
		t.Fatal(err)
	}
	if got != "?,?,?" {
		t.Fatalf("inPlaceholders(3) = %q", got)
	}
}

func TestInPlaceholders_zero(t *testing.T) {
	_, err := inPlaceholders(0)
	if err == nil {
		t.Fatal("inPlaceholders(0) should return error")
	}
}

func TestInPlaceholders_negative(t *testing.T) {
	_, err := inPlaceholders(-1)
	if err == nil {
		t.Fatal("inPlaceholders(-1) should return error")
	}
}

// --- userEffectivePermissionArgs ---

func TestUserEffectivePermissionArgs(t *testing.T) {
	args := userEffectivePermissionArgs("uid-1")
	if len(args) != 2 {
		t.Fatalf("want 2 args, got %d", len(args))
	}
	for i, a := range args {
		if a != "uid-1" {
			t.Fatalf("arg[%d] = %v, want uid-1", i, a)
		}
	}
}

// --- buildUserAuthzMaskQueryAndArgs ---

func TestBuildUserAuthzMaskQueryAndArgs_basic(t *testing.T) {
	q, args, err := buildUserAuthzMaskQueryAndArgs("dom", []string{"r1", "r2"}, []any{"u1", "u1"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(q, "$1") {
		t.Fatalf("query missing $1 placeholder: %q", q)
	}
	if len(args) != 1+2+2 {
		t.Fatalf("want 5 args (domainID + 2 resourceIDs + 2 predicateArgs), got %d", len(args))
	}
}

func TestBuildUserAuthzMaskQueryAndArgs_emptyResources(t *testing.T) {
	_, _, err := buildUserAuthzMaskQueryAndArgs("dom", []string{}, []any{})
	if err == nil {
		t.Fatal("empty resourceIDs should return error from inPlaceholders")
	}
}

// --- stripTransactionMarkers ---

func TestStripTransactionMarkers_removesBeginCommit(t *testing.T) {
	input := "BEGIN;\nCREATE TABLE t (id INT);\nCOMMIT;\n"
	got := stripTransactionMarkers(input)
	if strings.Contains(got, "BEGIN") || strings.Contains(got, "COMMIT") {
		t.Fatalf("BEGIN/COMMIT not removed: %q", got)
	}
	if !strings.Contains(got, "CREATE TABLE") {
		t.Fatalf("body content removed unexpectedly: %q", got)
	}
}

func TestStripTransactionMarkers_noMarkers(t *testing.T) {
	input := "CREATE TABLE t (id INT);"
	got := stripTransactionMarkers(input)
	if got != input {
		t.Fatalf("no-marker body changed: got %q, want %q", got, input)
	}
}

func TestStripTransactionMarkers_caseInsensitive(t *testing.T) {
	input := "begin;\nSELECT 1;\nCOMMIT;\n"
	got := stripTransactionMarkers(input)
	if strings.Contains(got, "begin") || strings.Contains(got, "COMMIT") {
		t.Fatalf("case-insensitive removal failed: %q", got)
	}
}

// --- parseVersion ---

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
	_, ok := parseVersion("migrations.sql")
	if ok {
		t.Fatal("file without underscore should return ok=false")
	}
}

func TestParseVersion_nonNumericPrefix(t *testing.T) {
	_, ok := parseVersion("abc_create.up.sql")
	if ok {
		t.Fatal("non-numeric prefix should return ok=false")
	}
}

// --- resourceAuthzUsersBaseArgs ---

func TestResourceAuthzUsersBaseArgs(t *testing.T) {
	args := resourceAuthzUsersBaseArgs("dom-1", "res-1")
	if len(args) != 3 {
		t.Fatalf("want 3 args, got %d", len(args))
	}
	if args[0] != "dom-1" || args[1] != "dom-1" || args[2] != "res-1" {
		t.Fatalf("unexpected args: %v", args)
	}
}
