package rflow_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnodeexecution"
	"the-dev-tools/server/pkg/service/snodeexecution"

	_ "modernc.org/sqlite"
)

// createTestDB creates an in-memory SQLite database with the node_execution table
func createTestDB(t *testing.T) (*sql.DB, *gen.Queries) {
	ctx := context.Background()
	
	// Create in-memory SQLite database with proper settings for concurrency
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Create the schema
	createTableSQL := `
	CREATE TABLE node_execution (
		id BLOB NOT NULL PRIMARY KEY,
		node_id BLOB NOT NULL,
		name TEXT NOT NULL,
		state INT8 NOT NULL,
		error TEXT,
		input_data BLOB,
		input_data_compress_type INT8 NOT NULL DEFAULT 0,
		output_data BLOB,
		output_data_compress_type INT8 NOT NULL DEFAULT 0,
		response_id BLOB,
		completed_at BIGINT
	);`

	_, err = db.ExecContext(ctx, createTableSQL)
	if err != nil {
		t.Fatalf("Failed to create node_execution table: %v", err)
	}

	// Create queries instance (using non-prepared queries for simplicity)
	queries := gen.New(db)
	
	return db, queries
}

// TestHighVolumeUpsertWithConcurrentCleanup tests the UPSERT implementation with high-volume operations
// and concurrent cleanup operations to verify race condition handling
func TestHighVolumeUpsertWithConcurrentCleanup(t *testing.T) {
	const (
		totalOperations = 1000  // Reduced for test stability
		numGoroutines   = 10    // Manageable concurrency
		numCleanupOps   = 20
		cleanupInterval = 50 * time.Millisecond
	)

	ctx := context.Background()
	db, queries := createTestDB(t)
	defer db.Close()

	// Initialize node execution service
	nes := snodeexecution.New(queries)

	// Shared state tracking
	var (
		operationCount       = atomic.Int64{}
		errorCount           = atomic.Int64{}
		noRowsErrorCount     = atomic.Int64{}
		successfulUpserts    = atomic.Int64{}
		successfulCleanups   = atomic.Int64{}
	)

	// Pre-create some node IDs for realistic testing
	nodeIDs := make([]idwrap.IDWrap, 5)
	for i := range nodeIDs {
		nodeIDs[i] = idwrap.NewNow()
	}

	t.Logf("Starting high-volume test with %d operations across %d goroutines", totalOperations, numGoroutines)
	startTime := time.Now()

	var wg sync.WaitGroup

	// Start cleanup goroutines - these simulate the race condition scenario
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(cleanupID int) {
			defer wg.Done()
			ticker := time.NewTicker(cleanupInterval)
			defer ticker.Stop()

			cleanupOps := 0
			for cleanupOps < numCleanupOps && operationCount.Load() < totalOperations {
				select {
				case <-ticker.C:
					// Pick a random node ID to cleanup
					nodeID := nodeIDs[cleanupOps%len(nodeIDs)]
					
					err := nes.DeleteNodeExecutionsByNodeID(ctx, nodeID)
					if err != nil {
						t.Logf("Cleanup %d failed for node %x: %v", cleanupID, nodeID.Bytes(), err)
					} else {
						successfulCleanups.Add(1)
					}
					cleanupOps++
				case <-time.After(5 * time.Second):
					// Timeout protection
					return
				}
			}
		}(i)
	}

	// Start worker goroutines for UPSERT operations
	operationsPerGoroutine := totalOperations / numGoroutines
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for j := 0; j < operationsPerGoroutine; j++ {
				// Create execution with varied data
				executionID := idwrap.NewNow()
				nodeID := nodeIDs[j%len(nodeIDs)]
				
				execution := mnodeexecution.NodeExecution{
					ID:                     executionID,
					NodeID:                 nodeID,
					Name:                   fmt.Sprintf("Worker%d-Op%d", workerID, j),
					State:                  mnnode.NodeState(j % 4), // Vary states
					InputData:              []byte(fmt.Sprintf(`{"worker":%d,"operation":%d}`, workerID, j)),
					InputDataCompressType:  0,
					OutputData:             []byte(`{"result":"success"}`),
					OutputDataCompressType: 0,
				}

				// Some operations include errors and completion times
				if j%3 == 0 {
					errorMsg := "test error"
					execution.Error = &errorMsg
					completedAt := time.Now().UnixMilli()
					execution.CompletedAt = &completedAt
				}

				// Perform UPSERT operation
				err := nes.UpsertNodeExecution(ctx, execution)
				
				operationCount.Add(1)
				
				if err != nil {
					errorCount.Add(1)
					// Check for the specific "no rows in result set" error that indicates race condition
					if strings.Contains(err.Error(), "no rows in result set") {
						noRowsErrorCount.Add(1)
						t.Logf("Worker %d detected 'no rows in result set' error on operation %d: %v", workerID, j, err)
					}
				} else {
					successfulUpserts.Add(1)
				}

				// Brief pause to allow interleaving
				if j%20 == 0 {
					time.Sleep(time.Millisecond)
				}
			}
		}(i)
	}

	// Wait for all operations to complete
	wg.Wait()
	
	elapsed := time.Since(startTime)
	
	// Collect final statistics
	finalOps := operationCount.Load()
	finalErrors := errorCount.Load()
	finalNoRowsErrors := noRowsErrorCount.Load()
	finalSuccessful := successfulUpserts.Load()
	finalCleanups := successfulCleanups.Load()

	t.Logf("High-volume test completed in %v", elapsed)
	t.Logf("Total operations: %d", finalOps)
	t.Logf("Successful UPSERTs: %d", finalSuccessful)
	t.Logf("Total errors: %d", finalErrors)
	t.Logf("'No rows in result set' errors: %d", finalNoRowsErrors)
	t.Logf("Successful cleanups: %d", finalCleanups)
	t.Logf("Operations per second: %.2f", float64(finalOps)/elapsed.Seconds())

	// Verify test completed within reasonable time
	if elapsed > 30*time.Second {
		t.Errorf("Test took too long: %v (should complete in < 30 seconds)", elapsed)
	}

	// The key assertion: UPSERT should handle race conditions gracefully
	// We expect zero "no rows in result set" errors with proper UPSERT implementation
	if finalNoRowsErrors > 0 {
		t.Errorf("UPSERT implementation has race condition issues: %d 'no rows in result set' errors detected", finalNoRowsErrors)
	}

	// Verify we processed expected number of operations
	expectedMinOps := int64(totalOperations)
	if finalOps < expectedMinOps {
		t.Errorf("Expected at least %d operations, got %d", expectedMinOps, finalOps)
	}

	// Verify reasonable success rate (allowing for cleanup interference)
	successRate := float64(finalSuccessful) / float64(finalOps) * 100
	if successRate < 70.0 { // Allow for some failures due to cleanup
		t.Errorf("Success rate too low: %.2f%% (expected > 70%%)", successRate)
	}

	t.Logf("Test passed: UPSERT implementation correctly handles race conditions")
}

