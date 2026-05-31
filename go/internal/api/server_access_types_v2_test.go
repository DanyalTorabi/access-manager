package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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

	var at1 accessTypeResponseV2
	if err := json.Unmarshal(mustPostJSON201(t, domainBaseV2(ts, domID)+"/access-types", `{"title":"read"}`), &at1); err != nil {
		t.Fatal(err)
	}
	if at1.Bit != 1 {
		t.Fatalf("first auto-allocated bit: want 1, got %d", at1.Bit)
	}
	if at1.DomainID != domID {
		t.Fatalf("first auto-allocated: want domain_id %q, got %q", domID, at1.DomainID)
	}

	var at2 accessTypeResponseV2
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

	var at accessTypeResponseV2
	if err := json.Unmarshal(mustPostJSON201(t, domainBaseV2(ts, domID)+"/access-types", `{"title":"delete","bit":"4"}`), &at); err != nil {
		t.Fatal(err)
	}
	if at.Bit != 4 || at.Title != "delete" {
		t.Fatalf("explicit bit: want Bit=4 Title=delete, got %+v", at)
	}
	// Auto-allocate after explicit creates: lowest unused should be 1 (not 2 or 4)
	var at2 accessTypeResponseV2
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

func TestAPI_v2_accessTypeCreate_nonPowerOfTwoBit_400(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")

	// 3 is a valid uint64 but not a power of two (covers bits 0 and 1 simultaneously).
	mustPostJSON(t, domainBaseV2(ts, domID)+"/access-types", `{"title":"rw","bit":"3"}`, http.StatusBadRequest)
	// 0 is also explicitly rejected.
	mustPostJSON(t, domainBaseV2(ts, domID)+"/access-types", `{"title":"zero","bit":"0"}`, http.StatusBadRequest)
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

	// Create via v2; decode into accessTypeResponseV2 to validate snake_case fields.
	var created accessTypeResponseV2
	if err := json.Unmarshal(mustPostJSON201(t, domainBaseV2(ts, domID)+"/access-types", `{"title":"read"}`), &created); err != nil {
		t.Fatal(err)
	}

	// GET via v1 should succeed and return the same record (PascalCase).
	b := mustGet(t, domainBase(ts, domID)+"/access-types/"+created.ID, http.StatusOK)
	var got store.AccessType
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != created.ID || got.Bit != created.Bit || got.Title != created.Title {
		t.Fatalf("v1 GET: want id=%s bit=%d title=%s, got %+v", created.ID, created.Bit, created.Title, got)
	}
}

// --- List tests (T70) ---

func TestAPI_v2_accessTypeList_empty(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")

	b := mustGet(t, domainBaseV2(ts, domID)+"/access-types", http.StatusOK)
	var env listResponse[store.AccessType]
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data) != 0 {
		t.Fatalf("want empty list, got %d items", len(env.Data))
	}
}

func TestAPI_v2_accessTypeList_multiple(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")

	// Create 3 via V2 (auto-allocated bits: 1, 2, 4).
	for _, title := range []string{"Zebra", "Alpha", "Middle"} {
		mustPostJSON201(t, domainBaseV2(ts, domID)+"/access-types", fmt.Sprintf(`{"title":%q}`, title))
	}

	// Default sort: title asc.
	b := mustGet(t, domainBaseV2(ts, domID)+"/access-types", http.StatusOK)
	var env listResponse[store.AccessType]
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatal(err)
	}
	if env.Meta.Total != 3 || len(env.Data) != 3 {
		t.Fatalf("want total=3 len=3, got total=%d len=%d", env.Meta.Total, len(env.Data))
	}
	if env.Data[0].Title != "Alpha" || env.Data[1].Title != "Middle" || env.Data[2].Title != "Zebra" {
		t.Fatalf("expected title-asc order, got %q %q %q",
			env.Data[0].Title, env.Data[1].Title, env.Data[2].Title)
	}
	if env.Meta.Sort != "title" || env.Meta.Order != "asc" {
		t.Fatalf("meta: sort=%q order=%q", env.Meta.Sort, env.Meta.Order)
	}

	// Search by title substring.
	bSearch := mustGet(t, domainBaseV2(ts, domID)+"/access-types?search=a", http.StatusOK)
	var envSearch listResponse[store.AccessType]
	if err := json.Unmarshal(bSearch, &envSearch); err != nil {
		t.Fatal(err)
	}
	if envSearch.Meta.Total != 2 {
		t.Fatalf("search 'a': want total=2, got %d", envSearch.Meta.Total)
	}

	// Sort desc.
	bDesc := mustGet(t, domainBaseV2(ts, domID)+"/access-types?sort=title&order=desc", http.StatusOK)
	var envDesc listResponse[store.AccessType]
	if err := json.Unmarshal(bDesc, &envDesc); err != nil {
		t.Fatal(err)
	}
	if len(envDesc.Data) != 3 || envDesc.Data[0].Title != "Zebra" {
		t.Fatalf("desc sort: first want Zebra, got %q", envDesc.Data[0].Title)
	}

	// Pagination: limit=1 offset=1.
	bPage := mustGet(t, domainBaseV2(ts, domID)+"/access-types?limit=1&offset=1", http.StatusOK)
	var envPage listResponse[store.AccessType]
	if err := json.Unmarshal(bPage, &envPage); err != nil {
		t.Fatal(err)
	}
	if envPage.Meta.Total != 3 || len(envPage.Data) != 1 {
		t.Fatalf("page: want total=3 len=1, got total=%d len=%d", envPage.Meta.Total, len(envPage.Data))
	}
	if envPage.Data[0].Title != "Middle" {
		t.Fatalf("page[1]: want Middle, got %q", envPage.Data[0].Title)
	}
}

func TestAPI_v2_accessTypeList_invalidSort(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")
	mustGet(t, domainBaseV2(ts, domID)+"/access-types?sort=bad_field", http.StatusBadRequest)
}

func TestAPI_v2_accessTypeList_invalidOrder(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")
	mustGet(t, domainBaseV2(ts, domID)+"/access-types?order=diagonal", http.StatusBadRequest)
}

// TestAPI_v2_accessTypePatch_titleOnly patches an access type via the V2 path
// (which reuses the V1 handler) and verifies V1 GET returns the updated title.
func TestAPI_v2_accessTypePatch_titleOnly(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")

	// Create via V2.
	var created accessTypeResponseV2
	if err := json.Unmarshal(mustPostJSON201(t, domainBaseV2(ts, domID)+"/access-types", `{"title":"original"}`), &created); err != nil {
		t.Fatal(err)
	}

	// PATCH title via V2 path.
	patchBody := `{"title":"updated"}`
	patchReq := strings.NewReader(patchBody)
	req, err := http.NewRequest(http.MethodPatch, domainBaseV2(ts, domID)+"/access-types/"+created.ID, patchReq)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := testClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("PATCH via v2: want 200, got %d", res.StatusCode)
	}

	// V1 GET same ID must return the updated title (backward-compatibility check).
	bV1 := mustGet(t, domainBase(ts, domID)+"/access-types/"+created.ID, http.StatusOK)
	var gotV1 store.AccessType
	if err := json.Unmarshal(bV1, &gotV1); err != nil {
		t.Fatal(err)
	}
	if gotV1.Title != "updated" {
		t.Fatalf("V1 GET after V2 PATCH: want title=updated, got %q", gotV1.Title)
	}
	// Bit must be unchanged.
	if gotV1.Bit != created.Bit {
		t.Fatalf("V1 GET after V2 PATCH: want bit=%d, got %d", created.Bit, gotV1.Bit)
	}
}


