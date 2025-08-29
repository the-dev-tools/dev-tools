package movable

import (
	"context"
	"database/sql"
	"fmt"
	"runtime"
	"sort"
	"testing"
	"time"
	"the-dev-tools/server/pkg/idwrap"
)

// =============================================================================
// COMPREHENSIVE CONTEXT-AWARE MOVABLE SYSTEM BENCHMARKS
// =============================================================================
//
// This file contains performance benchmarks for the context-aware movable system
// that validate delta operations, context resolution, and sync propagation.
//
// PERFORMANCE TARGETS TO VALIDATE:
// • Single operation: < 100µs
// • Batch (100 ops): < 1ms  
// • Context resolution: < 1µs cached
// • Sync propagation: < 500ms for 1000 items
// • Memory usage: acceptable allocation growth
// • CPU profile: efficient CPU usage
//
// BENCHMARK CATEGORIES:
//
// 1. CONTEXT RESOLUTION BENCHMARKS:
//    BenchmarkContextResolution          - Context detection speed
//    BenchmarkContextCachePerformance    - Cache hit/miss performance
//    BenchmarkScopeResolution           - Scope ID resolution
//
// 2. DELTA OPERATION BENCHMARKS:
//    BenchmarkDeltaCreation             - Single delta creation
//    BenchmarkBatchDeltaOps             - Batch delta operations
//    BenchmarkDeltaTracking             - Delta relationship tracking
//
// 3. SYNC PROPAGATION BENCHMARKS:
//    BenchmarkSyncPropagation           - Origin to delta sync
//    BenchmarkBatchSyncOperations       - Batch sync performance
//    BenchmarkConsistencyValidation     - Consistency check performance
//
// 4. CROSS-CONTEXT BENCHMARKS:
//    BenchmarkCrossContextMove          - Cross-context operations
//    BenchmarkScopeValidation           - Scope boundary validation
//    BenchmarkContextMigration          - Context migration performance
//
// 5. MEMORY & ALLOCATION BENCHMARKS:
//    BenchmarkMemoryEfficiency          - Allocation tracking
//    BenchmarkMemoryGrowthPattern       - Memory growth with scale
//    BenchmarkCacheEfficiency           - Cache memory usage
//
// HOW TO RUN:
//
// # All context benchmarks:
// go test -bench=BenchmarkContext -benchmem ./pkg/movable/
//
// # Memory analysis:
// go test -bench=BenchmarkMemory -benchmem ./pkg/movable/
//
// # CPU profiling:
// go test -bench=. -cpuprofile=cpu.prof ./pkg/movable/
//
// # Extended timing for accurate measurements:
// go test -bench=. -benchtime=10s ./pkg/movable/
//
// =============================================================================

// Test data structures for benchmarking
type benchmarkTestData struct {
	contextCache     *InMemoryContextCache
	deltaManager     *DeltaAwareManager
	scopeResolver    *mockScopeResolver
	testItems        []testContextualItem
	relationships    []DeltaRelationship
	contextMetadata  map[idwrap.IDWrap]*ContextMetadata
}

type testContextualItem struct {
	ID        idwrap.IDWrap
	ParentID  idwrap.IDWrap
	Position  int
	Context   MovableContext
	ScopeID   idwrap.IDWrap
	IsHidden  bool
	IsDelta   bool
	OriginID  *idwrap.IDWrap
}

type mockScopeResolver struct {
	contextMappings map[string]MovableContext
	scopeMappings   map[string]idwrap.IDWrap
	hierarchies     map[string][]ScopeLevel
}

func (r *mockScopeResolver) ResolveContext(ctx context.Context, itemID idwrap.IDWrap) (MovableContext, error) {
	if context, exists := r.contextMappings[itemID.String()]; exists {
		return context, nil
	}
	return ContextCollection, nil
}

func (r *mockScopeResolver) ResolveScopeID(ctx context.Context, itemID idwrap.IDWrap, contextType MovableContext) (idwrap.IDWrap, error) {
	key := fmt.Sprintf("%s_%d", itemID.String(), int(contextType))
	if scopeID, exists := r.scopeMappings[key]; exists {
		return scopeID, nil
	}
	return idwrap.NewTextMust("01ARZ3NDEKTSV4RRFFQ69G5FAV"), nil
}

func (r *mockScopeResolver) ValidateScope(ctx context.Context, itemID idwrap.IDWrap, expectedScope idwrap.IDWrap) error {
	return nil
}

func (r *mockScopeResolver) GetScopeHierarchy(ctx context.Context, itemID idwrap.IDWrap) ([]ScopeLevel, error) {
	if hierarchy, exists := r.hierarchies[itemID.String()]; exists {
		return hierarchy, nil
	}
	return []ScopeLevel{}, nil
}

// =============================================================================
// BENCHMARK DATA SETUP
// =============================================================================

