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
