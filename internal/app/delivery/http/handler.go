package http

import (
	"net/http"

	"github/master_go_cqrs/internal/app/application/command"
	"github/master_go_cqrs/internal/app/application/dto"
	"github/master_go_cqrs/internal/app/application/query"
	"github/master_go_cqrs/internal/httpapi/response"
	"github/master_go_cqrs/internal/httpapi/route"
	"github/master_go_cqrs/internal/middleware"
	"github/master_go_cqrs/internal/observability/logger"
	"github/master_go_cqrs/internal/security/gatewayauth"

	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type Handler struct {
	logger       logger.Factory
	auth         *command.Authentication
	verifyToken  *query.VerifyTokenHandler
	getInfo      *query.GetInfoHandler
	tracer       trace.Tracer
	authenticate gatewayauth.Middleware
}

type HandlerParams struct {
	fx.In
	Logger      logger.Factory
	Auth        *command.Authentication
	VerifyToken *query.VerifyTokenHandler
	GetInfo     *query.GetInfoHandler
	TokenAuth   gatewayauth.Middleware
}

func NewHandler(params HandlerParams) route.Registrar {
	return &Handler{
		logger:       params.Logger,
		auth:         params.Auth,
		verifyToken:  params.VerifyToken,
		getInfo:      params.GetInfo,
		tracer:       otel.Tracer("app.authentication.delivery.http"),
		authenticate: params.TokenAuth,
	}
}

func (h *Handler) Pattern(prefix *echo.Group) string {
	return "/auth"
}

func (h *Handler) RegisterRoutes(g *echo.Group) {
	g.GET("/verify-token", h.verify,
		h.authenticate.Authorize())

	g.POST(
		"/login",
		h.loginUser,
		middleware.ValidateHandler(new(dto.LoginRequestDTO), dto.MessageCode),
	)

	g.POST("/refresh",
		h.refresh,
		middleware.ValidateHandler(new(dto.RefreshTokenRequestDTO), dto.MessageCode),
	)

	g.GET("/info",
		h.info,
		h.authenticate.Authorize(),
	)
}

// verify godoc
//
//	@Summary		Verify token
//	@Description	Verify the authentication token.
//	@Tags			authentication
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	dto.VerifyTokenResponseDTO	"Verify token successful"
//	@Failure		401	{object}	response.ErrorResponse	"Invalid credentials"
//	@Failure		500	{object}	response.ErrorResponse	"Internal server error"
//	@Security		BearerAuth
//	@Router			/auth/verify-token [get]
func (h *Handler) verify(c echo.Context) error {
	payload := h.verifyToken.Handle(c.Request().Context())
	return c.JSON(http.StatusOK, response.Wrap(dto.MessageCode, dto.VerifyTokenSuccess, payload))
}

// loginUser godoc
//
//	@Summary		User Login
//	@Description	Authenticate user with username and password.
//	@Tags			authentication
//	@Accept			json
//	@Produce		json
//	@Param			loginRequest	body		dto.LoginRequestDTO	true	"Login credentials"
//	@Success		200				{object}	dto.LoginResponseDTO	"Login successful"
//	@Failure		401				{object}	response.ErrorResponse	"Invalid credentials"
//	@Failure		500				{object}	response.ErrorResponse	"Internal server error"
//	@Router			/auth/login [post]
func (h *Handler) loginUser(c echo.Context) error {
	ctx := c.Request().Context()
	request := c.Get("dto").(*dto.LoginRequestDTO)

	result, err := h.auth.Login.Handle(ctx, dto.LoginRequest{
		Username: request.Username,
		Password: request.Password,
	})
	if err != nil {
		h.logger.For(ctx).Error("%v", err.Error()).WithFields(zap.Error(err))
		return echo.NewHTTPError(http.StatusUnauthorized, response.WrapError(dto.MessageCode, dto.LoginFailed, nil))
	}

	h.logger.For(ctx).Info("Login success")

	return c.JSON(http.StatusOK, response.Wrap(dto.MessageCode, dto.LoginSuccess, dto.LoginResponseDTO{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
	}))
}

// refresh godoc
//
//	@Summary		Refresh access token
//	@Description	Refresh the authentication token.
//	@Tags			authentication
//	@Accept			json
//	@Produce		json
//	@Param			refreshRequest	body		dto.RefreshTokenRequestDTO	true	"Refresh token payload"
//	@Success		200				{object}	dto.LoginResponseDTO		"Refresh successful"
//	@Failure		401				{object}	response.ErrorResponse		"Invalid refresh token"
//	@Failure		500				{object}	response.ErrorResponse		"Internal server error"
//	@Router			/auth/refresh [post]
func (h *Handler) refresh(c echo.Context) error {
	ctx := c.Request().Context()
	request := c.Get("dto").(*dto.RefreshTokenRequestDTO)

	result, err := h.auth.RefreshToken.Handle(ctx, dto.RefreshTokenRequest{
		RefreshToken: request.RefreshToken,
	})
	if err != nil {
		h.logger.For(ctx).Error("%v", err.Error()).WithFields(zap.Error(err))
		return echo.NewHTTPError(http.StatusUnauthorized, response.WrapError(dto.MessageCode, dto.RefreshTokenFailed, nil))
	}

	h.logger.For(ctx).Info("Refresh token success")

	return c.JSON(http.StatusOK, response.Wrap(dto.MessageCode, dto.RefreshTokenSuccess, dto.LoginResponseDTO{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
	}))
}

// info godoc
//
//	@Summary		Get user info
//	@Description	Return authenticated user claims and permissions.
//	@Tags			authentication
//	@Produce		json
//	@Success		200	{object}	dto.InfoResponseDTO		"User info"
//	@Failure		401	{object}	response.ErrorResponse	"Invalid credentials"
//	@Security		BearerAuth
//	@Router			/auth/info [get]
func (h *Handler) info(c echo.Context) error {
	user := h.getInfo.Handle(c.Request().Context(), c.Get("user").(gatewayauth.User))
	return c.JSON(http.StatusOK, response.Wrap(dto.MessageCode, dto.InfoSuccess, dto.InfoDataDTO{Info: user}))
}
