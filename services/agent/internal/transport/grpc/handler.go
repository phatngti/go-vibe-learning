package grpc

import (
	"context"
	"errors"
	"time"

	agentv1 "github/master_go_cqrs/contracts/genproto/agent"
	"github/master_go_cqrs/internal/platform/security/gatewayauth"
	"github/master_go_cqrs/services/agent/internal/application/command"
	"github/master_go_cqrs/services/agent/internal/application/dto"
	"github/master_go_cqrs/services/agent/internal/application/query"
	"github/master_go_cqrs/services/agent/internal/domain"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ServerHandler struct {
	agentv1.UnimplementedAgentConfigServiceServer
	configureCallback *command.ConfigureCallbackHandler
	verifyCallback    *command.VerifyCallbackHandler
	replayCallback    *command.ReplayCallbackHandler
	generateAPIKey    *command.GenerateAPIKeyHandler
	validateAPIKey    *query.ValidateAPIKeyHandler
	recordAuditLog    *command.RecordAuditLogHandler
}

func NewServerHandler(grpcServer *grpc.Server, configureCallback *command.ConfigureCallbackHandler, verifyCallback *command.VerifyCallbackHandler, replayCallback *command.ReplayCallbackHandler, generateAPIKey *command.GenerateAPIKeyHandler, validateAPIKey *query.ValidateAPIKeyHandler, recordAuditLog *command.RecordAuditLogHandler) *ServerHandler {
	h := &ServerHandler{
		configureCallback: configureCallback,
		verifyCallback:    verifyCallback,
		replayCallback:    replayCallback,
		generateAPIKey:    generateAPIKey,
		validateAPIKey:    validateAPIKey,
		recordAuditLog:    recordAuditLog,
	}
	agentv1.RegisterAgentConfigServiceServer(grpcServer, h)
	return h
}

func (h *ServerHandler) ReplayCallback(ctx context.Context, req *agentv1.ReplayCallbackRequest) (*agentv1.ReplayCallbackResponse, error) {
	principal, err := gatewayauth.RequireUser(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	delivery, err := h.replayCallback.Handle(ctx, &dto.ReplayCallback{
		EventID: req.EventId, Reason: req.Reason, TenantID: principal.TenantID,
		OperatorID: principal.ID, PlatformOperator: principal.HasRole("platform_operator"),
	})
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrCallbackReplayForbidden), errors.Is(err, domain.ErrTenantOwnership):
			return nil, status.Error(codes.PermissionDenied, err.Error())
		case errors.Is(err, domain.ErrCallbackDeliveryNotFound):
			return nil, status.Error(codes.NotFound, err.Error())
		case errors.Is(err, domain.ErrCallbackDeliveryNotDead):
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		default:
			return nil, status.Error(codes.Internal, "callback replay failed")
		}
	}
	return &agentv1.ReplayCallbackResponse{
		EventId: delivery.EventID, Status: delivery.Status,
		OutcomeGeneration: delivery.OutcomeGeneration, ReplayCount: int32(delivery.ReplayCount),
	}, nil
}

func RequireServerHandler(*ServerHandler) {}

func (h *ServerHandler) ConfigureCallback(ctx context.Context, req *agentv1.ConfigureCallbackRequest) (*agentv1.ConfigureCallbackResponse, error) {
	ownership, err := gatewayauth.RequireAgentOwnership(ctx, req.AgentId)
	if err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}
	configured, err := h.configureCallback.Handle(ctx, &dto.ConfigureCallback{
		AgentID: ownership.AgentID, TenantID: ownership.TenantID,
		CallbackURL: req.CallbackUrl, AllowedIPs: req.AllowedIps,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to configure callback settings: %v", err)
	}

	return &agentv1.ConfigureCallbackResponse{
		AgentId: ownership.AgentID, Success: true,
		Message:            "Callback ownership challenge created",
		CallbackStatus:     configured.Config.CallbackStatus,
		ChallengeId:        configured.Challenge.ChallengeID,
		ChallengeToken:     configured.Challenge.Token,
		ChallengeProofUrl:  configured.Challenge.ProofURL,
		ChallengeExpiresAt: configured.Challenge.ExpiresAt.Format(time.RFC3339),
	}, nil
}

func (h *ServerHandler) VerifyCallback(ctx context.Context, req *agentv1.VerifyCallbackRequest) (*agentv1.VerifyCallbackResponse, error) {
	ownership, err := gatewayauth.RequireAgentOwnership(ctx, req.AgentId)
	if err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}
	cfg, err := h.verifyCallback.Handle(ctx, &dto.VerifyCallback{
		AgentID: ownership.AgentID, TenantID: ownership.TenantID,
		ChallengeID: req.ChallengeId, ChallengeToken: req.ChallengeToken,
	})
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrTenantOwnership):
			return nil, status.Error(codes.PermissionDenied, err.Error())
		case errors.Is(err, domain.ErrCallbackChallengeReplayed):
			return nil, status.Error(codes.AlreadyExists, err.Error())
		case errors.Is(err, domain.ErrCallbackChallengeMissing),
			errors.Is(err, domain.ErrCallbackChallengeExpired),
			errors.Is(err, domain.ErrCallbackChallengeInvalid):
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		default:
			return nil, status.Error(codes.Internal, "callback verification failed")
		}
	}
	verifiedAt := ""
	if cfg.CallbackVerifiedAt != nil {
		verifiedAt = cfg.CallbackVerifiedAt.UTC().Format(time.RFC3339)
	}
	return &agentv1.VerifyCallbackResponse{
		AgentId: cfg.AgentID, Success: true, CallbackStatus: cfg.CallbackStatus,
		CallbackVerifiedAt: verifiedAt,
	}, nil
}

func (h *ServerHandler) GenerateApiKey(ctx context.Context, req *agentv1.GenerateApiKeyRequest) (*agentv1.GenerateApiKeyResponse, error) {
	ownership, err := gatewayauth.RequireAgentOwnership(ctx, req.AgentId)
	if err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}
	cfg, err := h.generateAPIKey.Handle(ctx, &dto.GenerateAPIKey{AgentID: ownership.AgentID, TenantID: ownership.TenantID})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to issue api credentials: %v", err)
	}

	return &agentv1.GenerateApiKeyResponse{
		AgentId:   cfg.AgentID,
		ApiKey:    cfg.ApiKey,
		CreatedAt: cfg.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}

func (h *ServerHandler) ValidateApiKey(ctx context.Context, req *agentv1.ValidateApiKeyRequest) (*agentv1.ValidateApiKeyResponse, error) {
	cfg, err := h.validateAPIKey.Handle(ctx, req.ApiKey)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "invalid credentials: %v", err)
	}

	return &agentv1.ValidateApiKeyResponse{
		Valid:   true,
		AgentId: cfg.AgentID,
	}, nil
}

func (h *ServerHandler) RecordAuditLog(ctx context.Context, req *agentv1.RecordAuditLogRequest) (*agentv1.RecordAuditLogResponse, error) {
	err := h.recordAuditLog.Handle(ctx, &dto.RecordAuditLog{AgentID: req.AgentId, Action: req.Action, Details: req.Details})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to store audit log: %v", err)
	}

	return &agentv1.RecordAuditLogResponse{
		Success: true,
	}, nil
}
