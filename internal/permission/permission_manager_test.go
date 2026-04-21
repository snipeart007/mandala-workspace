// Package permission contains tests for the permission manager and bitmask-based access control.
// It verifies that permissions are correctly checked and inherited through the folder hierarchy.
package permission

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"mandala-workspace/internal/db"
	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) (*db.DBManager, uint64, string) {
	tmpDir := t.TempDir()
	sqlPath := filepath.Join(tmpDir, "init.sql")
	// Navigate up to find the schema
	schema, err := os.ReadFile("../db/sql/InitializeDB.sql")
	if err != nil {
		t.Fatalf("failed to read schema: %v", err)
	}
	os.WriteFile(sqlPath, schema, 0644)

	sqlDB, _ := sql.Open("sqlite3", ":memory:")
	mgr := db.NewDBManagerWithDB(sqlDB, &db.DBManagerConfig{InitialSchemePath: sqlPath})
	mgr.Setup()

	userID, _, err := mgr.CreateUser("test", "test@example.com", []byte("hash"), nil)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	return mgr, userID, tmpDir
}

func TestPermissionManager_HasPermission(t *testing.T) {
	mgr, userID, _ := setupTestDB(t)
	defer mgr.Close()
	pm := NewPermissionManager(mgr)

	folderID := uint64(1) // Root is created by Setup()

	// 1. Initially no permissions
	has, err := pm.HasPermission(userID, folderID, PermRead)
	if err != nil {
		t.Fatalf("HasPermission failed: %v", err)
	}
	if has {
		t.Errorf("expected no permission, got true")
	}

	// 2. Grant permission
	mgr.SetUserPermission(userID, folderID, uint64(PermRead))
	has, _ = pm.HasPermission(userID, folderID, PermRead)
	if !has {
		t.Errorf("expected PermRead, got false")
	}

	// 3. Grant Admin
	mgr.SetUserPermission(userID, folderID, uint64(PermAdmin))
	has, _ = pm.HasPermission(userID, folderID, PermWrite) // Should have Write because of Admin
	if !has {
		t.Errorf("expected PermWrite via Admin, got false")
	}
}

func TestPermissionManager_CheckPermission(t *testing.T) {
	pm := NewPermissionManager(nil)

	p := &Permission{
		bitmask: PermRead | PermWrite,
	}

	if !pm.CheckPermission(p, PermRead) {
		t.Errorf("CheckPermission(PermRead) failed")
	}
	if !pm.CheckPermission(p, PermWrite) {
		t.Errorf("CheckPermission(PermWrite) failed")
	}
	if pm.CheckPermission(p, PermCreate) {
		t.Errorf("CheckPermission(PermCreate) should have failed")
	}

	// Admin override
	adminP := &Permission{
		bitmask: PermAdmin,
	}
	if !pm.CheckPermission(adminP, PermDelete) {
		t.Errorf("Admin should have PermDelete")
	}
}
