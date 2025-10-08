package nrequest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"

	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/tracking"
	"the-dev-tools/server/pkg/http/request"
	"the-dev-tools/server/pkg/httpclient"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mexamplequery"
	"the-dev-tools/server/pkg/model/mexampleresp"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
)

func legacyBuildNodeRequestOutputMap(output NodeRequestOutput) (map[string]any, error) {
	marshaledResp, err := json.Marshal(output)
	if err != nil {
		return nil, err
	}

	respMap := make(map[string]any)
	if err := json.Unmarshal(marshaledResp, &respMap); err != nil {
		return nil, err
	}
	return respMap, nil
}

func sampleOutput() NodeRequestOutput {
	reqVar := request.RequestResponseVar{
		Method:  "GET",
		URL:     "https://example.test/resource",
		Headers: map[string]string{"Authorization": "Bearer token", "X-Test": "true"},
		Queries: map[string]string{"q": "value", "limit": "10"},
		Body:    "{}",
	}

	respVar := httpclient.ResponseVar{
		StatusCode: 200,
		Body: map[string]any{
			"message": "ok",
			"count":   float64(2),
		},
		Headers:  map[string]string{"Content-Type": "application/json"},
		Duration: 123,
	}

	return NodeRequestOutput{Request: reqVar, Response: respVar}
}

func TestBuildNodeRequestOutputMapMatchesLegacy(t *testing.T) {
	output := sampleOutput()

	expected, err := legacyBuildNodeRequestOutputMap(output)
	if err != nil {
		t.Fatalf("legacy builder returned error: %v", err)
	}

	got := buildNodeRequestOutputMap(output)

	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("map mismatch\nexpected: %#v\n     got: %#v", expected, got)
	}
}

func BenchmarkLegacyBuildNodeRequestOutputMap(b *testing.B) {
	output := sampleOutput()
	for i := 0; i < b.N; i++ {
		if _, err := legacyBuildNodeRequestOutputMap(output); err != nil {
			b.Fatalf("legacy builder error: %v", err)
		}
	}
}

func BenchmarkNewBuildNodeRequestOutputMap(b *testing.B) {
	output := sampleOutput()
	for i := 0; i < b.N; i++ {
		if result := buildNodeRequestOutputMap(output); len(result) == 0 {
			b.Fatalf("builder returned empty map")
		}
	}
}

type stubHTTPClient struct{}

func (stubHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader("{}")),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}, nil
}

type requestNodeFixture struct {
	node      *NodeRequest
	flowReq   *node.FlowNodeRequest
	exampleID idwrap.IDWrap
}

func newRequestNodeFixture(asserts []massert.Assert, respChan chan NodeRequestSideResp) requestNodeFixture {
	nodeID := idwrap.NewNow()
	exampleID := idwrap.NewNow()
	endpoint := mitemapi.ItemApi{ID: idwrap.NewNow(), Name: "req", Url: "https://example.dev", Method: "GET"}
	example := mitemapiexample.ItemApiExample{ID: exampleID, ItemApiID: endpoint.ID, Name: "req"}
	rawBody := mbodyraw.ExampleBodyRaw{ID: idwrap.NewNow(), ExampleID: example.ID, Data: []byte("{}")}
	exampleResp := mexampleresp.ExampleResp{ExampleID: example.ID}

	requestNode := New(
		nodeID,
		"req",
		endpoint,
		example,
		nil,
		nil,
		rawBody,
		nil,
		nil,
		exampleResp,
		nil,
		asserts,
		stubHTTPClient{},
		respChan,
		nil,
	)

	flowReq := &node.FlowNodeRequest{
		VarMap:        map[string]any{},
		ReadWriteLock: &sync.RWMutex{},
		NodeMap:       map[idwrap.IDWrap]node.FlowNode{nodeID: requestNode},
		EdgeSourceMap: edge.EdgesMap{},
		ExecutionID:   idwrap.NewNow(),
	}

	return requestNodeFixture{
		node:      requestNode,
		flowReq:   flowReq,
		exampleID: exampleID,
	}
}

