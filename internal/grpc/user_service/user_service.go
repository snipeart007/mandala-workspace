package user_service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"mandala-workspace/gen"
	"mandala-workspace/internal/crypto/argon2"
	"mandala-workspace/internal/crypto/ed25519"
	"mandala-workspace/internal/crypto/paseto"
	"mandala-workspace/internal/db"
	"mandala-workspace/internal/grpc/interceptors"
	"mandala-workspace/internal/grpc/session"
	"mandala-workspace/internal/permission"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type challengeEntry struct {
	bytes []byte
}

type UserService struct {
	gen.UnimplementedUserServiceServer
	db_manager         *db.DBManager
	paseto_manager     *paseto.Manager
	permission_manager *permission.PermissionManager
	session_manager    *session.SessionManager

	challengeCache sync.Map // key: string "userId:deviceId", value: *challengeEntry
}

// LoginUser initiates the challenge-response authentication flow.
// Step 1: Client sends UserID and DeviceID
// Step 2: Server verifies the device exists and returns a challenge (UserID, DeviceID, current timestamp)
// The client will then sign this challenge with their private key and send it back via VerifyLoginSignature.
func (s *UserService) LoginUser(ctx context.Context, req *gen.LoginUserRequest) (*gen.LoginUserChallenge, error) {
	// Step 1: Validate input parameters
	if req.UserId == 0 || req.DeviceId == 0 {
		slog.Warn("Login attempt with invalid IDs", "user_id", req.UserId, "device_id", req.DeviceId)
		return nil, status.Error(codes.InvalidArgument, "user_id and device_id must be non-zero")
	}

	// Step 2: Verify that the device exists in the database by fetching its public key
	// This ensures we don't create challenges for non-existent devices
	_, err := s.db_manager.GetDevicePublicKey(req.UserId, req.DeviceId)
	if err != nil {
		slog.Warn("Login attempt for non-existent or revoked device", "user_id", req.UserId, "device_id", req.DeviceId, "error", err)
		return nil, status.Errorf(codes.NotFound, "device not found: %v", err)
	}

	// Step 3: Create and return the challenge payload
	// The challenge includes the current server time (Unix timestamp in seconds)
	// The client will sign this exact message, and we'll verify it later
	challenge := &gen.LoginUserChallenge{
		UserId:    req.UserId,
		DeviceId:  req.DeviceId,
		Timestamp: uint64(time.Now().Unix()),
	}

	challengeBytes, err := marshalChallengeForSignature(challenge)
	if err != nil {
		slog.Error("Failed to marshal login challenge", "error", err, "user_id", req.UserId)
		return nil, status.Errorf(codes.Internal, "failed to marshal challenge: %v", err)
	}

	cacheKey := fmt.Sprintf("%d:%d", req.UserId, req.DeviceId)
	entry := &challengeEntry{bytes: challengeBytes}
	s.challengeCache.Store(cacheKey, entry)

	// delete any cached login challenge after 3 mins
	time.AfterFunc(3*time.Minute, func() {
		s.challengeCache.CompareAndDelete(cacheKey, entry)
	})

	slog.Info("Login challenge issued", "user_id", req.UserId, "device_id", req.DeviceId)
	return challenge, nil
}

