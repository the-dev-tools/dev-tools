package flowlocalrunner

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/nrequest"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"

	"github.com/stretchr/testify/require"
)

type inputTrackingNode struct {
	id   idwrap.IDWrap
	name string
}
type staticHTTPClient struct{}

func (s staticHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
		Header:     make(http.Header),
	}, nil
}

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
	edges := []mflow.Edge{
		mflow.NewEdge(edgeID, startID, targetID, mflow.HandleUnspecified),
	}
	edgesMap := mflow.NewEdgesMap(edges)

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
			if status.NodeID == targetID && status.State == mflow.NODE_STATE_SUCCESS {
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
	require.NoError(t, err, "runner returned error")

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

	endpoint := mhttp.HTTP{
		ID:       idwrap.NewNow(),
		Name:     "request",
		Method:   "GET",
		Url:      "{{ baseUrl }}/api/categories/{{ foreach_4.item.id }}",
		BodyKind: mhttp.HttpBodyKindRaw,
	}
	rawBody := &mhttp.HTTPBodyRaw{
		ID:      idwrap.NewNow(),
		HttpID:  endpoint.ID,
		RawData: []byte(`{"payload":"{{ foreach_4.item.id }}"}`),
	}

	respChan := make(chan nrequest.NodeRequestSideResp, 10)
	go func() {
		for resp := range respChan {
			if resp.Done != nil {
				close(resp.Done)
			}
		}
	}()
	defer close(respChan)

	requestNode := nrequest.New(
		requestNodeID,
		"request",
		endpoint,
		nil, // headers
		nil, // params
		rawBody,
		nil, // formBody
		nil, // urlBody
		nil, // asserts
		staticHTTPClient{},
		respChan,
		slog.Default(),
	)

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		startID:       startNode,
		requestNodeID: requestNode,
	}

	edges := []mflow.Edge{
		mflow.NewEdge(idwrap.NewNow(), startID, requestNodeID, mflow.HandleUnspecified),
	}
	edgesMap := mflow.NewEdgesMap(edges)

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
			if status.NodeID == requestNodeID && status.State == mflow.NODE_STATE_SUCCESS {
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
	require.NoError(t, err, "runner returned error")

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

// TestFlowLocalRunnerRequestNodeEmitsInputDataForBodyOnlyVariables verifies that
// variables used ONLY in the body (not in URL or headers) are properly tracked.
// This is a regression test for the issue where body variables were not being tracked
// while header variables were.
func TestFlowLocalRunnerRequestNodeEmitsInputDataForBodyOnlyVariables(t *testing.T) {
	t.Parallel()

	startID := idwrap.NewNow()
	requestNodeID := idwrap.NewNow()

	startNode := &singleEdgeStartNode{id: startID, next: requestNodeID}

	// URL has NO variables - only the body has variables
	endpoint := mhttp.HTTP{
		ID:       idwrap.NewNow(),
		Name:     "request",
		Method:   "POST",
		Url:      "https://api.example.com/categories", // Static URL, NO variables
		BodyKind: mhttp.HttpBodyKindRaw,
	}
	// Body has a variable referencing another request's response
	rawBody := &mhttp.HTTPBodyRaw{
		ID:      idwrap.NewNow(),
		HttpID:  endpoint.ID,
		RawData: []byte(`{"categoryId": "{{ prev_request.response.body.id }}"}`),
	}

	respChan := make(chan nrequest.NodeRequestSideResp, 10)
	go func() {
		for resp := range respChan {
			if resp.Done != nil {
				close(resp.Done)
			}
		}
	}()
	defer close(respChan)

	requestNode := nrequest.New(
		requestNodeID,
		"request",
		endpoint,
		nil, // no headers
		nil, // no params
		rawBody,
		nil, // formBody
		nil, // urlBody
		nil, // asserts
		staticHTTPClient{},
		respChan,
		slog.Default(),
	)

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		startID:       startNode,
		requestNodeID: requestNode,
	}

	edges := []mflow.Edge{
		mflow.NewEdge(idwrap.NewNow(), startID, requestNodeID, mflow.HandleUnspecified),
	}
	edgesMap := mflow.NewEdgesMap(edges)

	runnerID := idwrap.NewNow()
	flowID := idwrap.NewNow()
	fr := CreateFlowRunner(runnerID, flowID, startID, nodeMap, edgesMap, 0, nil)

	nodeStates := make(chan runner.FlowNodeStatus, 10)
	flowStatus := make(chan runner.FlowStatus, 2)

	// Variables that simulate a previous request's response
	baseVars := map[string]any{
		"prev_request": map[string]any{
			"response": map[string]any{
				"body": map[string]any{
					"id": "category-123",
				},
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
			if status.NodeID == requestNodeID && status.State == mflow.NODE_STATE_SUCCESS {
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
	require.NoError(t, err, "runner returned error")

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
		t.Fatalf("expected input data for request node (body-only variable), got empty inputData")
	}

	// The body-only variable should be tracked
	prevReqVal, ok := inputData["prev_request"].(map[string]any)
	if !ok {
		t.Fatalf("expected prev_request subtree in inputData for body-only variable, got %+v", inputData)
	}
	respVal, ok := prevReqVal["response"].(map[string]any)
	if !ok {
		t.Fatalf("expected prev_request.response subtree, got %+v", prevReqVal)
	}
	bodyVal, ok := respVal["body"].(map[string]any)
	if !ok {
		t.Fatalf("expected prev_request.response.body subtree, got %+v", respVal)
	}
	if bodyVal["id"] != "category-123" {
		t.Fatalf("expected prev_request.response.body.id to be 'category-123', got %+v", bodyVal["id"])
	}
}
