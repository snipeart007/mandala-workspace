package db

// DBProvider defines the interface for database operations.
type DBProvider interface {
	Setup() error
	Close()

	// User operations
	GetDevicePublicKey(userID uint64, deviceID uint64) ([]byte, error)
	CreateUser(name string, email string, passwordHash []byte, metadata []byte) (uint64, uint64, error)
	GetUserCount() (int, error)
	RegisterDevice(userID uint64, publicKey []byte, metadata []byte) (uint64, uint64, error)
	RevokeDevice(userID uint64, deviceID uint64) error

	// Folder operations
	EnsureRootFolder() error
	GetFolderPath(folderID uint64) (string, error)
	CreateFolder(name string, parentID uint64, path string, inheritance bool, retention uint32, metadata []byte) (uint64, uint64, error)
	GetFolder(folderID uint64) (*FolderModel, error)
	SetRetentionPolicy(folderID uint64, lastN uint32) error
	GetVersionRetention(folderID uint64) (uint32, error)
	ListFolders(parentID uint64) ([]FolderModel, error)
	MoveFolder(folderID uint64, newParentID uint64, newPath string) error
	SoftDeleteFolder(folderID uint64) error

	// File operations
	ListFiles(folderID uint64) ([]FileModel, error)
	CreateFile(name string, folderID uint64, path string, storagePath string, location string, metadata []byte) (uint64, uint64, error)
	GetFile(fileID uint64) (*FileModel, error)
	GetFileByName(folderID uint64, name string) (*FileModel, error)
	CreateVersion(fileID uint64, version string, hash []byte, userID uint64, metadata []byte, comment string) (uint64, error)
	UpdateFileVersion(fileID uint64, versionID uint64, storagePath string, location string) error
	ListVersions(fileID uint64) ([]VersionModel, error)
	DeleteOldVersions(fileID uint64, keepLastN uint32) (int64, error)
	SoftDeleteFile(fileID uint64) error
	RenameFile(fileID uint64, newName string) error
	MoveFile(fileID uint64, newFolderID uint64, newPath string) error
	UpdateFileMetadata(fileID uint64, metadata []byte) error

	// Permission operations
	GetUserPermissionBitmask(userID uint64, folderID uint64) (uint64, error)
	GetUserEffectivePermissions(userID uint64, folderID uint64) (uint64, error)
	SetUserPermission(userID uint64, folderID uint64, permissions uint64) error
}
