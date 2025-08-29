package movable

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"
	"the-dev-tools/server/pkg/idwrap"
)

// =============================================================================
// TEST HELPERS (reusing mock from mock_repository.go)
// =============================================================================

// createTestIDsForManager creates test ID wrappers for delta manager tests
func createTestIDsForManager(count int) []idwrap.IDWrap {
	ids := make([]idwrap.IDWrap, count)
	for i := 0; i < count; i++ {
		ids[i] = idwrap.NewNow() // Use actual ULID for test
	}
	return ids
}

// =============================================================================
// CONSTRUCTOR AND BASIC FUNCTIONALITY TESTS
// =============================================================================

func TestNewDeltaAwareManager(t *testing.T) {
	ctx := context.Background()
	repo := newMockMovableRepository()
	config := DefaultDeltaManagerConfig()
	
	manager := NewDeltaAwareManager(ctx, repo, config)
	
	if manager == nil {
		t.Fatal("NewDeltaAwareManager returned nil")
	}
	
	if manager.repo != repo {
		t.Error("Repository not set correctly")
	}
	
	if manager.ctx != ctx {
		t.Error("Context not set correctly")
	}
	
	if manager.config != config {
		t.Error("Config not set correctly")
	}
	
	if manager.Size() != 0 {
		t.Error("New manager should have zero relationships")
	}
}

func TestDefaultDeltaManagerConfig(t *testing.T) {
	config := DefaultDeltaManagerConfig()
	
	if config.CacheSize <= 0 {
		t.Error("Cache size should be positive")
	}
	
	if config.CacheTTL <= 0 {
		t.Error("Cache TTL should be positive")
	}
	
	if config.BatchSize <= 0 {
		t.Error("Batch size should be positive")
	}
	
	if config.BatchTimeout <= 0 {
		t.Error("Batch timeout should be positive")
	}
}

// =============================================================================
// DELTA TRACKING TESTS
// =============================================================================

func TestTrackDelta(t *testing.T) {
	manager := setupTestManager(t)
	ids := createTestIDsForManager(2)
	deltaID, originID := ids[0], ids[1]
	
	metadata := DeltaMetadata{
		LastUpdated: time.Now(),
		SyncCount:   0,
		IsStale:     false,
		Priority:    1,
	}
	
	err := manager.TrackDelta(deltaID, originID, DeltaManagerRelationTypeEndpoint, metadata)
	if err != nil {
		t.Fatalf("Failed to track delta: %v", err)
	}
	
	if manager.Size() != 1 {
		t.Error("Manager should have exactly one relationship")
	}
	
	if !manager.HasDelta(deltaID) {
		t.Error("Manager should have the tracked delta")
	}
}

