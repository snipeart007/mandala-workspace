package postgres

import (
	"database/sql"
	"log/slog"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"mandala-workspace/internal/db"
)

type PostgresManager struct {
	db     *sql.DB
	config *db.DBManagerConfig
}

func NewPostgresManager(config *db.DBManagerConfig) (*PostgresManager, error) {
	dsn := config.DBPath // For postgres, we'll use DBPath as DSN for now, or add a DSN field to DBManagerConfig.
	slog.Info("Opening PostgreSQL database", "db_type", "postgres")
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		slog.Error("Failed to open PostgreSQL database", "error", err)
		return nil, err
	}
	return &PostgresManager{sqlDB, config}, nil
}

func (self *PostgresManager) Close() {
	slog.Info("Closing PostgreSQL database connection", "db_type", "postgres")
	self.db.Close()
}

func (self *PostgresManager) Setup() error {
	slog.Info("Setting up PostgreSQL database schema", "path", self.config.InitialSchemePath, "db_type", "postgres")
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

	slog.Info("Database schema initialized successfully", "db_type", "postgres")
	err = self.EnsureRootFolder()
	if err != nil {
		slog.Error("Failed to ensure root folder", "error", err)
		return err
	}
	return nil
}
