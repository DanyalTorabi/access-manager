//go:build e2e

package e2e

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestSmoke_fullJourney mirrors test/e2e/bash/run.sh against BASE_URL.
// It is a minimal happy-path gate; comprehensive journeys live in other files.
func TestSmoke_fullJourney(t *testing.T) {
	base := baseURL()
	c := httpClient()

	var h healthOK
	if err := json.Unmarshal(mustGET(t, c, base+"/health", http.StatusOK), &h); err != nil {
		t.Fatal(err)
	}
	if h.Status != "ok" {
		t.Fatalf("health: %+v", h)
	}

	did := seedDomain(t, c, "e2e-smoke-domain")

	uid := seedUser(t, c, did, "e2e-user")
	gid := seedGroup(t, c, did, "e2e-group")
	rid := seedResource(t, c, did, "e2e-resource")
	pid := seedPermission(t, c, did, "e2e-perm", rid, "0x3")

	addMembership(t, c, did, uid, gid)
	grantGroupPerm(t, c, did, gid, pid)

	assertAuthzCheck(t, c, did, uid, rid, "0x1", true)
}
