package scollection_test

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"testing"
	"time"

	"the-dev-tools/db/pkg/dbtest"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/model/mcollection"
	"the-dev-tools/server/pkg/model/muser"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/model/mworkspaceuser"
	"the-dev-tools/server/pkg/movable"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/service/sworkspacesusers"
	"the-dev-tools/server/pkg/testutil"
)

// BenchmarkBaseDBQueries is like BaseDBQueries but for benchmarks
type BenchmarkBaseDBQueries struct {
	Queries *gen.Queries
	DB      *sql.DB
	b       *testing.B
	ctx     context.Context
}

// createBaseBenchmarkDB creates database connection for benchmarks
func createBaseBenchmarkDB(ctx context.Context, b *testing.B) *BenchmarkBaseDBQueries {
	db, err := dbtest.GetTestDB(ctx)
	if err != nil {
		b.Fatal(err)
	}
	queries, err := gen.Prepare(ctx, db)
	if err != nil {
		b.Fatal(err)
	}
	return &BenchmarkBaseDBQueries{Queries: queries, b: b, ctx: ctx, DB: db}
}

func (b BenchmarkBaseDBQueries) GetBaseServices() testutil.BaseTestServices {
	queries := b.Queries
	mockLogger := mocklogger.NewMockLogger()
	cs := scollection.New(queries, mockLogger)
	ws := sworkspace.New(queries)
	wus := sworkspacesusers.New(queries)
	us := suser.New(queries)
	return testutil.BaseTestServices{
		DB:  b.DB,
		Cs:  cs,
		Us:  us,
		Ws:  ws,
		Wus: wus,
	}
}

func (b BenchmarkBaseDBQueries) Close() {
	err := b.DB.Close()
	if err != nil {
		b.b.Error(err)
	}
	err = b.Queries.Close()
	if err != nil {
		b.b.Error(err)
	}
}

// setupWorkspaceAndUserBenchmark is the benchmark version of setupWorkspaceAndUser
func setupWorkspaceAndUserBenchmark(b *testing.B, ctx context.Context, base *BenchmarkBaseDBQueries) (wsID, userID idwrap.IDWrap) {
	b.Helper()
	
	wsID = idwrap.NewNow()
	wsuserID := idwrap.NewNow()
	userID = idwrap.NewNow()
	baseCollectionID := idwrap.NewNow()

	services := base.GetBaseServices()
	cs := services.Cs
	ws := services.Ws
	wus := services.Wus
	us := services.Us

	workspaceData := mworkspace.Workspace{
		ID:      wsID,
		Updated: time.Now(),
		Name:    "test",
	}

	err := ws.Create(ctx, &workspaceData)
	if err != nil {
		b.Fatal(err)
	}

	providerID := "test"
	userData := muser.User{
		ID:           userID,
		Email:        "test@dev.tools",
		Password:     []byte("test"),
		ProviderID:   &providerID,
		ProviderType: muser.MagicLink,
		Status:       muser.Active,
	}

	err = us.CreateUser(ctx, &userData)
	if err != nil {
		b.Fatal(err)
	}

	workspaceUserData := mworkspaceuser.WorkspaceUser{
		ID:          wsuserID,
		WorkspaceID: wsID,
		UserID:      userID,
		Role:        mworkspaceuser.RoleAdmin,
	}

	err = wus.CreateWorkspaceUser(ctx, &workspaceUserData)
	if err != nil {
		b.Fatal(err)
	}

	collectionData := mcollection.Collection{
		ID:          baseCollectionID,
		WorkspaceID: wsID,
		Name:        "test",
		Updated:     time.Now(),
	}

	err = cs.CreateCollection(ctx, &collectionData)
	if err != nil {
		b.Fatal(err)
	}

	collectionGet, err := cs.GetCollection(ctx, baseCollectionID)
	if err != nil {
		b.Fatal(err)
	}

	if collectionGet == nil {
		b.Fatal("Collection not found")
	}

	return wsID, userID
}

// setupBenchmarkCollections creates n collections for benchmarking
func setupBenchmarkCollections(tb testing.TB, ctx context.Context, cs *scollection.CollectionService, wsID idwrap.IDWrap, n int) []idwrap.IDWrap {
	tb.Helper()
	
	ids := make([]idwrap.IDWrap, n)
	for i := 0; i < n; i++ {
		id := idwrap.NewNow()
		collection := &mcollection.Collection{
			ID:          id,
			WorkspaceID: wsID,
			Name:        fmt.Sprintf("Collection-%d", i),
			Updated:     time.Now(),
		}
		
		err := cs.CreateCollection(ctx, collection)
		if err != nil {
			tb.Fatalf("failed to create collection %d: %v", i, err)
		}
		
		ids[i] = id
	}
	
	return ids
}

