package grpc

import (
	"context"

	userv1 "github/master_go_cqrs/internal/genproto/user"
	"github/master_go_cqrs/internal/user/application/query"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ServerHandler implements the user.v1.UserServiceServer gRPC service.
type ServerHandler struct {
	userv1.UnimplementedUserServiceServer
	getUser           *query.GetUserHandler
	getUserByProvider *query.GetUserByProviderIDHandler
	listUsers         *query.ListUsersHandler
}

func NewServerHandler(
	grpcServer *grpc.Server,
	getUser *query.GetUserHandler,
	getUserByProvider *query.GetUserByProviderIDHandler,
	listUsers *query.ListUsersHandler,
) *ServerHandler {
	h := &ServerHandler{
		getUser:           getUser,
		getUserByProvider: getUserByProvider,
		listUsers:         listUsers,
	}
	userv1.RegisterUserServiceServer(grpcServer, h)
	return h
}

// RequireServerHandler is an fx.Invoke target that forces Fx to construct the handler.
func RequireServerHandler(*ServerHandler) {}

func (h *ServerHandler) GetUser(ctx context.Context, req *userv1.GetUserRequest) (*userv1.GetUserResponse, error) {
	user, err := h.getUser.Handle(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "user not found: %v", err)
	}

	return &userv1.GetUserResponse{
		UserId:           user.ID,
		Username:         user.Username,
		Email:            derefStr(user.Email),
		FirstName:        derefStr(user.FirstName),
		LastName:         derefStr(user.LastName),
		EmailVerified:    user.EmailVerified,
		Enabled:          user.Enabled,
		IdentityProvider: user.IdentityProvider,
	}, nil
}

func (h *ServerHandler) GetUserByProviderID(ctx context.Context, req *userv1.GetUserByProviderIDRequest) (*userv1.GetUserResponse, error) {
	user, err := h.getUserByProvider.Handle(ctx, req.IdentityProvider, req.IdentityProviderUserId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "user not found: %v", err)
	}

	return &userv1.GetUserResponse{
		UserId:           user.ID,
		Username:         user.Username,
		Email:            derefStr(user.Email),
		FirstName:        derefStr(user.FirstName),
		LastName:         derefStr(user.LastName),
		EmailVerified:    user.EmailVerified,
		Enabled:          user.Enabled,
		IdentityProvider: user.IdentityProvider,
	}, nil
}

func (h *ServerHandler) ListUsers(ctx context.Context, req *userv1.ListUsersRequest) (*userv1.ListUsersResponse, error) {
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 50
	}
	offset := int(req.Offset)
	if offset < 0 {
		offset = 0
	}

	users, total, err := h.listUsers.Handle(ctx, limit, offset)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list users: %v", err)
	}

	resp := &userv1.ListUsersResponse{
		Total: int32(total),
		Users: make([]*userv1.GetUserResponse, 0, len(users)),
	}
	for _, u := range users {
		resp.Users = append(resp.Users, &userv1.GetUserResponse{
			UserId:           u.ID,
			Username:         u.Username,
			Email:            derefStr(u.Email),
			FirstName:        derefStr(u.FirstName),
			LastName:         derefStr(u.LastName),
			EmailVerified:    u.EmailVerified,
			Enabled:          u.Enabled,
			IdentityProvider: u.IdentityProvider,
		})
	}
	return resp, nil
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
