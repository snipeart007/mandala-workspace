package user_service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"mandala-workspace/gen"
	"mandala-workspace/internal/crypto/ed25519"
	"mandala-workspace/internal/db"
	"mandala-workspace/internal/db/sqlite"
	"mandala-workspace/internal/crypto/paseto"
	"mandala-workspace/internal/grpc/session"
	"mandala-workspace/internal/permission"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func newGRPCServerEmpty(t *testing.T) (*UserService, func()) {
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	// Find the absolute path to InitializeDB.sql
	schemaPath := filepath.Join(oldWd, "../../db/sql/InitializeDB.sql")
	if _, err := os.Stat(schemaPath); err != nil {
		schemaPath = filepath.Join(oldWd, "internal/db/sql/InitializeDB.sql")
	}

	mgr, err := sqlite.NewSQLiteManager(&db.DBManagerConfig{
		DBPath:            filepath.Join(tmpDir, "db.sqlite"),
		InitialSchemePath: schemaPath,
	})
	if err != nil {
		t.Fatalf("failed to create DB manager: %v", err)
	}

	if err := mgr.Setup(); err != nil {
		mgr.Close()
		t.Fatalf("failed to setup DB: %v", err)
	}

	pasetoKey := make([]byte, 32)
	pasetoMgr, _ := paseto.NewManager(pasetoKey)
	permMgr := permission.NewPermissionManager(mgr)
	sessionMgr := session.NewSessionManager()

	return &UserService{
			db_manager:         mgr,
			paseto_manager:     pasetoMgr,
			permission_manager: permMgr,
			session_manager:    sessionMgr,
		}, func() {
			mgr.Close()
		}
}

func TestSetupAdmin_Success(t *testing.T) {
	server, cleanup := newGRPCServerEmpty(t)
	defer cleanup()

	pubKey, _, _ := ed25519.GenerateKeyPair()

	req := &gen.SetupAdminRequest{
		Name:           "Admin User",
		Email:          "admin@example.com",
		Password:       "adminpass",
		PublicKey:      pubKey,
		UserMetadata:   []byte(`{"init": true}`),
		DeviceMetadata: []byte(`{"device": "primary"}`),
	}

	resp, err := server.SetupAdmin(context.Background(), req)
	if err != nil {
		t.Fatalf("SetupAdmin failed: %v", err)
	}

	if resp.UserId == 0 || resp.DeviceId == 0 {
		t.Fatalf("expected non-zero IDs, got user=%d, device=%d", resp.UserId, resp.DeviceId)
	}

	// Verify permissions
	hasPerm, err := server.permission_manager.HasPermission(resp.UserId, 1, permission.PermAdmin)
	if err != nil {
		t.Fatalf("failed to check permissions: %v", err)
	}
	if !hasPerm {
		t.Fatal("expected user to have PermAdmin on root folder")
	}
}

func TestSetupAdmin_AlreadyInitialized(t *testing.T) {
	server, cleanup := newGRPCServerEmpty(t)
	defer cleanup()

	pubKey, _, _ := ed25519.GenerateKeyPair()

	// Setup first admin
	req := &gen.SetupAdminRequest{
		Name:      "First Admin",
		Email:     "first@example.com",
		Password:  "pass1",
		PublicKey: pubKey,
	}
	_, err := server.SetupAdmin(context.Background(), req)
	if err != nil {
		t.Fatalf("First SetupAdmin failed: %v", err)
	}

	// Try setting up another admin
	req2 := &gen.SetupAdminRequest{
		Name:      "Second Admin",
		Email:     "second@example.com",
		Password:  "pass2",
		PublicKey: pubKey,
	}
	_, err = server.SetupAdmin(context.Background(), req2)
	if err == nil {
		t.Fatal("expected second SetupAdmin to fail")
	}

	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied, got %v", err)
	}
}
