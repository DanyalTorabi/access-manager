-- T51: Enforce at the schema level that junction tables cannot reference an
-- entity from a different domain. A composite UNIQUE KEY (id, domain_id) on
-- users/groups/permissions provides the FK target; each junction column
-- pair references it.
--
-- MySQL/MariaDB note: MySQL does not support DDL transactions; this migration
-- is not transactional. If the pre-check fires, the migration is aborted and
-- should be retried after fixing the cross-domain data.
--
-- Audit query to identify cross-domain rows before retrying:
--
--   SELECT 'group_permissions.group_domain' AS src, gp.domain_id, gp.group_id, gp.permission_id
--     FROM group_permissions gp JOIN `groups` g ON gp.group_id = g.id
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
--     FROM group_members gm JOIN `groups` g ON gm.group_id = g.id
--     WHERE gm.domain_id <> g.domain_id;

-- Step 0: pre-check via a BEFORE INSERT trigger on a temporary marker table.
-- SIGNAL SQLSTATE aborts the triggering INSERT, halting the migration.
-- Note: MySQL does not support transactional DDL; the pre-check fires on the
-- INSERT statement only. If the pre-check passes, subsequent ALTER TABLE
-- statements run independently (non-transactionally).
-- The DELIMITER change below is required by the mysql CLI to handle the
-- trigger body that contains semicolons. A future Go-based migration runner
-- (T59/T60) should split statements by $$ or use multiStatements=true with
-- DELIMITER-aware parsing.

CREATE TABLE _mig_t51_marker (x INT) ENGINE=InnoDB;

DELIMITER $$
CREATE TRIGGER _mig_t51_check BEFORE INSERT ON _mig_t51_marker
FOR EACH ROW
BEGIN
    DECLARE n BIGINT DEFAULT 0;
    SELECT COALESCE(SUM(c), 0) INTO n FROM (
        SELECT COUNT(*) AS c FROM group_permissions gp JOIN `groups`     g ON gp.group_id      = g.id WHERE gp.domain_id <> g.domain_id
        UNION ALL
        SELECT COUNT(*)        FROM group_permissions gp JOIN permissions p ON gp.permission_id = p.id WHERE gp.domain_id <> p.domain_id
        UNION ALL
        SELECT COUNT(*)        FROM user_permissions  up JOIN users        u ON up.user_id      = u.id WHERE up.domain_id <> u.domain_id
        UNION ALL
        SELECT COUNT(*)        FROM user_permissions  up JOIN permissions  p ON up.permission_id = p.id WHERE up.domain_id <> p.domain_id
        UNION ALL
        SELECT COUNT(*)        FROM group_members     gm JOIN users        u ON gm.user_id      = u.id WHERE gm.domain_id <> u.domain_id
        UNION ALL
        SELECT COUNT(*)        FROM group_members     gm JOIN `groups`     g ON gm.group_id     = g.id WHERE gm.domain_id <> g.domain_id
    ) sub;
    IF n > 0 THEN
        SIGNAL SQLSTATE '45000'
            SET MESSAGE_TEXT = 'T51: cross-domain junction rows detected; see audit query in migration header before retrying';
    END IF;
END$$
DELIMITER ;

INSERT INTO _mig_t51_marker (x) VALUES (1);

DROP TRIGGER _mig_t51_check;
DROP TABLE   _mig_t51_marker;

-- Step 1: add composite UNIQUE KEY (id, domain_id) to users, groups, permissions.
-- The leading `id` column keeps the planner preferring the PK / per-domain index
-- for domain-scoped scans, avoiding optimizer issues with ORDER BY + LIMIT/OFFSET.
ALTER TABLE users       ADD CONSTRAINT uq_users_id_domain       UNIQUE KEY (id, domain_id);
ALTER TABLE `groups`    ADD CONSTRAINT uq_groups_id_domain      UNIQUE KEY (id, domain_id);
ALTER TABLE permissions ADD CONSTRAINT uq_permissions_id_domain UNIQUE KEY (id, domain_id);

-- Step 2: replace single-column FKs on junction tables with composite FKs.

-- group_members
ALTER TABLE group_members
    DROP FOREIGN KEY fk_group_members_user,
    DROP FOREIGN KEY fk_group_members_group;

ALTER TABLE group_members
    ADD CONSTRAINT fk_group_members_user_domain
        FOREIGN KEY (user_id,  domain_id) REFERENCES users    (id, domain_id) ON DELETE RESTRICT,
    ADD CONSTRAINT fk_group_members_group_domain
        FOREIGN KEY (group_id, domain_id) REFERENCES `groups` (id, domain_id) ON DELETE RESTRICT;

-- user_permissions
ALTER TABLE user_permissions
    DROP FOREIGN KEY fk_user_permissions_user,
    DROP FOREIGN KEY fk_user_permissions_permission;

ALTER TABLE user_permissions
    ADD CONSTRAINT fk_user_permissions_user_domain
        FOREIGN KEY (user_id,       domain_id) REFERENCES users       (id, domain_id) ON DELETE RESTRICT,
    ADD CONSTRAINT fk_user_permissions_permission_domain
        FOREIGN KEY (permission_id, domain_id) REFERENCES permissions (id, domain_id) ON DELETE RESTRICT;

-- group_permissions
ALTER TABLE group_permissions
    DROP FOREIGN KEY fk_group_permissions_group,
    DROP FOREIGN KEY fk_group_permissions_permission;

ALTER TABLE group_permissions
    ADD CONSTRAINT fk_group_permissions_group_domain
        FOREIGN KEY (group_id,      domain_id) REFERENCES `groups`    (id, domain_id) ON DELETE RESTRICT,
    ADD CONSTRAINT fk_group_permissions_permission_domain
        FOREIGN KEY (permission_id, domain_id) REFERENCES permissions (id, domain_id) ON DELETE RESTRICT;
