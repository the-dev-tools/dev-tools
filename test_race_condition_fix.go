package main

import (
	"context"
	"fmt"
	"sync"
	"time"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/node/nfor"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/flow/runner/flowlocalrunner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
)

// TestRequestNode simulates a REQUEST node with execution tracking
type TestRequestNode struct {
	id           idwrap.IDWrap
	name         string
	executionLog []ExecutionRecord
	mu           sync.Mutex
}

type ExecutionRecord struct {
	ExecutionID      string
	IterationPath    []int
	ExecutionIndex   int
	ProcessingTime   time.Duration
	RaceDetected     bool
	TrackerPresent   bool
}

func NewTestRequestNode(id idwrap.IDWrap, name string) *TestRequestNode {
	return &TestRequestNode{
		id:           id,
		name:         name,
		executionLog: []ExecutionRecord{},
	}
}

func (m *TestRequestNode) GetID() idwrap.IDWrap {
	return m.id
}

func (m *TestRequestNode) GetName() string {
	return m.name
}

func (m *TestRequestNode) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	startTime := time.Now()
	
	// Capture ExecutionID at the start
	capturedExecutionID := req.ExecutionID
	trackerPresent := req.VariableTracker != nil
	
	// Simulate some processing time to allow race conditions to manifest
	time.Sleep(10 * time.Millisecond)
	
	// Check if ExecutionID changed during processing (race condition indicator)
	currentExecutionID := req.ExecutionID
	raceDetected := capturedExecutionID != currentExecutionID
	
	processingTime := time.Since(startTime)
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	record := ExecutionRecord{
		ExecutionID:    capturedExecutionID.String()[:8],
		ProcessingTime: processingTime,
		RaceDetected:   raceDetected,
		TrackerPresent: trackerPresent,
	}
	
	if req.IterationContext != nil {
		record.IterationPath = req.IterationContext.IterationPath
		record.ExecutionIndex = req.IterationContext.ExecutionIndex
	}
	
	m.executionLog = append(m.executionLog, record)
	
	return node.FlowNodeResult{}
}

func (m *TestRequestNode) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	result := m.RunSync(ctx, req)
	resultChan <- result
}

func (m *TestRequestNode) GetExecutionLog() []ExecutionRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	log := make([]ExecutionRecord, len(m.executionLog))
	copy(log, m.executionLog)
	return log
}

func TestRaceConditionFix() {
	fmt.Println("=== TESTING RACE CONDITION FIX ===")
	
	// Test different scenarios
	scenarios := []struct {
		name       string
		iterations int
		concurrent bool
	}{
		{"Sequential 5 iterations", 5, false},
		{"Concurrent 10 iterations", 10, true},
		{"Concurrent 20 iterations", 20, true},
	}
	
	for _, scenario := range scenarios {
		fmt.Printf("\n--- %s ---\n", scenario.name)
		testScenario(scenario.iterations, scenario.concurrent)
	}
}

