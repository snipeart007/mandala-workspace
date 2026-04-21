package user_service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"mandala-workspace/gen"
	"mandala-workspace/internal/crypto/ed25519"
	"mandala-workspace/internal/crypto/paseto"
	"mandala-workspace/internal/db"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type challengeEntry struct {
	bytes []byte
}

type UserService struct {
	gen.UnimplementedUserServiceServer
	db_manager     *db.DBManager
	paseto_manager *paseto.Manager

	challengeCache sync.Map // key: string "userId:deviceId", value: *challengeEntry
}

// LoginUser initiates the challenge-response authentication flow.
// Step 1: Client sends UserID and DeviceID
// Step 2: Server verifies the device exists and returns a challenge (UserID, DeviceID, current timestamp)
// The client will then sign this challenge with their private key and send it back via VerifyLoginSignature.
func (s *UserService) LoginUser(ctx context.Context, req *gen.LoginUserRequest) (*gen.LoginUserChallenge, error) {
	// Step 1: Validate input parameters
	if req.UserId == 0 || req.DeviceId == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id and device_id must be non-zero")
	}

	// Step 2: Verify that the device exists in the database by fetching its public key
	// This ensures we don't create challenges for non-existent devices
	_, err := s.db_manager.GetDevicePublicKey(req.UserId, req.DeviceId)
	if err != nil {
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
		return nil, status.Errorf(codes.Internal, "failed to marshal challenge: %v", err)
	}

	cacheKey := fmt.Sprintf("%d:%d", req.UserId, req.DeviceId)
	entry := &challengeEntry{bytes: challengeBytes}
	s.challengeCache.Store(cacheKey, entry)

	// delete any cached login challenge after 3 mins
	time.AfterFunc(3*time.Minute, func() {
		s.challengeCache.CompareAndDelete(cacheKey, entry)
	})

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
		return nil, status.Error(codes.InvalidArgument, "user_id and device_id must be non-zero")
	}

	if len(req.Signature) == 0 {
		return nil, status.Error(codes.InvalidArgument, "signature cannot be empty")
	}

	// Step 2: Retrieve the cached challenge
	cacheKey := fmt.Sprintf("%d:%d", req.UserId, req.DeviceId)
	cachedEntry, ok := s.challengeCache.Load(cacheKey)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "login challenge expired or not found")
	}
	challengeBytes := cachedEntry.(*challengeEntry).bytes

	// Delete challenge to prevent replay attacks
	s.challengeCache.Delete(cacheKey)

	// Step 3: Retrieve the device's public key from the database
	// This key will be used to verify the signature
	publicKey, err := s.db_manager.GetDevicePublicKey(req.UserId, req.DeviceId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "device not found: %v", err)
	}

	// Step 4: Verify the ed25519 signature
	// This ensures the signature was created by the holder of the private key corresponding to the public key stored in the database
	err = ed25519.VerifySignature(publicKey, challengeBytes, req.Signature)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "signature verification failed: %v", err)
	}

	// Step 6: Signature is valid - create and return a Paseto token
	// The token contains the UserID and DeviceID for use in subsequent authenticated requests
	token, err := s.paseto_manager.CreateToken(req.UserId, req.DeviceId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create token: %v", err)
	}

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
