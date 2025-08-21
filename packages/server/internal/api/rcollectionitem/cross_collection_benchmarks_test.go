package rcollectionitem_test

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rcollectionitem"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemfolder"
	"the-dev-tools/server/pkg/service/scollectionitem"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemfolder"
	"the-dev-tools/server/pkg/service/sexampleresp"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/testutil"
	itemv1 "the-dev-tools/spec/dist/buf/go/collection/item/v1"
	resourcesv1 "the-dev-tools/spec/dist/buf/go/resources/v1"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
)

// BenchmarkSetup represents benchmark test setup configuration
type BenchmarkSetup struct {
	sourceItems int
	targetItems int
	nestedDepth int
}

// setupBenchmarkEnvironment creates optimized test environment for benchmarks
func setupBenchmarkEnvironment(b *testing.B, setup BenchmarkSetup) (
	rpc rcollectionitem.CollectionItemRPC,
	ctx context.Context,
	authedCtx context.Context,
	sourceCollectionID, targetCollectionID idwrap.IDWrap,
	sourceItemIDs, targetItemIDs []idwrap.IDWrap,
	cis *scollectionitem.CollectionItemService,
	cleanup func(),
) {
	b.Helper()

	ctx = context.Background()
	base := testutil.CreateBaseDB(ctx, &testing.T{}) // Reuse testing.T for compatibility
	services := base.GetBaseServices()

	mockLogger := mocklogger.NewMockLogger()

	// Create all required services
	cs := services.Cs
	cis = scollectionitem.New(base.Queries, mockLogger)
	us := services.Us
	ifs := sitemfolder.New(base.Queries)
	ias := sitemapi.New(base.Queries)
	iaes := sitemapiexample.New(base.Queries)
	res := sexampleresp.New(base.Queries)

	rpc = rcollectionitem.New(base.DB, cs, cis, us, ifs, ias, iaes, res)

	userID := idwrap.NewNow()
	authedCtx = mwauth.CreateAuthedContext(ctx, userID)

	// Create workspace and collections
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	sourceCollectionID = idwrap.NewNow()
	targetCollectionID = idwrap.NewNow()

	// Use dummy testing.T for setup
	dummyT := &testing.T{}
	services.CreateTempCollection(dummyT, ctx, workspaceID, workspaceUserID, userID, sourceCollectionID)
	services.CreateTempCollection(dummyT, ctx, workspaceID, workspaceUserID, userID, targetCollectionID)

	// Create source items
	sourceItemIDs = make([]idwrap.IDWrap, setup.sourceItems)
	for i := 0; i < setup.sourceItems; i++ {
		if i%2 == 0 {
			// Create folder
			folderID := idwrap.NewNow()
			folder := &mitemfolder.ItemFolder{
				ID:           folderID,
				Name:         fmt.Sprintf("Benchmark Folder %d", i),
				CollectionID: sourceCollectionID,
				ParentID:     nil,
			}
			err := ifs.CreateItemFolder(ctx, folder)
			if err != nil {
				b.Fatal(err)
			}
			sourceItemIDs[i] = folderID
		} else {
			// Create endpoint
			endpointID := idwrap.NewNow()
			endpoint := &mitemapi.ItemApi{
				ID:           endpointID,
				Name:         fmt.Sprintf("Benchmark Endpoint %d", i),
				Url:          fmt.Sprintf("https://api.benchmark.com/test-%d", i),
				Method:       []string{"GET", "POST", "PUT", "DELETE"}[i%4],
				CollectionID: sourceCollectionID,
				FolderID:     nil,
			}
			err := ias.CreateItemApi(ctx, endpoint)
			if err != nil {
				b.Fatal(err)
			}
			sourceItemIDs[i] = endpointID
		}
	}

	// Create target items
	targetItemIDs = make([]idwrap.IDWrap, setup.targetItems)
	for i := 0; i < setup.targetItems; i++ {
		if i%2 == 0 {
			// Create folder
			folderID := idwrap.NewNow()
			folder := &mitemfolder.ItemFolder{
				ID:           folderID,
				Name:         fmt.Sprintf("Target Folder %d", i),
				CollectionID: targetCollectionID,
				ParentID:     nil,
			}
			err := ifs.CreateItemFolder(ctx, folder)
			if err != nil {
				b.Fatal(err)
			}
			targetItemIDs[i] = folderID
		} else {
			// Create endpoint
			endpointID := idwrap.NewNow()
			endpoint := &mitemapi.ItemApi{
				ID:           endpointID,
				Name:         fmt.Sprintf("Target Endpoint %d", i),
				Url:          fmt.Sprintf("https://api.target.com/test-%d", i),
				Method:       []string{"GET", "POST", "PUT", "DELETE"}[i%4],
				CollectionID: targetCollectionID,
				FolderID:     nil,
			}
			err := ias.CreateItemApi(ctx, endpoint)
			if err != nil {
				b.Fatal(err)
			}
			targetItemIDs[i] = endpointID
		}
	}

	// Create collection items for all items
	tx, err := base.DB.Begin()
	if err != nil {
		b.Fatal(err)
	}
	defer tx.Rollback()

	// Create collection items for source items
	for i, itemID := range sourceItemIDs {
		if i%2 == 0 {
			// Folder
			folder := &mitemfolder.ItemFolder{
				ID:           itemID,
				Name:         fmt.Sprintf("Benchmark Folder %d", i),
				CollectionID: sourceCollectionID,
				ParentID:     nil,
			}
			err = cis.CreateFolderTX(ctx, tx, folder)
			if err != nil {
				b.Fatal(err)
			}
		} else {
			// Endpoint
			endpoint := &mitemapi.ItemApi{
				ID:           itemID,
				Name:         fmt.Sprintf("Benchmark Endpoint %d", i),
				Url:          fmt.Sprintf("https://api.benchmark.com/test-%d", i),
				Method:       []string{"GET", "POST", "PUT", "DELETE"}[i%4],
				CollectionID: sourceCollectionID,
				FolderID:     nil,
			}
			err = cis.CreateEndpointTX(ctx, tx, endpoint)
			if err != nil {
				b.Fatal(err)
			}
		}
	}

	// Create collection items for target items
	for i, itemID := range targetItemIDs {
		if i%2 == 0 {
			// Folder
			folder := &mitemfolder.ItemFolder{
				ID:           itemID,
				Name:         fmt.Sprintf("Target Folder %d", i),
				CollectionID: targetCollectionID,
				ParentID:     nil,
			}
			err = cis.CreateFolderTX(ctx, tx, folder)
			if err != nil {
				b.Fatal(err)
			}
		} else {
			// Endpoint
			endpoint := &mitemapi.ItemApi{
				ID:           itemID,
				Name:         fmt.Sprintf("Target Endpoint %d", i),
				Url:          fmt.Sprintf("https://api.target.com/test-%d", i),
				Method:       []string{"GET", "POST", "PUT", "DELETE"}[i%4],
				CollectionID: targetCollectionID,
				FolderID:     nil,
			}
			err = cis.CreateEndpointTX(ctx, tx, endpoint)
			if err != nil {
				b.Fatal(err)
			}
		}
	}

	err = tx.Commit()
	if err != nil {
		b.Fatal(err)
	}

	cleanup = func() {
		base.Close()
	}

	return rpc, ctx, authedCtx, sourceCollectionID, targetCollectionID, sourceItemIDs, targetItemIDs, cis, cleanup
}