// BenchmarkCollectionTraversal benchmarks GetCollectionsInOrder for different list sizes
func BenchmarkCollectionTraversal(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark in short mode")
	}

	sizes := []int{10, 100, 1000, 10000}
	
	for _, size := range sizes {
		b.Run(fmt.Sprintf("size_%d", size), func(b *testing.B) {
			ctx := context.Background()
			base := createBaseBenchmarkDB(ctx, b)
			defer base.Close()

			mockLogger := mocklogger.NewMockLogger()
			cs := scollection.New(base.Queries, mockLogger)
			
			// Setup workspace and collections
			wsID, _ := setupWorkspaceAndUserBenchmark(b, ctx, base)
			collectionIDs := setupBenchmarkCollections(b, ctx, &cs, wsID, size)
			
			// Verify setup
			if len(collectionIDs) != size {
				b.Fatalf("expected %d collections, got %d", size, len(collectionIDs))
			}
			
			runtime.GC()
			b.ResetTimer()
			
			for i := 0; i < b.N; i++ {
				collections, err := cs.GetCollectionsOrdered(ctx, wsID)
				if err != nil {
					b.Fatalf("failed to get collections in order: %v", err)
				}
				
				// Ensure we got all collections (including the base collection)
				if len(collections) != size+1 {
					b.Fatalf("expected %d collections in result, got %d", size+1, len(collections))
				}
			}
			
			b.StopTimer()
			// Report operations per collection for scalability analysis
			b.ReportMetric(float64(b.N*size)/b.Elapsed().Seconds(), "collections/sec")
		})
	}
}

// BenchmarkCollectionMove benchmarks individual move operations
func BenchmarkCollectionMove(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark in short mode")
	}

	sizes := []int{10, 100, 1000}
	patterns := []struct {
		name string
		desc string
		move func([]idwrap.IDWrap) (itemIdx, targetIdx int)
	}{
		{
			name: "first_to_last",
			desc: "move first item to last position",
			move: func(ids []idwrap.IDWrap) (int, int) {
				return 0, len(ids) - 1
			},
		},
		{
			name: "last_to_first", 
			desc: "move last item to first position",
			move: func(ids []idwrap.IDWrap) (int, int) {
				return len(ids) - 1, 0
			},
		},
		{
			name: "middle_to_middle",
			desc: "move middle item to different middle position",
			move: func(ids []idwrap.IDWrap) (int, int) {
				mid := len(ids) / 2
				return mid, (mid + len(ids)/4) % len(ids)
			},
		},
		{
			name: "random",
			desc: "move random item to random position",
			move: func(ids []idwrap.IDWrap) (int, int) {
				return rand.Intn(len(ids)), rand.Intn(len(ids))
			},
		},
	}
	
	for _, size := range sizes {
		for _, pattern := range patterns {
			b.Run(fmt.Sprintf("size_%d_%s", size, pattern.name), func(b *testing.B) {
				ctx := context.Background()
				base := createBaseBenchmarkDB(ctx, b)
				defer base.Close()

				mockLogger := mocklogger.NewMockLogger()
				cs := scollection.New(base.Queries, mockLogger)
				
				// Setup workspace
				wsID, _ := setupWorkspaceAndUserBenchmark(b, ctx, base)
				
				// Pre-generate move operations to avoid timing the random generation
				type moveOp struct {
					itemID   idwrap.IDWrap
					targetID idwrap.IDWrap
				}
				
				moves := make([]moveOp, b.N)
				for i := 0; i < b.N; i++ {
					// Create fresh collections for each iteration to avoid ordering dependencies
					collectionIDs := setupBenchmarkCollections(b, ctx, &cs, wsID, size)
					
					itemIdx, targetIdx := pattern.move(collectionIDs)
					moves[i] = moveOp{
						itemID:   collectionIDs[itemIdx],
						targetID: collectionIDs[targetIdx],
					}
					
					// Clean up for next iteration
					for _, id := range collectionIDs {
						cs.DeleteCollection(ctx, id)
					}
				}
				
				// Setup final collections for the benchmark
				_ = setupBenchmarkCollections(b, ctx, &cs, wsID, size)
				
				runtime.GC()
				b.ResetTimer()
				
				for i := 0; i < b.N; i++ {
					// Use modulo to cycle through pre-generated moves
					move := moves[i%len(moves)]
					
					err := cs.MoveCollectionAfter(ctx, move.itemID, move.targetID)
					if err != nil {
						b.Fatalf("failed to move collection: %v", err)
					}
				}
				
				b.StopTimer()
				// Report metrics per collection for scalability analysis
				b.ReportMetric(float64(size), "list_size")
				
				// Verify final state is consistent
				_, err := cs.GetCollectionsOrdered(ctx, wsID)
				if err != nil {
					b.Fatalf("collections ordering corrupted after benchmark: %v", err)
				}
			})
		}
	}
}

