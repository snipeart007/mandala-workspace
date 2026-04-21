// Package ed25519 contains tests for the Ed25519 cryptographic primitives used for device authentication.
// It verifies key pair generation, message signing, and signature verification.
package ed25519

import (
	"crypto/ed25519"
	"testing"
)

func TestGenerateKeyPair(t *testing.T) {
	publicKey, privateKey, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("unexpected error generating key pair: %v", err)
	}

	if len(publicKey) != ed25519.PublicKeySize {
		t.Fatalf("invalid public key length, expected %d, got %d", ed25519.PublicKeySize, len(publicKey))
	}

	if len(privateKey) != ed25519.PrivateKeySize {
		t.Fatalf("invalid private key length, expected %d, got %d", ed25519.PrivateKeySize, len(privateKey))
	}
}

func TestSignAndVerify(t *testing.T) {
	// Generate a key pair
	publicKey, privateKey, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("unexpected error generating key pair: %v", err)
	}

	// Message to sign
	message := []byte("test message for signing")

	// Sign the message
	signature, err := SignMessage(privateKey, message)
	if err != nil {
		t.Fatalf("unexpected error signing message: %v", err)
	}

	if len(signature) != ed25519.SignatureSize {
		t.Fatalf("invalid signature length, expected %d, got %d", ed25519.SignatureSize, len(signature))
	}

	// Verify the signature
	err = VerifySignature(publicKey, message, signature)
	if err != nil {
		t.Fatalf("unexpected error verifying signature: %v", err)
	}
}

func TestVerifySignatureInvalidPublicKey(t *testing.T) {
	invalidPublicKey := []byte("invalid")
	message := []byte("test message")
	signature := make([]byte, ed25519.SignatureSize)

	err := VerifySignature(invalidPublicKey, message, signature)
	if err == nil {
		t.Fatal("expected error for invalid public key length")
	}
}

func TestVerifySignatureInvalidSignature(t *testing.T) {
	// Generate a key pair
	publicKey, _, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("unexpected error generating key pair: %v", err)
	}

	message := []byte("test message")
	invalidSignature := make([]byte, ed25519.SignatureSize)

	err = VerifySignature(publicKey, message, invalidSignature)
	if err == nil {
		t.Fatal("expected error for invalid signature")
	}
}

func TestVerifySignatureInvalidSignatureLength(t *testing.T) {
	// Generate a key pair
	publicKey, _, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("unexpected error generating key pair: %v", err)
	}

	message := []byte("test message")
	invalidSignature := []byte("short")

	err = VerifySignature(publicKey, message, invalidSignature)
	if err == nil {
		t.Fatal("expected error for invalid signature length")
	}
}

func TestSignMessageInvalidPrivateKeyLength(t *testing.T) {
	invalidPrivateKey := []byte("invalid")
	message := []byte("test message")

	_, err := SignMessage(invalidPrivateKey, message)
	if err == nil {
		t.Fatal("expected error for invalid private key length")
	}
}

func TestVerifySignatureWithModifiedMessage(t *testing.T) {
	// Generate a key pair
	publicKey, privateKey, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("unexpected error generating key pair: %v", err)
	}

	// Original message
	originalMessage := []byte("original message")

	// Sign the original message
	signature, err := SignMessage(privateKey, originalMessage)
	if err != nil {
		t.Fatalf("unexpected error signing message: %v", err)
	}

	// Try to verify with a different message
	modifiedMessage := []byte("modified message")

	err = VerifySignature(publicKey, modifiedMessage, signature)
	if err == nil {
		t.Fatal("expected error when verifying with modified message")
	}
}
