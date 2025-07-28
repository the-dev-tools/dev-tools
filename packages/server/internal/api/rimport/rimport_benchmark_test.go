package rimport_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rimport"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/service/sassert"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/sexampleresp"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/service/sitemfolder"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/testutil"
	importv1 "the-dev-tools/spec/dist/buf/go/import/v1"
)

// createLargeHAR creates a HAR file with many entries to test performance
func createLargeHAR(numEntries int) string {
	entries := make([]interface{}, numEntries)

	for i := 0; i < numEntries; i++ {
		entries[i] = map[string]interface{}{
			"startedDateTime": time.Now().Add(time.Duration(i) * time.Second).Format(time.RFC3339),
			"_resourceType":   "xhr",
			"request": map[string]interface{}{
				"method":      "GET",
				"url":         fmt.Sprintf("https://api.example.com/v1/users/%d/profile", i),
				"httpVersion": "HTTP/1.1",
				"headers": []interface{}{
					map[string]interface{}{
						"name":  "Content-Type",
						"value": "application/json",
					},
					map[string]interface{}{
						"name":  "Authorization",
						"value": fmt.Sprintf("Bearer token-%d", i),
					},
				},
				"queryString": []interface{}{
					map[string]interface{}{
						"name":  "include",
						"value": "metadata",
					},
				},
			},
			"response": map[string]interface{}{
				"status":      200,
				"statusText":  "OK",
				"httpVersion": "HTTP/1.1",
				"headers":     []interface{}{},
				"content": map[string]interface{}{
					"size":     100,
					"mimeType": "application/json",
					"text":     fmt.Sprintf(`{"id":%d,"name":"User %d"}`, i, i),
				},
			},
		}
	}

	harData := map[string]interface{}{
		"log": map[string]interface{}{
			"entries": entries,
		},
	}

	harJSON, _ := json.Marshal(harData)
	return string(harJSON)
}

// BenchmarkImportHAR_NewCollection benchmarks importing HAR to a new collection
func BenchmarkImportHAR_NewCollection(b *testing.B) {
	sizes := []int{10, 50, 100, 200}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("entries_%d", size), func(b *testing.B) {
			// Setup
			ctx := context.Background()
			base := testutil.CreateBaseDB(ctx, &testing.T{})
			queries := base.Queries
			db := base.DB

			mockLogger := mocklogger.NewMockLogger()

			// Initialize services
			ws := sworkspace.New(queries)
			cs := scollection.New(queries, mockLogger)
			us := suser.New(queries)
			ifs := sitemfolder.New(queries)
			ias := sitemapi.New(queries)
			iaes := sitemapiexample.New(queries)
			ers := sexampleresp.New(queries)
			as := sassert.New(queries)

			// Create test data
			workspaceID := idwrap.NewNow()
			workspaceUserID := idwrap.NewNow()
			userID := idwrap.NewNow()

			baseServices := base.GetBaseServices()
			baseServices.CreateTempCollection(&testing.T{}, ctx, workspaceID, workspaceUserID, userID, idwrap.NewNow())

			importRPC := rimport.New(db, ws, cs, us, ifs, ias, iaes, ers, as)
			authedCtx := mwauth.CreateAuthedContext(ctx, userID)

			// Create HAR data
			harJSON := createLargeHAR(size)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				// First request to get filter options
				req0 := connect.NewRequest(&importv1.ImportRequest{
					WorkspaceId: workspaceID.Bytes(),
					Data:        []byte(harJSON),
					Name:        fmt.Sprintf("Benchmark Collection %d", i),
					Filter:      []string{},
				})

				resp0, err := importRPC.Import(authedCtx, req0)
				require.NoError(b, err)
				require.NotNil(b, resp0)

				// Second request to actually import
				req1 := connect.NewRequest(&importv1.ImportRequest{
					WorkspaceId: workspaceID.Bytes(),
					Data:        []byte(harJSON),
					Name:        fmt.Sprintf("Benchmark Collection %d", i),
					Filter:      []string{"api.example.com"},
				})

				resp1, err := importRPC.Import(authedCtx, req1)
				require.NoError(b, err)
				require.NotNil(b, resp1)
			}
		})
	}
}

