// Package main implements the 33-Linux init system (PID 1).
// In production, it mounts the immutable root and bootstraps all modules.
// In dev mode (DEV_MODE=1), it skips real mounts and uses local directories.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/j33pguy/33-linux/internal/authd"
	"github.com/j33pguy/33-linux/internal/cryptd"
	"github.com/j33pguy/33-linux/internal/dispatcher"
	"github.com/j33pguy/33-linux/internal/filed"
	"github.com/j33pguy/33-linux/internal/hwspawn"
	"github.com/j33pguy/33-linux/internal/mount"
	"github.com/j33pguy/33-linux/internal/netd"
	"github.com/j33pguy/33-linux/internal/procsd"

	authpb "github.com/j33pguy/33-linux/proto/auth/v1"
	cryptopb "github.com/j33pguy/33-linux/proto/crypto/v1"
	filepb "github.com/j33pguy/33-linux/proto/file/v1"
	hwpb "github.com/j33pguy/33-linux/proto/hw/v1"
	netpb "github.com/j33pguy/33-linux/proto/net/v1"
	procpb "github.com/j33pguy/33-linux/proto/proc/v1"
)

const (
	// SOCKET_PATH is the default gRPC Unix socket.
	SOCKET_PATH = "/run/33linux/rpc.sock"
	// DEV_SOCKET_PATH is used in dev mode.
	DEV_SOCKET_PATH = "/tmp/33linux-rpc.sock"
	// DEV_CACHE_DIR is the dev mode cache directory.
	DEV_CACHE_DIR = "/tmp/33linux-cache"
	// DEV_QUEUE_DIR is the dev mode sync queue directory.
	DEV_QUEUE_DIR = "/tmp/33linux-queue"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	devMode := os.Getenv("DEV_MODE") == "1"
	isPID1 := os.Getpid() == 1

	log.Printf("33-linux initd starting (PID %d, dev=%v)", os.Getpid(), devMode)

	// Phase 1: Mount filesystem
	if isPID1 && !devMode {
		log.Printf("initd: running as PID 1 — mounting filesystems")
		if err := mount.MountProc(); err != nil {
			log.Fatalf("initd: mount /proc: %v", err)
		}
		if err := mount.MountSys(); err != nil {
			log.Fatalf("initd: mount /sys: %v", err)
		}
		if err := mount.SetupImmutableRoot(mount.DefaultConfig()); err != nil {
			log.Fatalf("initd: setup immutable root: %v", err)
		}
	} else if devMode {
		if err := mount.SetupDevMode(DEV_CACHE_DIR, DEV_QUEUE_DIR); err != nil {
			log.Fatalf("initd: setup dev mode: %v", err)
		}
	}

	// Determine paths
	socketPath := SOCKET_PATH
	cacheDir := "/var/cache/33linux"
	queueDir := "/var/sync-queue"
	if devMode {
		socketPath = DEV_SOCKET_PATH
		cacheDir = DEV_CACHE_DIR
		queueDir = DEV_QUEUE_DIR
	}

	// Ensure socket directory exists
	socketDir := socketPath[:len(socketPath)-len("/rpc.sock")]
	if socketDir != "" {
		if err := os.MkdirAll(socketDir, 0700); err != nil {
			log.Fatalf("initd: create socket dir: %v", err)
		}
	}

	// Create dispatcher
	srv := dispatcher.New(socketPath)

	// Register modules in dependency order
	log.Printf("initd: registering modules")

	// 1. authd — must be first (other modules need auth)
	authSvc := authd.NewService()
	authpb.RegisterAuthServiceServer(srv.GRPCServer(), authSvc)
	log.Printf("initd: ✓ authd registered")

	// 2. cryptd — encryption primitives
	cryptoSvc := cryptd.NewService()
	cryptopb.RegisterCryptoServiceServer(srv.GRPCServer(), cryptoSvc)
	log.Printf("initd: ✓ cryptd registered")

	// 3. filed — file proxy and cache
	fileSvc := filed.NewService(cacheDir, queueDir)
	if err := fileSvc.EnsureDirs(); err != nil {
		log.Fatalf("initd: filed setup: %v", err)
	}
	filepb.RegisterFileServiceServer(srv.GRPCServer(), fileSvc)
	log.Printf("initd: ✓ filed registered")

	// 4. netd — network/sync (Phase 1 stub)
	netSvc := netd.NewService()
	netpb.RegisterNetServiceServer(srv.GRPCServer(), netSvc)
	log.Printf("initd: ✓ netd registered (stub)")

	// 5. procsd — process spawner
	procSvc := procsd.NewService()
	procpb.RegisterProcServiceServer(srv.GRPCServer(), procSvc)
	log.Printf("initd: ✓ procsd registered")

	// 6. hw-spawner — hardware detection
	hwSvc := hwspawn.NewService()
	hwpb.RegisterHWSpawnerServiceServer(srv.GRPCServer(), hwSvc)
	log.Printf("initd: ✓ hwspawn registered")

	// Start queue monitor
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fileSvc.StartQueueMonitor(ctx, 30*time.Second)

	// Handle shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		sig := <-sigCh
		log.Printf("initd: received %v, shutting down", sig)
		cancel()
		srv.Stop()
	}()

	// Start serving
	fmt.Fprintf(os.Stderr, "\n"+
		"  ╔══════════════════════════════════╗\n"+
		"  ║         33-Linux v0.1.0          ║\n"+
		"  ║     Secure. Immutable. Yours.    ║\n"+
		"  ╚══════════════════════════════════╝\n\n")
	log.Printf("initd: all modules registered, starting gRPC server on %s", socketPath)

	if err := srv.Start(); err != nil {
		log.Fatalf("initd: server failed: %v", err)
	}
}
