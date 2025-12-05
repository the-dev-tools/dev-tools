package sworkspace_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/db/pkg/sqlitemem"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/muser"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/model/mworkspaceuser"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/service/sworkspacesusers"
)

func TestWorkspaceDeletion(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	require.NoError(t, err, "failed to create db")
	defer cleanup()

	queries, err := gen.Prepare(ctx, db)
	require.NoError(t, err, "failed to prepare queries")

	wsService := sworkspace.New(queries)
	userService := suser.New(queries)
	wusService := sworkspacesusers.New(queries)

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
			ID:           wsID,
			Name:         name,
			Updated:      dbtime.DBNow(),
			Order:        order,
		}
		err := wsService.Create(ctx, ws)
		require.NoError(t, err, "create workspace")
		err = wusService.CreateWorkspaceUser(ctx, &mworkspaceuser.WorkspaceUser{
			ID:          idwrap.NewNow(),
			WorkspaceID: wsID,
			UserID:      userID,
			Role:        mworkspaceuser.RoleOwner,
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
