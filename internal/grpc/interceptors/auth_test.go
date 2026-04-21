package interceptors

import (
	"bytes"
	"context"
	"testing"

	"mandala-workspace/internal/crypto/paseto"
	"mandala-workspace/internal/grpc/session"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestAuthInterceptorValidToken(t *testing.T) {
	key := bytes.Repeat([]byte{0x01}, 32)
	manager, err := paseto.NewManager(key)
	if err != nil {
		t.Fatalf("failed to create Paseto manager: %v", err)
	}

	sessionMgr := session.NewSessionManager()
	interceptor := NewAuthInterceptor(manager, sessionMgr)

	userID := uint64(123)
	deviceID := uint64(456)
	token, err := manager.CreateToken(userID, deviceID)
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	// Add session to cache
	sessionMgr.AddSession(userID, deviceID)

	// Create context with authorization metadata
	md := metadata.Pairs("authorization", "Bearer "+token)
	ctx := metadata.NewIncomingContext(context.Background(), md)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		claims, err := GetTokenClaims(ctx)
		if err != nil {
			return nil, err
		}
		if claims.UserID != userID || claims.DeviceID != deviceID {
			t.Fatalf("claims mismatch: got UserID=%d DeviceID=%d, want UserID=%d DeviceID=%d",
				claims.UserID, claims.DeviceID, userID, deviceID)
		}
		return "ok", nil
	}

	info := &grpc.UnaryServerInfo{
		FullMethod: "/v1.UserService/TestMethod",
	}

	resp, err := interceptor.UnaryServerInterceptor(ctx, nil, info, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp != "ok" {
		t.Fatalf("expected response 'ok', got %v", resp)
	}
}

func TestAuthInterceptorInvalidToken(t *testing.T) {
	key := bytes.Repeat([]byte{0x01}, 32)
	manager, err := paseto.NewManager(key)
	if err != nil {
		t.Fatalf("failed to create Paseto manager: %v", err)
	}

	sessionMgr := session.NewSessionManager()
	interceptor := NewAuthInterceptor(manager, sessionMgr)

	// Create context with invalid authorization token
	md := metadata.Pairs("authorization", "Bearer invalid-token")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "ok", nil
	}

	info := &grpc.UnaryServerInfo{
		FullMethod: "/v1.UserService/TestMethod",
	}

	_, err = interceptor.UnaryServerInterceptor(ctx, nil, info, handler)
	if err == nil {
		t.Fatal("expected error for invalid token")
	}

	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated error, got: %v", err)
	}
}

func TestAuthInterceptorMissingToken(t *testing.T) {
	key := bytes.Repeat([]byte{0x01}, 32)
	manager, err := paseto.NewManager(key)
	if err != nil {
		t.Fatalf("failed to create Paseto manager: %v", err)
	}

	sessionMgr := session.NewSessionManager()
	interceptor := NewAuthInterceptor(manager, sessionMgr)

	// Create context without authorization metadata
	ctx := context.Background()

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "ok", nil
	}

	info := &grpc.UnaryServerInfo{
		FullMethod: "/v1.UserService/TestMethod",
	}

	_, err = interceptor.UnaryServerInterceptor(ctx, nil, info, handler)
	if err == nil {
		t.Fatal("expected error for missing metadata")
	}

	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated error, got: %v", err)
	}
}

func TestAuthInterceptorTokenWithoutBearer(t *testing.T) {
	key := bytes.Repeat([]byte{0x01}, 32)
	manager, err := paseto.NewManager(key)
	if err != nil {
		t.Fatalf("failed to create Paseto manager: %v", err)
	}

	sessionMgr := session.NewSessionManager()
	interceptor := NewAuthInterceptor(manager, sessionMgr)

	userID := uint64(789)
	deviceID := uint64(101)
	token, err := manager.CreateToken(userID, deviceID)
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	// Add session to cache
	sessionMgr.AddSession(userID, deviceID)

	// Create context with authorization token without Bearer prefix
	md := metadata.Pairs("authorization", token)
	ctx := metadata.NewIncomingContext(context.Background(), md)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		claims, err := GetTokenClaims(ctx)
		if err != nil {
			return nil, err
		}
		return claims.UserID, nil
	}

	info := &grpc.UnaryServerInfo{
		FullMethod: "/v1.UserService/TestMethod",
	}

	resp, err := interceptor.UnaryServerInterceptor(ctx, nil, info, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp != userID {
		t.Fatalf("expected UserID %d, got %v", userID, resp)
	}
}

