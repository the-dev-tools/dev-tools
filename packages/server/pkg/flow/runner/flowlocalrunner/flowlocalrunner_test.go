package flowlocalrunner_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
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
	flowRunner := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), nodeID, nodeMap, edgesMap, 0)

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

	flowRunner := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), loopID, nodeMap, edgesMap, 0)

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
	flowRunner := flowlocalrunner.CreateFlowRunner(runnerID, idwrap.NewNow(), startID, nodeMap, edgesMap, 0)
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

func TestFlowLocalRunnerAutoModeSelection(t *testing.T) {
	linearStart, linearNodeMap, linearEdges, _ := buildLinearStubFlow(3, false)
	linearRunner := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), linearStart, linearNodeMap, linearEdges, 0)

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
	branchRunner := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), branchStart, branchNodes, branchEdges, 0)

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
}

func TestFlowLocalRunnerModeOverride(t *testing.T) {
	startID, nodeMap, edgesMap, _ := buildLinearStubFlow(2, false)
	flowRunner := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), startID, nodeMap, edgesMap, 0)
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

func runExecutionModeBenchmark(b *testing.B, startID idwrap.IDWrap, nodeMap map[idwrap.IDWrap]node.FlowNode, edgesMap edge.EdgesMap, mode flowlocalrunner.ExecutionMode) {
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		flowRunner := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), startID, nodeMap, edgesMap, 0)
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
