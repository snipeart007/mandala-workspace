package folder_service

import (
	"context"
	"fmt"

	v1 "mandala-workspace/gen"
	"mandala-workspace/internal/grpc/interceptors"
	"mandala-workspace/internal/permission"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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
