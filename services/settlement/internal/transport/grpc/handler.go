package grpc

import (
	"context"

	settlementv1 "github/master_go_cqrs/contracts/genproto/settlement"
	"github/master_go_cqrs/services/settlement/internal/application/command"
	"github/master_go_cqrs/services/settlement/internal/application/dto"
	"github/master_go_cqrs/services/settlement/internal/application/query"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ServerHandler struct {
	settlementv1.UnimplementedSettlementServiceServer
	getBalance        *query.GetBalanceHandler
	requestWithdrawal *command.RequestWithdrawalHandler
	adjustCreditLimit *command.AdjustCreditLimitHandler
}

func NewServerHandler(grpcServer *grpc.Server, getBalance *query.GetBalanceHandler, requestWithdrawal *command.RequestWithdrawalHandler, adjustCreditLimit *command.AdjustCreditLimitHandler) *ServerHandler {
	h := &ServerHandler{
		getBalance:        getBalance,
		requestWithdrawal: requestWithdrawal,
		adjustCreditLimit: adjustCreditLimit,
	}
	settlementv1.RegisterSettlementServiceServer(grpcServer, h)
	return h
}

func RequireServerHandler(*ServerHandler) {}

func (h *ServerHandler) GetAgentBalance(ctx context.Context, req *settlementv1.GetAgentBalanceRequest) (*settlementv1.GetAgentBalanceResponse, error) {
	bal, err := h.getBalance.Handle(ctx, req.AgentId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "agent balance not found: %v", err)
	}

	return &settlementv1.GetAgentBalanceResponse{
		AgentId:       bal.AgentID,
		ActiveBalance: bal.ActiveBalance,
		CreditLimit:   bal.CreditLimit,
		LastUpdated:   bal.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}

func (h *ServerHandler) RequestWithdrawal(ctx context.Context, req *settlementv1.RequestWithdrawalRequest) (*settlementv1.RequestWithdrawalResponse, error) {
	bal, err := h.requestWithdrawal.Handle(ctx, &dto.RequestWithdrawal{AgentID: req.AgentId, Amount: req.Amount})
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "withdrawal rejected: %v", err)
	}

	return &settlementv1.RequestWithdrawalResponse{
		WithdrawalId: "withdraw-tx-" + req.AgentId,
		AgentId:      bal.AgentID,
		Amount:       req.Amount,
		Status:       "PROCESSED",
		Message:      "Withdrawal completed successfully",
	}, nil
}

func (h *ServerHandler) AdjustCreditLimit(ctx context.Context, req *settlementv1.AdjustCreditLimitRequest) (*settlementv1.AdjustCreditLimitResponse, error) {
	bal, err := h.adjustCreditLimit.Handle(ctx, &dto.AdjustCreditLimit{AgentID: req.AgentId, Limit: req.Limit})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to adjust credit limit: %v", err)
	}

	return &settlementv1.AdjustCreditLimitResponse{
		AgentId:  bal.AgentID,
		NewLimit: bal.CreditLimit,
		Success:  true,
	}, nil
}
