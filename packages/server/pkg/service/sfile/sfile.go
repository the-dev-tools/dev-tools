package sfile

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mfile"
)

// FileService provides operations for managing files in the unified file system
// Supports the union type content pattern with two-query approach for content retrieval
type FileService struct {
	queries *gen.Queries
	logger  *slog.Logger
}

var (
	ErrFileNotFound       = fmt.Errorf("file not found")
	ErrContentNotFound    = fmt.Errorf("content not found")
	ErrInvalidContentKind = fmt.Errorf("invalid content kind")
	ErrFolderIntoItself   = fmt.Errorf("cannot move folder into itself")
	ErrWorkspaceMismatch  = fmt.Errorf("workspace mismatch")
)

// New creates a new FileService
func New(queries *gen.Queries, logger *slog.Logger) *FileService {
	if logger == nil {
		logger = slog.Default()
	}
	return &FileService{
		queries: queries,
		logger:  logger,
	}
}

// TX returns a new service instance with transaction support
func (s *FileService) TX(tx *sql.Tx) *FileService {
	if tx == nil {
		return s
	}
	return &FileService{
		queries: s.queries.WithTx(tx),
		logger:  s.logger,
	}
}

// NewTX creates a new service instance with prepared transaction queries
func NewTX(ctx context.Context, tx *sql.Tx, logger *slog.Logger) (*FileService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare transaction queries: %w", err)
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &FileService{
		queries: queries,
		logger:  logger,
	}, nil
}

// ConvertToDBFile converts model to DB representation
func ConvertToDBFile(file mfile.File) gen.File {
	return gen.File{
		ID:           file.ID,
		WorkspaceID:  file.WorkspaceID,
		FolderID:     file.FolderID,
		ContentID:    file.ContentID,
		ContentKind:  int8(file.ContentType),
		Name:         file.Name,
		DisplayOrder: file.Order,
		UpdatedAt:    file.UpdatedAt.Unix(),
	}
}

// ConvertToModelFile converts DB to model representation
func ConvertToModelFile(file gen.File) *mfile.File {
	return &mfile.File{
		ID:          file.ID,
		WorkspaceID: file.WorkspaceID,
		FolderID:    file.FolderID,
		ContentID:   file.ContentID,
		ContentType: mfile.ContentType(file.ContentKind),
		Name:        file.Name,
		Order:       file.DisplayOrder,
		UpdatedAt:   time.Unix(file.UpdatedAt, 0),
	}
}

// GetFile retrieves a single file by ID (metadata only)
func (s *FileService) GetFile(ctx context.Context, id idwrap.IDWrap) (*mfile.File, error) {
	s.logger.Debug("Getting file", "file_id", id.String())

	file, err := s.queries.GetFile(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			s.logger.Debug("File not found", "file_id", id.String())
			return nil, ErrFileNotFound
		}
		return nil, fmt.Errorf("failed to get file: %w", err)
	}

	return ConvertToModelFile(file), nil
}

// GetFileContentID retrieves the content ID for a file
func (s *FileService) GetFileContentID(ctx context.Context, id idwrap.IDWrap) (*idwrap.IDWrap, mfile.ContentType, error) {
	s.logger.Debug("Getting file content ID", "file_id", id.String())

	file, err := s.GetFile(ctx, id)
	if err != nil {
		return nil, mfile.ContentTypeUnknown, err
	}

	if !file.HasContent() {
		return nil, mfile.ContentTypeUnknown, fmt.Errorf("file has no content")
	}

	return file.ContentID, file.ContentType, nil
}

// ListFilesByWorkspace retrieves all files for a workspace
func (s *FileService) ListFilesByWorkspace(ctx context.Context, workspaceID idwrap.IDWrap) ([]mfile.File, error) {
	s.logger.Debug("Listing files by workspace", "workspace_id", workspaceID.String())

	files, err := s.queries.GetFilesByWorkspaceIDOrdered(ctx, workspaceID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mfile.File{}, nil
		}
		return nil, fmt.Errorf("failed to list files by workspace: %w", err)
	}

	result := make([]mfile.File, len(files))
	for i, file := range files {
		converted := ConvertToModelFile(file)
		if converted != nil {
			result[i] = *converted
		}
	}

	s.logger.Debug("Successfully listed files by workspace",
		"workspace_id", workspaceID.String(),
		"count", len(result))

	return result, nil
}

