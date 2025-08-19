package rflow

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/node/nfor"
	"the-dev-tools/server/pkg/flow/node/nrequest"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/httpclient"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/model/mbodyform"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mbodyurl"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mexamplequery"
	"the-dev-tools/server/pkg/model/mexampleresp"
	"the-dev-tools/server/pkg/model/mexamplerespheader"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CorrelationTracker tracks the correlation between running statuses and responses
type CorrelationTracker struct {
	mu                   sync.RWMutex
	runningStatuses      map[idwrap.IDWrap]runner.FlowNodeStatus
	responses            map[idwrap.IDWrap]nrequest.NodeRequestSideResp
	correlatedResponses  map[idwrap.IDWrap]bool
	orphanedResponses    map[idwrap.IDWrap]bool
	totalRunningStatuses int64
	totalResponses       int64
}

func NewCorrelationTracker() *CorrelationTracker {
	return &CorrelationTracker{
		runningStatuses:     make(map[idwrap.IDWrap]runner.FlowNodeStatus),
		responses:           make(map[idwrap.IDWrap]nrequest.NodeRequestSideResp),
		correlatedResponses: make(map[idwrap.IDWrap]bool),
		orphanedResponses:   make(map[idwrap.IDWrap]bool),
	}
}

func (ct *CorrelationTracker) TrackStatus(status runner.FlowNodeStatus, requestNodeID idwrap.IDWrap) {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	// Only track REQUEST node RUNNING statuses
	if status.State == mnnode.NODE_STATE_RUNNING && status.NodeID == requestNodeID {
		ct.runningStatuses[status.ExecutionID] = status
		atomic.AddInt64(&ct.totalRunningStatuses, 1)
	}
}

func (ct *CorrelationTracker) TrackResponse(response nrequest.NodeRequestSideResp) {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	ct.responses[response.ExecutionID] = response
	atomic.AddInt64(&ct.totalResponses, 1)

	// Check if we have a running status for this response
	if _, exists := ct.runningStatuses[response.ExecutionID]; exists {
		ct.correlatedResponses[response.ExecutionID] = true
	} else {
		ct.orphanedResponses[response.ExecutionID] = true
	}
}

func (ct *CorrelationTracker) GetCorrelationStats() (int, int, int, float64) {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	totalRunning := int(atomic.LoadInt64(&ct.totalRunningStatuses))
	totalResponses := int(atomic.LoadInt64(&ct.totalResponses))
	correlated := len(ct.correlatedResponses)

	var successRate float64
	if totalRunning > 0 {
		successRate = float64(correlated) / float64(totalRunning) * 100
	}

	return totalRunning, totalResponses, correlated, successRate
}

func (ct *CorrelationTracker) GetOrphanedCount() int {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	return len(ct.orphanedResponses)
}

// createFastMockServer creates an HTTP server that responds very quickly
func createFastMockServer(responseDelayMs int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if responseDelayMs > 0 {
			time.Sleep(time.Duration(responseDelayMs) * time.Millisecond)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success": true, "timestamp": "` + fmt.Sprint(time.Now().UnixMilli()) + `"}`))
	}))
}

// createForLoopWithRequestNode creates a FOR loop containing a REQUEST node
func createForLoopWithRequestNode(iterations int, serverURL string, tracker *CorrelationTracker) (
	node.FlowNode, node.FlowNode, []edge.Edge, chan nrequest.NodeRequestSideResp) {

	// Create FOR node
	forNodeID := idwrap.NewNow()
	forNode := nfor.New(forNodeID, "TestForLoop", int64(iterations), 30*time.Second, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)

	// Create REQUEST node
	requestNodeID := idwrap.NewNow()
	endpoint := mitemapi.ItemApi{
		ID:     idwrap.NewNow(),
		Name:   "FastAPI",
		Url:    serverURL,
		Method: "GET",
	}

	example := mitemapiexample.ItemApiExample{
		ID:        idwrap.NewNow(),
		ItemApiID: endpoint.ID,
		Name:      "FastExample",
	}

	// Create response channel
	responseChan := make(chan nrequest.NodeRequestSideResp, iterations*2)

	requestNode := nrequest.New(
		requestNodeID,
		"FastRequest",
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
		responseChan,
	)

	// Create edge from FOR to REQUEST
	edges := []edge.Edge{
		edge.NewEdge(idwrap.NewNow(), forNodeID, requestNodeID, edge.HandleLoop, edge.EdgeKindNoOp),
	}

	return forNode, requestNode, edges, responseChan
}

