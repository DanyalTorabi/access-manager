-- Enforce unique (domain_id, title) pairs so that v2 title-based lookups
-- are unambiguous.
CREATE UNIQUE INDEX idx_access_types_domain_title ON access_types (domain_id, title);
