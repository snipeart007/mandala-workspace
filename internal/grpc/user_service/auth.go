// Package user_service implements authentication flows for the mandala service.
package user_service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"mandala-workspace/gen"
	"mandala-workspace/internal/crypto/ed25519"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// LoginUser initiates the challenge-response authentication flow.
func (s *UserService) LoginUser(ctx context.Context, req *gen.LoginUserRequest) (*gen.LoginUserChallenge, error) {
	slog.Info("LoginUser RPC entry", "user_id", req.UserId, "device_id", req.DeviceId)
	if req.UserId == 0 || req.DeviceId == 0 {
		slog.Warn("Login attempt with invalid IDs", "user_id", req.UserId, "device_id", req.DeviceId)
		return nil, status.Error(codes.InvalidArgument, "user_id and device_id must be non-zero")
	}

	_, err := s.db_manager.GetDevicePublicKey(req.UserId, req.DeviceId)
	if err != nil {
		slog.Warn("Login attempt for non-existent or revoked device", "user_id", req.UserId, "device_id", req.DeviceId, "error", err)
		return nil, status.Errorf(codes.NotFound, "device not found: %v", err)
	}

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

	time.AfterFunc(3*time.Minute, func() {
		s.challengeCache.CompareAndDelete(cacheKey, entry)
	})

	slog.Info("Login challenge issued", "user_id", req.UserId, "device_id", req.DeviceId)
	return challenge, nil
}

// VerifyLoginSignature verifies the ed25519 signature and issues a Paseto token.
func (s *UserService) VerifyLoginSignature(ctx context.Context, req *gen.LoginUserSignatureRequest) (*gen.LoginUserTokenResponse, error) {
	slog.Info("VerifyLoginSignature RPC entry", "user_id", req.UserId, "device_id", req.DeviceId)
	if req.UserId == 0 || req.DeviceId == 0 {
		slog.Warn("Verify signature attempt with invalid IDs", "user_id", req.UserId, "device_id", req.DeviceId)
		return nil, status.Error(codes.InvalidArgument, "user_id and device_id must be non-zero")
	}

	if len(req.Signature) == 0 {
		slog.Warn("Verify signature attempt with empty signature", "user_id", req.UserId, "device_id", req.DeviceId)
		return nil, status.Error(codes.InvalidArgument, "signature cannot be empty")
	}

	cacheKey := fmt.Sprintf("%d:%d", req.UserId, req.DeviceId)
	cachedEntry, ok := s.challengeCache.Load(cacheKey)
	if !ok {
		slog.Warn("Login challenge not found or expired", "user_id", req.UserId, "device_id", req.DeviceId)
		return nil, status.Errorf(codes.Unauthenticated, "login challenge expired or not found")
	}
	challengeBytes := cachedEntry.(*challengeEntry).bytes
	s.challengeCache.Delete(cacheKey)

	publicKey, err := s.db_manager.GetDevicePublicKey(req.UserId, req.DeviceId)
	if err != nil {
		slog.Warn("Public key not found for signature verification", "user_id", req.UserId, "device_id", req.DeviceId, "error", err)
		return nil, status.Errorf(codes.NotFound, "device not found: %v", err)
	}

	err = ed25519.VerifySignature(publicKey, challengeBytes, req.Signature)
	if err != nil {
		slog.Warn("Signature verification failed", "user_id", req.UserId, "device_id", req.DeviceId, "error", err)
		return nil, status.Errorf(codes.Unauthenticated, "signature verification failed: %v", err)
	}

	s.session_manager.AddSession(req.UserId, req.DeviceId)

	token, err := s.paseto_manager.CreateToken(req.UserId, req.DeviceId)
	if err != nil {
		slog.Error("Failed to create paseto token", "error", err, "user_id", req.UserId)
		return nil, status.Errorf(codes.Internal, "failed to create token: %v", err)
	}

	slog.Info("User logged in successfully", "user_id", req.UserId, "device_id", req.DeviceId)
	return &gen.LoginUserTokenResponse{Token: token}, nil
}

func marshalChallengeForSignature(challenge *gen.LoginUserChallenge) ([]byte, error) {
	return proto.MarshalOptions{Deterministic: true}.Marshal(challenge)
}
