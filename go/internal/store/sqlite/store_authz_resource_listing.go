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
		`SELECT gp.group_id, p.access_mask FROM permissions p `+ // #nosec G202
			`INNER JOIN group_permissions gp ON gp.permission_id = p.id `+
			`INNER JOIN groups g ON g.id = gp.group_id `+
			`WHERE p.domain_id = ? AND p.resource_id = ? AND p.access_mask > 0 `+
			`AND gp.group_id `,
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

// resourceAuthzUsersBaseSQL selects users in the resource's domain who have a
// non-zero effective mask on (domainID, resourceID) via direct grants OR via
// any group they belong to.
//
// `p.access_mask > 0` excludes both zero masks (no-op grants) AND any legacy
// rows with negative int64 mask values — see maskFromSQL, which similarly
// coerces negative DB values to 0 with a warning. Such rows can only exist
// from out-of-band/legacy writes (PermissionCreate validates the range), so
// silently ignoring them in the listing is intentional and matches T42/T43.
//
// Placeholder map (three ?'s, built from {domainID, resourceID}; keep this
// table in sync with resourceAuthzUsersBaseArgs):
//
//	1: u.domain_id   = domainID
//	2: p.domain_id   = domainID
//	3: p.resource_id = resourceID
//
// Example with domainID="D" / resourceID="R": args are ["D","D","R"].
//
// T51 composite FKs guarantee that user_permissions / group_permissions /
// group_members rows cannot reference cross-domain parents, so no
// defensive up.domain_id / gp.domain_id / gm.domain_id filter is needed
// in the sub-EXISTS clauses.
const resourceAuthzUsersBaseSQL = `
FROM users u
WHERE u.domain_id = ? AND EXISTS (
	SELECT 1 FROM permissions p
	WHERE p.domain_id = ? AND p.resource_id = ? AND p.access_mask > 0
	AND (
		EXISTS (
			SELECT 1 FROM user_permissions up
			WHERE up.permission_id = p.id AND up.user_id = u.id
		)
		OR EXISTS (
			SELECT 1 FROM group_permissions gp
			INNER JOIN group_members gm ON gm.group_id = gp.group_id AND gm.user_id = u.id
			WHERE gp.permission_id = p.id
		)
	)
)
`

// resourceAuthzUsersBaseArgs returns the three positional args for
// resourceAuthzUsersBaseSQL in placeholder order. Centralised so callers
// (count + page-select) cannot drift out of sync with the SQL.
func resourceAuthzUsersBaseArgs(domainID, resourceID string) []any {
	return []any{domainID, domainID, resourceID}
}

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

	baseArgs := resourceAuthzUsersBaseArgs(domainID, resourceID)

	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) `+resourceAuthzUsersBaseSQL, baseArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// opts.Sort / opts.Order are populated by the handler and reflected in the
	// meta response via writeList. The store always uses a fixed ORDER BY
	// u.id ASC for stable, deterministic pagination — opts.Sort/Order are
	// intentionally NOT honoured here. The handler exposes the meta label
	// "user_id" which is the public name for the same users.id column.
	listArgs := append(append([]any{}, baseArgs...), opts.Limit, opts.Offset)
	rows, err := s.db.QueryContext(ctx,
		`SELECT u.id `+resourceAuthzUsersBaseSQL+` ORDER BY u.id ASC LIMIT ? OFFSET ?`,
		listArgs...,
	)
	if err != nil {
		return nil, 0, err
	}

	var userIDs []string
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			_ = rows.Close()
			return nil, 0, err
		}
		userIDs = append(userIDs, uid)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, 0, err
	}
	if err := rows.Close(); err != nil {
		return nil, 0, err
	}
	if len(userIDs) == 0 {
		return []store.ResourceAuthzUser{}, total, nil
	}

	// Invariant: len(userIDs) <= opts.Limit which is clamped to store.MaxLimit
	// (100) by SanitizeListOpts. SQLite's default SQLITE_MAX_VARIABLE_NUMBER
	// is well above this (>=999), so the IN (?,…) expansions below are safe.
	// If MaxLimit is ever raised above the SQLite parameter cap, batch the
	// IN clauses or chunk userIDs.
	masksByUser := make(map[string]uint64, len(userIDs))

	// Direct user grants on this resource.
	directSQL, directArgs, err := buildInQueryAndArgs(
		`SELECT up.user_id, p.access_mask FROM user_permissions up `+ // #nosec G202
			`INNER JOIN permissions p ON p.id = up.permission_id `+
			`WHERE p.domain_id = ? AND p.resource_id = ? AND p.access_mask > 0 `+
			`AND up.user_id `,
		[]any{domainID, resourceID},
		userIDs,
	)
	if err != nil {
		return nil, 0, err
	}
	if err := scanUserMasks(ctx, s, directSQL, directArgs, masksByUser); err != nil {
		return nil, 0, err
	}

	// Indirect grants via group membership.
	indirectSQL, indirectArgs, err := buildInQueryAndArgs(
		`SELECT gm.user_id, p.access_mask FROM group_members gm `+ // #nosec G202
			`INNER JOIN group_permissions gp ON gp.group_id = gm.group_id `+
			`INNER JOIN permissions p ON p.id = gp.permission_id `+
			`WHERE p.domain_id = ? AND p.resource_id = ? AND p.access_mask > 0 `+
			`AND gm.user_id `,
		[]any{domainID, resourceID},
		userIDs,
	)
	if err != nil {
		return nil, 0, err
	}
	if err := scanUserMasks(ctx, s, indirectSQL, indirectArgs, masksByUser); err != nil {
		return nil, 0, err
	}

	result := make([]store.ResourceAuthzUser, 0, len(userIDs))
	for _, uid := range userIDs {
		result = append(result, store.ResourceAuthzUser{UserID: uid, EffectiveMask: masksByUser[uid]})
	}
	return result, total, nil
}

func scanUserMasks(ctx context.Context, s *Store, query string, args []any, into map[string]uint64) error {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var uid string
		var m int64
		if err := rows.Scan(&uid, &m); err != nil {
			return err
		}
		into[uid] |= s.maskFromSQL(m)
	}
	return rows.Err()
}
