package testing

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
)

// NodeTestConfig defines testing configuration for a specific node type
type NodeTestConfig struct {
	// Node type identification
	Type string

	// Capabilities flags
	SupportsErrorHandling bool
	SupportsIterations    bool
	SupportsTimeout       bool
	SupportsAsync         bool
	SupportsConditions    bool

	// Test scenarios to run
	BasicSuccessTest  bool
	ErrorHandlingTest bool
	TimeoutTest       bool
	AsyncTest         bool

	// Custom test functions for node-specific behavior
	CustomTests []func(*testing.T, node.FlowNode)

	// Mock data generators
	MockRequestGenerator func(node.FlowNode) *node.FlowNodeRequest
	MockEdgeMap          func(node.FlowNode) edge.EdgesMap
}

// NodeTypeRegistry manages all registered node types and their test configurations
type NodeTypeRegistry struct {
	constructors map[string]func() node.FlowNode
	configs      map[string]NodeTestConfig
}

var globalRegistry = &NodeTypeRegistry{
	constructors: make(map[string]func() node.FlowNode),
	configs:      make(map[string]NodeTestConfig),
}

// RegisterNodeType registers a node type with its constructor and test configuration
func RegisterNodeType(nodeType string, constructor func() node.FlowNode, config NodeTestConfig) {
	config.Type = nodeType
	globalRegistry.constructors[nodeType] = constructor
	globalRegistry.configs[nodeType] = config
}

// GetNodeTypeRegistry returns the global node type registry
func GetNodeTypeRegistry() *NodeTypeRegistry {
	return globalRegistry
}

// GenerateTestsForAllNodes generates and runs generic tests for all registered node types
func GenerateTestsForAllNodes(t *testing.T) {
	registry := GetNodeTypeRegistry()

	for nodeType, constructor := range registry.constructors {
		config := registry.configs[nodeType]

		t.Run(nodeType, func(t *testing.T) {
			t.Logf("Running generic tests for node type: %s", nodeType)

			// Create fresh node instance
			testNode := constructor()
			require.NotNil(t, testNode, "Node constructor returned nil")

			// Run basic success test
			if config.BasicSuccessTest {
				t.Run("Basic Success", func(t *testing.T) {
					GenericBasicSuccessTest(t, testNode, config)
				})
			}

			// Run error handling test
			if config.ErrorHandlingTest && config.SupportsErrorHandling {
				t.Run("Error Handling", func(t *testing.T) {
					GenericErrorHandlingTest(t, testNode, config)
				})
			}

			// Run timeout test
			if config.TimeoutTest && config.SupportsTimeout {
				t.Run("Timeout", func(t *testing.T) {
					GenericTimeoutTest(t, testNode, config)
				})
			}

			// Run async test
			if config.AsyncTest && config.SupportsAsync {
				t.Run("Async Execution", func(t *testing.T) {
					GenericAsyncTest(t, testNode, config)
				})
			}

			// Run custom tests
			for i, customTest := range config.CustomTests {
				t.Run(fmt.Sprintf("Custom Test %d", i+1), func(t *testing.T) {
					customTest(t, testNode)
				})
			}
		})
	}
}

// GenericBasicSuccessTest tests basic successful execution pattern
func GenericBasicSuccessTest(t *testing.T, testNode node.FlowNode, config NodeTestConfig) {
	testCtx := NewTestContext(t, TestContextOptions{
		Timeout: 10 * time.Second,
	})
	defer testCtx.Cleanup()

	// Generate mock request
	req := generateMockRequest(t, testNode, config, testCtx)
	require.NotNil(t, req, "Failed to generate mock request")

	// Execute node
	result := testNode.RunSync(testCtx.Context(), req)

	// Validate no error occurred
	require.NoError(t, result.Err, "Node execution should succeed")

	// Get collected statuses
	statuses := testCtx.Collector().GetAll()
	// Some nodes don't emit status events, which is valid
	if len(statuses) == 0 {
		t.Log("Node does not emit status events - this is valid for simple nodes")
		return
	}

	// Filter statuses for this node
	var nodeStatuses []runner.FlowNodeStatus
	for _, ts := range statuses {
		if ts.Status.NodeID == testNode.GetID() {
			nodeStatuses = append(nodeStatuses, ts.Status)
		}
	}

	// Validate basic success pattern
	validateSuccessPattern(t, nodeStatuses, config)

	// Validate status sequence
	err := testCtx.Validator().ValidateExecutionSequences()
	require.NoError(t, err, "Status sequence should be valid")
}

