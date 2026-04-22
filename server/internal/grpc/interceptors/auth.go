/*
Package interceptors provides gRPC middleware for the Mandala workspace services.
This file implements the authentication interceptor that validates PASETO tokens.
*/
package interceptors

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"mandala-workspace/internal/crypto/paseto"
	"mandala-workspace/internal/grpc/session"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type contextKey string

const (
	TokenClaimsContextKey contextKey = "tokenClaims"
)

// unauthenticatedMethods is a list of gRPC FullMethods that do not require authentication
var unauthenticatedMethods = []string{
	// Login endpoints that don't require authentication
	"/v1.UserService/LoginUser",
	"/v1.UserService/VerifyLoginSignature",
}

type AuthInterceptor struct {
	pasetoManager  *paseto.Manager
	sessionManager *session.SessionManager
}

func NewAuthInterceptor(pasetoManager *paseto.Manager, sessionManager *session.SessionManager) *AuthInterceptor {
	return &AuthInterceptor{
		pasetoManager:  pasetoManager,
		sessionManager: sessionManager,
	}
}

// isUnauthenticatedMethod checks if the given FullMethod requires authentication
func isUnauthenticatedMethod(fullMethod string) bool {
	for _, method := range unauthenticatedMethods {
		if method == fullMethod {
			return true
		}
	}
	return false
}

// SetUnauthenticatedMethods updates the list of unauthenticated methods (primarily for testing)
func SetUnauthenticatedMethods(methods []string) {
	unauthenticatedMethods = methods
}

// GetUnauthenticatedMethods returns the current list of unauthenticated methods
func GetUnauthenticatedMethods() []string {
	methods := make([]string, len(unauthenticatedMethods))
	copy(methods, unauthenticatedMethods)
	return methods
}

func (ai *AuthInterceptor) UnaryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	slog.Info("Unary RPC Entry", "method", info.FullMethod)

	// Skip authentication for unauthenticated methods
	if isUnauthenticatedMethod(info.FullMethod) {
		slog.Debug("Skipping authentication for method", "method", info.FullMethod)
		return handler(ctx, req)
	}

	claims, err := ai.authenticate(ctx, info.FullMethod)
	if err != nil {
		return nil, err
	}

	// Store claims in context for handlers to access
	ctx = context.WithValue(ctx, TokenClaimsContextKey, claims)

	resp, err := handler(ctx, req)
	if err == nil {
		slog.Info("Unary RPC Success", "method", info.FullMethod)
	} else {
		slog.Error("Unary RPC Failure", "method", info.FullMethod, "error", err)
	}
	return resp, err
}

func (ai *AuthInterceptor) StreamServerInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	slog.Info("Stream RPC Entry", "method", info.FullMethod)

	// Skip authentication for unauthenticated methods
	if isUnauthenticatedMethod(info.FullMethod) {
		slog.Debug("Skipping authentication for method", "method", info.FullMethod)
		return handler(srv, ss)
	}

	claims, err := ai.authenticate(ss.Context(), info.FullMethod)
	if err != nil {
		return err
	}

	// Wrap the stream with the new context containing the claims
	wrapped := &wrappedStream{
		ServerStream: ss,
		ctx:          context.WithValue(ss.Context(), TokenClaimsContextKey, claims),
	}

	err = handler(srv, wrapped)
	if err == nil {
		slog.Info("Stream RPC Success", "method", info.FullMethod)
	} else {
		slog.Error("Stream RPC Failure", "method", info.FullMethod, "error", err)
	}
	return err
}

func (ai *AuthInterceptor) authenticate(ctx context.Context, method string) (paseto.TokenClaims, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		slog.Warn("Incoming gRPC request missing metadata", "method", method)
		return paseto.TokenClaims{}, status.Error(codes.Unauthenticated, "missing metadata")
	}

	authHeader := md.Get("authorization")
	if len(authHeader) == 0 {
		slog.Warn("Incoming gRPC request missing authorization header", "method", method)
		return paseto.TokenClaims{}, status.Error(codes.Unauthenticated, "missing authorization token")
	}

	token := strings.TrimPrefix(authHeader[0], "Bearer ")
	if token == authHeader[0] {
		// No "Bearer " prefix found, try using the raw token
		token = authHeader[0]
	}

	claims, err := ai.pasetoManager.VerifyToken(token)
	if err != nil {
		slog.Warn("PASETO token verification failed", "method", method, "error", err)
		return paseto.TokenClaims{}, status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
	}

	// Check if the session is active in the cache
	if !ai.sessionManager.IsSessionActive(claims.UserID, claims.DeviceID) {
		slog.Warn("Session expired or revoked", "method", method, "user_id", claims.UserID, "device_id", claims.DeviceID)
		return paseto.TokenClaims{}, status.Error(codes.Unauthenticated, "session expired or revoked")
	}

	slog.Info("Authentication successful", "method", method, "user_id", claims.UserID)
	return claims, nil
}

type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context {
	return w.ctx
}

// GetTokenClaims extracts the token claims from the context
func GetTokenClaims(ctx context.Context) (paseto.TokenClaims, error) {
	claims, ok := ctx.Value(TokenClaimsContextKey).(paseto.TokenClaims)
	if !ok {
		return paseto.TokenClaims{}, fmt.Errorf("token claims not found in context")
	}
	return claims, nil
}
