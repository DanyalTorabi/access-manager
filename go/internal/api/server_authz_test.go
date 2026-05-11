package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/dtorabi/access-manager/internal/store"
	sqlstore "github.com/dtorabi/access-manager/internal/store/sqlite"
	"github.com/dtorabi/access-manager/internal/testutil"
	"github.com/google/uuid"
)

func TestAPI_authzCheck_validation(t *testing.T) {
	ts, _ := newTestAPI(t)
	domainID := uuid.NewString()
	url := ts.URL + "/api/v1/domains/" + domainID + "/authz/check?user_id=u&resource_id=r"
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400 missing access_bit, got %d", res.StatusCode)
	}
}

func TestAPI_authzCheck_viaGroup_integration(t *testing.T) {
	ts, st := newTestAPI(t)
	ctx := context.Background()

	domainID := uuid.NewString()
	uid := uuid.NewString()
	gid := uuid.NewString()
	rid := uuid.NewString()
	pid := uuid.NewString()

	if err := st.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	if err := st.UserCreate(ctx, &store.User{ID: uid, DomainID: domainID, Title: "u"}); err != nil {
		t.Fatal(err)
	}
	if err := st.GroupCreate(ctx, &store.Group{ID: gid, DomainID: domainID, Title: "g"}); err != nil {
		t.Fatal(err)
	}
	if err := st.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}
	if err := st.PermissionCreate(ctx, &store.Permission{ID: pid, DomainID: domainID, Title: "p", ResourceID: rid, AccessMask: 0x3}); err != nil {
		t.Fatal(err)
	}
	if err := st.AddUserToGroup(ctx, domainID, uid, gid); err != nil {
		t.Fatal(err)
	}
	if err := st.GrantGroupPermission(ctx, domainID, gid, pid); err != nil {
		t.Fatal(err)
	}

	q := ts.URL + "/api/v1/domains/" + domainID + "/authz/check?user_id=" + uid + "&resource_id=" + rid + "&access_bit=0x1"
	res, err := http.Get(q)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d: %s", res.StatusCode, body)
	}
	var out struct {
		Allowed       bool   `json:"allowed"`
		EffectiveMask string `json:"effective_mask"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if !out.Allowed {
		t.Fatalf("expected allowed, got %+v", out)
	}
}

func TestAPI_authzCheck_invalidAccessBit(t *testing.T) {
	ts, _ := newTestAPI(t)
	did := uuid.NewString()
	url := fmt.Sprintf("%s/api/v1/domains/%s/authz/check?user_id=u&resource_id=r&access_bit=xyz", ts.URL, did)
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("want 400 invalid access_bit, got %d: %s", res.StatusCode, body)
	}
}

func TestAPI_authzCheck_deniedWithoutGrants(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	if dom.ID == "" {
		t.Fatal("empty domain id")
	}

	var user store.User
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains/"+dom.ID+"/users", `{"title":"u"}`), &user); err != nil {
		t.Fatal(err)
	}
	if user.ID == "" {
		t.Fatal("empty user id")
	}

	var resource store.Resource
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains/"+dom.ID+"/resources", `{"title":"r"}`), &resource); err != nil {
		t.Fatal(err)
	}
	if resource.ID == "" {
		t.Fatal("empty resource id")
	}

	q := fmt.Sprintf("%s/api/v1/domains/%s/authz/check?user_id=%s&resource_id=%s&access_bit=0x1",
		ts.URL, dom.ID, user.ID, resource.ID)
	res4, err := http.Get(q)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res4.Body.Close() }()
	if res4.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res4.Body)
		t.Fatalf("status %d: %s", res4.StatusCode, body)
	}
	var out struct {
		Allowed bool `json:"allowed"`
	}
	if err := json.NewDecoder(res4.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Allowed {
		t.Fatalf("expected denied without grants, got %+v", out)
	}
}

func TestAPI_authzMasks_validation(t *testing.T) {
	ts, _ := newTestAPI(t)
	did := uuid.NewString()
	res, err := http.Get(ts.URL + "/api/v1/domains/" + did + "/authz/masks?user_id=u")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400 missing resource_id, got %d", res.StatusCode)
	}
}

func TestAPI_userAuthzResources_integration(t *testing.T) {
	ts, st := newTestAPI(t)
	ctx := context.Background()

	domainID := uuid.NewString()
	uid := uuid.NewString()
	gid := uuid.NewString()
	ridA := uuid.NewString()
	ridB := uuid.NewString()
	ridC := uuid.NewString()

	if err := st.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	if err := st.UserCreate(ctx, &store.User{ID: uid, DomainID: domainID, Title: "u"}); err != nil {
		t.Fatal(err)
	}
	if err := st.GroupCreate(ctx, &store.Group{ID: gid, DomainID: domainID, Title: "g"}); err != nil {
		t.Fatal(err)
	}
	if err := st.AddUserToGroup(ctx, domainID, uid, gid); err != nil {
		t.Fatal(err)
	}
	for _, rid := range []string{ridA, ridB, ridC} {
		if err := st.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r" + rid}); err != nil {
			t.Fatal(err)
		}
	}

	pUserA := uuid.NewString()
	pGroupA := uuid.NewString()
	pGroupB := uuid.NewString()
	pUserC1 := uuid.NewString()
	pUserC2 := uuid.NewString()

	if err := st.PermissionCreate(ctx, &store.Permission{ID: pUserA, DomainID: domainID, Title: "pUserA", ResourceID: ridA, AccessMask: 0x1}); err != nil {
		t.Fatal(err)
	}
	if err := st.PermissionCreate(ctx, &store.Permission{ID: pGroupA, DomainID: domainID, Title: "pGroupA", ResourceID: ridA, AccessMask: 0x4}); err != nil {
		t.Fatal(err)
	}
	if err := st.PermissionCreate(ctx, &store.Permission{ID: pGroupB, DomainID: domainID, Title: "pGroupB", ResourceID: ridB, AccessMask: 0x2}); err != nil {
		t.Fatal(err)
	}
	if err := st.PermissionCreate(ctx, &store.Permission{ID: pUserC1, DomainID: domainID, Title: "pUserC1", ResourceID: ridC, AccessMask: 0x8}); err != nil {
		t.Fatal(err)
	}
	if err := st.PermissionCreate(ctx, &store.Permission{ID: pUserC2, DomainID: domainID, Title: "pUserC2", ResourceID: ridC, AccessMask: 0x10}); err != nil {
		t.Fatal(err)
	}

	if err := st.GrantUserPermission(ctx, domainID, uid, pUserA); err != nil {
		t.Fatal(err)
	}
	if err := st.GrantGroupPermission(ctx, domainID, gid, pGroupA); err != nil {
		t.Fatal(err)
	}
	if err := st.GrantGroupPermission(ctx, domainID, gid, pGroupB); err != nil {
		t.Fatal(err)
	}
	if err := st.GrantUserPermission(ctx, domainID, uid, pUserC1); err != nil {
		t.Fatal(err)
	}
	if err := st.GrantUserPermission(ctx, domainID, uid, pUserC2); err != nil {
		t.Fatal(err)
	}

	base := ts.URL + "/api/v1/domains/" + domainID + "/users/" + uid + "/authz/resources"

	res, err := http.Get(base + "?offset=0&limit=10")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d: %s", res.StatusCode, b)
	}
	var env listResponse[struct {
		ResourceID    string `json:"resource_id"`
		EffectiveMask string `json:"effective_mask"`
	}]
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if env.Meta.Total != 3 {
		t.Fatalf("total: want 3, got %d", env.Meta.Total)
	}
	if len(env.Data) != 3 {
		t.Fatalf("len: want 3, got %d", len(env.Data))
	}
	gotMasks := map[string]string{}
	for _, it := range env.Data {
		gotMasks[it.ResourceID] = it.EffectiveMask
	}
	if gotMasks[ridA] != "5" {
		t.Fatalf("ridA mask: want 5, got %q", gotMasks[ridA])
	}
	if gotMasks[ridB] != "2" {
		t.Fatalf("ridB mask: want 2, got %q", gotMasks[ridB])
	}
	if gotMasks[ridC] != "24" {
		t.Fatalf("ridC mask: want 24, got %q", gotMasks[ridC])
	}

	resPage, err := http.Get(base + "?offset=1&limit=1")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resPage.Body.Close() }()
	if resPage.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resPage.Body)
		t.Fatalf("page status %d: %s", resPage.StatusCode, b)
	}
	var page listResponse[struct {
		ResourceID    string `json:"resource_id"`
		EffectiveMask string `json:"effective_mask"`
	}]
	if err := json.NewDecoder(resPage.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	if page.Meta.Total != 3 || len(page.Data) != 1 {
		t.Fatalf("page total=%d len=%d", page.Meta.Total, len(page.Data))
	}
	orderedIDs := []string{ridA, ridB, ridC}
	sort.Strings(orderedIDs)
	if page.Data[0].ResourceID != orderedIDs[1] {
		t.Fatalf("page resource: want %s, got %s", orderedIDs[1], page.Data[0].ResourceID)
	}
}

func TestAPI_userAuthzResources_unsupportedQueryParams(t *testing.T) {
	ts, st := newTestAPI(t)
	ctx := context.Background()
	domainID := uuid.NewString()
	uid := uuid.NewString()
	if err := st.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	if err := st.UserCreate(ctx, &store.User{ID: uid, DomainID: domainID, Title: "u"}); err != nil {
		t.Fatal(err)
	}
	res, err := http.Get(ts.URL + "/api/v1/domains/" + domainID + "/users/" + uid + "/authz/resources?search=foo")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("unsupported params: want 400, got %d: %s", res.StatusCode, b)
	}
	var out map[string]string
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out["error"] != "only limit and offset are supported" {
		t.Fatalf("unexpected error message: %q", out["error"])
	}

	longSearch := strings.Repeat("x", 300)
	resLong, err := http.Get(ts.URL + "/api/v1/domains/" + domainID + "/users/" + uid + "/authz/resources?search=" + longSearch)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resLong.Body.Close() }()
	if resLong.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(resLong.Body)
		t.Fatalf("unsupported long search: want 400, got %d: %s", resLong.StatusCode, b)
	}
	var outLong map[string]string
	if err := json.NewDecoder(resLong.Body).Decode(&outLong); err != nil {
		t.Fatal(err)
	}
	if outLong["error"] != "only limit and offset are supported" {
		t.Fatalf("unexpected long-search error message: %q", outLong["error"])
	}
}

func TestAPI_userAuthzResources_notFound(t *testing.T) {
	ts, st := newTestAPI(t)
	ctx := context.Background()
	domainID := uuid.NewString()
	if err := st.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	uid := uuid.NewString()
	if err := st.UserCreate(ctx, &store.User{ID: uid, DomainID: domainID, Title: "u"}); err != nil {
		t.Fatal(err)
	}

	resUnknownDomain, err := http.Get(ts.URL + "/api/v1/domains/" + uuid.NewString() + "/users/" + uid + "/authz/resources")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resUnknownDomain.Body.Close() }()
	if resUnknownDomain.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown domain: want 404, got %d", resUnknownDomain.StatusCode)
	}

	resUnknownUser, err := http.Get(ts.URL + "/api/v1/domains/" + domainID + "/users/" + uuid.NewString() + "/authz/resources")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resUnknownUser.Body.Close() }()
	if resUnknownUser.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown user: want 404, got %d", resUnknownUser.StatusCode)
	}
}

func TestAPI_groupAuthzResources_integration(t *testing.T) {
	ts, st := newTestAPI(t)
	ctx := context.Background()

	domainID := uuid.NewString()
	gid := uuid.NewString()
	ridA := uuid.NewString()
	ridB := uuid.NewString()

	if err := st.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	if err := st.GroupCreate(ctx, &store.Group{ID: gid, DomainID: domainID, Title: "g"}); err != nil {
		t.Fatal(err)
	}
	for _, rid := range []string{ridA, ridB} {
		if err := st.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r" + rid}); err != nil {
			t.Fatal(err)
		}
	}

	// Two permissions on ridA (OR of 0x1 | 0x4 = 0x5), one on ridB (0x2).
	pA1 := uuid.NewString()
	pA2 := uuid.NewString()
	pB := uuid.NewString()

	if err := st.PermissionCreate(ctx, &store.Permission{ID: pA1, DomainID: domainID, Title: "pA1", ResourceID: ridA, AccessMask: 0x1}); err != nil {
		t.Fatal(err)
	}
	if err := st.PermissionCreate(ctx, &store.Permission{ID: pA2, DomainID: domainID, Title: "pA2", ResourceID: ridA, AccessMask: 0x4}); err != nil {
		t.Fatal(err)
	}
	if err := st.PermissionCreate(ctx, &store.Permission{ID: pB, DomainID: domainID, Title: "pB", ResourceID: ridB, AccessMask: 0x2}); err != nil {
		t.Fatal(err)
	}
	if err := st.GrantGroupPermission(ctx, domainID, gid, pA1); err != nil {
		t.Fatal(err)
	}
	if err := st.GrantGroupPermission(ctx, domainID, gid, pA2); err != nil {
		t.Fatal(err)
	}
	if err := st.GrantGroupPermission(ctx, domainID, gid, pB); err != nil {
		t.Fatal(err)
	}

	base := ts.URL + "/api/v1/domains/" + domainID + "/groups/" + gid + "/authz/resources"

	res, err := http.Get(base + "?offset=0&limit=10")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d: %s", res.StatusCode, b)
	}
	var env listResponse[struct {
		ResourceID string `json:"resource_id"`
		Mask       string `json:"mask"`
	}]
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if env.Meta.Total != 2 {
		t.Fatalf("total: want 2, got %d", env.Meta.Total)
	}
	if len(env.Data) != 2 {
		t.Fatalf("len: want 2, got %d", len(env.Data))
	}
	gotMasks := map[string]string{}
	for _, it := range env.Data {
		gotMasks[it.ResourceID] = it.Mask
	}
	if gotMasks[ridA] != "5" {
		t.Fatalf("ridA mask: want 5, got %q", gotMasks[ridA])
	}
	if gotMasks[ridB] != "2" {
		t.Fatalf("ridB mask: want 2, got %q", gotMasks[ridB])
	}

	// Pagination.
	resPage, err := http.Get(base + "?offset=1&limit=1")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resPage.Body.Close() }()
	if resPage.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resPage.Body)
		t.Fatalf("page status %d: %s", resPage.StatusCode, b)
	}
	var page listResponse[struct {
		ResourceID string `json:"resource_id"`
		Mask       string `json:"mask"`
	}]
	if err := json.NewDecoder(resPage.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	if page.Meta.Total != 2 || len(page.Data) != 1 {
		t.Fatalf("page total=%d len=%d", page.Meta.Total, len(page.Data))
	}
	orderedIDs := []string{ridA, ridB}
	sort.Strings(orderedIDs)
	if page.Data[0].ResourceID != orderedIDs[1] {
		t.Fatalf("page resource: want %s, got %s", orderedIDs[1], page.Data[0].ResourceID)
	}
}

func TestAPI_groupAuthzResources_unsupportedQueryParams(t *testing.T) {
	ts, st := newTestAPI(t)
	ctx := context.Background()
	domainID := uuid.NewString()
	gid := uuid.NewString()
	if err := st.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	if err := st.GroupCreate(ctx, &store.Group{ID: gid, DomainID: domainID, Title: "g"}); err != nil {
		t.Fatal(err)
	}
	res, err := http.Get(ts.URL + "/api/v1/domains/" + domainID + "/groups/" + gid + "/authz/resources?search=foo")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("unsupported params: want 400, got %d: %s", res.StatusCode, b)
	}
	var out map[string]string
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out["error"] != "only limit and offset are supported" {
		t.Fatalf("unexpected error message: %q", out["error"])
	}
}

func TestAPI_groupAuthzResources_notFound(t *testing.T) {
	ts, st := newTestAPI(t)
	ctx := context.Background()
	domainID := uuid.NewString()
	if err := st.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	gid := uuid.NewString()
	if err := st.GroupCreate(ctx, &store.Group{ID: gid, DomainID: domainID, Title: "g"}); err != nil {
		t.Fatal(err)
	}

	resUnknownDomain, err := http.Get(ts.URL + "/api/v1/domains/" + uuid.NewString() + "/groups/" + gid + "/authz/resources")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resUnknownDomain.Body.Close() }()
	if resUnknownDomain.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown domain: want 404, got %d", resUnknownDomain.StatusCode)
	}

	resUnknownGroup, err := http.Get(ts.URL + "/api/v1/domains/" + domainID + "/groups/" + uuid.NewString() + "/authz/resources")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resUnknownGroup.Body.Close() }()
	if resUnknownGroup.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown group: want 404, got %d", resUnknownGroup.StatusCode)
	}
}

func TestAPI_resourceAuthzUsers_integration(t *testing.T) {
	ts, st := newTestAPI(t)
	ctx := context.Background()

	domainID := uuid.NewString()
	rid := uuid.NewString()
	uA := uuid.NewString()
	uB := uuid.NewString()

	if err := st.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	if err := st.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}
	for _, uid := range []string{uA, uB} {
		if err := st.UserCreate(ctx, &store.User{ID: uid, DomainID: domainID, Title: "u" + uid}); err != nil {
			t.Fatal(err)
		}
	}
	gid := uuid.NewString()
	if err := st.GroupCreate(ctx, &store.Group{ID: gid, DomainID: domainID, Title: "g"}); err != nil {
		t.Fatal(err)
	}
	if err := st.AddUserToGroup(ctx, domainID, uA, gid); err != nil {
		t.Fatal(err)
	}

	pDirectA1 := uuid.NewString()
	pDirectA2 := uuid.NewString()
	pGroup := uuid.NewString()
	if err := st.PermissionCreate(ctx, &store.Permission{ID: pDirectA1, DomainID: domainID, Title: "pA1", ResourceID: rid, AccessMask: 0x1}); err != nil {
		t.Fatal(err)
	}
	if err := st.PermissionCreate(ctx, &store.Permission{ID: pDirectA2, DomainID: domainID, Title: "pA2", ResourceID: rid, AccessMask: 0x4}); err != nil {
		t.Fatal(err)
	}
	if err := st.PermissionCreate(ctx, &store.Permission{ID: pGroup, DomainID: domainID, Title: "pG", ResourceID: rid, AccessMask: 0x2}); err != nil {
		t.Fatal(err)
	}
	if err := st.GrantUserPermission(ctx, domainID, uA, pDirectA1); err != nil {
		t.Fatal(err)
	}
	if err := st.GrantUserPermission(ctx, domainID, uA, pDirectA2); err != nil {
		t.Fatal(err)
	}
	if err := st.GrantGroupPermission(ctx, domainID, gid, pGroup); err != nil {
		t.Fatal(err)
	}

	base := ts.URL + "/api/v1/domains/" + domainID + "/resources/" + rid + "/authz/users"

	res, err := http.Get(base + "?offset=0&limit=10")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d: %s", res.StatusCode, b)
	}
	var env listResponse[struct {
		UserID        string `json:"user_id"`
		EffectiveMask string `json:"effective_mask"`
	}]
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if env.Meta.Total != 1 {
		t.Fatalf("total: want 1 (uB has no membership/grant), got %d", env.Meta.Total)
	}
	if len(env.Data) != 1 {
		t.Fatalf("len: want 1, got %d", len(env.Data))
	}
	if env.Data[0].UserID != uA {
		t.Fatalf("user: want %s, got %s", uA, env.Data[0].UserID)
	}
	if env.Data[0].EffectiveMask != "7" {
		t.Fatalf("uA mask: want 7 (0x1|0x4 direct | 0x2 group), got %q", env.Data[0].EffectiveMask)
	}

	// Add uB to the group too -> both users now appear.
	if err := st.AddUserToGroup(ctx, domainID, uB, gid); err != nil {
		t.Fatal(err)
	}
	res2, err := http.Get(base + "?offset=0&limit=10")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res2.Body.Close() }()
	if res2.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res2.StatusCode)
	}
	var env2 listResponse[struct {
		UserID        string `json:"user_id"`
		EffectiveMask string `json:"effective_mask"`
	}]
	if err := json.NewDecoder(res2.Body).Decode(&env2); err != nil {
		t.Fatal(err)
	}
	if env2.Meta.Total != 2 || len(env2.Data) != 2 {
		t.Fatalf("after second membership: total=%d len=%d", env2.Meta.Total, len(env2.Data))
	}
	gotMasks := map[string]string{}
	for _, it := range env2.Data {
		gotMasks[it.UserID] = it.EffectiveMask
	}
	if gotMasks[uA] != "7" {
		t.Fatalf("uA mask: want 7, got %q", gotMasks[uA])
	}
	if gotMasks[uB] != "2" {
		t.Fatalf("uB mask: want 2, got %q", gotMasks[uB])
	}

	// Pagination: ordered by user_id ASC.
	resPage, err := http.Get(base + "?offset=1&limit=1")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resPage.Body.Close() }()
	if resPage.StatusCode != http.StatusOK {
		t.Fatalf("page status %d", resPage.StatusCode)
	}
	var page listResponse[struct {
		UserID        string `json:"user_id"`
		EffectiveMask string `json:"effective_mask"`
	}]
	if err := json.NewDecoder(resPage.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	if page.Meta.Total != 2 || len(page.Data) != 1 {
		t.Fatalf("page: total=%d len=%d", page.Meta.Total, len(page.Data))
	}
	orderedIDs := []string{uA, uB}
	sort.Strings(orderedIDs)
	if page.Data[0].UserID != orderedIDs[1] {
		t.Fatalf("page user: want %s, got %s", orderedIDs[1], page.Data[0].UserID)
	}
	if page.Meta.Sort != "user_id" || page.Meta.Order != "asc" {
		t.Fatalf("page meta sort/order: got sort=%q order=%q", page.Meta.Sort, page.Meta.Order)
	}
}

func TestAPI_resourceAuthzUsers_unsupportedQueryParams(t *testing.T) {
	ts, st := newTestAPI(t)
	ctx := context.Background()
	domainID := uuid.NewString()
	rid := uuid.NewString()
	if err := st.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	if err := st.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}
	res, err := http.Get(ts.URL + "/api/v1/domains/" + domainID + "/resources/" + rid + "/authz/users?search=foo")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("unsupported params: want 400, got %d: %s", res.StatusCode, b)
	}
	var out map[string]string
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out["error"] != "only limit and offset are supported" {
		t.Fatalf("unexpected error message: %q", out["error"])
	}
}

func TestAPI_resourceAuthzUsers_notFound(t *testing.T) {
	ts, st := newTestAPI(t)
	ctx := context.Background()
	domainID := uuid.NewString()
	if err := st.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	rid := uuid.NewString()
	if err := st.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}

	resUnknownDomain, err := http.Get(ts.URL + "/api/v1/domains/" + uuid.NewString() + "/resources/" + rid + "/authz/users")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resUnknownDomain.Body.Close() }()
	if resUnknownDomain.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown domain: want 404, got %d", resUnknownDomain.StatusCode)
	}

	resUnknownResource, err := http.Get(ts.URL + "/api/v1/domains/" + domainID + "/resources/" + uuid.NewString() + "/authz/users")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resUnknownResource.Body.Close() }()
	if resUnknownResource.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown resource: want 404, got %d", resUnknownResource.StatusCode)
	}
}

func TestAPI_resourceAuthzGroups_integration(t *testing.T) {
	ts, st := newTestAPI(t)
	ctx := context.Background()

	domainID := uuid.NewString()
	rid := uuid.NewString()
	gA := uuid.NewString()
	gB := uuid.NewString()

	if err := st.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	if err := st.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}
	for _, gid := range []string{gA, gB} {
		if err := st.GroupCreate(ctx, &store.Group{ID: gid, DomainID: domainID, Title: "g" + gid}); err != nil {
			t.Fatal(err)
		}
	}

	// gA gets two grants on rid (OR = 0x1 | 0x4 = 0x5), gB gets one (0x2).
	pA1 := uuid.NewString()
	pA2 := uuid.NewString()
	pB := uuid.NewString()
	if err := st.PermissionCreate(ctx, &store.Permission{ID: pA1, DomainID: domainID, Title: "pA1", ResourceID: rid, AccessMask: 0x1}); err != nil {
		t.Fatal(err)
	}
	if err := st.PermissionCreate(ctx, &store.Permission{ID: pA2, DomainID: domainID, Title: "pA2", ResourceID: rid, AccessMask: 0x4}); err != nil {
		t.Fatal(err)
	}
	if err := st.PermissionCreate(ctx, &store.Permission{ID: pB, DomainID: domainID, Title: "pB", ResourceID: rid, AccessMask: 0x2}); err != nil {
		t.Fatal(err)
	}
	if err := st.GrantGroupPermission(ctx, domainID, gA, pA1); err != nil {
		t.Fatal(err)
	}
	if err := st.GrantGroupPermission(ctx, domainID, gA, pA2); err != nil {
		t.Fatal(err)
	}
	if err := st.GrantGroupPermission(ctx, domainID, gB, pB); err != nil {
		t.Fatal(err)
	}

	base := ts.URL + "/api/v1/domains/" + domainID + "/resources/" + rid + "/authz/groups"

	res, err := http.Get(base + "?offset=0&limit=10")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d: %s", res.StatusCode, b)
	}
	var env listResponse[struct {
		GroupID string `json:"group_id"`
		Mask    string `json:"mask"`
	}]
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if env.Meta.Total != 2 {
		t.Fatalf("total: want 2, got %d", env.Meta.Total)
	}
	if len(env.Data) != 2 {
		t.Fatalf("len: want 2, got %d", len(env.Data))
	}
	gotMasks := map[string]string{}
	for _, it := range env.Data {
		gotMasks[it.GroupID] = it.Mask
	}
	if gotMasks[gA] != "5" {
		t.Fatalf("gA mask: want 5, got %q", gotMasks[gA])
	}
	if gotMasks[gB] != "2" {
		t.Fatalf("gB mask: want 2, got %q", gotMasks[gB])
	}

	// Pagination: ordered by group_id ASC.
	resPage, err := http.Get(base + "?offset=1&limit=1")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resPage.Body.Close() }()
	if resPage.StatusCode != http.StatusOK {
		t.Fatalf("page status %d", resPage.StatusCode)
	}
	var page listResponse[struct {
		GroupID string `json:"group_id"`
		Mask    string `json:"mask"`
	}]
	if err := json.NewDecoder(resPage.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	if page.Meta.Total != 2 || len(page.Data) != 1 {
		t.Fatalf("page: total=%d len=%d", page.Meta.Total, len(page.Data))
	}
	orderedIDs := []string{gA, gB}
	sort.Strings(orderedIDs)
	if page.Data[0].GroupID != orderedIDs[1] {
		t.Fatalf("page group: want %s, got %s", orderedIDs[1], page.Data[0].GroupID)
	}
	if page.Meta.Sort != "group_id" || page.Meta.Order != "asc" {
		t.Fatalf("page meta sort/order: got sort=%q order=%q", page.Meta.Sort, page.Meta.Order)
	}
}

func TestAPI_resourceAuthzGroups_unsupportedQueryParams(t *testing.T) {
	ts, st := newTestAPI(t)
	ctx := context.Background()
	domainID := uuid.NewString()
	rid := uuid.NewString()
	if err := st.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	if err := st.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}
	res, err := http.Get(ts.URL + "/api/v1/domains/" + domainID + "/resources/" + rid + "/authz/groups?search=foo")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("unsupported params: want 400, got %d: %s", res.StatusCode, b)
	}
	var out map[string]string
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out["error"] != "only limit and offset are supported" {
		t.Fatalf("unexpected error message: %q", out["error"])
	}
}

func TestAPI_resourceAuthzGroups_notFound(t *testing.T) {
	ts, st := newTestAPI(t)
	ctx := context.Background()
	domainID := uuid.NewString()
	if err := st.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	rid := uuid.NewString()
	if err := st.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}

	resUnknownDomain, err := http.Get(ts.URL + "/api/v1/domains/" + uuid.NewString() + "/resources/" + rid + "/authz/groups")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resUnknownDomain.Body.Close() }()
	if resUnknownDomain.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown domain: want 404, got %d", resUnknownDomain.StatusCode)
	}

	resUnknownResource, err := http.Get(ts.URL + "/api/v1/domains/" + domainID + "/resources/" + uuid.NewString() + "/authz/groups")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resUnknownResource.Body.Close() }()
	if resUnknownResource.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown resource: want 404, got %d", resUnknownResource.StatusCode)
	}
}

func TestAPI_health_publicWhenAPIUsesBearer(t *testing.T) {
	db, err := sqlstore.Open("file:" + filepath.Join(t.TempDir(), "api.db") + "?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlstore.MigrateUp(db, testutil.SQLiteMigrationsDir(t)); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	st := sqlstore.New(db)
	srv := &Server{Store: st, APIBearerToken: "secret-token"}
	ts := httptest.NewServer(srv.Router(nil, nil))
	t.Cleanup(ts.Close)

	res, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("health should stay public, got %d", res.StatusCode)
	}
}

func TestAPI_membershipPostDelete_notFound(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	base := ts.URL + "/api/v1/domains/" + dom.ID

	var user store.User
	if err := json.Unmarshal(mustPostJSON201(t, base+"/users", `{"title":"u"}`), &user); err != nil {
		t.Fatal(err)
	}
	var g store.Group
	if err := json.Unmarshal(mustPostJSON201(t, base+"/groups", `{"title":"g"}`), &g); err != nil {
		t.Fatal(err)
	}

	addURL := base + "/users/" + user.ID + "/groups/" + g.ID
	reqPost, err := http.NewRequest(http.MethodPost, addURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resPost, err := http.DefaultClient.Do(reqPost)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resPost.Body.Close() }()
	if resPost.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resPost.Body)
		t.Fatalf("POST membership want 204, got %d: %s", resPost.StatusCode, b)
	}

	reqDel, err := http.NewRequest(http.MethodDelete, addURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resDel, err := http.DefaultClient.Do(reqDel)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resDel.Body.Close() }()
	if resDel.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resDel.Body)
		t.Fatalf("DELETE membership want 204, got %d: %s", resDel.StatusCode, b)
	}

	reqDel2, err := http.NewRequest(http.MethodDelete, addURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resDel2, err := http.DefaultClient.Do(reqDel2)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resDel2.Body.Close() }()
	if resDel2.StatusCode != http.StatusNotFound {
		t.Fatalf("second DELETE want 404, got %d", resDel2.StatusCode)
	}
}

func TestAPI_addUserToGroup_unknownUser(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	base := ts.URL + "/api/v1/domains/" + dom.ID
	var g store.Group
	if err := json.Unmarshal(mustPostJSON201(t, base+"/groups", `{"title":"g"}`), &g); err != nil {
		t.Fatal(err)
	}
	addURL := base + "/users/" + uuid.NewString() + "/groups/" + g.ID
	reqPost, err := http.NewRequest(http.MethodPost, addURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(reqPost)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("want 400, got %d: %s", res.StatusCode, b)
	}
}

func TestAPI_userPermissionGrantRevoke_notFound(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	base := ts.URL + "/api/v1/domains/" + dom.ID

	var user store.User
	if err := json.Unmarshal(mustPostJSON201(t, base+"/users", `{"title":"u"}`), &user); err != nil {
		t.Fatal(err)
	}
	var resource store.Resource
	if err := json.Unmarshal(mustPostJSON201(t, base+"/resources", `{"title":"r"}`), &resource); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"title":"p","resource_id":%q,"access_mask":"0x1"}`, resource.ID)
	var perm store.Permission
	if err := json.Unmarshal(mustPostJSON201(t, base+"/permissions", body), &perm); err != nil {
		t.Fatal(err)
	}

	grantURL := base + "/users/" + user.ID + "/permissions/" + perm.ID
	reqPost, err := http.NewRequest(http.MethodPost, grantURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resPost, err := http.DefaultClient.Do(reqPost)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resPost.Body.Close() }()
	if resPost.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resPost.Body)
		t.Fatalf("POST grant want 204, got %d: %s", resPost.StatusCode, b)
	}

	reqDel, err := http.NewRequest(http.MethodDelete, grantURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resDel, err := http.DefaultClient.Do(reqDel)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resDel.Body.Close() }()
	if resDel.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resDel.Body)
		t.Fatalf("DELETE revoke want 204, got %d: %s", resDel.StatusCode, b)
	}

	reqDel2, err := http.NewRequest(http.MethodDelete, grantURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resDel2, err := http.DefaultClient.Do(reqDel2)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resDel2.Body.Close() }()
	if resDel2.StatusCode != http.StatusNotFound {
		t.Fatalf("second DELETE want 404, got %d", resDel2.StatusCode)
	}
}