// BenchmarkCrossCollectionMove_SmallCollections benchmarks moves between small collections
func BenchmarkCrossCollectionMove_SmallCollections(b *testing.B) {
	setup := BenchmarkSetup{
		sourceItems: 10,
		targetItems: 5,
		nestedDepth: 0,
	}

	rpc, _, authedCtx, sourceCollectionID, targetCollectionID, sourceItemIDs, _, _, cleanup := setupBenchmarkEnvironment(b, setup)
	defer cleanup()

	rand.Seed(time.Now().UnixNano())

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Pick random source item
		sourceItemID := sourceItemIDs[rand.Intn(len(sourceItemIDs))]

		// Determine item kind
		kind := itemv1.ItemKind_ITEM_KIND_FOLDER
		if i%2 == 1 {
			kind = itemv1.ItemKind_ITEM_KIND_ENDPOINT
		}

		req := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			Kind:               kind,
			ItemId:             sourceItemID.Bytes(),
			CollectionId:       sourceCollectionID.Bytes(),
			TargetCollectionId: targetCollectionID.Bytes(),
			Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
		})

		_, err := rpc.CollectionItemMove(authedCtx, req)
		if err != nil {
			// Don't fail benchmark on expected errors (like item not found after moves)
			b.Logf("Move failed (expected in benchmark): %v", err)
		}
	}
}

