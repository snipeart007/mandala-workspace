package auth

import (
	"crypto/ed25519"
	"os"
	"testing"
)

func TestKeyring(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "keyring-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	k := NewKeyring(tmpDir)
	password := "strong-password"

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	_ = pub

	initialData := &KeyringData{
		DevicePrivateKey: priv,
		PasetoToken:      "some-token",
		UserID:           "user-1",
		DeviceID:         "device-1",
	}

	// Test Init
	err = k.Init(password, initialData)
	if err != nil {
		t.Fatalf("failed to init keyring: %v", err)
	}

	// Test Unlock
	k2 := NewKeyring(tmpDir)
	err = k2.Unlock(password)
	if err != nil {
		t.Fatalf("failed to unlock keyring: %v", err)
	}

	data := k2.GetData()
	if data.UserID != "user-1" {
		t.Errorf("expected user-1, got %s", data.UserID)
	}
	if data.PasetoToken != "some-token" {
		t.Errorf("expected some-token, got %s", data.PasetoToken)
	}

	// Test Unlock with wrong password
	k3 := NewKeyring(tmpDir)
	err = k3.Unlock("wrong-password")
	if err == nil {
		t.Error("expected error with wrong password, got nil")
	}

	// Test Save
	data.PasetoToken = "new-token"
	err = k2.Save()
	if err != nil {
		t.Fatalf("failed to save keyring: %v", err)
	}

	k4 := NewKeyring(tmpDir)
	err = k4.Unlock(password)
	if err != nil {
		t.Fatalf("failed to unlock after save: %v", err)
	}
	if k4.GetData().PasetoToken != "new-token" {
		t.Errorf("expected new-token, got %s", k4.GetData().PasetoToken)
	}
}
