package flowlocalrunner_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/node/mocknode"
	"the-dev-tools/server/pkg/flow/node/nfor"
	"the-dev-tools/server/pkg/flow/node/nnoop"
	"the-dev-tools/server/pkg/flow/runner"
	flowlocalrunner "the-dev-tools/server/pkg/flow/runner/flowlocalrunner"
	"the-dev-tools/server/pkg/httpclient"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
)

func legacyGetPredecessorNodes(nodeID idwrap.IDWrap, edgesMap edge.EdgesMap) []idwrap.IDWrap {
	var predecessors []idwrap.IDWrap
	seen := make(map[idwrap.IDWrap]bool)

	for sourceID, edges := range edgesMap {
		for _, targetNodes := range edges {
			for _, targetID := range targetNodes {
				if targetID == nodeID && !seen[sourceID] {
					predecessors = append(predecessors, sourceID)
					seen[sourceID] = true
				}
			}
		}
	}

	return predecessors
}

func buildDenseEdges(nodeCount int, fanout int) edge.EdgesMap {
	nodes := make([]idwrap.IDWrap, nodeCount)
	for i := 0; i < nodeCount; i++ {
		nodes[i] = idwrap.NewNow()
	}

	var edges []edge.Edge
	for i := 0; i < nodeCount; i++ {
		for j := 1; j <= fanout; j++ {
			targetIndex := (i + j) % nodeCount
			edges = append(edges, edge.NewEdge(idwrap.NewNow(), nodes[i], nodes[targetIndex], edge.HandleUnspecified, int32(edge.EdgeKindNoOp)))
		}
	}

	return edge.NewEdgesMap(edges)
}

func BenchmarkLegacyPredecessorLookup(b *testing.B) {
	edgesMap := buildDenseEdges(100, 4)
	var targets []idwrap.IDWrap
	for id := range edgesMap {
		targets = append(targets, id)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, target := range targets {
			_ = legacyGetPredecessorNodes(target, edgesMap)
		}
	}
}

func BenchmarkCachedPredecessorLookup(b *testing.B) {
	edgesMap := buildDenseEdges(100, 4)
	predecessors := flowlocalrunner.BuildPredecessorMap(edgesMap)
	var targets []idwrap.IDWrap
	for id := range edgesMap {
		targets = append(targets, id)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, target := range targets {
			_ = predecessors[target]
		}
	}
}

func BenchmarkBuildPredecessorMap(b *testing.B) {
	edgesMap := buildDenseEdges(100, 4)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = flowlocalrunner.BuildPredecessorMap(edgesMap)
	}
}

type stubNode struct {
	id      idwrap.IDWrap
	name    string
	next    []idwrap.IDWrap
	callLog *([]string)
}

func (s *stubNode) GetID() idwrap.IDWrap { return s.id }

func (s *stubNode) GetName() string { return s.name }

func (s *stubNode) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	if s.callLog != nil {
		log := append(*s.callLog, s.name)
		*s.callLog = log
	}
	return node.FlowNodeResult{NextNodeID: append([]idwrap.IDWrap(nil), s.next...)}
}

func (s *stubNode) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	resultChan <- s.RunSync(ctx, req)
}

type failingNode struct {
	id     idwrap.IDWrap
	name   string
	output map[string]any
	err    error
}

func newFailingNode(id idwrap.IDWrap, name string, output map[string]any, err error) *failingNode {
	return &failingNode{id: id, name: name, output: output, err: err}
}

func (f *failingNode) GetID() idwrap.IDWrap { return f.id }

func (f *failingNode) GetName() string { return f.name }

func (f *failingNode) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	if f.output != nil {
		var err error
		if req.VariableTracker != nil {
			err = node.WriteNodeVarBulkWithTracking(req, f.name, f.output, req.VariableTracker)
		} else {
			err = node.WriteNodeVarBulk(req, f.name, f.output)
		}
		if err != nil {
			return node.FlowNodeResult{Err: err}
		}
	}
	return node.FlowNodeResult{Err: f.err}
}

func (f *failingNode) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	resultChan <- f.RunSync(ctx, req)
}

type blockingNode struct {
	id      idwrap.IDWrap
	name    string
	release <-chan struct{}
	started chan struct{}
	once    sync.Once
}

func newBlockingNode(name string, release <-chan struct{}) *blockingNode {
	return &blockingNode{
		id:      idwrap.NewNow(),
		name:    name,
		release: release,
		started: make(chan struct{}),
	}
}

func (b *blockingNode) markStarted() {
	b.once.Do(func() {
		close(b.started)
	})
}

func (b *blockingNode) GetID() idwrap.IDWrap { return b.id }

func (b *blockingNode) GetName() string { return b.name }

