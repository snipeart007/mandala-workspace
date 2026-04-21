package argon2

import (
	"testing"
)

func TestHashAndVerifyPassword(t *testing.T) {
	password := "secure-password-123"

	// Hash the password
	encodedHash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	if encodedHash == "" {
		t.Fatal("encoded hash is empty")
	}

	// Verify the correct password
	match, err := VerifyPassword(password, encodedHash)
	if err != nil {
		t.Fatalf("failed to verify password: %v", err)
	}
	if !match {
		t.Error("expected password to match")
	}

	// Verify an incorrect password
	match, err = VerifyPassword("wrong-password", encodedHash)
	if err != nil {
		t.Fatalf("failed to verify password: %v", err)
	}
	if match {
		t.Error("expected password to not match")
	}
}

func TestVerifyInvalidFormat(t *testing.T) {
	invalidHashes := []string{
		"plain-text",
		"$argon2id$v=19$m=65536,t=1,p=4$short",
		"$argon2i$v=19$m=65536,t=1,p=4$salt$hash",
		"$argon2id$v=18$m=65536,t=1,p=4$salt$hash",
		"$argon2id$v=19$m=abc,t=1,p=4$salt$hash",
	}

	for _, h := range invalidHashes {
		match, err := VerifyPassword("password", h)
		if err == nil {
			t.Errorf("expected error for invalid hash format: %s", h)
		}
		if match {
			t.Errorf("expected no match for invalid hash format: %s", h)
		}
	}
}
