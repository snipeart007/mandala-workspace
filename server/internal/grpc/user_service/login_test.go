// Package user_service contains tests for the user login and authentication flow.
// It verifies the challenge-response mechanism and the issuance of authentication tokens.
package user_service

import (
	"context"
	"testing"

	"mandala-workspace/gen"
	"mandala-workspace/internal/crypto/ed25519"

	"google.golang.org/protobuf/proto"
)

func TestLoginUserReturnsChallenge(t *testing.T) {
	userID := uint64(1)
	deviceID := uint64(2)
	publicKey, _, err := ed25519.GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}

	server, cleanup := newGRPCServerWithDevice(t, userID, deviceID, publicKey)
	defer cleanup()

	challenge, err := server.LoginUser(context.Background(), &gen.LoginUserRequest{UserId: userID, DeviceId: deviceID})
	if err != nil {
		t.Fatalf("LoginUser returned error: %v", err)
	}

	if challenge.UserId != userID {
		t.Fatalf("expected UserId %d, got %d", userID, challenge.UserId)
	}
	if challenge.DeviceId != deviceID {
		t.Fatalf("expected DeviceId %d, got %d", deviceID, challenge.DeviceId)
	}
	if challenge.Timestamp == 0 {
		t.Fatal("expected non-zero timestamp in challenge")
	}
}

func TestVerifyLoginSignatureIssuesToken(t *testing.T) {
	userID := uint64(10)
	deviceID := uint64(20)
	publicKey, privateKey, err := ed25519.GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}

	server, cleanup := newGRPCServerWithDevice(t, userID, deviceID, publicKey)
	defer cleanup()

	challenge, err := server.LoginUser(context.Background(), &gen.LoginUserRequest{
		UserId:   userID,
		DeviceId: deviceID,
	})
	if err != nil {
		t.Fatalf("LoginUser returned error: %v", err)
	}

	challengeBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(challenge)
	if err != nil {
		t.Fatalf("failed to marshal challenge: %v", err)
	}

	signature, err := ed25519.SignMessage(privateKey, challengeBytes)
	if err != nil {
		t.Fatalf("failed to sign challenge: %v", err)
	}

	resp, err := server.VerifyLoginSignature(context.Background(), &gen.LoginUserSignatureRequest{
		UserId:    userID,
		DeviceId:  deviceID,
		Timestamp: challenge.Timestamp,
		Signature: signature,
	})
	if err != nil {
		t.Fatalf("VerifyLoginSignature returned error: %v", err)
	}

	if resp == nil || resp.Token == "" {
		t.Fatal("expected non-empty token")
	}
}
