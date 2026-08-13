package query

import (
	"context"

	"github/master_go_cqrs/internal/platform/security/gatewayauth"
	"github/master_go_cqrs/services/user/internal/application/dto"
)

type AuthenticationQuery struct {
	VerifyToken *VerifyTokenHandler
	GetInfo     *GetInfoHandler
}

func NewAuthenticationQuery() AuthenticationQuery {
	return AuthenticationQuery{
		VerifyToken: NewVerifyTokenHandler(),
		GetInfo:     NewGetInfoHandler(),
	}
}

type VerifyTokenHandler struct{}

func NewVerifyTokenHandler() *VerifyTokenHandler {
	return &VerifyTokenHandler{}
}

func (h *VerifyTokenHandler) Handle(ctx context.Context) dto.VerifyTokenResponse {
	return dto.VerifyTokenResponse{Valid: true}
}

type GetInfoHandler struct{}

func NewGetInfoHandler() *GetInfoHandler {
	return &GetInfoHandler{}
}

func (h *GetInfoHandler) Handle(ctx context.Context, user gatewayauth.User) gatewayauth.User {
	return user
}
