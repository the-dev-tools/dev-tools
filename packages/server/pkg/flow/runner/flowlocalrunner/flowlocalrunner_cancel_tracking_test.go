package flowlocalrunner

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
	"time"
)

// CancelableTestNode is a test node that supports controlled cancellation scenarios
type CancelableTestNode struct {
	ID               idwrap.IDWrap
	Name             string
	NextIDs          []idwrap.IDWrap
	DelayBeforeRead  time.Duration
	DelayAfterRead   time.Duration
	DelayBeforeWrite time.Duration
	DelayAfterWrite  time.Duration
	ReadKeys         []string
	WriteData        map[string]interface{}
	ShouldFail       bool
}

func NewCancelableTestNode(id idwrap.IDWrap, name string, nextIDs []idwrap.IDWrap) *CancelableTestNode {
	return &CancelableTestNode{
		ID:        id,
		Name:      name,
		NextIDs:   nextIDs,
		WriteData: make(map[string]interface{}),
	}
}

func (cn *CancelableTestNode) WithDelays(beforeRead, afterRead, beforeWrite, afterWrite time.Duration) *CancelableTestNode {
	cn.DelayBeforeRead = beforeRead
	cn.DelayAfterRead = afterRead
	cn.DelayBeforeWrite = beforeWrite
	cn.DelayAfterWrite = afterWrite
	return cn
}

func (cn *CancelableTestNode) WithReads(keys ...string) *CancelableTestNode {
	cn.ReadKeys = keys
	return cn
}

func (cn *CancelableTestNode) WithWrites(data map[string]interface{}) *CancelableTestNode {
	cn.WriteData = data
	return cn
}

func (cn *CancelableTestNode) WithFailure() *CancelableTestNode {
	cn.ShouldFail = true
	return cn
}

func (cn *CancelableTestNode) GetID() idwrap.IDWrap {
	return cn.ID
}

func (cn *CancelableTestNode) GetName() string {
	return cn.Name
}

func (cn *CancelableTestNode) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	// Check for cancellation before starting
	select {
	case <-ctx.Done():
		return node.FlowNodeResult{
			NextNodeID: nil,
			Err:        ctx.Err(),
		}
	default:
	}

	// Delay before reading
	if cn.DelayBeforeRead > 0 {
		select {
		case <-time.After(cn.DelayBeforeRead):
		case <-ctx.Done():
			return node.FlowNodeResult{
				NextNodeID: nil,
				Err:        ctx.Err(),
			}
		}
	}

	// Perform reads with tracking
	for _, key := range cn.ReadKeys {
		if req.VariableTracker != nil {
			_, err := node.ReadVarRawWithTracking(req, key, req.VariableTracker)
			if err != nil && err != node.ErrVarKeyNotFound {
				return node.FlowNodeResult{NextNodeID: nil, Err: err}
			}
		}
		// Check cancellation after each read
		select {
		case <-ctx.Done():
			return node.FlowNodeResult{
				NextNodeID: nil,
				Err:        ctx.Err(),
			}
		default:
		}
	}

	// Delay after reading
	if cn.DelayAfterRead > 0 {
		select {
		case <-time.After(cn.DelayAfterRead):
		case <-ctx.Done():
			return node.FlowNodeResult{
				NextNodeID: nil,
				Err:        ctx.Err(),
			}
		}
	}

	// Delay before writing
	if cn.DelayBeforeWrite > 0 {
		select {
		case <-time.After(cn.DelayBeforeWrite):
		case <-ctx.Done():
			return node.FlowNodeResult{
				NextNodeID: nil,
				Err:        ctx.Err(),
			}
		}
	}

	// Perform writes with tracking
	for key, value := range cn.WriteData {
		if req.VariableTracker != nil {
			err := node.WriteNodeVarWithTracking(req, cn.Name, key, value, req.VariableTracker)
			if err != nil {
				return node.FlowNodeResult{NextNodeID: nil, Err: err}
			}
		} else {
			err := node.WriteNodeVar(req, cn.Name, key, value)
			if err != nil {
				return node.FlowNodeResult{NextNodeID: nil, Err: err}
			}
		}
		// Check cancellation after each write
		select {
		case <-ctx.Done():
			return node.FlowNodeResult{
				NextNodeID: nil,
				Err:        ctx.Err(),
			}
		default:
		}
	}

	// Delay after writing
	if cn.DelayAfterWrite > 0 {
		select {
		case <-time.After(cn.DelayAfterWrite):
		case <-ctx.Done():
			return node.FlowNodeResult{
				NextNodeID: nil,
				Err:        ctx.Err(),
			}
		}
	}

	if cn.ShouldFail {
		return node.FlowNodeResult{
			NextNodeID: nil,
			Err:        fmt.Errorf("intentional test failure"),
		}
	}

	return node.FlowNodeResult{
		NextNodeID: cn.NextIDs,
		Err:        nil,
	}
}

