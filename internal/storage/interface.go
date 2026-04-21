package storage

import (
	"context"
	"errors"
	"io"
)

var (
	ErrIntegrityCheckFailed = errors.New("storage: integrity check failed (hash mismatch)")
	ErrNotFound             = errors.New("storage: content not found")
)

// CASProvider defines the interface for Content-Addressable Storage backends.
type CASProvider interface {
	// Store saves the content from the reader and returns its SHA-256 hash.
	Store(ctx context.Context, r io.Reader) (string, error)

	// Retrieve returns a reader for the content and performs an integrity check.
	// The caller is responsible for closing the reader.
	Retrieve(ctx context.Context, hash string) (io.ReadCloser, error)

	// Exists checks if content is already present.
	Exists(ctx context.Context, hash string) (bool, error)

	// GetLocationType returns the type (e.g., "file", "s3") this provider handles.
	GetLocationType() string
}
