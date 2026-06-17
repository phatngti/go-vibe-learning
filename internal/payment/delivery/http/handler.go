package http

import (
	"net/http"
	"time"

	"github/master_go_cqrs/internal/httpapi/route"
	"github/master_go_cqrs/internal/middleware"
	"github/master_go_cqrs/internal/observability/logger"
	"github/master_go_cqrs/internal/payment/application/command"
	"github/master_go_cqrs/internal/payment/application/dto"
	"github/master_go_cqrs/internal/payment/application/query"
	"github/master_go_cqrs/internal/security/gatewayauth"

	"github.com/labstack/echo/v4"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type Handler struct {
	logger          logger.Factory
	initiatePayment *command.InitiatePaymentHandler
	processPayment  *command.ProcessPaymentHandler
	getTransaction  *query.GetTransactionHandler
	authenticate    gatewayauth.Middleware
}

type HandlerParams struct {
	fx.In
	Logger          logger.Factory
	InitiatePayment *command.InitiatePaymentHandler
	ProcessPayment  *command.ProcessPaymentHandler
	GetTransaction  *query.GetTransactionHandler
	TokenAuth       gatewayauth.Middleware
}

func NewHandler(params HandlerParams) route.Registrar {
	return &Handler{
		logger:          params.Logger,
		initiatePayment: params.InitiatePayment,
		processPayment:  params.ProcessPayment,
		getTransaction:  params.GetTransaction,
		authenticate:    params.TokenAuth,
	}
}

func (h *Handler) Pattern(prefix *echo.Group) string {
	return ""
}

func (h *Handler) RegisterRoutes(g *echo.Group) {
	payment := g.Group("/payment")
	payment.POST("/initiate", h.Initiate,
		h.authenticate.Authorize(),
		middleware.ValidateHandler(new(dto.InitiatePayment), dto.MessageCode),
	)
	payment.GET("/checkout/:id", h.CheckoutPage)
	payment.POST("/checkout/:id", h.ProcessCheckout,
		middleware.ValidateHandler(new(dto.ProcessCheckoutRequest), dto.MessageCode),
	)
}

// Initiate godoc
//
//	@Summary		Initiate payment
//	@Description	Create a payment transaction and return checkout details.
//	@Tags			payment
//	@Accept			json
//	@Produce		json
//	@Param			request	body		dto.InitiatePayment	true	"Payment initiation payload"
//	@Success		201		{object}	dto.TransactionResponse
//	@Failure		400		{object}	map[string]string
//	@Failure		500		{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/payment/initiate [post]
func (h *Handler) Initiate(c echo.Context) error {
	req := c.Get("dto").(*dto.InitiatePayment)

	tx, err := h.initiatePayment.Handle(c.Request().Context(), req)
	if err != nil {
		h.logger.For(c.Request().Context()).Error("%v", err.Error()).WithFields(zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusCreated, dto.NewTransactionResponse(
		tx.ID,
		tx.AgentID,
		tx.CustomerID,
		tx.Amount,
		tx.Currency,
		string(tx.Status),
		tx.CallbackURL,
		tx.CreatedAt.Format(time.RFC3339),
		tx.UpdatedAt.Format(time.RFC3339),
	))
}

// CheckoutPage godoc
//
//	@Summary		Get checkout transaction
//	@Description	Return public checkout details for a transaction.
//	@Tags			payment
//	@Produce		json
//	@Param			id	path		string	true	"Transaction ID"
//	@Success		200	{object}	dto.CheckoutPageResponse
//	@Failure		404	{object}	map[string]string
//	@Router			/payment/checkout/{id} [get]
func (h *Handler) CheckoutPage(c echo.Context) error {
	id := c.Param("id")
	tx, err := h.getTransaction.Handle(c.Request().Context(), id)
	if err != nil {
		h.logger.For(c.Request().Context()).Error("%v", err.Error()).WithFields(zap.Error(err))
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Transaction not found"})
	}

	return c.JSON(http.StatusOK, dto.CheckoutPageResponse{
		Message:       "Welcome to Secure Checkout Page",
		TransactionID: tx.ID,
		Amount:        tx.Amount,
		Currency:      tx.Currency,
		AgentID:       tx.AgentID,
		Status:        string(tx.Status),
		Help:          "Submit a POST to this same URL with payment details to pay",
	})
}

// ProcessCheckout godoc
//
//	@Summary		Process checkout payment
//	@Description	Process a public checkout payment for an existing transaction.
//	@Tags			payment
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string						true	"Transaction ID"
//	@Param			request	body		dto.ProcessCheckoutRequest	true	"Checkout card payload"
//	@Success		200		{object}	dto.TransactionResponse
//	@Failure		400		{object}	map[string]string
//	@Failure		500		{object}	map[string]string
//	@Router			/payment/checkout/{id} [post]
func (h *Handler) ProcessCheckout(c echo.Context) error {
	id := c.Param("id")
	req := c.Get("dto").(*dto.ProcessCheckoutRequest)

	cmd := &dto.ProcessPayment{
		TransactionID: id,
		PaymentMethod: "credit_card",
		CardNumber:    req.CardNumber,
		CVV:           req.CVV,
	}

	tx, err := h.processPayment.Handle(c.Request().Context(), cmd)
	if err != nil {
		h.logger.For(c.Request().Context()).Error("%v", err.Error()).WithFields(zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, dto.NewTransactionResponse(
		tx.ID,
		tx.AgentID,
		tx.CustomerID,
		tx.Amount,
		tx.Currency,
		string(tx.Status),
		tx.CallbackURL,
		tx.CreatedAt.Format(time.RFC3339),
		tx.UpdatedAt.Format(time.RFC3339),
	))
}
