package sqlite

import (
	"context"
	"database/sql"
	"errors"
)

// userEffectivePermissionPredicateSQL filters permissions p down to those
// effectively held by a given user (direct grant OR via group membership).
// T51 composite FKs guarantee that user_permissions / group_permissions /
// group_members rows cannot reference cross-domain parents, so no
// defensive domain_id filter is needed in the sub-EXISTS clauses.
const userEffectivePermissionPredicateSQL = `
AND (
	EXISTS (
		SELECT 1 FROM user_permissions up
		WHERE up.permission_id = p.id AND up.user_id = ?
	)
	OR EXISTS (
		SELECT 1 FROM group_permissions gp
		INNER JOIN group_members gm ON gm.group_id = gp.group_id AND gm.user_id = ?
		WHERE gp.permission_id = p.id
	)
)
`

const effectiveMaskSQL = `
SELECT p.access_mask FROM permissions p
WHERE p.domain_id = ? AND p.resource_id = ?
` + userEffectivePermissionPredicateSQL

// PermissionMasksForUserResource returns the raw per-permission access_mask
// values for the given (domain, user, resource) triple. It reads directly
// from the permissions + junction tables (the ground-truth query path) and
// is used by computeAndUpsertMask to populate user_resource_masks.
func (s *Store) PermissionMasksForUserResource(ctx context.Context, domainID, userID, resourceID string) ([]uint64, error) {
	args := make([]any, 0, 2+2)
	args = append(args, domainID, resourceID)
	args = append(args, userEffectivePermissionArgs(userID)...)
	rows, err := s.db.QueryContext(ctx, effectiveMaskSQL, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var masks []uint64
	for rows.Next() {
		var m int64
		if err := rows.Scan(&m); err != nil {
			return nil, err
		}
		masks = append(masks, s.maskFromSQL(m))
	}
	return masks, rows.Err()
}

// EffectiveMask reads the precomputed mask from user_resource_masks. Returns
// 0 with no error when no row exists (the user has no access to the resource).
func (s *Store) EffectiveMask(ctx context.Context, domainID, userID, resourceID string) (uint64, error) {
	var m int64
	err := s.db.QueryRowContext(ctx,
		`SELECT access_mask FROM user_resource_masks WHERE domain_id = ? AND user_id = ? AND resource_id = ?`,
		domainID, userID, resourceID,
	).Scan(&m)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	return s.maskFromSQL(m), nil
}

// computeAndUpsertMask recomputes the effective mask for (domainID, userID,
// resourceID) using the ground-truth query path and writes the result into
// user_resource_masks inside the supplied transaction. If the combined mask
// is zero the row is deleted so the table only holds non-zero entries.
// Must be called inside an active transaction after the triggering mutation
// has already been applied so the SELECT sees the post-mutation state.
func (s *Store) computeAndUpsertMask(ctx context.Context, tx *sql.Tx, domainID, userID, resourceID string) error {
	args := make([]any, 0, 4)
	args = append(args, domainID, resourceID)
	args = append(args, userEffectivePermissionArgs(userID)...)

	rows, err := tx.QueryContext(ctx, effectiveMaskSQL, args...)
	if err != nil {
		return err
	}
	var combined uint64
	for rows.Next() {
		var m int64
		if err := rows.Scan(&m); err != nil {
			_ = rows.Close()
			return err
		}
		combined |= s.maskFromSQL(m)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}

	if combined == 0 {
		_, err = tx.ExecContext(ctx,
			`DELETE FROM user_resource_masks WHERE domain_id = ? AND user_id = ? AND resource_id = ?`,
			domainID, userID, resourceID)
		return err
	}

	maskVal, err := maskToSQL(combined)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, // #nosec G202
		`INSERT INTO user_resource_masks (domain_id, user_id, resource_id, access_mask) VALUES (?, ?, ?, ?)`+
			` ON CONFLICT(domain_id, user_id, resource_id) DO UPDATE SET access_mask = excluded.access_mask`,
		domainID, userID, resourceID, maskVal)
	return err
}

// groupResourceIDs returns the distinct resource IDs covered by the group's
// direct group_permissions grants. Runs inside the supplied transaction.
func groupResourceIDs(ctx context.Context, tx *sql.Tx, domainID, groupID string) ([]string, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT DISTINCT p.resource_id FROM group_permissions gp`+
			` INNER JOIN permissions p ON p.id = gp.permission_id`+
			` WHERE gp.domain_id = ? AND gp.group_id = ? AND p.access_mask > 0`,
		domainID, groupID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// permissionResourceID returns the resource_id of the given permission.
// Runs inside the supplied transaction.
func permissionResourceID(ctx context.Context, tx *sql.Tx, domainID, permissionID string) (string, error) {
	var resourceID string
	err := tx.QueryRowContext(ctx,
		`SELECT resource_id FROM permissions WHERE id = ? AND domain_id = ?`,
		permissionID, domainID,
	).Scan(&resourceID)
	return resourceID, err
}

// groupMemberIDs returns the user IDs of all direct members in the group.
// Runs inside the supplied transaction.
func groupMemberIDs(ctx context.Context, tx *sql.Tx, domainID, groupID string) ([]string, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT user_id FROM group_members WHERE domain_id = ? AND group_id = ?`,
		domainID, groupID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
