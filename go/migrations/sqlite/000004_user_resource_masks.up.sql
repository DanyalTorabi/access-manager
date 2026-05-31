-- T04: materialized effective-mask cache.
-- Stores the precomputed OR of all access_mask values from permissions that
-- a given user can exercise on a given resource (via direct user_permissions
-- or via group_permissions + group_members). Written transactionally by the
-- store's Grant/Revoke/AddUserToGroup/RemoveUserFromGroup methods. Read by
-- EffectiveMask, UserAuthzResourcesList, and ResourceAuthzUsersList.
--
-- TODO(T02): ancestor group inheritance will require recomputing masks when
-- a parent group's permissions change. The schema is designed to support
-- this by keeping (domain_id, user_id, resource_id) as a natural composite
-- key — no structural changes are needed, only additional write-through
-- paths in the mutation methods.
CREATE TABLE user_resource_masks (
    domain_id   TEXT NOT NULL REFERENCES domains   (id) ON DELETE CASCADE,
    user_id     TEXT NOT NULL REFERENCES users     (id) ON DELETE CASCADE,
    resource_id TEXT NOT NULL REFERENCES resources (id) ON DELETE CASCADE,
    access_mask INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (domain_id, user_id, resource_id)
);

-- Enables UserAuthzResourcesList: "all resources user U can access in domain D"
CREATE INDEX idx_urm_domain_user     ON user_resource_masks (domain_id, user_id);
-- Enables ResourceAuthzUsersList: "all users who can access resource R in domain D"
CREATE INDEX idx_urm_domain_resource ON user_resource_masks (domain_id, resource_id);
