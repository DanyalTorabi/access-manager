package api

import (
	"strconv"

	"github.com/dtorabi/access-manager/internal/store"
)

// userAuthzResourceDTO is the JSON response row for
// GET /api/v1/domains/{domainID}/users/{userID}/authz/resources.
// OpenAPI schema: UserAuthzResource.
type userAuthzResourceDTO struct {
	ResourceID    string `json:"resource_id"`
	EffectiveMask string `json:"effective_mask"`
}

// groupAuthzResourceDTO is the JSON response row for
// GET /api/v1/domains/{domainID}/groups/{groupID}/authz/resources.
// OpenAPI schema: GroupAuthzResource.
type groupAuthzResourceDTO struct {
	ResourceID string `json:"resource_id"`
	Mask       string `json:"mask"`
}

// resourceAuthzUserDTO is the JSON response row for
// GET /api/v1/domains/{domainID}/resources/{resourceID}/authz/users.
// OpenAPI schema: ResourceAuthzUser.
type resourceAuthzUserDTO struct {
	UserID        string `json:"user_id"`
	EffectiveMask string `json:"effective_mask"`
}

// resourceAuthzGroupDTO is the JSON response row for
// GET /api/v1/domains/{domainID}/resources/{resourceID}/authz/groups.
// OpenAPI schema: ResourceAuthzGroup.
type resourceAuthzGroupDTO struct {
	GroupID string `json:"group_id"`
	Mask    string `json:"mask"`
}

// userAuthzResourceDTOs converts a UserAuthzResource store slice to DTOs.
func userAuthzResourceDTOs(list []store.UserAuthzResource) []userAuthzResourceDTO {
	result := make([]userAuthzResourceDTO, 0, len(list))
	for _, it := range list {
		result = append(result, userAuthzResourceDTO{
			ResourceID:    it.ResourceID,
			EffectiveMask: strconv.FormatUint(it.EffectiveMask, 10),
		})
	}
	return result
}

// groupAuthzResourceDTOs converts a GroupAuthzResource store slice to DTOs.
func groupAuthzResourceDTOs(list []store.GroupAuthzResource) []groupAuthzResourceDTO {
	result := make([]groupAuthzResourceDTO, 0, len(list))
	for _, it := range list {
		result = append(result, groupAuthzResourceDTO{
			ResourceID: it.ResourceID,
			Mask:       strconv.FormatUint(it.Mask, 10),
		})
	}
	return result
}

// resourceAuthzUserDTOs converts a ResourceAuthzUser store slice to DTOs.
func resourceAuthzUserDTOs(list []store.ResourceAuthzUser) []resourceAuthzUserDTO {
	result := make([]resourceAuthzUserDTO, 0, len(list))
	for _, it := range list {
		result = append(result, resourceAuthzUserDTO{
			UserID:        it.UserID,
			EffectiveMask: strconv.FormatUint(it.EffectiveMask, 10),
		})
	}
	return result
}

// resourceAuthzGroupDTOs converts a ResourceAuthzGroup store slice to DTOs.
func resourceAuthzGroupDTOs(list []store.ResourceAuthzGroup) []resourceAuthzGroupDTO {
	result := make([]resourceAuthzGroupDTO, 0, len(list))
	for _, it := range list {
		result = append(result, resourceAuthzGroupDTO{
			GroupID: it.GroupID,
			Mask:    strconv.FormatUint(it.Mask, 10),
		})
	}
	return result
}
