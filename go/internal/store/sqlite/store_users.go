package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/dtorabi/access-manager/internal/store"
)

func (s *Store) UserCreate(ctx context.Context, u *store.User) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO users (id, domain_id, title) VALUES (?, ?, ?)`,
		u.ID, u.DomainID, u.Title)
	return wrapConstraintError(err)
}

func (s *Store) UserGet(ctx context.Context, domainID, id string) (*store.User, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, domain_id, title FROM users WHERE id = ? AND domain_id = ?`, id, domainID)
	var out store.User
	if err := row.Scan(&out.ID, &out.DomainID, &out.Title); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, err
	}
	return &out, nil
}

func (s *Store) UserList(ctx context.Context, domainID string, opts store.ListOpts) ([]store.User, int64, error) {
	opts = store.SanitizeListOpts(opts)

	where := "WHERE domain_id = ?"
	args := []any{domainID}
	if opts.Search != "" {
		where += ` AND title LIKE ? ESCAPE '\'`
		args = append(args, likePattern(opts.Search, opts.SearchType))
	}

	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, domain_id, title FROM users `+where+orderByClause(opts.Sort, opts.Order, userSortColumns, "title")+` LIMIT ? OFFSET ?`, // #nosec G202: ORDER BY column from allow-list, not user input
		append(args, opts.Limit, opts.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	var list []store.User
	for rows.Next() {
		var u store.User
		if err := rows.Scan(&u.ID, &u.DomainID, &u.Title); err != nil {
			return nil, 0, err
		}
		list = append(list, u)
	}
	return list, total, rows.Err()
}

func (s *Store) UserDelete(ctx context.Context, domainID, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = ? AND domain_id = ?`, id, domainID)
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

func (s *Store) UserPatch(ctx context.Context, domainID, id string, title *string) (*store.User, error) {
	if title == nil {
		return nil, store.NewInvalidInput("empty patch")
	}
	if _, err := s.UserGet(ctx, domainID, id); err != nil {
		return nil, err
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE users SET title = ? WHERE id = ? AND domain_id = ?`, *title, id, domainID); err != nil {
		return nil, wrapConstraintError(err)
	}
	return s.UserGet(ctx, domainID, id)
}
