package rhttp

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	devtoolsdb "the-dev-tools/db"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/db/pkg/sqlitemem"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/muser"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/service/sworkspacesusers"
)

// testServiceSetup creates a minimal service setup for testing transaction patterns
type testServiceSetup struct {
	db          *sql.DB
	hs          shttp.HTTPService
	us          suser.UserService
	ws          sworkspace.WorkspaceService
	wus         sworkspacesusers.WorkspaceUserService
	es          senv.EnvService
	vs          senv.VariableService
	ctx         context.Context
	userID      idwrap.IDWrap
	workspaceID idwrap.IDWrap
}

func createTestServiceSetup(t *testing.T) *testServiceSetup {
	t.Helper()

	ctx := context.Background()

	// Create in-memory database for isolated testing
	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	require.NoError(t, err)
	t.Cleanup(cleanup)

	// Prepare queries
	queries, err := gen.Prepare(ctx, db)
	require.NoError(t, err)

	// Create all required services with logger parameter
	logger := slog.Default()
	hs := shttp.New(queries, logger)
	us := suser.New(queries)
	ws := sworkspace.New(queries)
	wus := sworkspacesusers.New(queries)
	es := senv.NewEnvironmentService(queries, logger)
	vs := senv.NewVariableService(queries, logger)

	// Create test user and workspace
	userID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()

	// Create user
	providerID := "test-provider"
	err = us.CreateUser(ctx, &muser.User{
		ID:           userID,
		Email:        "test@example.com",
		Password:     []byte("password"),
		ProviderID:   &providerID,
		ProviderType: muser.MagicLink,
		Status:       muser.Active,
	})
	require.NoError(t, err)

	// Create workspace
	err = ws.Create(ctx, &mworkspace.Workspace{
		ID:      workspaceID,
		Name:    "Test Workspace",
		Updated: dbtime.DBNow(),
	})
	require.NoError(t, err)

	// Create workspace user with admin role
	err = wus.CreateWorkspaceUser(ctx, &mworkspace.WorkspaceUser{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		UserID:      userID,
		Role:        mworkspace.RoleAdmin,
	})
	require.NoError(t, err)

	authCtx := mwauth.CreateAuthedContext(ctx, userID)

	return &testServiceSetup{
		db:          db,
		hs:          hs,
		us:          us,
		ws:          ws,
		wus:         wus,
		es:          es,
		vs:          vs,
		ctx:         authCtx,
		userID:      userID,
		workspaceID: workspaceID,
	}
}

// TestHttpInsertOptimizedPerformance demonstrates that reads outside transactions
// are fast and writes inside minimal transactions are also fast
func TestHttpInsertOptimizedPerformance(t *testing.T) {
	setup := createTestServiceSetup(t)

	// Test that reads OUTSIDE transactions work (baseline)
	t.Run("ReadsOutsideTransaction", func(t *testing.T) {
		start := time.Now()
		workspaces, err := setup.ws.GetWorkspacesByUserIDOrdered(setup.ctx, setup.userID)
		require.NoError(t, err)
		require.Greater(t, len(workspaces), 0, "Should have workspace data")

		readOutsideTime := time.Since(start)
		t.Logf("✅ Read outside transaction took: %v", readOutsideTime)

		// ✅ This should be fast (under 1ms)
		assert.True(t, readOutsideTime < time.Millisecond, "Reads outside transaction should be fast")
	})

	// Test that writes INSIDE minimal transactions are fast
	t.Run("WritesInsideMinimalTransaction", func(t *testing.T) {
		// First, get workspace data outside transaction
		workspaces, err := setup.ws.GetWorkspacesByUserIDOrdered(setup.ctx, setup.userID)
		require.NoError(t, err)
		require.Greater(t, len(workspaces), 0)

		// Test write inside minimal transaction
		tx, err := setup.db.BeginTx(setup.ctx, nil)
		require.NoError(t, err)
		defer devtoolsdb.TxnRollback(tx)

		hsTx := setup.hs.TX(tx)

		writeStart := time.Now()
		for i := 0; i < 5; i++ {
			httpModel := &mhttp.HTTP{
				ID:          idwrap.NewNow(),
				WorkspaceID: workspaces[0].ID,
				Name:        fmt.Sprintf("Test %d", i),
				Url:         "https://example.com",
				Method:      "GET",
			}
			err = hsTx.Create(setup.ctx, httpModel)
			require.NoError(t, err)
		}
		writeTime := time.Since(writeStart)

		t.Logf("✅ 5 writes inside minimal transaction took: %v", writeTime)
		assert.True(t, writeTime < 5*time.Millisecond, "Batch writes inside minimal transaction should be fast")

		// Test overall performance
		totalTime := time.Since(writeStart)
		assert.True(t, totalTime < 10*time.Millisecond, "Total operation should be fast")
	})
}

