package query

import (
	"context"

	"github/master_go_cqrs/services/user/internal/application/port"
	"github/master_go_cqrs/services/user/internal/domain"
)

// GetUserHandler retrieves a user identity by its internal UUID.
type GetUserHandler struct {
	repo port.UserRepository
}

func NewGetUserHandler(repo port.UserRepository) *GetUserHandler {
	return &GetUserHandler{repo: repo}
}

func (h *GetUserHandler) Handle(ctx context.Context, userID string) (*domain.UserIdentity, error) {
	return h.repo.GetByID(ctx, userID)
}

// GetUserByProviderIDHandler retrieves a user by the external identity provider's user ID.
type GetUserByProviderIDHandler struct {
	repo port.UserRepository
}

func NewGetUserByProviderIDHandler(repo port.UserRepository) *GetUserByProviderIDHandler {
	return &GetUserByProviderIDHandler{repo: repo}
}

func (h *GetUserByProviderIDHandler) Handle(ctx context.Context, provider, providerUserID string) (*domain.UserIdentity, error) {
	return h.repo.GetByProviderUserID(ctx, provider, providerUserID)
}

// ListUsersHandler retrieves a paginated list of all user identities.
type ListUsersHandler struct {
	repo port.UserRepository
}

func NewListUsersHandler(repo port.UserRepository) *ListUsersHandler {
	return &ListUsersHandler{repo: repo}
}

func (h *ListUsersHandler) Handle(ctx context.Context, limit, offset int) ([]*domain.UserIdentity, int64, error) {
	return h.repo.ListAll(ctx, limit, offset)
}
