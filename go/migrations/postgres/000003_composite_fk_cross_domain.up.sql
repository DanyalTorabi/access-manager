-- T51: Enforce at the schema level that junction tables cannot reference an
-- entity from a different domain. A composite UNIQUE (id, domain_id) on
-- users/groups/permissions provides the FK target; each junction column
-- pair references it.
--
-- Audit query to identify cross-domain rows before retrying:
--
--   SELECT 'group_permissions.group_domain' AS src, gp.domain_id, gp.group_id, gp.permission_id
--     FROM group_permissions gp JOIN groups g ON gp.group_id = g.id
--     WHERE gp.domain_id <> g.domain_id
--   UNION ALL
--   SELECT 'group_permissions.permission_domain', gp.domain_id, gp.group_id, gp.permission_id
--     FROM group_permissions gp JOIN permissions p ON gp.permission_id = p.id
--     WHERE gp.domain_id <> p.domain_id
--   UNION ALL
--   SELECT 'user_permissions.user_domain', up.domain_id, up.user_id, up.permission_id
--     FROM user_permissions up JOIN users u ON up.user_id = u.id
--     WHERE up.domain_id <> u.domain_id
--   UNION ALL
--   SELECT 'user_permissions.permission_domain', up.domain_id, up.user_id, up.permission_id
--     FROM user_permissions up JOIN permissions p ON up.permission_id = p.id
--     WHERE up.domain_id <> p.domain_id
--   UNION ALL
--   SELECT 'group_members.user_domain', gm.domain_id, gm.user_id, gm.group_id
--     FROM group_members gm JOIN users u ON gm.user_id = u.id
--     WHERE gm.domain_id <> u.domain_id
--   UNION ALL
--   SELECT 'group_members.group_domain', gm.domain_id, gm.user_id, gm.group_id
--     FROM group_members gm JOIN groups g ON gm.group_id = g.id
--     WHERE gm.domain_id <> g.domain_id;

BEGIN;

-- Step 0: pre-check. Abort if any cross-domain rows exist.
DO $$
DECLARE
    n BIGINT;
BEGIN
    SELECT COALESCE(SUM(c), 0) INTO n FROM (
        SELECT COUNT(*) AS c FROM group_permissions gp JOIN groups g       ON gp.group_id       = g.id WHERE gp.domain_id <> g.domain_id
        UNION ALL
        SELECT COUNT(*)        FROM group_permissions gp JOIN permissions p ON gp.permission_id  = p.id WHERE gp.domain_id <> p.domain_id
        UNION ALL
        SELECT COUNT(*)        FROM user_permissions  up JOIN users u       ON up.user_id        = u.id WHERE up.domain_id <> u.domain_id
        UNION ALL
        SELECT COUNT(*)        FROM user_permissions  up JOIN permissions p ON up.permission_id  = p.id WHERE up.domain_id <> p.domain_id
        UNION ALL
        SELECT COUNT(*)        FROM group_members     gm JOIN users u       ON gm.user_id        = u.id WHERE gm.domain_id <> u.domain_id
        UNION ALL
        SELECT COUNT(*)        FROM group_members     gm JOIN groups g      ON gm.group_id       = g.id WHERE gm.domain_id <> g.domain_id
    ) sub;
    IF n > 0 THEN
        RAISE EXCEPTION 'T51: cross-domain junction rows detected; see audit query in migration header before retrying';
    END IF;
END $$;

-- Step 1: add composite UNIQUE (id, domain_id) to users, groups, permissions.
-- The leading `id` column keeps the planner preferring the existing PK / per-domain
-- index for domain-scoped scans (avoids optimizer issues with ORDER BY + LIMIT/OFFSET).
ALTER TABLE users       ADD CONSTRAINT uq_users_id_domain       UNIQUE (id, domain_id);
ALTER TABLE groups      ADD CONSTRAINT uq_groups_id_domain      UNIQUE (id, domain_id);
ALTER TABLE permissions ADD CONSTRAINT uq_permissions_id_domain UNIQUE (id, domain_id);

-- Step 2: drop the existing single-column FKs on the three junction tables
-- and re-add them as composite FKs referencing (id, domain_id).

-- group_members
ALTER TABLE group_members
    DROP CONSTRAINT group_members_user_id_fkey,
    DROP CONSTRAINT group_members_group_id_fkey;

ALTER TABLE group_members
    ADD CONSTRAINT fk_group_members_user
        FOREIGN KEY (user_id,  domain_id) REFERENCES users  (id, domain_id) ON DELETE RESTRICT,
    ADD CONSTRAINT fk_group_members_group
        FOREIGN KEY (group_id, domain_id) REFERENCES groups (id, domain_id) ON DELETE RESTRICT;

-- user_permissions
ALTER TABLE user_permissions
    DROP CONSTRAINT user_permissions_user_id_fkey,
    DROP CONSTRAINT user_permissions_permission_id_fkey;

ALTER TABLE user_permissions
    ADD CONSTRAINT fk_user_permissions_user
        FOREIGN KEY (user_id,       domain_id) REFERENCES users       (id, domain_id) ON DELETE RESTRICT,
    ADD CONSTRAINT fk_user_permissions_permission
        FOREIGN KEY (permission_id, domain_id) REFERENCES permissions (id, domain_id) ON DELETE RESTRICT;

-- group_permissions
ALTER TABLE group_permissions
    DROP CONSTRAINT group_permissions_group_id_fkey,
    DROP CONSTRAINT group_permissions_permission_id_fkey;

ALTER TABLE group_permissions
    ADD CONSTRAINT fk_group_permissions_group
        FOREIGN KEY (group_id,      domain_id) REFERENCES groups      (id, domain_id) ON DELETE RESTRICT,
    ADD CONSTRAINT fk_group_permissions_permission
        FOREIGN KEY (permission_id, domain_id) REFERENCES permissions (id, domain_id) ON DELETE RESTRICT;

COMMIT;
