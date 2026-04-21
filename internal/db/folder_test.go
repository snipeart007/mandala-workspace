// Package db contains tests for folder-related database operations and hierarchy management.
// It verifies that folders and files are correctly stored, listed, moved, and soft-deleted.
package db

import (
	"testing"
)

func TestFolderOperations(t *testing.T) {
	mgr := NewTestManager(t, "")
	defer mgr.Close()

	// 1. Setup tables
	_, err := mgr.db.Exec(`
		CREATE TABLE folders (
			folder_id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			parent_folder_id INTEGER,
			path TEXT NOT NULL,
			inheritance BOOLEAN DEFAULT 1,
			version_retention INTEGER DEFAULT 0,
			metadata BLOB,
			merkle_root BLOB,
			created_at INTEGER NOT NULL,
			deleted_at INTEGER,
			FOREIGN KEY(parent_folder_id) REFERENCES folders(folder_id)
		);
		CREATE TABLE files (
			file_id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			folder_id INTEGER NOT NULL,
			metadata BLOB,
			path TEXT NOT NULL,
			storage_path TEXT NOT NULL,
			location TEXT NOT NULL,
			version_id INTEGER, 
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			deleted_at INTEGER,
			FOREIGN KEY(folder_id) REFERENCES folders(folder_id)
		);
		CREATE TABLE permissions (
			user_id INTEGER NOT NULL,
			folder_id INTEGER NOT NULL,
			permissions INTEGER NOT NULL,
			UNIQUE(user_id, folder_id)
		);
	`)
	if err != nil {
		t.Fatalf("failed to setup tables: %v", err)
	}

	// 2. Test CreateFolder
	rootID, _, err := mgr.CreateFolder("root", 0, "", true, 0, nil)
	if err != nil {
		t.Fatalf("failed to create root: %v", err)
	}
	if rootID != 1 {
		t.Errorf("expected root ID 1, got %d", rootID)
	}

	childID, _, err := mgr.CreateFolder("child", rootID, "1/", true, 0, nil)
	if err != nil {
		t.Fatalf("failed to create child: %v", err)
	}

	// 3. Test GetFolder
	folder, err := mgr.GetFolder(childID)
	if err != nil {
		t.Fatalf("failed to get folder: %v", err)
	}
	if folder.Name != "child" || folder.ParentFolderID != rootID || folder.Path != "1/" {
		t.Errorf("folder mismatch: %+v", folder)
	}

	// 4. Test ListFolders
	folders, err := mgr.ListFolders(rootID)
	if err != nil {
		t.Fatalf("failed to list folders: %v", err)
	}
	if len(folders) != 1 || folders[0].FolderID != childID {
		t.Errorf("expected 1 child folder, got %d", len(folders))
	}

	// 5. Test MoveFolder
	otherID, _, err := mgr.CreateFolder("other", rootID, "1/", true, 0, nil)
	if err != nil {
		t.Fatalf("failed to create other root child: %v", err)
	}

	// Move 'child' under 'other'
	newPath := "1/3/" // otherID is 3
	err = mgr.MoveFolder(childID, otherID, newPath)
	if err != nil {
		t.Fatalf("failed to move folder: %v", err)
	}

	folder, _ = mgr.GetFolder(childID)
	if folder.ParentFolderID != otherID || folder.Path != newPath {
		t.Errorf("move failed: %+v", folder)
	}

	// 6. Test SoftDelete
	err = mgr.SoftDeleteFolder(otherID)
	if err != nil {
		t.Fatalf("failed to delete folder: %v", err)
	}

	// Should not be able to get deleted folder
	_, err = mgr.GetFolder(otherID)
	if err == nil {
		t.Error("expected error getting deleted folder, got nil")
	}

	// Child should also be deleted (recursive)
	_, err = mgr.GetFolder(childID)
	if err == nil {
		t.Error("expected error getting deleted child folder, got nil")
	}

	// ListFolders should not return deleted folders
	folders, _ = mgr.ListFolders(rootID)
	if len(folders) != 0 {
		t.Errorf("expected 0 folders under root after deletion, got %d", len(folders))
	}
}