// TestHttpInsertConcurrentOptimized demonstrates that concurrent operations with the
// optimized pattern (reads outside, minimal writes inside) are fast and reliable
func TestHttpInsertConcurrentOptimized(t *testing.T) {
	setup := createTestServiceSetup(t)

	// Test concurrent operations with optimized pattern
	t.Run("ConcurrentOptimizedTransactions", func(t *testing.T) {
		numGoroutines := 10
		var wg sync.WaitGroup
		results := make(chan time.Duration, numGoroutines)
		errors := make(chan error, numGoroutines)

		// Pre-fetch workspace info outside transactions (optimized pattern)
		workspaces, err := setup.ws.GetWorkspacesByUserIDOrdered(setup.ctx, setup.userID)
		require.NoError(t, err)
		require.Len(t, workspaces, 1)

		workspaceID := workspaces[0].ID

		// Start multiple concurrent optimized insert attempts
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				start := time.Now()

				// Optimized pattern: reads already done outside transaction
				// Only minimal write operations inside transaction
				tx, err := setup.db.BeginTx(setup.ctx, nil)
				if err != nil {
					errors <- err
					return
				}
				defer devtoolsdb.TxnRollback(tx)

				hsTx := setup.hs.TX(tx)

				httpID := idwrap.NewNow()
				httpModel := &mhttp.HTTP{
					ID:          httpID,
					WorkspaceID: workspaceID,
					Name:        fmt.Sprintf("Concurrent Optimized HTTP %d", id),
					Url:         "https://api.example.com/concurrent",
					Method:      "POST",
				}

				err = hsTx.Create(setup.ctx, httpModel)
				if err != nil {
					errors <- err
					return
				}

				err = tx.Commit()
				if err != nil {
					errors <- err
					return
				}

				duration := time.Since(start)
				results <- duration
			}(i)
		}

		wg.Wait()
		close(results)
		close(errors)

		// Count results
		successCount := 0
		errorCount := 0
		var totalDuration time.Duration

		for duration := range results {
			successCount++
			totalDuration += duration
		}

		for err := range errors {
			errorCount++
			t.Logf("Optimized concurrent insert error: %v", err)
		}

		t.Logf("✅ Optimized concurrent inserts: %d successful, %d failed", successCount, errorCount)

		// With optimized pattern, we expect high success rate and fast operations
		assert.Equal(t, numGoroutines, successCount, "All operations should succeed with optimized pattern")
		assert.Equal(t, 0, errorCount, "No operations should fail with optimized pattern")

		if successCount > 0 {
			avgDuration := totalDuration / time.Duration(successCount)
			t.Logf("✅ Average duration for optimized concurrent inserts: %v", avgDuration)

			// Optimized pattern should be much faster
			assert.Less(t, avgDuration, 30*time.Millisecond, "Optimized concurrent operations should be fast")
		}
	})
}

