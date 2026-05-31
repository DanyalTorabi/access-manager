-- Enforce unique (domain_id, title) pairs so that v2 title-based lookups
-- are unambiguous. Existing rows must not have duplicate titles within a
-- domain; the insert-time path already rejects blank titles (NOT NULL) but
-- did not enforce uniqueness until this migration.
CREATE UNIQUE INDEX idx_access_types_domain_title ON access_types (domain_id, title);
