package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// Helper to create a manager using an in-memory database
func NewTestManager(t *testing.T, schemaPath string) *DBManager {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open in-memory db: %v", err)
	}

	return &DBManager{
		db: db,
		config: &DBManagerConfig{
			InitialSchemePath: schemaPath,
		},
	}
}

func TestSetup_Success(t *testing.T) {
	// 1. Create a valid SQL file
	tmpDir := t.TempDir()
	sqlPath := filepath.Join(tmpDir, "valid.sql")
	content := "CREATE TABLE users (id INTEGER PRIMARY KEY); INSERT INTO users (id) VALUES (1);"

	if err := os.WriteFile(sqlPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write sql file: %v", err)
	}

	// 2. Initialize and Run Setup
	mgr := NewTestManager(t, sqlPath)
	defer mgr.Close()

	if err := mgr.Setup(); err != nil {
		t.Errorf("Setup failed with valid SQL: %v", err)
	}

	// 3. Verify side effects
	var id int
	err := mgr.db.QueryRow("SELECT id FROM users LIMIT 1").Scan(&id)
	if err != nil {
		t.Errorf("Could not query created table: %v", err)
	}
}

func TestSetup_InvalidSQL(t *testing.T) {
	// 1. Create a file with broken SQL syntax
	tmpDir := t.TempDir()
	sqlPath := filepath.Join(tmpDir, "broken.sql")
	content := "CREATE TABLE users (id INTEGER PRIMARY KEY" // Missing closing parenthesis

	if err := os.WriteFile(sqlPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write sql file: %v", err)
	}

	// 2. Initialize and Run Setup
	mgr := NewTestManager(t, sqlPath)
	defer mgr.Close()

	// 3. Assert that it returns an error
	err := mgr.Setup()
	if err == nil {
		t.Error("Expected error for malformed SQL, but got nil")
	}
}
func TestGetDevicePublicKey_Success(t *testing.T) {
	mgr := NewTestManager(t, "")
	defer mgr.Close()

	_, err := mgr.db.Exec(`CREATE TABLE devices (
		device_id INTEGER PRIMARY KEY,
		user_id INTEGER NOT NULL,
		public_key BLOB NOT NULL
	)`)
	if err != nil {
		t.Fatalf("failed to create devices table: %v", err)
	}

	expectedKey := []byte{0x01, 0x02, 0x03}
	_, err = mgr.db.Exec("INSERT INTO devices (device_id, user_id, public_key) VALUES (?, ?, ?)", 2, 1, expectedKey)
	if err != nil {
		t.Fatalf("failed to insert device row: %v", err)
	}

	publicKey, err := mgr.GetDevicePublicKey(1, 2)
	if err != nil {
		t.Fatalf("GetDevicePublicKey returned error: %v", err)
	}

	if len(publicKey) != len(expectedKey) {
		t.Fatalf("expected public key length %d, got %d", len(expectedKey), len(publicKey))
	}
	for i := range expectedKey {
		if publicKey[i] != expectedKey[i] {
			t.Fatalf("public key byte mismatch at index %d: got %d, want %d", i, publicKey[i], expectedKey[i])
		}
	}
}

func TestGetDevicePublicKey_NotFound(t *testing.T) {
	mgr := NewTestManager(t, "")
	defer mgr.Close()

	_, err := mgr.db.Exec(`CREATE TABLE devices (
		device_id INTEGER PRIMARY KEY,
		user_id INTEGER NOT NULL,
		public_key BLOB NOT NULL
	)`)
	if err != nil {
		t.Fatalf("failed to create devices table: %v", err)
	}

	_, err = mgr.GetDevicePublicKey(1, 2)
	if err == nil {
		t.Fatal("expected error when device not found, but got nil")
	}
}
func TestSetup_FileNotFound(t *testing.T) {
	// Pass a path that definitely doesn't exist
	mgr := NewTestManager(t, "non_existent_file.sql")
	defer mgr.Close()

	err := mgr.Setup()
	if err == nil {
		t.Error("Expected error for missing file, but got nil")
	}
}
