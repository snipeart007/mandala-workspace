// Package permission defines the permission system for the workspace, using bitmasks for efficient access control.
// This file implements the PermissionManager, which handles evaluating permissions based on user context and folder hierarchy.
package permission

import (
	"log/slog"
	"mandala-workspace/internal/db"
)

type PermissionManager struct {
	db_manager db.DBProvider
}

func NewPermissionManager(db_manager db.DBProvider) *PermissionManager {
	return &PermissionManager{db_manager}
}

func (pm *PermissionManager) CheckPermission(p *Permission, target PermissionBitMask) bool {
	if p == nil {
		slog.Warn("Permission check failed: permission object is nil")
		return false
	}
	if p.bitmask.Has(PermAdmin) {
		slog.Debug("Permission check: user has admin bypass", "user_id", p.user_id, "folder_id", p.folder_id)
		return true
	}
	hasPerm := p.bitmask.Has(target)
	slog.Debug("Permission check evaluated", "user_id", p.user_id, "folder_id", p.folder_id, "target", target, "granted", hasPerm)
	return hasPerm
}

func (pm *PermissionManager) HasPermission(userID uint64, folderID uint64, target PermissionBitMask) (bool, error) {
	bitmask, err := pm.db_manager.GetUserEffectivePermissions(userID, folderID)
	if err != nil {
		slog.Error("Failed to fetch effective permissions from database", "user_id", userID, "folder_id", folderID, "error", err)
		return false, err
	}

	p := &Permission{
		bitmask:   PermissionBitMask(bitmask),
		user_id:   uint64(userID),
		folder_id: uint64(folderID),
	}

	return pm.CheckPermission(p, target), nil
}
