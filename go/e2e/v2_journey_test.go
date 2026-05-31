//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

// apiBaseV2 returns the /api/v2 root for the current test server.
func apiBaseV2() string { return baseURL() + "/api/v2" }

func domainBaseV2(domainID string) string {
	return apiBaseV2() + "/domains/" + domainID
}

// v2 response types

type atV2Resp struct {
	ID       string `json:"id"`
	DomainID string `json:"domain_id"`
	Title    string `json:"title"`
	Bit      uint64 `json:"bit"`
}

type permV2Resp struct {
	ID          string   `json:"id"`
	DomainID    string   `json:"domain_id"`
	Title       string   `json:"title"`
	ResourceID  string   `json:"resource_id"`
	Permissions []string `json:"permissions"`
}

type userAuthzResourceV2Resp struct {
	ResourceID  string   `json:"resource_id"`
	Permissions []string `json:"permissions"`
}

type userResourcePermsV2Resp struct {
	Permissions []string `json:"permissions"`
}

// seedAccessTypeV2 creates an access type via /api/v2 (no explicit bit).
func seedAccessTypeV2(t *testing.T, c *http.Client, domainID, title string) atV2Resp {
	t.Helper()
	body := fmt.Sprintf(`{"title":%q}`, title)
	b := mustPostJSON(t, c, domainBaseV2(domainID)+"/access-types", body, http.StatusCreated)
	var at atV2Resp
	if err := json.Unmarshal(b, &at); err != nil {
		t.Fatalf("decode access type v2: %v body=%s", err, b)
	}
	return at
}

// seedPermissionV2 creates a permission via /api/v2 using title array.
func seedPermissionV2(t *testing.T, c *http.Client, domainID, title, resourceID string, perms []string) permV2Resp {
	t.Helper()
	permJSON, _ := json.Marshal(perms)
	body := fmt.Sprintf(`{"title":%q,"resource_id":%q,"permissions":%s}`, title, resourceID, permJSON)
	b := mustPostJSON(t, c, domainBaseV2(domainID)+"/permissions", body, http.StatusCreated)
	var p permV2Resp
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatalf("decode permission v2: %v body=%s", err, b)
	}
	return p
}