func (b *blockingNode) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	b.markStarted()
	if b.release != nil {
		select {
		case <-b.release:
		case <-ctx.Done():
			return node.FlowNodeResult{Err: ctx.Err()}
		}
	}
	return node.FlowNodeResult{}
}

func (b *blockingNode) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	resultChan <- b.RunSync(ctx, req)
}

func waitForStart(tb testing.TB, ch <-chan struct{}, name string) {
	tb.Helper()
	select {
	case <-ch:
	case <-time.After(time.Second):
		tb.Fatalf("timed out waiting for %s to start", name)
	}
}

func buildLinearStubFlow(count int, captureOrder bool) (idwrap.IDWrap, map[idwrap.IDWrap]node.FlowNode, edge.EdgesMap, *[]string) {
	ids := make([]idwrap.IDWrap, count)
	for i := range ids {
		ids[i] = idwrap.NewNow()
	}

	var callLog *[]string
	if captureOrder {
		log := make([]string, 0, count)
		callLog = &log
	}

	nodeMap := make(map[idwrap.IDWrap]node.FlowNode, count)
	edgesMap := make(edge.EdgesMap, count)

	for i := 0; i < count; i++ {
		name := fmt.Sprintf("node-%d", i)
		var next []idwrap.IDWrap
		if i+1 < count {
			next = []idwrap.IDWrap{ids[i+1]}
		}
		nodeMap[ids[i]] = &stubNode{id: ids[i], name: name, next: next, callLog: callLog}
		edgesMap[ids[i]] = map[edge.EdgeHandle][]idwrap.IDWrap{
			edge.HandleUnspecified: next,
		}
	}

	return ids[0], nodeMap, edgesMap, callLog
}

func drainStates(ch <-chan runner.FlowNodeStatus) []runner.FlowNodeStatus {
	var statuses []runner.FlowNodeStatus
	for status := range ch {
		statuses = append(statuses, status)
	}
	return statuses
}

func drainLogs(ch <-chan runner.FlowNodeLogPayload) []runner.FlowNodeLogPayload {
	var logs []runner.FlowNodeLogPayload
	for entry := range ch {
		logs = append(logs, entry)
	}
	return logs
}

func drainFlowStatus(ch <-chan runner.FlowStatus) []runner.FlowStatus {
	var statuses []runner.FlowStatus
	for status := range ch {
		statuses = append(statuses, status)
	}
	return statuses
}

func TestFlowLocalRunnerEmitsLogEvents(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	startID := idwrap.NewNow()
	stub := &stubNode{id: startID, name: "start"}
	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		startID: stub,
	}
	edgesMap := edge.EdgesMap{
		startID: {
			edge.HandleUnspecified: nil,
		},
	}

	flowRunner := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), startID, nodeMap, edgesMap, 0, nil)

	stateChan := make(chan runner.FlowNodeStatus, 8)
	logChan := make(chan runner.FlowNodeLogPayload, 8)
	flowStatusChan := make(chan runner.FlowStatus, 8)

	err := flowRunner.RunWithEvents(ctx, runner.FlowEventChannels{
		NodeStates: stateChan,
		NodeLogs:   logChan,
		FlowStatus: flowStatusChan,
	}, nil)
	if err != nil {
		t.Fatalf("RunWithEvents returned error: %v", err)
	}

	states := drainStates(stateChan)
	if len(states) == 0 {
		t.Fatalf("expected node states, got none")
	}

	logs := drainLogs(logChan)
	if len(logs) == 0 {
		t.Fatalf("expected log payloads, got none")
	}
	for _, entry := range logs {
		if entry.State == mnnode.NODE_STATE_RUNNING {
			t.Fatalf("unexpected running state in log payloads: %+v", entry)
		}
	}

	flowStatuses := drainFlowStatus(flowStatusChan)
	if len(flowStatuses) == 0 {
		t.Fatalf("expected flow statuses, got none")
	}
	if flowStatuses[0] != runner.FlowStatusStarting {
		t.Fatalf("expected first flow status to be Starting, got %v", flowStatuses[0])
	}
	if flowStatuses[len(flowStatuses)-1] != runner.FlowStatusSuccess {
		t.Fatalf("expected final flow status Success, got %v", flowStatuses[len(flowStatuses)-1])
	}
}

