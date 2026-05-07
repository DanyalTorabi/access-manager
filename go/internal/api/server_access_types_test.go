package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/dtorabi/access-manager/internal/store"
)

func TestAPI_accessTypeCreateList_invalidBit(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	base := ts.URL + "/api/v1/domains/" + dom.ID

	resBad, err := http.Post(base+"/access-types", "application/json", strings.NewReader(`{"title":"read","bit":"nope"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resBad.Body.Close() }()
	if resBad.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(resBad.Body)
		t.Fatalf("want 400 invalid bit, got %d: %s", resBad.StatusCode, b)
	}

	var at store.AccessType
	if err := json.Unmarshal(mustPostJSON201(t, base+"/access-types", `{"title":"read","bit":"0x1"}`), &at); err != nil {
		t.Fatal(err)
	}
	if at.ID == "" || at.Title != "read" || at.Bit != 1 {
		t.Fatalf("access type: %+v", at)
	}

	resList, err := http.Get(base + "/access-types")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resList.Body.Close() }()
	if resList.StatusCode != http.StatusOK {
		t.Fatalf("list status %d", resList.StatusCode)
	}
	var env listResponse[store.AccessType]
	if err := json.NewDecoder(resList.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data) != 1 || env.Data[0].ID != at.ID {
		t.Fatalf("list: %+v", env.Data)
	}
}

func TestAPI_accessTypeCreate_unknownField(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	res, err := http.Post(ts.URL+"/api/v1/domains/"+dom.ID+"/access-types", "application/json", strings.NewReader(`{"title":"x","bit":"1","extra":1}`))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("want 400, got %d: %s", res.StatusCode, b)
	}
}

func TestAPI_accessTypeList_empty(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := mustCreateDomain(t, ts)
	res, err := http.Get(ts.URL + "/api/v1/domains/" + domID + "/access-types")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	var env listResponse[store.AccessType]
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data) != 0 {
		t.Fatalf("want empty list, got %d items", len(env.Data))
	}
}

// --- duplicate-create 409 tests ---

func TestAPI_accessTypeCreate_duplicateBit(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	base := ts.URL + "/api/v1/domains/" + dom.ID
	mustPostJSON201(t, base+"/access-types", `{"title":"read","bit":"0x1"}`)

	res, err := http.Post(base+"/access-types", "application/json", strings.NewReader(`{"title":"write","bit":"0x1"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusConflict {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("duplicate bit want 409, got %d: %s", res.StatusCode, b)
	}
}

func TestAPI_accessTypeList_defaultSortByTitle(t *testing.T) {
	ts, _ := newTestAPI(t)
	domainID := mustCreateDomain(t, ts)
	base := ts.URL + "/api/v1/domains/" + domainID

	mustPostJSON201(t, base+"/access-types", `{"title":"Zebra","bit":"0x1"}`)
	mustPostJSON201(t, base+"/access-types", `{"title":"Alpha","bit":"0x2"}`)
	mustPostJSON201(t, base+"/access-types", `{"title":"Middle","bit":"0x4"}`)

	res, err := http.Get(base + "/access-types")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d: %s", res.StatusCode, b)
	}
	var env listResponse[store.AccessType]
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data) != 3 {
		t.Fatalf("want 3 items, got %d", len(env.Data))
	}
	if env.Data[0].Title != "Alpha" || env.Data[1].Title != "Middle" || env.Data[2].Title != "Zebra" {
		t.Fatalf("expected title-asc order, got %q %q %q",
			env.Data[0].Title, env.Data[1].Title, env.Data[2].Title)
	}
	if env.Meta.Sort != "title" || env.Meta.Order != "asc" {
		t.Fatalf("meta: sort=%q order=%q", env.Meta.Sort, env.Meta.Order)
	}
}

func TestAPI_accessTypeList_search(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := mustCreateDomain(t, ts)
	base := ts.URL + "/api/v1/domains/" + domID
	for i, title := range []string{"read", "write", "readonly"} {
		mustPostJSON201(t, base+"/access-types", fmt.Sprintf(`{"title":%q,"bit":"%d"}`, title, 1<<i))
	}

	res, err := http.Get(base + "/access-types?search=read")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d: %s", res.StatusCode, b)
	}
	var env listResponse[store.AccessType]
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if env.Meta.Total != 2 || len(env.Data) != 2 {
		t.Fatalf("want 2, got total=%d len=%d", env.Meta.Total, len(env.Data))
	}
}

