package movable

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"
	"the-dev-tools/server/pkg/idwrap"

	_ "github.com/mattn/go-sqlite3"
)

// =============================================================================
// TEST SETUP AND HELPERS
// =============================================================================

// MockDeltaCoordinator implements DeltaCoordinator for testing
type MockDeltaCoordinator struct {
	coordinateDeltaCalled bool
	syncDeltasCalled      bool
	resolveConflictsCalled bool
	
	coordinateError   error
	syncError        error
	resolveError     error
}

func (m *MockDeltaCoordinator) CoordinateDelta(ctx context.Context, tx *sql.Tx, operation *CrossContextDeltaOperation) error {
	m.coordinateDeltaCalled = true
	return m.coordinateError
}

func (m *MockDeltaCoordinator) SyncDeltas(ctx context.Context, scopeID idwrap.IDWrap, contextType MovableContext) error {
	m.syncDeltasCalled = true
	return m.syncError
}

func (m *MockDeltaCoordinator) ResolveConflicts(ctx context.Context, conflicts []DeltaConflict) (*UnifiedConflictResolution, error) {
	m.resolveConflictsCalled = true
	return &UnifiedConflictResolution{}, m.resolveError
}

// createTestSyncManager creates a sync manager for testing
func createTestSyncManager(t *testing.T) (*DeltaSyncManager, *MockDeltaCoordinator, *RepositoryRegistry) {
	// Create in-memory database
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Create registry
	config := DefaultUnifiedServiceConfig()
	registry, err := NewRepositoryRegistry(db, config)
	if err != nil {
		t.Fatalf("Failed to create repository registry: %v", err)
	}

	// Create mock coordinator
	coordinator := &MockDeltaCoordinator{}

	// Create sync manager with test config
	syncConfig := DefaultSyncManagerConfig()
	syncConfig.EnableAsync = false // Disable async for predictable testing
	
	manager := NewDeltaSyncManager(coordinator, registry, syncConfig)
	
	return manager, coordinator, registry
}

// createTestSyncIDs creates test ID wraps for sync manager tests
func createTestSyncIDs(prefix string, count int) []idwrap.IDWrap {
	ids := make([]idwrap.IDWrap, count)
	for i := 0; i < count; i++ {
		ids[i] = idwrap.NewTextMust(fmt.Sprintf("%s_%d", prefix, i))
	}
	return ids
}

// =============================================================================
// CORE FUNCTIONALITY TESTS
// =============================================================================

func TestDeltaSyncManager_OnDeltaCreate(t *testing.T) {
	manager, _, _ := createTestSyncManager(t)
	
	// Test data
	deltaID := idwrap.NewTextMust("delta_1")
	originID := idwrap.NewTextMust("origin_1") 
	scopeID := idwrap.NewTextMust("scope_1")
	contextType := ContextEndpoint
	
	relation := &DeltaRelation{
		DeltaID:  deltaID,
		OriginID: originID,
		Context:  contextType,
		ScopeID:  scopeID,
	}
	
	// Execute
	err := manager.OnDeltaCreate(context.Background(), deltaID, originID, contextType, scopeID, relation)
	
	// Verify
	if err != nil {
		t.Errorf("OnDeltaCreate failed: %v", err)
	}
	
	// Check if sync relationship was created
	manager.mu.RLock()
	syncRel, exists := manager.syncRelationships[deltaID]
	manager.mu.RUnlock()
	
	if !exists {
		t.Error("Sync relationship not created")
	}
	
	if syncRel.DeltaID != deltaID {
		t.Errorf("Expected DeltaID %v, got %v", deltaID, syncRel.DeltaID)
	}
	
	if syncRel.OriginID != originID {
		t.Errorf("Expected OriginID %v, got %v", originID, syncRel.OriginID)
	}
	
	if syncRel.Context != contextType {
		t.Errorf("Expected Context %v, got %v", contextType, syncRel.Context)
	}
}

