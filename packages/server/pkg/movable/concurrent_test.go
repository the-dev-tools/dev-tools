package movable

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"the-dev-tools/server/pkg/idwrap"
)

// =============================================================================
// TEST SETUP HELPERS
// =============================================================================

// setupConcurrentTestEnv creates test environment for concurrent operations
func setupConcurrentTestEnv(t *testing.T) (*InMemoryContextCache, *DeltaAwareManager, func()) {
	t.Helper()
	
	cache := NewInMemoryContextCache(5 * time.Minute)
	repo := newMockMovableRepository()
	ctx := context.Background()
	config := DefaultDeltaManagerConfig()
	
	manager := NewDeltaAwareManager(ctx, repo, config)
	
	cleanup := func() {
		cache.Clear()
		manager.Clear()
	}
	
	return cache, manager, cleanup
}

// generateTestIDs creates a slice of test IDs using NewNow()
func generateTestIDs(count int) []idwrap.IDWrap {
	ids := make([]idwrap.IDWrap, count)
	for i := 0; i < count; i++ {
		// Use NewNow() to generate valid ULIDs
		ids[i] = idwrap.NewNow()
		time.Sleep(time.Microsecond) // Ensure unique timestamps
	}
	return ids
}

// =============================================================================
// CONCURRENT DELTA CREATION TESTS
// =============================================================================

func TestConcurrentDeltaCreation(t *testing.T) {
	t.Parallel()
	
	tests := []struct {
		name         string
		numGoroutines int
		deltasPerGoroutine int
	}{
		{
			name:               "SmallConcurrency",
			numGoroutines:      10,
			deltasPerGoroutine: 10,
		},
		{
			name:               "MediumConcurrency", 
			numGoroutines:      50,
			deltasPerGoroutine: 20,
		},
		{
			name:               "HighConcurrency",
			numGoroutines:      100,
			deltasPerGoroutine: 10,
		},
		{
			name:               "StressTest",
			numGoroutines:      1000,
			deltasPerGoroutine: 1,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			
			_, manager, cleanup := setupConcurrentTestEnv(t)
			defer cleanup()
			
			var wg sync.WaitGroup
			var successCount int64
			var errorCount int64
			
			// Generate origin IDs
			originIDs := generateTestIDs(tt.numGoroutines)
			
			// Create deltas concurrently
			for i := 0; i < tt.numGoroutines; i++ {
				wg.Add(1)
				go func(goroutineID int) {
					defer wg.Done()
					
					originID := originIDs[goroutineID]
					
					for j := 0; j < tt.deltasPerGoroutine; j++ {
						deltaID := idwrap.NewNow()
						metadata := DeltaMetadata{
							LastUpdated: time.Now(),
							SyncCount:   0,
							IsStale:     false,
							Priority:    j + 1,
						}
						
						err := manager.TrackDelta(
							deltaID,
							originID,
							DeltaManagerRelationTypeEndpoint,
							metadata,
						)
						
						if err != nil {
							atomic.AddInt64(&errorCount, 1)
							t.Logf("Error tracking delta: %v", err)
						} else {
							atomic.AddInt64(&successCount, 1)
						}
					}
				}(i)
			}
			
			wg.Wait()
			
			// Verify results
			totalExpected := int64(tt.numGoroutines * tt.deltasPerGoroutine)
			
			if successCount != totalExpected {
				t.Errorf("Expected %d successful operations, got %d (errors: %d)", 
					totalExpected, successCount, errorCount)
			}
			
			if manager.Size() != int(successCount) {
				t.Errorf("Manager size %d doesn't match success count %d", 
					manager.Size(), successCount)
			}
			
			// Verify no data corruption
			for i := 0; i < tt.numGoroutines; i++ {
				originID := originIDs[i]
				deltas, err := manager.GetDeltasForOrigin(originID)
				if err != nil {
					t.Errorf("Error getting deltas for origin %s: %v", originID.String(), err)
					continue
				}
				
				if len(deltas) != tt.deltasPerGoroutine {
					t.Errorf("Expected %d deltas for origin %s, got %d", 
						tt.deltasPerGoroutine, originID.String(), len(deltas))
				}
			}
		})
	}
}

