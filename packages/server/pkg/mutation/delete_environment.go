package mutation

import (
	"context"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

// DeleteEnvironment deletes an environment and collects cascade events for all variables.
func (c *Context) DeleteEnvironment(ctx context.Context, envID, workspaceID idwrap.IDWrap) error {
	// Collect environment variables (children of environment)
	if vars, err := c.q.GetVariablesByEnvironmentID(ctx, envID); err == nil {
		for i := range vars {
			c.track(Event{
				Entity:      EntityEnvironmentValue,
				Op:          OpDelete,
				ID:          vars[i].ID,
				WorkspaceID: workspaceID,
				ParentID:    envID, // Environment is the parent
			})
		}
	}

	// Track environment delete
	c.track(Event{
		Entity:      EntityEnvironment,
		Op:          OpDelete,
		ID:          envID,
		WorkspaceID: workspaceID,
	})

	// Delete - DB CASCADE handles variables
	return c.q.DeleteEnvironment(ctx, envID)
}
