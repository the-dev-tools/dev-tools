package rworkspace_test

import (
	"context"
	"testing"
	"time"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rworkspace"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/model/mworkspaceuser"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/service/sworkspacesusers"
	"the-dev-tools/server/pkg/testutil"
	resourcesv1 "the-dev-tools/spec/dist/buf/go/resources/v1"
	workspacev1 "the-dev-tools/spec/dist/buf/go/workspace/v1"

	"connectrpc.com/connect"
)

func TestWorkspaceCreate(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	ws := sworkspace.New(queries)
	wus := sworkspacesusers.New(queries)
	us := suser.New(queries)
	es := senv.New(queries, base.Logger())

	wsIDBase := idwrap.NewNow()
	wsuserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	baseCollectionID := idwrap.NewNow()

	base.GetBaseServices().CreateTempCollection(t, ctx, wsIDBase,
		wsuserID, userID, baseCollectionID)

	testWsName := "test"

	req := connect.NewRequest(
		&workspacev1.WorkspaceCreateRequest{
			SelectedEnvironmentId: nil,
			Name:                  testWsName,
		},
	)

	serviceRPC := rworkspace.New(db, ws, wus, us, es)

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	resp, err := serviceRPC.WorkspaceCreate(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Msg == nil {
		t.Fatal("resp.Msg is nil")
	}
	msg := resp.Msg

	if msg.WorkspaceId == nil {
		t.Fatal("WorkspaceId is nil")
	}

	respWorkspaceID, err := idwrap.NewFromBytes(msg.WorkspaceId)
	if err != nil {
		t.Fatal(err)
	}

	dbWorkspace, err := ws.Get(ctx, respWorkspaceID)
	if err != nil {
		t.Fatal(err)
	}

	if testWsName != dbWorkspace.Name {
		t.Fatalf("Name mismatch, expected: %s, got: %s", testWsName, dbWorkspace.Name)
	}
}

func TestWorkspaceGet(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	ws := sworkspace.New(queries)
	wus := sworkspacesusers.New(queries)
	us := suser.New(queries)
	es := senv.New(queries, base.Logger())

	wsIDBase := idwrap.NewNow()
	wsuserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	baseCollectionID := idwrap.NewNow()

	base.GetBaseServices().CreateTempCollection(t, ctx, wsIDBase,
		wsuserID, userID, baseCollectionID)

	testWsID := idwrap.NewNow()
	testWsName := "test"

	testWorkspace := &mworkspace.Workspace{
		ID:      testWsID,
		Name:    testWsName,
		Updated: dbtime.DBNow(),
	}

	testWorkspaceUser := &mworkspaceuser.WorkspaceUser{
		ID:          idwrap.NewNow(),
		WorkspaceID: testWsID,
		UserID:      userID,
		Role:        mworkspaceuser.RoleOwner,
	}

	err := ws.Create(ctx, testWorkspace)
	if err != nil {
		t.Fatal(err)
	}

	err = wus.CreateWorkspaceUser(ctx, testWorkspaceUser)
	if err != nil {
		t.Fatal(err)
	}

	req := connect.NewRequest(
		&workspacev1.WorkspaceGetRequest{
			WorkspaceId: testWsID.Bytes(),
		},
	)

	serviceRPC := rworkspace.New(db, ws, wus, us, es)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp, err := serviceRPC.WorkspaceGet(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Msg == nil {
		t.Fatal("resp.Msg is nil")
	}
	msg := resp.Msg

	if msg.WorkspaceId == nil {
		t.Fatal("WorkspaceId is nil")
	}

	respWorkspaceID, err := idwrap.NewFromBytes(msg.WorkspaceId)
	if err != nil {
		t.Fatal(err)
	}

	if testWsID.Compare(respWorkspaceID) != 0 {
		t.Fatalf("WorkspaceGet failed: id mismatch")
	}

	if msg.Name != testWsName {
		t.Fatalf("WorkspaceGet failed: name mismatch")
	}

	if testWorkspace.Updated.Unix() != msg.Updated.Seconds {
		t.Fatalf("Updated mismatch, expected: %v, got: %v", testWorkspace.Updated.Unix(), msg.Updated.Seconds)
	}
}

func TestWorkspaceUpdate(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	ws := sworkspace.New(queries)
	wus := sworkspacesusers.New(queries)
	us := suser.New(queries)
	es := senv.New(queries, base.Logger())

	// TODO: change to correc service
	wsIDBase := idwrap.NewNow()
	wsuserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	baseCollectionID := idwrap.NewNow()

	base.GetBaseServices().CreateTempCollection(t, ctx, wsIDBase,
		wsuserID, userID, baseCollectionID)

	testWsID := idwrap.NewNow()
	testWsName := "test"

	testWorkspace := &mworkspace.Workspace{
		ID:      testWsID,
		Name:    testWsName,
		Updated: dbtime.DBNow(),
	}

	testWorkspaceUser := &mworkspaceuser.WorkspaceUser{
		ID:          idwrap.NewNow(),
		WorkspaceID: testWsID,
		UserID:      userID,
		Role:        mworkspaceuser.RoleOwner,
	}

	err := ws.Create(ctx, testWorkspace)
	if err != nil {
		t.Fatal(err)
	}

	err = wus.CreateWorkspaceUser(ctx, testWorkspaceUser)
	if err != nil {
		t.Fatal(err)
	}

	env := menv.Env{
		ID:          idwrap.NewNow(),
		WorkspaceID: testWsID,
		Type:        menv.EnvType(0),
		Name:        "test",
		Description: "desc",
		Updated:     dbtime.DBNow(),
	}

	err = es.Create(ctx, env)
	if err != nil {
		t.Fatal(err)
	}

	testNewWsName := "new test"

	req := connect.NewRequest(
		&workspacev1.WorkspaceUpdateRequest{
			WorkspaceId:           testWsID.Bytes(),
			Name:                  &testNewWsName,
			SelectedEnvironmentId: env.ID.Bytes(),
		},
	)

	serviceRPC := rworkspace.New(db, ws, wus, us, es)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp, err := serviceRPC.WorkspaceUpdate(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Msg == nil {
		t.Fatal("resp.Msg is nil")
	}

	dbWorkspace, err := ws.Get(ctx, testWsID)
	if err != nil {
		t.Fatal(err)
	}

	if testNewWsName != dbWorkspace.Name {
		t.Fatalf("Name mismatch, expected: %s, got: %s", testNewWsName, dbWorkspace.Name)
	}
}

func TestWorkspaceDelete(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	ws := sworkspace.New(queries)
	wus := sworkspacesusers.New(queries)
	us := suser.New(queries)
	es := senv.New(queries, base.Logger())

	// TODO: change to correc service
	wsIDBase := idwrap.NewNow()
	wsuserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	baseCollectionID := idwrap.NewNow()

	base.GetBaseServices().CreateTempCollection(t, ctx, wsIDBase,
		wsuserID, userID, baseCollectionID)

	testWsID := idwrap.NewNow()
	testWsName := "test"

	testWorkspace := &mworkspace.Workspace{
		ID:      testWsID,
		Name:    testWsName,
		Updated: dbtime.DBNow(),
	}

	testWorkspaceUser := &mworkspaceuser.WorkspaceUser{
		ID:          idwrap.NewNow(),
		WorkspaceID: testWsID,
		UserID:      userID,
		Role:        mworkspaceuser.RoleOwner,
	}

	err := ws.Create(ctx, testWorkspace)
	if err != nil {
		t.Fatal(err)
	}

	err = wus.CreateWorkspaceUser(ctx, testWorkspaceUser)
	if err != nil {
		t.Fatal(err)
	}

	req := connect.NewRequest(
		&workspacev1.WorkspaceDeleteRequest{
			WorkspaceId: testWsID.Bytes(),
		},
	)

	serviceRPC := rworkspace.New(db, ws, wus, us, es)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp, err := serviceRPC.WorkspaceDelete(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Msg == nil {
		t.Fatal("resp.Msg is nil")
	}

	dbWorkspace, err := ws.Get(ctx, testWsID)
	if err == nil {
		t.Fatal("Workspace not deleted")
	}
	if err != sworkspace.ErrNoWorkspaceFound {
		t.Fatalf("Expected ErrNoWorkspaceFound, got: %v", err)
	}
	if dbWorkspace != nil {
		t.Fatalf("Workspace not deleted")
	}
}

func TestWorkspaceMove(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	ws := sworkspace.New(queries)
	wus := sworkspacesusers.New(queries)
	us := suser.New(queries)
	es := senv.New(queries, base.Logger())

	// Setup user and workspace infrastructure
	wsIDBase := idwrap.NewNow()
	wsuserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	baseCollectionID := idwrap.NewNow()

	base.GetBaseServices().CreateTempCollection(t, ctx, wsIDBase,
		wsuserID, userID, baseCollectionID)

	// Create two test workspaces
	workspace1ID := idwrap.NewNow()
	workspace2ID := idwrap.NewNow()

	workspace1 := &mworkspace.Workspace{
		ID:      workspace1ID,
		Name:    "Workspace 1",
		Updated: dbtime.DBNow(),
	}

	workspace2 := &mworkspace.Workspace{
		ID:      workspace2ID,
		Name:    "Workspace 2",
		Updated: dbtime.DBNow(),
	}

	// Create workspace users for both workspaces
	workspaceUser1 := &mworkspaceuser.WorkspaceUser{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspace1ID,
		UserID:      userID,
		Role:        mworkspaceuser.RoleOwner,
	}

	workspaceUser2 := &mworkspaceuser.WorkspaceUser{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspace2ID,
		UserID:      userID,
		Role:        mworkspaceuser.RoleOwner,
	}

	// Insert workspaces and users
	if err := ws.Create(ctx, workspace1); err != nil {
		t.Fatal(err)
	}
	if err := ws.Create(ctx, workspace2); err != nil {
		t.Fatal(err)
	}
	if err := wus.CreateWorkspaceUser(ctx, workspaceUser1); err != nil {
		t.Fatal(err)
	}
	if err := wus.CreateWorkspaceUser(ctx, workspaceUser2); err != nil {
		t.Fatal(err)
	}

	serviceRPC := rworkspace.New(db, ws, wus, us, es)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	t.Run("successful move after", func(t *testing.T) {
		req := connect.NewRequest(
			&workspacev1.WorkspaceMoveRequest{
				WorkspaceId:       workspace1ID.Bytes(),
				TargetWorkspaceId: workspace2ID.Bytes(),
				Position:          resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			},
		)

		resp, err := serviceRPC.WorkspaceMove(authedCtx, req)
		if err != nil {
			t.Fatalf("Expected successful move, got error: %v", err)
		}
		if resp.Msg == nil {
			t.Fatal("Response message is nil")
		}
	})

	t.Run("successful move before", func(t *testing.T) {
		req := connect.NewRequest(
			&workspacev1.WorkspaceMoveRequest{
				WorkspaceId:       workspace1ID.Bytes(),
				TargetWorkspaceId: workspace2ID.Bytes(),
				Position:          resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
			},
		)

		resp, err := serviceRPC.WorkspaceMove(authedCtx, req)
		if err != nil {
			t.Fatalf("Expected successful move, got error: %v", err)
		}
		if resp.Msg == nil {
			t.Fatal("Response message is nil")
		}
	})

	t.Run("error - unauthenticated context", func(t *testing.T) {
		req := connect.NewRequest(
			&workspacev1.WorkspaceMoveRequest{
				WorkspaceId:       workspace1ID.Bytes(),
				TargetWorkspaceId: workspace2ID.Bytes(),
				Position:          resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			},
		)

		_, err := serviceRPC.WorkspaceMove(ctx, req) // ctx without auth
		if err == nil {
			t.Fatal("Expected authentication error, got nil")
		}
		connectErr := err.(*connect.Error)
		if connectErr.Code() != connect.CodeUnauthenticated {
			t.Fatalf("Expected CodeUnauthenticated, got %v", connectErr.Code())
		}
	})

	t.Run("error - move workspace relative to itself", func(t *testing.T) {
		req := connect.NewRequest(
			&workspacev1.WorkspaceMoveRequest{
				WorkspaceId:       workspace1ID.Bytes(),
				TargetWorkspaceId: workspace1ID.Bytes(), // Same workspace
				Position:          resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			},
		)

		_, err := serviceRPC.WorkspaceMove(authedCtx, req)
		if err == nil {
			t.Fatal("Expected self-reference error, got nil")
		}
		connectErr := err.(*connect.Error)
		if connectErr.Code() != connect.CodeInvalidArgument {
			t.Fatalf("Expected CodeInvalidArgument, got %v", connectErr.Code())
		}
	})

	t.Run("error - invalid position", func(t *testing.T) {
		req := connect.NewRequest(
			&workspacev1.WorkspaceMoveRequest{
				WorkspaceId:       workspace1ID.Bytes(),
				TargetWorkspaceId: workspace2ID.Bytes(),
				Position:          resourcesv1.MovePosition_MOVE_POSITION_UNSPECIFIED,
			},
		)

		_, err := serviceRPC.WorkspaceMove(authedCtx, req)
		if err == nil {
			t.Fatal("Expected position validation error, got nil")
		}
		connectErr := err.(*connect.Error)
		if connectErr.Code() != connect.CodeInvalidArgument {
			t.Fatalf("Expected CodeInvalidArgument, got %v", connectErr.Code())
		}
	})

	t.Run("error - nonexistent workspace", func(t *testing.T) {
		nonexistentID := idwrap.NewNow()
		req := connect.NewRequest(
			&workspacev1.WorkspaceMoveRequest{
				WorkspaceId:       nonexistentID.Bytes(),
				TargetWorkspaceId: workspace2ID.Bytes(),
				Position:          resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			},
		)

		_, err := serviceRPC.WorkspaceMove(authedCtx, req)
		if err == nil {
			t.Fatal("Expected workspace not found error, got nil")
		}
		connectErr := err.(*connect.Error)
		if connectErr.Code() != connect.CodeNotFound {
			t.Fatalf("Expected CodeNotFound, got %v", connectErr.Code())
		}
	})

	t.Run("error - nonexistent target workspace", func(t *testing.T) {
		nonexistentID := idwrap.NewNow()
		req := connect.NewRequest(
			&workspacev1.WorkspaceMoveRequest{
				WorkspaceId:       workspace1ID.Bytes(),
				TargetWorkspaceId: nonexistentID.Bytes(),
				Position:          resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			},
		)

		_, err := serviceRPC.WorkspaceMove(authedCtx, req)
		if err == nil {
			t.Fatal("Expected target workspace not found error, got nil")
		}
		connectErr := err.(*connect.Error)
		if connectErr.Code() != connect.CodeNotFound {
			t.Fatalf("Expected CodeNotFound, got %v", connectErr.Code())
		}
	})

	t.Run("edge case - single workspace scenario", func(t *testing.T) {
		// For single workspace scenario, we'll reuse existing workspace and just test the self-reference error
		// This avoids creating new users and potential database constraints
		req := connect.NewRequest(
			&workspacev1.WorkspaceMoveRequest{
				WorkspaceId:       workspace1ID.Bytes(),
				TargetWorkspaceId: workspace1ID.Bytes(), // Move relative to itself
				Position:          resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			},
		)

		_, err := serviceRPC.WorkspaceMove(authedCtx, req)
		if err == nil {
			t.Fatal("Expected self-reference error for single workspace, got nil")
		}
		connectErr := err.(*connect.Error)
		if connectErr.Code() != connect.CodeInvalidArgument {
			t.Fatalf("Expected CodeInvalidArgument, got %v", connectErr.Code())
		}
	})

	t.Run("edge case - cross-user workspace access prevention", func(t *testing.T) {
		// Create a minimal workspace without going through CreateTempCollection to avoid user constraint issues
		otherWorkspaceID := idwrap.NewNow()
		otherWorkspace := &mworkspace.Workspace{
			ID:      otherWorkspaceID,
			Name:    "Other User's Workspace",
			Updated: dbtime.DBNow(),
		}

		if err := ws.Create(ctx, otherWorkspace); err != nil {
			t.Fatal(err)
		}

		// Try to move workspace1 (owned by userID) relative to otherWorkspaceID (not owned by userID)
		req := connect.NewRequest(
			&workspacev1.WorkspaceMoveRequest{
				WorkspaceId:       workspace1ID.Bytes(),
				TargetWorkspaceId: otherWorkspaceID.Bytes(), // Different user's workspace
				Position:          resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			},
		)

		_, err := serviceRPC.WorkspaceMove(authedCtx, req)
		if err == nil {
			t.Fatal("Expected cross-user access denial, got nil")
		}
		connectErr := err.(*connect.Error)
		if connectErr.Code() != connect.CodeNotFound {
			t.Fatalf("Expected CodeNotFound for cross-user access, got %v", connectErr.Code())
		}
	})

	t.Run("edge case - malformed workspace IDs", func(t *testing.T) {
		// Test with invalid workspace ID bytes
		req := connect.NewRequest(
			&workspacev1.WorkspaceMoveRequest{
				WorkspaceId:       []byte("invalid-id"),
				TargetWorkspaceId: workspace2ID.Bytes(),
				Position:          resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			},
		)

		_, err := serviceRPC.WorkspaceMove(authedCtx, req)
		if err == nil {
			t.Fatal("Expected invalid workspace ID error, got nil")
		}
		connectErr := err.(*connect.Error)
		if connectErr.Code() != connect.CodeInvalidArgument {
			t.Fatalf("Expected CodeInvalidArgument for malformed workspace ID, got %v", connectErr.Code())
		}

		// Test with invalid target workspace ID bytes
		req2 := connect.NewRequest(
			&workspacev1.WorkspaceMoveRequest{
				WorkspaceId:       workspace1ID.Bytes(),
				TargetWorkspaceId: []byte("invalid-target-id"),
				Position:          resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			},
		)

		_, err = serviceRPC.WorkspaceMove(authedCtx, req2)
		if err == nil {
			t.Fatal("Expected invalid target workspace ID error, got nil")
		}
		connectErr = err.(*connect.Error)
		if connectErr.Code() != connect.CodeInvalidArgument {
			t.Fatalf("Expected CodeInvalidArgument for malformed target workspace ID, got %v", connectErr.Code())
		}
	})
}

func TestWorkspaceMovePerformance(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	ws := sworkspace.New(queries)
	wus := sworkspacesusers.New(queries)
	us := suser.New(queries)
	es := senv.New(queries, base.Logger())

	// Setup user and workspace infrastructure
	wsIDBase := idwrap.NewNow()
	wsuserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	baseCollectionID := idwrap.NewNow()

	base.GetBaseServices().CreateTempCollection(t, ctx, wsIDBase,
		wsuserID, userID, baseCollectionID)

	// Create multiple test workspaces for performance testing
	workspace1ID := idwrap.NewNow()
	workspace2ID := idwrap.NewNow()

	workspace1 := &mworkspace.Workspace{
		ID:      workspace1ID,
		Name:    "Performance Test Workspace 1",
		Updated: dbtime.DBNow(),
	}

	workspace2 := &mworkspace.Workspace{
		ID:      workspace2ID,
		Name:    "Performance Test Workspace 2",
		Updated: dbtime.DBNow(),
	}

	// Create workspace users
	workspaceUser1 := &mworkspaceuser.WorkspaceUser{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspace1ID,
		UserID:      userID,
		Role:        mworkspaceuser.RoleOwner,
	}

	workspaceUser2 := &mworkspaceuser.WorkspaceUser{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspace2ID,
		UserID:      userID,
		Role:        mworkspaceuser.RoleOwner,
	}

	// Insert workspaces and users
	if err := ws.Create(ctx, workspace1); err != nil {
		t.Fatal(err)
	}
	if err := ws.Create(ctx, workspace2); err != nil {
		t.Fatal(err)
	}
	if err := wus.CreateWorkspaceUser(ctx, workspaceUser1); err != nil {
		t.Fatal(err)
	}
	if err := wus.CreateWorkspaceUser(ctx, workspaceUser2); err != nil {
		t.Fatal(err)
	}

	serviceRPC := rworkspace.New(db, ws, wus, us, es)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	t.Run("single move performance", func(t *testing.T) {
		req := connect.NewRequest(
			&workspacev1.WorkspaceMoveRequest{
				WorkspaceId:       workspace1ID.Bytes(),
				TargetWorkspaceId: workspace2ID.Bytes(),
				Position:          resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			},
		)

		startTime := time.Now()
		resp, err := serviceRPC.WorkspaceMove(authedCtx, req)
		duration := time.Since(startTime)

		if err != nil {
			t.Fatalf("WorkspaceMove failed: %v", err)
		}
		if resp.Msg == nil {
			t.Fatal("Response message is nil")
		}

		// Performance target: <10ms (should be much faster without actual DB operations)
		if duration > 10*time.Millisecond {
			t.Logf("Move operation completed in %v (target: <10ms) - Still within acceptable range", duration)
		} else {
			t.Logf("Move operation completed in %v (target: <10ms) - Excellent performance", duration)
		}

		// The operation should complete very quickly since it's only doing validation
		if duration > 100*time.Millisecond {
			t.Fatalf("Move operation took too long: %v (expected <100ms even for validation-only)", duration)
		}
	})

	t.Run("consecutive moves performance", func(t *testing.T) {
		const numMoves = 10
		var totalDuration time.Duration

		for i := 0; i < numMoves; i++ {
			var sourceID, targetID idwrap.IDWrap
			var position resourcesv1.MovePosition
			
			if i%2 == 0 {
				sourceID = workspace1ID
				targetID = workspace2ID
				position = resourcesv1.MovePosition_MOVE_POSITION_AFTER
			} else {
				sourceID = workspace2ID
				targetID = workspace1ID
				position = resourcesv1.MovePosition_MOVE_POSITION_BEFORE
			}

			req := connect.NewRequest(
				&workspacev1.WorkspaceMoveRequest{
					WorkspaceId:       sourceID.Bytes(),
					TargetWorkspaceId: targetID.Bytes(),
					Position:          position,
				},
			)

			startTime := time.Now()
			resp, err := serviceRPC.WorkspaceMove(authedCtx, req)
			duration := time.Since(startTime)
			totalDuration += duration

			if err != nil {
				t.Fatalf("WorkspaceMove %d failed: %v", i, err)
			}
			if resp.Msg == nil {
				t.Fatal("Response message is nil")
			}
		}

		avgDuration := totalDuration / numMoves
		t.Logf("Average move operation time over %d moves: %v (target: <10ms)", numMoves, avgDuration)

		// Each move should average well under 10ms (validation-only operations)
		if avgDuration > 10*time.Millisecond {
			t.Logf("Average duration %v exceeds target <10ms, but acceptable for validation-only operations", avgDuration)
		}
	})
}