func TestFlowLocalRunnerMultiFailureIncludesOutputData(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	startID := idwrap.NewNow()
	failID := idwrap.NewNow()
	failErr := errors.New("boom")
	outputSnapshot := map[string]any{
		"request":  map[string]any{"method": "POST", "url": "https://example.test"},
		"response": map[string]any{"status": float64(500)},
	}

	start := &stubNode{id: startID, name: "root", next: []idwrap.IDWrap{failID}}
	failure := newFailingNode(failID, "request_node", outputSnapshot, failErr)

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		startID: start,
		failID:  failure,
	}

	edgesMap := edge.EdgesMap{
		startID: {
			edge.HandleUnspecified: []idwrap.IDWrap{failID},
		},
		failID: {
			edge.HandleUnspecified: nil,
		},
	}

	flowRunner := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), startID, nodeMap, edgesMap, 0, nil)
	flowRunner.SetExecutionMode(flowlocalrunner.ExecutionModeMulti)

	stateChan := make(chan runner.FlowNodeStatus, 8)
	logChan := make(chan runner.FlowNodeLogPayload, 8)

	err := flowRunner.RunWithEvents(ctx, runner.FlowEventChannels{
		NodeStates: stateChan,
		NodeLogs:   logChan,
	}, map[string]any{})
	if !errors.Is(err, failErr) {
		t.Fatalf("expected error %v, got %v", failErr, err)
	}

	logs := drainLogs(logChan)
	var failureLog runner.FlowNodeLogPayload
	for _, entry := range logs {
		if entry.NodeID == failID && entry.State == mnnode.NODE_STATE_FAILURE {
			failureLog = entry
			break
		}
	}
	if failureLog.NodeID == (idwrap.IDWrap{}) {
		t.Fatalf("did not observe failure log for node %s", failID)
	}

	outputData, ok := failureLog.OutputData.(map[string]any)
	if !ok {
		t.Fatalf("expected map output data, got %T", failureLog.OutputData)
	}
	if _, ok := outputData[failure.GetName()]; ok {
		t.Fatalf("unexpected node-scoped key in output data: %#v", outputData)
	}
	if _, ok := outputData["request"]; !ok {
		t.Fatalf("expected request payload in output data: %#v", outputData)
	}
	if _, ok := outputData["response"]; !ok {
		t.Fatalf("expected response payload in output data: %#v", outputData)
	}
}

func buildBranchingStubFlow() (idwrap.IDWrap, map[idwrap.IDWrap]node.FlowNode, edge.EdgesMap) {
	startID := idwrap.NewNow()
	leftID := idwrap.NewNow()
	rightID := idwrap.NewNow()
	joinID := idwrap.NewNow()

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		startID: &stubNode{id: startID, name: "start", next: []idwrap.IDWrap{leftID, rightID}},
		leftID:  &stubNode{id: leftID, name: "left", next: []idwrap.IDWrap{joinID}},
		rightID: &stubNode{id: rightID, name: "right", next: []idwrap.IDWrap{joinID}},
		joinID:  &stubNode{id: joinID, name: "join"},
	}

	edgesMap := edge.EdgesMap{
		startID: map[edge.EdgeHandle][]idwrap.IDWrap{
			edge.HandleUnspecified: []idwrap.IDWrap{leftID, rightID},
		},
		leftID: map[edge.EdgeHandle][]idwrap.IDWrap{
			edge.HandleUnspecified: []idwrap.IDWrap{joinID},
		},
		rightID: map[edge.EdgeHandle][]idwrap.IDWrap{
			edge.HandleUnspecified: []idwrap.IDWrap{joinID},
		},
		joinID: map[edge.EdgeHandle][]idwrap.IDWrap{
			edge.HandleUnspecified: nil,
		},
	}

	return startID, nodeMap, edgesMap
}

func buildStarStubFlow(branches int) (idwrap.IDWrap, map[idwrap.IDWrap]node.FlowNode, edge.EdgesMap) {
	startID := idwrap.NewNow()
	sinkID := idwrap.NewNow()

	nodeMap := make(map[idwrap.IDWrap]node.FlowNode, branches+2)
	edgesMap := make(edge.EdgesMap, branches+2)

	branchIDs := make([]idwrap.IDWrap, branches)
	for i := 0; i < branches; i++ {
		branchIDs[i] = idwrap.NewNow()
	}

	nodeMap[startID] = &stubNode{id: startID, name: "start", next: append([]idwrap.IDWrap(nil), branchIDs...)}
	edgesMap[startID] = map[edge.EdgeHandle][]idwrap.IDWrap{
		edge.HandleUnspecified: branchIDs,
	}

	for i, branchID := range branchIDs {
		name := fmt.Sprintf("branch-%d", i)
		nodeMap[branchID] = &stubNode{id: branchID, name: name, next: []idwrap.IDWrap{sinkID}}
		edgesMap[branchID] = map[edge.EdgeHandle][]idwrap.IDWrap{
			edge.HandleUnspecified: []idwrap.IDWrap{sinkID},
		}
	}

	nodeMap[sinkID] = &stubNode{id: sinkID, name: "sink"}
	edgesMap[sinkID] = map[edge.EdgeHandle][]idwrap.IDWrap{
		edge.HandleUnspecified: nil,
	}

	return startID, nodeMap, edgesMap
}

