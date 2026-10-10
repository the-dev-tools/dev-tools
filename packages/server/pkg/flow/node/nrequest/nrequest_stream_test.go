package nrequest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/http/request"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/httpclient"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
)

// A streamed step exposes response.text, response.events, response.event_count and
// response.ttft_ms to its assertions and to later steps.
func TestStreamedResponseInAssertionsAndOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, c := range []string{"Hel", "lo"} {
			_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"content":"` + c + `"}}]}` + "\n\n"))
			w.(http.Flusher).Flush()
		}
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	t.Cleanup(srv.Close)

	nodeID, httpID := idwrap.NewNow(), idwrap.NewNow()
	asserts := []mhttp.HTTPAssert{}
	for _, a := range []string{`response.text == "Hello"`, "response.event_count == 3", "response.ttft_ms >= 0",
		`response.events[0].data.choices[0].delta.content == "Hel"`} {
		asserts = append(asserts, mhttp.HTTPAssert{ID: idwrap.NewNow(), HttpID: httpID, Enabled: true, Value: a})
	}
	respChan := make(chan NodeRequestSideResp, 1)
	startResponseConsumer(respChan)
	requestNode := New(nodeID, "Ask", mhttp.HTTP{ID: httpID, Name: "Ask", Url: srv.URL, Method: "POST", BodyKind: mhttp.HttpBodyKindRaw},
		nil, nil, nil, nil, nil, asserts, http.DefaultClient, respChan, nil)
	requestNode.Stream = &httpclient.StreamOptions{Preset: httpclient.StreamPresetOpenAI}
	req := &node.FlowNodeRequest{
		VarMap:        map[string]any{},
		ReadWriteLock: &sync.RWMutex{},
		NodeMap:       map[idwrap.IDWrap]node.FlowNode{nodeID: requestNode},
		EdgeSourceMap: mflow.EdgesMap{},
		ExecutionID:   idwrap.NewNow(),
	}
	require.NoError(t, requestNode.RunSync(context.Background(), req).Err)

	out, ok := req.VarMap["Ask"].(map[string]any)
	require.True(t, ok)
	resp, ok := out["response"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "Hello", resp["text"])
	require.Equal(t, 3, resp["event_count"])
	require.Contains(t, resp, "ttft_ms")
	require.Len(t, resp["events"], 3)
}

// Lean (load) mode keeps the streamed text and counts, not the events.
func TestLeanModeDropsStreamEvents(t *testing.T) {
	ttft := 5 * time.Millisecond
	v := buildResponseVar(request.RequestResponse{HttpResp: httpclient.Response{StatusCode: 200, Stream: &httpclient.StreamResult{
		Text: "Hello", EventCount: 2, Events: []httpclient.StreamEvent{{Data: "Hel"}, {Data: "lo"}}, TTFT: &ttft}}}, true)
	m := v.StreamFields()
	require.Equal(t, "Hello", m["text"])
	require.Equal(t, 2, m["event_count"])
	require.Empty(t, m["events"])
}
