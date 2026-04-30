-- T33: Replace ON DELETE CASCADE with RESTRICT so deleting an entity that is
-- still referenced fails instead of silently removing dependents.
-- PostgreSQL supports ALTER TABLE … DROP CONSTRAINT … ADD CONSTRAINT …, so
-- no table-rebuild dance is required (unlike SQLite).

BEGIN;

-- users.domain_id
ALTER TABLE users
    DROP CONSTRAINT users_domain_id_fkey,
    ADD  CONSTRAINT users_domain_id_fkey
        FOREIGN KEY (domain_id) REFERENCES domains (id) ON DELETE RESTRICT;

-- groups.domain_id
ALTER TABLE groups
    DROP CONSTRAINT groups_domain_id_fkey,
    ADD  CONSTRAINT groups_domain_id_fkey
        FOREIGN KEY (domain_id) REFERENCES domains (id) ON DELETE RESTRICT;

-- resources.domain_id
ALTER TABLE resources
    DROP CONSTRAINT resources_domain_id_fkey,
    ADD  CONSTRAINT resources_domain_id_fkey
        FOREIGN KEY (domain_id) REFERENCES domains (id) ON DELETE RESTRICT;

-- access_types.domain_id
ALTER TABLE access_types
    DROP CONSTRAINT access_types_domain_id_fkey,
    ADD  CONSTRAINT access_types_domain_id_fkey
        FOREIGN KEY (domain_id) REFERENCES domains (id) ON DELETE RESTRICT;

-- permissions.domain_id
ALTER TABLE permissions
    DROP CONSTRAINT permissions_domain_id_fkey,
    ADD  CONSTRAINT permissions_domain_id_fkey
        FOREIGN KEY (domain_id) REFERENCES domains (id) ON DELETE RESTRICT;

-- permissions.resource_id
ALTER TABLE permissions
    DROP CONSTRAINT permissions_resource_id_fkey,
    ADD  CONSTRAINT permissions_resource_id_fkey
        FOREIGN KEY (resource_id) REFERENCES resources (id) ON DELETE RESTRICT;

-- group_members.domain_id
ALTER TABLE group_members
    DROP CONSTRAINT group_members_domain_id_fkey,
    ADD  CONSTRAINT group_members_domain_id_fkey
        FOREIGN KEY (domain_id) REFERENCES domains (id) ON DELETE RESTRICT;

-- group_members.user_id
ALTER TABLE group_members
    DROP CONSTRAINT group_members_user_id_fkey,
    ADD  CONSTRAINT group_members_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE RESTRICT;

-- group_members.group_id
ALTER TABLE group_members
    DROP CONSTRAINT group_members_group_id_fkey,
    ADD  CONSTRAINT group_members_group_id_fkey
        FOREIGN KEY (group_id) REFERENCES groups (id) ON DELETE RESTRICT;

-- user_permissions.domain_id
ALTER TABLE user_permissions
    DROP CONSTRAINT user_permissions_domain_id_fkey,
    ADD  CONSTRAINT user_permissions_domain_id_fkey
        FOREIGN KEY (domain_id) REFERENCES domains (id) ON DELETE RESTRICT;

-- user_permissions.user_id
ALTER TABLE user_permissions
    DROP CONSTRAINT user_permissions_user_id_fkey,
    ADD  CONSTRAINT user_permissions_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE RESTRICT;

-- user_permissions.permission_id
ALTER TABLE user_permissions
    DROP CONSTRAINT user_permissions_permission_id_fkey,
    ADD  CONSTRAINT user_permissions_permission_id_fkey
        FOREIGN KEY (permission_id) REFERENCES permissions (id) ON DELETE RESTRICT;

-- group_permissions.domain_id
ALTER TABLE group_permissions
    DROP CONSTRAINT group_permissions_domain_id_fkey,
    ADD  CONSTRAINT group_permissions_domain_id_fkey
        FOREIGN KEY (domain_id) REFERENCES domains (id) ON DELETE RESTRICT;

-- group_permissions.group_id
ALTER TABLE group_permissions
    DROP CONSTRAINT group_permissions_group_id_fkey,
    ADD  CONSTRAINT group_permissions_group_id_fkey
        FOREIGN KEY (group_id) REFERENCES groups (id) ON DELETE RESTRICT;

-- group_permissions.permission_id
ALTER TABLE group_permissions
    DROP CONSTRAINT group_permissions_permission_id_fkey,
    ADD  CONSTRAINT group_permissions_permission_id_fkey
        FOREIGN KEY (permission_id) REFERENCES permissions (id) ON DELETE RESTRICT;

COMMIT;
