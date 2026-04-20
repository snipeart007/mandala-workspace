package permission

import "mandala-workspace/internal/db"

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
