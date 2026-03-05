// Package servergrpc implements the daemon-side gRPC: server lifecycle,
// listener transport, and proto↔domain conversion for inbound requests.
package servergrpc

import (
	"fmt"
	"log/slog"
	"net"

	"google.golang.org/grpc"

	recurv1 "github.com/directedbits/recur/src/infra/grpc/v1"
)

// Server wraps a gRPC server listening on a Unix socket or TCP port.
type Server struct {
	grpcServer *grpc.Server
	listener   net.Listener
	socketPath string
}

// NewServer creates a gRPC server bound to the given address.
// On Linux this is a Unix socket path; on Windows it is a TCP address.
func NewServer(socketPath string, svc recurv1.RecurServiceServer) (*Server, error) {
	if err := cleanupListener(socketPath); err != nil {
		return nil, fmt.Errorf("could not clean up stale listener: %w", err)
	}

	lis, err := listen(socketPath)
	if err != nil {
		return nil, fmt.Errorf("could not listen on %s: %w", socketPath, err)
	}

	gs := grpc.NewServer()
	recurv1.RegisterRecurServiceServer(gs, svc)

	return &Server{
		grpcServer: gs,
		listener:   lis,
		socketPath: socketPath,
	}, nil
}

// Serve starts accepting connections. This blocks until Stop is called.
func (s *Server) Serve() error {
	slog.Info("gRPC server listening", "address", s.socketPath)
	return s.grpcServer.Serve(s.listener)
}

// Stop gracefully stops the gRPC server and cleans up the listener.
func (s *Server) Stop() {
	s.grpcServer.GracefulStop()
	cleanupOnStop(s.socketPath)
}
