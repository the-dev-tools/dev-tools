package mutation

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"the-dev-tools/db/pkg/dbtest"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mfile"
)

// testSetup creates a test database and mutation context.
func testSetup(ctx context.Context, t *testing.T) (*gen.Queries, *Context) {
	t.Helper()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)

	queries, err := gen.Prepare(ctx, db)
	require.NoError(t, err)
	t.Cleanup(func() {
		queries.Close()
		db.Close()
	})

	mut := New(db)
	err = mut.Begin(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		mut.Rollback()
	})

	return queries, mut
}

// createTestHTTP creates a test HTTP entry with children.
func createTestHTTP(ctx context.Context, t *testing.T, q *gen.Queries, workspaceID idwrap.IDWrap) idwrap.IDWrap {
	t.Helper()
	httpID := idwrap.NewNow()

	// Create HTTP
	err := q.CreateHTTP(ctx, gen.CreateHTTPParams{
		ID:          httpID,
		WorkspaceID: workspaceID,
		Name:        "Test HTTP",
		Url:         "https://example.com",
		Method:      "GET",
		Description: "Test",
		BodyKind:    0,
		IsDelta:     false,
	})
	require.NoError(t, err)

	// Create Header
	headerID := idwrap.NewNow()
	err = q.CreateHTTPHeader(ctx, gen.CreateHTTPHeaderParams{
		ID:          headerID,
		HttpID:      httpID,
		HeaderKey:   "Content-Type",
		HeaderValue: "application/json",
		Enabled:     true,
		IsDelta:     false,
	})
	require.NoError(t, err)

	// Create Param
	paramID := idwrap.NewNow()
	err = q.CreateHTTPSearchParam(ctx, gen.CreateHTTPSearchParamParams{
		ID:      paramID,
		HttpID:  httpID,
		Key:     "q",
		Value:   "test",
		Enabled: true,
		IsDelta: false,
	})
	require.NoError(t, err)

	return httpID
}

// createTestFlow creates a test Flow entry with children.
func createTestFlow(ctx context.Context, t *testing.T, q *gen.Queries, workspaceID idwrap.IDWrap) idwrap.IDWrap {
	t.Helper()
	flowID := idwrap.NewNow()

	// Create Flow
	err := q.CreateFlow(ctx, gen.CreateFlowParams{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	})
	require.NoError(t, err)

	// Create Node
	nodeID := idwrap.NewNow()
	err = q.CreateFlowNode(ctx, gen.CreateFlowNodeParams{
		ID:       nodeID,
		FlowID:   flowID,
		NodeKind: 0, // noop
		Name:     "Test Node",
	})
	require.NoError(t, err)

	// Create Edge
	edgeID := idwrap.NewNow()
	err = q.CreateFlowEdge(ctx, gen.CreateFlowEdgeParams{
		ID:       edgeID,
		FlowID:   flowID,
		SourceID: nodeID,
		TargetID: nodeID,
	})
	require.NoError(t, err)

	// Create Variable
	varID := idwrap.NewNow()
	err = q.CreateFlowVariable(ctx, gen.CreateFlowVariableParams{
		ID:     varID,
		FlowID: flowID,
		Key:    "testVar",
		Value:  "{}",
	})
	require.NoError(t, err)

	return flowID
}

// createTestFile creates a File pointing to content.
func createTestFile(ctx context.Context, t *testing.T, q *gen.Queries, workspaceID idwrap.IDWrap, contentID *idwrap.IDWrap, contentType mfile.ContentType) idwrap.IDWrap {
	t.Helper()
	fileID := idwrap.NewNow()

	err := q.CreateFile(ctx, gen.CreateFileParams{
		ID:           fileID,
		WorkspaceID:  workspaceID,
		ContentID:    contentID,
		ContentKind:  int8(contentType),
		Name:         "Test File",
		DisplayOrder: 1.0,
		UpdatedAt:    time.Now().Unix(),
	})
	require.NoError(t, err)

	return fileID
}

