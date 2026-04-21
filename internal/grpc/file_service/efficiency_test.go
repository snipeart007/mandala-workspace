package file_service

import (
	"runtime"
	"testing"

	"mandala-workspace/gen"
	"mandala-workspace/internal/permission"
)

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
