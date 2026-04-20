package user_service

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mandala-workspace/gen"
	"mandala-workspace/internal/crypto"
	"mandala-workspace/internal/crypto/paseto"
	"mandala-workspace/internal/db"

	_ "github.com/mattn/go-sqlite3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func newGRPCServerWithDevice(t *testing.T, userID, deviceID uint64, publicKey []byte) (*UserService, func()) {
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir to temp dir: %v", err)
	}

	mgr, err := db.NewDBManager(&db.DBManagerConfig{})
	if err != nil {
		os.Chdir(oldWd)
		t.Fatalf("failed to create DB manager: %v", err)
	}

	setupConn, err := sql.Open("sqlite3", filepath.Join(tmpDir, "db.sqlite"))
	if err != nil {
		mgr.Close()
		os.Chdir(oldWd)
		t.Fatalf("failed to open setup sqlite connection: %v", err)
	}

	_, err = setupConn.Exec(`CREATE TABLE devices (
		device_id INTEGER PRIMARY KEY,
		user_id INTEGER NOT NULL,
		public_key BLOB NOT NULL
	)`)
	if err != nil {
		setupConn.Close()
		mgr.Close()
		os.Chdir(oldWd)
		t.Fatalf("failed to create devices table: %v", err)
	}

	_, err = setupConn.Exec("INSERT INTO devices (device_id, user_id, public_key) VALUES (?, ?, ?)", deviceID, userID, publicKey)
	if err != nil {
		setupConn.Close()
		mgr.Close()
		os.Chdir(oldWd)
		t.Fatalf("failed to insert device row: %v", err)
	}

	setupConn.Close()

	pasetoKey := bytes.Repeat([]byte{0x02}, 32)
	pasetoMgr, err := paseto.NewManager(pasetoKey)
	if err != nil {
		mgr.Close()
		os.Chdir(oldWd)
		t.Fatalf("failed to create paseto manager: %v", err)
	}

	return &UserService{
			db_manager:     mgr,
			paseto_manager: pasetoMgr,
		}, func() {
			mgr.Close()
			os.Chdir(oldWd)
		}
}

func TestLoginUserReturnsChallenge(t *testing.T) {
	userID := uint64(1)
	deviceID := uint64(2)
	publicKey, _, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}

	server, cleanup := newGRPCServerWithDevice(t, userID, deviceID, publicKey)
	defer cleanup()

	challenge, err := server.LoginUser(context.Background(), &gen.LoginUserRequest{UserId: userID, DeviceId: deviceID})
	if err != nil {
		t.Fatalf("LoginUser returned error: %v", err)
	}

	if challenge.UserId != userID {
		t.Fatalf("expected UserId %d, got %d", userID, challenge.UserId)
	}
	if challenge.DeviceId != deviceID {
		t.Fatalf("expected DeviceId %d, got %d", deviceID, challenge.DeviceId)
	}
	if challenge.Timestamp == 0 {
		t.Fatal("expected non-zero timestamp in challenge")
	}
}

func TestVerifyLoginSignatureIssuesToken(t *testing.T) {
	userID := uint64(10)
	deviceID := uint64(20)
	publicKey, privateKey, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}

	server, cleanup := newGRPCServerWithDevice(t, userID, deviceID, publicKey)
	defer cleanup()

	challenge := &gen.LoginUserChallenge{
		UserId:    userID,
		DeviceId:  deviceID,
		Timestamp: uint64(time.Now().Unix()),
	}

	challengeBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(challenge)
	if err != nil {
		t.Fatalf("failed to marshal challenge: %v", err)
	}

	signature, err := crypto.SignMessage(privateKey, challengeBytes)
	if err != nil {
		t.Fatalf("failed to sign challenge: %v", err)
	}

	resp, err := server.VerifyLoginSignature(context.Background(), &gen.LoginUserSignatureRequest{
		UserId:    userID,
		DeviceId:  deviceID,
		Timestamp: challenge.Timestamp,
		Signature: signature,
	})
	if err != nil {
		t.Fatalf("VerifyLoginSignature returned error: %v", err)
	}

	if resp == nil || resp.Token == "" {
		t.Fatal("expected non-empty token")
	}
}

func TestVerifyLoginSignatureInvalidSignature(t *testing.T) {
	userID := uint64(30)
	deviceID := uint64(40)
	publicKey, _, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}

	server, cleanup := newGRPCServerWithDevice(t, userID, deviceID, publicKey)
	defer cleanup()

	challenge := &gen.LoginUserChallenge{
		UserId:    userID,
		DeviceId:  deviceID,
		Timestamp: uint64(time.Now().Unix()),
	}

	challengeBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(challenge)
	if err != nil {
		t.Fatalf("failed to marshal challenge: %v", err)
	}

	badSignature := make([]byte, 64)
	if len(challengeBytes) == 0 {
		t.Fatal("expected challenge bytes")
	}

	resp, err := server.VerifyLoginSignature(context.Background(), &gen.LoginUserSignatureRequest{
		UserId:    userID,
		DeviceId:  deviceID,
		Timestamp: challenge.Timestamp,
		Signature: badSignature,
	})
	if err == nil {
		t.Fatal("expected error for invalid signature")
	}

	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated error, got: %v", err)
	}
	if resp != nil {
		t.Fatal("expected nil response when signature verification fails")
	}
}
