package node_test

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/idwrap"
	"time"
)

// MockNode represents a simple node implementation for testing
type MockNode struct {
	ID       string
	NodeType string
	RunFunc  func(req *node.FlowNodeRequest) (interface{}, error)
}

func (m *MockNode) Run(req *node.FlowNodeRequest) (interface{}, error) {
	// Set current node ID for tracking
	nodeID, err := idwrap.NewText(m.ID)
	if err != nil {
		// Use a new ID if parsing fails
		nodeID = idwrap.NewNow()
	}
	req.CurrentNodeID = nodeID

	// Initialize tracking if needed
	if req.ReadTracker == nil {
		req.ReadTracker = make(map[string]interface{})
	}
	if req.ReadTrackerMutex == nil {
		req.ReadTrackerMutex = &sync.Mutex{}
	}

	// Run the node's logic
	result, err := m.RunFunc(req)
	if err != nil {
		return nil, err
	}

	// Store the result for other nodes to read
	if err := node.WriteNodeVar(req, m.ID, "output", result); err != nil {
		return nil, fmt.Errorf("failed to write node var: %w", err)
	}

	return result, nil
}

// TestRealFlowIntegration simulates a realistic flow with multiple nodes
func TestRealFlowIntegration(t *testing.T) {
	// Create a shared flow request
	flowReq := &node.FlowNodeRequest{
		VarMap:           make(map[string]interface{}),
		ReadWriteLock:    &sync.RWMutex{},
		ReadTracker:      make(map[string]interface{}),
		ReadTrackerMutex: &sync.Mutex{},
	}

	// Node execution tracking
	nodeExecutions := make(map[string]map[string]interface{})
	executionMutex := &sync.Mutex{}

	// Helper to capture node execution data
	captureExecution := func(nodeID string) map[string]interface{} {
		// Capture current tracking state for this node
		flowReq.ReadTrackerMutex.Lock()
		trackedData := make(map[string]interface{})
		for k, v := range flowReq.ReadTracker {
			trackedData[k] = v
		}
		flowReq.ReadTrackerMutex.Unlock()

		// Clear tracker for next node
		flowReq.ReadTrackerMutex.Lock()
		flowReq.ReadTracker = make(map[string]interface{})
		flowReq.ReadTrackerMutex.Unlock()

		return trackedData
	}

	// Node A: Makes an HTTP request (simulated)
	nodeA := &MockNode{
		ID:       "nodeA",
		NodeType: "REQUEST",
		RunFunc: func(req *node.FlowNodeRequest) (interface{}, error) {
			t.Log("Node A: Making HTTP request")

			// Simulate reading config
			_, _ = node.ReadVarRaw(req, "baseURL")

			// Simulate HTTP response
			response := map[string]interface{}{
				"status": 200,
				"body": map[string]interface{}{
					"users": []interface{}{
						map[string]interface{}{"id": 1, "name": "Alice", "role": "admin"},
						map[string]interface{}{"id": 2, "name": "Bob", "role": "user"},
						map[string]interface{}{"id": 3, "name": "Charlie", "role": "user"},
					},
					"total": 3,
				},
				"headers": map[string]string{
					"Content-Type": "application/json",
				},
			}

			return response, nil
		},
	}

	// Node B: Transforms the response from Node A
	nodeB := &MockNode{
		ID:       "nodeB",
		NodeType: "JAVASCRIPT",
		RunFunc: func(req *node.FlowNodeRequest) (interface{}, error) {
			t.Log("Node B: Transforming data from Node A")

			// Read Node A's output
			nodeAOutput, err := node.ReadNodeVar(req, "nodeA", "output")
			if err != nil {
				return nil, fmt.Errorf("failed to read nodeA output: %w", err)
			}

			// Extract users from response
			response := nodeAOutput.(map[string]interface{})
			body := response["body"].(map[string]interface{})
			users := body["users"].([]interface{})

			// Transform: filter admin users and add timestamp
			adminUsers := []interface{}{}
			for _, user := range users {
				userMap := user.(map[string]interface{})
				if userMap["role"] == "admin" {
					userMap["processedAt"] = time.Now().Format(time.RFC3339)
					adminUsers = append(adminUsers, userMap)
				}
			}

			transformed := map[string]interface{}{
				"adminUsers": adminUsers,
				"count":      len(adminUsers),
				"source":     "nodeA",
			}

			return transformed, nil
		},
	}

	// Node C: Reads from both Node A and Node B
	nodeC := &MockNode{
		ID:       "nodeC",
		NodeType: "JAVASCRIPT",
		RunFunc: func(req *node.FlowNodeRequest) (interface{}, error) {
			t.Log("Node C: Combining data from Node A and Node B")

			// Read from both previous nodes
			nodeAOutput, err := node.ReadNodeVar(req, "nodeA", "output")
			if err != nil {
				return nil, fmt.Errorf("failed to read nodeA: %w", err)
			}

			nodeBOutput, err := node.ReadNodeVar(req, "nodeB", "output")
			if err != nil {
				return nil, fmt.Errorf("failed to read nodeB: %w", err)
			}

			// Read a flow variable
			reportTitle, _ := node.ReadVarRaw(req, "reportTitle")

			// Create combined report
			responseA := nodeAOutput.(map[string]interface{})
			transformedB := nodeBOutput.(map[string]interface{})

			report := map[string]interface{}{
				"title":       reportTitle,
				"totalUsers":  responseA["body"].(map[string]interface{})["total"],
				"adminUsers":  transformedB["adminUsers"],
				"adminCount":  transformedB["count"],
				"httpStatus":  responseA["status"],
				"generatedAt": time.Now().Format(time.RFC3339),
			}

			return report, nil
		},
	}

	// Set up initial flow variables
	flowReq.VarMap["baseURL"] = "https://api.example.com"
	flowReq.VarMap["reportTitle"] = "User Analysis Report"

	// Execute the flow
	t.Log("=== Starting Flow Execution ===")

	// Execute Node A
	resultA, err := nodeA.Run(flowReq)
	if err != nil {
		t.Fatalf("Node A failed: %v", err)
	}
	nodeAInputs := captureExecution("nodeA")
	executionMutex.Lock()
	nodeExecutions["nodeA"] = nodeAInputs
	executionMutex.Unlock()

	t.Logf("Node A completed. Result type: %T", resultA)
	t.Logf("Node A input data: %v", formatJSON(nodeAInputs))

	// Execute Node B
	resultB, err := nodeB.Run(flowReq)
	if err != nil {
		t.Fatalf("Node B failed: %v", err)
	}
	nodeBInputs := captureExecution("nodeB")
	executionMutex.Lock()
	nodeExecutions["nodeB"] = nodeBInputs
	executionMutex.Unlock()

	t.Logf("Node B completed. Result type: %T", resultB)
	t.Logf("Node B input data keys: %v", getKeys(nodeBInputs))

	// Execute Node C
	resultC, err := nodeC.Run(flowReq)
	if err != nil {
		t.Fatalf("Node C failed: %v", err)
	}
	nodeCInputs := captureExecution("nodeC")
	executionMutex.Lock()
	nodeExecutions["nodeC"] = nodeCInputs
	executionMutex.Unlock()

	t.Logf("Node C completed. Result type: %T", resultC)
	t.Logf("Node C input data keys: %v", getKeys(nodeCInputs))

	// Verify the execution tracking
	t.Log("\n=== Verification ===")

	// Node A should have read baseURL
	if nodeAInputs["baseURL"] != "https://api.example.com" {
		t.Error("Node A didn't track baseURL correctly")
	}
	if len(nodeAInputs) != 1 {
		t.Errorf("Node A should have tracked 1 input, got %d", len(nodeAInputs))
	}

	// Node B should have read Node A's output
	if _, exists := nodeBInputs["nodeA"]; !exists {
		t.Error("Node B didn't track reading from Node A")
	}
	if len(nodeBInputs) != 1 {
		t.Errorf("Node B should have tracked 1 input, got %d", len(nodeBInputs))
	}

	// Node C should have read from both A and B, plus reportTitle
	if _, exists := nodeCInputs["nodeA"]; !exists {
		t.Error("Node C didn't track reading from Node A")
	}
	if _, exists := nodeCInputs["nodeB"]; !exists {
		t.Error("Node C didn't track reading from Node B")
	}
	if nodeCInputs["reportTitle"] != "User Analysis Report" {
		t.Error("Node C didn't track reportTitle correctly")
	}
	if len(nodeCInputs) != 3 {
		t.Errorf("Node C should have tracked 3 inputs, got %d", len(nodeCInputs))
	}

	// Verify final result structure
	finalReport := resultC.(map[string]interface{})
	if finalReport["title"] != "User Analysis Report" {
		t.Error("Final report title incorrect")
	}
	// totalUsers is coming from nested structure, need to check type
	totalUsers := finalReport["totalUsers"]
	if intVal, ok := totalUsers.(int); ok && intVal != 3 {
		t.Errorf("Final report total users incorrect: expected 3, got %d", intVal)
	} else if floatVal, ok := totalUsers.(float64); ok && floatVal != 3 {
		t.Errorf("Final report total users incorrect: expected 3, got %f", floatVal)
	}
	if finalReport["adminCount"] != 1 {
		t.Error("Final report admin count incorrect")
	}

	t.Log("\n=== Flow Execution Summary ===")
	t.Logf("Total nodes executed: 3")
	t.Logf("Total unique data sources tracked: %d", countUniqueKeys(nodeExecutions))

	// Print full execution trace
	t.Log("\nDetailed Execution Trace:")
	for nodeID, inputs := range nodeExecutions {
		t.Logf("\n%s inputs:", nodeID)
		for key, value := range inputs {
			t.Logf("  - %s: %s", key, summarizeValue(value))
		}
	}
}

