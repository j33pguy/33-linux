// Package filed implements the file proxy module.
// Handles local cache storage and queues writes for cloud sync.
package filed

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/j33pguy/33-linux/internal/cryptd"
	pb "github.com/j33pguy/33-linux/proto/file/v1"
)

const (
	// DEFAULT_CACHE_DIR is the default local cache directory.
	DEFAULT_CACHE_DIR = "/var/cache/33linux"
	// DEFAULT_QUEUE_DIR is the default sync queue directory.
	DEFAULT_QUEUE_DIR = "/var/sync-queue"
)

// Service implements the FileService gRPC server.
type Service struct {
	pb.UnimplementedFileServiceServer

	mu       sync.RWMutex
	cacheDir string
	queueDir string
	// encryptionKey is set per-session via authd key derivation.
	encryptionKey []byte
}

// NewService creates a new file service with the given directories.
// If dirs are empty, uses defaults.
func NewService(cacheDir, queueDir string) *Service {
	if cacheDir == "" {
		cacheDir = DEFAULT_CACHE_DIR
	}
	if queueDir == "" {
		queueDir = DEFAULT_QUEUE_DIR
	}
	return &Service{
		cacheDir: cacheDir,
		queueDir: queueDir,
	}
}

// SetEncryptionKey sets the key used for encrypting queued files.
func (s *Service) SetEncryptionKey(key []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.encryptionKey = make([]byte, len(key))
	copy(s.encryptionKey, key)
}

// StoreFile stores a file in the local cache and queues it for sync.
func (s *Service) StoreFile(ctx context.Context, req *pb.StoreFileRequest) (*pb.StoreFileResponse, error) {
	if req.Path == "" {
		return nil, fmt.Errorf("filed: path required")
	}
	if len(req.Data) == 0 {
		return nil, fmt.Errorf("filed: data required")
	}

	// Generate file ID from path hash
	fileID := hashPath(req.Path)

	// Store in cache (plaintext for local reads)
	cachePath := filepath.Join(s.cacheDir, fileID)
	if err := os.MkdirAll(filepath.Dir(cachePath), 0700); err != nil {
		return nil, fmt.Errorf("filed: create cache dir: %w", err)
	}
	if err := os.WriteFile(cachePath, req.Data, 0600); err != nil {
		return nil, fmt.Errorf("filed: write cache: %w", err)
	}

	// Queue encrypted copy for sync
	queued := false
	s.mu.RLock()
	key := s.encryptionKey
	s.mu.RUnlock()

	if key != nil {
		encrypted, err := cryptd.EncryptBytes(req.Data, key)
		if err != nil {
			log.Printf("filed: encrypt for queue failed: %v", err)
		} else {
			queuePath := filepath.Join(s.queueDir, fileID+".enc")
			if err := os.MkdirAll(filepath.Dir(queuePath), 0700); err != nil {
				log.Printf("filed: create queue dir: %v", err)
			} else if err := os.WriteFile(queuePath, encrypted, 0600); err != nil {
				log.Printf("filed: write queue: %v", err)
			} else {
				queued = true
			}
		}
	}

	return &pb.StoreFileResponse{
		FileId:        fileID,
		QueuedForSync: queued,
	}, nil
}

// LoadFile loads a file from the local cache.
func (s *Service) LoadFile(ctx context.Context, req *pb.LoadFileRequest) (*pb.LoadFileResponse, error) {
	if req.Path == "" {
		return nil, fmt.Errorf("filed: path required")
	}

	fileID := hashPath(req.Path)
	cachePath := filepath.Join(s.cacheDir, fileID)

	data, err := os.ReadFile(cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("filed: file not found: %s", req.Path)
		}
		return nil, fmt.Errorf("filed: read cache: %w", err)
	}

	info, err := os.Stat(cachePath)
	if err != nil {
		return nil, fmt.Errorf("filed: stat cache: %w", err)
	}

	return &pb.LoadFileResponse{
		Data:       data,
		ModifiedAt: info.ModTime().Unix(),
	}, nil
}

// QueueSize returns the number of files waiting in the sync queue.
func (s *Service) QueueSize() (int, error) {
	entries, err := os.ReadDir(s.queueDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("filed: read queue dir: %w", err)
	}
	return len(entries), nil
}

// EnsureDirs creates the cache and queue directories if they don't exist.
func (s *Service) EnsureDirs() error {
	for _, dir := range []string{s.cacheDir, s.queueDir} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("filed: create dir %s: %w", dir, err)
		}
	}
	return nil
}

// hashPath creates a deterministic file ID from a path.
func hashPath(path string) string {
	h := sha256.Sum256([]byte(path))
	return hex.EncodeToString(h[:16]) // 128-bit, collision-resistant enough
}

// StartQueueMonitor logs queue status periodically (Phase 1 stub).
func (s *Service) StartQueueMonitor(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				size, err := s.QueueSize()
				if err != nil {
					log.Printf("filed: queue monitor error: %v", err)
				} else if size > 0 {
					log.Printf("filed: %d items in sync queue (waiting for netd)", size)
				}
			}
		}
	}()
}
