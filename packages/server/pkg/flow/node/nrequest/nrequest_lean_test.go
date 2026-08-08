package nrequest

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
)

const leanTestBody = `{"token":"s3cret","items":[1,2,3]}`

type leanStubHTTPClient struct{}

func (leanStubHTTPClient) Do(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: 201,
		Body:       io.NopCloser(strings.NewReader(leanTestBody)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}, nil
}

// newLeanFixture builds a request node whose stub response carries a real JSON
// body, wired to a flow request with the given lean setting.
func newLeanFixture(t *testing.T, lean bool, asserts []mhttp.HTTPAssert) (*NodeRequest, *node.FlowNodeRequest) {
	t.Helper()

	nodeID := idwrap.NewNow()
	httpID := idwrap.NewNow()

	respChan := make(chan NodeRequestSideResp, 1)
	startResponseConsumer(respChan)

	requestNode := New(
		nodeID,
		"req",
		mhttp.HTTP{ID: httpID, Name: "req", Url: "https://example.dev", Method: "GET", BodyKind: mhttp.HttpBodyKindRaw},
		nil, // headers
		nil, // params
		&mhttp.HTTPBodyRaw{ID: idwrap.NewNow(), HttpID: httpID, RawData: []byte("{}")},
		nil, // formBody
		nil, // urlBody
		asserts,
		leanStubHTTPClient{},
		respChan,
		nil, // logger
	)

	flowReq := &node.FlowNodeRequest{
		VarMap:        map[string]any{},
		ReadWriteLock: &sync.RWMutex{},
		NodeMap:       map[idwrap.IDWrap]node.FlowNode{nodeID: requestNode},
		EdgeSourceMap: mflow.EdgesMap{},
		ExecutionID:   idwrap.NewNow(),
		LeanMode:      lean,
	}

	return requestNode, flowReq
}

// responseOutput digs the "response" map out of the node's flow output.
func responseOutput(t *testing.T, flowReq *node.FlowNodeRequest) map[string]any {
	t.Helper()

	nodeOut, ok := flowReq.VarMap["req"].(map[string]any)
	if !ok {
		t.Fatalf("VarMap[\"req\"] = %#v, want map[string]any", flowReq.VarMap["req"])
	}
	respOut, ok := nodeOut[OUTPUT_RESPONSE_NAME].(map[string]any)
	if !ok {
		t.Fatalf("node output %q = %#v, want map[string]any", OUTPUT_RESPONSE_NAME, nodeOut[OUTPUT_RESPONSE_NAME])
	}
	return respOut
}

func assertResponseMetadataIntact(t *testing.T, respOut map[string]any) {
	t.Helper()

	if got := respOut["status"]; got != float64(201) {
		t.Errorf("response.status = %#v, want 201", got)
	}
	if _, ok := respOut["duration"].(float64); !ok {
		t.Errorf("response.duration = %#v, want a float64 to survive lean mode", respOut["duration"])
	}
	headers, ok := respOut["headers"].(map[string]any)
	if !ok {
		t.Fatalf("response.headers = %#v, want map[string]any", respOut["headers"])
	}
	if headers["Content-Type"] != "application/json" {
		t.Errorf("response.headers[Content-Type] = %#v, want application/json", headers["Content-Type"])
	}
}

func TestRunSyncLeanModeDropsResponseBody(t *testing.T) {
	requestNode, flowReq := newLeanFixture(t, true, nil)

	if result := requestNode.RunSync(context.Background(), flowReq); result.Err != nil {
		t.Fatalf("RunSync() error = %v, want nil", result.Err)
	}

	respOut := responseOutput(t, flowReq)
	if got := respOut["body"]; got != LeanBodyPlaceholder {
		t.Errorf("response.body = %#v, want %q", got, LeanBodyPlaceholder)
	}
	assertResponseMetadataIntact(t, respOut)
}

func TestRunSyncDefaultKeepsResponseBody(t *testing.T) {
	requestNode, flowReq := newLeanFixture(t, false, nil)

	if result := requestNode.RunSync(context.Background(), flowReq); result.Err != nil {
		t.Fatalf("RunSync() error = %v, want nil", result.Err)
	}

	respOut := responseOutput(t, flowReq)
	body, ok := respOut["body"].(map[string]any)
	if !ok {
		t.Fatalf("response.body = %#v, want the decoded JSON body", respOut["body"])
	}
	if body["token"] != "s3cret" {
		t.Errorf("response.body.token = %#v, want s3cret", body["token"])
	}
	assertResponseMetadataIntact(t, respOut)
}

func TestRunAsyncLeanModeDropsResponseBody(t *testing.T) {
	for _, tc := range []struct {
		name     string
		lean     bool
		wantBody func(*testing.T, any)
	}{
		{
			name: "lean drops body",
			lean: true,
			wantBody: func(t *testing.T, body any) {
				t.Helper()
				if body != LeanBodyPlaceholder {
					t.Errorf("response.body = %#v, want %q", body, LeanBodyPlaceholder)
				}
			},
		},
		{
			name: "default keeps body",
			lean: false,
			wantBody: func(t *testing.T, body any) {
				t.Helper()
				decoded, ok := body.(map[string]any)
				if !ok {
					t.Fatalf("response.body = %#v, want the decoded JSON body", body)
				}
				if decoded["token"] != "s3cret" {
					t.Errorf("response.body.token = %#v, want s3cret", decoded["token"])
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requestNode, flowReq := newLeanFixture(t, tc.lean, nil)

			resultChan := make(chan node.FlowNodeResult, 1)
			go requestNode.RunAsync(context.Background(), flowReq, resultChan)

			result := <-resultChan
			if result.Err != nil {
				t.Fatalf("RunAsync() error = %v, want nil", result.Err)
			}

			respOut := responseOutput(t, flowReq)
			tc.wantBody(t, respOut["body"])
			assertResponseMetadataIntact(t, respOut)
		})
	}
}

// Assertions read the response directly rather than the flow output, so lean
// mode must not change whether they pass.
func TestLeanModeKeepsBodyAssertionsWorking(t *testing.T) {
	asserts := []mhttp.HTTPAssert{
		{ID: idwrap.NewNow(), Enabled: true, Value: `response.body.token == "s3cret"`},
		{ID: idwrap.NewNow(), Enabled: true, Value: "response.status == 201"},
	}

	for _, lean := range []bool{false, true} {
		requestNode, flowReq := newLeanFixture(t, lean, asserts)

		if result := requestNode.RunSync(context.Background(), flowReq); result.Err != nil {
			t.Errorf("lean=%t: RunSync() error = %v, want nil (assertions must be unaffected)", lean, result.Err)
		}
	}
}

// buildNodeRequestOutputMap keeps its historical body-retaining behaviour so
// existing callers and tests are unaffected.
func TestBuildNodeRequestOutputMapDefaultsToFullBody(t *testing.T) {
	full := buildNodeRequestOutputMap(sampleOutput())
	respOut, ok := full[OUTPUT_RESPONSE_NAME].(map[string]any)
	if !ok {
		t.Fatalf("response output = %#v, want map[string]any", full[OUTPUT_RESPONSE_NAME])
	}
	if _, isPlaceholder := respOut["body"].(string); isPlaceholder {
		t.Errorf("response.body = %#v, want the full body by default", respOut["body"])
	}
}
