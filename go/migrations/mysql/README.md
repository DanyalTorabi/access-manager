# MySQL/MariaDB migrations

Versioned DDL for MySQL 8+ and MariaDB 10.6+, mirroring the SQLite migration set.

| File | Description |
|------|-------------|
| `000001_init.{up,down}.sql` | Initial schema (tables + indexes); `BIGINT UNSIGNED` for `bit`; `ENGINE=InnoDB` on all tables |
| `000002_restrict_foreign_keys.{up,down}.sql` | Switch all FKs to `ON DELETE RESTRICT` (T33) |
| `000003_composite_fk_cross_domain.{up,down}.sql` | Composite `UNIQUE KEY (id, domain_id)` + composite FKs on junction tables; cross-domain pre-check via `SIGNAL SQLSTATE '45000'` in a BEFORE INSERT trigger (T51, #77) |

**Note:** MySQL does not support transactional DDL. Migration 3 is non-transactional; if the pre-check fires the migration is aborted and can be retried once cross-domain rows are cleaned up.

Implemented in T57 (#92). Store implementation (`internal/store/mysql`) and driver wiring (`internal/database.Open`) are tracked in T59 and T60. Docker Compose service and integration-test targets are in T61 (#96).
