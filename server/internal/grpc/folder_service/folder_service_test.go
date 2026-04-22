// Package folder_service contains tests for folder management operations like creation, listing, moving, and deletion.
// It ensures that folder operations respect permissions and correctly update the database hierarchy.
package folder_service

import (
	"context"
	"fmt"
	v1 "mandala-workspace/gen"
	"mandala-workspace/internal/crypto/paseto"
	"mandala-workspace/internal/db"
	"mandala-workspace/internal/db/sqlite"
	"mandala-workspace/internal/grpc/interceptors"
	"mandala-workspace/internal/permission"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func NewTestFolderService(t *testing.T) (*FolderService, db.DBProvider, uint64) {
	tmpDir := t.TempDir()
	sqlPath := filepath.Join(tmpDir, "init.sql")
	// Use the real schema
	content, err := os.ReadFile("../../db/sql/InitializeDB.sql")
	if err != nil {
		t.Fatalf("failed to read schema: %v", err)
	}
	if err := os.WriteFile(sqlPath, content, 0644); err != nil {
		t.Fatalf("failed to write schema: %v", err)
	}

	mgr, err := sqlite.NewSQLiteManager(&db.DBManagerConfig{
		InitialSchemePath: sqlPath,
		DBPath:            filepath.Join(tmpDir, "db.sqlite"),
	})
	if err != nil {
		t.Fatalf("failed to create DB manager: %v", err)
	}

	if err := mgr.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Create a test user
	userID, _, err := mgr.CreateUser("test", "test@example.com", []byte("hash"), nil)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	pm := permission.NewPermissionManager(mgr)
	return NewFolderService(mgr, pm), mgr, userID
}

func contextWithUser(userID uint64) context.Context {
	claims := paseto.TokenClaims{
		UserID:   userID,
		DeviceID: 1,
	}
	return context.WithValue(context.Background(), interceptors.TokenClaimsContextKey, claims)
}

func TestFolderService_CreateFolder(t *testing.T) {
	service, mgr, userID := NewTestFolderService(t)
	defer mgr.Close()

	ctx := contextWithUser(userID)

	// Root is created with ID 1 by Setup()
	// User must have PermCreateFolder on Root
	if err := mgr.SetUserPermission(userID, 1, uint64(permission.PermCreateFolder)); err != nil {
		t.Fatalf("SetUserPermission failed: %v", err)
	}

	req := &v1.CreateFolderRequest{
		Name:           "my_folder",
		ParentFolderId: 1,
		Inheritance:    true,
	}

	resp, err := service.CreateFolder(ctx, req)
	if err != nil {
		t.Fatalf("CreateFolder failed: %v", err)
	}

	if resp.Folder.Name != "my_folder" {
		t.Errorf("expected name my_folder, got %s", resp.Folder.Name)
	}
	if resp.Folder.Path != "1/" { // Root id is 1, its path is ""
		t.Errorf("expected path 1/, got %s", resp.Folder.Path)
	}
}

func TestFolderService_ListFolder(t *testing.T) {
	service, mgr, userID := NewTestFolderService(t)
	defer mgr.Close()

	ctx := contextWithUser(userID)

	// Create a folder and file
	fid, _, _ := mgr.CreateFolder("sub", 1, "1/", true, 0, nil)
	// We need a file table entry too
	mgr.CreateFile("myfile.txt", 1, "1/", "cas/123", "local", nil)

	// Grant Read permission
	mgr.SetUserPermission(userID, 1, uint64(permission.PermRead))

	resp, err := service.ListFolder(ctx, &v1.ListFolderRequest{FolderId: 1})
	if err != nil {
		t.Fatalf("ListFolder failed: %v", err)
	}

	if len(resp.Folders) != 1 || resp.Folders[0].FolderId != fid {
		t.Errorf("expected 1 folder, got %d", len(resp.Folders))
	}
	if len(resp.Files) != 1 || resp.Files[0].Name != "myfile.txt" {
		t.Errorf("expected 1 file, got %d", len(resp.Files))
	}
}

func TestFolderService_MoveFolder(t *testing.T) {
	service, mgr, userID := NewTestFolderService(t)
	defer mgr.Close()

	ctx := contextWithUser(userID)

	f1, _, _ := mgr.CreateFolder("f1", 1, "1/", true, 0, nil)
	f2, _, _ := mgr.CreateFolder("f2", 1, "1/", true, 0, nil)

	// Grant Move on f1 and Create on f2
	mgr.SetUserPermission(userID, f1, uint64(permission.PermMoveFolder))
	mgr.SetUserPermission(userID, f2, uint64(permission.PermCreateFolder))

	_, err := service.MoveFolder(ctx, &v1.MoveFolderRequest{
		FolderId:           f1,
		NewParentFolderId: f2,
	})
	if err != nil {
		t.Fatalf("MoveFolder failed: %v", err)
	}

	// Verify path updated in DB
	folder, _ := mgr.GetFolder(f1)
	expectedPath := fmt.Sprintf("1/%d/", f2)
	if folder.Path != expectedPath {
		t.Errorf("expected path %s, got %s", expectedPath, folder.Path)
	}
}

func TestFolderService_DeleteFolder(t *testing.T) {
	service, mgr, userID := NewTestFolderService(t)
	defer mgr.Close()

	ctx := contextWithUser(userID)

	f1, _, _ := mgr.CreateFolder("f1", 1, "1/", true, 0, nil)

	// Grant Delete on f1
	mgr.SetUserPermission(userID, f1, uint64(permission.PermDeleteFolder))

	_, err := service.DeleteFolder(ctx, &v1.DeleteFolderRequest{FolderId: f1})
	if err != nil {
		t.Fatalf("DeleteFolder failed: %v", err)
	}

	// Verify soft delete
	_, err = mgr.GetFolder(f1)
	if err == nil {
		t.Fatal("expected folder to be deleted")
	}
}
