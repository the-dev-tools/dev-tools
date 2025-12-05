package sworkspace_test

import (
	"context"
	"fmt"
	"testing"

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
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	defer cleanup()

	queries, err := gen.Prepare(ctx, db)
	if err != nil {
		t.Fatalf("failed to prepare queries: %v", err)
	}

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
	if err := userService.CreateUser(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}

	createWS := func(name string, order float64) idwrap.IDWrap {
		wsID := idwrap.NewNow()
		ws := &mworkspace.Workspace{
			ID:           wsID,
			Name:         name,
			Updated:      dbtime.DBNow(),
			Order:        order,
		}
		if err := wsService.Create(ctx, ws); err != nil {
			t.Fatalf("create workspace: %v", err)
		}
		if err := wusService.CreateWorkspaceUser(ctx, &mworkspaceuser.WorkspaceUser{
			ID:          idwrap.NewNow(),
			WorkspaceID: wsID,
			UserID:      userID,
			Role:        mworkspaceuser.RoleOwner,
		}); err != nil {
			t.Fatalf("create workspace user: %v", err)
		}
		return wsID
	}

	ws1 := createWS("WS1", 0)
	ws2 := createWS("WS2", 1)
	ws3 := createWS("WS3", 2)

	// Verify initial state
	list, _ := wsService.GetWorkspacesByUserIDOrdered(ctx, userID)
	if len(list) != 3 {
		t.Fatalf("setup failed, expected 3 workspaces, got %d", len(list))
	}

	// Verify order is respected (0, 1, 2)
	if list[0].ID.Compare(ws1) != 0 {
		t.Errorf("Expected first workspace to be WS1")
	}
	if list[1].ID.Compare(ws2) != 0 {
		t.Errorf("Expected second workspace to be WS2")
	}
	if list[2].ID.Compare(ws3) != 0 {
		t.Errorf("Expected third workspace to be WS3")
	}

	// Delete WS2 (Middle)
	if err := wsService.Delete(ctx, userID, ws2); err != nil {
		t.Fatalf("delete ws2: %v", err)
	}

	// Check list again
	listAfter, err := wsService.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	
	if len(listAfter) != 2 {
		t.Errorf("Expected 2 workspaces, got %d", len(listAfter))
	}
	
	// Verify remaining order
	if listAfter[0].ID.Compare(ws1) != 0 {
		t.Errorf("Expected first workspace to be WS1")
	}
	if listAfter[1].ID.Compare(ws3) != 0 {
		t.Errorf("Expected second workspace to be WS3")
	}
}
