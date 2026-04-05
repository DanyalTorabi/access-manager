package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/dtorabi/access-manager/internal/access"
	"github.com/dtorabi/access-manager/internal/store"
	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// Store implements store.Store for SQLite.
type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

func constraintCode(err error) int {
	var e *sqlite.Error
	if errors.As(err, &e) {
		return e.Code()
	}
	return 0
}

func wrapConstraintError(err error) error {
	if err == nil {
		return nil
	}
	switch constraintCode(err) {
	case sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY:
		return errors.Join(store.ErrFKViolation, err)
	case sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY, sqlite3.SQLITE_CONSTRAINT_UNIQUE:
		return errors.Join(store.ErrConflict, err)
	}
	// database/sql sometimes returns errors that do not unwrap to *sqlite.Error.
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "foreign key constraint failed") {
		return errors.Join(store.ErrFKViolation, err)
	}
	if strings.Contains(msg, "unique constraint failed") || strings.Contains(msg, "primary key constraint") {
		return errors.Join(store.ErrConflict, err)
	}
	return err
}

func maskToSQL(m uint64) int64 { return int64(m) }

func maskFromSQL(v int64) uint64 { return uint64(v) }

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func escapeLikePattern(s string) string { return likeEscaper.Replace(s) }

var (
	domainSortColumns     = map[string]string{"title": "title"}
	userSortColumns       = map[string]string{"title": "title"}
	groupSortColumns      = map[string]string{"title": "title"}
	resourceSortColumns   = map[string]string{"title": "title"}
	accessTypeSortColumns = map[string]string{"title": "title"}
	permissionSortColumns = map[string]string{"title": "title", "resource_id": "resource_id"}
)

// orderByClause returns a safe " ORDER BY <col> <dir>" clause.
// sort must already be validated against the allow-list. If the field is
// missing from allowed (defensive, or empty from direct store calls),
// it falls back to fallbackCol.
func orderByClause(sort string, order store.SortOrder, allowed map[string]string, fallbackCol string) string {
	col, ok := allowed[sort]
	if !ok {
		col = fallbackCol
	}
	dir := "ASC"
	if order == store.OrderDesc {
		dir = "DESC"
	}
	return " ORDER BY " + col + " " + dir
}

func likePattern(search string, st store.SearchType) string {
	escaped := escapeLikePattern(search)
	switch st {
	case store.SearchContains, "":
		return "%" + escaped + "%"
	case store.SearchStartsWith:
		return escaped + "%"
	case store.SearchEndsWith:
		return "%" + escaped
	default:
		// Unknown SearchType — likely a call-site typo. Log and fall back to contains.
		slog.Warn("unknown SearchType, falling back to contains", "search_type", string(st))
		return "%" + escaped + "%"
	}
}

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
		`SELECT id, title FROM domains`+where+orderByClause(opts.Sort, opts.Order, domainSortColumns, "title")+` LIMIT ? OFFSET ?`,
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
		return nil, fmt.Errorf("%w: empty patch", store.ErrInvalidInput)
	}
	if _, err := s.DomainGet(ctx, id); err != nil {
		return nil, err
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE domains SET title = ? WHERE id = ?`, *title, id); err != nil {
		return nil, wrapConstraintError(err)
	}
	return s.DomainGet(ctx, id)
}

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
		`SELECT id, domain_id, title FROM users `+where+orderByClause(opts.Sort, opts.Order, userSortColumns, "title")+` LIMIT ? OFFSET ?`,
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
		return nil, fmt.Errorf("%w: empty patch", store.ErrInvalidInput)
	}
	if _, err := s.UserGet(ctx, domainID, id); err != nil {
		return nil, err
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE users SET title = ? WHERE id = ? AND domain_id = ?`, *title, id, domainID); err != nil {
		return nil, wrapConstraintError(err)
	}
	return s.UserGet(ctx, domainID, id)
}

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
		`SELECT id, domain_id, title, parent_group_id FROM groups `+where+orderByClause(opts.Sort, opts.Order, groupSortColumns, "title")+` LIMIT ? OFFSET ?`,
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
		return fmt.Errorf("%w: group cannot be its own parent", store.ErrInvalidInput)
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
			return fmt.Errorf("%w: parent group wrong domain", store.ErrInvalidInput)
		}
		walk := *parentID
		const maxSteps = 1_000_000
		for i := 0; i < maxSteps; i++ {
			if walk == groupID {
				return fmt.Errorf("%w: cycle detected in group parent chain", store.ErrInvalidInput)
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
		return nil, fmt.Errorf("%w: empty patch", store.ErrInvalidInput)
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
		`SELECT id, domain_id, title FROM resources `+where+orderByClause(opts.Sort, opts.Order, resourceSortColumns, "title")+` LIMIT ? OFFSET ?`,
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
		return nil, fmt.Errorf("%w: empty patch", store.ErrInvalidInput)
	}
	if _, err := s.ResourceGet(ctx, domainID, id); err != nil {
		return nil, err
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE resources SET title = ? WHERE id = ? AND domain_id = ?`, *title, id, domainID); err != nil {
		return nil, wrapConstraintError(err)
	}
	return s.ResourceGet(ctx, domainID, id)
}

func (s *Store) AccessTypeCreate(ctx context.Context, a *store.AccessType) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO access_types (id, domain_id, title, bit) VALUES (?, ?, ?, ?)`,
		a.ID, a.DomainID, a.Title, maskToSQL(a.Bit))
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
		`SELECT id, domain_id, title, bit FROM access_types `+where+orderByClause(opts.Sort, opts.Order, accessTypeSortColumns, "title")+` LIMIT ? OFFSET ?`,
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
		a.Bit = maskFromSQL(bit)
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
	out.Bit = maskFromSQL(bit)
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
		return nil, fmt.Errorf("%w: empty patch", store.ErrInvalidInput)
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
	bit := maskFromSQL(curBit)
	if p.Bit != nil {
		bit = *p.Bit
	}
	if _, err := tx.ExecContext(ctx, `UPDATE access_types SET title = ?, bit = ? WHERE id = ? AND domain_id = ?`,
		title, maskToSQL(bit), id, domainID); err != nil {
		return nil, wrapConstraintError(err)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.AccessTypeGet(ctx, domainID, id)
}

func (s *Store) PermissionCreate(ctx context.Context, p *store.Permission) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO permissions (id, domain_id, title, resource_id, access_mask) VALUES (?, ?, ?, ?, ?)`,
		p.ID, p.DomainID, p.Title, p.ResourceID, maskToSQL(p.AccessMask))
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
	out.AccessMask = maskFromSQL(m)
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
		`SELECT id, domain_id, title, resource_id, access_mask FROM permissions `+where+orderByClause(opts.Sort, opts.Order, permissionSortColumns, "title")+` LIMIT ? OFFSET ?`,
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
		p.AccessMask = maskFromSQL(m)
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
		return nil, fmt.Errorf("%w: empty patch", store.ErrInvalidInput)
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
	mask := maskFromSQL(curMask)
	if p.AccessMask != nil {
		mask = *p.AccessMask
	}
	if _, err := tx.ExecContext(ctx, `UPDATE permissions SET title = ?, resource_id = ?, access_mask = ? WHERE id = ? AND domain_id = ?`,
		title, resourceID, maskToSQL(mask), id, domainID); err != nil {
		return nil, wrapConstraintError(err)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.PermissionGet(ctx, domainID, id)
}

func (s *Store) AddUserToGroup(ctx context.Context, domainID, userID, groupID string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO group_members (domain_id, user_id, group_id) VALUES (?, ?, ?)`,
		domainID, userID, groupID)
	return wrapConstraintError(err)
}

