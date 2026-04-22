// Package db handles database interactions and schema initialization for the mandala-workspace.
package db

import (
	"database/sql"
	"log/slog"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

type DBManagerConfig struct {
	InitialSchemePath string
	DBPath            string
}

type DBManager struct {
	db     *sql.DB
	config *DBManagerConfig
}

func NewDBManager(config *DBManagerConfig) (*DBManager, error) {
	dbPath := config.DBPath
	if dbPath == "" {
		dbPath = "db.sqlite"
	}
	slog.Info("Opening SQLite database", "file", dbPath)
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		slog.Error("Failed to open SQLite database", "error", err)
		return nil, err
	}
	return &DBManager{db, config}, nil
}

func NewDBManagerWithDB(db *sql.DB, config *DBManagerConfig) *DBManager {
	slog.Info("Using existing database connection")
	return &DBManager{db, config}
}

func (self *DBManager) Close() {
	slog.Info("Closing database connection")
	self.db.Close()
}

// Setup initializes the database schema and ensures the root folder exists.
func (self *DBManager) Setup() error {
	slog.Info("Setting up database schema", "path", self.config.InitialSchemePath)
	query, err := os.ReadFile(self.config.InitialSchemePath)
	if err != nil {
		slog.Error("Cannot read initial scheme file", "path", self.config.InitialSchemePath, "error", err)
		return err
	}
	_, err = self.db.Exec(string(query))
	if err != nil {
		slog.Error("Failed to initialize database schema", "error", err)
		return err
	}

	slog.Info("Database schema initialized successfully")
	err = self.EnsureRootFolder()
	if err != nil {
		slog.Error("Failed to ensure root folder", "error", err)
		return err
	}
	return nil
}