func buildLoopFlow(iterations int64, client httpclient.HttpClient) (idwrap.IDWrap, map[idwrap.IDWrap]node.FlowNode, edge.EdgesMap) {
	startID := idwrap.NewNow()
	loopID := idwrap.NewNow()
	requestID := idwrap.NewNow()

	startNode := nnoop.New(startID, "start")
	loopNode := nfor.New(loopID, "loop", iterations, time.Millisecond, mnfor.ErrorHandling_ERROR_HANDLING_IGNORE)
	requestNode := &benchRequestNode{id: requestID, name: "request", client: client}

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		startID:   startNode,
		loopID:    loopNode,
		requestID: requestNode,
	}

	edgesMap := edge.EdgesMap{
		startID: {
			edge.HandleUnspecified: []idwrap.IDWrap{loopID},
		},
		loopID: {
			edge.HandleLoop: []idwrap.IDWrap{requestID},
			edge.HandleThen: nil,
		},
		requestID: {
			edge.HandleUnspecified: nil,
		},
	}

	return startID, nodeMap, edgesMap
}

type mockHTTPClient struct {
	body []byte
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(m.body)),
		Header:     make(http.Header),
	}
	return resp, nil
}

type benchRequestNode struct {
	id     idwrap.IDWrap
	name   string
	client httpclient.HttpClient
}

func (n *benchRequestNode) GetID() idwrap.IDWrap { return n.id }

func (n *benchRequestNode) SetID(id idwrap.IDWrap) { n.id = id }

func (n *benchRequestNode) GetName() string { return n.name }

func (n *benchRequestNode) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://jsonplaceholder.typicode.com/photos", nil)
	if err != nil {
		return node.FlowNodeResult{Err: err}
	}

	resp, err := n.client.Do(httpReq)
	if err != nil {
		return node.FlowNodeResult{Err: err}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return node.FlowNodeResult{Err: err}
	}

	if err := node.WriteNodeVar(req, n.name, "body", body); err != nil {
		return node.FlowNodeResult{Err: err}
	}

	return node.FlowNodeResult{}
}

func (n *benchRequestNode) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	resultChan <- n.RunSync(ctx, req)
}

func TestLoopNodeEmitsFinalSuccessStatus(t *testing.T) {
	nodeID := idwrap.NewNow()
	loopNode := nfor.New(nodeID, "loop", 0, time.Millisecond, mnfor.ErrorHandling_ERROR_HANDLING_IGNORE)
	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		nodeID: loopNode,
	}
	edgesMap := make(edge.EdgesMap)
	flowRunner := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), nodeID, nodeMap, edgesMap, 0, nil)

	statusChan := make(chan runner.FlowNodeStatus, 8)
	flowStatusChan := make(chan runner.FlowStatus, 2)

	if err := flowRunner.Run(context.Background(), statusChan, flowStatusChan, nil); err != nil {
		t.Fatalf("flow runner returned error: %v", err)
	}

	var states []mnnode.NodeState
	for status := range statusChan {
		states = append(states, status.State)
	}

	if len(states) < 2 {
		t.Fatalf("expected at least 2 statuses (RUNNING and SUCCESS), got %d", len(states))
	}
	if states[0] != mnnode.NODE_STATE_RUNNING {
		t.Fatalf("expected first status to be RUNNING, got %v", states[0])
	}
	if states[len(states)-1] != mnnode.NODE_STATE_SUCCESS {
		t.Fatalf("expected final status to be SUCCESS, got %v", states[len(states)-1])
	}
}

func BenchmarkFlowRunnerForLoopWithMockRequest(b *testing.B) {
	ctx := context.Background()
	mockBody := []byte(`[{"albumId":1,"id":1,"title":"accusamus beatae","url":"https://example.com"}]`)
	client := &mockHTTPClient{body: mockBody}

	loopID := idwrap.NewNow()
	requestID := idwrap.NewNow()

	loopNode := nfor.New(loopID, "loop", 1000, time.Millisecond, mnfor.ErrorHandling_ERROR_HANDLING_IGNORE)
	requestNode := &benchRequestNode{id: requestID, name: "request", client: client}

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		loopID:    loopNode,
		requestID: requestNode,
	}

	edgesMap := edge.EdgesMap{
		loopID: {
			edge.HandleLoop: []idwrap.IDWrap{requestID},
			edge.HandleThen: nil,
		},
		requestID: {
			edge.HandleUnspecified: nil,
		},
	}

	flowRunner := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), loopID, nodeMap, edgesMap, 0, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		statusChan := make(chan runner.FlowNodeStatus, 4096)
		flowStatusChan := make(chan runner.FlowStatus, 2)

		if err := flowRunner.Run(ctx, statusChan, flowStatusChan, nil); err != nil {
			b.Fatalf("run failed: %v", err)
		}

		for range statusChan {
		}
		for range flowStatusChan {
		}
	}
}

