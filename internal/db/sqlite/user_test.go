package sqlite

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"mandala-workspace/internal/db"
)

func TestUserDeviceLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	sqlPath := filepath.Join(tmpDir, "init.sql")
	schema, err := os.ReadFile("../sql/InitializeDB.sql")
	if err != nil {
		t.Fatalf("failed to read schema: %v", err)
	}
	os.WriteFile(sqlPath, schema, 0644)

	sqlDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	mgr := &SQLiteManager{db: sqlDB, config: &db.DBManagerConfig{InitialSchemePath: sqlPath}}
	err = mgr.Setup()
	if err != nil {
		t.Fatalf("failed to setup db: %v", err)
	}
	defer mgr.Close()

	// 1. Create User
	userID, _, err := mgr.CreateUser("Alice", "alice@example.com", []byte("hash"), nil)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	count, _ := mgr.GetUserCount()
	if count != 1 {
		t.Errorf("expected 1 user, got %d", count)
	}

	// 2. Register Device
	pubKey := []byte("public-key")
	deviceID, _, err := mgr.RegisterDevice(userID, pubKey, nil)
	if err != nil {
		t.Fatalf("failed to register device: %v", err)
	}

	// 3. Get Public Key
	key, err := mgr.GetDevicePublicKey(userID, deviceID)
	if err != nil {
		t.Fatalf("failed to get public key: %v", err)
	}
	if string(key) != string(pubKey) {
		t.Errorf("key mismatch")
	}

	// 4. Revoke Device
	err = mgr.RevokeDevice(userID, deviceID)
	if err != nil {
		t.Fatalf("failed to revoke device: %v", err)
	}

	// 5. Verify Revoked (should fail to get key)
	_, err = mgr.GetDevicePublicKey(userID, deviceID)
	if err == nil {
		t.Errorf("expected error for revoked device")
	}
}
