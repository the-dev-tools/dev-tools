package tracking

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVariableTracker_TrackReadWrite(t *testing.T) {
	tracker := NewVariableTracker()

	// Test tracking reads
	tracker.TrackRead("key1", "value1")
	tracker.TrackRead("key2", 42)
	tracker.TrackRead("key3", map[string]interface{}{"nested": "data"})

	// Test tracking writes
	tracker.TrackWrite("output1", "result1")
	tracker.TrackWrite("output2", []int{1, 2, 3})

	// Verify reads
	readVars := tracker.GetReadVars()
	require.Len(t, readVars, 3, "Expected 3 read variables")
	require.Equal(t, "value1", readVars["key1"])
	require.Equal(t, 42, readVars["key2"])

	// Verify writes
	writtenVars := tracker.GetWrittenVars()
	require.Len(t, writtenVars, 2, "Expected 2 written variables")
	require.Equal(t, "result1", writtenVars["output1"])
}

func TestVariableTracker_NilTracker(t *testing.T) {
	var tracker *VariableTracker = nil

	// Should not panic
	tracker.TrackRead("key", "value")
	tracker.TrackWrite("key", "value")

	// Should return empty maps
	reads := tracker.GetReadVars()
	writes := tracker.GetWrittenVars()

	require.Empty(t, reads, "Expected empty reads from nil tracker")
	require.Empty(t, writes, "Expected empty writes from nil tracker")
}

func TestVariableTracker_DeepCopy(t *testing.T) {
	tracker := NewVariableTracker()

	// Create complex nested structure
	original := map[string]interface{}{
		"nested": map[string]interface{}{
			"deep": []interface{}{1, 2, 3},
		},
	}

	tracker.TrackRead("complex", original)

	// Modify the original
	if nestedMap, ok := original["nested"].(map[string]interface{}); ok {
		nestedMap["modified"] = true
	}

	// Verify tracked value wasn't modified
	readVars := tracker.GetReadVars()
	trackedVal := readVars["complex"]
	trackedMap, ok := trackedVal.(map[string]interface{})
	require.True(t, ok, "Tracked value is not a map")
	nestedMap, ok := trackedMap["nested"].(map[string]interface{})
	require.True(t, ok, "Nested map not found")
	_, exists := nestedMap["modified"]
	require.False(t, exists, "Tracked value was modified - deep copy failed")
}

func TestVariableTracker_Concurrent(t *testing.T) {
	tracker := NewVariableTracker()
	numGoroutines := 10
	readsPerGoroutine := 50

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 2) // readers and writers

	// Concurrent readers
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < readsPerGoroutine; j++ {
				key := fmt.Sprintf("read_%d_%d", goroutineID, j)
				value := fmt.Sprintf("value_%d_%d", goroutineID, j)
				tracker.TrackRead(key, value)
			}
		}(i)
	}

	// Concurrent writers
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < readsPerGoroutine; j++ {
				key := fmt.Sprintf("write_%d_%d", goroutineID, j)
				value := fmt.Sprintf("result_%d_%d", goroutineID, j)
				tracker.TrackWrite(key, value)
			}
		}(i)
	}

	wg.Wait()

	// Verify all operations were tracked
	readVars := tracker.GetReadVars()
	writtenVars := tracker.GetWrittenVars()

	expectedCount := numGoroutines * readsPerGoroutine
	require.Len(t, readVars, expectedCount, "Expected %d read variables", expectedCount)
	require.Len(t, writtenVars, expectedCount, "Expected %d written variables", expectedCount)
}

func TestVariableTracker_ClearWritesWithPrefix(t *testing.T) {
	tracker := NewVariableTracker()

	// Track various writes with different prefixes
	tracker.TrackWrite("ai_1.userId", 42)
	tracker.TrackWrite("ai_1.userName", "alice")
	tracker.TrackWrite("ai_1.deep.nested", "value")
	tracker.TrackWrite("http_1.response.status", 200)
	tracker.TrackWrite("http_1.response.body", "hello")
	tracker.TrackWrite("other", "data")

	// Clear writes with prefix "ai_1."
	tracker.ClearWritesWithPrefix("ai_1.")

	// Verify ai_1.* writes are cleared
	writtenVars := tracker.GetWrittenVars()
	require.Len(t, writtenVars, 3, "Expected 3 written variables after clearing ai_1.*")

	_, exists := writtenVars["ai_1.userId"]
	require.False(t, exists, "ai_1.userId should be cleared")
	_, exists = writtenVars["ai_1.userName"]
	require.False(t, exists, "ai_1.userName should be cleared")
	_, exists = writtenVars["ai_1.deep.nested"]
	require.False(t, exists, "ai_1.deep.nested should be cleared")

	// Verify other writes remain
	require.Equal(t, 200, writtenVars["http_1.response.status"])
	require.Equal(t, "hello", writtenVars["http_1.response.body"])
	require.Equal(t, "data", writtenVars["other"])
}

