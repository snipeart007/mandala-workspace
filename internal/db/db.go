package db

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"

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

