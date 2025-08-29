package rvar_test

import (
	"context"
	"testing"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rvar"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/svar"
	"the-dev-tools/server/pkg/testutil"
	resourcesv1 "the-dev-tools/spec/dist/buf/go/resources/v1"
	variablev1 "the-dev-tools/spec/dist/buf/go/variable/v1"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVariableListRespectsMoveOrdering(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	us := suser.New(queries)
	es := senv.New(queries, base.Logger())
	vs := svar.New(queries, base.Logger())

	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	CollectionID := idwrap.NewNow()
	UserID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID,
		workspaceUserID, UserID, CollectionID)

	envID := idwrap.NewNow()
	env := menv.Env{
		ID:          envID,
		WorkspaceID: workspaceID,
		Type:        menv.EnvGlobal,
		Description: "test environment",
		Name:        "Test Environment",
		Updated:     time.Now(),
	}
	err := es.Create(ctx, env)
	require.NoError(t, err)

	// Create RPC handler
	rpc := rvar.New(db, us, es, vs)
	
	// Create authenticated context
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)

	// Test both environment-level and workspace-level listing
	tests := []struct {
		name    string
		testEnv bool // true for environment-level, false for workspace-level
	}{
		{
			name:    "Environment-level variable listing",
			testEnv: true,
		},
		{
			name:    "Workspace-level variable listing", 
			testEnv: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create 4 variables via RPC - they should be in creation order initially
			var createdVariableIDs [][]byte
			expectedNames := []string{"var1", "var2", "var3", "var4"}

			for i, name := range expectedNames {
				createReq := &variablev1.VariableCreateRequest{
					EnvironmentId: envID.Bytes(),
					Name:          name,
					Value:         "value" + string(rune('1'+i)),
					Enabled:       true,
					Description:   "Test variable " + string(rune('1'+i)),
				}

				createResp, err := rpc.VariableCreate(authedCtx, connect.NewRequest(createReq))
				require.NoError(t, err)
				createdVariableIDs = append(createdVariableIDs, createResp.Msg.VariableId)
			}

			// Verify initial order through RPC
			var listResp *connect.Response[variablev1.VariableListResponse]
			if tt.testEnv {
				listReq := &variablev1.VariableListRequest{
					EnvironmentId: envID.Bytes(),
				}
				listResp, err = rpc.VariableList(authedCtx, connect.NewRequest(listReq))
			} else {
				listReq := &variablev1.VariableListRequest{
					WorkspaceId: workspaceID.Bytes(),
				}
				listResp, err = rpc.VariableList(authedCtx, connect.NewRequest(listReq))
			}
			require.NoError(t, err)
			require.Len(t, listResp.Msg.Items, 4)

			// Check initial order: var1, var2, var3, var4
			for i, expectedName := range expectedNames {
				assert.Equal(t, expectedName, listResp.Msg.Items[i].Name, 
					"Initial order incorrect at position %d", i)
			}

			// Move first variable (var1) to after last variable (var4)
			// This should result in: var2, var3, var4, var1
			moveReq := &variablev1.VariableMoveRequest{
				VariableId:       createdVariableIDs[0], // var1
				TargetVariableId: createdVariableIDs[3], // var4
				Position:         resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			}
			
			_, err = rpc.VariableMove(authedCtx, connect.NewRequest(moveReq))
			require.NoError(t, err)

			// Verify new order through RPC
			if tt.testEnv {
				listReq := &variablev1.VariableListRequest{
					EnvironmentId: envID.Bytes(),
				}
				listResp, err = rpc.VariableList(authedCtx, connect.NewRequest(listReq))
			} else {
				listReq := &variablev1.VariableListRequest{
					WorkspaceId: workspaceID.Bytes(),
				}
				listResp, err = rpc.VariableList(authedCtx, connect.NewRequest(listReq))
			}
			require.NoError(t, err)
			require.Len(t, listResp.Msg.Items, 4)

			expectedOrderAfterMove := []string{"var2", "var3", "var4", "var1"}
			for i, expectedName := range expectedOrderAfterMove {
				assert.Equal(t, expectedName, listResp.Msg.Items[i].Name,
					"Order after move incorrect at position %d", i)
			}

			// Move last variable (var1) to before first variable (var2)  
			// This should result in: var1, var2, var3, var4 (back to original)
			moveReq = &variablev1.VariableMoveRequest{
				VariableId:       createdVariableIDs[0], // var1 (currently last)
				TargetVariableId: createdVariableIDs[1], // var2 (currently first)
				Position:         resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
			}
			
			_, err = rpc.VariableMove(authedCtx, connect.NewRequest(moveReq))
			require.NoError(t, err)

			// Verify final order through RPC
			if tt.testEnv {
				listReq := &variablev1.VariableListRequest{
					EnvironmentId: envID.Bytes(),
				}
				listResp, err = rpc.VariableList(authedCtx, connect.NewRequest(listReq))
			} else {
				listReq := &variablev1.VariableListRequest{
					WorkspaceId: workspaceID.Bytes(),
				}
				listResp, err = rpc.VariableList(authedCtx, connect.NewRequest(listReq))
			}
			require.NoError(t, err)
			require.Len(t, listResp.Msg.Items, 4)

			// Should be back to original order
			for i, expectedName := range expectedNames {
				assert.Equal(t, expectedName, listResp.Msg.Items[i].Name,
					"Final order incorrect at position %d", i)
			}

			// Test complex move: move middle variable (var3) to beginning
			// This should result in: var3, var1, var2, var4
			moveReq = &variablev1.VariableMoveRequest{
				VariableId:       createdVariableIDs[2], // var3
				TargetVariableId: createdVariableIDs[0], // var1 (currently first)
				Position:         resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
			}
			
			_, err = rpc.VariableMove(authedCtx, connect.NewRequest(moveReq))
			require.NoError(t, err)

			// Verify complex move order through RPC
			if tt.testEnv {
				listReq := &variablev1.VariableListRequest{
					EnvironmentId: envID.Bytes(),
				}
				listResp, err = rpc.VariableList(authedCtx, connect.NewRequest(listReq))
			} else {
				listReq := &variablev1.VariableListRequest{
					WorkspaceId: workspaceID.Bytes(),
				}
				listResp, err = rpc.VariableList(authedCtx, connect.NewRequest(listReq))
			}
			require.NoError(t, err)
			require.Len(t, listResp.Msg.Items, 4)

			expectedComplexOrder := []string{"var3", "var1", "var2", "var4"}
			for i, expectedName := range expectedComplexOrder {
				assert.Equal(t, expectedName, listResp.Msg.Items[i].Name,
					"Complex move order incorrect at position %d", i)
			}

			// Clean up variables for next test iteration
			for _, varID := range createdVariableIDs {
				deleteReq := &variablev1.VariableDeleteRequest{
					VariableId: varID,
				}
				_, err = rpc.VariableDelete(authedCtx, connect.NewRequest(deleteReq))
				require.NoError(t, err)
			}
		})
	}
}

