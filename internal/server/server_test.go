package server

import (
	"mandala-workspace/internal/db"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewServerInstance(t *testing.T) {
	// Create a temp directory for DB and storage
	tmpDir, err := os.MkdirTemp("", "server-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a dummy schema file
	schemaPath := filepath.Join(tmpDir, "schema.sql")
	err = os.WriteFile(schemaPath, []byte("CREATE TABLE dummy (id INTEGER);"), 0644)
	if err != nil {
		t.Fatalf("failed to create dummy schema: %v", err)
	}

	validConfig := &ServerInstanceConfig{
		GRPCAddr: ":0", // Random port
		DB: db.DBManagerConfig{
			InitialSchemePath: schemaPath,
			DBPath:            filepath.Join(tmpDir, "test.db"),
		},
		DBType:               "sqlite",
		PasetoSecretKey:      []byte("01234567890123456789012345678901"), // 32 bytes
		LocalStoragePath:     filepath.Join(tmpDir, "storage"),
		DefaultStorageScheme: "file",
	}

	t.Run("ValidConfig", func(t *testing.T) {
		instance, err := NewServerInstance(validConfig)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if instance == nil {
			t.Fatal("expected instance to not be nil")
		}

		// Check if helper methods return the correct components
		if instance.GetDBProvider() == nil {
			t.Error("expected DBProvider to be initialized")
		}
		if instance.GetPasetoManager() == nil {
			t.Error("expected PasetoManager to be initialized")
		}
		if instance.GetSessionManager() == nil {
			t.Error("expected SessionManager to be initialized")
		}
		if instance.GetPermissionManager() == nil {
			t.Error("expected PermissionManager to be initialized")
		}
		if instance.GetStorageRegistry() == nil {
			t.Error("expected StorageRegistry to be initialized")
		}
	})

	t.Run("InvalidPasetoKey", func(t *testing.T) {
		invalidConfig := *validConfig
		invalidConfig.PasetoSecretKey = []byte("short")
		_, err := NewServerInstance(&invalidConfig)
		if err == nil {
			t.Error("expected error for short PASETO key, got nil")
		}
	})
}

func TestServerLifecycle(t *testing.T) {
	// Create a temp directory for DB and storage
	tmpDir, err := os.MkdirTemp("", "server-lifecycle-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Use real schema if possible, or just a minimal one. 
	// Since DBManager.Setup calls EnsureRootFolder, we need a schema that matches what it expects.
	// Let's find the real schema path.
	realSchemaPath := "../../internal/db/sql/InitializeDB.sql"

	config := &ServerInstanceConfig{
		GRPCAddr: "localhost:0", // Random port
		DB: db.DBManagerConfig{
			InitialSchemePath: realSchemaPath,
			DBPath:            filepath.Join(tmpDir, "lifecycle.db"),
		},
		DBType:               "sqlite",
		PasetoSecretKey:      []byte("01234567890123456789012345678901"),
		LocalStoragePath:     filepath.Join(tmpDir, "storage"),
		DefaultStorageScheme: "file",
	}

	instance, err := NewServerInstance(config)
	if err != nil {
		t.Fatalf("failed to create server instance: %v", err)
	}

	// Start server in a goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := instance.Start(); err != nil {
			errChan <- err
		}
	}()

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Check for immediate errors
	select {
	case err := <-errChan:
		t.Fatalf("server failed to start: %v", err)
	default:
		// Server started successfully
	}

	// Stop server
	instance.Stop()

	// Wait a bit for it to stop
	time.Sleep(100 * time.Millisecond)
}