// VerifyLoginSignature verifies the ed25519 signature of the challenge and issues a Paseto token.
// Step 1: Client sends the signed challenge
// Step 2: Server reconstructs the challenge from the request
// Step 3: Server retrieves the public key from the database
// Step 4: Server verifies the signature using ed25519
// Step 5: If signature is valid, server creates and returns a Paseto token containing UserID and DeviceID
func (s *UserService) VerifyLoginSignature(ctx context.Context, req *gen.LoginUserSignatureRequest) (*gen.LoginUserTokenResponse, error) {
	// Step 1: Validate input parameters
	if req.UserId == 0 || req.DeviceId == 0 {
		slog.Warn("Verify signature attempt with invalid IDs", "user_id", req.UserId, "device_id", req.DeviceId)
		return nil, status.Error(codes.InvalidArgument, "user_id and device_id must be non-zero")
	}

	if len(req.Signature) == 0 {
		slog.Warn("Verify signature attempt with empty signature", "user_id", req.UserId, "device_id", req.DeviceId)
		return nil, status.Error(codes.InvalidArgument, "signature cannot be empty")
	}

	// Step 2: Retrieve the cached challenge
	cacheKey := fmt.Sprintf("%d:%d", req.UserId, req.DeviceId)
	cachedEntry, ok := s.challengeCache.Load(cacheKey)
	if !ok {
		slog.Warn("Login challenge not found or expired", "user_id", req.UserId, "device_id", req.DeviceId)
		return nil, status.Errorf(codes.Unauthenticated, "login challenge expired or not found")
	}
	challengeBytes := cachedEntry.(*challengeEntry).bytes

	// Delete challenge to prevent replay attacks
	s.challengeCache.Delete(cacheKey)

	// Step 3: Retrieve the device's public key from the database
	// This key will be used to verify the signature
	publicKey, err := s.db_manager.GetDevicePublicKey(req.UserId, req.DeviceId)
	if err != nil {
		slog.Warn("Public key not found for signature verification", "user_id", req.UserId, "device_id", req.DeviceId, "error", err)
		return nil, status.Errorf(codes.NotFound, "device not found: %v", err)
	}

	// Step 4: Verify the ed25519 signature
	// This ensures the signature was created by the holder of the private key corresponding to the public key stored in the database
	err = ed25519.VerifySignature(publicKey, challengeBytes, req.Signature)
	if err != nil {
		slog.Warn("Signature verification failed", "user_id", req.UserId, "device_id", req.DeviceId, "error", err)
		return nil, status.Errorf(codes.Unauthenticated, "signature verification failed: %v", err)
	}

	// Step 5: Add to active session cache
	s.session_manager.AddSession(req.UserId, req.DeviceId)

	// Step 6: Signature is valid - create and return a Paseto token
	// The token contains the UserID and DeviceID for use in subsequent authenticated requests
	token, err := s.paseto_manager.CreateToken(req.UserId, req.DeviceId)
	if err != nil {
		slog.Error("Failed to create paseto token", "error", err, "user_id", req.UserId)
		return nil, status.Errorf(codes.Internal, "failed to create token: %v", err)
	}

	slog.Info("User logged in successfully", "user_id", req.UserId, "device_id", req.DeviceId)
	response := &gen.LoginUserTokenResponse{
		Token: token,
	}

	return response, nil
}

// marshalChallengeForSignature converts a LoginUserChallenge to bytes for signature verification
// This helper ensures deterministic protobuf serialization between the client signing and server verification
func marshalChallengeForSignature(challenge *gen.LoginUserChallenge) ([]byte, error) {
	return proto.MarshalOptions{Deterministic: true}.Marshal(challenge)
}

// CreateUser creates a new user account with the given details.
// It requires the caller to have PermUserCreate permission on the root folder (ID 1).
func (s *UserService) CreateUser(ctx context.Context, req *gen.CreateUserRequest) (*gen.CreateUserResponse, error) {
	// Step 1: Extract claims from context
	claims, err := interceptors.GetTokenClaims(ctx)
	if err != nil {
		slog.Warn("CreateUser attempt with missing claims", "error", err)
		return nil, status.Errorf(codes.Unauthenticated, "failed to get token claims: %v", err)
	}

	// Step 2: Check for PermUserCreate permission on the root folder (ID 1)
	hasPerm, err := s.permission_manager.HasPermission(claims.UserID, 1, permission.PermUserCreate)
	if err != nil {
		slog.Error("Failed to check PermUserCreate", "error", err, "user_id", claims.UserID)
		return nil, status.Errorf(codes.Internal, "failed to check permissions: %v", err)
	}

	if !hasPerm {
		slog.Warn("PermUserCreate denied", "user_id", claims.UserID)
		return nil, status.Error(codes.PermissionDenied, "missing PermUserCreate permission")
	}

	// Step 3: Hash the password using argon2
	hashedPassword, err := argon2.HashPassword(req.Password)
	if err != nil {
		slog.Error("Failed to hash password", "error", err)
		return nil, status.Errorf(codes.Internal, "failed to hash password: %v", err)
	}

	// Step 4: Create the user in the database
	userID, createdAt, err := s.db_manager.CreateUser(req.Name, req.Email, []byte(hashedPassword), req.Metadata)
	if err != nil {
		slog.Error("Failed to create user in DB", "error", err, "name", req.Name, "email", req.Email)
		return nil, status.Errorf(codes.Internal, "failed to create user: %v", err)
	}

	slog.Info("New user created by admin", "new_user_id", userID, "admin_user_id", claims.UserID)
	return &gen.CreateUserResponse{
		UserId:    userID,
		CreatedAt: createdAt,
	}, nil
}

