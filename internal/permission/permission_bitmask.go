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

	// --- Versioning Control (Bits 8-15) ---
	PermViewHistory PermissionBitMask = 1 << (8 + iota) // 512
	PermRestoreVersion                            // 1024
	PermDeleteVersion                             // 2048

	// --- Folder & Structural Management (Bits 16-23) ---
	PermCreateFolder PermissionBitMask = 1 << (16 + iota)
	PermMoveFolder
	PermDeleteFolder

	// --- Governance & Administrative (Bits 24-31) ---
	PermShare     PermissionBitMask = 1 << (24 + iota)
	PermUserCreate								// Create Users
	PermDeviceSetup								// Setup Devices
	PermAuditView                               // View access logs
	PermAdmin                                   // Full control override
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
