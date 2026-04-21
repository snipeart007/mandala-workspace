package user_service

import (
	"context"
	"testing"

	"mandala-workspace/gen"
	"mandala-workspace/internal/crypto/ed25519"
	"mandala-workspace/internal/crypto/paseto"
	"mandala-workspace/internal/grpc/interceptors"
	"mandala-workspace/internal/permission"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCreateUser_Success(t *testing.T) {
	adminID := uint64(1)
	deviceID := uint64(1)
	publicKey, _, _ := ed25519.GenerateKeyPair()
	server, cleanup := newGRPCServerWithDevice(t, adminID, deviceID, publicKey)
	defer cleanup()

	// Setup permissions for adminID on root folder (ID 1)
	_, err := server.db_manager.GetDevicePublicKey(adminID, deviceID)
	if err != nil {
		t.Fatalf("failed to get device: %v", err)
	}

	server.db_manager.SetUserPermission(adminID, 1, uint64(permission.PermUserCreate))

	// Mock context with claims
	ctx := context.WithValue(context.Background(), interceptors.TokenClaimsContextKey, paseto.TokenClaims{
		UserID:   adminID,
		DeviceID: deviceID,
	})

	req := &gen.CreateUserRequest{
		Name:     "Test User",
		Email:    "test@example.com",
		Password: "securepassword",
		Metadata: []byte(`{"role": "user"}`),
	}

	resp, err := server.CreateUser(ctx, req)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	if resp.UserId == 0 {
		t.Fatal("expected non-zero user id")
	}
	if resp.CreatedAt == 0 {
		t.Fatal("expected non-zero created_at")
	}
}

func TestCreateUser_PermissionDenied(t *testing.T) {
	userID := uint64(2)
	deviceID := uint64(2)
	publicKey, _, _ := ed25519.GenerateKeyPair()
	server, cleanup := newGRPCServerWithDevice(t, userID, deviceID, publicKey)
	defer cleanup()

	// No permissions set for userID

	ctx := context.WithValue(context.Background(), interceptors.TokenClaimsContextKey, paseto.TokenClaims{
		UserID:   userID,
		DeviceID: deviceID,
	})

	req := &gen.CreateUserRequest{
		Name:     "Another User",
		Email:    "another@example.com",
		Password: "password",
	}

	_, err := server.CreateUser(ctx, req)
	if err == nil {
		t.Fatal("expected error due to missing permission")
	}

	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied error, got: %v", err)
	}
}