func (cn *CancelableTestNode) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	result := cn.RunSync(ctx, req)
	resultChan <- result
}

func TestCanceledNode_NoDataReadOrWritten(t *testing.T) {
	// Test case: Node canceled before any reads/writes → should show empty tracking
	nodeID := idwrap.NewNow()

	// Create a node that delays before doing anything
	testNode := NewCancelableTestNode(nodeID, "testNode", []idwrap.IDWrap{}).
		WithDelays(100*time.Millisecond, 0, 0, 0).
		WithWrites(map[string]interface{}{"result": "success"})

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		nodeID: testNode,
	}

	var statuses []runner.FlowNodeStatus
	statusFunc := func(status runner.FlowNodeStatus) {
		statuses = append(statuses, status)
	}

	req := &node.FlowNodeRequest{
		VarMap:           make(map[string]any),
		ReadWriteLock:    &sync.RWMutex{},
		NodeMap:          nodeMap,
		EdgeSourceMap:    make(edge.EdgesMap),
		Timeout:          5 * time.Second,
		LogPushFunc:      statusFunc,
		PendingAtmoicMap: make(map[idwrap.IDWrap]uint32),
	}

	// Create a context that will be canceled quickly
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := RunNodeSync(ctx, nodeID, req, statusFunc)
	if err == nil {
		t.Error("Expected error due to context cancellation")
	}

	// Find the canceled status
	var canceledStatus *runner.FlowNodeStatus
	for i := range statuses {
		if statuses[i].State == mnnode.NODE_STATE_CANCELED {
			canceledStatus = &statuses[i]
			break
		}
	}

	if canceledStatus == nil {
		t.Fatal("Expected to find a canceled node status")
	}

	// Should have empty input/output data since nothing was read or written
	if canceledStatus.InputData != nil {
		inputMap, ok := canceledStatus.InputData.(map[string]any)
		if ok && len(inputMap) > 0 {
			t.Errorf("Expected empty input data for canceled node, got %v", canceledStatus.InputData)
		}
	}

	if canceledStatus.OutputData != nil {
		outputMap, ok := canceledStatus.OutputData.(map[string]any)
		if ok && len(outputMap) > 0 {
			t.Errorf("Expected empty output data for canceled node, got %v", canceledStatus.OutputData)
		}
	}
}

