package movable

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"
	
	"the-dev-tools/server/pkg/idwrap"
)

// =============================================================================
// BENCHMARK SETUP AND HELPERS
// =============================================================================

// BenchmarkSetup provides common setup for benchmarks
type BenchmarkSetup struct {
	repo       *EnhancedExampleRepository
	ctx        context.Context
	originIDs  []idwrap.IDWrap
	deltaIDs   []idwrap.IDWrap
	endpointID idwrap.IDWrap
}

// setupBenchmark creates a benchmark setup with specified number of items
func setupBenchmark(b *testing.B, numItems int) *BenchmarkSetup {
	b.Helper()
	
	repo := mockExampleRepository()
	repo.SimpleRepository = &mockMovableRepository{
		items: make(map[idwrap.IDWrap]MovableItem),
	}
	
	ctx := context.Background()
	endpointID := idwrap.NewTextMust("benchmark-endpoint")
	
	originIDs := make([]idwrap.IDWrap, numItems)
	deltaIDs := make([]idwrap.IDWrap, numItems)
	
	for i := 0; i < numItems; i++ {
		originIDs[i] = idwrap.NewTextMust(fmt.Sprintf("origin-bench-%d", i))
		deltaIDs[i] = idwrap.NewTextMust(fmt.Sprintf("delta-bench-%d", i))
	}
	
	return &BenchmarkSetup{
		repo:       repo,
		ctx:        ctx,
		originIDs:  originIDs,
		deltaIDs:   deltaIDs,
		endpointID: endpointID,
	}
}

// createPreexistingDeltas creates delta examples for benchmarking resolution
func (s *BenchmarkSetup) createPreexistingDeltas(b *testing.B) {
	b.Helper()
	
	for i := range s.originIDs {
		err := s.repo.CreateDeltaExample(s.ctx, nil, s.originIDs[i], s.deltaIDs[i], 
			s.endpointID, OverrideLevelHeaders)
		if err != nil {
			b.Fatalf("Failed to create delta example: %v", err)
		}
	}
}

// =============================================================================
// CREATION BENCHMARKS
// =============================================================================

func BenchmarkCreateDeltaExample(b *testing.B) {
	setup := setupBenchmark(b, b.N)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		err := setup.repo.CreateDeltaExample(setup.ctx, nil, setup.originIDs[i], 
			setup.deltaIDs[i], setup.endpointID, OverrideLevelHeaders)
		if err != nil {
			b.Fatalf("CreateDeltaExample failed: %v", err)
		}
	}
}

func BenchmarkCreateDeltaExampleConcurrent(b *testing.B) {
	setup := setupBenchmark(b, b.N)
	
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			originID := idwrap.NewTextMust(fmt.Sprintf("origin-concurrent-%d", i))
			deltaID := idwrap.NewTextMust(fmt.Sprintf("delta-concurrent-%d", i))
			
			err := setup.repo.CreateDeltaExample(setup.ctx, nil, originID, deltaID, 
				setup.endpointID, OverrideLevelHeaders)
			if err != nil {
				b.Fatalf("CreateDeltaExample failed: %v", err)
			}
			i++
		}
	})
}

// =============================================================================
// RESOLUTION BENCHMARKS
// =============================================================================

func BenchmarkResolveDeltaExample(b *testing.B) {
	setup := setupBenchmark(b, 1000) // Create 1000 deltas for resolution
	setup.createPreexistingDeltas(b)
	
	contextMeta := &ContextMetadata{
		Type:    ContextEndpoint,
		ScopeID: setup.endpointID.Bytes(),
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		originID := setup.originIDs[i%len(setup.originIDs)]
		_, err := setup.repo.ResolveDeltaExample(setup.ctx, originID, contextMeta)
		if err != nil {
			b.Fatalf("ResolveDeltaExample failed: %v", err)
		}
	}
}

func BenchmarkResolveDeltaExampleConcurrent(b *testing.B) {
	setup := setupBenchmark(b, 1000)
	setup.createPreexistingDeltas(b)
	
	contextMeta := &ContextMetadata{
		Type:    ContextEndpoint,
		ScopeID: setup.endpointID.Bytes(),
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			originID := setup.originIDs[i%len(setup.originIDs)]
			_, err := setup.repo.ResolveDeltaExample(setup.ctx, originID, contextMeta)
			if err != nil {
				b.Fatalf("ResolveDeltaExample failed: %v", err)
			}
			i++
		}
	})
}