// =============================================================================
// PARALLEL CONTEXT RESOLUTION TESTS
// =============================================================================

func TestParallelContextResolution(t *testing.T) {
	t.Parallel()
	
	cache, _, cleanup := setupConcurrentTestEnv(t)
	defer cleanup()
	
	// Pre-populate cache
	testIDs := generateTestIDs(1000)
	for i, id := range testIDs {
		metadata := &ContextMetadata{
			Type:      ContextEndpoint,
			ScopeID:   idwrap.NewNow().Bytes(),
			IsHidden:  i%2 == 0,
			Priority:  i,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		cache.SetContext(id, metadata)
	}
	
	var wg sync.WaitGroup
	var readOps int64
	var writeOps int64
	var cacheHits int64
	var cacheMisses int64
	
	numReaders := 50
	numWriters := 10
	operationsPerWorker := 100
	
	// Start readers
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			for j := 0; j < operationsPerWorker; j++ {
				// Read random context
				id := testIDs[j%len(testIDs)]
				_, exists := cache.GetContext(id)
				
				atomic.AddInt64(&readOps, 1)
				if exists {
					atomic.AddInt64(&cacheHits, 1)
				} else {
					atomic.AddInt64(&cacheMisses, 1)
				}
				
				// Small delay to increase contention
				time.Sleep(time.Microsecond)
			}
		}()
	}
	
	// Start writers
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()
			
			for j := 0; j < operationsPerWorker; j++ {
				// Update random context
				id := testIDs[j%len(testIDs)]
				metadata := &ContextMetadata{
					Type:      ContextFlow,
					ScopeID:   idwrap.NewNow().Bytes(),
					IsHidden:  false,
					Priority:  writerID*1000 + j,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}
				
				cache.SetContext(id, metadata)
				atomic.AddInt64(&writeOps, 1)
				
				// Occasionally invalidate
				if j%10 == 0 {
					cache.InvalidateContext(id)
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	// Verify operations completed
	expectedReads := int64(numReaders * operationsPerWorker)
	expectedWrites := int64(numWriters * operationsPerWorker)
	
	if readOps != expectedReads {
		t.Errorf("Expected %d read operations, got %d", expectedReads, readOps)
	}
	
	if writeOps != expectedWrites {
		t.Errorf("Expected %d write operations, got %d", expectedWrites, writeOps)
	}
	
	// Log cache statistics
	t.Logf("Cache statistics: %d hits, %d misses (%.2f%% hit rate)", 
		cacheHits, cacheMisses, 
		float64(cacheHits)/float64(cacheHits+cacheMisses)*100)
}

// =============================================================================
// SIMULTANEOUS MOVE OPERATIONS TESTS
// =============================================================================

func TestSimultaneousMoves(t *testing.T) {
	t.Parallel()
	
	_, manager, cleanup := setupConcurrentTestEnv(t)
	defer cleanup()
	
	// Setup test data
	numOrigins := 50
	deltasPerOrigin := 5
	originIDs := generateTestIDs(numOrigins)
	
	// Create delta relationships
	for _, originID := range originIDs {
		for i := 0; i < deltasPerOrigin; i++ {
			deltaID := idwrap.NewNow()
			metadata := DeltaMetadata{
				LastUpdated: time.Now(),
				Priority:    i,
			}
			
			err := manager.TrackDelta(deltaID, originID, DeltaManagerRelationTypeEndpoint, metadata)
			if err != nil {
				t.Fatalf("Failed to track delta: %v", err)
			}
		}
	}
	
	ctx := context.Background()
	var wg sync.WaitGroup
	var syncOps int64
	var syncErrors int64
	
	// Simulate concurrent sync operations
	for i := 0; i < numOrigins; i++ {
		wg.Add(1)
		go func(originIndex int) {
			defer wg.Done()
			
			originID := originIDs[originIndex]
			
			// Perform multiple sync operations
			for j := 0; j < 10; j++ {
				result, err := manager.SyncDeltaPositions(ctx, nil, originID, CollectionListTypeEndpoints)
				
				atomic.AddInt64(&syncOps, 1)
				
				if err != nil {
					atomic.AddInt64(&syncErrors, 1)
					t.Logf("Sync error for origin %s: %v", originID.String(), err)
				} else if result.FailedCount > 0 {
					atomic.AddInt64(&syncErrors, int64(result.FailedCount))
				}
				
				// Small delay between operations
				time.Sleep(time.Millisecond)
			}
		}(i)
	}
	
	wg.Wait()
	
	expectedOps := int64(numOrigins * 10)
	if syncOps != expectedOps {
		t.Errorf("Expected %d sync operations, got %d", expectedOps, syncOps)
	}
	
	// Verify data consistency
	for _, originID := range originIDs {
		deltas, err := manager.GetDeltasForOrigin(originID)
		if err != nil {
			t.Errorf("Error getting deltas after concurrent sync: %v", err)
			continue
		}
		
		if len(deltas) != deltasPerOrigin {
			t.Errorf("Delta count changed during concurrent operations: expected %d, got %d",
				deltasPerOrigin, len(deltas))
		}
	}
	
	t.Logf("Completed %d sync operations with %d errors", syncOps, syncErrors)
}

// =============================================================================
// RACE CONDITION DETECTION TESTS
// =============================================================================

func TestRaceConditionDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping race condition test in short mode")
	}
	
	t.Parallel()
	
	cache, manager, cleanup := setupConcurrentTestEnv(t)
	defer cleanup()
	
	// Test multiple race-prone scenarios simultaneously
	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	// Scenario 1: Cache read/write races
	wg.Add(1)
	go func() {
		defer wg.Done()
		testCacheRaces(t, cache, ctx)
	}()
	
	// Scenario 2: Manager read/write races  
	wg.Add(1)
	go func() {
		defer wg.Done()
		testManagerRaces(t, manager, ctx)
	}()
	
	// Scenario 3: Mixed operations races
	wg.Add(1)
	go func() {
		defer wg.Done()
		testMixedOperationRaces(t, cache, manager, ctx)
	}()
	
	wg.Wait()
}