func TestCanceledNode_PartialReadsCompleted(t *testing.T) {
	// Test case: Node canceled after partial reads → should show only completed reads
	nodeID := idwrap.NewNow()

	// Set up some initial data to read
	initialData := map[string]any{
		"header1": "value1",
		"header2": "value2",
		"body":    "large_body_data",
	}

	// Create a node that reads some data, then delays, then tries to write
	testNode := NewCancelableTestNode(nodeID, "testNode", []idwrap.IDWrap{}).
		WithReads("header1", "header2").
		WithDelays(0, 100*time.Millisecond, 0, 0). // Delay after reads, before writes
		WithWrites(map[string]interface{}{"result": "processed"})

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		nodeID: testNode,
	}

	var statuses []runner.FlowNodeStatus
	statusFunc := func(status runner.FlowNodeStatus) {
		statuses = append(statuses, status)
	}

	req := &node.FlowNodeRequest{
		VarMap:           initialData,
		ReadWriteLock:    &sync.RWMutex{},
		NodeMap:          nodeMap,
		EdgeSourceMap:    make(edge.EdgesMap),
		Timeout:          5 * time.Second,
		LogPushFunc:      statusFunc,
		PendingAtmoicMap: make(map[idwrap.IDWrap]uint32),
	}

	// Cancel context after reads should complete but before writes
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := RunNodeSync(ctx, nodeID, req, statusFunc)
	if err == nil {
		t.Error("Expected error due to context cancellation")
	}

	// Find the canceled status
	var canceledStatus *runner.FlowNodeStatus
	for i := range statuses {
		if statuses[i].State == mnnode.NODE_STATE_CANCELED {
			canceledStatus = &statuses[i]
			break
		}
	}

	if canceledStatus == nil {
		t.Fatal("Expected to find a canceled node status")
	}

	// Should have input data showing the reads that completed
	if canceledStatus.InputData == nil {
		t.Error("Expected input data to be captured for canceled node")
	} else {
		inputMap, ok := canceledStatus.InputData.(map[string]any)
		if !ok {
			t.Errorf("Expected input data to be a map, got %T", canceledStatus.InputData)
		} else {
			// Should contain tracked reads
			if variables, exists := inputMap["variables"]; exists {
				variablesMap, ok := variables.(map[string]any)
				if !ok {
					t.Errorf("Expected variables to be a map, got %T", variables)
				} else {
					// Check that the reads were tracked
					if len(variablesMap) == 0 {
						t.Error("Expected some variables to be tracked in canceled node")
					}
					if variablesMap["header1"] != "value1" {
						t.Errorf("Expected header1='value1', got %v", variablesMap["header1"])
					}
					if variablesMap["header2"] != "value2" {
						t.Errorf("Expected header2='value2', got %v", variablesMap["header2"])
					}
				}
			}
		}
	}

	// Should have no output data since writes were not completed
	if canceledStatus.OutputData != nil {
		outputMap, ok := canceledStatus.OutputData.(map[string]any)
		if ok && len(outputMap) > 0 {
			t.Errorf("Expected no output data for canceled node (writes not completed), got %v", canceledStatus.OutputData)
		}
	}
}

func TestCanceledNode_PartialWritesCompleted(t *testing.T) {
	// Test case: Node canceled after partial writes → should show completed reads + writes
	nodeID := idwrap.NewNow()

	initialData := map[string]any{
		"input1": "test_input",
		"input2": 42,
	}

	// Create a node that reads, writes some data, then delays
	testNode := NewCancelableTestNode(nodeID, "testNode", []idwrap.IDWrap{}).
		WithReads("input1", "input2").
		WithWrites(map[string]interface{}{
			"intermediate": "step1_complete",
			"status":       "processing",
		}).
		WithDelays(0, 0, 0, 100*time.Millisecond) // Delay after writes

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		nodeID: testNode,
	}

	var statuses []runner.FlowNodeStatus
	statusFunc := func(status runner.FlowNodeStatus) {
		statuses = append(statuses, status)
	}

	req := &node.FlowNodeRequest{
		VarMap:           initialData,
		ReadWriteLock:    &sync.RWMutex{},
		NodeMap:          nodeMap,
		EdgeSourceMap:    make(edge.EdgesMap),
		Timeout:          5 * time.Second,
		LogPushFunc:      statusFunc,
		PendingAtmoicMap: make(map[idwrap.IDWrap]uint32),
	}

	// Cancel after writes should complete
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Millisecond)
	defer cancel()

	err := RunNodeSync(ctx, nodeID, req, statusFunc)
	if err == nil {
		t.Error("Expected error due to context cancellation")
	}

	// Find the canceled status
	var canceledStatus *runner.FlowNodeStatus
	for i := range statuses {
		if statuses[i].State == mnnode.NODE_STATE_CANCELED {
			canceledStatus = &statuses[i]
			break
		}
	}

	if canceledStatus == nil {
		t.Fatal("Expected to find a canceled node status")
	}

	// Should have input data with tracked reads
	if canceledStatus.InputData == nil {
		t.Error("Expected input data to be captured for canceled node")
	}

	// Should have output data with completed writes
	if canceledStatus.OutputData == nil {
		t.Error("Expected output data to be captured for canceled node")
	} else {
		outputMap, ok := canceledStatus.OutputData.(map[string]any)
		if !ok {
			t.Errorf("Expected output data to be a map, got %T", canceledStatus.OutputData)
		} else {
			// Check for nested structure: testNode -> {intermediate, status}
			if testNodeData, exists := outputMap["testNode"]; exists {
				if nodeMap, ok := testNodeData.(map[string]any); ok {
					if nodeMap["intermediate"] != "step1_complete" {
						t.Errorf("Expected intermediate='step1_complete', got %v", nodeMap["intermediate"])
					}
					if nodeMap["status"] != "processing" {
						t.Errorf("Expected status='processing', got %v", nodeMap["status"])
					}
				} else {
					t.Errorf("Expected testNode data to be a map, got %T", testNodeData)
				}
			} else {
				t.Error("Expected testNode key in output data")
			}
		}
	}
}