func TestDeltaSyncManager_OnDeltaDelete(t *testing.T) {
	manager, _, _ := createTestSyncManager(t)
	
	// Setup: Create a sync relationship first
	deltaID := idwrap.NewTextMust("delta_1")
	originID := idwrap.NewTextMust("origin_1")
	scopeID := idwrap.NewTextMust("scope_1") 
	contextType := ContextEndpoint
	
	relation := &DeltaRelation{
		DeltaID:  deltaID,
		OriginID: originID,
		Context:  contextType,
		ScopeID:  scopeID,
	}
	
	err := manager.OnDeltaCreate(context.Background(), deltaID, originID, contextType, scopeID, relation)
	if err != nil {
		t.Fatalf("Failed to create delta for test: %v", err)
	}
	
	// Verify relationship exists
	manager.mu.RLock()
	_, exists := manager.syncRelationships[deltaID]
	manager.mu.RUnlock()
	
	if !exists {
		t.Fatal("Sync relationship should exist before delete")
	}
	
	// Execute delete
	err = manager.OnDeltaDelete(context.Background(), deltaID)
	if err != nil {
		t.Errorf("OnDeltaDelete failed: %v", err)
	}
	
	// Verify relationship was removed
	manager.mu.RLock()
	_, exists = manager.syncRelationships[deltaID]
	manager.mu.RUnlock()
	
	if exists {
		t.Error("Sync relationship should have been deleted")
	}
}

func TestDeltaSyncManager_OnOriginMove(t *testing.T) {
	manager, _, _ := createTestSyncManager(t)
	
	// Setup: Create deltas for an origin
	originID := idwrap.NewTextMust("origin_1")
	deltaID1 := idwrap.NewTextMust("delta_1")
	deltaID2 := idwrap.NewTextMust("delta_2")
	scopeID := idwrap.NewTextMust("scope_1")
	contextType := ContextEndpoint
	
	// Create sync relationships
	relations := []*DeltaRelation{
		{DeltaID: deltaID1, OriginID: originID, Context: contextType, ScopeID: scopeID},
		{DeltaID: deltaID2, OriginID: originID, Context: contextType, ScopeID: scopeID},
	}
	
	for i, relation := range relations {
		deltaID := []idwrap.IDWrap{deltaID1, deltaID2}[i]
		err := manager.OnDeltaCreate(context.Background(), deltaID, originID, contextType, scopeID, relation)
		if err != nil {
			t.Fatalf("Failed to create delta %d: %v", i, err)
		}
	}
	
	// Execute origin move
	oldPos, newPos := 5, 10
	listType := CollectionListTypeEndpoints
	
	err := manager.OnOriginMove(context.Background(), originID, listType, oldPos, newPos)
	if err != nil {
		t.Errorf("OnOriginMove failed: %v", err)
	}
	
	// Verify sync relationships were updated (checking metadata)
	manager.mu.RLock()
	rel1 := manager.syncRelationships[deltaID1]
	rel2 := manager.syncRelationships[deltaID2]
	manager.mu.RUnlock()
	
	if rel1.LastSync == nil {
		t.Error("Delta 1 sync metadata not updated")
	}
	
	if rel2.LastSync == nil {
		t.Error("Delta 2 sync metadata not updated")
	}
	
	if rel1.SyncVersion == 0 || rel2.SyncVersion == 0 {
		t.Error("Sync versions should have been incremented")
	}
}

// =============================================================================
// CONFLICT RESOLUTION TESTS
// =============================================================================

func TestDeltaSyncManager_ResolveConflicts_LastWriteWins(t *testing.T) {
	manager, _, _ := createTestSyncManager(t)
	
	// Create test conflicts
	now := time.Now()
	older := now.Add(-1 * time.Hour)
	
	conflicts := []SyncConflict{
		{
			ConflictID: "conflict_1",
			OriginID:   idwrap.NewTextMust("origin_1"),
			ConflictingDeltas: []ConflictingDelta{
				{
					DeltaID:      idwrap.NewTextMust("delta_old"),
					LastModified: older,
					Position:     5,
				},
				{
					DeltaID:      idwrap.NewTextMust("delta_new"), 
					LastModified: now,
					Position:     7,
				},
			},
			ConflictType: ConflictTypePositionMismatch,
			Context:      ContextEndpoint,
		},
	}
	
	// Execute resolution
	result, err := manager.ResolveConflicts(context.Background(), conflicts)
	if err != nil {
		t.Errorf("ResolveConflicts failed: %v", err)
	}
	
	// Verify results
	if result.TotalConflicts != 1 {
		t.Errorf("Expected 1 total conflict, got %d", result.TotalConflicts)
	}
	
	if result.ResolvedCount != 1 {
		t.Errorf("Expected 1 resolved conflict, got %d", result.ResolvedCount)
	}
	
	if result.FailedCount != 0 {
		t.Errorf("Expected 0 failed conflicts, got %d", result.FailedCount)
	}
	
	// Check resolution details
	if len(result.ResolvedConflicts) != 1 {
		t.Fatal("Expected 1 resolved conflict in results")
	}
	
	resolved := result.ResolvedConflicts[0]
	if resolved.Strategy != ConflictStrategyLastWriteWins {
		t.Errorf("Expected LastWriteWins strategy, got %v", resolved.Strategy)
	}
	
	expectedWinner := idwrap.NewTextMust("delta_new")
	if resolved.Resolution.WinnerID != expectedWinner {
		t.Errorf("Expected winner %v, got %v", expectedWinner, resolved.Resolution.WinnerID)
	}
}

