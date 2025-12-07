package sfile

import (
	"context"
	"database/sql"
	"errors"
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
		ParentID:     file.ParentID,
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
		ParentID:    file.ParentID,
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
		if errors.Is(err, sql.ErrNoRows) {
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
		if errors.Is(err, sql.ErrNoRows) {
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

// ListFilesByParent retrieves files directly under a parent
func (s *FileService) ListFilesByParent(ctx context.Context, workspaceID idwrap.IDWrap, parentID *idwrap.IDWrap) ([]mfile.File, error) {
	s.logger.Debug("Listing files by parent",
		"workspace_id", workspaceID.String(),
		"parent_id", getIDString(parentID))

	var files []gen.File
	var err error

	if parentID == nil {
		// Get root-level files
		files, err = s.queries.GetRootFilesByWorkspaceID(ctx, workspaceID)
	} else {
		// Get files in specific parent
		files, err = s.queries.GetFilesByParentIDOrdered(ctx, parentID)
	}

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mfile.File{}, nil
		}
		return nil, fmt.Errorf("failed to list files by parent: %w", err)
	}

	result := make([]mfile.File, len(files))
	for i, file := range files {
		converted := ConvertToModelFile(file)
		if converted != nil {
			result[i] = *converted
		}
	}

	s.logger.Debug("Successfully listed files by parent",
		"workspace_id", workspaceID.String(),
		"parent_id", getIDString(parentID),
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
		nextOrder, err := s.NextDisplayOrder(ctx, file.WorkspaceID, file.ParentID)
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
		ParentID:     dbFile.ParentID,
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

// UpsertFile creates or updates a file
func (s *FileService) UpsertFile(ctx context.Context, file *mfile.File) error {
	s.logger.Debug("Upserting file",
		"file_id", file.ID.String(),
		"workspace_id", file.WorkspaceID.String())

	// Check if file exists
	_, err := s.GetFile(ctx, file.ID)
	if err != nil {
		if errors.Is(err, ErrFileNotFound) {
			// File doesn't exist, create it
			return s.CreateFile(ctx, file)
		}
		// Other error
		return fmt.Errorf("failed to check file existence: %w", err)
	}

	// File exists, update it
	return s.UpdateFile(ctx, file)
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
			if errors.Is(err, sql.ErrNoRows) {
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
		ParentID:     dbFile.ParentID,
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
		if errors.Is(err, sql.ErrNoRows) {
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
		if errors.Is(err, sql.ErrNoRows) {
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

// NextDisplayOrder calculates the next order value for a file in a workspace/parent
func (s *FileService) NextDisplayOrder(ctx context.Context, workspaceID idwrap.IDWrap, parentID *idwrap.IDWrap) (float64, error) {
	var files []gen.File
	var err error

	if parentID == nil {
		files, err = s.queries.GetFilesByWorkspaceID(ctx, workspaceID)
	} else {
		files, err = s.queries.GetFilesByParentID(ctx, parentID)
	}

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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

// MoveFile moves a file to a different parent
func (s *FileService) MoveFile(ctx context.Context, fileID idwrap.IDWrap, newParentID *idwrap.IDWrap) error {
	s.logger.Debug("Moving file",
		"file_id", fileID.String(),
		"new_parent_id", getIDString(newParentID))

	file, err := s.GetFile(ctx, fileID)
	if err != nil {
		return err
	}

	// Prevent moving a folder into itself
	if newParentID != nil && file.IsFolder() {
		if fileID.Compare(*newParentID) == 0 {
			return ErrFolderIntoItself
		}
		// TODO: Add cycle detection if needed, but for now just direct parent check
	}

	// Validate workspace consistency if moving to a different parent
	if newParentID != nil {
		newParentWorkspaceID, err := s.GetWorkspaceID(ctx, *newParentID)
		if err != nil {
			return fmt.Errorf("failed to get new parent workspace ID: %w", err)
		}
		if newParentWorkspaceID.Compare(file.WorkspaceID) != 0 {
			return ErrWorkspaceMismatch
		}
	}

	file.ParentID = newParentID
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
