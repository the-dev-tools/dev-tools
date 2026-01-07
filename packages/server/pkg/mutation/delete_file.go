package mutation

import (
	"context"
	"database/sql"
	"errors"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mfile"
)

// FileDeleteItem represents a file to delete with its context.
type FileDeleteItem struct {
	ID          idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
	ContentID   *idwrap.IDWrap
	ContentKind mfile.ContentType
}

// DeleteFile deletes a file and the content it points to (HTTP/Flow).
// File -> points to -> Content, so deleting file cascades DOWN to content.
// This is the PUBLIC entry point for content deletion.
func (c *Context) DeleteFile(ctx context.Context, file FileDeleteItem) error {
	// If it's a folder, we MUST recursively delete children first
	if file.ContentKind == mfile.ContentTypeFolder {
		if err := c.deleteFolderChildren(ctx, file.ID, file.WorkspaceID); err != nil {
			return err
		}
	}

	// Delete content based on content_kind (cascade DOWN - internal methods)
	if file.ContentID != nil {
		switch file.ContentKind {
		case mfile.ContentTypeHTTP:
			// HTTP - cascade to headers, params, etc.
			if err := c.deleteHTTPContent(ctx, *file.ContentID, file.WorkspaceID, false); err != nil {
				return err
			}
		case mfile.ContentTypeHTTPDelta:
			// HTTP Delta - same cascade
			if err := c.deleteHTTPContent(ctx, *file.ContentID, file.WorkspaceID, true); err != nil {
				return err
			}
		case mfile.ContentTypeFlow:
			// Flow - cascade to nodes, edges, variables
			if err := c.deleteFlowContent(ctx, *file.ContentID, file.WorkspaceID); err != nil {
				return err
			}
		case mfile.ContentTypeFolder:
			// Content deletion handled by recursion above (folders don't have separate content tables)
		}
	}

	// Track file delete
	c.track(Event{
		Entity:      EntityFile,
		Op:          OpDelete,
		ID:          file.ID,
		WorkspaceID: file.WorkspaceID,
	})

	// Delete file record
	return c.q.DeleteFile(ctx, file.ID)
}

// DeleteFileBatch deletes multiple files and their content.
func (c *Context) DeleteFileBatch(ctx context.Context, items []FileDeleteItem) error {
	if len(items) == 0 {
		return nil
	}

	// First pass: Handle folders recursively (cannot be easily batched due to depth)
	// We do this one-by-one for simplicity and safety
	for _, item := range items {
		if item.ContentKind == mfile.ContentTypeFolder {
			if err := c.deleteFolderChildren(ctx, item.ID, item.WorkspaceID); err != nil {
				return err
			}
		}
	}

	// Group by content type for efficient batch deletion of LEAF content
	var httpItems []HTTPDeleteItem
	var flowItems []FlowDeleteItem

	for _, item := range items {
		if item.ContentID != nil {
			//nolint:exhaustive
			switch item.ContentKind {
			case mfile.ContentTypeHTTP:
				httpItems = append(httpItems, HTTPDeleteItem{
					ID:          *item.ContentID,
					WorkspaceID: item.WorkspaceID,
					IsDelta:     false,
				})
			case mfile.ContentTypeHTTPDelta:
				httpItems = append(httpItems, HTTPDeleteItem{
					ID:          *item.ContentID,
					WorkspaceID: item.WorkspaceID,
					IsDelta:     true,
				})
			case mfile.ContentTypeFlow:
				flowItems = append(flowItems, FlowDeleteItem{
					ID:          *item.ContentID,
					WorkspaceID: item.WorkspaceID,
				})
			}
		}
	}

	// Delete HTTP content batch (internal method - no File lookup)
	if len(httpItems) > 0 {
		if err := c.deleteHTTPContentBatch(ctx, httpItems); err != nil {
			return err
		}
	}

	// Delete Flow content batch (internal method - no File lookup)
	if len(flowItems) > 0 {
		if err := c.deleteFlowContentBatch(ctx, flowItems); err != nil {
			return err
		}
	}

	// Track file deletes and delete file records
	for _, item := range items {
		c.track(Event{
			Entity:      EntityFile,
			Op:          OpDelete,
			ID:          item.ID,
			WorkspaceID: item.WorkspaceID,
		})
		if err := c.q.DeleteFile(ctx, item.ID); err != nil {
			return err
		}
	}

	return nil
}

// deleteFolderChildren recursively finds and deletes all children of a folder.
func (c *Context) deleteFolderChildren(ctx context.Context, folderID, workspaceID idwrap.IDWrap) error {
	children, err := c.q.GetFilesByParentID(ctx, &folderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}

	if len(children) == 0 {
		return nil
	}

	// Convert DB models to FileDeleteItem
	var itemsToDelete []FileDeleteItem
	for _, child := range children {
		itemsToDelete = append(itemsToDelete, FileDeleteItem{
			ID:          child.ID,
			WorkspaceID: workspaceID,
			ContentID:   child.ContentID,
			ContentKind: mfile.ContentType(child.ContentKind),
		})
	}

	// Recursively delete children using batch method
	// This handles their content (HTTP/Flow) and their own children (if nested folders)
	return c.DeleteFileBatch(ctx, itemsToDelete)
}