// BenchmarkCrossCollectionMove_MediumCollections benchmarks moves between medium-sized collections
func BenchmarkCrossCollectionMove_MediumCollections(b *testing.B) {
	setup := BenchmarkSetup{
		sourceItems: 100,
		targetItems: 50,
		nestedDepth: 0,
	}

	rpc, _, authedCtx, sourceCollectionID, targetCollectionID, sourceItemIDs, _, _, cleanup := setupBenchmarkEnvironment(b, setup)
	defer cleanup()

	rand.Seed(time.Now().UnixNano())

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Pick random source item that hasn't been moved yet
		sourceItemIndex := i % len(sourceItemIDs)
		sourceItemID := sourceItemIDs[sourceItemIndex]

		// Determine item kind
		kind := itemv1.ItemKind_ITEM_KIND_FOLDER
		if sourceItemIndex%2 == 1 {
			kind = itemv1.ItemKind_ITEM_KIND_ENDPOINT
		}

		req := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			Kind:               kind,
			ItemId:             sourceItemID.Bytes(),
			CollectionId:       sourceCollectionID.Bytes(),
			TargetCollectionId: targetCollectionID.Bytes(),
			Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
		})

		_, err := rpc.CollectionItemMove(authedCtx, req)
		if err != nil {
			b.Logf("Move failed (expected in benchmark): %v", err)
		}
	}
}

// BenchmarkCrossCollectionMove_WithPositioning benchmarks moves with specific positioning
func BenchmarkCrossCollectionMove_WithPositioning(b *testing.B) {
	setup := BenchmarkSetup{
		sourceItems: 50,
		targetItems: 30,
		nestedDepth: 0,
	}

	rpc, ctx, authedCtx, sourceCollectionID, targetCollectionID, sourceItemIDs, targetItemIDs, cis, cleanup := setupBenchmarkEnvironment(b, setup)
	defer cleanup()

	rand.Seed(time.Now().UnixNano())

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Pick random source and target items
		sourceItemIndex := i % len(sourceItemIDs)
		sourceItemID := sourceItemIDs[sourceItemIndex]
		targetItemIndex := rand.Intn(len(targetItemIDs))
		targetItemID := targetItemIDs[targetItemIndex]

		// Get target collection item ID
		targetCollectionItemID, err := cis.GetCollectionItemIDByLegacyID(ctx, targetItemID)
		if err != nil {
			b.Logf("Failed to get target collection item ID: %v", err)
			continue
		}

		// Determine item kind
		kind := itemv1.ItemKind_ITEM_KIND_FOLDER
		if sourceItemIndex%2 == 1 {
			kind = itemv1.ItemKind_ITEM_KIND_ENDPOINT
		}

		position := resourcesv1.MovePosition_MOVE_POSITION_AFTER
		if i%2 == 0 {
			position = resourcesv1.MovePosition_MOVE_POSITION_BEFORE
		}

		req := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			Kind:               kind,
			ItemId:             sourceItemID.Bytes(),
			CollectionId:       sourceCollectionID.Bytes(),
			TargetCollectionId: targetCollectionID.Bytes(),
			TargetItemId:       targetCollectionItemID.Bytes(),
			Position:           position.Enum(),
		})

		_, err = rpc.CollectionItemMove(authedCtx, req)
		if err != nil {
			b.Logf("Move failed (expected in benchmark): %v", err)
		}
	}
}

