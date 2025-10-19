package node_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"the-dev-tools/server/pkg/flow/node"
	nodetesting "the-dev-tools/server/pkg/flow/node/testing"
	"the-dev-tools/server/pkg/idwrap"
)

// TestNodeVariableOperations_FrameworkTests demonstrates using the testing framework for node variable operations
func TestNodeVariableOperations_FrameworkTests(t *testing.T) {
	// Test cases for node variable operations
	testCases := []nodetesting.NodeTestCase{
		{
			Name: "Write and Read Node Variable",
			TestFunc: func(t *testing.T, ctx *nodetesting.TestContext, testNode node.FlowNode) {
				req := ctx.CreateNodeRequest(idwrap.NewNow(), "test-node")

				key := "testKey"
				value := "testValue"
				nodeName := "test-node"

				// Write variable
				err := node.WriteNodeVar(req, nodeName, key, value)
				require.NoError(t, err, "Should write node variable without error")

				// Read variable
				storedValue, err := node.ReadNodeVar(req, nodeName, key)
				require.NoError(t, err, "Should read node variable without error")
				require.Equal(t, value, storedValue, "Stored value should match original")

				t.Log("Write and read node variable test passed")
			},
		},
		{
			Name: "Read Raw Variable",
			TestFunc: func(t *testing.T, ctx *nodetesting.TestContext, testNode node.FlowNode) {
				req := ctx.CreateNodeRequest(idwrap.NewNow(), "test-node")

				key := "testKey"
				value := "testValue"

				// Write raw variable directly
				req.VarMap[key] = value

				// Read raw variable
				storedValue, err := node.ReadVarRaw(req, key)
				require.NoError(t, err, "Should read raw variable without error")
				require.Equal(t, value, storedValue, "Stored value should match original")

				t.Log("Read raw variable test passed")
			},
		},
		{
			Name: "Read Node Variable with Pre-existing Data",
			TestFunc: func(t *testing.T, ctx *nodetesting.TestContext, testNode node.FlowNode) {
				req := ctx.CreateNodeRequest(idwrap.NewNow(), "test-node")

				key := "testKey"
				value := "testValue"
				nodeName := "test-node"

				// Pre-populate node data
				req.VarMap[nodeName] = map[string]any{key: value}

				// Read node variable
				storedValue, err := node.ReadNodeVar(req, nodeName, key)
				require.NoError(t, err, "Should read node variable without error")
				require.Equal(t, value, storedValue, "Stored value should match original")

				t.Log("Read node variable with pre-existing data test passed")
			},
		},
		{
			Name: "Read Node Variable - Node Not Found",
			TestFunc: func(t *testing.T, ctx *nodetesting.TestContext, testNode node.FlowNode) {
				req := ctx.CreateNodeRequest(idwrap.NewNow(), "test-node")

				key := "testKey"
				nodeName := "non-existent-node"

				// Try to read from non-existent node
				_, err := node.ReadNodeVar(req, nodeName, key)
				require.Error(t, err, "Should return error for non-existent node")
				require.Equal(t, node.ErrVarNodeNotFound, err, "Should return specific node not found error")

				t.Log("Read node variable - node not found test passed")
			},
		},
		{
			Name: "Read Node Variable - Key Not Found",
			TestFunc: func(t *testing.T, ctx *nodetesting.TestContext, testNode node.FlowNode) {
				req := ctx.CreateNodeRequest(idwrap.NewNow(), "test-node")

				key := "nonExistentKey"
				nodeName := "test-node"

				// Create empty node data
				req.VarMap[nodeName] = map[string]any{}

				// Try to read non-existent key
				_, err := node.ReadNodeVar(req, nodeName, key)
				require.Error(t, err, "Should return error for non-existent key")
				require.Contains(t, err.Error(), "key not found", "Error should mention key not found")

				t.Log("Read node variable - key not found test passed")
			},
		},
		{
			Name: "Write Node Variable Bulk",
			TestFunc: func(t *testing.T, ctx *nodetesting.TestContext, testNode node.FlowNode) {
				req := ctx.CreateNodeRequest(idwrap.NewNow(), "test-node")

				nodeName := "test-node"
				data := map[string]any{
					"key1": "value1",
					"key2": 42,
					"key3": true,
				}

				// Write bulk data
				err := node.WriteNodeVarBulk(req, nodeName, data)
				require.NoError(t, err, "Should write bulk data without error")

				// Verify each key-value pair
				for key, expectedValue := range data {
					storedValue, err := node.ReadNodeVar(req, nodeName, key)
					require.NoError(t, err, "Should read key %s without error", key)
					require.Equal(t, expectedValue, storedValue, "Value for key %s should match", key)
				}

				t.Log("Write node variable bulk test passed")
			},
		},
		{
			Name: "Complex Data Types",
			TestFunc: func(t *testing.T, ctx *nodetesting.TestContext, testNode node.FlowNode) {
				req := ctx.CreateNodeRequest(idwrap.NewNow(), "test-node")

				nodeName := "test-node"

				// Test complex data types
				testCases := []struct {
					key   string
					value any
				}{
					{"string", "test string"},
					{"integer", 42},
					{"float", 3.14},
					{"boolean", true},
					{"slice", []string{"a", "b", "c"}},
					{"map", map[string]any{"nested": "value"}},
				}

				for _, tc := range testCases {
					// Write
					err := node.WriteNodeVar(req, nodeName, tc.key, tc.value)
					require.NoError(t, err, "Should write %s without error", tc.key)

					// Read
					storedValue, err := node.ReadNodeVar(req, nodeName, tc.key)
					require.NoError(t, err, "Should read %s without error", tc.key)
					require.Equal(t, tc.value, storedValue, "Value for %s should match", tc.key)
				}

				t.Log("Complex data types test passed")
			},
		},
		{
			Name: "Variable Overwrite",
			TestFunc: func(t *testing.T, ctx *nodetesting.TestContext, testNode node.FlowNode) {
				req := ctx.CreateNodeRequest(idwrap.NewNow(), "test-node")

				key := "testKey"
				nodeName := "test-node"
				initialValue := "initial"
				newValue := "updated"

				// Write initial value
				err := node.WriteNodeVar(req, nodeName, key, initialValue)
				require.NoError(t, err, "Should write initial value")

				// Verify initial value
				storedValue, err := node.ReadNodeVar(req, nodeName, key)
				require.NoError(t, err, "Should read initial value")
				require.Equal(t, initialValue, storedValue, "Initial value should match")

				// Overwrite with new value
				err = node.WriteNodeVar(req, nodeName, key, newValue)
				require.NoError(t, err, "Should overwrite value")

				// Verify new value
				storedValue, err = node.ReadNodeVar(req, nodeName, key)
				require.NoError(t, err, "Should read new value")
				require.Equal(t, newValue, storedValue, "New value should match")

				t.Log("Variable overwrite test passed")
			},
		},
	}

	// Create a dummy node for testing (we're testing variable operations, not node behavior)
	dummyNode := &DummyFlowNode{id: idwrap.NewNow(), name: "dummy"}

	// Run all test cases using the framework
	nodetesting.RunNodeTests(t, dummyNode, testCases)
}