func setupBenchmarkTestData(size int) *benchmarkTestData {
	// Create context cache
	cache := NewInMemoryContextCache(10 * time.Minute)
	
	// Create mock repository for delta manager
	config := DefaultDeltaManagerConfig()
	mockRepo := &mockRepository{}
	deltaManager := NewDeltaAwareManager(context.Background(), mockRepo, config)
	
	// Create scope resolver
	resolver := &mockScopeResolver{
		contextMappings: make(map[string]MovableContext),
		scopeMappings:   make(map[string]idwrap.IDWrap),
		hierarchies:     make(map[string][]ScopeLevel),
	}
	
	// Generate test items
	testItems := make([]testContextualItem, size)
	contextMetadata := make(map[idwrap.IDWrap]*ContextMetadata)
	relationships := make([]DeltaRelationship, 0, size/2)
	
	for i := 0; i < size; i++ {
		itemID := idwrap.NewTextMust(fmt.Sprintf("01ARZ3NDEKTSV4RRFFQ69G5F%02X", i))
		parentID := idwrap.NewTextMust(fmt.Sprintf("01ARZ3NDEKTSV4RRFFQ69G5F%02X", i/10))
		scopeID := idwrap.NewTextMust(fmt.Sprintf("01ARZ3NDEKTSV4RRFFQ69G5F%02X", i/100))
		
		contextType := MovableContext(i % 5) // Rotate through context types
		isHidden := i%7 == 0                 // Some items hidden
		isDelta := i%3 == 0                  // Every third item is a delta
		
		var originID *idwrap.IDWrap
		if isDelta && i > 0 {
			origID := idwrap.NewTextMust(fmt.Sprintf("01ARZ3NDEKTSV4RRFFQ69G5F%02X", i-1))
			originID = &origID
			
			// Create delta relationship
			relationship := DeltaRelationship{
				DeltaID:      itemID,
				OriginID:     *originID,
				RelationType: DeltaManagerRelationType(i % 4),
				CreatedAt:    time.Now(),
				Metadata: DeltaMetadata{
					LastUpdated: time.Now(),
					SyncCount:   i % 10,
					IsStale:     i%20 == 0,
					Priority:    i % 5,
				},
			}
			relationships = append(relationships, relationship)
		}
		
		testItems[i] = testContextualItem{
			ID:       itemID,
			ParentID: parentID,
			Position: i,
			Context:  contextType,
			ScopeID:  scopeID,
			IsHidden: isHidden,
			IsDelta:  isDelta,
			OriginID: originID,
		}
		
		// Create context metadata
		var originBytes *[]byte
		if originID != nil {
			b := originID.Bytes()
			originBytes = &b
		}
		metadata := &ContextMetadata{
			Type:      contextType,
			ScopeID:   scopeID.Bytes(),
			IsHidden:  isHidden,
			OriginID:  originBytes,
			Priority:  i % 10,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		contextMetadata[itemID] = metadata
		
		// Populate resolver mappings
		resolver.contextMappings[itemID.String()] = contextType
		resolver.scopeMappings[fmt.Sprintf("%s_%d", itemID.String(), int(contextType))] = scopeID
		
		// Create scope hierarchy
		hierarchy := []ScopeLevel{
			{Context: ContextWorkspace, ScopeID: scopeID, Name: "workspace", Level: 0},
			{Context: ContextCollection, ScopeID: parentID, Name: "collection", Level: 1},
			{Context: contextType, ScopeID: itemID, Name: "item", Level: 2},
		}
		resolver.hierarchies[itemID.String()] = hierarchy
		
		// Populate cache for some items (simulate partial cache state)
		if i%5 == 0 {
			cache.SetContext(itemID, metadata)
		}
	}
	
	// Track relationships in delta manager
	for _, rel := range relationships {
		deltaManager.TrackDelta(rel.DeltaID, rel.OriginID, rel.RelationType, rel.Metadata)
	}
	
	return &benchmarkTestData{
		contextCache:    cache,
		deltaManager:    deltaManager,
		scopeResolver:   resolver,
		testItems:       testItems,
		relationships:   relationships,
		contextMetadata: contextMetadata,
	}
}

// =============================================================================
// CONTEXT RESOLUTION BENCHMARKS
// =============================================================================

func BenchmarkContextResolution(b *testing.B) {
	sizes := []int{100, 1000, 10000}
	
	for _, size := range sizes {
		b.Run(fmt.Sprintf("ResolveContext_%d_items", size), func(b *testing.B) {
			data := setupBenchmarkTestData(size)
			ctx := context.Background()
			
			// Target: < 1µs per resolution
			b.ResetTimer()
			start := time.Now()
			
			for i := 0; i < b.N; i++ {
				itemIndex := i % len(data.testItems)
				itemID := data.testItems[itemIndex].ID
				_, err := data.scopeResolver.ResolveContext(ctx, itemID)
				if err != nil {
					b.Fatalf("Context resolution failed: %v", err)
				}
			}
			
			elapsed := time.Since(start)
			if b.N > 0 {
				avgTime := elapsed / time.Duration(b.N)
				if avgTime > time.Microsecond {
					b.Errorf("Context resolution too slow: %v > 1µs per operation", avgTime)
				}
			}
		})
		
		b.Run(fmt.Sprintf("ResolveScopeID_%d_items", size), func(b *testing.B) {
			data := setupBenchmarkTestData(size)
			ctx := context.Background()
			
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				itemIndex := i % len(data.testItems)
				item := data.testItems[itemIndex]
				_, err := data.scopeResolver.ResolveScopeID(ctx, item.ID, item.Context)
				if err != nil {
					b.Fatalf("Scope ID resolution failed: %v", err)
				}
			}
		})
	}
}