// TestUpsertCreatesIfMissing verifies that UPSERT creates a record if it doesn't exist
func TestUpsertCreatesIfMissing(t *testing.T) {
	ctx := context.Background()
	db, queries := createTestDB(t)
	defer db.Close()

	nes := snodeexecution.New(queries)

	executionID := idwrap.NewNow()
	nodeID := idwrap.NewNow()

	// Create execution that doesn't exist yet
	execution := mnodeexecution.NodeExecution{
		ID:                     executionID,
		NodeID:                 nodeID,
		Name:                   "Test Create",
		State:                  mnnode.NODE_STATE_RUNNING,
		InputData:              []byte(`{"test":"create"}`),
		InputDataCompressType:  0,
		OutputData:             []byte(`{}`),
		OutputDataCompressType: 0,
	}

	// Verify it doesn't exist
	_, getErr := nes.GetNodeExecution(ctx, executionID)
	if getErr == nil {
		t.Fatal("Execution should not exist yet")
	}
	if !errors.Is(getErr, sql.ErrNoRows) {
		t.Fatalf("Expected sql.ErrNoRows, got: %v", getErr)
	}

	// UPSERT should create it
	upsertErr := nes.UpsertNodeExecution(ctx, execution)
	if upsertErr != nil {
		t.Fatalf("UPSERT failed: %v", upsertErr)
	}

	// Verify it was created
	retrieved, err := nes.GetNodeExecution(ctx, executionID)
	if err != nil {
		t.Fatalf("Failed to retrieve execution: %v", err)
	}
	if execution.ID != retrieved.ID {
		t.Errorf("ID mismatch: got %v, expected %v", retrieved.ID, execution.ID)
	}
	if execution.NodeID != retrieved.NodeID {
		t.Errorf("NodeID mismatch: got %v, expected %v", retrieved.NodeID, execution.NodeID)
	}
	if execution.Name != retrieved.Name {
		t.Errorf("Name mismatch: got %v, expected %v", retrieved.Name, execution.Name)
	}
	if execution.State != retrieved.State {
		t.Errorf("State mismatch: got %v, expected %v", retrieved.State, execution.State)
	}

	t.Log("UPSERT correctly creates missing records")
}