func (s *Store) RemoveUserFromGroup(ctx context.Context, domainID, userID, groupID string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM group_members WHERE domain_id = ? AND user_id = ? AND group_id = ?`,
		domainID, userID, groupID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *Store) GrantUserPermission(ctx context.Context, domainID, userID, permissionID string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO user_permissions (domain_id, user_id, permission_id) VALUES (?, ?, ?)`,
		domainID, userID, permissionID)
	return wrapConstraintError(err)
}

func (s *Store) RevokeUserPermission(ctx context.Context, domainID, userID, permissionID string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM user_permissions WHERE domain_id = ? AND user_id = ? AND permission_id = ?`,
		domainID, userID, permissionID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *Store) GrantGroupPermission(ctx context.Context, domainID, groupID, permissionID string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO group_permissions (domain_id, group_id, permission_id) VALUES (?, ?, ?)`,
		domainID, groupID, permissionID)
	return wrapConstraintError(err)
}

func (s *Store) RevokeGroupPermission(ctx context.Context, domainID, groupID, permissionID string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM group_permissions WHERE domain_id = ? AND group_id = ? AND permission_id = ?`,
		domainID, groupID, permissionID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return store.ErrNotFound
	}
	return nil
}

const effectiveMaskSQL = `
SELECT p.access_mask FROM permissions p
WHERE p.domain_id = ? AND p.resource_id = ?
AND (
	EXISTS (
		SELECT 1 FROM user_permissions up
		WHERE up.permission_id = p.id AND up.domain_id = ? AND up.user_id = ?
	)
	OR EXISTS (
		SELECT 1 FROM group_permissions gp
		INNER JOIN group_members gm ON gm.group_id = gp.group_id AND gm.user_id = ?
		WHERE gp.permission_id = p.id AND gp.domain_id = ? AND gm.domain_id = ?
	)
)
`

func (s *Store) PermissionMasksForUserResource(ctx context.Context, domainID, userID, resourceID string) ([]uint64, error) {
	rows, err := s.db.QueryContext(ctx, effectiveMaskSQL,
		domainID, resourceID,
		domainID, userID,
		userID, domainID, domainID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var masks []uint64
	for rows.Next() {
		var m int64
		if err := rows.Scan(&m); err != nil {
			return nil, err
		}
		masks = append(masks, maskFromSQL(m))
	}
	return masks, rows.Err()
}

func (s *Store) EffectiveMask(ctx context.Context, domainID, userID, resourceID string) (uint64, error) {
	masks, err := s.PermissionMasksForUserResource(ctx, domainID, userID, resourceID)
	if err != nil {
		return 0, err
	}
	return access.CombineMasks(masks), nil
}
