package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func (self *DBManager) ListFiles(folderID uint64) ([]FileModel, error) {
	rows, err := self.db.Query(`
		SELECT file_id, name, folder_id, path, storage_path, location, version_id, metadata, created_at, updated_at
		FROM files 
		WHERE folder_id = ? AND deleted_at IS NULL
	`, folderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var files []FileModel
	for rows.Next() {
		var file FileModel
		var vid sql.NullInt64
		err := rows.Scan(
			&file.FileID, &file.Name, &file.FolderID, &file.Path, 
			&file.StoragePath, &file.Location, &vid, &file.Metadata, &file.CreatedAt, &file.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		if vid.Valid {
			file.VersionID = uint64(vid.Int64)
		}
		files = append(files, file)
	}
	return files, nil
}

func (self *DBManager) CreateFile(name string, folderID uint64, path string, storagePath string, location string, metadata []byte) (uint64, uint64, error) {
	now := uint64(time.Now().Unix())
	result, err := self.db.Exec(`
		INSERT INTO files (name, folder_id, path, storage_path, location, metadata, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, name, folderID, path, storagePath, location, metadata, now, now)
	if err != nil {
		return 0, 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, 0, err
	}
	return uint64(id), now, nil
}

func (self *DBManager) GetFile(fileID uint64) (*FileModel, error) {
	var file FileModel
	var vid sql.NullInt64
	err := self.db.QueryRow(`
		SELECT file_id, name, folder_id, path, storage_path, location, version_id, metadata, created_at, updated_at
		FROM files WHERE file_id = ? AND deleted_at IS NULL
	`, fileID).Scan(
		&file.FileID, &file.Name, &file.FolderID, &file.Path,
		&file.StoragePath, &file.Location, &vid, &file.Metadata, &file.CreatedAt, &file.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if vid.Valid {
		file.VersionID = uint64(vid.Int64)
	}
	return &file, nil
}

func (self *DBManager) GetFileByName(folderID uint64, name string) (*FileModel, error) {
	var file FileModel
	var vid sql.NullInt64
	err := self.db.QueryRow(`
		SELECT file_id, name, folder_id, path, storage_path, location, version_id, metadata, created_at, updated_at
		FROM files WHERE folder_id = ? AND name = ? AND deleted_at IS NULL
	`, folderID, name).Scan(
		&file.FileID, &file.Name, &file.FolderID, &file.Path,
		&file.StoragePath, &file.Location, &vid, &file.Metadata, &file.CreatedAt, &file.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if vid.Valid {
		file.VersionID = uint64(vid.Int64)
	}
	return &file, nil
}

func (self *DBManager) CreateVersion(fileID uint64, version string, hash []byte, userID uint64, metadata []byte, comment string) (uint64, error) {
	now := uint64(time.Now().Unix())
	result, err := self.db.Exec(`
		INSERT INTO versions (file_id, version, hash, user_id, metadata, version_comment, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, fileID, version, hash, userID, metadata, comment, now)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return uint64(id), nil
}

func (self *DBManager) UpdateFileVersion(fileID uint64, versionID uint64, storagePath string, location string) error {
	now := uint64(time.Now().Unix())
	_, err := self.db.Exec(`
		UPDATE files SET version_id = ?, storage_path = ?, location = ?, updated_at = ?
		WHERE file_id = ?
	`, versionID, storagePath, location, now, fileID)
	return err
}

func (self *DBManager) ListVersions(fileID uint64) ([]VersionModel, error) {
	rows, err := self.db.Query(`
		SELECT version_id, file_id, version, hash, user_id, metadata, version_comment, created_at
		FROM versions WHERE file_id = ? ORDER BY created_at DESC
	`, fileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []VersionModel
	for rows.Next() {
		var v VersionModel
		err := rows.Scan(&v.VersionID, &v.FileID, &v.Version, &v.Hash, &v.UserID, &v.Metadata, &v.VersionComment, &v.CreatedAt)
		if err != nil {
			return nil, err
		}
		versions = append(versions, v)
	}
	return versions, nil
}

func (self *DBManager) DeleteOldVersions(fileID uint64, keepLastN uint32) (int64, error) {
	if keepLastN == 0 {
		return 0, nil
	}

	// 1. Get the IDs of the versions we want to KEEP
	rows, err := self.db.Query(`
		SELECT version_id FROM versions 
		WHERE file_id = ? 
		ORDER BY version_id DESC LIMIT ?
	`, fileID, keepLastN)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var keepIDs []any
	for rows.Next() {
		var id uint64
		if err := rows.Scan(&id); err != nil {
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
		placeholders[i] = "?"
	}

	query := fmt.Sprintf("DELETE FROM versions WHERE file_id = ? AND version_id NOT IN (%s)", strings.Join(placeholders, ","))
	args := append([]any{fileID}, keepIDs...)

	result, err := self.db.Exec(query, args...)
	if err != nil {
		return 0, err
	}
	affected, _ := result.RowsAffected()
	fmt.Printf("DEBUG: DeleteOldVersions fileID=%d keepIDs=%v affected=%d\n", fileID, keepIDs, affected)
	return affected, nil
}

func (self *DBManager) SoftDeleteFile(fileID uint64) error {
	now := uint64(time.Now().Unix())
	_, err := self.db.Exec("UPDATE files SET deleted_at = ? WHERE file_id = ?", now, fileID)
	return err
}

func (self *DBManager) RenameFile(fileID uint64, newName string) error {
	now := uint64(time.Now().Unix())
	_, err := self.db.Exec("UPDATE files SET name = ?, updated_at = ? WHERE file_id = ?", newName, now, fileID)
	return err
}

func (self *DBManager) MoveFile(fileID uint64, newFolderID uint64, newPath string) error {
	now := uint64(time.Now().Unix())
	_, err := self.db.Exec("UPDATE files SET folder_id = ?, path = ?, updated_at = ? WHERE file_id = ?", newFolderID, newPath, now, fileID)
	return err
}

func (self *DBManager) UpdateFileMetadata(fileID uint64, metadata []byte) error {
	now := uint64(time.Now().Unix())
	_, err := self.db.Exec("UPDATE files SET metadata = ?, updated_at = ? WHERE file_id = ?", metadata, now, fileID)
	return err
}