// =============================================================================
// BATCH OPERATION BENCHMARKS
// =============================================================================

func BenchmarkBatchCreateDeltaExamples_10(b *testing.B) {
	benchmarkBatchCreateDeltaExamples(b, 10)
}

func BenchmarkBatchCreateDeltaExamples_25(b *testing.B) {
	benchmarkBatchCreateDeltaExamples(b, 25)
}

func BenchmarkBatchCreateDeltaExamples_50(b *testing.B) {
	benchmarkBatchCreateDeltaExamples(b, 50)
}

func benchmarkBatchCreateDeltaExamples(b *testing.B, batchSize int) {
	setup := setupBenchmark(b, batchSize)
	
	// Create operations batch
	operations := make([]DeltaExampleOperation, batchSize)
	for i := 0; i < batchSize; i++ {
		operations[i] = DeltaExampleOperation{
			OriginID:      setup.originIDs[i],
			DeltaID:       setup.deltaIDs[i],
			EndpointID:    setup.endpointID,
			OverrideLevel: OverrideLevelHeaders,
		}
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		// Create new IDs for each iteration to avoid conflicts
		for j := range operations {
			operations[j].OriginID = idwrap.NewTextMust(fmt.Sprintf("origin-batch-%d-%d", i, j))
			operations[j].DeltaID = idwrap.NewTextMust(fmt.Sprintf("delta-batch-%d-%d", i, j))
		}
		
		err := setup.repo.BatchCreateDeltaExamples(setup.ctx, nil, operations)
		if err != nil {
			b.Fatalf("BatchCreateDeltaExamples failed: %v", err)
		}
	}
}

func BenchmarkBatchResolveExamples_10(b *testing.B) {
	benchmarkBatchResolveExamples(b, 10)
}

func BenchmarkBatchResolveExamples_50(b *testing.B) {
	benchmarkBatchResolveExamples(b, 50)
}

func BenchmarkBatchResolveExamples_100(b *testing.B) {
	benchmarkBatchResolveExamples(b, 100)
}

func benchmarkBatchResolveExamples(b *testing.B, batchSize int) {
	setup := setupBenchmark(b, batchSize)
	setup.createPreexistingDeltas(b)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_, err := setup.repo.BatchResolveExamples(setup.ctx, setup.originIDs, setup.endpointID)
		if err != nil {
			b.Fatalf("BatchResolveExamples failed: %v", err)
		}
	}
}

// =============================================================================
// SYNC AND MAINTENANCE BENCHMARKS
// =============================================================================

func BenchmarkSyncDeltaExamples(b *testing.B) {
	setup := setupBenchmark(b, 100)
	setup.createPreexistingDeltas(b)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		originID := setup.originIDs[i%len(setup.originIDs)]
		err := setup.repo.SyncDeltaExamples(setup.ctx, nil, originID)
		if err != nil {
			b.Fatalf("SyncDeltaExamples failed: %v", err)
		}
	}
}

func BenchmarkPruneStaleDeltaExamples(b *testing.B) {
	setup := setupBenchmark(b, 100)
	setup.createPreexistingDeltas(b)
	
	// Make some deltas stale
	for i := 0; i < len(setup.deltaIDs)/2; i++ {
		relation := setup.repo.getExampleDeltaRelation(setup.deltaIDs[i])
		if relation != nil {
			relation.LastSyncTime = time.Now().Add(-48 * time.Hour)
		}
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		err := setup.repo.PruneStaleDeltaExamples(setup.ctx, nil, setup.endpointID)
		if err != nil {
			b.Fatalf("PruneStaleDeltaExamples failed: %v", err)
		}
	}
}

// =============================================================================
// CONTEXT OPERATION BENCHMARKS
// =============================================================================

func BenchmarkGetEndpointContext(b *testing.B) {
	setup := setupBenchmark(b, 1)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_, err := setup.repo.GetEndpointContext(setup.endpointID)
		if err != nil {
			b.Fatalf("GetEndpointContext failed: %v", err)
		}
	}
}

func BenchmarkUpdateExamplePositionInEndpoint(b *testing.B) {
	setup := setupBenchmark(b, 1)
	exampleID := idwrap.NewTextMust("benchmark-example")
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		position := i % 100 // Vary positions
		err := setup.repo.UpdateExamplePositionInEndpoint(setup.ctx, nil, 
			exampleID, setup.endpointID, position)
		if err != nil {
			b.Fatalf("UpdateExamplePositionInEndpoint failed: %v", err)
		}
	}
}

