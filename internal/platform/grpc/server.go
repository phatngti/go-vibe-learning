// File: internal/platform/grpc/server.go
// Description: Instantiates and controls the lifecycle of the global gRPC server with trace and log interceptors.

package grpc

import (
	"context"
	"net"
	"strconv"

	"github/master_go_cqrs/internal/platform/config"
	appmiddleware "github/master_go_cqrs/internal/platform/middleware"
	"github/master_go_cqrs/internal/platform/observability/logger"
	"github/master_go_cqrs/internal/platform/security/gatewayauth"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type Server struct {
	GRPCServer *grpc.Server
	logger     logger.Factory
	port       int
}

func NewServer(lifecycle fx.Lifecycle, cfg *config.Server, l logger.Factory) (*Server, error) {
	s := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(gatewayauth.UnaryServerInterceptor(), appmiddleware.UnaryRequestLogger(l)),
		grpc.ChainStreamInterceptor(appmiddleware.StreamRequestLogger(l)),
	)

	// Default port or dynamic depending on service
	// (we can allow microservices to supply their own port via Fx)
	// For simplicity, we can load a port or inject it. Let's make it load from a default or custom config.
	port := cfg.Port + 1000 // Offset gRPC port by 1000 from Echo port

	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			addr := ":" + strconv.Itoa(port)
			listener, err := net.Listen("tcp", addr)
			if err != nil {
				l.Bg().Error("gRPC server failed to listen", zap.Error(err), zap.String("addr", addr))
				return err
			}

			l.Bg().Info("Starting gRPC server", zap.String("addr", addr))
			go func() {
				if err := s.Serve(listener); err != nil && err != grpc.ErrServerStopped {
					l.Bg().Error("gRPC server stopped unexpectedly", zap.Error(err))
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			l.Bg().Info("Stopping gRPC server gracefully")
			s.GracefulStop()
			return nil
		},
	})

	return &Server{
		GRPCServer: s,
		logger:     l,
		port:       port,
	}, nil
}

func ProvideRawServer(server *Server) *grpc.Server {
	return server.GRPCServer
}
