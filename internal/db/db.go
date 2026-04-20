package db

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type DBManagerConfig struct {
	InitialSchemePath string
}

type DBManager struct {
	db     *sql.DB
	config *DBManagerConfig
}

func NewDBManager(config *DBManagerConfig) (*DBManager, error) {
	db, err := sql.Open("sqlite3", "db.sqlite")
	if err != nil {
		return nil, err
	}
	return &DBManager{db, config}, nil
}

func (self *DBManager) Close() {
	self.db.Close()
}

// Only expects one filepath, not required as of now, only to facilitate testing
func (self *DBManager) Setup() error {
	query, err := os.ReadFile(self.config.InitialSchemePath)
	if err != nil {
		slog.Error("Cannot open" + self.config.InitialSchemePath)
		return err
	}
	_, err = self.db.Exec(string(query))
	if err != nil {
		slog.Error("Failed to initialize database schema")
		return err
	}

	slog.Info("Database schema initialized successfully")
	return nil
}

// GetDevicePublicKey retrieves the public key for a device
// userID: the user ID
// deviceID: the device ID
// Returns: the public key bytes, or error if device not found
func (self *DBManager) GetDevicePublicKey(userID uint64, deviceID uint64) ([]byte, error) {
	var publicKey []byte
	
	err := self.db.QueryRow(
		"SELECT public_key FROM devices WHERE user_id = ? AND device_id = ?",
		userID,
		deviceID,
	).Scan(&publicKey)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("device not found for user_id=%d, device_id=%d", userID, deviceID)
		}
		return nil, fmt.Errorf("database error: %w", err)
	}
	
	return publicKey, nil
}

func (self *DBManager) EnsureRootFolder() error {
	var count int
	err := self.db.QueryRow("SELECT COUNT(*) FROM folders WHERE folder_id = 1").Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		_, err = self.db.Exec(`
			INSERT INTO folders (folder_id, name, path, created_at)
			VALUES (1, 'root', '', ?)
		`, time.Now().Unix())
		if err != nil {
			return err
		}
	}
	return nil
}

func (self *DBManager) GetUserPermissionBitmask(userID uint64, folderID uint64) (uint64, error) {
	var permissions uint64
	err := self.db.QueryRow(
		"SELECT permissions FROM permissions WHERE user_id = ? AND folder_id = ?",
		userID, folderID,
	).Scan(&permissions)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}
	
	return permissions, nil
}

func (self *DBManager) CreateUser(name string, email string, passwordHash []byte, metadata []byte) (uint64, uint64, error) {
	createdAt := uint64(time.Now().Unix())
	
	result, err := self.db.Exec(
		"INSERT INTO users (name, email, password_hash, metadata, created_at) VALUES (?, ?, ?, ?, ?)",
		name, email, passwordHash, metadata, createdAt,
	)
	if err != nil {
		return 0, 0, err
	}
	
	id, err := result.LastInsertId()
	if err != nil {
		return 0, 0, err
	}
	
	return uint64(id), createdAt, nil
}

func (self *DBManager) GetUserCount() (int, error) {
	var count int
	err := self.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (self *DBManager) SetUserPermission(userID uint64, folderID uint64, permissions uint64) error {
	_, err := self.db.Exec(`
		INSERT INTO permissions (user_id, folder_id, permissions)
		VALUES (?, ?, ?)
		ON CONFLICT(user_id, folder_id) DO UPDATE SET permissions = excluded.permissions
	`, userID, folderID, permissions)
	return err
}


