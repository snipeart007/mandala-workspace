package file_service

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"mandala-workspace/gen"
	"mandala-workspace/internal/db"
	"mandala-workspace/internal/grpc/interceptors"
	"mandala-workspace/internal/permission"
	"mandala-workspace/internal/storage"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type FileServiceServer struct {
	gen.UnimplementedFileServiceServer
	dbManager         *db.DBManager
	storageRegistry   *storage.CASRegistry
	permissionManager *permission.PermissionManager
	defaultScheme     string
}

func NewFileServiceServer(dbManager *db.DBManager, storageRegistry *storage.CASRegistry, permissionManager *permission.PermissionManager, defaultScheme string) *FileServiceServer {
	return &FileServiceServer{
		dbManager:         dbManager,
		storageRegistry:   storageRegistry,
		permissionManager: permissionManager,
		defaultScheme:     defaultScheme,
	}
}

// uploadStreamToCAS handles the heavy lifting of piping gRPC bytes to storage.
func (s *FileServiceServer) uploadStreamToCAS(ctx context.Context, recv func() ([]byte, error)) (string, error) {
	pr, pw := io.Pipe()

	type storeResult struct {
		uri string
		err error
	}
	resultChan := make(chan storeResult, 1)

	go func() {
		uri, err := s.storageRegistry.StoreInDefault(ctx, s.defaultScheme, pr)
		resultChan <- storeResult{uri, err}
	}()

	for {
		chunk, err := recv()
		if err == io.EOF {
			pw.Close()
			break
		}
		if err != nil {
			pw.CloseWithError(err)
			return "", err
		}
		if _, err := pw.Write(chunk); err != nil {
			return "", err
		}
	}

	res := <-resultChan
	return res.uri, res.err
}

func (s *FileServiceServer) UploadFile(stream gen.FileService_UploadFileServer) error {
	claims, err := interceptors.GetTokenClaims(stream.Context())
	if err != nil {
		return status.Error(codes.Unauthenticated, "unauthenticated")
	}

	// 1. First message must be metadata
	req, err := stream.Recv()
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "failed to receive metadata: %v", err)
	}
	meta := req.GetMetadata()
	if meta == nil {
		return status.Error(codes.InvalidArgument, "first message must be metadata")
	}

	// 2. Permission check
	bitmask, err := s.dbManager.GetUserEffectivePermissions(claims.UserID, meta.FolderId)
	if err != nil {
		return status.Errorf(codes.Internal, "permission check failed: %v", err)
	}
	if !permission.PermissionBitMask(bitmask).Has(permission.PermCreate) {
		return status.Error(codes.PermissionDenied, "missing PermCreate on folder")
	}

	// 3. Prevent duplicate names in same folder
	if _, err := s.dbManager.GetFileByName(meta.FolderId, meta.Name); err == nil {
		return status.Errorf(codes.AlreadyExists, "file '%s' already exists in folder %d", meta.Name, meta.FolderId)
	}

	// 4. Stream to CAS
	uri, err := s.uploadStreamToCAS(stream.Context(), func() ([]byte, error) {
		r, err := stream.Recv()
		if err != nil {
			return nil, err
		}
		return r.GetChunk(), nil
	})
	if err != nil {
		return status.Errorf(codes.Internal, "storage error: %v", err)
	}

	// 5. DB Updates (File and First Version)
	// Get folder path for the file record
	folder, err := s.dbManager.GetFolder(meta.FolderId)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to get folder path: %v", err)
	}
	filePath := fmt.Sprintf("%s%d/", folder.Path, folder.FolderID)

	fileID, _, err := s.dbManager.CreateFile(meta.Name, meta.FolderId, filePath, uri, s.defaultScheme, meta.Metadata)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to create file record: %v", err)
	}

	// Extract hash from URI (e.g. "local:///hash")
	parts := strings.Split(uri, "/")
	hashStr := parts[len(parts)-1]
	hash, _ := hex.DecodeString(hashStr)

	comment := meta.VersionComment
	if comment == "" {
		comment = "Initial upload"
	}

	versionID, err := s.dbManager.CreateVersion(fileID, "v1", hash, claims.UserID, meta.Metadata, comment)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to create version record: %v", err)
	}

	if err := s.dbManager.UpdateFileVersion(fileID, versionID, uri, s.defaultScheme); err != nil {
		return status.Errorf(codes.Internal, "failed to link file to version: %v", err)
	}

	return stream.SendAndClose(&gen.FileResponse{
		File: &gen.File{
			FileId:      fileID,
			Name:        meta.Name,
			FolderId:    meta.FolderId,
			Path:        filePath,
			StoragePath: uri,
			Location:    s.defaultScheme,
			VersionId:   versionID,
		},
		VersionId: versionID,
	})
}