func TestCanceledNode_ConcurrentAccess(t *testing.T) {
	// Test case: Multiple nodes canceled simultaneously with concurrent read/write operations
	numNodes := 5
	nodeIDs := make([]idwrap.IDWrap, numNodes)
	nodeMap := make(map[idwrap.IDWrap]node.FlowNode)

	for i := 0; i < numNodes; i++ {
		nodeIDs[i] = idwrap.NewNow()

		// Each node reads shared data and writes unique data
		testNode := NewCancelableTestNode(nodeIDs[i], fmt.Sprintf("node%d", i), []idwrap.IDWrap{}).
			WithReads("shared_input").
			WithWrites(map[string]interface{}{
				fmt.Sprintf("result_%d", i): fmt.Sprintf("output_%d", i),
				"timestamp":                 time.Now().Unix(),
			}).
			WithDelays(10*time.Millisecond, 0, 10*time.Millisecond, 50*time.Millisecond)

		nodeMap[nodeIDs[i]] = testNode
	}

	initialData := map[string]any{
		"shared_input": "shared_value",
	}

	var statuses []runner.FlowNodeStatus
	var statusMutex sync.Mutex
	statusFunc := func(status runner.FlowNodeStatus) {
		statusMutex.Lock()
		statuses = append(statuses, status)
		statusMutex.Unlock()
	}

	req := &node.FlowNodeRequest{
		VarMap:           initialData,
		ReadWriteLock:    &sync.RWMutex{},
		NodeMap:          nodeMap,
		EdgeSourceMap:    make(edge.EdgesMap),
		Timeout:          5 * time.Second,
		LogPushFunc:      statusFunc,
		PendingAtmoicMap: make(map[idwrap.IDWrap]uint32),
	}

	// Cancel context to trigger cancellation during execution
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	// Run multiple nodes concurrently
	var wg sync.WaitGroup
	for _, nodeID := range nodeIDs {
		wg.Add(1)
		go func(id idwrap.IDWrap) {
			defer wg.Done()
			_ = RunNodeSync(ctx, id, req, statusFunc)
		}(nodeID)
	}

	wg.Wait()

	// Count canceled nodes and verify they have appropriate data
	canceledCount := 0
	statusMutex.Lock()
	for _, status := range statuses {
		if status.State == mnnode.NODE_STATE_CANCELED {
			canceledCount++

			// Each canceled node should have input data from reads that completed
			if status.InputData != nil {
				inputMap, ok := status.InputData.(map[string]any)
				if ok {
					if variables, exists := inputMap["variables"]; exists {
						variablesMap, ok := variables.(map[string]any)
						if ok && len(variablesMap) > 0 {
							// Node managed to read some data before cancellation
							t.Logf("Node %s read %d variables before cancellation", status.Name, len(variablesMap))
						}
					}
				}
			}

			// Some nodes might have output data if they completed writes before cancellation
			if status.OutputData != nil {
				outputMap, ok := status.OutputData.(map[string]any)
				if ok && len(outputMap) > 0 {
					t.Logf("Node %s wrote %d variables before cancellation", status.Name, len(outputMap))
				}
			}
		}
	}
	statusMutex.Unlock()

	if canceledCount == 0 {
		t.Error("Expected at least some nodes to be canceled")
	}

	t.Logf("Successfully tested concurrent cancellation with %d canceled nodes", canceledCount)
}

