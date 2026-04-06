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

func TestPagination_users(t *testing.T) {
	c := httpClient()
	did := seedDomain(t, c, "pagination-dom")
	base := domainBase(did)

	for i := 0; i < paginationN; i++ {
		seedUser(t, c, did, fmt.Sprintf("page-user-%02d", i))
	}

	t.Run("page1", func(t *testing.T) {
		env := mustList(t, c, base+"/users?offset=0&limit=5")
		if env.Meta.Total != paginationN {
			t.Fatalf("total: want %d, got %d", paginationN, env.Meta.Total)
		}
		var items []entityTitle
		if err := json.Unmarshal(env.Data, &items); err != nil {
			t.Fatal(err)
		}
		if len(items) != 5 {
			t.Fatalf("page1 len: want 5, got %d", len(items))
		}
	})

	t.Run("page2", func(t *testing.T) {
		env := mustList(t, c, base+"/users?offset=5&limit=5")
		if env.Meta.Total != paginationN {
			t.Fatalf("total: want %d, got %d", paginationN, env.Meta.Total)
		}
		var items []entityTitle
		if err := json.Unmarshal(env.Data, &items); err != nil {
			t.Fatal(err)
		}
		if len(items) != 5 {
			t.Fatalf("page2 len: want 5, got %d", len(items))
		}
	})

	t.Run("page3_partial", func(t *testing.T) {
		env := mustList(t, c, base+"/users?offset=10&limit=5")
		if env.Meta.Total != paginationN {
			t.Fatalf("total: want %d, got %d", paginationN, env.Meta.Total)
		}
		var items []entityTitle
		if err := json.Unmarshal(env.Data, &items); err != nil {
			t.Fatal(err)
		}
		if len(items) != 2 {
			t.Fatalf("page3 len: want 2, got %d", len(items))
		}
	})

	t.Run("offset_past_total", func(t *testing.T) {
		env := mustList(t, c, base+"/users?offset=100&limit=5")
		if env.Meta.Total != paginationN {
			t.Fatalf("total: want %d, got %d", paginationN, env.Meta.Total)
		}
		var items []entityTitle
		if err := json.Unmarshal(env.Data, &items); err != nil {
			t.Fatal(err)
		}
		if len(items) != 0 {
			t.Fatalf("past total len: want 0, got %d", len(items))
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
