package sqlite

import (
	"context"

	"github.com/dtorabi/access-manager/internal/access"
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

func (s *Store) EffectiveMask(ctx context.Context, domainID, userID, resourceID string) (uint64, error) {
	masks, err := s.PermissionMasksForUserResource(ctx, domainID, userID, resourceID)
	if err != nil {
		return 0, err
	}
	return access.CombineMasks(masks), nil
}