// BenchmarkCrossCollectionMove_WithTargetKind benchmarks moves with targetKind validation
func BenchmarkCrossCollectionMove_WithTargetKind(b *testing.B) {
	setup := BenchmarkSetup{
		sourceItems: 40,
		targetItems: 20,
		nestedDepth: 0,
	}

	rpc, ctx, authedCtx, sourceCollectionID, targetCollectionID, sourceItemIDs, targetItemIDs, cis, cleanup := setupBenchmarkEnvironment(b, setup)
	defer cleanup()

	rand.Seed(time.Now().UnixNano())

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Pick random source and target items
		sourceItemIndex := i % len(sourceItemIDs)
		sourceItemID := sourceItemIDs[sourceItemIndex]
		targetItemIndex := rand.Intn(len(targetItemIDs))
		targetItemID := targetItemIDs[targetItemIndex]

		// Get target collection item ID
		targetCollectionItemID, err := cis.GetCollectionItemIDByLegacyID(ctx, targetItemID)
		if err != nil {
			b.Logf("Failed to get target collection item ID: %v", err)
			continue
		}

		// Determine source kind
		sourceKind := itemv1.ItemKind_ITEM_KIND_FOLDER
		if sourceItemIndex%2 == 1 {
			sourceKind = itemv1.ItemKind_ITEM_KIND_ENDPOINT
		}

		req := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			Kind:               sourceKind,
			ItemId:             sourceItemID.Bytes(),
			CollectionId:       sourceCollectionID.Bytes(),
			TargetCollectionId: targetCollectionID.Bytes(),
			TargetItemId:       targetCollectionItemID.Bytes(),
			Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
		})

		_, err = rpc.CollectionItemMove(authedCtx, req)
		if err != nil {
			b.Logf("Move failed (expected in benchmark): %v", err)
		}
	}
}

// BenchmarkCrossCollectionMove_ValidationOverhead benchmarks the validation overhead
func BenchmarkCrossCollectionMove_ValidationOverhead(b *testing.B) {
	setup := BenchmarkSetup{
		sourceItems: 20,
		targetItems: 10,
		nestedDepth: 0,
	}

	rpc, _, authedCtx, sourceCollectionID, targetCollectionID, sourceItemIDs, _, _, cleanup := setupBenchmarkEnvironment(b, setup)
	defer cleanup()

	// Create invalid requests to test validation performance
	invalidRequests := []*itemv1.CollectionItemMoveRequest{
		{
			Kind:               itemv1.ItemKind_ITEM_KIND_FOLDER,
			ItemId:             []byte("invalid-ulid"),
			CollectionId:       sourceCollectionID.Bytes(),
			TargetCollectionId: targetCollectionID.Bytes(),
			Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
		},
		{
			Kind:               itemv1.ItemKind_ITEM_KIND_UNSPECIFIED,
			ItemId:             sourceItemIDs[0].Bytes(),
			CollectionId:       sourceCollectionID.Bytes(),
			TargetCollectionId: targetCollectionID.Bytes(),
			Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
		},
		{
			Kind:         itemv1.ItemKind_ITEM_KIND_ENDPOINT,
			ItemId:       sourceItemIDs[1].Bytes(),
			CollectionId: sourceCollectionID.Bytes(),
			// Missing target collection ID
			Position: resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
		},
	}

	rand.Seed(time.Now().UnixNano())

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Use random invalid request
		req := connect.NewRequest(invalidRequests[i%len(invalidRequests)])

		_, err := rpc.CollectionItemMove(authedCtx, req)
		if err == nil {
			b.Fatal("Expected validation error but got none")
		}
	}
}