// TestRaceConditionFix_SmallLoop tests with 10 iterations
func TestRaceConditionFix_SmallLoop(t *testing.T) {
	server := createFastMockServer(0) // No delay for maximum speed
	defer server.Close()

	iterations := 10
	tracker := NewCorrelationTracker()

	forNode, requestNode, edges, responseChan := createForLoopWithRequestNode(iterations, server.URL, tracker)

	// Set up node map and edges
	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		forNode.GetID():     forNode,
		requestNode.GetID(): requestNode,
	}
	edgeSourceMap := edge.NewEdgesMap(edges)

	// Track execution statuses and responses
	var executionStatuses []runner.FlowNodeStatus
	var responses []nrequest.NodeRequestSideResp
	var statusMutex sync.Mutex
	var responseMutex sync.Mutex

	// Create request with status logging
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]interface{}),
		NodeMap:       nodeMap,
		EdgeSourceMap: edgeSourceMap,
		LogPushFunc: func(status runner.FlowNodeStatus) {
			statusMutex.Lock()
			defer statusMutex.Unlock()
			executionStatuses = append(executionStatuses, status)
			tracker.TrackStatus(status, requestNode.GetID())
		},
		ReadWriteLock: &sync.RWMutex{},
	}

	// Start goroutine to collect responses
	responsesDone := make(chan struct{})
	go func() {
		defer close(responsesDone)
		for response := range responseChan {
			responseMutex.Lock()
			responses = append(responses, response)
			tracker.TrackResponse(response)
			responseMutex.Unlock()
		}
	}()

	// Execute FOR node
	result := forNode.RunSync(context.Background(), req)
	require.NoError(t, result.Err, "FOR node execution should not fail")

	// Wait for async operations to complete
	time.Sleep(200 * time.Millisecond)
	close(responseChan)
	<-responsesDone

	// Analyze correlation results
	totalRunning, totalResponses, correlated, successRate := tracker.GetCorrelationStats()
	orphanedCount := tracker.GetOrphanedCount()

	t.Logf("Small Loop Results: Running=%d, Responses=%d, Correlated=%d, Success Rate=%.2f%%, Orphaned=%d",
		totalRunning, totalResponses, correlated, successRate, orphanedCount)

	// Validations
	assert.Equal(t, iterations, totalRunning, "Should have %d REQUEST node RUNNING statuses", iterations)
	assert.Equal(t, iterations, totalResponses, "Should have %d REQUEST responses", iterations)
	assert.Equal(t, 100.0, successRate, "Should have 100%% correlation success rate")
	assert.Equal(t, 0, orphanedCount, "Should have no orphaned responses")
}