// DummyFlowNode is a minimal implementation of node.FlowNode for testing variable operations
type DummyFlowNode struct {
	id   idwrap.IDWrap
	name string
}

func (d *DummyFlowNode) GetID() idwrap.IDWrap { return d.id }
func (d *DummyFlowNode) GetName() string      { return d.name }
func (d *DummyFlowNode) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	return node.FlowNodeResult{NextNodeID: []idwrap.IDWrap{}}
}
func (d *DummyFlowNode) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	resultChan <- node.FlowNodeResult{NextNodeID: []idwrap.IDWrap{}}
}

// TestNodeVariableOperations_FrameworkIntegration demonstrates integration with test context
func TestNodeVariableOperations_FrameworkIntegration(t *testing.T) {
	ctx := nodetesting.NewTestContext(t)

	t.Run("Using Test Context Helpers", func(t *testing.T) {
		req := ctx.CreateNodeRequest(idwrap.NewNow(), "test-node")

		// Use test context for variable operations
		err := node.WriteNodeVar(req, "test-node", "framework-key", "framework-value")
		require.NoError(t, err, "Should write variable using test context")

		value, err := node.ReadNodeVar(req, "test-node", "framework-key")
		require.NoError(t, err, "Should read variable using test context")
		require.Equal(t, "framework-value", value, "Value should match using test context")

		t.Log("Test context integration test passed")
	})

	t.Run("Custom Test Context Options", func(t *testing.T) {
		// Create request with custom variables
		req := ctx.CreateNodeRequest(idwrap.NewNow(), "test-node", nodetesting.NodeRequestOptions{
			VarMap: map[string]any{
				"pre-existing": "value",
			},
		})

		// Verify pre-existing variable
		value, err := node.ReadVarRaw(req, "pre-existing")
		require.NoError(t, err, "Should read pre-existing variable")
		require.Equal(t, "value", value, "Pre-existing value should match")

		// Add new variable
		err = node.WriteNodeVar(req, "new-node", "new-key", "new-value")
		require.NoError(t, err, "Should write new variable")

		newValue, err := node.ReadNodeVar(req, "new-node", "new-key")
		require.NoError(t, err, "Should read new variable")
		require.Equal(t, "new-value", newValue, "New value should match")

		t.Log("Custom test context options test passed")
	})
}

// TestNodeVariableOperations_ErrorHandling demonstrates comprehensive error handling
func TestNodeVariableOperations_ErrorHandling(t *testing.T) {
	ctx := nodetesting.NewTestContext(t)

	t.Run("Nil VarMap", func(t *testing.T) {
		req := &node.FlowNodeRequest{
			VarMap:        nil,
			ReadWriteLock: &sync.RWMutex{},
		}

		// Should handle nil VarMap gracefully
		err := node.WriteNodeVar(req, "test", "key", "value")
		require.Error(t, err, "Should handle nil VarMap with error")

		t.Log("Nil VarMap error handling test passed")
	})

	t.Run("Invalid Node Data Type", func(t *testing.T) {
		req := ctx.CreateNodeRequest(idwrap.NewNow(), "test-node")

		// Set invalid node data type (not a map)
		req.VarMap["invalid-node"] = "not-a-map"

		// Should handle invalid node data type
		_, err := node.ReadNodeVar(req, "invalid-node", "key")
		require.Error(t, err, "Should handle invalid node data type with error")

		t.Log("Invalid node data type error handling test passed")
	})
}
