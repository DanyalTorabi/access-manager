package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/dtorabi/access-manager/internal/store"
)

func (s *Store) GroupCreate(ctx context.Context, g *store.Group) error {
	var parent any
	if g.ParentGroupID != nil {
		parent = *g.ParentGroupID
	} else {
		parent = nil
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO groups (id, domain_id, title, parent_group_id) VALUES (?, ?, ?, ?)`,
		g.ID, g.DomainID, g.Title, parent)
	return wrapConstraintError(err)
}

func (s *Store) GroupGet(ctx context.Context, domainID, id string) (*store.Group, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, domain_id, title, parent_group_id FROM groups WHERE id = ? AND domain_id = ?`, id, domainID)
	var out store.Group
	var parent sql.NullString
	if err := row.Scan(&out.ID, &out.DomainID, &out.Title, &parent); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, err
	}
	if parent.Valid {
		out.ParentGroupID = &parent.String
	}
	return &out, nil
}

func (s *Store) GroupList(ctx context.Context, domainID string, opts store.GroupListOpts) ([]store.Group, int64, error) {
	opts.ListOpts = store.SanitizeListOpts(opts.ListOpts)

	where := "WHERE domain_id = ?"
	args := []any{domainID}
	if opts.Search != "" {
		where += ` AND title LIKE ? ESCAPE '\'`
		args = append(args, likePattern(opts.Search, opts.SearchType))
	}
	if opts.ParentGroupID != nil {
		where += " AND parent_group_id = ?"
		args = append(args, *opts.ParentGroupID)
	}

	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM groups `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, domain_id, title, parent_group_id FROM groups `+where+orderByClause(opts.Sort, opts.Order, groupSortColumns, "title")+` LIMIT ? OFFSET ?`, // #nosec G202: ORDER BY column from allow-list, not user input
		append(args, opts.Limit, opts.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	var list []store.Group
	for rows.Next() {
		var g store.Group
		var parent sql.NullString
		if err := rows.Scan(&g.ID, &g.DomainID, &g.Title, &parent); err != nil {
			return nil, 0, err
		}
		if parent.Valid {
			g.ParentGroupID = &parent.String
		}
		list = append(list, g)
	}
	return list, total, rows.Err()
}

func groupGetTx(ctx context.Context, tx *sql.Tx, domainID, id string) (*store.Group, error) {
	row := tx.QueryRowContext(ctx, `SELECT id, domain_id, title, parent_group_id FROM groups WHERE id = ? AND domain_id = ?`, id, domainID)
	var out store.Group
	var parent sql.NullString
	if err := row.Scan(&out.ID, &out.DomainID, &out.Title, &parent); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, err
	}
	if parent.Valid {
		out.ParentGroupID = &parent.String
	}
	return &out, nil
}

func groupSetParentTx(ctx context.Context, tx *sql.Tx, domainID, groupID string, parentID *string) error {
	if parentID != nil && *parentID == groupID {
		return store.NewInvalidInput("group cannot be its own parent")
	}
	if _, err := groupGetTx(ctx, tx, domainID, groupID); err != nil {
		return err
	}
	if parentID != nil {
		p, err := groupGetTx(ctx, tx, domainID, *parentID)
		if err != nil {
			return err
		}
		if p.DomainID != domainID {
			return store.NewInvalidInput("parent group wrong domain")
		}
		walk := *parentID
		const maxSteps = 1_000_000
		for i := 0; i < maxSteps; i++ {
			if walk == groupID {
				return store.NewInvalidInput("cycle detected in group parent chain")
			}
			pg, err := groupGetTx(ctx, tx, domainID, walk)
			if err != nil {
				return err
			}
			if pg.ParentGroupID == nil {
				break
			}
			walk = *pg.ParentGroupID
		}
	}
	var parent any
	if parentID != nil {
		parent = *parentID
	}
	_, err := tx.ExecContext(ctx, `UPDATE groups SET parent_group_id = ? WHERE id = ? AND domain_id = ?`, parent, groupID, domainID)
	return wrapConstraintError(err)
}

func (s *Store) GroupSetParent(ctx context.Context, domainID, groupID string, parentID *string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := groupSetParentTx(ctx, tx, domainID, groupID, parentID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) GroupDelete(ctx context.Context, domainID, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM groups WHERE id = ? AND domain_id = ?`, id, domainID)
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

func (s *Store) GroupPatch(ctx context.Context, domainID, groupID string, p store.GroupPatchParams) (*store.Group, error) {
	if p.Title == nil && !p.UpdateParent {
		return nil, store.NewInvalidInput("empty patch")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := groupGetTx(ctx, tx, domainID, groupID); err != nil {
		return nil, err
	}
	if p.Title != nil {
		if _, err := tx.ExecContext(ctx, `UPDATE groups SET title = ? WHERE id = ? AND domain_id = ?`, *p.Title, groupID, domainID); err != nil {
			return nil, wrapConstraintError(err)
		}
	}
	if p.UpdateParent {
		if err := groupSetParentTx(ctx, tx, domainID, groupID, p.ParentGroupID); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GroupGet(ctx, domainID, groupID)
}
