# T35 — List Filtering (GitHub [#46](https://github.com/DanyalTorabi/access-manager/issues/46))

## Goal

Add title substring search and entity-specific filters to the six
paginated list endpoints so clients can narrow results without
downloading full pages.

## Design decisions

- **Title search:** `?search=<term>` → case-insensitive `LIKE '%term%'` on `title`.
  Empty or whitespace-only value is ignored. Max length 255 characters.
  SQL LIKE wildcards (`%`, `_`, `\`) are escaped before binding.
- **Entity-specific filters:**
  - Groups: `?parent_group_id=<uuid>` → `WHERE parent_group_id = ?`
  - Permissions: `?resource_id=<uuid>` → `WHERE resource_id = ?`
- **Store types:** `Search` field added to existing `ListOpts`.
  New `GroupListOpts` and `PermissionListOpts` embed `ListOpts` and
  add optional pointer fields (`*string`) for entity-specific filters.
- **Filtering affects `meta.total`:** both `COUNT(*)` and the data query
  share the same dynamic `WHERE` clause.
- **Query parameter trimming:** `search`, `parent_group_id`, and
  `resource_id` are trimmed with `strings.TrimSpace`.

## Affected layers

| Layer | Change |
|-------|--------|
| `internal/store` | Add `Search` to `ListOpts`; add `GroupListOpts`, `PermissionListOpts`; update `GroupList`/`PermissionList` signatures |
| `internal/store/sqlite` | Dynamic `WHERE` with escaped `LIKE` in all 6 `*List` methods; `parent_group_id`/`resource_id` filter in `GroupList`/`PermissionList` |
| `internal/api` | Rename `parsePagination` → `parseListOpts`; add `parseGroupListOpts`, `parsePermissionListOpts`; update 6 list handlers |
| `api/openapi.yaml` | Add `search`, `parent_group_id`, `resource_id` query params; paginated envelope response schemas |
| Postman collection | Add query params to 6 list requests |

## Security

- `escapeLikePattern` prevents `%` and `_` in user input from acting as SQL wildcards.
- `ESCAPE '\'` clause makes the escape character explicit and portable.
- All filter values use parameterized bind parameters (no string concatenation into SQL).
- `search` max length (255 chars) limits resource usage.

## Dependencies

- T34 (list pagination) — uses `ListOpts`, `writeList`, envelope format.

## Out of scope

- Root-group filter (`parent_group_id IS NULL`). See T2.
- Full-text / ranked search.
- Filtering by dates, masks, or other non-title fields.
