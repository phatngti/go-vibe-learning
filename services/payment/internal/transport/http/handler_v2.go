package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github/master_go_cqrs/internal/platform/httpapi/route"
	"github/master_go_cqrs/internal/platform/security/gatewayauth"
	sharedmoney "github/master_go_cqrs/internal/shared/money"
	"github/master_go_cqrs/services/payment/internal/application/command"
	"github/master_go_cqrs/services/payment/internal/application/dto"
	"github/master_go_cqrs/services/payment/internal/application/query"
	"github/master_go_cqrs/services/payment/internal/config"
	"github/master_go_cqrs/services/payment/internal/domain"

	"github.com/labstack/echo/v4"
	"go.uber.org/fx"
)

const maxWebhookBodyBytes = 1 << 20

type V2Handler struct {
	createPayment    *command.CreatePaymentV2Handler
	getPayment       *query.GetPaymentV2Handler
	handleWebhook    *command.HandleProviderWebhookHandler
	authenticate     gatewayauth.Middleware
	selectedProvider string
}

type V2HandlerParams struct {
	fx.In

	CreatePayment *command.CreatePaymentV2Handler
	GetPayment    *query.GetPaymentV2Handler
	HandleWebhook *command.HandleProviderWebhookHandler
	TokenAuth     gatewayauth.Middleware
	Config        *config.Config
}

func NewV2Handler(params V2HandlerParams) route.Registrar {
	return &V2Handler{
		createPayment: params.CreatePayment, getPayment: params.GetPayment,
		handleWebhook: params.HandleWebhook, authenticate: params.TokenAuth,
		selectedProvider: strings.ToLower(strings.TrimSpace(params.Config.Provider.Kind)),
	}
}

func (h *V2Handler) Pattern(*echo.Group) string { return "/payments" }

func (h *V2Handler) RegisterRoutes(group *echo.Group) {
	group.POST("", h.Create, h.authenticate.Authorize())
	group.GET("/:transaction_id", h.Get, h.authenticate.Authorize())
	// Stripe verifies a signature over the exact bytes. This endpoint therefore
	// remains raw HTTP and must not pass through JSON transcoding.
	group.POST("/webhooks/stripe", h.StripeWebhook)
}

type createPaymentV2Request struct {
	Money              sharedmoney.Money `json:"money" swaggertype:"object"`
	CustomerReference  string            `json:"customer_reference"`
	CallbackEndpointID string            `json:"callback_endpoint_id"`
	Metadata           map[string]string `json:"metadata,omitempty"`
}

type createPaymentV2Response struct {
	TransactionID string            `json:"transaction_id"`
	Provider      string            `json:"provider"`
	ClientSecret  string            `json:"client_secret"`
	Status        string            `json:"status"`
	Money         sharedmoney.Money `json:"money" swaggertype:"object"`
}

type getPaymentV2Response struct {
	TransactionID      string            `json:"transaction_id"`
	CustomerReference  string            `json:"customer_reference"`
	Money              sharedmoney.Money `json:"money" swaggertype:"object"`
	Status             string            `json:"status"`
	Provider           string            `json:"provider"`
	ProviderReference  string            `json:"provider_reference"`
	ProviderStatus     string            `json:"provider_status"`
	CallbackEndpointID string            `json:"callback_endpoint_id"`
	CreatedAt          string            `json:"created_at"`
	UpdatedAt          string            `json:"updated_at"`
}

func (h *V2Handler) Create(c echo.Context) error {
	owner, err := gatewayauth.RequireAgentOwnership(c.Request().Context(), "")
	if err != nil {
		return ownershipHTTPError(err)
	}
	var request createPaymentV2Request
	decoder := json.NewDecoder(c.Request().Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid payment request"})
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid payment request"})
	}
	result, err := h.createPayment.Handle(c.Request().Context(), dto.CreatePaymentV2{
		AgentID: owner.AgentID, Money: request.Money,
		CustomerReference: request.CustomerReference, CallbackEndpointID: request.CallbackEndpointID,
		Metadata: request.Metadata, IdempotencyKey: c.Request().Header.Get("Idempotency-Key"),
	})
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInitiationIdempotencyRequired):
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Idempotency-Key header is required"})
		case errors.Is(err, domain.ErrInitiationIdempotencyConflict):
			return c.JSON(http.StatusConflict, map[string]string{"error": "idempotency key conflicts with another request"})
		default:
			return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "payment intent could not be created"})
		}
	}
	c.Response().Header().Set(echo.HeaderCacheControl, "no-store")
	return c.JSON(http.StatusCreated, createPaymentV2Response{
		TransactionID: result.Transaction.ID, Provider: result.Provider,
		ClientSecret: result.ClientSecret, Status: result.Transaction.Status,
		Money: result.Transaction.Money,
	})
}

func (h *V2Handler) Get(c echo.Context) error {
	owner, err := gatewayauth.RequireAgentOwnership(c.Request().Context(), "")
	if err != nil {
		return ownershipHTTPError(err)
	}
	tx, err := h.getPayment.Handle(c.Request().Context(), c.Param("transaction_id"), owner.AgentID)
	if err != nil {
		if errors.Is(err, domain.ErrTransactionNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "payment not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "payment could not be loaded"})
	}
	value, err := tx.MonetaryAmount()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "payment money is invalid"})
	}
	return c.JSON(http.StatusOK, getPaymentV2Response{
		TransactionID: tx.ID, CustomerReference: tx.CustomerID, Money: value,
		Status: string(tx.Status), Provider: tx.Provider, ProviderReference: tx.ProviderReference,
		ProviderStatus: tx.ProviderStatus, CallbackEndpointID: tx.CallbackEndpointID,
		CreatedAt: tx.CreatedAt.Format(time.RFC3339), UpdatedAt: tx.UpdatedAt.Format(time.RFC3339),
	})
}

func (h *V2Handler) StripeWebhook(c echo.Context) error {
	if h.selectedProvider != "stripe" {
		return c.NoContent(http.StatusNotFound)
	}
	body, err := io.ReadAll(io.LimitReader(c.Request().Body, maxWebhookBodyBytes+1))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid webhook body"})
	}
	if len(body) > maxWebhookBodyBytes {
		return c.JSON(http.StatusRequestEntityTooLarge, map[string]string{"error": "webhook body is too large"})
	}
	result, err := h.handleWebhook.Handle(
		c.Request().Context(), body, c.Request().Header.Get("Stripe-Signature"), time.Now(),
	)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "webhook verification or processing failed"})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"received": true, "event_id": result.EventID, "duplicate": result.Duplicate,
	})
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("multiple JSON values")
		}
		return err
	}
	return nil
}

func ownershipHTTPError(err error) *echo.HTTPError {
	if errors.Is(err, gatewayauth.ErrOwnershipMismatch) {
		return echo.NewHTTPError(http.StatusForbidden, "authenticated principal does not own the requested resource")
	}
	return echo.NewHTTPError(http.StatusUnauthorized, "verified tenant and agent identity is required")
}
