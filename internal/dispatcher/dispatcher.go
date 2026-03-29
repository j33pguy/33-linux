// Package dispatcher orchestrates the gRPC server and module registration.
package dispatcher

import (
	"fmt"
	"log"
	"net"
	"os"

	"google.golang.org/grpc"
)

// Server wraps the gRPC server and manages the Unix socket listener.
type Server struct {
	grpcServer *grpc.Server
	socketPath string
	listener   net.Listener
}

// New creates a new dispatcher Server bound to the given Unix socket path.
func New(socketPath string) *Server {
	return &Server{
		grpcServer: grpc.NewServer(),
		socketPath: socketPath,
	}
}

// GRPCServer returns the underlying gRPC server for service registration.
func (s *Server) GRPCServer() *grpc.Server {
	return s.grpcServer
}

// Start removes any stale socket and begins serving.
func (s *Server) Start() error {
	// Clean up stale socket
	if err := os.Remove(s.socketPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("dispatcher: remove stale socket: %w", err)
	}

	lis, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("dispatcher: listen on %s: %w", s.socketPath, err)
	}

	// Restrict socket permissions
	if err := os.Chmod(s.socketPath, 0600); err != nil {
		lis.Close()
		return fmt.Errorf("dispatcher: chmod socket: %w", err)
	}

	s.listener = lis
	log.Printf("dispatcher: serving on %s", s.socketPath)
	return s.grpcServer.Serve(lis)
}

// Stop gracefully shuts down the gRPC server.
func (s *Server) Stop() {
	s.grpcServer.GracefulStop()
	if s.listener != nil {
		s.listener.Close()
	}
	os.Remove(s.socketPath)
	log.Printf("dispatcher: stopped")
}