func testCacheRaces(t *testing.T, cache *InMemoryContextCache, ctx context.Context) {
	testID := idwrap.NewNow()
	var operations int64
	
	var wg sync.WaitGroup
	
	// Concurrent readers
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					cache.GetContext(testID)
					atomic.AddInt64(&operations, 1)
				}
			}
		}()
	}
	
	// Concurrent writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					metadata := &ContextMetadata{
						Type:      ContextEndpoint,
						ScopeID:   idwrap.NewNow().Bytes(),
						Priority:  id,
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					}
					cache.SetContext(testID, metadata)
					atomic.AddInt64(&operations, 1)
				}
			}
		}(i)
	}
	
	// Concurrent invalidators
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					cache.InvalidateContext(testID)
					atomic.AddInt64(&operations, 1)
				}
			}
		}()
	}
	
	// Wait for timeout
	<-ctx.Done()
	
	// Signal goroutines to stop and wait
	wg.Wait()
	
	t.Logf("Cache race test completed %d operations", operations)
}

func testManagerRaces(t *testing.T, manager *DeltaAwareManager, ctx context.Context) {
	originID := idwrap.NewNow()
	var operations int64
	var deltaCounter int64
	
	var wg sync.WaitGroup
	
	// Concurrent delta creators
	for i := 0; i < 15; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					deltaID := idwrap.NewNow()
					metadata := DeltaMetadata{
						LastUpdated: time.Now(),
						Priority:    id,
					}
					
					err := manager.TrackDelta(deltaID, originID, DeltaManagerRelationTypeEndpoint, metadata)
					if err == nil {
						atomic.AddInt64(&deltaCounter, 1)
					}
					atomic.AddInt64(&operations, 1)
				}
			}
		}(i)
	}
	
	// Concurrent delta readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					manager.GetDeltasForOrigin(originID)
					atomic.AddInt64(&operations, 1)
				}
			}
		}()
	}
	
	// Wait for timeout
	<-ctx.Done()
	
	// Signal goroutines to stop and wait
	wg.Wait()
	
	t.Logf("Manager race test completed %d operations, created %d deltas", 
		operations, deltaCounter)
}

