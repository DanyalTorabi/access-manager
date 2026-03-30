package store

import (
	"context"
	"errors"
)

var (
	ErrNotFound     = errors.New("store: not found")
	ErrFKViolation  = errors.New("store: foreign key constraint violation")
	ErrInvalidInput = errors.New("store: invalid input")
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
	DomainList(ctx context.Context) ([]Domain, error)

	UserCreate(ctx context.Context, u *User) error
	UserGet(ctx context.Context, domainID, id string) (*User, error)
	UserList(ctx context.Context, domainID string) ([]User, error)

	GroupCreate(ctx context.Context, g *Group) error
	GroupGet(ctx context.Context, domainID, id string) (*Group, error)
	GroupList(ctx context.Context, domainID string) ([]Group, error)
	GroupSetParent(ctx context.Context, domainID, groupID string, parentID *string) error

	ResourceCreate(ctx context.Context, r *Resource) error
	ResourceGet(ctx context.Context, domainID, id string) (*Resource, error)
	ResourceList(ctx context.Context, domainID string) ([]Resource, error)

	AccessTypeCreate(ctx context.Context, a *AccessType) error
	AccessTypeList(ctx context.Context, domainID string) ([]AccessType, error)

	PermissionCreate(ctx context.Context, p *Permission) error
	PermissionGet(ctx context.Context, domainID, id string) (*Permission, error)
	PermissionList(ctx context.Context, domainID string) ([]Permission, error)

	AddUserToGroup(ctx context.Context, domainID, userID, groupID string) error
	RemoveUserFromGroup(ctx context.Context, domainID, userID, groupID string) error

	GrantUserPermission(ctx context.Context, domainID, userID, permissionID string) error
	RevokeUserPermission(ctx context.Context, domainID, userID, permissionID string) error
	GrantGroupPermission(ctx context.Context, domainID, groupID, permissionID string) error
	RevokeGroupPermission(ctx context.Context, domainID, groupID, permissionID string) error
}