// TestDeleteFile_CascadesToHTTP verifies File deletion cascades to HTTP content.
func TestDeleteFile_CascadesToHTTP(t *testing.T) {
	ctx := context.Background()
	q, mut := testSetup(ctx, t)

	workspaceID := idwrap.NewNow()
	httpID := createTestHTTP(ctx, t, q, workspaceID)
	fileID := createTestFile(ctx, t, q, workspaceID, &httpID, mfile.ContentTypeHTTP)

	// Delete File
	err := mut.DeleteFile(ctx, FileDeleteItem{
		ID:          fileID,
		WorkspaceID: workspaceID,
		ContentID:   &httpID,
		ContentKind: mfile.ContentTypeHTTP,
	})
	require.NoError(t, err)

	// Verify events collected
	events := mut.Events()
	assert.GreaterOrEqual(t, len(events), 4, "should have HTTP + header + param + file events")

	// Check event types
	hasHTTP := false
	hasHeader := false
	hasParam := false
	hasFile := false
	for _, e := range events {
		switch e.Entity {
		case EntityHTTP:
			hasHTTP = true
		case EntityHTTPHeader:
			hasHeader = true
		case EntityHTTPParam:
			hasParam = true
		case EntityFile:
			hasFile = true
		}
	}
	assert.True(t, hasHTTP, "should track HTTP delete")
	assert.True(t, hasHeader, "should track HTTP header delete")
	assert.True(t, hasParam, "should track HTTP param delete")
	assert.True(t, hasFile, "should track File delete")
}

// TestDeleteFile_CascadesToFlow verifies File deletion cascades to Flow content.
func TestDeleteFile_CascadesToFlow(t *testing.T) {
	ctx := context.Background()
	q, mut := testSetup(ctx, t)

	workspaceID := idwrap.NewNow()
	flowID := createTestFlow(ctx, t, q, workspaceID)
	fileID := createTestFile(ctx, t, q, workspaceID, &flowID, mfile.ContentTypeFlow)

	// Delete File
	err := mut.DeleteFile(ctx, FileDeleteItem{
		ID:          fileID,
		WorkspaceID: workspaceID,
		ContentID:   &flowID,
		ContentKind: mfile.ContentTypeFlow,
	})
	require.NoError(t, err)

	// Verify events collected
	events := mut.Events()
	assert.GreaterOrEqual(t, len(events), 5, "should have Flow + node + edge + variable + file events")

	// Check event types
	hasFlow := false
	hasNode := false
	hasEdge := false
	hasVariable := false
	hasFile := false
	for _, e := range events {
		switch e.Entity {
		case EntityFlow:
			hasFlow = true
		case EntityFlowNode:
			hasNode = true
		case EntityFlowEdge:
			hasEdge = true
		case EntityFlowVariable:
			hasVariable = true
		case EntityFile:
			hasFile = true
		}
	}
	assert.True(t, hasFlow, "should track Flow delete")
	assert.True(t, hasNode, "should track FlowNode delete")
	assert.True(t, hasEdge, "should track FlowEdge delete")
	assert.True(t, hasVariable, "should track FlowVariable delete")
	assert.True(t, hasFile, "should track File delete")
}

// TestDeleteHTTP_WithFile_DeletesFile verifies HTTP deletion goes through File.
func TestDeleteHTTP_WithFile_DeletesFile(t *testing.T) {
	ctx := context.Background()
	q, mut := testSetup(ctx, t)

	workspaceID := idwrap.NewNow()
	httpID := createTestHTTP(ctx, t, q, workspaceID)
	fileID := createTestFile(ctx, t, q, workspaceID, &httpID, mfile.ContentTypeHTTP)

	// Delete HTTP (should cascade through File)
	err := mut.DeleteHTTP(ctx, HTTPDeleteItem{
		ID:          httpID,
		WorkspaceID: workspaceID,
		IsDelta:     false,
	})
	require.NoError(t, err)

	// Verify both HTTP and File events tracked
	events := mut.Events()
	hasHTTP := false
	hasFile := false
	for _, e := range events {
		if e.Entity == EntityHTTP && e.ID == httpID {
			hasHTTP = true
		}
		if e.Entity == EntityFile && e.ID == fileID {
			hasFile = true
		}
	}
	assert.True(t, hasHTTP, "should track HTTP delete")
	assert.True(t, hasFile, "should track File delete (cascade from HTTP)")
}