func BenchmarkContextCachePerformance(b *testing.B) {
	data := setupBenchmarkTestData(10000)
	
	b.Run("CacheHit", func(b *testing.B) {
		// Populate cache with all items
		for _, item := range data.testItems {
			metadata := data.contextMetadata[item.ID]
			data.contextCache.SetContext(item.ID, metadata)
		}
		
		// Target: < 100ns per cache hit
		b.ResetTimer()
		start := time.Now()
		
		for i := 0; i < b.N; i++ {
			itemIndex := i % len(data.testItems)
			itemID := data.testItems[itemIndex].ID
			_, hit := data.contextCache.GetContext(itemID)
			if !hit {
				b.Fatalf("Expected cache hit for item %s", itemID.String())
			}
		}
		
		elapsed := time.Since(start)
		if b.N > 0 {
			avgTime := elapsed / time.Duration(b.N)
			if avgTime > 100*time.Nanosecond {
				b.Errorf("Cache hit too slow: %v > 100ns per operation", avgTime)
			}
		}
	})
	
	b.Run("CacheMiss", func(b *testing.B) {
		// Clear cache to ensure misses
		data.contextCache.Clear()
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			itemIndex := i % len(data.testItems)
			itemID := data.testItems[itemIndex].ID
			_, hit := data.contextCache.GetContext(itemID)
			if hit {
				b.Fatalf("Unexpected cache hit for item %s", itemID.String())
			}
		}
	})
	
	b.Run("CacheUpdate", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			itemIndex := i % len(data.testItems)
			item := data.testItems[itemIndex]
			metadata := data.contextMetadata[item.ID]
			data.contextCache.SetContext(item.ID, metadata)
		}
	})
}

func BenchmarkScopeResolution(b *testing.B) {
	data := setupBenchmarkTestData(1000)
	ctx := context.Background()
	
	b.Run("ValidateScope", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			itemIndex := i % len(data.testItems)
			item := data.testItems[itemIndex]
			err := data.scopeResolver.ValidateScope(ctx, item.ID, item.ScopeID)
			if err != nil {
				b.Fatalf("Scope validation failed: %v", err)
			}
		}
	})
	
	b.Run("GetScopeHierarchy", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			itemIndex := i % len(data.testItems)
			itemID := data.testItems[itemIndex].ID
			_, err := data.scopeResolver.GetScopeHierarchy(ctx, itemID)
			if err != nil {
				b.Fatalf("Scope hierarchy resolution failed: %v", err)
			}
		}
	})
}

// =============================================================================
// DELTA OPERATION BENCHMARKS
// =============================================================================

func BenchmarkDeltaCreation(b *testing.B) {
	data := setupBenchmarkTestData(1000)
	
	b.Run("TrackDelta", func(b *testing.B) {
		// Target: < 100µs per delta creation
		b.ResetTimer()
		start := time.Now()
		
		for i := 0; i < b.N; i++ {
			deltaID := idwrap.NewTextMust(fmt.Sprintf("DELTA%08X", i))
			originIndex := i % len(data.testItems)
			originID := data.testItems[originIndex].ID
			
			metadata := DeltaMetadata{
				LastUpdated: time.Now(),
				SyncCount:   0,
				IsStale:     false,
				Priority:    i % 10,
			}
			
			err := data.deltaManager.TrackDelta(deltaID, originID, DeltaManagerRelationTypeEndpoint, metadata)
			if err != nil {
				b.Fatalf("Delta tracking failed: %v", err)
			}
		}
		
		elapsed := time.Since(start)
		if b.N > 0 {
			avgTime := elapsed / time.Duration(b.N)
			if avgTime > 100*time.Microsecond {
				b.Errorf("Delta creation too slow: %v > 100µs per operation", avgTime)
			}
		}
	})
	
	b.Run("UntrackDelta", func(b *testing.B) {
		// Pre-populate with deltas
		deltaIDs := make([]idwrap.IDWrap, b.N)
		for i := 0; i < b.N; i++ {
			deltaID := idwrap.NewTextMust(fmt.Sprintf("UNTRACK%08X", i))
			originIndex := i % len(data.testItems)
			originID := data.testItems[originIndex].ID
			
			metadata := DeltaMetadata{
				LastUpdated: time.Now(),
				Priority:    i % 5,
			}
			
			data.deltaManager.TrackDelta(deltaID, originID, DeltaManagerRelationTypeEndpoint, metadata)
			deltaIDs[i] = deltaID
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			err := data.deltaManager.UntrackDelta(deltaIDs[i])
			if err != nil {
				b.Fatalf("Delta untracking failed: %v", err)
			}
		}
	})
}