// TestUpsertUpdatesIfExists verifies that UPSERT updates a record if it exists
func TestUpsertUpdatesIfExists(t *testing.T) {
	ctx := context.Background()
	db, queries := createTestDB(t)
	defer db.Close()

	nes := snodeexecution.New(queries)

	executionID := idwrap.NewNow()
	nodeID := idwrap.NewNow()

	// Create initial execution
	execution := mnodeexecution.NodeExecution{
		ID:                     executionID,
		NodeID:                 nodeID,
		Name:                   "Original",
		State:                  mnnode.NODE_STATE_RUNNING,
		InputData:              []byte(`{"original":true}`),
		InputDataCompressType:  0,
		OutputData:             []byte(`{}`),
		OutputDataCompressType: 0,
	}

	createErr := nes.CreateNodeExecution(ctx, execution)
	if createErr != nil {
		t.Fatalf("Failed to create execution: %v", createErr)
	}

	// Update with UPSERT (only updates fields defined in the UPSERT query)
	execution.State = mnnode.NODE_STATE_SUCCESS
	execution.OutputData = []byte(`{"updated":true}`)
	completedAt := time.Now().UnixMilli()
	execution.CompletedAt = &completedAt

	updateErr := nes.UpsertNodeExecution(ctx, execution)
	if updateErr != nil {
		t.Fatalf("UPSERT update failed: %v", updateErr)
	}

	// Verify it was updated (name stays original, but state/output/completedAt are updated)
	retrieved, err := nes.GetNodeExecution(ctx, executionID)
	if err != nil {
		t.Fatalf("Failed to retrieve updated execution: %v", err)
	}
	if execution.ID != retrieved.ID {
		t.Errorf("ID mismatch: got %v, expected %v", retrieved.ID, execution.ID)
	}
	if "Original" != retrieved.Name {
		t.Errorf("Name should not change on UPSERT: got %v, expected Original", retrieved.Name)
	}
	if mnnode.NODE_STATE_SUCCESS != retrieved.State {
		t.Errorf("State mismatch: got %v, expected %v", retrieved.State, mnnode.NODE_STATE_SUCCESS)
	}
	if `{"updated":true}` != string(retrieved.OutputData) {
		t.Errorf("OutputData mismatch: got %v, expected %v", string(retrieved.OutputData), `{"updated":true}`)
	}

	if retrieved.CompletedAt == nil {
		t.Fatal("CompletedAt should be set")
	}

	t.Log("UPSERT correctly updates existing records")
}

