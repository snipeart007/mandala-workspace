package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"mandala-workspace/internal/db"
	"mandala-workspace/internal/server"
)

func main() {
	// For production, these would be loaded from environment variables or a config file.
	config := &server.ServerInstanceConfig{
		GRPCAddr: ":50051",
		DB: db.DBManagerConfig{
			DBPath:            "db.sqlite",
			InitialSchemePath: "internal/db/sql/InitializeDB.sql",
		},
		DBType:               "sqlite",
		PasetoSecretKey:      []byte("01234567890123456789012345678901"), // Exactly 32 bytes
		LocalStoragePath:     "storage_data",
		DefaultStorageScheme: "file",
	}

	// Override config with env variables if present
	if addr := os.Getenv("MANDALA_GRPC_ADDR"); addr != "" {
		config.GRPCAddr = addr
	}
	if key := os.Getenv("MANDALA_PASETO_KEY"); key != "" {
		config.PasetoSecretKey = []byte(key)
	}

	instance, err := server.NewServerInstance(config)
	if err != nil {
		slog.Error("Failed to initialize server instance", "error", err)
		os.Exit(1)
	}

	// Handle graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := instance.Start(); err != nil {
			slog.Error("Server stopped with error", "error", err)
			os.Exit(1)
		}
	}()

	slog.Info("Server is running. Press Ctrl+C to stop.")
	<-stop

	slog.Info("Shutting down server...")
	instance.Stop()
	slog.Info("Server gracefully stopped")
}
