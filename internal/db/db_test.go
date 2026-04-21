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
	content := "CREATE TABLE users (id INTEGER PRIMARY KEY); CREATE TABLE folders (folder_id INTEGER PRIMARY KEY, name TEXT, path TEXT, created_at INTEGER); INSERT INTO users (id) VALUES (1);"

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
		public_key BLOB NOT NULL,
		revoked_at INTEGER
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
		public_key BLOB NOT NULL,
		revoked_at INTEGER
	)`)
	if err != nil {
		t.Fatalf("failed to create devices table: %v", err)
	}

	_, err = mgr.GetDevicePublicKey(1, 2)
	if err == nil {
		t.Fatal("expected error when device not found, but got nil")
	}
}
func TestGetUserEffectivePermissions(t *testing.T) {
	mgr := NewTestManager(t, "")
	defer mgr.Close()

	// 1. Setup tables
	_, err := mgr.db.Exec(`
		CREATE TABLE users (user_id INTEGER PRIMARY KEY, name TEXT, email TEXT, password_hash BLOB, created_at INTEGER);
		CREATE TABLE folders (folder_id INTEGER PRIMARY KEY, name TEXT, parent_folder_id INTEGER, path TEXT, inheritance BOOLEAN, created_at INTEGER, deleted_at INTEGER);
		CREATE TABLE permissions (user_id INTEGER NOT NULL, folder_id INTEGER NOT NULL, permissions INTEGER NOT NULL, UNIQUE(user_id, folder_id));
	`)
	if err != nil {
		t.Fatalf("failed to setup tables: %v", err)
	}

	// 2. Setup hierarchy
	// 1 (root, path="")
	//   2 (dept, path="1/", inh=true)
	//     3 (project, path="1/2/", inh=false) -> BREAK HERE
	//       4 (secret, path="1/2/3/", inh=true)
	mgr.db.Exec("INSERT INTO folders (folder_id, name, path, inheritance, created_at) VALUES (1, 'root', '', 1, 0)")
	mgr.db.Exec("INSERT INTO folders (folder_id, name, path, inheritance, created_at) VALUES (2, 'dept', '1/', 1, 0)")
	mgr.db.Exec("INSERT INTO folders (folder_id, name, path, inheritance, created_at) VALUES (3, 'project', '1/2/', 0, 0)")
	mgr.db.Exec("INSERT INTO folders (folder_id, name, path, inheritance, created_at) VALUES (4, 'secret', '1/2/3/', 1, 0)")

	userID := uint64(100)
	const PermRead uint64 = 1
	const PermWrite uint64 = 2
	const PermAdmin uint64 = 1 << 31

	// Test Case 1: Simple inheritance (Root -> Dept)
	mgr.db.Exec("INSERT INTO permissions (user_id, folder_id, permissions) VALUES (?, ?, ?)", userID, 1, PermRead)
	p, _ := mgr.GetUserEffectivePermissions(userID, 2)
	if p != PermRead {
		t.Errorf("Expected PermRead (1), got %d", p)
	}

	// Test Case 2: Inheritance break (Dept -> Project)
	// User has Read on Root, but Project has inheritance=0.
	p, _ = mgr.GetUserEffectivePermissions(userID, 3)
	if p != 0 {
		t.Errorf("Expected 0 due to inheritance break, got %d", p)
	}

	// Test Case 3: Explicit permission on broken folder
	mgr.db.Exec("INSERT INTO permissions (user_id, folder_id, permissions) VALUES (?, ?, ?)", userID, 3, PermWrite)
	p, _ = mgr.GetUserEffectivePermissions(userID, 4) // Secret inherits from Project
	if p != PermWrite {
		t.Errorf("Expected PermWrite (2) from explicit folder, got %d", p)
	}

	// Test Case 4: Admin bypasses break
	adminID := uint64(999)
	mgr.SetUserPermission(adminID, 1, PermAdmin)
	p, _ = mgr.GetUserEffectivePermissions(adminID, 4) // Project (id=3) breaks inheritance
	if (p & PermAdmin) == 0 {
		t.Errorf("Expected Admin to bypass break, but bit not found. Got %d", p)
	}
}