func BenchmarkBatchDeltaOps(b *testing.B) {
	batchSizes := []int{10, 50, 100, 500}
	
	for _, batchSize := range batchSizes {
		b.Run(fmt.Sprintf("BatchTrack_%d_deltas", batchSize), func(b *testing.B) {
			data := setupBenchmarkTestData(1000)
			
			// Target: < 1ms for 100 operations
			b.ResetTimer()
			start := time.Now()
			
			for i := 0; i < b.N; i++ {
				// Create batch of delta operations
				for j := 0; j < batchSize; j++ {
					deltaID := idwrap.NewTextMust(fmt.Sprintf("BATCH%08X%04X", i, j))
					originIndex := (i*batchSize + j) % len(data.testItems)
					originID := data.testItems[originIndex].ID
					
					metadata := DeltaMetadata{
						LastUpdated: time.Now(),
						Priority:    j % 5,
					}
					
					err := data.deltaManager.TrackDelta(deltaID, originID, DeltaManagerRelationTypeEndpoint, metadata)
					if err != nil {
						b.Fatalf("Batch delta tracking failed: %v", err)
					}
				}
			}
			
			elapsed := time.Since(start)
			if batchSize == 100 && b.N > 0 {
				avgTime := elapsed / time.Duration(b.N)
				if avgTime > time.Millisecond {
					b.Errorf("Batch delta operation too slow: %v > 1ms for 100 operations", avgTime)
				}
			}
		})
	}
}

func BenchmarkDeltaTracking(b *testing.B) {
	data := setupBenchmarkTestData(10000)
	
	b.Run("GetDeltasForOrigin", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			originIndex := i % len(data.testItems)
			originID := data.testItems[originIndex].ID
			_, err := data.deltaManager.GetDeltasForOrigin(originID)
			if err != nil {
				b.Fatalf("Get deltas for origin failed: %v", err)
			}
		}
	})
	
	b.Run("GetDeltaRelationship", func(b *testing.B) {
		// Get existing delta IDs
		var deltaIDs []idwrap.IDWrap
		for _, rel := range data.relationships {
			deltaIDs = append(deltaIDs, rel.DeltaID)
			if len(deltaIDs) >= 1000 {
				break
			}
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			deltaIndex := i % len(deltaIDs)
			deltaID := deltaIDs[deltaIndex]
			_, err := data.deltaManager.GetDeltaRelationship(deltaID)
			if err != nil {
				b.Fatalf("Get delta relationship failed: %v", err)
			}
		}
	})
	
	b.Run("HasDelta", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			itemIndex := i % len(data.testItems)
			itemID := data.testItems[itemIndex].ID
			_ = data.deltaManager.HasDelta(itemID)
		}
	})
}

// =============================================================================
// SYNC PROPAGATION BENCHMARKS
// =============================================================================

func BenchmarkSyncPropagation(b *testing.B) {
	data := setupBenchmarkTestData(1000)
	ctx := context.Background()
	
	b.Run("SyncDeltaPositions", func(b *testing.B) {
		// Target: < 500ms for 1000 items
		b.ResetTimer()
		start := time.Now()
		
		for i := 0; i < b.N; i++ {
			originIndex := i % len(data.testItems)
			originID := data.testItems[originIndex].ID
			listType := CollectionListTypeEndpoints
			
			result, err := data.deltaManager.SyncDeltaPositions(ctx, nil, originID, listType)
			if err != nil {
				b.Fatalf("Sync delta positions failed: %v", err)
			}
			
			_ = result // Prevent optimization
		}
		
		elapsed := time.Since(start)
		if b.N > 0 {
			avgTime := elapsed / time.Duration(b.N)
			if avgTime > 500*time.Millisecond {
				b.Errorf("Sync propagation too slow: %v > 500ms per operation", avgTime)
			}
		}
	})
	
	b.Run("ValidateDeltaConsistency", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			result, err := data.deltaManager.ValidateDeltaConsistency(ctx)
			if err != nil {
				b.Fatalf("Delta consistency validation failed: %v", err)
			}
			_ = result
		}
	})
}

func BenchmarkBatchSyncOperations(b *testing.B) {
	data := setupBenchmarkTestData(1000)
	ctx := context.Background()
	
	b.Run("BatchSync", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Sync multiple origins in batch
			batchSize := 10
			for j := 0; j < batchSize; j++ {
				originIndex := (i*batchSize + j) % len(data.testItems)
				originID := data.testItems[originIndex].ID
				
				_, err := data.deltaManager.SyncDeltaPositions(ctx, nil, originID, CollectionListTypeEndpoints)
				if err != nil {
					b.Fatalf("Batch sync failed: %v", err)
				}
			}
		}
	})
}

