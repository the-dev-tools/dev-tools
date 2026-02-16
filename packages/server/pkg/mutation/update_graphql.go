package mutation

import (
	"context"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mgraphql"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/patch"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sgraphql"
)

// GraphQLUpdateItem represents a GraphQL entry to update.
type GraphQLUpdateItem struct {
	GraphQL     *mgraphql.GraphQL
	WorkspaceID idwrap.IDWrap
}

// UpdateGraphQL updates a GraphQL entry and tracks the event.
func (c *Context) UpdateGraphQL(ctx context.Context, item GraphQLUpdateItem) error {
	writer := sgraphql.NewWriterFromQueries(c.q)

	if err := writer.Update(ctx, item.GraphQL); err != nil {
		return err
	}

	c.track(Event{
		Entity:      EntityGraphQL,
		Op:          OpUpdate,
		ID:          item.GraphQL.ID,
		WorkspaceID: item.WorkspaceID,
		Payload:     item.GraphQL,
	})

	return nil
}

// UpdateGraphQLBatch updates multiple GraphQL entries.
func (c *Context) UpdateGraphQLBatch(ctx context.Context, items []GraphQLUpdateItem) error {
	for _, item := range items {
		if err := c.UpdateGraphQL(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// GraphQLAssertUpdateItem represents a GraphQL assert to update.
type GraphQLAssertUpdateItem struct {
	ID          idwrap.IDWrap
	GraphQLID   idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
	IsDelta     bool
	Params      gen.UpdateGraphQLAssertParams
	Patch       patch.GraphQLAssertPatch
}

// UpdateGraphQLAssert updates a GraphQL assert and tracks the event.
func (c *Context) UpdateGraphQLAssert(ctx context.Context, item GraphQLAssertUpdateItem) error {
	if err := c.q.UpdateGraphQLAssert(ctx, item.Params); err != nil {
		return err
	}
	c.track(Event{
		Entity:      EntityGraphQLAssert,
		Op:          OpUpdate,
		ID:          item.ID,
		WorkspaceID: item.WorkspaceID,
		ParentID:    item.GraphQLID,
		IsDelta:     item.IsDelta,
		Patch:       item.Patch,
	})
	return nil
}

// UpdateGraphQLAssertBatch updates multiple GraphQL asserts.
func (c *Context) UpdateGraphQLAssertBatch(ctx context.Context, items []GraphQLAssertUpdateItem) error {
	for _, item := range items {
		if err := c.UpdateGraphQLAssert(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// GraphQLAssertDeltaUpdateItem represents a GraphQL assert delta to update.
type GraphQLAssertDeltaUpdateItem struct {
	ID          idwrap.IDWrap
	GraphQLID   idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
	Params      gen.UpdateGraphQLAssertDeltaParams
	Patch       any
	Payload     any
}

// UpdateGraphQLAssertDelta updates a GraphQL assert delta and tracks the event.
func (c *Context) UpdateGraphQLAssertDelta(ctx context.Context, item GraphQLAssertDeltaUpdateItem) error {
	if err := c.q.UpdateGraphQLAssertDelta(ctx, item.Params); err != nil {
		return err
	}
	c.track(Event{
		Entity:      EntityGraphQLAssert,
		Op:          OpUpdate,
		ID:          item.ID,
		WorkspaceID: item.WorkspaceID,
		IsDelta:     true,
		Patch:       item.Patch,
		Payload:     item.Payload,
	})
	return nil
}

// UpdateGraphQLAssertDeltaBatch updates multiple GraphQL assert deltas.
func (c *Context) UpdateGraphQLAssertDeltaBatch(ctx context.Context, items []GraphQLAssertDeltaUpdateItem) error {
	for _, item := range items {
		if err := c.UpdateGraphQLAssertDelta(ctx, item); err != nil {
			return err
		}
	}
	return nil
}