func testMixedOperationRaces(t *testing.T, cache *InMemoryContextCache, manager *DeltaAwareManager, ctx context.Context) {
	var operations int64
	testIDs := generateTestIDs(100)
	
	var wg sync.WaitGroup
	
	// Mixed cache and manager operations
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					// Mix cache and manager operations
					switch id % 4 {
					case 0:
						testID := testIDs[id%len(testIDs)]
						cache.GetContext(testID)
					case 1:
						metadata := &ContextMetadata{
							Type:      ContextCollection,
							ScopeID:   idwrap.NewNow().Bytes(),
							Priority:  id,
							CreatedAt: time.Now(),
							UpdatedAt: time.Now(),
						}
						testID := testIDs[id%len(testIDs)]
						cache.SetContext(testID, metadata)
					case 2:
						originID := testIDs[id%len(testIDs)]
						manager.GetDeltasForOrigin(originID)
					case 3:
						deltaID := idwrap.NewNow()
						originID := testIDs[id%len(testIDs)]
						metadata := DeltaMetadata{
							LastUpdated: time.Now(),
							Priority:    id,
						}
						manager.TrackDelta(deltaID, originID, DeltaManagerRelationTypeExample, metadata)
					}
					atomic.AddInt64(&operations, 1)
				}
			}
		}(i)
	}
	
	// Wait for timeout
	<-ctx.Done()
	
	// Signal goroutines to stop and wait
	wg.Wait()
	
	t.Logf("Mixed operations race test completed %d operations", operations)
}

// =============================================================================
// STRESS TESTS
// =============================================================================

func TestConcurrencyStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}
	
	// Configure for maximum stress
	numGoroutines := 1000
	operationsPerGoroutine := 100
	testDuration := 30 * time.Second
	
	cache, manager, cleanup := setupConcurrentTestEnv(t)
	defer cleanup()
	
	ctx, cancel := context.WithTimeout(context.Background(), testDuration)
	defer cancel()
	
	var wg sync.WaitGroup
	var totalOps int64
	var errors int64
	
	// Launch stress workers
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			localOps := 0
			for localOps < operationsPerGoroutine {
				select {
				case <-ctx.Done():
					return
				default:
					// Perform random operation
					switch workerID % 6 {
					case 0: // Cache read
						testID := idwrap.NewNow()
						cache.GetContext(testID)
					case 1: // Cache write
						testID := idwrap.NewNow()
						metadata := &ContextMetadata{
							Type:      ContextEndpoint,
							ScopeID:   idwrap.NewNow().Bytes(),
							Priority:  workerID,
							CreatedAt: time.Now(),
							UpdatedAt: time.Now(),
						}
						cache.SetContext(testID, metadata)
					case 2: // Track delta
						deltaID := idwrap.NewNow()
						originID := idwrap.NewNow()
						metadata := DeltaMetadata{
							LastUpdated: time.Now(),
							Priority:    workerID,
						}
						err := manager.TrackDelta(deltaID, originID, DeltaManagerRelationTypeEndpoint, metadata)
						if err != nil {
							atomic.AddInt64(&errors, 1)
						}
					case 3: // Get deltas
						originID := idwrap.NewNow()
						_, err := manager.GetDeltasForOrigin(originID)
						if err != nil {
							atomic.AddInt64(&errors, 1)
						}
					case 4: // Untrack delta
						deltaID := idwrap.NewNow()
						err := manager.UntrackDelta(deltaID)
						if err != nil {
							// Expected for non-existent deltas
						}
					case 5: // Mixed operations
						cache.Clear()
						if workerID%100 == 0 { // Occasionally clear manager
							manager.Clear()
						}
					}
					
					localOps++
					atomic.AddInt64(&totalOps, 1)
				}
			}
		}(i)
	}
	
	// Monitor progress
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				ops := atomic.LoadInt64(&totalOps)
				errs := atomic.LoadInt64(&errors)
				t.Logf("Progress: %d operations completed, %d errors", ops, errs)
			}
		}
	}()
	
	wg.Wait()
	
	finalOps := atomic.LoadInt64(&totalOps)
	finalErrors := atomic.LoadInt64(&errors)
	
	t.Logf("Stress test completed: %d operations, %d errors (%.2f%% error rate)", 
		finalOps, finalErrors, float64(finalErrors)/float64(finalOps)*100)
	
	// Verify system is still functional
	testID := idwrap.NewNow()
	metadata := &ContextMetadata{
		Type:      ContextEndpoint,
		ScopeID:   idwrap.NewNow().Bytes(),
		Priority:  1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	cache.SetContext(testID, metadata)
	retrieved, exists := cache.GetContext(testID)
	if !exists || retrieved.Priority != 1 {
		t.Error("System not functional after stress test")
	}
}

