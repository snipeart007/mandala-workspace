// Package user_service provides helper functions for unit testing the user service implementation.
// These helpers facilitate the setup of test gRPC servers with pre-configured database states.
package user_service

import (
	"bytes"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"mandala-workspace/internal/crypto/paseto"
	"mandala-workspace/internal/db"
	"mandala-workspace/internal/grpc/session"
	"mandala-workspace/internal/permission"

	_ "github.com/mattn/go-sqlite3"
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
