package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/dtorabi/access-manager/internal/store"
)

// resourceAuthzGroupsBaseSQL joins permissions with group_permissions and
// the groups table to select groups holding at least one direct
// group_permissions grant on (domainID, resourceID).
//
// T51 composite FKs ((group_id, domain_id) -> groups(id, domain_id) and
// (permission_id, domain_id) -> permissions(id, domain_id)) enforce
// cross-domain isolation at the schema layer, so no defensive
// gp.domain_id / g.domain_id filters are needed on the join.
//
// p.access_mask > 0 mirrors GroupAuthzResourcesList and
// ResourceAuthzUsersList: zero masks are no-ops, and any negative legacy
// values (which PermissionCreate disallows) are excluded for parity with
// maskFromSQL.
const resourceAuthzGroupsBaseSQL = `
FROM permissions p
INNER JOIN group_permissions gp ON gp.permission_id = p.id
INNER JOIN groups g ON g.id = gp.group_id
WHERE p.domain_id = ? AND p.resource_id = ? AND p.access_mask > 0
`

func (s *Store) ResourceAuthzGroupsList(ctx context.Context, domainID, resourceID string, opts store.ListOpts) ([]store.ResourceAuthzGroup, int64, error) {
	opts = store.SanitizeListOpts(opts)

	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT 1 FROM domains WHERE id = ?`, domainID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, 0, store.ErrNotFound
		}
		return nil, 0, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT 1 FROM resources WHERE id = ? AND domain_id = ?`, resourceID, domainID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, 0, store.ErrNotFound
		}
		return nil, 0, err
	}

	baseArgs := []any{domainID, resourceID}

	// COUNT and the page SELECT below are issued as separate statements,
	// not wrapped in a read transaction. Under concurrent writes the page
	// and total may briefly disagree (a row counted here may be deleted
	// before the page query, or vice versa). This is acceptable for a
	// listing endpoint; if strict consistency is ever required, both
	// queries should run inside a single read transaction.
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT gp.group_id) `+resourceAuthzGroupsBaseSQL, baseArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// opts.Sort / opts.Order are populated by the handler and reflected in the
	// meta response via writeList. The store always uses a fixed ORDER BY
	// gp.group_id ASC — Sort/Order opts are not honoured here because the
	// endpoint intentionally exposes only stable deterministic ordering.
	listArgs := append(append([]any{}, baseArgs...), opts.Limit, opts.Offset)
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT gp.group_id `+resourceAuthzGroupsBaseSQL+` ORDER BY gp.group_id ASC LIMIT ? OFFSET ?`, // #nosec G202
		listArgs...,
	)
	if err != nil {
		return nil, 0, err
	}

	var groupIDs []string
	for rows.Next() {
		var gid string
		if err := rows.Scan(&gid); err != nil {
			_ = rows.Close()
			return nil, 0, err
		}
		groupIDs = append(groupIDs, gid)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, 0, err
	}
	if err := rows.Close(); err != nil {
		return nil, 0, err
	}
	if len(groupIDs) == 0 {
		return []store.ResourceAuthzGroup{}, total, nil
	}

	maskSQL, maskArgs, err := buildInQueryAndArgs(
		`SELECT gp.group_id, p.access_mask FROM permissions p`+
			` INNER JOIN group_permissions gp ON gp.permission_id = p.id`+
			` INNER JOIN groups g ON g.id = gp.group_id`+
			` WHERE p.domain_id = ? AND p.resource_id = ? AND p.access_mask > 0`,
		"gp.group_id",
		[]any{domainID, resourceID},
		groupIDs,
	)
	if err != nil {
		return nil, 0, err
	}

	maskRows, err := s.db.QueryContext(ctx, maskSQL, maskArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = maskRows.Close() }()

	masksByGroup := make(map[string]uint64, len(groupIDs))
	for maskRows.Next() {
		var gid string
		var m int64
		if err := maskRows.Scan(&gid, &m); err != nil {
			return nil, 0, err
		}
		masksByGroup[gid] |= s.maskFromSQL(m)
	}
	if err := maskRows.Err(); err != nil {
		return nil, 0, err
	}

	result := make([]store.ResourceAuthzGroup, 0, len(groupIDs))
	for _, gid := range groupIDs {
		result = append(result, store.ResourceAuthzGroup{GroupID: gid, Mask: masksByGroup[gid]})
	}
	return result, total, nil
}

// Note: resourceAuthzUsersBaseSQL and resourceAuthzUsersBaseArgs are test
// helpers for verifying ground-truth user lookups against the materialized
// cache; they live in store_authz_resource_users_test.go.

func (s *Store) ResourceAuthzUsersList(ctx context.Context, domainID, resourceID string, opts store.ListOpts) ([]store.ResourceAuthzUser, int64, error) {
	opts = store.SanitizeListOpts(opts)

	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT 1 FROM domains WHERE id = ?`, domainID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, 0, store.ErrNotFound
		}
		return nil, 0, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT 1 FROM resources WHERE id = ? AND domain_id = ?`, resourceID, domainID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, 0, store.ErrNotFound
		}
		return nil, 0, err
	}

	var total int64
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM user_resource_masks WHERE domain_id = ? AND resource_id = ?`,
		domainID, resourceID,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT user_id, access_mask FROM user_resource_masks`+
			` WHERE domain_id = ? AND resource_id = ?`+
			` ORDER BY user_id ASC LIMIT ? OFFSET ?`,
		domainID, resourceID, opts.Limit, opts.Offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	var result []store.ResourceAuthzUser
	for rows.Next() {
		var uid string
		var m int64
		if err := rows.Scan(&uid, &m); err != nil {
			return nil, 0, err
		}
		result = append(result, store.ResourceAuthzUser{UserID: uid, EffectiveMask: s.maskFromSQL(m)})
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if result == nil {
		result = []store.ResourceAuthzUser{}
	}
	return result, total, nil
}
