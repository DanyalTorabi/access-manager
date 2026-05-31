package sqlite

import (
	"context"

	"github.com/dtorabi/access-manager/internal/store"
)

func (s *Store) AddUserToGroup(ctx context.Context, domainID, userID, groupID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO group_members (domain_id, user_id, group_id) VALUES (?, ?, ?)`,
		domainID, userID, groupID); err != nil {
		return wrapConstraintError(err)
	}

	affectedResources, err := groupResourceIDs(ctx, tx, domainID, groupID)
	if err != nil {
		return err
	}
	for _, resourceID := range affectedResources {
		if err := s.computeAndUpsertMask(ctx, tx, domainID, userID, resourceID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) RemoveUserFromGroup(ctx context.Context, domainID, userID, groupID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Capture the group's resource IDs before deleting so we know which
	// (user, resource) pairs need recomputation. group_permissions is
	// unaffected by this deletion so it is still queryable after the delete.
	affectedResources, err := groupResourceIDs(ctx, tx, domainID, groupID)
	if err != nil {
		return err
	}

	res, err := tx.ExecContext(ctx,
		`DELETE FROM group_members WHERE domain_id = ? AND user_id = ? AND group_id = ?`,
		domainID, userID, groupID)
	if err != nil {
		return wrapConstraintError(err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return store.ErrNotFound
	}

	// computeAndUpsertMask runs inside the tx where the group_members row is
	// already deleted, so the ground-truth query correctly excludes this
	// group's contribution when recomputing the user's mask.
	for _, resourceID := range affectedResources {
		if err := s.computeAndUpsertMask(ctx, tx, domainID, userID, resourceID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) GrantUserPermission(ctx context.Context, domainID, userID, permissionID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO user_permissions (domain_id, user_id, permission_id) VALUES (?, ?, ?)`,
		domainID, userID, permissionID); err != nil {
		return wrapConstraintError(err)
	}

	resourceID, err := permissionResourceID(ctx, tx, domainID, permissionID)
	if err != nil {
		return err
	}
	if err := s.computeAndUpsertMask(ctx, tx, domainID, userID, resourceID); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) RevokeUserPermission(ctx context.Context, domainID, userID, permissionID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx,
		`DELETE FROM user_permissions WHERE domain_id = ? AND user_id = ? AND permission_id = ?`,
		domainID, userID, permissionID)
	if err != nil {
		return wrapConstraintError(err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return store.ErrNotFound
	}

	// permissions row still exists after revoking user_permissions, so the
	// resource_id lookup succeeds. computeAndUpsertMask runs in the tx where
	// user_permissions is already deleted, correctly reflecting the revocation.
	resourceID, err := permissionResourceID(ctx, tx, domainID, permissionID)
	if err != nil {
		return err
	}
	if err := s.computeAndUpsertMask(ctx, tx, domainID, userID, resourceID); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) GrantGroupPermission(ctx context.Context, domainID, groupID, permissionID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO group_permissions (domain_id, group_id, permission_id) VALUES (?, ?, ?)`,
		domainID, groupID, permissionID); err != nil {
		return wrapConstraintError(err)
	}

	memberIDs, err := groupMemberIDs(ctx, tx, domainID, groupID)
	if err != nil {
		return err
	}
	if len(memberIDs) > 0 {
		resourceID, err := permissionResourceID(ctx, tx, domainID, permissionID)
		if err != nil {
			return err
		}
		for _, uid := range memberIDs {
			if err := s.computeAndUpsertMask(ctx, tx, domainID, uid, resourceID); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

func (s *Store) RevokeGroupPermission(ctx context.Context, domainID, groupID, permissionID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Capture current member IDs before the delete so we know which users
	// are affected.
	memberIDs, err := groupMemberIDs(ctx, tx, domainID, groupID)
	if err != nil {
		return err
	}

	res, err := tx.ExecContext(ctx,
		`DELETE FROM group_permissions WHERE domain_id = ? AND group_id = ? AND permission_id = ?`,
		domainID, groupID, permissionID)
	if err != nil {
		return wrapConstraintError(err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return store.ErrNotFound
	}

	if len(memberIDs) > 0 {
		// permissions row still exists (only group_permissions was deleted),
		// so resource_id lookup succeeds. computeAndUpsertMask runs in the tx
		// where group_permissions is already deleted, correctly reflecting the
		// revocation.
		resourceID, err := permissionResourceID(ctx, tx, domainID, permissionID)
		if err != nil {
			return err
		}
		for _, uid := range memberIDs {
			if err := s.computeAndUpsertMask(ctx, tx, domainID, uid, resourceID); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}
