package folder_service

import (
	"context"
	"fmt"
	"log/slog"
	v1 "mandala-workspace/gen"
	"mandala-workspace/internal/db"
	"mandala-workspace/internal/grpc/interceptors"
	"mandala-workspace/internal/permission"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type FolderService struct {
	v1.UnimplementedFolderServiceServer
	dbManager         *db.DBManager
	permissionManager *permission.PermissionManager
}

func NewFolderService(dbManager *db.DBManager, permissionManager *permission.PermissionManager) *FolderService {
	return &FolderService{
		dbManager:         dbManager,
		permissionManager: permissionManager,
	}
}

func (s *FolderService) CreateFolder(ctx context.Context, req *v1.CreateFolderRequest) (*v1.CreateFolderResponse, error) {
	claims, err := interceptors.GetTokenClaims(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}

	// 1. Check permission on parent
	hasPerm, err := s.permissionManager.HasPermission(claims.UserID, req.ParentFolderId, permission.PermCreateFolder)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to check permission: %v", err)
	}
	if !hasPerm {
		return nil, status.Error(codes.PermissionDenied, "missing PermCreateFolder on parent")
	}

	// 2. Get parent path
	parentFolder, err := s.dbManager.GetFolder(req.ParentFolderId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "parent folder not found: %v", err)
	}

	// 3. Compute new path
	newPath := fmt.Sprintf("%s%d/", parentFolder.Path, parentFolder.FolderID)
// 4. Create folder
id, createdAt, err := s.dbManager.CreateFolder(req.Name, req.ParentFolderId, newPath, req.Inheritance, req.VersionRetention, req.Metadata)
if err != nil {
	return nil, status.Errorf(codes.Internal, "failed to create folder: %v", err)
}

// 5. If inheritance is broken, copy creator's effective permissions from parent
if !req.Inheritance {
	effPerm, err := s.dbManager.GetUserEffectivePermissions(claims.UserID, req.ParentFolderId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch effective permissions: %v", err)
	}
	err = s.dbManager.SetUserPermission(claims.UserID, id, effPerm)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to set explicit permissions: %v", err)
	}
}

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

func (s *FolderService) ListFolder(ctx context.Context, req *v1.ListFolderRequest) (*v1.ListFolderResponse, error) {
	claims, err := interceptors.GetTokenClaims(ctx)
	if err != nil {
		slog.Warn("ListFolder attempt without token claims")
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}

	// 1. Check PermRead on the folder
	hasPerm, err := s.permissionManager.HasPermission(claims.UserID, req.FolderId, permission.PermRead)
	if err != nil {
		slog.Error("Failed to check PermRead", "error", err, "user_id", claims.UserID, "folder_id", req.FolderId)
		return nil, status.Errorf(codes.Internal, "failed to check permission: %v", err)
	}
	if !hasPerm {
		slog.Warn("PermRead denied", "user_id", claims.UserID, "folder_id", req.FolderId)
		return nil, status.Error(codes.PermissionDenied, "missing PermRead on folder")
	}

	// 2. Fetch subfolders
	dbFolders, err := s.dbManager.ListFolders(req.FolderId)
	if err != nil {
		slog.Error("Failed to list subfolders", "error", err, "folder_id", req.FolderId)
		return nil, status.Errorf(codes.Internal, "failed to list folders: %v", err)
	}

	// 3. Fetch files
	dbFiles, err := s.dbManager.ListFiles(req.FolderId)
	if err != nil {
		slog.Error("Failed to list files in folder", "error", err, "folder_id", req.FolderId)
		return nil, status.Errorf(codes.Internal, "failed to list files: %v", err)
	}

	// 4. Map to proto
	respFolders := make([]*v1.Folder, len(dbFolders))
	for i, f := range dbFolders {
		respFolders[i] = &v1.Folder{
			FolderId:         f.FolderID,
			Name:             f.Name,
			ParentFolderId:   f.ParentFolderID,
			Path:             f.Path,
			Inheritance:      f.Inheritance,
			VersionRetention: f.VersionRetention,
			Metadata:         f.Metadata,
			MerkleRoot:       f.MerkleRoot,
			CreatedAt:        f.CreatedAt,
		}
	}

	respFiles := make([]*v1.File, len(dbFiles))
	for i, f := range dbFiles {
		respFiles[i] = &v1.File{
			FileId:      f.FileID,
			Name:        f.Name,
			FolderId:    f.FolderID,
			Path:        f.Path,
			StoragePath: f.StoragePath,
			Location:    f.Location,
			VersionId:   f.VersionID,
			CreatedAt:   f.CreatedAt,
			UpdatedAt:   f.UpdatedAt,
		}
	}

	slog.Debug("Folder listed", "folder_id", req.FolderId, "user_id", claims.UserID, "subfolder_count", len(respFolders), "file_count", len(respFiles))
	return &v1.ListFolderResponse{
		Folders: respFolders,
		Files:   respFiles,
	}, nil
}

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
