// Package hwspawn implements the hardware detection and container spawning module.
// Scans /sys/class for devices, authenticates them against policy, and spawns
// isolated LXC containers per device.
package hwspawn

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	pb "github.com/j33pguy/33-linux/proto/hw/v1"
)

const (
	// SYS_CLASS_PATH is the sysfs path for device enumeration.
	SYS_CLASS_PATH = "/sys/class"
)

// DeviceClass represents a category of hardware to scan.
type DeviceClass struct {
	Name string
	Path string
}

// DEFAULT_DEVICE_CLASSES are the device classes scanned by default.
var DEFAULT_DEVICE_CLASSES = []DeviceClass{
	{Name: "net", Path: "/sys/class/net"},
	{Name: "block", Path: "/sys/class/block"},
	{Name: "input", Path: "/sys/class/input"},
	{Name: "sound", Path: "/sys/class/sound"},
	{Name: "usb", Path: "/sys/bus/usb/devices"},
}

// Service implements the HWSpawnerService gRPC server.
type Service struct {
	pb.UnimplementedHWSpawnerServiceServer

	// allowlist of device IDs that are authorized.
	// Phase 1: static list. Phase 2: cloud policy.
	allowlist map[string]bool
}

// NewService creates a new hardware spawner service.
func NewService() *Service {
	return &Service{
		allowlist: make(map[string]bool),
	}
}

// AllowDevice adds a device ID to the allowlist.
func (s *Service) AllowDevice(deviceID string) {
	s.allowlist[deviceID] = true
}

// DetectDevices scans sysfs for connected hardware.
func (s *Service) DetectDevices(ctx context.Context, req *pb.DetectDevicesRequest) (*pb.DetectDevicesResponse, error) {
	var devices []*pb.Device

	for _, class := range DEFAULT_DEVICE_CLASSES {
		entries, err := os.ReadDir(class.Path)
		if err != nil {
			// Not all classes exist on all systems
			continue
		}

		for _, entry := range entries {
			dev := &pb.Device{
				Id:   fmt.Sprintf("%s/%s", class.Name, entry.Name()),
				Type: class.Name,
				Name: entry.Name(),
				Path: filepath.Join(class.Path, entry.Name()),
			}
			devices = append(devices, dev)
		}
	}

	log.Printf("hwspawn: detected %d devices", len(devices))
	return &pb.DetectDevicesResponse{
		Devices: devices,
	}, nil
}

// AuthDevice checks if a device is authorized and optionally spawns a container.
func (s *Service) AuthDevice(ctx context.Context, req *pb.AuthDeviceRequest) (*pb.AuthDeviceResponse, error) {
	if req.DeviceId == "" {
		return nil, fmt.Errorf("hwspawn: device_id required")
	}

	// Phase 1: check static allowlist
	// Phase 2: query cloud policy via authd
	authorized := s.isAuthorized(req.DeviceId)

	var containerID string
	if authorized {
		// In Phase 1, we just report auth status.
		// Phase 2: spawn LXC via procsd for isolated device access.
		containerID = fmt.Sprintf("hw-%s", sanitizeID(req.DeviceId))
		log.Printf("hwspawn: device %s authorized (container: %s)", req.DeviceId, containerID)
	} else {
		log.Printf("hwspawn: device %s DENIED", req.DeviceId)
	}

	return &pb.AuthDeviceResponse{
		Authorized:  authorized,
		ContainerId: containerID,
	}, nil
}

// isAuthorized checks the device against the allowlist.
// Phase 1: if allowlist is empty, allow all (dev mode).
// Phase 2: deny by default, require cloud policy.
func (s *Service) isAuthorized(deviceID string) bool {
	if len(s.allowlist) == 0 {
		// Dev mode: allow all when no policy configured
		return true
	}
	return s.allowlist[deviceID]
}

// sanitizeID creates a safe container name from a device ID.
func sanitizeID(id string) string {
	r := strings.NewReplacer("/", "-", " ", "-", ".", "-")
	return r.Replace(id)
}