func TestNodeRequestRunSyncTracksVariableReads(t *testing.T) {
	nodeID := idwrap.NewNow()
	exampleID := idwrap.NewNow()

	endpoint := mitemapi.ItemApi{
		ID:     idwrap.NewNow(),
		Name:   "req",
		Method: "POST",
		Url:    "{{baseUrl}}/users",
	}
	example := mitemapiexample.ItemApiExample{
		ID:        exampleID,
		ItemApiID: endpoint.ID,
		Name:      "req",
		BodyType:  mitemapiexample.BodyTypeRaw,
	}
	rawBody := mbodyraw.ExampleBodyRaw{
		ID:        idwrap.NewNow(),
		ExampleID: example.ID,
		Data:      []byte(`{"name": "{{name}}"}`),
	}

	queries := []mexamplequery.Query{
		{QueryKey: "limit", Value: "{{limit}}", Enable: true},
	}
	headers := []mexampleheader.Header{
		{HeaderKey: "Authorization", Value: "Bearer {{token}}", Enable: true},
	}

	respChan := make(chan NodeRequestSideResp, 1)

	requestNode := New(
		nodeID,
		"req",
		endpoint,
		example,
		queries,
		headers,
		rawBody,
		nil,
		nil,
		mexampleresp.ExampleResp{ExampleID: example.ID},
		nil,
		nil,
		stubHTTPClient{},
		respChan,
		nil,
	)

	tracker := tracking.NewVariableTracker()
	req := &node.FlowNodeRequest{
		VarMap: map[string]any{
			"baseUrl": "https://api.example.com",
			"limit":   "10",
			"token":   "abc123",
			"name":    "Ada Lovelace",
		},
		ReadWriteLock:   &sync.RWMutex{},
		NodeMap:         map[idwrap.IDWrap]node.FlowNode{nodeID: requestNode},
		EdgeSourceMap:   edge.EdgesMap{},
		ExecutionID:     idwrap.NewNow(),
		VariableTracker: tracker,
	}

	result := requestNode.RunSync(context.Background(), req)
	if result.Err != nil {
		t.Fatalf("expected success, got error: %v", result.Err)
	}

	var sideResp NodeRequestSideResp
	select {
	case sideResp = <-respChan:
	default:
		t.Fatalf("expected response channel to receive payload")
	}
	if len(sideResp.InputData) == 0 {
		t.Fatalf("expected InputData to be captured, got %+v", sideResp.InputData)
	}
	if val, ok := sideResp.InputData["baseUrl"].(string); !ok || val != "https://api.example.com" {
		t.Fatalf("expected baseUrl to be tracked in side response, got %+v", sideResp.InputData)
	}

	readVars := tracker.GetReadVars()
	expectedValues := map[string]string{
		"baseUrl": "https://api.example.com",
		"limit":   "10",
		"token":   "abc123",
		"name":    "Ada Lovelace",
	}
	for key, expected := range expectedValues {
		value, ok := readVars[key]
		if !ok {
			t.Fatalf("expected tracker to capture %s, got %#v", key, readVars)
		}
		strValue, ok := value.(string)
		if !ok {
			t.Fatalf("expected %s to be a string, got %T", key, value)
		}
		if strValue != expected {
			t.Fatalf("expected tracker to capture %s=%s, got %s", key, expected, strValue)
		}
	}
}

