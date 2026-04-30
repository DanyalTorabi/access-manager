# T65 — Split long files in `internal/api` and `internal/store/sqlite`

## Ticket

**T65** — Split long files into smaller, topic-focused files (GitHub [#101](https://github.com/DanyalTorabi/access-manager/issues/101))

## Phase

**Phase 8** — Maintainability

## Goal

Reduce the four largest Go files in the codebase to manageable sizes by splitting them by **topic** (one file per resource / per concern), without changing public API or test behaviour. Pure refactor — no logic changes.

## Current sizes (top 4)

| File | Lines |
|------|-------|
| `go/internal/api/server_test.go` | **4948** |
| `go/internal/store/sqlite/store_test.go` | **4527** |
| `go/internal/store/sqlite/store.go` | **1475** |
| `go/internal/api/server.go` | **1370** |

## Deliverables

- `go/internal/api/server.go` split into per-resource files, e.g.:
  - `server.go` — `Server` struct, constructors, helpers (`writeJSON`, `writeErr`, `writeList`), middleware wiring, route registration entrypoint.
  - `server_domains.go`, `server_users.go`, `server_groups.go`, `server_resources.go`, `server_access_types.go`, `server_permissions.go`, `server_authz.go` — handler funcs grouped by entity.
- `go/internal/store/sqlite/store.go` split similarly:
  - `store.go` — `Store` type, `Open`, shared helpers (`scanRow`, `inPlaceholders`, mask-aggregation helper if extracted).
  - `store_domains.go`, `store_users.go`, `store_groups.go`, `store_resources.go`, `store_access_types.go`, `store_permissions.go`, `store_authz.go`.
- The two test files split to **mirror** the production split: each `*_<topic>.go` has a corresponding `*_<topic>_test.go`. This is the standard Go convention and makes it obvious where a new test for a feature should live.

## Non-goals

- No public API change. `internal/api.Server` and `internal/store/sqlite.Store` keep the exact same exported surface.
- No behaviour change. `git diff --stat` should show large *moves* but no edits.
- No reformatting / no comment cleanup beyond what `gofmt` does.
- No new helpers / no extracting things that aren't already there. (Helpers like a shared mask-aggregation function are listed in T5 (#16) deferred follow-ups; do not pull them in here.)

## Steps

1. **Plan the topic groups first** by skimming the existing files and listing every top-level declaration with its target file. Capture the mapping in a comment in the PR description so reviewers can follow.
2. **Move declarations** in small commits:
   - One commit per resource group (e.g. "split: move user handlers to server_users.go").
   - Each commit must keep `make test` and `make lint` green.
3. **Tests**: move tests in matching commits, immediately after their production counterpart, so each commit leaves the package self-contained.
4. **Imports**: rely on `goimports` to keep imports tidy. Avoid `// nolint` additions.
5. **Verify with diff tooling**:
   - `gofmt -d` on every changed file → empty.
   - `git log -p --since=...` should show predominantly `move` operations (use `git diff -M` or `git log --follow` to confirm rename detection).
6. **Cover** with the existing `make cover` target; coverage should be unchanged (±0.1%).

## Acceptance criteria

- After the split, **no single file exceeds 500 lines** in the affected packages (target; small overruns OK if a file is logically a single block).
- `make test`, `make lint`, `make cover` all pass with no test changes other than file location.
- `go test -race ./internal/api/... ./internal/store/sqlite/...` passes.
- Public `go doc ./internal/api` and `go doc ./internal/store/sqlite` outputs are byte-identical (or only differ in declaration ordering, never content).

## Files / paths

- **Move/split** files in `go/internal/api/` and `go/internal/store/sqlite/` only.

## Risk / mitigation

- **Reviewer fatigue**: ~9000 lines of moves are hard to review. Mitigation: one PR per package (api / sqlite) and one commit per topic within. Optionally land production-code split first, then test-file split, so each PR is half the size.
- **Merge conflicts with parallel work**: schedule this when `internal/api` and `internal/store/sqlite` have no other open PRs, or coordinate with whoever has them.
- **Hidden coupling**: package-private symbols may rely on file order via `init()`. None today, but verify by running `go vet ./...` and `go test -race ./...` after each commit.

## Out of scope

- Splitting `e2e/*` files.
- Extracting new helpers / refactoring (separate ticket if needed).
- Renaming exported types or methods.

## Dependencies

- Coordinate with **T62** (introduces v2 handlers in `internal/api`) — land T65 first if possible to avoid collision, otherwise rebase T62 onto T65's split.