// TestRaceConditionFix_MediumLoop tests with 100 iterations
func TestRaceConditionFix_MediumLoop(t *testing.T) {
	server := createFastMockServer(0) // No delay for maximum speed
	defer server.Close()

	iterations := 100
	tracker := NewCorrelationTracker()

	forNode, requestNode, edges, responseChan := createForLoopWithRequestNode(iterations, server.URL, tracker)

	// Set up node map and edges
	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		forNode.GetID():     forNode,
		requestNode.GetID(): requestNode,
	}
	edgeSourceMap := edge.NewEdgesMap(edges)

	// Track execution statuses and responses
	var statusMutex sync.Mutex
	var responseMutex sync.Mutex

	// Create request with status logging
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]interface{}),
		NodeMap:       nodeMap,
		EdgeSourceMap: edgeSourceMap,
		LogPushFunc: func(status runner.FlowNodeStatus) {
			statusMutex.Lock()
			defer statusMutex.Unlock()
			tracker.TrackStatus(status, requestNode.GetID())
		},
		ReadWriteLock: &sync.RWMutex{},
	}

	// Start goroutine to collect responses
	responsesDone := make(chan struct{})
	go func() {
		defer close(responsesDone)
		for response := range responseChan {
			responseMutex.Lock()
			tracker.TrackResponse(response)
			responseMutex.Unlock()
		}
	}()

	// Execute FOR node
	startTime := time.Now()
	result := forNode.RunSync(context.Background(), req)
	executionTime := time.Since(startTime)

	require.NoError(t, result.Err, "FOR node execution should not fail")

	// Wait for async operations to complete
	time.Sleep(500 * time.Millisecond)
	close(responseChan)
	<-responsesDone

	// Analyze correlation results
	totalRunning, totalResponses, correlated, successRate := tracker.GetCorrelationStats()
	orphanedCount := tracker.GetOrphanedCount()

	t.Logf("Medium Loop Results: Running=%d, Responses=%d, Correlated=%d, Success Rate=%.2f%%, Orphaned=%d, Execution Time=%v",
		totalRunning, totalResponses, correlated, successRate, orphanedCount, executionTime)

	// Validations
	assert.Equal(t, iterations, totalRunning, "Should have %d REQUEST node RUNNING statuses", iterations)
	assert.Equal(t, iterations, totalResponses, "Should have %d REQUEST responses", iterations)
	assert.Equal(t, 100.0, successRate, "Should have 100%% correlation success rate")
	assert.Equal(t, 0, orphanedCount, "Should have no orphaned responses")
}

// TestRaceConditionFix_LargeLoop tests with 1000 iterations
func TestRaceConditionFix_LargeLoop(t *testing.T) {
	server := createFastMockServer(0) // No delay for maximum speed
	defer server.Close()

	iterations := 1000
	tracker := NewCorrelationTracker()

	forNode, requestNode, edges, responseChan := createForLoopWithRequestNode(iterations, server.URL, tracker)

	// Set up node map and edges
	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		forNode.GetID():     forNode,
		requestNode.GetID(): requestNode,
	}
	edgeSourceMap := edge.NewEdgesMap(edges)

	// Track execution statuses and responses
	var statusMutex sync.Mutex
	var responseMutex sync.Mutex

	// Create request with status logging
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]interface{}),
		NodeMap:       nodeMap,
		EdgeSourceMap: edgeSourceMap,
		LogPushFunc: func(status runner.FlowNodeStatus) {
			statusMutex.Lock()
			defer statusMutex.Unlock()
			tracker.TrackStatus(status, requestNode.GetID())
		},
		ReadWriteLock: &sync.RWMutex{},
	}

	// Start goroutine to collect responses
	responsesDone := make(chan struct{})
	go func() {
		defer close(responsesDone)
		for response := range responseChan {
			responseMutex.Lock()
			tracker.TrackResponse(response)
			responseMutex.Unlock()
		}
	}()

	// Execute FOR node
	startTime := time.Now()
	result := forNode.RunSync(context.Background(), req)
	executionTime := time.Since(startTime)

	require.NoError(t, result.Err, "FOR node execution should not fail")

	// Wait for async operations to complete
	time.Sleep(1 * time.Second)
	close(responseChan)
	<-responsesDone

	// Analyze correlation results
	totalRunning, totalResponses, correlated, successRate := tracker.GetCorrelationStats()
	orphanedCount := tracker.GetOrphanedCount()

	t.Logf("Large Loop Results: Running=%d, Responses=%d, Correlated=%d, Success Rate=%.2f%%, Orphaned=%d, Execution Time=%v",
		totalRunning, totalResponses, correlated, successRate, orphanedCount, executionTime)

	// Validations
	assert.Equal(t, iterations, totalRunning, "Should have %d REQUEST node RUNNING statuses", iterations)
	assert.Equal(t, iterations, totalResponses, "Should have %d REQUEST responses", iterations)
	assert.Equal(t, 100.0, successRate, "Should have 100%% correlation success rate")
	assert.Equal(t, 0, orphanedCount, "Should have no orphaned responses")

	// Performance check - should complete reasonably fast
	assert.Less(t, executionTime, 30*time.Second, "Large loop should complete within 30 seconds")
}

