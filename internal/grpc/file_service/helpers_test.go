// Package file_service provides helper functions and mock implementations for testing the file service.
// These utilities simplify the creation of test environments and mock gRPC streams.
package file_service

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"mandala-workspace/gen"
	"mandala-workspace/internal/crypto/paseto"
	"mandala-workspace/internal/db"
	"mandala-workspace/internal/db/sqlite"
	"mandala-workspace/internal/grpc/interceptors"
	"mandala-workspace/internal/permission"
	"mandala-workspace/internal/storage"

	"google.golang.org/grpc/metadata"
)

func setupTestServer(t *testing.T) (*FileServiceServer, db.DBProvider, uint64, string) {
	tmpDir := t.TempDir()
	sqlPath := filepath.Join(tmpDir, "init.sql")
	schema, _ := os.ReadFile("../../db/sql/InitializeDB.sql")
	os.WriteFile(sqlPath, schema, 0644)

	mgr, err := sqlite.NewSQLiteManager(&db.DBManagerConfig{
		InitialSchemePath: sqlPath,
		DBPath:            filepath.Join(tmpDir, "db.sqlite"),
	})
	if err != nil {
		t.Fatalf("failed to create DB manager: %v", err)
	}
	mgr.Setup()

	userID, _, _ := mgr.CreateUser("testuser", "test@example.com", []byte("hash"), nil)
	
	registry := storage.NewCASRegistry()
	localStore, _ := storage.NewLocalStorage(filepath.Join(tmpDir, "storage"))
	registry.Register(localStore)

	pm := permission.NewPermissionManager(mgr)
	server := NewFileServiceServer(mgr, registry, pm, "file")

	return server, mgr, userID, tmpDir
}

func contextWithUser(userID uint64) context.Context {
	claims := paseto.TokenClaims{UserID: userID, DeviceID: 1}
	return context.WithValue(context.Background(), interceptors.TokenClaimsContextKey, claims)
}

type mockUploadStream struct {
	ctx          context.Context
	metadataMsg  *gen.UploadFileRequest
	chunkSize    int
	numChunks    int
	currentIndex int
	sentMetadata bool
	recvFunc     func() (*gen.UploadFileRequest, error)
	resp         *gen.FileResponse
	isClosed     bool
}

func (m *mockUploadStream) Recv() (*gen.UploadFileRequest, error) {
	if m.recvFunc != nil {
		return m.recvFunc()
	}
	if !m.sentMetadata {
		m.sentMetadata = true
		return m.metadataMsg, nil
	}
	if m.currentIndex >= m.numChunks {
		return nil, io.EOF
	}
	m.currentIndex++
	return &gen.UploadFileRequest{
		Data: &gen.UploadFileRequest_Chunk{Chunk: make([]byte, m.chunkSize)},
	}, nil
}
func (m *mockUploadStream) SendAndClose(r *gen.FileResponse) error { m.resp = r; m.isClosed = true; return nil }
func (m *mockUploadStream) Context() context.Context               { return m.ctx }
func (m *mockUploadStream) SetHeader(metadata.MD) error            { return nil }
func (m *mockUploadStream) SendHeader(metadata.MD) error           { return nil }
func (m *mockUploadStream) SetTrailer(metadata.MD)                 {}
func (m *mockUploadStream) SendMsg(m_ interface{}) error           { return nil }
func (m *mockUploadStream) RecvMsg(m_ interface{}) error           { return nil }

type mockUploadVersionStream struct {
	ctx      context.Context
	msgs     []*gen.UploadVersionRequest
	index    int
	resp     *gen.FileResponse
	isClosed bool
}

func (m *mockUploadVersionStream) Recv() (*gen.UploadVersionRequest, error) {
	if m.index >= len(m.msgs) {
		return nil, io.EOF
	}
	msg := m.msgs[m.index]
	m.index++
	return msg, nil
}
func (m *mockUploadVersionStream) SendAndClose(r *gen.FileResponse) error { m.resp = r; m.isClosed = true; return nil }
func (m *mockUploadVersionStream) Context() context.Context               { return m.ctx }
func (m *mockUploadVersionStream) SetHeader(metadata.MD) error            { return nil }
func (m *mockUploadVersionStream) SendHeader(metadata.MD) error           { return nil }
func (m *mockUploadVersionStream) SetTrailer(metadata.MD)                 {}
func (m *mockUploadVersionStream) SendMsg(m_ interface{}) error           { return nil }
func (m *mockUploadVersionStream) RecvMsg(m_ interface{}) error           { return nil }

type mockDownloadStream struct {
	ctx  context.Context
	msgs []*gen.DownloadFileResponse
}

func (m *mockDownloadStream) Send(r *gen.DownloadFileResponse) error { m.msgs = append(m.msgs, r); return nil }
func (m *mockDownloadStream) Context() context.Context              { return m.ctx }
func (m *mockDownloadStream) SetHeader(metadata.MD) error           { return nil }
func (m *mockDownloadStream) SendHeader(metadata.MD) error          { return nil }
func (m *mockDownloadStream) SetTrailer(metadata.MD)                {}
func (m *mockDownloadStream) SendMsg(m_ interface{}) error          { return nil }
func (m *mockDownloadStream) RecvMsg(m_ interface{}) error          { return nil }