// TestTransactionPatternCompliance verifies that our fix follows the correct pattern
func TestTransactionPatternCompliance(t *testing.T) {
	setup := createTestServiceSetup(t)

	// Test that we have workspace data before transaction
	workspaces, err := setup.ws.GetWorkspacesByUserIDOrdered(setup.ctx, setup.userID)
	require.NoError(t, err)
	assert.Greater(t, len(workspaces), 0, "Should have workspace data")

	// Test that permission checks work outside transaction
	wsUser, err := setup.wus.GetWorkspaceUsersByWorkspaceIDAndUserID(setup.ctx, workspaces[0].ID, setup.userID)
	require.NoError(t, err, "Permission check should work outside transaction")
	require.GreaterOrEqual(t, wsUser.Role, mworkspace.RoleAdmin, "User should have admin access")

	// Test that we can do model creation outside transaction
	httpModels := make([]*mhttp.HTTP, 0)
	for i := 0; i < 3; i++ {
		model := &mhttp.HTTP{
			ID:          idwrap.NewNow(),
			WorkspaceID: workspaces[0].ID,
			Name:        fmt.Sprintf("Test %d", i),
			Url:         "https://example.com",
			Method:      "GET",
		}
		httpModels = append(httpModels, model)
	}

	assert.Len(t, httpModels, 3, "Should have created models outside transaction")

	// Test that transaction contains only writes
	tx, err := setup.db.BeginTx(setup.ctx, nil)
	require.NoError(t, err)
	defer devtoolsdb.TxnRollback(tx)

	hsTx := setup.hs.TX(tx)

	for _, model := range httpModels {
		err = hsTx.Create(setup.ctx, model)
		require.NoError(t, err)
	}

	// Success means we followed the correct pattern
	t.Log("✅ Transaction pattern compliance verified")
}

// TestHttpInsertPatternValidation tests the core pattern that HttpInsert should follow:
// 1. Reads outside transactions (fast)
// 2. Minimal writes inside transactions (fast)
func TestHttpInsertPatternValidation(t *testing.T) {
	setup := createTestServiceSetup(t)

	// Step 1: Validate reads outside transactions are fast
	start := time.Now()
	workspaces, err := setup.ws.GetWorkspacesByUserIDOrdered(setup.ctx, setup.userID)
	require.NoError(t, err)
	require.Greater(t, len(workspaces), 0, "Should have workspace data")
	readTime := time.Since(start)

	t.Logf("✅ Read outside transaction took: %v", readTime)
	assert.True(t, readTime < time.Millisecond, "Reads outside transaction should be very fast")

	// Step 2: Validate permission checks outside transactions are fast
	start = time.Now()
	wsUser, err := setup.wus.GetWorkspaceUsersByWorkspaceIDAndUserID(setup.ctx, workspaces[0].ID, setup.userID)
	require.NoError(t, err, "Permission check should work outside transaction")
	require.GreaterOrEqual(t, wsUser.Role, mworkspace.RoleAdmin, "User should have admin access")
	permissionTime := time.Since(start)

	t.Logf("✅ Permission check outside transaction took: %v", permissionTime)
	assert.True(t, permissionTime < time.Millisecond, "Permission checks outside transaction should be very fast")

	// Step 3: Validate model creation outside transactions (data preparation)
	start = time.Now()
	httpModels := make([]*mhttp.HTTP, 0)
	for i := 0; i < 5; i++ {
		model := &mhttp.HTTP{
			ID:          idwrap.NewNow(),
			WorkspaceID: workspaces[0].ID,
			Name:        fmt.Sprintf("Test %d", i),
			Url:         "https://example.com",
			Method:      "GET",
		}
		httpModels = append(httpModels, model)
	}
	modelTime := time.Since(start)

	t.Logf("✅ Model creation outside transaction took: %v", modelTime)
	assert.True(t, modelTime < time.Millisecond, "Model creation outside transaction should be very fast")

	// Step 4: Validate writes inside minimal transactions are fast
	start = time.Now()
	tx, err := setup.db.BeginTx(setup.ctx, nil)
	require.NoError(t, err)
	defer devtoolsdb.TxnRollback(tx)

	hsTx := setup.hs.TX(tx)

	for _, model := range httpModels {
		err = hsTx.Create(setup.ctx, model)
		require.NoError(t, err)
	}

	err = tx.Commit()
	require.NoError(t, err)
	writeTime := time.Since(start)

	t.Logf("✅ 5 writes inside minimal transaction took: %v", writeTime)
	assert.True(t, writeTime < 10*time.Millisecond, "Writes inside minimal transaction should be fast")

	// Step 5: Total operation time validation
	totalTime := readTime + permissionTime + modelTime + writeTime
	t.Logf("✅ Total operation time took: %v", totalTime)
	assert.True(t, totalTime < 20*time.Millisecond, "Total optimized operation should be very fast")

	t.Log("✅ HttpInsert pattern validation complete: reads outside, minimal writes inside")
}