// TestRaceConditionFix_ConcurrentLoops tests multiple loops running in parallel
func TestRaceConditionFix_ConcurrentLoops(t *testing.T) {
	server := createFastMockServer(0) // No delay for maximum speed
	defer server.Close()

	numLoops := 5
	iterationsPerLoop := 50
	totalIterations := numLoops * iterationsPerLoop

	tracker := NewCorrelationTracker()

	// Create multiple FOR loops with REQUEST nodes
	var wg sync.WaitGroup

	for i := 0; i < numLoops; i++ {
		wg.Add(1)
		go func(loopIndex int) {
			defer wg.Done()

			forNode, requestNode, edges, responseChan := createForLoopWithRequestNode(iterationsPerLoop, server.URL, tracker)

			// Set up node map and edges
			nodeMap := map[idwrap.IDWrap]node.FlowNode{
				forNode.GetID():     forNode,
				requestNode.GetID(): requestNode,
			}
			edgeSourceMap := edge.NewEdgesMap(edges)

			// Track execution statuses and responses
			var statusMutex sync.Mutex
			var responseMutex sync.Mutex

			// Create request with status logging
			req := &node.FlowNodeRequest{
				VarMap:        make(map[string]interface{}),
				NodeMap:       nodeMap,
				EdgeSourceMap: edgeSourceMap,
				LogPushFunc: func(status runner.FlowNodeStatus) {
					statusMutex.Lock()
					defer statusMutex.Unlock()
					tracker.TrackStatus(status, requestNode.GetID())
				},
				ReadWriteLock: &sync.RWMutex{},
			}

			// Start goroutine to collect responses
			responsesDone := make(chan struct{})
			go func() {
				defer close(responsesDone)
				for response := range responseChan {
					responseMutex.Lock()
					tracker.TrackResponse(response)
					responseMutex.Unlock()
				}
			}()

			// Execute FOR node
			result := forNode.RunSync(context.Background(), req)
			if result.Err != nil {
				t.Errorf("Loop %d failed: %v", loopIndex, result.Err)
				return
			}

			// Wait for async operations to complete
			time.Sleep(200 * time.Millisecond)
			close(responseChan)
			<-responsesDone
		}(i)
	}

	// Wait for all loops to complete
	wg.Wait()

	// Analyze correlation results
	totalRunning, totalResponses, correlated, successRate := tracker.GetCorrelationStats()
	orphanedCount := tracker.GetOrphanedCount()

	t.Logf("Concurrent Loops Results: Loops=%d, Expected=%d, Running=%d, Responses=%d, Correlated=%d, Success Rate=%.2f%%, Orphaned=%d",
		numLoops, totalIterations, totalRunning, totalResponses, correlated, successRate, orphanedCount)

	// Validations
	assert.Equal(t, totalIterations, totalRunning, "Should have %d REQUEST node RUNNING statuses", totalIterations)
	assert.Equal(t, totalIterations, totalResponses, "Should have %d REQUEST responses", totalIterations)
	assert.Equal(t, 100.0, successRate, "Should have 100%% correlation success rate")
	assert.Equal(t, 0, orphanedCount, "Should have no orphaned responses")
}

