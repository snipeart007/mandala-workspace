package postgres

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"
)

func (self *PostgresManager) GetDevicePublicKey(userID uint64, deviceID uint64) ([]byte, error) {
	slog.Debug("Fetching device public key", "user_id", userID, "device_id", deviceID, "db_type", "postgres")
	var publicKey []byte
	
	err := self.db.QueryRow(
		"SELECT public_key FROM devices WHERE user_id = $1 AND device_id = $2 AND revoked_at IS NULL",
		userID,
		deviceID,
	).Scan(&publicKey)
	
	if err != nil {
		if err == sql.ErrNoRows {
			slog.Warn("Device not found or revoked", "user_id", userID, "device_id", deviceID)
			return nil, fmt.Errorf("device not found or revoked for user_id=%d, device_id=%d", userID, deviceID)
		}
		slog.Error("Database error fetching device public key", "error", err, "user_id", userID, "device_id", deviceID)
		return nil, fmt.Errorf("database error: %w", err)
	}
	
	return publicKey, nil
}

func (self *PostgresManager) CreateUser(name string, email string, passwordHash []byte, metadata []byte) (uint64, uint64, error) {
	createdAt := uint64(time.Now().Unix())
	
	slog.Info("Creating user", "name", name, "email", email, "db_type", "postgres")
	var id int64
	err := self.db.QueryRow(
		"INSERT INTO users (name, email, password_hash, metadata, created_at) VALUES ($1, $2, $3, $4, $5) RETURNING user_id",
		name, email, passwordHash, metadata, createdAt,
	).Scan(&id)
	
	if err != nil {
		slog.Error("Failed to insert user", "error", err, "email", email)
		return 0, 0, err
	}
	
	slog.Info("User created", "user_id", id, "email", email)
	return uint64(id), createdAt, nil
}

func (self *PostgresManager) GetUserCount() (int, error) {
	slog.Debug("Getting user count", "db_type", "postgres")
	var count int
	err := self.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		slog.Error("Failed to get user count", "error", err)
		return 0, err
	}
	return count, nil
}

func (self *PostgresManager) RegisterDevice(userID uint64, publicKey []byte, metadata []byte) (uint64, uint64, error) {
	createdAt := uint64(time.Now().Unix())

	slog.Info("Registering device", "user_id", userID, "db_type", "postgres")
	var id int64
	err := self.db.QueryRow(
		"INSERT INTO devices (user_id, public_key, metadata, created_at) VALUES ($1, $2, $3, $4) RETURNING device_id",
		userID, publicKey, metadata, createdAt,
	).Scan(&id)
	
	if err != nil {
		slog.Error("Failed to insert device", "error", err, "user_id", userID)
		return 0, 0, err
	}

	slog.Info("Device registered", "device_id", id, "user_id", userID)
	return uint64(id), createdAt, nil
}

func (self *PostgresManager) RevokeDevice(userID uint64, deviceID uint64) error {
	revokedAt := uint64(time.Now().Unix())
	slog.Info("Revoking device", "user_id", userID, "device_id", deviceID, "db_type", "postgres")
	_, err := self.db.Exec(
		"UPDATE devices SET revoked_at = $1 WHERE user_id = $2 AND device_id = $3 AND revoked_at IS NULL",
		revokedAt, userID, deviceID,
	)
	if err != nil {
		slog.Error("Failed to revoke device", "error", err, "user_id", userID, "device_id", deviceID)
		return err
	}
	slog.Info("Device revoked", "user_id", userID, "device_id", deviceID)
	return nil
}
