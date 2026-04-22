// Package user_service provides administrative functions for user and device management.
package user_service

import (
	"context"
	"log/slog"

	"mandala-workspace/gen"
	"mandala-workspace/internal/crypto/argon2"
	"mandala-workspace/internal/grpc/interceptors"
	"mandala-workspace/internal/permission"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CreateUser creates a new user account.
func (s *UserService) CreateUser(ctx context.Context, req *gen.CreateUserRequest) (*gen.CreateUserResponse, error) {
	slog.Info("CreateUser RPC entry", "name", req.Name, "email", req.Email)
	claims, err := interceptors.GetTokenClaims(ctx)
	if err != nil {
		slog.Warn("CreateUser attempt with missing claims", "error", err)
		return nil, status.Errorf(codes.Unauthenticated, "failed to get token claims: %v", err)
	}

	slog.Debug("Checking PermUserCreate", "user_id", claims.UserID)
	hasPerm, err := s.permission_manager.HasPermission(claims.UserID, 1, permission.PermUserCreate)
	if err != nil {
		slog.Error("Failed to check PermUserCreate", "error", err, "user_id", claims.UserID)
		return nil, status.Errorf(codes.Internal, "failed to check permissions: %v", err)
	}

	if !hasPerm {
		slog.Warn("PermUserCreate denied", "user_id", claims.UserID)
		return nil, status.Error(codes.PermissionDenied, "missing PermUserCreate permission")
	}

	slog.Debug("Hashing password for new user")
	hashedPassword, err := argon2.HashPassword(req.Password)
	if err != nil {
		slog.Error("Failed to hash password", "error", err)
		return nil, status.Errorf(codes.Internal, "failed to hash password: %v", err)
	}

	slog.Debug("Executing CreateUser in database")
	userID, createdAt, err := s.db_manager.CreateUser(req.Name, req.Email, []byte(hashedPassword), req.Metadata)
	if err != nil {
		slog.Error("Failed to create user in DB", "error", err, "name", req.Name, "email", req.Email)
		return nil, status.Errorf(codes.Internal, "failed to create user: %v", err)
	}

	slog.Info("New user created by admin", "new_user_id", userID, "admin_user_id", claims.UserID)
	return &gen.CreateUserResponse{UserId: userID, CreatedAt: createdAt}, nil
}

// RegisterDevice registers a new device for a user.
func (s *UserService) RegisterDevice(ctx context.Context, req *gen.RegisterDeviceRequest) (*gen.RegisterDeviceResponse, error) {
	slog.Info("RegisterDevice RPC entry", "target_user_id", req.UserId)
	claims, err := interceptors.GetTokenClaims(ctx)
	if err != nil {
		slog.Warn("RegisterDevice attempt with missing claims", "error", err)
		return nil, status.Errorf(codes.Unauthenticated, "failed to get token claims: %v", err)
	}

	slog.Debug("Checking PermDeviceSetup", "user_id", claims.UserID)
	hasPerm, err := s.permission_manager.HasPermission(claims.UserID, 1, permission.PermDeviceSetup)
	if err != nil {
		slog.Error("Failed to check PermDeviceSetup", "error", err, "user_id", claims.UserID)
		return nil, status.Errorf(codes.Internal, "failed to check permissions: %v", err)
	}

	if !hasPerm {
		slog.Warn("PermDeviceSetup denied", "user_id", claims.UserID)
		return nil, status.Error(codes.PermissionDenied, "missing PermDeviceSetup permission")
	}

	slog.Debug("Executing RegisterDevice in database", "target_user_id", req.UserId)
	deviceID, createdAt, err := s.db_manager.RegisterDevice(req.UserId, req.PublicKey, req.Metadata)
	if err != nil {
		slog.Error("Failed to register device in DB", "error", err, "target_user_id", req.UserId)
		return nil, status.Errorf(codes.Internal, "failed to register device: %v", err)
	}

	slog.Info("Device registered for user", "device_id", deviceID, "target_user_id", req.UserId, "admin_user_id", claims.UserID)
	return &gen.RegisterDeviceResponse{DeviceId: deviceID, CreatedAt: createdAt}, nil
}

// RevokeDevice revokes a device and invalidates its current session.
func (s *UserService) RevokeDevice(ctx context.Context, req *gen.RevokeDeviceRequest) (*gen.RevokeDeviceResponse, error) {
	slog.Info("RevokeDevice RPC entry", "target_user_id", req.UserId, "device_id", req.DeviceId)
	claims, err := interceptors.GetTokenClaims(ctx)
	if err != nil {
		slog.Warn("RevokeDevice attempt with missing claims", "error", err)
		return nil, status.Errorf(codes.Unauthenticated, "failed to get token claims: %v", err)
	}

	slog.Debug("Checking PermAdmin", "user_id", claims.UserID)
	hasPerm, err := s.permission_manager.HasPermission(claims.UserID, 1, permission.PermAdmin)
	if err != nil {
		slog.Error("Failed to check PermAdmin", "error", err, "user_id", claims.UserID)
		return nil, status.Errorf(codes.Internal, "failed to check permissions: %v", err)
	}

	if !hasPerm {
		slog.Warn("PermAdmin denied for RevokeDevice", "user_id", claims.UserID)
		return nil, status.Error(codes.PermissionDenied, "missing PermAdmin permission")
	}

	slog.Debug("Executing RevokeDevice in database", "target_user_id", req.UserId, "device_id", req.DeviceId)
	err = s.db_manager.RevokeDevice(req.UserId, req.DeviceId)
	if err != nil {
		slog.Error("Failed to revoke device in DB", "error", err, "target_user_id", req.UserId, "device_id", req.DeviceId)
		return nil, status.Errorf(codes.Internal, "failed to revoke device in db: %v", err)
	}

	slog.Debug("Invalidating session", "target_user_id", req.UserId, "device_id", req.DeviceId)
	s.session_manager.RemoveSession(req.UserId, req.DeviceId)

	slog.Info("Device revoked by admin", "target_user_id", req.UserId, "device_id", req.DeviceId, "admin_user_id", claims.UserID)
	return &gen.RevokeDeviceResponse{Success: true}, nil
}
