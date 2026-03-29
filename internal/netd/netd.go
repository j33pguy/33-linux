// Package netd implements the network and cloud sync module.
// Phase 1: stub implementation that logs sync requests.
// Phase 2: full cloud integration with queue drain, offline handling, conflict resolution.
package netd

import (
	"context"
	"fmt"
	"log"

	pb "github.com/j33pguy/33-linux/proto/net/v1"
)

// Service implements the NetService gRPC server.
type Service struct {
	pb.UnimplementedNetServiceServer
}

// NewService creates a new network service.
func NewService() *Service {
	return &Service{}
}

// SyncQueue attempts to sync queued items to the cloud backend.
// Phase 1: stub that reports queue status without actual sync.
func (s *Service) SyncQueue(ctx context.Context, req *pb.SyncQueueRequest) (*pb.SyncQueueResponse, error) {
	log.Printf("netd: sync requested (Phase 1 stub — no cloud backend yet)")
	return &pb.SyncQueueResponse{
		ItemsSynced:    0,
		ItemsRemaining: 0,
	}, nil
}

// APIGet makes an HTTP GET to a cloud API endpoint.
// Phase 1: stub that returns an error indicating cloud is not configured.
func (s *Service) APIGet(ctx context.Context, req *pb.APIGetRequest) (*pb.APIGetResponse, error) {
	return nil, fmt.Errorf("netd: cloud backend not configured (Phase 1)")
}