func TestAPI_accessTypeList_sortDesc(t *testing.T) {
	ts, _ := newTestAPI(t)
	domainID := mustCreateDomain(t, ts)
	base := ts.URL + "/api/v1/domains/" + domainID

	mustPostJSON201(t, base+"/access-types", `{"title":"Alpha","bit":"0x1"}`)
	mustPostJSON201(t, base+"/access-types", `{"title":"Beta","bit":"0x2"}`)
	mustPostJSON201(t, base+"/access-types", `{"title":"Gamma","bit":"0x4"}`)

	res, err := http.Get(base + "/access-types?sort=title&order=desc")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d: %s", res.StatusCode, b)
	}
	var env listResponse[store.AccessType]
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data) != 3 {
		t.Fatalf("want 3, got %d", len(env.Data))
	}
	if env.Data[0].Title != "Gamma" || env.Data[2].Title != "Alpha" {
		t.Fatalf("order: got %q %q %q", env.Data[0].Title, env.Data[1].Title, env.Data[2].Title)
	}
	if env.Meta.Sort != "title" || env.Meta.Order != "desc" {
		t.Fatalf("meta: sort=%q order=%q", env.Meta.Sort, env.Meta.Order)
	}
}

func TestAPI_accessTypeList_invalidSort(t *testing.T) {
	ts, _ := newTestAPI(t)
	domainID := mustCreateDomain(t, ts)
	res, err := http.Get(ts.URL + "/api/v1/domains/" + domainID + "/access-types?sort=bad")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", res.StatusCode)
	}
}

func TestAPI_accessTypeList_invalidOrder(t *testing.T) {
	ts, _ := newTestAPI(t)
	domainID := mustCreateDomain(t, ts)
	res, err := http.Get(ts.URL + "/api/v1/domains/" + domainID + "/access-types?order=bad")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", res.StatusCode)
	}
}

// TestPublicInvalidInputMsg_typedExtraction asserts that publicInvalidInputMsg
// uses errors.As (not string-prefix parsing): a typed
// store.InvalidInputError detail must be returned to the client even when
// wrapped by an outer fmt.Errorf("%w", err). This is the regression that T48
// fixed.

func TestAPI_accessTypePatch_invalidBit(t *testing.T) {
	ts, _ := newTestAPI(t)
	dom := mustCreateDomain(t, ts)
	base := ts.URL + "/api/v1/domains/" + dom
	atBody := mustPostJSON201(t, base+"/access-types", `{"title":"read","bit":"0x1"}`)
	var at store.AccessType
	if err := json.Unmarshal(atBody, &at); err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest(http.MethodPatch, base+"/access-types/"+at.ID,
		strings.NewReader(`{"bit":"notanumber"}`))
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400 for invalid bit, got %d", res.StatusCode)
	}
}

func TestAPI_accessTypePatch_bitOnly(t *testing.T) {
	ts, _ := newTestAPI(t)
	dom := mustCreateDomain(t, ts)
	base := ts.URL + "/api/v1/domains/" + dom
	atBody := mustPostJSON201(t, base+"/access-types", `{"title":"read","bit":"0x1"}`)
	var at store.AccessType
	if err := json.Unmarshal(atBody, &at); err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest(http.MethodPatch, base+"/access-types/"+at.ID,
		strings.NewReader(`{"bit":"0x4"}`))
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", res.StatusCode, b)
	}
	var got store.AccessType
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Bit != 4 {
		t.Fatalf("bit: want 4, got %d", got.Bit)
	}
}

