package sfile

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mfile"
)

type Reader struct {
	queries *gen.Queries
	logger  *slog.Logger
}

func NewReader(db *sql.DB, logger *slog.Logger) *Reader {
	if logger == nil {
		logger = slog.Default()
	}
	return &Reader{
		queries: gen.New(db),
		logger:  logger,
	}
}

func NewReaderFromQueries(queries *gen.Queries, logger *slog.Logger) *Reader {
	if logger == nil {
		logger = slog.Default()
	}
	return &Reader{
		queries: queries,
		logger:  logger,
	}
}

func (r *Reader) GetFile(ctx context.Context, id idwrap.IDWrap) (*mfile.File, error) {
	r.logger.Debug("Getting file", "file_id", id.String())

	file, err := r.queries.GetFile(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			r.logger.Debug("File not found", "file_id", id.String())
			return nil, ErrFileNotFound
		}
		return nil, fmt.Errorf("failed to get file: %w", err)
	}

	return ConvertToModelFile(file), nil
}

func (r *Reader) GetFileContentID(ctx context.Context, id idwrap.IDWrap) (*idwrap.IDWrap, mfile.ContentType, error) {
	r.logger.Debug("Getting file content ID", "file_id", id.String())

	file, err := r.GetFile(ctx, id)
	if err != nil {
		return nil, mfile.ContentTypeUnknown, err
	}

	if !file.HasContent() {
		return nil, mfile.ContentTypeUnknown, fmt.Errorf("file has no content")
	}

	return file.ContentID, file.ContentType, nil
}

func (r *Reader) ListFilesByWorkspace(ctx context.Context, workspaceID idwrap.IDWrap) ([]mfile.File, error) {
	r.logger.Debug("Listing files by workspace", "workspace_id", workspaceID.String())

	files, err := r.queries.GetFilesByWorkspaceIDOrdered(ctx, workspaceID)
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

	r.logger.Debug("Successfully listed files by workspace",
		"workspace_id", workspaceID.String(),
		"count", len(result))

	return result, nil
}

func (r *Reader) ListFilesByParent(ctx context.Context, workspaceID idwrap.IDWrap, parentID *idwrap.IDWrap) ([]mfile.File, error) {
	r.logger.Debug("Listing files by parent",
		"workspace_id", workspaceID.String(),
		"parent_id", getIDString(parentID))

	var files []gen.File
	var err error

	if parentID == nil {
		files, err = r.queries.GetRootFilesByWorkspaceID(ctx, workspaceID)
	} else {
		files, err = r.queries.GetFilesByParentIDOrdered(ctx, parentID)
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

	r.logger.Debug("Successfully listed files by parent",
		"workspace_id", workspaceID.String(),
		"parent_id", getIDString(parentID),
		"count", len(result))

	return result, nil
}

func (r *Reader) GetWorkspaceID(ctx context.Context, fileID idwrap.IDWrap) (idwrap.IDWrap, error) {
	r.logger.Debug("Getting workspace ID for file", "file_id", fileID.String())

	workspaceID, err := r.queries.GetFileWorkspaceID(ctx, fileID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return idwrap.IDWrap{}, ErrFileNotFound
		}
		return idwrap.IDWrap{}, fmt.Errorf("failed to get file workspace ID: %w", err)
	}

	return workspaceID, nil
}

func (r *Reader) FindFileByPathHash(ctx context.Context, workspaceID idwrap.IDWrap, pathHash string) (idwrap.IDWrap, error) {
	r.logger.Debug("Finding file by path hash", "workspace_id", workspaceID.String(), "path_hash", pathHash)

	id, err := r.queries.FindFileByPathHash(ctx, gen.FindFileByPathHashParams{
		WorkspaceID: workspaceID,
		PathHash:    sql.NullString{String: pathHash, Valid: true},
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return idwrap.IDWrap{}, ErrFileNotFound
		}
		return idwrap.IDWrap{}, fmt.Errorf("failed to find file by path hash: %w", err)
	}

	return id, nil
}

func (r *Reader) CheckWorkspaceID(ctx context.Context, fileID, workspaceID idwrap.IDWrap) (bool, error) {
	fileWorkspaceID, err := r.GetWorkspaceID(ctx, fileID)
	if err != nil {
		return false, err
	}
	return fileWorkspaceID.Compare(workspaceID) == 0, nil
}

// GetFileByContentID retrieves a file by its content ID
func (r *Reader) GetFileByContentID(ctx context.Context, contentID idwrap.IDWrap) (*mfile.File, error) {
	r.logger.Debug("Getting file by content ID", "content_id", contentID.String())

	file, err := r.queries.GetFileByContentID(ctx, &contentID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			r.logger.Debug("File not found for content ID", "content_id", contentID.String())
			return nil, ErrFileNotFound
		}
		return nil, fmt.Errorf("failed to get file by content ID: %w", err)
	}

	return ConvertToModelFile(file), nil
}

func (r *Reader) isDescendant(ctx context.Context, descendantID, ancestorID idwrap.IDWrap) (bool, error) {
	currentID := descendantID
	// Limit recursion depth to prevent infinite loops in case of existing corruption
	const maxDepth = 100

	for range maxDepth {
		// If current node is the ancestor, then yes it is a descendant (or same node)
		if currentID.Compare(ancestorID) == 0 {
			return true, nil
		}

		file, err := r.GetFile(ctx, currentID)
		if err != nil {
			return false, err
		}

		if file.ParentID == nil {
			return false, nil // Reached root without finding ancestor
		}

		currentID = *file.ParentID
	}

	return false, fmt.Errorf("max depth exceeded while checking hierarchy")
}
