package nrequest_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/node/nrequest"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/model/mbodyform"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mbodyurl"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mexamplequery"
	"the-dev-tools/server/pkg/model/mexampleresp"
	"the-dev-tools/server/pkg/model/mexamplerespheader"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/testutil"
	"time"
)

func TestNodeRequest_Timeout(t *testing.T) {
	t.Parallel() // Run this test in parallel with others

	// Create a test server that delays response - reduced from 5s to 500ms
	delayDuration := 500 * time.Millisecond
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(delayDuration)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Delayed response"))
	}))
	defer server.Close()

	id := idwrap.NewNow()
	next := idwrap.NewNow()

	api := mitemapi.ItemApi{
		Method: "GET",
		Url:    server.URL,
	}

	example := mitemapiexample.ItemApiExample{
		ID:   idwrap.NewNow(),
		Name: "timeout-test",
	}

	queries := []mexamplequery.Query{}
	headers := []mexampleheader.Header{}

	rawBody := mbodyraw.ExampleBodyRaw{}
	formBody := []mbodyform.BodyForm{}
	urlBody := []mbodyurl.BodyURLEncoded{}

	exampleResp := mexampleresp.ExampleResp{}
	exampleRespHeader := []mexamplerespheader.ExampleRespHeader{}
	asserts := []massert.Assert{}

	t.Run("Short timeout should fail", func(t *testing.T) {
		// Create HTTP client
		httpClient := &http.Client{
			Timeout: 30 * time.Second, // Client timeout longer than context timeout
		}

		requestNodeRespChan := make(chan nrequest.NodeRequestSideResp, 1)
		requestNode := nrequest.New(id, "timeout-test", api, example, queries, headers, rawBody, formBody, urlBody,
			exampleResp, exampleRespHeader, asserts,
			httpClient, requestNodeRespChan)

		edge1 := edge.NewEdge(idwrap.NewNow(), id, next, edge.HandleUnspecified, edge.EdgeKindUnspecified)
		edges := []edge.Edge{edge1}
		edgesMap := edge.NewEdgesMap(edges)

		var RWLock sync.RWMutex
		req := &node.FlowNodeRequest{
			VarMap:        map[string]interface{}{},
			ReadWriteLock: &RWLock,
			EdgeSourceMap: edgesMap,
		}

		// Create a context with 200ms timeout (shorter than server delay of 500ms)
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		start := time.Now()
		result := requestNode.RunSync(ctx, req)
		elapsed := time.Since(start)

		// Should fail due to timeout
		testutil.AssertNot(t, nil, result.Err)

		// Should timeout in around 200ms, not 500ms
		if elapsed > 300*time.Millisecond {
			t.Errorf("Expected timeout in ~200ms, but took %v", elapsed)
		}
	})

	t.Run("Long timeout should succeed", func(t *testing.T) {
		// Create a faster test server for this test
		fastServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond) // Reduced from 1s to 100ms
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("Fast response"))
		}))
		defer fastServer.Close()

		api.Url = fastServer.URL

		// Create HTTP client
		httpClient := &http.Client{
			Timeout: 30 * time.Second,
		}

		requestNodeRespChan := make(chan nrequest.NodeRequestSideResp, 1)
		requestNode := nrequest.New(id, "timeout-test", api, example, queries, headers, rawBody, formBody, urlBody,
			exampleResp, exampleRespHeader, asserts,
			httpClient, requestNodeRespChan)

		edge1 := edge.NewEdge(idwrap.NewNow(), id, next, edge.HandleUnspecified, edge.EdgeKindUnspecified)
		edges := []edge.Edge{edge1}
		edgesMap := edge.NewEdgesMap(edges)

		var RWLock sync.RWMutex
		req := &node.FlowNodeRequest{
			VarMap:        map[string]interface{}{},
			ReadWriteLock: &RWLock,
			EdgeSourceMap: edgesMap,
		}

		// Create a context with 1 second timeout (longer than server delay of 100ms)
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		start := time.Now()
		result := requestNode.RunSync(ctx, req)
		elapsed := time.Since(start)

		// Should succeed
		testutil.Assert(t, nil, result.Err)

		// Should complete in around 100ms
		if elapsed > 200*time.Millisecond {
			t.Errorf("Expected completion in ~100ms, but took %v", elapsed)
		}
	})

	t.Run("Context cancellation should work", func(t *testing.T) {
		api.Url = server.URL // Use slow server again (500ms delay)

		// Create HTTP client
		httpClient := &http.Client{
			Timeout: 30 * time.Second,
		}

		requestNodeRespChan := make(chan nrequest.NodeRequestSideResp, 1)
		requestNode := nrequest.New(id, "timeout-test", api, example, queries, headers, rawBody, formBody, urlBody,
			exampleResp, exampleRespHeader, asserts,
			httpClient, requestNodeRespChan)

		edge1 := edge.NewEdge(idwrap.NewNow(), id, next, edge.HandleUnspecified, edge.EdgeKindUnspecified)
		edges := []edge.Edge{edge1}
		edgesMap := edge.NewEdgesMap(edges)

		var RWLock sync.RWMutex
		req := &node.FlowNodeRequest{
			VarMap:        map[string]interface{}{},
			ReadWriteLock: &RWLock,
			EdgeSourceMap: edgesMap,
		}

		// Create a context that we'll cancel manually
		ctx, cancel := context.WithCancel(context.Background())

		// Cancel after 100ms (before server responds at 500ms)
		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()

		start := time.Now()
		result := requestNode.RunSync(ctx, req)
		elapsed := time.Since(start)

		// Should fail due to cancellation
		testutil.AssertNot(t, nil, result.Err)

		// Should be cancelled in around 100ms
		if elapsed > 200*time.Millisecond {
			t.Errorf("Expected cancellation in ~100ms, but took %v", elapsed)
		}
	})
}
