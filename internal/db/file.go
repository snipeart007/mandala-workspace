package db

import (
	"database/sql"
	"time"
)

func (self *DBManager) ListFiles(folderID uint64) ([]FileModel, error) {
	rows, err := self.db.Query(`
		SELECT file_id, name, folder_id, path, storage_path, location, version_id, created_at, updated_at
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
			&file.StoragePath, &file.Location, &vid, &file.CreatedAt, &file.UpdatedAt,
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
