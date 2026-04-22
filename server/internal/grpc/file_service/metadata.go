/*
Package file_service provides the gRPC server implementation for file operations.
This file specifically handles file metadata operations, version listing, retention policies, and file deletion.
*/
package file_service

import (
	"context"
	"fmt"
	"log/slog"

	"mandala-workspace/gen"
	"mandala-workspace/internal/grpc/interceptors"
	"mandala-workspace/internal/permission"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *FileServiceServer) ModifyFile(ctx context.Context, req *gen.ModifyFileRequest) (*gen.FileResponse, error) {
	claims, err := interceptors.GetTokenClaims(ctx)
	if err != nil {
		slog.Warn("ModifyFile: unauthenticated request")
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}

	slog.Info("ModifyFile: entry", "user_id", claims.UserID, "file_id", req.FileId, "new_name", req.Name, "new_folder_id", req.FolderId)

	file, err := s.dbManager.GetFile(req.FileId)
	if err != nil {
		slog.Error("ModifyFile: file not found", "file_id", req.FileId, "error", err)
		return nil, status.Errorf(codes.NotFound, "file not found")
	}

	bitmask, err := s.dbManager.GetUserEffectivePermissions(claims.UserID, file.FolderID)
	if err != nil {
		slog.Error("ModifyFile: permission check failed", "user_id", claims.UserID, "folder_id", file.FolderID, "error", err)
		return nil, status.Errorf(codes.Internal, "permission check failed")
	}
	perms := permission.PermissionBitMask(bitmask)

	// Renaming/Moving requires specific permissions
	if req.Name != nil || req.FolderId != nil {
		if !perms.Has(permission.PermRename) {
			slog.Warn("ModifyFile: permission denied (PermRename)", "user_id", claims.UserID, "folder_id", file.FolderID)
			return nil, status.Error(codes.PermissionDenied, "missing PermRename")
		}
	} else if len(req.Metadata) > 0 {
		if !perms.Has(permission.PermWrite) {
			slog.Warn("ModifyFile: permission denied (PermWrite)", "user_id", claims.UserID, "folder_id", file.FolderID)
			return nil, status.Error(codes.PermissionDenied, "missing PermWrite for metadata update")
		}
	}

	// Apply updates
	if req.Name != nil {
		slog.Info("ModifyFile: renaming file", "file_id", req.FileId, "new_name", *req.Name)
		if err := s.dbManager.RenameFile(req.FileId, *req.Name); err != nil {
			slog.Error("ModifyFile: failed to rename file", "file_id", req.FileId, "error", err)
			return nil, status.Errorf(codes.Internal, "failed to rename file: %v", err)
		}
	}

	if req.FolderId != nil {
		slog.Info("ModifyFile: moving file", "file_id", req.FileId, "target_folder_id", *req.FolderId)
		targetFolder, err := s.dbManager.GetFolder(*req.FolderId)
		if err != nil {
			slog.Error("ModifyFile: target folder not found", "folder_id", *req.FolderId, "error", err)
			return nil, status.Errorf(codes.NotFound, "target folder not found")
		}
		// Compute new path
		newPath := fmt.Sprintf("%s%d/", targetFolder.Path, targetFolder.FolderID)
		if err := s.dbManager.MoveFile(req.FileId, *req.FolderId, newPath); err != nil {
			slog.Error("ModifyFile: failed to move file", "file_id", req.FileId, "target_folder_id", *req.FolderId, "error", err)
			return nil, status.Errorf(codes.Internal, "failed to move file: %v", err)
		}
	}

	if len(req.Metadata) > 0 {
		slog.Info("ModifyFile: updating metadata", "file_id", req.FileId)
		if err := s.dbManager.UpdateFileMetadata(req.FileId, req.Metadata); err != nil {
			slog.Error("ModifyFile: failed to update metadata", "file_id", req.FileId, "error", err)
			return nil, status.Errorf(codes.Internal, "failed to update metadata: %v", err)
		}
	}

	// Fetch updated file
	updatedFile, err := s.dbManager.GetFile(req.FileId)
	if err != nil {
		slog.Error("ModifyFile: failed to fetch updated file", "file_id", req.FileId, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to fetch updated file")
	}

	slog.Info("ModifyFile: success", "file_id", req.FileId)

	return &gen.FileResponse{
		File: &gen.File{
			FileId:      updatedFile.FileID,
			Name:        updatedFile.Name,
			FolderId:    updatedFile.FolderID,
			Path:        updatedFile.Path,
			StoragePath: updatedFile.StoragePath,
			Location:    updatedFile.Location,
			VersionId:   updatedFile.VersionID,
			CreatedAt:   updatedFile.CreatedAt,
			UpdatedAt:   updatedFile.UpdatedAt,
		},
		VersionId: updatedFile.VersionID,
	}, nil
}

func (s *FileServiceServer) ListVersions(ctx context.Context, req *gen.ListVersionsRequest) (*gen.ListVersionsResponse, error) {
	claims, err := interceptors.GetTokenClaims(ctx)
	if err != nil {
		slog.Warn("ListVersions: unauthenticated request")
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}

	slog.Info("ListVersions: entry", "user_id", claims.UserID, "file_id", req.FileId)

	file, err := s.dbManager.GetFile(req.FileId)
	if err != nil {
		slog.Error("ListVersions: file not found", "file_id", req.FileId, "error", err)
		return nil, status.Errorf(codes.NotFound, "file not found")
	}

	bitmask, err := s.dbManager.GetUserEffectivePermissions(claims.UserID, file.FolderID)
	if err != nil {
		slog.Error("ListVersions: permission check failed", "user_id", claims.UserID, "folder_id", file.FolderID, "error", err)
		return nil, status.Errorf(codes.Internal, "permission check failed")
	}
	if !permission.PermissionBitMask(bitmask).Has(permission.PermViewHistory) {
		slog.Warn("ListVersions: permission denied (PermViewHistory)", "user_id", claims.UserID, "folder_id", file.FolderID)
		return nil, status.Error(codes.PermissionDenied, "missing PermViewHistory")
	}

	dbVersions, err := s.dbManager.ListVersions(req.FileId)
	if err != nil {
		slog.Error("ListVersions: failed to list versions", "file_id", req.FileId, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to list versions")
	}

	respVersions := make([]*gen.FileVersion, len(dbVersions))
	for i, v := range dbVersions {
		respVersions[i] = &gen.FileVersion{
			VersionId:   v.VersionID,
			FileId:      v.FileID,
			VersionName: v.Version,
			Hash:        v.Hash,
			UserId:      v.UserID,
			Metadata:    v.Metadata,
			Comment:     v.VersionComment,
			CreatedAt:   v.CreatedAt,
		}
	}
	slog.Info("ListVersions: success", "file_id", req.FileId, "count", len(respVersions))
	return &gen.ListVersionsResponse{Versions: respVersions}, nil
}

func (s *FileServiceServer) SetRetentionPolicy(ctx context.Context, req *gen.SetRetentionPolicyRequest) (*gen.SetRetentionPolicyResponse, error) {
	claims, err := interceptors.GetTokenClaims(ctx)
	if err != nil {
		slog.Warn("SetRetentionPolicy: unauthenticated request")
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}

	slog.Info("SetRetentionPolicy: entry", "user_id", claims.UserID, "folder_id", req.FolderId, "last_n_versions", req.LastNVersions)

	bitmask, err := s.dbManager.GetUserEffectivePermissions(claims.UserID, req.FolderId)
	if err != nil {
		slog.Error("SetRetentionPolicy: permission check failed", "user_id", claims.UserID, "folder_id", req.FolderId, "error", err)
		return nil, status.Errorf(codes.Internal, "permission check failed")
	}
	if !permission.PermissionBitMask(bitmask).Has(permission.PermAdmin) {
		slog.Warn("SetRetentionPolicy: permission denied (PermAdmin)", "user_id", claims.UserID, "folder_id", req.FolderId)
		return nil, status.Error(codes.PermissionDenied, "missing PermAdmin")
	}

	if err := s.dbManager.SetRetentionPolicy(req.FolderId, req.LastNVersions); err != nil {
		slog.Error("SetRetentionPolicy: failed to update policy", "folder_id", req.FolderId, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to update policy")
	}

	slog.Info("SetRetentionPolicy: success", "folder_id", req.FolderId, "last_n_versions", req.LastNVersions)
	return &gen.SetRetentionPolicyResponse{Success: true}, nil
}

func (s *FileServiceServer) DeleteFile(ctx context.Context, req *gen.DeleteFileRequest) (*gen.DeleteFileResponse, error) {
	claims, err := interceptors.GetTokenClaims(ctx)
	if err != nil {
		slog.Warn("DeleteFile: unauthenticated request")
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}

	slog.Info("DeleteFile: entry", "user_id", claims.UserID, "file_id", req.FileId)

	file, err := s.dbManager.GetFile(req.FileId)
	if err != nil {
		slog.Error("DeleteFile: file not found", "file_id", req.FileId, "error", err)
		return nil, status.Errorf(codes.NotFound, "file not found")
	}

	bitmask, err := s.dbManager.GetUserEffectivePermissions(claims.UserID, file.FolderID)
	if err != nil {
		slog.Error("DeleteFile: permission check failed", "user_id", claims.UserID, "folder_id", file.FolderID, "error", err)
		return nil, status.Errorf(codes.Internal, "permission check failed")
	}
	if !permission.PermissionBitMask(bitmask).Has(permission.PermDelete) {
		slog.Warn("DeleteFile: permission denied (PermDelete)", "user_id", claims.UserID, "folder_id", file.FolderID)
		return nil, status.Error(codes.PermissionDenied, "missing PermDelete")
	}

	if err := s.dbManager.SoftDeleteFile(req.FileId); err != nil {
		slog.Error("DeleteFile: failed to soft-delete file", "file_id", req.FileId, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to soft-delete file: %v", err)
	}

	slog.Info("DeleteFile: success", "file_id", req.FileId)
	return &gen.DeleteFileResponse{Success: true}, nil
}