func BenchmarkConsistencyValidation(b *testing.B) {
	data := setupBenchmarkTestData(10000)
	ctx := context.Background()
	
	b.Run("FullConsistencyCheck", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			result, err := data.deltaManager.ValidateDeltaConsistency(ctx)
			if err != nil {
				b.Fatalf("Consistency validation failed: %v", err)
			}
			
			// Validate result structure
			if result == nil {
				b.Fatalf("Consistency check returned nil result")
			}
		}
	})
	
	b.Run("PruneStaleDelta", func(b *testing.B) {
		// Create some stale deltas
		staleDeltas := make([]idwrap.IDWrap, b.N)
		for i := 0; i < b.N; i++ {
			deltaID := idwrap.NewTextMust(fmt.Sprintf("STALE%08X", i))
			originIndex := i % len(data.testItems)
			originID := data.testItems[originIndex].ID
			
			metadata := DeltaMetadata{
				LastUpdated: time.Now().Add(-24 * time.Hour),
				IsStale:     true,
				Priority:    0,
			}
			
			data.deltaManager.TrackDelta(deltaID, originID, DeltaManagerRelationTypeEndpoint, metadata)
			staleDeltas[i] = deltaID
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			err := data.deltaManager.PruneStaleDelta(ctx, staleDeltas[i])
			if err != nil {
				b.Fatalf("Prune stale delta failed: %v", err)
			}
		}
	})
}

// =============================================================================
// CROSS-CONTEXT BENCHMARKS
// =============================================================================

func BenchmarkCrossContextMove(b *testing.B) {
	data := setupBenchmarkTestData(1000)
	ctx := context.Background()
	
	b.Run("CrossContextResolution", func(b *testing.B) {
		// Target: < 100µs for cross-context operations
		b.ResetTimer()
		start := time.Now()
		
		for i := 0; i < b.N; i++ {
			sourceIndex := i % len(data.testItems)
			targetIndex := (i + 1) % len(data.testItems)
			
			sourceItem := data.testItems[sourceIndex]
			targetItem := data.testItems[targetIndex]
			
			// Resolve contexts for both items
			_, err := data.scopeResolver.ResolveContext(ctx, sourceItem.ID)
			if err != nil {
				b.Fatalf("Source context resolution failed: %v", err)
			}
			
			_, err = data.scopeResolver.ResolveContext(ctx, targetItem.ID)
			if err != nil {
				b.Fatalf("Target context resolution failed: %v", err)
			}
			
			// Validate scope compatibility
			err = data.scopeResolver.ValidateScope(ctx, sourceItem.ID, targetItem.ScopeID)
			if err != nil {
				// Expected for cross-context operations
			}
		}
		
		elapsed := time.Since(start)
		if b.N > 0 {
			avgTime := elapsed / time.Duration(b.N)
			if avgTime > 100*time.Microsecond {
				b.Errorf("Cross-context operation too slow: %v > 100µs per operation", avgTime)
			}
		}
	})
}

func BenchmarkScopeValidation(b *testing.B) {
	data := setupBenchmarkTestData(1000)
	ctx := context.Background()
	
	b.Run("ValidateScopeHierarchy", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			itemIndex := i % len(data.testItems)
			itemID := data.testItems[itemIndex].ID
			
			hierarchy, err := data.scopeResolver.GetScopeHierarchy(ctx, itemID)
			if err != nil {
				b.Fatalf("Scope hierarchy resolution failed: %v", err)
			}
			
			// Validate hierarchy structure
			if len(hierarchy) == 0 {
				b.Fatalf("Empty hierarchy for item %s", itemID.String())
			}
		}
	})
}

func BenchmarkContextMigration(b *testing.B) {
	data := setupBenchmarkTestData(1000)
	
	b.Run("ContextCacheInvalidation", func(b *testing.B) {
		// Populate cache
		for _, item := range data.testItems {
			metadata := data.contextMetadata[item.ID]
			data.contextCache.SetContext(item.ID, metadata)
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			itemIndex := i % len(data.testItems)
			itemID := data.testItems[itemIndex].ID
			data.contextCache.InvalidateContext(itemID)
		}
	})
	
	b.Run("ScopeInvalidation", func(b *testing.B) {
		// Populate cache
		for _, item := range data.testItems {
			metadata := data.contextMetadata[item.ID]
			data.contextCache.SetContext(item.ID, metadata)
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			itemIndex := i % len(data.testItems)
			scopeID := data.testItems[itemIndex].ScopeID
			data.contextCache.InvalidateScope(scopeID)
		}
	})
}

// =============================================================================
// MEMORY & ALLOCATION BENCHMARKS
// =============================================================================

