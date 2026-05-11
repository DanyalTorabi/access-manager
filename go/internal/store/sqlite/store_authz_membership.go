package sqlite

import (
	"context"

	"github.com/dtorabi/access-manager/internal/store"
)

func (s *Store) AddUserToGroup(ctx context.Context, domainID, userID, groupID string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO group_members (domain_id, user_id, group_id) VALUES (?, ?, ?)`,
		domainID, userID, groupID)
	return wrapConstraintError(err)
}

func (s *Store) RemoveUserFromGroup(ctx context.Context, domainID, userID, groupID string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM group_members WHERE domain_id = ? AND user_id = ? AND group_id = ?`,
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
	return nil
}

func (s *Store) GrantUserPermission(ctx context.Context, domainID, userID, permissionID string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO user_permissions (domain_id, user_id, permission_id) VALUES (?, ?, ?)`,
		domainID, userID, permissionID)
	return wrapConstraintError(err)
}

func (s *Store) RevokeUserPermission(ctx context.Context, domainID, userID, permissionID string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM user_permissions WHERE domain_id = ? AND user_id = ? AND permission_id = ?`,
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
	return nil
}

func (s *Store) GrantGroupPermission(ctx context.Context, domainID, groupID, permissionID string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO group_permissions (domain_id, group_id, permission_id) VALUES (?, ?, ?)`,
		domainID, groupID, permissionID)
	return wrapConstraintError(err)
}

func (s *Store) RevokeGroupPermission(ctx context.Context, domainID, groupID, permissionID string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM group_permissions WHERE domain_id = ? AND group_id = ? AND permission_id = ?`,
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
	return nil
}
