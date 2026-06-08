package domain

import "context"

type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

type AuthProvider interface {
	Login(ctx context.Context, username, password string) (TokenPair, error)
	RefreshToken(ctx context.Context, refreshToken string) (TokenPair, error)
}
