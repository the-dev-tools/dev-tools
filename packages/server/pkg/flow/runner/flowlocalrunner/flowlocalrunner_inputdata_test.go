package flowlocalrunner

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/node/nrequest"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mexampleresp"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mnnode"
)

type inputTrackingNode struct {
	id   idwrap.IDWrap
	name string
}
type staticHTTPClient struct{}

func (n *inputTrackingNode) GetID() idwrap.IDWrap { return n.id }

func (n *inputTrackingNode) GetName() string { return n.name }

func (n *inputTrackingNode) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	if req.VariableTracker != nil {
		req.VariableTracker.TrackRead("baseUrl", "https://api.example.com")
		req.VariableTracker.TrackRead("foreach_4.item.id", "cat-42")
	}
	return node.FlowNodeResult{}
}

func (n *inputTrackingNode) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	resultChan <- n.RunSync(ctx, req)
}

type singleEdgeStartNode struct {
	id   idwrap.IDWrap
	next idwrap.IDWrap
}

func (n *singleEdgeStartNode) GetID() idwrap.IDWrap { return n.id }

func (n *singleEdgeStartNode) GetName() string { return "Start" }

func (n *singleEdgeStartNode) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	return node.FlowNodeResult{NextNodeID: []idwrap.IDWrap{n.next}}
}

func (n *singleEdgeStartNode) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	resultChan <- n.RunSync(ctx, req)
}

