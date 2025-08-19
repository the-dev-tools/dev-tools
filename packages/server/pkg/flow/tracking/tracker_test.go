package tracking

import (
	"fmt"
	"sync"
	"testing"
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
	if len(readVars) != 3 {
		t.Errorf("Expected 3 read variables, got %d", len(readVars))
	}
	if readVars["key1"] != "value1" {
		t.Errorf("Expected key1='value1', got %v", readVars["key1"])
	}
	if readVars["key2"] != 42 {
		t.Errorf("Expected key2=42, got %v", readVars["key2"])
	}

	// Verify writes
	writtenVars := tracker.GetWrittenVars()
	if len(writtenVars) != 2 {
		t.Errorf("Expected 2 written variables, got %d", len(writtenVars))
	}
	if writtenVars["output1"] != "result1" {
		t.Errorf("Expected output1='result1', got %v", writtenVars["output1"])
	}
}

func TestVariableTracker_NilTracker(t *testing.T) {
	var tracker *VariableTracker = nil

	// Should not panic
	tracker.TrackRead("key", "value")
	tracker.TrackWrite("key", "value")

	// Should return empty maps
	reads := tracker.GetReadVars()
	writes := tracker.GetWrittenVars()

	if len(reads) != 0 {
		t.Errorf("Expected empty reads from nil tracker, got %d items", len(reads))
	}
	if len(writes) != 0 {
		t.Errorf("Expected empty writes from nil tracker, got %d items", len(writes))
	}
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
	if trackedMap, ok := trackedVal.(map[string]interface{}); ok {
		if nestedMap, ok := trackedMap["nested"].(map[string]interface{}); ok {
			if _, exists := nestedMap["modified"]; exists {
				t.Error("Tracked value was modified - deep copy failed")
			}
		}
	} else {
		t.Error("Tracked value is not a map")
	}
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
	if len(readVars) != expectedCount {
		t.Errorf("Expected %d read variables, got %d", expectedCount, len(readVars))
	}
	if len(writtenVars) != expectedCount {
		t.Errorf("Expected %d written variables, got %d", expectedCount, len(writtenVars))
	}
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
	if !exists {
		t.Error("Expected var1 to exist")
	}
	if value != "value1" {
		t.Errorf("Expected var1='value1', got %v", value)
	}

	// Test non-existent key
	_, exists = trackingEnv.Get("nonexistent")
	if exists {
		t.Error("Expected nonexistent key to not exist")
	}

	// Verify tracking occurred
	readVars := tracker.GetReadVars()
	if len(readVars) != 1 {
		t.Errorf("Expected 1 tracked read, got %d", len(readVars))
	}
	if readVars["var1"] != "value1" {
		t.Errorf("Expected tracked var1='value1', got %v", readVars["var1"])
	}

	// Non-existent key should not be tracked
	if _, exists := readVars["nonexistent"]; exists {
		t.Error("Non-existent key should not be tracked")
	}
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
	if len(envMap) != 2 {
		t.Errorf("Expected 2 environment variables, got %d", len(envMap))
	}
	if envMap["var1"] != "value1" {
		t.Errorf("Expected var1='value1', got %v", envMap["var1"])
	}
	if envMap["var2"] != 42 {
		t.Errorf("Expected var2=42, got %v", envMap["var2"])
	}

	// GetMap should not trigger tracking
	readVars := tracker.GetReadVars()
	if len(readVars) != 0 {
		t.Errorf("Expected no tracked reads from GetMap, got %d", len(readVars))
	}
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

	if len(readVars) != 3 {
		t.Errorf("Expected 3 tracked reads, got %d", len(readVars))
	}

	if readVars["var1"] != "value1" {
		t.Errorf("Expected var1='value1', got %v", readVars["var1"])
	}

	if readVars["var2"] != 42 {
		t.Errorf("Expected var2=42, got %v", readVars["var2"])
	}

	if nestedMap, ok := readVars["var3"].(map[string]interface{}); ok {
		if nestedMap["nested"] != "data" {
			t.Errorf("Expected nested data, got %v", nestedMap["nested"])
		}
	} else {
		t.Errorf("Expected var3 to be a map, got %T", readVars["var3"])
	}
}

func TestTrackingEnv_NilEnvironment(t *testing.T) {
	tracker := NewVariableTracker()
	trackingEnv := NewTrackingEnv(nil, tracker)

	// Get from nil environment
	_, exists := trackingEnv.Get("key")
	if exists {
		t.Error("Expected no key to exist in nil environment")
	}

	// GetMap from nil environment
	envMap := trackingEnv.GetMap()
	if len(envMap) != 0 {
		t.Errorf("Expected empty map from nil environment, got %d items", len(envMap))
	}

	// No tracking should occur
	readVars := tracker.GetReadVars()
	if len(readVars) != 0 {
		t.Errorf("Expected no tracked reads, got %d", len(readVars))
	}
}

func TestTrackingEnv_NilTracker(t *testing.T) {
	originalEnv := map[string]any{
		"var1": "value1",
	}

	trackingEnv := NewTrackingEnv(originalEnv, nil)

	// Should still work without tracker
	value, exists := trackingEnv.Get("var1")
	if !exists || value != "value1" {
		t.Error("Get should work even with nil tracker")
	}

	// GetMap should work
	envMap := trackingEnv.GetMap()
	if len(envMap) != 1 {
		t.Error("GetMap should work even with nil tracker")
	}
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
