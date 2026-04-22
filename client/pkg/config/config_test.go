package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mandala-config-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	path := filepath.Join(tmpDir, "config.json")
	cfg := &Config{
		ServerAddr: "test:1234",
		UseTLS:     true,
		TempDir:    "/tmp/test",
	}

	// Test Manual Save to specific path
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}
	err = os.WriteFile(path, data, 0644)
	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	// Test Manual Load from specific path
	data2, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	var cfg2 Config
	err = json.Unmarshal(data2, &cfg2)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if cfg2.ServerAddr != cfg.ServerAddr {
		t.Errorf("expected %s, got %s", cfg.ServerAddr, cfg2.ServerAddr)
	}
}