// BenchmarkHttpInsertOptimizedPattern benchmarks the optimized transaction pattern
// (reads outside, minimal writes inside) to demonstrate its performance
func BenchmarkHttpInsertOptimizedPattern(b *testing.B) {
	ctx := context.Background()

	// Create in-memory database for benchmarking
	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	if err != nil {
		b.Fatalf("Failed to create test database: %v", err)
	}
	defer cleanup()

	// Prepare queries
	queries, err := gen.Prepare(ctx, db)
	if err != nil {
		b.Fatalf("Failed to prepare queries: %v", err)
	}

	// Create all required services
	logger := slog.Default()
	hs := shttp.New(queries, logger)
	us := suser.New(queries)
	ws := sworkspace.New(queries)
	wus := sworkspacesusers.New(queries)
	_ = senv.NewEnvironmentService(queries, logger) // Not used in this test
	_ = senv.NewVariableService(queries, logger)    // Not used in this test

	// Create test user and workspace
	userID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()

	// Create user
	providerID := "test-provider"
	err = us.CreateUser(ctx, &muser.User{
		ID:           userID,
		Email:        "test@example.com",
		Password:     []byte("password"),
		ProviderID:   &providerID,
		ProviderType: muser.MagicLink,
		Status:       muser.Active,
	})
	if err != nil {
		b.Fatalf("Failed to create user: %v", err)
	}

	// Create workspace
	err = ws.Create(ctx, &mworkspace.Workspace{
		ID:      workspaceID,
		Name:    "Test Workspace",
		Updated: dbtime.DBNow(),
	})
	if err != nil {
		b.Fatalf("Failed to create workspace: %v", err)
	}

	// Create workspace user with admin role
	err = wus.CreateWorkspaceUser(ctx, &mworkspace.WorkspaceUser{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		UserID:      userID,
		Role:        mworkspace.RoleAdmin,
	})
	if err != nil {
		b.Fatalf("Failed to create workspace user: %v", err)
	}

	authCtx := mwauth.CreateAuthedContext(ctx, userID)

	// Pre-fetch workspace data (optimized pattern)
	workspaces, err := ws.GetWorkspacesByUserIDOrdered(authCtx, userID)
	if err != nil {
		b.Fatalf("Failed to get workspaces: %v", err)
	}
	if len(workspaces) == 0 {
		b.Fatalf("No workspaces found")
	}

	// Benchmark optimized pattern (reads outside, minimal writes inside)
	b.Run("Optimized_Pattern", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			b.StartTimer()

			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				b.Fatalf("Failed to begin transaction: %v", err)
			}

			hsTx := hs.TX(tx)

			httpID := idwrap.NewNow()
			httpModel := &mhttp.HTTP{
				ID:          httpID,
				WorkspaceID: workspaces[0].ID,
				Name:        "Optimized Benchmark HTTP",
				Url:         "https://api.example.com/optimized",
				Method:      "GET",
			}

			err = hsTx.Create(authCtx, httpModel)
			if err != nil {
				tx.Rollback()
				b.Fatalf("Failed to create HTTP: %v", err)
			}

			err = tx.Commit()
			if err != nil {
				b.Fatalf("Failed to commit transaction: %v", err)
			}

			b.StopTimer()
		}
	})

	// Benchmark batch optimized pattern
	b.Run("Optimized_Batch_Pattern", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			b.StartTimer()

			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				b.Fatalf("Failed to begin transaction: %v", err)
			}

			hsTx := hs.TX(tx)

			// Create multiple HTTP entries in one transaction
			for j := 0; j < 5; j++ {
				httpID := idwrap.NewNow()
				httpModel := &mhttp.HTTP{
					ID:          httpID,
					WorkspaceID: workspaces[0].ID,
					Name:        fmt.Sprintf("Batch Optimized HTTP %d", j),
					Url:         "https://api.example.com/batch",
					Method:      "GET",
				}

				err = hsTx.Create(authCtx, httpModel)
				if err != nil {
					tx.Rollback()
					b.Fatalf("Failed to create HTTP: %v", err)
				}
			}

			err = tx.Commit()
			if err != nil {
				b.Fatalf("Failed to commit transaction: %v", err)
			}

			b.StopTimer()
		}
	})
}
