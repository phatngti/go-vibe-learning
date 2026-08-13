package http

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github/master_go_cqrs/internal/platform/httpapi/response"
	"github/master_go_cqrs/internal/platform/httpapi/route"
	"github/master_go_cqrs/internal/platform/middleware"
	"github/master_go_cqrs/internal/platform/observability/logger"
	"github/master_go_cqrs/internal/platform/security/gatewayauth"
	"github/master_go_cqrs/services/user/internal/application/command"
	"github/master_go_cqrs/services/user/internal/application/dto"
	"github/master_go_cqrs/services/user/internal/application/query"

	"github.com/labstack/echo/v4"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type Handler struct {
	logger             logger.Factory
	auth               *command.Authentication
	emitUserRegistered *command.EmitUserRegisteredHandler
	verifyToken        *query.VerifyTokenHandler
	getInfo            *query.GetInfoHandler
	authenticate       gatewayauth.Middleware

	// User management handlers
	getUser   *query.GetUserHandler
	listUsers *query.ListUsersHandler
}

type HandlerParams struct {
	fx.In
	Logger             logger.Factory
	Auth               *command.Authentication
	EmitUserRegistered *command.EmitUserRegisteredHandler
	VerifyToken        *query.VerifyTokenHandler
	GetInfo            *query.GetInfoHandler
	TokenAuth          gatewayauth.Middleware

	// User management dependencies
	GetUser   *query.GetUserHandler   `optional:"true"`
	ListUsers *query.ListUsersHandler `optional:"true"`
}

func NewHandler(params HandlerParams) route.Registrar {
	return &Handler{
		logger:             params.Logger,
		auth:               params.Auth,
		emitUserRegistered: params.EmitUserRegistered,
		verifyToken:        params.VerifyToken,
		getInfo:            params.GetInfo,
		authenticate:       params.TokenAuth,
		getUser:            params.GetUser,
		listUsers:          params.ListUsers,
	}
}

func (h *Handler) Pattern(prefix *echo.Group) string {
	return "/auth"
}

func (h *Handler) RegisterRoutes(g *echo.Group) {
	// Existing auth routes
	g.GET("/verify-token", h.verify,
		h.authenticate.Authorize())

	g.POST(
		"/login",
		h.loginUser,
		middleware.ValidateHandler(new(LoginRequestDTO), dto.MessageCode),
	)

	g.POST("/refresh",
		h.refresh,
		middleware.ValidateHandler(new(RefreshTokenRequestDTO), dto.MessageCode),
	)

	g.GET("/info",
		h.info,
		h.authenticate.Authorize(),
	)

	g.POST("/register", h.register, middleware.ValidateHandler(new(RegisterUserRequestDTO), dto.MessageCode))

	// User management routes (only registered when handlers are available)
	if h.getUser != nil {
		g.GET("/users/:id", h.getUserHandler, h.authenticate.Authorize())
	}
	if h.listUsers != nil {
		g.GET("/users", h.listUsersHandler, h.authenticate.Authorize())
	}
}

