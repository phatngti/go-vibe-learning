package domain

import (
	"context"
	"github/master_go_cqrs/internal/user/application/dto"
)

type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

type AuthProvider interface {
	Register(ctx context.Context, userInfo dto.RegisterUserRequestDTO) (TokenPair, error)
	Login(ctx context.Context, username, password string) (TokenPair, error)
	RefreshToken(ctx context.Context, refreshToken string) (TokenPair, error)
}
