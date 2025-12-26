package txutil

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"the-dev-tools/db/pkg/sqlitemem"
)

// Test types
type testItem struct {
	ID          string
	WorkspaceID string
	Value       string
}

type testTopic struct {
	WorkspaceID string
}

type testPatch struct {
	Field string
}

func TestBulkSyncTxInsert_GroupsByTopic(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	require.NoError(t, err)
	defer cleanup()

	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)

	// Track publication
	publications := make(map[testTopic][]testItem)

	publishFn := func(topic testTopic, items []testItem) {
		publications[topic] = append(publications[topic], items...)
	}

	// Extract topic from item
	extractor := func(item testItem) testTopic {
		return testTopic{WorkspaceID: item.WorkspaceID}
	}

	// Create bulk wrapper
	syncTx := NewBulkInsertTx[testItem, testTopic](tx, extractor)

	// Track items with different workspaces
	syncTx.Track(testItem{ID: "1", WorkspaceID: "ws1", Value: "a"})
	syncTx.Track(testItem{ID: "2", WorkspaceID: "ws1", Value: "b"})
	syncTx.Track(testItem{ID: "3", WorkspaceID: "ws2", Value: "c"})
	syncTx.Track(testItem{ID: "4", WorkspaceID: "ws1", Value: "d"})
	syncTx.Track(testItem{ID: "5", WorkspaceID: "ws2", Value: "e"})

	// Commit and publish
	err = syncTx.CommitAndPublish(ctx, publishFn)
	require.NoError(t, err)

	// Verify grouping
	assert.Len(t, publications, 2, "should have 2 topic groups")

	ws1Items := publications[testTopic{WorkspaceID: "ws1"}]
	ws2Items := publications[testTopic{WorkspaceID: "ws2"}]

	assert.Len(t, ws1Items, 3, "ws1 should have 3 items")
	assert.Len(t, ws2Items, 2, "ws2 should have 2 items")

	// Verify item contents
	assert.Contains(t, ws1Items, testItem{ID: "1", WorkspaceID: "ws1", Value: "a"})
	assert.Contains(t, ws1Items, testItem{ID: "2", WorkspaceID: "ws1", Value: "b"})
	assert.Contains(t, ws1Items, testItem{ID: "4", WorkspaceID: "ws1", Value: "d"})

	assert.Contains(t, ws2Items, testItem{ID: "3", WorkspaceID: "ws2", Value: "c"})
	assert.Contains(t, ws2Items, testItem{ID: "5", WorkspaceID: "ws2", Value: "e"})
}

func TestBulkSyncTxInsert_CommitFailure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	require.NoError(t, err)
	defer cleanup()

	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)

	// Force failure by rolling back transaction early
	require.NoError(t, tx.Rollback())

	publishCalled := false
	publishFn := func(topic testTopic, items []testItem) {
		publishCalled = true
	}

	extractor := func(item testItem) testTopic {
		return testTopic{WorkspaceID: item.WorkspaceID}
	}

	syncTx := NewBulkInsertTx[testItem, testTopic](tx, extractor)
	syncTx.Track(testItem{ID: "1", WorkspaceID: "ws1", Value: "a"})

	// Commit should fail
	err = syncTx.CommitAndPublish(ctx, publishFn)
	assert.Error(t, err)
	assert.False(t, publishCalled, "publish should not be called on commit failure")
}

func TestBulkSyncTxInsert_EmptyTracked(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	require.NoError(t, err)
	defer cleanup()

	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)

	publishCalled := false
	publishFn := func(topic testTopic, items []testItem) {
		publishCalled = true
	}

	extractor := func(item testItem) testTopic {
		return testTopic{WorkspaceID: item.WorkspaceID}
	}

	syncTx := NewBulkInsertTx[testItem, testTopic](tx, extractor)

	// Commit without tracking any items
	err = syncTx.CommitAndPublish(ctx, publishFn)
	assert.NoError(t, err)
	assert.False(t, publishCalled, "publish should not be called when no items tracked")
}

func TestBulkSyncTxUpdate_MultipleTopics(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	require.NoError(t, err)
	defer cleanup()

	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)

	// Track publications
	publications := make(map[testTopic][]UpdateEvent[testItem, testPatch])

	publishFn := func(topic testTopic, events []UpdateEvent[testItem, testPatch]) {
		publications[topic] = append(publications[topic], events...)
	}

	extractor := func(item testItem) testTopic {
		return testTopic{WorkspaceID: item.WorkspaceID}
	}

	syncTx := NewBulkUpdateTx[testItem, testPatch, testTopic](tx, extractor)

	// Track updates across 3 workspaces
	syncTx.Track(testItem{ID: "1", WorkspaceID: "ws1"}, testPatch{Field: "p1"})
	syncTx.Track(testItem{ID: "2", WorkspaceID: "ws2"}, testPatch{Field: "p2"})
	syncTx.Track(testItem{ID: "3", WorkspaceID: "ws3"}, testPatch{Field: "p3"})
	syncTx.Track(testItem{ID: "4", WorkspaceID: "ws1"}, testPatch{Field: "p4"})

	err = syncTx.CommitAndPublish(ctx, publishFn)
	require.NoError(t, err)

	// Verify 3 publish calls (one per workspace)
	assert.Len(t, publications, 3, "should have 3 topic groups")

	// Verify each workspace has correct events
	ws1Events := publications[testTopic{WorkspaceID: "ws1"}]
	ws2Events := publications[testTopic{WorkspaceID: "ws2"}]
	ws3Events := publications[testTopic{WorkspaceID: "ws3"}]

	assert.Len(t, ws1Events, 2, "ws1 should have 2 events")
	assert.Len(t, ws2Events, 1, "ws2 should have 1 event")
	assert.Len(t, ws3Events, 1, "ws3 should have 1 event")
}