// Helper function to create a test tracking helper
func TestTrackingHelper(t *testing.T) {
	// This demonstrates a helper function that could be used in tests
	// to easily verify tracked data

	helper := &TrackingTestHelper{
		t: t,
	}

	// Setup test flow
	req := &node.FlowNodeRequest{
		VarMap:           make(map[string]interface{}),
		ReadWriteLock:    &sync.RWMutex{},
		ReadTracker:      make(map[string]interface{}),
		ReadTrackerMutex: &sync.Mutex{},
		CurrentNodeID:    idwrap.NewNow(),
	}

	// Add test data
	req.VarMap["testVar"] = "testValue"
	req.VarMap["node1"] = map[string]interface{}{"data": "node1Data"}

	// Perform reads
	_, _ = node.ReadVarRaw(req, "testVar")
	_, _ = node.ReadNodeVar(req, "node1", "data")

	// Use helper to verify
	helper.AssertTracked(req, "testVar", "testValue")
	helper.AssertTrackedNode(req, "node1")
	helper.AssertTrackedCount(req, 2)
	helper.AssertNotTracked(req, "notRead")
}

// TrackingTestHelper provides convenient assertions for tracking tests
type TrackingTestHelper struct {
	t *testing.T
}

func (h *TrackingTestHelper) AssertTracked(req *node.FlowNodeRequest, key string, expectedValue interface{}) {
	req.ReadTrackerMutex.Lock()
	defer req.ReadTrackerMutex.Unlock()

	value, exists := req.ReadTracker[key]
	if !exists {
		h.t.Errorf("Expected key '%s' to be tracked, but it wasn't", key)
		return
	}

	if expectedValue != nil && value != expectedValue {
		h.t.Errorf("Tracked value for '%s' mismatch: expected %v, got %v", key, expectedValue, value)
	}
}

