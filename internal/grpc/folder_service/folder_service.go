package folder_service

import (
	v1 "mandala-workspace/gen"
	"mandala-workspace/internal/db"
	"mandala-workspace/internal/permission"
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
