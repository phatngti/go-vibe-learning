package keycloak

import (
	"context"
	"sync"
	"time"

	"github/master_go_cqrs/internal/config"
	"github/master_go_cqrs/internal/observability/logger"

	"github.com/Nerzal/gocloak/v13"
	"github.com/golang-jwt/jwt/v5"
)

type (
	Client interface {
		// HARD CODE
		// 1. AUTHENTICATE
		Authorize(ctx context.Context, accessToken string, resourceWithScopes *map[string][]string) (*string, *string)
		DecodeToken(ctx context.Context, token string) (*jwt.MapClaims, error)
		LoginUser(ctx context.Context, username, password string) (*gocloak.JWT, error)
		RefreshToken(ctx context.Context, refreshToken string) (*gocloak.JWT, error)
		// 2. USER
		CreateNewUser(ctx context.Context, user gocloak.User) (string, error)
		GetUserByID(ctx context.Context, userID string) (*gocloak.User, error)
		GetUser(ctx context.Context, accessToken string) (*gocloak.UserInfo, error)
		// 3. CLIENT
		// 4. ROLE
		// 4.1 CLIENT ROLE
		GetClientRoles(ctx context.Context) ([]*gocloak.Role, error)
		GetClientRoleByName(ctx context.Context, roleName string) (*gocloak.Role, error)
		GetClientRoleByNames(ctx context.Context, roleNames []string) (*[]gocloak.Role, error)
		GetClientRole(ctx context.Context, roleName string) (*gocloak.Role, error)
		CreateClientRole(ctx context.Context, roleName string, description string) (string, error)
		UpdateClientRole(ctx context.Context, role gocloak.Role) (string, error)
		DeleteClientRole(ctx context.Context, roleName string) (string, error)
		AssignClientRoleToUser(ctx context.Context, userID string, roles []gocloak.Role) ([]string, error)
		UnassignClientRoleOfUser(ctx context.Context, userID string, roleName string) (string, error)
		// 4.2 REALM ROLE
		GetRealmRoles(ctx context.Context) ([]*gocloak.Role, error)
		GetRealmRole(ctx context.Context, roleName string) (*gocloak.Role, error)
		CreateRealmRole(ctx context.Context, roleName string, description string) (string, error)
		UpdateRealmRole(ctx context.Context, role gocloak.Role) (string, error)
		DeleteRealmRole(ctx context.Context, roleName string) (string, error)
		AssignRealmRoleToUser(ctx context.Context, userID string, roleNames []string) ([]string, error)
		UnassignRealmRoleOfUser(ctx context.Context, userID string, roleName string) (string, error)
		// 5. USER
		GetClientUserRoles(ctx context.Context, userID string) ([]*gocloak.Role, error)
		GetRealmUserRoles(ctx context.Context, userID string) ([]*gocloak.Role, error)

		// 6. RESOURCE
		GetResources(ctx context.Context) ([]*gocloak.ResourceRepresentation, error)
		GetResourceByID(ctx context.Context, resourceID string) (*gocloak.ResourceRepresentation, error)
		CreateResource(ctx context.Context, resourceData gocloak.ResourceRepresentation) (*gocloak.ResourceRepresentation, error)
		UpdateResource(ctx context.Context, resource gocloak.ResourceRepresentation) error
		DeleteResource(ctx context.Context, resourceID string) error

		// 7. SCOPE
		GetScopes(ctx context.Context) ([]*gocloak.ScopeRepresentation, error)
		GetScopeByIDs(ctx context.Context, scopeIDs []string) (*[]gocloak.ScopeRepresentation, error)
		CreateScope(ctx context.Context, scopeData gocloak.ScopeRepresentation) (*gocloak.ScopeRepresentation, error)
		// 7. POLICY
		GetPolicies(ctx context.Context) ([]*gocloak.PolicyRepresentation, error)
		GetPolicyByID(ctx context.Context, policyID string) (*gocloak.PolicyRepresentation, error)
		GetPolicyByName(ctx context.Context, policyName string) (*gocloak.PolicyRepresentation, error)
		CreatePolicy(ctx context.Context, policyData gocloak.PolicyRepresentation) (*gocloak.PolicyRepresentation, error)
		UpdatePolicy(ctx context.Context, policyData gocloak.PolicyRepresentation) error
		DeletePolicy(ctx context.Context, policyID string) error

		// 8. PERMISSION
		GetPermissions(ctx context.Context) ([]*gocloak.PermissionRepresentation, error)
		GetPermissionByID(ctx context.Context, permissionID string) (*gocloak.PermissionRepresentation, error)
		GetPermissionByName(ctx context.Context, permissionName string) (*gocloak.PermissionRepresentation, error)
		CreatePermission(ctx context.Context, permissionData gocloak.PermissionRepresentation) (*gocloak.PermissionRepresentation, error)
		UpdatePermission(ctx context.Context, permissionData gocloak.PermissionRepresentation) error
	}
)

type keycloak struct {
	Instance      *gocloak.GoCloak
	Realm         string
	ClientId      string
	clientSecret  string
	logger        logger.Factory
	mu            sync.RWMutex
	cachedClient  *AuthenticateClient
	cachedExpires time.Time
}

func NewClient(cfg *config.Config, logger logger.Factory) Client {
	logger.Bg().Info("Creating Keycloak connection")
	client := gocloak.NewClient(cfg.Keycloak.Endpoint)

	return &keycloak{
		logger:       logger,
		Instance:     client,
		Realm:        cfg.Keycloak.Realm,
		ClientId:     cfg.Keycloak.ClientID,
		clientSecret: cfg.Keycloak.ClientSecret,
	}

}
