package file_service

import (
	"mandala-workspace/gen"
	"mandala-workspace/internal/db"
	"mandala-workspace/internal/permission"
	"mandala-workspace/internal/storage"
)

type FileServiceServer struct {
	gen.UnimplementedFileServiceServer
	dbManager         *db.DBManager
	storageRegistry   *storage.CASRegistry
	permissionManager *permission.PermissionManager
	defaultScheme     string
}

func NewFileServiceServer(dbManager *db.DBManager, storageRegistry *storage.CASRegistry, permissionManager *permission.PermissionManager, defaultScheme string) *FileServiceServer {
	return &FileServiceServer{
		dbManager:         dbManager,
		storageRegistry:   storageRegistry,
		permissionManager: permissionManager,
		defaultScheme:     defaultScheme,
	}
}
