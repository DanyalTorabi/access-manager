// Package e2e holds black-box HTTP smoke tests.
//
// Run against a live server (default BASE_URL http://127.0.0.1:8080):
//
//	go test -tags=e2e -race -count=1 ./e2e/...
//
// See ../../test/e2e/bash/run.sh for the same journey using curl+jq.
package e2e
