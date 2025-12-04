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

func TestWorkspaceAutoLink(t *testing.T) {
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

	// Helper to create workspace
	createWS := func(name string) idwrap.IDWrap {
		wsID := idwrap.NewNow()
		ws := &mworkspace.Workspace{
			ID:      wsID,
			Name:    name,
			Updated: dbtime.DBNow(),
		}
		if err := wsService.Create(ctx, ws); err != nil {
			t.Fatalf("create workspace %s: %v", name, err)
		}
		
		// Link user to workspace
		wu := &mworkspaceuser.WorkspaceUser{
			ID:          idwrap.NewNow(),
			WorkspaceID: wsID,
			UserID:      userID,
			Role:        mworkspaceuser.RoleOwner,
		}
		if err := wusService.CreateWorkspaceUser(ctx, wu); err != nil {
			t.Fatalf("create workspace user for %s: %v", name, err)
		}

		// AutoLink
		if err := wsService.AutoLinkWorkspaceToUserList(ctx, wsID, userID); err != nil {
			t.Fatalf("autolink workspace %s: %v", name, err)
		}
		return wsID
	}

	// 1. Create first workspace
	ws1 := createWS("WS1")

	// Verify order
	list1, err := wsService.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		t.Fatalf("list 1: %v", err)
	}
	if len(list1) != 1 || list1[0].ID.Compare(ws1) != 0 {
		t.Fatalf("expected [WS1], got %v", list1)
	}

	// 2. Create second workspace
	ws2 := createWS("WS2")

	// Verify order
	list2, err := wsService.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		t.Fatalf("list 2: %v", err)
	}
	if len(list2) != 2 {
		t.Fatalf("expected 2 workspaces, got %d", len(list2))
	}
	if list2[0].ID.Compare(ws1) != 0 {
		t.Errorf("expected first to be WS1, got %s", list2[0].Name)
	}
	if list2[1].ID.Compare(ws2) != 0 {
		t.Errorf("expected second to be WS2, got %s", list2[1].Name)
	}

	// 3. Create third workspace
	ws3 := createWS("WS3")

	// Verify order
	list3, err := wsService.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		t.Fatalf("list 3: %v", err)
	}
	if len(list3) != 3 {
		t.Fatalf("expected 3 workspaces, got %d", len(list3))
	}
	if list3[2].ID.Compare(ws3) != 0 {
		t.Errorf("expected third to be WS3, got %s", list3[2].Name)
	}
}

func TestWorkspaceDeleteBreaksChain(t *testing.T) {
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
	userService.CreateUser(ctx, user)

	createWS := func(name string) idwrap.IDWrap {
		wsID := idwrap.NewNow()
		ws := &mworkspace.Workspace{
			ID:      wsID,
			Name:    name,
			Updated: dbtime.DBNow(),
		}
		wsService.Create(ctx, ws)
		wusService.CreateWorkspaceUser(ctx, &mworkspaceuser.WorkspaceUser{
			ID:          idwrap.NewNow(),
			WorkspaceID: wsID,
			UserID:      userID,
			Role:        mworkspaceuser.RoleOwner,
		})
		wsService.AutoLinkWorkspaceToUserList(ctx, wsID, userID)
		return wsID
	}

	ws1 := createWS("WS1")
	ws2 := createWS("WS2")
	ws3 := createWS("WS3")

	// Verify initial state
	list, _ := wsService.GetWorkspacesByUserIDOrdered(ctx, userID)
	if len(list) != 3 {
		t.Fatalf("setup failed, expected 3 workspaces, got %d", len(list))
	}

	// Delete WS2 (Middle)
	if err := wsService.Delete(ctx, ws2); err != nil {
		t.Fatalf("delete ws2: %v", err)
	}

	// Check list again
	listAfter, err := wsService.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	
	fmt.Printf("List after deleting WS2: %v\n", listAfter)
	
	// Expectation: If chain is broken, we lose WS3.
	if len(listAfter) != 2 {
		t.Errorf("BROKEN CHAIN DETECTED: Expected 2 workspaces (WS1, WS3), got %d", len(listAfter))
		for _, w := range listAfter {
			t.Logf("Found: %s (%s)", w.Name, w.ID)
		}
		// If we only find WS1
		if len(listAfter) == 1 {
			if listAfter[0].ID.Compare(ws1) == 0 {
				t.Log("Only WS1 found (Head). WS3 is lost because WS2 link is broken.")
			} else if listAfter[0].ID.Compare(ws3) == 0 {
				t.Log("Only WS3 found? Unexpected.")
			}
		}
	}
}