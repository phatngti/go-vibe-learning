package query

import (
	"context"

	"github/master_go_cqrs/internal/accesscontrol/application/dto"
	"github/master_go_cqrs/internal/accesscontrol/domain"
)

type CheckAuthnHandler struct{}

func NewCheckAuthnHandler() *CheckAuthnHandler {
	return &CheckAuthnHandler{}
}

func (h *CheckAuthnHandler) Handle(ctx context.Context, user domain.User) dto.AuthnCheckDataDTO {
	return dto.AuthnCheckDataDTO{
		Authenticated: true,
		User:          user,
	}
}