func TestAPI_grantUserPermission_unknownPermission(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	base := ts.URL + "/api/v1/domains/" + dom.ID
	var user store.User
	if err := json.Unmarshal(mustPostJSON201(t, base+"/users", `{"title":"u"}`), &user); err != nil {
		t.Fatal(err)
	}
	grantURL := base + "/users/" + user.ID + "/permissions/" + uuid.NewString()
	reqPost, err := http.NewRequest(http.MethodPost, grantURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(reqPost)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("want 400, got %d: %s", res.StatusCode, b)
	}
}

func TestAPI_groupPermissionGrantRevoke_notFound(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	base := ts.URL + "/api/v1/domains/" + dom.ID

	var g store.Group
	if err := json.Unmarshal(mustPostJSON201(t, base+"/groups", `{"title":"g"}`), &g); err != nil {
		t.Fatal(err)
	}
	var resource store.Resource
	if err := json.Unmarshal(mustPostJSON201(t, base+"/resources", `{"title":"r"}`), &resource); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"title":"p","resource_id":%q,"access_mask":"0x2"}`, resource.ID)
	var perm store.Permission
	if err := json.Unmarshal(mustPostJSON201(t, base+"/permissions", body), &perm); err != nil {
		t.Fatal(err)
	}

	grantURL := base + "/groups/" + g.ID + "/permissions/" + perm.ID
	reqPost, err := http.NewRequest(http.MethodPost, grantURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resPost, err := http.DefaultClient.Do(reqPost)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resPost.Body.Close() }()
	if resPost.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resPost.Body)
		t.Fatalf("POST group grant want 204, got %d: %s", resPost.StatusCode, b)
	}

	reqDel, err := http.NewRequest(http.MethodDelete, grantURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resDel, err := http.DefaultClient.Do(reqDel)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resDel.Body.Close() }()
	if resDel.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resDel.Body)
		t.Fatalf("DELETE group revoke want 204, got %d: %s", resDel.StatusCode, b)
	}

	reqDel2, err := http.NewRequest(http.MethodDelete, grantURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resDel2, err := http.DefaultClient.Do(reqDel2)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resDel2.Body.Close() }()
	if resDel2.StatusCode != http.StatusNotFound {
		t.Fatalf("second DELETE want 404, got %d", resDel2.StatusCode)
	}
}

func TestAPI_grantGroupPermission_unknownPermission(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	base := ts.URL + "/api/v1/domains/" + dom.ID
	var g store.Group
	if err := json.Unmarshal(mustPostJSON201(t, base+"/groups", `{"title":"g"}`), &g); err != nil {
		t.Fatal(err)
	}
	grantURL := base + "/groups/" + g.ID + "/permissions/" + uuid.NewString()
	reqPost, err := http.NewRequest(http.MethodPost, grantURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(reqPost)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("want 400, got %d: %s", res.StatusCode, b)
	}
}

func TestAPI_addUserToGroup_duplicate(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	base := ts.URL + "/api/v1/domains/" + dom.ID
	var user store.User
	if err := json.Unmarshal(mustPostJSON201(t, base+"/users", `{"title":"u"}`), &user); err != nil {
		t.Fatal(err)
	}
	var group store.Group
	if err := json.Unmarshal(mustPostJSON201(t, base+"/groups", `{"title":"g"}`), &group); err != nil {
		t.Fatal(err)
	}
	addURL := base + "/users/" + user.ID + "/groups/" + group.ID
	req1, err := http.NewRequest(http.MethodPost, addURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	res1, err := http.DefaultClient.Do(req1)
	if err != nil {
		t.Fatal(err)
	}
	_ = res1.Body.Close()
	if res1.StatusCode != http.StatusNoContent {
		t.Fatalf("first add: want 204, got %d", res1.StatusCode)
	}
	req2, err := http.NewRequest(http.MethodPost, addURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	res2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	_ = res2.Body.Close()
	if res2.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate add: want 409, got %d", res2.StatusCode)
	}
}

func TestAPI_grantUserPermission_duplicate(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	base := ts.URL + "/api/v1/domains/" + dom.ID
	var user store.User
	if err := json.Unmarshal(mustPostJSON201(t, base+"/users", `{"title":"u"}`), &user); err != nil {
		t.Fatal(err)
	}
	var resource store.Resource
	if err := json.Unmarshal(mustPostJSON201(t, base+"/resources", `{"title":"r"}`), &resource); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"title":"p","resource_id":%q,"access_mask":"0x1"}`, resource.ID)
	var perm store.Permission
	if err := json.Unmarshal(mustPostJSON201(t, base+"/permissions", body), &perm); err != nil {
		t.Fatal(err)
	}
	grantURL := base + "/users/" + user.ID + "/permissions/" + perm.ID
	req1, err := http.NewRequest(http.MethodPost, grantURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	res1, err := http.DefaultClient.Do(req1)
	if err != nil {
		t.Fatal(err)
	}
	_ = res1.Body.Close()
	if res1.StatusCode != http.StatusNoContent {
		t.Fatalf("first grant: want 204, got %d", res1.StatusCode)
	}
	req2, err := http.NewRequest(http.MethodPost, grantURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	res2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	_ = res2.Body.Close()
	if res2.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate grant: want 409, got %d", res2.StatusCode)
	}
}

