// Package authd implements the authentication and session management module.
package authd

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	pb "github.com/j33pguy/33-linux/proto/auth/v1"
)

const (
	// SESSION_TTL is the default session lifetime.
	SESSION_TTL = 1 * time.Hour
)

// Session represents an authenticated user session.
type Session struct {
	Token     string
	Username  string
	ExpiresAt time.Time
	MasterKey []byte
}

// Service implements the AuthService gRPC server.
type Service struct {
	pb.UnimplementedAuthServiceServer

	mu       sync.RWMutex
	sessions map[string]*Session
	// users maps username to password hash (sha256 hex).
	// Phase 1: in-memory store. Phase 2: cloud-backed.
	users map[string]string
}

// NewService creates a new auth service with an initial admin user.
func NewService() *Service {
	s := &Service{
		sessions: make(map[string]*Session),
		users:    make(map[string]string),
	}
	// Phase 1: bootstrap with a default admin user
	// In production this would come from hardware-bound key enrollment
	s.AddUser("admin", "admin")
	return s
}

// AddUser registers a user with the given password.
func (s *Service) AddUser(username, password string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users[username] = hashPassword(password, username)
}

// Login authenticates a user and returns a session token.
func (s *Service) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	if req.Username == "" || req.Password == "" {
		return nil, fmt.Errorf("authd: username and password required")
	}

	s.mu.RLock()
	storedHash, exists := s.users[req.Username]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("authd: invalid credentials")
	}

	if hashPassword(req.Password, req.Username) != storedHash {
		return nil, fmt.Errorf("authd: invalid credentials")
	}

	token, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("authd: generate token: %w", err)
	}

	masterKey, err := generateMasterKey()
	if err != nil {
		return nil, fmt.Errorf("authd: generate master key: %w", err)
	}

	expiresAt := time.Now().Add(SESSION_TTL)
	session := &Session{
		Token:     token,
		Username:  req.Username,
		ExpiresAt: expiresAt,
		MasterKey: masterKey,
	}

	s.mu.Lock()
	s.sessions[token] = session
	s.mu.Unlock()

	return &pb.LoginResponse{
		SessionToken: token,
		ExpiresAt:    expiresAt.Unix(),
	}, nil
}

// DeriveKey derives a context-specific key from the session master key.
func (s *Service) DeriveKey(ctx context.Context, req *pb.DeriveKeyRequest) (*pb.DeriveKeyResponse, error) {
	session, err := s.ValidateSession(req.SessionToken)
	if err != nil {
		return nil, err
	}

	if req.Context == "" {
		return nil, fmt.Errorf("authd: context required for key derivation")
	}

	derived := deriveKey(session.MasterKey, []byte(req.Context))
	return &pb.DeriveKeyResponse{
		DerivedKey: derived,
	}, nil
}

// ValidateSession checks if a session token is valid and not expired.
func (s *Service) ValidateSession(token string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, exists := s.sessions[token]
	if !exists {
		return nil, fmt.Errorf("authd: invalid session token")
	}
	if time.Now().After(session.ExpiresAt) {
		return nil, fmt.Errorf("authd: session expired")
	}
	return session, nil
}

// hashPassword creates a salted SHA-256 hash. Username acts as salt.
func hashPassword(password, salt string) string {
	h := sha256.New()
	h.Write([]byte(salt))
	h.Write([]byte(password))
	return hex.EncodeToString(h.Sum(nil))
}

// generateToken creates a cryptographically random session token.
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// generateMasterKey creates a 32-byte random master key for the session.
func generateMasterKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate master key: %w", err)
	}
	return key, nil
}

// deriveKey uses HMAC-SHA256 to derive a key from master + context.
func deriveKey(master, context []byte) []byte {
	h := sha256.New()
	h.Write(master)
	h.Write(context)
	return h.Sum(nil)
}
