package gatewayauth

import (
	"context"
	"errors"
	"slices"
	"strings"
)

type userContextKey struct{}

var (
	ErrUnauthenticated   = errors.New("authenticated gateway principal is required")
	ErrTenantRequired    = errors.New("authenticated principal is missing tenant_id")
	ErrAgentRequired     = errors.New("authenticated principal is missing agent_id")
	ErrOwnershipMismatch = errors.New("requested agent is not owned by the authenticated principal")
)

type Ownership struct {
	TenantID string
	AgentID  string
}

func WithUser(ctx context.Context, user User) context.Context {
	return context.WithValue(ctx, userContextKey{}, user)
}

func FromContext(ctx context.Context) (User, bool) {
	if ctx == nil {
		return User{}, false
	}
	user, ok := ctx.Value(userContextKey{}).(User)
	return user, ok && strings.TrimSpace(user.ID) != ""
}

func RequireUser(ctx context.Context) (User, error) {
	user, ok := FromContext(ctx)
	if !ok {
		return User{}, ErrUnauthenticated
	}
	return user, nil
}

// RequireAgentOwnership resolves the authoritative tenant and agent from the
// verified gateway principal. A client may repeat its agent ID for backwards
// compatibility, but it cannot select a different owner.
func RequireAgentOwnership(ctx context.Context, requestedAgentID string) (Ownership, error) {
	user, err := RequireUser(ctx)
	if err != nil {
		return Ownership{}, err
	}
	tenantID := strings.TrimSpace(user.TenantID)
	if tenantID == "" {
		return Ownership{}, ErrTenantRequired
	}
	agentID := strings.TrimSpace(user.AgentID)
	if agentID == "" {
		return Ownership{}, ErrAgentRequired
	}
	requestedAgentID = strings.TrimSpace(requestedAgentID)
	if requestedAgentID != "" && requestedAgentID != agentID {
		return Ownership{}, ErrOwnershipMismatch
	}
	return Ownership{TenantID: tenantID, AgentID: agentID}, nil
}

func (u User) HasRole(role string) bool {
	return slices.Contains(u.Roles, role)
}

func (u User) HasScope(scope string) bool {
	return slices.Contains(u.Scopes, scope)
}

func (u User) OwnsTenant(tenantID string) bool {
	return strings.TrimSpace(u.TenantID) != "" && strings.TrimSpace(u.TenantID) == strings.TrimSpace(tenantID)
}
