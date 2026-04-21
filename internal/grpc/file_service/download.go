package file_service

import (
	"encoding/hex"
	"fmt"
	"io"

	"mandala-workspace/gen"
	"mandala-workspace/internal/grpc/interceptors"
	"mandala-workspace/internal/permission"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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
