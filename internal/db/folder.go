// Package db provides folder-related database operations including folder creation, path retrieval, and lifecycle management.
package db

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"
)

func (self *DBManager) EnsureRootFolder() error {
	slog.Debug("Ensuring root folder exists")
	var count int
	err := self.db.QueryRow("SELECT count(*) FROM folders WHERE folder_id = 1").Scan(&count)
	if err != nil {
		slog.Error("Failed to check for root folder", "error", err)
		return err
	}
	if count == 0 {
		_, err = self.db.Exec(`
			INSERT INTO folders (folder_id, name, path, created_at)
			VALUES (1, 'root', '', ?)
		`, time.Now().Unix())
		if err != nil {
			slog.Error("Failed to insert root folder", "error", err)
			return err
		}
		slog.Info("Root folder created successfully")
	}
	return nil
}

func (self *DBManager) GetFolderPath(folderID uint64) (string, error) {
	slog.Debug("Getting folder path", "folder_id", folderID)
	var path string
	err := self.db.QueryRow("SELECT path FROM folders WHERE folder_id = ?", folderID).Scan(&path)
	if err != nil {
		slog.Error("Failed to get folder path", "folder_id", folderID, "error", err)
		return "", err
	}
	return path, nil
}

func (self *DBManager) CreateFolder(name string, parentID uint64, path string, inheritance bool, retention uint32, metadata []byte) (uint64, uint64, error) {
	createdAt := uint64(time.Now().Unix())
	
	slog.Info("Creating folder", "name", name, "parent_id", parentID, "path", path)
	result, err := self.db.Exec(`
		INSERT INTO folders (name, parent_folder_id, path, inheritance, version_retention, metadata, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, name, parentID, path, inheritance, retention, metadata, createdAt)
	if err != nil {
		slog.Error("Failed to insert folder", "error", err, "name", name, "parent_id", parentID)
		return 0, 0, err
	}
	
	id, err := result.LastInsertId()
	if err != nil {
		slog.Error("Failed to get folder insert ID", "error", err)
		return 0, 0, err
	}
	
	slog.Info("Folder created", "folder_id", id, "name", name, "path", path)
	return uint64(id), createdAt, nil
}

func (self *DBManager) GetFolder(folderID uint64) (*FolderModel, error) {
	slog.Debug("Getting folder", "folder_id", folderID)
	var folder FolderModel
	var parentID sql.NullInt64
	
	err := self.db.QueryRow(`
		SELECT folder_id, name, parent_folder_id, path, inheritance, version_retention, metadata, created_at
		FROM folders 
		WHERE folder_id = ? AND deleted_at IS NULL
	`, folderID).Scan(
		&folder.FolderID, &folder.Name, &parentID, &folder.Path, 
		&folder.Inheritance, &folder.VersionRetention, &folder.Metadata, &folder.CreatedAt,
	)
	
	if err != nil {
		if err == sql.ErrNoRows {
			slog.Warn("Folder not found", "folder_id", folderID)
		} else {
			slog.Error("Failed to get folder", "folder_id", folderID, "error", err)
		}
		return nil, err
	}
	
	if parentID.Valid {
		folder.ParentFolderID = uint64(parentID.Int64)
	}
	
	return &folder, nil
}

func (self *DBManager) SetRetentionPolicy(folderID uint64, lastN uint32) error {
	slog.Info("Setting retention policy", "folder_id", folderID, "last_n", lastN)
	_, err := self.db.Exec(`
		UPDATE folders SET version_retention = ? WHERE folder_id = ?
	`, lastN, folderID)
	if err != nil {
		slog.Error("Failed to set retention policy", "folder_id", folderID, "error", err)
	}
	return err
}

func (self *DBManager) GetVersionRetention(folderID uint64) (uint32, error) {
	slog.Debug("Getting version retention", "folder_id", folderID)
	var retention uint32
	err := self.db.QueryRow("SELECT version_retention FROM folders WHERE folder_id = ?", folderID).Scan(&retention)
	if err != nil {
		slog.Error("Failed to get version retention", "folder_id", folderID, "error", err)
		return 0, err
	}
	return retention, nil
}

func (self *DBManager) ListFolders(parentID uint64) ([]FolderModel, error) {
	slog.Debug("Listing folders", "parent_id", parentID)
	rows, err := self.db.Query(`
		SELECT folder_id, name, parent_folder_id, path, inheritance, version_retention, metadata, created_at
		FROM folders 
		WHERE parent_folder_id = ? AND deleted_at IS NULL
	`, parentID)
	if err != nil {
		slog.Error("Failed to list folders", "parent_id", parentID, "error", err)
		return nil, err
	}
	defer rows.Close()
	
	var folders []FolderModel
	for rows.Next() {
		var folder FolderModel
		var pid sql.NullInt64
		err := rows.Scan(
			&folder.FolderID, &folder.Name, &pid, &folder.Path, 
			&folder.Inheritance, &folder.VersionRetention, &folder.Metadata, &folder.CreatedAt,
		)
		if err != nil {
			slog.Error("Failed to scan folder row", "error", err)
			return nil, err
		}
		if pid.Valid {
			folder.ParentFolderID = uint64(pid.Int64)
		}
		folders = append(folders, folder)
	}
	return folders, nil
}

func (self *DBManager) MoveFolder(folderID uint64, newParentID uint64, newPath string) error {
	slog.Info("Moving folder", "folder_id", folderID, "new_parent_id", newParentID, "new_path", newPath)
	tx, err := self.db.Begin()
	if err != nil {
		slog.Error("Failed to begin transaction for move folder", "error", err)
		return err
	}
	defer tx.Rollback()

	// 1. Get old path
	var oldPath string
	err = tx.QueryRow("SELECT path FROM folders WHERE folder_id = ? AND deleted_at IS NULL", folderID).Scan(&oldPath)
	if err != nil {
		slog.Error("Failed to get old path for move folder", "folder_id", folderID, "error", err)
		return err
	}
	
	// Full old prefix path would be "oldPath/folderID/"
	oldPrefix := fmt.Sprintf("%s%d/", oldPath, folderID)
	newPrefix := fmt.Sprintf("%s%d/", newPath, folderID)

	// 2. Update target folder
	_, err = tx.Exec(`
		UPDATE folders SET parent_folder_id = ?, path = ? WHERE folder_id = ?
	`, newParentID, newPath, folderID)
	if err != nil {
		slog.Error("Failed to update folder parent/path", "folder_id", folderID, "error", err)
		return err
	}

	// 3. Update all descendants' paths
	// We replace the oldPrefix with newPrefix in the path column
	_, err = tx.Exec(`
		UPDATE folders 
		SET path = ? || substr(path, ?)
		WHERE path LIKE ? || '%' AND deleted_at IS NULL
	`, newPrefix, len(oldPrefix)+1, oldPrefix)
	if err != nil {
		slog.Error("Failed to update descendants paths", "folder_id", folderID, "error", err)
		return err
	}

	if err := tx.Commit(); err != nil {
		slog.Error("Failed to commit transaction for move folder", "error", err)
		return err
	}
	slog.Info("Folder moved successfully", "folder_id", folderID)
	return nil
}

func (self *DBManager) SoftDeleteFolder(folderID uint64) error {
	slog.Info("Soft deleting folder", "folder_id", folderID)
	deletedAt := uint64(time.Now().Unix())
	tx, err := self.db.Begin()
	if err != nil {
		slog.Error("Failed to begin transaction for soft delete folder", "error", err)
		return err
	}
	defer tx.Rollback()

	// Get path to delete descendants too
	var path string
	err = tx.QueryRow("SELECT path FROM folders WHERE folder_id = ? AND deleted_at IS NULL", folderID).Scan(&path)
	if err != nil {
		slog.Error("Failed to get path for soft delete folder", "folder_id", folderID, "error", err)
		return err
	}
	
	prefix := fmt.Sprintf("%s%d/", path, folderID)

	// Delete folder
	_, err = tx.Exec("UPDATE folders SET deleted_at = ? WHERE folder_id = ?", deletedAt, folderID)
	if err != nil {
		slog.Error("Failed to mark folder as deleted", "folder_id", folderID, "error", err)
		return err
	}

	// Delete descendants
	_, err = tx.Exec("UPDATE folders SET deleted_at = ? WHERE path LIKE ? || '%' AND deleted_at IS NULL", deletedAt, prefix)
	if err != nil {
		slog.Error("Failed to mark descendants as deleted", "folder_id", folderID, "error", err)
		return err
	}
	
	// Delete files in deleted folders
	_, err = tx.Exec(`
		UPDATE files SET deleted_at = ? 
		WHERE (folder_id = ? OR folder_id IN (SELECT folder_id FROM folders WHERE path LIKE ? || '%'))
		AND deleted_at IS NULL
	`, deletedAt, folderID, prefix)
	if err != nil {
		slog.Error("Failed to mark files as deleted in folders", "folder_id", folderID, "error", err)
		return err
	}

	if err := tx.Commit(); err != nil {
		slog.Error("Failed to commit transaction for soft delete folder", "error", err)
		return err
	}
	slog.Info("Folder and descendants soft deleted successfully", "folder_id", folderID)
	return nil
}
