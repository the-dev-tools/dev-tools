package nrequest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/httpclient"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
)

func runRedirectStep(t *testing.T, url string, noFollow bool, assertions ...string) (map[string]any, error) {
	t.Helper()
	nodeID, httpID := idwrap.NewNow(), idwrap.NewNow()
	asserts := []mhttp.HTTPAssert{}
	for _, a := range assertions {
		asserts = append(asserts, mhttp.HTTPAssert{ID: idwrap.NewNow(), HttpID: httpID, Enabled: true, Value: a})
	}
	respChan := make(chan NodeRequestSideResp, 1)
	startResponseConsumer(respChan)
	requestNode := New(nodeID, "Dashboard", mhttp.HTTP{ID: httpID, Name: "Dashboard", Url: url, Method: "GET"},
		nil, nil, nil, nil, nil, asserts, httpclient.New(), respChan, nil)
	requestNode.NoFollowRedirects = noFollow
	req := &node.FlowNodeRequest{
		VarMap:        map[string]any{},
		ReadWriteLock: &sync.RWMutex{},
		NodeMap:       map[idwrap.IDWrap]node.FlowNode{nodeID: requestNode},
		EdgeSourceMap: mflow.EdgesMap{},
		ExecutionID:   idwrap.NewNow(),
	}
	err := requestNode.RunSync(context.Background(), req).Err
	out, _ := req.VarMap["Dashboard"].(map[string]any)
	resp, _ := out["response"].(map[string]any)
	return resp, err
}

func loginRedirectServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/dashboard" {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		_, _ = w.Write([]byte("login"))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// response.url is the final URL after redirects, so a flow can tell it landed on a login page.
func TestResponseURLAfterRedirects(t *testing.T) {
	srv := loginRedirectServer(t)
	resp, err := runRedirectStep(t, srv.URL+"/dashboard", false,
		`response.status == 200`, `response.url endsWith "/login"`, `response.redirects == 1`)
	require.NoError(t, err)
	require.Equal(t, srv.URL+"/login", resp["url"])
	require.InDelta(t, 1, resp["redirects"], 0)

	_, err = runRedirectStep(t, srv.URL+"/dashboard", false, `!(response.url endsWith "/login")`)
	require.Error(t, err)
}

// follow_redirects: false returns the 30x so the flow can assert it.
func TestNoFollowRedirectsReturnsThe30x(t *testing.T) {
	srv := loginRedirectServer(t)
	resp, err := runRedirectStep(t, srv.URL+"/dashboard", true,
		`response.status == 302`, `response.headers.Location == "/login"`, `response.redirects == 0`)
	require.NoError(t, err)
	require.Equal(t, srv.URL+"/dashboard", resp["url"])
}
