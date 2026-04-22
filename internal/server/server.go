// Package server provides the primary server orchestration and configuration for the mandala-workspace.
package server

import (
	"fmt"
	"log/slog"
	"net"

	"mandala-workspace/gen"
	"mandala-workspace/internal/crypto/paseto"
	"mandala-workspace/internal/db"
	"mandala-workspace/internal/grpc/file_service"
	"mandala-workspace/internal/grpc/folder_service"
	"mandala-workspace/internal/grpc/interceptors"
	"mandala-workspace/internal/grpc/session"
	"mandala-workspace/internal/grpc/user_service"
	"mandala-workspace/internal/permission"
	"mandala-workspace/internal/storage"

	"google.golang.org/grpc"
)

// ServerInstanceConfig holds all configuration and secrets for the mandala server.
type ServerInstanceConfig struct {
	GRPCAddr           string
	DBPath             string
	InitialSchemePath  string
	PasetoSecretKey    []byte
	LocalStoragePath   string
	DefaultStorageScheme string
}

// ServerInstance orchestrates all backend components and the gRPC server.
type ServerInstance struct {
	config            *ServerInstanceConfig
	grpcServer        *grpc.Server
	dbManager         *db.DBManager
	pasetoManager     *paseto.Manager
	sessionManager    *session.SessionManager
	permissionManager *permission.PermissionManager
	storageRegistry   *storage.CASRegistry

	// Services
	userService   *user_service.UserService
	folderService *folder_service.FolderService
	fileService   *file_service.FileServiceServer
}

// NewServerInstance creates and initializes all components for the server.
func NewServerInstance(config *ServerInstanceConfig) (*ServerInstance, error) {
	slog.Info("Initializing ServerInstance", "addr", config.GRPCAddr)

	// 1. Initialize DBManager
	dbManager, err := db.NewDBManager(&db.DBManagerConfig{
		InitialSchemePath: config.InitialSchemePath,
		DBPath:            config.DBPath,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize DBManager: %w", err)
	}

	// 2. Initialize Crypto and Auth components
	pasetoManager, err := paseto.NewManager(config.PasetoSecretKey)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize PASETO manager: %w", err)
	}
	sessionManager := session.NewSessionManager()

	// 3. Initialize Permission Manager
	permissionManager := permission.NewPermissionManager(dbManager)

	// 4. Initialize Storage
	storageRegistry := storage.NewCASRegistry()
	localStorage, err := storage.NewLocalStorage(config.LocalStoragePath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize local storage: %w", err)
	}
	storageRegistry.Register(localStorage)

	// 5. Initialize Services
	slog.Debug("Initializing gRPC service implementations")
	userService := user_service.NewUserService(dbManager, pasetoManager, permissionManager, sessionManager)
	folderService := folder_service.NewFolderService(dbManager, permissionManager)
	fileService := file_service.NewFileServiceServer(dbManager, storageRegistry, permissionManager, config.DefaultStorageScheme)

	// 6. Setup Auth Interceptor
	slog.Debug("Setting up authentication interceptor")
	authInterceptor := interceptors.NewAuthInterceptor(pasetoManager, sessionManager)

	// 7. Initialize gRPC Server
	slog.Debug("Creating gRPC server with interceptors")
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(authInterceptor.UnaryServerInterceptor),
		grpc.StreamInterceptor(authInterceptor.StreamServerInterceptor),
	)

	// Register Services
	slog.Info("Registering gRPC services")
	gen.RegisterUserServiceServer(grpcServer, userService)
	gen.RegisterFolderServiceServer(grpcServer, folderService)
	gen.RegisterFileServiceServer(grpcServer, fileService)

	slog.Info("ServerInstance initialized successfully")

	return &ServerInstance{
		config:            config,
		grpcServer:        grpcServer,
		dbManager:         dbManager,
		pasetoManager:     pasetoManager,
		sessionManager:    sessionManager,
		permissionManager: permissionManager,
		storageRegistry:   storageRegistry,
		userService:       userService,
		folderService:     folderService,
		fileService:       fileService,
	}, nil
}

// Start launches the gRPC server and handles initialization.
func (s *ServerInstance) Start() error {
	slog.Info("Starting server", "addr", s.config.GRPCAddr)

	// Setup database schema if needed
	if err := s.dbManager.Setup(); err != nil {
		return fmt.Errorf("failed to setup database: %w", err)
	}

	lis, err := net.Listen("tcp", s.config.GRPCAddr)
	if err != nil {
		slog.Error("Failed to listen", "addr", s.config.GRPCAddr, "error", err)
		return fmt.Errorf("failed to listen: %w", err)
	}

	slog.Info("gRPC server listening", "addr", s.config.GRPCAddr)
	if err := s.grpcServer.Serve(lis); err != nil {
		slog.Error("Failed to serve", "error", err)
		return fmt.Errorf("failed to serve: %w", err)
	}

	return nil
}

// Stop gracefully shuts down the gRPC server and cleans up resources.
func (s *ServerInstance) Stop() {
	slog.Info("Stopping server")
	s.grpcServer.GracefulStop()
	s.dbManager.Close()
}

// GetDBManager returns the underlying DBManager.
func (s *ServerInstance) GetDBManager() *db.DBManager {
	return s.dbManager
}

// GetPasetoManager returns the PASETO manager.
func (s *ServerInstance) GetPasetoManager() *paseto.Manager {
	return s.pasetoManager
}

// GetSessionManager returns the session manager.
func (s *ServerInstance) GetSessionManager() *session.SessionManager {
	return s.sessionManager
}

// GetPermissionManager returns the permission manager.
func (s *ServerInstance) GetPermissionManager() *permission.PermissionManager {
	return s.permissionManager
}

// GetStorageRegistry returns the storage registry.
func (s *ServerInstance) GetStorageRegistry() *storage.CASRegistry {
	return s.storageRegistry
}

// GetUserService returns the user service.
func (s *ServerInstance) GetUserService() *user_service.UserService {
	return s.userService
}

// GetFolderService returns the folder service.
func (s *ServerInstance) GetFolderService() *folder_service.FolderService {
	return s.folderService
}

// GetFileService returns the file service server.
func (s *ServerInstance) GetFileService() *file_service.FileServiceServer {
	return s.fileService
}