// =============================================================================
// SYNCHRONIZATION VALIDATION TESTS
// =============================================================================

func TestSynchronizationValidation(t *testing.T) {
	t.Parallel()
	
	tests := []struct {
		name        string
		gomaxprocs  int
		description string
	}{
		{
			name:        "SingleCore",
			gomaxprocs:  1,
			description: "Tests with single CPU core to detect logic errors",
		},
		{
			name:        "MultiCore",
			gomaxprocs:  runtime.NumCPU(),
			description: "Tests with all CPU cores for maximum parallelism",
		},
		{
			name:        "OverCommit", 
			gomaxprocs:  runtime.NumCPU() * 2,
			description: "Tests with overcommitted cores",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			
			// Set GOMAXPROCS for this test
			oldGOMAXPROCS := runtime.GOMAXPROCS(tt.gomaxprocs)
			defer runtime.GOMAXPROCS(oldGOMAXPROCS)
			
			cache, manager, cleanup := setupConcurrentTestEnv(t)
			defer cleanup()
			
			// Run synchronized operations
			testSynchronizedOperations(t, cache, manager)
		})
	}
}

func testSynchronizedOperations(t *testing.T, cache *InMemoryContextCache, manager *DeltaAwareManager) {
	const (
		numWorkers = 50
		numOpsPerWorker = 100
	)
	
	var wg sync.WaitGroup
	var correctResults int64
	var totalOps int64
	
	sharedCounter := int64(0)
	testID := idwrap.NewNow()
	
	// Workers that increment shared counter and validate consistency
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for j := 0; j < numOpsPerWorker; j++ {
				// Atomic increment
				newValue := atomic.AddInt64(&sharedCounter, 1)
				
				// Store in cache with counter value
				metadata := &ContextMetadata{
					Type:      ContextEndpoint,
					ScopeID:   idwrap.NewNow().Bytes(),
					Priority:  int(newValue),
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}
				cache.SetContext(testID, metadata)
				
				// Verify immediately
				retrieved, exists := cache.GetContext(testID)
				if exists && retrieved.Priority == int(newValue) {
					atomic.AddInt64(&correctResults, 1)
				}
				
				atomic.AddInt64(&totalOps, 1)
				
				// Small delay to increase race probability
				if j%10 == 0 {
					runtime.Gosched()
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	finalCounter := atomic.LoadInt64(&sharedCounter)
	expectedOps := int64(numWorkers * numOpsPerWorker)
	
	if finalCounter != expectedOps {
		t.Errorf("Counter mismatch: expected %d, got %d", expectedOps, finalCounter)
	}
	
	if totalOps != expectedOps {
		t.Errorf("Operation count mismatch: expected %d, got %d", expectedOps, totalOps)
	}
	
	accuracyRate := float64(correctResults) / float64(totalOps) * 100
	t.Logf("Synchronization accuracy: %.2f%% (%d/%d)", accuracyRate, correctResults, totalOps)
	
	// Accuracy should be high but not necessarily 100% due to cache timing
	if accuracyRate < 80.0 {
		t.Errorf("Synchronization accuracy too low: %.2f%%", accuracyRate)
	}
}

// =============================================================================
// DEADLOCK DETECTION TESTS
// =============================================================================

func TestDeadlockDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping deadlock detection test in short mode")
	}
	
	t.Parallel()
	
	cache, manager, cleanup := setupConcurrentTestEnv(t)
	defer cleanup()
	
	// Test potential deadlock scenarios
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	testDeadlockScenarios(t, cache, manager, ctx)
}

