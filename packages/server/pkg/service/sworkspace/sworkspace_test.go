package sworkspace_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlitemem"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/dbtime"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/muser"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/suser"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
)

func TestWorkspaceDeletion(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	require.NoError(t, err, "failed to create db")
	defer cleanup()

	queries, err := gen.Prepare(ctx, db)
	require.NoError(t, err, "failed to prepare queries")

	wsService := sworkspace.NewWorkspaceService(queries)
	userService := suser.New(queries)
	wusService := sworkspace.NewUserService(queries)

	// Create user
	userID := idwrap.NewNow()
	providerID := fmt.Sprintf("test-%s", userID.String())
	user := &muser.User{
		ID:           userID,
		Email:        "test@example.com",
		ProviderID:   &providerID,
		ProviderType: muser.MagicLink,
		Status:       muser.Active,
	}
	err = userService.CreateUser(ctx, user)
	require.NoError(t, err, "create user")

	createWS := func(name string, order float64) idwrap.IDWrap {
		wsID := idwrap.NewNow()
		ws := &mworkspace.Workspace{
			ID:      wsID,
			Name:    name,
			Updated: dbtime.DBNow(),
			Order:   order,
		}
		err := wsService.Create(ctx, ws)
		require.NoError(t, err, "create workspace")
		err = wusService.CreateWorkspaceUser(ctx, &mworkspace.WorkspaceUser{
			ID:          idwrap.NewNow(),
			WorkspaceID: wsID,
			UserID:      userID,
			Role:        mworkspace.RoleOwner,
		})
		require.NoError(t, err, "create workspace user")
		return wsID
	}

	ws1 := createWS("WS1", 0)
	ws2 := createWS("WS2", 1)
	ws3 := createWS("WS3", 2)

	// Verify initial state
	list, _ := wsService.GetWorkspacesByUserIDOrdered(ctx, userID)
	require.Len(t, list, 3, "setup failed, expected 3 workspaces")

	// Verify order is respected (0, 1, 2)
	require.Equal(t, 0, list[0].ID.Compare(ws1), "Expected first workspace to be WS1")
	require.Equal(t, 0, list[1].ID.Compare(ws2), "Expected second workspace to be WS2")
	require.Equal(t, 0, list[2].ID.Compare(ws3), "Expected third workspace to be WS3")

	// Delete WS2 (Middle)
	err = wsService.Delete(ctx, userID, ws2)
	require.NoError(t, err, "delete ws2")

	// Check list again
	listAfter, err := wsService.GetWorkspacesByUserIDOrdered(ctx, userID)
	require.NoError(t, err, "list after delete")
	require.Len(t, listAfter, 2, "Expected 2 workspaces")

	// Verify remaining order
	require.Equal(t, 0, listAfter[0].ID.Compare(ws1), "Expected first workspace to be WS1")
	require.Equal(t, 0, listAfter[1].ID.Compare(ws3), "Expected second workspace to be WS3")
}
