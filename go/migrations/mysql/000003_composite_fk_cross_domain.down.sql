-- T51 (down): revert composite UNIQUE KEY / composite FK schema back to the
-- post-T33 state: single-column FKs ON DELETE RESTRICT, no composite UNIQUE KEY.

-- group_members: restore single-column FKs
ALTER TABLE group_members
    DROP FOREIGN KEY fk_group_members_user_domain,
    DROP FOREIGN KEY fk_group_members_group_domain;

ALTER TABLE group_members
    ADD CONSTRAINT fk_group_members_user
        FOREIGN KEY (user_id)  REFERENCES users    (id) ON DELETE RESTRICT,
    ADD CONSTRAINT fk_group_members_group
        FOREIGN KEY (group_id) REFERENCES `groups` (id) ON DELETE RESTRICT;

-- user_permissions: restore single-column FKs
ALTER TABLE user_permissions
    DROP FOREIGN KEY fk_user_permissions_user_domain,
    DROP FOREIGN KEY fk_user_permissions_permission_domain;

ALTER TABLE user_permissions
    ADD CONSTRAINT fk_user_permissions_user
        FOREIGN KEY (user_id)       REFERENCES users       (id) ON DELETE RESTRICT,
    ADD CONSTRAINT fk_user_permissions_permission
        FOREIGN KEY (permission_id) REFERENCES permissions (id) ON DELETE RESTRICT;

-- group_permissions: restore single-column FKs
ALTER TABLE group_permissions
    DROP FOREIGN KEY fk_group_permissions_group_domain,
    DROP FOREIGN KEY fk_group_permissions_permission_domain;

ALTER TABLE group_permissions
    ADD CONSTRAINT fk_group_permissions_group
        FOREIGN KEY (group_id)      REFERENCES `groups`    (id) ON DELETE RESTRICT,
    ADD CONSTRAINT fk_group_permissions_permission
        FOREIGN KEY (permission_id) REFERENCES permissions (id) ON DELETE RESTRICT;

-- Drop composite UNIQUE KEY constraints
ALTER TABLE users       DROP INDEX uq_users_id_domain;
ALTER TABLE `groups`    DROP INDEX uq_groups_id_domain;
ALTER TABLE permissions DROP INDEX uq_permissions_id_domain;
