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

- New SQLite migration adding composite `FOREIGN KEY (domain_id, group_id)
  REFERENCES groups (domain_id, id)` (and the user equivalent), with a
  matching `UNIQUE (domain_id, id)` index on `groups`/`users` so the FK
  target exists.
- Backfill / data-cleanup step that detects existing cross-domain rows and
  either deletes them or refuses to migrate (loud failure with operator
  guidance).
- Equivalent migrations for any future MySQL/PostgreSQL dialects (T01).
- Once the schema invariant is enforced, the redundant `g.domain_id =
  ?` / `u.domain_id = ?` filters in the authz list queries can be dropped
  in a follow-up. Until then, every authz query that joins through
  `group_permissions`/`user_permissions` must keep the defensive filter.

## Steps

1. Audit current production data (or, in tests, generate an audit query) to
   surface rows where `group_permissions.domain_id != groups.domain_id`.
2. Decide on cleanup strategy (delete vs. fail migration).
3. Add migration; verify SQLite raises a foreign-key error on a deliberate
   cross-domain insert.
4. Add a regression test asserting the schema rejects cross-domain inserts
   so any future migration that drops the constraint fails loudly.
5. Drop the redundant defensive filters in the authz list queries and update
   their long-form comments accordingly.

## Files / paths

- `go/migrations/sqlite/000003_*.sql` (new migration)
- `go/internal/store/sqlite/store.go` — eventually drop the redundant
  `g.domain_id` / `u.domain_id` filters; for now the comments reference T51.

## Acceptance criteria

- A direct insert with mismatched `domain_id` fails at the DB layer.
- Authz listings remain correct on a cleaned dataset, with or without the
  defensive Go-side filter.

## Related

- Surfaced during PR review for T45 (`ResourceAuthzGroupsList`).
