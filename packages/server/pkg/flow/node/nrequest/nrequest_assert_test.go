package nrequest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"

	"github.com/stretchr/testify/require"
)

// runAssertAgainst sends one GET to url through a request node carrying a single
// assertion and returns the node's error.
func runAssertAgainst(t *testing.T, url, assertion string) error {
	t.Helper()
	nodeID := idwrap.NewNow()
	httpID := idwrap.NewNow()
	httpReq := mhttp.HTTP{ID: httpID, Name: "req", Url: url, Method: "GET", BodyKind: mhttp.HttpBodyKindRaw}
	asserts := []mhttp.HTTPAssert{{ID: idwrap.NewNow(), HttpID: httpID, Enabled: true, Value: assertion}}

	respChan := make(chan NodeRequestSideResp, 1)
	startResponseConsumer(respChan)

	requestNode := New(nodeID, "req", httpReq, nil, nil, nil, nil, nil, asserts, http.DefaultClient, respChan, nil)
	req := &node.FlowNodeRequest{
		VarMap:        map[string]any{},
		ReadWriteLock: &sync.RWMutex{},
		NodeMap:       map[idwrap.IDWrap]node.FlowNode{nodeID: requestNode},
		EdgeSourceMap: mflow.EdgesMap{},
		ExecutionID:   idwrap.NewNow(),
	}
	return requestNode.RunSync(context.Background(), req).Err
}

func serverAnswering(t *testing.T, delay time.Duration, status int, contentType, body string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(delay)
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestResponseDurationAssertionFailsOnSlowServer(t *testing.T) {
	url := serverAnswering(t, 150*time.Millisecond, http.StatusOK, "application/json", `{}`)
	err := runAssertAgainst(t, url, "response.duration < 50")
	require.Error(t, err, "a 150 ms response must fail response.duration < 50")
	t.Log(err)
	require.Contains(t, err.Error(), "response.duration = ")
}

func TestResponseDurationAssertionPassesOnFastServer(t *testing.T) {
	url := serverAnswering(t, 0, http.StatusOK, "application/json", `{}`)
	require.NoError(t, runAssertAgainst(t, url, "response.duration < 5000"))
	require.NoError(t, runAssertAgainst(t, url, "response.duration >= 0"))
}

func TestAssertionFailureShowsCheckedValues(t *testing.T) {
	url := serverAnswering(t, 0, http.StatusOK, "application/json", `{"name":"Outdoor","count":3}`)
	err := runAssertAgainst(t, url, `response.body.name == "Indoor" && response.body.count > 1`)
	require.Error(t, err)
	msg := err.Error()
	t.Log(msg)
	require.Contains(t, msg, `response.body.name = "Outdoor"`)
	require.Contains(t, msg, `response.body.count = 3`)
	require.NotContains(t, msg, "(got 200)")
}

func TestAssertionTypeErrorExplainsNonJSONResponse(t *testing.T) {
	url := serverAnswering(t, 0, http.StatusNotFound, "text/html; charset=utf-8", `<html><body>Not Found</body></html>`)
	err := runAssertAgainst(t, url, "response.body.count > 0")
	require.Error(t, err)
	msg := err.Error()
	t.Log(msg)
	require.Contains(t, msg, "404")
	require.Contains(t, msg, "text/html")
	require.Contains(t, msg, "not JSON")
}
