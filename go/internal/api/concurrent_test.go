// Package api — concurrent_test.go
//
// Concurrency tests launch many goroutines against a real in-process
// httptest.Server to surface races, deadlocks, and metric skew under load.
// All tests are included in `make test` (no build tag) and run under -race.
//
// Every test is guarded with testing.Short() so `go test -short` (fast
// unit-only CI pass) skips them; `make test` / `make test-concurrent` keep
// them enabled.
//
// # runConcurrent helper
//
// Spawned goroutines must never call t.Fatal or t.Error directly — both are
// unsafe from non-test goroutines. Instead, they return an error which the
// helper collects via a buffered channel. The test goroutine reports all errors
// after wg.Wait() and calls t.FailNow if any were found.
//
// Goroutines receive t.Context() so they are cancelled when the test times out
// or is interrupted (requires Go 1.21+; module uses Go 1.25).
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// runConcurrent launches n goroutines, calls fn(ctx, i) in each, and collects
// errors via a buffered channel. After all goroutines finish it reports every
// error via t.Errorf (from the test goroutine) and calls t.FailNow if at least
// one error was collected. Spawned goroutines must return errors instead of
// calling t.Fatal/t.Error directly.
func runConcurrent(t *testing.T, n int, fn func(ctx context.Context, i int) error) {
	t.Helper()
	ctx := t.Context()
	errs := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		j := i
		go func() {
			defer wg.Done()
			if err := fn(ctx, j); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	var failed bool
	for err := range errs {
		t.Errorf("goroutine error: %v", err)
		failed = true
	}
	if failed {
		t.FailNow()
	}
}

// ---------------------------------------------------------------------------
// 1. Read-mostly: concurrent authz/check
// ---------------------------------------------------------------------------

// TestConcurrent_readMostlyAuthzCheck launches 50 goroutines each issuing 20
// GET /authz/check requests against a pre-seeded domain. The intent is to
// expose data races in the read-path handlers, the store, and shared state
// (e.g. the logger singleton) when the -race detector is active.
func TestConcurrent_readMostlyAuthzCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping concurrent read-mostly test in -short mode")
	}
	ts, _ := newTestAPI(t)

	// Seed domain, user, group, resource, access-type, permission, and grant.
	domID := seedDomain(t, ts, "concurrent-read")
	userID := seedUser(t, ts, domID, "reader")
	groupID := seedGroup(t, ts, domID, "readers")
	resID := seedResource(t, ts, domID, "doc")
	_ = seedAccessType(t, ts, domID, "read", "0x1")
	permID := seedPermission(t, ts, domID, "read-doc", resID, "0x1")
	addMembership(t, ts, domID, userID, groupID)
	grantGroupPerm(t, ts, domID, groupID, permID)

	checkURL := domainBase(ts, domID) +
		fmt.Sprintf("/authz/check?user_id=%s&resource_id=%s&access_bit=0x1", userID, resID)

	const goroutines = 50
	const itersEach = 20

	runConcurrent(t, goroutines, func(ctx context.Context, _ int) error {
		for iter := 0; iter < itersEach; iter++ {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, checkURL, nil)
			if err != nil {
				return fmt.Errorf("build request iter %d: %w", iter, err)
			}
			resp, err := testClient.Do(req)
			if err != nil {
				return fmt.Errorf("GET authz/check iter %d: %w", iter, err)
			}
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("GET authz/check iter %d: want 200, got %d", iter, resp.StatusCode)
			}
		}
		return nil
	})
}

// ---------------------------------------------------------------------------
// 2. Write contention: parallel user creation
// ---------------------------------------------------------------------------

