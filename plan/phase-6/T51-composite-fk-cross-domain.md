# T51 — Tighten composite FKs to prevent cross-domain rows

## Phase

**Phase 6** — P3 product options

## Goal

The current SQLite schema lets `group_permissions(group_id)` reference
`groups(id)` (and `user_permissions(user_id)` reference `users(id)`) without a
composite `(domain_id, id)` foreign key. This means an out-of-band insert (or
a future cross-domain bug in store code) can create a row whose
`group_permissions.domain_id` differs from `groups.domain_id`. The authz list
queries (`UserAuthzResourcesList`, `GroupAuthzResourcesList`,
`ResourceAuthzUsersList`, `ResourceAuthzGroupsList`) defensively re-filter on
`g.domain_id` / `u.domain_id` to keep listings correct, but the invariant
should be enforced at the schema level.

## Deliverables

- New SQLite migration (`go/migrations/sqlite/000003_composite_fk_cross_domain.up.sql`)
  adding composite `FOREIGN KEY (domain_id, group_id) REFERENCES groups
  (id, domain_id)` (and the user / permission equivalents on the three
  junction tables), with a matching `UNIQUE (id, domain_id)` constraint on
  `users`/`groups`/`permissions` so the FK target exists.
  - **Column order rationale.** The UNIQUE leads with `id` (not
    `domain_id`) because SQLite's query planner picks the auto-index on
    `UNIQUE (domain_id, id)` for `WHERE domain_id = ?` predicates inside
    the EXISTS+OR subqueries used by the authz listings, then evaluates
    `ORDER BY u.id ASC LIMIT/OFFSET` on the wrong index and returns rows
    in non-id order. Leading with `id` keeps the existing `idx_*_domain`
    indexes (or the PK) preferred for domain-scoped scans, so listings
    keep their stable id-order.
- Backfill / data-cleanup step that detects existing cross-domain rows and
  either deletes them or refuses to migrate (loud failure with operator
  guidance). Implemented as a temporary `BEFORE INSERT` trigger on a marker
  table that fires `RAISE(ABORT, ...)` so the operator-facing message
  surfaces through the migration runner.
- Equivalent migrations for any future MySQL/PostgreSQL dialects (T01).
  **Deferred** in the initial PR because the MySQL/PostgreSQL drivers are
  not yet wired in; tracked under T01 (per-dialect migrations).
- Once the schema invariant is enforced, the redundant `g.domain_id =
  ?` / `u.domain_id = ?` / `p.domain_id = ?` filters in the authz list
  queries are dropped in lockstep across all four authz listings
  (`UserAuthzResourcesList`, `GroupAuthzResourcesList`,
  `ResourceAuthzUsersList`, `ResourceAuthzGroupsList`) and
  `PermissionMasksForUserResource`. Done in this PR; isolation is now
  enforced solely by the composite FKs.

## Steps

1. Audit current production data (or, in tests, generate an audit query) to
   surface rows where `group_permissions.domain_id != groups.domain_id`.
   The audit query is included verbatim as a comment in the migration
   header so operators can paste-and-run it on demand.
2. Decide on cleanup strategy (delete vs. fail migration). Chose
   "fail loudly via `RAISE(ABORT, ...)` trigger" so operators must
   acknowledge and clean up before re-running.
3. Add migration; verify SQLite raises a foreign-key error on a deliberate
   cross-domain insert.
4. Add a regression test
   (`TestSchema_compositeFKRejectsCrossDomain`) asserting the schema
   rejects cross-domain inserts so any future migration that drops the
   constraint fails loudly. Each junction table's two FKs are exercised
   independently and at least one positive same-domain insert is asserted
   per junction table.
5. Drop the redundant defensive filters in the authz list queries and update
   their long-form comments accordingly. Done in this PR.

## Files / paths

- `go/migrations/sqlite/000003_composite_fk_cross_domain.up.sql` (new migration)
- `go/internal/store/sqlite/store.go` — defensive
  `g.domain_id` / `u.domain_id` / `p.domain_id` filters dropped from the
  authz list queries; comments now reference the schema invariant and the
  per-table `UNIQUE (id, domain_id)` rationale.
- `go/migrations/sqlite/000003_composite_fk_cross_domain.down.sql` —
  operator-run rollback companion (the in-tree migrator only applies
  `.up.sql`). Covered by `TestT51_DownMigration_revertsCompositeFKInvariant`.

## Acceptance criteria

- A direct insert with mismatched `domain_id` fails at the DB layer.
- Authz listings remain correct on a cleaned dataset without any
  defensive Go-side `domain_id` filter; the schema-only invariant is
  asserted by `TestResourceAuthzGroupsList_schemaEnforcesIsolationWithoutGoFilter`
  and the production listings (which no longer carry the defensive
  filter) agree.

## Related

- Surfaced during PR review for T45 (`ResourceAuthzGroupsList`).
