package streamregistry

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"the-dev-tools/db/pkg/dbtest"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/mutation"
)

// TestRegistry_PublishAll verifies Registry routes events to correct handlers.
func TestRegistry_PublishAll(t *testing.T) {
	registry := New()

	var mu sync.Mutex
	published := make(map[mutation.EntityType][]mutation.Event)

	// Register handlers for each entity type
	for _, entity := range []mutation.EntityType{
		mutation.EntityFile,
		mutation.EntityHTTP,
		mutation.EntityHTTPHeader,
		mutation.EntityHTTPParam,
	} {
		e := entity // capture
		registry.Register(e, func(evt mutation.Event) {
			mu.Lock()
			defer mu.Unlock()
			published[e] = append(published[e], evt)
		})
	}

	workspaceID := idwrap.NewNow()

	// Simulate events from cascade deletion
	events := []mutation.Event{
		{Entity: mutation.EntityHTTPHeader, Op: mutation.OpDelete, ID: idwrap.NewNow(), WorkspaceID: workspaceID},
		{Entity: mutation.EntityHTTPParam, Op: mutation.OpDelete, ID: idwrap.NewNow(), WorkspaceID: workspaceID},
		{Entity: mutation.EntityHTTP, Op: mutation.OpDelete, ID: idwrap.NewNow(), WorkspaceID: workspaceID},
		{Entity: mutation.EntityFile, Op: mutation.OpDelete, ID: idwrap.NewNow(), WorkspaceID: workspaceID},
	}

	// Publish all events
	registry.PublishAll(events)

	// Verify each handler received its events
	assert.Len(t, published[mutation.EntityFile], 1, "File handler should receive 1 event")
	assert.Len(t, published[mutation.EntityHTTP], 1, "HTTP handler should receive 1 event")
	assert.Len(t, published[mutation.EntityHTTPHeader], 1, "HTTPHeader handler should receive 1 event")
	assert.Len(t, published[mutation.EntityHTTPParam], 1, "HTTPParam handler should receive 1 event")
}

// TestRegistry_UnregisteredEntities verifies unregistered entities are silently skipped.
func TestRegistry_UnregisteredEntities(t *testing.T) {
	registry := New()

	var httpCalled bool
	registry.Register(mutation.EntityHTTP, func(evt mutation.Event) {
		httpCalled = true
	})

	// Publish event for unregistered entity - should not panic
	events := []mutation.Event{
		{Entity: mutation.EntityFlow, Op: mutation.OpDelete, ID: idwrap.NewNow()}, // Not registered
		{Entity: mutation.EntityHTTP, Op: mutation.OpDelete, ID: idwrap.NewNow()}, // Registered
	}

	registry.PublishAll(events)

	assert.True(t, httpCalled, "Registered handler should be called")
}

