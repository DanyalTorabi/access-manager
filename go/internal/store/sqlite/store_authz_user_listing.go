package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/dtorabi/access-manager/internal/store"
)

func userEffectivePermissionArgs(userID string) []any {
	return []any{userID, userID}
}

// inPlaceholders returns n comma-separated '?' placeholders for use in
// SQL IN clauses. n must be > 0.
func inPlaceholders(n int) (string, error) {
	if n <= 0 {
		return "", fmt.Errorf("in placeholders: n must be > 0")
	}
	return strings.TrimSuffix(strings.Repeat("?,", n), ","), nil
}

// buildUserAuthzMaskQueryAndArgs builds the batched mask query that returns
// per-permission access_mask values for a user across a set of resources
// (direct grants + group membership). Used by property tests to compare
// against the materialized cache.
func buildUserAuthzMaskQueryAndArgs(domainID string, resourceIDs []string, predicateArgs []any) (string, []any, error) {
	baseSQL := `SELECT p.resource_id, p.access_mask FROM permissions p WHERE p.domain_id = ? AND p.access_mask > 0`
	baseArgs := []any{domainID}
	query, args, err := buildInQueryAndArgs(baseSQL, "p.resource_id", baseArgs, resourceIDs)
	if err != nil {
		return "", nil, err
	}
	query += userEffectivePermissionPredicateSQL
	args = append(args, predicateArgs...)
	return query, args, nil
}

func (s *Store) UserAuthzResourcesList(ctx context.Context, domainID, userID string, opts store.ListOpts) ([]store.UserAuthzResource, int64, error) {
	opts = store.SanitizeListOpts(opts)

	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT 1 FROM domains WHERE id = ?`, domainID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, 0, store.ErrNotFound
		}
		return nil, 0, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT 1 FROM users WHERE id = ? AND domain_id = ?`, userID, domainID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, 0, store.ErrNotFound
		}
		return nil, 0, err
	}

	var total int64
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM user_resource_masks WHERE domain_id = ? AND user_id = ? AND access_mask != 0`,
		domainID, userID,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT resource_id, access_mask FROM user_resource_masks`+
			` WHERE domain_id = ? AND user_id = ? AND access_mask != 0`+
			` ORDER BY resource_id ASC LIMIT ? OFFSET ?`,
		domainID, userID, opts.Limit, opts.Offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	var list []store.UserAuthzResource
	for rows.Next() {
		var resourceID string
		var m int64
		if err := rows.Scan(&resourceID, &m); err != nil {
			return nil, 0, err
		}
		list = append(list, store.UserAuthzResource{ResourceID: resourceID, EffectiveMask: s.maskFromSQL(m)})
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if list == nil {
		list = []store.UserAuthzResource{}
	}
	return list, total, nil
}
