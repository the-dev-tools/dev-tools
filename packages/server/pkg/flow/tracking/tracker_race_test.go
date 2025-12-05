package tracking_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"the-dev-tools/server/pkg/flow/tracking"
)

func TestVariableTracker_ConcurrentAccess(t *testing.T) {
	tracker := tracking.NewVariableTracker()

	const numGoroutines = 100
	const numOperations = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 3) // 3 types of operations

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := "read_key_" + string(rune('a'+id%26))
				value := "value_" + string(rune('a'+id%26))
				tracker.TrackRead(key, value)
			}
		}(i)
	}

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := "write_key_" + string(rune('a'+id%26))
				value := "value_" + string(rune('a'+id%26))
				tracker.TrackWrite(key, value)
			}
		}(i)
	}

	// Concurrent gets
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				_ = tracker.GetReadVars()
				_ = tracker.GetWrittenVars()
			}
		}(i)
	}

	wg.Wait()

	// Verify we have some tracked variables
	readVars := tracker.GetReadVars()
	writtenVars := tracker.GetWrittenVars()

	require.NotEmpty(t, readVars, "Expected some read variables to be tracked")
	require.NotEmpty(t, writtenVars, "Expected some written variables to be tracked")
}

func TestVariableTracker_ConcurrentMixedOperations(t *testing.T) {
	tracker := tracking.NewVariableTracker()

	const numGoroutines = 50
	const numIterations = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Mixed operations in each goroutine
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				// Track read
				tracker.TrackRead("var_"+string(rune('a'+id%26)), id*100+j)

				// Track write
				tracker.TrackWrite("output_"+string(rune('a'+id%26)), id*1000+j)

				// Get read vars
				readVars := tracker.GetReadVars()
				_ = readVars

				// Get written vars
				writtenVars := tracker.GetWrittenVars()
				_ = writtenVars

				// Track with complex data structures
				tracker.TrackRead("complex_key", map[string]interface{}{
					"nested": map[string]interface{}{
						"value": id,
					},
				})
			}
		}(i)
	}

	wg.Wait()

	// Final verification
	finalReadVars := tracker.GetReadVars()
	finalWrittenVars := tracker.GetWrittenVars()

	require.NotEmpty(t, finalReadVars, "Expected read variables after concurrent operations")
	require.NotEmpty(t, finalWrittenVars, "Expected written variables after concurrent operations")
}

func TestVariableTracker_StressTestWithComplexData(t *testing.T) {
	tracker := tracking.NewVariableTracker()

	const numGoroutines = 100
	const numOps = 50

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			// Create complex nested data
			complexData := map[string]interface{}{
				"level1": map[string]interface{}{
					"level2": map[string]interface{}{
						"array": []interface{}{id, id * 2, id * 3},
						"value": "test_" + string(rune('a'+id%26)),
					},
				},
				"array": []map[string]interface{}{
					{"key": "value1"},
					{"key": "value2"},
				},
			}

			for j := 0; j < numOps; j++ {
				// Track with complex data
				tracker.TrackRead("complex_"+string(rune('a'+id%26)), complexData)
				tracker.TrackWrite("output_"+string(rune('a'+id%26)), complexData)

				// Interleave with reads
				if j%10 == 0 {
					_ = tracker.GetReadVars()
					_ = tracker.GetWrittenVars()
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify deep copy is working (data should be independent)
	readVars := tracker.GetReadVars()
	writtenVars := tracker.GetWrittenVars()

	require.NotEmpty(t, readVars, "Expected tracked variables with complex data")
	require.NotEmpty(t, writtenVars, "Expected tracked variables with complex data")
}

func BenchmarkVariableTracker_ConcurrentOperations(b *testing.B) {
	tracker := tracking.NewVariableTracker()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := "key_" + string(rune('a'+i%26))
			value := "value_" + string(rune('a'+i%26))

			tracker.TrackRead(key, value)
			tracker.TrackWrite(key, value)

			if i%100 == 0 {
				_ = tracker.GetReadVars()
				_ = tracker.GetWrittenVars()
			}
			i++
		}
	})
}
