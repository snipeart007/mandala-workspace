package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestLogger(t *testing.T) {
	var buf bytes.Buffer
	Setup(&buf, slog.LevelInfo)

	Info("test message", "key", "value")

	var result map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &result)
	if err != nil {
		t.Fatalf("failed to unmarshal log output: %v", err)
	}

	if result["msg"] != "test message" {
		t.Errorf("expected msg 'test message', got '%v'", result["msg"])
	}

	if result["key"] != "value" {
		t.Errorf("expected key 'value', got '%v'", result["key"])
	}

	if result["level"] != "INFO" {
		t.Errorf("expected level 'INFO', got '%v'", result["level"])
	}
}
