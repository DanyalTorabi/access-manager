# PostgreSQL migrations

Versioned DDL for PostgreSQL 15+, mirroring the SQLite migration set.

| File | Description |
|------|-------------|
| `000001_init.{up,down}.sql` | Initial schema (tables + indexes) |
| `000002_restrict_foreign_keys.{up,down}.sql` | Switch all FKs to `ON DELETE RESTRICT` (T33) |
| `000003_composite_fk_cross_domain.{up,down}.sql` | Composite `UNIQUE (id, domain_id)` + composite FKs on junction tables; cross-domain pre-check via `DO $$ RAISE EXCEPTION $$` (T51, #77) |

Implemented in T56 (#91). Store implementation (`internal/store/postgres`) and driver wiring (`internal/database.Open`) are tracked in T58 and T60. Docker Compose service and integration-test targets are in T61 (#96).
