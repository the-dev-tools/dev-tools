package sworkspace_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/muser"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/model/mworkspaceuser"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/service/sworkspacesusers"
	"the-dev-tools/server/pkg/testutil"
)

func TestAutoLinkWorkspaceToUserList(t *testing.T) {
	ctx := context.Background()
	
	t.Run("first workspace for user - should remain isolated", func(t *testing.T) {
		base := testutil.CreateBaseDB(ctx, t)
		queries := base.Queries
		ws := sworkspace.New(queries)
		wus := sworkspacesusers.New(queries)
		us := suser.New(queries)
		
		// Create user
		userID := idwrap.NewNow()
		user := &muser.User{
			ID:           userID,
			Email:        "test@example.com",
			Password:     []byte("password"),
			ProviderType: muser.MagicLink,
			ProviderID:   nil,
			Status:       muser.Active,
		}
		err := us.CreateUser(ctx, user)
		if err != nil {
			t.Fatal(err)
		}
		
		// Create first workspace
		workspaceID := idwrap.NewNow()
		workspace := &mworkspace.Workspace{
			ID:      workspaceID,
			Name:    "First Workspace",
			Updated: dbtime.DBNow(),
		}
		err = ws.Create(ctx, workspace)
		if err != nil {
			t.Fatal(err)
		}
		
		// Create workspace user
		wsUser := &mworkspaceuser.WorkspaceUser{
			ID:          idwrap.NewNow(),
			WorkspaceID: workspaceID,
			UserID:      userID,
			Role:        mworkspaceuser.RoleOwner,
		}
		err = wus.CreateWorkspaceUser(ctx, wsUser)
		if err != nil {
			t.Fatal(err)
		}
		
		// Test auto-linking for first workspace
		err = ws.AutoLinkWorkspaceToUserList(ctx, workspaceID, userID)
		if err != nil {
			t.Fatal(err)
		}
		
		// Verify workspace state - should be isolated (prev=NULL, next=NULL) for first workspace
		var prev, next sql.NullString
		err = base.DB.QueryRow("SELECT prev, next FROM workspaces WHERE id = ?", workspaceID).Scan(&prev, &next)
		if err != nil {
			t.Fatal(err)
		}
		
		if prev.Valid || next.Valid {
			t.Errorf("First workspace should remain isolated (prev=NULL, next=NULL), got prev=%v, next=%v", prev.Valid, next.Valid)
		}
		
		// Verify it appears in ordered list
		workspaces, err := ws.GetWorkspacesByUserIDOrdered(ctx, userID)
		if err != nil {
			t.Fatal(err)
		}
		
		if len(workspaces) != 1 {
			t.Fatalf("Expected 1 workspace, got %d", len(workspaces))
		}
		
		if workspaces[0].ID.Compare(workspaceID) != 0 {
			t.Errorf("Expected workspace ID %s, got %s", workspaceID.String(), workspaces[0].ID.String())
		}
	})
	
	t.Run("second workspace - should be linked to end of chain", func(t *testing.T) {
		base := testutil.CreateBaseDB(ctx, t)
		queries := base.Queries
		ws := sworkspace.New(queries)
		wus := sworkspacesusers.New(queries)
		us := suser.New(queries)
		
		// Create user
		userID := idwrap.NewNow()
		user := &muser.User{
			ID:           userID,
			Email:        "test2@example.com",
			Password:     []byte("password"),
			ProviderType: muser.MagicLink,
			ProviderID:   nil,
			Status:       muser.Active,
		}
		err := us.CreateUser(ctx, user)
		if err != nil {
			t.Fatal(err)
		}
		
		// Create first workspace
		workspace1ID := idwrap.NewNow()
		workspace1 := &mworkspace.Workspace{
			ID:      workspace1ID,
			Name:    "First Workspace",
			Updated: dbtime.DBNow(),
		}
		err = ws.Create(ctx, workspace1)
		if err != nil {
			t.Fatal(err)
		}
		
		wsUser1 := &mworkspaceuser.WorkspaceUser{
			ID:          idwrap.NewNow(),
			WorkspaceID: workspace1ID,
			UserID:      userID,
			Role:        mworkspaceuser.RoleOwner,
		}
		err = wus.CreateWorkspaceUser(ctx, wsUser1)
		if err != nil {
			t.Fatal(err)
		}
		
		// Auto-link first workspace
		err = ws.AutoLinkWorkspaceToUserList(ctx, workspace1ID, userID)
		if err != nil {
			t.Fatal(err)
		}
		
		// Create second workspace
		workspace2ID := idwrap.NewNow()
		workspace2 := &mworkspace.Workspace{
			ID:      workspace2ID,
			Name:    "Second Workspace",
			Updated: dbtime.DBNow(),
		}
		err = ws.Create(ctx, workspace2)
		if err != nil {
			t.Fatal(err)
		}
		
		wsUser2 := &mworkspaceuser.WorkspaceUser{
			ID:          idwrap.NewNow(),
			WorkspaceID: workspace2ID,
			UserID:      userID,
			Role:        mworkspaceuser.RoleOwner,
		}
		err = wus.CreateWorkspaceUser(ctx, wsUser2)
		if err != nil {
			t.Fatal(err)
		}
		
		// Auto-link second workspace - this should link it to the end
		err = ws.AutoLinkWorkspaceToUserList(ctx, workspace2ID, userID)
		if err != nil {
			t.Fatal(err)
		}
		
		// Verify workspace state
		var ws1Prev, ws1Next, ws2Prev, ws2Next sql.NullString
		err = base.DB.QueryRow("SELECT prev, next FROM workspaces WHERE id = ?", workspace1ID).Scan(&ws1Prev, &ws1Next)
		if err != nil {
			t.Fatal(err)
		}
		err = base.DB.QueryRow("SELECT prev, next FROM workspaces WHERE id = ?", workspace2ID).Scan(&ws2Prev, &ws2Next)
		if err != nil {
			t.Fatal(err)
		}
		
		// First workspace should be head (prev=NULL, next=workspace2ID)
		if ws1Prev.Valid {
			t.Errorf("First workspace should have prev=NULL, got valid=%v", ws1Prev.Valid)
		}
		if !ws1Next.Valid || string(ws1Next.String) != string(workspace2ID.Bytes()) {
			t.Errorf("First workspace should point to second workspace")
		}
		
		// Second workspace should be tail (prev=workspace1ID, next=NULL)
		if !ws2Prev.Valid || string(ws2Prev.String) != string(workspace1ID.Bytes()) {
			t.Errorf("Second workspace should point back to first workspace")
		}
		if ws2Next.Valid {
			t.Errorf("Second workspace should have next=NULL, got valid=%v", ws2Next.Valid)
		}
		
		// Verify ordered list
		workspaces, err := ws.GetWorkspacesByUserIDOrdered(ctx, userID)
		if err != nil {
			t.Fatal(err)
		}
		
		if len(workspaces) != 2 {
			t.Fatalf("Expected 2 workspaces, got %d", len(workspaces))
		}
		
		if workspaces[0].ID.Compare(workspace1ID) != 0 {
			t.Errorf("Expected first workspace in position 0, got %s", workspaces[0].ID.String())
		}
		if workspaces[1].ID.Compare(workspace2ID) != 0 {
			t.Errorf("Expected second workspace in position 1, got %s", workspaces[1].ID.String())
		}
	})
	
	t.Run("third workspace - should be linked to end of chain", func(t *testing.T) {
		base := testutil.CreateBaseDB(ctx, t)
		queries := base.Queries
		ws := sworkspace.New(queries)
		wus := sworkspacesusers.New(queries)
		us := suser.New(queries)
		
		// Create user
		userID := idwrap.NewNow()
		user := &muser.User{
			ID:           userID,
			Email:        "test3@example.com",
			Password:     []byte("password"),
			ProviderType: muser.MagicLink,
			ProviderID:   nil,
			Status:       muser.Active,
		}
		err := us.CreateUser(ctx, user)
		if err != nil {
			t.Fatal(err)
		}
		
		// Create and link three workspaces
		workspaceIDs := make([]idwrap.IDWrap, 3)
		for i := 0; i < 3; i++ {
			workspaceIDs[i] = idwrap.NewNow()
			workspace := &mworkspace.Workspace{
				ID:      workspaceIDs[i],
				Name:    fmt.Sprintf("Workspace %d", i+1),
				Updated: dbtime.DBNow(),
			}
			err = ws.Create(ctx, workspace)
			if err != nil {
				t.Fatal(err)
			}
			
			wsUser := &mworkspaceuser.WorkspaceUser{
				ID:          idwrap.NewNow(),
				WorkspaceID: workspaceIDs[i],
				UserID:      userID,
				Role:        mworkspaceuser.RoleOwner,
			}
			err = wus.CreateWorkspaceUser(ctx, wsUser)
			if err != nil {
				t.Fatal(err)
			}
			
			err = ws.AutoLinkWorkspaceToUserList(ctx, workspaceIDs[i], userID)
			if err != nil {
				t.Fatal(err)
			}
		}
		
		// Verify ordered list has correct order
		workspaces, err := ws.GetWorkspacesByUserIDOrdered(ctx, userID)
		if err != nil {
			t.Fatal(err)
		}
		
		if len(workspaces) != 3 {
			t.Fatalf("Expected 3 workspaces, got %d", len(workspaces))
		}
		
		// Check order matches creation order
		for i, workspace := range workspaces {
			if workspace.ID.Compare(workspaceIDs[i]) != 0 {
				t.Errorf("Expected workspace %d in position %d, got %s", i, i, workspace.ID.String())
			}
		}
		
		// Verify linked list integrity
		var ws1Prev, ws1Next, ws2Prev, ws2Next, ws3Prev, ws3Next sql.NullString
		err = base.DB.QueryRow("SELECT prev, next FROM workspaces WHERE id = ?", workspaceIDs[0]).Scan(&ws1Prev, &ws1Next)
		if err != nil {
			t.Fatal(err)
		}
		err = base.DB.QueryRow("SELECT prev, next FROM workspaces WHERE id = ?", workspaceIDs[1]).Scan(&ws2Prev, &ws2Next)
		if err != nil {
			t.Fatal(err)
		}
		err = base.DB.QueryRow("SELECT prev, next FROM workspaces WHERE id = ?", workspaceIDs[2]).Scan(&ws3Prev, &ws3Next)
		if err != nil {
			t.Fatal(err)
		}
		
		// Check chain: ws1 <-> ws2 <-> ws3
		// ws1: prev=NULL, next=ws2
		if ws1Prev.Valid {
			t.Errorf("Workspace 1 should have prev=NULL")
		}
		if !ws1Next.Valid || string(ws1Next.String) != string(workspaceIDs[1].Bytes()) {
			t.Errorf("Workspace 1 should point to workspace 2")
		}
		
		// ws2: prev=ws1, next=ws3
		if !ws2Prev.Valid || string(ws2Prev.String) != string(workspaceIDs[0].Bytes()) {
			t.Errorf("Workspace 2 should point back to workspace 1")
		}
		if !ws2Next.Valid || string(ws2Next.String) != string(workspaceIDs[2].Bytes()) {
			t.Errorf("Workspace 2 should point to workspace 3")
		}
		
		// ws3: prev=ws2, next=NULL
		if !ws3Prev.Valid || string(ws3Prev.String) != string(workspaceIDs[1].Bytes()) {
			t.Errorf("Workspace 3 should point back to workspace 2")
		}
		if ws3Next.Valid {
			t.Errorf("Workspace 3 should have next=NULL")
		}
	})
	
	t.Run("no workspaces for user - should handle gracefully", func(t *testing.T) {
		base := testutil.CreateBaseDB(ctx, t)
		queries := base.Queries
		ws := sworkspace.New(queries)
		us := suser.New(queries)
		
		// Create user
		userID := idwrap.NewNow()
		user := &muser.User{
			ID:           userID,
			Email:        "noworkspaces@example.com",
			Password:     []byte("password"),
			ProviderType: muser.MagicLink,
			ProviderID:   nil,
			Status:       muser.Active,
		}
		err := us.CreateUser(ctx, user)
		if err != nil {
			t.Fatal(err)
		}
		
		// Create a workspace but don't create workspace_user yet
		workspaceID := idwrap.NewNow()
		workspace := &mworkspace.Workspace{
			ID:      workspaceID,
			Name:    "Orphan Workspace",
			Updated: dbtime.DBNow(),
		}
		err = ws.Create(ctx, workspace)
		if err != nil {
			t.Fatal(err)
		}
		
		// Try to auto-link - should succeed but find no existing workspaces
		err = ws.AutoLinkWorkspaceToUserList(ctx, workspaceID, userID)
		if err != nil {
			t.Fatal(err)
		}
		
		// The workspace should remain isolated since no workspace_user exists
		var prev, next sql.NullString
		err = base.DB.QueryRow("SELECT prev, next FROM workspaces WHERE id = ?", workspaceID).Scan(&prev, &next)
		if err != nil {
			t.Fatal(err)
		}
		
		if prev.Valid || next.Valid {
			t.Errorf("Workspace should remain isolated when no workspace_user exists, got prev=%v, next=%v", prev.Valid, next.Valid)
		}
	})
	
	t.Run("workspace not in user list - should handle gracefully", func(t *testing.T) {
		base := testutil.CreateBaseDB(ctx, t)
		queries := base.Queries
		ws := sworkspace.New(queries)
		
		// Try to auto-link non-existent workspace
		fakeWorkspaceID := idwrap.NewNow()
		fakeUserID := idwrap.NewNow()
		
		// This should not fail, but should do nothing
		err := ws.AutoLinkWorkspaceToUserList(ctx, fakeWorkspaceID, fakeUserID)
		if err != nil {
			t.Fatal(err)
		}
	})
	
	t.Run("transaction isolation - auto-link works within transaction", func(t *testing.T) {
		base := testutil.CreateBaseDB(ctx, t)
		queries := base.Queries
		ws := sworkspace.New(queries)
		us := suser.New(queries)
		
		// Create user
		userID := idwrap.NewNow()
		user := &muser.User{
			ID:           userID,
			Email:        "tx@example.com",
			Password:     []byte("password"),
			ProviderType: muser.MagicLink,
			ProviderID:   nil,
			Status:       muser.Active,
		}
		err := us.CreateUser(ctx, user)
		if err != nil {
			t.Fatal(err)
		}
		
		// Start transaction
		tx, err := base.DB.Begin()
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		
		// Create workspace within transaction
		workspaceServiceTX, err := sworkspace.NewTX(ctx, tx)
		if err != nil {
			t.Fatal(err)
		}
		
		workspaceID := idwrap.NewNow()
		workspace := &mworkspace.Workspace{
			ID:      workspaceID,
			Name:    "TX Workspace",
			Updated: dbtime.DBNow(),
		}
		err = workspaceServiceTX.Create(ctx, workspace)
		if err != nil {
			t.Fatal(err)
		}
		
		// Create workspace user within transaction
		workspaceUserServiceTX, err := sworkspacesusers.NewTX(ctx, tx)
		if err != nil {
			t.Fatal(err)
		}
		
		wsUser := &mworkspaceuser.WorkspaceUser{
			ID:          idwrap.NewNow(),
			WorkspaceID: workspaceID,
			UserID:      userID,
			Role:        mworkspaceuser.RoleOwner,
		}
		err = workspaceUserServiceTX.CreateWorkspaceUser(ctx, wsUser)
		if err != nil {
			t.Fatal(err)
		}
		
		// Auto-link within transaction
		err = workspaceServiceTX.AutoLinkWorkspaceToUserList(ctx, workspaceID, userID)
		if err != nil {
			t.Fatal(err)
		}
		
		// Commit transaction
		err = tx.Commit()
		if err != nil {
			t.Fatal(err)
		}
		
		// Verify workspace appears in list after transaction commit
		workspaces, err := ws.GetWorkspacesByUserIDOrdered(ctx, userID)
		if err != nil {
			t.Fatal(err)
		}
		
		if len(workspaces) != 1 {
			t.Fatalf("Expected 1 workspace after transaction, got %d", len(workspaces))
		}
		
		if workspaces[0].ID.Compare(workspaceID) != 0 {
			t.Errorf("Expected workspace ID %s, got %s", workspaceID.String(), workspaces[0].ID.String())
		}
	})
}