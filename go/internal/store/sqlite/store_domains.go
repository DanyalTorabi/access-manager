package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/dtorabi/access-manager/internal/store"
)

func (s *Store) DomainCreate(ctx context.Context, d *store.Domain) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO domains (id, title) VALUES (?, ?)`, d.ID, d.Title)
	return wrapConstraintError(err)
}

func (s *Store) DomainGet(ctx context.Context, id string) (*store.Domain, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, title FROM domains WHERE id = ?`, id)
	var out store.Domain
	if err := row.Scan(&out.ID, &out.Title); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, err
	}
	return &out, nil
}

func (s *Store) DomainList(ctx context.Context, opts store.ListOpts) ([]store.Domain, int64, error) {
	opts = store.SanitizeListOpts(opts)

	where := ""
	var args []any
	if opts.Search != "" {
		where = ` WHERE title LIKE ? ESCAPE '\'`
		args = append(args, likePattern(opts.Search, opts.SearchType))
	}

	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM domains`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, title FROM domains`+where+orderByClause(opts.Sort, opts.Order, domainSortColumns, "title")+` LIMIT ? OFFSET ?`, // #nosec G202: ORDER BY column from allow-list, not user input
		append(args, opts.Limit, opts.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	var list []store.Domain
	for rows.Next() {
		var d store.Domain
		if err := rows.Scan(&d.ID, &d.Title); err != nil {
			return nil, 0, err
		}
		list = append(list, d)
	}
	return list, total, rows.Err()
}

func (s *Store) DomainDelete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM domains WHERE id = ?`, id)
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

func (s *Store) DomainPatch(ctx context.Context, id string, title *string) (*store.Domain, error) {
	if title == nil {
		return nil, store.NewInvalidInput("empty patch")
	}
	if _, err := s.DomainGet(ctx, id); err != nil {
		return nil, err
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE domains SET title = ? WHERE id = ?`, *title, id); err != nil {
		return nil, wrapConstraintError(err)
	}
	return s.DomainGet(ctx, id)
}
