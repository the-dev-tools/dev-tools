//nolint:revive // exported
package sfile

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mfile"
)

// FileService provides operations for managing files in the unified file system
// Supports the union type content pattern with two-query approach for content retrieval
type FileService struct {
	reader  *Reader
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
		reader:  NewReaderFromQueries(queries, logger),
		queries: queries,
		logger:  logger,
	}
}

// TX returns a new service instance with transaction support
func (s *FileService) TX(tx *sql.Tx) *FileService {
	if tx == nil {
		return s
	}
	newQueries := s.queries.WithTx(tx)
	return &FileService{
		reader:  NewReaderFromQueries(newQueries, s.logger),
		queries: newQueries,
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
		reader:  NewReaderFromQueries(queries, logger),
		queries: queries,
		logger:  logger,
	}, nil
}

// GetFile retrieves a single file by ID (metadata only)
func (s *FileService) GetFile(ctx context.Context, id idwrap.IDWrap) (*mfile.File, error) {
	return s.reader.GetFile(ctx, id)
}

// GetFileContentID retrieves the content ID for a file
func (s *FileService) GetFileContentID(ctx context.Context, id idwrap.IDWrap) (*idwrap.IDWrap, mfile.ContentType, error) {
	return s.reader.GetFileContentID(ctx, id)
}

// ListFilesByWorkspace retrieves all files for a workspace
func (s *FileService) ListFilesByWorkspace(ctx context.Context, workspaceID idwrap.IDWrap) ([]mfile.File, error) {
	return s.reader.ListFilesByWorkspace(ctx, workspaceID)
}

// ListFilesByParent retrieves files directly under a parent
func (s *FileService) ListFilesByParent(ctx context.Context, workspaceID idwrap.IDWrap, parentID *idwrap.IDWrap) ([]mfile.File, error) {
	return s.reader.ListFilesByParent(ctx, workspaceID, parentID)
}

// CreateFile creates a new file
func (s *FileService) CreateFile(ctx context.Context, file *mfile.File) error {
	return NewWriterFromQueries(s.queries, s.logger).CreateFile(ctx, file)
}

// UpsertFile creates or updates a file
func (s *FileService) UpsertFile(ctx context.Context, file *mfile.File) error {
	return NewWriterFromQueries(s.queries, s.logger).UpsertFile(ctx, file)
}

// UpdateFile updates an existing file
func (s *FileService) UpdateFile(ctx context.Context, file *mfile.File) error {
	return NewWriterFromQueries(s.queries, s.logger).UpdateFile(ctx, file)
}

// DeleteFile deletes a file
func (s *FileService) DeleteFile(ctx context.Context, id idwrap.IDWrap) error {
	return NewWriterFromQueries(s.queries, s.logger).DeleteFile(ctx, id)
}

// GetWorkspaceID retrieves the workspace ID for a file
func (s *FileService) GetWorkspaceID(ctx context.Context, fileID idwrap.IDWrap) (idwrap.IDWrap, error) {
	return s.reader.GetWorkspaceID(ctx, fileID)
}

// CheckWorkspaceID verifies if a file belongs to a specific workspace
func (s *FileService) CheckWorkspaceID(ctx context.Context, fileID, workspaceID idwrap.IDWrap) (bool, error) {
	return s.reader.CheckWorkspaceID(ctx, fileID, workspaceID)
}

// NextDisplayOrder calculates the next order value for a file in a workspace/parent
func (s *FileService) NextDisplayOrder(ctx context.Context, workspaceID idwrap.IDWrap, parentID *idwrap.IDWrap) (float64, error) {
	return NewWriterFromQueries(s.queries, s.logger).NextDisplayOrder(ctx, workspaceID, parentID)
}

// MoveFile moves a file to a different parent
func (s *FileService) MoveFile(ctx context.Context, fileID idwrap.IDWrap, newParentID *idwrap.IDWrap) error {
	return NewWriterFromQueries(s.queries, s.logger).MoveFile(ctx, fileID, newParentID)
}

func (s *FileService) Reader() *Reader {
	return s.reader
}

// Helper functions

func getIDString(id *idwrap.IDWrap) string {
	if id == nil {
		return "nil"
	}
	return id.String()
}
