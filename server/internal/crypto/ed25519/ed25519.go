// Package ed25519 provides cryptographic primitives for digital signatures using the Ed25519 algorithm.
// It includes functions for key pair generation, message signing, and signature verification.
package ed25519

import (
	"crypto/ed25519"
	"fmt"
	"log/slog"
)

// VerifySignature verifies an ed25519 signature against a public key and message
// publicKey: the ed25519 public key (32 bytes)
// message: the original message that was signed
// signature: the ed25519 signature (64 bytes)
// Returns: nil if signature is valid, error otherwise
func VerifySignature(publicKey []byte, message []byte, signature []byte) error {
	// Validate public key length (ed25519 public keys are 32 bytes)
	if len(publicKey) != ed25519.PublicKeySize {
		slog.Warn("Ed25519 signature verification failed: invalid public key length", "expected", ed25519.PublicKeySize, "got", len(publicKey))
		return fmt.Errorf("ed25519: invalid public key length, expected %d bytes, got %d", ed25519.PublicKeySize, len(publicKey))
	}

	// Validate signature length (ed25519 signatures are 64 bytes)
	if len(signature) != ed25519.SignatureSize {
		slog.Warn("Ed25519 signature verification failed: invalid signature length", "expected", ed25519.SignatureSize, "got", len(signature))
		return fmt.Errorf("ed25519: invalid signature length, expected %d bytes, got %d", ed25519.SignatureSize, len(signature))
	}

	// Convert the public key bytes to an ed25519.PublicKey
	pubKey := ed25519.PublicKey(publicKey)

	// Verify the signature using ed25519.Verify
	if !ed25519.Verify(pubKey, message, signature) {
		slog.Debug("Ed25519 signature verification failed for provided message and key")
		return fmt.Errorf("ed25519: signature verification failed")
	}

	slog.Debug("Ed25519 signature verified successfully")
	return nil
}

// GenerateKeyPair generates a new ed25519 key pair
// Returns: (publicKey, privateKey, error)
// publicKey: 32 bytes
// privateKey: 64 bytes (includes the seed)
func GenerateKeyPair() ([]byte, []byte, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		slog.Error("Failed to generate ed25519 key pair", "error", err)
		return nil, nil, fmt.Errorf("ed25519: failed to generate ed25519 key pair: %w", err)
	}

	slog.Debug("New ed25519 key pair generated successfully")
	return publicKey, privateKey, nil
}

// SignMessage signs a message using an ed25519 private key
// privateKey: the ed25519 private key (64 bytes)
// message: the message to sign
// Returns: the signature (64 bytes)
func SignMessage(privateKey []byte, message []byte) ([]byte, error) {
	// Validate private key length (ed25519 private keys are 64 bytes)
	if len(privateKey) != ed25519.PrivateKeySize {
		slog.Error("Ed25519 signing failed: invalid private key length", "expected", ed25519.PrivateKeySize, "got", len(privateKey))
		return nil, fmt.Errorf("ed25519: invalid private key length, expected %d bytes, got %d", ed25519.PrivateKeySize, len(privateKey))
	}

	// Create an ed25519.PrivateKey from bytes
	privKey := ed25519.PrivateKey(privateKey)

	// Sign the message
	signature := ed25519.Sign(privKey, message)

	slog.Debug("Message signed successfully using ed25519")
	return signature, nil
}
