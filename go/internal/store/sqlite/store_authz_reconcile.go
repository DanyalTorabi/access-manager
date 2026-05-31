package sqlite

import (
	"context"
)

// ReconcileUserResourceMasks rebuilds the user_resource_masks table from
// scratch inside a single transaction. It should be called once at server
// startup (after migrations) to backfill data from any pre-T04 rows and to
// repair the cache if it has drifted out of sync with the source tables.
//
// The rebuild is a full DELETE + re-insert rather than an incremental update
// because the table is a derived cache; correctness is simpler to reason about
// with a complete rebuild than with differential logic.
func (s *Store) ReconcileUserResourceMasks(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM user_resource_masks`); err != nil {
		return err
	}

	// Collect all unique (domain_id, user_id, resource_id) triples that have
	// at least one permission grant (direct via user_permissions OR via
	// group_permissions + group_members).
	const triplesSQL = `
SELECT DISTINCT p.domain_id, up.user_id, p.resource_id
FROM permissions p
INNER JOIN user_permissions up ON up.permission_id = p.id AND up.domain_id = p.domain_id
UNION
SELECT DISTINCT p.domain_id, gm.user_id, p.resource_id
FROM permissions p
INNER JOIN group_permissions gp ON gp.permission_id = p.id AND gp.domain_id = p.domain_id
INNER JOIN group_members gm ON gm.group_id = gp.group_id AND gm.domain_id = p.domain_id
`
	rows, err := tx.QueryContext(ctx, triplesSQL)
	if err != nil {
		return err
	}

	type triple struct{ domainID, userID, resourceID string }
	var triples []triple
	for rows.Next() {
		var t triple
		if err := rows.Scan(&t.domainID, &t.userID, &t.resourceID); err != nil {
			_ = rows.Close()
			return err
		}
		triples = append(triples, t)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}

	for _, t := range triples {
		if err := s.computeAndUpsertMask(ctx, tx, t.domainID, t.userID, t.resourceID); err != nil {
			return err
		}
	}

	return tx.Commit()
}