func TestFlowLocalRunnerSingleModeSequential(t *testing.T) {
	startID, nodeMap, edgesMap, callLog := buildLinearStubFlow(3, true)
	if callLog == nil {
		t.Fatalf("expected call log to be initialized")
	}

	runnerID := idwrap.NewNow()
	flowRunner := flowlocalrunner.CreateFlowRunner(runnerID, idwrap.NewNow(), startID, nodeMap, edgesMap, 0, nil)
	flowRunner.SetExecutionMode(flowlocalrunner.ExecutionModeSingle)

	statusChan := make(chan runner.FlowNodeStatus, 16)
	flowStatusChan := make(chan runner.FlowStatus, 4)

	if err := flowRunner.Run(context.Background(), statusChan, flowStatusChan, nil); err != nil {
		t.Fatalf("flow runner returned error: %v", err)
	}

	var statuses []runner.FlowNodeStatus
	for status := range statusChan {
		statuses = append(statuses, status)
	}
	for range flowStatusChan {
	}

	if flowRunner.SelectedMode() != flowlocalrunner.ExecutionModeSingle {
		t.Fatalf("expected selected mode SINGLE, got %v", flowRunner.SelectedMode())
	}

	if len(statuses) != len(nodeMap)*2 {
		t.Fatalf("expected %d statuses, got %d", len(nodeMap)*2, len(statuses))
	}

	for i := 0; i < len(nodeMap); i++ {
		running := statuses[2*i]
		success := statuses[2*i+1]
		if running.State != mnnode.NODE_STATE_RUNNING {
			t.Fatalf("expected RUNNING state at index %d, got %v", 2*i, running.State)
		}
		if success.State != mnnode.NODE_STATE_SUCCESS {
			t.Fatalf("expected SUCCESS state at index %d, got %v", 2*i+1, success.State)
		}
		if running.NodeID != success.NodeID {
			t.Fatalf("expected matching node IDs for RUNNING/SUCCESS pair, got %v and %v", running.NodeID, success.NodeID)
		}
	}

	expectedOrder := []string{"node-0", "node-1", "node-2"}
	if len(*callLog) != len(expectedOrder) {
		t.Fatalf("expected call log length %d, got %d", len(expectedOrder), len(*callLog))
	}
	for i, name := range expectedOrder {
		if (*callLog)[i] != name {
			t.Fatalf("expected call order %v, got %v", expectedOrder, *callLog)
		}
	}
}

func TestFlowLocalRunnerMultiModeConcurrentExecution(t *testing.T) {
	release := make(chan struct{})
	leftNode := newBlockingNode("left", release)
	rightNode := newBlockingNode("right", release)
	startID := idwrap.NewNow()
	startNode := &stubNode{
		id:   startID,
		name: "start",
		next: []idwrap.IDWrap{leftNode.GetID(), rightNode.GetID()},
	}

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		startID:           startNode,
		leftNode.GetID():  leftNode,
		rightNode.GetID(): rightNode,
	}

	edgesMap := edge.EdgesMap{
		startID: {
			edge.HandleUnspecified: []idwrap.IDWrap{leftNode.GetID(), rightNode.GetID()},
		},
		leftNode.GetID():  map[edge.EdgeHandle][]idwrap.IDWrap{},
		rightNode.GetID(): map[edge.EdgeHandle][]idwrap.IDWrap{},
	}

	flowRunner := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), startID, nodeMap, edgesMap, 0, nil)
	flowRunner.SetExecutionMode(flowlocalrunner.ExecutionModeMulti)

	statusChan := make(chan runner.FlowNodeStatus, 16)
	flowStatusChan := make(chan runner.FlowStatus, 4)
	errCh := make(chan error, 1)

	go func() {
		errCh <- flowRunner.Run(context.Background(), statusChan, flowStatusChan, nil)
	}()

	waitForStart(t, leftNode.started, "left")
	waitForStart(t, rightNode.started, "right")

	close(release)

	if err := <-errCh; err != nil {
		t.Fatalf("flow runner returned error: %v", err)
	}

	runningCount := make(map[idwrap.IDWrap]int)
	successCount := make(map[idwrap.IDWrap]int)
	for status := range statusChan {
		switch status.State {
		case mnnode.NODE_STATE_RUNNING:
			runningCount[status.NodeID]++
		case mnnode.NODE_STATE_SUCCESS:
			successCount[status.NodeID]++
		}
	}
	for range flowStatusChan {
	}

	if flowRunner.SelectedMode() != flowlocalrunner.ExecutionModeMulti {
		t.Fatalf("expected selected mode MULTI, got %v", flowRunner.SelectedMode())
	}

	if runningCount[leftNode.GetID()] != 1 || runningCount[rightNode.GetID()] != 1 {
		t.Fatalf("expected each node to emit one RUNNING status, got left=%d right=%d", runningCount[leftNode.GetID()], runningCount[rightNode.GetID()])
	}

	if successCount[leftNode.GetID()] != 1 || successCount[rightNode.GetID()] != 1 {
		t.Fatalf("expected each node to emit one SUCCESS status, got left=%d right=%d", successCount[leftNode.GetID()], successCount[rightNode.GetID()])
	}
}