// =============================================================================
// MEMORY AND CONCURRENCY BENCHMARKS
// =============================================================================

func BenchmarkConcurrentDeltaOperations(b *testing.B) {
	setup := setupBenchmark(b, 1000)
	
	contextMeta := &ContextMetadata{
		Type:    ContextEndpoint,
		ScopeID: setup.endpointID.Bytes(),
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			originID := idwrap.NewTextMust(fmt.Sprintf("concurrent-origin-%d", i))
			deltaID := idwrap.NewTextMust(fmt.Sprintf("concurrent-delta-%d", i))
			
			// Perform create and resolve operations
			err := setup.repo.CreateDeltaExample(setup.ctx, nil, originID, deltaID, 
				setup.endpointID, OverrideLevelHeaders)
			if err != nil {
				b.Fatalf("CreateDeltaExample failed: %v", err)
			}
			
			_, err = setup.repo.ResolveDeltaExample(setup.ctx, originID, contextMeta)
			if err != nil {
				b.Fatalf("ResolveDeltaExample failed: %v", err)
			}
			
			i++
		}
	})
}

func BenchmarkMemoryUsageScaling(b *testing.B) {
	sizes := []int{10, 100, 1000, 5000}
	
	for _, size := range sizes {
		b.Run(fmt.Sprintf("Size%d", size), func(b *testing.B) {
			setup := setupBenchmark(b, size)
			
			b.ResetTimer()
			b.ReportAllocs()
			
			for i := 0; i < b.N; i++ {
				// Create all deltas
				for j := 0; j < size; j++ {
					originID := idwrap.NewTextMust(fmt.Sprintf("mem-origin-%d-%d", i, j))
					deltaID := idwrap.NewTextMust(fmt.Sprintf("mem-delta-%d-%d", i, j))
					
					err := setup.repo.CreateDeltaExample(setup.ctx, nil, originID, deltaID, 
						setup.endpointID, OverrideLevelHeaders)
					if err != nil {
						b.Fatalf("CreateDeltaExample failed: %v", err)
					}
				}
				
				// Clear for next iteration to test scaling
				setup.repo.exampleDeltas = make(map[string]*ExampleDeltaRelation)
				setup.repo.endpointContexts = make(map[string]*EndpointContext)
			}
		})
	}
}

// =============================================================================
// OVERRIDE LEVEL PERFORMANCE COMPARISON
// =============================================================================

func BenchmarkOverrideLevelNone(b *testing.B) {
	benchmarkOverrideLevel(b, OverrideLevelNone)
}

func BenchmarkOverrideLevelHeaders(b *testing.B) {
	benchmarkOverrideLevel(b, OverrideLevelHeaders)
}

func BenchmarkOverrideLevelBody(b *testing.B) {
	benchmarkOverrideLevel(b, OverrideLevelBody)
}

func BenchmarkOverrideLevelComplete(b *testing.B) {
	benchmarkOverrideLevel(b, OverrideLevelComplete)
}

func benchmarkOverrideLevel(b *testing.B, level ExampleOverrideLevel) {
	setup := setupBenchmark(b, 100)
	
	// Create deltas with specific override level
	for i := range setup.originIDs {
		err := setup.repo.CreateDeltaExample(setup.ctx, nil, setup.originIDs[i], 
			setup.deltaIDs[i], setup.endpointID, level)
		if err != nil {
			b.Fatalf("Failed to create delta example: %v", err)
		}
	}
	
	contextMeta := &ContextMetadata{
		Type:    ContextEndpoint,
		ScopeID: setup.endpointID.Bytes(),
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		originID := setup.originIDs[i%len(setup.originIDs)]
		_, err := setup.repo.ResolveDeltaExample(setup.ctx, originID, contextMeta)
		if err != nil {
			b.Fatalf("ResolveDeltaExample failed: %v", err)
		}
	}
}

// =============================================================================
// PERFORMANCE VALIDATION BENCHMARKS
// =============================================================================

