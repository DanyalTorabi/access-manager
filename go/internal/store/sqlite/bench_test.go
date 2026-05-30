package sqlite

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dtorabi/access-manager/internal/store"
)

// sqliteMigrationsDir returns the migrations/sqlite directory relative to this
// file. It is the *testing.B equivalent of testutil.SQLiteMigrationsDir (which
// takes *testing.T) and avoids importing testutil in benchmark code.
func sqliteMigrationsDir(b *testing.B) string {
	b.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		b.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations", "sqlite"))
}

func newBenchStore(b *testing.B) *Store {
	b.Helper()
	db, err := Open("file:" + filepath.Join(b.TempDir(), "bench.db") + "?_pragma=foreign_keys(1)")
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = db.Close() })
	if err := MigrateUp(db, sqliteMigrationsDir(b)); err != nil {
		b.Fatal(err)
	}
	return New(db)
}

func seedBenchDomain(b *testing.B, s *Store) string {
	b.Helper()
	ctx := context.Background()
	id := fmt.Sprintf("bench-domain-%s", b.Name())
	if err := s.DomainCreate(ctx, &store.Domain{ID: id, Title: "bench"}); err != nil {
		b.Fatal(err)
	}
	return id
}

func seedBenchUsers(b *testing.B, s *Store, domainID string, n int) []string {
	b.Helper()
	ctx := context.Background()
	ids := make([]string, n)
	for i := 0; i < n; i++ {
		ids[i] = fmt.Sprintf("u-%d", i)
		if err := s.UserCreate(ctx, &store.User{ID: ids[i], DomainID: domainID, Title: fmt.Sprintf("user-%d", i)}); err != nil {
			b.Fatal(err)
		}
	}
	return ids
}

func seedBenchGroups(b *testing.B, s *Store, domainID string, n int) []string {
	b.Helper()
	ctx := context.Background()
	ids := make([]string, n)
	for i := 0; i < n; i++ {
		ids[i] = fmt.Sprintf("g-%d", i)
		if err := s.GroupCreate(ctx, &store.Group{ID: ids[i], DomainID: domainID, Title: fmt.Sprintf("group-%d", i)}); err != nil {
			b.Fatal(err)
		}
	}
	return ids
}

func seedBenchResource(b *testing.B, s *Store, domainID string) string {
	b.Helper()
	ctx := context.Background()
	rid := "bench-resource"
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "bench resource"}); err != nil {
		b.Fatal(err)
	}
	return rid
}

// seedBenchPermissions creates one permission per mask value and returns the
// permission IDs in the same order as masks.
func seedBenchPermissions(b *testing.B, s *Store, domainID, resourceID string, masks []uint64) []string {
	b.Helper()
	ctx := context.Background()
	ids := make([]string, len(masks))
	for i, m := range masks {
		ids[i] = fmt.Sprintf("perm-%d", i)
		if err := s.PermissionCreate(ctx, &store.Permission{
			ID:         ids[i],
			DomainID:   domainID,
			Title:      fmt.Sprintf("perm-%d", i),
			ResourceID: resourceID,
			AccessMask: m,
		}); err != nil {
			b.Fatal(err)
		}
	}
	return ids
}

