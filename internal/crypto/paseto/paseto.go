package paseto

import (
	"fmt"

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
		return "", fmt.Errorf("paseto: manager is nil")
	}

	claims := TokenClaims{
		UserID:   userID,
		DeviceID: deviceID,
	}

	return m.paseto.Encrypt(m.key, claims, nil)
}

func (m *Manager) VerifyToken(token string) (TokenClaims, error) {
	if m == nil {
		return TokenClaims{}, fmt.Errorf("paseto: manager is nil")
	}

	var claims TokenClaims
	if err := m.paseto.Decrypt(token, m.key, &claims, nil); err != nil {
		return TokenClaims{}, err
	}

	return claims, nil
}
