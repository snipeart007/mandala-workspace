/*
Package file_service provides the gRPC server implementation for file operations.
This file specifically handles file and version uploads to Content Addressable Storage (CAS).
*/
package file_service

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"mandala-workspace/gen"
	"mandala-workspace/internal/grpc/interceptors"
	"mandala-workspace/internal/permission"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// uploadStreamToCAS handles the heavy lifting of piping gRPC bytes to storage.
func (s *FileServiceServer) uploadStreamToCAS(ctx context.Context, recv func() ([]byte, error)) (string, error) {
	slog.Info("Starting file stream upload to CAS")
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
			slog.Error("Failed to receive chunk from stream", "error", err)
			pw.CloseWithError(err)
			return "", err
		}
		if _, err := pw.Write(chunk); err != nil {
			slog.Error("Failed to write chunk to pipe", "error", err)
			return "", err
		}
	}

	res := <-resultChan
	if res.err != nil {
		slog.Error("CAS storage error during upload", "error", res.err)
	} else {
		slog.Info("File stream upload to CAS completed", "uri", res.uri)
	}
	return res.uri, res.err
}

func (s *FileServiceServer) UploadFile(stream gen.FileService_UploadFileServer) error {
	claims, err := interceptors.GetTokenClaims(stream.Context())
	if err != nil {
		slog.Warn("UploadFile: unauthenticated request")
		return status.Error(codes.Unauthenticated, "unauthenticated")
	}

	// 1. First message must be metadata
	req, err := stream.Recv()
	if err != nil {
		slog.Error("UploadFile: failed to receive metadata", "error", err)
		return status.Errorf(codes.InvalidArgument, "failed to receive metadata: %v", err)
	}
	meta := req.GetMetadata()
	if meta == nil {
		slog.Warn("UploadFile: first message missing metadata")
		return status.Error(codes.InvalidArgument, "first message must be metadata")
	}

	slog.Info("UploadFile: entry", "user_id", claims.UserID, "folder_id", meta.FolderId, "name", meta.Name)

	// 2. Permission check
	bitmask, err := s.dbManager.GetUserEffectivePermissions(claims.UserID, meta.FolderId)
	if err != nil {
		slog.Error("UploadFile: permission check failed", "user_id", claims.UserID, "folder_id", meta.FolderId, "error", err)
		return status.Errorf(codes.Internal, "permission check failed: %v", err)
	}
	if !permission.PermissionBitMask(bitmask).Has(permission.PermCreate) {
		slog.Warn("UploadFile: permission denied", "user_id", claims.UserID, "folder_id", meta.FolderId)
		return status.Error(codes.PermissionDenied, "missing PermCreate on folder")
	}

	// 3. Prevent duplicate names in same folder
	if _, err := s.dbManager.GetFileByName(meta.FolderId, meta.Name); err == nil {
		slog.Warn("UploadFile: file already exists", "folder_id", meta.FolderId, "name", meta.Name)
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
		slog.Error("UploadFile: storage error", "error", err)
		return status.Errorf(codes.Internal, "storage error: %v", err)
	}

	// 5. DB Updates (File and First Version)
	// Get folder path for the file record
	folder, err := s.dbManager.GetFolder(meta.FolderId)
	if err != nil {
		slog.Error("UploadFile: failed to get folder", "folder_id", meta.FolderId, "error", err)
		return status.Errorf(codes.Internal, "failed to get folder path: %v", err)
	}
	filePath := fmt.Sprintf("%s%d/", folder.Path, folder.FolderID)

	fileID, _, err := s.dbManager.CreateFile(meta.Name, meta.FolderId, filePath, uri, s.defaultScheme, meta.Metadata)
	if err != nil {
		slog.Error("UploadFile: failed to create file record", "error", err)
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
		slog.Error("UploadFile: failed to create version record", "error", err)
		return status.Errorf(codes.Internal, "failed to create version record: %v", err)
	}

	if err := s.dbManager.UpdateFileVersion(fileID, versionID, uri, s.defaultScheme); err != nil {
		slog.Error("UploadFile: failed to link file to version", "error", err)
		return status.Errorf(codes.Internal, "failed to link file to version: %v", err)
	}

	slog.Info("UploadFile: success", "file_id", fileID, "version_id", versionID)

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
		slog.Warn("UploadVersion: unauthenticated request")
		return status.Error(codes.Unauthenticated, "unauthenticated")
	}

	// 1. First message must be metadata
	req, err := stream.Recv()
	if err != nil {
		slog.Error("UploadVersion: failed to receive metadata", "error", err)
		return status.Errorf(codes.InvalidArgument, "failed to receive metadata: %v", err)
	}
	meta := req.GetMetadata()
	if meta == nil {
		slog.Warn("UploadVersion: first message missing metadata")
		return status.Error(codes.InvalidArgument, "first message must be metadata")
	}

	slog.Info("UploadVersion: entry", "user_id", claims.UserID, "file_id", meta.FileId)

	// 2. Fetch file and check permissions
	file, err := s.dbManager.GetFile(meta.FileId)
	if err != nil {
		slog.Error("UploadVersion: file not found", "file_id", meta.FileId, "error", err)
		return status.Errorf(codes.NotFound, "file %d not found", meta.FileId)
	}

	bitmask, err := s.dbManager.GetUserEffectivePermissions(claims.UserID, file.FolderID)
	if err != nil {
		slog.Error("UploadVersion: permission check failed", "user_id", claims.UserID, "folder_id", file.FolderID, "error", err)
		return status.Errorf(codes.Internal, "permission check failed: %v", err)
	}
	if !permission.PermissionBitMask(bitmask).Has(permission.PermWrite) {
		slog.Warn("UploadVersion: permission denied", "user_id", claims.UserID, "folder_id", file.FolderID)
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
		slog.Error("UploadVersion: storage error", "error", err)
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
		slog.Error("UploadVersion: failed to create version record", "error", err)
		return status.Errorf(codes.Internal, "failed to create version record: %v", err)
	}

	if err := s.dbManager.UpdateFileVersion(file.FileID, versionID, uri, s.defaultScheme); err != nil {
		slog.Error("UploadVersion: failed to link file to version", "error", err)
		return status.Errorf(codes.Internal, "failed to link file to version: %v", err)
	}

	// 5. Retention Policy Enforcement
	retention, _ := s.dbManager.GetVersionRetention(file.FolderID)
	if retention > 0 {
		slog.Info("Enforcing retention policy", "folder_id", file.FolderID, "keep_last", retention)
		s.dbManager.DeleteOldVersions(file.FileID, retention)
	}

	slog.Info("UploadVersion: success", "file_id", file.FileID, "version_id", versionID)

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