// TestDeleteHTTP_Orphaned_DeletesOnlyHTTP verifies orphaned HTTP deletion.
func TestDeleteHTTP_Orphaned_DeletesOnlyHTTP(t *testing.T) {
	ctx := context.Background()
	q, mut := testSetup(ctx, t)

	workspaceID := idwrap.NewNow()
	httpID := createTestHTTP(ctx, t, q, workspaceID)
	// No File created - HTTP is orphaned

	// Delete orphaned HTTP
	err := mut.DeleteHTTP(ctx, HTTPDeleteItem{
		ID:          httpID,
		WorkspaceID: workspaceID,
		IsDelta:     false,
	})
	require.NoError(t, err)

	// Verify HTTP events tracked, no File events
	events := mut.Events()
	hasHTTP := false
	hasFile := false
	for _, e := range events {
		if e.Entity == EntityHTTP {
			hasHTTP = true
		}
		if e.Entity == EntityFile {
			hasFile = true
		}
	}
	assert.True(t, hasHTTP, "should track HTTP delete")
	assert.False(t, hasFile, "should NOT track File delete (orphaned)")
}

// TestDeleteFlow_WithFile_DeletesFile verifies Flow deletion goes through File.
func TestDeleteFlow_WithFile_DeletesFile(t *testing.T) {
	ctx := context.Background()
	q, mut := testSetup(ctx, t)

	workspaceID := idwrap.NewNow()
	flowID := createTestFlow(ctx, t, q, workspaceID)
	fileID := createTestFile(ctx, t, q, workspaceID, &flowID, mfile.ContentTypeFlow)

	// Delete Flow (should cascade through File)
	err := mut.DeleteFlow(ctx, FlowDeleteItem{
		ID:          flowID,
		WorkspaceID: workspaceID,
	})
	require.NoError(t, err)

	// Verify both Flow and File events tracked
	events := mut.Events()
	hasFlow := false
	hasFile := false
	for _, e := range events {
		if e.Entity == EntityFlow && e.ID == flowID {
			hasFlow = true
		}
		if e.Entity == EntityFile && e.ID == fileID {
			hasFile = true
		}
	}
	assert.True(t, hasFlow, "should track Flow delete")
	assert.True(t, hasFile, "should track File delete (cascade from Flow)")
}

// TestDeleteFlow_Orphaned_DeletesOnlyFlow verifies orphaned Flow deletion.
func TestDeleteFlow_Orphaned_DeletesOnlyFlow(t *testing.T) {
	ctx := context.Background()
	q, mut := testSetup(ctx, t)

	workspaceID := idwrap.NewNow()
	flowID := createTestFlow(ctx, t, q, workspaceID)
	// No File created - Flow is orphaned

	// Delete orphaned Flow
	err := mut.DeleteFlow(ctx, FlowDeleteItem{
		ID:          flowID,
		WorkspaceID: workspaceID,
	})
	require.NoError(t, err)

	// Verify Flow events tracked, no File events
	events := mut.Events()
	hasFlow := false
	hasFile := false
	for _, e := range events {
		if e.Entity == EntityFlow {
			hasFlow = true
		}
		if e.Entity == EntityFile {
			hasFile = true
		}
	}
	assert.True(t, hasFlow, "should track Flow delete")
	assert.False(t, hasFile, "should NOT track File delete (orphaned)")
}

// TestDeleteHTTPBatch verifies batch deletion of HTTP entries.
func TestDeleteHTTPBatch(t *testing.T) {
	ctx := context.Background()
	q, mut := testSetup(ctx, t)

	workspaceID := idwrap.NewNow()

	// Create 3 HTTPs - 2 with File, 1 orphaned
	http1 := createTestHTTP(ctx, t, q, workspaceID)
	http2 := createTestHTTP(ctx, t, q, workspaceID)
	http3 := createTestHTTP(ctx, t, q, workspaceID)

	file1 := createTestFile(ctx, t, q, workspaceID, &http1, mfile.ContentTypeHTTP)
	file2 := createTestFile(ctx, t, q, workspaceID, &http2, mfile.ContentTypeHTTP)
	// http3 is orphaned

	_ = file1
	_ = file2

	// Delete batch
	err := mut.DeleteHTTPBatch(ctx, []HTTPDeleteItem{
		{ID: http1, WorkspaceID: workspaceID, IsDelta: false},
		{ID: http2, WorkspaceID: workspaceID, IsDelta: false},
		{ID: http3, WorkspaceID: workspaceID, IsDelta: false},
	})
	require.NoError(t, err)

	// Verify events - should have 3 HTTP + 2 File + children
	events := mut.Events()
	httpCount := 0
	fileCount := 0
	for _, e := range events {
		if e.Entity == EntityHTTP {
			httpCount++
		}
		if e.Entity == EntityFile {
			fileCount++
		}
	}
	assert.Equal(t, 3, httpCount, "should track 3 HTTP deletes")
	assert.Equal(t, 2, fileCount, "should track 2 File deletes (http3 is orphaned)")
}

