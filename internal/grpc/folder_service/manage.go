package folder_service

import (
	"context"
	"fmt"
	"log/slog"

	v1 "mandala-workspace/gen"
	"mandala-workspace/internal/grpc/interceptors"
	"mandala-workspace/internal/permission"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *FolderService) MoveFolder(ctx context.Context, req *v1.MoveFolderRequest) (*v1.MoveFolderResponse, error) {
	claims, err := interceptors.GetTokenClaims(ctx)
	if err != nil {
		slog.Warn("MoveFolder attempt without token claims")
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}

	// 1. Check PermMoveFolder on source folder
	hasPerm, err := s.permissionManager.HasPermission(claims.UserID, req.FolderId, permission.PermMoveFolder)
	if err != nil {
		slog.Error("Failed to check PermMoveFolder", "error", err, "user_id", claims.UserID, "folder_id", req.FolderId)
		return nil, status.Errorf(codes.Internal, "failed to check permission: %v", err)
	}
	if !hasPerm {
		slog.Warn("PermMoveFolder denied", "user_id", claims.UserID, "folder_id", req.FolderId)
		return nil, status.Error(codes.PermissionDenied, "missing PermMoveFolder on folder")
	}

	// 2. Check PermCreateFolder on target parent
	hasPerm, err = s.permissionManager.HasPermission(claims.UserID, req.NewParentFolderId, permission.PermCreateFolder)
	if err != nil {
		slog.Error("Failed to check PermCreateFolder for move target", "error", err, "user_id", claims.UserID, "new_parent_id", req.NewParentFolderId)
		return nil, status.Errorf(codes.Internal, "failed to check permission: %v", err)
	}
	if !hasPerm {
		slog.Warn("PermCreateFolder denied for move target", "user_id", claims.UserID, "new_parent_id", req.NewParentFolderId)
		return nil, status.Error(codes.PermissionDenied, "missing PermCreateFolder on new parent")
	}

	// 3. Get target parent path
	parentFolder, err := s.dbManager.GetFolder(req.NewParentFolderId)
	if err != nil {
		slog.Warn("New parent folder not found for MoveFolder", "new_parent_id", req.NewParentFolderId, "error", err)
		return nil, status.Errorf(codes.NotFound, "new parent folder not found: %v", err)
	}

	// 4. Compute new path
	newPath := fmt.Sprintf("%s%d/", parentFolder.Path, parentFolder.FolderID)

	// 5. Update DB
	err = s.dbManager.MoveFolder(req.FolderId, req.NewParentFolderId, newPath)
	if err != nil {
		slog.Error("Failed to move folder in DB", "error", err, "folder_id", req.FolderId, "new_parent_id", req.NewParentFolderId)
		return nil, status.Errorf(codes.Internal, "failed to move folder: %v", err)
	}

	slog.Info("Folder moved", "folder_id", req.FolderId, "new_parent_id", req.NewParentFolderId, "user_id", claims.UserID)
	return &v1.MoveFolderResponse{Success: true}, nil
}

func (s *FolderService) DeleteFolder(ctx context.Context, req *v1.DeleteFolderRequest) (*v1.DeleteFolderResponse, error) {
	claims, err := interceptors.GetTokenClaims(ctx)
	if err != nil {
		slog.Warn("DeleteFolder attempt without token claims")
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}

	// 1. Check PermDeleteFolder
	hasPerm, err := s.permissionManager.HasPermission(claims.UserID, req.FolderId, permission.PermDeleteFolder)
	if err != nil {
		slog.Error("Failed to check PermDeleteFolder", "error", err, "user_id", claims.UserID, "folder_id", req.FolderId)
		return nil, status.Errorf(codes.Internal, "failed to check permission: %v", err)
	}
	if !hasPerm {
		slog.Warn("PermDeleteFolder denied", "user_id", claims.UserID, "folder_id", req.FolderId)
		return nil, status.Error(codes.PermissionDenied, "missing PermDeleteFolder")
	}

	// 2. Soft delete
	err = s.dbManager.SoftDeleteFolder(req.FolderId)
	if err != nil {
		slog.Error("Failed to soft delete folder in DB", "error", err, "folder_id", req.FolderId)
		return nil, status.Errorf(codes.Internal, "failed to delete folder: %v", err)
	}

	slog.Info("Folder soft-deleted", "folder_id", req.FolderId, "user_id", claims.UserID)
	return &v1.DeleteFolderResponse{Success: true}, nil
}
