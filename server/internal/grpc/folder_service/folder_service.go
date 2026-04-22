// Package folder_service provides the gRPC service implementation for folder management.
package folder_service

import (
	"log/slog"

	v1 "mandala-workspace/gen"
	"mandala-workspace/internal/db"
	"mandala-workspace/internal/permission"
)

type FolderService struct {
	v1.UnimplementedFolderServiceServer
	dbManager         db.DBProvider
	permissionManager *permission.PermissionManager
}

func NewFolderService(dbManager db.DBProvider, permissionManager *permission.PermissionManager) *FolderService {
	slog.Debug("Initializing FolderService")
	return &FolderService{
		dbManager:         dbManager,
		permissionManager: permissionManager,
	}
}