// TestConcurrentUpsertSameExecution tests multiple goroutines updating the same execution ID
func TestConcurrentUpsertSameExecution(t *testing.T) {
	const numGoroutines = 20
	const operationsPerGoroutine = 5

	ctx := context.Background()
	db, queries := createTestDB(t)
	defer db.Close()

	nes := snodeexecution.New(queries)

	// Single execution ID that all goroutines will update
	executionID := idwrap.NewNow()
	nodeID := idwrap.NewNow()

	var (
		successCount = atomic.Int64{}
		errorCount   = atomic.Int64{}
		noRowsErrors = atomic.Int64{}
	)

	t.Logf("Starting concurrency test: %d goroutines updating same execution", numGoroutines)
	startTime := time.Now()

	var wg sync.WaitGroup
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < operationsPerGoroutine; j++ {
				execution := mnodeexecution.NodeExecution{
					ID:                     executionID,
					NodeID:                 nodeID,
					Name:                   fmt.Sprintf("Goroutine%d-Op%d", goroutineID, j),
					State:                  mnnode.NodeState(goroutineID % 4),
					InputData:              []byte(fmt.Sprintf(`{"goroutine":%d,"op":%d}`, goroutineID, j)),
					InputDataCompressType:  0,
					OutputData:             []byte(fmt.Sprintf(`{"result":%d}`, goroutineID*operationsPerGoroutine+j)),
					OutputDataCompressType: 0,
				}

				err := nes.UpsertNodeExecution(ctx, execution)
				if err != nil {
					errorCount.Add(1)
					if strings.Contains(err.Error(), "no rows in result set") {
						noRowsErrors.Add(1)
					}
				} else {
					successCount.Add(1)
				}

				// Small delay to encourage interleaving
				time.Sleep(time.Microsecond * 100)
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(startTime)

	finalSuccess := successCount.Load()
	finalErrors := errorCount.Load()
	finalNoRowsErrors := noRowsErrors.Load()
	totalOps := finalSuccess + finalErrors

	t.Logf("Concurrency test completed in %v", elapsed)
	t.Logf("Total operations: %d", totalOps)
	t.Logf("Successful operations: %d", finalSuccess)
	t.Logf("Failed operations: %d", finalErrors)
	t.Logf("'No rows in result set' errors: %d", finalNoRowsErrors)

	// Verify the execution exists and contains data from one of the updates
	execution, err := nes.GetNodeExecution(ctx, executionID)
	if err != nil {
		t.Fatalf("Failed to retrieve final execution: %v", err)
	}
	if executionID != execution.ID {
		t.Errorf("Final execution ID mismatch: got %v, expected %v", execution.ID, executionID)
	}
	if nodeID != execution.NodeID {
		t.Errorf("Final execution NodeID mismatch: got %v, expected %v", execution.NodeID, nodeID)
	}

	// The key test: no "no rows in result set" errors should occur with proper UPSERT
	if finalNoRowsErrors > 0 {
		t.Errorf("UPSERT failed under concurrency: %d 'no rows in result set' errors", finalNoRowsErrors)
	}

	// Verify reasonable performance
	opsPerSecond := float64(totalOps) / elapsed.Seconds()
	t.Logf("Operations per second: %.2f", opsPerSecond)

	if elapsed > 10*time.Second {
		t.Errorf("Concurrency test took too long: %v", elapsed)
	}

	// Expect high success rate since this is a more controlled test
	successRate := float64(finalSuccess) / float64(totalOps) * 100
	if successRate < 80.0 {
		t.Errorf("Success rate too low: %.2f%% (expected > 80%%)", successRate)
	}

	t.Log("UPSERT implementation handles concurrency correctly")
}

// TestUpsertWithCleanupRace specifically tests the race condition between UPSERT and cleanup
func TestUpsertWithCleanupRace(t *testing.T) {
	const iterations = 100

	ctx := context.Background()
	db, queries := createTestDB(t)
	defer db.Close()

	nes := snodeexecution.New(queries)
	nodeID := idwrap.NewNow()

	var (
		upsertErrors     = atomic.Int64{}
		cleanupErrors    = atomic.Int64{}
		noRowsErrors     = atomic.Int64{}
		raceConditions   = atomic.Int64{}
	)

	t.Logf("Testing UPSERT vs cleanup race condition with %d iterations", iterations)

	var wg sync.WaitGroup

	// Goroutine 1: Continuous UPSERT operations
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			executionID := idwrap.NewNow()
			execution := mnodeexecution.NodeExecution{
				ID:                     executionID,
				NodeID:                 nodeID,
				Name:                   fmt.Sprintf("Race-Test-%d", i),
				State:                  mnnode.NODE_STATE_RUNNING,
				InputData:              []byte(`{"race":"test"}`),
				InputDataCompressType:  0,
				OutputData:             []byte(`{}`),
				OutputDataCompressType: 0,
			}

			err := nes.UpsertNodeExecution(ctx, execution)
			if err != nil {
				upsertErrors.Add(1)
				if strings.Contains(err.Error(), "no rows in result set") {
					noRowsErrors.Add(1)
					raceConditions.Add(1)
				}
			}

			// Small delay to allow cleanup to interleave
			if i%20 == 0 {
				time.Sleep(time.Millisecond)
			}
		}
	}()

	// Goroutine 2: Continuous cleanup operations
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations/5; i++ {
			err := nes.DeleteNodeExecutionsByNodeID(ctx, nodeID)
			if err != nil {
				cleanupErrors.Add(1)
			}

			// Brief pause
			time.Sleep(5 * time.Millisecond)
		}
	}()

	wg.Wait()

	finalUpsertErrors := upsertErrors.Load()
	finalCleanupErrors := cleanupErrors.Load()
	finalNoRowsErrors := noRowsErrors.Load()
	finalRaceConditions := raceConditions.Load()

	t.Logf("Race condition test results:")
	t.Logf("UPSERT errors: %d", finalUpsertErrors)
	t.Logf("Cleanup errors: %d", finalCleanupErrors)
	t.Logf("'No rows in result set' errors: %d", finalNoRowsErrors)
	t.Logf("Detected race conditions: %d", finalRaceConditions)

	// The critical test: UPSERT should handle the race condition gracefully
	if finalRaceConditions > 0 {
		t.Errorf("UPSERT implementation vulnerable to race conditions: %d occurrences detected", finalRaceConditions)
	}

	t.Log("UPSERT implementation properly handles cleanup race conditions")
}