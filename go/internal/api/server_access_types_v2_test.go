package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dtorabi/access-manager/internal/store"
)

// domainBaseV2 returns the /api/v2/domains/{domainID} base path for a test server.
func domainBaseV2(ts *httptest.Server, domainID string) string {
	return ts.URL + "/api/v2/domains/" + domainID
}

func TestAPI_v2_accessTypeCreate_autoAllocatesLowestBit(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")

	var at1 store.AccessType
	if err := json.Unmarshal(mustPostJSON201(t, domainBaseV2(ts, domID)+"/access-types", `{"title":"read"}`), &at1); err != nil {
		t.Fatal(err)
	}
	if at1.Bit != 1 {
		t.Fatalf("first auto-allocated bit: want 1, got %d", at1.Bit)
	}

	var at2 store.AccessType
	if err := json.Unmarshal(mustPostJSON201(t, domainBaseV2(ts, domID)+"/access-types", `{"title":"write"}`), &at2); err != nil {
		t.Fatal(err)
	}
	if at2.Bit != 2 {
		t.Fatalf("second auto-allocated bit: want 2, got %d", at2.Bit)
	}
}

func TestAPI_v2_accessTypeCreate_withExplicitBit(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")

	var at store.AccessType
	if err := json.Unmarshal(mustPostJSON201(t, domainBaseV2(ts, domID)+"/access-types", `{"title":"delete","bit":"4"}`), &at); err != nil {
		t.Fatal(err)
	}
	if at.Bit != 4 || at.Title != "delete" {
		t.Fatalf("explicit bit: want Bit=4 Title=delete, got %+v", at)
	}
	// Auto-allocate after explicit creates: lowest unused should be 1 (not 2 or 4)
	var at2 store.AccessType
	if err := json.Unmarshal(mustPostJSON201(t, domainBaseV2(ts, domID)+"/access-types", `{"title":"read"}`), &at2); err != nil {
		t.Fatal(err)
	}
	if at2.Bit != 1 {
		t.Fatalf("auto-alloc after explicit 4: want 1, got %d", at2.Bit)
	}
}

func TestAPI_v2_accessTypeCreate_duplicateTitle_409(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")

	mustPostJSON201(t, domainBaseV2(ts, domID)+"/access-types", `{"title":"read"}`)
	mustPostJSON(t, domainBaseV2(ts, domID)+"/access-types", `{"title":"read"}`, http.StatusConflict)
}

func TestAPI_v2_accessTypeCreate_invalidBit_400(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")

	mustPostJSON(t, domainBaseV2(ts, domID)+"/access-types", `{"title":"read","bit":"nope"}`, http.StatusBadRequest)
}

func TestAPI_v2_accessTypeCreate_allBitsExhausted_409(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")

	// Fill all 63 bits via v1 (explicit bit values).
	for pos := uint64(0); pos <= 62; pos++ {
		bit := uint64(1) << pos
		body := fmt.Sprintf(`{"title":%q,"bit":"%d"}`, fmt.Sprintf("type-%d", pos), bit)
		mustPostJSON201(t, domainBase(ts, domID)+"/access-types", body)
	}

	// Now v2 auto-alloc must fail with 409.
	mustPostJSON(t, domainBaseV2(ts, domID)+"/access-types", `{"title":"overflow"}`, http.StatusConflict)
}

func TestAPI_v2_accessTypeCreate_v1StillWorksOnV2Types(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")

	// Create via v2.
	var created store.AccessType
	if err := json.Unmarshal(mustPostJSON201(t, domainBaseV2(ts, domID)+"/access-types", `{"title":"read"}`), &created); err != nil {
		t.Fatal(err)
	}

	// GET via v1 should succeed and return the same record.
	b := mustGet(t, domainBase(ts, domID)+"/access-types/"+created.ID, http.StatusOK)
	var got store.AccessType
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != created.ID || got.Bit != created.Bit || got.Title != created.Title {
		t.Fatalf("v1 GET: want %+v, got %+v", created, got)
	}
}
