package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/dtorabi/access-manager/internal/store"
)

func (s *Store) AccessTypeCreate(ctx context.Context, a *store.AccessType) error {
	bitVal, err := maskToSQL(a.Bit)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO access_types (id, domain_id, title, bit) VALUES (?, ?, ?, ?)`,
		a.ID, a.DomainID, a.Title, bitVal)
	return wrapConstraintError(err)
}

func (s *Store) AccessTypeList(ctx context.Context, domainID string, opts store.ListOpts) ([]store.AccessType, int64, error) {
	opts = store.SanitizeListOpts(opts)

	where := "WHERE domain_id = ?"
	args := []any{domainID}
	if opts.Search != "" {
		where += ` AND title LIKE ? ESCAPE '\'`
		args = append(args, likePattern(opts.Search, opts.SearchType))
	}

	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM access_types `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, domain_id, title, bit FROM access_types `+where+orderByClause(opts.Sort, opts.Order, accessTypeSortColumns, "title")+` LIMIT ? OFFSET ?`, // #nosec G202: ORDER BY column from allow-list, not user input
		append(args, opts.Limit, opts.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	var list []store.AccessType
	for rows.Next() {
		var a store.AccessType
		var bit int64
		if err := rows.Scan(&a.ID, &a.DomainID, &a.Title, &bit); err != nil {
			return nil, 0, err
		}
		a.Bit = s.maskFromSQL(bit)
		list = append(list, a)
	}
	return list, total, rows.Err()
}

func (s *Store) AccessTypeGet(ctx context.Context, domainID, id string) (*store.AccessType, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, domain_id, title, bit FROM access_types WHERE id = ? AND domain_id = ?`, id, domainID)
	var out store.AccessType
	var bit int64
	if err := row.Scan(&out.ID, &out.DomainID, &out.Title, &bit); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, err
	}
	out.Bit = s.maskFromSQL(bit)
	return &out, nil
}

func (s *Store) AccessTypeDelete(ctx context.Context, domainID, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM access_types WHERE id = ? AND domain_id = ?`, id, domainID)
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

func (s *Store) AccessTypePatch(ctx context.Context, domainID, id string, p store.AccessTypePatchParams) (*store.AccessType, error) {
	if p.Title == nil && p.Bit == nil {
		return nil, store.NewInvalidInput("empty patch")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	row := tx.QueryRowContext(ctx, `SELECT title, bit FROM access_types WHERE id = ? AND domain_id = ?`, id, domainID)
	var curTitle string
	var curBit int64
	if err := row.Scan(&curTitle, &curBit); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, err
	}
	title := curTitle
	if p.Title != nil {
		title = *p.Title
	}
	bit := s.maskFromSQL(curBit)
	if p.Bit != nil {
		bit = *p.Bit
	}
	bitVal, err := maskToSQL(bit)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE access_types SET title = ?, bit = ? WHERE id = ? AND domain_id = ?`,
		title, bitVal, id, domainID); err != nil {
		return nil, wrapConstraintError(err)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.AccessTypeGet(ctx, domainID, id)
}
