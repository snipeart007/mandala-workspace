package db

type FolderModel struct {
	FolderID       uint64
	Name           string
	ParentFolderID uint64
	Path           string
	Inheritance    bool
	Metadata       []byte
	MerkleRoot     []byte
	CreatedAt      uint64
}

type FileModel struct {
	FileID      uint64
	Name        string
	FolderID    uint64
	Path        string
	StoragePath string
	Location    string
	VersionID   uint64
	CreatedAt   uint64
	UpdatedAt   uint64
}
