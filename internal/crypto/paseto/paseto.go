// Package paseto provides stateless session token management using the PASETO (Platform-Agnostic Security Tokens) V2 implementation.
// It handles token creation and verification with encrypted payloads.
package paseto

import (
	"fmt"
	"log/slog"

	"github.com/o1egl/paseto"
)

type Manager struct {
	paseto *paseto.V2
	key    []byte
}

type TokenClaims struct {
	UserID   uint64 `json:"user_id"`
	DeviceID uint64 `json:"device_id"`
}

func NewManager(key []byte) (*Manager, error) {
	if len(key) != 32 {
		slog.Error("Failed to create PASETO manager: invalid key length", "expected", 32, "got", len(key))
		return nil, fmt.Errorf("paseto: key must be 32 bytes, got %d", len(key))
	}

	copied := make([]byte, len(key))
	copy(copied, key)

	return &Manager{
		paseto: paseto.NewV2(),
		key:    copied,
	}, nil
}

func (m *Manager) CreateToken(userID uint64, deviceID uint64) (string, error) {
	if m == nil {
		slog.Error("PASETO token creation failed: manager is nil")
		return "", fmt.Errorf("paseto: manager is nil")
	}

	claims := TokenClaims{
		UserID:   userID,
		DeviceID: deviceID,
	}

	token, err := m.paseto.Encrypt(m.key, claims, nil)
	if err != nil {
		slog.Error("Failed to encrypt PASETO token", "user_id", userID, "device_id", deviceID, "error", err)
		return "", err
	}

	slog.Debug("PASETO token created successfully", "user_id", userID, "device_id", deviceID)
	return token, nil
}

func (m *Manager) VerifyToken(token string) (TokenClaims, error) {
	if m == nil {
		slog.Error("PASETO token verification failed: manager is nil")
		return TokenClaims{}, fmt.Errorf("paseto: manager is nil")
	}

	var claims TokenClaims
	if err := m.paseto.Decrypt(token, m.key, &claims, nil); err != nil {
		slog.Debug("PASETO token decryption failed", "error", err)
		return TokenClaims{}, err
	}

	slog.Debug("PASETO token verified successfully", "user_id", claims.UserID, "device_id", claims.DeviceID)
	return claims, nil
}
