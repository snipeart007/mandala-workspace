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
	// Skip authentication for unauthenticated methods
	if isUnauthenticatedMethod(info.FullMethod) {
		return handler(ctx, req)
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		slog.Warn("Incoming gRPC request missing metadata", "method", info.FullMethod)
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	authHeader := md.Get("authorization")
	if len(authHeader) == 0 {
		slog.Warn("Incoming gRPC request missing authorization header", "method", info.FullMethod)
		return nil, status.Error(codes.Unauthenticated, "missing authorization token")
	}

	token := strings.TrimPrefix(authHeader[0], "Bearer ")
	if token == authHeader[0] {
		// No "Bearer " prefix found, try using the raw token
		token = authHeader[0]
	}

	claims, err := ai.pasetoManager.VerifyToken(token)
	if err != nil {
		slog.Warn("PASETO token verification failed", "method", info.FullMethod, "error", err)
		return nil, status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
	}

	// Check if the session is active in the cache
	if !ai.sessionManager.IsSessionActive(claims.UserID, claims.DeviceID) {
		slog.Warn("Session expired or revoked", "method", info.FullMethod, "user_id", claims.UserID, "device_id", claims.DeviceID)
		return nil, status.Error(codes.Unauthenticated, "session expired or revoked")
	}

	// Store claims in context for handlers to access
	ctx = context.WithValue(ctx, TokenClaimsContextKey, claims)

	return handler(ctx, req)
}

// GetTokenClaims extracts the token claims from the context
func GetTokenClaims(ctx context.Context) (paseto.TokenClaims, error) {
	claims, ok := ctx.Value(TokenClaimsContextKey).(paseto.TokenClaims)
	if !ok {
		return paseto.TokenClaims{}, fmt.Errorf("token claims not found in context")
	}
	return claims, nil
}
