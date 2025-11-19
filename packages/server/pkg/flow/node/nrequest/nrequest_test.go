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
	"the-dev-tools/server/pkg/model/mhttp"
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
	node    *NodeRequest
	flowReq *node.FlowNodeRequest
	httpID  idwrap.IDWrap
}

func newRequestNodeFixture(asserts []mhttp.HTTPAssert, respChan chan NodeRequestSideResp) requestNodeFixture {
	nodeID := idwrap.NewNow()
	httpID := idwrap.NewNow()

	httpReq := mhttp.HTTP{
		ID:       httpID,
		Name:     "req",
		Url:      "https://example.dev",
		Method:   "GET",
		BodyKind: mhttp.HttpBodyKindRaw,
	}

	rawBody := &mhttp.HTTPBodyRaw{
		ID:      idwrap.NewNow(),
		HttpID:  httpID,
		RawData: []byte("{}"),
	}

	requestNode := New(
		nodeID,
		"req",
		httpReq,
		nil, // headers
		nil, // params
		rawBody,
		nil, // formBody
		nil, // urlBody
		asserts,
		stubHTTPClient{},
		respChan,
		nil, // logger
	)

	flowReq := &node.FlowNodeRequest{
		VarMap:        map[string]any{},
		ReadWriteLock: &sync.RWMutex{},
		NodeMap:       map[idwrap.IDWrap]node.FlowNode{nodeID: requestNode},
		EdgeSourceMap: edge.EdgesMap{},
		ExecutionID:   idwrap.NewNow(),
	}

	return requestNodeFixture{
		node:    requestNode,
		flowReq: flowReq,
		httpID:  httpID,
	}
}

func TestNodeRequestRunSyncTracksVariableReads(t *testing.T) {
	nodeID := idwrap.NewNow()
	httpID := idwrap.NewNow()

	httpReq := mhttp.HTTP{
		ID:       httpID,
		Name:     "req",
		Method:   "POST",
		Url:      "{{baseUrl}}/users",
		BodyKind: mhttp.HttpBodyKindRaw,
	}

	rawBody := &mhttp.HTTPBodyRaw{
		ID:      idwrap.NewNow(),
		HttpID:  httpID,
		RawData: []byte(`{"name": "{{name}}"}`),
	}

	queries := []mhttp.HTTPSearchParam{
		{ParamKey: "limit", ParamValue: "{{limit}}", Enabled: true},
	}
	headers := []mhttp.HTTPHeader{
		{HeaderKey: "Authorization", HeaderValue: "Bearer {{token}}", Enabled: true},
	}

	respChan := make(chan NodeRequestSideResp, 1)

	requestNode := New(
		nodeID,
		"req",
		httpReq,
		headers,
		queries,
		rawBody,
		nil,
		nil,
		nil, // asserts
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

	select {
	case <-respChan:
	default:
		t.Fatalf("expected response channel to receive payload")
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
	httpID := idwrap.NewNow()

	httpReq := mhttp.HTTP{
		ID:       httpID,
		Name:     "req",
		Url:      "https://example.dev",
		Method:   "GET",
		BodyKind: mhttp.HttpBodyKindRaw,
	}
	rawBody := &mhttp.HTTPBodyRaw{
		ID:      idwrap.NewNow(),
		HttpID:  httpID,
		RawData: []byte("{}"),
	}

	asserts := []mhttp.HTTPAssert{
		{
			ID:          idwrap.NewNow(),
			HttpID:      httpID,
			Enabled:     true,
			AssertValue: "response.status == 205",
		},
	}

	respChan := make(chan NodeRequestSideResp, 1)
	requestNode := New(
		nodeID,
		"req",
		httpReq,
		nil,
		nil,
		rawBody,
		nil,
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
	httpID := idwrap.NewNow()
	httpReq := mhttp.HTTP{
		ID:       httpID,
		Name:     "req",
		Url:      "https://example.dev",
		Method:   "GET",
		BodyKind: mhttp.HttpBodyKindRaw,
	}
	rawBody := &mhttp.HTTPBodyRaw{
		ID:      idwrap.NewNow(),
		HttpID:  httpID,
		RawData: []byte("{}"),
	}
	asserts := []mhttp.HTTPAssert{
		{
			ID:          idwrap.NewNow(),
			HttpID:      httpID,
			Enabled:     true,
			AssertValue: "response.status == 205",
		},
	}

	respChan := make(chan NodeRequestSideResp, 1)
	requestNode := New(
		nodeID,
		"req",
		httpReq,
		nil,
		nil,
		rawBody,
		nil,
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
	httpID := idwrap.NewNow()
	httpReq := mhttp.HTTP{
		ID:       httpID,
		Name:     "req",
		Url:      "https://example.dev",
		Method:   "GET",
		BodyKind: mhttp.HttpBodyKindRaw,
	}
	rawBody := &mhttp.HTTPBodyRaw{
		ID:      idwrap.NewNow(),
		HttpID:  httpID,
		RawData: []byte("{}"),
	}
	asserts := []mhttp.HTTPAssert{
		{
			ID:          idwrap.NewNow(),
			HttpID:      httpID,
			Enabled:     true,
			AssertValue: "response.status == 205",
		},
	}

	respChan := make(chan NodeRequestSideResp, 1)
	requestNode := New(
		nodeID,
		"req",
		httpReq,
		nil,
		nil,
		rawBody,
		nil,
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
		if resp.Resp.HTTPResponse.ID == (idwrap.IDWrap{}) {
			t.Fatalf("expected response ID to be set: %#v", resp.Resp.HTTPResponse)
		}
		if resp.Resp.HTTPResponse.HttpID != httpID {
			t.Fatalf("expected response to target http %s, got %s", httpID, resp.Resp.HTTPResponse.HttpID)
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
		if resp.Resp.HTTPResponse.ID == (idwrap.IDWrap{}) {
			t.Fatalf("expected response id to be set on success: %#v", resp.Resp.HTTPResponse)
		}
		if resp.Resp.HTTPResponse.HttpID != fixture.httpID {
			t.Fatalf("expected response http id %s, got %s", fixture.httpID, resp.Resp.HTTPResponse.HttpID)
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
		if resp.Resp.HTTPResponse.ID == (idwrap.IDWrap{}) {
			t.Fatalf("expected response id to be set on async success: %#v", resp.Resp.HTTPResponse)
		}
		if resp.Resp.HTTPResponse.HttpID != fixture.httpID {
			t.Fatalf("expected response http id %s, got %s", fixture.httpID, resp.Resp.HTTPResponse.HttpID)
		}
	default:
		t.Fatalf("expected async response side channel to receive entry")
	}
}
