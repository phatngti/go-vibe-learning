package middleware

import (
	"context"
	"time"

	"github/master_go_cqrs/internal/observability/logger"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

func UnaryRequestLogger(logger logger.Factory) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		start := time.Now()
		resp, err = handler(ctx, req)
		logGRPCRequest(logger, ctx, info.FullMethod, time.Since(start), err)
		return resp, err
	}
}

func StreamRequestLogger(logger logger.Factory) grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		err := handler(srv, stream)
		logGRPCRequest(logger, stream.Context(), info.FullMethod, time.Since(start), err)
		return err
	}
}

func logGRPCRequest(logger logger.Factory, ctx context.Context, method string, duration time.Duration, err error) {
	code := status.Code(err)
	remoteAddr := ""
	if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
		remoteAddr = p.Addr.String()
	}

	logger.For(ctx).WithShowTerminal(true).Info("gRPC method=%s, code=%s, remote_addr=%s, time=%s",
		method,
		code.String(),
		remoteAddr,
		duration.String(),
	).WithFields(
		zap.String("grpc_method", method),
		zap.String("grpc_code", code.String()),
		zap.String("remote_addr", remoteAddr),
		zap.Duration("duration", duration),
	)
}
