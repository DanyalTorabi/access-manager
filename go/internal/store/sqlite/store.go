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

var _ store.Store = (*Store)(nil)

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

const maxInt64 = 1<<63 - 1

// maskToSQL converts a uint64 mask into a signed int64 suitable for SQLite
// storage. Returns an error if the value cannot be represented in signed
// 64-bit (i.e. uses bit 63). Callers should validate input and return a
// client-facing validation error when necessary.
func maskToSQL(m uint64) (int64, error) {
	if m > uint64(maxInt64) {
		return 0, store.NewInvalidInput(store.InvalidInputDetailMaskOverflow)
	}
	return int64(m), nil
}

// maskFromSQL converts an int64 value read from SQLite into uint64. If a
// negative value is encountered, log a warning and treat it as zero to avoid
// propagating unexpected large unsigned values.
func maskFromSQL(v int64) uint64 {
	if v < 0 {
		slog.Warn("negative mask value read from DB; treating as 0", "value", v)
		return 0
	}
	return uint64(v)
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func escapeLikePattern(s string) string { return likeEscaper.Replace(s) }

// sortColumns builds a field→column map from the store's allowed sort fields.
// By default field name == column name; pass overrides for any allowed fields
// whose column names differ. Override keys not present in fields are ignored.
func sortColumns(fields []string, overrides map[string]string) map[string]string {
	cols := make(map[string]string, len(fields))
	for _, f := range fields {
		cols[f] = f
	}
	for f, col := range overrides {
		if _, ok := cols[f]; ok {
			cols[f] = col
		}
	}
	return cols
}

var (
	domainSortColumns     = sortColumns(store.DomainSortFields, nil)
	userSortColumns       = sortColumns(store.UserSortFields, nil)
	groupSortColumns      = sortColumns(store.GroupSortFields, nil)
	resourceSortColumns   = sortColumns(store.ResourceSortFields, nil)
	accessTypeSortColumns = sortColumns(store.AccessTypeSortFields, nil)
	permissionSortColumns = sortColumns(store.PermissionSortFields, nil)
)

// orderByClause returns a safe " ORDER BY <col> <dir>, id <dir>" clause.
// sort should already be validated against the allow-list by the caller.
// An empty sort falls back to fallbackCol. An unknown non-empty sort also
// falls back to fallbackCol for compatibility, but emits a warning so
// call-site bugs are not silently masked.
// A secondary ", id" tiebreaker is always appended to guarantee
// deterministic pagination when the primary column has duplicates.
func orderByClause(sort string, order store.SortOrder, allowed map[string]string, fallbackCol string) string {
	col := fallbackCol
	if sort != "" {
		if mapped, ok := allowed[sort]; ok {
			col = mapped
		} else {
			slog.Warn("unknown sort field, falling back to default", "sort", sort, "fallback", fallbackCol)
		}
	}
	dir := "ASC"
	if order == store.OrderDesc {
		dir = "DESC"
	}
	clause := " ORDER BY " + col + " " + dir
	if col != "id" {
		clause += ", id " + dir
	}
	return clause
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
	bit := maskFromSQL(curBit)
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
	mask := maskFromSQL(curMask)
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

func (s *Store) AddUserToGroup(ctx context.Context, domainID, userID, groupID string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO group_members (domain_id, user_id, group_id) VALUES (?, ?, ?)`,
		domainID, userID, groupID)
	return wrapConstraintError(err)
}

func (s *Store) RemoveUserFromGroup(ctx context.Context, domainID, userID, groupID string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM group_members WHERE domain_id = ? AND user_id = ? AND group_id = ?`,
		domainID, userID, groupID)
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

func (s *Store) GrantUserPermission(ctx context.Context, domainID, userID, permissionID string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO user_permissions (domain_id, user_id, permission_id) VALUES (?, ?, ?)`,
		domainID, userID, permissionID)
	return wrapConstraintError(err)
}

func (s *Store) RevokeUserPermission(ctx context.Context, domainID, userID, permissionID string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM user_permissions WHERE domain_id = ? AND user_id = ? AND permission_id = ?`,
		domainID, userID, permissionID)
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

func (s *Store) GrantGroupPermission(ctx context.Context, domainID, groupID, permissionID string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO group_permissions (domain_id, group_id, permission_id) VALUES (?, ?, ?)`,
		domainID, groupID, permissionID)
	return wrapConstraintError(err)
}

func (s *Store) RevokeGroupPermission(ctx context.Context, domainID, groupID, permissionID string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM group_permissions WHERE domain_id = ? AND group_id = ? AND permission_id = ?`,
		domainID, groupID, permissionID)
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

// userEffectivePermissionPredicateSQL filters permissions p down to those
// effectively held by a given user (direct grant OR via group membership).
// T51 composite FKs guarantee that user_permissions / group_permissions /
// group_members rows cannot reference cross-domain parents, so no
// defensive domain_id filter is needed in the sub-EXISTS clauses.
const userEffectivePermissionPredicateSQL = `
AND (
	EXISTS (
		SELECT 1 FROM user_permissions up
		WHERE up.permission_id = p.id AND up.user_id = ?
	)
	OR EXISTS (
		SELECT 1 FROM group_permissions gp
		INNER JOIN group_members gm ON gm.group_id = gp.group_id AND gm.user_id = ?
		WHERE gp.permission_id = p.id
	)
)
`

const effectiveMaskSQL = `
SELECT p.access_mask FROM permissions p
WHERE p.domain_id = ? AND p.resource_id = ?
` + userEffectivePermissionPredicateSQL

// userAuthzResourcesBaseSQL selects resources where the user has a non-
// zero effective mask via direct grants OR group membership. T51 composite
// FKs enforce cross-domain isolation at the schema level, so no
// defensive domain_id filters are layered on top of the join.
const userAuthzResourcesBaseSQL = `
FROM permissions p
WHERE p.domain_id = ? AND p.access_mask > 0
` + userEffectivePermissionPredicateSQL

func userEffectivePermissionArgs(userID string) []any {
	return []any{userID, userID}
}

func inPlaceholders(n int) (string, error) {
	if n <= 0 {
		return "", fmt.Errorf("in placeholders: n must be > 0")
	}
	return strings.TrimSuffix(strings.Repeat("?,", n), ","), nil
}

// buildUserAuthzMaskQueryAndArgs builds the batched mask query used by
// UserAuthzResourcesList and returns the SQL and args in the exact placeholder
// order to avoid call-site mistakes.
func buildUserAuthzMaskQueryAndArgs(domainID string, resourceIDs []string, predicateArgs []any) (string, []any, error) {
	placeholders, err := inPlaceholders(len(resourceIDs))
	if err != nil {
		return "", nil, err
	}
	query := `SELECT p.resource_id, p.access_mask FROM permissions p WHERE p.domain_id = ? AND p.resource_id IN (` + placeholders + `) AND p.access_mask > 0` + userEffectivePermissionPredicateSQL // #nosec G202
	args := make([]any, 0, 1+len(resourceIDs)+len(predicateArgs))
	args = append(args, domainID)
	for _, resourceID := range resourceIDs {
		args = append(args, resourceID)
	}
	args = append(args, predicateArgs...)
	return query, args, nil
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

	predicateArgs := userEffectivePermissionArgs(userID)
	countArgs := append([]any{domainID}, predicateArgs...)
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT p.resource_id) `+userAuthzResourcesBaseSQL, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	listArgs := append([]any{domainID}, predicateArgs...)
	listArgs = append(listArgs, opts.Limit, opts.Offset)
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT p.resource_id `+userAuthzResourcesBaseSQL+` ORDER BY p.resource_id ASC LIMIT ? OFFSET ?`,
		listArgs...,
	)
	if err != nil {
		return nil, 0, err
	}

	var resourceIDs []string
	for rows.Next() {
		var resourceID string
		if err := rows.Scan(&resourceID); err != nil {
			_ = rows.Close()
			return nil, 0, err
		}
		resourceIDs = append(resourceIDs, resourceID)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, 0, err
	}
	if err := rows.Close(); err != nil {
		return nil, 0, err
	}
	if len(resourceIDs) == 0 {
		return []store.UserAuthzResource{}, total, nil
	}

	maskSQL, maskArgs, err := buildUserAuthzMaskQueryAndArgs(domainID, resourceIDs, predicateArgs)
	if err != nil {
		return nil, 0, err
	}

	maskRows, err := s.db.QueryContext(ctx, maskSQL, maskArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = maskRows.Close() }()

	masksByResource := make(map[string]uint64, len(resourceIDs))
	for maskRows.Next() {
		var resourceID string
		var m int64
		if err := maskRows.Scan(&resourceID, &m); err != nil {
			return nil, 0, err
		}
		masksByResource[resourceID] |= maskFromSQL(m)
	}
	if err := maskRows.Err(); err != nil {
		return nil, 0, err
	}

	list := make([]store.UserAuthzResource, 0, len(resourceIDs))
	for _, resourceID := range resourceIDs {
		mask := masksByResource[resourceID]
		list = append(list, store.UserAuthzResource{ResourceID: resourceID, EffectiveMask: mask})
	}
	return list, total, nil
}

