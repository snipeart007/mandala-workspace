package session

import (
	"fmt"
	"sync"
)

// SessionManager handles the in-memory cache of active user sessions.
type SessionManager struct {
	// key: string "userID:deviceID", value: bool
	sessions sync.Map
}

func NewSessionManager() *SessionManager {
	return &SessionManager{}
}

// AddSession marks a user:device pair as logged in.
func (sm *SessionManager) AddSession(userID uint64, deviceID uint64) {
	key := fmt.Sprintf("%d:%d", userID, deviceID)
	sm.sessions.Store(key, true)
}

// RemoveSession removes a user:device pair from active sessions.
func (sm *SessionManager) RemoveSession(userID uint64, deviceID uint64) {
	key := fmt.Sprintf("%d:%d", userID, deviceID)
	sm.sessions.Delete(key)
}

// IsSessionActive checks if a user:device pair has an active session.
func (sm *SessionManager) IsSessionActive(userID uint64, deviceID uint64) bool {
	key := fmt.Sprintf("%d:%d", userID, deviceID)
	_, ok := sm.sessions.Load(key)
	return ok
}