func (s *FileServiceServer) UploadVersion(stream gen.FileService_UploadVersionServer) error {
	claims, err := interceptors.GetTokenClaims(stream.Context())
	if err != nil {
		return status.Error(codes.Unauthenticated, "unauthenticated")
	}

	// 1. First message must be metadata
	req, err := stream.Recv()
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "failed to receive metadata: %v", err)
	}
	meta := req.GetMetadata()
	if meta == nil {
		return status.Error(codes.InvalidArgument, "first message must be metadata")
	}

	// 2. Fetch file and check permissions
	file, err := s.dbManager.GetFile(meta.FileId)
	if err != nil {
		return status.Errorf(codes.NotFound, "file %d not found", meta.FileId)
	}

	bitmask, err := s.dbManager.GetUserEffectivePermissions(claims.UserID, file.FolderID)
	if err != nil {
		return status.Errorf(codes.Internal, "permission check failed: %v", err)
	}
	if !permission.PermissionBitMask(bitmask).Has(permission.PermWrite) {
		return status.Error(codes.PermissionDenied, "missing PermWrite on folder")
	}

	// 3. Stream to CAS
	uri, err := s.uploadStreamToCAS(stream.Context(), func() ([]byte, error) {
		r, err := stream.Recv()
		if err != nil {
			return nil, err
		}
		return r.GetChunk(), nil
	})
	if err != nil {
		return status.Errorf(codes.Internal, "storage error: %v", err)
	}

	// 4. DB Updates
	parts := strings.Split(uri, "/")
	hashStr := parts[len(parts)-1]
	hash, _ := hex.DecodeString(hashStr)

	// Determine version name (vN)
	nextVer := 1
	versions, _ := s.dbManager.ListVersions(file.FileID)
	if len(versions) > 0 {
		maxVer := 0
		for _, v := range versions {
			var verNum int
			if _, err := fmt.Sscanf(v.Version, "v%d", &verNum); err == nil {
				if verNum > maxVer {
					maxVer = verNum
				}
			}
		}
		nextVer = maxVer + 1
	}
	versionName := fmt.Sprintf("v%d", nextVer)

	versionID, err := s.dbManager.CreateVersion(file.FileID, versionName, hash, claims.UserID, meta.Metadata, meta.Comment)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to create version record: %v", err)
	}

	if err := s.dbManager.UpdateFileVersion(file.FileID, versionID, uri, s.defaultScheme); err != nil {
		return status.Errorf(codes.Internal, "failed to link file to version: %v", err)
	}

	// 5. Retention Policy Enforcement
	retention, _ := s.dbManager.GetVersionRetention(file.FolderID)
	if retention > 0 {
		s.dbManager.DeleteOldVersions(file.FileID, retention)
	}

	return stream.SendAndClose(&gen.FileResponse{
		File: &gen.File{
			FileId:      file.FileID,
			Name:        file.Name,
			FolderId:    file.FolderID,
			Path:        file.Path,
			StoragePath: uri,
			Location:    s.defaultScheme,
			VersionId:   versionID,
		},
		VersionId: versionID,
	})
}

func (s *FileServiceServer) ModifyFile(ctx context.Context, req *gen.ModifyFileRequest) (*gen.FileResponse, error) {
	claims, err := interceptors.GetTokenClaims(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}

	file, err := s.dbManager.GetFile(req.FileId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "file not found")
	}

	bitmask, err := s.dbManager.GetUserEffectivePermissions(claims.UserID, file.FolderID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "permission check failed")
	}
	perms := permission.PermissionBitMask(bitmask)

	// Renaming/Moving requires specific permissions
	if req.Name != nil || req.FolderId != nil {
		if !perms.Has(permission.PermRename) {
			return nil, status.Error(codes.PermissionDenied, "missing PermRename")
		}
	} else if len(req.Metadata) > 0 {
		if !perms.Has(permission.PermWrite) {
			return nil, status.Error(codes.PermissionDenied, "missing PermWrite for metadata update")
		}
	}

	// Apply updates
	if req.Name != nil {
		if err := s.dbManager.RenameFile(req.FileId, *req.Name); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to rename file: %v", err)
		}
	}

	if req.FolderId != nil {
		targetFolder, err := s.dbManager.GetFolder(*req.FolderId)
		if err != nil {
			return nil, status.Errorf(codes.NotFound, "target folder not found")
		}
		// Compute new path
		newPath := fmt.Sprintf("%s%d/", targetFolder.Path, targetFolder.FolderID)
		if err := s.dbManager.MoveFile(req.FileId, *req.FolderId, newPath); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to move file: %v", err)
		}
	}

	if len(req.Metadata) > 0 {
		if err := s.dbManager.UpdateFileMetadata(req.FileId, req.Metadata); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to update metadata: %v", err)
		}
	}

	// Fetch updated file
	updatedFile, err := s.dbManager.GetFile(req.FileId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch updated file")
	}

	return &gen.FileResponse{
		File: &gen.File{
			FileId:      updatedFile.FileID,
			Name:        updatedFile.Name,
			FolderId:    updatedFile.FolderID,
			Path:        updatedFile.Path,
			StoragePath: updatedFile.StoragePath,
			Location:    updatedFile.Location,
			VersionId:   updatedFile.VersionID,
			CreatedAt:   updatedFile.CreatedAt,
			UpdatedAt:   updatedFile.UpdatedAt,
		},
		VersionId: updatedFile.VersionID,
	}, nil
}

