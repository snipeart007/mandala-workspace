/*
Package file_service provides the gRPC server implementation for file operations.
This file specifically handles file downloading from Content Addressable Storage (CAS).
*/
package file_service

import (
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"

	"mandala-workspace/gen"
	"mandala-workspace/internal/grpc/interceptors"
	"mandala-workspace/internal/permission"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *FileServiceServer) DownloadFile(req *gen.DownloadFileRequest, stream gen.FileService_DownloadFileServer) error {
	claims, err := interceptors.GetTokenClaims(stream.Context())
	if err != nil {
		slog.Warn("DownloadFile: unauthenticated request")
		return status.Error(codes.Unauthenticated, "unauthenticated")
	}

	slog.Info("DownloadFile: entry", "user_id", claims.UserID, "file_id", req.FileId, "version_id", req.VersionId)

	file, err := s.dbManager.GetFile(req.FileId)
	if err != nil {
		slog.Error("DownloadFile: file not found", "file_id", req.FileId, "error", err)
		return status.Errorf(codes.NotFound, "file not found")
	}

	bitmask, err := s.dbManager.GetUserEffectivePermissions(claims.UserID, file.FolderID)
	if err != nil {
		slog.Error("DownloadFile: permission check failed", "user_id", claims.UserID, "folder_id", file.FolderID, "error", err)
		return status.Errorf(codes.Internal, "permission check failed")
	}
	if !permission.PermissionBitMask(bitmask).Has(permission.PermRead) {
		slog.Warn("DownloadFile: permission denied", "user_id", claims.UserID, "folder_id", file.FolderID)
		return status.Error(codes.PermissionDenied, "missing PermRead")
	}

	uri := file.StoragePath
	if req.VersionId != 0 {
		// Fetch specific version URI
		versions, err := s.dbManager.ListVersions(file.FileID)
		if err != nil {
			slog.Error("DownloadFile: failed to list versions", "file_id", file.FileID, "error", err)
			return status.Errorf(codes.Internal, "failed to list versions")
		}
		found := false
		for _, v := range versions {
			if v.VersionID == req.VersionId {
				uri = fmt.Sprintf("%s:///%s", file.Location, hex.EncodeToString(v.Hash))
				found = true
				break
			}
		}
		if !found {
			slog.Warn("DownloadFile: version not found", "file_id", file.FileID, "version_id", req.VersionId)
			return status.Errorf(codes.NotFound, "version %d not found", req.VersionId)
		}
	}

	slog.Info("DownloadFile: retrieving from storage", "uri", uri)
	reader, err := s.storageRegistry.RetrieveByURI(stream.Context(), uri)
	if err != nil {
		slog.Error("DownloadFile: storage retrieval error", "uri", uri, "error", err)
		return status.Errorf(codes.Internal, "storage error: %v", err)
	}
	defer reader.Close()

	buf := make([]byte, 32*1024)
	totalBytes := 0
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			if err := stream.Send(&gen.DownloadFileResponse{Chunk: buf[:n]}); err != nil {
				slog.Error("DownloadFile: failed to send chunk", "error", err)
				return err
			}
			totalBytes += n
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			slog.Error("DownloadFile: error reading from storage", "error", err)
			return err
		}
	}

	slog.Info("DownloadFile: success", "file_id", req.FileId, "bytes_sent", totalBytes)
	return nil
}
