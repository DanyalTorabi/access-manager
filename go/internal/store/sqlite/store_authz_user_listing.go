package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/dtorabi/access-manager/internal/store"
)

// userAuthzResourcesBaseSQL selects resources where the user has a non-
// zero effective mask via direct grants OR group membership. T51 composite
// FKs enforce cross-domain isolation at the schema level, so no
// defensive domain_id filters are layered on top of the join.
const userAuthzResourcesBaseSQL = `
FROM permissions p
WHERE p.domain_id = ? AND p.access_mask > 0
` + userEffectivePermissionPredicateSQL

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

// buildUserAuthzMaskQueryAndArgs builds the batched mask query used by
// UserAuthzResourcesList and returns the SQL and args in the exact placeholder
// order to avoid call-site mistakes.
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

	predicateArgs := userEffectivePermissionArgs(userID)
	countArgs := append([]any{domainID}, predicateArgs...)
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT p.resource_id) `+userAuthzResourcesBaseSQL, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	listArgs := append([]any{domainID}, predicateArgs...)
	listArgs = append(listArgs, opts.Limit, opts.Offset)
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT p.resource_id `+userAuthzResourcesBaseSQL+` ORDER BY p.resource_id ASC LIMIT ? OFFSET ?`,
		listArgs...,
	)
	if err != nil {
		return nil, 0, err
	}

	var resourceIDs []string
	for rows.Next() {
		var resourceID string
		if err := rows.Scan(&resourceID); err != nil {
			_ = rows.Close()
			return nil, 0, err
		}
		resourceIDs = append(resourceIDs, resourceID)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, 0, err
	}
	if err := rows.Close(); err != nil {
		return nil, 0, err
	}
	if len(resourceIDs) == 0 {
		return []store.UserAuthzResource{}, total, nil
	}

	maskSQL, maskArgs, err := buildUserAuthzMaskQueryAndArgs(domainID, resourceIDs, predicateArgs)
	if err != nil {
		return nil, 0, err
	}

	maskRows, err := s.db.QueryContext(ctx, maskSQL, maskArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = maskRows.Close() }()

	masksByResource := make(map[string]uint64, len(resourceIDs))
	for maskRows.Next() {
		var resourceID string
		var m int64
		if err := maskRows.Scan(&resourceID, &m); err != nil {
			return nil, 0, err
		}
		masksByResource[resourceID] |= s.maskFromSQL(m)
	}
	if err := maskRows.Err(); err != nil {
		return nil, 0, err
	}

	list := make([]store.UserAuthzResource, 0, len(resourceIDs))
	for _, resourceID := range resourceIDs {
		mask := masksByResource[resourceID]
		list = append(list, store.UserAuthzResource{ResourceID: resourceID, EffectiveMask: mask})
	}
	return list, total, nil
}
