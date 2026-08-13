package http

import (
	"github/master_go_cqrs/internal/platform/security/gatewayauth"
	"github/master_go_cqrs/services/user/internal/accesscontrol/domain"
)

func toDomainUser(user gatewayauth.User) domain.User {
	resources := make([]domain.ResourceAccess, 0, len(user.ResourceAccess))
	for _, resource := range user.ResourceAccess {
		resources = append(resources, domain.ResourceAccess{
			ResourceID:   resource.ResourceID,
			ResourceName: resource.ResourceName,
			Scopes:       resource.Scopes,
		})
	}

	return domain.User{
		ID:             user.ID,
		Username:       user.Username,
		FirstName:      user.FirstName,
		LastName:       user.LastName,
		Email:          user.Email,
		Scope:          user.Scope,
		Roles:          user.Roles,
		Audience:       user.Audience,
		EmailVerified:  user.EmailVerified,
		ResourceAccess: resources,
	}
}
