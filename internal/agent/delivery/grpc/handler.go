package grpc

import (
	"context"

	"github/master_go_cqrs/internal/agent/application/command"
	"github/master_go_cqrs/internal/agent/application/dto"
	"github/master_go_cqrs/internal/agent/application/query"
	agentv1 "github/master_go_cqrs/internal/genproto/agent"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ServerHandler struct {
	agentv1.UnimplementedAgentConfigServiceServer
	configureCallback *command.ConfigureCallbackHandler
	generateAPIKey    *command.GenerateAPIKeyHandler
	validateAPIKey    *query.ValidateAPIKeyHandler
	recordAuditLog    *command.RecordAuditLogHandler
}

func NewServerHandler(grpcServer *grpc.Server, configureCallback *command.ConfigureCallbackHandler, generateAPIKey *command.GenerateAPIKeyHandler, validateAPIKey *query.ValidateAPIKeyHandler, recordAuditLog *command.RecordAuditLogHandler) *ServerHandler {
	h := &ServerHandler{
		configureCallback: configureCallback,
		generateAPIKey:    generateAPIKey,
		validateAPIKey:    validateAPIKey,
		recordAuditLog:    recordAuditLog,
	}
	agentv1.RegisterAgentConfigServiceServer(grpcServer, h)
	return h
}

func RequireServerHandler(*ServerHandler) {}

func (h *ServerHandler) ConfigureCallback(ctx context.Context, req *agentv1.ConfigureCallbackRequest) (*agentv1.ConfigureCallbackResponse, error) {
	_, err := h.configureCallback.Handle(ctx, &dto.ConfigureCallback{AgentID: req.AgentId, CallbackURL: req.CallbackUrl, AllowedIPs: req.AllowedIps})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to configure callback settings: %v", err)
	}

	return &agentv1.ConfigureCallbackResponse{
		AgentId: req.AgentId,
		Success: true,
		Message: "Callback configurations updated successfully",
	}, nil
}

func (h *ServerHandler) GenerateApiKey(ctx context.Context, req *agentv1.GenerateApiKeyRequest) (*agentv1.GenerateApiKeyResponse, error) {
	cfg, err := h.generateAPIKey.Handle(ctx, &dto.GenerateAPIKey{AgentID: req.AgentId})
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
