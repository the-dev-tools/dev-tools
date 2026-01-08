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
	"time"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/tracking"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/http/request"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/httpclient"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"

	"github.com/stretchr/testify/require"
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
	require.NoError(t, err, "legacy builder returned error")

	got := buildNodeRequestOutputMap(output)

	if !reflect.DeepEqual(got, expected) {
		t.Errorf("map mismatch\nexpected: %#v\n     got: %#v", expected, got)
		t.FailNow()
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
		EdgeSourceMap: mflow.EdgesMap{},
		ExecutionID:   idwrap.NewNow(),
	}

	return requestNodeFixture{
		node:    requestNode,
		flowReq: flowReq,
		httpID:  httpID,
	}
}

// helper to start a consumer that closes Done channel and forwards the response for assertion
func startResponseConsumer(respChan <-chan NodeRequestSideResp) <-chan NodeRequestSideResp {
	out := make(chan NodeRequestSideResp, 1)
	go func() {
		select {
		case resp := <-respChan:
			if resp.Done != nil {
				close(resp.Done)
			}
			out <- resp
		case <-time.After(5 * time.Second):
			// prevent leaking goroutine if test times out/fails before sending
			close(out)
		}
	}()
	return out
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
		{Key: "limit", Value: "{{limit}}", Enabled: true},
	}
	headers := []mhttp.HTTPHeader{
		{Key: "Authorization", Value: "Bearer {{token}}", Enabled: true},
	}

	respChan := make(chan NodeRequestSideResp, 1)
	consumedChan := startResponseConsumer(respChan)

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
		EdgeSourceMap:   mflow.EdgesMap{},
		ExecutionID:     idwrap.NewNow(),
		VariableTracker: tracker,
	}

	result := requestNode.RunSync(context.Background(), req)
	require.NoError(t, result.Err, "expected success, got error")

	select {
	case <-consumedChan:
	default:
		t.Error("expected response channel to receive payload")
		t.FailNow()
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
			t.Errorf("expected tracker to capture %s, got %#v", key, readVars)
			t.FailNow()
		}
		strValue, ok := value.(string)
		if !ok {
			t.Errorf("expected %s to be a string, got %T", key, value)
			t.FailNow()
		}
		if strValue != expected {
			t.Errorf("expected tracker to capture %s=%s, got %s", key, expected, strValue)
			t.FailNow()
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
			ID:      idwrap.NewNow(),
			HttpID:  httpID,
			Enabled: true,
			Value:   "response.status == 205",
		},
	}

	respChan := make(chan NodeRequestSideResp, 1)
	// Ensure consumer is running before RunSync
	startResponseConsumer(respChan)

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
		EdgeSourceMap: mflow.EdgesMap{},
		ExecutionID:   idwrap.NewNow(),
	}

	result := requestNode.RunSync(context.Background(), req)
	if result.Err == nil {
		t.Error("expected assertion failure, got nil error")
		t.FailNow()
	}
	if !strings.Contains(result.Err.Error(), "assertion failed") {
		t.Errorf("expected assertion failure message, got %v", result.Err)
		t.FailNow()
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
			ID:      idwrap.NewNow(),
			HttpID:  httpID,
			Enabled: true,
			Value:   "response.status == 205",
		},
	}

	respChan := make(chan NodeRequestSideResp, 1)
	consumedChan := startResponseConsumer(respChan)

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
		EdgeSourceMap:   mflow.EdgesMap{},
		ExecutionID:     idwrap.NewNow(),
		VariableTracker: tracker,
	}

	result := requestNode.RunSync(context.Background(), req)
	if result.Err == nil {
		t.Error("expected assertion failure, got nil error")
		t.FailNow()
	}

	select {
	case <-consumedChan:
	default:
		t.Error("expected response side channel to receive entry")
		t.FailNow()
	}

	writen := tracker.GetWrittenVarsAsTree()
	reqData, ok := writen["req"]
	if !ok {
		t.Errorf("expected tracker to record req writes, got %+v", writen)
		t.FailNow()
	}
	reqMap, ok := reqData.(map[string]any)
	if !ok {
		t.Errorf("req entry is not a map: %#v", reqData)
		t.FailNow()
	}
	respSection, ok := reqMap["response"].(map[string]any)
	if !ok {
		t.Errorf("missing response payload: %#v", reqMap)
		t.FailNow()
	}
	if respSection["status"] == nil {
		t.Errorf("response status not tracked: %#v", respSection)
		t.FailNow()
	}
	requestSection, ok := reqMap["request"].(map[string]any)
	if !ok {
		t.Errorf("missing request payload: %#v", reqMap)
		t.FailNow()
	}
	if requestSection["url"] == nil {
		t.Errorf("request url not tracked: %#v", requestSection)
		t.FailNow()
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
			ID:      idwrap.NewNow(),
			HttpID:  httpID,
			Enabled: true,
			Value:   "response.status == 205",
		},
	}

	respChan := make(chan NodeRequestSideResp, 1)
	consumedChan := startResponseConsumer(respChan)

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
		EdgeSourceMap: mflow.EdgesMap{},
		ExecutionID:   idwrap.NewNow(),
	}

	result := requestNode.RunSync(context.Background(), req)
	if result.Err == nil {
		t.Error("expected assertion failure, got nil error")
		t.FailNow()
	}

	select {
	case resp := <-consumedChan:
		if resp.Resp.HTTPResponse.ID == (idwrap.IDWrap{}) {
			t.Errorf("expected response ID to be set: %#v", resp.Resp.HTTPResponse)
			t.FailNow()
		}
		if resp.Resp.HTTPResponse.HttpID != httpID {
			t.Errorf("expected response to target http %s, got %s", httpID, resp.Resp.HTTPResponse.HttpID)
			t.FailNow()
		}
	default:
		t.Error("expected response side channel to receive entry")
		t.FailNow()
	}
}