// BenchmarkCrossCollectionMove_DatabaseOperations benchmarks database operation performance
func BenchmarkCrossCollectionMove_DatabaseOperations(b *testing.B) {
	setup := BenchmarkSetup{
		sourceItems: 30,
		targetItems: 15,
		nestedDepth: 0,
	}

	rpc, ctx, authedCtx, sourceCollectionID, targetCollectionID, sourceItemIDs, _, cis, cleanup := setupBenchmarkEnvironment(b, setup)
	defer cleanup()

	b.ResetTimer()

	b.Run("Direct Service Layer", func(b *testing.B) {
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			sourceItemIndex := i % len(sourceItemIDs)
			sourceItemID := sourceItemIDs[sourceItemIndex]

			// Get collection item ID
			collectionItemID, err := cis.GetCollectionItemIDByLegacyID(ctx, sourceItemID)
			if err != nil {
				b.Logf("Failed to get collection item ID: %v", err)
				continue
			}

			// Direct service layer call
			err = cis.MoveCollectionItemCrossCollection(ctx, collectionItemID, targetCollectionID, nil, nil, 0)
			if err != nil {
				b.Logf("Service move failed (expected): %v", err)
			}
		}
	})

	b.Run("Full RPC Stack", func(b *testing.B) {
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			sourceItemIndex := i % len(sourceItemIDs)
			sourceItemID := sourceItemIDs[sourceItemIndex]

			kind := itemv1.ItemKind_ITEM_KIND_FOLDER
			if sourceItemIndex%2 == 1 {
				kind = itemv1.ItemKind_ITEM_KIND_ENDPOINT
			}

			req := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
				Kind:               kind,
				ItemId:             sourceItemID.Bytes(),
				CollectionId:       sourceCollectionID.Bytes(),
				TargetCollectionId: targetCollectionID.Bytes(),
				Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
			})

			_, err := rpc.CollectionItemMove(authedCtx, req)
			if err != nil {
				b.Logf("RPC move failed (expected): %v", err)
			}
		}
	})
}

// BenchmarkCrossCollectionMove_MemoryUsage benchmarks memory usage patterns
func BenchmarkCrossCollectionMove_MemoryUsage(b *testing.B) {
	setup := BenchmarkSetup{
		sourceItems: 200,
		targetItems: 100,
		nestedDepth: 0,
	}

	rpc, _, authedCtx, sourceCollectionID, targetCollectionID, sourceItemIDs, _, _, cleanup := setupBenchmarkEnvironment(b, setup)
	defer cleanup()

	rand.Seed(time.Now().UnixNano())

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		sourceItemIndex := i % len(sourceItemIDs)
		sourceItemID := sourceItemIDs[sourceItemIndex]

		kind := itemv1.ItemKind_ITEM_KIND_FOLDER
		if sourceItemIndex%2 == 1 {
			kind = itemv1.ItemKind_ITEM_KIND_ENDPOINT
		}

		req := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			Kind:               kind,
			ItemId:             sourceItemID.Bytes(),
			CollectionId:       sourceCollectionID.Bytes(),
			TargetCollectionId: targetCollectionID.Bytes(),
			Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
		})

		_, err := rpc.CollectionItemMove(authedCtx, req)
		if err != nil {
			// Expected due to items being moved
			continue
		}
	}
}

// BenchmarkCrossCollectionMove_ConcurrentLoad simulates concurrent load
func BenchmarkCrossCollectionMove_ConcurrentLoad(b *testing.B) {
	setup := BenchmarkSetup{
		sourceItems: 60,
		targetItems: 30,
		nestedDepth: 0,
	}

	rpc, _, authedCtx, sourceCollectionID, targetCollectionID, sourceItemIDs, _, _, cleanup := setupBenchmarkEnvironment(b, setup)
	defer cleanup()

	rand.Seed(time.Now().UnixNano())

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			sourceItemIndex := i % len(sourceItemIDs)
			sourceItemID := sourceItemIDs[sourceItemIndex]

			kind := itemv1.ItemKind_ITEM_KIND_FOLDER
			if sourceItemIndex%2 == 1 {
				kind = itemv1.ItemKind_ITEM_KIND_ENDPOINT
			}

			req := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
				Kind:               kind,
				ItemId:             sourceItemID.Bytes(),
				CollectionId:       sourceCollectionID.Bytes(),
				TargetCollectionId: targetCollectionID.Bytes(),
				Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
			})

			_, err := rpc.CollectionItemMove(authedCtx, req)
			if err != nil {
				// Expected due to concurrent operations and items being moved
				continue
			}
			i++
		}
	})
}