// BenchmarkEffectiveMask measures EffectiveMask under various grant
// configurations: direct-only vs. group-inherited, at small and larger scale.
func BenchmarkEffectiveMask(b *testing.B) {
	ctx := context.Background()

	cases := []struct {
		name        string
		directUsers int
		groupUsers  int
	}{
		{"Direct10", 10, 0},
		{"Direct100", 100, 0},
		{"GroupInherited10", 0, 10},
		{"GroupInherited100", 0, 100},
		{"Mixed1000", 500, 500},
	}

	for _, tc := range cases {
		tc := tc
		b.Run(tc.name, func(b *testing.B) {
			s := newBenchStore(b)
			domainID := seedBenchDomain(b, s)
			rid := seedBenchResource(b, s, domainID)

			// Target user whose effective mask we'll query.
			targetUID := "target-user"
			if err := s.UserCreate(ctx, &store.User{ID: targetUID, DomainID: domainID, Title: "target"}); err != nil {
				b.Fatal(err)
			}

			totalPerms := tc.directUsers + tc.groupUsers
			if totalPerms == 0 {
				totalPerms = 1
			}
			masks := make([]uint64, totalPerms)
			for i := range masks {
				masks[i] = uint64(1 << (i % 62))
			}
			permIDs := seedBenchPermissions(b, s, domainID, rid, masks)

			// Direct grants to target user.
			for i := 0; i < tc.directUsers; i++ {
				if err := s.GrantUserPermission(ctx, domainID, targetUID, permIDs[i]); err != nil {
					b.Fatal(err)
				}
			}

			// Group-inherited grants.
			if tc.groupUsers > 0 {
				gid := "bench-group-target"
				if err := s.GroupCreate(ctx, &store.Group{ID: gid, DomainID: domainID, Title: "g"}); err != nil {
					b.Fatal(err)
				}
				if err := s.AddUserToGroup(ctx, domainID, targetUID, gid); err != nil {
					b.Fatal(err)
				}
				for i := tc.directUsers; i < totalPerms; i++ {
					if err := s.GrantGroupPermission(ctx, domainID, gid, permIDs[i]); err != nil {
						b.Fatal(err)
					}
				}
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := s.EffectiveMask(ctx, domainID, targetUID, rid); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkResourceAuthzUsersList measures listing users with effective masks
// on a resource at increasing population sizes with mixed direct+group grants.
func BenchmarkResourceAuthzUsersList(b *testing.B) {
	ctx := context.Background()

	for _, n := range []int{100, 500, 1000} {
		n := n
		b.Run(fmt.Sprintf("Users%d", n), func(b *testing.B) {
			s := newBenchStore(b)
			domainID := seedBenchDomain(b, s)
			rid := seedBenchResource(b, s, domainID)

			// One permission for direct grants, one for group-inherited.
			directPermID := "perm-direct"
			groupPermID := "perm-group"
			if err := s.PermissionCreate(ctx, &store.Permission{ID: directPermID, DomainID: domainID, Title: "direct", ResourceID: rid, AccessMask: 1}); err != nil {
				b.Fatal(err)
			}
			if err := s.PermissionCreate(ctx, &store.Permission{ID: groupPermID, DomainID: domainID, Title: "group", ResourceID: rid, AccessMask: 2}); err != nil {
				b.Fatal(err)
			}

			gid := "g-all"
			if err := s.GroupCreate(ctx, &store.Group{ID: gid, DomainID: domainID, Title: "all"}); err != nil {
				b.Fatal(err)
			}
			if err := s.GrantGroupPermission(ctx, domainID, gid, groupPermID); err != nil {
				b.Fatal(err)
			}

			uids := seedBenchUsers(b, s, domainID, n)
			half := n / 2
			for i, uid := range uids {
				if i < half {
					// Direct grant.
					if err := s.GrantUserPermission(ctx, domainID, uid, directPermID); err != nil {
						b.Fatal(err)
					}
				} else {
					// Group-inherited grant.
					if err := s.AddUserToGroup(ctx, domainID, uid, gid); err != nil {
						b.Fatal(err)
					}
				}
			}

			opts := store.ListOpts{Limit: store.MaxLimit}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, _, err := s.ResourceAuthzUsersList(ctx, domainID, rid, opts); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkUserAuthzResourcesList measures listing resources with effective
// masks for a user at increasing resource counts.
func BenchmarkUserAuthzResourcesList(b *testing.B) {
	ctx := context.Background()

	for _, n := range []int{10, 100} {
		n := n
		b.Run(fmt.Sprintf("Resources%d", n), func(b *testing.B) {
			s := newBenchStore(b)
			domainID := seedBenchDomain(b, s)

			uid := "bench-user"
			if err := s.UserCreate(ctx, &store.User{ID: uid, DomainID: domainID, Title: "u"}); err != nil {
				b.Fatal(err)
			}

			for i := 0; i < n; i++ {
				rid := fmt.Sprintf("r-%d", i)
				if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: rid}); err != nil {
					b.Fatal(err)
				}
				pid := fmt.Sprintf("p-%d", i)
				if err := s.PermissionCreate(ctx, &store.Permission{ID: pid, DomainID: domainID, Title: pid, ResourceID: rid, AccessMask: 1}); err != nil {
					b.Fatal(err)
				}
				if err := s.GrantUserPermission(ctx, domainID, uid, pid); err != nil {
					b.Fatal(err)
				}
			}

			opts := store.ListOpts{Limit: store.MaxLimit}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, _, err := s.UserAuthzResourcesList(ctx, domainID, uid, opts); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkResourceAuthzGroupsList_PageNearParamCap measures the IN-clause
// approach as the number of seeded groups varies, verifying behaviour near
// the SQLite parameter cap. Sub-benchmarks seed 50, 100, and 200 groups
// (> MaxLimit to exercise pagination) but query one page of MaxLimit (100).
func BenchmarkResourceAuthzGroupsList_PageNearParamCap(b *testing.B) {
	ctx := context.Background()

	for _, n := range []int{50, 100, 200} {
		n := n
		b.Run(fmt.Sprintf("Groups%d", n), func(b *testing.B) {
			s := newBenchStore(b)
			domainID := seedBenchDomain(b, s)
			rid := seedBenchResource(b, s, domainID)

			gids := seedBenchGroups(b, s, domainID, n)

			// One permission per group so every group has a non-zero mask.
			for i, gid := range gids {
				pid := fmt.Sprintf("pg-%d", i)
				if err := s.PermissionCreate(ctx, &store.Permission{ID: pid, DomainID: domainID, Title: pid, ResourceID: rid, AccessMask: 1}); err != nil {
					b.Fatal(err)
				}
				if err := s.GrantGroupPermission(ctx, domainID, gid, pid); err != nil {
					b.Fatal(err)
				}
			}

			opts := store.ListOpts{Limit: store.MaxLimit}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, _, err := s.ResourceAuthzGroupsList(ctx, domainID, rid, opts); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkGroupSetParent_DeepChain measures the parent-chain cycle-detection
// walk triggered by GroupSetParent on linear chains of depths 100, 1000, and
// 10000. The chain is seeded once before b.ResetTimer so each iteration only
// measures the GroupSetParent call itself.
//
// Seeding uses a raw transaction to batch-insert chain members quickly;
// depth=10000 may still take ~30 s on a reference machine — run with
// -benchtime=30s and -bench=BenchmarkGroupSetParent_DeepChain when measuring
// this sub-benchmark in isolation (see go/README.md).
func BenchmarkGroupSetParent_DeepChain(b *testing.B) {
	ctx := context.Background()

	for _, depth := range []int{100, 1000, 10000} {
		depth := depth
		b.Run(fmt.Sprintf("Depth%d", depth), func(b *testing.B) {
			s := newBenchStore(b)
			domainID := seedBenchDomain(b, s)

			// Batch-insert the chain inside one transaction to avoid 10 000
			// individual auto-commit round-trips, which would dominate setup time.
			tx, err := s.db.BeginTx(ctx, nil)
			if err != nil {
				b.Fatal(err)
			}
			ids := make([]string, depth)
			for i := 0; i < depth; i++ {
				ids[i] = fmt.Sprintf("chain-g-%d", i)
				var parent any
				if i > 0 {
					parent = ids[i-1]
				}
				if _, err := tx.ExecContext(ctx,
					`INSERT INTO groups (id, domain_id, title, parent_group_id) VALUES (?, ?, ?, ?)`,
					ids[i], domainID, ids[i], parent,
				); err != nil {
					_ = tx.Rollback()
					b.Fatal(err)
				}
			}
			if err := tx.Commit(); err != nil {
				b.Fatal(err)
			}

			// Create a leaf group that will be re-parented to group(depth-1)
			// on every iteration. Re-parenting triggers the full chain walk.
			leafID := "chain-leaf"
			leafParent := ids[depth-1]
			if err := s.GroupCreate(ctx, &store.Group{ID: leafID, DomainID: domainID, Title: "leaf"}); err != nil {
				b.Fatal(err)
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := s.GroupSetParent(ctx, domainID, leafID, &leafParent); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
