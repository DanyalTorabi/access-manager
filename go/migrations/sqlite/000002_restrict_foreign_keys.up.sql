-- T33: Replace ON DELETE CASCADE with RESTRICT so deleting an entity that is
-- still referenced fails instead of silently removing dependents.

PRAGMA foreign_keys = OFF;

CREATE TABLE _mig_domains AS SELECT * FROM domains;
CREATE TABLE _mig_users AS SELECT * FROM users;
CREATE TABLE _mig_groups AS SELECT * FROM groups;
CREATE TABLE _mig_resources AS SELECT * FROM resources;
CREATE TABLE _mig_access_types AS SELECT * FROM access_types;
CREATE TABLE _mig_permissions AS SELECT * FROM permissions;
CREATE TABLE _mig_group_members AS SELECT * FROM group_members;
CREATE TABLE _mig_user_permissions AS SELECT * FROM user_permissions;
CREATE TABLE _mig_group_permissions AS SELECT * FROM group_permissions;

DROP TABLE group_permissions;
DROP TABLE user_permissions;
DROP TABLE group_members;
DROP TABLE permissions;
DROP TABLE access_types;
DROP TABLE resources;
DROP TABLE groups;
DROP TABLE users;
DROP TABLE domains;

CREATE TABLE domains (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL
);
INSERT INTO domains SELECT * FROM _mig_domains;

CREATE TABLE users (
    id TEXT PRIMARY KEY,
    domain_id TEXT NOT NULL REFERENCES domains (id) ON DELETE RESTRICT,
    title TEXT NOT NULL
);
INSERT INTO users SELECT * FROM _mig_users;

CREATE TABLE groups (
    id TEXT PRIMARY KEY,
    domain_id TEXT NOT NULL REFERENCES domains (id) ON DELETE RESTRICT,
    title TEXT NOT NULL,
    parent_group_id TEXT REFERENCES groups (id) ON DELETE RESTRICT
);
INSERT INTO groups SELECT * FROM _mig_groups;

CREATE TABLE resources (
    id TEXT PRIMARY KEY,
    domain_id TEXT NOT NULL REFERENCES domains (id) ON DELETE RESTRICT,
    title TEXT NOT NULL
);
INSERT INTO resources SELECT * FROM _mig_resources;

CREATE TABLE access_types (
    id TEXT PRIMARY KEY,
    domain_id TEXT NOT NULL REFERENCES domains (id) ON DELETE RESTRICT,
    title TEXT NOT NULL,
    bit INTEGER NOT NULL,
    UNIQUE (domain_id, bit)
);
INSERT INTO access_types SELECT * FROM _mig_access_types;

CREATE TABLE permissions (
    id TEXT PRIMARY KEY,
    domain_id TEXT NOT NULL REFERENCES domains (id) ON DELETE RESTRICT,
    title TEXT NOT NULL,
    resource_id TEXT NOT NULL REFERENCES resources (id) ON DELETE RESTRICT,
    access_mask INTEGER NOT NULL
);
INSERT INTO permissions SELECT * FROM _mig_permissions;

CREATE TABLE group_members (
    domain_id TEXT NOT NULL REFERENCES domains (id) ON DELETE RESTRICT,
    user_id TEXT NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    group_id TEXT NOT NULL REFERENCES groups (id) ON DELETE RESTRICT,
    PRIMARY KEY (user_id, group_id)
);
INSERT INTO group_members SELECT * FROM _mig_group_members;

CREATE TABLE user_permissions (
    domain_id TEXT NOT NULL REFERENCES domains (id) ON DELETE RESTRICT,
    user_id TEXT NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    permission_id TEXT NOT NULL REFERENCES permissions (id) ON DELETE RESTRICT,
    PRIMARY KEY (user_id, permission_id)
);
INSERT INTO user_permissions SELECT * FROM _mig_user_permissions;

CREATE TABLE group_permissions (
    domain_id TEXT NOT NULL REFERENCES domains (id) ON DELETE RESTRICT,
    group_id TEXT NOT NULL REFERENCES groups (id) ON DELETE RESTRICT,
    permission_id TEXT NOT NULL REFERENCES permissions (id) ON DELETE RESTRICT,
    PRIMARY KEY (group_id, permission_id)
);
INSERT INTO group_permissions SELECT * FROM _mig_group_permissions;

CREATE INDEX idx_users_domain ON users (domain_id);
CREATE INDEX idx_groups_domain ON groups (domain_id);
CREATE INDEX idx_resources_domain ON resources (domain_id);
CREATE INDEX idx_access_types_domain_bit ON access_types (domain_id, bit);
CREATE INDEX idx_permissions_domain_resource ON permissions (domain_id, resource_id);
CREATE INDEX idx_group_members_domain_user ON group_members (domain_id, user_id);
CREATE INDEX idx_group_members_domain_group ON group_members (domain_id, group_id);
CREATE INDEX idx_user_permissions_domain_user ON user_permissions (domain_id, user_id);
CREATE INDEX idx_group_permissions_domain_group ON group_permissions (domain_id, group_id);

DROP TABLE _mig_domains;
DROP TABLE _mig_users;
DROP TABLE _mig_groups;
DROP TABLE _mig_resources;
DROP TABLE _mig_access_types;
DROP TABLE _mig_permissions;
DROP TABLE _mig_group_members;
DROP TABLE _mig_user_permissions;
DROP TABLE _mig_group_permissions;

PRAGMA foreign_keys = ON;
