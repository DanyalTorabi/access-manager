-- T51: Enforce, at the schema level, that the junction tables
-- (group_members, user_permissions, group_permissions) cannot reference an
-- entity from a different domain. A composite UNIQUE (domain_id, id) on
-- users/groups/permissions provides the FK target; each junction column
-- pair references it. Authz list queries can then drop the redundant
-- defensive g.domain_id / u.domain_id / p.domain_id filters in a
-- follow-up (kept for now per the plan note).

-- Step 0: pre-check. Abort migration if any cross-domain row exists in the
-- three junction tables. The pattern uses a CHECK that fires only when the
-- INSERT actually receives a row (i.e. when at least one violation is
-- present). On clean datasets the INSERT is a no-op and the table is
-- dropped at the end.

CREATE TABLE _mig_abort_t51 (msg TEXT NOT NULL CHECK (msg = ''));

INSERT INTO _mig_abort_t51 (msg)
SELECT 'cross-domain rows detected in junction tables: '
       || COALESCE(SUM(n), 0)
       || ' (run audit query before retrying)'
FROM (
  SELECT COUNT(*) AS n FROM group_permissions gp JOIN groups g ON gp.group_id = g.id WHERE gp.domain_id <> g.domain_id
  UNION ALL
  SELECT COUNT(*) FROM group_permissions gp JOIN permissions p ON gp.permission_id = p.id WHERE gp.domain_id <> p.domain_id
  UNION ALL
  SELECT COUNT(*) FROM user_permissions up JOIN users u ON up.user_id = u.id WHERE up.domain_id <> u.domain_id
  UNION ALL
  SELECT COUNT(*) FROM user_permissions up JOIN permissions p ON up.permission_id = p.id WHERE up.domain_id <> p.domain_id
  UNION ALL
  SELECT COUNT(*) FROM group_members gm JOIN users u ON gm.user_id = u.id WHERE gm.domain_id <> u.domain_id
  UNION ALL
  SELECT COUNT(*) FROM group_members gm JOIN groups g ON gm.group_id = g.id WHERE gm.domain_id <> g.domain_id
)
HAVING COALESCE(SUM(n), 0) > 0;

DROP TABLE _mig_abort_t51;

-- Step 1: rebuild schema. SQLite cannot ALTER TABLE to add UNIQUE or FK
-- constraints, so use the standard CREATE-COPY-DROP-RENAME dance with
-- foreign_keys disabled. Existing per-column FKs (ON DELETE RESTRICT from
-- T33) and indexes are preserved.

PRAGMA foreign_keys = OFF;

CREATE TABLE _mig_users AS SELECT * FROM users;
CREATE TABLE _mig_groups AS SELECT * FROM groups;
CREATE TABLE _mig_permissions AS SELECT * FROM permissions;
CREATE TABLE _mig_group_members AS SELECT * FROM group_members;
CREATE TABLE _mig_user_permissions AS SELECT * FROM user_permissions;
CREATE TABLE _mig_group_permissions AS SELECT * FROM group_permissions;

DROP TABLE group_permissions;
DROP TABLE user_permissions;
DROP TABLE group_members;
DROP TABLE permissions;
DROP TABLE groups;
DROP TABLE users;

CREATE TABLE users (
    id TEXT PRIMARY KEY,
    domain_id TEXT NOT NULL REFERENCES domains (id) ON DELETE RESTRICT,
    title TEXT NOT NULL,
    -- UNIQUE(id, domain_id) provides an FK target for composite references
    -- without competing with idx_users_domain for `WHERE domain_id = ?`
    -- queries. The leading `id` column makes the planner prefer
    -- idx_users_domain (or the PK) for domain-scoped scans, which avoids a
    -- known optimizer issue with ORDER BY u.id LIMIT/OFFSET over the
    -- composite (domain_id, id) auto-index in EXISTS+OR queries.
    UNIQUE (id, domain_id)
);
INSERT INTO users SELECT id, domain_id, title FROM _mig_users;

CREATE TABLE groups (
    id TEXT PRIMARY KEY,
    domain_id TEXT NOT NULL REFERENCES domains (id) ON DELETE RESTRICT,
    title TEXT NOT NULL,
    parent_group_id TEXT REFERENCES groups (id) ON DELETE RESTRICT,
    UNIQUE (id, domain_id)
);
INSERT INTO groups SELECT id, domain_id, title, parent_group_id FROM _mig_groups;

CREATE TABLE permissions (
    id TEXT PRIMARY KEY,
    domain_id TEXT NOT NULL REFERENCES domains (id) ON DELETE RESTRICT,
    title TEXT NOT NULL,
    resource_id TEXT NOT NULL REFERENCES resources (id) ON DELETE RESTRICT,
    access_mask INTEGER NOT NULL,
    UNIQUE (id, domain_id)
);
INSERT INTO permissions SELECT id, domain_id, title, resource_id, access_mask FROM _mig_permissions;

CREATE TABLE group_members (
    domain_id TEXT NOT NULL REFERENCES domains (id) ON DELETE RESTRICT,
    user_id TEXT NOT NULL,
    group_id TEXT NOT NULL,
    PRIMARY KEY (user_id, group_id),
    FOREIGN KEY (user_id, domain_id) REFERENCES users (id, domain_id) ON DELETE RESTRICT,
    FOREIGN KEY (group_id, domain_id) REFERENCES groups (id, domain_id) ON DELETE RESTRICT
);
INSERT INTO group_members SELECT domain_id, user_id, group_id FROM _mig_group_members;

CREATE TABLE user_permissions (
    domain_id TEXT NOT NULL REFERENCES domains (id) ON DELETE RESTRICT,
    user_id TEXT NOT NULL,
    permission_id TEXT NOT NULL,
    PRIMARY KEY (user_id, permission_id),
    FOREIGN KEY (user_id, domain_id) REFERENCES users (id, domain_id) ON DELETE RESTRICT,
    FOREIGN KEY (permission_id, domain_id) REFERENCES permissions (id, domain_id) ON DELETE RESTRICT
);
INSERT INTO user_permissions SELECT domain_id, user_id, permission_id FROM _mig_user_permissions;

CREATE TABLE group_permissions (
    domain_id TEXT NOT NULL REFERENCES domains (id) ON DELETE RESTRICT,
    group_id TEXT NOT NULL,
    permission_id TEXT NOT NULL,
    PRIMARY KEY (group_id, permission_id),
    FOREIGN KEY (group_id, domain_id) REFERENCES groups (id, domain_id) ON DELETE RESTRICT,
    FOREIGN KEY (permission_id, domain_id) REFERENCES permissions (id, domain_id) ON DELETE RESTRICT
);
INSERT INTO group_permissions SELECT domain_id, group_id, permission_id FROM _mig_group_permissions;

CREATE INDEX idx_users_domain ON users (domain_id);
CREATE INDEX idx_groups_domain ON groups (domain_id);
CREATE INDEX idx_permissions_domain_resource ON permissions (domain_id, resource_id);
CREATE INDEX idx_group_members_domain_user ON group_members (domain_id, user_id);
CREATE INDEX idx_group_members_domain_group ON group_members (domain_id, group_id);
CREATE INDEX idx_user_permissions_domain_user ON user_permissions (domain_id, user_id);
CREATE INDEX idx_group_permissions_domain_group ON group_permissions (domain_id, group_id);

DROP TABLE _mig_users;
DROP TABLE _mig_groups;
DROP TABLE _mig_permissions;
DROP TABLE _mig_group_members;
DROP TABLE _mig_user_permissions;
DROP TABLE _mig_group_permissions;

PRAGMA foreign_keys = ON;
