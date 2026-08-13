package command

import (
	"context"

	"github/master_go_cqrs/internal/platform/middleware"
	"github/master_go_cqrs/services/user/internal/application/dto"
	"github/master_go_cqrs/services/user/internal/application/port"
	"github/master_go_cqrs/services/user/internal/domain"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	loginOperation        = "Login"
	refreshTokenOperation = "RefreshToken"
	registerOperation     = "Register"
)

type Authentication struct {
	Login        *loginHandler
	RefreshToken *refreshTokenHandler
	Register     *registerHandler
}

func NewAuthentication(authProvider port.AuthProvider) *Authentication {
	return &Authentication{
		Login:        newLoginHandler(authProvider),
		RefreshToken: newRefreshTokenHandler(authProvider),
		Register:     newRegisterHandler(authProvider),
	}
}

type loginHandler struct {
	authProvider port.AuthProvider
	tracer       middleware.Tracer
}

func newLoginHandler(authProvider port.AuthProvider) *loginHandler {
	return &loginHandler{
		authProvider: authProvider,
		tracer:       middleware.NewTracer(otel.Tracer("user.authentication.command.login")),
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
	authProvider port.AuthProvider
	tracer       middleware.Tracer
}

func newRefreshTokenHandler(authProvider port.AuthProvider) *refreshTokenHandler {
	return &refreshTokenHandler{
		authProvider: authProvider,
		tracer:       middleware.NewTracer(otel.Tracer("user.authentication.command.refresh_token")),
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

type registerHandler struct {
	authProvider port.AuthProvider
	tracer       middleware.Tracer
}

func newRegisterHandler(authProvider port.AuthProvider) *registerHandler {
	return &registerHandler{
		authProvider: authProvider,
		tracer:       middleware.NewTracer(otel.Tracer("user.authentication.command.register")),
	}
}

func (h *registerHandler) Handle(ctx context.Context, cmd dto.RegisterRequest) (result dto.RegisterResponse, err error) {
	defer h.tracer.Trace(&ctx, registerOperation,
		middleware.TraceAttributes(
			attribute.String("auth.operation", registerOperation),
			attribute.String("email", cmd.Email),
		),
		middleware.TraceSpanKind(trace.SpanKindInternal),
		middleware.TraceError(&err),
	)()

	tokens, err := h.authProvider.Register(ctx, domain.RegisterCredentials{
		Email:    cmd.Email,
		Password: cmd.Password,
	})
	if err != nil {
		return dto.RegisterResponse{}, err
	}

	return dto.RegisterResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
	}, nil
}