func TestVariableListOrderingEdgeCases(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	us := suser.New(queries)
	es := senv.New(queries, base.Logger())
	vs := svar.New(queries, base.Logger())

	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	CollectionID := idwrap.NewNow()
	UserID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID,
		workspaceUserID, UserID, CollectionID)

	envID := idwrap.NewNow()
	env := menv.Env{
		ID:          envID,
		WorkspaceID: workspaceID,
		Type:        menv.EnvGlobal,
		Description: "test environment for edge cases",
		Name:        "Edge Case Environment",
		Updated:     time.Now(),
	}
	err := es.Create(ctx, env)
	require.NoError(t, err)

	// Create RPC handler
	rpc := rvar.New(db, us, es, vs)
	
	// Create authenticated context
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)

	t.Run("Empty environment returns empty list", func(t *testing.T) {
		listReq := &variablev1.VariableListRequest{
			EnvironmentId: envID.Bytes(),
		}
		listResp, err := rpc.VariableList(authedCtx, connect.NewRequest(listReq))
		require.NoError(t, err)
		assert.Empty(t, listResp.Msg.Items)
	})

	t.Run("Single variable maintains order", func(t *testing.T) {
		// Create single variable
		createReq := &variablev1.VariableCreateRequest{
			EnvironmentId: envID.Bytes(),
			Name:          "single_var",
			Value:         "value",
			Enabled:       true,
			Description:   "Single test variable",
		}

		createResp, err := rpc.VariableCreate(authedCtx, connect.NewRequest(createReq))
		require.NoError(t, err)

		// List variables
		listReq := &variablev1.VariableListRequest{
			EnvironmentId: envID.Bytes(),
		}
		listResp, err := rpc.VariableList(authedCtx, connect.NewRequest(listReq))
		require.NoError(t, err)
		require.Len(t, listResp.Msg.Items, 1)
		assert.Equal(t, "single_var", listResp.Msg.Items[0].Name)

		// Clean up
		deleteReq := &variablev1.VariableDeleteRequest{
			VariableId: createResp.Msg.VariableId,
		}
		_, err = rpc.VariableDelete(authedCtx, connect.NewRequest(deleteReq))
		require.NoError(t, err)
	})

	t.Run("Multiple consecutive moves maintain consistency", func(t *testing.T) {
		// Create 3 variables
		var variableIDs [][]byte
		for i := 1; i <= 3; i++ {
			createReq := &variablev1.VariableCreateRequest{
				EnvironmentId: envID.Bytes(),
				Name:          "var" + string(rune('0'+i)),
				Value:         "value" + string(rune('0'+i)),
				Enabled:       true,
				Description:   "Test variable " + string(rune('0'+i)),
			}

			createResp, err := rpc.VariableCreate(authedCtx, connect.NewRequest(createReq))
			require.NoError(t, err)
			variableIDs = append(variableIDs, createResp.Msg.VariableId)
		}

		// Perform multiple moves
		// Move var1 after var3: var2, var3, var1
		moveReq := &variablev1.VariableMoveRequest{
			VariableId:       variableIDs[0], // var1
			TargetVariableId: variableIDs[2], // var3
			Position:         resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}
		_, err = rpc.VariableMove(authedCtx, connect.NewRequest(moveReq))
		require.NoError(t, err)

		// Move var3 before var2: var3, var2, var1
		moveReq = &variablev1.VariableMoveRequest{
			VariableId:       variableIDs[2], // var3
			TargetVariableId: variableIDs[1], // var2
			Position:         resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
		}
		_, err = rpc.VariableMove(authedCtx, connect.NewRequest(moveReq))
		require.NoError(t, err)

		// Verify final order
		listReq := &variablev1.VariableListRequest{
			EnvironmentId: envID.Bytes(),
		}
		listResp, err := rpc.VariableList(authedCtx, connect.NewRequest(listReq))
		require.NoError(t, err)
		require.Len(t, listResp.Msg.Items, 3)

		expectedOrder := []string{"var3", "var2", "var1"}
		for i, expectedName := range expectedOrder {
			assert.Equal(t, expectedName, listResp.Msg.Items[i].Name,
				"Multiple move order incorrect at position %d", i)
		}

		// Clean up
		for _, varID := range variableIDs {
			deleteReq := &variablev1.VariableDeleteRequest{
				VariableId: varID,
			}
			_, err = rpc.VariableDelete(authedCtx, connect.NewRequest(deleteReq))
			require.NoError(t, err)
		}
	})
}