func TestDeltaSyncManager_ResolveConflicts_OriginPriority(t *testing.T) {
	manager, _, _ := createTestSyncManager(t)
	
	// Set strategy to origin priority
	manager.config.DefaultStrategy = ConflictStrategyOriginPriority
	
	originID := idwrap.NewTextMust("origin_1")
	conflicts := []SyncConflict{
		{
			ConflictID: "conflict_1",
			OriginID:   originID,
			ConflictingDeltas: []ConflictingDelta{
				{DeltaID: idwrap.NewTextMust("delta_1"), Position: 5},
				{DeltaID: idwrap.NewTextMust("delta_2"), Position: 7},
			},
			ConflictType: ConflictTypePositionMismatch,
			Context:      ContextEndpoint,
		},
	}
	
	result, err := manager.ResolveConflicts(context.Background(), conflicts)
	if err != nil {
		t.Errorf("ResolveConflicts failed: %v", err)
	}
	
	if result.ResolvedCount != 1 {
		t.Errorf("Expected 1 resolved conflict, got %d", result.ResolvedCount)
	}
	
	resolved := result.ResolvedConflicts[0]
	if resolved.Strategy != ConflictStrategyOriginPriority {
		t.Errorf("Expected OriginPriority strategy, got %v", resolved.Strategy)
	}
	
	if resolved.Resolution.WinnerID != originID {
		t.Errorf("Expected origin as winner, got %v", resolved.Resolution.WinnerID)
	}
}

func TestDeltaSyncManager_ResolveConflicts_MergeStrategy(t *testing.T) {
	manager, _, _ := createTestSyncManager(t)
	
	// Set strategy to merge
	manager.config.DefaultStrategy = ConflictStrategyMergeStrategy
	manager.config.EnableMerging = true
	
	conflicts := []SyncConflict{
		{
			ConflictID: "conflict_1",
			OriginID:   idwrap.NewTextMust("origin_1"),
			ConflictingDeltas: []ConflictingDelta{
				{DeltaID: idwrap.NewTextMust("delta_1"), Position: 4},
				{DeltaID: idwrap.NewTextMust("delta_2"), Position: 6},
				{DeltaID: idwrap.NewTextMust("delta_3"), Position: 8},
			},
			ConflictType: ConflictTypePositionMismatch,
			Context:      ContextEndpoint,
		},
	}
	
	result, err := manager.ResolveConflicts(context.Background(), conflicts)
	if err != nil {
		t.Errorf("ResolveConflicts failed: %v", err)
	}
	
	if result.ResolvedCount != 1 {
		t.Errorf("Expected 1 resolved conflict, got %d", result.ResolvedCount)
	}
	
	resolved := result.ResolvedConflicts[0]
	if resolved.Strategy != ConflictStrategyMergeStrategy {
		t.Errorf("Expected MergeStrategy, got %v", resolved.Strategy)
	}
	
	// Check that merged position is average of 4, 6, 8 = 6
	expectedPos := 6
	if resolved.Resolution.MergedPosition == nil {
		t.Error("Expected merged position to be set")
	} else if *resolved.Resolution.MergedPosition != expectedPos {
		t.Errorf("Expected merged position %d, got %d", expectedPos, *resolved.Resolution.MergedPosition)
	}
}

// =============================================================================
// PROPAGATION TESTS
// =============================================================================

