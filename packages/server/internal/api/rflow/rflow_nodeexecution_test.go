package rflow

import (
	"encoding/json"
	"fmt"
	"testing"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnodeexecution"
)

func TestNodeExecutionInputTracking(t *testing.T) {
	// This test verifies that node executions correctly capture
	// the actual data that nodes read during execution

	// Test scenarios:
	// 1. Node reads from another node's output
	// 2. Node reads flow variables
	// 3. Node reads both node outputs and flow variables
	// 4. Node doesn't read anything

	t.Run("captures node output reads", func(t *testing.T) {
		// When a node reads another node's output using ReadNodeVar,
		// it should be captured in the InputData field

		// TODO: Create a test flow with two nodes where the second
		// reads from the first, then verify the InputData
	})

	t.Run("captures flow variable reads", func(t *testing.T) {
		// When a node reads flow variables using ReadVarRaw,
		// it should be captured in the InputData field

		// TODO: Create a test flow with a node that reads flow vars
	})

	t.Run("captures mixed reads", func(t *testing.T) {
		// When a node reads both node outputs and flow variables,
		// both should be captured in the InputData field

		// TODO: Create a test with mixed reads
	})

	t.Run("empty input for no reads", func(t *testing.T) {
		// When a node doesn't read any data,
		// InputData should be an empty object

		// TODO: Create a test with a node that doesn't read anything
	})
}

func TestNodeExecutionDataCompression(t *testing.T) {
	// Test that large input/output data is properly compressed

	t.Run("compresses large input data", func(t *testing.T) {
		// Create a node execution with large input data
		nodeExec := &mnodeexecution.NodeExecution{
			ID:     idwrap.NewNow(),
			NodeID: idwrap.NewNow(),
			Name:   "Test Execution",
			State:  mnnode.NODE_STATE_SUCCESS,
		}

		// Create large input data (>1KB to trigger compression)
		largeData := make(map[string]any)
		for i := 0; i < 100; i++ {
			key := fmt.Sprintf("key_%d", i)
			largeData[key] = fmt.Sprintf("This is a long value for testing compression of node execution data. Iteration %d", i)
		}

		inputJSON, err := json.Marshal(largeData)
		if err != nil {
			t.Fatalf("Failed to marshal test data: %v", err)
		}

		// Set the input data using the compression helper
		err = nodeExec.SetInputJSON(inputJSON)
		if err != nil {
			t.Fatalf("Failed to set input JSON: %v", err)
		}

		// Verify compression was applied
		if nodeExec.InputDataCompressType == 0 {
			t.Error("Expected input data to be compressed, but compression type is 0")
		}

		// Verify we can retrieve the original data
		retrievedJSON, err := nodeExec.GetInputJSON()
		if err != nil {
			t.Fatalf("Failed to get input JSON: %v", err)
		}

		var retrievedData map[string]any
		err = json.Unmarshal(retrievedJSON, &retrievedData)
		if err != nil {
			t.Fatalf("Failed to unmarshal retrieved data: %v", err)
		}

		if len(retrievedData) != len(largeData) {
			t.Errorf("Retrieved data has different length: expected %d, got %d", len(largeData), len(retrievedData))
		}
	})
}

func TestNodeExecutionThreadSafety(t *testing.T) {
	// Test that read tracking is thread-safe when multiple nodes
	// execute concurrently

	t.Run("concurrent node executions", func(t *testing.T) {
		// TODO: Create a flow with multiple parallel nodes
		// that all read from shared data and verify each
		// node's InputData is correctly isolated
	})
}

func TestNodeExecutionWithDifferentNodeTypes(t *testing.T) {
	// Test input tracking for different node types

	t.Run("REQUEST node input tracking", func(t *testing.T) {
		// REQUEST nodes might read flow variables or other node outputs
		// TODO: Test REQUEST node input tracking
	})

	t.Run("JAVASCRIPT node input tracking", func(t *testing.T) {
		// JS nodes typically read from multiple sources
		// TODO: Test JS node input tracking
	})

	t.Run("CONDITION node input tracking", func(t *testing.T) {
		// Condition nodes read data to evaluate conditions
		// TODO: Test CONDITION node input tracking
	})

	t.Run("FOR_EACH node input tracking", func(t *testing.T) {
		// FOR_EACH nodes read arrays to iterate over
		// TODO: Test FOR_EACH node input tracking
	})
}
