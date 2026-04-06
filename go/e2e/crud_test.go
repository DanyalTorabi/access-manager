//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

// ---------------------------------------------------------------------------
// Full CRUD lifecycle: create -> list -> get -> patch -> get -> delete -> 404
// ---------------------------------------------------------------------------

func TestCRUD_domains(t *testing.T) {
	c := httpClient()

	id := seedDomain(t, c, "crud-domain")

	// List — should contain the new domain.
	env := mustList(t, c, apiBase()+"/domains")
	if env.Meta.Total < 1 {
		t.Fatalf("domain list total < 1: %d", env.Meta.Total)
	}

	// Get
	var got entityTitle
	if err := json.Unmarshal(mustGET(t, c, apiBase()+"/domains/"+id, http.StatusOK), &got); err != nil {
		t.Fatal(err)
	}
	if got.Title != "crud-domain" {
		t.Fatalf("get title: want crud-domain, got %q", got.Title)
	}

	// Patch
	var patched entityTitle
	if err := json.Unmarshal(mustPATCH(t, c, apiBase()+"/domains/"+id, `{"title":"renamed"}`, http.StatusOK), &patched); err != nil {
		t.Fatal(err)
	}
	if patched.Title != "renamed" {
		t.Fatalf("patch title: want renamed, got %q", patched.Title)
	}

	// Get after patch
	var got2 entityTitle
	if err := json.Unmarshal(mustGET(t, c, apiBase()+"/domains/"+id, http.StatusOK), &got2); err != nil {
		t.Fatal(err)
	}
	if got2.Title != "renamed" {
		t.Fatalf("get after patch: want renamed, got %q", got2.Title)
	}

	// Delete
	mustDELETE(t, c, apiBase()+"/domains/"+id, http.StatusNoContent)

	// Verify 404
	mustGET(t, c, apiBase()+"/domains/"+id, http.StatusNotFound)
}

func TestCRUD_users(t *testing.T) {
	c := httpClient()
	did := seedDomain(t, c, "crud-users-dom")
	base := domainBase(did)

	uid := seedUser(t, c, did, "crud-user")

	// List
	env := mustList(t, c, base+"/users")
	if env.Meta.Total != 1 {
		t.Fatalf("user list total: want 1, got %d", env.Meta.Total)
	}

	// Get
	var got entityTitle
	if err := json.Unmarshal(mustGET(t, c, base+"/users/"+uid, http.StatusOK), &got); err != nil {
		t.Fatal(err)
	}
	if got.Title != "crud-user" {
		t.Fatalf("get title: want crud-user, got %q", got.Title)
	}

	// Patch
	var patched entityTitle
	if err := json.Unmarshal(mustPATCH(t, c, base+"/users/"+uid, `{"title":"patched-user"}`, http.StatusOK), &patched); err != nil {
		t.Fatal(err)
	}
	if patched.Title != "patched-user" {
		t.Fatalf("patch title: want patched-user, got %q", patched.Title)
	}

	// Get after patch
	var got2 entityTitle
	if err := json.Unmarshal(mustGET(t, c, base+"/users/"+uid, http.StatusOK), &got2); err != nil {
		t.Fatal(err)
	}
	if got2.Title != "patched-user" {
		t.Fatalf("get after patch: want patched-user, got %q", got2.Title)
	}

	// Delete
	mustDELETE(t, c, base+"/users/"+uid, http.StatusNoContent)

	// Verify 404
	mustGET(t, c, base+"/users/"+uid, http.StatusNotFound)

	// Cleanup domain
	mustDELETE(t, c, apiBase()+"/domains/"+did, http.StatusNoContent)
}

func TestCRUD_groups(t *testing.T) {
	c := httpClient()
	did := seedDomain(t, c, "crud-groups-dom")
	base := domainBase(did)

	parentID := seedGroup(t, c, did, "parent-group")
	childID := seedGroupWithParent(t, c, did, "child-group", parentID)

	// List — both groups present.
	env := mustList(t, c, base+"/groups")
	if env.Meta.Total != 2 {
		t.Fatalf("group list total: want 2, got %d", env.Meta.Total)
	}

	// Get child — verify parent_group_id.
	type groupResp struct {
		ID            string  `json:"ID"`
		Title         string  `json:"Title"`
		ParentGroupID *string `json:"ParentGroupID"`
	}
	var child groupResp
	if err := json.Unmarshal(mustGET(t, c, base+"/groups/"+childID, http.StatusOK), &child); err != nil {
		t.Fatal(err)
	}
	if child.ParentGroupID == nil || *child.ParentGroupID != parentID {
		t.Fatalf("child parent: want %s, got %v", parentID, child.ParentGroupID)
	}

	// Patch title
	var patched entityTitle
	if err := json.Unmarshal(mustPATCH(t, c, base+"/groups/"+parentID, `{"title":"renamed-parent"}`, http.StatusOK), &patched); err != nil {
		t.Fatal(err)
	}
	if patched.Title != "renamed-parent" {
		t.Fatalf("patch title: want renamed-parent, got %q", patched.Title)
	}

	// Set parent to null (unlink child)
	mustPATCH(t, c, base+"/groups/"+childID+"/parent", `{"parent_group_id":null}`, http.StatusNoContent)

	// Verify child has no parent
	var child2 groupResp
	if err := json.Unmarshal(mustGET(t, c, base+"/groups/"+childID, http.StatusOK), &child2); err != nil {
		t.Fatal(err)
	}
	if child2.ParentGroupID != nil {
		t.Fatalf("child parent after unlink: want nil, got %v", child2.ParentGroupID)
	}

	// Delete both
	mustDELETE(t, c, base+"/groups/"+childID, http.StatusNoContent)
	mustDELETE(t, c, base+"/groups/"+parentID, http.StatusNoContent)

	// Verify 404
	mustGET(t, c, base+"/groups/"+childID, http.StatusNotFound)
	mustGET(t, c, base+"/groups/"+parentID, http.StatusNotFound)

	// Cleanup
	mustDELETE(t, c, apiBase()+"/domains/"+did, http.StatusNoContent)
}

