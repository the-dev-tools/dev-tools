package rflow

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/node/nfor"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/flow/runner/flowlocalrunner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockRequestNode simulates a REQUEST node that tracks its executions
type MockRequestNode struct {
	id           idwrap.IDWrap
	name         string
	executionLog []string
	mu           sync.Mutex
}

func NewMockRequestNode(id idwrap.IDWrap, name string) *MockRequestNode {
	return &MockRequestNode{
		id:           id,
		name:         name,
		executionLog: []string{},
	}
}

func (m *MockRequestNode) GetID() idwrap.IDWrap {
	return m.id
}

func (m *MockRequestNode) GetName() string {
	return m.name
}

func (m *MockRequestNode) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Log the execution with iteration context (check for nil)
	if req.IterationContext != nil {
		execLog := fmt.Sprintf("Execution at iteration path %v, ExecutionIndex=%d", 
			req.IterationContext.IterationPath, 
			req.IterationContext.ExecutionIndex)
		m.executionLog = append(m.executionLog, execLog)
	} else {
		m.executionLog = append(m.executionLog, "Execution without iteration context")
	}
	
	return node.FlowNodeResult{}
}

func (m *MockRequestNode) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	result := m.RunSync(ctx, req)
	resultChan <- result
}

func (m *MockRequestNode) GetExecutionLog() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	log := make([]string, len(m.executionLog))
	copy(log, m.executionLog)
	return log
}

func TestLoopExecutionNaming_E2E(t *testing.T) {
	// Create FOR node with 10 iterations
	forNodeID := idwrap.NewNow()
	forNode := nfor.New(forNodeID, "TestLoop", 10, 5*time.Second, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)
	
	// Create mock REQUEST node
	requestNodeID := idwrap.NewNow()
	requestNode := NewMockRequestNode(requestNodeID, "TestRequest")
	
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
	
	// Track execution names
	var executionNames []string
	var executionMutex sync.Mutex
	
	// Create request with status logging
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]interface{}),
		NodeMap:       nodeMap,
		EdgeSourceMap: edgeSourceMap,
		LogPushFunc: func(status runner.FlowNodeStatus) {
			// Capture execution names for REQUEST node (only RUNNING state)
			if status.NodeID == requestNodeID && status.State == mnnode.NODE_STATE_RUNNING {
				executionMutex.Lock()
				defer executionMutex.Unlock()
				
				// Build execution name similar to formatIterationContext
				if status.IterationContext != nil && len(status.IterationContext.IterationPath) > 0 {
					// This should produce names like: "TestLoop iteration 1 | TestRequest - Execution 1"
					// where the execution number comes from ExecutionIndex
					execNum := status.IterationContext.ExecutionIndex + 1
					name := fmt.Sprintf("TestLoop iteration %d | TestRequest - Execution %d",
						status.IterationContext.IterationPath[0]+1,
						execNum)
					executionNames = append(executionNames, name)
				}
			}
		},
		ReadWriteLock: &sync.RWMutex{},
	}
	
	// Execute FOR node
	result := forNode.RunSync(context.Background(), req)
	require.NoError(t, result.Err)
	
	// Verify REQUEST node was executed 10 times
	execLog := requestNode.GetExecutionLog()
	assert.Len(t, execLog, 10, "REQUEST node should be executed 10 times")
	
	// Verify each execution has unique ExecutionIndex
	for i, log := range execLog {
		expectedLog := fmt.Sprintf("Execution at iteration path [%d], ExecutionIndex=%d", i, i)
		assert.Equal(t, expectedLog, log, "Execution %d should have correct context", i)
	}
	
	// Verify execution names show correct execution numbers
	assert.Len(t, executionNames, 10, "Should have 10 execution names")
	
	// Each execution should have unique execution number matching iteration
	for i, name := range executionNames {
		expectedName := fmt.Sprintf("TestLoop iteration %d | TestRequest - Execution %d", i+1, i+1)
		assert.Equal(t, expectedName, name, "Execution name %d should be correct", i)
	}
}