func TestNodeRequestRunSyncFailsOnAssertion(t *testing.T) {
	nodeID := idwrap.NewNow()
	exampleID := idwrap.NewNow()
	endpoint := mitemapi.ItemApi{ID: idwrap.NewNow(), Name: "req", Url: "https://example.dev", Method: "GET"}
	example := mitemapiexample.ItemApiExample{ID: exampleID, ItemApiID: endpoint.ID, Name: "req"}
	rawBody := mbodyraw.ExampleBodyRaw{ID: idwrap.NewNow(), ExampleID: example.ID, Data: []byte("{}")}
	exampleResp := mexampleresp.ExampleResp{ID: idwrap.NewNow(), ExampleID: example.ID}
	asserts := []massert.Assert{
		{
			ID:        idwrap.NewNow(),
			ExampleID: example.ID,
			Enable:    true,
			Condition: mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "response.status == 205"}},
		},
	}

	respChan := make(chan NodeRequestSideResp, 1)
	requestNode := New(
		nodeID,
		"req",
		endpoint,
		example,
		nil,
		nil,
		rawBody,
		nil,
		nil,
		exampleResp,
		nil,
		asserts,
		stubHTTPClient{},
		respChan,
		nil,
	)

	req := &node.FlowNodeRequest{
		VarMap:        map[string]any{},
		ReadWriteLock: &sync.RWMutex{},
		NodeMap:       map[idwrap.IDWrap]node.FlowNode{nodeID: requestNode},
		EdgeSourceMap: edge.EdgesMap{},
		ExecutionID:   idwrap.NewNow(),
	}

	result := requestNode.RunSync(context.Background(), req)
	if result.Err == nil {
		t.Fatalf("expected assertion failure, got nil error")
	}
	if !strings.Contains(result.Err.Error(), "assertion failed") {
		t.Fatalf("expected assertion failure message, got %v", result.Err)
	}
}

func TestNodeRequestRunSyncTracksOutputOnAssertionFailure(t *testing.T) {
	nodeID := idwrap.NewNow()
	exampleID := idwrap.NewNow()
	endpoint := mitemapi.ItemApi{ID: idwrap.NewNow(), Name: "req", Url: "https://example.dev", Method: "GET"}
	example := mitemapiexample.ItemApiExample{ID: exampleID, ItemApiID: endpoint.ID, Name: "req"}
	rawBody := mbodyraw.ExampleBodyRaw{ID: idwrap.NewNow(), ExampleID: example.ID, Data: []byte("{}")}
	exampleResp := mexampleresp.ExampleResp{ID: idwrap.NewNow(), ExampleID: example.ID}
	asserts := []massert.Assert{
		{
			ID:        idwrap.NewNow(),
			ExampleID: example.ID,
			Enable:    true,
			Condition: mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "response.status == 205"}},
		},
	}

	respChan := make(chan NodeRequestSideResp, 1)
	requestNode := New(
		nodeID,
		"req",
		endpoint,
		example,
		nil,
		nil,
		rawBody,
		nil,
		nil,
		exampleResp,
		nil,
		asserts,
		stubHTTPClient{},
		respChan,
		nil,
	)

	tracker := tracking.NewVariableTracker()
	req := &node.FlowNodeRequest{
		VarMap:          map[string]any{},
		ReadWriteLock:   &sync.RWMutex{},
		NodeMap:         map[idwrap.IDWrap]node.FlowNode{nodeID: requestNode},
		EdgeSourceMap:   edge.EdgesMap{},
		ExecutionID:     idwrap.NewNow(),
		VariableTracker: tracker,
	}

	result := requestNode.RunSync(context.Background(), req)
	if result.Err == nil {
		t.Fatalf("expected assertion failure, got nil error")
	}

	select {
	case <-respChan:
	default:
		t.Fatalf("expected response side channel to receive entry")
	}

	written := tracker.GetWrittenVarsAsTree()
	reqData, ok := written["req"]
	if !ok {
		t.Fatalf("expected tracker to record req writes, got %+v", written)
	}
	reqMap, ok := reqData.(map[string]any)
	if !ok {
		t.Fatalf("req entry is not a map: %#v", reqData)
	}
	respSection, ok := reqMap["response"].(map[string]any)
	if !ok {
		t.Fatalf("missing response payload: %#v", reqMap)
	}
	if respSection["status"] == nil {
		t.Fatalf("response status not tracked: %#v", respSection)
	}
	requestSection, ok := reqMap["request"].(map[string]any)
	if !ok {
		t.Fatalf("missing request payload: %#v", reqMap)
	}
	if requestSection["url"] == nil {
		t.Fatalf("request url not tracked: %#v", requestSection)
	}
}

