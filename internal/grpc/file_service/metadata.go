package file_service

import (
	"context"
	"fmt"

	"mandala-workspace/gen"
	"mandala-workspace/internal/grpc/interceptors"
	"mandala-workspace/internal/permission"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *FileServiceServer) ModifyFile(ctx context.Context, req *gen.ModifyFileRequest) (*gen.FileResponse, error) {
	claims, err := interceptors.GetTokenClaims(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}

	file, err := s.dbManager.GetFile(req.FileId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "file not found")
	}

	bitmask, err := s.dbManager.GetUserEffectivePermissions(claims.UserID, file.FolderID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "permission check failed")
	}
	perms := permission.PermissionBitMask(bitmask)

	// Renaming/Moving requires specific permissions
	if req.Name != nil || req.FolderId != nil {
		if !perms.Has(permission.PermRename) {
			return nil, status.Error(codes.PermissionDenied, "missing PermRename")
		}
	} else if len(req.Metadata) > 0 {
		if !perms.Has(permission.PermWrite) {
			return nil, status.Error(codes.PermissionDenied, "missing PermWrite for metadata update")
		}
	}

	// Apply updates
	if req.Name != nil {
		if err := s.dbManager.RenameFile(req.FileId, *req.Name); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to rename file: %v", err)
		}
	}

	if req.FolderId != nil {
		targetFolder, err := s.dbManager.GetFolder(*req.FolderId)
		if err != nil {
			return nil, status.Errorf(codes.NotFound, "target folder not found")
		}
		// Compute new path
		newPath := fmt.Sprintf("%s%d/", targetFolder.Path, targetFolder.FolderID)
		if err := s.dbManager.MoveFile(req.FileId, *req.FolderId, newPath); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to move file: %v", err)
		}
	}

	if len(req.Metadata) > 0 {
		if err := s.dbManager.UpdateFileMetadata(req.FileId, req.Metadata); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to update metadata: %v", err)
		}
	}

	// Fetch updated file
	updatedFile, err := s.dbManager.GetFile(req.FileId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch updated file")
	}

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
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}

	file, err := s.dbManager.GetFile(req.FileId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "file not found")
	}

	bitmask, err := s.dbManager.GetUserEffectivePermissions(claims.UserID, file.FolderID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "permission check failed")
	}
	if !permission.PermissionBitMask(bitmask).Has(permission.PermViewHistory) {
		return nil, status.Error(codes.PermissionDenied, "missing PermViewHistory")
	}

	dbVersions, err := s.dbManager.ListVersions(req.FileId)
	if err != nil {
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
	return &gen.ListVersionsResponse{Versions: respVersions}, nil
}

func (s *FileServiceServer) SetRetentionPolicy(ctx context.Context, req *gen.SetRetentionPolicyRequest) (*gen.SetRetentionPolicyResponse, error) {
	claims, err := interceptors.GetTokenClaims(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}

	bitmask, err := s.dbManager.GetUserEffectivePermissions(claims.UserID, req.FolderId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "permission check failed")
	}
	if !permission.PermissionBitMask(bitmask).Has(permission.PermAdmin) {
		return nil, status.Error(codes.PermissionDenied, "missing PermAdmin")
	}

	if err := s.dbManager.SetRetentionPolicy(req.FolderId, req.LastNVersions); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update policy")
	}

	return &gen.SetRetentionPolicyResponse{Success: true}, nil
}

func (s *FileServiceServer) DeleteFile(ctx context.Context, req *gen.DeleteFileRequest) (*gen.DeleteFileResponse, error) {
	claims, err := interceptors.GetTokenClaims(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}

	file, err := s.dbManager.GetFile(req.FileId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "file not found")
	}

	bitmask, err := s.dbManager.GetUserEffectivePermissions(claims.UserID, file.FolderID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "permission check failed")
	}
	if !permission.PermissionBitMask(bitmask).Has(permission.PermDelete) {
		return nil, status.Error(codes.PermissionDenied, "missing PermDelete")
	}

	if err := s.dbManager.SoftDeleteFile(req.FileId); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to soft-delete file: %v", err)
	}

	return &gen.DeleteFileResponse{Success: true}, nil
}
