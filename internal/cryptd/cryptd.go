// Package cryptd implements the encryption and decryption module.
// Uses AES-256-GCM for all symmetric encryption operations.
package cryptd

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"

	pb "github.com/j33pguy/33-linux/proto/crypto/v1"
)

// Service implements the CryptoService gRPC server.
type Service struct {
	pb.UnimplementedCryptoServiceServer
}

// NewService creates a new crypto service.
func NewService() *Service {
	return &Service{}
}

// Encrypt encrypts plaintext using AES-256-GCM with the provided key.
// The nonce is generated from crypto/rand and returned separately.
func (s *Service) Encrypt(ctx context.Context, req *pb.EncryptRequest) (*pb.EncryptResponse, error) {
	if len(req.Key) != 32 {
		return nil, fmt.Errorf("cryptd: key must be 32 bytes (AES-256), got %d", len(req.Key))
	}
	if len(req.Plaintext) == 0 {
		return nil, fmt.Errorf("cryptd: plaintext cannot be empty")
	}

	block, err := aes.NewCipher(req.Key)
	if err != nil {
		return nil, fmt.Errorf("cryptd: new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cryptd: new GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("cryptd: generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, req.Plaintext, nil)

	return &pb.EncryptResponse{
		Ciphertext: ciphertext,
		Nonce:      nonce,
	}, nil
}

// Decrypt decrypts ciphertext using AES-256-GCM with the provided key and nonce.
func (s *Service) Decrypt(ctx context.Context, req *pb.DecryptRequest) (*pb.DecryptResponse, error) {
	if len(req.Key) != 32 {
		return nil, fmt.Errorf("cryptd: key must be 32 bytes (AES-256), got %d", len(req.Key))
	}
	if len(req.Ciphertext) == 0 {
		return nil, fmt.Errorf("cryptd: ciphertext cannot be empty")
	}

	block, err := aes.NewCipher(req.Key)
	if err != nil {
		return nil, fmt.Errorf("cryptd: new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cryptd: new GCM: %w", err)
	}

	if len(req.Nonce) != gcm.NonceSize() {
		return nil, fmt.Errorf("cryptd: nonce must be %d bytes, got %d", gcm.NonceSize(), len(req.Nonce))
	}

	plaintext, err := gcm.Open(nil, req.Nonce, req.Ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("cryptd: decrypt failed (tampered or wrong key): %w", err)
	}

	return &pb.DecryptResponse{
		Plaintext: plaintext,
	}, nil
}

// EncryptBytes is a helper for direct Go usage (not via gRPC).
// Returns nonce prepended to ciphertext.
func EncryptBytes(data, key []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("cryptd: key must be 32 bytes")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("cryptd: new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cryptd: new GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("cryptd: generate nonce: %w", err)
	}

	// Prepend nonce to ciphertext for self-contained storage
	return gcm.Seal(nonce, nonce, data, nil), nil
}

// DecryptBytes is a helper for direct Go usage (not via gRPC).
// Expects nonce prepended to ciphertext.
func DecryptBytes(data, key []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("cryptd: key must be 32 bytes")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("cryptd: new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cryptd: new GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("cryptd: data too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}
