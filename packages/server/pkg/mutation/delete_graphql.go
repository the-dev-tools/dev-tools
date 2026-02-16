package mutation

import (
	"context"
	"database/sql"
	"errors"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

// GraphQLDeleteItem represents a GraphQL entry to delete.
type GraphQLDeleteItem struct {
	ID          idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
}

// DeleteGraphQL deletes a GraphQL entry and tracks cascade events.
func (c *Context) DeleteGraphQL(ctx context.Context, item GraphQLDeleteItem) error {
	// Collect children before delete
	c.collectGraphQLChildren(ctx, item.ID, item.WorkspaceID)

	// Track parent delete
	c.track(Event{
		Entity:      EntityGraphQL,
		Op:          OpDelete,
		ID:          item.ID,
		WorkspaceID: item.WorkspaceID,
	})

	// Delete - DB CASCADE handles actual child deletion
	err := c.q.DeleteGraphQL(ctx, item.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	return nil
}

// DeleteGraphQLBatch deletes multiple GraphQL entries.
func (c *Context) DeleteGraphQLBatch(ctx context.Context, items []GraphQLDeleteItem) error {
	for _, item := range items {
		if err := c.DeleteGraphQL(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// collectGraphQLChildren collects cascade events for a single GraphQL entry.
func (c *Context) collectGraphQLChildren(ctx context.Context, graphqlID, workspaceID idwrap.IDWrap) {
	// Headers - cascaded by DB FK
	if headers, err := c.q.GetGraphQLHeaders(ctx, graphqlID); err == nil {
		for i := range headers {
			c.track(Event{
				Entity:      EntityGraphQLHeader,
				Op:          OpDelete,
				ID:          headers[i].ID,
				ParentID:    graphqlID,
				WorkspaceID: workspaceID,
			})
		}
	}

	// Asserts - cascaded by DB FK
	if asserts, err := c.q.GetGraphQLAssertsByGraphQLID(ctx, graphqlID.Bytes()); err == nil {
		for i := range asserts {
			id, _ := idwrap.NewFromBytes(asserts[i].ID)
			c.track(Event{
				Entity:      EntityGraphQLAssert,
				Op:          OpDelete,
				ID:          id,
				ParentID:    graphqlID,
				WorkspaceID: workspaceID,
				IsDelta:     asserts[i].IsDelta,
			})
		}
	}
}
