package keycloak

import (
	"context"

	"github.com/Nerzal/gocloak/v13"
	"github.com/golang-jwt/jwt/v5"
	"github/master_go_cqrs/internal/security/tokenauth"
	"go.uber.org/zap"
)

func (k *keycloak) Authorize(ctx context.Context, accessToken string, resourceWithScopes *map[string][]string) (*string, *string) {
	permissions := []string{}
	if resourceWithScopes != nil {
		for resource, scopes := range *resourceWithScopes {
			for _, scope := range scopes {
				permissions = append(permissions, resource+"#"+scope)
			}
		}
	}

	var rs *string
	if len(permissions) > 0 {
		authorized, err := k.Instance.GetRequestingPartyToken(
			ctx,
			accessToken,
			k.Realm,
			gocloak.RequestingPartyTokenOptions{
				Audience:    &k.ClientId,
				Permissions: &permissions,
			},
		)
		if err != nil {
			k.logger.Bg().Error("Error authorizing %v", zap.Error(err))
			errCode := tokenauth.AuthorizeFailed
			return nil, &errCode
		}
		rs = &authorized.AccessToken
	} else {
		introspect, err := k.Instance.RetrospectToken(
			ctx,
			accessToken,
			k.ClientId,
			k.clientSecret,
			k.Realm,
		)

		if err != nil {
			k.logger.Bg().Error("Error authorizing %v", zap.Error(err))
			errCode := tokenauth.AuthorizeFailed
			return nil, &errCode
		}

		if !*introspect.Active {
			k.logger.Bg().Error("Token is not active")
			errCode := tokenauth.SessionExpired
			return nil, &errCode
		}
		rs = &accessToken
	}

	return rs, nil
}

func (k *keycloak) LoginUser(ctx context.Context, username, password string) (*gocloak.JWT, error) {
	jwt, err := k.Instance.Login(ctx, k.ClientId, k.clientSecret, k.Realm, username, password)
	if err != nil {
		return nil, err
	}
	return jwt, nil
}

func (k *keycloak) DecodeToken(ctx context.Context, token string) (*jwt.MapClaims, error) {
	_, jwtMapClaim, err := k.Instance.DecodeAccessToken(ctx, token, k.Realm)
	if err != nil {
		return nil, err
	}
	return jwtMapClaim, nil
}

func (k *keycloak) RefreshToken(ctx context.Context, refreshToken string) (*gocloak.JWT, error) {
	jwt, err := k.Instance.RefreshToken(ctx, refreshToken, k.ClientId, k.clientSecret, k.Realm)
	if err != nil {
		return nil, err
	}
	return jwt, nil
}