func TestTrackDeltaValidation(t *testing.T) {
	manager := setupTestManager(t)
	ids := createTestIDsForManager(2)
	deltaID, originID := ids[0], ids[1]
	
	tests := []struct {
		name     string
		deltaID  idwrap.IDWrap
		originID idwrap.IDWrap
		wantErr  bool
	}{
		{
			name:     "empty delta ID",
			deltaID:  idwrap.IDWrap{},
			originID: originID,
			wantErr:  true,
		},
		{
			name:     "empty origin ID",
			deltaID:  deltaID,
			originID: idwrap.IDWrap{},
			wantErr:  true,
		},
		{
			name:     "valid IDs",
			deltaID:  deltaID,
			originID: originID,
			wantErr:  false,
		},
	}
	
	metadata := DeltaMetadata{}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.TrackDelta(tt.deltaID, tt.originID, DeltaManagerRelationTypeEndpoint, metadata)
			if (err != nil) != tt.wantErr {
				t.Errorf("TrackDelta() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTrackDeltaDuplicate(t *testing.T) {
	manager := setupTestManager(t)
	ids := createTestIDsForManager(3)
	deltaID, originID1, originID2 := ids[0], ids[1], ids[2]
	
	metadata := DeltaMetadata{}
	
	// Track first relationship
	err := manager.TrackDelta(deltaID, originID1, DeltaManagerRelationTypeEndpoint, metadata)
	if err != nil {
		t.Fatalf("Failed to track first delta: %v", err)
	}
	
	// Try to track duplicate
	err = manager.TrackDelta(deltaID, originID2, DeltaManagerRelationTypeEndpoint, metadata)
	if err == nil {
		t.Error("Expected error when tracking duplicate delta")
	}
}

func TestUntrackDelta(t *testing.T) {
	manager := setupTestManager(t)
	ids := createTestIDsForManager(2)
	deltaID, originID := ids[0], ids[1]
	
	metadata := DeltaMetadata{}
	
	// Track delta first
	err := manager.TrackDelta(deltaID, originID, DeltaManagerRelationTypeEndpoint, metadata)
	if err != nil {
		t.Fatalf("Failed to track delta: %v", err)
	}
	
	// Untrack delta
	err = manager.UntrackDelta(deltaID)
	if err != nil {
		t.Fatalf("Failed to untrack delta: %v", err)
	}
	
	if manager.Size() != 0 {
		t.Error("Manager should have zero relationships after untracking")
	}
	
	if manager.HasDelta(deltaID) {
		t.Error("Manager should not have the untracked delta")
	}
}

func TestUntrackDeltaNotFound(t *testing.T) {
	manager := setupTestManager(t)
	ids := createTestIDsForManager(1)
	deltaID := ids[0]
	
	err := manager.UntrackDelta(deltaID)
	if err == nil {
		t.Error("Expected error when untracking non-existent delta")
	}
}

func TestGetDeltasForOrigin(t *testing.T) {
	manager := setupTestManager(t)
	ids := createTestIDsForManager(4)
	originID := ids[0]
	delta1, delta2, delta3 := ids[1], ids[2], ids[3]
	
	metadata := DeltaMetadata{}
	
	// Track multiple deltas for same origin
	err := manager.TrackDelta(delta1, originID, DeltaManagerRelationTypeEndpoint, metadata)
	if err != nil {
		t.Fatalf("Failed to track delta1: %v", err)
	}
	
	err = manager.TrackDelta(delta2, originID, DeltaManagerRelationTypeExample, metadata)
	if err != nil {
		t.Fatalf("Failed to track delta2: %v", err)
	}
	
	// Track delta for different origin
	err = manager.TrackDelta(delta3, ids[3], DeltaManagerRelationTypeHeader, metadata)
	if err != nil {
		t.Fatalf("Failed to track delta3: %v", err)
	}
	
	// Get deltas for origin
	deltas, err := manager.GetDeltasForOrigin(originID)
	if err != nil {
		t.Fatalf("Failed to get deltas for origin: %v", err)
	}
	
	if len(deltas) != 2 {
		t.Errorf("Expected 2 deltas, got %d", len(deltas))
	}
	
	// Verify correct deltas returned
	deltaIDs := make(map[idwrap.IDWrap]bool)
	for _, delta := range deltas {
		deltaIDs[delta.DeltaID] = true
		if delta.OriginID != originID {
			t.Error("Delta has wrong origin ID")
		}
	}
	
	if !deltaIDs[delta1] || !deltaIDs[delta2] {
		t.Error("Missing expected delta IDs")
	}
	
	if deltaIDs[delta3] {
		t.Error("Unexpected delta ID returned")
	}
}

func TestGetDeltasForOriginEmpty(t *testing.T) {
	manager := setupTestManager(t)
	ids := createTestIDsForManager(1)
	originID := ids[0]
	
	deltas, err := manager.GetDeltasForOrigin(originID)
	if err != nil {
		t.Fatalf("Failed to get deltas for origin: %v", err)
	}
	
	if len(deltas) != 0 {
		t.Errorf("Expected 0 deltas, got %d", len(deltas))
	}
}

func TestGetDeltaRelationship(t *testing.T) {
	manager := setupTestManager(t)
	ids := createTestIDsForManager(2)
	deltaID, originID := ids[0], ids[1]
	
	expectedMetadata := DeltaMetadata{
		LastUpdated: time.Now(),
		SyncCount:   5,
		IsStale:     false,
		Priority:    3,
	}
	
	// Track delta
	err := manager.TrackDelta(deltaID, originID, DeltaManagerRelationTypeEndpoint, expectedMetadata)
	if err != nil {
		t.Fatalf("Failed to track delta: %v", err)
	}
	
	// Get relationship
	relationship, err := manager.GetDeltaRelationship(deltaID)
	if err != nil {
		t.Fatalf("Failed to get delta relationship: %v", err)
	}
	
	if relationship.DeltaID != deltaID {
		t.Error("Wrong delta ID in relationship")
	}
	
	if relationship.OriginID != originID {
		t.Error("Wrong origin ID in relationship")
	}
	
	if relationship.RelationType != DeltaManagerRelationTypeEndpoint {
		t.Error("Wrong relation type")
	}
	
	if relationship.Metadata.SyncCount != expectedMetadata.SyncCount {
		t.Error("Wrong sync count in metadata")
	}
}

func TestGetDeltaRelationshipNotFound(t *testing.T) {
	manager := setupTestManager(t)
	ids := createTestIDsForManager(1)
	deltaID := ids[0]
	
	_, err := manager.GetDeltaRelationship(deltaID)
	if err == nil {
		t.Error("Expected error when getting non-existent delta relationship")
	}
}

// =============================================================================
// SYNCHRONIZATION TESTS
// =============================================================================

func TestSyncDeltaPositions(t *testing.T) {
	ctx := context.Background()
	repo := newMockMovableRepository()
	manager := NewDeltaAwareManager(ctx, repo, DefaultDeltaManagerConfig())
	
	ids := createTestIDsForManager(4)
	originID, deltaID1, deltaID2, parentID := ids[0], ids[1], ids[2], ids[3]
	
	// Setup mock repository with items
	repo.addTestItem(originID, parentID, 1)
	repo.addTestItem(deltaID1, parentID, 0)
	repo.addTestItem(deltaID2, parentID, 2)
	
	// Track deltas
	metadata := DeltaMetadata{}
	err := manager.TrackDelta(deltaID1, originID, DeltaManagerRelationTypeEndpoint, metadata)
	if err != nil {
		t.Fatalf("Failed to track delta1: %v", err)
	}
	
	err = manager.TrackDelta(deltaID2, originID, DeltaManagerRelationTypeExample, metadata)
	if err != nil {
		t.Fatalf("Failed to track delta2: %v", err)
	}
	
	// Sync positions
	result, err := manager.SyncDeltaPositions(ctx, nil, originID, CollectionListTypeItems)
	if err != nil {
		t.Fatalf("Failed to sync delta positions: %v", err)
	}
	
	if result.ProcessedCount != 2 {
		t.Errorf("Expected 2 processed deltas, got %d", result.ProcessedCount)
	}
	
	if result.SuccessCount != 2 {
		t.Errorf("Expected 2 successful syncs, got %d", result.SuccessCount)
	}
	
	if result.FailedCount != 0 {
		t.Errorf("Expected 0 failed syncs, got %d", result.FailedCount)
	}
	
	// Verify positions were updated
	repo.mu.RLock()
	delta1Item := repo.items[deltaID1]
	delta2Item := repo.items[deltaID2]
	repo.mu.RUnlock()
	
	if delta1Item.Position != 1 {
		t.Errorf("Delta1 position should be 1, got %d", delta1Item.Position)
	}
	
	if delta2Item.Position != 1 {
		t.Errorf("Delta2 position should be 1, got %d", delta2Item.Position)
	}
}

func TestSyncDeltaPositionsNoDeltas(t *testing.T) {
	ctx := context.Background()
	repo := newMockMovableRepository()
	manager := NewDeltaAwareManager(ctx, repo, DefaultDeltaManagerConfig())
	
	ids := createTestIDsForManager(1)
	originID := ids[0]
	
	result, err := manager.SyncDeltaPositions(ctx, nil, originID, CollectionListTypeItems)
	if err != nil {
		t.Fatalf("Failed to sync delta positions: %v", err)
	}
	
	if result.ProcessedCount != 0 {
		t.Errorf("Expected 0 processed deltas, got %d", result.ProcessedCount)
	}
}

// =============================================================================
// VALIDATION TESTS
// =============================================================================

func TestValidateDeltaConsistency(t *testing.T) {
	manager := setupTestManager(t)
	ids := createTestIDsForManager(2)
	deltaID, originID := ids[0], ids[1]
	
	metadata := DeltaMetadata{}
	
	// Track valid delta
	err := manager.TrackDelta(deltaID, originID, DeltaManagerRelationTypeEndpoint, metadata)
	if err != nil {
		t.Fatalf("Failed to track delta: %v", err)
	}
	
	// Validate consistency
	check, err := manager.ValidateDeltaConsistency(context.Background())
	if err != nil {
		t.Fatalf("Failed to validate consistency: %v", err)
	}
	
	if !check.IsValid {
		t.Error("Consistency check should be valid")
	}
	
	if len(check.OrphanedDeltas) != 0 {
		t.Error("Should have no orphaned deltas")
	}
	
	if len(check.Issues) != 0 {
		t.Errorf("Should have no issues, got: %v", check.Issues)
	}
}

func TestPruneStaleDelta(t *testing.T) {
	config := DefaultDeltaManagerConfig()
	config.AutoPrune = true
	
	ctx := context.Background()
	repo := newMockMovableRepository()
	manager := NewDeltaAwareManager(ctx, repo, config)
	
	ids := createTestIDsForManager(2)
	deltaID, originID := ids[0], ids[1]
	
	metadata := DeltaMetadata{}
	
	// Track delta
	err := manager.TrackDelta(deltaID, originID, DeltaManagerRelationTypeEndpoint, metadata)
	if err != nil {
		t.Fatalf("Failed to track delta: %v", err)
	}
	
	// Prune delta
	err = manager.PruneStaleDelta(context.Background(), deltaID)
	if err != nil {
		t.Fatalf("Failed to prune delta: %v", err)
	}
	
	if manager.Size() != 0 {
		t.Error("Manager should have zero relationships after pruning")
	}
}

func TestPruneStaleDeltaNoPrune(t *testing.T) {
	config := DefaultDeltaManagerConfig()
	config.AutoPrune = false
	
	ctx := context.Background()
	repo := newMockMovableRepository()
	manager := NewDeltaAwareManager(ctx, repo, config)
	
	ids := createTestIDsForManager(2)
	deltaID, originID := ids[0], ids[1]
	
	metadata := DeltaMetadata{}
	
	// Track delta
	err := manager.TrackDelta(deltaID, originID, DeltaManagerRelationTypeEndpoint, metadata)
	if err != nil {
		t.Fatalf("Failed to track delta: %v", err)
	}
	
	// Prune delta
	err = manager.PruneStaleDelta(context.Background(), deltaID)
	if err != nil {
		t.Fatalf("Failed to prune delta: %v", err)
	}
	
	if manager.Size() != 1 {
		t.Error("Manager should still have the relationship (marked stale)")
	}
	
	// Verify it's marked as stale
	relationship, err := manager.GetDeltaRelationship(deltaID)
	if err != nil {
		t.Fatalf("Failed to get relationship: %v", err)
	}
	
	if !relationship.Metadata.IsStale {
		t.Error("Relationship should be marked as stale")
	}
}

// =============================================================================
// UTILITY TESTS
// =============================================================================

func TestClear(t *testing.T) {
	manager := setupTestManager(t)
	ids := createTestIDsForManager(4)
	
	metadata := DeltaMetadata{}
	
	// Track multiple deltas
	for i := 0; i < 3; i++ {
		err := manager.TrackDelta(ids[i], ids[3], DeltaManagerRelationTypeEndpoint, metadata)
		if err != nil {
			t.Fatalf("Failed to track delta %d: %v", i, err)
		}
	}
	
	if manager.Size() != 3 {
		t.Error("Manager should have 3 relationships before clear")
	}
	
	manager.Clear()
	
	if manager.Size() != 0 {
		t.Error("Manager should have 0 relationships after clear")
	}
}

func TestListOrigins(t *testing.T) {
	manager := setupTestManager(t)
	ids := createTestIDsForManager(5)
	
	origin1, origin2 := ids[0], ids[1]
	delta1, delta2, delta3 := ids[2], ids[3], ids[4]
	
	metadata := DeltaMetadata{}
	
	// Track deltas for two origins
	err := manager.TrackDelta(delta1, origin1, DeltaManagerRelationTypeEndpoint, metadata)
	if err != nil {
		t.Fatalf("Failed to track delta1: %v", err)
	}
	
	err = manager.TrackDelta(delta2, origin1, DeltaManagerRelationTypeExample, metadata)
	if err != nil {
		t.Fatalf("Failed to track delta2: %v", err)
	}
	
	err = manager.TrackDelta(delta3, origin2, DeltaManagerRelationTypeHeader, metadata)
	if err != nil {
		t.Fatalf("Failed to track delta3: %v", err)
	}
	
	origins := manager.ListOrigins()
	if len(origins) != 2 {
		t.Errorf("Expected 2 origins, got %d", len(origins))
	}
	
	// Verify origins are present
	originMap := make(map[idwrap.IDWrap]bool)
	for _, origin := range origins {
		originMap[origin] = true
	}
	
	if !originMap[origin1] || !originMap[origin2] {
		t.Error("Missing expected origins")
	}
}

func TestGetMetrics(t *testing.T) {
	manager := setupTestManager(t)
	ids := createTestIDsForManager(4)
	
	metadata := DeltaMetadata{}
	
	// Track some deltas
	for i := 0; i < 3; i++ {
		err := manager.TrackDelta(ids[i], ids[3], DeltaManagerRelationTypeEndpoint, metadata)
		if err != nil {
			t.Fatalf("Failed to track delta %d: %v", i, err)
		}
	}
	
	metrics := manager.GetMetrics()
	
	if metrics.TotalRelationships != 3 {
		t.Errorf("Expected 3 total relationships, got %d", metrics.TotalRelationships)
	}
	
	if metrics.TotalOrigins != 1 {
		t.Errorf("Expected 1 total origin, got %d", metrics.TotalOrigins)
	}
	
	if metrics.AverageDeltas != 3.0 {
		t.Errorf("Expected 3.0 average deltas, got %f", metrics.AverageDeltas)
	}
	
	if metrics.MemoryUsage <= 0 {
		t.Error("Memory usage should be positive")
	}
}

// =============================================================================
// CONCURRENCY TESTS
// =============================================================================

func TestConcurrentTrackUntrack(t *testing.T) {
	manager := setupTestManager(t)
	
	const numGoroutines = 10
	const operationsPerGoroutine = 100
	
	var wg sync.WaitGroup
	
	// Start tracking goroutines
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(startOffset int) {
			defer wg.Done()
			
			metadata := DeltaMetadata{}
			
			for j := 0; j < operationsPerGoroutine; j++ {
				deltaID := idwrap.NewNow()
				originID := idwrap.NewNow()
				
				// Track delta
				err := manager.TrackDelta(deltaID, originID, DeltaManagerRelationTypeEndpoint, metadata)
				if err != nil {
					t.Errorf("Failed to track delta: %v", err)
					return
				}
				
				// Immediately untrack
				err = manager.UntrackDelta(deltaID)
				if err != nil {
					t.Errorf("Failed to untrack delta: %v", err)
					return
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	if manager.Size() != 0 {
		t.Errorf("Expected 0 relationships after concurrent operations, got %d", manager.Size())
	}
}

func TestConcurrentReadWrite(t *testing.T) {
	manager := setupTestManager(t)
	ids := createTestIDsForManager(100)
	
	metadata := DeltaMetadata{}
	
	// Pre-populate some deltas
	for i := 0; i < 50; i++ {
		err := manager.TrackDelta(ids[i], ids[99], DeltaManagerRelationTypeEndpoint, metadata)
		if err != nil {
			t.Fatalf("Failed to track initial delta %d: %v", i, err)
		}
	}
	
	const numReaders = 5
	const numWriters = 2
	const operations = 100
	
	var wg sync.WaitGroup
	
	// Start reader goroutines
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			for j := 0; j < operations; j++ {
				// Read operations
				_ = manager.Size()
				_ = manager.HasDelta(ids[j%50])
				_, _ = manager.GetDeltasForOrigin(ids[99])
				_ = manager.ListOrigins()
			}
		}()
	}
	
	// Start writer goroutines
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()
			
			for j := 0; j < operations; j++ {
				deltaID := idwrap.NewNow()
				originID := ids[99]
				
				// Write operations
				err := manager.TrackDelta(deltaID, originID, DeltaManagerRelationTypeExample, metadata)
				if err != nil {
					continue // Ignore conflicts in concurrent test
				}
				
				// Occasionally untrack
				if j%10 == 0 {
					_ = manager.UntrackDelta(deltaID)
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	// Verify manager is still functional
	if manager.Size() < 0 {
		t.Error("Manager size should not be negative")
	}
	
	// Should still be able to perform operations
	testDelta := idwrap.NewNow()
	testOrigin := idwrap.NewNow()
	
	err := manager.TrackDelta(testDelta, testOrigin, DeltaManagerRelationTypeHeader, metadata)
	if err != nil {
		t.Errorf("Manager should still be functional after concurrent operations: %v", err)
	}
}

// =============================================================================
// BENCHMARK TESTS
// =============================================================================

func BenchmarkDeltaTrackUntrack(b *testing.B) {
	manager := setupBenchmarkManager(b)
	metadata := DeltaMetadata{}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		deltaID := idwrap.NewNow()
		originID := idwrap.NewNow()
		
		manager.TrackDelta(deltaID, originID, DeltaManagerRelationTypeEndpoint, metadata)
		manager.UntrackDelta(deltaID)
	}
}

func BenchmarkDeltaLookup(b *testing.B) {
	manager := setupBenchmarkManager(b)
	metadata := DeltaMetadata{}
	
	// Pre-populate with test data
	const prePopulate = 1000
	ids := make([]idwrap.IDWrap, prePopulate)
	
	for i := 0; i < prePopulate; i++ {
		deltaID := idwrap.NewNow()
		originID := idwrap.NewNow()
		ids[i] = deltaID
		
		manager.TrackDelta(deltaID, originID, DeltaManagerRelationTypeEndpoint, metadata)
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		deltaID := ids[i%prePopulate]
		_, _ = manager.GetDeltaRelationship(deltaID)
	}
}

func BenchmarkDeltaGetDeltasForOrigin(b *testing.B) {
	manager := setupBenchmarkManager(b)
	metadata := DeltaMetadata{}
	
	// Pre-populate with test data - many deltas per origin
	const numOrigins = 100
	const deltasPerOrigin = 10
	
	origins := make([]idwrap.IDWrap, numOrigins)
	for i := 0; i < numOrigins; i++ {
		originID := idwrap.NewNow()
		origins[i] = originID
		
		for j := 0; j < deltasPerOrigin; j++ {
			deltaID := idwrap.NewNow()
			manager.TrackDelta(deltaID, originID, DeltaManagerRelationTypeEndpoint, metadata)
		}
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		originID := origins[i%numOrigins]
		_, _ = manager.GetDeltasForOrigin(originID)
	}
}

func BenchmarkDeltaHasDelta(b *testing.B) {
	manager := setupBenchmarkManager(b)
	metadata := DeltaMetadata{}
	
	// Pre-populate with test data
	const prePopulate = 1000
	ids := make([]idwrap.IDWrap, prePopulate)
	
	for i := 0; i < prePopulate; i++ {
		deltaID := idwrap.NewNow()
		originID := idwrap.NewNow()
		ids[i] = deltaID
		
		manager.TrackDelta(deltaID, originID, DeltaManagerRelationTypeEndpoint, metadata)
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		deltaID := ids[i%prePopulate]
		_ = manager.HasDelta(deltaID)
	}
}

func BenchmarkDeltaConcurrentAccess(b *testing.B) {
	manager := setupBenchmarkManager(b)
	metadata := DeltaMetadata{}
	
	// Pre-populate with test data
	const prePopulate = 100
	ids := make([]idwrap.IDWrap, prePopulate)
	origins := make([]idwrap.IDWrap, 10)
	
	// Create origins first
	for i := 0; i < 10; i++ {
		origins[i] = idwrap.NewNow()
	}
	
	for i := 0; i < prePopulate; i++ {
		deltaID := idwrap.NewNow()
		originID := origins[i%10]
		ids[i] = deltaID
		
		manager.TrackDelta(deltaID, originID, DeltaManagerRelationTypeEndpoint, metadata)
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			deltaID := ids[i%prePopulate]
			
			// Mix of read operations
			switch i % 4 {
			case 0:
				_ = manager.HasDelta(deltaID)
			case 1:
				_, _ = manager.GetDeltaRelationship(deltaID)
			case 2:
				originID := origins[i%10]
				_, _ = manager.GetDeltasForOrigin(originID)
			case 3:
				_ = manager.Size()
			}
			
			i++
		}
	})
}

// =============================================================================
// BENCHMARK SETUP HELPERS
// =============================================================================

func setupTestManager(t *testing.T) *DeltaAwareManager {
	ctx := context.Background()
	repo := newMockMovableRepository()
	config := DefaultDeltaManagerConfig()
	
	return NewDeltaAwareManager(ctx, repo, config)
}

func setupBenchmarkManager(b *testing.B) *DeltaAwareManager {
	ctx := context.Background()
	repo := newMockMovableRepository()
	config := DefaultDeltaManagerConfig()
	
	return NewDeltaAwareManager(ctx, repo, config)
}