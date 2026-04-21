// Package storage provides the Content-Addressable Storage (CAS) interfaces and implementations.
// This file implements a local disk-based storage backend for the Mandala project.
package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

type LocalStorage struct {
	rootDir string
}

func NewLocalStorage(rootDir string) (*LocalStorage, error) {
	slog.Info("Initializing local storage", "root_dir", rootDir)
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		slog.Error("Failed to create local storage root", "root_dir", rootDir, "error", err)
		return nil, fmt.Errorf("failed to create storage root: %w", err)
	}
	return &LocalStorage{rootDir: rootDir}, nil
}

func (s *LocalStorage) GetLocationType() string {
	return "file"
}

func (s *LocalStorage) getPath(hash string) (string, string) {
	if len(hash) < 4 {
		return s.rootDir, filepath.Join(s.rootDir, hash)
	}
	shard1 := hash[:2]
	shard2 := hash[2:4]
	dir := filepath.Join(s.rootDir, shard1, shard2)
	return dir, filepath.Join(dir, hash)
}

func (s *LocalStorage) Store(ctx context.Context, r io.Reader) (string, error) {
	// Create a temp file to stream data into
	tmpFile, err := os.CreateTemp(s.rootDir, "upload-*")
	if err != nil {
		slog.Error("Failed to create temp file for storage", "error", err)
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tempName := tmpFile.Name()
	defer os.Remove(tempName)

	// Use TeeReader to calculate hash while writing
	hasher := sha256.New()
	multiWriter := io.MultiWriter(tmpFile, hasher)

	if _, err := io.Copy(multiWriter, r); err != nil {
		tmpFile.Close()
		slog.Error("Failed to write content to storage", "error", err)
		return "", fmt.Errorf("failed to write and hash content: %w", err)
	}
	tmpFile.Close()

	hash := hex.EncodeToString(hasher.Sum(nil))
	dir, path := s.getPath(hash)

	// If it already exists, just return the hash and delete temp
	if _, err := os.Stat(path); err == nil {
		slog.Debug("Content already exists in storage, skipping write", "hash", hash)
		return hash, nil
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Error("Failed to create shard directory", "error", err, "dir", dir)
		return "", fmt.Errorf("failed to create shard directory: %w", err)
	}

	if err := os.Rename(tempName, path); err != nil {
		slog.Error("Failed to move file to final storage path", "error", err, "path", path)
		return "", fmt.Errorf("failed to move file to storage: %w", err)
	}

	os.Chmod(path, 0444)
	slog.Info("Content stored in local storage", "hash", hash, "path", path)
	return hash, nil
}

func (s *LocalStorage) Retrieve(ctx context.Context, hash string) (io.ReadCloser, error) {
	slog.Debug("Retrieving content from local storage", "hash", hash)
	_, path := s.getPath(hash)

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Warn("Content not found in local storage", "hash", hash, "path", path)
			return nil, ErrNotFound
		}
		slog.Error("Failed to open local storage file", "hash", hash, "path", path, "error", err)
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return file, nil
}

func (s *LocalStorage) Exists(ctx context.Context, hash string) (bool, error) {
	slog.Debug("Checking if content exists in local storage", "hash", hash)
	_, path := s.getPath(hash)
	_, err := os.Stat(path)
	exists := err == nil || !os.IsNotExist(err)
	return exists, nil
}

func (s *LocalStorage) Delete(ctx context.Context, hash string) error {
	slog.Info("Deleting content from local storage", "hash", hash)
	_, path := s.getPath(hash)
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		slog.Error("Failed to delete local storage file", "hash", hash, "path", path, "error", err)
		return fmt.Errorf("failed to delete file: %w", err)
	}
	return nil
}