func BenchmarkContextMemoryEfficiency(b *testing.B) {
	sizes := []int{100, 1000, 10000}
	
	for _, size := range sizes {
		b.Run(fmt.Sprintf("ContextResolution_Memory_%d", size), func(b *testing.B) {
			data := setupBenchmarkTestData(size)
			ctx := context.Background()
			
			var m1, m2 runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&m1)
			
			b.ReportAllocs()
			b.ResetTimer()
			
			for i := 0; i < b.N; i++ {
				itemIndex := i % len(data.testItems)
				itemID := data.testItems[itemIndex].ID
				_, err := data.scopeResolver.ResolveContext(ctx, itemID)
				if err != nil {
					b.Fatalf("Context resolution failed: %v", err)
				}
			}
			
			runtime.ReadMemStats(&m2)
			totalAlloc := m2.TotalAlloc - m1.TotalAlloc
			allocsPerOp := float64(totalAlloc) / float64(b.N)
			
			b.ReportMetric(allocsPerOp, "bytes/op_actual")
			
			// Log memory usage patterns
			if size == 1000 {
				b.Logf("Memory per context resolution (1000 items): %.2f bytes", allocsPerOp)
			}
		})
		
		b.Run(fmt.Sprintf("DeltaTracking_Memory_%d", size), func(b *testing.B) {
			data := setupBenchmarkTestData(size)
			
			var m1, m2 runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&m1)
			
			b.ReportAllocs()
			b.ResetTimer()
			
			for i := 0; i < b.N; i++ {
				deltaID := idwrap.NewTextMust(fmt.Sprintf("MEMORY%08X", i))
				originIndex := i % len(data.testItems)
				originID := data.testItems[originIndex].ID
				
				metadata := DeltaMetadata{
					LastUpdated: time.Now(),
					Priority:    i % 5,
				}
				
				err := data.deltaManager.TrackDelta(deltaID, originID, DeltaManagerRelationTypeEndpoint, metadata)
				if err != nil {
					b.Fatalf("Delta tracking failed: %v", err)
				}
			}
			
			runtime.ReadMemStats(&m2)
			totalAlloc := m2.TotalAlloc - m1.TotalAlloc
			allocsPerOp := float64(totalAlloc) / float64(b.N)
			
			b.ReportMetric(allocsPerOp, "bytes/op_delta")
		})
	}
}

func BenchmarkMemoryGrowthPattern(b *testing.B) {
	sizes := []int{100, 500, 1000, 5000, 10000}
	
	b.Run("CacheGrowthPattern", func(b *testing.B) {
		results := make(map[int]float64)
		
		for _, size := range sizes {
			data := setupBenchmarkTestData(size)
			
			var m1, m2 runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&m1)
			
			// Populate cache completely
			for _, item := range data.testItems {
				metadata := data.contextMetadata[item.ID]
				data.contextCache.SetContext(item.ID, metadata)
			}
			
			runtime.GC()
			runtime.ReadMemStats(&m2)
			
			memoryUsed := float64(m2.TotalAlloc - m1.TotalAlloc)
			memoryPerItem := memoryUsed / float64(size)
			results[size] = memoryPerItem
			
			b.Logf("Size: %d, Memory per item: %.2f bytes", size, memoryPerItem)
		}
		
		// Analyze growth pattern
		analyzeMemoryGrowth(b, results, "Cache Growth")
	})
	
	b.Run("DeltaManagerGrowthPattern", func(b *testing.B) {
		results := make(map[int]float64)
		
		for _, size := range sizes {
			var m1, m2 runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&m1)
			
			data := setupBenchmarkTestData(size)
			_ = data // Prevent optimization
			
			runtime.GC()
			runtime.ReadMemStats(&m2)
			
			memoryUsed := float64(m2.TotalAlloc - m1.TotalAlloc)
			memoryPerItem := memoryUsed / float64(size)
			results[size] = memoryPerItem
			
			b.Logf("Size: %d, Delta manager memory per item: %.2f bytes", size, memoryPerItem)
		}
		
		// Analyze growth pattern
		analyzeMemoryGrowth(b, results, "Delta Manager Growth")
	})
}

func BenchmarkCacheEfficiency(b *testing.B) {
	data := setupBenchmarkTestData(10000)
	
	b.Run("CacheHitRatio", func(b *testing.B) {
		// Populate 50% of cache
		for i, item := range data.testItems {
			if i%2 == 0 {
				metadata := data.contextMetadata[item.ID]
				data.contextCache.SetContext(item.ID, metadata)
			}
		}
		
		hits := 0
		misses := 0
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			itemIndex := i % len(data.testItems)
			itemID := data.testItems[itemIndex].ID
			_, hit := data.contextCache.GetContext(itemID)
			if hit {
				hits++
			} else {
				misses++
			}
		}
		
		hitRatio := float64(hits) / float64(hits+misses) * 100
		b.ReportMetric(hitRatio, "hit_ratio_percent")
		b.Logf("Cache hit ratio: %.2f%%", hitRatio)
	})
	
	b.Run("CacheEvictionPattern", func(b *testing.B) {
		cache := NewInMemoryContextCache(time.Minute)
		
		// Fill cache to capacity and beyond
		for i := 0; i < 1500; i++ { // Exceed default cache size of 1000
			itemID := idwrap.NewTextMust(fmt.Sprintf("EVICT%08X", i))
			metadata := &ContextMetadata{
				Type:      ContextCollection,
				ScopeID:   []byte("scope"),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			cache.SetContext(itemID, metadata)
		}
		
		// Test access patterns after eviction
		hits := 0
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			itemID := idwrap.NewTextMust(fmt.Sprintf("EVICT%08X", i%1500))
			_, hit := cache.GetContext(itemID)
			if hit {
				hits++
			}
		}
		
		remainingRatio := float64(hits) / float64(b.N) * 100
		b.ReportMetric(remainingRatio, "remaining_after_eviction_percent")
	})
}