// RegisterDevice registers a new device for a user.
// It requires the caller to have PermDeviceSetup permission on the root folder (ID 1).
func (s *UserService) RegisterDevice(ctx context.Context, req *gen.RegisterDeviceRequest) (*gen.RegisterDeviceResponse, error) {
	// Step 1: Extract claims from context
	claims, err := interceptors.GetTokenClaims(ctx)
	if err != nil {
		slog.Warn("RegisterDevice attempt with missing claims", "error", err)
		return nil, status.Errorf(codes.Unauthenticated, "failed to get token claims: %v", err)
	}

	// Step 2: Check for PermDeviceSetup permission on the root folder (ID 1)
	hasPerm, err := s.permission_manager.HasPermission(claims.UserID, 1, permission.PermDeviceSetup)
	if err != nil {
		slog.Error("Failed to check PermDeviceSetup", "error", err, "user_id", claims.UserID)
		return nil, status.Errorf(codes.Internal, "failed to check permissions: %v", err)
	}

	if !hasPerm {
		slog.Warn("PermDeviceSetup denied", "user_id", claims.UserID)
		return nil, status.Error(codes.PermissionDenied, "missing PermDeviceSetup permission")
	}

	// Step 3: Register the device in the database
	deviceID, createdAt, err := s.db_manager.RegisterDevice(req.UserId, req.PublicKey, req.Metadata)
	if err != nil {
		slog.Error("Failed to register device in DB", "error", err, "target_user_id", req.UserId)
		return nil, status.Errorf(codes.Internal, "failed to register device: %v", err)
	}

	slog.Info("Device registered for user", "device_id", deviceID, "target_user_id", req.UserId, "admin_user_id", claims.UserID)
	return &gen.RegisterDeviceResponse{
		DeviceId:  deviceID,
		CreatedAt: createdAt,
	}, nil
}

// RevokeDevice revokes a device and invalidates its current session.
// It requires the caller to have PermAdmin permission on the root folder (ID 1).
func (s *UserService) RevokeDevice(ctx context.Context, req *gen.RevokeDeviceRequest) (*gen.RevokeDeviceResponse, error) {
	// Step 1: Extract claims from context
	claims, err := interceptors.GetTokenClaims(ctx)
	if err != nil {
		slog.Warn("RevokeDevice attempt with missing claims", "error", err)
		return nil, status.Errorf(codes.Unauthenticated, "failed to get token claims: %v", err)
	}

	// Step 2: Check for PermAdmin permission on the root folder (ID 1)
	hasPerm, err := s.permission_manager.HasPermission(claims.UserID, 1, permission.PermAdmin)
	if err != nil {
		slog.Error("Failed to check PermAdmin", "error", err, "user_id", claims.UserID)
		return nil, status.Errorf(codes.Internal, "failed to check permissions: %v", err)
	}

	if !hasPerm {
		slog.Warn("PermAdmin denied for RevokeDevice", "user_id", claims.UserID)
		return nil, status.Error(codes.PermissionDenied, "missing PermAdmin permission")
	}

	// Step 3: Revoke device in database
	err = s.db_manager.RevokeDevice(req.UserId, req.DeviceId)
	if err != nil {
		slog.Error("Failed to revoke device in DB", "error", err, "target_user_id", req.UserId, "device_id", req.DeviceId)
		return nil, status.Errorf(codes.Internal, "failed to revoke device in db: %v", err)
	}

	// Step 4: Remove from active session cache
	s.session_manager.RemoveSession(req.UserId, req.DeviceId)

	slog.Info("Device revoked by admin", "target_user_id", req.UserId, "device_id", req.DeviceId, "admin_user_id", claims.UserID)
	return &gen.RevokeDeviceResponse{
		Success: true,
	}, nil
}
