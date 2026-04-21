package storage

import (
	"context"
	"io"
	"strings"
	"testing"
)

type mockProvider struct {
	scheme string
	stored map[string]string
}

func (m *mockProvider) Store(ctx context.Context, r io.Reader) (string, error) {
	data, _ := io.ReadAll(r)
	hash := string(data) // simplified for test
	m.stored[hash] = hash
	return hash, nil
}

func (m *mockProvider) Retrieve(ctx context.Context, hash string) (io.ReadCloser, error) {
	if _, ok := m.stored[hash]; !ok {
		return nil, ErrNotFound
	}
	return io.NopCloser(strings.NewReader(hash)), nil
}

func (m *mockProvider) Exists(ctx context.Context, hash string) (bool, error) {
	_, ok := m.stored[hash]
	return ok, nil
}

func (m *mockProvider) GetLocationType() string {
	return m.scheme
}

func (m *mockProvider) Delete(ctx context.Context, hash string) error {
	delete(m.stored, hash)
	return nil
}

func TestCASRegistry(t *testing.T) {
	registry := NewCASRegistry()
	
	p1 := &mockProvider{scheme: "local", stored: make(map[string]string)}
	p2 := &mockProvider{scheme: "s3", stored: make(map[string]string)}
	
	registry.Register(p1)
	registry.Register(p2)
	
	ctx := context.Background()
	
	// 1. Store in local
	uri1, err := registry.StoreInDefault(ctx, "local", strings.NewReader("data1"))
	if err != nil {
		t.Fatalf("StoreInDefault(local) failed: %v", err)
	}
	if !strings.HasPrefix(uri1, "local:///") {
		t.Errorf("expected local URI, got %s", uri1)
	}
	
	// 2. Store in s3
	uri2, err := registry.StoreInDefault(ctx, "s3", strings.NewReader("data2"))
	if err != nil {
		t.Fatalf("StoreInDefault(s3) failed: %v", err)
	}
	if !strings.HasPrefix(uri2, "s3:///") {
		t.Errorf("expected s3 URI, got %s", uri2)
	}
	
	// 3. Retrieve from local
	r1, err := registry.RetrieveByURI(ctx, uri1)
	if err != nil {
		t.Fatalf("RetrieveByURI(local) failed: %v", err)
	}
	d1, _ := io.ReadAll(r1)
	if string(d1) != "data1" {
		t.Errorf("expected data1, got %s", string(d1))
	}
	
	// 4. Retrieve from s3
	r2, err := registry.RetrieveByURI(ctx, uri2)
	if err != nil {
		t.Fatalf("RetrieveByURI(s3) failed: %v", err)
	}
	d2, _ := io.ReadAll(r2)
	if string(d2) != "data2" {
		t.Errorf("expected data2, got %s", string(d2))
	}
	
	// 5. Invalid scheme
	_, err = registry.RetrieveByURI(ctx, "invalid:///hash")
	if err == nil {
		t.Errorf("expected error for invalid scheme")
	}
}
