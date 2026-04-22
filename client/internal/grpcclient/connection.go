package grpcclient

import (
	"context"
	"fmt"
	"strings"

	"mandala-workspace/client/pkg/logger"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type TokenProvider interface {
	GetToken() string
}

type ConnectionManager struct {
	Conn             *grpc.ClientConn
	OnUnauthenticated func() // Callback for when a 401 is received
}

func NewConnectionManager(addr string, useTLS bool, tokenProvider TokenProvider, onUnauth func()) (*ConnectionManager, error) {
	m := &ConnectionManager{
		OnUnauthenticated: onUnauth,
	}

	var opts []grpc.DialOption

	if useTLS {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(nil, "")))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Add Interceptors
	opts = append(opts, grpc.WithUnaryInterceptor(m.AuthUnaryInterceptor(tokenProvider)))
	opts = append(opts, grpc.WithStreamInterceptor(m.AuthStreamInterceptor(tokenProvider)))

	conn, err := grpc.Dial(addr, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to gRPC server: %w", err)
	}

	m.Conn = conn
	return m, nil
}

func (m *ConnectionManager) Close() error {
	if m.Conn != nil {
		return m.Conn.Close()
	}
	return nil
}

func (m *ConnectionManager) AuthUnaryInterceptor(tp TokenProvider) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if shouldSkipAuth(method) {
			return invoker(ctx, method, req, reply, cc, opts...)
		}

		token := tp.GetToken()
		if token == "" {
			logger.Warn("Attempted authenticated request without token", "method", method)
			return status.Error(codes.Unauthenticated, "client: no authentication token available")
		}

		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
		err := invoker(ctx, method, req, reply, cc, opts...)
		
		if status.Code(err) == codes.Unauthenticated {
			logger.Error("Received Unauthenticated response from server", "method", method)
			if m.OnUnauthenticated != nil {
				m.OnUnauthenticated()
			}
		}
		
		return err
	}
}

func (m *ConnectionManager) AuthStreamInterceptor(tp TokenProvider) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		if shouldSkipAuth(method) {
			return streamer(ctx, desc, cc, method, opts...)
		}

		token := tp.GetToken()
		if token == "" {
			logger.Warn("Attempted authenticated stream without token", "method", method)
			return nil, status.Error(codes.Unauthenticated, "client: no authentication token available")
		}

		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
		stream, err := streamer(ctx, desc, cc, method, opts...)
		if err != nil {
			if status.Code(err) == codes.Unauthenticated {
				logger.Error("Received Unauthenticated response from server (stream)", "method", method)
				if m.OnUnauthenticated != nil {
					m.OnUnauthenticated()
				}
			}
			return nil, err
		}
		
		return stream, nil
	}
}

func shouldSkipAuth(method string) bool {
	skipMethods := []string{
		"/v1.UserService/LoginUser",
		"/v1.UserService/VerifyLoginSignature",
		"/v1.UserService/SetupAdmin",
	}

	for _, m := range skipMethods {
		if strings.HasSuffix(method, m) {
			return true
		}
	}
	return false
}
