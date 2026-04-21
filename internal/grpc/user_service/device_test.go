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

func TestRegisterDevice_Success(t *testing.T) {
	adminID := uint64(1)
	adminDeviceID := uint64(1)
	targetUserID := uint64(2)
	publicKey, _, _ := ed25519.GenerateKeyPair()
	server, cleanup := newGRPCServerWithDevice(t, adminID, adminDeviceID, publicKey)
	defer cleanup()

	// Setup permissions for adminID on root folder (ID 1)
	server.db_manager.SetUserPermission(adminID, 1, uint64(permission.PermDeviceSetup))

	// Create target user
	_, _, err := server.db_manager.CreateUser("Target User", "target@example.com", []byte("hash"), nil)
	if err != nil {
		t.Fatalf("failed to create target user: %v", err)
	}

	// Mock context with claims
	ctx := context.WithValue(context.Background(), interceptors.TokenClaimsContextKey, paseto.TokenClaims{
		UserID:   adminID,
		DeviceID: adminDeviceID,
	})

	newDevicePublicKey, _, _ := ed25519.GenerateKeyPair()
	req := &gen.RegisterDeviceRequest{
		UserId:    targetUserID,
		PublicKey: newDevicePublicKey,
		Metadata:  []byte(`{"name": "test device"}`),
	}

	resp, err := server.RegisterDevice(ctx, req)
	if err != nil {
		t.Fatalf("RegisterDevice failed: %v", err)
	}

	if resp.DeviceId == 0 {
		t.Fatal("expected non-zero device id")
	}
	if resp.CreatedAt == 0 {
		t.Fatal("expected non-zero created_at")
	}
}

func TestRegisterDevice_PermissionDenied(t *testing.T) {
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

	newDevicePublicKey, _, _ := ed25519.GenerateKeyPair()
	req := &gen.RegisterDeviceRequest{
		UserId:    userID,
		PublicKey: newDevicePublicKey,
	}

	_, err := server.RegisterDevice(ctx, req)
	if err == nil {
		t.Fatal("expected error due to missing permission")
	}

	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied error, got: %v", err)
	}
}

func TestRevokeDevice_FullFlow(t *testing.T) {
	adminID := uint64(1)
	adminDeviceID := uint64(1)
	publicKey, _, _ := ed25519.GenerateKeyPair()
	server, cleanup := newGRPCServerWithDevice(t, adminID, adminDeviceID, publicKey)
	defer cleanup()

	// 1. Setup Admin permissions
	server.db_manager.SetUserPermission(adminID, 1, uint64(permission.PermAdmin))

	// 2. Mock context for admin
	ctx := context.WithValue(context.Background(), interceptors.TokenClaimsContextKey, paseto.TokenClaims{
		UserID:   adminID,
		DeviceID: adminDeviceID,
	})

	// 3. Mark device as logged in in session manager (simulating VerifyLoginSignature)
	server.session_manager.AddSession(adminID, adminDeviceID)

	// 4. Verify session is active
	if !server.session_manager.IsSessionActive(adminID, adminDeviceID) {
		t.Fatal("session should be active")
	}

	// 5. Revoke the device
	req := &gen.RevokeDeviceRequest{
		UserId:   adminID,
		DeviceId: adminDeviceID,
	}
	resp, err := server.RevokeDevice(ctx, req)
	if err != nil {
		t.Fatalf("RevokeDevice failed: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}

	// 6. Verify session is NO LONGER active
	if server.session_manager.IsSessionActive(adminID, adminDeviceID) {
		t.Fatal("session should be revoked")
	}

	// 7. Verify login attempt fails
	_, err = server.LoginUser(context.Background(), &gen.LoginUserRequest{
		UserId:   adminID,
		DeviceId: adminDeviceID,
	})
	if err == nil {
		t.Fatal("expected login to fail for revoked device")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.NotFound { // DBManager returns NotFound when revoked_at IS NOT NULL
		t.Fatalf("expected NotFound (revoked), got %v", st.Code())
	}
}
