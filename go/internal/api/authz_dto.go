package api

import (
	"strconv"

	"github.com/dtorabi/access-manager/internal/store"
)

// UserAuthzResourceDTO is the JSON response row for
// GET /domains/{domainID}/users/{userID}/authz/resources.
// OpenAPI schema: UserAuthzResource.
type UserAuthzResourceDTO struct {
	ResourceID    string `json:"resource_id"`
	EffectiveMask string `json:"effective_mask"`
}

// GroupAuthzResourceDTO is the JSON response row for
// GET /domains/{domainID}/groups/{groupID}/authz/resources.
// OpenAPI schema: GroupAuthzResource.
type GroupAuthzResourceDTO struct {
	ResourceID string `json:"resource_id"`
	Mask       string `json:"mask"`
}

// ResourceAuthzUserDTO is the JSON response row for
// GET /domains/{domainID}/resources/{resourceID}/authz/users.
// OpenAPI schema: ResourceAuthzUser.
type ResourceAuthzUserDTO struct {
	UserID        string `json:"user_id"`
	EffectiveMask string `json:"effective_mask"`
}

// ResourceAuthzGroupDTO is the JSON response row for
// GET /domains/{domainID}/resources/{resourceID}/authz/groups.
// OpenAPI schema: ResourceAuthzGroup.
type ResourceAuthzGroupDTO struct {
	GroupID string `json:"group_id"`
	Mask    string `json:"mask"`
}

// toAuthzDTOs maps a store authz list into a DTO slice using fn.
// It replaces the identical for-loop in each authz listing handler.
func toAuthzDTOs[S, D any](list []S, fn func(S) D) []D {
	result := make([]D, 0, len(list))
	for _, it := range list {
		result = append(result, fn(it))
	}
	return result
}

// userAuthzResourceDTOs converts a UserAuthzResource store slice to DTOs.
func userAuthzResourceDTOs(list []store.UserAuthzResource) []UserAuthzResourceDTO {
	return toAuthzDTOs(list, func(it store.UserAuthzResource) UserAuthzResourceDTO {
		return UserAuthzResourceDTO{
			ResourceID:    it.ResourceID,
			EffectiveMask: strconv.FormatUint(it.EffectiveMask, 10),
		}
	})
}

// groupAuthzResourceDTOs converts a GroupAuthzResource store slice to DTOs.
func groupAuthzResourceDTOs(list []store.GroupAuthzResource) []GroupAuthzResourceDTO {
	return toAuthzDTOs(list, func(it store.GroupAuthzResource) GroupAuthzResourceDTO {
		return GroupAuthzResourceDTO{
			ResourceID: it.ResourceID,
			Mask:       strconv.FormatUint(it.Mask, 10),
		}
	})
}

// resourceAuthzUserDTOs converts a ResourceAuthzUser store slice to DTOs.
func resourceAuthzUserDTOs(list []store.ResourceAuthzUser) []ResourceAuthzUserDTO {
	return toAuthzDTOs(list, func(it store.ResourceAuthzUser) ResourceAuthzUserDTO {
		return ResourceAuthzUserDTO{
			UserID:        it.UserID,
			EffectiveMask: strconv.FormatUint(it.EffectiveMask, 10),
		}
	})
}

// resourceAuthzGroupDTOs converts a ResourceAuthzGroup store slice to DTOs.
func resourceAuthzGroupDTOs(list []store.ResourceAuthzGroup) []ResourceAuthzGroupDTO {
	return toAuthzDTOs(list, func(it store.ResourceAuthzGroup) ResourceAuthzGroupDTO {
		return ResourceAuthzGroupDTO{
			GroupID: it.GroupID,
			Mask:    strconv.FormatUint(it.Mask, 10),
		}
	})
}