func TestFlowLocalRunnerAutoModeSelection(t *testing.T) {
	linearStart, linearNodeMap, linearEdges, _ := buildLinearStubFlow(3, false)
	linearRunner := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), linearStart, linearNodeMap, linearEdges, 0, nil)

	statusChan := make(chan runner.FlowNodeStatus, 8)
	flowStatusChan := make(chan runner.FlowStatus, 4)
	if err := linearRunner.Run(context.Background(), statusChan, flowStatusChan, nil); err != nil {
		t.Fatalf("linear runner failed: %v", err)
	}
	for range statusChan {
	}
	for range flowStatusChan {
	}

	if linearRunner.SelectedMode() != flowlocalrunner.ExecutionModeSingle {
		t.Fatalf("expected auto mode to select SINGLE for linear flow, got %v", linearRunner.SelectedMode())
	}

	branchStart, branchNodes, branchEdges := buildBranchingStubFlow()
	branchRunner := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), branchStart, branchNodes, branchEdges, 0, nil)

	statusChan = make(chan runner.FlowNodeStatus, 8)
	flowStatusChan = make(chan runner.FlowStatus, 4)
	if err := branchRunner.Run(context.Background(), statusChan, flowStatusChan, nil); err != nil {
		t.Fatalf("branching runner failed: %v", err)
	}
	for range statusChan {
	}
	for range flowStatusChan {
	}

	if branchRunner.SelectedMode() != flowlocalrunner.ExecutionModeMulti {
		t.Fatalf("expected auto mode to select MULTI for branching flow, got %v", branchRunner.SelectedMode())
	}

	loopID := idwrap.NewNow()
	bodyID := idwrap.NewNow()
	loopNode := nfor.New(loopID, "loop", 1, time.Millisecond, mnfor.ErrorHandling_ERROR_HANDLING_IGNORE)
	bodyNode := &stubNode{id: bodyID, name: "body"}
	loopNodes := map[idwrap.IDWrap]node.FlowNode{
		loopID: loopNode,
		bodyID: bodyNode,
	}
	loopEdges := edge.EdgesMap{
		loopID: {
			edge.HandleLoop: []idwrap.IDWrap{bodyID},
		},
		bodyID: map[edge.EdgeHandle][]idwrap.IDWrap{},
	}

	loopRunner := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), loopID, loopNodes, loopEdges, 0, nil)
	statusChan = make(chan runner.FlowNodeStatus, 8)
	flowStatusChan = make(chan runner.FlowStatus, 4)
	if err := loopRunner.Run(context.Background(), statusChan, flowStatusChan, nil); err != nil {
		t.Fatalf("loop runner failed: %v", err)
	}
	for range statusChan {
	}
	for range flowStatusChan {
	}

	if loopRunner.SelectedMode() != flowlocalrunner.ExecutionModeSingle {
		t.Fatalf("expected auto mode to select SINGLE for simple loop flow, got %v", loopRunner.SelectedMode())
	}

	loopID2 := idwrap.NewNow()
	bodyAID := idwrap.NewNow()
	bodyBID := idwrap.NewNow()
	complexLoopNode := nfor.New(loopID2, "loop", 1, time.Millisecond, mnfor.ErrorHandling_ERROR_HANDLING_IGNORE)
	bodyANode := &stubNode{id: bodyAID, name: "bodyA", next: []idwrap.IDWrap{bodyBID}}
	bodyBNode := &stubNode{id: bodyBID, name: "bodyB"}
	complexNodes := map[idwrap.IDWrap]node.FlowNode{
		loopID2: complexLoopNode,
		bodyAID: bodyANode,
		bodyBID: bodyBNode,
	}
	complexEdges := edge.EdgesMap{
		loopID2: map[edge.EdgeHandle][]idwrap.IDWrap{
			edge.HandleLoop: []idwrap.IDWrap{bodyAID},
			edge.HandleThen: []idwrap.IDWrap{bodyBID},
		},
		bodyAID: map[edge.EdgeHandle][]idwrap.IDWrap{
			edge.HandleUnspecified: []idwrap.IDWrap{bodyBID},
		},
		bodyBID: map[edge.EdgeHandle][]idwrap.IDWrap{},
	}

	complexRunner := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), loopID2, complexNodes, complexEdges, 0, nil)
	statusChan = make(chan runner.FlowNodeStatus, 8)
	flowStatusChan = make(chan runner.FlowStatus, 4)
	if err := complexRunner.Run(context.Background(), statusChan, flowStatusChan, nil); err != nil {
		t.Fatalf("complex loop runner failed: %v", err)
	}
	for range statusChan {
	}
	for range flowStatusChan {
	}

	if complexRunner.SelectedMode() != flowlocalrunner.ExecutionModeMulti {
		t.Fatalf("expected auto mode to select MULTI for complex loop flow, got %v", complexRunner.SelectedMode())
	}
}

