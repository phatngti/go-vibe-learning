package query

import (
	"context"

	"github/master_go_cqrs/internal/user/application/dto"
	"github/master_go_cqrs/internal/user/domain"
	"github/master_go_cqrs/internal/security/gatewayauth"
)

type AuthenticationQuery struct {
	VerifyToken *VerifyTokenHandler
	GetInfo     *GetInfoHandler
}

func NewAuthenticationQuery(authProvider domain.AuthProvider) AuthenticationQuery {
	return AuthenticationQuery{
		VerifyToken: NewVerifyTokenHandler(),
		GetInfo:     NewGetInfoHandler(),
	}
}

type VerifyTokenHandler struct{}

func NewVerifyTokenHandler() *VerifyTokenHandler {
	return &VerifyTokenHandler{}
}

func (h *VerifyTokenHandler) Handle(ctx context.Context) dto.VerifyTokenResponseDTO {
	return dto.VerifyTokenResponseDTO{Valid: true}
}

type GetInfoHandler struct{}

func NewGetInfoHandler() *GetInfoHandler {
	return &GetInfoHandler{}
}

func (h *GetInfoHandler) Handle(ctx context.Context, user gatewayauth.User) gatewayauth.User {
	return user
}
