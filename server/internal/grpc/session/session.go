/*
Package session provides an in-memory session manager for tracking active user sessions.
It allows for fast validation of session status and supports session revocation.
*/
package session

import (
	"fmt"
	"log/slog"
	"sync"
)

// SessionManager handles the in-memory cache of active user sessions.
type SessionManager struct {
	// key: string "userID:deviceID", value: bool
	sessions sync.Map
}

func NewSessionManager() *SessionManager {
	slog.Info("Initializing SessionManager")
	return &SessionManager{}
}

// AddSession marks a user:device pair as logged in.
func (sm *SessionManager) AddSession(userID uint64, deviceID uint64) {
	key := fmt.Sprintf("%d:%d", userID, deviceID)
	slog.Info("Adding session", "user_id", userID, "device_id", deviceID)
	sm.sessions.Store(key, true)
}

// RemoveSession removes a user:device pair from active sessions.
func (sm *SessionManager) RemoveSession(userID uint64, deviceID uint64) {
	key := fmt.Sprintf("%d:%d", userID, deviceID)
	slog.Info("Removing session", "user_id", userID, "device_id", deviceID)
	sm.sessions.Delete(key)
}

// IsSessionActive checks if a user:device pair has an active session.
func (sm *SessionManager) IsSessionActive(userID uint64, deviceID uint64) bool {
	key := fmt.Sprintf("%d:%d", userID, deviceID)
	_, ok := sm.sessions.Load(key)
	if !ok {
		slog.Debug("Session check: inactive", "user_id", userID, "device_id", deviceID)
	} else {
		slog.Debug("Session check: active", "user_id", userID, "device_id", deviceID)
	}
	return ok
}