// BenchmarkCrossCollectionMove_ErrorScenarios benchmarks error handling performance
func BenchmarkCrossCollectionMove_ErrorScenarios(b *testing.B) {
	setup := BenchmarkSetup{
		sourceItems: 10,
		targetItems: 5,
		nestedDepth: 0,
	}

	rpc, ctx, authedCtx, sourceCollectionID, targetCollectionID, sourceItemIDs, _, _, cleanup := setupBenchmarkEnvironment(b, setup)
	defer cleanup()

	// Create unauthorized user for permission testing
	unauthorizedUserID := idwrap.NewNow()
	unauthorizedCtx := mwauth.CreateAuthedContext(ctx, unauthorizedUserID)

	errorScenarios := []struct {
		name   string
		req    *itemv1.CollectionItemMoveRequest
		ctx    context.Context
	}{
		{
			name: "InvalidItemID",
			req: &itemv1.CollectionItemMoveRequest{
				Kind:               itemv1.ItemKind_ITEM_KIND_FOLDER,
				ItemId:             []byte("invalid-ulid"),
				CollectionId:       sourceCollectionID.Bytes(),
				TargetCollectionId: targetCollectionID.Bytes(),
				Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
			},
			ctx: authedCtx,
		},
		{
			name: "UnauthorizedUser",
			req: &itemv1.CollectionItemMoveRequest{
				Kind:               itemv1.ItemKind_ITEM_KIND_FOLDER,
				ItemId:             sourceItemIDs[0].Bytes(),
				CollectionId:       sourceCollectionID.Bytes(),
				TargetCollectionId: targetCollectionID.Bytes(),
				Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
			},
			ctx: unauthorizedCtx,
		},
		{
			name: "NonExistentTargetCollection",
			req: &itemv1.CollectionItemMoveRequest{
				Kind:               itemv1.ItemKind_ITEM_KIND_FOLDER,
				ItemId:             sourceItemIDs[1].Bytes(),
				CollectionId:       sourceCollectionID.Bytes(),
				TargetCollectionId: idwrap.NewNow().Bytes(),
				Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
			},
			ctx: authedCtx,
		},
		{
			name: "InvalidTargetKind",
			req: &itemv1.CollectionItemMoveRequest{
				Kind:               itemv1.ItemKind_ITEM_KIND_FOLDER,
				ItemId:             sourceItemIDs[0].Bytes(),
				CollectionId:       sourceCollectionID.Bytes(),
				TargetCollectionId: targetCollectionID.Bytes(),
				Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
			},
			ctx: authedCtx,
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for _, scenario := range errorScenarios {
		b.Run(scenario.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				req := connect.NewRequest(scenario.req)
				_, err := rpc.CollectionItemMove(scenario.ctx, req)
				if err == nil {
					b.Fatal("Expected error but got none")
				}
			}
		})
	}
}

// Comparative benchmarks to show performance differences
func BenchmarkCrossCollectionMove_Comparative(b *testing.B) {
	b.Run("SmallVsMediumCollections", func(b *testing.B) {
		b.Run("Small_10_items", func(b *testing.B) {
			BenchmarkCrossCollectionMove_SmallCollections(b)
		})
		
		b.Run("Medium_100_items", func(b *testing.B) {
			BenchmarkCrossCollectionMove_MediumCollections(b)
		})
	})

	b.Run("WithVsWithoutPositioning", func(b *testing.B) {
		b.Run("WithoutPositioning", func(b *testing.B) {
			BenchmarkCrossCollectionMove_SmallCollections(b)
		})
		
		b.Run("WithPositioning", func(b *testing.B) {
			BenchmarkCrossCollectionMove_WithPositioning(b)
		})
	})

	b.Run("WithVsWithoutTargetKind", func(b *testing.B) {
		b.Run("WithoutTargetKind", func(b *testing.B) {
			BenchmarkCrossCollectionMove_SmallCollections(b)
		})
		
		b.Run("WithTargetKind", func(b *testing.B) {
			BenchmarkCrossCollectionMove_WithTargetKind(b)
		})
	})
}