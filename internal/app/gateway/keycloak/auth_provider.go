package keycloak

import (
	"context"

	"github/master_go_cqrs/internal/app/domain"
	platformkeycloak "github/master_go_cqrs/internal/platform/keycloak"
)

type AuthProvider struct {
	client platformkeycloak.Client
}

func NewAuthProvider(client platformkeycloak.Client) domain.AuthProvider {
	return &AuthProvider{client: client}
}

func (p *AuthProvider) Login(ctx context.Context, username, password string) (domain.TokenPair, error) {
	jwt, err := p.client.LoginUser(ctx, username, password)
	if err != nil {
		return domain.TokenPair{}, err
	}

	return domain.TokenPair{
		AccessToken:  jwt.AccessToken,
		RefreshToken: jwt.RefreshToken,
	}, nil
}

func (p *AuthProvider) RefreshToken(ctx context.Context, refreshToken string) (domain.TokenPair, error) {
	jwt, err := p.client.RefreshToken(ctx, refreshToken)
	if err != nil {
		return domain.TokenPair{}, err
	}

	return domain.TokenPair{
		AccessToken:  jwt.AccessToken,
		RefreshToken: jwt.RefreshToken,
	}, nil
}
