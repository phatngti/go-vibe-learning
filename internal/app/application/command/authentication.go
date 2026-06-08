package command

import (
	"context"

	"github/master_go_cqrs/internal/app/application/dto"
	"github/master_go_cqrs/internal/app/domain"
	"github/master_go_cqrs/internal/middleware"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	loginOperation        = "Login"
	refreshTokenOperation = "RefreshToken"
)

type Authentication struct {
	Login        *loginHandler
	RefreshToken *refreshTokenHandler
}

func NewAuthentication(authProvider domain.AuthProvider) *Authentication {
	return &Authentication{
		Login:        newLoginHandler(authProvider),
		RefreshToken: newRefreshTokenHandler(authProvider),
	}
}

type loginHandler struct {
	authProvider domain.AuthProvider
	tracer       middleware.Tracer
}

func newLoginHandler(authProvider domain.AuthProvider) *loginHandler {
	return &loginHandler{
		authProvider: authProvider,
		tracer:       middleware.NewTracer(otel.Tracer("app.authentication.command.login")),
	}
}

func (h *loginHandler) Handle(ctx context.Context, cmd dto.LoginRequest) (result dto.LoginResponse, err error) {
	defer h.tracer.Trace(&ctx, loginOperation,
		middleware.TraceAttributes(
			attribute.String("auth.operation", loginOperation),
			attribute.String("username", cmd.Username),
		),
		middleware.TraceSpanKind(trace.SpanKindInternal),
		middleware.TraceError(&err),
	)()

	tokens, err := h.authProvider.Login(ctx, cmd.Username, cmd.Password)
	if err != nil {
		return dto.LoginResponse{}, err
	}

	return dto.LoginResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
	}, nil
}

type refreshTokenHandler struct {
	authProvider domain.AuthProvider
	tracer       middleware.Tracer
}

func newRefreshTokenHandler(authProvider domain.AuthProvider) *refreshTokenHandler {
	return &refreshTokenHandler{
		authProvider: authProvider,
		tracer:       middleware.NewTracer(otel.Tracer("app.authentication.command.refresh_token")),
	}
}

func (h *refreshTokenHandler) Handle(ctx context.Context, cmd dto.RefreshTokenRequest) (result dto.RefreshTokenResponse, err error) {
	defer h.tracer.Trace(&ctx, refreshTokenOperation,
		middleware.TraceAttributes(attribute.String("auth.operation", refreshTokenOperation)),
		middleware.TraceSpanKind(trace.SpanKindInternal),
		middleware.TraceError(&err),
	)()

	tokens, err := h.authProvider.RefreshToken(ctx, cmd.RefreshToken)
	if err != nil {
		return dto.RefreshTokenResponse{}, err
	}

	return dto.RefreshTokenResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
	}, nil
}
