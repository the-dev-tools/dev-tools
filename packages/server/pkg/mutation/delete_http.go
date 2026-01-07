package mutation

import (
	"context"
	"database/sql"
	"errors"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mfile"
)

// HTTPDeleteItem represents an HTTP entry to delete with its context.
type HTTPDeleteItem struct {
	ID          idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
	IsDelta     bool
}

// DeleteHTTP deletes an HTTP entry, handling File ownership.
// If HTTP has a File owner, deletion goes through File (which cascades back to HTTP).
// If HTTP is orphaned (no File), it's deleted directly.
// This is the PUBLIC API - safe to call from RPC handlers.
func (c *Context) DeleteHTTP(ctx context.Context, item HTTPDeleteItem) error {
	// Check if HTTP has a File owner
	itemID := item.ID
	file, err := c.q.GetFileByContentID(ctx, &itemID)
	if err == nil && file.ID != (idwrap.IDWrap{}) {
		// Has File - delete through File (cascades to HTTP content)
		contentID := item.ID
		return c.DeleteFile(ctx, FileDeleteItem{
			ID:          file.ID,
			WorkspaceID: item.WorkspaceID,
			ContentID:   &contentID,
			ContentKind: contentKindFromIsDelta(item.IsDelta),
		})
	}
	// Orphaned or not found - delete content directly
	return c.deleteHTTPContent(ctx, item.ID, item.WorkspaceID, item.IsDelta)
}

// DeleteHTTPBatch deletes multiple HTTP entries, handling File ownership.
// Groups items by whether they have File owners for efficient processing.
func (c *Context) DeleteHTTPBatch(ctx context.Context, items []HTTPDeleteItem) error {
	if len(items) == 0 {
		return nil
	}

	// Separate items with File owners from orphaned items
	var fileItems []FileDeleteItem
	var orphanedItems []HTTPDeleteItem

	for _, item := range items {
		itemID := item.ID
		file, err := c.q.GetFileByContentID(ctx, &itemID)
		if err == nil && file.ID != (idwrap.IDWrap{}) {
			// Has File owner
			contentID := item.ID
			fileItems = append(fileItems, FileDeleteItem{
				ID:          file.ID,
				WorkspaceID: item.WorkspaceID,
				ContentID:   &contentID,
				ContentKind: contentKindFromIsDelta(item.IsDelta),
			})
		} else {
			// Orphaned
			orphanedItems = append(orphanedItems, item)
		}
	}

	// Delete File-owned items through File
	if len(fileItems) > 0 {
		if err := c.DeleteFileBatch(ctx, fileItems); err != nil {
			return err
		}
	}

	// Delete orphaned items directly
	if len(orphanedItems) > 0 {
		if err := c.deleteHTTPContentBatch(ctx, orphanedItems); err != nil {
			return err
		}
	}

	return nil
}

// contentKindFromIsDelta returns the appropriate content kind.
func contentKindFromIsDelta(isDelta bool) mfile.ContentType {
	if isDelta {
		return mfile.ContentTypeHTTPDelta
	}
	return mfile.ContentTypeHTTP
}