// TestConcurrent_writeContention starts 30 goroutines that each create a
// distinctly-titled user in the same domain. It asserts:
//   - All requests succeed (201 Created).
//   - All returned IDs are unique (no UUID collision).
func TestConcurrent_writeContention(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping concurrent write-contention test in -short mode")
	}
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "concurrent-write")

	const n = 30
	ids := make([]string, n)
	var mu sync.Mutex

	runConcurrent(t, n, func(_ context.Context, i int) error {
		title := fmt.Sprintf("concurrent-user-%d", i)
		b, err := doRequestErr(http.MethodPost,
			domainBase(ts, domID)+"/users",
			fmt.Sprintf(`{"title":%q}`, title),
			http.StatusCreated,
		)
		if err != nil {
			return err
		}
		var created struct{ ID string }
		if err := json.Unmarshal(b, &created); err != nil {
			return fmt.Errorf("unmarshal create user %d: %w", i, err)
		}
		if created.ID == "" {
			return fmt.Errorf("create user %d: empty ID in response", i)
		}
		mu.Lock()
		ids[i] = created.ID
		mu.Unlock()
		return nil
	})

	// Verify all returned IDs are unique.
	seen := make(map[string]struct{}, n)
	for i, id := range ids {
		if _, dup := seen[id]; dup {
			t.Errorf("duplicate ID at index %d: %s", i, id)
		}
		seen[id] = struct{}{}
	}
}

// ---------------------------------------------------------------------------
// 3. Mixed read/write: list reads concurrent with group-membership mutations
// ---------------------------------------------------------------------------

// TestConcurrent_mixedReadWrite runs writer goroutines that add then remove
// group memberships while reader goroutines concurrently list users. This
// exercises the handler and store under a mixed workload that is typical of
// production traffic patterns. Asserts no 500s and no race-detector hits.
func TestConcurrent_mixedReadWrite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping concurrent mixed read/write test in -short mode")
	}
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "concurrent-mixed")
	groupID := seedGroup(t, ts, domID, "mixed-group")

	const nWriters = 10
	const nReaders = 10

	// Pre-seed users so writers only need to manage memberships.
	uids := make([]string, nWriters)
	for i := 0; i < nWriters; i++ {
		uids[i] = seedUser(t, ts, domID, fmt.Sprintf("mixed-u-%d", i))
	}

	listURL := domainBase(ts, domID) + "/users"
	base := domainBase(ts, domID)

	var wg sync.WaitGroup
	errs := make(chan error, nWriters+nReaders)
	ctx := t.Context()

	// Writers: add then remove membership for each pre-seeded user.
	for i := 0; i < nWriters; i++ {
		wg.Add(1)
		uid := uids[i]
		go func() {
			defer wg.Done()
			if ctx.Err() != nil {
				return
			}
			if _, err := doRequestErr(http.MethodPost,
				base+"/users/"+uid+"/groups/"+groupID,
				"", http.StatusNoContent); err != nil {
				errs <- fmt.Errorf("add membership %s: %w", uid, err)
				return
			}
			if _, err := doRequestErr(http.MethodDelete,
				base+"/users/"+uid+"/groups/"+groupID,
				"", http.StatusNoContent); err != nil {
				errs <- fmt.Errorf("remove membership %s: %w", uid, err)
			}
		}()
	}

	// Readers: list users 5 times each.
	for i := 0; i < nReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for k := 0; k < 5; k++ {
				if ctx.Err() != nil {
					return
				}
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, listURL, nil)
				if err != nil {
					errs <- fmt.Errorf("build list request: %w", err)
					return
				}
				resp, err := testClient.Do(req)
				if err != nil {
					errs <- fmt.Errorf("list users iter %d: %w", k, err)
					return
				}
				_ = resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					errs <- fmt.Errorf("list users iter %d: want 200, got %d", k, resp.StatusCode)
					return
				}
			}
		}()
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

// ---------------------------------------------------------------------------
// 4. Cycle-detection thrash: concurrent groupSetParent races
// ---------------------------------------------------------------------------

