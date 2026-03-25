PRAGMA foreign_keys = ON;

CREATE TABLE domains (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL
);

CREATE TABLE users (
    id TEXT PRIMARY KEY,
    domain_id TEXT NOT NULL REFERENCES domains (id) ON DELETE CASCADE,
    title TEXT NOT NULL
);

CREATE TABLE groups (
    id TEXT PRIMARY KEY,
    domain_id TEXT NOT NULL REFERENCES domains (id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    parent_group_id TEXT REFERENCES groups (id) ON DELETE RESTRICT
);

CREATE TABLE resources (
    id TEXT PRIMARY KEY,
    domain_id TEXT NOT NULL REFERENCES domains (id) ON DELETE CASCADE,
    title TEXT NOT NULL
);

CREATE TABLE access_types (
    id TEXT PRIMARY KEY,
    domain_id TEXT NOT NULL REFERENCES domains (id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    bit INTEGER NOT NULL,
    UNIQUE (domain_id, bit)
);

CREATE TABLE permissions (
    id TEXT PRIMARY KEY,
    domain_id TEXT NOT NULL REFERENCES domains (id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    resource_id TEXT NOT NULL REFERENCES resources (id) ON DELETE CASCADE,
    access_mask INTEGER NOT NULL
);

CREATE TABLE group_members (
    domain_id TEXT NOT NULL REFERENCES domains (id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    group_id TEXT NOT NULL REFERENCES groups (id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, group_id)
);

CREATE TABLE user_permissions (
    domain_id TEXT NOT NULL REFERENCES domains (id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    permission_id TEXT NOT NULL REFERENCES permissions (id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, permission_id)
);

CREATE TABLE group_permissions (
    domain_id TEXT NOT NULL REFERENCES domains (id) ON DELETE CASCADE,
    group_id TEXT NOT NULL REFERENCES groups (id) ON DELETE CASCADE,
    permission_id TEXT NOT NULL REFERENCES permissions (id) ON DELETE CASCADE,
    PRIMARY KEY (group_id, permission_id)
);

CREATE INDEX idx_users_domain ON users (domain_id);

CREATE INDEX idx_groups_domain ON groups (domain_id);

CREATE INDEX idx_resources_domain ON resources (domain_id);

CREATE INDEX idx_access_types_domain_bit ON access_types (domain_id, bit);

CREATE INDEX idx_permissions_domain_resource ON permissions (domain_id, resource_id);

CREATE INDEX idx_group_members_domain_user ON group_members (domain_id, user_id);

CREATE INDEX idx_group_members_domain_group ON group_members (domain_id, group_id);

CREATE INDEX idx_user_permissions_domain_user ON user_permissions (domain_id, user_id);

CREATE INDEX idx_group_permissions_domain_group ON group_permissions (domain_id, group_id);