func TestDeltaSyncManager_PropagateChanges(t *testing.T) {
	manager, _, _ := createTestSyncManager(t)
	
	// Setup sync relationships
	originID := idwrap.NewTextMust("origin_1")
	deltaID1 := idwrap.NewTextMust("delta_1")
	deltaID2 := idwrap.NewTextMust("delta_2")
	scopeID := idwrap.NewTextMust("scope_1")
	contextType := ContextEndpoint
	
	relations := []*DeltaRelation{
		{DeltaID: deltaID1, OriginID: originID, Context: contextType, ScopeID: scopeID},
		{DeltaID: deltaID2, OriginID: originID, Context: contextType, ScopeID: scopeID},
	}
	
	for i, relation := range relations {
		deltaID := []idwrap.IDWrap{deltaID1, deltaID2}[i]
		err := manager.OnDeltaCreate(context.Background(), deltaID, originID, contextType, scopeID, relation)
		if err != nil {
			t.Fatalf("Failed to create delta %d: %v", i, err)
		}
	}
	
	// Create changes
	changes := &ChangeSet{
		ListType: CollectionListTypeEndpoints,
		PositionChanges: map[idwrap.IDWrap]int{
			originID: 15,
		},
	}
	
	// Execute propagation
	err := manager.PropagateChanges(context.Background(), nil, originID, changes)
	if err != nil {
		t.Errorf("PropagateChanges failed: %v", err)
	}
	
	// Verify sync metadata was updated
	manager.mu.RLock()
	rel1 := manager.syncRelationships[deltaID1]
	rel2 := manager.syncRelationships[deltaID2]
	manager.mu.RUnlock()
	
	if rel1.IsDirty {
		t.Error("Delta 1 should not be dirty after successful propagation")
	}
	
	if rel2.IsDirty {
		t.Error("Delta 2 should not be dirty after successful propagation")
	}
	
	if rel1.SyncVersion == 0 || rel2.SyncVersion == 0 {
		t.Error("Sync versions should have been incremented")
	}
}

// =============================================================================
// CONFIGURATION TESTS
// =============================================================================

func TestSyncManagerConfig_Defaults(t *testing.T) {
	config := DefaultSyncManagerConfig()
	
	// Verify key defaults
	if config.EventQueueSize <= 0 {
		t.Error("EventQueueSize should be positive")
	}
	
	if config.WorkerCount <= 0 {
		t.Error("WorkerCount should be positive")
	}
	
	if config.BatchSize <= 0 {
		t.Error("BatchSize should be positive")
	}
	
	if config.SyncInterval <= 0 {
		t.Error("SyncInterval should be positive")
	}
	
	if config.DefaultStrategy < 0 || config.DefaultStrategy > ConflictStrategyUserChoice {
		t.Error("DefaultStrategy should be valid")
	}
	
	if !config.EnableBatching {
		t.Error("EnableBatching should be true by default")
	}
}

func TestSyncManagerConfig_CustomConfig(t *testing.T) {
	config := &SyncManagerConfig{
		EventQueueSize:    500,
		WorkerCount:       2,
		BatchSize:         25,
		DefaultStrategy:   ConflictStrategyDeltaPriority,
		EnableBatching:    false,
		EnableAsync:       false,
		MaxRetries:        1,
	}
	
	manager := NewDeltaSyncManager(nil, nil, config)
	
	if manager.config.EventQueueSize != 500 {
		t.Errorf("Expected EventQueueSize 500, got %d", manager.config.EventQueueSize)
	}
	
	if manager.config.WorkerCount != 2 {
		t.Errorf("Expected WorkerCount 2, got %d", manager.config.WorkerCount)
	}
	
	if manager.config.DefaultStrategy != ConflictStrategyDeltaPriority {
		t.Errorf("Expected DeltaPriority strategy, got %v", manager.config.DefaultStrategy)
	}
}

// =============================================================================
// PERFORMANCE AND METRICS TESTS
// =============================================================================

func TestSyncMetrics_TrackOperation(t *testing.T) {
	metrics := NewSyncMetrics()
	
	// Track successful operation
	duration := 100 * time.Millisecond
	metrics.TrackOperation("test_op", duration, nil)
	
	stats := metrics.GetStats()
	
	if stats.TotalOperations != 1 {
		t.Errorf("Expected 1 total operation, got %d", stats.TotalOperations)
	}
	
	if stats.SuccessfulSyncs != 1 {
		t.Errorf("Expected 1 successful sync, got %d", stats.SuccessfulSyncs)
	}
	
	if stats.FailedSyncs != 0 {
		t.Errorf("Expected 0 failed syncs, got %d", stats.FailedSyncs)
	}
	
	if stats.AverageSyncTime != duration {
		t.Errorf("Expected average time %v, got %v", duration, stats.AverageSyncTime)
	}
}

