package postgres

import (
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"mandala-workspace/internal/db"
)

func (self *PostgresManager) ListFiles(folderID uint64) ([]db.FileModel, error) {
	slog.Debug("Listing files", "folder_id", folderID, "db_type", "postgres")
	rows, err := self.db.Query(`
		SELECT file_id, name, folder_id, path, storage_path, location, version_id, metadata, created_at, updated_at
		FROM files 
		WHERE folder_id = $1 AND deleted_at IS NULL
	`, folderID)
	if err != nil {
		slog.Error("Failed to list files", "folder_id", folderID, "error", err)
		return nil, err
	}
	defer rows.Close()
	
	var files []db.FileModel
	for rows.Next() {
		var file db.FileModel
		var vid sql.NullInt64
		err := rows.Scan(
			&file.FileID, &file.Name, &file.FolderID, &file.Path, 
			&file.StoragePath, &file.Location, &vid, &file.Metadata, &file.CreatedAt, &file.UpdatedAt,
		)
		if err != nil {
			slog.Error("Failed to scan file row", "error", err)
			return nil, err
		}
		if vid.Valid {
			file.VersionID = uint64(vid.Int64)
		}
		files = append(files, file)
	}
	return files, nil
}

func (self *PostgresManager) CreateFile(name string, folderID uint64, path string, storagePath string, location string, metadata []byte) (uint64, uint64, error) {
	now := uint64(time.Now().Unix())
	slog.Info("Creating file", "name", name, "folder_id", folderID, "path", path, "db_type", "postgres")
	var id int64
	err := self.db.QueryRow(`
		INSERT INTO files (name, folder_id, path, storage_path, location, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING file_id
	`, name, folderID, path, storagePath, location, metadata, now, now).Scan(&id)
	
	if err != nil {
		slog.Error("Failed to insert file", "name", name, "folder_id", folderID, "error", err)
		return 0, 0, err
	}
	slog.Info("File created successfully", "file_id", id, "name", name)
	return uint64(id), now, nil
}

func (self *PostgresManager) GetFile(fileID uint64) (*db.FileModel, error) {
	slog.Debug("Getting file", "file_id", fileID, "db_type", "postgres")
	var file db.FileModel
	var vid sql.NullInt64
	err := self.db.QueryRow(`
		SELECT file_id, name, folder_id, path, storage_path, location, version_id, metadata, created_at, updated_at
		FROM files WHERE file_id = $1 AND deleted_at IS NULL
	`, fileID).Scan(
		&file.FileID, &file.Name, &file.FolderID, &file.Path,
		&file.StoragePath, &file.Location, &vid, &file.Metadata, &file.CreatedAt, &file.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			slog.Warn("File not found", "file_id", fileID)
		} else {
			slog.Error("Failed to get file", "file_id", fileID, "error", err)
		}
		return nil, err
	}
	if vid.Valid {
		file.VersionID = uint64(vid.Int64)
	}
	return &file, nil
}

func (self *PostgresManager) GetFileByName(folderID uint64, name string) (*db.FileModel, error) {
	slog.Debug("Getting file by name", "folder_id", folderID, "name", name, "db_type", "postgres")
	var file db.FileModel
	var vid sql.NullInt64
	err := self.db.QueryRow(`
		SELECT file_id, name, folder_id, path, storage_path, location, version_id, metadata, created_at, updated_at
		FROM files WHERE folder_id = $1 AND name = $2 AND deleted_at IS NULL
	`, folderID, name).Scan(
		&file.FileID, &file.Name, &file.FolderID, &file.Path,
		&file.StoragePath, &file.Location, &vid, &file.Metadata, &file.CreatedAt, &file.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			slog.Warn("File not found by name", "folder_id", folderID, "name", name)
		} else {
			slog.Error("Failed to get file by name", "folder_id", folderID, "name", name, "error", err)
		}
		return nil, err
	}
	if vid.Valid {
		file.VersionID = uint64(vid.Int64)
	}
	return &file, nil
}

func (self *PostgresManager) CreateVersion(fileID uint64, version string, hash []byte, userID uint64, metadata []byte, comment string) (uint64, error) {
	now := uint64(time.Now().Unix())
	slog.Info("Creating file version", "file_id", fileID, "version", version, "user_id", userID, "db_type", "postgres")
	var id int64
	err := self.db.QueryRow(`
		INSERT INTO versions (file_id, version, hash, user_id, metadata, version_comment, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING version_id
	`, fileID, version, hash, userID, metadata, comment, now).Scan(&id)
	
	if err != nil {
		slog.Error("Failed to insert file version", "file_id", fileID, "version", version, "error", err)
		return 0, err
	}
	slog.Info("File version created successfully", "version_id", id, "file_id", fileID)
	return uint64(id), nil
}

func (self *PostgresManager) UpdateFileVersion(fileID uint64, versionID uint64, storagePath string, location string) error {
	now := uint64(time.Now().Unix())
	slog.Info("Updating file with new version", "file_id", fileID, "version_id", versionID, "db_type", "postgres")
	_, err := self.db.Exec(`
		UPDATE files SET version_id = $1, storage_path = $2, location = $3, updated_at = $4
		WHERE file_id = $5
	`, versionID, storagePath, location, now, fileID)
	if err != nil {
		slog.Error("Failed to update file version", "file_id", fileID, "version_id", versionID, "error", err)
	}
	return err
}

