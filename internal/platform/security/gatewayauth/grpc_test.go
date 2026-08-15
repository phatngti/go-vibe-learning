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

func TestUnaryServerInterceptorProtectsPaymentV2Methods(t *testing.T) {
	payload := testPayload(t)
	interceptor := UnaryServerInterceptor()

	for _, method := range []string{
		"/payment.v2.PaymentService/CreatePayment",
		"/payment.v2.PaymentService/GetPayment",
	} {
		t.Run(method, func(t *testing.T) {
			ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(PayloadHeader, payload))
			_, err := interceptor(ctx, "request", &grpc.UnaryServerInfo{FullMethod: method}, func(ctx context.Context, req any) (any, error) {
				if _, ok := FromContext(ctx); !ok {
					t.Fatal("verified gateway principal is missing from context")
				}
				return "ok", nil
			})
			if err != nil {
				t.Fatalf("interceptor error = %v", err)
			}
		})
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