func (h *TrackingTestHelper) AssertTrackedNode(req *node.FlowNodeRequest, nodeID string) {
	req.ReadTrackerMutex.Lock()
	defer req.ReadTrackerMutex.Unlock()

	_, exists := req.ReadTracker[nodeID]
	if !exists {
		h.t.Errorf("Expected node '%s' to be tracked, but it wasn't", nodeID)
	}
}

func (h *TrackingTestHelper) AssertNotTracked(req *node.FlowNodeRequest, key string) {
	req.ReadTrackerMutex.Lock()
	defer req.ReadTrackerMutex.Unlock()

	if _, exists := req.ReadTracker[key]; exists {
		h.t.Errorf("Expected key '%s' NOT to be tracked, but it was", key)
	}
}

func (h *TrackingTestHelper) AssertTrackedCount(req *node.FlowNodeRequest, expected int) {
	req.ReadTrackerMutex.Lock()
	defer req.ReadTrackerMutex.Unlock()

	actual := len(req.ReadTracker)
	if actual != expected {
		h.t.Errorf("Expected %d tracked items, got %d", expected, actual)
	}
}

func (h *TrackingTestHelper) GetTrackedKeys(req *node.FlowNodeRequest) []string {
	req.ReadTrackerMutex.Lock()
	defer req.ReadTrackerMutex.Unlock()

	keys := make([]string, 0, len(req.ReadTracker))
	for k := range req.ReadTracker {
		keys = append(keys, k)
	}
	return keys
}

// Helper functions
func formatJSON(v interface{}) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func countUniqueKeys(executions map[string]map[string]interface{}) int {
	unique := make(map[string]bool)
	for _, inputs := range executions {
		for k := range inputs {
			unique[k] = true
		}
	}
	return len(unique)
}

func summarizeValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		if len(val) > 50 {
			return fmt.Sprintf("string(%d chars)", len(val))
		}
		return fmt.Sprintf("'%s'", val)
	case map[string]interface{}:
		return fmt.Sprintf("map[%d keys]", len(val))
	case []interface{}:
		return fmt.Sprintf("array[%d items]", len(val))
	default:
		return fmt.Sprintf("%T(%v)", v, v)
	}
}
