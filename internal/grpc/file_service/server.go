/*
Package file_service provides the gRPC server implementation for file operations.
It handles file uploads, downloads, and metadata management with CAS storage.
*/
package file_service

import (
	"log/slog"
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
	slog.Info("Initializing FileServiceServer", "default_scheme", defaultScheme)
	return &FileServiceServer{
		dbManager:         dbManager,
		storageRegistry:   storageRegistry,
		permissionManager: permissionManager,
		defaultScheme:     defaultScheme,
	}
}
