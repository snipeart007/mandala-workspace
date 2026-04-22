package grpcclient

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type mockTokenProvider struct {
	token string
}

func (m *mockTokenProvider) GetToken() string {
	return m.token
}

func TestAuthUnaryInterceptor(t *testing.T) {
	m := &ConnectionManager{}
	tp := &mockTokenProvider{token: "test-token"}

	interceptor := m.AuthUnaryInterceptor(tp)

	// Test authenticated method
	invoker := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			t.Fatal("no metadata in context")
		}
		authHeader := md.Get("authorization")
		if len(authHeader) == 0 || authHeader[0] != "Bearer test-token" {
			t.Errorf("expected Bearer test-token, got %v", authHeader)
		}
		return nil
	}

	err := interceptor(context.Background(), "/v1.FileService/ListFiles", nil, nil, nil, invoker)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Test skip method
	invokerSkip := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		md, _ := metadata.FromOutgoingContext(ctx)
		authHeader := md.Get("authorization")
		if len(authHeader) > 0 {
			t.Errorf("expected no auth header for skip method, got %v", authHeader)
		}
		return nil
	}
	err = interceptor(context.Background(), "/v1.UserService/LoginUser", nil, nil, nil, invokerSkip)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Test missing token
	tpEmpty := &mockTokenProvider{token: ""}
	interceptorEmpty := m.AuthUnaryInterceptor(tpEmpty)
	err = interceptorEmpty(context.Background(), "/v1.FileService/ListFiles", nil, nil, nil, nil)
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated code, got %v", status.Code(err))
	}
}

func TestAuthInterceptorUnauthCallback(t *testing.T) {
	callbackCalled := false
	m := &ConnectionManager{
		OnUnauthenticated: func() {
			callbackCalled = true
		},
	}
	tp := &mockTokenProvider{token: "expired-token"}

	interceptor := m.AuthUnaryInterceptor(tp)

	invoker := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		return status.Error(codes.Unauthenticated, "invalid token")
	}

	_ = interceptor(context.Background(), "/v1.FileService/ListFiles", nil, nil, nil, invoker)

	if !callbackCalled {
		t.Error("expected OnUnauthenticated callback to be called")
	}
}