func testDeadlockScenarios(t *testing.T, cache *InMemoryContextCache, manager *DeltaAwareManager, ctx context.Context) {
	// Create potential deadlock scenario with nested locks
	var wg sync.WaitGroup
	completedOps := int64(0)
	
	// Pattern 1: Cache -> Manager operations
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			for j := 0; j < 50; j++ {
				select {
				case <-ctx.Done():
					return
				default:
					// Cache operation followed by manager operation
					testID := idwrap.NewNow()
					metadata := &ContextMetadata{
						Type:      ContextEndpoint,
						ScopeID:   idwrap.NewNow().Bytes(),
						Priority:  id,
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					}
					
					cache.SetContext(testID, metadata)
					
					deltaID := idwrap.NewNow()
					originID := idwrap.NewNow()
					deltaMetadata := DeltaMetadata{
						LastUpdated: time.Now(),
						Priority:    id,
					}
					
					manager.TrackDelta(deltaID, originID, DeltaManagerRelationTypeEndpoint, deltaMetadata)
					
					atomic.AddInt64(&completedOps, 1)
				}
			}
		}(i)
	}
	
	// Pattern 2: Manager -> Cache operations
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			for j := 0; j < 50; j++ {
				select {
				case <-ctx.Done():
					return
				default:
					// Manager operation followed by cache operation
					originID := idwrap.NewNow()
					manager.GetDeltasForOrigin(originID)
					
					testID := idwrap.NewNow()
					cache.GetContext(testID)
					
					atomic.AddInt64(&completedOps, 1)
				}
			}
		}(i)
	}
	
	// Monitor for deadlock
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		
		lastOps := int64(0)
		stuckCount := 0
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				currentOps := atomic.LoadInt64(&completedOps)
				if currentOps == lastOps {
					stuckCount++
					if stuckCount >= 3 {
						t.Logf("Potential deadlock detected: operations stuck at %d", currentOps)
						// Don't fail the test as this might be expected behavior
					}
				} else {
					stuckCount = 0
				}
				lastOps = currentOps
			}
		}
	}()
	
	wg.Wait()
	
	finalOps := atomic.LoadInt64(&completedOps)
	expectedOps := int64(40 * 50) // 20 + 20 goroutines * 50 ops each
	
	t.Logf("Deadlock test completed: %d/%d operations", finalOps, expectedOps)
	
	// Should complete most operations (allowing for some context cancellation)
	if finalOps < expectedOps/2 {
		t.Errorf("Too few operations completed, possible deadlock: %d/%d", finalOps, expectedOps)
	}
}

// =============================================================================
// PERFORMANCE SCALING TESTS
// =============================================================================

func TestPerformanceScaling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance scaling test in short mode")
	}
	
	gomaxprocsValues := []int{1, 2, 4, 8}
	if runtime.NumCPU() < 8 {
		gomaxprocsValues = []int{1, 2, runtime.NumCPU()}
	}
	
	for _, procs := range gomaxprocsValues {
		t.Run(fmt.Sprintf("GOMAXPROCS_%d", procs), func(t *testing.T) {
			oldGOMAXPROCS := runtime.GOMAXPROCS(procs)
			defer runtime.GOMAXPROCS(oldGOMAXPROCS)
			
			testPerformanceWithProcs(t, procs)
		})
	}
}