// BenchmarkSingleOperationLatency ensures single operations complete under 100μs
func BenchmarkSingleOperationLatency(b *testing.B) {
	setup := setupBenchmark(b, 1)
	
	operations := map[string]func() error{
		"CreateDelta": func() error {
			originID := idwrap.NewTextMust("latency-origin")
			deltaID := idwrap.NewTextMust("latency-delta")
			return setup.repo.CreateDeltaExample(setup.ctx, nil, originID, deltaID, 
				setup.endpointID, OverrideLevelHeaders)
		},
		"ResolveDelta": func() error {
			originID := idwrap.NewTextMust("latency-resolve-origin")
			contextMeta := &ContextMetadata{
				Type:    ContextEndpoint,
				ScopeID: setup.endpointID.Bytes(),
			}
			_, err := setup.repo.ResolveDeltaExample(setup.ctx, originID, contextMeta)
			return err
		},
		"GetContext": func() error {
			_, err := setup.repo.GetEndpointContext(setup.endpointID)
			return err
		},
	}
	
	for opName, opFunc := range operations {
		b.Run(opName, func(b *testing.B) {
			b.ResetTimer()
			
			start := time.Now()
			for i := 0; i < b.N; i++ {
				if err := opFunc(); err != nil {
					b.Fatalf("Operation %s failed: %v", opName, err)
				}
			}
			elapsed := time.Since(start)
			
			avgLatency := elapsed / time.Duration(b.N)
			if avgLatency > 100*time.Microsecond {
				b.Errorf("Operation %s average latency %v exceeds 100μs threshold", 
					opName, avgLatency)
			}
		})
	}
}

// BenchmarkThroughput measures operations per second
func BenchmarkThroughput(b *testing.B) {
	setup := setupBenchmark(b, 1000)
	
	b.ResetTimer()
	start := time.Now()
	
	for i := 0; i < b.N; i++ {
		originID := idwrap.NewTextMust(fmt.Sprintf("throughput-origin-%d", i))
		deltaID := idwrap.NewTextMust(fmt.Sprintf("throughput-delta-%d", i))
		
		err := setup.repo.CreateDeltaExample(setup.ctx, nil, originID, deltaID, 
			setup.endpointID, OverrideLevelHeaders)
		if err != nil {
			b.Fatalf("CreateDeltaExample failed: %v", err)
		}
	}
	
	elapsed := time.Since(start)
	opsPerSecond := float64(b.N) / elapsed.Seconds()
	
	b.ReportMetric(opsPerSecond, "ops/sec")
	
	// Log performance for validation
	if opsPerSecond < 1000 { // Expecting at least 1000 ops/sec
		b.Logf("Warning: Throughput %0.2f ops/sec may be below expected threshold", opsPerSecond)
	}
}

// =============================================================================
// EXTENDED MOCK FOR BENCHMARKS
// =============================================================================

// EnhancedMockSimpleRepository provides additional mocking for benchmarks
type EnhancedMockSimpleRepository struct {
	*MockSimpleRepository
	createDeltaCallCount    int
	resolveDeltaCallCount   int
	updatePositionCallCount int
}

// CreateDelta tracks call count for benchmark analysis
func (m *EnhancedMockSimpleRepository) CreateDelta(ctx context.Context, tx *sql.Tx,
	originID idwrap.IDWrap, deltaID idwrap.IDWrap, relation *DeltaRelation) error {
	m.createDeltaCallCount++
	return nil
}

// ResolveDelta tracks call count for benchmark analysis
func (m *EnhancedMockSimpleRepository) ResolveDelta(ctx context.Context, originID idwrap.IDWrap,
	contextMeta *ContextMetadata) (*ResolvedItem, error) {
	m.resolveDeltaCallCount++
	return &ResolvedItem{
		ID:             originID,
		EffectiveID:    originID,
		OriginID:       &originID,
		ResolutionType: ResolutionOrigin,
		ResolutionTime: time.Now(),
	}, nil
}

// UpdatePositionWithContext tracks call count for benchmark analysis
func (m *EnhancedMockSimpleRepository) UpdatePositionWithContext(ctx context.Context, tx *sql.Tx,
	itemID idwrap.IDWrap, listType ListType, position int, contextMeta *ContextMetadata) error {
	m.updatePositionCallCount++
	return nil
}

// GetCallCounts returns the call counts for analysis
func (m *EnhancedMockSimpleRepository) GetCallCounts() (create, resolve, updatePos int) {
	return m.createDeltaCallCount, m.resolveDeltaCallCount, m.updatePositionCallCount
}