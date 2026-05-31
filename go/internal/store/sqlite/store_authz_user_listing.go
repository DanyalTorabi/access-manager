package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/dtorabi/access-manager/internal/store"
)

// Note: buildUserAuthzMaskQueryAndArgs lives in store_authz_user_listing_test.go
// — it is a test helper for verifying ground-truth masks against the materialized
// cache and has no production callers.

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
	// COUNT and the page SELECT below are issued as separate statements,
	// not wrapped in a read transaction. Under concurrent writes the page
	// and total may briefly disagree (a row counted here may be deleted
	// before the page query, or vice versa). This is acceptable for a
	// listing endpoint; if strict consistency is ever required, both
	// queries should run inside a single read transaction.
	//
	// user_resource_masks only holds non-zero mask rows (computeAndUpsertMask
	// deletes the row when the combined mask is zero), so no access_mask filter
	// is required here.
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM user_resource_masks WHERE domain_id = ? AND user_id = ?`,
		domainID, userID,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT resource_id, access_mask FROM user_resource_masks`+
			` WHERE domain_id = ? AND user_id = ?`+
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