func (s *FileServiceServer) DownloadFile(req *gen.DownloadFileRequest, stream gen.FileService_DownloadFileServer) error {
	claims, err := interceptors.GetTokenClaims(stream.Context())
	if err != nil {
		return status.Error(codes.Unauthenticated, "unauthenticated")
	}

	file, err := s.dbManager.GetFile(req.FileId)
	if err != nil {
		return status.Errorf(codes.NotFound, "file not found")
	}

	bitmask, err := s.dbManager.GetUserEffectivePermissions(claims.UserID, file.FolderID)
	if err != nil {
		return status.Errorf(codes.Internal, "permission check failed")
	}
	if !permission.PermissionBitMask(bitmask).Has(permission.PermRead) {
		return status.Error(codes.PermissionDenied, "missing PermRead")
	}

	uri := file.StoragePath
	if req.VersionId != 0 {
		// Fetch specific version URI
		versions, err := s.dbManager.ListVersions(file.FileID)
		if err != nil {
			return status.Errorf(codes.Internal, "failed to list versions")
		}
		found := false
		for _, v := range versions {
			if v.VersionID == req.VersionId {
				uri = fmt.Sprintf("%s:///%s", file.Location, hex.EncodeToString(v.Hash))
				found = true
				break
			}
		}
		if !found {
			return status.Errorf(codes.NotFound, "version %d not found", req.VersionId)
		}
	}

	reader, err := s.storageRegistry.RetrieveByURI(stream.Context(), uri)
	if err != nil {
		return status.Errorf(codes.Internal, "storage error: %v", err)
	}
	defer reader.Close()

	buf := make([]byte, 32*1024)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			if err := stream.Send(&gen.DownloadFileResponse{Chunk: buf[:n]}); err != nil {
				return err
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *FileServiceServer) ListVersions(ctx context.Context, req *gen.ListVersionsRequest) (*gen.ListVersionsResponse, error) {
	claims, err := interceptors.GetTokenClaims(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}

	file, err := s.dbManager.GetFile(req.FileId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "file not found")
	}

	bitmask, err := s.dbManager.GetUserEffectivePermissions(claims.UserID, file.FolderID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "permission check failed")
	}
	if !permission.PermissionBitMask(bitmask).Has(permission.PermViewHistory) {
		return nil, status.Error(codes.PermissionDenied, "missing PermViewHistory")
	}

	dbVersions, err := s.dbManager.ListVersions(req.FileId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list versions")
	}

	respVersions := make([]*gen.FileVersion, len(dbVersions))
	for i, v := range dbVersions {
		respVersions[i] = &gen.FileVersion{
			VersionId:   v.VersionID,
			FileId:      v.FileID,
			VersionName: v.Version,
			Hash:        v.Hash,
			UserId:      v.UserID,
			Metadata:    v.Metadata,
			Comment:     v.VersionComment,
			CreatedAt:   v.CreatedAt,
		}
	}
	return &gen.ListVersionsResponse{Versions: respVersions}, nil
}

func (s *FileServiceServer) SetRetentionPolicy(ctx context.Context, req *gen.SetRetentionPolicyRequest) (*gen.SetRetentionPolicyResponse, error) {
	claims, err := interceptors.GetTokenClaims(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}

	bitmask, err := s.dbManager.GetUserEffectivePermissions(claims.UserID, req.FolderId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "permission check failed")
	}
	if !permission.PermissionBitMask(bitmask).Has(permission.PermAdmin) {
		return nil, status.Error(codes.PermissionDenied, "missing PermAdmin")
	}

	if err := s.dbManager.SetRetentionPolicy(req.FolderId, req.LastNVersions); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update policy")
	}

	return &gen.SetRetentionPolicyResponse{Success: true}, nil
}

func (s *FileServiceServer) DeleteFile(ctx context.Context, req *gen.DeleteFileRequest) (*gen.DeleteFileResponse, error) {
	claims, err := interceptors.GetTokenClaims(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}

	file, err := s.dbManager.GetFile(req.FileId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "file not found")
	}

	bitmask, err := s.dbManager.GetUserEffectivePermissions(claims.UserID, file.FolderID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "permission check failed")
	}
	if !permission.PermissionBitMask(bitmask).Has(permission.PermDelete) {
		return nil, status.Error(codes.PermissionDenied, "missing PermDelete")
	}

	if err := s.dbManager.SoftDeleteFile(req.FileId); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to soft-delete file: %v", err)
	}

	return &gen.DeleteFileResponse{Success: true}, nil
}
