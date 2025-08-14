package nrequest

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/node/nfor"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mbodyform"
	"the-dev-tools/server/pkg/model/mbodyurl"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mexamplequery"
	"the-dev-tools/server/pkg/model/mexampleresp"
	"the-dev-tools/server/pkg/model/mexamplerespheader"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	httpclient "the-dev-tools/server/pkg/httpclient"
)

// MockHTTPClient simulates HTTP requests for testing
type MockHTTPClient struct {
	mu             sync.Mutex
	executionCount map[string]int // Track executions per ExecutionID
}

func NewMockHTTPClient() *MockHTTPClient {
	return &MockHTTPClient{
		executionCount: make(map[string]int),
	}
}

func (m *MockHTTPClient) Do(req any) (any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Simulate a successful response
	return map[string]interface{}{
		"status": 200,
		"body":   fmt.Sprintf("Response for execution"),
	}, nil
}

func (m *MockHTTPClient) GetExecutionCount() map[string]int {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	result := make(map[string]int)
	for k, v := range m.executionCount {
		result[k] = v
	}
	return result
}

func TestRequestNodeLoopExecution(t *testing.T) {
	// Create FOR node with 10 iterations
	forNodeID := idwrap.NewNow()
	forNode := nfor.New(forNodeID, "TestLoop", 10, 5*time.Second, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)
	
	// Create REQUEST node
	requestNodeID := idwrap.NewNow()
	
	// Create mock endpoint and example
	endpoint := mitemapi.ItemApi{
		ID:   idwrap.NewNow(),
		Name: "TestAPI",
		Url:  "http://example.com/api",
	}
	
	example := mitemapiexample.ItemApiExample{
		ID:        idwrap.NewNow(),
		ItemApiID: endpoint.ID,
		Name:      "TestExample",
	}
	
	// Create a channel to collect responses
	respChan := make(chan NodeRequestSideResp, 100)
	
	// Track unique ExecutionIDs
	var executionIDs []idwrap.IDWrap
	var executionMutex sync.Mutex
	
	// Create REQUEST node
	requestNode := New(
		requestNodeID,
		"TestRequest",
		endpoint,
		example,
		[]mexamplequery.Query{},
		[]mexampleheader.Header{},
		mbodyraw.ExampleBodyRaw{},
		[]mbodyform.BodyForm{},
		[]mbodyurl.BodyURLEncoded{},
		mexampleresp.ExampleResp{ID: idwrap.NewNow()},
		[]mexamplerespheader.ExampleRespHeader{},
		[]massert.Assert{},
		httpclient.New(),
		respChan,
	)
	
	// Set up node map
	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		forNodeID:     forNode,
		requestNodeID: requestNode,
	}
	
	// Connect FOR node to REQUEST node
	edges := []edge.Edge{
		edge.NewEdge(idwrap.NewNow(), forNodeID, requestNodeID, edge.HandleLoop, edge.EdgeKindNoOp),
	}
	edgeSourceMap := edge.NewEdgesMap(edges)
	
	// Track execution statuses
	var executionStatuses []runner.FlowNodeStatus
	
	// Create request with status logging
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]interface{}),
		NodeMap:       nodeMap,
		EdgeSourceMap: edgeSourceMap,
		LogPushFunc: func(status runner.FlowNodeStatus) {
			if status.NodeID == requestNodeID && status.State == mnnode.NODE_STATE_RUNNING {
				executionMutex.Lock()
				defer executionMutex.Unlock()
				executionStatuses = append(executionStatuses, status)
				
				// Each execution should have a unique ExecutionID
				if status.ExecutionID != (idwrap.IDWrap{}) {
					executionIDs = append(executionIDs, status.ExecutionID)
				}
			}
		},
		ReadWriteLock: &sync.RWMutex{},
	}
	
	// Start a goroutine to collect responses
	go func() {
		for resp := range respChan {
			// Each response should have an ExecutionID
			assert.NotEqual(t, idwrap.IDWrap{}, resp.ExecutionID, "Response should have ExecutionID")
		}
	}()
	
	// Execute FOR node
	result := forNode.RunSync(context.Background(), req)
	require.NoError(t, result.Err)
	
	// Wait a bit for async operations to complete
	time.Sleep(100 * time.Millisecond)
	
	// Verify REQUEST node was executed 10 times
	assert.Len(t, executionStatuses, 10, "REQUEST node should have 10 execution statuses")
	assert.Len(t, executionIDs, 10, "Should have 10 unique ExecutionIDs")
	
	// Verify all ExecutionIDs are unique
	uniqueIDs := make(map[idwrap.IDWrap]bool)
	for _, id := range executionIDs {
		assert.False(t, uniqueIDs[id], "ExecutionID %s should be unique", id.String())
		uniqueIDs[id] = true
	}
	
	// Verify each execution has correct iteration context
	for i, status := range executionStatuses {
		require.NotNil(t, status.IterationContext, "Execution %d should have iteration context", i)
		assert.Equal(t, []int{i}, status.IterationContext.IterationPath, 
			"Execution %d should have correct iteration path", i)
		assert.Equal(t, i, status.IterationContext.ExecutionIndex,
			"Execution %d should have correct ExecutionIndex", i)
	}
	
	close(respChan)
}

