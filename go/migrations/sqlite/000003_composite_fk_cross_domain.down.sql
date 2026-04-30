-- T51 (down): revert the composite UNIQUE / composite FK schema produced by
-- 000003_composite_fk_cross_domain.up.sql back to the post-T33 schema:
-- per-column FKs ON DELETE RESTRICT and no UNIQUE (id, domain_id) on
-- users/groups/permissions.
--
-- SQLite cannot ALTER TABLE to drop UNIQUE / FK constraints, so this uses
-- the CREATE-COPY-DROP-RENAME dance with foreign_keys disabled, mirroring
-- 000002_restrict_foreign_keys.up.sql. The down migration is operator-run
-- (the in-tree migrator currently only applies .up.sql); it ships so a
-- rollback can be performed manually with sqlite3 < this file.

PRAGMA foreign_keys = OFF;

CREATE TABLE _down_users AS SELECT * FROM users;
CREATE TABLE _down_groups AS SELECT * FROM groups;
CREATE TABLE _down_permissions AS SELECT * FROM permissions;
CREATE TABLE _down_group_members AS SELECT * FROM group_members;
CREATE TABLE _down_user_permissions AS SELECT * FROM user_permissions;
CREATE TABLE _down_group_permissions AS SELECT * FROM group_permissions;

DROP TABLE group_permissions;
DROP TABLE user_permissions;
DROP TABLE group_members;
DROP TABLE permissions;
DROP TABLE groups;
DROP TABLE users;

CREATE TABLE users (
    id TEXT PRIMARY KEY,
    domain_id TEXT NOT NULL REFERENCES domains (id) ON DELETE RESTRICT,
    title TEXT NOT NULL
);
INSERT INTO users SELECT id, domain_id, title FROM _down_users;

CREATE TABLE groups (
    id TEXT PRIMARY KEY,
    domain_id TEXT NOT NULL REFERENCES domains (id) ON DELETE RESTRICT,
    title TEXT NOT NULL,
    parent_group_id TEXT REFERENCES groups (id) ON DELETE RESTRICT
);
INSERT INTO groups SELECT id, domain_id, title, parent_group_id FROM _down_groups;

CREATE TABLE permissions (
    id TEXT PRIMARY KEY,
    domain_id TEXT NOT NULL REFERENCES domains (id) ON DELETE RESTRICT,
    title TEXT NOT NULL,
    resource_id TEXT NOT NULL REFERENCES resources (id) ON DELETE RESTRICT,
    access_mask INTEGER NOT NULL
);
INSERT INTO permissions SELECT id, domain_id, title, resource_id, access_mask FROM _down_permissions;

CREATE TABLE group_members (
    domain_id TEXT NOT NULL REFERENCES domains (id) ON DELETE RESTRICT,
    user_id TEXT NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    group_id TEXT NOT NULL REFERENCES groups (id) ON DELETE RESTRICT,
    PRIMARY KEY (user_id, group_id)
);
INSERT INTO group_members SELECT domain_id, user_id, group_id FROM _down_group_members;

CREATE TABLE user_permissions (
    domain_id TEXT NOT NULL REFERENCES domains (id) ON DELETE RESTRICT,
    user_id TEXT NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    permission_id TEXT NOT NULL REFERENCES permissions (id) ON DELETE RESTRICT,
    PRIMARY KEY (user_id, permission_id)
);
INSERT INTO user_permissions SELECT domain_id, user_id, permission_id FROM _down_user_permissions;

CREATE TABLE group_permissions (
    domain_id TEXT NOT NULL REFERENCES domains (id) ON DELETE RESTRICT,
    group_id TEXT NOT NULL REFERENCES groups (id) ON DELETE RESTRICT,
    permission_id TEXT NOT NULL REFERENCES permissions (id) ON DELETE RESTRICT,
    PRIMARY KEY (group_id, permission_id)
);
INSERT INTO group_permissions SELECT domain_id, group_id, permission_id FROM _down_group_permissions;

CREATE INDEX idx_users_domain ON users (domain_id);
CREATE INDEX idx_groups_domain ON groups (domain_id);
CREATE INDEX idx_permissions_domain_resource ON permissions (domain_id, resource_id);
CREATE INDEX idx_group_members_domain_user ON group_members (domain_id, user_id);
CREATE INDEX idx_group_members_domain_group ON group_members (domain_id, group_id);
CREATE INDEX idx_user_permissions_domain_user ON user_permissions (domain_id, user_id);
CREATE INDEX idx_group_permissions_domain_group ON group_permissions (domain_id, group_id);

DROP TABLE _down_users;
DROP TABLE _down_groups;
DROP TABLE _down_permissions;
DROP TABLE _down_group_members;
DROP TABLE _down_user_permissions;
DROP TABLE _down_group_permissions;

DELETE FROM schema_migrations WHERE version = 3;

PRAGMA foreign_keys = ON;
