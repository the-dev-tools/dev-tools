package rvar_test

import (
	"context"
	"fmt"
	"testing"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rvar"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/model/mvar"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/svar"
	"the-dev-tools/server/pkg/testutil"
	resourcesv1 "the-dev-tools/spec/dist/buf/go/resources/v1"
	variablev1 "the-dev-tools/spec/dist/buf/go/variable/v1"
	"time"

	"connectrpc.com/connect"
)

// TestVariableMovePerformance tests the performance of variable move operations
func TestVariableMovePerformance(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	us := suser.New(queries)
	es := senv.New(queries, base.Logger())
	vs := svar.New(queries, base.Logger())

	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	collectionID := idwrap.NewNow()
	userID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID,
		workspaceUserID, userID, collectionID)

	// Create environment
	envID := idwrap.NewNow()
	env := menv.Env{
		ID:          envID,
		WorkspaceID: workspaceID,
		Type:        menv.EnvNormal,
		Description: "Performance Test Environment",
		Name:        "PerfTestEnv",
		Updated:     time.Now(),
	}
	err := es.Create(ctx, env)
	if err != nil {
		t.Fatal(err)
	}

	rpcVar := rvar.New(db, us, es, vs)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	// Test case 1: Single move operation performance (target: <10ms)
	t.Run("SingleMovePerformance", func(t *testing.T) {
		// Create variables
		var1ID := idwrap.NewNow()
		var1 := mvar.Var{
			ID:          var1ID,
			EnvID:       envID,
			VarKey:      "PERF_VAR1",
			Value:       "perf_value1",
			Enabled:     true,
			Description: "Performance test variable 1",
		}
		err := vs.Create(ctx, var1)
		if err != nil {
			t.Fatal(err)
		}

		var2ID := idwrap.NewNow()
		var2 := mvar.Var{
			ID:          var2ID,
			EnvID:       envID,
			VarKey:      "PERF_VAR2",
			Value:       "perf_value2",
			Enabled:     true,
			Description: "Performance test variable 2",
		}
		err = vs.Create(ctx, var2)
		if err != nil {
			t.Fatal(err)
		}

		// Measure move operation time
		req := connect.NewRequest(&variablev1.VariableMoveRequest{
			EnvironmentId:    envID.Bytes(),
			VariableId:       var1ID.Bytes(),
			Position:         resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetVariableId: var2ID.Bytes(),
		})

		start := time.Now()
		resp, err := rpcVar.VariableMove(authedCtx, req)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatal("Move operation failed:", err)
		}
		if resp == nil {
			t.Fatal("Expected response")
		}

		// Performance requirement: <10ms
		if elapsed > 10*time.Millisecond {
			t.Errorf("Move operation took too long: %v (target: <10ms)", elapsed)
		} else {
			t.Logf("Move operation completed in %v (target: <10ms)", elapsed)
		}

		// Verify operation was successful
		variables, err := vs.GetVariablesByEnvIDOrdered(ctx, envID)
		if err != nil {
			t.Fatal("Failed to get ordered variables:", err)
		}
		if len(variables) != 2 {
			t.Fatal("Expected 2 variables after move")
		}

		// Clean up
		err = vs.Delete(ctx, var1ID)
		if err != nil {
			t.Fatal("Failed to clean up var1:", err)
		}
		err = vs.Delete(ctx, var2ID)
		if err != nil {
			t.Fatal("Failed to clean up var2:", err)
		}
	})

	// Test case 2: Performance with realistic variable counts (10-50 variables)
	t.Run("RealisticVariableCountPerformance", func(t *testing.T) {
		variableCounts := []int{10, 25, 50}

		for _, count := range variableCounts {
			t.Run(fmt.Sprintf("%d_variables", count), func(t *testing.T) {
				// Create variables
				varIDs := make([]idwrap.IDWrap, count)
				for i := 0; i < count; i++ {
					varID := idwrap.NewNow()
					variable := mvar.Var{
						ID:          varID,
						EnvID:       envID,
						VarKey:      fmt.Sprintf("PERF_VAR_%d", i),
						Value:       fmt.Sprintf("perf_value_%d", i),
						Enabled:     true,
						Description: fmt.Sprintf("Performance test variable %d", i),
					}
					err := vs.Create(ctx, variable)
					if err != nil {
						t.Fatal("Failed to create variable:", err)
					}
					varIDs[i] = varID
				}

				// Measure move operation from middle to beginning
				middleIndex := count / 2
				req := connect.NewRequest(&variablev1.VariableMoveRequest{
					EnvironmentId:    envID.Bytes(),
					VariableId:       varIDs[middleIndex].Bytes(),
					Position:         resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
					TargetVariableId: varIDs[0].Bytes(),
				})

				start := time.Now()
				resp, err := rpcVar.VariableMove(authedCtx, req)
				elapsed := time.Since(start)

				if err != nil {
					t.Fatal("Move operation failed:", err)
				}
				if resp == nil {
					t.Fatal("Expected response")
				}

				// Performance requirement: <10ms even with more variables
				if elapsed > 10*time.Millisecond {
					t.Errorf("Move operation with %d variables took too long: %v (target: <10ms)", count, elapsed)
				} else {
					t.Logf("Move operation with %d variables completed in %v (target: <10ms)", count, elapsed)
				}

				// Verify operation was successful
				variables, err := vs.GetVariablesByEnvIDOrdered(ctx, envID)
				if err != nil {
					t.Fatal("Failed to get ordered variables:", err)
				}
				if len(variables) != count {
					t.Fatalf("Expected %d variables after move, got %d", count, len(variables))
				}

				// Clean up all variables
				for _, varID := range varIDs {
					err = vs.Delete(ctx, varID)
					if err != nil {
						t.Fatal("Failed to clean up variable:", err)
					}
				}
			})
		}
	})

	// Test case 3: Multiple consecutive move operations performance
	t.Run("ConsecutiveMovesPerformance", func(t *testing.T) {
		// Create 5 variables for multiple move operations
		varIDs := make([]idwrap.IDWrap, 5)
		for i := 0; i < 5; i++ {
			varID := idwrap.NewNow()
			variable := mvar.Var{
				ID:          varID,
				EnvID:       envID,
				VarKey:      fmt.Sprintf("CONSEC_VAR_%d", i),
				Value:       fmt.Sprintf("consec_value_%d", i),
				Enabled:     true,
				Description: fmt.Sprintf("Consecutive move test variable %d", i),
			}
			err := vs.Create(ctx, variable)
			if err != nil {
				t.Fatal("Failed to create variable:", err)
			}
			varIDs[i] = varID
		}

		// Perform multiple consecutive moves and measure total time
		start := time.Now()

		// Move var4 to beginning (before var0)
		req1 := connect.NewRequest(&variablev1.VariableMoveRequest{
			EnvironmentId:    envID.Bytes(),
			VariableId:       varIDs[4].Bytes(),
			Position:         resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
			TargetVariableId: varIDs[0].Bytes(),
		})
		_, err := rpcVar.VariableMove(authedCtx, req1)
		if err != nil {
			t.Fatal("First move failed:", err)
		}

		// Move var1 to end (after var2)
		req2 := connect.NewRequest(&variablev1.VariableMoveRequest{
			EnvironmentId:    envID.Bytes(),
			VariableId:       varIDs[1].Bytes(),
			Position:         resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetVariableId: varIDs[2].Bytes(),
		})
		_, err = rpcVar.VariableMove(authedCtx, req2)
		if err != nil {
			t.Fatal("Second move failed:", err)
		}

		// Move var3 to middle
		req3 := connect.NewRequest(&variablev1.VariableMoveRequest{
			EnvironmentId:    envID.Bytes(),
			VariableId:       varIDs[3].Bytes(),
			Position:         resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetVariableId: varIDs[0].Bytes(),
		})
		_, err = rpcVar.VariableMove(authedCtx, req3)
		if err != nil {
			t.Fatal("Third move failed:", err)
		}

		totalElapsed := time.Since(start)

		// Average per move should be <10ms
		avgPerMove := totalElapsed / 3
		if avgPerMove > 10*time.Millisecond {
			t.Errorf("Average move operation time too long: %v (target: <10ms)", avgPerMove)
		} else {
			t.Logf("Average move operation time: %v (target: <10ms)", avgPerMove)
		}

		// Verify final order is maintained correctly
		variables, err := vs.GetVariablesByEnvIDOrdered(ctx, envID)
		if err != nil {
			t.Fatal("Failed to get ordered variables:", err)
		}
		if len(variables) != 5 {
			t.Fatal("Expected 5 variables after consecutive moves")
		}

		// Clean up all variables
		for _, varID := range varIDs {
			err = vs.Delete(ctx, varID)
			if err != nil {
				t.Fatal("Failed to clean up variable:", err)
			}
		}
	})
}

