package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalStorage_StoreAndRetrieve(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cas-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	s, err := NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	content := []byte("hello mandala workspace")
	
	hashBytes := sha256.Sum256(content)
	expectedHash := hex.EncodeToString(hashBytes[:])

	// 1. Store
	hash, err := s.Store(ctx, bytes.NewReader(content))
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}
	if hash != expectedHash {
		t.Fatalf("expected hash %s, got %s", expectedHash, hash)
	}

	// 2. Exists
	exists, err := s.Exists(ctx, hash)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Fatal("expected file to exist")
	}

	// 3. Retrieve
	rc, err := s.Retrieve(ctx, hash)
	if err != nil {
		t.Fatalf("Retrieve failed: %v", err)
	}
	defer rc.Close()

	retrieved, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("failed to read from retrieved reader: %v", err)
	}

	if string(retrieved) != string(content) {
		t.Fatalf("expected content %s, got %s", string(content), string(retrieved))
	}

	// 4. Sharding Check
	shard1 := hash[:2]
	shard2 := hash[2:4]
	expectedPath := filepath.Join(tmpDir, shard1, shard2, hash)
	if _, err := os.Stat(expectedPath); err != nil {
		t.Fatalf("file not found at expected sharded path: %v", err)
	}
}

func TestLocalStorage_DuplicateStore(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cas-dup-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	s, err := NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	content := []byte("duplicate data")

	hash1, err := s.Store(ctx, bytes.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}

	hash2, err := s.Store(ctx, bytes.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}

	if hash1 != hash2 {
		t.Fatalf("expected same hash for same content, got %s and %s", hash1, hash2)
	}
}
