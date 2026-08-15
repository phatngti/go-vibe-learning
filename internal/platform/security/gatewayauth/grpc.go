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
	"/payment.v2.PaymentService/CreatePayment",
	"/payment.v2.PaymentService/GetPayment",
	"/affiliate.v1.AffiliateService/RegisterReferral",
	"/affiliate.v1.AffiliateService/GetAgentRank",
	"/affiliate.v1.AffiliateService/GetDownline",
	"/settlement.v1.SettlementService/GetAgentBalance",
	"/settlement.v1.SettlementService/RequestWithdrawal",
	"/settlement.v1.SettlementService/AdjustCreditLimit",
	"/agent.v1.AgentConfigService/ConfigureCallback",
	"/agent.v1.AgentConfigService/VerifyCallback",
	"/agent.v1.AgentConfigService/ReplayCallback",
	"/agent.v1.AgentConfigService/GenerateApiKey",
}

func UnaryServerInterceptor(authenticators ...PrincipalAuthenticator) grpc.UnaryServerInterceptor {
	authenticator := PrincipalAuthenticator(forwardedPayloadAuthenticator{})
	if len(authenticators) > 0 && authenticators[0] != nil {
		authenticator = authenticators[0]
	}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !slices.Contains(protectedUnaryMethods, info.FullMethod) {
			return handler(ctx, req)
		}

		user, err := userFromMetadata(ctx, authenticator)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "missing verified gateway JWT payload")
		}
		return handler(WithUser(ctx, user), req)
	}
}

func userFromMetadata(ctx context.Context, authenticator PrincipalAuthenticator) (User, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return User{}, status.Error(codes.Unauthenticated, "missing metadata")
	}
	payload := ""
	if values := md.Get(PayloadHeader); len(values) > 0 {
		payload = values[0]
	}
	authorization := ""
	if values := md.Get("authorization"); len(values) > 0 {
		authorization = values[0]
	}
	user, err := authenticator.Authenticate(ctx, authorization, payload)
	if err != nil {
		return User{}, status.Error(codes.Unauthenticated, "invalid service-boundary credentials")
	}
	return user, nil
}
