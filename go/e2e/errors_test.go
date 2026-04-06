//go:build e2e

package e2e

import (
	"net/http"
	"testing"
)

// ---------------------------------------------------------------------------
// Error path journeys
// ---------------------------------------------------------------------------

func TestError_notFoundForBogusID(t *testing.T) {
	c := httpClient()
	did := seedDomain(t, c, "err-notfound-dom")
	base := domainBase(did)

	bogus := "00000000-0000-0000-0000-000000000000"

	t.Run("get_user", func(t *testing.T) {
		mustGET(t, c, base+"/users/"+bogus, http.StatusNotFound)
	})
	t.Run("patch_user", func(t *testing.T) {
		mustPATCH(t, c, base+"/users/"+bogus, `{"title":"x"}`, http.StatusNotFound)
	})
	t.Run("delete_user", func(t *testing.T) {
		mustDELETE(t, c, base+"/users/"+bogus, http.StatusNotFound)
	})
	t.Run("get_group", func(t *testing.T) {
		mustGET(t, c, base+"/groups/"+bogus, http.StatusNotFound)
	})
	t.Run("get_resource", func(t *testing.T) {
		mustGET(t, c, base+"/resources/"+bogus, http.StatusNotFound)
	})
	t.Run("get_permission", func(t *testing.T) {
		mustGET(t, c, base+"/permissions/"+bogus, http.StatusNotFound)
	})
	t.Run("get_access_type", func(t *testing.T) {
		mustGET(t, c, base+"/access-types/"+bogus, http.StatusNotFound)
	})
	t.Run("get_domain", func(t *testing.T) {
		mustGET(t, c, apiBase()+"/domains/"+bogus, http.StatusNotFound)
	})

	mustDELETE(t, c, apiBase()+"/domains/"+did, http.StatusNoContent)
}

func TestError_invalidJSON(t *testing.T) {
	c := httpClient()

	t.Run("malformed_body", func(t *testing.T) {
		mustDo(t, c, http.MethodPost, apiBase()+"/domains", `{bad json}`, http.StatusBadRequest)
	})
	t.Run("unknown_field", func(t *testing.T) {
		mustDo(t, c, http.MethodPost, apiBase()+"/domains", `{"title":"x","unknown":1}`, http.StatusBadRequest)
	})
}

func TestError_permissionMissingAccessMask(t *testing.T) {
	c := httpClient()
	did := seedDomain(t, c, "err-perm-mask-dom")
	rid := seedResource(t, c, did, "r")

	// access_mask is empty string -> parseUint64 fails -> 400
	body := `{"title":"p","resource_id":"` + rid + `","access_mask":""}`
	mustDo(t, c, http.MethodPost, domainBase(did)+"/permissions", body, http.StatusBadRequest)
}

func TestError_duplicateAccessTypeBit(t *testing.T) {
	c := httpClient()
	did := seedDomain(t, c, "err-dup-at-dom")

	seedAccessType(t, c, did, "read", "0x1")

	// Same bit in same domain -> 409
	mustDo(t, c, http.MethodPost, domainBase(did)+"/access-types",
		`{"title":"write","bit":"0x1"}`, http.StatusConflict)
}

func TestError_deleteWithDependents(t *testing.T) {
	c := httpClient()

	t.Run("domain_with_users", func(t *testing.T) {
		did := seedDomain(t, c, "err-del-dom")
		seedUser(t, c, did, "u")
		mustDELETE(t, c, apiBase()+"/domains/"+did, http.StatusBadRequest)
	})

	t.Run("resource_with_permissions", func(t *testing.T) {
		did := seedDomain(t, c, "err-del-res-dom")
		rid := seedResource(t, c, did, "r")
		seedPermission(t, c, did, "p", rid, "0x1")
		mustDELETE(t, c, domainBase(did)+"/resources/"+rid, http.StatusBadRequest)
	})

	t.Run("group_with_members", func(t *testing.T) {
		did := seedDomain(t, c, "err-del-grp-dom")
		gid := seedGroup(t, c, did, "g")
		uid := seedUser(t, c, did, "u")
		addMembership(t, c, did, uid, gid)
		mustDELETE(t, c, domainBase(did)+"/groups/"+gid, http.StatusBadRequest)
	})

	t.Run("user_with_membership", func(t *testing.T) {
		did := seedDomain(t, c, "err-del-usr-dom")
		gid := seedGroup(t, c, did, "g")
		uid := seedUser(t, c, did, "u")
		addMembership(t, c, did, uid, gid)
		mustDELETE(t, c, domainBase(did)+"/users/"+uid, http.StatusBadRequest)
	})

	t.Run("permission_with_user_grant", func(t *testing.T) {
		did := seedDomain(t, c, "err-del-perm-ug")
		rid := seedResource(t, c, did, "r")
		pid := seedPermission(t, c, did, "p", rid, "0x1")
		uid := seedUser(t, c, did, "u")
		grantUserPerm(t, c, did, uid, pid)
		mustDELETE(t, c, domainBase(did)+"/permissions/"+pid, http.StatusBadRequest)
	})

	t.Run("permission_with_group_grant", func(t *testing.T) {
		did := seedDomain(t, c, "err-del-perm-gg")
		rid := seedResource(t, c, did, "r")
		pid := seedPermission(t, c, did, "p", rid, "0x1")
		gid := seedGroup(t, c, did, "g")
		grantGroupPerm(t, c, did, gid, pid)
		mustDELETE(t, c, domainBase(did)+"/permissions/"+pid, http.StatusBadRequest)
	})
}

func TestError_duplicateMembership(t *testing.T) {
	c := httpClient()
	did := seedDomain(t, c, "err-dup-mem-dom")
	uid := seedUser(t, c, did, "u")
	gid := seedGroup(t, c, did, "g")
	addMembership(t, c, did, uid, gid)

	mustDo(t, c, http.MethodPost, domainBase(did)+"/users/"+uid+"/groups/"+gid, "", http.StatusConflict)
}

func TestError_invalidPagination(t *testing.T) {
	c := httpClient()
	did := seedDomain(t, c, "err-pag-dom")
	base := domainBase(did)

	t.Run("negative_limit_clamped", func(t *testing.T) {
		// The API clamps negative limit to DefaultLimit rather than rejecting.
		env := mustList(t, c, base+"/users?limit=-1")
		if env.Meta.Limit <= 0 {
			t.Fatalf("negative limit should be clamped to positive, got %d", env.Meta.Limit)
		}
	})
	t.Run("non_integer_offset", func(t *testing.T) {
		mustDo(t, c, http.MethodGet, base+"/users?offset=abc", "", http.StatusBadRequest)
	})
	t.Run("non_integer_limit", func(t *testing.T) {
		mustDo(t, c, http.MethodGet, base+"/users?limit=xyz", "", http.StatusBadRequest)
	})
}
