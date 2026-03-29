// Package procsd implements the process spawner and LXC container manager.
package procsd

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"sync"
	"sync/atomic"

	pb "github.com/j33pguy/33-linux/proto/proc/v1"
)

// Service implements the ProcService gRPC server.
type Service struct {
	pb.UnimplementedProcServiceServer

	mu        sync.Mutex
	processes map[int32]*exec.Cmd
	nextID    atomic.Int32
}

// NewService creates a new process spawner service.
func NewService() *Service {
	return &Service{
		processes: make(map[int32]*exec.Cmd),
	}
}

// SpawnProc spawns a process with the given binary and args.
func (s *Service) SpawnProc(ctx context.Context, req *pb.SpawnProcRequest) (*pb.SpawnProcResponse, error) {
	if req.Binary == "" {
		return nil, fmt.Errorf("procsd: binary path required")
	}

	cmd := exec.CommandContext(ctx, req.Binary, req.Args...)

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("procsd: spawn %s: %w", req.Binary, err)
	}

	pid := int32(cmd.Process.Pid)

	s.mu.Lock()
	s.processes[pid] = cmd
	s.mu.Unlock()

	log.Printf("procsd: spawned %s (PID %d)", req.Binary, pid)

	// Clean up when process exits
	go func() {
		if err := cmd.Wait(); err != nil {
			log.Printf("procsd: process %d exited with error: %v", pid, err)
		} else {
			log.Printf("procsd: process %d exited cleanly", pid)
		}
		s.mu.Lock()
		delete(s.processes, pid)
		s.mu.Unlock()
	}()

	return &pb.SpawnProcResponse{
		Pid:    pid,
		Status: "running",
	}, nil
}

// SpawnLXC creates and starts an LXC container.
// Phase 1: uses lxc-create/lxc-start via os/exec.
func (s *Service) SpawnLXC(ctx context.Context, req *pb.SpawnLXCRequest) (*pb.SpawnLXCResponse, error) {
	if req.ContainerName == "" {
		return nil, fmt.Errorf("procsd: container name required")
	}

	// Check if lxc tools are available
	if _, err := exec.LookPath("lxc-create"); err != nil {
		return nil, fmt.Errorf("procsd: lxc-create not found (install lxc): %w", err)
	}

	image := req.Image
	if image == "" {
		image = "download"
	}

	// Create the container
	createCmd := exec.CommandContext(ctx, "lxc-create",
		"-n", req.ContainerName,
		"-t", image,
	)
	if output, err := createCmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("procsd: lxc-create failed: %s: %w", string(output), err)
	}

	// Start the container
	startCmd := exec.CommandContext(ctx, "lxc-start",
		"-n", req.ContainerName,
		"-d",
	)
	if output, err := startCmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("procsd: lxc-start failed: %s: %w", string(output), err)
	}

	containerID := fmt.Sprintf("lxc-%s-%d", req.ContainerName, s.nextID.Add(1))
	log.Printf("procsd: started LXC container %s (%s)", req.ContainerName, containerID)

	return &pb.SpawnLXCResponse{
		ContainerId: containerID,
		Status:      "running",
	}, nil
}