// groupAuthzResourcesBaseSQL joins permissions with group_permissions.
// p.domain_id is the primary scope; T51 composite FKs guarantee that
// matching group_permissions rows share the same domain, so no
// gp.domain_id filter is needed.
const groupAuthzResourcesBaseSQL = `
FROM permissions p
INNER JOIN group_permissions gp ON gp.permission_id = p.id
WHERE p.domain_id = ? AND gp.group_id = ? AND p.access_mask > 0
`

func (s *Store) GroupAuthzResourcesList(ctx context.Context, domainID, groupID string, opts store.ListOpts) ([]store.GroupAuthzResource, int64, error) {
	opts = store.SanitizeListOpts(opts)

	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT 1 FROM domains WHERE id = ?`, domainID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, 0, store.ErrNotFound
		}
		return nil, 0, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT 1 FROM groups WHERE id = ? AND domain_id = ?`, groupID, domainID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, 0, store.ErrNotFound
		}
		return nil, 0, err
	}

	baseArgs := []any{domainID, groupID}

	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT p.resource_id) `+groupAuthzResourcesBaseSQL, baseArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// opts.Sort / opts.Order are populated by the handler and reflected in the
	// meta response via writeList. The store always uses a fixed ORDER BY
	// p.resource_id ASC — Sort/Order opts are not honoured here because the
	// endpoint intentionally exposes only stable deterministic ordering.
	listArgs := append(append([]any{}, baseArgs...), opts.Limit, opts.Offset)
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT p.resource_id `+groupAuthzResourcesBaseSQL+` ORDER BY p.resource_id ASC LIMIT ? OFFSET ?`, // #nosec G202
		listArgs...,
	)
	if err != nil {
		return nil, 0, err
	}

	var resourceIDs []string
	for rows.Next() {
		var resourceID string
		if err := rows.Scan(&resourceID); err != nil {
			_ = rows.Close()
			return nil, 0, err
		}
		resourceIDs = append(resourceIDs, resourceID)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, 0, err
	}
	if err := rows.Close(); err != nil {
		return nil, 0, err
	}
	if len(resourceIDs) == 0 {
		return []store.GroupAuthzResource{}, total, nil
	}

	placeholders, err := inPlaceholders(len(resourceIDs))
	if err != nil {
		return nil, 0, err
	}
	maskSQL := `SELECT p.resource_id, p.access_mask FROM permissions p ` + // #nosec G202
		`INNER JOIN group_permissions gp ON gp.permission_id = p.id ` +
		`WHERE p.domain_id = ? AND gp.domain_id = ? AND gp.group_id = ? AND p.resource_id IN (` + placeholders + `) AND p.access_mask > 0`
	maskArgs := make([]any, 0, 3+len(resourceIDs))
	maskArgs = append(maskArgs, domainID, domainID, groupID)
	for _, rid := range resourceIDs {
		maskArgs = append(maskArgs, rid)
	}

	maskRows, err := s.db.QueryContext(ctx, maskSQL, maskArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = maskRows.Close() }()

	masksByResource := make(map[string]uint64, len(resourceIDs))
	for maskRows.Next() {
		var resourceID string
		var m int64
		if err := maskRows.Scan(&resourceID, &m); err != nil {
			return nil, 0, err
		}
		masksByResource[resourceID] |= maskFromSQL(m)
	}
	if err := maskRows.Err(); err != nil {
		return nil, 0, err
	}

	result := make([]store.GroupAuthzResource, 0, len(resourceIDs))
	for _, resourceID := range resourceIDs {
		mask := masksByResource[resourceID]
		result = append(result, store.GroupAuthzResource{ResourceID: resourceID, Mask: mask})
	}
	return result, total, nil
}

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

	placeholders, err := inPlaceholders(len(groupIDs))
	if err != nil {
		return nil, 0, err
	}
	maskSQL := `SELECT gp.group_id, p.access_mask FROM permissions p ` + // #nosec G202
		`INNER JOIN group_permissions gp ON gp.permission_id = p.id ` +
		`INNER JOIN groups g ON g.id = gp.group_id ` +
		`WHERE p.domain_id = ? AND p.resource_id = ? AND p.access_mask > 0 ` +
		`AND gp.group_id IN (` + placeholders + `)`
	maskArgs := make([]any, 0, 2+len(groupIDs))
	maskArgs = append(maskArgs, domainID, resourceID)
	for _, gid := range groupIDs {
		maskArgs = append(maskArgs, gid)
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
		masksByGroup[gid] |= maskFromSQL(m)
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
	placeholders, err := inPlaceholders(len(userIDs))
	if err != nil {
		return nil, 0, err
	}

	masksByUser := make(map[string]uint64, len(userIDs))

	// Direct user grants on this resource.
	directSQL := `SELECT up.user_id, p.access_mask FROM user_permissions up ` + // #nosec G202
		`INNER JOIN permissions p ON p.id = up.permission_id ` +
		`WHERE p.domain_id = ? AND p.resource_id = ? AND p.access_mask > 0 ` +
		`AND up.user_id IN (` + placeholders + `)`
	directArgs := make([]any, 0, 2+len(userIDs))
	directArgs = append(directArgs, domainID, resourceID)
	for _, uid := range userIDs {
		directArgs = append(directArgs, uid)
	}
	if err := scanUserMasks(ctx, s.db, directSQL, directArgs, masksByUser); err != nil {
		return nil, 0, err
	}

	// Indirect grants via group membership.
	indirectSQL := `SELECT gm.user_id, p.access_mask FROM group_members gm ` + // #nosec G202
		`INNER JOIN group_permissions gp ON gp.group_id = gm.group_id ` +
		`INNER JOIN permissions p ON p.id = gp.permission_id ` +
		`WHERE p.domain_id = ? AND p.resource_id = ? AND p.access_mask > 0 ` +
		`AND gm.user_id IN (` + placeholders + `)`
	indirectArgs := make([]any, 0, 2+len(userIDs))
	indirectArgs = append(indirectArgs, domainID, resourceID)
	for _, uid := range userIDs {
		indirectArgs = append(indirectArgs, uid)
	}
	if err := scanUserMasks(ctx, s.db, indirectSQL, indirectArgs, masksByUser); err != nil {
		return nil, 0, err
	}

	result := make([]store.ResourceAuthzUser, 0, len(userIDs))
	for _, uid := range userIDs {
		result = append(result, store.ResourceAuthzUser{UserID: uid, EffectiveMask: masksByUser[uid]})
	}
	return result, total, nil
}

func scanUserMasks(ctx context.Context, db *sql.DB, query string, args []any, into map[string]uint64) error {
	rows, err := db.QueryContext(ctx, query, args...)
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
		into[uid] |= maskFromSQL(m)
	}
	return rows.Err()
}

func (s *Store) PermissionMasksForUserResource(ctx context.Context, domainID, userID, resourceID string) ([]uint64, error) {
	args := make([]any, 0, 2+2)
	args = append(args, domainID, resourceID)
	args = append(args, userEffectivePermissionArgs(userID)...)
	rows, err := s.db.QueryContext(ctx, effectiveMaskSQL, args...)
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
