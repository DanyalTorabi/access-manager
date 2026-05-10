package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/dtorabi/access-manager/internal/store"
)

// groupAuthzResourcesBaseSQL joins permissions with group_permissions.
// p.domain_id is the primary scope; T51 composite FKs guarantee that
// matching group_permissions rows share the same domain, so no
// gp.domain_id filter is needed.
const groupAuthzResourcesBaseSQL = `
FROM permissions p
INNER JOIN group_permissions gp ON gp.permission_id = p.id
WHERE p.domain_id = ? AND gp.group_id = ? AND p.access_mask > 0
`

func (s *Store) GroupAuthzResourcesList(ctx context.Context, domainID, groupID string, opts store.ListOpts) ([]store.GroupAuthzResource, int64, error) {
	opts = store.SanitizeListOpts(opts)

	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT 1 FROM domains WHERE id = ?`, domainID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, 0, store.ErrNotFound
		}
		return nil, 0, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT 1 FROM groups WHERE id = ? AND domain_id = ?`, groupID, domainID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, 0, store.ErrNotFound
		}
		return nil, 0, err
	}

	baseArgs := []any{domainID, groupID}

	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT p.resource_id) `+groupAuthzResourcesBaseSQL, baseArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// opts.Sort / opts.Order are populated by the handler and reflected in the
	// meta response via writeList. The store always uses a fixed ORDER BY
	// p.resource_id ASC — Sort/Order opts are not honoured here because the
	// endpoint intentionally exposes only stable deterministic ordering.
	listArgs := append(append([]any{}, baseArgs...), opts.Limit, opts.Offset)
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT p.resource_id `+groupAuthzResourcesBaseSQL+` ORDER BY p.resource_id ASC LIMIT ? OFFSET ?`, // #nosec G202
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
		return []store.GroupAuthzResource{}, total, nil
	}

	placeholders, err := inPlaceholders(len(resourceIDs))
	if err != nil {
		return nil, 0, err
	}
	maskSQL := `SELECT p.resource_id, p.access_mask FROM permissions p ` + // #nosec G202
		`INNER JOIN group_permissions gp ON gp.permission_id = p.id ` +
		`WHERE p.domain_id = ? AND gp.group_id = ? AND p.resource_id IN (` + placeholders + `) AND p.access_mask > 0`
	maskArgs := make([]any, 0, 2+len(resourceIDs))
	maskArgs = append(maskArgs, domainID, groupID)
	for _, rid := range resourceIDs {
		maskArgs = append(maskArgs, rid)
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

	result := make([]store.GroupAuthzResource, 0, len(resourceIDs))
	for _, resourceID := range resourceIDs {
		mask := masksByResource[resourceID]
		result = append(result, store.GroupAuthzResource{ResourceID: resourceID, Mask: mask})
	}
	return result, total, nil
}
