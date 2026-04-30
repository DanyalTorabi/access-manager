-- Revert T33: restore ON DELETE CASCADE on all FKs changed by migration 2.

-- users.domain_id
ALTER TABLE users
    DROP CONSTRAINT users_domain_id_fkey,
    ADD  CONSTRAINT users_domain_id_fkey
        FOREIGN KEY (domain_id) REFERENCES domains (id) ON DELETE CASCADE;

-- groups.domain_id
ALTER TABLE groups
    DROP CONSTRAINT groups_domain_id_fkey,
    ADD  CONSTRAINT groups_domain_id_fkey
        FOREIGN KEY (domain_id) REFERENCES domains (id) ON DELETE CASCADE;

-- resources.domain_id
ALTER TABLE resources
    DROP CONSTRAINT resources_domain_id_fkey,
    ADD  CONSTRAINT resources_domain_id_fkey
        FOREIGN KEY (domain_id) REFERENCES domains (id) ON DELETE CASCADE;

-- access_types.domain_id
ALTER TABLE access_types
    DROP CONSTRAINT access_types_domain_id_fkey,
    ADD  CONSTRAINT access_types_domain_id_fkey
        FOREIGN KEY (domain_id) REFERENCES domains (id) ON DELETE CASCADE;

-- permissions.domain_id
ALTER TABLE permissions
    DROP CONSTRAINT permissions_domain_id_fkey,
    ADD  CONSTRAINT permissions_domain_id_fkey
        FOREIGN KEY (domain_id) REFERENCES domains (id) ON DELETE CASCADE;

-- permissions.resource_id
ALTER TABLE permissions
    DROP CONSTRAINT permissions_resource_id_fkey,
    ADD  CONSTRAINT permissions_resource_id_fkey
        FOREIGN KEY (resource_id) REFERENCES resources (id) ON DELETE CASCADE;

-- group_members.domain_id
ALTER TABLE group_members
    DROP CONSTRAINT group_members_domain_id_fkey,
    ADD  CONSTRAINT group_members_domain_id_fkey
        FOREIGN KEY (domain_id) REFERENCES domains (id) ON DELETE CASCADE;

-- group_members.user_id
ALTER TABLE group_members
    DROP CONSTRAINT group_members_user_id_fkey,
    ADD  CONSTRAINT group_members_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE;

-- group_members.group_id
ALTER TABLE group_members
    DROP CONSTRAINT group_members_group_id_fkey,
    ADD  CONSTRAINT group_members_group_id_fkey
        FOREIGN KEY (group_id) REFERENCES groups (id) ON DELETE CASCADE;

-- user_permissions.domain_id
ALTER TABLE user_permissions
    DROP CONSTRAINT user_permissions_domain_id_fkey,
    ADD  CONSTRAINT user_permissions_domain_id_fkey
        FOREIGN KEY (domain_id) REFERENCES domains (id) ON DELETE CASCADE;

-- user_permissions.user_id
ALTER TABLE user_permissions
    DROP CONSTRAINT user_permissions_user_id_fkey,
    ADD  CONSTRAINT user_permissions_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE;

-- user_permissions.permission_id
ALTER TABLE user_permissions
    DROP CONSTRAINT user_permissions_permission_id_fkey,
    ADD  CONSTRAINT user_permissions_permission_id_fkey
        FOREIGN KEY (permission_id) REFERENCES permissions (id) ON DELETE CASCADE;

-- group_permissions.domain_id
ALTER TABLE group_permissions
    DROP CONSTRAINT group_permissions_domain_id_fkey,
    ADD  CONSTRAINT group_permissions_domain_id_fkey
        FOREIGN KEY (domain_id) REFERENCES domains (id) ON DELETE CASCADE;

-- group_permissions.group_id
ALTER TABLE group_permissions
    DROP CONSTRAINT group_permissions_group_id_fkey,
    ADD  CONSTRAINT group_permissions_group_id_fkey
        FOREIGN KEY (group_id) REFERENCES groups (id) ON DELETE CASCADE;

-- group_permissions.permission_id
ALTER TABLE group_permissions
    DROP CONSTRAINT group_permissions_permission_id_fkey,
    ADD  CONSTRAINT group_permissions_permission_id_fkey
        FOREIGN KEY (permission_id) REFERENCES permissions (id) ON DELETE CASCADE;
