# T34 — List Pagination

**Issue:** [#45](https://github.com/DanyalTorabi/access-manager/issues/45)

## Goal

Add offset/limit pagination to all six list endpoints. Return results in a
`{"data": [...], "meta": {"total": N, "offset": 0, "limit": 20}}` envelope so
clients can page through results and know the total record count.

## Design decisions

- **Pagination style:** offset-based (`offset` / `limit` query params).
- **Response envelope:** `{"data": [...], "meta": {"total", "offset", "limit"}}`.
- **Defaults:** offset = 0, limit = 20, max limit = 100.
- **Validation:** negative offset → 400; limit clamped to [1, 100].
- **Breaking change:** list responses change from bare array to envelope (pre-1.0).
- **Total count:** `SELECT COUNT(*)` in every paginated store call; no separate endpoint.

## Affected layers

| Layer | Change |
|-------|--------|
| `internal/store` | Add `ListOpts` struct; update 6 `*List` signatures to accept `ListOpts` and return `(items, total int, err)` |
| `internal/store/sqlite` | Add `COUNT(*)` + `LIMIT ? OFFSET ?` to 6 `*List` methods; defensive clamping of `ListOpts` |
| `internal/api` | Add `parsePagination`, `listEnvelope`, `writeList`; update 6 list handlers |

## Security

- All `LIMIT ?` / `OFFSET ?` values use parameterized bind parameters.
- `parsePagination` parses with `strconv.Atoi`; returns 400 on non-integer input.
- Negative offset → 400; limit clamped to [1, 100] (DoS prevention).
- Store layer clamps defensively for internal callers that bypass HTTP.
- `total` count discloses record cardinality; acceptable for admin API, revisit with T07 auth.

## Dependencies

- None (builds on existing CRUD).

## Out of scope

- Cursor-based pagination.
- Filtering / search (T35).