// TestAPI_accessMask_rejectsBit63 documents the temporary 63-bit limit on
// access masks (issue #67 / T46). Bit 63 (1<<63) would overflow signed-64
// storage in SQLite, so the API rejects it with 400 on access-type and
// permission create/patch. Values <= MaxInt64 are accepted.
func TestAPI_accessMask_rejectsBit63(t *testing.T) {
	ts, _ := newTestAPI(t)
	dom := mustCreateDomain(t, ts)
	base := ts.URL + "/api/v1/domains/" + dom
	rBody := mustPostJSON201(t, base+"/resources", `{"title":"r"}`)
	var resrc store.Resource
	if err := json.Unmarshal(rBody, &resrc); err != nil {
		t.Fatal(err)
	}

	// 1<<63 is the first value that overflows signed-64 and must be rejected.
	const tooBig = `"0x8000000000000000"`
	// MaxInt64 == 1<<63 - 1 must still be accepted at the API boundary.
	const maxOK = `"0x7FFFFFFFFFFFFFFF"`

	postBad := func(t *testing.T, url, body string) {
		t.Helper()
		res, err := http.Post(url, "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		if res.StatusCode != http.StatusBadRequest {
			t.Fatalf("want 400, got %d: %s", res.StatusCode, b)
		}
		if !strings.Contains(string(b), "mask value must be within signed 64-bit range") {
			t.Fatalf("want stable error message, got %s", b)
		}
	}
	patchBad := func(t *testing.T, url, body string) {
		t.Helper()
		req, _ := http.NewRequest(http.MethodPatch, url, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		if res.StatusCode != http.StatusBadRequest {
			t.Fatalf("want 400, got %d: %s", res.StatusCode, b)
		}
		if !strings.Contains(string(b), "mask value must be within signed 64-bit range") {
			t.Fatalf("want stable error message, got %s", b)
		}
	}

	t.Run("accessTypeCreate_bit63", func(t *testing.T) {
		postBad(t, base+"/access-types", `{"title":"x","bit":`+tooBig+`}`)
	})
	t.Run("permissionCreate_bit63", func(t *testing.T) {
		postBad(t, base+"/permissions", `{"title":"p","resource_id":"`+resrc.ID+`","access_mask":`+tooBig+`}`)
	})

	// Create a valid access type and permission to use for patch tests.
	atBody := mustPostJSON201(t, base+"/access-types", `{"title":"read","bit":"0x1"}`)
	var at store.AccessType
	if err := json.Unmarshal(atBody, &at); err != nil {
		t.Fatal(err)
	}
	pBody := mustPostJSON201(t, base+"/permissions", `{"title":"p","resource_id":"`+resrc.ID+`","access_mask":"0x1"}`)
	var perm store.Permission
	if err := json.Unmarshal(pBody, &perm); err != nil {
		t.Fatal(err)
	}

	t.Run("accessTypePatch_bit63", func(t *testing.T) {
		patchBad(t, base+"/access-types/"+at.ID, `{"bit":`+tooBig+`}`)
	})
	t.Run("permissionPatch_bit63", func(t *testing.T) {
		patchBad(t, base+"/permissions/"+perm.ID, `{"access_mask":`+tooBig+`}`)
	})

	// MaxInt64 (bit 62 fully set) is accepted on create and round-trips.
	t.Run("accessTypeCreate_maxInt64_ok", func(t *testing.T) {
		body := mustPostJSON201(t, base+"/access-types", `{"title":"max","bit":`+maxOK+`}`)
		var got store.AccessType
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatal(err)
		}
		if got.Bit != 1<<63-1 {
			t.Fatalf("bit: want %d, got %d", uint64(1<<63-1), got.Bit)
		}
	})
	t.Run("permissionCreate_maxInt64_ok", func(t *testing.T) {
		body := mustPostJSON201(t, base+"/permissions",
			`{"title":"pmax","resource_id":"`+resrc.ID+`","access_mask":`+maxOK+`}`)
		var got store.Permission
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatal(err)
		}
		if got.AccessMask != 1<<63-1 {
			t.Fatalf("mask: want %d, got %d", uint64(1<<63-1), got.AccessMask)
		}
	})
}
