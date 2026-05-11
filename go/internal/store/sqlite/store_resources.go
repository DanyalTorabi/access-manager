package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/dtorabi/access-manager/internal/store"
)

func (s *Store) ResourceCreate(ctx context.Context, r *store.Resource) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO resources (id, domain_id, title) VALUES (?, ?, ?)`,
		r.ID, r.DomainID, r.Title)
	return wrapConstraintError(err)
}

func (s *Store) ResourceGet(ctx context.Context, domainID, id string) (*store.Resource, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, domain_id, title FROM resources WHERE id = ? AND domain_id = ?`, id, domainID)
	var out store.Resource
	if err := row.Scan(&out.ID, &out.DomainID, &out.Title); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, err
	}
	return &out, nil
}

func (s *Store) ResourceList(ctx context.Context, domainID string, opts store.ListOpts) ([]store.Resource, int64, error) {
	opts = store.SanitizeListOpts(opts)

	where := "WHERE domain_id = ?"
	args := []any{domainID}
	if opts.Search != "" {
		where += ` AND title LIKE ? ESCAPE '\'`
		args = append(args, likePattern(opts.Search, opts.SearchType))
	}

	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM resources `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, domain_id, title FROM resources `+where+orderByClause(opts.Sort, opts.Order, resourceSortColumns, "title")+` LIMIT ? OFFSET ?`, // #nosec G202: ORDER BY column from allow-list, not user input
		append(args, opts.Limit, opts.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	var list []store.Resource
	for rows.Next() {
		var r store.Resource
		if err := rows.Scan(&r.ID, &r.DomainID, &r.Title); err != nil {
			return nil, 0, err
		}
		list = append(list, r)
	}
	return list, total, rows.Err()
}

func (s *Store) ResourceDelete(ctx context.Context, domainID, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM resources WHERE id = ? AND domain_id = ?`, id, domainID)
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

func (s *Store) ResourcePatch(ctx context.Context, domainID, id string, title *string) (*store.Resource, error) {
	if title == nil {
		return nil, store.NewInvalidInput("empty patch")
	}
	if _, err := s.ResourceGet(ctx, domainID, id); err != nil {
		return nil, err
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE resources SET title = ? WHERE id = ? AND domain_id = ?`, *title, id, domainID); err != nil {
		return nil, wrapConstraintError(err)
	}
	return s.ResourceGet(ctx, domainID, id)
}