// TestConcurrent_cycleDetectionThrash races 20 goroutines — half setting
// g1.parent=g2 and half setting g2.parent=g1 — to exercise the cycle-
// detection path under write contention. SQLite serialises the transactions,
// so the first-committed direction wins; subsequent calls that would introduce
// a cycle receive 400. A 500 from any goroutine is a test failure.
func TestConcurrent_cycleDetectionThrash(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping concurrent cycle-detection test in -short mode")
	}
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "concurrent-cycle")
	g1 := seedGroup(t, ts, domID, "g1")
	g2 := seedGroup(t, ts, domID, "g2")

	base := domainBase(ts, domID)
	const n = 20

	var wg sync.WaitGroup
	errs := make(chan error, n)
	ctx := t.Context()

	for i := 0; i < n; i++ {
		wg.Add(1)
		j := i
		go func() {
			defer wg.Done()
			if ctx.Err() != nil {
				return
			}
			// Alternate: even → g1.parent=g2, odd → g2.parent=g1.
			var childID, parentID string
			if j%2 == 0 {
				childID, parentID = g1, g2
			} else {
				childID, parentID = g2, g1
			}
			url := fmt.Sprintf("%s/groups/%s/parent", base, childID)
			body := fmt.Sprintf(`{"parent_group_id":%q}`, parentID)
			req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url,
				strings.NewReader(body))
			if err != nil {
				errs <- fmt.Errorf("build patch request: %w", err)
				return
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := testClient.Do(req)
			if err != nil {
				errs <- fmt.Errorf("PATCH groups/%s/parent: %w", childID, err)
				return
			}
			_ = resp.Body.Close()
			// 204 = parent set; 400 = cycle detected — both are valid outcomes.
			// 500 is never acceptable.
			if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusBadRequest {
				errs <- fmt.Errorf("PATCH groups/%s/parent: unexpected status %d", childID, resp.StatusCode)
			}
		}()
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}

	// Sanity: both groups must still be reachable (no group was corrupted).
	mustGet(t, base+"/groups/"+g1, http.StatusOK)
	mustGet(t, base+"/groups/"+g2, http.StatusOK)
}

// ---------------------------------------------------------------------------
// 5. Metrics invariant: authz_checks_total under concurrency
// ---------------------------------------------------------------------------

// TestConcurrent_metricsInvariant verifies the T50 single-increment invariant:
// authz_checks_total{result="ok"} must equal exactly the number of goroutines
// that completed a successful authz/check call. Each goroutine calls the
// endpoint once; after all finish the counter is read from the Prometheus
// registry and compared against the atomic success count.
func TestConcurrent_metricsInvariant(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping concurrent metrics invariant test in -short mode")
	}
	ts, _, reg := newTestAPIWithMetrics(t)

	// Seed domain, user, resource, access-type, permission, direct grant.
	domID := seedDomain(t, ts, "concurrent-metrics")
	userID := seedUser(t, ts, domID, "metrics-user")
	resID := seedResource(t, ts, domID, "metrics-res")
	_ = seedAccessType(t, ts, domID, "read", "0x1")
	permID := seedPermission(t, ts, domID, "metrics-perm", resID, "0x1")
	grantUserPerm(t, ts, domID, userID, permID)

	checkURL := domainBase(ts, domID) +
		fmt.Sprintf("/authz/check?user_id=%s&resource_id=%s&access_bit=0x1", userID, resID)

	const n = 40
	var okCount int64

	runConcurrent(t, n, func(ctx context.Context, _ int) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, checkURL, nil)
		if err != nil {
			return fmt.Errorf("build request: %w", err)
		}
		resp, err := testClient.Do(req)
		if err != nil {
			return fmt.Errorf("authz/check: %w", err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("authz/check: want 200, got %d", resp.StatusCode)
		}
		atomic.AddInt64(&okCount, 1)
		return nil
	})

	wantOK := float64(atomic.LoadInt64(&okCount))
	gotOK := findCounterWithLabels(t, reg, "authz_checks_total",
		map[string]string{"result": authzResultOK})
	if gotOK != wantOK {
		t.Fatalf("authz_checks_total{result=%q}: want %.0f, got %.0f"+
			" (T50 single-increment invariant violated under concurrency)",
			authzResultOK, wantOK, gotOK)
	}

	// Also verify the error counter did not spuriously fire.
	gotErr := findCounterWithLabelsOrZero(t, reg, "authz_checks_total",
		map[string]string{"result": authzResultErr})
	if gotErr != 0 {
		t.Fatalf("authz_checks_total{result=%q}: want 0, got %.0f",
			authzResultErr, gotErr)
	}

	// Sanity: the registry itself must still be gatherable.
	if _, err := reg.Gather(); err != nil {
		t.Fatalf("registry.Gather after concurrent run: %v", err)
	}
}