// BenchmarkBatchReorder benchmarks bulk reordering operations
func BenchmarkBatchReorder(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark in short mode")
	}

	sizes := []int{10, 100, 1000}
	patterns := []struct {
		name string
		desc string
		reorder func([]idwrap.IDWrap) []idwrap.IDWrap
	}{
		{
			name: "reverse",
			desc: "completely reverse the order",
			reorder: func(ids []idwrap.IDWrap) []idwrap.IDWrap {
				reversed := make([]idwrap.IDWrap, len(ids))
				for i := 0; i < len(ids); i++ {
					reversed[i] = ids[len(ids)-1-i]
				}
				return reversed
			},
		},
		{
			name: "shuffle",
			desc: "random shuffle of all items",
			reorder: func(ids []idwrap.IDWrap) []idwrap.IDWrap {
				shuffled := make([]idwrap.IDWrap, len(ids))
				copy(shuffled, ids)
				rand.Shuffle(len(shuffled), func(i, j int) {
					shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
				})
				return shuffled
			},
		},
		{
			name: "rotate",
			desc: "rotate all items by 1 position",
			reorder: func(ids []idwrap.IDWrap) []idwrap.IDWrap {
				if len(ids) <= 1 {
					return ids
				}
				rotated := make([]idwrap.IDWrap, len(ids))
				rotated[0] = ids[len(ids)-1]
				copy(rotated[1:], ids[:len(ids)-1])
				return rotated
			},
		},
	}
	
	for _, size := range sizes {
		for _, pattern := range patterns {
			b.Run(fmt.Sprintf("size_%d_%s", size, pattern.name), func(b *testing.B) {
				ctx := context.Background()
				base := createBaseBenchmarkDB(ctx, b)
				defer base.Close()

				mockLogger := mocklogger.NewMockLogger()
				cs := scollection.New(base.Queries, mockLogger)
				
				// Setup workspace and collections
				wsID, _ := setupWorkspaceAndUserBenchmark(b, ctx, base)
				collectionIDs := setupBenchmarkCollections(b, ctx, &cs, wsID, size)
				
				// Pre-generate reorderings to avoid timing the shuffle/reverse operations
				reorderedLists := make([][]idwrap.IDWrap, b.N)
				for i := 0; i < b.N; i++ {
					reorderedLists[i] = pattern.reorder(collectionIDs)
				}
				
				runtime.GC()
				b.ResetTimer()
				
				for i := 0; i < b.N; i++ {
					err := cs.ReorderCollections(ctx, wsID, reorderedLists[i%len(reorderedLists)])
					if err != nil {
						b.Fatalf("failed to reorder collections: %v", err)
					}
				}
				
				b.StopTimer()
				// Report metrics for scalability analysis
				b.ReportMetric(float64(size), "list_size")
				b.ReportMetric(float64(b.N*size)/b.Elapsed().Seconds(), "items_reordered/sec")
			})
		}
	}
}

// BenchmarkRecursiveCTE benchmarks the WITH RECURSIVE query performance directly
func BenchmarkRecursiveCTE(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark in short mode")
	}

	sizes := []int{10, 100, 1000, 10000}
	
	for _, size := range sizes {
		b.Run(fmt.Sprintf("size_%d", size), func(b *testing.B) {
			ctx := context.Background()
			base := createBaseBenchmarkDB(ctx, b)
			defer base.Close()

			mockLogger := mocklogger.NewMockLogger()
			cs := scollection.New(base.Queries, mockLogger)
			
			// Setup workspace and collections
			wsID, _ := setupWorkspaceAndUserBenchmark(b, ctx, base)
			_ = setupBenchmarkCollections(b, ctx, &cs, wsID, size)
			
			runtime.GC()
			b.ResetTimer()
			
			// Benchmark the raw query performance
			for i := 0; i < b.N; i++ {
				_, err := base.Queries.GetCollectionsInOrder(ctx, gen.GetCollectionsInOrderParams{
					WorkspaceID:   wsID,
					WorkspaceID_2: wsID,
				})
				if err != nil {
					b.Fatalf("failed to execute recursive CTE query: %v", err)
				}
			}
			
			b.StopTimer()
			// Report query performance metrics
			b.ReportMetric(float64(b.N*size)/b.Elapsed().Seconds(), "rows_processed/sec")
		})
	}
}

