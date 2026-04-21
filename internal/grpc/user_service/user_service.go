package user_service

import (
	"sync"

	"mandala-workspace/gen"
	"mandala-workspace/internal/crypto/paseto"
	"mandala-workspace/internal/db"
	"mandala-workspace/internal/grpc/session"
	"mandala-workspace/internal/permission"
)

type challengeEntry struct {
	bytes []byte
}

type UserService struct {
	gen.UnimplementedUserServiceServer
	db_manager         *db.DBManager
	paseto_manager     *paseto.Manager
	permission_manager *permission.PermissionManager
	session_manager    *session.SessionManager

	challengeCache sync.Map // key: string "userId:deviceId", value: *challengeEntry
}

func NewUserService(db_manager *db.DBManager, paseto_manager *paseto.Manager, permission_manager *permission.PermissionManager, session_manager *session.SessionManager) *UserService {
	return &UserService{
		db_manager:         db_manager,
		paseto_manager:     paseto_manager,
		permission_manager: permission_manager,
		session_manager:    session_manager,
	}
}