func (staticHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"status":"ok"}`)),
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
	}, nil
}

func TestFlowLocalRunnerEmitsInputDataForTrackedReads(t *testing.T) {
	t.Parallel()
	startID := idwrap.NewNow()
	targetID := idwrap.NewNow()

	startNode := &singleEdgeStartNode{id: startID, next: targetID}
	trackingNode := &inputTrackingNode{id: targetID, name: "request"}

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		startID:  startNode,
		targetID: trackingNode,
	}

	edgeID := idwrap.NewNow()
	edges := []edge.Edge{
		edge.NewEdge(edgeID, startID, targetID, edge.HandleUnspecified, int32(edge.EdgeKindNoOp)),
	}
	edgesMap := edge.NewEdgesMap(edges)

	runnerID := idwrap.NewNow()
	flowID := idwrap.NewNow()
	fr := CreateFlowRunner(runnerID, flowID, startID, nodeMap, edgesMap, 0, nil)

	nodeStates := make(chan runner.FlowNodeStatus, 10)
	flowStatus := make(chan runner.FlowStatus, 2)

	baseVars := map[string]any{
		"baseUrl": "https://api.example.com",
	}

	var (
		mu          sync.Mutex
		successSeen bool
		inputData   map[string]any
	)

	statesDone := make(chan struct{})
	go func() {
		defer close(statesDone)
		for status := range nodeStates {
			if status.NodeID == targetID && status.State == mnnode.NODE_STATE_SUCCESS {
				mu.Lock()
				successSeen = true
				if data, ok := status.InputData.(map[string]any); ok {
					inputData = data
				}
				mu.Unlock()
			}
		}
	}()

	statusDone := make(chan struct{})
	go func() {
		for range flowStatus {
		}
		close(statusDone)
	}()

	err := fr.RunWithEvents(
		context.Background(),
		runner.FlowEventChannels{
			NodeStates: nodeStates,
			FlowStatus: flowStatus,
		},
		baseVars,
	)
	if err != nil {
		t.Fatalf("runner returned error: %v", err)
	}

	select {
	case <-statesDone:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for node states to drain")
	}

	select {
	case <-statusDone:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for flow status to drain")
	}

	if !successSeen {
		t.Fatalf("did not observe success status for tracking node")
	}
	if len(inputData) == 0 {
		t.Fatalf("expected input data to be captured, got %+v", inputData)
	}
	if inputData["baseUrl"] != "https://api.example.com" {
		t.Fatalf("expected baseUrl to be tracked, got %+v", inputData["baseUrl"])
	}
	// Ensure nested key is represented (tree builder will expand to nested maps)
	foreachVal, ok := inputData["foreach_4"].(map[string]any)
	if !ok {
		t.Fatalf("expected foreach_4 subtree in input data, got %+v", inputData)
	}
	itemVal, ok := foreachVal["item"].(map[string]any)
	if !ok || itemVal["id"] != "cat-42" {
		t.Fatalf("expected foreach_4.item.id to be tracked, got %+v", foreachVal)
	}
}

func TestFlowLocalRunnerRequestNodeEmitsInputData(t *testing.T) {
	t.Parallel()

	startID := idwrap.NewNow()
	requestNodeID := idwrap.NewNow()

	startNode := &singleEdgeStartNode{id: startID, next: requestNodeID}

	endpoint := mitemapi.ItemApi{
		ID:     idwrap.NewNow(),
		Name:   "request",
		Method: "GET",
		Url:    "{{ baseUrl }}/api/categories/{{ foreach_4.item.id }}",
	}
	example := mitemapiexample.ItemApiExample{
		ID:        idwrap.NewNow(),
		ItemApiID: endpoint.ID,
		Name:      "example",
		BodyType:  mitemapiexample.BodyTypeRaw,
	}
	rawBody := mbodyraw.ExampleBodyRaw{
		ID:        idwrap.NewNow(),
		ExampleID: example.ID,
		Data:      []byte(`{"payload":"{{ foreach_4.item.id }}"}`),
	}
	exampleResp := mexampleresp.ExampleResp{
		ID:        idwrap.NewNow(),
		ExampleID: example.ID,
		Status:    200,
		Body:      []byte(`{"ok":true}`),
	}

	requestNode := nrequest.New(
		requestNodeID,
		"request",
		endpoint,
		example,
		nil,
		nil,
		rawBody,
		nil,
		nil,
		exampleResp,
		nil,
		nil,
		staticHTTPClient{},
		make(chan nrequest.NodeRequestSideResp, 1),
		nil,
	)

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		startID:       startNode,
		requestNodeID: requestNode,
	}

	edges := []edge.Edge{
		edge.NewEdge(idwrap.NewNow(), startID, requestNodeID, edge.HandleUnspecified, int32(edge.EdgeKindNoOp)),
	}
	edgesMap := edge.NewEdgesMap(edges)

	runnerID := idwrap.NewNow()
	flowID := idwrap.NewNow()
	fr := CreateFlowRunner(runnerID, flowID, startID, nodeMap, edgesMap, 0, nil)

	nodeStates := make(chan runner.FlowNodeStatus, 10)
	flowStatus := make(chan runner.FlowStatus, 2)

	baseVars := map[string]any{
		"baseUrl": "https://api.example.com",
		"foreach_4": map[string]any{
			"item": map[string]any{
				"id": "cat-42",
			},
		},
	}

	var (
		mu          sync.Mutex
		successSeen bool
		inputData   map[string]any
	)

	statesDone := make(chan struct{})
	go func() {
		defer close(statesDone)
		for status := range nodeStates {
			if status.NodeID == requestNodeID && status.State == mnnode.NODE_STATE_SUCCESS {
				mu.Lock()
				successSeen = true
				if data, ok := status.InputData.(map[string]any); ok {
					inputData = data
				}
				mu.Unlock()
			}
		}
	}()

	statusDone := make(chan struct{})
	go func() {
		for range flowStatus {
		}
		close(statusDone)
	}()

	err := fr.RunWithEvents(
		context.Background(),
		runner.FlowEventChannels{
			NodeStates: nodeStates,
			FlowStatus: flowStatus,
		},
		baseVars,
	)
	if err != nil {
		t.Fatalf("runner returned error: %v", err)
	}

	select {
	case <-statesDone:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for node states to drain")
	}

	select {
	case <-statusDone:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for flow status to drain")
	}

	if !successSeen {
		t.Fatalf("did not observe success status for request node")
	}
	if len(inputData) == 0 {
		t.Fatalf("expected input data for request node, got %+v", inputData)
	}
	if inputData["baseUrl"] != "https://api.example.com" {
		t.Fatalf("expected baseUrl to be tracked, got %+v", inputData["baseUrl"])
	}
	foreachVal, ok := inputData["foreach_4"].(map[string]any)
	if !ok {
		t.Fatalf("expected foreach_4 subtree, got %+v", inputData)
	}
	itemVal, ok := foreachVal["item"].(map[string]any)
	if !ok || itemVal["id"] != "cat-42" {
		t.Fatalf("expected foreach_4.item.id to be tracked, got %+v", foreachVal)
	}
}
