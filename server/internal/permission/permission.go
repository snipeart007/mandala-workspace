// Package permission defines the permission system for the workspace, using bitmasks for efficient access control.
// This file defines the Permission struct and methods for generating permissions from a hierarchy.
package permission


// type PermissionID uint64

type Permission struct {
	bitmask PermissionBitMask
	user_id uint64
	folder_id uint64
	perm_ids []uint64
}

type PermissionHierarchy []struct {
	perm_id uint64
	bitmask PermissionBitMask
}

// Generate Permission using all the permissions of parent folders
func GeneratePermission(user_id uint64, folder_id uint64, hierarchy PermissionHierarchy) *Permission {
	var final_bitmask PermissionBitMask
	var perm_ids []uint64
	for _, perm := range hierarchy {
		final_bitmask = final_bitmask.Add(perm.bitmask)
		perm_ids = append(perm_ids, perm.perm_id)
	}
	return &Permission{final_bitmask, user_id, folder_id, perm_ids}
}


