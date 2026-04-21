package folder_service

import (
	"context"
	"log/slog"

	v1 "mandala-workspace/gen"
	"mandala-workspace/internal/grpc/interceptors"
	"mandala-workspace/internal/permission"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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
