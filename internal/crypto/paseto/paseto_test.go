package paseto

import (
	"bytes"
	"testing"
)

func TestNewManagerKeyLength(t *testing.T) {
	_, err := NewManager([]byte("short key"))
	if err == nil {
		t.Fatal("expected error for invalid key length")
	}
}

func TestCreateAndVerifyToken(t *testing.T) {
	key := bytes.Repeat([]byte{0x01}, 32)
	manager, err := NewManager(key)
	if err != nil {
		t.Fatalf("unexpected error creating manager: %v", err)
	}

	userID := uint64(12345)
	deviceID := uint64(54321)

	token, err := manager.CreateToken(userID, deviceID)
	if err != nil {
		t.Fatalf("unexpected error creating token: %v", err)
	}

	claims, err := manager.VerifyToken(token)
	if err != nil {
		t.Fatalf("unexpected error verifying token: %v", err)
	}

	if claims.UserID != userID {
		t.Fatalf("expected user ID %d, got %d", userID, claims.UserID)
	}
	if claims.DeviceID != deviceID {
		t.Fatalf("expected device ID %d, got %d", deviceID, claims.DeviceID)
	}
}

func TestVerifyTokenRejectsInvalidToken(t *testing.T) {
	key := bytes.Repeat([]byte{0x01}, 32)
	manager, err := NewManager(key)
	if err != nil {
		t.Fatalf("unexpected error creating manager: %v", err)
	}

	_, err = manager.VerifyToken("not-a-valid-token")
	if err == nil {
		t.Fatal("expected error verifying invalid token")
	}
}