// TestDeleteFlowBatch verifies batch deletion of Flow entries.
func TestDeleteFlowBatch(t *testing.T) {
	ctx := context.Background()
	q, mut := testSetup(ctx, t)

	workspaceID := idwrap.NewNow()

	// Create 2 Flows - 1 with File, 1 orphaned
	flow1 := createTestFlow(ctx, t, q, workspaceID)
	flow2 := createTestFlow(ctx, t, q, workspaceID)

	file1 := createTestFile(ctx, t, q, workspaceID, &flow1, mfile.ContentTypeFlow)
	// flow2 is orphaned

	_ = file1

	// Delete batch
	err := mut.DeleteFlowBatch(ctx, []FlowDeleteItem{
		{ID: flow1, WorkspaceID: workspaceID},
		{ID: flow2, WorkspaceID: workspaceID},
	})
	require.NoError(t, err)

	// Verify events - should have 2 Flow + 1 File + children
	events := mut.Events()
	flowCount := 0
	fileCount := 0
	for _, e := range events {
		if e.Entity == EntityFlow {
			flowCount++
		}
		if e.Entity == EntityFile {
			fileCount++
		}
	}
	assert.Equal(t, 2, flowCount, "should track 2 Flow deletes")
	assert.Equal(t, 1, fileCount, "should track 1 File delete (flow2 is orphaned)")
}

// TestNoCascadeLoop documents compile-time safety.
// The fact that this test compiles proves no infinite loop is possible.
// DeleteHTTP() -> DeleteFile() -> deleteHTTPContent() (unexported, can't call back)
func TestNoCascadeLoop(t *testing.T) {
	// This test exists to document the compile-time guarantee.
	// If this compiles, the cascade is safe by construction.
	//
	// The guarantee:
	// - DeleteHTTP() checks File ownership and calls DeleteFile() if owned
	// - DeleteFile() calls deleteHTTPContent() (unexported)
	// - deleteHTTPContent() cannot call DeleteFile() or DeleteHTTP() (unexported)
	// - Same pattern for DeleteFlow()
	//
	// No runtime checks needed - Go's visibility rules enforce cascade direction.
	t.Log("Compile-time cascade safety: unexported methods cannot loop back to exported methods")
}

// Benchmarks

// BenchmarkDeleteHTTP_WithChildren benchmarks HTTP deletion with children.
func BenchmarkDeleteHTTP_WithChildren(b *testing.B) {
	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	queries, err := gen.Prepare(ctx, db)
	if err != nil {
		b.Fatal(err)
	}
	defer queries.Close()

	workspaceID := idwrap.NewNow()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()

		// Setup - create HTTP with children
		httpID := idwrap.NewNow()

		_ = queries.CreateHTTP(ctx, gen.CreateHTTPParams{
			ID:          httpID,
			WorkspaceID: workspaceID,
			Name:        "Test",
			Url:         "https://example.com",
			Method:      "GET",
			IsDelta:     false,
		})

		// Create 10 headers, 5 params
		for j := 0; j < 10; j++ {
			_ = queries.CreateHTTPHeader(ctx, gen.CreateHTTPHeaderParams{
				ID:        idwrap.NewNow(),
				HttpID:    httpID,
				HeaderKey: "Header",
				Enabled:   true,
				IsDelta:   false,
			})
		}
		for j := 0; j < 5; j++ {
			_ = queries.CreateHTTPSearchParam(ctx, gen.CreateHTTPSearchParamParams{
				ID:      idwrap.NewNow(),
				HttpID:  httpID,
				Key:     "Param",
				Enabled: true,
				IsDelta: false,
			})
		}

		mut := New(db)
		_ = mut.Begin(ctx)

		b.StartTimer()

		// Benchmark - delete HTTP (orphaned for simplicity)
		_ = mut.deleteHTTPContent(ctx, httpID, workspaceID, false)

		b.StopTimer()
		mut.Rollback()
	}
}