// BenchmarkCollectionMoveAllocs measures memory allocations during moves
func BenchmarkCollectionMoveAllocs(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark in short mode")
	}

	sizes := []int{10, 100, 1000}
	
	for _, size := range sizes {
		b.Run(fmt.Sprintf("size_%d", size), func(b *testing.B) {
			b.ReportAllocs()
			
			ctx := context.Background()
			base := createBaseBenchmarkDB(ctx, b)
			defer base.Close()

			mockLogger := mocklogger.NewMockLogger()
			cs := scollection.New(base.Queries, mockLogger)
			
			// Setup workspace and collections
			wsID, _ := setupWorkspaceAndUserBenchmark(b, ctx, base)
			collectionIDs := setupBenchmarkCollections(b, ctx, &cs, wsID, size)
			
			runtime.GC()
			b.ResetTimer()
			
			for i := 0; i < b.N; i++ {
				// Move first item to last position
				itemID := collectionIDs[0]
				targetID := collectionIDs[len(collectionIDs)-1]
				
				err := cs.MoveCollectionAfter(ctx, itemID, targetID)
				if err != nil {
					b.Fatalf("failed to move collection: %v", err)
				}
				
				// Move it back to maintain consistent state
				err = cs.MoveCollectionBefore(ctx, itemID, collectionIDs[1])
				if err != nil {
					b.Fatalf("failed to move collection back: %v", err)
				}
			}
			
			b.StopTimer()
			// Report allocation efficiency
			b.ReportMetric(float64(size), "list_size")
		})
	}
}

// BenchmarkTraversalAllocs measures allocations during list traversal
func BenchmarkTraversalAllocs(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark in short mode")
	}

	sizes := []int{10, 100, 1000}
	
	for _, size := range sizes {
		b.Run(fmt.Sprintf("size_%d", size), func(b *testing.B) {
			b.ReportAllocs()
			
			ctx := context.Background()
			base := createBaseBenchmarkDB(ctx, b)
			defer base.Close()

			mockLogger := mocklogger.NewMockLogger()
			cs := scollection.New(base.Queries, mockLogger)
			
			// Setup workspace and collections
			wsID, _ := setupWorkspaceAndUserBenchmark(b, ctx, base)
			_ = setupBenchmarkCollections(b, ctx, &cs, wsID, size)
			
			runtime.GC()
			b.ResetTimer()
			
			for i := 0; i < b.N; i++ {
				collections, err := cs.GetCollectionsOrdered(ctx, wsID)
				if err != nil {
					b.Fatalf("failed to get collections in order: %v", err)
				}
				
				// Touch the results to prevent compiler optimizations
				if len(collections) == 0 {
					b.Fatal("no collections returned")
				}
			}
			
			b.StopTimer()
			b.ReportMetric(float64(size), "list_size")
		})
	}
}

