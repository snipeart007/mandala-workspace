package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
)

// CASRegistry acts as a router for different storage providers.
type CASRegistry struct {
	providers map[string]CASProvider
}

func NewCASRegistry() *CASRegistry {
	return &CASRegistry{
		providers: make(map[string]CASProvider),
	}
}

// Register adds a provider to the registry.
func (r *CASRegistry) Register(p CASProvider) {
	r.providers[p.GetLocationType()] = p
}

func (r *CASRegistry) getProviderFromURI(uri string) (CASProvider, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("invalid storage URI: %w", err)
	}

	scheme := u.Scheme
	if scheme == "" {
		return nil, fmt.Errorf("missing scheme in storage URI: %s", uri)
	}

	provider, ok := r.providers[scheme]
	if !ok {
		return nil, fmt.Errorf("no storage provider registered for scheme: %s", scheme)
	}

	return provider, nil
}

// RetrieveByURI fetches content based on a full locator URI (e.g., file:///path/to/hash)
func (r *CASRegistry) RetrieveByURI(ctx context.Context, uri string) (io.ReadCloser, error) {
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
	provider, ok := r.providers[scheme]
	if !ok {
		return "", fmt.Errorf("no storage provider for scheme: %s", scheme)
	}

	hash, err := provider.Store(ctx, reader)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s:///%s", scheme, hash), nil
}
