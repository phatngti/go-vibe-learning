package gatewayauth

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestUnaryServerInterceptorProtectsConfiguredMethod(t *testing.T) {
	payload := testPayload(t)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(PayloadHeader, payload))
	interceptor := UnaryServerInterceptor()

	_, err := interceptor(ctx, "request", &grpc.UnaryServerInfo{FullMethod: "/payment.v1.PaymentService/InitiatePayment"}, func(ctx context.Context, req any) (any, error) {
		user, ok := ctx.Value(userContextKey{}).(User)
		if !ok || user.ID != "user-1" {
			t.Fatalf("context user = %+v, %v; want user-1", user, ok)
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("interceptor error = %v", err)
	}
}

func TestUnaryServerInterceptorLeavesInternalMethodUnblocked(t *testing.T) {
	interceptor := UnaryServerInterceptor()

	_, err := interceptor(context.Background(), "request", &grpc.UnaryServerInfo{FullMethod: "/payment.v1.PaymentService/ProcessPayment"}, func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("interceptor error = %v", err)
	}
}
