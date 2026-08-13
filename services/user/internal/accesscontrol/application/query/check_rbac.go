package query

import (
	"context"

	"github/master_go_cqrs/services/user/internal/accesscontrol/application/dto"
	"github/master_go_cqrs/services/user/internal/accesscontrol/domain"
)

type CheckRBACHandler struct{}

func NewCheckRBACHandler() *CheckRBACHandler {
	return &CheckRBACHandler{}
}

func (h *CheckRBACHandler) Handle(ctx context.Context, user domain.User, request dto.CheckRBACRequest) dto.RBACCheckDataDTO {
	return dto.RBACCheckDataDTO{
		Allowed:  domain.HasResourceScopes(user.ResourceAccess, request.Resource, request.Scopes),
		Resource: request.Resource,
		Scopes:   request.Scopes,
	}
}