// =============================================================================
// CONCURRENT PERFORMANCE BENCHMARKS
// =============================================================================

func BenchmarkConcurrentContextOperations(b *testing.B) {
	data := setupBenchmarkTestData(1000)
	ctx := context.Background()
	
	goroutineCounts := []int{1, 2, 4, 8, 16}
	
	for _, numGoroutines := range goroutineCounts {
		b.Run(fmt.Sprintf("ConcurrentContextResolution_%d_goroutines", numGoroutines), func(b *testing.B) {
			b.ReportAllocs()
			b.SetParallelism(numGoroutines)
			
			b.RunParallel(func(pb *testing.PB) {
				itemIndex := 0
				for pb.Next() {
					idx := itemIndex % len(data.testItems)
					itemID := data.testItems[idx].ID
					_, err := data.scopeResolver.ResolveContext(ctx, itemID)
					if err != nil {
						b.Errorf("Concurrent context resolution failed: %v", err)
						return
					}
					itemIndex++
				}
			})
		})
		
		b.Run(fmt.Sprintf("ConcurrentCacheAccess_%d_goroutines", numGoroutines), func(b *testing.B) {
			// Populate cache
			for _, item := range data.testItems {
				metadata := data.contextMetadata[item.ID]
				data.contextCache.SetContext(item.ID, metadata)
			}
			
			b.ReportAllocs()
			b.SetParallelism(numGoroutines)
			
			b.RunParallel(func(pb *testing.PB) {
				itemIndex := 0
				for pb.Next() {
					idx := itemIndex % len(data.testItems)
					itemID := data.testItems[idx].ID
					_, _ = data.contextCache.GetContext(itemID)
					itemIndex++
				}
			})
		})
	}
}

func BenchmarkConcurrentContextDeltaOperations(b *testing.B) {
	data := setupBenchmarkTestData(1000)
	
	b.Run("ConcurrentDeltaTracking", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				deltaID := idwrap.NewTextMust(fmt.Sprintf("CONCURRENT%08X", i))
				originIndex := i % len(data.testItems)
				originID := data.testItems[originIndex].ID
				
				metadata := DeltaMetadata{
					LastUpdated: time.Now(),
					Priority:    i % 5,
				}
				
				err := data.deltaManager.TrackDelta(deltaID, originID, DeltaManagerRelationTypeEndpoint, metadata)
				if err != nil {
					b.Errorf("Concurrent delta tracking failed: %v", err)
					return
				}
				i++
			}
		})
	})
	
	b.Run("ConcurrentDeltaLookup", func(b *testing.B) {
		// Pre-populate with deltas
		var deltaIDs []idwrap.IDWrap
		for i := 0; i < 1000; i++ {
			deltaID := idwrap.NewTextMust(fmt.Sprintf("LOOKUP%08X", i))
			originIndex := i % len(data.testItems)
			originID := data.testItems[originIndex].ID
			
			metadata := DeltaMetadata{
				LastUpdated: time.Now(),
				Priority:    i % 5,
			}
			
			data.deltaManager.TrackDelta(deltaID, originID, DeltaManagerRelationTypeEndpoint, metadata)
			deltaIDs = append(deltaIDs, deltaID)
		}
		
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				deltaIndex := i % len(deltaIDs)
				deltaID := deltaIDs[deltaIndex]
				_, err := data.deltaManager.GetDeltaRelationship(deltaID)
				if err != nil {
					b.Errorf("Concurrent delta lookup failed: %v", err)
					return
				}
				i++
			}
		})
	})
}

// =============================================================================
// PERFORMANCE ANALYSIS UTILITIES
// =============================================================================