func TestCanceledNode_TimeoutVsManualCancel(t *testing.T) {
	// Test case: Distinguish between timeout-based and manual cancellation
	tests := []struct {
		name         string
		useTimeout   bool
		cancelManual bool
		expectCancel bool
		timeoutDur   time.Duration
		nodeDelay    time.Duration
	}{
		{
			name:         "Manual cancellation",
			useTimeout:   false,
			cancelManual: true,
			expectCancel: true,
			nodeDelay:    100 * time.Millisecond,
		},
		{
			name:         "Timeout cancellation",
			useTimeout:   true,
			cancelManual: false,
			expectCancel: true,
			timeoutDur:   50 * time.Millisecond,
			nodeDelay:    100 * time.Millisecond,
		},
		{
			name:         "No cancellation",
			useTimeout:   false,
			cancelManual: false,
			expectCancel: false,
			nodeDelay:    10 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodeID := idwrap.NewNow()

			testNode := NewCancelableTestNode(nodeID, "testNode", []idwrap.IDWrap{}).
				WithReads("input").
				WithWrites(map[string]interface{}{"result": "success"}).
				WithDelays(0, 0, 0, tt.nodeDelay)

			nodeMap := map[idwrap.IDWrap]node.FlowNode{
				nodeID: testNode,
			}

			var statuses []runner.FlowNodeStatus
			statusFunc := func(status runner.FlowNodeStatus) {
				statuses = append(statuses, status)
			}

			initialData := map[string]any{"input": "test_value"}
			req := &node.FlowNodeRequest{
				VarMap:           initialData,
				ReadWriteLock:    &sync.RWMutex{},
				NodeMap:          nodeMap,
				EdgeSourceMap:    make(edge.EdgesMap),
				Timeout:          5 * time.Second,
				LogPushFunc:      statusFunc,
				PendingAtmoicMap: make(map[idwrap.IDWrap]uint32),
			}

			var ctx context.Context
			var cancel context.CancelFunc

			if tt.useTimeout {
				ctx, cancel = context.WithTimeout(context.Background(), tt.timeoutDur)
			} else {
				ctx, cancel = context.WithCancel(context.Background())
			}
			defer cancel()

			// Start execution
			done := make(chan error, 1)
			go func() {
				err := RunNodeSync(ctx, nodeID, req, statusFunc)
				done <- err
			}()

			// Manual cancellation if requested
			if tt.cancelManual {
				time.Sleep(25 * time.Millisecond)
				cancel()
			}

			// Wait for completion
			err := <-done

			// Check results
			if tt.expectCancel {
				if err == nil {
					t.Error("Expected cancellation error")
				}

				// Should have canceled status with tracked data
				var foundCanceled bool
				for _, status := range statuses {
					if status.State == mnnode.NODE_STATE_CANCELED {
						foundCanceled = true

						// Should have input data from reads
						if status.InputData == nil {
							t.Error("Expected input data for canceled node")
						}

						// Might have output data if writes completed before cancellation
						break
					}
				}

				if !foundCanceled {
					t.Error("Expected to find canceled node status")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}

				// Should have success status
				var foundSuccess bool
				for _, status := range statuses {
					if status.State == mnnode.NODE_STATE_SUCCESS {
						foundSuccess = true
						break
					}
				}

				if !foundSuccess {
					t.Error("Expected to find success node status")
				}
			}
		})
	}
}