// ListFilesByFolder retrieves files directly under a folder
func (s *FileService) ListFilesByFolder(ctx context.Context, workspaceID idwrap.IDWrap, folderID *idwrap.IDWrap) ([]mfile.File, error) {
	s.logger.Debug("Listing files by folder",
		"workspace_id", workspaceID.String(),
		"folder_id", getIDString(folderID))

	var files []gen.File
	var err error

	if folderID == nil {
		// Get root-level files
		files, err = s.queries.GetRootFilesByWorkspaceID(ctx, workspaceID)
	} else {
		// Get files in specific folder
		files, err = s.queries.GetFilesByFolderIDOrdered(ctx, folderID)
	}

	if err != nil {
		if err == sql.ErrNoRows {
			return []mfile.File{}, nil
		}
		return nil, fmt.Errorf("failed to list files by folder: %w", err)
	}

	result := make([]mfile.File, len(files))
	for i, file := range files {
		converted := ConvertToModelFile(file)
		if converted != nil {
			result[i] = *converted
		}
	}

	s.logger.Debug("Successfully listed files by folder",
		"workspace_id", workspaceID.String(),
		"folder_id", getIDString(folderID),
		"count", len(result))

	return result, nil
}

// CreateFile creates a new file
func (s *FileService) CreateFile(ctx context.Context, file *mfile.File) error {
	s.logger.Debug("Creating file",
		"file_id", file.ID.String(),
		"workspace_id", file.WorkspaceID.String(),
		"name", file.Name,
		"content_kind", file.ContentType.String())

	// Validate file
	if err := file.Validate(); err != nil {
		return fmt.Errorf("file validation failed: %w", err)
	}

	// Auto-assign order if not provided
	if file.Order == 0 {
		nextOrder, err := s.NextDisplayOrder(ctx, file.WorkspaceID, file.FolderID)
		if err != nil {
			return fmt.Errorf("failed to get next display order: %w", err)
		}
		file.Order = nextOrder
	}

	// Set updated_at timestamp
	file.UpdatedAt = time.Now()

	dbFile := ConvertToDBFile(*file)
	err := s.queries.CreateFile(ctx, gen.CreateFileParams{
		ID:           dbFile.ID,
		WorkspaceID:  dbFile.WorkspaceID,
		FolderID:     dbFile.FolderID,
		ContentID:    dbFile.ContentID,
		ContentKind:  dbFile.ContentKind,
		Name:         dbFile.Name,
		DisplayOrder: dbFile.DisplayOrder,
		UpdatedAt:    dbFile.UpdatedAt,
	})
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}

	s.logger.Debug("Successfully created file",
		"file_id", file.ID.String(),
		"name", file.Name)

	return nil
}


// UpdateFile updates an existing file
func (s *FileService) UpdateFile(ctx context.Context, file *mfile.File) error {
	s.logger.Debug("Updating file",
		"file_id", file.ID.String(),
		"name", file.Name,
		"content_kind", file.ContentType.String())

	// Validate file
	if err := file.Validate(); err != nil {
		return fmt.Errorf("file validation failed: %w", err)
	}

	// Preserve order if not provided
	if file.Order == 0 {
		current, err := s.queries.GetFile(ctx, file.ID)
		if err != nil {
			if err == sql.ErrNoRows {
				return ErrFileNotFound
			}
			return fmt.Errorf("failed to get current file: %w", err)
		}
		file.Order = current.DisplayOrder
	}

	// Set updated_at timestamp
	file.UpdatedAt = time.Now()

	dbFile := ConvertToDBFile(*file)
	err := s.queries.UpdateFile(ctx, gen.UpdateFileParams{
		WorkspaceID:  dbFile.WorkspaceID,
		FolderID:     dbFile.FolderID,
		ContentID:    dbFile.ContentID,
		ContentKind:  dbFile.ContentKind,
		Name:         dbFile.Name,
		DisplayOrder: dbFile.DisplayOrder,
		UpdatedAt:    dbFile.UpdatedAt,
		ID:           dbFile.ID,
	})
	if err != nil {
		return fmt.Errorf("failed to update file: %w", err)
	}

	s.logger.Debug("Successfully updated file",
		"file_id", file.ID.String(),
		"name", file.Name)

	return nil
}

