package mutation

import (
	"context"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mcredential"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/scredential"
)

// CredentialInsertItem represents a credential to insert.
type CredentialInsertItem struct {
	Credential  *mcredential.Credential
	WorkspaceID idwrap.IDWrap
}

// InsertCredential inserts a credential and tracks the event.
func (c *Context) InsertCredential(ctx context.Context, item CredentialInsertItem) error {
	writer := scredential.NewCredentialWriterFromQueries(c.q)

	if err := writer.CreateCredential(ctx, item.Credential); err != nil {
		return err
	}

	c.track(Event{
		Entity:      EntityCredential,
		Op:          OpInsert,
		ID:          item.Credential.ID,
		WorkspaceID: item.WorkspaceID,
		Payload:     item.Credential,
	})

	return nil
}

// InsertCredentialBatch inserts multiple credentials.
func (c *Context) InsertCredentialBatch(ctx context.Context, items []CredentialInsertItem) error {
	for _, item := range items {
		if err := c.InsertCredential(ctx, item); err != nil {
			return err
		}
	}
	return nil
}