func TestCanceledNode_AsyncExecution(t *testing.T) {
	// Test case: Verify cancellation tracking works with async execution
	nodeID := idwrap.NewNow()

	testNode := NewCancelableTestNode(nodeID, "asyncNode", []idwrap.IDWrap{}).
		WithReads("async_input").
		WithWrites(map[string]interface{}{"async_result": "processing"}).
		WithDelays(0, 50*time.Millisecond, 0, 0) // Delay after reads

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		nodeID: testNode,
	}

	var statuses []runner.FlowNodeStatus
	statusFunc := func(status runner.FlowNodeStatus) {
		statuses = append(statuses, status)
	}

	initialData := map[string]any{"async_input": "async_test_value"}
	req := &node.FlowNodeRequest{
		VarMap:           initialData,
		ReadWriteLock:    &sync.RWMutex{},
		NodeMap:          nodeMap,
		EdgeSourceMap:    make(edge.EdgesMap),
		Timeout:          20 * time.Millisecond, // Short timeout to trigger cancellation
		LogPushFunc:      statusFunc,
		PendingAtmoicMap: make(map[idwrap.IDWrap]uint32),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := RunNodeASync(ctx, nodeID, req, statusFunc)
	if err == nil {
		t.Error("Expected timeout error from async execution")
	}

	// Find the canceled status
	var canceledStatus *runner.FlowNodeStatus
	for i := range statuses {
		if statuses[i].State == mnnode.NODE_STATE_CANCELED {
			canceledStatus = &statuses[i]
			break
		}
	}

	if canceledStatus == nil {
		t.Fatal("Expected to find a canceled node status in async execution")
	}

	// Verify tracking data is captured for async canceled node
	if canceledStatus.InputData == nil {
		t.Error("Expected input data to be captured for async canceled node")
	}

	// Output data might be present if writes completed before timeout
	t.Logf("Async canceled node - InputData: %v, OutputData: %v",
		canceledStatus.InputData, canceledStatus.OutputData)
}

func TestCanceledNode_HeaderVsBodyDataScenario(t *testing.T) {
	// Test case: Simulate real scenario where headers are read but body processing is canceled
	nodeID := idwrap.NewNow()

	// Simulate reading headers (fast) then processing body (slow)
	testNode := NewCancelableTestNode(nodeID, "httpNode", []idwrap.IDWrap{}).
		WithReads("headers", "content-type", "status-code").       // Fast header reads
		WithDelays(0, 5*time.Millisecond, 50*time.Millisecond, 0). // Delay before body processing
		WithWrites(map[string]interface{}{
			"parsed_body":   "processed_content",
			"response_time": 123,
			"processed_at":  time.Now(),
		})

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		nodeID: testNode,
	}

	var statuses []runner.FlowNodeStatus
	statusFunc := func(status runner.FlowNodeStatus) {
		statuses = append(statuses, status)
	}

	// Simulate HTTP response data
	initialData := map[string]any{
		"headers":      map[string]string{"authorization": "bearer token", "user-agent": "test"},
		"content-type": "application/json",
		"status-code":  200,
		"body":         `{"data": "large response body that takes time to process"}`,
	}

	req := &node.FlowNodeRequest{
		VarMap:           initialData,
		ReadWriteLock:    &sync.RWMutex{},
		NodeMap:          nodeMap,
		EdgeSourceMap:    make(edge.EdgesMap),
		Timeout:          5 * time.Second,
		LogPushFunc:      statusFunc,
		PendingAtmoicMap: make(map[idwrap.IDWrap]uint32),
	}

	// Cancel during body processing (after headers read)
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()

	err := RunNodeSync(ctx, nodeID, req, statusFunc)
	if err == nil {
		t.Error("Expected cancellation error")
	}

	// Find the canceled status
	var canceledStatus *runner.FlowNodeStatus
	for i := range statuses {
		if statuses[i].State == mnnode.NODE_STATE_CANCELED {
			canceledStatus = &statuses[i]
			break
		}
	}

	if canceledStatus == nil {
		t.Fatal("Expected to find a canceled node status")
	}

	// Should show header data that was read successfully
	if canceledStatus.InputData == nil {
		t.Error("Expected input data (headers) to be captured")
	} else {
		inputMap, ok := canceledStatus.InputData.(map[string]any)
		if ok {
			if variables, exists := inputMap["variables"]; exists {
				variablesMap, ok := variables.(map[string]any)
				if ok {
					// Should have captured the fast header reads
					if len(variablesMap) == 0 {
						t.Error("Expected header variables to be tracked")
					}

					expectedHeaders := []string{"headers", "content-type", "status-code"}
					for _, header := range expectedHeaders {
						if _, exists := variablesMap[header]; !exists {
							t.Errorf("Expected header '%s' to be tracked", header)
						}
					}
				}
			}
		}
	}

	// Should NOT show body processing output (since it was canceled)
	if canceledStatus.OutputData != nil {
		outputMap, ok := canceledStatus.OutputData.(map[string]any)
		if ok && len(outputMap) > 0 {
			t.Errorf("Expected no body processing output since canceled, got %v", outputMap)
		}
	}

	t.Log("Successfully verified header tracking vs body processing cancellation scenario")
}

