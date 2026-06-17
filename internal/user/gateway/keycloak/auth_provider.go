package keycloak

import (
	"context"

	"github.com/Nerzal/gocloak/v13"

	platformkeycloak "github/master_go_cqrs/internal/platform/keycloak"
	"github/master_go_cqrs/internal/user/application/dto"
	"github/master_go_cqrs/internal/user/domain"
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

func (p *AuthProvider) Register(ctx context.Context, userInfo dto.RegisterUserRequestDTO) (domain.TokenPair, error) {
	user := gocloak.User{
		Email:    &userInfo.Email,
		Username: &userInfo.Email,
		Credentials: &[]gocloak.CredentialRepresentation{
			{
				Type:      gocloak.StringP("password"),
				Value:     &userInfo.Password,
				Temporary: gocloak.BoolP(false),
			},
		},
	}

	_, err := p.client.RegisterUser(ctx, user)
	if err != nil {
		return domain.TokenPair{}, err
	}

	// Log the user in to return a valid token pair.
	jwt, err := p.client.LoginUser(ctx, userInfo.Email, userInfo.Password)
	if err != nil {
		return domain.TokenPair{}, err
	}

	return domain.TokenPair{
		AccessToken:  jwt.AccessToken,
		RefreshToken: jwt.RefreshToken,
	}, nil
}