// TestRegistry_WithMutation verifies registry works with mutation.Context.
func TestRegistry_WithMutation(t *testing.T) {
	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

	queries, err := gen.Prepare(ctx, db)
	require.NoError(t, err)
	defer queries.Close()

	// Create mock registry that records published events
	var mu sync.Mutex
	var publishedEvents []mutation.Event
	registry := New()

	// Register handlers for all cascade entities
	for _, entity := range []mutation.EntityType{
		mutation.EntityFile,
		mutation.EntityHTTP,
		mutation.EntityHTTPHeader,
		mutation.EntityHTTPParam,
		mutation.EntityHTTPBodyForm,
		mutation.EntityHTTPBodyURL,
		mutation.EntityHTTPBodyRaw,
		mutation.EntityHTTPAssert,
	} {
		e := entity
		registry.Register(e, func(evt mutation.Event) {
			mu.Lock()
			defer mu.Unlock()
			publishedEvents = append(publishedEvents, evt)
		})
	}

	// Setup test data
	workspaceID := idwrap.NewNow()
	httpID := idwrap.NewNow()
	fileID := idwrap.NewNow()

	// Create HTTP with children
	err = queries.CreateHTTP(ctx, gen.CreateHTTPParams{
		ID:          httpID,
		WorkspaceID: workspaceID,
		Name:        "Test HTTP",
		Url:         "https://example.com",
		Method:      "GET",
		BodyKind:    0,
		IsDelta:     false,
	})
	require.NoError(t, err)

	// Create Header
	headerID := idwrap.NewNow()
	err = queries.CreateHTTPHeader(ctx, gen.CreateHTTPHeaderParams{
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
	err = queries.CreateHTTPSearchParam(ctx, gen.CreateHTTPSearchParamParams{
		ID:      paramID,
		HttpID:  httpID,
		Key:     "q",
		Value:   "test",
		Enabled: true,
		IsDelta: false,
	})
	require.NoError(t, err)

	// Create File pointing to HTTP
	err = queries.CreateFile(ctx, gen.CreateFileParams{
		ID:           fileID,
		WorkspaceID:  workspaceID,
		ContentID:    &httpID,
		ContentKind:  int8(mfile.ContentTypeHTTP),
		Name:         "Test File",
		DisplayOrder: 1.0,
		UpdatedAt:    time.Now().Unix(),
	})
	require.NoError(t, err)

	// Create mutation context with registry as publisher
	mut := mutation.New(db, mutation.WithPublisher(registry))
	err = mut.Begin(ctx)
	require.NoError(t, err)

	// Delete file - should cascade to HTTP and children
	err = mut.DeleteFile(ctx, mutation.FileDeleteItem{
		ID:          fileID,
		WorkspaceID: workspaceID,
		ContentID:   &httpID,
		ContentKind: mfile.ContentTypeHTTP,
	})
	require.NoError(t, err)

	// Commit - this triggers registry.PublishAll()
	err = mut.Commit(ctx)
	require.NoError(t, err)

	// Verify events were published to registry
	mu.Lock()
	defer mu.Unlock()

	assert.GreaterOrEqual(t, len(publishedEvents), 4, "should publish at least 4 events (header, param, http, file)")

	// Check specific entity types
	entityCounts := make(map[mutation.EntityType]int)
	for _, evt := range publishedEvents {
		entityCounts[evt.Entity]++
	}

	assert.Equal(t, 1, entityCounts[mutation.EntityFile], "should publish 1 File delete")
	assert.Equal(t, 1, entityCounts[mutation.EntityHTTP], "should publish 1 HTTP delete")
	assert.Equal(t, 1, entityCounts[mutation.EntityHTTPHeader], "should publish 1 HTTPHeader delete")
	assert.Equal(t, 1, entityCounts[mutation.EntityHTTPParam], "should publish 1 HTTPParam delete")

	// Verify all events have correct workspace ID
	for _, evt := range publishedEvents {
		assert.Equal(t, workspaceID, evt.WorkspaceID, "all events should have correct workspace ID")
		assert.Equal(t, mutation.OpDelete, evt.Op, "all events should be delete operations")
	}
}

// TestRegistry_EventOrder verifies events are published in collection order.
func TestRegistry_EventOrder(t *testing.T) {
	registry := New()

	var order []mutation.EntityType
	var mu sync.Mutex

	// Register handlers
	for _, entity := range []mutation.EntityType{
		mutation.EntityHTTPHeader,
		mutation.EntityHTTP,
		mutation.EntityFile,
	} {
		e := entity
		registry.Register(e, func(evt mutation.Event) {
			mu.Lock()
			defer mu.Unlock()
			order = append(order, e)
		})
	}

	// Events in cascade order: children before parents
	events := []mutation.Event{
		{Entity: mutation.EntityHTTPHeader, Op: mutation.OpDelete, ID: idwrap.NewNow()},
		{Entity: mutation.EntityHTTP, Op: mutation.OpDelete, ID: idwrap.NewNow()},
		{Entity: mutation.EntityFile, Op: mutation.OpDelete, ID: idwrap.NewNow()},
	}

	registry.PublishAll(events)

	// Verify order preserved
	require.Len(t, order, 3)
	assert.Equal(t, mutation.EntityHTTPHeader, order[0], "Header should be first")
	assert.Equal(t, mutation.EntityHTTP, order[1], "HTTP should be second")
	assert.Equal(t, mutation.EntityFile, order[2], "File should be third")
}