func testScenario(iterations int, useConcurrent bool) {
	// Create start node
	startNodeID := idwrap.NewNow()
	startNode := &TestRequestNode{id: startNodeID, name: "Start"}
	
	// Create FOR node
	forNodeID := idwrap.NewNow()
	forNode := nfor.New(forNodeID, "TestLoop", int64(iterations), 5*time.Second, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)
	
	// Create REQUEST node
	requestNodeID := idwrap.NewNow()
	requestNode := NewTestRequestNode(requestNodeID, "TestRequest")
	
	// Set up node map
	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		startNodeID:   startNode,
		forNodeID:     forNode,
		requestNodeID: requestNode,
	}
	
	// Connect: Start -> FOR -> REQUEST
	edges := []edge.Edge{
		edge.NewEdge(idwrap.NewNow(), startNodeID, forNodeID, edge.HandleThen, edge.EdgeKindNoOp),
		edge.NewEdge(idwrap.NewNow(), forNodeID, requestNodeID, edge.HandleLoop, edge.EdgeKindNoOp),
	}
	edgeMap := edge.NewEdgesMap(edges)
	
	// Create flow runner
	runnerID := idwrap.NewNow()
	flowID := idwrap.NewNow()
	timeout := time.Duration(0)
	if useConcurrent {
		timeout = 5 * time.Second // Use async with timeout
	}
	flowRunner := flowlocalrunner.CreateFlowRunner(runnerID, flowID, startNodeID, nodeMap, edgeMap, timeout)
	
	// Track all statuses
	var allStatuses []runner.FlowNodeStatus
	var statusMutex sync.Mutex
	
	flowNodeStatusChan := make(chan runner.FlowNodeStatus, 1000)
	flowStatusChan := make(chan runner.FlowStatus, 10)
	
	// Status collector
	go func() {
		for status := range flowNodeStatusChan {
			statusMutex.Lock()
			allStatuses = append(allStatuses, status)
			statusMutex.Unlock()
		}
	}()
	
	// Run the flow
	startTime := time.Now()
	ctx := context.Background()
	err := flowRunner.Run(ctx, flowNodeStatusChan, flowStatusChan, nil)
	executionTime := time.Since(startTime)
	
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}
	
	// Wait for all status processing
	time.Sleep(100 * time.Millisecond)
	
	// Analysis
	statusMutex.Lock()
	requestStatuses := 0
	runningStatuses := 0
	for _, status := range allStatuses {
		if status.NodeID == requestNodeID {
			requestStatuses++
			if status.State == 1 { // RUNNING
				runningStatuses++
			}
		}
	}
	statusMutex.Unlock()
	
	executionLog := requestNode.GetExecutionLog()
	
	// Check for race conditions
	raceCount := 0
	missingTrackers := 0
	uniqueExecutionIDs := make(map[string]bool)
	
	for _, record := range executionLog {
		if record.RaceDetected {
			raceCount++
		}
		if !record.TrackerPresent {
			missingTrackers++
		}
		uniqueExecutionIDs[record.ExecutionID] = true
	}
	
	// Results
	fmt.Printf("Execution time: %v\n", executionTime)
	fmt.Printf("Expected executions: %d\n", iterations)
	fmt.Printf("Actual executions: %d\n", len(executionLog))
	fmt.Printf("Status records: %d (RUNNING: %d)\n", requestStatuses, runningStatuses)
	fmt.Printf("Unique ExecutionIDs: %d\n", len(uniqueExecutionIDs))
	fmt.Printf("Race conditions detected: %d\n", raceCount)
	fmt.Printf("Missing trackers: %d\n", missingTrackers)
	
	// Success criteria
	success := true
	if len(executionLog) != iterations {
		fmt.Printf("❌ FAIL: Expected %d executions, got %d\n", iterations, len(executionLog))
		success = false
	}
	if raceCount > 0 {
		fmt.Printf("❌ FAIL: %d race conditions detected\n", raceCount)
		success = false
	}
	if missingTrackers > 0 {
		fmt.Printf("❌ FAIL: %d executions missing variable trackers\n", missingTrackers)
		success = false
	}
	if len(uniqueExecutionIDs) != len(executionLog) {
		fmt.Printf("❌ FAIL: ExecutionIDs are not unique\n")
		success = false
	}
	
	if success {
		fmt.Printf("✅ PASS: All executions tracked correctly, no race conditions\n")
	}
	
	// Show detailed execution log for failures or small tests
	if !success || iterations <= 10 {
		fmt.Println("\nDetailed execution log:")
		for i, record := range executionLog {
			fmt.Printf("  %d: ID=%s Path=%v ExecIdx=%d Race=%v Tracker=%v Duration=%v\n", 
				i+1, record.ExecutionID, record.IterationPath, record.ExecutionIndex, 
				record.RaceDetected, record.TrackerPresent, record.ProcessingTime)
		}
	}
}

func main() {
	TestRaceConditionFix()
}