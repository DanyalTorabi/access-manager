package sqlite

import (
	"strings"
	"testing"
)

func TestBuildInQueryAndArgs_EmptyIDs(t *testing.T) {
	_, _, err := buildInQueryAndArgs("SELECT 1 FROM t WHERE a = ?", "t.id", []any{"x"}, nil)
	if err == nil {
		t.Fatal("want error for empty ids, got nil")
	}
}

func TestBuildInQueryAndArgs_SingleID(t *testing.T) {
	query, args, err := buildInQueryAndArgs(
		"SELECT id FROM t WHERE domain_id = ?",
		"t.id",
		[]any{"d1"},
		[]string{"id1"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "SELECT id FROM t WHERE domain_id = ? AND t.id IN (?)"
	if query != want {
		t.Errorf("query = %q, want %q", query, want)
	}
	if len(args) != 2 {
		t.Fatalf("len(args) = %d, want 2", len(args))
	}
	if args[0] != "d1" {
		t.Errorf("args[0] = %v, want d1", args[0])
	}
	if args[1] != "id1" {
		t.Errorf("args[1] = %v, want id1", args[1])
	}
}

func TestBuildInQueryAndArgs_MultipleIDs(t *testing.T) {
	ids := []string{"a", "b", "c"}
	query, args, err := buildInQueryAndArgs(
		"SELECT id FROM t WHERE domain_id = ?",
		"t.id",
		[]any{"d1"},
		ids,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	placeholderCount := strings.Count(query, "?")
	if placeholderCount != 1+len(ids) {
		t.Errorf("placeholder count = %d, want %d", placeholderCount, 1+len(ids))
	}
	// baseArgs first, then id args.
	if len(args) != 1+len(ids) {
		t.Fatalf("len(args) = %d, want %d", len(args), 1+len(ids))
	}
	if args[0] != "d1" {
		t.Errorf("args[0] = %v, want d1 (baseArgs must precede id args)", args[0])
	}
	for i, id := range ids {
		if args[1+i] != id {
			t.Errorf("args[%d] = %v, want %v", 1+i, args[1+i], id)
		}
	}
}

func TestBuildInQueryAndArgs_BaseArgsOrderedBeforeIDArgs(t *testing.T) {
	query, args, err := buildInQueryAndArgs(
		"SELECT 1 FROM t WHERE a = ? AND b = ?",
		"t.id",
		[]any{"base1", "base2"},
		[]string{"id1", "id2"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = query
	// Expect [base1, base2, id1, id2]
	want := []any{"base1", "base2", "id1", "id2"}
	if len(args) != len(want) {
		t.Fatalf("len(args) = %d, want %d", len(args), len(want))
	}
	for i, w := range want {
		if args[i] != w {
			t.Errorf("args[%d] = %v, want %v", i, args[i], w)
		}
	}
}