// verify godoc
//
//	@Summary		Verify token
//	@Description	Verify the authentication token.
//	@Tags			authentication
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	dto.VerifyTokenResponse	"Verify token successful"
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
//	@Param			loginRequest	body		LoginRequestDTO			true	"Login credentials"
//	@Success		200				{object}	dto.LoginResponse		"Login successful"
//	@Failure		401				{object}	response.ErrorResponse	"Invalid credentials"
//	@Failure		500				{object}	response.ErrorResponse	"Internal server error"
//	@Router			/auth/login [post]
func (h *Handler) loginUser(c echo.Context) error {
	ctx := c.Request().Context()
	request := c.Get("dto").(*LoginRequestDTO)

	result, err := h.auth.Login.Handle(ctx, dto.LoginRequest{
		Username: request.Username,
		Password: request.Password,
	})
	if err != nil {
		h.logger.For(ctx).Error("%v", err.Error()).WithFields(zap.Error(err))
		return echo.NewHTTPError(http.StatusUnauthorized, response.WrapError(dto.MessageCode, dto.LoginFailed, nil))
	}

	h.logger.For(ctx).Info("Login success")

	return c.JSON(http.StatusOK, response.Wrap(dto.MessageCode, dto.LoginSuccess, dto.LoginResponse{
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
//	@Param			refreshRequest	body		RefreshTokenRequestDTO		true	"Refresh token payload"
//	@Success		200				{object}	dto.LoginResponse			"Refresh successful"
//	@Failure		401				{object}	response.ErrorResponse		"Invalid refresh token"
//	@Failure		500				{object}	response.ErrorResponse		"Internal server error"
//	@Router			/auth/refresh [post]
func (h *Handler) refresh(c echo.Context) error {
	ctx := c.Request().Context()
	request := c.Get("dto").(*RefreshTokenRequestDTO)

	result, err := h.auth.RefreshToken.Handle(ctx, dto.RefreshTokenRequest{
		RefreshToken: request.RefreshToken,
	})
	if err != nil {
		h.logger.For(ctx).Error("%v", err.Error()).WithFields(zap.Error(err))
		return echo.NewHTTPError(http.StatusUnauthorized, response.WrapError(dto.MessageCode, dto.RefreshTokenFailed, nil))
	}

	h.logger.For(ctx).Info("Refresh token success")

	return c.JSON(http.StatusOK, response.Wrap(dto.MessageCode, dto.RefreshTokenSuccess, dto.LoginResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
	}))
}

// register godoc
//
//	@Summary		Register
//	@Description	Register
//	@ID				Handler.register
//	@Tags			authentication
//	@Accept			json
//	@Produce		json
//	@Param			request	body		RegisterUserRequestDTO	true	"Request payload"
//	@Success		201		{object}	ResponseType			"OK"
//	@Failure		401		{object}	response.ErrorResponse	"Unauthorized"
//	@Failure		400		{object}	response.ErrorResponse	"Bad request"
//	@Failure		500		{object}	response.ErrorResponse	"Internal error"
//	@Router			/auth/register [post]
func (h *Handler) register(c echo.Context) error {
	ctx := c.Request().Context()
	request := c.Get("dto").(*RegisterUserRequestDTO)

	result, err := h.auth.Register.Handle(ctx, dto.RegisterRequest{
		Email:    request.Email,
		Password: request.Password,
	})
	if err != nil {
		h.logger.For(ctx).Error("%v", err.Error()).WithFields(zap.Error(err))
		return echo.NewHTTPError(http.StatusUnauthorized, response.WrapError(dto.MessageCode, dto.RegisterFailed, nil))
	}

	// Emit UserRegistered domain event for async sync to local DB.
	// Event emission is now delegated to the application command layer.
	providerUserID, err := extractSubFromToken(result.AccessToken)
	if err != nil {
		h.logger.For(ctx).Error("Failed to extract sub from token").WithFields(zap.Error(err))
	} else if err := h.emitUserRegistered.Handle(ctx, providerUserID, request.Email); err != nil {
		h.logger.For(ctx).Error("Failed to emit UserRegistered event").WithFields(zap.Error(err))
	}

	h.logger.For(ctx).Info("Register success")

	return c.JSON(http.StatusOK, response.Wrap(dto.MessageCode, dto.RegisterSuccess, result))
}

// extractSubFromToken decodes the JWT payload (without signature verification)
// and returns the "sub" claim. The token is trusted because it was just received
// from our own Keycloak instance.
func extractSubFromToken(tokenStr string) (string, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("malformed JWT: expected 3 parts, got %d", len(parts))
	}

	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("decode JWT payload: %w", err)
	}

	var claims struct {
		Sub string `json:"sub"`
	}
	if err := json.Unmarshal(raw, &claims); err != nil {
		return "", fmt.Errorf("parse JWT claims: %w", err)
	}
	if claims.Sub == "" {
		return "", fmt.Errorf("JWT missing sub claim")
	}
	return claims.Sub, nil
}

// info godoc
//
//	@Summary		Get user info
//	@Description	Return authenticated user claims and permissions.
//	@Tags			authentication
//	@Produce		json
//	@Success		200	{object}	dto.InfoDataDTO			"User info"
//	@Failure		401	{object}	response.ErrorResponse	"Invalid credentials"
//	@Security		BearerAuth
//	@Router			/auth/info [get]
func (h *Handler) info(c echo.Context) error {
	user := h.getInfo.Handle(c.Request().Context(), c.Get("user").(gatewayauth.User))
	return c.JSON(http.StatusOK, response.Wrap(dto.MessageCode, dto.InfoSuccess, dto.InfoDataDTO{Info: user}))
}

// getUserHandler godoc
//
//	@Summary		Get user by ID
//	@Description	Retrieve a user identity by its internal UUID.
//	@Tags			user-management
//	@Produce		json
//	@Param			id	path		string					true	"User ID (UUID)"
//	@Success		200	{object}	dto.GetUserResponse		"User found"
//	@Failure		404	{object}	response.ErrorResponse	"User not found"
//	@Security		BearerAuth
//	@Router			/auth/users/{id} [get]
func (h *Handler) getUserHandler(c echo.Context) error {
	ctx := c.Request().Context()
	user, err := h.getUser.Handle(ctx, c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound,
			response.WrapError(dto.UserMessageCode, dto.GetUserFailed, nil))
	}
	return c.JSON(http.StatusOK, response.Wrap(dto.UserMessageCode, dto.GetUserSuccess,
		dto.GetUserResponse{
			ID:                     user.ID,
			IdentityProvider:       user.IdentityProvider,
			IdentityProviderUserID: user.IdentityProviderUserID,
			Username:               user.Username,
			Email:                  user.Email,
			FirstName:              user.FirstName,
			LastName:               user.LastName,
			EmailVerified:          user.EmailVerified,
			Enabled:                user.Enabled,
		}))
}