// TestV2Journey_TitleBasedPermissions is an end-to-end journey test that:
//  1. Creates a domain (v1).
//  2. Creates access types via /api/v2 (no explicit bit) — verifies auto-allocation.
//  3. Creates a user and resource (v1).
//  4. Creates a permission via /api/v2 with a title array.
//  5. Grants the permission to the user (v1).
//  6. Reads effective permissions via /api/v2 user-authz-resources endpoint.
//  7. Reads effective permissions via the new /api/v2 user+resource endpoint.
//  8. Patches the permission via /api/v2 to add a title — asserts sorted output.
//  9. Verifies /api/v1 GET on the same permission still returns numeric access_mask.
func TestV2Journey_TitleBasedPermissions(t *testing.T) {
	c := httpClient()

	// Step 1: domain
	domID := seedDomain(t, c, "v2-journey-domain")

	// Step 2: access types via v2 (auto-allocated bits)
	readAT := seedAccessTypeV2(t, c, domID, "read")
	writeAT := seedAccessTypeV2(t, c, domID, "write")

	if readAT.Bit == 0 {
		t.Fatalf("read bit should be auto-allocated non-zero, got %d", readAT.Bit)
	}
	if writeAT.Bit == 0 || writeAT.Bit == readAT.Bit {
		t.Fatalf("write bit should differ from read bit, got read=%d write=%d", readAT.Bit, writeAT.Bit)
	}
	t.Logf("auto-allocated bits: read=%d write=%d", readAT.Bit, writeAT.Bit)

	// Verify duplicate title is rejected.
	mustPostJSON(t, c, domainBaseV2(domID)+"/access-types", `{"title":"read"}`, http.StatusConflict)

	// Step 3: user and resource (v1)
	userID := seedUser(t, c, domID, "alice")
	resID := seedResource(t, c, domID, "repo")

	// Step 4: permission via v2 with single title
	perm := seedPermissionV2(t, c, domID, "read-only", resID, []string{"read"})
	if len(perm.Permissions) != 1 || perm.Permissions[0] != "read" {
		t.Fatalf("step 4: want permissions=[read], got %v", perm.Permissions)
	}

	// Step 5: grant to user (v1 membership endpoint)
	grantUserPerm(t, c, domID, userID, perm.ID)

	// Step 6: v2 authz resources listing
	b := mustGET(t, c, domainBaseV2(domID)+"/users/"+userID+"/authz/resources", http.StatusOK)
	var env listEnvelope
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatalf("step 6 decode list: %v", err)
	}
	var authzRows []userAuthzResourceV2Resp
	if err := json.Unmarshal(env.Data, &authzRows); err != nil {
		t.Fatalf("step 6 decode data: %v", err)
	}
	if len(authzRows) != 1 {
		t.Fatalf("step 6: want 1 authz resource, got %d", len(authzRows))
	}
	if authzRows[0].ResourceID != resID {
		t.Fatalf("step 6: resource_id want %q got %q", resID, authzRows[0].ResourceID)
	}
	if len(authzRows[0].Permissions) != 1 || authzRows[0].Permissions[0] != "read" {
		t.Fatalf("step 6: permissions want [read], got %v", authzRows[0].Permissions)
	}

	// Step 7: new per-user/resource permissions endpoint
	b = mustGET(t, c, domainBaseV2(domID)+"/users/"+userID+"/resources/"+resID+"/permissions", http.StatusOK)
	var effPerms userResourcePermsV2Resp
	if err := json.Unmarshal(b, &effPerms); err != nil {
		t.Fatalf("step 7 decode: %v", err)
	}
	if len(effPerms.Permissions) != 1 || effPerms.Permissions[0] != "read" {
		t.Fatalf("step 7: effective permissions want [read], got %v", effPerms.Permissions)
	}

	// Step 8: patch via v2 to add "write" — response must be sorted
	patchBody := `{"permissions":["write","read"]}` // intentionally unsorted in request
	b = mustPATCH(t, c, domainBaseV2(domID)+"/permissions/"+perm.ID, patchBody, http.StatusOK)
	var patched permV2Resp
	if err := json.Unmarshal(b, &patched); err != nil {
		t.Fatalf("step 8 decode: %v", err)
	}
	if len(patched.Permissions) != 2 {
		t.Fatalf("step 8: want 2 permissions, got %d: %v", len(patched.Permissions), patched.Permissions)
	}
	if patched.Permissions[0] != "read" || patched.Permissions[1] != "write" {
		t.Fatalf("step 8: want sorted [read write], got %v", patched.Permissions)
	}

	// Step 9: v1 backward compatibility — same permission via v1 returns numeric mask
	b = mustGET(t, c, domainBase(domID)+"/permissions/"+perm.ID, http.StatusOK)
	var v1perm struct {
		AccessMask uint64 `json:"AccessMask"`
	}
	if err := json.Unmarshal(b, &v1perm); err != nil {
		t.Fatalf("step 9 decode: %v", err)
	}
	expectedMask := readAT.Bit | writeAT.Bit
	if v1perm.AccessMask != expectedMask {
		t.Fatalf("step 9: v1 access_mask want %d (0x%x), got %d (0x%x)",
			expectedMask, expectedMask, v1perm.AccessMask, v1perm.AccessMask)
	}
}

// TestV2Journey_AccessTypeExhaustion verifies that creating 63 access types
// exhausts all bits and the 64th POST returns 409.
func TestV2Journey_AccessTypeExhaustion(t *testing.T) {
	c := httpClient()
	domID := seedDomain(t, c, "v2-exhaustion-domain")

	for i := 0; i < 63; i++ {
		seedAccessTypeV2(t, c, domID, fmt.Sprintf("type-%02d", i))
	}
	mustPostJSON(t, c, domainBaseV2(domID)+"/access-types", `{"title":"overflow"}`, http.StatusConflict)
}