func TestRequestNodeNestedLoopExecution(t *testing.T) {
	// Create outer FOR node with 3 iterations
	outerForID := idwrap.NewNow()
	outerFor := nfor.New(outerForID, "OuterLoop", 3, 5*time.Second, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)
	
	// Create inner FOR node with 3 iterations
	innerForID := idwrap.NewNow()
	innerFor := nfor.New(innerForID, "InnerLoop", 3, 5*time.Second, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)
	
	// Create REQUEST node
	requestNodeID := idwrap.NewNow()
	
	// Create mock endpoint and example
	endpoint := mitemapi.ItemApi{
		ID:   idwrap.NewNow(),
		Name: "TestAPI",
		Url:  "http://example.com/api",
	}
	
	example := mitemapiexample.ItemApiExample{
		ID:        idwrap.NewNow(),
		ItemApiID: endpoint.ID,
		Name:      "TestExample",
	}
	
	// Create a channel to collect responses
	respChan := make(chan NodeRequestSideResp, 100)
	
	// Track unique ExecutionIDs
	var executionIDs []idwrap.IDWrap
	var executionMutex sync.Mutex
	
	// Create REQUEST node
	requestNode := New(
		requestNodeID,
		"NestedRequest",
		endpoint,
		example,
		[]mexamplequery.Query{},
		[]mexampleheader.Header{},
		mbodyraw.ExampleBodyRaw{},
		[]mbodyform.BodyForm{},
		[]mbodyurl.BodyURLEncoded{},
		mexampleresp.ExampleResp{ID: idwrap.NewNow()},
		[]mexamplerespheader.ExampleRespHeader{},
		[]massert.Assert{},
		httpclient.New(),
		respChan,
	)
	
	// Set up node map
	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		outerForID:    outerFor,
		innerForID:    innerFor,
		requestNodeID: requestNode,
	}
	
	// Connect: OuterFor -> InnerFor -> Request
	edges := []edge.Edge{
		edge.NewEdge(idwrap.NewNow(), outerForID, innerForID, edge.HandleLoop, edge.EdgeKindNoOp),
		edge.NewEdge(idwrap.NewNow(), innerForID, requestNodeID, edge.HandleLoop, edge.EdgeKindNoOp),
	}
	edgeSourceMap := edge.NewEdgesMap(edges)
	
	// Track execution statuses
	var executionStatuses []runner.FlowNodeStatus
	
	// Create request with status logging
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]interface{}),
		NodeMap:       nodeMap,
		EdgeSourceMap: edgeSourceMap,
		LogPushFunc: func(status runner.FlowNodeStatus) {
			if status.NodeID == requestNodeID && status.State == mnnode.NODE_STATE_RUNNING {
				executionMutex.Lock()
				defer executionMutex.Unlock()
				executionStatuses = append(executionStatuses, status)
				
				// Each execution should have a unique ExecutionID
				if status.ExecutionID != (idwrap.IDWrap{}) {
					executionIDs = append(executionIDs, status.ExecutionID)
				}
			}
		},
		ReadWriteLock: &sync.RWMutex{},
	}
	
	// Start a goroutine to collect responses
	responsesReceived := 0
	go func() {
		for resp := range respChan {
			// Each response should have an ExecutionID
			assert.NotEqual(t, idwrap.IDWrap{}, resp.ExecutionID, "Response should have ExecutionID")
			responsesReceived++
		}
	}()
	
	// Execute outer FOR node
	result := outerFor.RunSync(context.Background(), req)
	require.NoError(t, result.Err)
	
	// Wait a bit for async operations to complete
	time.Sleep(100 * time.Millisecond)
	
	// Should have 3 * 3 = 9 executions
	assert.Len(t, executionStatuses, 9, "REQUEST node should have 9 execution statuses")
	assert.Len(t, executionIDs, 9, "Should have 9 unique ExecutionIDs")
	
	// Verify all ExecutionIDs are unique
	uniqueIDs := make(map[idwrap.IDWrap]bool)
	for _, id := range executionIDs {
		assert.False(t, uniqueIDs[id], "ExecutionID %s should be unique", id.String())
		uniqueIDs[id] = true
	}
	
	// Verify iteration paths are correct for nested loops
	expectedPaths := [][]int{
		{0, 0}, {0, 1}, {0, 2}, // Outer 0, Inner 0-2
		{1, 0}, {1, 1}, {1, 2}, // Outer 1, Inner 0-2
		{2, 0}, {2, 1}, {2, 2}, // Outer 2, Inner 0-2
	}
	
	for i, status := range executionStatuses {
		require.NotNil(t, status.IterationContext, "Execution %d should have iteration context", i)
		assert.Equal(t, expectedPaths[i], status.IterationContext.IterationPath,
			"Execution %d should have correct iteration path", i)
	}
	
	close(respChan)
}