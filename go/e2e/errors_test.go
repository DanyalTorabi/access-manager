//go:build e2e

package e2e

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Error path journeys
// ---------------------------------------------------------------------------

func TestError_notFoundForBogusID(t *testing.T) {
	c := httpClient()
	did := seedDomain(t, c, "err-notfound-dom")
	cleanupDelete(t, c, apiBase()+"/domains/"+did)
	base := domainBase(did)

	bogus := uuid.New().String()

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
	cleanupDelete(t, c, apiBase()+"/domains/"+did)
	rid := seedResource(t, c, did, "r")
	cleanupDelete(t, c, domainBase(did)+"/resources/"+rid)

	body := `{"title":"p","resource_id":"` + rid + `","access_mask":""}`
	mustDo(t, c, http.MethodPost, domainBase(did)+"/permissions", body, http.StatusBadRequest)
}

func TestError_duplicateAccessTypeBit(t *testing.T) {
	c := httpClient()
	did := seedDomain(t, c, "err-dup-at-dom")
	cleanupDelete(t, c, apiBase()+"/domains/"+did)

	atID := seedAccessType(t, c, did, "read", "0x1")
	cleanupDelete(t, c, domainBase(did)+"/access-types/"+atID)

	mustDo(t, c, http.MethodPost, domainBase(did)+"/access-types",
		`{"title":"write","bit":"0x1"}`, http.StatusConflict)
}

func TestError_deleteWithDependents(t *testing.T) {
	c := httpClient()

	t.Run("domain_with_users", func(t *testing.T) {
		did := seedDomain(t, c, "err-del-dom")
		cleanupDelete(t, c, apiBase()+"/domains/"+did)
		uid := seedUser(t, c, did, "u")
		cleanupDelete(t, c, domainBase(did)+"/users/"+uid)

		mustDELETE(t, c, apiBase()+"/domains/"+did, http.StatusBadRequest)
	})

	t.Run("resource_with_permissions", func(t *testing.T) {
		did := seedDomain(t, c, "err-del-res-dom")
		cleanupDelete(t, c, apiBase()+"/domains/"+did)
		rid := seedResource(t, c, did, "r")
		cleanupDelete(t, c, domainBase(did)+"/resources/"+rid)
		pid := seedPermission(t, c, did, "p", rid, "0x1")
		cleanupDelete(t, c, domainBase(did)+"/permissions/"+pid)

		mustDELETE(t, c, domainBase(did)+"/resources/"+rid, http.StatusBadRequest)
	})

	t.Run("group_with_members", func(t *testing.T) {
		did := seedDomain(t, c, "err-del-grp-dom")
		cleanupDelete(t, c, apiBase()+"/domains/"+did)
		gid := seedGroup(t, c, did, "g")
		cleanupDelete(t, c, domainBase(did)+"/groups/"+gid)
		uid := seedUser(t, c, did, "u")
		cleanupDelete(t, c, domainBase(did)+"/users/"+uid)
		addMembership(t, c, did, uid, gid)
		cleanupRevokeMembership(t, c, did, uid, gid)

		mustDELETE(t, c, domainBase(did)+"/groups/"+gid, http.StatusBadRequest)
	})

	t.Run("user_with_membership", func(t *testing.T) {
		did := seedDomain(t, c, "err-del-usr-dom")
		cleanupDelete(t, c, apiBase()+"/domains/"+did)
		gid := seedGroup(t, c, did, "g")
		cleanupDelete(t, c, domainBase(did)+"/groups/"+gid)
		uid := seedUser(t, c, did, "u")
		cleanupDelete(t, c, domainBase(did)+"/users/"+uid)
		addMembership(t, c, did, uid, gid)
		cleanupRevokeMembership(t, c, did, uid, gid)

		mustDELETE(t, c, domainBase(did)+"/users/"+uid, http.StatusBadRequest)
	})

	t.Run("permission_with_user_grant", func(t *testing.T) {
		did := seedDomain(t, c, "err-del-perm-ug")
		cleanupDelete(t, c, apiBase()+"/domains/"+did)
		rid := seedResource(t, c, did, "r")
		cleanupDelete(t, c, domainBase(did)+"/resources/"+rid)
		pid := seedPermission(t, c, did, "p", rid, "0x1")
		cleanupDelete(t, c, domainBase(did)+"/permissions/"+pid)
		uid := seedUser(t, c, did, "u")
		cleanupDelete(t, c, domainBase(did)+"/users/"+uid)
		grantUserPerm(t, c, did, uid, pid)
		cleanupRevokeUserPerm(t, c, did, uid, pid)

		mustDELETE(t, c, domainBase(did)+"/permissions/"+pid, http.StatusBadRequest)
	})

	t.Run("permission_with_group_grant", func(t *testing.T) {
		did := seedDomain(t, c, "err-del-perm-gg")
		cleanupDelete(t, c, apiBase()+"/domains/"+did)
		rid := seedResource(t, c, did, "r")
		cleanupDelete(t, c, domainBase(did)+"/resources/"+rid)
		pid := seedPermission(t, c, did, "p", rid, "0x1")
		cleanupDelete(t, c, domainBase(did)+"/permissions/"+pid)
		gid := seedGroup(t, c, did, "g")
		cleanupDelete(t, c, domainBase(did)+"/groups/"+gid)
		grantGroupPerm(t, c, did, gid, pid)
		cleanupRevokeGroupPerm(t, c, did, gid, pid)

		mustDELETE(t, c, domainBase(did)+"/permissions/"+pid, http.StatusBadRequest)
	})
}

func TestError_duplicateMembership(t *testing.T) {
	c := httpClient()
	did := seedDomain(t, c, "err-dup-mem-dom")
	cleanupDelete(t, c, apiBase()+"/domains/"+did)
	uid := seedUser(t, c, did, "u")
	cleanupDelete(t, c, domainBase(did)+"/users/"+uid)
	gid := seedGroup(t, c, did, "g")
	cleanupDelete(t, c, domainBase(did)+"/groups/"+gid)
	addMembership(t, c, did, uid, gid)
	cleanupRevokeMembership(t, c, did, uid, gid)

	mustDo(t, c, http.MethodPost, domainBase(did)+"/users/"+uid+"/groups/"+gid, "", http.StatusConflict)
}

func TestError_badPagination(t *testing.T) {
	c := httpClient()
	did := seedDomain(t, c, "err-pag-dom")
	cleanupDelete(t, c, apiBase()+"/domains/"+did)
	base := domainBase(did)

	t.Run("non_integer_offset", func(t *testing.T) {
		mustDo(t, c, http.MethodGet, base+"/users?offset=abc", "", http.StatusBadRequest)
	})
	t.Run("non_integer_limit", func(t *testing.T) {
		mustDo(t, c, http.MethodGet, base+"/users?limit=xyz", "", http.StatusBadRequest)
	})
	t.Run("negative_offset", func(t *testing.T) {
		mustDo(t, c, http.MethodGet, base+"/users?offset=-1", "", http.StatusBadRequest)
	})
}