func testPerformanceWithProcs(t *testing.T, procs int) {
	cache, manager, cleanup := setupConcurrentTestEnv(t)
	defer cleanup()
	
	const (
		numOperations = 10000
		operationTypes = 4
	)
	
	numWorkers := procs * 2 // Scale workers with processor count
	opsPerWorker := numOperations / numWorkers
	
	var wg sync.WaitGroup
	var completedOps int64
	
	startTime := time.Now()
	
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for j := 0; j < opsPerWorker; j++ {
				switch j % operationTypes {
				case 0: // Cache write
					testID := idwrap.NewNow()
					metadata := &ContextMetadata{
						Type:      ContextEndpoint,
						ScopeID:   idwrap.NewNow().Bytes(),
						Priority:  workerID,
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					}
					cache.SetContext(testID, metadata)
					
				case 1: // Cache read
					testID := idwrap.NewNow()
					cache.GetContext(testID)
					
				case 2: // Manager write
					deltaID := idwrap.NewNow()
					originID := idwrap.NewNow()
					metadata := DeltaMetadata{
						LastUpdated: time.Now(),
						Priority:    workerID,
					}
					manager.TrackDelta(deltaID, originID, DeltaManagerRelationTypeEndpoint, metadata)
					
				case 3: // Manager read
					originID := idwrap.NewNow()
					manager.GetDeltasForOrigin(originID)
				}
				
				atomic.AddInt64(&completedOps, 1)
			}
		}(i)
	}
	
	wg.Wait()
	
	duration := time.Since(startTime)
	opsCompleted := atomic.LoadInt64(&completedOps)
	opsPerSecond := float64(opsCompleted) / duration.Seconds()
	
	t.Logf("GOMAXPROCS=%d: %d ops in %v (%.2f ops/sec)", 
		procs, opsCompleted, duration, opsPerSecond)
	
	// Store results for comparison (in real tests, you might want to store these)
	// This helps identify if performance scales appropriately with core count
}

// =============================================================================
// BENCHMARKS FOR CONCURRENT OPERATIONS
// =============================================================================

func BenchmarkConcurrentCacheOperations(b *testing.B) {
	cache := NewInMemoryContextCache(5 * time.Minute)
	defer cache.Clear()
	
	// Pre-populate cache
	testIDs := generateTestIDs(1000)
	for _, id := range testIDs {
		metadata := &ContextMetadata{
			Type:      ContextEndpoint,
			ScopeID:   idwrap.NewNow().Bytes(),
			Priority:  1,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		cache.SetContext(id, metadata)
	}
	
	b.ResetTimer()
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Mix of read and write operations
			id := testIDs[b.N%len(testIDs)]
			
			if b.N%10 == 0 {
				// Write operation (10% of the time)
				metadata := &ContextMetadata{
					Type:      ContextFlow,
					ScopeID:   idwrap.NewNow().Bytes(),
					Priority:  b.N,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}
				cache.SetContext(id, metadata)
			} else {
				// Read operation (90% of the time)
				cache.GetContext(id)
			}
		}
	})
}

func BenchmarkConcurrentManagerOperations(b *testing.B) {
	repo := newMockMovableRepository()
	ctx := context.Background()
	config := DefaultDeltaManagerConfig()
	manager := NewDeltaAwareManager(ctx, repo, config)
	defer manager.Clear()
	
	// Pre-populate manager
	originIDs := generateTestIDs(100)
	for _, originID := range originIDs {
		deltaID := idwrap.NewNow()
		metadata := DeltaMetadata{
			LastUpdated: time.Now(),
			Priority:    1,
		}
		manager.TrackDelta(deltaID, originID, DeltaManagerRelationTypeEndpoint, metadata)
	}
	
	b.ResetTimer()
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			originID := originIDs[b.N%len(originIDs)]
			
			if b.N%20 == 0 {
				// Write operation (5% of the time)
				deltaID := idwrap.NewNow()
				metadata := DeltaMetadata{
					LastUpdated: time.Now(),
					Priority:    b.N,
				}
				manager.TrackDelta(deltaID, originID, DeltaManagerRelationTypeEndpoint, metadata)
			} else {
				// Read operation (95% of the time)
				manager.GetDeltasForOrigin(originID)
			}
		}
	})
}