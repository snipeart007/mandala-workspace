package user_service

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"mandala-workspace/gen"
	"mandala-workspace/internal/crypto/ed25519"
	"mandala-workspace/internal/crypto/paseto"
	"mandala-workspace/internal/db"
	"mandala-workspace/internal/grpc/interceptors"
	"mandala-workspace/internal/grpc/session"
	"mandala-workspace/internal/permission"

	_ "github.com/mattn/go-sqlite3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func newGRPCServerWithDevice(t *testing.T, userID, deviceID uint64, publicKey []byte) (*UserService, func()) {
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	// Find the absolute path to InitializeDB.sql
	schemaPath := filepath.Join(oldWd, "../../../internal/db/sql/InitializeDB.sql")
	// If that doesn't work, try another way
	if _, err := os.Stat(schemaPath); err != nil {
		schemaPath = filepath.Join(oldWd, "internal/db/sql/InitializeDB.sql")
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir to temp dir: %v", err)
	}

	mgr, err := db.NewDBManager(&db.DBManagerConfig{
		InitialSchemePath: schemaPath,
	})
	if err != nil {
		os.Chdir(oldWd)
		t.Fatalf("failed to create DB manager: %v", err)
	}

	if err := mgr.Setup(); err != nil {
		mgr.Close()
		os.Chdir(oldWd)
		t.Fatalf("failed to setup DB: %v", err)
	}

	// Insert the device
	setupConn, err := sql.Open("sqlite3", filepath.Join(tmpDir, "db.sqlite"))
	if err != nil {
		mgr.Close()
		os.Chdir(oldWd)
		t.Fatalf("failed to open setup sqlite connection: %v", err)
	}
	defer setupConn.Close()

	// Insert a dummy user first to satisfy foreign key
	_, err = setupConn.Exec("INSERT INTO users (user_id, name, email, password_hash, created_at) VALUES (?, ?, ?, ?, ?)", userID, "Admin", "admin@example.com", []byte("hash"), 0)
	if err != nil {
		mgr.Close()
		os.Chdir(oldWd)
		t.Fatalf("failed to insert user row: %v", err)
	}

	_, err = setupConn.Exec("INSERT INTO devices (device_id, user_id, public_key, created_at) VALUES (?, ?, ?, ?)", deviceID, userID, publicKey, 0)
	if err != nil {
		mgr.Close()
		os.Chdir(oldWd)
		t.Fatalf("failed to insert device row: %v", err)
	}

	pasetoKey := bytes.Repeat([]byte{0x02}, 32)
	pasetoMgr, err := paseto.NewManager(pasetoKey)
	if err != nil {
		mgr.Close()
		os.Chdir(oldWd)
		t.Fatalf("failed to create paseto manager: %v", err)
	}

	permMgr := permission.NewPermissionManager(mgr)
	sessionMgr := session.NewSessionManager()

	return &UserService{
			db_manager:         mgr,
			paseto_manager:     pasetoMgr,
			permission_manager: permMgr,
			session_manager:    sessionMgr,
		}, func() {
			mgr.Close()
			os.Chdir(oldWd)
		}
}

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

func TestCreateUser_Success(t *testing.T) {
	adminID := uint64(1)
	deviceID := uint64(1)
	publicKey, _, _ := ed25519.GenerateKeyPair()
	server, cleanup := newGRPCServerWithDevice(t, adminID, deviceID, publicKey)
	defer cleanup()

	// Setup permissions for adminID on root folder (ID 1)
	// We need to create the folders table and root folder first because newGRPCServerWithDevice only creates devices table
	_, err := server.db_manager.GetDevicePublicKey(adminID, deviceID) // check if it works
	if err != nil {
		t.Fatalf("failed to get device: %v", err)
	}

	// Manually setup permissions for adminID on root folder (ID 1)
	// Actually, db.Setup() was already called in newGRPCServerWithDevice
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