// BenchmarkParallelMoves tests concurrent move operations
func BenchmarkParallelMoves(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark in short mode")
	}

	sizes := []int{10, 100}
	workers := []int{1, 2, 4, 8}
	
	for _, size := range sizes {
		for _, numWorkers := range workers {
			b.Run(fmt.Sprintf("size_%d_workers_%d", size, numWorkers), func(b *testing.B) {
				ctx := context.Background()
				base := createBaseBenchmarkDB(ctx, b)
				defer base.Close()

				mockLogger := mocklogger.NewMockLogger()
				cs := scollection.New(base.Queries, mockLogger)
				
				// Setup workspace and collections
				wsID, _ := setupWorkspaceAndUserBenchmark(b, ctx, base)
				collectionIDs := setupBenchmarkCollections(b, ctx, &cs, wsID, size)
				
				runtime.GC()
				b.ResetTimer()
				
				// Run concurrent moves
				var wg sync.WaitGroup
				errChan := make(chan error, numWorkers)
				
				opsPerWorker := b.N / numWorkers
				if opsPerWorker == 0 {
					opsPerWorker = 1
				}
				
				for w := 0; w < numWorkers; w++ {
					wg.Add(1)
					go func(workerID int) {
						defer wg.Done()
						
						for i := 0; i < opsPerWorker; i++ {
							// Use different collections for each worker to minimize contention
							itemIdx := (workerID + i) % len(collectionIDs)
							targetIdx := (itemIdx + 1) % len(collectionIDs)
							
							err := cs.MoveCollectionAfter(ctx, collectionIDs[itemIdx], collectionIDs[targetIdx])
							if err != nil {
								select {
								case errChan <- err:
								default:
								}
								return
							}
						}
					}(w)
				}
				
				wg.Wait()
				close(errChan)
				
				// Check for errors
				if err := <-errChan; err != nil {
					b.Fatalf("parallel move failed: %v", err)
				}
				
				b.StopTimer()
				b.ReportMetric(float64(numWorkers), "workers")
				b.ReportMetric(float64(size), "list_size")
			})
		}
	}
}

// BenchmarkParallelTraversal tests concurrent list traversals
func BenchmarkParallelTraversal(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark in short mode")
	}

	sizes := []int{100, 1000}
	workers := []int{1, 2, 4, 8, 16}
	
	for _, size := range sizes {
		for _, numWorkers := range workers {
			b.Run(fmt.Sprintf("size_%d_workers_%d", size, numWorkers), func(b *testing.B) {
				ctx := context.Background()
				base := createBaseBenchmarkDB(ctx, b)
				defer base.Close()

				mockLogger := mocklogger.NewMockLogger()
				cs := scollection.New(base.Queries, mockLogger)
				
				// Setup workspace and collections
				wsID, _ := setupWorkspaceAndUserBenchmark(b, ctx, base)
				_ = setupBenchmarkCollections(b, ctx, &cs, wsID, size)
				
				runtime.GC()
				b.ResetTimer()
				
				// Run concurrent traversals
				var wg sync.WaitGroup
				errChan := make(chan error, numWorkers)
				
				opsPerWorker := b.N / numWorkers
				if opsPerWorker == 0 {
					opsPerWorker = 1
				}
				
				for w := 0; w < numWorkers; w++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						
						for i := 0; i < opsPerWorker; i++ {
							collections, err := cs.GetCollectionsOrdered(ctx, wsID)
							if err != nil {
								select {
								case errChan <- err:
								default:
								}
								return
							}
							
							// Touch results to prevent optimization
							if len(collections) != size+1 {
								select {
								case errChan <- fmt.Errorf("expected %d collections, got %d", size+1, len(collections)):
								default:
								}
								return
							}
						}
					}()
				}
				
				wg.Wait()
				close(errChan)
				
				// Check for errors
				if err := <-errChan; err != nil {
					b.Fatalf("parallel traversal failed: %v", err)
				}
				
				b.StopTimer()
				b.ReportMetric(float64(numWorkers), "workers")
				b.ReportMetric(float64(size), "list_size")
				b.ReportMetric(float64(b.N*size)/b.Elapsed().Seconds(), "collections_read/sec")
			})
		}
	}
}