func TestCRUD_resources(t *testing.T) {
	c := httpClient()
	did := seedDomain(t, c, "crud-resources-dom")
	base := domainBase(did)

	rid := seedResource(t, c, did, "crud-res")

	env := mustList(t, c, base+"/resources")
	if env.Meta.Total != 1 {
		t.Fatalf("resource list total: want 1, got %d", env.Meta.Total)
	}

	var got entityTitle
	if err := json.Unmarshal(mustGET(t, c, base+"/resources/"+rid, http.StatusOK), &got); err != nil {
		t.Fatal(err)
	}
	if got.Title != "crud-res" {
		t.Fatalf("get title: want crud-res, got %q", got.Title)
	}

	var patched entityTitle
	if err := json.Unmarshal(mustPATCH(t, c, base+"/resources/"+rid, `{"title":"patched-res"}`, http.StatusOK), &patched); err != nil {
		t.Fatal(err)
	}
	if patched.Title != "patched-res" {
		t.Fatalf("patch title: want patched-res, got %q", patched.Title)
	}

	mustDELETE(t, c, base+"/resources/"+rid, http.StatusNoContent)
	mustGET(t, c, base+"/resources/"+rid, http.StatusNotFound)

	mustDELETE(t, c, apiBase()+"/domains/"+did, http.StatusNoContent)
}

func TestCRUD_accessTypes(t *testing.T) {
	c := httpClient()
	did := seedDomain(t, c, "crud-at-dom")
	base := domainBase(did)

	atID := seedAccessType(t, c, did, "read-bit", "0x1")

	env := mustList(t, c, base+"/access-types")
	if env.Meta.Total != 1 {
		t.Fatalf("access-type list total: want 1, got %d", env.Meta.Total)
	}

	type atResp struct {
		ID    string `json:"ID"`
		Title string `json:"Title"`
		Bit   uint64 `json:"Bit"`
	}
	var got atResp
	if err := json.Unmarshal(mustGET(t, c, base+"/access-types/"+atID, http.StatusOK), &got); err != nil {
		t.Fatal(err)
	}
	if got.Title != "read-bit" || got.Bit != 1 {
		t.Fatalf("get: want title=read-bit bit=1, got title=%q bit=%d", got.Title, got.Bit)
	}

	var patched atResp
	if err := json.Unmarshal(mustPATCH(t, c, base+"/access-types/"+atID, `{"title":"write-bit"}`, http.StatusOK), &patched); err != nil {
		t.Fatal(err)
	}
	if patched.Title != "write-bit" {
		t.Fatalf("patch title: want write-bit, got %q", patched.Title)
	}

	mustDELETE(t, c, base+"/access-types/"+atID, http.StatusNoContent)
	mustGET(t, c, base+"/access-types/"+atID, http.StatusNotFound)

	mustDELETE(t, c, apiBase()+"/domains/"+did, http.StatusNoContent)
}

func TestCRUD_permissions(t *testing.T) {
	c := httpClient()
	did := seedDomain(t, c, "crud-perm-dom")
	base := domainBase(did)
	rid := seedResource(t, c, did, "perm-resource")

	pid := seedPermission(t, c, did, "can-read", rid, "0x3")

	env := mustList(t, c, base+"/permissions")
	if env.Meta.Total != 1 {
		t.Fatalf("permission list total: want 1, got %d", env.Meta.Total)
	}

	type permResp struct {
		ID         string `json:"ID"`
		Title      string `json:"Title"`
		ResourceID string `json:"ResourceID"`
		AccessMask uint64 `json:"AccessMask"`
	}
	var got permResp
	if err := json.Unmarshal(mustGET(t, c, base+"/permissions/"+pid, http.StatusOK), &got); err != nil {
		t.Fatal(err)
	}
	if got.Title != "can-read" || got.AccessMask != 3 {
		t.Fatalf("get: want title=can-read mask=3, got title=%q mask=%d", got.Title, got.AccessMask)
	}
	if got.ResourceID != rid {
		t.Fatalf("resource_id: want %s, got %s", rid, got.ResourceID)
	}

	var patched permResp
	body := fmt.Sprintf(`{"title":"can-write","access_mask":"0x7"}`)
	if err := json.Unmarshal(mustPATCH(t, c, base+"/permissions/"+pid, body, http.StatusOK), &patched); err != nil {
		t.Fatal(err)
	}
	if patched.Title != "can-write" || patched.AccessMask != 7 {
		t.Fatalf("patch: want title=can-write mask=7, got title=%q mask=%d", patched.Title, patched.AccessMask)
	}

	mustDELETE(t, c, base+"/permissions/"+pid, http.StatusNoContent)
	mustGET(t, c, base+"/permissions/"+pid, http.StatusNotFound)

	// Cleanup resource + domain
	mustDELETE(t, c, base+"/resources/"+rid, http.StatusNoContent)
	mustDELETE(t, c, apiBase()+"/domains/"+did, http.StatusNoContent)
}