// DeleteFile deletes a file
func (s *FileService) DeleteFile(ctx context.Context, id idwrap.IDWrap) error {
	s.logger.Debug("Deleting file", "file_id", id.String())

	if err := s.queries.DeleteFile(ctx, id); err != nil {
		if err == sql.ErrNoRows {
			return ErrFileNotFound
		}
		return fmt.Errorf("failed to delete file: %w", err)
	}

	s.logger.Debug("Successfully deleted file", "file_id", id.String())
	return nil
}

// GetWorkspaceID retrieves the workspace ID for a file
func (s *FileService) GetWorkspaceID(ctx context.Context, fileID idwrap.IDWrap) (idwrap.IDWrap, error) {
	s.logger.Debug("Getting workspace ID for file", "file_id", fileID.String())

	workspaceID, err := s.queries.GetFileWorkspaceID(ctx, fileID)
	if err != nil {
		if err == sql.ErrNoRows {
			return idwrap.IDWrap{}, ErrFileNotFound
		}
		return idwrap.IDWrap{}, fmt.Errorf("failed to get file workspace ID: %w", err)
	}

	return workspaceID, nil
}

// CheckWorkspaceID verifies if a file belongs to a specific workspace
func (s *FileService) CheckWorkspaceID(ctx context.Context, fileID, workspaceID idwrap.IDWrap) (bool, error) {
	fileWorkspaceID, err := s.GetWorkspaceID(ctx, fileID)
	if err != nil {
		return false, err
	}
	return fileWorkspaceID.Compare(workspaceID) == 0, nil
}

// NextDisplayOrder calculates the next order value for a file in a workspace/folder
func (s *FileService) NextDisplayOrder(ctx context.Context, workspaceID idwrap.IDWrap, folderID *idwrap.IDWrap) (float64, error) {
	var files []gen.File
	var err error

	if folderID == nil {
		files, err = s.queries.GetFilesByWorkspaceID(ctx, workspaceID)
	} else {
		files, err = s.queries.GetFilesByFolderID(ctx, folderID)
	}

	if err != nil {
		if err == sql.ErrNoRows {
			return 1, nil
		}
		return 0, fmt.Errorf("failed to get files for order calculation: %w", err)
	}

	max := 0.0
	for _, file := range files {
		if file.DisplayOrder > max {
			max = file.DisplayOrder
		}
	}
	return max + 1, nil
}

// MoveFile moves a file to a different folder
func (s *FileService) MoveFile(ctx context.Context, fileID idwrap.IDWrap, newFolderID *idwrap.IDWrap) error {
	s.logger.Debug("Moving file",
		"file_id", fileID.String(),
		"new_folder_id", getIDString(newFolderID))

	file, err := s.GetFile(ctx, fileID)
	if err != nil {
		return err
	}

	// Prevent moving a folder into itself
	if newFolderID != nil && file.IsFolder() {
		if fileID.Compare(*newFolderID) == 0 {
			return ErrFolderIntoItself
		}
	}

	// Validate workspace consistency if moving to a different folder
	if newFolderID != nil {
		newParentWorkspaceID, err := s.GetWorkspaceID(ctx, *newFolderID)
		if err != nil {
			return fmt.Errorf("failed to get new parent folder workspace ID: %w", err)
		}
		if newParentWorkspaceID.Compare(file.WorkspaceID) != 0 {
			return ErrWorkspaceMismatch
		}
	}

	file.FolderID = newFolderID
	file.UpdatedAt = time.Now()
	return s.UpdateFile(ctx, file)
}

// Helper functions

func getIDString(id *idwrap.IDWrap) string {
	if id == nil {
		return "nil"
	}
	return id.String()
}