func TestNodeRequestRunSyncAssertionFailureSendsResponseID(t *testing.T) {
	nodeID := idwrap.NewNow()
	exampleID := idwrap.NewNow()
	endpoint := mitemapi.ItemApi{ID: idwrap.NewNow(), Name: "req", Url: "https://example.dev", Method: "GET"}
	example := mitemapiexample.ItemApiExample{ID: exampleID, ItemApiID: endpoint.ID, Name: "req"}
	rawBody := mbodyraw.ExampleBodyRaw{ID: idwrap.NewNow(), ExampleID: example.ID, Data: []byte("{}")}
	exampleResp := mexampleresp.ExampleResp{ExampleID: example.ID}
	asserts := []massert.Assert{
		{
			ID:        idwrap.NewNow(),
			ExampleID: example.ID,
			Enable:    true,
			Condition: mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "response.status == 205"}},
		},
	}

	respChan := make(chan NodeRequestSideResp, 1)
	requestNode := New(
		nodeID,
		"req",
		endpoint,
		example,
		nil,
		nil,
		rawBody,
		nil,
		nil,
		exampleResp,
		nil,
		asserts,
		stubHTTPClient{},
		respChan,
		nil,
	)

	req := &node.FlowNodeRequest{
		VarMap:        map[string]any{},
		ReadWriteLock: &sync.RWMutex{},
		NodeMap:       map[idwrap.IDWrap]node.FlowNode{nodeID: requestNode},
		EdgeSourceMap: edge.EdgesMap{},
		ExecutionID:   idwrap.NewNow(),
	}

	result := requestNode.RunSync(context.Background(), req)
	if result.Err == nil {
		t.Fatalf("expected assertion failure, got nil error")
	}

	select {
	case resp := <-respChan:
		if resp.Resp.ExampleResp.ID == (idwrap.IDWrap{}) {
			t.Fatalf("expected response ID to be set: %#v", resp.Resp.ExampleResp)
		}
		if resp.Resp.ExampleResp.ExampleID != example.ID {
			t.Fatalf("expected response to target example %s, got %s", example.ID, resp.Resp.ExampleResp.ExampleID)
		}
	default:
		t.Fatalf("expected response side channel to receive entry")
	}
}

func TestNodeRequestRunSyncSuccessSendsResponseID(t *testing.T) {
	respChan := make(chan NodeRequestSideResp, 1)
	fixture := newRequestNodeFixture(nil, respChan)

	result := fixture.node.RunSync(context.Background(), fixture.flowReq)
	if result.Err != nil {
		t.Fatalf("expected success, got error: %v", result.Err)
	}

	select {
	case resp := <-respChan:
		if resp.Resp.ExampleResp.ID == (idwrap.IDWrap{}) {
			t.Fatalf("expected response id to be set on success: %#v", resp.Resp.ExampleResp)
		}
		if resp.Resp.ExampleResp.ExampleID != fixture.exampleID {
			t.Fatalf("expected response example id %s, got %s", fixture.exampleID, resp.Resp.ExampleResp.ExampleID)
		}
	default:
		t.Fatalf("expected response side channel to receive entry")
	}
}

func TestNodeRequestRunAsyncSuccessSendsResponseID(t *testing.T) {
	respChan := make(chan NodeRequestSideResp, 1)
	resultChan := make(chan node.FlowNodeResult, 1)
	fixture := newRequestNodeFixture(nil, respChan)

	fixture.node.RunAsync(context.Background(), fixture.flowReq, resultChan)

	select {
	case result := <-resultChan:
		if result.Err != nil {
			t.Fatalf("expected async success, got error: %v", result.Err)
		}
	default:
		t.Fatalf("expected async result to be delivered")
	}

	select {
	case resp := <-respChan:
		if resp.Resp.ExampleResp.ID == (idwrap.IDWrap{}) {
			t.Fatalf("expected response id to be set on async success: %#v", resp.Resp.ExampleResp)
		}
		if resp.Resp.ExampleResp.ExampleID != fixture.exampleID {
			t.Fatalf("expected response example id %s, got %s", fixture.exampleID, resp.Resp.ExampleResp.ExampleID)
		}
	default:
		t.Fatalf("expected async response side channel to receive entry")
	}
}
