//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"testing"
)

// ---------------------------------------------------------------------------
// Pagination journey — users as representative entity
// ---------------------------------------------------------------------------

const paginationN = 12

func mustDecodeIDs(t *testing.T, data json.RawMessage) []string {
	t.Helper()
	var items []entityTitle
	if err := json.Unmarshal(data, &items); err != nil {
		t.Fatal(err)
	}
	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}
	return ids
}

func TestPagination_users(t *testing.T) {
	c := httpClient()
	did := seedDomain(t, c, "pagination-dom")
	cleanupDelete(t, c, apiBase()+"/domains/"+did)
	base := domainBase(did)

	uids := make([]string, paginationN)
	for i := 0; i < paginationN; i++ {
		uids[i] = seedUser(t, c, did, fmt.Sprintf("page-user-%02d", i))
		cleanupDelete(t, c, base+"/users/"+uids[i])
	}

	// Pin sort order so pages are deterministic.
	q := base + "/users?sort=title&order=asc"

	var page1IDs, page2IDs, page3IDs []string

	t.Run("page1", func(t *testing.T) {
		env := mustList(t, c, q+"&offset=0&limit=5")
		if env.Meta.Total != paginationN {
			t.Fatalf("total: want %d, got %d", paginationN, env.Meta.Total)
		}
		page1IDs = mustDecodeIDs(t, env.Data)
		if len(page1IDs) != 5 {
			t.Fatalf("page1 len: want 5, got %d", len(page1IDs))
		}
	})

	t.Run("page2", func(t *testing.T) {
		env := mustList(t, c, q+"&offset=5&limit=5")
		if env.Meta.Total != paginationN {
			t.Fatalf("total: want %d, got %d", paginationN, env.Meta.Total)
		}
		page2IDs = mustDecodeIDs(t, env.Data)
		if len(page2IDs) != 5 {
			t.Fatalf("page2 len: want 5, got %d", len(page2IDs))
		}
	})

	t.Run("page3_partial", func(t *testing.T) {
		env := mustList(t, c, q+"&offset=10&limit=5")
		if env.Meta.Total != paginationN {
			t.Fatalf("total: want %d, got %d", paginationN, env.Meta.Total)
		}
		page3IDs = mustDecodeIDs(t, env.Data)
		if len(page3IDs) != 2 {
			t.Fatalf("page3 len: want 2, got %d", len(page3IDs))
		}
	})

	t.Run("no_overlap", func(t *testing.T) {
		seen := make(map[string]int)
		for _, id := range page1IDs {
			seen[id] = 1
		}
		for _, id := range page2IDs {
			if seen[id] == 1 {
				t.Fatalf("page2 ID %s also on page1", id)
			}
			seen[id] = 2
		}
		for _, id := range page3IDs {
			if p := seen[id]; p != 0 {
				t.Fatalf("page3 ID %s also on page%d", id, p)
			}
		}
	})

	t.Run("offset_past_total", func(t *testing.T) {
		env := mustList(t, c, q+"&offset=100&limit=5")
		if env.Meta.Total != paginationN {
			t.Fatalf("total: want %d, got %d", paginationN, env.Meta.Total)
		}
		ids := mustDecodeIDs(t, env.Data)
		if len(ids) != 0 {
			t.Fatalf("past total len: want 0, got %d", len(ids))
		}
	})

	t.Run("default_params", func(t *testing.T) {
		env := mustList(t, c, base+"/users")
		if env.Meta.Total != paginationN {
			t.Fatalf("total: want %d, got %d", paginationN, env.Meta.Total)
		}
		if env.Meta.Offset != 0 {
			t.Fatalf("default offset: want 0, got %d", env.Meta.Offset)
		}
		if env.Meta.Limit <= 0 {
			t.Fatalf("default limit should be positive, got %d", env.Meta.Limit)
		}
	})
}

func TestPagination_negativeLimitClamped(t *testing.T) {
	c := httpClient()
	did := seedDomain(t, c, "pag-clamp-dom")
	cleanupDelete(t, c, apiBase()+"/domains/"+did)

	// The API clamps limit < 1 to 1 rather than rejecting with 400.
	env := mustList(t, c, domainBase(did)+"/users?limit=-1")
	if env.Meta.Limit != 1 {
		t.Fatalf("negative limit should be clamped to 1, got %d", env.Meta.Limit)
	}
}
