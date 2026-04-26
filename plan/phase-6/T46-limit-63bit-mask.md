# T46 — Limit access mask to 63 bits (temporary)

## Ticket

**T46** — Limit access mask to 63 bits (GitHub #67)

## Phase

**Phase 6** — P3 product options

## Goal

Avoid signed-64 overflow when storing effective/access masks in SQLite by
restricting masks to the lower 63 bits until a v2 migration that preserves the
full `uint64` range can be implemented.

This is a short-term compatibility rule and developer/user contract: callers
may only allocate/access bits in positions `0..62` (inclusive). Attempts to
create `access_types` or `permissions` using bit `63` (`1<<63`) should be
rejected with a clear `400` error and documented.

## Deliverables

- Validation at the API / store boundary rejecting bits >= 63
- Unit and integration tests that assert rejection behaviour and document
  the constraint
- CHANGELOG and AGENTS guidance referencing this ticket (PR #66 already
  added Changlog/AGENTS notes)
- A v2 migration plan (follow-up) to store full `uint64` masks (TEXT/BLOB or
  other DB change) with backfill strategy

## Steps

1. Add a short-term validation layer in `internal/api` (permission/access-type
   create/patch parsing) rejecting `bit` or `access_mask` values that would
   set the 63rd unsigned bit (values > `math.MaxInt64`). Return `400`.
2. Add unit+integration tests under `go/internal/*` asserting rejections and
   unchanged behaviour for values <= `math.MaxInt64`.
3. Link this ticket from CHANGELOG, AGENTS.md, and any release notes.
4. Plan v2 migration (separate ticket): decide storage format and migration
   strategy for existing data.

## Files / paths

- `go/internal/store/sqlite/store.go` — add runtime validation or store-side
  rejection helper.
- `go/internal/api/server.go` — return `400` for invalid bit/mask inputs.
- tests: `go/internal/store/sqlite/store_test.go`, `go/internal/api/server_test.go`.

## Acceptance criteria

- Attempting to create an `access_type` or `permission` that would use bit
  `1<<63` is rejected with `400`.
- Tests cover both API and store rejection paths.
- CHANGELOG/AGENTS mention the limitation and link this ticket.

## Related

- PR: #66
- Issue: #67
