package grpc

import (
	"context"
	"errors"
	"strings"

	paymentv2 "github/master_go_cqrs/contracts/genproto/paymentv2"
	"github/master_go_cqrs/internal/platform/security/gatewayauth"
	sharedmoney "github/master_go_cqrs/internal/shared/money"
	"github/master_go_cqrs/services/payment/internal/application/command"
	"github/master_go_cqrs/services/payment/internal/application/dto"
	"github/master_go_cqrs/services/payment/internal/application/query"
	"github/master_go_cqrs/services/payment/internal/domain"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type V2ServerHandler struct {
	paymentv2.UnimplementedPaymentServiceServer
	createPayment *command.CreatePaymentV2Handler
	getPayment    *query.GetPaymentV2Handler
}

func NewV2ServerHandler(grpcServer *grpc.Server, createPayment *command.CreatePaymentV2Handler, getPayment *query.GetPaymentV2Handler) *V2ServerHandler {
	handler := &V2ServerHandler{createPayment: createPayment, getPayment: getPayment}
	paymentv2.RegisterPaymentServiceServer(grpcServer, handler)
	return handler
}

func RequireV2ServerHandler(*V2ServerHandler) {}

func (h *V2ServerHandler) CreatePayment(ctx context.Context, request *paymentv2.CreatePaymentRequest) (*paymentv2.CreatePaymentResponse, error) {
	ownerID, err := authenticatedOwner(ctx)
	if err != nil {
		return nil, err
	}
	if request.GetMoney() == nil {
		return nil, status.Error(codes.InvalidArgument, "money is required")
	}
	value, err := sharedmoney.New(request.Money.AmountMinor, request.Money.Currency, request.Money.Exponent)
	if err != nil || !value.IsPositive() {
		return nil, status.Errorf(codes.InvalidArgument, "invalid money: %v", err)
	}
	result, err := h.createPayment.Handle(ctx, dto.CreatePaymentV2{
		AgentID: ownerID, Money: value, CustomerReference: request.CustomerReference,
		CallbackEndpointID: request.CallbackEndpointId, Metadata: request.Metadata,
		IdempotencyKey: incomingIdempotencyKey(ctx),
	})
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInitiationIdempotencyRequired):
			return nil, status.Error(codes.InvalidArgument, "Idempotency-Key metadata is required")
		case errors.Is(err, domain.ErrInitiationIdempotencyConflict):
			return nil, status.Error(codes.AlreadyExists, "idempotency key conflicts with another request")
		}
		return nil, status.Errorf(codes.Internal, "create payment: %v", err)
	}
	return &paymentv2.CreatePaymentResponse{
		TransactionId: result.Transaction.ID, Provider: result.Provider,
		ClientSecret: result.ClientSecret, Status: result.Transaction.Status,
		Money: toProtoMoney(result.Transaction.Money),
	}, nil
}

func (h *V2ServerHandler) GetPayment(ctx context.Context, request *paymentv2.GetPaymentRequest) (*paymentv2.GetPaymentResponse, error) {
	ownerID, err := authenticatedOwner(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(request.TransactionId) == "" {
		return nil, status.Error(codes.InvalidArgument, "transaction_id is required")
	}
	tx, err := h.getPayment.Handle(ctx, request.TransactionId, ownerID)
	if err != nil {
		if errors.Is(err, domain.ErrTransactionNotFound) {
			return nil, status.Error(codes.NotFound, "payment not found")
		}
		return nil, status.Errorf(codes.Internal, "get payment: %v", err)
	}
	value, err := tx.MonetaryAmount()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "payment money is invalid: %v", err)
	}
	return &paymentv2.GetPaymentResponse{
		TransactionId: tx.ID, CustomerReference: tx.CustomerID,
		CallbackEndpointId: tx.CallbackEndpointID, Provider: tx.Provider,
		ProviderReference: tx.ProviderReference, ProviderStatus: tx.ProviderStatus,
		Status: string(tx.Status), Money: toProtoMoney(value),
		CreatedAt: tx.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: tx.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}

func authenticatedOwner(ctx context.Context) (string, error) {
	owner, err := gatewayauth.RequireAgentOwnership(ctx, "")
	if err != nil {
		if errors.Is(err, gatewayauth.ErrOwnershipMismatch) {
			return "", status.Error(codes.PermissionDenied, "authenticated principal does not own the requested resource")
		}
		return "", status.Error(codes.Unauthenticated, "verified tenant and agent identity is required")
	}
	return owner.AgentID, nil
}

func incomingIdempotencyKey(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	for _, key := range []string{"idempotency-key", "x-idempotency-key"} {
		if values := md.Get(key); len(values) > 0 {
			return strings.TrimSpace(values[0])
		}
	}
	return ""
}

func toProtoMoney(value sharedmoney.Money) *paymentv2.Money {
	return &paymentv2.Money{AmountMinor: value.AmountMinor, Currency: value.Currency, Exponent: value.Exponent}
}