func TestNodeRequestRunSyncSuccessSendsResponseID(t *testing.T) {
	respChan := make(chan NodeRequestSideResp, 1)
	consumedChan := startResponseConsumer(respChan)
	fixture := newRequestNodeFixture(nil, respChan)

	result := fixture.node.RunSync(context.Background(), fixture.flowReq)
	require.NoError(t, result.Err, "expected success, got error")

	select {
	case resp := <-consumedChan:
		if resp.Resp.HTTPResponse.ID == (idwrap.IDWrap{}) {
			t.Errorf("expected response id to be set on success: %#v", resp.Resp.HTTPResponse)
			t.FailNow()
		}
		if resp.Resp.HTTPResponse.HttpID != fixture.httpID {
			t.Errorf("expected response http id %s, got %s", fixture.httpID, resp.Resp.HTTPResponse.HttpID)
			t.FailNow()
		}
	default:
		t.Error("expected response side channel to receive entry")
		t.FailNow()
	}
}

func TestNodeRequestRunAsyncSuccessSendsResponseID(t *testing.T) {
	respChan := make(chan NodeRequestSideResp, 1)
	consumedChan := startResponseConsumer(respChan)
	resultChan := make(chan node.FlowNodeResult, 1)
	fixture := newRequestNodeFixture(nil, respChan)

	fixture.node.RunAsync(context.Background(), fixture.flowReq, resultChan)

	select {
	case result := <-resultChan:
		if result.Err != nil {
			t.Errorf("expected async success, got error: %v", result.Err)
			t.FailNow()
		}
	default:
		t.Error("expected async result to be delivered")
		t.FailNow()
	}

	select {
	case resp := <-consumedChan:
		if resp.Resp.HTTPResponse.ID == (idwrap.IDWrap{}) {
			t.Errorf("expected response id to be set on async success: %#v", resp.Resp.HTTPResponse)
			t.FailNow()
		}
		if resp.Resp.HTTPResponse.HttpID != fixture.httpID {
			t.Errorf("expected response http id %s, got %s", fixture.httpID, resp.Resp.HTTPResponse.HttpID)
			t.FailNow()
		}
	default:
		t.Error("expected async response side channel to receive entry")
		t.FailNow()
	}
}