func TestFlowLocalRunnerModeOverride(t *testing.T) {
	startID, nodeMap, edgesMap, _ := buildLinearStubFlow(2, false)
	flowRunner := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), startID, nodeMap, edgesMap, 0, nil)
	flowRunner.SetExecutionMode(flowlocalrunner.ExecutionModeMulti)

	statusChan := make(chan runner.FlowNodeStatus, 8)
	flowStatusChan := make(chan runner.FlowStatus, 4)
	if err := flowRunner.Run(context.Background(), statusChan, flowStatusChan, nil); err != nil {
		t.Fatalf("runner returned error: %v", err)
	}
	for range statusChan {
	}
	for range flowStatusChan {
	}

	if flowRunner.SelectedMode() != flowlocalrunner.ExecutionModeMulti {
		t.Fatalf("expected selected mode MULTI when override is set, got %v", flowRunner.SelectedMode())
	}
}

func TestLoopCoordinatorPerNodeTimeout(t *testing.T) {
	const iterations = 4
	const perNodeTimeout = 25 * time.Millisecond
	const slowDelay = 20 * time.Millisecond

	loopID := idwrap.NewNow()
	bodyID := idwrap.NewNow()
	bodyNode := mocknode.NewDelayedMockNode(bodyID, nil, slowDelay)
	loopNode := nfor.New(loopID, "loop", iterations, 0, mnfor.ErrorHandling_ERROR_HANDLING_IGNORE)

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		loopID: loopNode,
		bodyID: bodyNode,
	}
	edgesMap := edge.EdgesMap{
		loopID: {
			edge.HandleLoop: []idwrap.IDWrap{bodyID},
		},
		bodyID: map[edge.EdgeHandle][]idwrap.IDWrap{},
	}

	flowRunner := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), loopID, nodeMap, edgesMap, perNodeTimeout, nil)
	flowRunner.SetExecutionMode(flowlocalrunner.ExecutionModeMulti)

	stateChan := make(chan runner.FlowNodeStatus, 64)
	flowStatusChan := make(chan runner.FlowStatus, 8)

	start := time.Now()
	err := flowRunner.Run(context.Background(), stateChan, flowStatusChan, nil)
	duration := time.Since(start)

	statuses := drainStates(stateChan)
	_ = drainFlowStatus(flowStatusChan)

	if err != nil {
		t.Fatalf("flow runner returned error: %v statuses=%+v", err, statuses)
	}

	if duration < perNodeTimeout*time.Duration(iterations-1) {
		t.Fatalf("expected total duration to exceed per-node timeout, got %v", duration)
	}

	for _, st := range statuses {
		if st.NodeID == loopID && st.State == mnnode.NODE_STATE_CANCELED {
			t.Fatalf("loop node was canceled unexpectedly: %+v", st)
		}
	}
}

