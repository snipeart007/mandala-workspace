package db

type FolderModel struct {
	FolderID         uint64
	Name             string
	ParentFolderID   uint64
	Path             string
	Inheritance      bool
	VersionRetention uint32
	Metadata         []byte
	MerkleRoot       []byte
	CreatedAt        uint64
}

type FileModel struct {
	FileID      uint64
	Name        string
	FolderID    uint64
	Path        string
	StoragePath string
	Location    string
	VersionID   uint64
	Metadata    []byte
	CreatedAt   uint64
	UpdatedAt   uint64
}

type VersionModel struct {
	VersionID      uint64
	FileID         uint64
	Version        string
	Hash           []byte
	UserID         uint64
	Metadata       []byte
	VersionComment string
	CreatedAt      uint64
}
