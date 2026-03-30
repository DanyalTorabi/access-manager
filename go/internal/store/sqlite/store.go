package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

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

func isFKViolation(err error) bool {
	var e *sqlite.Error
	return errors.As(err, &e) && e.Code() == sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY
}

func wrapFKError(err error) error {
	if isFKViolation(err) {
		return errors.Join(store.ErrFKViolation, err)
	}
	return err
}

func maskToSQL(m uint64) int64 { return int64(m) }

func maskFromSQL(v int64) uint64 { return uint64(v) }

func (s *Store) DomainCreate(ctx context.Context, d *store.Domain) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO domains (id, title) VALUES (?, ?)`, d.ID, d.Title)
	return err
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

func (s *Store) DomainList(ctx context.Context) ([]store.Domain, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, title FROM domains ORDER BY title`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var list []store.Domain
	for rows.Next() {
		var d store.Domain
		if err := rows.Scan(&d.ID, &d.Title); err != nil {
			return nil, err
		}
		list = append(list, d)
	}
	return list, rows.Err()
}

func (s *Store) UserCreate(ctx context.Context, u *store.User) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO users (id, domain_id, title) VALUES (?, ?, ?)`,
		u.ID, u.DomainID, u.Title)
	return err
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

func (s *Store) UserList(ctx context.Context, domainID string) ([]store.User, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, domain_id, title FROM users WHERE domain_id = ? ORDER BY title`, domainID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var list []store.User
	for rows.Next() {
		var u store.User
		if err := rows.Scan(&u.ID, &u.DomainID, &u.Title); err != nil {
			return nil, err
		}
		list = append(list, u)
	}
	return list, rows.Err()
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
	return err
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

func (s *Store) GroupList(ctx context.Context, domainID string) ([]store.Group, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, domain_id, title, parent_group_id FROM groups WHERE domain_id = ? ORDER BY title`, domainID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var list []store.Group
	for rows.Next() {
		var g store.Group
		var parent sql.NullString
		if err := rows.Scan(&g.ID, &g.DomainID, &g.Title, &parent); err != nil {
			return nil, err
		}
		if parent.Valid {
			g.ParentGroupID = &parent.String
		}
		list = append(list, g)
	}
	return list, rows.Err()
}

func (s *Store) GroupSetParent(ctx context.Context, domainID, groupID string, parentID *string) error {
	if parentID != nil && *parentID == groupID {
		return fmt.Errorf("%w: group cannot be its own parent", store.ErrInvalidInput)
	}
	if _, err := s.GroupGet(ctx, domainID, groupID); err != nil {
		return err
	}
	if parentID != nil {
		p, err := s.GroupGet(ctx, domainID, *parentID)
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
			pg, err := s.GroupGet(ctx, domainID, walk)
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
	_, err := s.db.ExecContext(ctx, `UPDATE groups SET parent_group_id = ? WHERE id = ? AND domain_id = ?`, parent, groupID, domainID)
	return err
}

func (s *Store) ResourceCreate(ctx context.Context, r *store.Resource) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO resources (id, domain_id, title) VALUES (?, ?, ?)`,
		r.ID, r.DomainID, r.Title)
	return err
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

func (s *Store) ResourceList(ctx context.Context, domainID string) ([]store.Resource, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, domain_id, title FROM resources WHERE domain_id = ? ORDER BY title`, domainID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var list []store.Resource
	for rows.Next() {
		var r store.Resource
		if err := rows.Scan(&r.ID, &r.DomainID, &r.Title); err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	return list, rows.Err()
}

func (s *Store) AccessTypeCreate(ctx context.Context, a *store.AccessType) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO access_types (id, domain_id, title, bit) VALUES (?, ?, ?, ?)`,
		a.ID, a.DomainID, a.Title, maskToSQL(a.Bit))
	return err
}

func (s *Store) AccessTypeList(ctx context.Context, domainID string) ([]store.AccessType, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, domain_id, title, bit FROM access_types WHERE domain_id = ? ORDER BY bit`, domainID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var list []store.AccessType
	for rows.Next() {
		var a store.AccessType
		var bit int64
		if err := rows.Scan(&a.ID, &a.DomainID, &a.Title, &bit); err != nil {
			return nil, err
		}
		a.Bit = maskFromSQL(bit)
		list = append(list, a)
	}
	return list, rows.Err()
}

func (s *Store) PermissionCreate(ctx context.Context, p *store.Permission) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO permissions (id, domain_id, title, resource_id, access_mask) VALUES (?, ?, ?, ?, ?)`,
		p.ID, p.DomainID, p.Title, p.ResourceID, maskToSQL(p.AccessMask))
	return err
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

func (s *Store) PermissionList(ctx context.Context, domainID string) ([]store.Permission, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, domain_id, title, resource_id, access_mask FROM permissions WHERE domain_id = ? ORDER BY title`, domainID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var list []store.Permission
	for rows.Next() {
		var p store.Permission
		var m int64
		if err := rows.Scan(&p.ID, &p.DomainID, &p.Title, &p.ResourceID, &m); err != nil {
			return nil, err
		}
		p.AccessMask = maskFromSQL(m)
		list = append(list, p)
	}
	return list, rows.Err()
}

func (s *Store) AddUserToGroup(ctx context.Context, domainID, userID, groupID string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO group_members (domain_id, user_id, group_id) VALUES (?, ?, ?)`,
		domainID, userID, groupID)
	return wrapFKError(err)
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
	return wrapFKError(err)
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
	return wrapFKError(err)
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
