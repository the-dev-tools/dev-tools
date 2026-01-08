package mutation

import (
	"context"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

// FlowNodeUpdateItem represents a flow node to update.
type FlowNodeUpdateItem struct {
	ID          idwrap.IDWrap
	FlowID      idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
	Name        string
	PositionX   float64
	PositionY   float64
}

// FlowNodePatch represents optional fields for partial update.
type FlowNodePatch struct {
	Name      *string
	PositionX *float64
	PositionY *float64
}

// UpdateFlowNode updates a flow node and tracks the event.
func (c *Context) UpdateFlowNode(ctx context.Context, item FlowNodeUpdateItem) error {
	err := c.q.UpdateFlowNode(ctx, gen.UpdateFlowNodeParams{
		ID:        item.ID,
		Name:      item.Name,
		PositionX: item.PositionX,
		PositionY: item.PositionY,
	})
	if err != nil {
		return err
	}
	c.track(Event{
		Entity:      EntityFlowNode,
		Op:          OpUpdate,
		ID:          item.ID,
		WorkspaceID: item.WorkspaceID,
		ParentID:    item.FlowID,
	})
	return nil
}

// UpdateFlowNodeBatch updates multiple flow nodes.
func (c *Context) UpdateFlowNodeBatch(ctx context.Context, items []FlowNodeUpdateItem) error {
	for _, item := range items {
		if err := c.UpdateFlowNode(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// FlowEdgeUpdateItem represents a flow edge to update.
type FlowEdgeUpdateItem struct {
	ID           idwrap.IDWrap
	FlowID       idwrap.IDWrap
	WorkspaceID  idwrap.IDWrap
	SourceID     idwrap.IDWrap
	TargetID     idwrap.IDWrap
	SourceHandle int32
}

// FlowEdgePatch represents optional fields for partial update.
type FlowEdgePatch struct {
	SourceID     *idwrap.IDWrap
	TargetID     *idwrap.IDWrap
	SourceHandle *int32
}

// UpdateFlowEdge updates a flow edge and tracks the event.
func (c *Context) UpdateFlowEdge(ctx context.Context, item FlowEdgeUpdateItem) error {
	err := c.q.UpdateFlowEdge(ctx, gen.UpdateFlowEdgeParams{
		ID:           item.ID,
		SourceID:     item.SourceID,
		TargetID:     item.TargetID,
		SourceHandle: item.SourceHandle,
	})
	if err != nil {
		return err
	}
	c.track(Event{
		Entity:      EntityFlowEdge,
		Op:          OpUpdate,
		ID:          item.ID,
		WorkspaceID: item.WorkspaceID,
		ParentID:    item.FlowID,
	})
	return nil
}

// UpdateFlowEdgeBatch updates multiple flow edges.
func (c *Context) UpdateFlowEdgeBatch(ctx context.Context, items []FlowEdgeUpdateItem) error {
	for _, item := range items {
		if err := c.UpdateFlowEdge(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// FlowVariableUpdateItem represents a flow variable to update.
type FlowVariableUpdateItem struct {
	ID          idwrap.IDWrap
	FlowID      idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
	Key         string
	Value       string
	Enabled     bool
	Description string
}

// FlowVariablePatch represents optional fields for partial update.
type FlowVariablePatch struct {
	Key         *string
	Value       *string
	Enabled     *bool
	Description *string
}

// UpdateFlowVariable updates a flow variable and tracks the event.
func (c *Context) UpdateFlowVariable(ctx context.Context, item FlowVariableUpdateItem) error {
	err := c.q.UpdateFlowVariable(ctx, gen.UpdateFlowVariableParams{
		ID:          item.ID,
		Key:         item.Key,
		Value:       item.Value,
		Enabled:     item.Enabled,
		Description: item.Description,
	})
	if err != nil {
		return err
	}
	c.track(Event{
		Entity:      EntityFlowVariable,
		Op:          OpUpdate,
		ID:          item.ID,
		WorkspaceID: item.WorkspaceID,
		ParentID:    item.FlowID,
	})
	return nil
}

// UpdateFlowVariableBatch updates multiple flow variables.
func (c *Context) UpdateFlowVariableBatch(ctx context.Context, items []FlowVariableUpdateItem) error {
	for _, item := range items {
		if err := c.UpdateFlowVariable(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// FlowNodeJSUpdateItem represents a flow node JS to update.
type FlowNodeJSUpdateItem struct {
	FlowNodeID       idwrap.IDWrap
	FlowID           idwrap.IDWrap
	WorkspaceID      idwrap.IDWrap
	Code             []byte
	CodeCompressType int8
}

// FlowNodeJSPatch represents optional fields for partial update.
type FlowNodeJSPatch struct {
	Code             []byte
	CodeCompressType *int8
}

// UpdateFlowNodeJS updates a flow node JS and tracks the event.
func (c *Context) UpdateFlowNodeJS(ctx context.Context, item FlowNodeJSUpdateItem) error {
	err := c.q.UpdateFlowNodeJs(ctx, gen.UpdateFlowNodeJsParams{
		FlowNodeID:       item.FlowNodeID,
		Code:             item.Code,
		CodeCompressType: item.CodeCompressType,
	})
	if err != nil {
		return err
	}
	c.track(Event{
		Entity:      EntityFlowNodeJS,
		Op:          OpUpdate,
		ID:          item.FlowNodeID,
		WorkspaceID: item.WorkspaceID,
		ParentID:    item.FlowID,
	})
	return nil
}

// UpdateFlowNodeJSBatch updates multiple flow node JS records.
func (c *Context) UpdateFlowNodeJSBatch(ctx context.Context, items []FlowNodeJSUpdateItem) error {
	for _, item := range items {
		if err := c.UpdateFlowNodeJS(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// FlowNodeHTTPUpdateItem represents a flow node HTTP to update.
type FlowNodeHTTPUpdateItem struct {
	FlowNodeID  idwrap.IDWrap
	FlowID      idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
	HttpID      idwrap.IDWrap
	DeltaHttpID []byte
}

// FlowNodeHTTPPatch represents optional fields for partial update.
type FlowNodeHTTPPatch struct {
	HttpID      *idwrap.IDWrap
	DeltaHttpID []byte
}

// UpdateFlowNodeHTTP updates a flow node HTTP and tracks the event.
func (c *Context) UpdateFlowNodeHTTP(ctx context.Context, item FlowNodeHTTPUpdateItem) error {
	err := c.q.UpdateFlowNodeHTTP(ctx, gen.UpdateFlowNodeHTTPParams{
		FlowNodeID:  item.FlowNodeID,
		HttpID:      item.HttpID,
		DeltaHttpID: item.DeltaHttpID,
	})
	if err != nil {
		return err
	}
	c.track(Event{
		Entity:      EntityFlowNodeHTTP,
		Op:          OpUpdate,
		ID:          item.FlowNodeID,
		WorkspaceID: item.WorkspaceID,
		ParentID:    item.FlowID,
	})
	return nil
}

// UpdateFlowNodeHTTPBatch updates multiple flow node HTTP records.
func (c *Context) UpdateFlowNodeHTTPBatch(ctx context.Context, items []FlowNodeHTTPUpdateItem) error {
	for _, item := range items {
		if err := c.UpdateFlowNodeHTTP(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// FlowNodeForUpdateItem represents a flow node for-loop to update.
type FlowNodeForUpdateItem struct {
	FlowNodeID    idwrap.IDWrap
	FlowID        idwrap.IDWrap
	WorkspaceID   idwrap.IDWrap
	IterCount     int64
	ErrorHandling int8
	Expression    string
}

// FlowNodeForPatch represents optional fields for partial update.
type FlowNodeForPatch struct {
	IterCount     *int64
	ErrorHandling *int8
	Expression    *string
}

// UpdateFlowNodeFor updates a flow node for-loop and tracks the event.
func (c *Context) UpdateFlowNodeFor(ctx context.Context, item FlowNodeForUpdateItem) error {
	err := c.q.UpdateFlowNodeFor(ctx, gen.UpdateFlowNodeForParams{
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
		Op:          OpUpdate,
		ID:          item.FlowNodeID,
		WorkspaceID: item.WorkspaceID,
		ParentID:    item.FlowID,
	})
	return nil
}

// UpdateFlowNodeForBatch updates multiple flow node for-loop records.
func (c *Context) UpdateFlowNodeForBatch(ctx context.Context, items []FlowNodeForUpdateItem) error {
	for _, item := range items {
		if err := c.UpdateFlowNodeFor(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// FlowNodeForEachUpdateItem represents a flow node for-each to update.
type FlowNodeForEachUpdateItem struct {
	FlowNodeID     idwrap.IDWrap
	FlowID         idwrap.IDWrap
	WorkspaceID    idwrap.IDWrap
	IterExpression string
	ErrorHandling  int8
	Expression     string
}

// FlowNodeForEachPatch represents optional fields for partial update.
type FlowNodeForEachPatch struct {
	IterExpression *string
	ErrorHandling  *int8
	Expression     *string
}

// UpdateFlowNodeForEach updates a flow node for-each and tracks the event.
func (c *Context) UpdateFlowNodeForEach(ctx context.Context, item FlowNodeForEachUpdateItem) error {
	err := c.q.UpdateFlowNodeForEach(ctx, gen.UpdateFlowNodeForEachParams{
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
		Op:          OpUpdate,
		ID:          item.FlowNodeID,
		WorkspaceID: item.WorkspaceID,
		ParentID:    item.FlowID,
	})
	return nil
}

// UpdateFlowNodeForEachBatch updates multiple flow node for-each records.
func (c *Context) UpdateFlowNodeForEachBatch(ctx context.Context, items []FlowNodeForEachUpdateItem) error {
	for _, item := range items {
		if err := c.UpdateFlowNodeForEach(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// FlowNodeConditionUpdateItem represents a flow node condition to update.
type FlowNodeConditionUpdateItem struct {
	FlowNodeID  idwrap.IDWrap
	FlowID      idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
	Expression  string
}

// FlowNodeConditionPatch represents optional fields for partial update.
type FlowNodeConditionPatch struct {
	Expression *string
}

// UpdateFlowNodeCondition updates a flow node condition and tracks the event.
func (c *Context) UpdateFlowNodeCondition(ctx context.Context, item FlowNodeConditionUpdateItem) error {
	err := c.q.UpdateFlowNodeCondition(ctx, gen.UpdateFlowNodeConditionParams{
		FlowNodeID: item.FlowNodeID,
		Expression: item.Expression,
	})
	if err != nil {
		return err
	}
	c.track(Event{
		Entity:      EntityFlowNodeCondition,
		Op:          OpUpdate,
		ID:          item.FlowNodeID,
		WorkspaceID: item.WorkspaceID,
		ParentID:    item.FlowID,
	})
	return nil
}

// UpdateFlowNodeConditionBatch updates multiple flow node condition records.
func (c *Context) UpdateFlowNodeConditionBatch(ctx context.Context, items []FlowNodeConditionUpdateItem) error {
	for _, item := range items {
		if err := c.UpdateFlowNodeCondition(ctx, item); err != nil {
			return err
		}
	}
	return nil
}
