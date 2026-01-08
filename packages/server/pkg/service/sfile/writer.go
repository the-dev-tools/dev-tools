package sfile

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mfile"
)

type Writer struct {
	queries *gen.Queries
	logger  *slog.Logger
	// Need reader logic for some checks, can embed or compose
	reader *Reader
}

func NewWriter(tx gen.DBTX, logger *slog.Logger) *Writer {
	if logger == nil {
		logger = slog.Default()
	}
	queries := gen.New(tx)
	// We need a reader that can operate on the SAME transaction for consistency checks
	// Since gen.DBTX is satisfied by *sql.Tx, we can create a temporary reader using it
	// Note: Reader expects *sql.DB for NewReader, but we can bypass that if we expose a constructor accepting gen.Queries
	// Or we can just duplicate the few read methods we need in private helpers or rely on the fact that
	// if tx is a *sql.Tx, we can't easily convert it to *sql.DB without interface trickery.
	// Best approach: Reader should accept an interface, or we just pass the queries directly.
	// For now, I'll instantiate a Reader using the queries derived from the TX.
	reader := NewReaderFromQueries(queries, logger)

	return &Writer{
		queries: queries,
		logger:  logger,
		reader:  reader,
	}
}

func NewWriterFromQueries(queries *gen.Queries, logger *slog.Logger) *Writer {
	if logger == nil {
		logger = slog.Default()
	}
	reader := NewReaderFromQueries(queries, logger)
	return &Writer{
		queries: queries,
		logger:  logger,
		reader:  reader,
	}
}

func (w *Writer) CreateFile(ctx context.Context, file *mfile.File) error {
	w.logger.Debug("Creating file",
		"file_id", file.ID.String(),
		"workspace_id", file.WorkspaceID.String(),
		"name", file.Name,
		"content_kind", file.ContentType.String())

	if err := file.Validate(); err != nil {
		return fmt.Errorf("file validation failed: %w", err)
	}

	if file.Order == 0 {
		nextOrder, err := w.NextDisplayOrder(ctx, file.WorkspaceID, file.ParentID)
		if err != nil {
			return fmt.Errorf("failed to get next display order: %w", err)
		}
		file.Order = nextOrder
	}

	file.UpdatedAt = time.Now()

	dbFile := ConvertToDBFile(*file)
	err := w.queries.CreateFile(ctx, gen.CreateFileParams(dbFile))
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}

	w.logger.Debug("Successfully created file",
		"file_id", file.ID.String(),
		"name", file.Name)

	return nil
}

func (w *Writer) UpsertFile(ctx context.Context, file *mfile.File) error {
	w.logger.Debug("Upserting file",
		"file_id", file.ID.String(),
		"workspace_id", file.WorkspaceID.String())

	// Use internal reader to check existence within the same transaction context
	_, err := w.reader.GetFile(ctx, file.ID)
	if err != nil {
		if errors.Is(err, ErrFileNotFound) {
			return w.CreateFile(ctx, file)
		}
		return fmt.Errorf("failed to check file existence: %w", err)
	}

	return w.UpdateFile(ctx, file)
}

func (w *Writer) UpdateFile(ctx context.Context, file *mfile.File) error {
	w.logger.Debug("Updating file",
		"file_id", file.ID.String(),
		"name", file.Name,
		"content_kind", file.ContentType.String())

	if err := file.Validate(); err != nil {
		return fmt.Errorf("file validation failed: %w", err)
	}

	if file.Order == 0 {
		current, err := w.queries.GetFile(ctx, file.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrFileNotFound
			}
			return fmt.Errorf("failed to get current file: %w", err)
		}
		file.Order = current.DisplayOrder
	}

	file.UpdatedAt = time.Now()

	dbFile := ConvertToDBFile(*file)
	err := w.queries.UpdateFile(ctx, gen.UpdateFileParams{
		WorkspaceID:  dbFile.WorkspaceID,
		ParentID:     dbFile.ParentID,
		ContentID:    dbFile.ContentID,
		ContentKind:  dbFile.ContentKind,
		Name:         dbFile.Name,
		DisplayOrder: dbFile.DisplayOrder,
		PathHash:     dbFile.PathHash,
		UpdatedAt:    dbFile.UpdatedAt,
		ID:           dbFile.ID,
	})
	if err != nil {
		return fmt.Errorf("failed to update file: %w", err)
	}

	w.logger.Debug("Successfully updated file",
		"file_id", file.ID.String(),
		"name", file.Name)

	return nil
}

func (w *Writer) DeleteFile(ctx context.Context, id idwrap.IDWrap) error {
	w.logger.Debug("Deleting file", "file_id", id.String())

	if err := w.queries.DeleteFile(ctx, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrFileNotFound
		}
		return fmt.Errorf("failed to delete file: %w", err)
	}

	w.logger.Debug("Successfully deleted file", "file_id", id.String())
	return nil
}

func (w *Writer) NextDisplayOrder(ctx context.Context, workspaceID idwrap.IDWrap, parentID *idwrap.IDWrap) (float64, error) {
	var files []gen.File
	var err error

	if parentID == nil {
		files, err = w.queries.GetFilesByWorkspaceID(ctx, workspaceID)
	} else {
		files, err = w.queries.GetFilesByParentID(ctx, parentID)
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

func (w *Writer) MoveFile(ctx context.Context, fileID idwrap.IDWrap, newParentID *idwrap.IDWrap) error {
	w.logger.Debug("Moving file",
		"file_id", fileID.String(),
		"new_parent_id", getIDString(newParentID))

	file, err := w.reader.GetFile(ctx, fileID)
	if err != nil {
		return err
	}

	if newParentID != nil && file.IsFolder() {
		if fileID.Compare(*newParentID) == 0 {
			return ErrFolderIntoItself
		}

		isDescendant, err := w.reader.isDescendant(ctx, *newParentID, fileID)
		if err != nil {
			return fmt.Errorf("failed to check for cycles: %w", err)
		}
		if isDescendant {
			return fmt.Errorf("cannot move folder into its own descendant")
		}
	}

	if newParentID != nil {
		newParentWorkspaceID, err := w.reader.GetWorkspaceID(ctx, *newParentID)
		if err != nil {
			return fmt.Errorf("failed to get new parent workspace ID: %w", err)
		}
		if newParentWorkspaceID.Compare(file.WorkspaceID) != 0 {
			return ErrWorkspaceMismatch
		}
	}

	file.ParentID = newParentID
	file.UpdatedAt = time.Now()
	return w.UpdateFile(ctx, file)
}