func TestSyncMetrics_TrackFailure(t *testing.T) {
	metrics := NewSyncMetrics()
	
	// Track failed operation
	duration := 200 * time.Millisecond
	err := fmt.Errorf("test error")
	metrics.TrackOperation("test_op", duration, err)
	
	stats := metrics.GetStats()
	
	if stats.TotalOperations != 1 {
		t.Errorf("Expected 1 total operation, got %d", stats.TotalOperations)
	}
	
	if stats.SuccessfulSyncs != 0 {
		t.Errorf("Expected 0 successful syncs, got %d", stats.SuccessfulSyncs)
	}
	
	if stats.FailedSyncs != 1 {
		t.Errorf("Expected 1 failed sync, got %d", stats.FailedSyncs)
	}
}

// =============================================================================
// EVENT QUEUE TESTS
// =============================================================================

func TestSyncEventQueue_EnqueueDequeue(t *testing.T) {
	queue := NewSyncEventQueue(10)
	defer queue.Close()
	
	// Create test event
	event := &SyncEvent{
		Type:      SyncEventOriginMove,
		ID:        "test_event_1",
		Timestamp: time.Now(),
		Priority:  5,
	}
	
	// Enqueue
	err := queue.Enqueue(event)
	if err != nil {
		t.Errorf("Enqueue failed: %v", err)
	}
	
	// Dequeue
	dequeuedEvent, ok := queue.Dequeue()
	if !ok {
		t.Error("Dequeue should have returned an event")
	}
	
	if dequeuedEvent.ID != event.ID {
		t.Errorf("Expected event ID %s, got %s", event.ID, dequeuedEvent.ID)
	}
}

func TestSyncEventQueue_FullQueue(t *testing.T) {
	queue := NewSyncEventQueue(2) // Small queue
	defer queue.Close()
	
	// Fill queue
	for i := 0; i < 2; i++ {
		event := &SyncEvent{
			Type:      SyncEventOriginMove,
			ID:        fmt.Sprintf("event_%d", i),
			Timestamp: time.Now(),
		}
		
		err := queue.Enqueue(event)
		if err != nil {
			t.Errorf("Enqueue %d failed: %v", i, err)
		}
	}
	
	// Try to add one more (should fail)
	overflowEvent := &SyncEvent{
		Type: SyncEventOriginMove,
		ID:   "overflow_event",
	}
	
	err := queue.Enqueue(overflowEvent)
	if err == nil {
		t.Error("Expected enqueue to fail on full queue")
	}
}

// =============================================================================
// EDGE CASES AND ERROR HANDLING TESTS
// =============================================================================

func TestDeltaSyncManager_OnDeltaCreate_EmptyIDs(t *testing.T) {
	manager, _, _ := createTestSyncManager(t)
	
	// Test with empty delta ID
	err := manager.OnDeltaCreate(context.Background(), idwrap.IDWrap{}, 
		idwrap.NewTextMust("origin_1"), ContextEndpoint, 
		idwrap.NewTextMust("scope_1"), &DeltaRelation{})
	
	if err == nil {
		t.Error("Expected error for empty delta ID")
	}
	
	// Test with empty origin ID
	err = manager.OnDeltaCreate(context.Background(), idwrap.NewTextMust("delta_1"),
		idwrap.IDWrap{}, ContextEndpoint,
		idwrap.NewTextMust("scope_1"), &DeltaRelation{})
	
	if err == nil {
		t.Error("Expected error for empty origin ID")
	}
}

func TestDeltaSyncManager_OnDeltaDelete_NonExistent(t *testing.T) {
	manager, _, _ := createTestSyncManager(t)
	
	// Try to delete non-existent delta (should not error)
	err := manager.OnDeltaDelete(context.Background(), idwrap.NewTextMust("nonexistent"))
	if err != nil {
		t.Errorf("OnDeltaDelete should handle non-existent deltas gracefully: %v", err)
	}
}

