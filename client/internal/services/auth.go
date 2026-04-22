package services

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"strconv"

	"mandala-workspace/client/gen"
	"mandala-workspace/client/pkg/auth"
	"mandala-workspace/client/pkg/config"
	"mandala-workspace/client/pkg/logger"

	"google.golang.org/protobuf/proto"
)

type AuthService struct {
	userClient gen.UserServiceClient
	keyring    *auth.Keyring
	config     *config.Config
}

func NewAuthService(userClient gen.UserServiceClient, keyring *auth.Keyring, cfg *config.Config) *AuthService {
	return &AuthService{
		userClient: userClient,
		keyring:    keyring,
		config:     cfg,
	}
}

// UnlockKeyring attempts to unlock the local keyring with the master password.
func (s *AuthService) UnlockKeyring(password string) error {
	err := s.keyring.Unlock(password)
	if err != nil {
		logger.Error("Failed to unlock keyring", "error", err)
		return err
	}
	logger.Info("Keyring unlocked successfully")
	return nil
}

// InitKeyring initializes a new keyring. Only used during first-time setup.
func (s *AuthService) InitKeyring(password string, userID string, deviceID string, privateKey []byte) error {
	data := &auth.KeyringData{
		DevicePrivateKey: privateKey,
		UserID:           userID,
		DeviceID:         deviceID,
	}
	err := s.keyring.Init(password, data)
	if err != nil {
		logger.Error("Failed to initialize keyring", "error", err)
		return err
	}
	logger.Info("Keyring initialized successfully")
	return nil
}

// IsKeyringUnlocked checks if the keyring is currently in memory.
func (s *AuthService) IsKeyringUnlocked() bool {
	return s.keyring.GetData() != nil
}

// Login handles the challenge-response authentication.
func (s *AuthService) Login() (string, error) {
	data := s.keyring.GetData()
	if data == nil {
		return "", fmt.Errorf("keyring is locked")
	}

	userID, _ := strconv.ParseUint(data.UserID, 10, 64)
	deviceID, _ := strconv.ParseUint(data.DeviceID, 10, 64)

	logger.Info("Attempting login", "user_id", userID, "device_id", deviceID)

	// 1. Request Challenge
	challenge, err := s.userClient.LoginUser(context.Background(), &gen.LoginUserRequest{
		UserId:   userID,
		DeviceId: deviceID,
	})
	if err != nil {
		return "", fmt.Errorf("failed to get challenge: %w", err)
	}

	// 2. Sign Challenge
	challengeBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(challenge)
	if err != nil {
		return "", fmt.Errorf("failed to marshal challenge: %w", err)
	}

	signature := ed25519.Sign(data.DevicePrivateKey, challengeBytes)

	// 3. Verify
	resp, err := s.userClient.VerifyLoginSignature(context.Background(), &gen.LoginUserSignatureRequest{
		UserId:    userID,
		DeviceId:  deviceID,
		Timestamp: challenge.Timestamp,
		Signature: signature,
	})
	if err != nil {
		return "", fmt.Errorf("failed to verify signature: %w", err)
	}

	// 4. Update Token
	data.PasetoToken = resp.Token
	err = s.keyring.Save()
	if err != nil {
		logger.Warn("Failed to save updated token to keyring", "error", err)
	}

	return resp.Token, nil
}

// SetupAdmin handles the initial system setup.
func (s *AuthService) SetupAdmin(password string, name string, email string) error {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return err
	}

	resp, err := s.userClient.SetupAdmin(context.Background(), &gen.SetupAdminRequest{
		Name:      name,
		Email:     email,
		Password:  "initial-admin-password", // This is the server-side password if applicable, but we use device keys.
		PublicKey: pub,
	})
	if err != nil {
		return fmt.Errorf("SetupAdmin failed: %w", err)
	}

	// Initialize local keyring
	kData := &auth.KeyringData{
		DevicePrivateKey: priv,
		UserID:           strconv.FormatUint(resp.UserId, 10),
		DeviceID:         strconv.FormatUint(resp.DeviceId, 10),
	}

	return s.keyring.Init(password, kData)
}

// GetDeviceInfo returns the current user and device ID from the keyring.
func (s *AuthService) GetDeviceInfo() (map[string]string, error) {
	data := s.keyring.GetData()
	if data == nil {
		return nil, fmt.Errorf("keyring is locked")
	}
	return map[string]string{
		"user_id":   data.UserID,
		"device_id": data.DeviceID,
	}, nil
}
