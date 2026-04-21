package file_service

import (
	"bytes"
	"io"
	"testing"

	"mandala-workspace/gen"
	"mandala-workspace/internal/permission"

	_ "github.com/mattn/go-sqlite3"
)

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
