CREATE TABLE domains (
    id    VARCHAR(255) PRIMARY KEY,
    title VARCHAR(255) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE users (
    id        VARCHAR(255) PRIMARY KEY,
    domain_id VARCHAR(255) NOT NULL,
    title     VARCHAR(255) NOT NULL,
    CONSTRAINT fk_users_domain FOREIGN KEY (domain_id) REFERENCES domains (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE `groups` (
    id              VARCHAR(255) PRIMARY KEY,
    domain_id       VARCHAR(255) NOT NULL,
    title           VARCHAR(255) NOT NULL,
    parent_group_id VARCHAR(255) DEFAULT NULL,
    CONSTRAINT fk_groups_domain  FOREIGN KEY (domain_id)       REFERENCES domains  (id) ON DELETE CASCADE,
    CONSTRAINT fk_groups_parent  FOREIGN KEY (parent_group_id) REFERENCES `groups` (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE resources (
    id        VARCHAR(255) PRIMARY KEY,
    domain_id VARCHAR(255) NOT NULL,
    title     VARCHAR(255) NOT NULL,
    CONSTRAINT fk_resources_domain FOREIGN KEY (domain_id) REFERENCES domains (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE access_types (
    id        VARCHAR(255) PRIMARY KEY,
    domain_id VARCHAR(255) NOT NULL,
    title     VARCHAR(255) NOT NULL,
    bit       BIGINT UNSIGNED NOT NULL,
    UNIQUE KEY uq_access_types_domain_bit (domain_id, bit),
    CONSTRAINT fk_access_types_domain FOREIGN KEY (domain_id) REFERENCES domains (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE permissions (
    id          VARCHAR(255) PRIMARY KEY,
    domain_id   VARCHAR(255) NOT NULL,
    title       VARCHAR(255) NOT NULL,
    resource_id VARCHAR(255) NOT NULL,
    access_mask BIGINT NOT NULL,
    CONSTRAINT fk_permissions_domain   FOREIGN KEY (domain_id)   REFERENCES domains   (id) ON DELETE CASCADE,
    CONSTRAINT fk_permissions_resource FOREIGN KEY (resource_id) REFERENCES resources (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE group_members (
    domain_id VARCHAR(255) NOT NULL,
    user_id   VARCHAR(255) NOT NULL,
    group_id  VARCHAR(255) NOT NULL,
    PRIMARY KEY (user_id, group_id),
    CONSTRAINT fk_group_members_domain FOREIGN KEY (domain_id) REFERENCES domains  (id) ON DELETE CASCADE,
    CONSTRAINT fk_group_members_user   FOREIGN KEY (user_id)   REFERENCES users    (id) ON DELETE CASCADE,
    CONSTRAINT fk_group_members_group  FOREIGN KEY (group_id)  REFERENCES `groups` (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE user_permissions (
    domain_id     VARCHAR(255) NOT NULL,
    user_id       VARCHAR(255) NOT NULL,
    permission_id VARCHAR(255) NOT NULL,
    PRIMARY KEY (user_id, permission_id),
    CONSTRAINT fk_user_permissions_domain     FOREIGN KEY (domain_id)     REFERENCES domains      (id) ON DELETE CASCADE,
    CONSTRAINT fk_user_permissions_user       FOREIGN KEY (user_id)       REFERENCES users        (id) ON DELETE CASCADE,
    CONSTRAINT fk_user_permissions_permission FOREIGN KEY (permission_id) REFERENCES permissions  (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE group_permissions (
    domain_id     VARCHAR(255) NOT NULL,
    group_id      VARCHAR(255) NOT NULL,
    permission_id VARCHAR(255) NOT NULL,
    PRIMARY KEY (group_id, permission_id),
    CONSTRAINT fk_group_permissions_domain     FOREIGN KEY (domain_id)     REFERENCES domains      (id) ON DELETE CASCADE,
    CONSTRAINT fk_group_permissions_group      FOREIGN KEY (group_id)      REFERENCES `groups`     (id) ON DELETE CASCADE,
    CONSTRAINT fk_group_permissions_permission FOREIGN KEY (permission_id) REFERENCES permissions  (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE INDEX idx_users_domain                ON users            (domain_id);
CREATE INDEX idx_groups_domain               ON `groups`         (domain_id);
CREATE INDEX idx_resources_domain            ON resources        (domain_id);
CREATE INDEX idx_access_types_domain_bit     ON access_types     (domain_id, bit);
CREATE INDEX idx_permissions_domain_resource ON permissions      (domain_id, resource_id);
CREATE INDEX idx_group_members_domain_user   ON group_members    (domain_id, user_id);
CREATE INDEX idx_group_members_domain_group  ON group_members    (domain_id, group_id);
CREATE INDEX idx_user_permissions_domain_user    ON user_permissions  (domain_id, user_id);
CREATE INDEX idx_group_permissions_domain_group  ON group_permissions (domain_id, group_id);