func benchmarkBlockingFlow(b *testing.B, mode flowlocalrunner.ExecutionMode, width int) {
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		startID := idwrap.NewNow()
		startNode := &stubNode{id: startID, name: "start"}

		branchIDs := make([]idwrap.IDWrap, width)
		nodeMap := map[idwrap.IDWrap]node.FlowNode{
			startID: startNode,
		}
		edgesMap := edge.EdgesMap{
			startID: {
				edge.HandleUnspecified: make([]idwrap.IDWrap, width),
			},
		}

		releaseChans := make([]chan struct{}, width)
		for idx := 0; idx < width; idx++ {
			releaseChans[idx] = make(chan struct{})
			n := newBlockingNode(fmt.Sprintf("worker-%d", idx), releaseChans[idx])
			branchIDs[idx] = n.GetID()
			nodeMap[n.GetID()] = n
			edgesMap[n.GetID()] = map[edge.EdgeHandle][]idwrap.IDWrap{}
		}
		edgesMap[startID][edge.HandleUnspecified] = append([]idwrap.IDWrap(nil), branchIDs...)
		startNode.next = append([]idwrap.IDWrap(nil), branchIDs...)

		flowRunner := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), startID, nodeMap, edgesMap, 0, nil)
		flowRunner.SetExecutionMode(mode)

		statusChan := make(chan runner.FlowNodeStatus, width*4)
		flowStatusChan := make(chan runner.FlowStatus, 4)
		errCh := make(chan error, 1)

		go func() {
			errCh <- flowRunner.Run(ctx, statusChan, flowStatusChan, nil)
		}()

		for idx, id := range branchIDs {
			bn := nodeMap[id].(*blockingNode)
			waitForStart(b, bn.started, bn.name)
			close(releaseChans[idx])
		}

		if err := <-errCh; err != nil {
			b.Fatalf("flow runner returned error: %v", err)
		}

		for range statusChan {
		}
		for range flowStatusChan {
		}
	}
}

func BenchmarkFlowLocalRunnerBlockingFlow(b *testing.B) {
	b.Run("single", func(b *testing.B) {
		benchmarkBlockingFlow(b, flowlocalrunner.ExecutionModeSingle, 4)
	})
	b.Run("multi", func(b *testing.B) {
		benchmarkBlockingFlow(b, flowlocalrunner.ExecutionModeMulti, 4)
	})
	b.Run("auto", func(b *testing.B) {
		benchmarkBlockingFlow(b, flowlocalrunner.ExecutionModeAuto, 4)
	})
}

func runExecutionModeBenchmark(b *testing.B, startID idwrap.IDWrap, nodeMap map[idwrap.IDWrap]node.FlowNode, edgesMap edge.EdgesMap, mode flowlocalrunner.ExecutionMode) {
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		flowRunner := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), startID, nodeMap, edgesMap, 0, nil)
		flowRunner.SetExecutionMode(mode)

		statusChan := make(chan runner.FlowNodeStatus, len(nodeMap)*4)
		flowStatusChan := make(chan runner.FlowStatus, len(nodeMap))
		if err := flowRunner.Run(ctx, statusChan, flowStatusChan, nil); err != nil {
			b.Fatalf("run failed: %v", err)
		}
		for range statusChan {
		}
		for range flowStatusChan {
		}
	}
}

func BenchmarkFlowLocalRunnerExecutionModesLinear(b *testing.B) {
	startID, nodeMap, edgesMap, _ := buildLinearStubFlow(24, false)

	b.Run("single", func(b *testing.B) {
		runExecutionModeBenchmark(b, startID, nodeMap, edgesMap, flowlocalrunner.ExecutionModeSingle)
	})

	b.Run("multi", func(b *testing.B) {
		runExecutionModeBenchmark(b, startID, nodeMap, edgesMap, flowlocalrunner.ExecutionModeMulti)
	})

	b.Run("auto", func(b *testing.B) {
		runExecutionModeBenchmark(b, startID, nodeMap, edgesMap, flowlocalrunner.ExecutionModeAuto)
	})
}

func BenchmarkFlowLocalRunnerExecutionModesBranching(b *testing.B) {
	startID, nodeMap, edgesMap := buildStarStubFlow(16)

	b.Run("single", func(b *testing.B) {
		runExecutionModeBenchmark(b, startID, nodeMap, edgesMap, flowlocalrunner.ExecutionModeSingle)
	})

	b.Run("multi", func(b *testing.B) {
		runExecutionModeBenchmark(b, startID, nodeMap, edgesMap, flowlocalrunner.ExecutionModeMulti)
	})

	b.Run("auto", func(b *testing.B) {
		runExecutionModeBenchmark(b, startID, nodeMap, edgesMap, flowlocalrunner.ExecutionModeAuto)
	})
}

func BenchmarkFlowLocalRunnerLoopFlow(b *testing.B) {
	client := &mockHTTPClient{body: []byte(`[{"ok":true}]`)}
	startID, nodeMap, edgesMap := buildLoopFlow(10, client)

	b.Run("single", func(b *testing.B) {
		runExecutionModeBenchmark(b, startID, nodeMap, edgesMap, flowlocalrunner.ExecutionModeSingle)
	})

	b.Run("multi", func(b *testing.B) {
		runExecutionModeBenchmark(b, startID, nodeMap, edgesMap, flowlocalrunner.ExecutionModeMulti)
	})

	b.Run("auto", func(b *testing.B) {
		runExecutionModeBenchmark(b, startID, nodeMap, edgesMap, flowlocalrunner.ExecutionModeAuto)
	})
}
