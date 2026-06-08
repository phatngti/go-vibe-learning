package tokenauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github/master_go_cqrs/internal/config"
	"github/master_go_cqrs/internal/httpapi/response"
	"github/master_go_cqrs/internal/observability/logger"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
)

type (
	Middleware interface {
		Authorize() echo.MiddlewareFunc
		AuthorizeWithResource(resource string, scopes []string) echo.MiddlewareFunc
	}

	AuthClient interface {
		Authorize(ctx context.Context, accessToken string, resourceWithScopes *map[string][]string) (*string, *string)
		DecodeToken(ctx context.Context, token string) (*jwt.MapClaims, error)
	}

	middleware struct {
		customHeaderName *string
		publicKey        string
		clientSecret     string
		authClient       AuthClient
		tracer           trace.Tracer
		logger           logger.Factory
	}

	MiddlewareParams struct {
		fx.In
		Config     *config.Config
		Logger     logger.Factory
		AuthClient AuthClient `optional:"true"`
	}
)

func NewMiddleware(params MiddlewareParams) Middleware {
	var customHeaderName *string
	if header := strings.TrimSpace(params.Config.Keycloak.CustomHeader); header != "" {
		customHeaderName = &header
	}

	return &middleware{
		customHeaderName: customHeaderName,
		publicKey:        params.Config.Keycloak.PublicKey,
		clientSecret:     params.Config.Keycloak.ClientSecret,
		authClient:       params.AuthClient,
		logger:           params.Logger,
		tracer:           otel.Tracer("authenticate"),
	}
}

func extractBearerToken(token string) string {
	return strings.Replace(token, "Bearer ", "", 1)
}

func (a *middleware) Authorize() echo.MiddlewareFunc {
	return a.authrorize(nil)
}

func (a *middleware) AuthorizeWithResource(resource string, scopes []string) echo.MiddlewareFunc {
	return a.authrorize(a.getAuthorizeResource(resource, scopes))
}

func (a middleware) getAuthorizeResource(resource string, scopes []string) *map[string][]string {
	return &map[string][]string{
		resource: scopes,
	}
}

func parsePublicKeyPEM(key string) string {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return ""
	}
	if strings.Contains(trimmed, "BEGIN PUBLIC KEY") {
		return trimmed
	}
	var formatted string
	formatted += "-----BEGIN PUBLIC KEY-----\n"
	raw := strings.ReplaceAll(trimmed, "\n", "")
	raw = strings.ReplaceAll(raw, "\r", "")
	raw = strings.ReplaceAll(raw, " ", "")
	for len(raw) > 0 {
		if len(raw) > 64 {
			formatted += raw[:64] + "\n"
			raw = raw[64:]
		} else {
			formatted += raw + "\n"
			raw = ""
		}
	}
	formatted += "-----END PUBLIC KEY-----"
	return formatted
}

func (a *middleware) verifyTokenOffline(tokenStr string) (*jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); ok {
			if a.publicKey == "" {
				return nil, fmt.Errorf("public key not configured for RS256 offline verification")
			}
			formattedKey := parsePublicKeyPEM(a.publicKey)
			parsedKey, err := jwt.ParseRSAPublicKeyFromPEM([]byte(formattedKey))
			if err != nil {
				return nil, fmt.Errorf("failed to parse RSA public key: %w", err)
			}
			return parsedKey, nil
		} else if _, ok := t.Method.(*jwt.SigningMethodHMAC); ok {
			if a.clientSecret == "" {
				return nil, fmt.Errorf("client secret not configured for HS256 offline verification")
			}
			return []byte(a.clientSecret), nil
		}
		return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return &claims, nil
	}

	return nil, fmt.Errorf("invalid token claims or signature")
}

func (a *middleware) verifyToken(ctx context.Context, tokenStr string, resourceWithScopes *map[string][]string) (*jwt.MapClaims, *string, error) {
	jwtMapClaim, err := a.verifyTokenOffline(tokenStr)
	if err == nil {
		return jwtMapClaim, nil, nil
	}
	if a.authClient == nil {
		return nil, nil, err
	}

	authorizedToken, errCode := a.authClient.Authorize(ctx, tokenStr, resourceWithScopes)
	if errCode != nil {
		return nil, errCode, fmt.Errorf("keycloak authorization failed: %s", *errCode)
	}
	if authorizedToken == nil || *authorizedToken == "" {
		errCode := InvalidToken
		return nil, &errCode, fmt.Errorf("keycloak returned an empty authorized token")
	}

	claims, decodeErr := a.authClient.DecodeToken(ctx, *authorizedToken)
	if decodeErr != nil {
		errCode := TokenClaimInvalid
		return nil, &errCode, decodeErr
	}
	return claims, nil, nil
}

