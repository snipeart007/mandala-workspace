// Package session contains tests for the session manager, which tracks active user sessions.
// It ensures that sessions can be added, checked for activity, and removed correctly.
package session

import (
	"testing"
)

func TestSessionManager(t *testing.T) {
	sm := NewSessionManager()
	
	userID := uint64(1)
	deviceID := uint64(10)
	
	// 1. Initially not active
	if sm.IsSessionActive(userID, deviceID) {
		t.Errorf("expected session to be inactive")
	}
	
	// 2. Add session
	sm.AddSession(userID, deviceID)
	if !sm.IsSessionActive(userID, deviceID) {
		t.Errorf("expected session to be active")
	}
	
	// 3. Different device
	if sm.IsSessionActive(userID, 11) {
		t.Errorf("expected different device to be inactive")
	}
	
	// 4. Remove session
	sm.RemoveSession(userID, deviceID)
	if sm.IsSessionActive(userID, deviceID) {
		t.Errorf("expected session to be inactive after removal")
	}
}
