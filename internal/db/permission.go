// Package db provides permission-related database operations including bitmask retrieval and effective permission calculation.
package db

import (
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
)

func (self *DBManager) GetUserPermissionBitmask(userID uint64, folderID uint64) (uint64, error) {
	slog.Debug("Getting user permission bitmask", "user_id", userID, "folder_id", folderID)
	var permissions uint64
	err := self.db.QueryRow(
		"SELECT permissions FROM permissions WHERE user_id = ? AND folder_id = ?",
		userID, folderID,
	).Scan(&permissions)
	
	if err != nil {
		if err == sql.ErrNoRows {
			slog.Debug("No explicit permissions found", "user_id", userID, "folder_id", folderID)
			return 0, nil
		}
		slog.Error("Failed to get user permission bitmask", "user_id", userID, "folder_id", folderID, "error", err)
		return 0, err
	}
	
	return permissions, nil
}

// GetUserEffectivePermissions implements path-based prefix search with inheritance break logic.
func (self *DBManager) GetUserEffectivePermissions(userID uint64, folderID uint64) (uint64, error) {
	slog.Debug("Calculating effective permissions", "user_id", userID, "folder_id", folderID)
	// 1. Get the target folder's path and inheritance bit
	var path string
	var inheritance bool
	err := self.db.QueryRow("SELECT path, inheritance FROM folders WHERE folder_id = ?", folderID).Scan(&path, &inheritance)
	if err != nil {
		slog.Error("Failed to fetch folder path for effective permissions", "folder_id", folderID, "error", err)
		return 0, fmt.Errorf("failed to fetch folder path: %w", err)
	}

	// 2. Parse the path to get all ancestor IDs
	// Path format is "1/2/3/"
	parts := strings.Split(strings.Trim(path, "/"), "/")
	var folderIDs []any
	for _, p := range parts {
		if p != "" {
			var id uint64
			fmt.Sscanf(p, "%d", &id)
			folderIDs = append(folderIDs, id)
		}
	}
	// Add the target folder itself
	folderIDs = append(folderIDs, folderID)

	// 3. Fetch inheritance status and permissions for all folders in the chain
	// We use a query to get everything in one go for performance
	placeholders := strings.Repeat("?,", len(folderIDs))
	placeholders = placeholders[:len(placeholders)-1]

	// Fetch inheritance for the chain
	inheritanceMap := make(map[uint64]bool)
	rows, err := self.db.Query(fmt.Sprintf("SELECT folder_id, inheritance FROM folders WHERE folder_id IN (%s) AND deleted_at IS NULL", placeholders), folderIDs...)
	if err != nil {
		slog.Error("Failed to fetch inheritance chain", "error", err)
		return 0, fmt.Errorf("failed to fetch inheritance chain: %w", err)
	}
	count := 0
	for rows.Next() {
		var id uint64
		var inh bool
		rows.Scan(&id, &inh)
		inheritanceMap[id] = inh
		count++
	}
	rows.Close()

	if count < len(folderIDs) {
		slog.Warn("Some folders in the chain are deleted or not found", "found", count, "expected", len(folderIDs))
		return 0, fmt.Errorf("folder or its ancestors are deleted or not found (count=%d, expected=%d)", count, len(folderIDs))
	}

	// Fetch explicit permissions for this user in the chain
	permissionMap := make(map[uint64]uint64)
	rows, err = self.db.Query(fmt.Sprintf("SELECT folder_id, permissions FROM permissions WHERE user_id = ? AND folder_id IN (%s)", placeholders), append([]interface{}{userID}, folderIDs...)...)
	if err != nil {
		slog.Error("Failed to fetch permissions chain", "user_id", userID, "error", err)
		return 0, fmt.Errorf("failed to fetch permissions chain: %w", err)
	}
	for rows.Next() {
		var id uint64
		var perm uint64
		if err := rows.Scan(&id, &perm); err != nil {
			slog.Error("Failed to scan permission row in chain", "error", err)
			return 0, fmt.Errorf("failed to scan permission row: %w", err)
		}
		permissionMap[id] = perm
	}
	rows.Close()

	// 4. Calculate effective permissions walking from ROOT downwards
	// We walk downwards to easily identify where the "break" happens
	var effectivePerm uint64
	
	// Convert strings to uint64 for the loop
	var idChain []uint64
	for _, p := range parts {
		if p == "" { continue }
		var id uint64
		fmt.Sscanf(p, "%d", &id)
		idChain = append(idChain, id)
	}
	idChain = append(idChain, folderID)

	for i, id := range idChain {
		perm := permissionMap[id]
		
		// Admin Bypasses Break: If we encounter PermAdmin, we keep it regardless of breaks later
		// (though usually Admin is granted at a level and carries down)
		const PermAdmin uint64 = 1 << 31 // Bit 31 as defined in permission_bitmask.go

		// If this folder breaks inheritance, clear all previous aggregated permissions
		// UNLESS we have Admin bypass (simplified: we keep Admin bit if set)
		if !inheritanceMap[id] && i > 0 {
			slog.Debug("Inheritance break detected", "folder_id", id)
			hasAdmin := (effectivePerm & PermAdmin) != 0
			effectivePerm = 0
			if hasAdmin {
				effectivePerm |= PermAdmin
			}
		}

		effectivePerm |= perm
	}

	slog.Debug("Effective permissions calculated", "user_id", userID, "folder_id", folderID, "permissions", effectivePerm)
	return effectivePerm, nil
}

func (self *DBManager) SetUserPermission(userID uint64, folderID uint64, permissions uint64) error {
	slog.Info("Setting user permission", "user_id", userID, "folder_id", folderID, "permissions", permissions)
	_, err := self.db.Exec(`
		INSERT INTO permissions (user_id, folder_id, permissions)
		VALUES (?, ?, ?)
		ON CONFLICT(user_id, folder_id) DO UPDATE SET permissions = excluded.permissions
	`, userID, folderID, permissions)
	if err != nil {
		slog.Error("Failed to set user permission", "user_id", userID, "folder_id", folderID, "error", err)
	}
	return err
}