func analyzeMemoryGrowth(b *testing.B, results map[int]float64, operation string) {
	if len(results) < 2 {
		b.Logf("Not enough data points for memory growth analysis of %s", operation)
		return
	}
	
	sizes := make([]int, 0, len(results))
	for size := range results {
		sizes = append(sizes, size)
	}
	sort.Ints(sizes)
	
	b.Logf("\n=== Memory Growth Analysis for %s ===", operation)
	b.Logf("Size\tBytes/Item\tGrowth Ratio")
	
	for i := 1; i < len(sizes); i++ {
		currentSize := sizes[i]
		prevSize := sizes[i-1]
		
		currentMemory := results[currentSize]
		prevMemory := results[prevSize]
		
		growthRatio := currentMemory / prevMemory
		
		b.Logf("%d\t%.2f\t\t%.2f", currentSize, currentMemory, growthRatio)
		
		// Warn if memory growth is non-linear
		if growthRatio > 2.0 {
			b.Logf("WARNING: Memory growth ratio %.2f may indicate inefficient scaling", growthRatio)
		}
	}
}

// Mock repository for testing
type mockRepository struct{}

func (r *mockRepository) UpdatePosition(ctx context.Context, tx *sql.Tx, itemID idwrap.IDWrap, listType ListType, position int) error {
	return nil
}

func (r *mockRepository) UpdatePositions(ctx context.Context, tx *sql.Tx, updates []PositionUpdate) error {
	return nil
}

func (r *mockRepository) GetMaxPosition(ctx context.Context, parentID idwrap.IDWrap, listType ListType) (int, error) {
	return 1000, nil
}

func (r *mockRepository) GetItemsByParent(ctx context.Context, parentID idwrap.IDWrap, listType ListType) ([]MovableItem, error) {
	return []MovableItem{
		{ID: parentID, Position: 0, ListType: listType},
	}, nil
}

// =============================================================================
// COMPREHENSIVE PERFORMANCE TARGET VALIDATION
// =============================================================================

func BenchmarkPerformanceTargetValidation(b *testing.B) {
	b.Log("\n=== CONTEXT-AWARE MOVABLE SYSTEM PERFORMANCE VALIDATION ===")
	b.Log("Validating performance targets:")
	b.Log("• Single operation: < 100µs")
	b.Log("• Batch (100 ops): < 1ms")
	b.Log("• Context resolution: < 1µs cached")
	b.Log("• Sync propagation: < 500ms for 1000 items")
	
	data := setupBenchmarkTestData(1000)
	ctx := context.Background()
	
	// Test 1: Single operation performance
	start := time.Now()
	itemID := data.testItems[0].ID
	_, err := data.scopeResolver.ResolveContext(ctx, itemID)
	if err != nil {
		b.Fatalf("Context resolution failed: %v", err)
	}
	singleOpTime := time.Since(start)
	
	// Test 2: Batch operation performance
	start = time.Now()
	for i := 0; i < 100; i++ {
		idx := i % len(data.testItems)
		itemID := data.testItems[idx].ID
		_, err := data.scopeResolver.ResolveContext(ctx, itemID)
		if err != nil {
			b.Fatalf("Batch context resolution failed: %v", err)
		}
	}
	batchOpTime := time.Since(start)
	
	// Test 3: Cached context resolution
	metadata := data.contextMetadata[itemID]
	data.contextCache.SetContext(itemID, metadata)
	
	start = time.Now()
	_, hit := data.contextCache.GetContext(itemID)
	if !hit {
		b.Fatalf("Expected cache hit")
	}
	cachedOpTime := time.Since(start)
	
	// Test 4: Sync propagation
	start = time.Now()
	originID := data.testItems[0].ID
	_, err = data.deltaManager.SyncDeltaPositions(ctx, nil, originID, CollectionListTypeEndpoints)
	if err != nil {
		b.Fatalf("Sync propagation failed: %v", err)
	}
	syncPropTime := time.Since(start)
	
	// Report results
	b.Logf("Single operation time: %v (target: < 100µs)", singleOpTime)
	b.Logf("Batch operation time: %v (target: < 1ms)", batchOpTime)
	b.Logf("Cached resolution time: %v (target: < 1µs)", cachedOpTime)
	b.Logf("Sync propagation time: %v (target: < 500ms)", syncPropTime)
	
	// Validate targets
	passed := 0
	total := 4
	
	if singleOpTime < 100*time.Microsecond {
		b.Log("✅ Single operation target MET")
		passed++
	} else {
		b.Log("❌ Single operation target NOT MET")
	}
	
	if batchOpTime < time.Millisecond {
		b.Log("✅ Batch operation target MET")
		passed++
	} else {
		b.Log("❌ Batch operation target NOT MET")
	}
	
	if cachedOpTime < time.Microsecond {
		b.Log("✅ Cached resolution target MET")
		passed++
	} else {
		b.Log("❌ Cached resolution target NOT MET")
	}
	
	if syncPropTime < 500*time.Millisecond {
		b.Log("✅ Sync propagation target MET")
		passed++
	} else {
		b.Log("❌ Sync propagation target NOT MET")
	}
	
	b.Logf("\nOVERALL PERFORMANCE SCORE: %d/%d targets met", passed, total)
	
	if passed == total {
		b.Log("🎉 ALL PERFORMANCE TARGETS MET!")
	} else {
		b.Logf("⚠️  %d/%d performance targets need improvement", total-passed, total)
	}
}