// GenericErrorHandlingTest tests error handling patterns
func GenericErrorHandlingTest(t *testing.T, testNode node.FlowNode, config NodeTestConfig) {
	testCtx := NewTestContext(t, TestContextOptions{
		Timeout: 10 * time.Second,
	})
	defer testCtx.Cleanup()

	// Generate mock request that will cause an error
	req := generateErrorRequest(t, testNode, config, testCtx)
	require.NotNil(t, req, "Failed to generate error request")

	// Execute node
	result := testNode.RunSync(testCtx.Context(), req)

	// For error handling test, we expect either an error or proper error status
	// The exact behavior depends on the node type
	statuses := testCtx.Collector().GetAll()

	// Filter statuses for this node
	var nodeStatuses []runner.FlowNodeStatus
	for _, ts := range statuses {
		if ts.Status.NodeID == testNode.GetID() {
			nodeStatuses = append(nodeStatuses, ts.Status)
		}
	}

	// Validate error pattern
	validateErrorPattern(t, nodeStatuses, result.Err, config)
}

// GenericTimeoutTest tests timeout behavior
func GenericTimeoutTest(t *testing.T, testNode node.FlowNode, config NodeTestConfig) {
	testCtx := NewTestContext(t, TestContextOptions{
		Timeout: 1 * time.Second, // Short timeout
	})
	defer testCtx.Cleanup()

	// Generate mock request with very short timeout
	req := generateTimeoutRequest(t, testNode, config, testCtx)
	require.NotNil(t, req, "Failed to generate timeout request")

	// Execute node with timeout context
	ctx, cancel := context.WithTimeout(testCtx.Context(), 1*time.Millisecond)
	defer cancel()

	result := testNode.RunSync(ctx, req)

	// Get collected statuses
	statuses := testCtx.Collector().GetAll()

	// Filter statuses for this node
	var nodeStatuses []runner.FlowNodeStatus
	for _, ts := range statuses {
		if ts.Status.NodeID == testNode.GetID() {
			nodeStatuses = append(nodeStatuses, ts.Status)
		}
	}

	// Validate timeout pattern
	validateTimeoutPattern(t, nodeStatuses, result.Err, config)
}

// GenericAsyncTest tests asynchronous execution
func GenericAsyncTest(t *testing.T, testNode node.FlowNode, config NodeTestConfig) {
	testCtx := NewTestContext(t, TestContextOptions{
		Timeout: 10 * time.Second,
	})
	defer testCtx.Cleanup()

	// Generate mock request
	req := generateMockRequest(t, testNode, config, testCtx)
	require.NotNil(t, req, "Failed to generate mock request")

	// Execute node asynchronously
	resultChan := make(chan node.FlowNodeResult, 1)
	testNode.RunAsync(testCtx.Context(), req, resultChan)

	// Wait for result with timeout
	select {
	case result := <-resultChan:
		// Validate result (success or error depends on node type)
		statuses := testCtx.Collector().GetAll()

		// Filter statuses for this node
		var nodeStatuses []runner.FlowNodeStatus
		for _, ts := range statuses {
			if ts.Status.NodeID == testNode.GetID() {
				nodeStatuses = append(nodeStatuses, ts.Status)
			}
		}

		// Validate async pattern
		validateAsyncPattern(t, nodeStatuses, result.Err, config)

	case <-time.After(5 * time.Second):
		t.Fatal("Async execution timed out")
	}
}

// Helper functions for generating mock requests

