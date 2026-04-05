package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrNotFound     = errors.New("store: not found")
	ErrFKViolation  = errors.New("store: foreign key constraint violation")
	ErrInvalidInput = errors.New("store: invalid input")
	ErrConflict     = errors.New("store: conflict")
)

type Domain struct {
	ID    string
	Title string
}

type User struct {
	ID       string
	DomainID string
	Title    string
}

type Group struct {
	ID            string
	DomainID      string
	Title         string
	ParentGroupID *string
}

type Resource struct {
	ID       string
	DomainID string
	Title    string
}

type AccessType struct {
	ID       string
	DomainID string
	Title    string
	Bit      uint64
}

type Permission struct {
	ID          string
	DomainID    string
	Title       string
	ResourceID  string
	AccessMask  uint64
}

const (
	DefaultLimit = 20
	MaxLimit     = 100
)

// SearchType controls how the search term is matched against the title column.
type SearchType string

const (
	SearchContains   SearchType = "contains"
	SearchStartsWith SearchType = "starts_with"
	SearchEndsWith   SearchType = "ends_with"
)

// SortOrder controls ascending vs descending sort direction.
type SortOrder string

const (
	OrderAsc  SortOrder = "asc"
	OrderDesc SortOrder = "desc"
)

// Per-entity allowed sort fields. The first element is the default.
var (
	DomainSortFields     = []string{"title"}
	UserSortFields       = []string{"title"}
	GroupSortFields      = []string{"title"}
	ResourceSortFields   = []string{"title"}
	AccessTypeSortFields = []string{"title"}
	PermissionSortFields = []string{"title", "resource_id"}
)

// ValidateSort returns a validated sort field. An empty input defaults to
// allowed[0]. An unrecognised field produces a descriptive error listing
// the accepted values.
func ValidateSort(sort string, allowed []string) (string, error) {
	if sort == "" {
		return allowed[0], nil
	}
	for _, f := range allowed {
		if sort == f {
			return sort, nil
		}
	}
	return "", fmt.Errorf("sort must be one of: %s", strings.Join(allowed, ", "))
}

// ListOpts controls pagination, optional title search, and sorting for list queries.
type ListOpts struct {
	Offset     int
	Limit      int
	Search     string     // case-insensitive match on title; empty = no filter
	SearchType SearchType // defaults to SearchContains when empty
	Sort       string     // validated per entity before reaching store
	Order      SortOrder  // defaults to OrderAsc
}

// GroupListOpts extends ListOpts with group-specific filters.
type GroupListOpts struct {
	ListOpts
	ParentGroupID *string // nil = no filter; non-nil = WHERE parent_group_id = ?
}

// PermissionListOpts extends ListOpts with permission-specific filters.
type PermissionListOpts struct {
	ListOpts
	ResourceID *string // nil = no filter; non-nil = WHERE resource_id = ?
}

// SanitizeListOpts defaults Limit to DefaultLimit when <= 0, caps it at
// MaxLimit, floors Offset at 0, and defaults Order to OrderAsc.
func SanitizeListOpts(opts ListOpts) ListOpts {
	if opts.Limit <= 0 {
		opts.Limit = DefaultLimit
	}
	if opts.Limit > MaxLimit {
		opts.Limit = MaxLimit
	}
	if opts.Offset < 0 {
		opts.Offset = 0
	}
	if opts.Order == "" {
		opts.Order = OrderAsc
	}
	return opts
}

// GroupPatchParams is a partial update for a group. When UpdateParent is true,
// ParentGroupID is applied: nil clears the parent (root), non-nil sets that parent id.
type GroupPatchParams struct {
	Title         *string
	UpdateParent  bool
	ParentGroupID *string
}

// AccessTypePatchParams is a partial update for an access type.
type AccessTypePatchParams struct {
	Title *string
	Bit   *uint64
}

// PermissionPatchParams is a partial update for a permission.
type PermissionPatchParams struct {
	Title        *string
	ResourceID   *string
	AccessMask   *uint64
}

// AuthzReader resolves effective access masks for the indexed hot path.
type AuthzReader interface {
	EffectiveMask(ctx context.Context, domainID, userID, resourceID string) (uint64, error)
	PermissionMasksForUserResource(ctx context.Context, domainID, userID, resourceID string) ([]uint64, error)
}

// Store aggregates CRUD and authorization reads for the application.
type Store interface {
	AuthzReader

	DomainCreate(ctx context.Context, d *Domain) error
	DomainGet(ctx context.Context, id string) (*Domain, error)
	DomainList(ctx context.Context, opts ListOpts) ([]Domain, int64, error)
	DomainDelete(ctx context.Context, id string) error
	DomainPatch(ctx context.Context, id string, title *string) (*Domain, error)

	UserCreate(ctx context.Context, u *User) error
	UserGet(ctx context.Context, domainID, id string) (*User, error)
	UserList(ctx context.Context, domainID string, opts ListOpts) ([]User, int64, error)
	UserDelete(ctx context.Context, domainID, id string) error
	UserPatch(ctx context.Context, domainID, id string, title *string) (*User, error)

	GroupCreate(ctx context.Context, g *Group) error
	GroupGet(ctx context.Context, domainID, id string) (*Group, error)
	GroupList(ctx context.Context, domainID string, opts GroupListOpts) ([]Group, int64, error)
	GroupSetParent(ctx context.Context, domainID, groupID string, parentID *string) error
	GroupDelete(ctx context.Context, domainID, id string) error
	GroupPatch(ctx context.Context, domainID, id string, p GroupPatchParams) (*Group, error)

	ResourceCreate(ctx context.Context, r *Resource) error
	ResourceGet(ctx context.Context, domainID, id string) (*Resource, error)
	ResourceList(ctx context.Context, domainID string, opts ListOpts) ([]Resource, int64, error)
	ResourceDelete(ctx context.Context, domainID, id string) error
	ResourcePatch(ctx context.Context, domainID, id string, title *string) (*Resource, error)

	AccessTypeCreate(ctx context.Context, a *AccessType) error
	AccessTypeGet(ctx context.Context, domainID, id string) (*AccessType, error)
	AccessTypeList(ctx context.Context, domainID string, opts ListOpts) ([]AccessType, int64, error)
	AccessTypeDelete(ctx context.Context, domainID, id string) error
	AccessTypePatch(ctx context.Context, domainID, id string, p AccessTypePatchParams) (*AccessType, error)

	PermissionCreate(ctx context.Context, p *Permission) error
	PermissionGet(ctx context.Context, domainID, id string) (*Permission, error)
	PermissionList(ctx context.Context, domainID string, opts PermissionListOpts) ([]Permission, int64, error)
	PermissionDelete(ctx context.Context, domainID, id string) error
	PermissionPatch(ctx context.Context, domainID, id string, p PermissionPatchParams) (*Permission, error)

	AddUserToGroup(ctx context.Context, domainID, userID, groupID string) error
	RemoveUserFromGroup(ctx context.Context, domainID, userID, groupID string) error

	GrantUserPermission(ctx context.Context, domainID, userID, permissionID string) error
	RevokeUserPermission(ctx context.Context, domainID, userID, permissionID string) error
	GrantGroupPermission(ctx context.Context, domainID, groupID, permissionID string) error
	RevokeGroupPermission(ctx context.Context, domainID, groupID, permissionID string) error
}
