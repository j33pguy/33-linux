// Package main implements the 33-Linux CLI client.
// Connects to the initd gRPC server and provides user-facing commands.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	authpb "github.com/j33pguy/33-linux/proto/auth/v1"
	cryptopb "github.com/j33pguy/33-linux/proto/crypto/v1"
	filepb "github.com/j33pguy/33-linux/proto/file/v1"
	hwpb "github.com/j33pguy/33-linux/proto/hw/v1"
	netpb "github.com/j33pguy/33-linux/proto/net/v1"
	procpb "github.com/j33pguy/33-linux/proto/proc/v1"
)

const (
	// DEFAULT_SOCKET is the default gRPC socket path.
	DEFAULT_SOCKET = "/run/33linux/rpc.sock"
	// DEV_SOCKET is the dev mode socket path.
	DEV_SOCKET = "/tmp/33linux-rpc.sock"
	// RPC_TIMEOUT is the default timeout for gRPC calls.
	RPC_TIMEOUT = 10 * time.Second
)

func main() {
	log.SetFlags(0) // Clean output for CLI

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Determine socket
	socketPath := DEFAULT_SOCKET
	if os.Getenv("DEV_MODE") == "1" {
		socketPath = DEV_SOCKET
	}
	if s := os.Getenv("SOCKET_PATH"); s != "" {
		socketPath = s
	}

	// Connect to initd
	conn, err := grpc.NewClient(
		"unix://"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot connect to initd at %s: %v\n", socketPath, err)
		os.Exit(1)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), RPC_TIMEOUT)
	defer cancel()

	// Route command
	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "auth":
		handleAuth(ctx, conn, args)
	case "file":
		handleFile(ctx, conn, args)
	case "crypto":
		handleCrypto(ctx, conn, args)
	case "proc":
		handleProc(ctx, conn, args)
	case "hw":
		handleHW(ctx, conn, args)
	case "sync":
		handleSync(ctx, conn, args)
	case "version":
		fmt.Println("33-linux v0.1.0")
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func handleAuth(ctx context.Context, conn *grpc.ClientConn, args []string) {
	client := authpb.NewAuthServiceClient(conn)

	if len(args) == 0 {
		fmt.Println("usage: 33 auth login -u <user> -p <pass>")
		return
	}

	switch args[0] {
	case "login":
		fs := flag.NewFlagSet("auth login", flag.ExitOnError)
		user := fs.String("u", "", "username")
		pass := fs.String("p", "", "password")
		fs.Parse(args[1:])

		if *user == "" || *pass == "" {
			fmt.Println("usage: 33 auth login -u <user> -p <pass>")
			return
		}

		resp, err := client.Login(ctx, &authpb.LoginRequest{
			Username: *user,
			Password: *pass,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "auth error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("session token: %s\n", resp.SessionToken)
		fmt.Printf("expires: %s\n", time.Unix(resp.ExpiresAt, 0).Format(time.RFC3339))
	default:
		fmt.Fprintf(os.Stderr, "unknown auth command: %s\n", args[0])
	}
}

func handleFile(ctx context.Context, conn *grpc.ClientConn, args []string) {
	client := filepb.NewFileServiceClient(conn)

	if len(args) == 0 {
		fmt.Println("usage: 33 file [store|load] ...")
		return
	}

	switch args[0] {
	case "store":
		fs := flag.NewFlagSet("file store", flag.ExitOnError)
		path := fs.String("path", "", "file path")
		token := fs.String("token", "", "session token")
		fs.Parse(args[1:])

		if *path == "" {
			fmt.Println("usage: 33 file store -path <path> -token <token> < data")
			return
		}

		data, err := os.ReadFile(*path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read file: %v\n", err)
			os.Exit(1)
		}

		resp, err := client.StoreFile(ctx, &filepb.StoreFileRequest{
			Path:         *path,
			Data:         data,
			SessionToken: *token,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "store error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("stored: %s (queued for sync: %v)\n", resp.FileId, resp.QueuedForSync)

	case "load":
		fs := flag.NewFlagSet("file load", flag.ExitOnError)
		path := fs.String("path", "", "file path")
		token := fs.String("token", "", "session token")
		fs.Parse(args[1:])

		if *path == "" {
			fmt.Println("usage: 33 file load -path <path> -token <token>")
			return
		}

		resp, err := client.LoadFile(ctx, &filepb.LoadFileRequest{
			Path:         *path,
			SessionToken: *token,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "load error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("modified: %s\n", time.Unix(resp.ModifiedAt, 0).Format(time.RFC3339))
		os.Stdout.Write(resp.Data)

	default:
		fmt.Fprintf(os.Stderr, "unknown file command: %s\n", args[0])
	}
}

func handleCrypto(ctx context.Context, conn *grpc.ClientConn, args []string) {
	client := cryptopb.NewCryptoServiceClient(conn)

	if len(args) == 0 {
		fmt.Println("usage: 33 crypto [encrypt|decrypt] ...")
		return
	}

	switch args[0] {
	case "encrypt":
		fs := flag.NewFlagSet("crypto encrypt", flag.ExitOnError)
		keyHex := fs.String("key", "", "32-byte hex key")
		input := fs.String("data", "", "plaintext string")
		fs.Parse(args[1:])

		if *keyHex == "" || *input == "" {
			fmt.Println("usage: 33 crypto encrypt -key <hex> -data <text>")
			return
		}

		key := decodeHexOrDie(*keyHex)
		resp, err := client.Encrypt(ctx, &cryptopb.EncryptRequest{
			Plaintext: []byte(*input),
			Key:       key,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "encrypt error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("ciphertext: %x\nnonce: %x\n", resp.Ciphertext, resp.Nonce)

	case "decrypt":
		fs := flag.NewFlagSet("crypto decrypt", flag.ExitOnError)
		keyHex := fs.String("key", "", "32-byte hex key")
		ctHex := fs.String("ct", "", "ciphertext hex")
		nonceHex := fs.String("nonce", "", "nonce hex")
		fs.Parse(args[1:])

		if *keyHex == "" || *ctHex == "" || *nonceHex == "" {
			fmt.Println("usage: 33 crypto decrypt -key <hex> -ct <hex> -nonce <hex>")
			return
		}

		resp, err := client.Decrypt(ctx, &cryptopb.DecryptRequest{
			Ciphertext: decodeHexOrDie(*ctHex),
			Key:        decodeHexOrDie(*keyHex),
			Nonce:      decodeHexOrDie(*nonceHex),
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "decrypt error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("plaintext: %s\n", string(resp.Plaintext))

	default:
		fmt.Fprintf(os.Stderr, "unknown crypto command: %s\n", args[0])
	}
}

func handleProc(ctx context.Context, conn *grpc.ClientConn, args []string) {
	client := procpb.NewProcServiceClient(conn)

	if len(args) == 0 {
		fmt.Println("usage: 33 proc spawn <binary> [args...]")
		return
	}

	switch args[0] {
	case "spawn":
		if len(args) < 2 {
			fmt.Println("usage: 33 proc spawn <binary> [args...]")
			return
		}
		resp, err := client.SpawnProc(ctx, &procpb.SpawnProcRequest{
			Binary: args[1],
			Args:   args[2:],
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "spawn error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("PID: %d (status: %s)\n", resp.Pid, resp.Status)

	default:
		fmt.Fprintf(os.Stderr, "unknown proc command: %s\n", args[0])
	}
}

func handleHW(ctx context.Context, conn *grpc.ClientConn, args []string) {
	client := hwpb.NewHWSpawnerServiceClient(conn)

	if len(args) == 0 {
		fmt.Println("usage: 33 hw [detect|auth] ...")
		return
	}

	switch args[0] {
	case "detect":
		resp, err := client.DetectDevices(ctx, &hwpb.DetectDevicesRequest{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "detect error: %v\n", err)
			os.Exit(1)
		}
		for _, dev := range resp.Devices {
			fmt.Printf("  [%s] %s (%s)\n", dev.Type, dev.Name, dev.Path)
		}
		fmt.Printf("\n%d devices detected\n", len(resp.Devices))

	case "auth":
		if len(args) < 2 {
			fmt.Println("usage: 33 hw auth <device-id>")
			return
		}
		resp, err := client.AuthDevice(ctx, &hwpb.AuthDeviceRequest{
			DeviceId: args[1],
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "auth error: %v\n", err)
			os.Exit(1)
		}
		if resp.Authorized {
			fmt.Printf("✓ authorized (container: %s)\n", resp.ContainerId)
		} else {
			fmt.Println("✗ denied")
		}

	default:
		fmt.Fprintf(os.Stderr, "unknown hw command: %s\n", args[0])
	}
}

func handleSync(ctx context.Context, conn *grpc.ClientConn, args []string) {
	client := netpb.NewNetServiceClient(conn)

	resp, err := client.SyncQueue(ctx, &netpb.SyncQueueRequest{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "sync error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("synced: %d, remaining: %d\n", resp.ItemsSynced, resp.ItemsRemaining)
}

func printUsage() {
	usage := `33-linux CLI v0.1.0

Usage: 33 <command> [options]

Commands:
  auth     Authentication (login, session management)
  file     File operations (store, load)
  crypto   Encryption/decryption
  proc     Process spawning
  hw       Hardware detection and authorization
  sync     Trigger cloud sync (Phase 1: stub)
  version  Show version
  help     Show this help

Environment:
  DEV_MODE=1     Use dev socket path
  SOCKET_PATH=   Override socket path`
	fmt.Println(usage)
}

func decodeHexOrDie(s string) []byte {
	s = strings.TrimSpace(s)
	b := make([]byte, len(s)/2)
	for i := 0; i < len(b); i++ {
		var v byte
		for j := 0; j < 2; j++ {
			c := s[i*2+j]
			switch {
			case c >= '0' && c <= '9':
				v = v*16 + (c - '0')
			case c >= 'a' && c <= 'f':
				v = v*16 + (c - 'a' + 10)
			case c >= 'A' && c <= 'F':
				v = v*16 + (c - 'A' + 10)
			default:
				fmt.Fprintf(os.Stderr, "invalid hex character: %c\n", c)
				os.Exit(1)
			}
		}
		b[i] = v
	}
	return b
}
