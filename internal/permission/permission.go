package permission

import (
	"mandala-workspace/internal/types/folder"
	"mandala-workspace/internal/types/user"
)

type PermissionID uint64

type Permission struct {
	bitmask PermissionBitMask
	user_id user.UserID
	folder_id folder.FolderID
	perm_ids []PermissionID
}

type PermissionHierarchy []struct {
	perm_id PermissionID
	bitmask PermissionBitMask
}

// Generate Permission using all the permissions of parent folders
func GeneratePermission(user_id user.UserID, folder_id folder.FolderID, hierarchy PermissionHierarchy) *Permission {
	var final_bitmask PermissionBitMask
	var perm_ids []PermissionID
	for _, perm := range hierarchy {
		final_bitmask = final_bitmask.Add(perm.bitmask)
		perm_ids = append(perm_ids, perm.perm_id)
	}
	return &Permission{final_bitmask, user_id, folder_id, perm_ids}
}


