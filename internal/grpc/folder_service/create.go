// Package folder_service implements the folder creation logic.
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

func (s *FolderService) CreateFolder(ctx context.Context, req *v1.CreateFolderRequest) (*v1.CreateFolderResponse, error) {
	slog.Info("CreateFolder RPC entry", "name", req.Name, "parent_folder_id", req.ParentFolderId)
	claims, err := interceptors.GetTokenClaims(ctx)
	if err != nil {
		slog.Warn("CreateFolder attempt with missing claims", "error", err)
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}

	// 1. Check permission on parent
	slog.Debug("Checking PermCreateFolder on parent", "user_id", claims.UserID, "parent_folder_id", req.ParentFolderId)
	hasPerm, err := s.permissionManager.HasPermission(claims.UserID, req.ParentFolderId, permission.PermCreateFolder)
	if err != nil {
		slog.Error("Failed to check PermCreateFolder", "error", err, "user_id", claims.UserID, "parent_folder_id", req.ParentFolderId)
		return nil, status.Errorf(codes.Internal, "failed to check permission: %v", err)
	}
	if !hasPerm {
		slog.Warn("PermCreateFolder denied", "user_id", claims.UserID, "parent_folder_id", req.ParentFolderId)
		return nil, status.Error(codes.PermissionDenied, "missing PermCreateFolder on parent")
	}

	// 2. Get parent path
	slog.Debug("Fetching parent folder details", "parent_folder_id", req.ParentFolderId)
	parentFolder, err := s.dbManager.GetFolder(req.ParentFolderId)
	if err != nil {
		slog.Error("Failed to fetch parent folder", "error", err, "parent_folder_id", req.ParentFolderId)
		return nil, status.Errorf(codes.NotFound, "parent folder not found: %v", err)
	}

	// 3. Compute new path
	newPath := fmt.Sprintf("%s%d/", parentFolder.Path, parentFolder.FolderID)

	// 4. Create folder
	slog.Debug("Creating folder in database", "name", req.Name, "path", newPath)
	id, createdAt, err := s.dbManager.CreateFolder(req.Name, req.ParentFolderId, newPath, req.Inheritance, req.VersionRetention, req.Metadata)
	if err != nil {
		slog.Error("Failed to create folder in DB", "error", err, "name", req.Name, "parent_folder_id", req.ParentFolderId)
		return nil, status.Errorf(codes.Internal, "failed to create folder: %v", err)
	}

	// 5. If inheritance is broken, copy creator's effective permissions from parent
	if !req.Inheritance {
		slog.Debug("Inheritance disabled, copying effective permissions from parent", "user_id", claims.UserID, "parent_folder_id", req.ParentFolderId)
		effPerm, err := s.dbManager.GetUserEffectivePermissions(claims.UserID, req.ParentFolderId)
		if err != nil {
			slog.Error("Failed to fetch effective permissions", "error", err, "user_id", claims.UserID, "parent_folder_id", req.ParentFolderId)
			return nil, status.Errorf(codes.Internal, "failed to fetch effective permissions: %v", err)
		}
		err = s.dbManager.SetUserPermission(claims.UserID, id, effPerm)
		if err != nil {
			slog.Error("Failed to set explicit permissions", "error", err, "user_id", claims.UserID, "folder_id", id)
			return nil, status.Errorf(codes.Internal, "failed to set explicit permissions: %v", err)
		}
	}

	slog.Info("Folder created successfully", "folder_id", id, "name", req.Name, "user_id", claims.UserID)
	return &v1.CreateFolderResponse{
		Folder: &v1.Folder{
			FolderId:         id,
			Name:             req.Name,
			ParentFolderId:   req.ParentFolderId,
			Path:             newPath,
			Inheritance:      req.Inheritance,
			VersionRetention: req.VersionRetention,
			Metadata:         req.Metadata,
			CreatedAt:        createdAt,
		},
	}, nil
}
