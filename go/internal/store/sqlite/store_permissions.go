package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/dtorabi/access-manager/internal/store"
)

func (s *Store) PermissionCreate(ctx context.Context, p *store.Permission) error {
	maskVal, err := maskToSQL(p.AccessMask)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO permissions (id, domain_id, title, resource_id, access_mask) VALUES (?, ?, ?, ?, ?)`,
		p.ID, p.DomainID, p.Title, p.ResourceID, maskVal)
	return wrapConstraintError(err)
}

func (s *Store) PermissionGet(ctx context.Context, domainID, id string) (*store.Permission, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, domain_id, title, resource_id, access_mask FROM permissions WHERE id = ? AND domain_id = ?`, id, domainID)
	var out store.Permission
	var m int64
	if err := row.Scan(&out.ID, &out.DomainID, &out.Title, &out.ResourceID, &m); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, err
	}
	out.AccessMask = s.maskFromSQL(m)
	return &out, nil
}

func (s *Store) PermissionList(ctx context.Context, domainID string, opts store.PermissionListOpts) ([]store.Permission, int64, error) {
	opts.ListOpts = store.SanitizeListOpts(opts.ListOpts)

	where := "WHERE domain_id = ?"
	args := []any{domainID}
	if opts.Search != "" {
		where += ` AND title LIKE ? ESCAPE '\'`
		args = append(args, likePattern(opts.Search, opts.SearchType))
	}
	if opts.ResourceID != nil {
		where += " AND resource_id = ?"
		args = append(args, *opts.ResourceID)
	}

	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM permissions `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, domain_id, title, resource_id, access_mask FROM permissions `+where+orderByClause(opts.Sort, opts.Order, permissionSortColumns, "title")+` LIMIT ? OFFSET ?`, // #nosec G202: ORDER BY column from allow-list, not user input
		append(args, opts.Limit, opts.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	var list []store.Permission
	for rows.Next() {
		var p store.Permission
		var m int64
		if err := rows.Scan(&p.ID, &p.DomainID, &p.Title, &p.ResourceID, &m); err != nil {
			return nil, 0, err
		}
		p.AccessMask = s.maskFromSQL(m)
		list = append(list, p)
	}
	return list, total, rows.Err()
}

func (s *Store) PermissionDelete(ctx context.Context, domainID, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM permissions WHERE id = ? AND domain_id = ?`, id, domainID)
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

func (s *Store) PermissionPatch(ctx context.Context, domainID, id string, p store.PermissionPatchParams) (*store.Permission, error) {
	if p.Title == nil && p.ResourceID == nil && p.AccessMask == nil {
		return nil, store.NewInvalidInput("empty patch")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	row := tx.QueryRowContext(ctx, `SELECT title, resource_id, access_mask FROM permissions WHERE id = ? AND domain_id = ?`, id, domainID)
	var curTitle, curResourceID string
	var curMask int64
	if err := row.Scan(&curTitle, &curResourceID, &curMask); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, err
	}
	title := curTitle
	if p.Title != nil {
		title = *p.Title
	}
	resourceID := curResourceID
	if p.ResourceID != nil {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT 1 FROM resources WHERE id = ? AND domain_id = ?`, *p.ResourceID, domainID).Scan(&exists); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, store.ErrNotFound
			}
			return nil, err
		}
		resourceID = *p.ResourceID
	}
	mask := s.maskFromSQL(curMask)
	if p.AccessMask != nil {
		mask = *p.AccessMask
	}
	maskVal, err := maskToSQL(mask)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE permissions SET title = ?, resource_id = ?, access_mask = ? WHERE id = ? AND domain_id = ?`,
		title, resourceID, maskVal, id, domainID); err != nil {
		return nil, wrapConstraintError(err)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.PermissionGet(ctx, domainID, id)
}