func TestAuthInterceptorUnauthenticatedMethod(t *testing.T) {
	key := bytes.Repeat([]byte{0x01}, 32)
	manager, err := paseto.NewManager(key)
	if err != nil {
		t.Fatalf("failed to create Paseto manager: %v", err)
	}

	sessionMgr := session.NewSessionManager()
	interceptor := NewAuthInterceptor(manager, sessionMgr)

	// Temporarily set an unauthenticated method for testing
	originalMethods := GetUnauthenticatedMethods()
	defer SetUnauthenticatedMethods(originalMethods)
	SetUnauthenticatedMethods([]string{"/v1.UserService/PublicMethod"})

	// Create context without authorization metadata
	ctx := context.Background()

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "public_response", nil
	}

	info := &grpc.UnaryServerInfo{
		FullMethod: "/v1.UserService/PublicMethod",
	}

	resp, err := interceptor.UnaryServerInterceptor(ctx, nil, info, handler)
	if err != nil {
		t.Fatalf("unexpected error for unauthenticated method: %v", err)
	}

	if resp != "public_response" {
		t.Fatalf("expected 'public_response', got %v", resp)
	}
}

func TestAuthInterceptorRevokedSession(t *testing.T) {
	key := bytes.Repeat([]byte{0x01}, 32)
	manager, err := paseto.NewManager(key)
	if err != nil {
		t.Fatalf("failed to create Paseto manager: %v", err)
	}

	sessionMgr := session.NewSessionManager()
	interceptor := NewAuthInterceptor(manager, sessionMgr)

	userID := uint64(123)
	deviceID := uint64(456)
	token, err := manager.CreateToken(userID, deviceID)
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	// We DON'T add the session to the manager, or we add and then remove it
	sessionMgr.AddSession(userID, deviceID)
	sessionMgr.RemoveSession(userID, deviceID)

	// Create context with authorization metadata
	md := metadata.Pairs("authorization", "Bearer "+token)
	ctx := metadata.NewIncomingContext(context.Background(), md)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "ok", nil
	}

	info := &grpc.UnaryServerInfo{
		FullMethod: "/v1.UserService/TestMethod",
	}

	_, err = interceptor.UnaryServerInterceptor(ctx, nil, info, handler)
	if err == nil {
		t.Fatal("expected error for revoked session")
	}

	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated error, got: %v", err)
	}

	if st.Message() != "session expired or revoked" {
		t.Fatalf("expected 'session expired or revoked' message, got: %s", st.Message())
	}
}

type mockServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (m *mockServerStream) Context() context.Context {
	return m.ctx
}

func TestAuthInterceptor_StreamInterceptor(t *testing.T) {
	key := bytes.Repeat([]byte{0x01}, 32)
	manager, err := paseto.NewManager(key)
	if err != nil {
		t.Fatalf("failed to create Paseto manager: %v", err)
	}

	sessionMgr := session.NewSessionManager()
	interceptor := NewAuthInterceptor(manager, sessionMgr)

	userID := uint64(123)
	deviceID := uint64(456)
	token, err := manager.CreateToken(userID, deviceID)
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}
	sessionMgr.AddSession(userID, deviceID)

	md := metadata.Pairs("authorization", "Bearer "+token)
	ctx := metadata.NewIncomingContext(context.Background(), md)
	ss := &mockServerStream{ctx: ctx}

	handler := func(srv interface{}, ss grpc.ServerStream) error {
		claims, err := GetTokenClaims(ss.Context())
		if err != nil {
			return err
		}
		if claims.UserID != userID {
			t.Errorf("expected userID %d, got %d", userID, claims.UserID)
		}
		return nil
	}

	err = interceptor.StreamServerInterceptor(nil, ss, &grpc.StreamServerInfo{FullMethod: "/v1.FileService/UploadFile"}, handler)
	if err != nil {
		t.Fatalf("StreamServerInterceptor failed: %v", err)
	}
}