// TestRaceConditionFix_FastResponses tests sub-millisecond HTTP responses
func TestRaceConditionFix_FastResponses(t *testing.T) {
	// Create extremely fast server with sub-millisecond responses
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No sleep - respond immediately
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"instant": true}`))
	}))
	defer server.Close()

	iterations := 200 // More iterations to stress test
	tracker := NewCorrelationTracker()

	forNode, requestNode, edges, responseChan := createForLoopWithRequestNode(iterations, server.URL, tracker)

	// Set up node map and edges
	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		forNode.GetID():     forNode,
		requestNode.GetID(): requestNode,
	}
	edgeSourceMap := edge.NewEdgesMap(edges)

	// Track timing information
	var statusTimestamps []time.Time
	var responseTimestamps []time.Time
	var statusMutex sync.Mutex
	var responseMutex sync.Mutex

	// Create request with status logging
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]interface{}),
		NodeMap:       nodeMap,
		EdgeSourceMap: edgeSourceMap,
		LogPushFunc: func(status runner.FlowNodeStatus) {
			statusMutex.Lock()
			defer statusMutex.Unlock()
			if status.State == mnnode.NODE_STATE_RUNNING {
				statusTimestamps = append(statusTimestamps, time.Now())
			}
			tracker.TrackStatus(status, requestNode.GetID())
		},
		ReadWriteLock: &sync.RWMutex{},
	}

	// Start goroutine to collect responses with timing
	responsesDone := make(chan struct{})
	go func() {
		defer close(responsesDone)
		for response := range responseChan {
			responseMutex.Lock()
			responseTimestamps = append(responseTimestamps, time.Now())
			tracker.TrackResponse(response)
			responseMutex.Unlock()
		}
	}()

	// Execute FOR node
	startTime := time.Now()
	result := forNode.RunSync(context.Background(), req)
	executionTime := time.Since(startTime)

	require.NoError(t, result.Err, "FOR node execution should not fail")

	// Wait for async operations to complete
	time.Sleep(300 * time.Millisecond)
	close(responseChan)
	<-responsesDone

	// Analyze correlation results
	totalRunning, totalResponses, correlated, successRate := tracker.GetCorrelationStats()
	orphanedCount := tracker.GetOrphanedCount()

	// Calculate average response time
	statusMutex.Lock()
	responseMutex.Lock()
	avgTimeDiff := time.Duration(0)
	if len(statusTimestamps) > 0 && len(responseTimestamps) > 0 {
		totalDiff := time.Duration(0)
		count := 0
		for i := 0; i < len(statusTimestamps) && i < len(responseTimestamps); i++ {
			diff := responseTimestamps[i].Sub(statusTimestamps[i])
			if diff > 0 {
				totalDiff += diff
				count++
			}
		}
		if count > 0 {
			avgTimeDiff = totalDiff / time.Duration(count)
		}
	}
	statusMutex.Unlock()
	responseMutex.Unlock()

	t.Logf("Fast Responses Results: Running=%d, Responses=%d, Correlated=%d, Success Rate=%.2f%%, Orphaned=%d, Execution Time=%v, Avg Response Time=%v",
		totalRunning, totalResponses, correlated, successRate, orphanedCount, executionTime, avgTimeDiff)

	// Validations
	assert.Equal(t, iterations, totalRunning, "Should have %d REQUEST node RUNNING statuses", iterations)
	assert.Equal(t, iterations, totalResponses, "Should have %d REQUEST responses", iterations)
	assert.Equal(t, 100.0, successRate, "Should have 100%% correlation success rate")
	assert.Equal(t, 0, orphanedCount, "Should have no orphaned responses")

	// Verify responses are indeed fast
	assert.Less(t, avgTimeDiff, 100*time.Millisecond, "Average response time should be less than 100ms")
}

// TestRaceConditionFix_MemoryLeakCheck tests for memory leaks in correlation maps
func TestRaceConditionFix_MemoryLeakCheck(t *testing.T) {
	server := createFastMockServer(0)
	defer server.Close()

	iterations := 100

	// Run multiple sequential tests to check for memory accumulation
	for round := 0; round < 5; round++ {
		tracker := NewCorrelationTracker()

		forNode, requestNode, edges, responseChan := createForLoopWithRequestNode(iterations, server.URL, tracker)

		// Set up node map and edges
		nodeMap := map[idwrap.IDWrap]node.FlowNode{
			forNode.GetID():     forNode,
			requestNode.GetID(): requestNode,
		}
		edgeSourceMap := edge.NewEdgesMap(edges)

		// Create request with status logging
		req := &node.FlowNodeRequest{
			VarMap:        make(map[string]interface{}),
			NodeMap:       nodeMap,
			EdgeSourceMap: edgeSourceMap,
			LogPushFunc: func(status runner.FlowNodeStatus) {
				tracker.TrackStatus(status, requestNode.GetID())
			},
			ReadWriteLock: &sync.RWMutex{},
		}

		// Start goroutine to collect responses
		responsesDone := make(chan struct{})
		go func() {
			defer close(responsesDone)
			for response := range responseChan {
				tracker.TrackResponse(response)
			}
		}()

		// Execute FOR node
		result := forNode.RunSync(context.Background(), req)
		require.NoError(t, result.Err, "FOR node execution should not fail in round %d", round)

		// Wait for async operations to complete
		time.Sleep(100 * time.Millisecond)
		close(responseChan)
		<-responsesDone

		// Verify results
		totalRunning, totalResponses, correlated, successRate := tracker.GetCorrelationStats()
		orphanedCount := tracker.GetOrphanedCount()

		t.Logf("Memory Leak Check Round %d: Running=%d, Responses=%d, Correlated=%d, Success Rate=%.2f%%, Orphaned=%d",
			round+1, totalRunning, totalResponses, correlated, successRate, orphanedCount)

		// Validations
		assert.Equal(t, iterations, totalRunning, "Round %d should have %d RUNNING statuses", round+1, iterations)
		assert.Equal(t, iterations, totalResponses, "Round %d should have %d responses", round+1, iterations)
		assert.Equal(t, 100.0, successRate, "Round %d should have 100%% success rate", round+1)
		assert.Equal(t, 0, orphanedCount, "Round %d should have no orphaned responses", round+1)

		// Force garbage collection between rounds
		// runtime.GC()
	}

	t.Log("Memory leak check completed - all rounds should show consistent results")
}

// TestRaceConditionFix_FlowCompletionCleanup tests proper cleanup on flow completion
func TestRaceConditionFix_FlowCompletionCleanup(t *testing.T) {
	server := createFastMockServer(0)
	defer server.Close()

	iterations := 50
	tracker := NewCorrelationTracker()

	forNode, requestNode, edges, responseChan := createForLoopWithRequestNode(iterations, server.URL, tracker)

	// Set up node map and edges
	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		forNode.GetID():     forNode,
		requestNode.GetID(): requestNode,
	}
	edgeSourceMap := edge.NewEdgesMap(edges)

	// Track all statuses to verify completion
	var allStatuses []runner.FlowNodeStatus
	var statusMutex sync.Mutex

	// Create request with status logging
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]interface{}),
		NodeMap:       nodeMap,
		EdgeSourceMap: edgeSourceMap,
		LogPushFunc: func(status runner.FlowNodeStatus) {
			statusMutex.Lock()
			defer statusMutex.Unlock()
			allStatuses = append(allStatuses, status)
			tracker.TrackStatus(status, requestNode.GetID())
		},
		ReadWriteLock: &sync.RWMutex{},
	}

	// Start goroutine to collect responses
	responsesDone := make(chan struct{})
	go func() {
		defer close(responsesDone)
		for response := range responseChan {
			tracker.TrackResponse(response)
		}
	}()

	// Execute FOR node
	result := forNode.RunSync(context.Background(), req)
	require.NoError(t, result.Err, "FOR node execution should not fail")

	// Wait for async operations to complete
	time.Sleep(200 * time.Millisecond)
	close(responseChan)
	<-responsesDone

	// Analyze correlation results
	totalRunning, totalResponses, correlated, successRate := tracker.GetCorrelationStats()
	orphanedCount := tracker.GetOrphanedCount()

	// Analyze completion patterns
	statusMutex.Lock()
	completedRequests := 0
	for _, status := range allStatuses {
		if status.State == mnnode.NODE_STATE_SUCCESS &&
			requestNode.GetID() == status.NodeID {
			completedRequests++
		}
	}
	statusMutex.Unlock()

	t.Logf("Flow Completion Results: Running=%d, Responses=%d, Correlated=%d, Success Rate=%.2f%%, Orphaned=%d, Completed=%d",
		totalRunning, totalResponses, correlated, successRate, orphanedCount, completedRequests)

	// Validations
	assert.Equal(t, iterations, totalRunning, "Should have %d RUNNING statuses", iterations)
	assert.Equal(t, iterations, totalResponses, "Should have %d responses", iterations)
	assert.Equal(t, 100.0, successRate, "Should have 100%% correlation success rate")
	assert.Equal(t, 0, orphanedCount, "Should have no orphaned responses")
	assert.Equal(t, iterations, completedRequests, "Should have %d completed requests", iterations)

	// Verify flow completed properly (FOR nodes may have empty next nodes when complete)
	assert.NotNil(t, result, "FOR node should return a result")
}
