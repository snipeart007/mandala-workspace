// Package storage provides the Content-Addressable Storage (CAS) interfaces and implementations.
// This file implements a registry to manage and route requests between different storage providers.
package storage

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"strings"
)

// CASRegistry acts as a router for different storage providers.
type CASRegistry struct {
	providers map[string]CASProvider
}

func NewCASRegistry() *CASRegistry {
	slog.Debug("Creating new CAS registry")
	return &CASRegistry{
		providers: make(map[string]CASProvider),
	}
}

// Register adds a provider to the registry.
func (r *CASRegistry) Register(p CASProvider) {
	slog.Info("Registering storage provider", "type", p.GetLocationType())
	r.providers[p.GetLocationType()] = p
}

func (r *CASRegistry) getProviderFromURI(uri string) (CASProvider, error) {
	slog.Debug("Parsing storage URI", "uri", uri)
	u, err := url.Parse(uri)
	if err != nil {
		slog.Error("Invalid storage URI format", "uri", uri, "error", err)
		return nil, fmt.Errorf("invalid storage URI: %w", err)
	}

	scheme := u.Scheme
	if scheme == "" {
		slog.Warn("Storage URI missing scheme", "uri", uri)
		return nil, fmt.Errorf("missing scheme in storage URI: %s", uri)
	}

	provider, ok := r.providers[scheme]
	if !ok {
		slog.Warn("No storage provider for scheme", "scheme", scheme, "uri", uri)
		return nil, fmt.Errorf("no storage provider registered for scheme: %s", scheme)
	}

	return provider, nil
}

// RetrieveByURI fetches content based on a full locator URI (e.g., file:///path/to/hash)
func (r *CASRegistry) RetrieveByURI(ctx context.Context, uri string) (io.ReadCloser, error) {
	slog.Debug("Retrieving content by URI", "uri", uri)
	provider, err := r.getProviderFromURI(uri)
	if err != nil {
		return nil, err
	}

	parts := strings.Split(uri, "/")
	hash := parts[len(parts)-1]

	return provider.Retrieve(ctx, hash)
}

// StoreInDefault saves content from the reader to a specific provider and returns the URI.
func (r *CASRegistry) StoreInDefault(ctx context.Context, scheme string, reader io.Reader) (string, error) {
	slog.Debug("Storing content in default provider", "scheme", scheme)
	provider, ok := r.providers[scheme]
	if !ok {
		slog.Warn("No storage provider for scheme", "scheme", scheme)
		return "", fmt.Errorf("no storage provider for scheme: %s", scheme)
	}

	hash, err := provider.Store(ctx, reader)
	if err != nil {
		slog.Error("Failed to store content in provider", "scheme", scheme, "error", err)
		return "", err
	}

	uri := fmt.Sprintf("%s:///%s", scheme, hash)
	slog.Info("Content stored successfully", "uri", uri)
	return uri, nil
}