func TestAPI_grantGroupPermission_duplicate(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	base := ts.URL + "/api/v1/domains/" + dom.ID
	var group store.Group
	if err := json.Unmarshal(mustPostJSON201(t, base+"/groups", `{"title":"g"}`), &group); err != nil {
		t.Fatal(err)
	}
	var resource store.Resource
	if err := json.Unmarshal(mustPostJSON201(t, base+"/resources", `{"title":"r"}`), &resource); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"title":"p","resource_id":%q,"access_mask":"0x1"}`, resource.ID)
	var perm store.Permission
	if err := json.Unmarshal(mustPostJSON201(t, base+"/permissions", body), &perm); err != nil {
		t.Fatal(err)
	}
	grantURL := base + "/groups/" + group.ID + "/permissions/" + perm.ID
	req1, err := http.NewRequest(http.MethodPost, grantURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	res1, err := http.DefaultClient.Do(req1)
	if err != nil {
		t.Fatal(err)
	}
	_ = res1.Body.Close()
	if res1.StatusCode != http.StatusNoContent {
		t.Fatalf("first grant: want 204, got %d", res1.StatusCode)
	}
	req2, err := http.NewRequest(http.MethodPost, grantURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	res2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	_ = res2.Body.Close()
	if res2.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate grant: want 409, got %d", res2.StatusCode)
	}
}

func TestAPI_authzMasks_happyPath(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	base := ts.URL + "/api/v1/domains/" + dom.ID

	var user store.User
	if err := json.Unmarshal(mustPostJSON201(t, base+"/users", `{"title":"u"}`), &user); err != nil {
		t.Fatal(err)
	}
	var g store.Group
	if err := json.Unmarshal(mustPostJSON201(t, base+"/groups", `{"title":"g"}`), &g); err != nil {
		t.Fatal(err)
	}
	var resource store.Resource
	if err := json.Unmarshal(mustPostJSON201(t, base+"/resources", `{"title":"r"}`), &resource); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"title":"p","resource_id":%q,"access_mask":"0x5"}`, resource.ID)
	var perm store.Permission
	if err := json.Unmarshal(mustPostJSON201(t, base+"/permissions", body), &perm); err != nil {
		t.Fatal(err)
	}

	addURL := base + "/users/" + user.ID + "/groups/" + g.ID
	reqMem, err := http.NewRequest(http.MethodPost, addURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resMem, err := http.DefaultClient.Do(reqMem)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resMem.Body.Close() }()
	if resMem.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resMem.Body)
		t.Fatalf("membership want 204, got %d: %s", resMem.StatusCode, b)
	}

	grantURL := base + "/groups/" + g.ID + "/permissions/" + perm.ID
	reqGr, err := http.NewRequest(http.MethodPost, grantURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resGr, err := http.DefaultClient.Do(reqGr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resGr.Body.Close() }()
	if resGr.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resGr.Body)
		t.Fatalf("group grant want 204, got %d: %s", resGr.StatusCode, b)
	}

	q := fmt.Sprintf("%s/authz/masks?user_id=%s&resource_id=%s", base, user.ID, resource.ID)
	res, err := http.Get(q)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("masks want 200, got %d: %s", res.StatusCode, b)
	}
	var out struct {
		Masks []uint64 `json:"masks"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out.Masks) != 1 || out.Masks[0] != 5 {
		t.Fatalf("masks: %+v", out.Masks)
	}
}

func TestAPI_authzMasks_emptyWithoutGrants(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	base := ts.URL + "/api/v1/domains/" + dom.ID
	var user store.User
	if err := json.Unmarshal(mustPostJSON201(t, base+"/users", `{"title":"u"}`), &user); err != nil {
		t.Fatal(err)
	}
	var resource store.Resource
	if err := json.Unmarshal(mustPostJSON201(t, base+"/resources", `{"title":"r"}`), &resource); err != nil {
		t.Fatal(err)
	}
	q := fmt.Sprintf("%s/authz/masks?user_id=%s&resource_id=%s", base, user.ID, resource.ID)
	res, err := http.Get(q)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("want 200, got %d: %s", res.StatusCode, b)
	}
	var out struct {
		Masks []uint64 `json:"masks"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out.Masks) != 0 {
		t.Fatalf("want empty masks, got %+v", out.Masks)
	}
}

// --- empty-list tests to cover the nil→[] fallback branches ---

// TestAPI_authzCheck_accessBitOutOfRange asserts authzCheck rejects a bit
// value above the signed-63 limit before reaching the store.
func TestAPI_authzCheck_accessBitOutOfRange(t *testing.T) {
	ts, _ := newTestAPI(t)
	dom := seedDomain(t, ts, "test-domain")
	url := fmt.Sprintf("%s/api/v1/domains/%s/authz/check?user_id=u&resource_id=r&access_bit=0x8000000000000000", ts.URL, dom)
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("want 400, got %d: %s", res.StatusCode, b)
	}
	var body map[string]string
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got, want := body["error"], "mask value must be within signed 64-bit range"; got != want {
		t.Fatalf(`body["error"] = %q, want %q`, got, want)
	}
}

// --- T52: request/response error hardening tests ---

// TestWriteJSON_encodeErrorLogged asserts that a response encoding failure is
// logged at ERROR level with method and path so operators can identify the
// failing endpoint. Because the status header is already committed, the only
// observable signal is the log entry.
//
// NOTE: This test mutates the package-level logger via logger.Init.
// t.Parallel() is intentionally omitted until T54 (injectable logger) lands.
// See TODO(T54) in writeJSON.