func TestCanceledNode_RaceConditionSafety(t *testing.T) {
	// Test case: Ensure thread safety during cancellation tracking capture
	nodeID := idwrap.NewNow()

	var accessCount int64

	testNode := NewCancelableTestNode(nodeID, "raceNode", []idwrap.IDWrap{}).
		WithReads("shared_resource").
		WithDelays(0, 20*time.Millisecond, 0, 20*time.Millisecond).
		WithWrites(map[string]interface{}{
			"access_count": &accessCount,
			"thread_id":    "test_thread",
		})

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		nodeID: testNode,
	}

	var statuses []runner.FlowNodeStatus
	var statusMutex sync.Mutex
	statusFunc := func(status runner.FlowNodeStatus) {
		statusMutex.Lock()
		statuses = append(statuses, status)
		statusMutex.Unlock()
	}

	initialData := map[string]any{
		"shared_resource": "race_test_data",
	}

	req := &node.FlowNodeRequest{
		VarMap:           initialData,
		ReadWriteLock:    &sync.RWMutex{},
		NodeMap:          nodeMap,
		EdgeSourceMap:    make(edge.EdgesMap),
		Timeout:          5 * time.Second,
		LogPushFunc:      statusFunc,
		PendingAtmoicMap: make(map[idwrap.IDWrap]uint32),
	}

	// Run multiple concurrent executions with rapid cancellation
	numGoroutines := 10
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(iteration int) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(),
				time.Duration(10+iteration)*time.Millisecond)
			defer cancel()

			_ = RunNodeSync(ctx, nodeID, req, statusFunc)
			atomic.AddInt64(&accessCount, 1)
		}(i)
	}

	wg.Wait()

	statusMutex.Lock()
	defer statusMutex.Unlock()

	// Verify no race conditions occurred and data is consistent
	cancelCount := 0
	for _, status := range statuses {
		if status.State == mnnode.NODE_STATE_CANCELED {
			cancelCount++

			// Verify data consistency - no nil pointer panics or corrupted data
			if status.InputData != nil {
				if _, ok := status.InputData.(map[string]any); !ok {
					t.Errorf("InputData type corruption detected: %T", status.InputData)
				}
			}

			if status.OutputData != nil {
				if _, ok := status.OutputData.(map[string]any); !ok {
					t.Errorf("OutputData type corruption detected: %T", status.OutputData)
				}
			}
		}
	}

	if cancelCount == 0 {
		t.Error("Expected some cancellations in race condition test")
	}

	t.Logf("Race condition test completed with %d cancellations, no data corruption detected", cancelCount)
}
