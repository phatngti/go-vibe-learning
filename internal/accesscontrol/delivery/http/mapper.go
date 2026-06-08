package http

import (
	"github/master_go_cqrs/internal/accesscontrol/domain"
	"github/master_go_cqrs/internal/security/gatewayauth"
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
