package http

import (
	"net/http"

	"github/master_go_cqrs/internal/httpapi/route"
	"github/master_go_cqrs/internal/middleware"
	"github/master_go_cqrs/internal/observability/logger"
	"github/master_go_cqrs/internal/security/gatewayauth"
	"github/master_go_cqrs/internal/settlement/application/command"
	"github/master_go_cqrs/internal/settlement/application/dto"
	"github/master_go_cqrs/internal/settlement/application/query"

	"github.com/labstack/echo/v4"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type Handler struct {
	logger            logger.Factory
	getBalance        *query.GetBalanceHandler
	requestWithdrawal *command.RequestWithdrawalHandler
	adjustCreditLimit *command.AdjustCreditLimitHandler
	authenticate      gatewayauth.Middleware
}

type HandlerParams struct {
	fx.In
	Logger            logger.Factory
	GetBalance        *query.GetBalanceHandler
	RequestWithdrawal *command.RequestWithdrawalHandler
	AdjustCreditLimit *command.AdjustCreditLimitHandler
	TokenAuth         gatewayauth.Middleware
}

func NewHandler(params HandlerParams) route.Registrar {
	return &Handler{
		logger:            params.Logger,
		getBalance:        params.GetBalance,
		requestWithdrawal: params.RequestWithdrawal,
		adjustCreditLimit: params.AdjustCreditLimit,
		authenticate:      params.TokenAuth,
	}
}

func (h *Handler) Pattern(prefix *echo.Group) string {
	return ""
}

func (h *Handler) RegisterRoutes(g *echo.Group) {
	settlement := g.Group("/settlement")
	settlement.GET("/balance/:id", h.GetBalance,
		h.authenticate.Authorize(),
	)
	settlement.POST("/withdraw", h.RequestWithdrawal,
		h.authenticate.Authorize(),
		middleware.ValidateHandler(new(dto.RequestWithdrawal), dto.MessageCode),
	)
	settlement.POST("/credit/limit", h.AdjustCreditLimit,
		h.authenticate.Authorize(),
		middleware.ValidateHandler(new(dto.AdjustCreditLimit), dto.MessageCode),
	)
}

// GetBalance godoc
//
//	@Summary		Get settlement balance
//	@Description	Return active balance, credit limit, and maximum withdrawal for an agent.
//	@Tags			settlement
//	@Produce		json
//	@Param			id	path		string	true	"Agent ID"
//	@Success		200	{object}	dto.BalanceResponse
//	@Failure		404	{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/settlement/balance/{id} [get]
func (h *Handler) GetBalance(c echo.Context) error {
	id := c.Param("id")
	bal, err := h.getBalance.Handle(c.Request().Context(), id)
	if err != nil {
		h.logger.For(c.Request().Context()).Error("%v", err.Error()).WithFields(zap.Error(err))
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Agent balance details not found"})
	}

	return c.JSON(http.StatusOK, dto.BalanceResponse{
		AgentID:       bal.AgentID,
		ActiveBalance: bal.ActiveBalance,
		CreditLimit:   bal.CreditLimit,
		MaxWithdrawal: bal.ActiveBalance + bal.CreditLimit,
	})
}

// RequestWithdrawal godoc
//
//	@Summary		Request settlement withdrawal
//	@Description	Request a withdrawal from an agent balance.
//	@Tags			settlement
//	@Accept			json
//	@Produce		json
//	@Param			request	body		dto.RequestWithdrawal	true	"Withdrawal payload"
//	@Success		200		{object}	dto.WithdrawalResponse
//	@Failure		400		{object}	map[string]string
//	@Failure		412		{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/settlement/withdraw [post]
func (h *Handler) RequestWithdrawal(c echo.Context) error {
	req := c.Get("dto").(*dto.RequestWithdrawal)

	bal, err := h.requestWithdrawal.Handle(c.Request().Context(), req)
	if err != nil {
		h.logger.For(c.Request().Context()).Error("%v", err.Error()).WithFields(zap.Error(err))
		return c.JSON(http.StatusPreconditionFailed, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, dto.WithdrawalResponse{
		AgentID:       bal.AgentID,
		Amount:        req.Amount,
		Status:        "SUCCESS",
		ActiveBalance: bal.ActiveBalance,
	})
}

// AdjustCreditLimit godoc
//
//	@Summary		Adjust settlement credit limit
//	@Description	Update an agent credit limit.
//	@Tags			settlement
//	@Accept			json
//	@Produce		json
//	@Param			request	body		dto.AdjustCreditLimit	true	"Credit limit payload"
//	@Success		200		{object}	dto.CreditLimitResponse
//	@Failure		400		{object}	map[string]string
//	@Failure		500		{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/settlement/credit/limit [post]
func (h *Handler) AdjustCreditLimit(c echo.Context) error {
	req := c.Get("dto").(*dto.AdjustCreditLimit)

	bal, err := h.adjustCreditLimit.Handle(c.Request().Context(), req)
	if err != nil {
		h.logger.For(c.Request().Context()).Error("%v", err.Error()).WithFields(zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, dto.CreditLimitResponse{
		AgentID:       bal.AgentID,
		ActiveBalance: bal.ActiveBalance,
		CreditLimit:   bal.CreditLimit,
	})
}