func TestDeltaSyncManager_ResolveConflicts_EmptyConflicts(t *testing.T) {
	manager, _, _ := createTestSyncManager(t)
	
	result, err := manager.ResolveConflicts(context.Background(), []SyncConflict{})
	if err != nil {
		t.Errorf("ResolveConflicts should handle empty conflicts: %v", err)
	}
	
	if result.TotalConflicts != 0 {
		t.Errorf("Expected 0 total conflicts, got %d", result.TotalConflicts)
	}
}

// =============================================================================
// CONCURRENCY TESTS
// =============================================================================

func TestDeltaSyncManager_ConcurrentAccess(t *testing.T) {
	manager, _, _ := createTestSyncManager(t)
	
	// Create multiple goroutines that operate on the manager
	const numWorkers = 10
	const operationsPerWorker = 50
	
	done := make(chan bool, numWorkers)
	
	for i := 0; i < numWorkers; i++ {
		go func(workerID int) {
			defer func() { done <- true }()
			
			for j := 0; j < operationsPerWorker; j++ {
				deltaID := idwrap.NewTextMust(fmt.Sprintf("delta_%d_%d", workerID, j))
				originID := idwrap.NewTextMust(fmt.Sprintf("origin_%d", workerID))
				scopeID := idwrap.NewTextMust(fmt.Sprintf("scope_%d", workerID))
				
				relation := &DeltaRelation{
					DeltaID:  deltaID,
					OriginID: originID,
					Context:  ContextEndpoint,
					ScopeID:  scopeID,
				}
				
				// Create delta
				err := manager.OnDeltaCreate(context.Background(), deltaID, originID, 
					ContextEndpoint, scopeID, relation)
				if err != nil {
					t.Errorf("Worker %d: OnDeltaCreate failed: %v", workerID, err)
				}
				
				// Delete delta
				err = manager.OnDeltaDelete(context.Background(), deltaID)
				if err != nil {
					t.Errorf("Worker %d: OnDeltaDelete failed: %v", workerID, err)
				}
			}
		}(i)
	}
	
	// Wait for all workers to complete
	for i := 0; i < numWorkers; i++ {
		<-done
	}
	
	// Verify final state is clean
	manager.mu.RLock()
	remainingRelationships := len(manager.syncRelationships)
	manager.mu.RUnlock()
	
	if remainingRelationships != 0 {
		t.Errorf("Expected 0 remaining relationships, got %d", remainingRelationships)
	}
}

// =============================================================================
// BENCHMARK TESTS
// =============================================================================

func BenchmarkDeltaSyncManager_OnDeltaCreate(b *testing.B) {
	manager, _, _ := createTestSyncManager(&testing.T{})
	
	// Pre-generate IDs to avoid allocation overhead in benchmark
	deltaIDs := createTestSyncIDs("delta", b.N)
	originIDs := createTestSyncIDs("origin", b.N)
	scopeIDs := createTestSyncIDs("scope", b.N)
	
	relations := make([]*DeltaRelation, b.N)
	for i := 0; i < b.N; i++ {
		relations[i] = &DeltaRelation{
			DeltaID:  deltaIDs[i],
			OriginID: originIDs[i],
			Context:  ContextEndpoint,
			ScopeID:  scopeIDs[i],
		}
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		manager.OnDeltaCreate(context.Background(), deltaIDs[i], originIDs[i],
			ContextEndpoint, scopeIDs[i], relations[i])
	}
}

func BenchmarkDeltaSyncManager_PropagateChanges(b *testing.B) {
	manager, _, _ := createTestSyncManager(&testing.T{})
	
	// Setup: Create many sync relationships
	const numDeltas = 100
	originID := idwrap.NewTextMust("benchmark_origin")
	
	for i := 0; i < numDeltas; i++ {
		deltaID := idwrap.NewTextMust(fmt.Sprintf("delta_%d", i))
		scopeID := idwrap.NewTextMust(fmt.Sprintf("scope_%d", i))
		
		relation := &DeltaRelation{
			DeltaID:  deltaID,
			OriginID: originID,
			Context:  ContextEndpoint,
			ScopeID:  scopeID,
		}
		
		manager.OnDeltaCreate(context.Background(), deltaID, originID,
			ContextEndpoint, scopeID, relation)
	}
	
	changes := &ChangeSet{
		ListType: CollectionListTypeEndpoints,
		PositionChanges: map[idwrap.IDWrap]int{
			originID: 42,
		},
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		manager.PropagateChanges(context.Background(), nil, originID, changes)
	}
}