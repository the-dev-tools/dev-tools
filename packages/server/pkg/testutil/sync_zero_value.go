//nolint:revive // exported
package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// ZeroValueTestCase defines a test case for zero-value field handling.
type ZeroValueTestCase[T any] struct {
	// Name is the human-readable test case name
	Name string

	// InitialValue is the initial (typically non-zero) value to set
	InitialValue T

	// ZeroValue is the zero value that should be correctly persisted
	ZeroValue T
}

// ZeroValueSyncTestConfig defines the configuration for testing zero-value sync handling.
// This catches bugs where `if value != 0` logic incorrectly excludes zero from sync events.
type ZeroValueSyncTestConfig[FieldType any, SyncItem any] struct {
	// Setup performs any necessary setup (e.g., creating base entities) and returns:
	// - context for the test
	// - cleanup function
	Setup func(t *testing.T) (context.Context, func())

	// StartSync starts the Sync stream and returns a channel of sync items.
	// The function should run the sync stream in a goroutine and push items to the channel.
	// It should return a cancel function to stop the stream.
	StartSync func(ctx context.Context, t *testing.T) (<-chan SyncItem, func())

	// TriggerUpdate performs an update with the given field value.
	// This should call the actual RPC or service method that triggers sync events.
	TriggerUpdate func(ctx context.Context, t *testing.T, value FieldType)

	// GetActualValue retrieves the actual persisted value from the database/collection.
	// This is used to verify the value was actually saved, not just synced.
	GetActualValue func(ctx context.Context, t *testing.T) FieldType

	// ExtractSyncedValue extracts the field value from a sync item.
	// Returns the value and whether it was present in the sync message.
	ExtractSyncedValue func(t *testing.T, syncItem SyncItem) (value FieldType, present bool)

	// CompareValues compares two values for equality.
	// If nil, uses require.Equal.
	CompareValues func(t *testing.T, expected, actual FieldType)
}

// VerifyZeroValueSync verifies that zero values are correctly handled in sync events.
// This is a regression test framework for the common bug pattern:
//
//	if value != 0 {  // BUG: excludes zero values!
//	    update.Field = &value
//	}
//
// The test:
// 1. Sets a non-zero value and verifies it syncs
// 2. Sets zero value and verifies it ALSO syncs (this is what the bug would break)
// 3. Verifies the actual persisted value matches
func VerifyZeroValueSync[FieldType any, SyncItem any](
	t *testing.T,
	cfg ZeroValueSyncTestConfig[FieldType, SyncItem],
	testCase ZeroValueTestCase[FieldType],
) {
	t.Helper()

	if cfg.Setup == nil {
		require.FailNow(t, "Setup function is required")
	}

	ctx, cleanup := cfg.Setup(t)
	if cleanup != nil {
		defer cleanup()
	}

	compareFunc := cfg.CompareValues
	if compareFunc == nil {
		compareFunc = func(t *testing.T, expected, actual FieldType) {
			require.Equal(t, expected, actual)
		}
	}

	// Start sync stream
	syncCh, cancelSync := cfg.StartSync(ctx, t)
	defer cancelSync()

	// Give subscription time to establish
	time.Sleep(100 * time.Millisecond)

	t.Run("non-zero value syncs correctly", func(t *testing.T) {
		// 1. Set initial (non-zero) value
		cfg.TriggerUpdate(ctx, t, testCase.InitialValue)

		// 2. Wait for sync event
		var syncItem SyncItem
		select {
		case item := <-syncCh:
			syncItem = item
		case <-time.After(2 * time.Second):
			require.FailNow(t, "Timeout waiting for sync event after setting non-zero value")
		}

		// 3. Extract and verify synced value
		syncedValue, present := cfg.ExtractSyncedValue(t, syncItem)
		require.True(t, present, "Field should be present in sync event for non-zero value")
		compareFunc(t, testCase.InitialValue, syncedValue)

		// 4. Verify actual persisted value
		actualValue := cfg.GetActualValue(ctx, t)
		compareFunc(t, testCase.InitialValue, actualValue)
	})

	t.Run("zero value syncs correctly", func(t *testing.T) {
		// 1. Set zero value
		cfg.TriggerUpdate(ctx, t, testCase.ZeroValue)

		// 2. Wait for sync event
		var syncItem SyncItem
		select {
		case item := <-syncCh:
			syncItem = item
		case <-time.After(2 * time.Second):
			require.FailNow(t, "Timeout waiting for sync event after setting zero value - this is the bug!")
		}

		// 3. Extract and verify synced value (THIS IS THE CRITICAL CHECK)
		syncedValue, present := cfg.ExtractSyncedValue(t, syncItem)
		require.True(t, present,
			"Field MUST be present in sync event even for zero value! "+
				"This indicates a bug where `if value != 0` incorrectly excludes zero values.")
		compareFunc(t, testCase.ZeroValue, syncedValue)

		// 4. Verify actual persisted value
		actualValue := cfg.GetActualValue(ctx, t)
		compareFunc(t, testCase.ZeroValue, actualValue)
	})
}

// VerifyZeroValueSyncMultiple runs VerifyZeroValueSync for multiple field types/values.
// Useful for testing all numeric fields on a model.
func VerifyZeroValueSyncMultiple[FieldType any, SyncItem any](
	t *testing.T,
	cfgFactory func(t *testing.T) ZeroValueSyncTestConfig[FieldType, SyncItem],
	testCases []ZeroValueTestCase[FieldType],
) {
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			cfg := cfgFactory(t)
			VerifyZeroValueSync(t, cfg, tc)
		})
	}
}

// CommonZeroValueTests provides pre-built test cases for common field types.
var CommonZeroValueTests = struct {
	Int32Cases []ZeroValueTestCase[int32]
	Int64Cases []ZeroValueTestCase[int64]
}{
	Int32Cases: []ZeroValueTestCase[int32]{
		{Name: "positive_to_zero", InitialValue: 5, ZeroValue: 0},
		{Name: "large_to_zero", InitialValue: 1000, ZeroValue: 0},
	},
	Int64Cases: []ZeroValueTestCase[int64]{
		{Name: "positive_to_zero", InitialValue: 5, ZeroValue: 0},
		{Name: "large_to_zero", InitialValue: 1000, ZeroValue: 0},
	},
}