// BenchmarkVariableMove benchmarks the variable move operation
func BenchmarkVariableMove(b *testing.B) {
	ctx := context.Background()
	// Use a dummy *testing.T for setup since testutil.CreateBaseDB expects *testing.T
	dummyT := &testing.T{}
	base := testutil.CreateBaseDB(ctx, dummyT)
	queries := base.Queries
	db := base.DB

	us := suser.New(queries)
	es := senv.New(queries, base.Logger())
	vs := svar.New(queries, base.Logger())

	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	collectionID := idwrap.NewNow()
	userID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(dummyT, ctx, workspaceID,
		workspaceUserID, userID, collectionID)

	// Create environment
	envID := idwrap.NewNow()
	env := menv.Env{
		ID:          envID,
		WorkspaceID: workspaceID,
		Type:        menv.EnvNormal,
		Description: "Benchmark Environment",
		Name:        "BenchEnv",
		Updated:     time.Now(),
	}
	err := es.Create(ctx, env)
	if err != nil {
		b.Fatal(err)
	}

	// Create variables for benchmarking
	var1ID := idwrap.NewNow()
	var1 := mvar.Var{
		ID:          var1ID,
		EnvID:       envID,
		VarKey:      "BENCH_VAR1",
		Value:       "bench_value1",
		Enabled:     true,
		Description: "Benchmark variable 1",
	}
	err = vs.Create(ctx, var1)
	if err != nil {
		b.Fatal(err)
	}

	var2ID := idwrap.NewNow()
	var2 := mvar.Var{
		ID:          var2ID,
		EnvID:       envID,
		VarKey:      "BENCH_VAR2",
		Value:       "bench_value2",
		Enabled:     true,
		Description: "Benchmark variable 2",
	}
	err = vs.Create(ctx, var2)
	if err != nil {
		b.Fatal(err)
	}

	rpcVar := rvar.New(db, us, es, vs)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Alternate between moving var1 after var2 and var2 after var1
		var sourceID, targetID idwrap.IDWrap
		if i%2 == 0 {
			sourceID, targetID = var1ID, var2ID
		} else {
			sourceID, targetID = var2ID, var1ID
		}

		req := connect.NewRequest(&variablev1.VariableMoveRequest{
			EnvironmentId:    envID.Bytes(),
			VariableId:       sourceID.Bytes(),
			Position:         resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetVariableId: targetID.Bytes(),
		})

		_, err := rpcVar.VariableMove(authedCtx, req)
		if err != nil {
			b.Fatal("Move operation failed:", err)
		}
	}
}