func (self *PostgresManager) ListVersions(fileID uint64) ([]db.VersionModel, error) {
	slog.Debug("Listing file versions", "file_id", fileID, "db_type", "postgres")
	rows, err := self.db.Query(`
		SELECT version_id, file_id, version, hash, user_id, metadata, version_comment, created_at
		FROM versions WHERE file_id = $1 ORDER BY created_at DESC
	`, fileID)
	if err != nil {
		slog.Error("Failed to list versions", "file_id", fileID, "error", err)
		return nil, err
	}
	defer rows.Close()

	var versions []db.VersionModel
	for rows.Next() {
		var v db.VersionModel
		err := rows.Scan(&v.VersionID, &v.FileID, &v.Version, &v.Hash, &v.UserID, &v.Metadata, &v.VersionComment, &v.CreatedAt)
		if err != nil {
			slog.Error("Failed to scan version row", "error", err)
			return nil, err
		}
		versions = append(versions, v)
	}
	return versions, nil
}

func (self *PostgresManager) DeleteOldVersions(fileID uint64, keepLastN uint32) (int64, error) {
	if keepLastN == 0 {
		return 0, nil
	}
	slog.Info("Deleting old file versions", "file_id", fileID, "keep_last_n", keepLastN, "db_type", "postgres")

	// 1. Get the IDs of the versions we want to KEEP
	rows, err := self.db.Query(`
		SELECT version_id FROM versions 
		WHERE file_id = $1 
		ORDER BY version_id DESC LIMIT $2
	`, fileID, keepLastN)
	if err != nil {
		slog.Error("Failed to get versions to keep", "file_id", fileID, "error", err)
		return 0, err
	}
	defer rows.Close()

	var keepIDs []any
	for rows.Next() {
		var id uint64
		if err := rows.Scan(&id); err != nil {
			slog.Error("Failed to scan version ID", "error", err)
			return 0, err
		}
		keepIDs = append(keepIDs, id)
	}

	if len(keepIDs) == 0 {
		return 0, nil
	}

	// 2. Delete versions NOT in the keep list
	placeholders := make([]string, len(keepIDs))
	for i := range placeholders {
		placeholders[i] = fmt.Sprintf("$%d", i+2)
	}

	query := fmt.Sprintf("DELETE FROM versions WHERE file_id = $1 AND version_id NOT IN (%s)", strings.Join(placeholders, ","))
	args := append([]any{fileID}, keepIDs...)

	result, err := self.db.Exec(query, args...)
	if err != nil {
		slog.Error("Failed to delete old versions", "file_id", fileID, "error", err)
		return 0, err
	}
	affected, _ := result.RowsAffected()
	slog.Info("Old versions deleted", "file_id", fileID, "affected_rows", affected)
	return affected, nil
}

func (self *PostgresManager) SoftDeleteFile(fileID uint64) error {
	now := uint64(time.Now().Unix())
	slog.Info("Soft deleting file", "file_id", fileID, "db_type", "postgres")
	_, err := self.db.Exec("UPDATE files SET deleted_at = $1 WHERE file_id = $2", now, fileID)
	if err != nil {
		slog.Error("Failed to soft delete file", "file_id", fileID, "error", err)
	}
	return err
}

func (self *PostgresManager) RenameFile(fileID uint64, newName string) error {
	now := uint64(time.Now().Unix())
	slog.Info("Renaming file", "file_id", fileID, "new_name", newName, "db_type", "postgres")
	_, err := self.db.Exec("UPDATE files SET name = $1, updated_at = $2 WHERE file_id = $3", newName, now, fileID)
	if err != nil {
		slog.Error("Failed to rename file", "file_id", fileID, "new_name", newName, "error", err)
	}
	return err
}

func (self *PostgresManager) MoveFile(fileID uint64, newFolderID uint64, newPath string) error {
	now := uint64(time.Now().Unix())
	slog.Info("Moving file", "file_id", fileID, "new_folder_id", newFolderID, "new_path", newPath, "db_type", "postgres")
	_, err := self.db.Exec("UPDATE files SET folder_id = $1, path = $2, updated_at = $3 WHERE file_id = $4", newFolderID, newPath, now, fileID)
	if err != nil {
		slog.Error("Failed to move file", "file_id", fileID, "new_folder_id", newFolderID, "error", err)
	}
	return err
}

func (self *PostgresManager) UpdateFileMetadata(fileID uint64, metadata []byte) error {
	now := uint64(time.Now().Unix())
	slog.Info("Updating file metadata", "file_id", fileID, "db_type", "postgres")
	_, err := self.db.Exec("UPDATE files SET metadata = $1, updated_at = $2 WHERE file_id = $3", metadata, now, fileID)
	if err != nil {
		slog.Error("Failed to update file metadata", "file_id", fileID, "error", err)
	}
	return err
}
