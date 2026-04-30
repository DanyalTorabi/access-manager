-- T51 (down): revert composite UNIQUE / composite FK schema back to the
-- post-T33 state: single-column FKs ON DELETE RESTRICT, no UNIQUE (id, domain_id).

-- group_members: restore single-column FKs
ALTER TABLE group_members
    DROP CONSTRAINT fk_group_members_user,
    DROP CONSTRAINT fk_group_members_group;

ALTER TABLE group_members
    ADD CONSTRAINT group_members_user_id_fkey
        FOREIGN KEY (user_id)  REFERENCES users  (id) ON DELETE RESTRICT,
    ADD CONSTRAINT group_members_group_id_fkey
        FOREIGN KEY (group_id) REFERENCES groups (id) ON DELETE RESTRICT;

-- user_permissions: restore single-column FKs
ALTER TABLE user_permissions
    DROP CONSTRAINT fk_user_permissions_user,
    DROP CONSTRAINT fk_user_permissions_permission;

ALTER TABLE user_permissions
    ADD CONSTRAINT user_permissions_user_id_fkey
        FOREIGN KEY (user_id)       REFERENCES users       (id) ON DELETE RESTRICT,
    ADD CONSTRAINT user_permissions_permission_id_fkey
        FOREIGN KEY (permission_id) REFERENCES permissions (id) ON DELETE RESTRICT;

-- group_permissions: restore single-column FKs
ALTER TABLE group_permissions
    DROP CONSTRAINT fk_group_permissions_group,
    DROP CONSTRAINT fk_group_permissions_permission;

ALTER TABLE group_permissions
    ADD CONSTRAINT group_permissions_group_id_fkey
        FOREIGN KEY (group_id)      REFERENCES groups      (id) ON DELETE RESTRICT,
    ADD CONSTRAINT group_permissions_permission_id_fkey
        FOREIGN KEY (permission_id) REFERENCES permissions (id) ON DELETE RESTRICT;

-- Drop composite UNIQUE constraints
ALTER TABLE users       DROP CONSTRAINT uq_users_id_domain;
ALTER TABLE groups      DROP CONSTRAINT uq_groups_id_domain;
ALTER TABLE permissions DROP CONSTRAINT uq_permissions_id_domain;
