package mutation

import (
	"context"
	"database/sql"
	"errors"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mfile"
)

// FlowDeleteItem represents a flow to delete with its context.
type FlowDeleteItem struct {
	ID          idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
}

// DeleteFlow deletes a flow, handling File ownership.
// If Flow has a File owner, deletion goes through File (which cascades back to Flow).
// If Flow is orphaned (no File), it's deleted directly.
// This is the PUBLIC API - safe to call from RPC handlers.
func (c *Context) DeleteFlow(ctx context.Context, item FlowDeleteItem) error {
	// Check if Flow has a File owner
	itemID := item.ID
	file, err := c.q.GetFileByContentID(ctx, &itemID)
	if err == nil && file.ID != (idwrap.IDWrap{}) {
		// Has File - delete through File (cascades to Flow content)
		contentID := item.ID
		return c.DeleteFile(ctx, FileDeleteItem{
			ID:          file.ID,
			WorkspaceID: item.WorkspaceID,
			ContentID:   &contentID,
			ContentKind: mfile.ContentTypeFlow,
		})
	}
	// Orphaned or not found - delete content directly
	return c.deleteFlowContent(ctx, item.ID, item.WorkspaceID)
}

// DeleteFlowBatch deletes multiple flows, handling File ownership.
// Groups items by whether they have File owners for efficient processing.
func (c *Context) DeleteFlowBatch(ctx context.Context, items []FlowDeleteItem) error {
	if len(items) == 0 {
		return nil
	}

	// Separate items with File owners from orphaned items
	var fileItems []FileDeleteItem
	var orphanedItems []FlowDeleteItem

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
				ContentKind: mfile.ContentTypeFlow,
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
		if err := c.deleteFlowContentBatch(ctx, orphanedItems); err != nil {
			return err
		}
	}

	return nil
}

// deleteFlowContent is the INTERNAL method that deletes Flow content directly.
// It collects cascade events for children and deletes the Flow record.
// This should only be called from DeleteFile or DeleteFlow (for orphans).
// Unexported to enforce compile-time cascade safety.
func (c *Context) deleteFlowContent(ctx context.Context, flowID, workspaceID idwrap.IDWrap) error {
	// Collect children before delete (all queries use indexes)
	c.collectFlowChildren(ctx, flowID, workspaceID)

	// Track parent delete
	c.track(Event{
		Entity:      EntityFlow,
		Op:          OpDelete,
		ID:          flowID,
		WorkspaceID: workspaceID,
	})

	// Delete - DB CASCADE handles actual child deletion
	err := c.q.DeleteFlow(ctx, flowID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	return nil
}

// deleteFlowContentBatch is the INTERNAL batch method for deleting Flow content.
// Uses IN clause queries - 3 queries total regardless of flow count.
// Unexported to enforce compile-time cascade safety.
func (c *Context) deleteFlowContentBatch(ctx context.Context, items []FlowDeleteItem) error {
	if len(items) == 0 {
		return nil
	}

	// Build ID list and lookup map
	flowIDs := make([]idwrap.IDWrap, len(items))
	itemMap := make(map[idwrap.IDWrap]FlowDeleteItem, len(items))
	for i, item := range items {
		flowIDs[i] = item.ID
		itemMap[item.ID] = item
	}

	// Batch collect all children (3 queries total)
	c.collectFlowChildrenBatch(ctx, flowIDs, itemMap)

	// Track parent deletes
	for _, item := range items {
		c.track(Event{
			Entity:      EntityFlow,
			Op:          OpDelete,
			ID:          item.ID,
			WorkspaceID: item.WorkspaceID,
		})
	}

	// Delete all (DB CASCADE handles children)
	for _, item := range items {
		if err := c.q.DeleteFlow(ctx, item.ID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
	}

	return nil
}

// collectFlowChildren collects cascade events for a single flow.
func (c *Context) collectFlowChildren(ctx context.Context, flowID, workspaceID idwrap.IDWrap) {
	// Nodes - uses idx: flow_node_idx1
	if nodes, err := c.q.GetFlowNodesByFlowID(ctx, flowID); err == nil {
		for i := range nodes {
			c.track(Event{
				Entity:      EntityFlowNode,
				Op:          OpDelete,
				ID:          nodes[i].ID,
				WorkspaceID: workspaceID,
				ParentID:    flowID,
			})
		}
	}

	// Edges - uses idx: flow_edge_idx1
	if edges, err := c.q.GetFlowEdgesByFlowID(ctx, flowID); err == nil {
		for i := range edges {
			c.track(Event{
				Entity:      EntityFlowEdge,
				Op:          OpDelete,
				ID:          edges[i].ID,
				WorkspaceID: workspaceID,
				ParentID:    flowID,
			})
		}
	}

	// Variables - uses idx: flow_variable_ordering
	if vars, err := c.q.GetFlowVariablesByFlowID(ctx, flowID); err == nil {
		for i := range vars {
			c.track(Event{
				Entity:      EntityFlowVariable,
				Op:          OpDelete,
				ID:          vars[i].ID,
				WorkspaceID: workspaceID,
				ParentID:    flowID,
			})
		}
	}
}

// collectFlowChildrenBatch collects cascade events for multiple flows.
// Uses IN clause queries - 3 queries total regardless of flow count.
func (c *Context) collectFlowChildrenBatch(ctx context.Context, flowIDs []idwrap.IDWrap, itemMap map[idwrap.IDWrap]FlowDeleteItem) {
	// Nodes - single query for all parents
	if nodes, err := c.q.GetFlowNodesByFlowIDs(ctx, flowIDs); err == nil {
		for i := range nodes {
			if item, ok := itemMap[nodes[i].FlowID]; ok {
				c.track(Event{
					Entity:      EntityFlowNode,
					Op:          OpDelete,
					ID:          nodes[i].ID,
					WorkspaceID: item.WorkspaceID,
					ParentID:    nodes[i].FlowID,
				})
			}
		}
	}

	// Edges - single query for all parents
	if edges, err := c.q.GetFlowEdgesByFlowIDs(ctx, flowIDs); err == nil {
		for i := range edges {
			if item, ok := itemMap[edges[i].FlowID]; ok {
				c.track(Event{
					Entity:      EntityFlowEdge,
					Op:          OpDelete,
					ID:          edges[i].ID,
					WorkspaceID: item.WorkspaceID,
					ParentID:    edges[i].FlowID,
				})
			}
		}
	}

	// Variables - single query for all parents
	if vars, err := c.q.GetFlowVariablesByFlowIDs(ctx, flowIDs); err == nil {
		for i := range vars {
			if item, ok := itemMap[vars[i].FlowID]; ok {
				c.track(Event{
					Entity:      EntityFlowVariable,
					Op:          OpDelete,
					ID:          vars[i].ID,
					WorkspaceID: item.WorkspaceID,
					ParentID:    vars[i].FlowID,
				})
			}
		}
	}
}