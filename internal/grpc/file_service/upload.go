package file_service

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"mandala-workspace/gen"
	"mandala-workspace/internal/grpc/interceptors"
	"mandala-workspace/internal/permission"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// uploadStreamToCAS handles the heavy lifting of piping gRPC bytes to storage.
func (s *FileServiceServer) uploadStreamToCAS(ctx context.Context, recv func() ([]byte, error)) (string, error) {
	pr, pw := io.Pipe()

	type storeResult struct {
		uri string
		err error
	}
	resultChan := make(chan storeResult, 1)

	go func() {
		uri, err := s.storageRegistry.StoreInDefault(ctx, s.defaultScheme, pr)
		resultChan <- storeResult{uri, err}
	}()

	for {
		chunk, err := recv()
		if err == io.EOF {
			pw.Close()
			break
		}
		if err != nil {
			pw.CloseWithError(err)
			return "", err
		}
		if _, err := pw.Write(chunk); err != nil {
			return "", err
		}
	}

	res := <-resultChan
	return res.uri, res.err
}

func (s *FileServiceServer) UploadFile(stream gen.FileService_UploadFileServer) error {
	claims, err := interceptors.GetTokenClaims(stream.Context())
	if err != nil {
		return status.Error(codes.Unauthenticated, "unauthenticated")
	}

	// 1. First message must be metadata
	req, err := stream.Recv()
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "failed to receive metadata: %v", err)
	}
	meta := req.GetMetadata()
	if meta == nil {
		return status.Error(codes.InvalidArgument, "first message must be metadata")
	}

	// 2. Permission check
	bitmask, err := s.dbManager.GetUserEffectivePermissions(claims.UserID, meta.FolderId)
	if err != nil {
		return status.Errorf(codes.Internal, "permission check failed: %v", err)
	}
	if !permission.PermissionBitMask(bitmask).Has(permission.PermCreate) {
		return status.Error(codes.PermissionDenied, "missing PermCreate on folder")
	}

	// 3. Prevent duplicate names in same folder
	if _, err := s.dbManager.GetFileByName(meta.FolderId, meta.Name); err == nil {
		return status.Errorf(codes.AlreadyExists, "file '%s' already exists in folder %d", meta.Name, meta.FolderId)
	}

	// 4. Stream to CAS
	uri, err := s.uploadStreamToCAS(stream.Context(), func() ([]byte, error) {
		r, err := stream.Recv()
		if err != nil {
			return nil, err
		}
		return r.GetChunk(), nil
	})
	if err != nil {
		return status.Errorf(codes.Internal, "storage error: %v", err)
	}

	// 5. DB Updates (File and First Version)
	// Get folder path for the file record
	folder, err := s.dbManager.GetFolder(meta.FolderId)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to get folder path: %v", err)
	}
	filePath := fmt.Sprintf("%s%d/", folder.Path, folder.FolderID)

	fileID, _, err := s.dbManager.CreateFile(meta.Name, meta.FolderId, filePath, uri, s.defaultScheme, meta.Metadata)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to create file record: %v", err)
	}

	// Extract hash from URI (e.g. "local:///hash")
	parts := strings.Split(uri, "/")
	hashStr := parts[len(parts)-1]
	hash, _ := hex.DecodeString(hashStr)

	comment := meta.VersionComment
	if comment == "" {
		comment = "Initial upload"
	}

	versionID, err := s.dbManager.CreateVersion(fileID, "v1", hash, claims.UserID, meta.Metadata, comment)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to create version record: %v", err)
	}

	if err := s.dbManager.UpdateFileVersion(fileID, versionID, uri, s.defaultScheme); err != nil {
		return status.Errorf(codes.Internal, "failed to link file to version: %v", err)
	}

	return stream.SendAndClose(&gen.FileResponse{
		File: &gen.File{
			FileId:      fileID,
			Name:        meta.Name,
			FolderId:    meta.FolderId,
			Path:        filePath,
			StoragePath: uri,
			Location:    s.defaultScheme,
			VersionId:   versionID,
		},
		VersionId: versionID,
	})
}

func (s *FileServiceServer) UploadVersion(stream gen.FileService_UploadVersionServer) error {
	claims, err := interceptors.GetTokenClaims(stream.Context())
	if err != nil {
		return status.Error(codes.Unauthenticated, "unauthenticated")
	}

	// 1. First message must be metadata
	req, err := stream.Recv()
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "failed to receive metadata: %v", err)
	}
	meta := req.GetMetadata()
	if meta == nil {
		return status.Error(codes.InvalidArgument, "first message must be metadata")
	}

	// 2. Fetch file and check permissions
	file, err := s.dbManager.GetFile(meta.FileId)
	if err != nil {
		return status.Errorf(codes.NotFound, "file %d not found", meta.FileId)
	}

	bitmask, err := s.dbManager.GetUserEffectivePermissions(claims.UserID, file.FolderID)
	if err != nil {
		return status.Errorf(codes.Internal, "permission check failed: %v", err)
	}
	if !permission.PermissionBitMask(bitmask).Has(permission.PermWrite) {
		return status.Error(codes.PermissionDenied, "missing PermWrite on folder")
	}

	// 3. Stream to CAS
	uri, err := s.uploadStreamToCAS(stream.Context(), func() ([]byte, error) {
		r, err := stream.Recv()
		if err != nil {
			return nil, err
		}
		return r.GetChunk(), nil
	})
	if err != nil {
		return status.Errorf(codes.Internal, "storage error: %v", err)
	}

	// 4. DB Updates
	parts := strings.Split(uri, "/")
	hashStr := parts[len(parts)-1]
	hash, _ := hex.DecodeString(hashStr)

	// Determine version name (vN)
	nextVer := 1
	versions, _ := s.dbManager.ListVersions(file.FileID)
	if len(versions) > 0 {
		maxVer := 0
		for _, v := range versions {
			var verNum int
			if _, err := fmt.Sscanf(v.Version, "v%d", &verNum); err == nil {
				if verNum > maxVer {
					maxVer = verNum
				}
			}
		}
		nextVer = maxVer + 1
	}
	versionName := fmt.Sprintf("v%d", nextVer)

	versionID, err := s.dbManager.CreateVersion(file.FileID, versionName, hash, claims.UserID, meta.Metadata, meta.Comment)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to create version record: %v", err)
	}

	if err := s.dbManager.UpdateFileVersion(file.FileID, versionID, uri, s.defaultScheme); err != nil {
		return status.Errorf(codes.Internal, "failed to link file to version: %v", err)
	}

	// 5. Retention Policy Enforcement
	retention, _ := s.dbManager.GetVersionRetention(file.FolderID)
	if retention > 0 {
		s.dbManager.DeleteOldVersions(file.FileID, retention)
	}

	return stream.SendAndClose(&gen.FileResponse{
		File: &gen.File{
			FileId:      file.FileID,
			Name:        file.Name,
			FolderId:    file.FolderID,
			Path:        file.Path,
			StoragePath: uri,
			Location:    s.defaultScheme,
			VersionId:   versionID,
		},
		VersionId: versionID,
	})
}
