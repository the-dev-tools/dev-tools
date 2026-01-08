package mutation

import (
	"context"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

// DeleteWorkspace deletes a workspace and collects cascade events for all children.
// This is a deep cascade - collects events for HTTP children, Flow children, etc.
// Files have FK to workspace so DB CASCADE handles them.
func (c *Context) DeleteWorkspace(ctx context.Context, workspaceID idwrap.IDWrap) error {
	// Collect all workspace children
	c.collectWorkspaceChildren(ctx, workspaceID)

	// Track workspace delete
	c.track(Event{
		Entity:      EntityWorkspace,
		Op:          OpDelete,
		ID:          workspaceID,
		WorkspaceID: workspaceID,
	})

	// Delete - DB CASCADE handles files, HTTPs, Flows, etc.
	return c.q.DeleteWorkspace(ctx, workspaceID)
}

// collectWorkspaceChildren collects cascade events for all workspace contents.
// Files have FK to workspace so DB CASCADE handles deletion.
func (c *Context) collectWorkspaceChildren(ctx context.Context, workspaceID idwrap.IDWrap) {
	// HTTPs - each cascades to its children
	if https, err := c.q.GetHTTPsByWorkspaceID(ctx, workspaceID); err == nil {
		for i := range https {
			// Collect HTTP children (headers, params, etc.)
			c.collectHTTPChildren(ctx, https[i].ID, workspaceID)
			// Track HTTP delete
			c.track(Event{
				Entity:      EntityHTTP,
				Op:          OpDelete,
				ID:          https[i].ID,
				WorkspaceID: workspaceID,
				IsDelta:     https[i].IsDelta,
			})
		}
	}

	// Flows - each cascades to its children
	if flows, err := c.q.GetFlowsByWorkspaceID(ctx, workspaceID); err == nil {
		for i := range flows {
			// Collect Flow children (nodes, edges, variables)
			c.collectFlowChildren(ctx, flows[i].ID, workspaceID)
			// Track Flow delete
			c.track(Event{
				Entity:      EntityFlow,
				Op:          OpDelete,
				ID:          flows[i].ID,
				WorkspaceID: workspaceID,
			})
		}
	}

	// Files - DB CASCADE handles deletion, just track events
	if files, err := c.q.GetFilesByWorkspaceID(ctx, workspaceID); err == nil {
		for i := range files {
			c.track(Event{
				Entity:      EntityFile,
				Op:          OpDelete,
				ID:          files[i].ID,
				WorkspaceID: workspaceID,
			})
		}
	}

	// Environments - each cascades to its variables
	if envs, err := c.q.GetEnvironmentsByWorkspaceID(ctx, workspaceID); err == nil {
		for i := range envs {
			// Collect environment variables (children of environment)
			if vars, err := c.q.GetVariablesByEnvironmentID(ctx, envs[i].ID); err == nil {
				for j := range vars {
					c.track(Event{
						Entity:      EntityEnvironmentValue,
						Op:          OpDelete,
						ID:          vars[j].ID,
						WorkspaceID: workspaceID,
						ParentID:    envs[i].ID, // Environment is the parent
					})
				}
			}
			// Track environment delete
			c.track(Event{
				Entity:      EntityEnvironment,
				Op:          OpDelete,
				ID:          envs[i].ID,
				WorkspaceID: workspaceID,
			})
		}
	}

	// Tags
	if tags, err := c.q.GetTagsByWorkspaceID(ctx, workspaceID); err == nil {
		for i := range tags {
			c.track(Event{
				Entity:      EntityTag,
				Op:          OpDelete,
				ID:          tags[i].ID,
				WorkspaceID: workspaceID,
			})
		}
	}

	// Workspace Users
	if users, err := c.q.GetWorkspaceUserByWorkspaceID(ctx, workspaceID); err == nil {
		for i := range users {
			c.track(Event{
				Entity:      EntityWorkspaceUser,
				Op:          OpDelete,
				ID:          users[i].ID,
				WorkspaceID: workspaceID,
			})
		}
	}
}
