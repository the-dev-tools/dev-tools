package mutation

import (
	"context"

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
			// Folder - no content to delete, just the file record
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

	// Group by content type for efficient batch deletion
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
