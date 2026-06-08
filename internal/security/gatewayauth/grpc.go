package gatewayauth

import (
	"context"
	"slices"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var protectedUnaryMethods = []string{
	"/payment.v1.PaymentService/InitiatePayment",
	"/affiliate.v1.AffiliateService/RegisterReferral",
	"/affiliate.v1.AffiliateService/GetAgentRank",
	"/affiliate.v1.AffiliateService/GetDownline",
	"/settlement.v1.SettlementService/GetAgentBalance",
	"/settlement.v1.SettlementService/RequestWithdrawal",
	"/settlement.v1.SettlementService/AdjustCreditLimit",
	"/agent.v1.AgentConfigService/ConfigureCallback",
	"/agent.v1.AgentConfigService/GenerateApiKey",
}

func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !slices.Contains(protectedUnaryMethods, info.FullMethod) {
			return handler(ctx, req)
		}

		user, err := userFromMetadata(ctx)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "missing verified gateway JWT payload")
		}
		return handler(WithUser(ctx, user), req)
	}
}

func userFromMetadata(ctx context.Context) (User, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return User{}, status.Error(codes.Unauthenticated, "missing metadata")
	}
	values := md.Get(PayloadHeader)
	if len(values) == 0 {
		return User{}, status.Error(codes.Unauthenticated, "missing gateway JWT payload")
	}
	return DecodeUserPayload(values[0])
}