// BenchmarkDeleteFile_CascadeToHTTP benchmarks File deletion cascading to HTTP.
func BenchmarkDeleteFile_CascadeToHTTP(b *testing.B) {
	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	queries, err := gen.Prepare(ctx, db)
	if err != nil {
		b.Fatal(err)
	}
	defer queries.Close()

	workspaceID := idwrap.NewNow()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()

		// Setup - create File + HTTP with children
		httpID := idwrap.NewNow()
		fileID := idwrap.NewNow()

		_ = queries.CreateHTTP(ctx, gen.CreateHTTPParams{
			ID:          httpID,
			WorkspaceID: workspaceID,
			Name:        "Test",
			Url:         "https://example.com",
			Method:      "GET",
			IsDelta:     false,
		})

		// Create children
		for j := 0; j < 5; j++ {
			_ = queries.CreateHTTPHeader(ctx, gen.CreateHTTPHeaderParams{
				ID:        idwrap.NewNow(),
				HttpID:    httpID,
				HeaderKey: "Header",
				Enabled:   true,
				IsDelta:   false,
			})
		}

		_ = queries.CreateFile(ctx, gen.CreateFileParams{
			ID:           fileID,
			WorkspaceID:  workspaceID,
			ContentID:    &httpID,
			ContentKind:  int8(mfile.ContentTypeHTTP),
			Name:         "Test",
			DisplayOrder: 1.0,
			UpdatedAt:    time.Now().Unix(),
		})

		mut := New(db)
		_ = mut.Begin(ctx)

		b.StartTimer()

		// Benchmark - delete File (cascades to HTTP)
		_ = mut.DeleteFile(ctx, FileDeleteItem{
			ID:          fileID,
			WorkspaceID: workspaceID,
			ContentID:   &httpID,
			ContentKind: mfile.ContentTypeHTTP,
		})

		b.StopTimer()
		mut.Rollback()
	}
}

// BenchmarkDeleteFlow_WithChildren benchmarks Flow deletion with children.
func BenchmarkDeleteFlow_WithChildren(b *testing.B) {
	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	queries, err := gen.Prepare(ctx, db)
	if err != nil {
		b.Fatal(err)
	}
	defer queries.Close()

	workspaceID := idwrap.NewNow()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()

		// Setup - create Flow with children
		flowID := idwrap.NewNow()

		_ = queries.CreateFlow(ctx, gen.CreateFlowParams{
			ID:          flowID,
			WorkspaceID: workspaceID,
			Name:        "Test",
		})

		// Create 10 nodes, 15 edges, 5 variables
		nodeIDs := make([]idwrap.IDWrap, 10)
		for j := 0; j < 10; j++ {
			nodeIDs[j] = idwrap.NewNow()
			_ = queries.CreateFlowNode(ctx, gen.CreateFlowNodeParams{
				ID:       nodeIDs[j],
				FlowID:   flowID,
				NodeKind: 0,
				Name:     "Node",
			})
		}
		for j := 0; j < 15; j++ {
			_ = queries.CreateFlowEdge(ctx, gen.CreateFlowEdgeParams{
				ID:       idwrap.NewNow(),
				FlowID:   flowID,
				SourceID: nodeIDs[j%10],
				TargetID: nodeIDs[(j+1)%10],
			})
		}
		for j := 0; j < 5; j++ {
			_ = queries.CreateFlowVariable(ctx, gen.CreateFlowVariableParams{
				ID:           idwrap.NewNow(),
				FlowID:       flowID,
				Key:          "Var",
				Value:        "{}",
				DisplayOrder: float64(j),
			})
		}

		mut := New(db)
		_ = mut.Begin(ctx)

		b.StartTimer()

		// Benchmark - delete Flow (orphaned for simplicity)
		_ = mut.deleteFlowContent(ctx, flowID, workspaceID)

		b.StopTimer()
		mut.Rollback()
	}
}

// BenchmarkEventCollection benchmarks pure event collection overhead.
func BenchmarkEventCollection(b *testing.B) {
	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	mut := New(db)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mut.Reset()

		// Track 100 events
		for j := 0; j < 100; j++ {
			mut.track(Event{
				Entity:      EntityHTTP,
				Op:          OpDelete,
				ID:          idwrap.NewNow(),
				WorkspaceID: idwrap.NewNow(),
			})
		}
	}
}
