package mutation

import (
	"context"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
)

// FlowNodeInsertItem represents a flow node to insert.
type FlowNodeInsertItem struct {
	ID          idwrap.IDWrap
	FlowID      idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
	Name        string
	NodeKind    int32
	PositionX   float64
	PositionY   float64
}

// InsertFlowNode inserts a flow node and tracks the event.
func (c *Context) InsertFlowNode(ctx context.Context, item FlowNodeInsertItem) error {
	err := c.q.CreateFlowNode(ctx, gen.CreateFlowNodeParams{
		ID:        item.ID,
		FlowID:    item.FlowID,
		Name:      item.Name,
		NodeKind:  item.NodeKind,
		PositionX: item.PositionX,
		PositionY: item.PositionY,
	})
	if err != nil {
		return err
	}
	c.track(Event{
		Entity:      EntityFlowNode,
		Op:          OpInsert,
		ID:          item.ID,
		WorkspaceID: item.WorkspaceID,
		ParentID:    item.FlowID,
	})
	return nil
}

// InsertFlowNodeBatch inserts multiple flow nodes.
func (c *Context) InsertFlowNodeBatch(ctx context.Context, items []FlowNodeInsertItem) error {
	for _, item := range items {
		if err := c.InsertFlowNode(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// FlowEdgeInsertItem represents a flow edge to insert.
type FlowEdgeInsertItem struct {
	ID           idwrap.IDWrap
	FlowID       idwrap.IDWrap
	WorkspaceID  idwrap.IDWrap
	SourceID     idwrap.IDWrap
	TargetID     idwrap.IDWrap
	SourceHandle int32
}

// InsertFlowEdge inserts a flow edge and tracks the event.
func (c *Context) InsertFlowEdge(ctx context.Context, item FlowEdgeInsertItem) error {
	err := c.q.CreateFlowEdge(ctx, gen.CreateFlowEdgeParams{
		ID:           item.ID,
		FlowID:       item.FlowID,
		SourceID:     item.SourceID,
		TargetID:     item.TargetID,
		SourceHandle: item.SourceHandle,
	})
	if err != nil {
		return err
	}
	c.track(Event{
		Entity:      EntityFlowEdge,
		Op:          OpInsert,
		ID:          item.ID,
		WorkspaceID: item.WorkspaceID,
		ParentID:    item.FlowID,
	})
	return nil
}

// InsertFlowEdgeBatch inserts multiple flow edges.
func (c *Context) InsertFlowEdgeBatch(ctx context.Context, items []FlowEdgeInsertItem) error {
	for _, item := range items {
		if err := c.InsertFlowEdge(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// FlowVariableInsertItem represents a flow variable to insert.
type FlowVariableInsertItem struct {
	ID           idwrap.IDWrap
	FlowID       idwrap.IDWrap
	WorkspaceID  idwrap.IDWrap
	Key          string
	Value        string
	Enabled      bool
	Description  string
	DisplayOrder float64
}

// InsertFlowVariable inserts a flow variable and tracks the event.
func (c *Context) InsertFlowVariable(ctx context.Context, item FlowVariableInsertItem) error {
	err := c.q.CreateFlowVariable(ctx, gen.CreateFlowVariableParams{
		ID:           item.ID,
		FlowID:       item.FlowID,
		Key:          item.Key,
		Value:        item.Value,
		Enabled:      item.Enabled,
		Description:  item.Description,
		DisplayOrder: item.DisplayOrder,
	})
	if err != nil {
		return err
	}
	c.track(Event{
		Entity:      EntityFlowVariable,
		Op:          OpInsert,
		ID:          item.ID,
		WorkspaceID: item.WorkspaceID,
		ParentID:    item.FlowID,
	})
	return nil
}

// InsertFlowVariableBatch inserts multiple flow variables.
func (c *Context) InsertFlowVariableBatch(ctx context.Context, items []FlowVariableInsertItem) error {
	for _, item := range items {
		if err := c.InsertFlowVariable(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// FlowNodeJSInsertItem represents a flow node JS to insert.
type FlowNodeJSInsertItem struct {
	FlowNodeID       idwrap.IDWrap
	FlowID           idwrap.IDWrap
	WorkspaceID      idwrap.IDWrap
	Code             []byte
	CodeCompressType int8
}

// InsertFlowNodeJS inserts a flow node JS and tracks the event.
func (c *Context) InsertFlowNodeJS(ctx context.Context, item FlowNodeJSInsertItem) error {
	err := c.q.CreateFlowNodeJs(ctx, gen.CreateFlowNodeJsParams{
		FlowNodeID:       item.FlowNodeID,
		Code:             item.Code,
		CodeCompressType: item.CodeCompressType,
	})
	if err != nil {
		return err
	}
	c.track(Event{
		Entity:      EntityFlowNodeJS,
		Op:          OpInsert,
		ID:          item.FlowNodeID,
		WorkspaceID: item.WorkspaceID,
		ParentID:    item.FlowID,
	})
	return nil
}

// InsertFlowNodeJSBatch inserts multiple flow node JS records.
func (c *Context) InsertFlowNodeJSBatch(ctx context.Context, items []FlowNodeJSInsertItem) error {
	for _, item := range items {
		if err := c.InsertFlowNodeJS(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// FlowNodeHTTPInsertItem represents a flow node HTTP to insert.
type FlowNodeHTTPInsertItem struct {
	FlowNodeID  idwrap.IDWrap
	FlowID      idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
	HttpID      idwrap.IDWrap
	DeltaHttpID []byte
}

// InsertFlowNodeHTTP inserts a flow node HTTP and tracks the event.
func (c *Context) InsertFlowNodeHTTP(ctx context.Context, item FlowNodeHTTPInsertItem) error {
	err := c.q.CreateFlowNodeHTTP(ctx, gen.CreateFlowNodeHTTPParams{
		FlowNodeID:  item.FlowNodeID,
		HttpID:      item.HttpID,
		DeltaHttpID: item.DeltaHttpID,
	})
	if err != nil {
		return err
	}
	c.track(Event{
		Entity:      EntityFlowNodeHTTP,
		Op:          OpInsert,
		ID:          item.FlowNodeID,
		WorkspaceID: item.WorkspaceID,
		ParentID:    item.FlowID,
	})
	return nil
}

// InsertFlowNodeHTTPBatch inserts multiple flow node HTTP records.
func (c *Context) InsertFlowNodeHTTPBatch(ctx context.Context, items []FlowNodeHTTPInsertItem) error {
	for _, item := range items {
		if err := c.InsertFlowNodeHTTP(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// FlowNodeForInsertItem represents a flow node for-loop to insert.
type FlowNodeForInsertItem struct {
	FlowNodeID    idwrap.IDWrap
	FlowID        idwrap.IDWrap
	WorkspaceID   idwrap.IDWrap
	IterCount     int64
	ErrorHandling int8
	Expression    string
}

// InsertFlowNodeFor inserts a flow node for-loop and tracks the event.
func (c *Context) InsertFlowNodeFor(ctx context.Context, item FlowNodeForInsertItem) error {
	err := c.q.CreateFlowNodeFor(ctx, gen.CreateFlowNodeForParams{
		FlowNodeID:    item.FlowNodeID,
		IterCount:     item.IterCount,
		ErrorHandling: item.ErrorHandling,
		Expression:    item.Expression,
	})
	if err != nil {
		return err
	}
	c.track(Event{
		Entity:      EntityFlowNodeFor,
		Op:          OpInsert,
		ID:          item.FlowNodeID,
		WorkspaceID: item.WorkspaceID,
		ParentID:    item.FlowID,
	})
	return nil
}

// InsertFlowNodeForBatch inserts multiple flow node for-loop records.
func (c *Context) InsertFlowNodeForBatch(ctx context.Context, items []FlowNodeForInsertItem) error {
	for _, item := range items {
		if err := c.InsertFlowNodeFor(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// FlowNodeForEachInsertItem represents a flow node for-each to insert.
type FlowNodeForEachInsertItem struct {
	FlowNodeID     idwrap.IDWrap
	FlowID         idwrap.IDWrap
	WorkspaceID    idwrap.IDWrap
	IterExpression string
	ErrorHandling  int8
	Expression     string
}

// InsertFlowNodeForEach inserts a flow node for-each and tracks the event.
func (c *Context) InsertFlowNodeForEach(ctx context.Context, item FlowNodeForEachInsertItem) error {
	err := c.q.CreateFlowNodeForEach(ctx, gen.CreateFlowNodeForEachParams{
		FlowNodeID:     item.FlowNodeID,
		IterExpression: item.IterExpression,
		ErrorHandling:  item.ErrorHandling,
		Expression:     item.Expression,
	})
	if err != nil {
		return err
	}
	c.track(Event{
		Entity:      EntityFlowNodeForEach,
		Op:          OpInsert,
		ID:          item.FlowNodeID,
		WorkspaceID: item.WorkspaceID,
		ParentID:    item.FlowID,
	})
	return nil
}

// InsertFlowNodeForEachBatch inserts multiple flow node for-each records.
func (c *Context) InsertFlowNodeForEachBatch(ctx context.Context, items []FlowNodeForEachInsertItem) error {
	for _, item := range items {
		if err := c.InsertFlowNodeForEach(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// FlowNodeConditionInsertItem represents a flow node condition to insert.
type FlowNodeConditionInsertItem struct {
	FlowNodeID  idwrap.IDWrap
	FlowID      idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
	Expression  string
}

// InsertFlowNodeCondition inserts a flow node condition and tracks the event.
func (c *Context) InsertFlowNodeCondition(ctx context.Context, item FlowNodeConditionInsertItem) error {
	err := c.q.CreateFlowNodeCondition(ctx, gen.CreateFlowNodeConditionParams{
		FlowNodeID: item.FlowNodeID,
		Expression: item.Expression,
	})
	if err != nil {
		return err
	}
	c.track(Event{
		Entity:      EntityFlowNodeCondition,
		Op:          OpInsert,
		ID:          item.FlowNodeID,
		WorkspaceID: item.WorkspaceID,
		ParentID:    item.FlowID,
	})
	return nil
}

// InsertFlowNodeConditionBatch inserts multiple flow node condition records.
func (c *Context) InsertFlowNodeConditionBatch(ctx context.Context, items []FlowNodeConditionInsertItem) error {
	for _, item := range items {
		if err := c.InsertFlowNodeCondition(ctx, item); err != nil {
			return err
		}
	}
	return nil
}