// deleteHTTPContent is the INTERNAL method that deletes HTTP content directly.
// It collects cascade events for children and deletes the HTTP record.
// This should only be called from DeleteFile or DeleteHTTP (for orphans).
// Unexported to enforce compile-time cascade safety.
func (c *Context) deleteHTTPContent(ctx context.Context, httpID, workspaceID idwrap.IDWrap, isDelta bool) error {
	// Collect children before delete (all queries use indexes)
	c.collectHTTPChildren(ctx, httpID, workspaceID)

	// Track parent delete
	c.track(Event{
		Entity:      EntityHTTP,
		Op:          OpDelete,
		ID:          httpID,
		WorkspaceID: workspaceID,
		IsDelta:     isDelta,
	})

	// Delete - DB CASCADE handles actual child deletion
	err := c.q.DeleteHTTP(ctx, httpID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	return nil
}

// deleteHTTPContentBatch is the INTERNAL batch method for deleting HTTP content.
// Uses IN clause queries - constant query count regardless of item count.
// Unexported to enforce compile-time cascade safety.
func (c *Context) deleteHTTPContentBatch(ctx context.Context, items []HTTPDeleteItem) error {
	if len(items) == 0 {
		return nil
	}

	// Build ID list and lookup map
	httpIDs := make([]idwrap.IDWrap, len(items))
	itemMap := make(map[idwrap.IDWrap]HTTPDeleteItem, len(items))
	for i, item := range items {
		httpIDs[i] = item.ID
		itemMap[item.ID] = item
	}

	// Batch collect all children (constant queries)
	c.collectHTTPChildrenBatch(ctx, httpIDs, itemMap)

	// Track parent deletes
	for _, item := range items {
		c.track(Event{
			Entity:      EntityHTTP,
			Op:          OpDelete,
			ID:          item.ID,
			WorkspaceID: item.WorkspaceID,
			IsDelta:     item.IsDelta,
		})
	}

	// Delete all (DB CASCADE handles children)
	for _, item := range items {
		if err := c.q.DeleteHTTP(ctx, item.ID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
	}

	return nil
}

// collectHTTPChildren collects cascade events for a single HTTP entry.
func (c *Context) collectHTTPChildren(ctx context.Context, httpID, workspaceID idwrap.IDWrap) {
	// Headers - uses idx: http_header_http_idx
	if headers, err := c.q.GetHTTPHeaders(ctx, httpID); err == nil {
		for i := range headers {
			c.track(Event{
				Entity:      EntityHTTPHeader,
				Op:          OpDelete,
				ID:          headers[i].ID,
				WorkspaceID: workspaceID,
				IsDelta:     headers[i].IsDelta,
			})
		}
	}

	// Search Params - uses idx: http_search_param_http_idx
	if params, err := c.q.GetHTTPSearchParams(ctx, httpID); err == nil {
		for i := range params {
			c.track(Event{
				Entity:      EntityHTTPParam,
				Op:          OpDelete,
				ID:          params[i].ID,
				WorkspaceID: workspaceID,
				IsDelta:     params[i].IsDelta,
			})
		}
	}

	// Body Forms - uses idx: http_body_form_http_idx
	if forms, err := c.q.GetHTTPBodyForms(ctx, httpID); err == nil {
		for i := range forms {
			c.track(Event{
				Entity:      EntityHTTPBodyForm,
				Op:          OpDelete,
				ID:          forms[i].ID,
				WorkspaceID: workspaceID,
				IsDelta:     forms[i].IsDelta,
			})
		}
	}

	// Body URL Encoded - uses idx: http_body_urlencoded_http_idx
	if urls, err := c.q.GetHTTPBodyUrlEncodedByHttpID(ctx, httpID); err == nil {
		for i := range urls {
			c.track(Event{
				Entity:      EntityHTTPBodyURL,
				Op:          OpDelete,
				ID:          urls[i].ID,
				WorkspaceID: workspaceID,
				IsDelta:     urls[i].IsDelta,
			})
		}
	}

	// Body Raw - uses idx: http_body_raw_http_idx
	if raw, err := c.q.GetHTTPBodyRaw(ctx, httpID); err == nil {
		c.track(Event{
			Entity:      EntityHTTPBodyRaw,
			Op:          OpDelete,
			ID:          raw.ID,
			WorkspaceID: workspaceID,
			IsDelta:     raw.IsDelta,
		})
	}

	// Asserts - uses idx: http_assert_http_idx
	if asserts, err := c.q.GetHTTPAssertsByHttpID(ctx, httpID); err == nil {
		for i := range asserts {
			c.track(Event{
				Entity:      EntityHTTPAssert,
				Op:          OpDelete,
				ID:          asserts[i].ID,
				WorkspaceID: workspaceID,
				IsDelta:     asserts[i].IsDelta,
			})
		}
	}
}

// collectHTTPChildrenBatch collects cascade events for multiple HTTP entries.
// Uses IN clause queries - 6 queries total regardless of HTTP count.
func (c *Context) collectHTTPChildrenBatch(ctx context.Context, httpIDs []idwrap.IDWrap, itemMap map[idwrap.IDWrap]HTTPDeleteItem) {
	// Headers - single query for all parents
	if headers, err := c.q.GetHTTPHeadersByHttpIDs(ctx, httpIDs); err == nil {
		for i := range headers {
			if item, ok := itemMap[headers[i].HttpID]; ok {
				c.track(Event{
					Entity:      EntityHTTPHeader,
					Op:          OpDelete,
					ID:          headers[i].ID,
					WorkspaceID: item.WorkspaceID,
					IsDelta:     headers[i].IsDelta,
				})
			}
		}
	}

	// Search Params - single query for all parents
	if params, err := c.q.GetHTTPSearchParamsByHttpIDs(ctx, httpIDs); err == nil {
		for i := range params {
			if item, ok := itemMap[params[i].HttpID]; ok {
				c.track(Event{
					Entity:      EntityHTTPParam,
					Op:          OpDelete,
					ID:          params[i].ID,
					WorkspaceID: item.WorkspaceID,
					IsDelta:     params[i].IsDelta,
				})
			}
		}
	}

	// Body Forms - single query for all parents
	if forms, err := c.q.GetHTTPBodyFormsByHttpIDs(ctx, httpIDs); err == nil {
		for i := range forms {
			if item, ok := itemMap[forms[i].HttpID]; ok {
				c.track(Event{
					Entity:      EntityHTTPBodyForm,
					Op:          OpDelete,
					ID:          forms[i].ID,
					WorkspaceID: item.WorkspaceID,
					IsDelta:     forms[i].IsDelta,
				})
			}
		}
	}

	// Body URL Encoded - single query for all parents
	if urls, err := c.q.GetHTTPBodyUrlencodedsByHttpIDs(ctx, httpIDs); err == nil {
		for i := range urls {
			if item, ok := itemMap[urls[i].HttpID]; ok {
				c.track(Event{
					Entity:      EntityHTTPBodyURL,
					Op:          OpDelete,
					ID:          urls[i].ID,
					WorkspaceID: item.WorkspaceID,
					IsDelta:     urls[i].IsDelta,
				})
			}
		}
	}

	// Body Raw - single query for all parents
	if raws, err := c.q.GetHTTPBodyRawsByHttpIDs(ctx, httpIDs); err == nil {
		for i := range raws {
			if item, ok := itemMap[raws[i].HttpID]; ok {
				c.track(Event{
					Entity:      EntityHTTPBodyRaw,
					Op:          OpDelete,
					ID:          raws[i].ID,
					WorkspaceID: item.WorkspaceID,
					IsDelta:     raws[i].IsDelta,
				})
			}
		}
	}

	// Asserts - single query for all parents
	if asserts, err := c.q.GetHTTPAssertsByHttpIDs(ctx, httpIDs); err == nil {
		for i := range asserts {
			if item, ok := itemMap[asserts[i].HttpID]; ok {
				c.track(Event{
					Entity:      EntityHTTPAssert,
					Op:          OpDelete,
					ID:          asserts[i].ID,
					WorkspaceID: item.WorkspaceID,
					IsDelta:     asserts[i].IsDelta,
				})
			}
		}
	}
}