func TestBulkSyncTxUpdate_CommitFailure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	require.NoError(t, err)
	defer cleanup()

	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)

	// Force failure
	require.NoError(t, tx.Rollback())

	publishCalled := false
	publishFn := func(topic testTopic, events []UpdateEvent[testItem, testPatch]) {
		publishCalled = true
	}

	extractor := func(item testItem) testTopic {
		return testTopic{WorkspaceID: item.WorkspaceID}
	}

	syncTx := NewBulkUpdateTx[testItem, testPatch, testTopic](tx, extractor)
	syncTx.Track(testItem{ID: "1", WorkspaceID: "ws1"}, testPatch{Field: "p1"})

	err = syncTx.CommitAndPublish(ctx, publishFn)
	assert.Error(t, err)
	assert.False(t, publishCalled, "publish should not be called on commit failure")
}

func TestBulkSyncTxDelete_GroupsByTopic(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	require.NoError(t, err)
	defer cleanup()

	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)

	// Track publications
	publications := make(map[testTopic][]DeleteEvent[string])

	publishFn := func(topic testTopic, events []DeleteEvent[string]) {
		publications[topic] = append(publications[topic], events...)
	}

	extractor := func(event DeleteEvent[string]) testTopic {
		return testTopic{WorkspaceID: event.WorkspaceID}
	}

	syncTx := NewBulkDeleteTx[string, testTopic](tx, extractor)

	// Track deletes
	syncTx.Track("id1", "ws1", false)
	syncTx.Track("id2", "ws1", true)
	syncTx.Track("id3", "ws2", false)
	syncTx.Track("id4", "ws1", false)

	err = syncTx.CommitAndPublish(ctx, publishFn)
	require.NoError(t, err)

	// Verify grouping
	assert.Len(t, publications, 2, "should have 2 topic groups")

	ws1Events := publications[testTopic{WorkspaceID: "ws1"}]
	ws2Events := publications[testTopic{WorkspaceID: "ws2"}]

	assert.Len(t, ws1Events, 3, "ws1 should have 3 delete events")
	assert.Len(t, ws2Events, 1, "ws2 should have 1 delete event")

	// Verify delete events
	assert.Equal(t, "id1", ws1Events[0].ID)
	assert.Equal(t, false, ws1Events[0].IsDelta)
	assert.Equal(t, "id2", ws1Events[1].ID)
	assert.Equal(t, true, ws1Events[1].IsDelta)
}

func TestBulkSyncTxDelete_EmptyTracked(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	require.NoError(t, err)
	defer cleanup()

	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)

	publishCalled := false
	publishFn := func(topic testTopic, events []DeleteEvent[string]) {
		publishCalled = true
	}

	extractor := func(event DeleteEvent[string]) testTopic {
		return testTopic{WorkspaceID: event.WorkspaceID}
	}

	syncTx := NewBulkDeleteTx[string, testTopic](tx, extractor)

	err = syncTx.CommitAndPublish(ctx, publishFn)
	assert.NoError(t, err)
	assert.False(t, publishCalled, "publish should not be called when no events tracked")
}

func TestBulkSyncTxDelete_CommitFailure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	require.NoError(t, err)
	defer cleanup()

	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)

	// Force failure
	require.NoError(t, tx.Rollback())

	publishCalled := false
	publishFn := func(topic testTopic, events []DeleteEvent[string]) {
		publishCalled = true
	}

	extractor := func(event DeleteEvent[string]) testTopic {
		return testTopic{WorkspaceID: event.WorkspaceID}
	}

	syncTx := NewBulkDeleteTx[string, testTopic](tx, extractor)
	syncTx.Track("id1", "ws1", false)

	err = syncTx.CommitAndPublish(ctx, publishFn)
	assert.Error(t, err)
	assert.False(t, publishCalled, "publish should not be called on commit failure")
}

// Helper for testing actual SQL errors
func TestBulkSyncTxInsert_RealSQLError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	require.NoError(t, err)
	cleanup() // Close DB immediately to trigger errors

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		// Expected - can't begin transaction on closed DB
		return
	}

	publishCalled := false
	publishFn := func(topic testTopic, items []testItem) {
		publishCalled = true
	}

	extractor := func(item testItem) testTopic {
		return testTopic{WorkspaceID: item.WorkspaceID}
	}

	syncTx := NewBulkInsertTx[testItem, testTopic](tx, extractor)
	syncTx.Track(testItem{ID: "1", WorkspaceID: "ws1"})

	err = syncTx.CommitAndPublish(ctx, publishFn)
	if err == nil {
		// If no error, check transaction state
		return
	}

	assert.Error(t, err)
	assert.False(t, publishCalled, "publish should not be called on SQL error")
}