func TestVariableTracker_ClearWritesWithPrefixNilTracker(t *testing.T) {
	var tracker *VariableTracker = nil

	// Should not panic
	tracker.ClearWritesWithPrefix("ai_1.")
}

func TestTrackingEnv_Get(t *testing.T) {
	originalEnv := map[string]any{
		"var1": "value1",
		"var2": 42,
		"var3": map[string]interface{}{"nested": "data"},
	}

	tracker := NewVariableTracker()
	trackingEnv := NewTrackingEnv(originalEnv, tracker)

	// Test successful get
	value, exists := trackingEnv.Get("var1")
	require.True(t, exists, "Expected var1 to exist")
	require.Equal(t, "value1", value)

	// Test non-existent key
	_, exists = trackingEnv.Get("nonexistent")
	require.False(t, exists, "Expected nonexistent key to not exist")

	// Verify tracking occurred
	readVars := tracker.GetReadVars()
	require.Len(t, readVars, 1, "Expected 1 tracked read")
	require.Equal(t, "value1", readVars["var1"])

	// Non-existent key should not be tracked
	_, exists = readVars["nonexistent"]
	require.False(t, exists, "Non-existent key should not be tracked")
}

func TestTrackingEnv_GetMap(t *testing.T) {
	originalEnv := map[string]any{
		"var1": "value1",
		"var2": 42,
	}

	tracker := NewVariableTracker()
	trackingEnv := NewTrackingEnv(originalEnv, tracker)

	// GetMap should return the original environment
	envMap := trackingEnv.GetMap()
	require.Len(t, envMap, 2, "Expected 2 environment variables")
	require.Equal(t, "value1", envMap["var1"])
	require.Equal(t, 42, envMap["var2"])

	// GetMap should not trigger tracking
	readVars := tracker.GetReadVars()
	require.Empty(t, readVars, "Expected no tracked reads from GetMap")
}

func TestTrackingEnv_TrackAllVariables(t *testing.T) {
	originalEnv := map[string]any{
		"var1": "value1",
		"var2": 42,
		"var3": map[string]interface{}{"nested": "data"},
	}

	tracker := NewVariableTracker()
	trackingEnv := NewTrackingEnv(originalEnv, tracker)

	// TrackAllVariables should track all environment variables
	trackingEnv.TrackAllVariables()

	readVars := tracker.GetReadVars()

	require.Len(t, readVars, 3, "Expected 3 tracked reads")
	require.Equal(t, "value1", readVars["var1"])
	require.Equal(t, 42, readVars["var2"])

	nestedMap, ok := readVars["var3"].(map[string]interface{})
	require.True(t, ok, "Expected var3 to be a map, got %T", readVars["var3"])
	require.Equal(t, "data", nestedMap["nested"])
}

func TestTrackingEnv_NilEnvironment(t *testing.T) {
	tracker := NewVariableTracker()
	trackingEnv := NewTrackingEnv(nil, tracker)

	// Get from nil environment
	_, exists := trackingEnv.Get("key")
	require.False(t, exists, "Expected no key to exist in nil environment")

	// GetMap from nil environment
	envMap := trackingEnv.GetMap()
	require.Empty(t, envMap, "Expected empty map from nil environment")

	// No tracking should occur
	readVars := tracker.GetReadVars()
	require.Empty(t, readVars, "Expected no tracked reads")
}

func TestTrackingEnv_NilTracker(t *testing.T) {
	originalEnv := map[string]any{
		"var1": "value1",
	}

	trackingEnv := NewTrackingEnv(originalEnv, nil)

	// Should still work without tracker
	value, exists := trackingEnv.Get("var1")
	require.True(t, exists, "Get should work even with nil tracker")
	require.Equal(t, "value1", value, "Get should work even with nil tracker")

	// GetMap should work
	envMap := trackingEnv.GetMap()
	require.Len(t, envMap, 1, "GetMap should work even with nil tracker")
}

func BenchmarkVariableTracker_TrackRead(b *testing.B) {
	tracker := NewVariableTracker()
	value := map[string]interface{}{
		"nested": []interface{}{1, 2, 3, 4, 5},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key_%d", i%100) // Reuse some keys
		tracker.TrackRead(key, value)
	}
}

func BenchmarkVariableTracker_TrackWrite(b *testing.B) {
	tracker := NewVariableTracker()
	value := []interface{}{
		map[string]interface{}{"id": 1, "name": "item1"},
		map[string]interface{}{"id": 2, "name": "item2"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("output_%d", i%100)
		tracker.TrackWrite(key, value)
	}
}