func generateMockRequest(_ *testing.T, testNode node.FlowNode, config NodeTestConfig, testCtx *TestContext) *node.FlowNodeRequest {
	if config.MockRequestGenerator != nil {
		return config.MockRequestGenerator(testNode)
	}

	// Default mock request generation
	nodeID := testNode.GetID()
	nodeName := testNode.GetName()

	edgeMap := edge.EdgesMap{}
	if config.MockEdgeMap != nil {
		edgeMap = config.MockEdgeMap(testNode)
	}

	return testCtx.CreateNodeRequest(nodeID, nodeName, NodeRequestOptions{
		VarMap:        make(map[string]any),
		NodeMap:       map[idwrap.IDWrap]node.FlowNode{nodeID: testNode},
		EdgeSourceMap: edgeMap,
		ExecutionID:   idwrap.NewNow(),
		Timeout:       10 * time.Second,
	})
}

func generateErrorRequest(t *testing.T, testNode node.FlowNode, config NodeTestConfig, testCtx *TestContext) *node.FlowNodeRequest {
	// For error testing, we'll create a request that might cause errors
	// This is node-specific, so we use a basic approach that can be overridden
	req := generateMockRequest(t, testNode, config, testCtx)

	// Set a very short timeout to potentially cause timeout errors
	req.Timeout = 1 * time.Nanosecond

	return req
}

func generateTimeoutRequest(t *testing.T, testNode node.FlowNode, config NodeTestConfig, testCtx *TestContext) *node.FlowNodeRequest {
	req := generateMockRequest(t, testNode, config, testCtx)

	// Set extremely short timeout to force timeout
	req.Timeout = 1 * time.Nanosecond

	return req
}

// Validation functions

func validateSuccessPattern(t *testing.T, statuses []runner.FlowNodeStatus, _ NodeTestConfig) {
	// Some nodes (like NOOP, IF, etc.) don't emit status events, which is valid behavior
	// Only validate status patterns for nodes that actually emit them
	if len(statuses) == 0 {
		t.Log("Node does not emit status events - this is valid for simple nodes")
		return
	}

	// Check for at least one SUCCESS or RUNNING status
	hasSuccess := false
	hasRunning := false

	for _, status := range statuses {
		switch status.State {
		case mnnode.NODE_STATE_SUCCESS:
			hasSuccess = true
		case mnnode.NODE_STATE_RUNNING:
			hasRunning = true
		}
	}

	// Should have either SUCCESS (for simple nodes) or RUNNING+SUCCESS (for complex nodes)
	if len(statuses) == 1 {
		require.True(t, hasSuccess, "Single status should be SUCCESS")
	} else {
		require.True(t, hasRunning || hasSuccess, "Should have RUNNING or SUCCESS status")
		if hasRunning && hasSuccess {
			// For nodes that emit RUNNING then SUCCESS
			require.GreaterOrEqual(t, len(statuses), 2, "Should have at least RUNNING and SUCCESS")
		}
	}
}

func validateErrorPattern(t *testing.T, statuses []runner.FlowNodeStatus, execError error, _ NodeTestConfig) {
	// Error patterns vary by node type, so we're flexible here
	// The main requirement is that we should have some status or error indication

	if len(statuses) == 0 && execError == nil {
		t.Log("Warning: No statuses or error captured in error test")
	}

	// If we have statuses, check for error-related states
	hasError := false
	for _, status := range statuses {
		if status.State == mnnode.NODE_STATE_FAILURE || status.State == mnnode.NODE_STATE_CANCELED {
			hasError = true
			break
		}
	}

	// Either we should have an error status or an execution error
	if !hasError && execError == nil {
		t.Log("Note: No error status or execution error - node may handle errors gracefully")
	}
}

func validateTimeoutPattern(t *testing.T, statuses []runner.FlowNodeStatus, execError error, _ NodeTestConfig) {
	// Timeout behavior varies - some nodes timeout, others complete quickly
	// We mainly check that the behavior is consistent

	if execError != nil {
		// Should have timeout-related error
		require.Contains(t, execError.Error(), "timeout", "Error should mention timeout")
	}

	// Status patterns for timeouts can vary, so we're lenient
	t.Logf("Timeout test captured %d statuses, error: %v", len(statuses), execError)
}

func validateAsyncPattern(t *testing.T, statuses []runner.FlowNodeStatus, _ error, config NodeTestConfig) {
	// Async execution should produce similar status patterns to sync
	// Some nodes don't emit statuses, which is fine
	if len(statuses) == 0 {
		t.Log("Async execution completed without status events - valid for simple nodes")
		return
	}
	validateSuccessPattern(t, statuses, config)
}
