// Package e2e holds black-box HTTP journey tests that run against a live
// server (default BASE_URL http://127.0.0.1:8080).
//
// The suite covers full CRUD lifecycles for every entity type, authz
// challenge scenarios, pagination, error paths, and bearer-auth journeys.
//
// Run:
//
//	go test -tags=e2e -race -count=1 ./e2e/...
//
// Or from the repo root:
//
//	make e2e
package e2e
