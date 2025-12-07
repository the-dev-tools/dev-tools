package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// SyncParityTestConfig defines the configuration for a sync parity test.
type SyncParityTestConfig[CollItem any, SyncItem any] struct {
	// Setup performs any necessary setup (e.g., creating base entities) and returns a cleanup function if needed.
	Setup func(t *testing.T) (context.Context, func())

	// TriggerUpdate performs an action that should trigger a sync event (e.g., inserting or updating a record).
	// It should return the ID of the item being tested.
	TriggerUpdate func(ctx context.Context, t *testing.T)

	// GetCollection calls the Collection RPC and returns the list of items.
	GetCollection func(ctx context.Context, t *testing.T) []CollItem

	// StartSync starts the Sync stream and returns a channel of sync items.
	// The function should run the sync stream in a goroutine and push items to the channel.
	// It should return a cancel function to stop the stream.
	StartSync func(ctx context.Context, t *testing.T) (<-chan SyncItem, func())

	// Compare performs assertions to verify that the collection item matches the sync item.
	Compare func(t *testing.T, collItem CollItem, syncItem SyncItem)
}

// VerifySyncParity verifies that the data returned by a Collection endpoint matches the data pushed by a Sync endpoint.
func VerifySyncParity[CollItem any, SyncItem any](t *testing.T, cfg SyncParityTestConfig[CollItem, SyncItem]) {
	t.Helper()

	if cfg.Setup == nil {
		require.FailNow(t, "Setup function is required")
	}

	ctx, cleanup := cfg.Setup(t)
	if cleanup != nil {
		defer cleanup()
	}

	// 1. Start Sync Stream
	syncCh, cancelSync := cfg.StartSync(ctx, t)
	defer cancelSync()

	// Give subscription time to establish
	time.Sleep(100 * time.Millisecond)

	// 2. Trigger Update
	cfg.TriggerUpdate(ctx, t)

	// 3. Get Collection Response
	collItems := cfg.GetCollection(ctx, t)
	require.NotEmpty(t, collItems, "Collection should return items")
	collItem := collItems[0] // Assuming the first item is the one we want (or we could pass a filter)

	// 4. Get Sync Response
	var syncItem SyncItem
	select {
	case item := <-syncCh:
		syncItem = item
	case <-time.After(2 * time.Second):
		require.FailNow(t, "Timeout waiting for sync event")
	}

	// 5. Compare
	cfg.Compare(t, collItem, syncItem)
}