// listUsersHandler godoc
//
//	@Summary		List users
//	@Description	Retrieve a paginated list of user identities.
//	@Tags			user-management
//	@Produce		json
//	@Param			limit	query		int						false	"Max results (default 50)"
//	@Param			offset	query		int						false	"Offset (default 0)"
//	@Success		200		{object}	dto.ListUsersResponse	"Users listed"
//	@Failure		500		{object}	response.ErrorResponse	"Failed to list"
//	@Security		BearerAuth
//	@Router			/auth/users [get]
func (h *Handler) listUsersHandler(c echo.Context) error {
	ctx := c.Request().Context()
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit <= 0 {
		limit = 50
	}
	offset, _ := strconv.Atoi(c.QueryParam("offset"))
	if offset < 0 {
		offset = 0
	}

	users, total, err := h.listUsers.Handle(ctx, limit, offset)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError,
			response.WrapError(dto.UserMessageCode, dto.ListUserFailed, nil))
	}

	items := make([]*dto.GetUserResponse, 0, len(users))
	for _, u := range users {
		items = append(items, &dto.GetUserResponse{
			ID:                     u.ID,
			IdentityProvider:       u.IdentityProvider,
			IdentityProviderUserID: u.IdentityProviderUserID,
			Username:               u.Username,
			Email:                  u.Email,
			FirstName:              u.FirstName,
			LastName:               u.LastName,
			EmailVerified:          u.EmailVerified,
			Enabled:                u.Enabled,
		})
	}

	return c.JSON(http.StatusOK, response.Wrap(dto.UserMessageCode, dto.ListUserSuccess,
		dto.ListUsersResponse{Users: items, Total: total}))
}
