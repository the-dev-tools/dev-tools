package mutation

import (
	"context"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mgraphql"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sgraphql"
)

// GraphQLInsertItem represents a GraphQL entry to insert.
type GraphQLInsertItem struct {
	GraphQL     *mgraphql.GraphQL
	WorkspaceID idwrap.IDWrap
}

// InsertGraphQL inserts a GraphQL entry and tracks the event.
func (c *Context) InsertGraphQL(ctx context.Context, item GraphQLInsertItem) error {
	writer := sgraphql.NewWriterFromQueries(c.q)

	if err := writer.Create(ctx, item.GraphQL); err != nil {
		return err
	}

	c.track(Event{
		Entity:      EntityGraphQL,
		Op:          OpInsert,
		ID:          item.GraphQL.ID,
		WorkspaceID: item.WorkspaceID,
		Payload:     item.GraphQL,
	})

	return nil
}

// InsertGraphQLBatch inserts multiple GraphQL entries.
func (c *Context) InsertGraphQLBatch(ctx context.Context, items []GraphQLInsertItem) error {
	for _, item := range items {
		if err := c.InsertGraphQL(ctx, item); err != nil {
			return err
		}
	}
	return nil
}
