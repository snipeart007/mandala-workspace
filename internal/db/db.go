package db

import (
	"database/sql"
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

func NewDBManagerWithDB(db *sql.DB, config *DBManagerConfig) *DBManager {
	return &DBManager{db, config}
}

func (self *DBManager) Close() {
	self.db.Close()
}

// Setup initializes the database schema and ensures the root folder exists.
func (self *DBManager) Setup() error {
	query, err := os.ReadFile(self.config.InitialSchemePath)
	if err != nil {
		slog.Error("Cannot open " + self.config.InitialSchemePath)
		return err
	}
	_, err = self.db.Exec(string(query))
	if err != nil {
		slog.Error("Failed to initialize database schema")
		return err
	}

	slog.Info("Database schema initialized successfully")
	err = self.EnsureRootFolder()
	if err != nil {
		slog.Error("Failed to ensure root folder")
		return err
	}
	return nil
}
