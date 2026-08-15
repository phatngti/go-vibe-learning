package grpc

import (
	"context"

	affiliatev1 "github/master_go_cqrs/contracts/genproto/affiliate"
	"github/master_go_cqrs/internal/platform/security/gatewayauth"
	"github/master_go_cqrs/services/affiliate/internal/application/command"
	"github/master_go_cqrs/services/affiliate/internal/application/dto"
	"github/master_go_cqrs/services/affiliate/internal/application/query"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ServerHandler struct {
	affiliatev1.UnimplementedAffiliateServiceServer
	registerReferral *command.RegisterReferralHandler
	getRank          *query.GetRankHandler
	getDownline      *query.GetDownlineHandler
}

func NewServerHandler(grpcServer *grpc.Server, registerReferral *command.RegisterReferralHandler, getRank *query.GetRankHandler, getDownline *query.GetDownlineHandler) *ServerHandler {
	h := &ServerHandler{
		registerReferral: registerReferral,
		getRank:          getRank,
		getDownline:      getDownline,
	}
	affiliatev1.RegisterAffiliateServiceServer(grpcServer, h)
	return h
}

func RequireServerHandler(*ServerHandler) {}

func (h *ServerHandler) RegisterReferral(ctx context.Context, req *affiliatev1.RegisterReferralRequest) (*affiliatev1.RegisterReferralResponse, error) {
	owner, err := gatewayauth.RequireAgentOwnership(ctx, req.ReferrerId)
	if err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}
	err = h.registerReferral.Handle(ctx, &dto.RegisterReferral{ReferrerID: owner.AgentID, RefereeID: req.RefereeId})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to register referral: %v", err)
	}

	return &affiliatev1.RegisterReferralResponse{
		Success: true,
		Message: "Referral relationship linked successfully",
	}, nil
}

func (h *ServerHandler) GetAgentRank(ctx context.Context, req *affiliatev1.GetAgentRankRequest) (*affiliatev1.GetAgentRankResponse, error) {
	owner, err := gatewayauth.RequireAgentOwnership(ctx, req.AgentId)
	if err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}
	level, metrics, err := h.getRank.Handle(ctx, owner.AgentID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "agent not found: %v", err)
	}

	nextThreshold := 0.0
	switch level.CurrentRank {
	case "BRONZE":
		nextThreshold = 1000.0
	case "SILVER":
		nextThreshold = 10000.0
	case "GOLD":
		nextThreshold = 100000.0
	case "PLATINUM":
		nextThreshold = 1000000.0
	case "DIAMOND":
		nextThreshold = 0.0 // Max rank
	}

	return &affiliatev1.GetAgentRankResponse{
		AgentId:           level.AgentID,
		CurrentRank:       string(level.CurrentRank),
		NextRankThreshold: nextThreshold,
		ProgressVolume:    metrics.AccumulatedVolume,
	}, nil
}

func (h *ServerHandler) GetDownline(ctx context.Context, req *affiliatev1.GetDownlineRequest) (*affiliatev1.GetDownlineResponse, error) {
	owner, err := gatewayauth.RequireAgentOwnership(ctx, req.AgentId)
	if err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}
	referees, err := h.getDownline.Handle(ctx, owner.AgentID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to query downline: %v", err)
	}

	return &affiliatev1.GetDownlineResponse{
		AgentId:        owner.AgentID,
		RefereeIds:     referees,
		TotalReferrals: int32(len(referees)),
	}, nil
}
