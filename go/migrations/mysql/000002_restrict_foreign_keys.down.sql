-- Revert T33: restore ON DELETE CASCADE on all FKs changed by migration 2.
-- Uses two ALTER TABLE statements per table (drop then add) for MySQL compatibility.

ALTER TABLE users DROP FOREIGN KEY fk_users_domain;
ALTER TABLE users ADD CONSTRAINT fk_users_domain
    FOREIGN KEY (domain_id) REFERENCES domains (id) ON DELETE CASCADE;

ALTER TABLE `groups` DROP FOREIGN KEY fk_groups_domain;
ALTER TABLE `groups` ADD CONSTRAINT fk_groups_domain
    FOREIGN KEY (domain_id) REFERENCES domains (id) ON DELETE CASCADE;

ALTER TABLE resources DROP FOREIGN KEY fk_resources_domain;
ALTER TABLE resources ADD CONSTRAINT fk_resources_domain
    FOREIGN KEY (domain_id) REFERENCES domains (id) ON DELETE CASCADE;

ALTER TABLE access_types DROP FOREIGN KEY fk_access_types_domain;
ALTER TABLE access_types ADD CONSTRAINT fk_access_types_domain
    FOREIGN KEY (domain_id) REFERENCES domains (id) ON DELETE CASCADE;

ALTER TABLE permissions DROP FOREIGN KEY fk_permissions_domain;
ALTER TABLE permissions ADD CONSTRAINT fk_permissions_domain
    FOREIGN KEY (domain_id) REFERENCES domains (id) ON DELETE CASCADE;

ALTER TABLE permissions DROP FOREIGN KEY fk_permissions_resource;
ALTER TABLE permissions ADD CONSTRAINT fk_permissions_resource
    FOREIGN KEY (resource_id) REFERENCES resources (id) ON DELETE CASCADE;

ALTER TABLE group_members DROP FOREIGN KEY fk_group_members_domain;
ALTER TABLE group_members ADD CONSTRAINT fk_group_members_domain
    FOREIGN KEY (domain_id) REFERENCES domains (id) ON DELETE CASCADE;

ALTER TABLE group_members DROP FOREIGN KEY fk_group_members_user;
ALTER TABLE group_members ADD CONSTRAINT fk_group_members_user
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE;

ALTER TABLE group_members DROP FOREIGN KEY fk_group_members_group;
ALTER TABLE group_members ADD CONSTRAINT fk_group_members_group
    FOREIGN KEY (group_id) REFERENCES `groups` (id) ON DELETE CASCADE;

ALTER TABLE user_permissions DROP FOREIGN KEY fk_user_permissions_domain;
ALTER TABLE user_permissions ADD CONSTRAINT fk_user_permissions_domain
    FOREIGN KEY (domain_id) REFERENCES domains (id) ON DELETE CASCADE;

ALTER TABLE user_permissions DROP FOREIGN KEY fk_user_permissions_user;
ALTER TABLE user_permissions ADD CONSTRAINT fk_user_permissions_user
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE;

ALTER TABLE user_permissions DROP FOREIGN KEY fk_user_permissions_permission;
ALTER TABLE user_permissions ADD CONSTRAINT fk_user_permissions_permission
    FOREIGN KEY (permission_id) REFERENCES permissions (id) ON DELETE CASCADE;

ALTER TABLE group_permissions DROP FOREIGN KEY fk_group_permissions_domain;
ALTER TABLE group_permissions ADD CONSTRAINT fk_group_permissions_domain
    FOREIGN KEY (domain_id) REFERENCES domains (id) ON DELETE CASCADE;

ALTER TABLE group_permissions DROP FOREIGN KEY fk_group_permissions_group;
ALTER TABLE group_permissions ADD CONSTRAINT fk_group_permissions_group
    FOREIGN KEY (group_id) REFERENCES `groups` (id) ON DELETE CASCADE;

ALTER TABLE group_permissions DROP FOREIGN KEY fk_group_permissions_permission;
ALTER TABLE group_permissions ADD CONSTRAINT fk_group_permissions_permission
    FOREIGN KEY (permission_id) REFERENCES permissions (id) ON DELETE CASCADE;

