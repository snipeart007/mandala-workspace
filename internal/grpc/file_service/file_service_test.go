package file_service

import (
	"bytes"
	"context"
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"mandala-workspace/gen"
	"mandala-workspace/internal/crypto/paseto"
	"mandala-workspace/internal/db"
	"mandala-workspace/internal/grpc/interceptors"
	"mandala-workspace/internal/permission"
	"mandala-workspace/internal/storage"

	"google.golang.org/grpc/metadata"
	_ "github.com/mattn/go-sqlite3"
)

func setupTestServer(t *testing.T) (*FileServiceServer, *db.DBManager, uint64, string) {
	tmpDir := t.TempDir()
	sqlPath := filepath.Join(tmpDir, "init.sql")
	schema, _ := os.ReadFile("../../db/sql/InitializeDB.sql")
	os.WriteFile(sqlPath, schema, 0644)

	sqlDB, _ := sql.Open("sqlite3", ":memory:")
	mgr := db.NewDBManagerWithDB(sqlDB, &db.DBManagerConfig{InitialSchemePath: sqlPath})
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

func TestFileService_Integration(t *testing.T) {
	server, mgr, userID, _ := setupTestServer(t)
	defer mgr.Close()
	ctx := contextWithUser(userID)

	// 1. Setup Permissions
	mgr.SetUserPermission(userID, 1, uint64(permission.PermAdmin|permission.PermCreate|permission.PermWrite|permission.PermRead|permission.PermDelete|permission.PermViewHistory|permission.PermRename))

	// 2. Test UploadFile
	content := []byte("hello world")
	stream := &mockUploadStream{
		ctx:         ctx,
		metadataMsg: &gen.UploadFileRequest{Data: &gen.UploadFileRequest_Metadata{Metadata: &gen.UploadMetadata{Name: "test.txt", FolderId: 1}}},
		chunkSize:   len(content),
		numChunks:   1,
	}
	// Use a wrapper to return the specific content for this test
	sentMetadata := false
	sentChunk := false
	stream.recvFunc = func() (*gen.UploadFileRequest, error) {
		if !sentMetadata {
			sentMetadata = true
			return stream.metadataMsg, nil
		}
		if !sentChunk {
			sentChunk = true
			return &gen.UploadFileRequest{Data: &gen.UploadFileRequest_Chunk{Chunk: content}}, nil
		}
		return nil, io.EOF
	}

	err := server.UploadFile(stream)
	if err != nil {
		t.Fatalf("UploadFile failed: %v", err)
	}

	if stream.resp.File.Name != "test.txt" {
		t.Errorf("expected test.txt, got %s", stream.resp.File.Name)
	}

	fileID := stream.resp.File.FileId

	// Verify DB state
	file, _ := mgr.GetFile(fileID)
	if file.Name != "test.txt" || file.VersionID == 0 {
		t.Errorf("DB state invalid after upload: %+v", file)
	}

	// 3. Test UploadVersion
	newContent := []byte("new version content")
	vStream := &mockUploadVersionStream{
		ctx: ctx,
		msgs: []*gen.UploadVersionRequest{
			{Data: &gen.UploadVersionRequest_Metadata{Metadata: &gen.VersionMetadata{FileId: fileID, Comment: "Updated"}}},
			{Data: &gen.UploadVersionRequest_Chunk{Chunk: newContent}},
		},
	}

	err = server.UploadVersion(vStream)
	if err != nil {
		t.Fatalf("UploadVersion failed: %v", err)
	}

	// 4. Test DownloadFile (Latest)
	dStream := &mockDownloadStream{ctx: ctx}
	err = server.DownloadFile(&gen.DownloadFileRequest{FileId: fileID}, dStream)
	if err != nil {
		t.Fatalf("DownloadFile failed: %v", err)
	}

	var downloaded []byte
	for _, m := range dStream.msgs {
		downloaded = append(downloaded, m.Chunk...)
	}

	if !bytes.Equal(downloaded, newContent) {
		t.Errorf("download mismatch. expected %s, got %s", string(newContent), string(downloaded))
	}

	// 5. Test Retention Policy
	mgr.SetRetentionPolicy(1, 1) // Keep only 1 version
	// Current versions: v1, v2. Uploading v3 should delete v1 and v2.
	vStream2 := &mockUploadVersionStream{
		ctx: ctx,
		msgs: []*gen.UploadVersionRequest{
			{Data: &gen.UploadVersionRequest_Metadata{Metadata: &gen.VersionMetadata{FileId: fileID, Comment: "V3"}}},
			{Data: &gen.UploadVersionRequest_Chunk{Chunk: []byte("v3")}},
		},
	}
	err = server.UploadVersion(vStream2)
	if err != nil {
		t.Fatalf("V3 upload failed: %v", err)
	}

	versions, _ := mgr.ListVersions(fileID)
	if len(versions) != 1 {
		t.Errorf("retention failed. expected 1 version, got %d", len(versions))
	}

	// 6. Test ModifyFile (Rename)
	newName := "renamed.txt"
	mResp, err := server.ModifyFile(ctx, &gen.ModifyFileRequest{FileId: fileID, Name: &newName})
	if err != nil {
		t.Fatalf("ModifyFile failed: %v", err)
	}
	if mResp.File.Name != newName {
		t.Errorf("rename failed in response")
	}

	file, _ = mgr.GetFile(fileID)
	if file.Name != newName {
		t.Errorf("rename failed in DB")
	}

	// 7. Test DeleteFile
	_, err = server.DeleteFile(ctx, &gen.DeleteFileRequest{FileId: fileID})
	if err != nil {
		t.Fatalf("DeleteFile failed: %v", err)
	}

	_, err = mgr.GetFile(fileID)
	if err == nil {
		t.Error("expected file to be soft-deleted in DB")
	}
}

func TestUploadVersion_CorrectNumberAfterPruning(t *testing.T) {
	server, mgr, userID, _ := setupTestServer(t)
	defer mgr.Close()
	ctx := contextWithUser(userID)
	mgr.SetUserPermission(userID, 1, uint64(permission.PermAdmin|permission.PermCreate|permission.PermWrite|permission.PermViewHistory))

	// 1. Initial Upload (v1)
	stream := &mockUploadStream{
		ctx:         ctx,
		metadataMsg: &gen.UploadFileRequest{Data: &gen.UploadFileRequest_Metadata{Metadata: &gen.UploadMetadata{Name: "test.txt", FolderId: 1}}},
		chunkSize:   10,
		numChunks:   1,
	}
	sentMetadata := false
	sentChunk := false
	stream.recvFunc = func() (*gen.UploadFileRequest, error) {
		if !sentMetadata {
			sentMetadata = true
			return stream.metadataMsg, nil
		}
		if !sentChunk {
			sentChunk = true
			return &gen.UploadFileRequest{Data: &gen.UploadFileRequest_Chunk{Chunk: []byte("v1 content")}}, nil
		}
		return nil, io.EOF
	}
	server.UploadFile(stream)
	fileID := stream.resp.File.FileId

	// 2. Upload v2
	vStream2 := &mockUploadVersionStream{
		ctx: ctx,
		msgs: []*gen.UploadVersionRequest{
			{Data: &gen.UploadVersionRequest_Metadata{Metadata: &gen.VersionMetadata{FileId: fileID, Comment: "V2"}}},
			{Data: &gen.UploadVersionRequest_Chunk{Chunk: []byte("v2")}},
		},
	}
	server.UploadVersion(vStream2)

	// 3. Set Retention to 1 and Upload v3 (v1, v2 pruned)
	mgr.SetRetentionPolicy(1, 1)
	vStream3 := &mockUploadVersionStream{
		ctx: ctx,
		msgs: []*gen.UploadVersionRequest{
			{Data: &gen.UploadVersionRequest_Metadata{Metadata: &gen.VersionMetadata{FileId: fileID, Comment: "V3"}}},
			{Data: &gen.UploadVersionRequest_Chunk{Chunk: []byte("v3")}},
		},
	}
	server.UploadVersion(vStream3)

	versions, _ := mgr.ListVersions(fileID)
	if len(versions) != 1 {
		t.Fatalf("Expected 1 version, got %d", len(versions))
	}
	if versions[0].Version != "v3" {
		t.Fatalf("Expected latest version to be v3, got %s", versions[0].Version)
	}

	// 4. Upload next version - should be v4
	vStream4 := &mockUploadVersionStream{
		ctx: ctx,
		msgs: []*gen.UploadVersionRequest{
			{Data: &gen.UploadVersionRequest_Metadata{Metadata: &gen.VersionMetadata{FileId: fileID, Comment: "V4"}}},
			{Data: &gen.UploadVersionRequest_Chunk{Chunk: []byte("v4")}},
		},
	}
	err := server.UploadVersion(vStream4)
	if err != nil {
		t.Fatalf("V4 upload failed: %v", err)
	}

	versions, _ = mgr.ListVersions(fileID)
	// Even with retention=1, after upload we have 1 version (the newest one)
	if versions[0].Version != "v4" {
		t.Errorf("Expected version v4, got %s. The logic likely reused v2 because len(versions) was 1.", versions[0].Version)
	}
}

func TestFileService_StreamingEfficiency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping streaming efficiency test in short mode")
	}

	server, mgr, userID, _ := setupTestServer(t)
	defer mgr.Close()
	ctx := contextWithUser(userID)
	mgr.SetUserPermission(userID, 1, uint64(permission.PermCreate|permission.PermRead))

	// Simulate a ~100MB file (enough to verify constant memory)
	chunkSize := 8 * 1024 * 1024 // 8MB
	numChunks := 13            // ~104MB

	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	stream := &mockUploadStream{
		ctx:         ctx,
		metadataMsg: &gen.UploadFileRequest{Data: &gen.UploadFileRequest_Metadata{Metadata: &gen.UploadMetadata{Name: "large.bin", FolderId: 1}}},
		chunkSize:   chunkSize,
		numChunks:   numChunks,
	}

	err := server.UploadFile(stream)
	if err != nil {
		t.Fatalf("Large upload failed: %v", err)
	}

	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	// Memory increase should not be proportional to 100MB
	increaseMB := (m2.Alloc - m1.Alloc) / 1024 / 1024
	if increaseMB > 10 { // Now we expect even lower overhead
		t.Errorf("Memory usage increased significantly: %d MB", increaseMB)
	}
}