func checkResourcePermissions(claims *jwt.MapClaims, resourceWithScopes *map[string][]string) bool {
	if resourceWithScopes == nil || len(*resourceWithScopes) == 0 {
		return true
	}
	authVal, ok := (*claims)["authorization"]
	if !ok {
		return false
	}
	authMap, ok := authVal.(map[string]interface{})
	if !ok {
		return false
	}
	permsVal, ok := authMap["permissions"]
	if !ok {
		return false
	}
	permsList, ok := permsVal.([]interface{})
	if !ok {
		return false
	}

	for reqRes, reqScopes := range *resourceWithScopes {
		matchedResource := false
		for _, permVal := range permsList {
			perm, ok := permVal.(map[string]interface{})
			if !ok {
				continue
			}
			rsName, _ := perm["rsname"].(string)
			if rsName == reqRes {
				matchedResource = true
				scopesVal, ok := perm["scopes"]
				if !ok {
					if len(reqScopes) == 0 {
						break
					}
					return false
				}
				scopesList, ok := scopesVal.([]interface{})
				if !ok {
					return false
				}
				hasScopes := make(map[string]bool)
				for _, sc := range scopesList {
					if scStr, ok := sc.(string); ok {
						hasScopes[scStr] = true
					}
				}
				for _, rs := range reqScopes {
					if !hasScopes[rs] {
						return false
					}
				}
				break
			}
		}
		if !matchedResource {
			return false
		}
	}
	return true
}

func (a *middleware) authrorize(resouceWithScopes *map[string][]string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx, span := a.tracer.Start(c.Request().Context(), "authenticate")
			defer span.End()
			if resouceWithScopes != nil {
				for resource, scopes := range *resouceWithScopes {
					span.SetAttributes(attribute.String("resource_name", resource))
					span.SetAttributes(attribute.StringSlice("resource_scopes", scopes))
				}
			}

			token := ""
			if a.customHeaderName != nil {
				token = c.Request().Header.Get(*a.customHeaderName)
			} else {
				token = c.Request().Header.Get("Authorization")
			}
			if token == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, response.WrapError(MessageCode, BearerIssued, nil))
			}

			token = extractBearerToken(token)

			if token == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, response.WrapError(MessageCode, TokenNotFound, nil))
			}

			jwtMapClaim, errCode, err := a.verifyToken(ctx, token, resouceWithScopes)
			if err != nil {
				a.logger.For(ctx).Error("JWT verification failed", "error", err)
				code := TokenAuthenFailed
				if errCode != nil {
					code = *errCode
				}
				status := http.StatusUnauthorized
				if code == AuthorizeFailed {
					status = http.StatusForbidden
				}
				return echo.NewHTTPError(status, response.WrapError(MessageCode, code, &[]response.ErrorDetailsInfo{{Code: code, Data: err.Error()}}))
			}

			if !checkResourcePermissions(jwtMapClaim, resouceWithScopes) {
				a.logger.For(ctx).Warn("Resource scope authorization failed offline")
				errCode := AuthorizeFailed
				return echo.NewHTTPError(http.StatusForbidden, response.WrapError(MessageCode, TokenAuthenFailed, &[]response.ErrorDetailsInfo{{Code: errCode, Data: "Insufficient UMA permissions"}}))
			}

			c.Set("user", a.GetUser(jwtMapClaim))
			return next(c)
		}
	}
}

func (a middleware) GetUser(jwtMapClaim *jwt.MapClaims) User {
	jsonBytes, err := json.Marshal(jwtMapClaim)
	if err != nil {
		return User{}
	}
	var userClaim UserClaim
	err = json.Unmarshal(jsonBytes, &userClaim)
	if err != nil {
		return User{}
	}

	resourceAccess := []ResourceAccess{}
	for _, resource := range userClaim.Authorization.Permissions {
		resourceAccess = append(resourceAccess, ResourceAccess{
			ResourceID:   resource.RsID,
			ResourceName: resource.RsName,
			Scopes:       resource.Scopes,
		})
	}
	return User{
		ID:             userClaim.Sub,
		Username:       userClaim.PreferredUsername,
		FirstName:      userClaim.GivenName,
		LastName:       userClaim.FamilyName,
		Email:          userClaim.Email,
		Scope:          userClaim.Scope,
		Roles:          userClaim.RealAccess.Roles,
		Audience:       userClaim.Audience,
		EmailVerified:  userClaim.EmailVerified,
		ResourceAccess: resourceAccess,
	}
}