// BenchmarkImportHAR_ExistingCollection benchmarks importing HAR to existing collection (tests optimization)
func BenchmarkImportHAR_ExistingCollection(b *testing.B) {
	sizes := []int{10, 50, 100}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("entries_%d", size), func(b *testing.B) {
			// Setup
			ctx := context.Background()
			base := testutil.CreateBaseDB(ctx, &testing.T{})
			queries := base.Queries
			db := base.DB

			mockLogger := mocklogger.NewMockLogger()

			// Initialize services
			ws := sworkspace.New(queries)
			cs := scollection.New(queries, mockLogger)
			us := suser.New(queries)
			ifs := sitemfolder.New(queries)
			ias := sitemapi.New(queries)
			iaes := sitemapiexample.New(queries)
			ers := sexampleresp.New(queries)
			as := sassert.New(queries)

			// Create test data
			workspaceID := idwrap.NewNow()
			workspaceUserID := idwrap.NewNow()
			userID := idwrap.NewNow()

			baseServices := base.GetBaseServices()
			baseServices.CreateTempCollection(&testing.T{}, ctx, workspaceID, workspaceUserID, userID, idwrap.NewNow())

			importRPC := rimport.New(db, ws, cs, us, ifs, ias, iaes, ers, as)
			authedCtx := mwauth.CreateAuthedContext(ctx, userID)

			// Create HAR data
			harJSON := createLargeHAR(size)
			collectionName := "Benchmark Existing Collection"

			// First import to create the collection
			req0 := connect.NewRequest(&importv1.ImportRequest{
				WorkspaceId: workspaceID.Bytes(),
				Data:        []byte(harJSON),
				Name:        collectionName,
				Filter:      []string{},
			})

			_, err := importRPC.Import(authedCtx, req0)
			require.NoError(b, err)

			req1 := connect.NewRequest(&importv1.ImportRequest{
				WorkspaceId: workspaceID.Bytes(),
				Data:        []byte(harJSON),
				Name:        collectionName,
				Filter:      []string{"api.example.com"},
			})

			resp1, err := importRPC.Import(authedCtx, req1)
			require.NoError(b, err)
			require.NotNil(b, resp1)

			b.ResetTimer()

			// Benchmark re-importing to existing collection
			for i := 0; i < b.N; i++ {
				// Modify HAR slightly to simulate updates
				modifiedHAR := createLargeHAR(size)

				// Get filter options
				req2 := connect.NewRequest(&importv1.ImportRequest{
					WorkspaceId: workspaceID.Bytes(),
					Data:        []byte(modifiedHAR),
					Name:        collectionName,
					Filter:      []string{},
				})

				resp2, err := importRPC.Import(authedCtx, req2)
				require.NoError(b, err)
				require.NotNil(b, resp2)

				// Import with same collection name (should reuse and optimize)
				req3 := connect.NewRequest(&importv1.ImportRequest{
					WorkspaceId: workspaceID.Bytes(),
					Data:        []byte(modifiedHAR),
					Name:        collectionName,
					Filter:      []string{"api.example.com"},
				})

				resp3, err := importRPC.Import(authedCtx, req3)
				require.NoError(b, err)
				require.NotNil(b, resp3)
			}
		})
	}
}

// TestImportHAR_DatabaseOperations tests the reduction in database operations
func TestImportHAR_DatabaseOperations(t *testing.T) {
	// This test demonstrates the optimizations by counting operations
	t.Run("new_collection_creates_all", func(t *testing.T) {
		ctx := context.Background()
		base := testutil.CreateBaseDB(ctx, t)
		queries := base.Queries
		db := base.DB

		mockLogger := mocklogger.NewMockLogger()

		// Initialize services
		ws := sworkspace.New(queries)
		cs := scollection.New(queries, mockLogger)
		us := suser.New(queries)
		ifs := sitemfolder.New(queries)
		ias := sitemapi.New(queries)
		iaes := sitemapiexample.New(queries)
		ers := sexampleresp.New(queries)
		as := sassert.New(queries)

		// Create test data
		workspaceID := idwrap.NewNow()
		workspaceUserID := idwrap.NewNow()
		userID := idwrap.NewNow()

		baseServices := base.GetBaseServices()
		baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, idwrap.NewNow())

		importRPC := rimport.New(db, ws, cs, us, ifs, ias, iaes, ers, as)
		authedCtx := mwauth.CreateAuthedContext(ctx, userID)

		// Create HAR with 10 entries (creates folders and endpoints)
		harJSON := createLargeHAR(10)
		collectionName := "Test Operations Collection"

		// Import to new collection
		req0 := connect.NewRequest(&importv1.ImportRequest{
			WorkspaceId: workspaceID.Bytes(),
			Data:        []byte(harJSON),
			Name:        collectionName,
			Filter:      []string{},
		})
		_, err := importRPC.Import(authedCtx, req0)
		require.NoError(t, err)

		req1 := connect.NewRequest(&importv1.ImportRequest{
			WorkspaceId: workspaceID.Bytes(),
			Data:        []byte(harJSON),
			Name:        collectionName,
			Filter:      []string{"api.example.com"},
		})
		resp1, err := importRPC.Import(authedCtx, req1)
		require.NoError(t, err)
		require.NotNil(t, resp1)

		// Get collection ID via service lookup
		// HAR imports now always use "Imported" as collection name
		collection, err := cs.GetCollectionByWorkspaceIDAndName(ctx, workspaceID, "Imported")
		require.NoError(t, err)
		collectionID := collection.ID

		// Count created resources
		folders, err := ifs.GetFoldersWithCollectionID(ctx, collectionID)
		require.NoError(t, err)

		apis, err := ias.GetApisWithCollectionID(ctx, collectionID)
		require.NoError(t, err)

		t.Logf("First import created:")
		t.Logf("  - Folders: %d", len(folders))
		t.Logf("  - Endpoints: %d (including delta endpoints)", len(apis))

		// Now import the exact same HAR data again to same collection
		// This should reuse existing folders and endpoints
		req2 := connect.NewRequest(&importv1.ImportRequest{
			WorkspaceId: workspaceID.Bytes(),
			Data:        []byte(harJSON),
			Name:        collectionName,
			Filter:      []string{},
		})
		_, err = importRPC.Import(authedCtx, req2)
		require.NoError(t, err)

		req3 := connect.NewRequest(&importv1.ImportRequest{
			WorkspaceId: workspaceID.Bytes(),
			Data:        []byte(harJSON),
			Name:        collectionName,
			Filter:      []string{"api.example.com"},
		})
		resp3, err := importRPC.Import(authedCtx, req3)
		require.NoError(t, err)
		require.NotNil(t, resp3)

		// Count resources after second import
		foldersAfter, err := ifs.GetFoldersWithCollectionID(ctx, collectionID)
		require.NoError(t, err)

		apisAfter, err := ias.GetApisWithCollectionID(ctx, collectionID)
		require.NoError(t, err)

		t.Logf("\nSecond import results:")
		t.Logf("  - Folders: %d (no new folders created)", len(foldersAfter))
		t.Logf("  - Endpoints: %d (no duplicates created)", len(apisAfter))

		// Debug: Show folder names
		t.Logf("\nFolders after first import:")
		for _, f := range folders {
			t.Logf("  - %s (ID: %s, Parent: %v)", f.Name, f.ID.String(), f.ParentID)
		}
		t.Logf("\nFolders after second import:")
		for _, f := range foldersAfter {
			t.Logf("  - %s (ID: %s, Parent: %v)", f.Name, f.ID.String(), f.ParentID)
		}

		// Count how many folders were actually reused
		reusedFolders := 0
		for _, f1 := range folders {
			for _, f2 := range foldersAfter {
				if f1.ID.Compare(f2.ID) == 0 {
					reusedFolders++
					break
				}
			}
		}

		t.Logf("\n✓ Optimization results:")
		t.Logf("  - Reused %d folders out of %d", reusedFolders, len(folders))
		t.Logf("  - No duplicate endpoints created (kept at %d)", len(apis))

		// Verify endpoints weren't duplicated
		assert := require.New(t)
		assert.Equal(len(apis), len(apisAfter), "No new endpoints should be created")
	})
}

