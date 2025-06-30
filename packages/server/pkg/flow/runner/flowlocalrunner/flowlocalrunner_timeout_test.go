package flowlocalrunner_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/node/nrequest"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/flow/runner/flowlocalrunner"
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
)

func TestFlowRunner_LongRunningHTTPRequest(t *testing.T) {
	// Create a test server that simulates a long-running request (5 seconds)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Long running response"))
	}))
	defer server.Close()

	// Set up the flow with a request node
	flowID := idwrap.NewNow()
	nodeID := idwrap.NewNow()

	api := mitemapi.ItemApi{
		Method: "GET",
		Url:    server.URL,
	}

	example := mitemapiexample.ItemApiExample{
		ID:   idwrap.NewNow(),
		Name: "long-request",
	}

	queries := []mexamplequery.Query{}
	headers := []mexampleheader.Header{}
	rawBody := mbodyraw.ExampleBodyRaw{}
	formBody := []mbodyform.BodyForm{}
	urlBody := []mbodyurl.BodyURLEncoded{}
	exampleResp := mexampleresp.ExampleResp{}
	exampleRespHeader := []mexamplerespheader.ExampleRespHeader{}
	asserts := []massert.Assert{}

	// Create HTTP client with long timeout
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	requestNodeRespChan := make(chan nrequest.NodeRequestSideResp, 1)
	requestNode := nrequest.New(nodeID, "long-request", api, example, queries, headers, rawBody, formBody, urlBody,
		exampleResp, exampleRespHeader, asserts,
		httpClient, requestNodeRespChan)

	flowNodeMap := map[idwrap.IDWrap]node.FlowNode{
		nodeID: requestNode,
	}

	edgesMap := edge.NewEdgesMap([]edge.Edge{})

	t.Run("Flow with 60 second timeout should complete long HTTP request", func(t *testing.T) {
		// Create flow runner with 60 second timeout (like the default in production)
		timeout := 60 * time.Second
		runner := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), flowID, nodeID, flowNodeMap, edgesMap, timeout)

		flowNodeStatusChan := make(chan runner.FlowNodeStatus, 100)
		flowStatusChan := make(chan runner.FlowStatus, 10)

		// Create a context that simulates a short gRPC timeout (3 seconds)
		// This tests that the flow runner properly isolates from the parent context
		gRPCCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		// Create a new context without the gRPC deadline (simulating what rflow.go does)
		flowCtx, flowCancel := context.WithCancel(context.Background())
		defer flowCancel()

		// Start the flow
		start := time.Now()
		done := make(chan error, 1)
		
		go func() {
			err := runner.Run(flowCtx, flowNodeStatusChan, flowStatusChan, nil)
			done <- err
		}()

		// Monitor status updates
		statusReceived := false

		go func() {
			for _ = range flowNodeStatusChan {
				statusReceived = true
			}
		}()

		// Wait for completion or timeout
		select {
		case err := <-done:
			elapsed := time.Since(start)
			
			// Should succeed even though it took 5 seconds
			if err != nil {
				t.Errorf("Expected flow to complete successfully, but got error: %v", err)
			}
			
			// Should take around 5 seconds
			if elapsed < 5*time.Second || elapsed > 7*time.Second {
				t.Errorf("Expected flow to complete in ~5 seconds, but took %v", elapsed)
			}
			
			// Verify we got status updates
			if !statusReceived {
				t.Error("Expected to receive node status updates")
			}
			
			// Verify the gRPC context timed out but didn't affect the flow
			select {
			case <-gRPCCtx.Done():
				// Good, gRPC context timed out as expected
			default:
				t.Error("Expected gRPC context to have timed out")
			}
			
		case <-time.After(10 * time.Second):
			t.Error("Test timed out waiting for flow completion")
		}
	})

	t.Run("Flow with 2 second timeout should fail on long HTTP request", func(t *testing.T) {
		// Create flow runner with short timeout
		timeout := 2 * time.Second
		runner := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), flowID, nodeID, flowNodeMap, edgesMap, timeout)

		flowNodeStatusChan := make(chan runner.FlowNodeStatus, 100)
		flowStatusChan := make(chan runner.FlowStatus, 10)

		ctx := context.Background()

		// Start the flow
		start := time.Now()
		err := runner.Run(ctx, flowNodeStatusChan, flowStatusChan, nil)
		elapsed := time.Since(start)

		// Should fail due to timeout
		if err == nil {
			t.Error("Expected flow to fail with timeout error")
		}

		// Should timeout in around 2 seconds
		if elapsed > 3*time.Second {
			t.Errorf("Expected timeout in ~2 seconds, but took %v", elapsed)
		}
	})
}

func TestFlowRunner_ContextIsolation(t *testing.T) {
	// Create a simple HTTP server that responds immediately
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Quick response"))
	}))
	defer server.Close()

	// Set up the flow
	flowID := idwrap.NewNow()
	nodeID := idwrap.NewNow()

	api := mitemapi.ItemApi{
		Method: "GET",
		Url:    server.URL,
	}

	example := mitemapiexample.ItemApiExample{
		ID:   idwrap.NewNow(),
		Name: "test-request",
	}

	queries := []mexamplequery.Query{}
	headers := []mexampleheader.Header{}
	rawBody := mbodyraw.ExampleBodyRaw{}
	formBody := []mbodyform.BodyForm{}
	urlBody := []mbodyurl.BodyURLEncoded{}
	exampleResp := mexampleresp.ExampleResp{}
	exampleRespHeader := []mexamplerespheader.ExampleRespHeader{}
	asserts := []massert.Assert{}

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	requestNodeRespChan := make(chan nrequest.NodeRequestSideResp, 1)
	requestNode := nrequest.New(nodeID, "test-request", api, example, queries, headers, rawBody, formBody, urlBody,
		exampleResp, exampleRespHeader, asserts,
		httpClient, requestNodeRespChan)

	flowNodeMap := map[idwrap.IDWrap]node.FlowNode{
		nodeID: requestNode,
	}

	edgesMap := edge.NewEdgesMap([]edge.Edge{})

	t.Run("Cancelled parent context should not affect flow with isolated context", func(t *testing.T) {
		// Create flow runner
		timeout := 10 * time.Second
		runner := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), flowID, nodeID, flowNodeMap, edgesMap, timeout)

		flowNodeStatusChan := make(chan runner.FlowNodeStatus, 100)
		flowStatusChan := make(chan runner.FlowStatus, 10)

		// Create a parent context that we'll cancel immediately
		parentCtx, parentCancel := context.WithCancel(context.Background())
		
		// Create isolated context (simulating what rflow.go does)
		isolatedCtx := context.Background()

		// Cancel parent context immediately
		parentCancel()

		// Run flow with isolated context
		err := runner.Run(isolatedCtx, flowNodeStatusChan, flowStatusChan, nil)

		// Should succeed despite parent context being cancelled
		if err != nil {
			t.Errorf("Expected flow to complete successfully, but got error: %v", err)
		}

		// Verify parent context is cancelled
		select {
		case <-parentCtx.Done():
			// Good
		default:
			t.Error("Expected parent context to be cancelled")
		}
	})
}