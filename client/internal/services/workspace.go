package services

import (
	"context"
	"fmt"

	"mandala-workspace/client/gen"
	"mandala-workspace/client/pkg/logger"
)

type WorkspaceService struct {
	folderClient gen.FolderServiceClient
}

func NewWorkspaceService(folderClient gen.FolderServiceClient) *WorkspaceService {
	return &WorkspaceService{
		folderClient: folderClient,
	}
}

// ListFolder returns the contents of a folder.
func (s *WorkspaceService) ListFolder(folderID uint64) (*gen.ListFolderResponse, error) {
	logger.Debug("Listing folder", "folder_id", folderID)
	resp, err := s.folderClient.ListFolder(context.Background(), &gen.ListFolderRequest{
		FolderId: folderID,
	})
	if err != nil {
		logger.Error("Failed to list folder", "folder_id", folderID, "error", err)
		return nil, fmt.Errorf("failed to list folder: %w", err)
	}
	return resp, nil
}

// CreateFolder creates a new subfolder.
func (s *WorkspaceService) CreateFolder(parentID uint64, name string) (*gen.CreateFolderResponse, error) {
	logger.Info("Creating folder", "parent_id", parentID, "name", name)
	resp, err := s.folderClient.CreateFolder(context.Background(), &gen.CreateFolderRequest{
		ParentFolderId: parentID,
		Name:           name,
		Inheritance:    true,
	})
	if err != nil {
		logger.Error("Failed to create folder", "name", name, "error", err)
		return nil, fmt.Errorf("failed to create folder: %w", err)
	}
	return resp, nil
}

// MoveFolder moves a folder to a new parent.
func (s *WorkspaceService) MoveFolder(folderID uint64, newParentID uint64) error {
	logger.Info("Moving folder", "folder_id", folderID, "new_parent_id", newParentID)
	_, err := s.folderClient.MoveFolder(context.Background(), &gen.MoveFolderRequest{
		FolderId:          folderID,
		NewParentFolderId: newParentID,
	})
	if err != nil {
		logger.Error("Failed to move folder", "folder_id", folderID, "error", err)
		return fmt.Errorf("failed to move folder: %w", err)
	}
	return nil
}

// DeleteFolder deletes a folder and its contents.
func (s *WorkspaceService) DeleteFolder(folderID uint64) error {
	logger.Info("Deleting folder", "folder_id", folderID)
	_, err := s.folderClient.DeleteFolder(context.Background(), &gen.DeleteFolderRequest{
		FolderId: folderID,
	})
	if err != nil {
		logger.Error("Failed to delete folder", "folder_id", folderID, "error", err)
		return fmt.Errorf("failed to delete folder: %w", err)
	}
	return nil
}