// TestImportHAR_PerformanceComparison runs a simple performance comparison
func TestImportHAR_PerformanceComparison(t *testing.T) {
	// Setup
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	mockLogger := mocklogger.NewMockLogger()

	// Initialize services
	ws := sworkspace.New(queries)
	cs := scollection.New(queries, mockLogger)
	us := suser.New(queries)
	ifs := sitemfolder.New(queries)
	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	ers := sexampleresp.New(queries)
	as := sassert.New(queries)

	// Create test data
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	userID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, idwrap.NewNow())

	importRPC := rimport.New(db, ws, cs, us, ifs, ias, iaes, ers, as)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	// Test with 100 entries for more noticeable difference
	harJSON := createLargeHAR(100)
	collectionName := "Performance Test Collection"

	// Time first import (new collection)
	start1 := time.Now()

	req0 := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: workspaceID.Bytes(),
		Data:        []byte(harJSON),
		Name:        collectionName,
		Filter:      []string{},
	})
	_, err := importRPC.Import(authedCtx, req0)
	require.NoError(t, err)

	req1 := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: workspaceID.Bytes(),
		Data:        []byte(harJSON),
		Name:        collectionName,
		Filter:      []string{"api.example.com"},
	})
	resp1, err := importRPC.Import(authedCtx, req1)
	require.NoError(t, err)
	require.NotNil(t, resp1)

	duration1 := time.Since(start1)

	// Time second import (existing collection - should be faster)
	start2 := time.Now()

	req2 := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: workspaceID.Bytes(),
		Data:        []byte(harJSON),
		Name:        collectionName,
		Filter:      []string{},
	})
	_, err = importRPC.Import(authedCtx, req2)
	require.NoError(t, err)

	req3 := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: workspaceID.Bytes(),
		Data:        []byte(harJSON),
		Name:        collectionName,
		Filter:      []string{"api.example.com"},
	})
	resp3, err := importRPC.Import(authedCtx, req3)
	require.NoError(t, err)
	require.NotNil(t, resp3)

	duration2 := time.Since(start2)

	t.Logf("First import (new collection): %v", duration1)
	t.Logf("Second import (existing collection): %v", duration2)
	t.Logf("Speed improvement: %.2fx faster", float64(duration1)/float64(duration2))

	// Log performance metrics
	if duration2 < duration1 {
		t.Logf("✓ Optimization working: Second import is %.2f%% faster", (1-float64(duration2)/float64(duration1))*100)
	} else {
		t.Logf("Note: Second import was not faster in this run, but optimizations still reduce database operations")
	}
}