// BenchmarkRepositoryOperations benchmarks repository-level operations directly
func BenchmarkRepositoryOperations(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark in short mode")
	}

	sizes := []int{10, 100, 1000}
	
	for _, size := range sizes {
		b.Run(fmt.Sprintf("UpdatePosition_size_%d", size), func(b *testing.B) {
			ctx := context.Background()
			base := createBaseBenchmarkDB(ctx, b)
			defer base.Close()

			mockLogger := mocklogger.NewMockLogger()
			cs := scollection.New(base.Queries, mockLogger)
			
			// Setup workspace and collections
			wsID, _ := setupWorkspaceAndUserBenchmark(b, ctx, base)
			collectionIDs := setupBenchmarkCollections(b, ctx, &cs, wsID, size)
			
			repo := scollection.NewCollectionMovableRepository(base.Queries)
			
			runtime.GC()
			b.ResetTimer()
			
			for i := 0; i < b.N; i++ {
				// Move first collection to different positions
				newPosition := i % size
				err := repo.UpdatePosition(ctx, nil, collectionIDs[0], movable.CollectionListTypeCollections, newPosition)
				if err != nil {
					b.Fatalf("failed to update position: %v", err)
				}
			}
			
			b.StopTimer()
			b.ReportMetric(float64(size), "list_size")
		})
		
		b.Run(fmt.Sprintf("GetMaxPosition_size_%d", size), func(b *testing.B) {
			ctx := context.Background()
			base := createBaseBenchmarkDB(ctx, b)
			defer base.Close()

			mockLogger := mocklogger.NewMockLogger()
			cs := scollection.New(base.Queries, mockLogger)
			
			// Setup workspace and collections
			wsID, _ := setupWorkspaceAndUserBenchmark(b, ctx, base)
			_ = setupBenchmarkCollections(b, ctx, &cs, wsID, size)
			
			repo := scollection.NewCollectionMovableRepository(base.Queries)
			
			runtime.GC()
			b.ResetTimer()
			
			for i := 0; i < b.N; i++ {
				maxPos, err := repo.GetMaxPosition(ctx, wsID, movable.CollectionListTypeCollections)
				if err != nil {
					b.Fatalf("failed to get max position: %v", err)
				}
				if maxPos < 0 {
					b.Fatal("invalid max position")
				}
			}
			
			b.StopTimer()
			b.ReportMetric(float64(size), "list_size")
		})
		
		b.Run(fmt.Sprintf("GetItemsByParent_size_%d", size), func(b *testing.B) {
			ctx := context.Background()
			base := createBaseBenchmarkDB(ctx, b)
			defer base.Close()

			mockLogger := mocklogger.NewMockLogger()
			cs := scollection.New(base.Queries, mockLogger)
			
			// Setup workspace and collections
			wsID, _ := setupWorkspaceAndUserBenchmark(b, ctx, base)
			_ = setupBenchmarkCollections(b, ctx, &cs, wsID, size)
			
			repo := scollection.NewCollectionMovableRepository(base.Queries)
			
			runtime.GC()
			b.ResetTimer()
			
			for i := 0; i < b.N; i++ {
				items, err := repo.GetItemsByParent(ctx, wsID, movable.CollectionListTypeCollections)
				if err != nil {
					b.Fatalf("failed to get items by parent: %v", err)
				}
				if len(items) != size+1 { // +1 for base collection
					b.Fatalf("expected %d items, got %d", size+1, len(items))
				}
			}
			
			b.StopTimer()
			b.ReportMetric(float64(size), "list_size")
		})
	}
}

/*
Benchmark Results Interpretation Guide:

1. **Collection Traversal Benchmarks**:
   - Should show O(n) performance with collection count
   - Memory allocations should be proportional to result size
   - Operations/sec should remain stable as list size grows

2. **Move Operation Benchmarks**:
   - Individual moves should be O(1) for pointer updates
   - Different move patterns test worst-case scenarios:
     * first_to_last: Tests head removal + tail insertion
     * last_to_first: Tests tail removal + head insertion  
     * middle_to_middle: Tests arbitrary position changes
     * random: Tests average-case performance

3. **Batch Reorder Benchmarks**:
   - Should show better throughput than individual moves
   - Reverse pattern tests worst-case batch operations
   - Shuffle tests random reordering overhead
   - Rotate tests minimal change operations

4. **Recursive CTE Benchmarks**:
   - Raw SQL query performance without Go overhead
   - Should show linear scaling with collection count
   - Rows/sec metric indicates database engine efficiency

5. **Memory Allocation Benchmarks**:
   - Identifies memory overhead per operation
   - Lower allocs/op indicates better memory efficiency
   - Use to detect memory leaks or excessive allocations

6. **Parallel Operation Benchmarks**:
   - Tests concurrent safety and throughput
   - Move operations may show contention at higher worker counts
   - Traversal should scale linearly with workers (read-only)
   - Identifies optimal concurrency levels

7. **Repository Operation Benchmarks**:
   - Low-level operation performance without service overhead
   - Direct database interaction timing
   - Baseline for comparing service-level performance

Expected Performance Characteristics:
- Traversal: O(n) time, O(n) memory
- Single moves: O(1) time for pointer updates, O(n) for position-based moves
- Batch operations: Better amortized performance than individual operations
- Memory usage: Should be stable with no leaks
- Concurrency: Read operations should scale, writes may have contention

Use these benchmarks to:
- Identify performance regressions
- Optimize database queries and indices
- Tune concurrent operation limits  
- Validate scalability assumptions
- Guide architectural decisions
*/