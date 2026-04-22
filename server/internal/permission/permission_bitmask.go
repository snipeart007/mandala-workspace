// Package permission defines the permission system for the workspace, using bitmasks for efficient access control.
// This file specifically implements the PermissionBitMask type and its associated constants and helper methods.
package permission

import "fmt"

// The 63rd bit should be used only if very necessary
// PermissionBitMask represents a bitmask of access rights.
type PermissionBitMask uint64

const (
	// --- Basic Data Operations (Bits 0-7) ---
	PermRead   PermissionBitMask = 1 << iota // 1 (2^0)
	PermWrite                         // 2 (2^1)
	PermCreate                        // 4 (2^2)
	PermDelete                        // 8 (2^3)
	PermRename                        // 16 (2^4)
)

const (
	// --- Versioning Control (Bits 8-15) ---
	PermViewHistory PermissionBitMask = 1 << (8 + iota) // 512
	PermRestoreVersion                            // 1024
	PermDeleteVersion                             // 2048
)

const (
	// --- Folder & Structural Management (Bits 16-23) ---
	PermCreateFolder PermissionBitMask = 1 << (16 + iota)
	PermMoveFolder
	PermDeleteFolder
)

const (
	// --- Governance & Administrative (Bits 24-31) ---
	PermShare          PermissionBitMask = 1 << (24 + iota)
	PermUserCreate                                  // 25
	PermDeviceSetup                                 // 26
	PermAuditView                                   // 27
	PermSetPermissions                              // 28
	PermAdmin          PermissionBitMask = 1 << 31  // 31
)

// --- Helper Methods ---

// Has checks if the bitmask contains a specific permission.
func (p PermissionBitMask) Has(target PermissionBitMask) bool {
	return (p & target) == target
}

// Add returns a new bitmask with the permission added.
func (p PermissionBitMask) Add(target PermissionBitMask) PermissionBitMask {
	return p | target
}

// Remove returns a new bitmask with the permission removed.
func (p PermissionBitMask) Remove(target PermissionBitMask) PermissionBitMask {
	return p &^ target
}

func (p PermissionBitMask) String() string {
	return fmt.Sprintf("PermissionBitMask(%064b)", p)
}
