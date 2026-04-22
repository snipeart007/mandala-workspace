package sqlite

import (
	"database/sql"
	"log/slog"
	"os"

	_ "github.com/mattn/go-sqlite3"
	"mandala-workspace/internal/db"
)

type SQLiteManager struct {
	db     *sql.DB
	config *db.DBManagerConfig
}

func NewSQLiteManager(config *db.DBManagerConfig) (*SQLiteManager, error) {
	dbPath := config.DBPath
	if dbPath == "" {
		dbPath = "db.sqlite"
	}
	slog.Info("Opening SQLite database", "file", dbPath, "db_type", "sqlite")
	sqlDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		slog.Error("Failed to open SQLite database", "error", err)
		return nil, err
	}
	return &SQLiteManager{sqlDB, config}, nil
}

func (self *SQLiteManager) Close() {
	slog.Info("Closing SQLite database connection", "db_type", "sqlite")
	self.db.Close()
}

// Setup initializes the database schema and ensures the root folder exists.
func (self *SQLiteManager) Setup() error {
	slog.Info("Setting up SQLite database schema", "path", self.config.InitialSchemePath, "db_type", "sqlite")
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

	slog.Info("Database schema initialized successfully", "db_type", "sqlite")
	err = self.EnsureRootFolder()
	if err != nil {
		slog.Error("Failed to ensure root folder", "error", err)
		return err
	}
	return nil
}
