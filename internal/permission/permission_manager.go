package permission

import (
	"mandala-workspace/internal/db"
)

type PermissionManager struct {
	db_manager *db.DBManager
}

func NewPermissionManager(db_manager *db.DBManager) *PermissionManager {
	return &PermissionManager{db_manager}
}

func (pm *PermissionManager) CheckPermission(p *Permission, target PermissionBitMask) bool {
	if p == nil {
		return false
	}
	if p.bitmask.Has(PermAdmin) {
		return true
	}
	return p.bitmask.Has(target)
}

func (pm *PermissionManager) HasPermission(userID uint64, folderID uint64, target PermissionBitMask) (bool, error) {
	bitmask, err := pm.db_manager.GetUserEffectivePermissions(userID, folderID)
	if err != nil {
		return false, err
	}

	p := &Permission{
		bitmask:   PermissionBitMask(bitmask),
		user_id:   uint64(userID),
		folder_id: uint64(folderID),
	}

	return pm.CheckPermission(p, target), nil
}
