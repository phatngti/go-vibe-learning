package grpc

import (
	"context"
	"errors"
	"strings"

	paymentv1 "github/master_go_cqrs/contracts/genproto/payment"
	"github/master_go_cqrs/internal/platform/security/gatewayauth"
	"github/master_go_cqrs/services/payment/internal/application/command"
	"github/master_go_cqrs/services/payment/internal/application/dto"
	"github/master_go_cqrs/services/payment/internal/application/query"
	"github/master_go_cqrs/services/payment/internal/config"
	"github/master_go_cqrs/services/payment/internal/domain"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ServerHandler struct {
	paymentv1.UnimplementedPaymentServiceServer
	initiatePayment *command.InitiatePaymentHandler
	processPayment  *command.ProcessPaymentHandler
	getTransaction  *query.GetTransactionHandler
	publicBaseURL   string
	allowRawCard    bool
}

func NewServerHandler(grpcServer *grpc.Server, cfg *config.Config, initiatePayment *command.InitiatePaymentHandler, processPayment *command.ProcessPaymentHandler, getTransaction *query.GetTransactionHandler) *ServerHandler {
	publicBaseURL := strings.TrimRight(cfg.Gateway.PublicBaseURL, "/")
	if publicBaseURL == "" {
		publicBaseURL = "http://localhost:8888"
	}
	h := &ServerHandler{
		initiatePayment: initiatePayment,
		processPayment:  processPayment,
		getTransaction:  getTransaction,
		publicBaseURL:   publicBaseURL,
		allowRawCard:    cfg.RawCardEnabled(),
	}
	paymentv1.RegisterPaymentServiceServer(grpcServer, h)
	return h
}

func RequireServerHandler(*ServerHandler) {}

func (h *ServerHandler) InitiatePayment(ctx context.Context, req *paymentv1.InitiatePaymentRequest) (*paymentv1.InitiatePaymentResponse, error) {
	owner, err := gatewayauth.RequireAgentOwnership(ctx, req.AgentId)
	if err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}
	cmd := &dto.InitiatePayment{
		AgentID:        owner.AgentID,
		Amount:         req.Amount,
		Currency:       req.Currency,
		CustomerID:     req.CustomerId,
		CallbackURL:    req.CallbackUrl,
		IdempotencyKey: req.IdempotencyKey,
	}

	tx, err := h.initiatePayment.Handle(ctx, cmd)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInitiationIdempotencyRequired):
			return nil, status.Errorf(codes.InvalidArgument, "initiation idempotency key is required")
		case errors.Is(err, domain.ErrInitiationIdempotencyConflict):
			return nil, status.Errorf(codes.AlreadyExists, "initiation idempotency conflict: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to initiate payment: %v", err)
	}

	return &paymentv1.InitiatePaymentResponse{
		TransactionId: tx.ID,
		PaymentUrl:    h.publicBaseURL + "/api/v1/payment/checkout/" + tx.ID,
		Status:        string(tx.Status),
	}, nil
}

func (h *ServerHandler) ProcessPayment(ctx context.Context, req *paymentv1.ProcessPaymentRequest) (*paymentv1.ProcessPaymentResponse, error) {
	if !h.allowRawCard {
		return nil, status.Error(codes.Unimplemented, "legacy raw-card checkout is disabled")
	}
	cmd := &dto.ProcessPayment{
		TransactionID:  req.TransactionId,
		PaymentMethod:  req.PaymentMethod,
		CardNumber:     req.CardNumber,
		CVV:            req.Cvv,
		IdempotencyKey: req.IdempotencyKey,
	}

	tx, err := h.processPayment.Handle(ctx, cmd)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrPaymentInProgress):
			return nil, status.Errorf(codes.Aborted, "payment processing is already in progress: %v", err)
		case errors.Is(err, domain.ErrIdempotencyConflict):
			return nil, status.Errorf(codes.AlreadyExists, "payment idempotency conflict: %v", err)
		case errors.Is(err, domain.ErrTransactionNotFound):
			return nil, status.Errorf(codes.NotFound, "transaction not found: %v", err)
		}
		return nil, status.Errorf(codes.InvalidArgument, "failed to process payment: %v", err)
	}

	return &paymentv1.ProcessPaymentResponse{
		TransactionId: tx.ID,
		Status:        string(tx.Status),
		Message:       "Payment processing result recorded",
	}, nil
}

func (h *ServerHandler) GetTransactionStatus(ctx context.Context, req *paymentv1.GetTransactionStatusRequest) (*paymentv1.GetTransactionStatusResponse, error) {
	tx, err := h.getTransaction.Handle(ctx, req.TransactionId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "transaction not found: %v", err)
	}

	return &paymentv1.GetTransactionStatusResponse{
		TransactionId: tx.ID,
		Status:        string(tx.Status),
		Amount:        tx.Amount,
		Currency:      tx.Currency,
		AgentId:       tx.AgentID,
		CreatedAt:     tx.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}
