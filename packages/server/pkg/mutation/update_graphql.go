package mutation

import (
	"context"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mgraphql"
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