func TestLoopExecutionTracking_FlowRunner(t *testing.T) {
	// Test with the actual flow runner to ensure execution tracking works end-to-end
	
	// Create FOR node with 5 iterations (start with FOR node directly)
	forNodeID := idwrap.NewNow()
	forNode := nfor.New(forNodeID, "MainLoop", 5, 5*time.Second, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)
	
	// Create mock REQUEST node
	requestNodeID := idwrap.NewNow()
	requestNode := NewMockRequestNode(requestNodeID, "APICall")
	
	// Set up node map
	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		forNodeID:     forNode,
		requestNodeID: requestNode,
	}
	
	// Connect: FOR -> REQUEST
	edges := []edge.Edge{
		edge.NewEdge(idwrap.NewNow(), forNodeID, requestNodeID, edge.HandleLoop, edge.EdgeKindNoOp),
	}
	edgeMap := edge.NewEdgesMap(edges)
	
	// Create flow runner starting directly with FOR node
	runnerID := idwrap.NewNow()
	flowID := idwrap.NewNow()
	flowRunner := flowlocalrunner.CreateFlowRunner(runnerID, flowID, forNodeID, nodeMap, edgeMap, 0)
	
	// Channels for flow execution
	flowNodeStatusChan := make(chan runner.FlowNodeStatus, 1000)
	flowStatusChan := make(chan runner.FlowStatus, 10)
	
	// Track REQUEST node executions
	var requestExecutions []runner.FlowNodeStatus
	var allStatuses []runner.FlowNodeStatus
	statusDone := make(chan struct{})
	go func() {
		defer close(statusDone)
		for status := range flowNodeStatusChan {
			allStatuses = append(allStatuses, status)
			if status.NodeID == requestNodeID && status.State == mnnode.NODE_STATE_RUNNING {
				requestExecutions = append(requestExecutions, status)
			}
		}
	}()
	
	// Run the flow
	ctx := context.Background()
	err := flowRunner.Run(ctx, flowNodeStatusChan, flowStatusChan, nil)
	require.NoError(t, err)
	
	// Wait for channels to close
	<-statusDone
	
	// Debug: Print all statuses if test fails
	if len(requestExecutions) != 5 {
		t.Logf("All statuses received (%d total):", len(allStatuses))
		for i, status := range allStatuses {
			t.Logf("  %d: NodeID=%v, State=%v, Name=%s, IterContext=%v", 
				i, status.NodeID, status.State, status.Name, status.IterationContext)
		}
	}
	
	// Verify REQUEST node was executed 5 times
	assert.Len(t, requestExecutions, 5, "REQUEST node should have 5 RUNNING status records")
	
	// Verify each execution has unique ExecutionIndex in the context
	seenIndexes := make(map[int]bool)
	for i, status := range requestExecutions {
		require.NotNil(t, status.IterationContext, "Execution %d should have iteration context", i)
		
		execIndex := status.IterationContext.ExecutionIndex
		assert.False(t, seenIndexes[execIndex], 
			"ExecutionIndex %d should be unique but was already seen", execIndex)
		seenIndexes[execIndex] = true
		
		// ExecutionIndex should match the iteration number
		assert.Equal(t, i, execIndex, 
			"ExecutionIndex should match iteration number")
	}
}

func TestNestedLoopExecutionTracking(t *testing.T) {
	// Test nested loops to ensure ExecutionIndex is properly propagated
	
	// Create outer FOR node with 3 iterations
	outerForID := idwrap.NewNow()
	outerFor := nfor.New(outerForID, "OuterLoop", 3, 5*time.Second, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)
	
	// Create inner FOR node with 2 iterations
	innerForID := idwrap.NewNow()
	innerFor := nfor.New(innerForID, "InnerLoop", 2, 5*time.Second, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)
	
	// Create mock REQUEST node
	requestNodeID := idwrap.NewNow()
	requestNode := NewMockRequestNode(requestNodeID, "NestedRequest")
	
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
	
	// Track execution contexts - only capture REQUEST node RUNNING statuses
	var executionContexts []runner.IterationContext
	var allStatuses []runner.FlowNodeStatus
	var execMutex sync.Mutex
	
	// Create request with status logging
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]interface{}),
		NodeMap:       nodeMap,
		EdgeSourceMap: edgeSourceMap,
		LogPushFunc: func(status runner.FlowNodeStatus) {
			execMutex.Lock()
			defer execMutex.Unlock()
			allStatuses = append(allStatuses, status)
			// Only capture REQUEST node RUNNING statuses to avoid duplication
			if status.NodeID == requestNodeID && status.State == mnnode.NODE_STATE_RUNNING && status.IterationContext != nil {
				executionContexts = append(executionContexts, *status.IterationContext)
			}
		},
		ReadWriteLock: &sync.RWMutex{},
	}
	
	// Execute outer FOR node
	result := outerFor.RunSync(context.Background(), req)
	require.NoError(t, result.Err)
	
	// Debug: Print all statuses if test fails
	if len(executionContexts) != 6 {
		t.Logf("All statuses received (%d total):", len(allStatuses))
		for i, status := range allStatuses {
			t.Logf("  %d: NodeID=%v, State=%v, Name=%s, IterContext=%v", 
				i, status.NodeID, status.State, status.Name, status.IterationContext)
		}
		t.Logf("REQUEST execution contexts (%d):", len(executionContexts))
		for i, ctx := range executionContexts {
			t.Logf("  %d: Path=%v, ExecutionIndex=%d", i, ctx.IterationPath, ctx.ExecutionIndex)
		}
	}
	
	// Should have 3 * 2 = 6 executions
	assert.Len(t, executionContexts, 6, "Should have 6 execution contexts")
	
	// Verify ExecutionIndex values - inner loop ExecutionIndex resets for each outer iteration
	expectedContexts := []struct {
		outerIteration int
		innerIteration int
		executionIndex int
	}{
		{0, 0, 0}, // Outer 0, Inner 0
		{0, 1, 1}, // Outer 0, Inner 1
		{1, 0, 0}, // Outer 1, Inner 0 - ExecutionIndex resets
		{1, 1, 1}, // Outer 1, Inner 1
		{2, 0, 0}, // Outer 2, Inner 0 - ExecutionIndex resets
		{2, 1, 1}, // Outer 2, Inner 1
	}
	
	// Safely check only the number of contexts we actually have
	maxIndex := len(executionContexts)
	if maxIndex > len(expectedContexts) {
		maxIndex = len(expectedContexts)
	}
	
	for i := 0; i < maxIndex; i++ {
		ctx := executionContexts[i]
		expected := expectedContexts[i]
		assert.Equal(t, []int{expected.outerIteration, expected.innerIteration}, ctx.IterationPath,
			"Context %d should have correct iteration path", i)
		assert.Equal(t, expected.executionIndex, ctx.ExecutionIndex,
			"Context %d should have correct ExecutionIndex", i)
	}
}