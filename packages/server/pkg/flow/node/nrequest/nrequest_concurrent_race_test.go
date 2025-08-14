package nrequest_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"sync"
	"testing"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/node/nrequest"
	"the-dev-tools/server/pkg/flow/tracking"
	"the-dev-tools/server/pkg/httpclient/httpmockclient"
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

func TestNodeRequest_ConcurrentExecutionWithSharedTracker(t *testing.T) {
	// Create a shared variable tracker that will be used by all nodes
	sharedTracker := tracking.NewVariableTracker()
	
	// Shared variable map
	varMap := map[string]interface{}{
		"baseUrl": "https://api.example.com",
		"version": "v1",
		"token":   "bearer-token-123",
		"limit":   "100",
		"offset":  "0",
		"userId":  "user-456",
	}
	
	// Number of concurrent request nodes
	const numNodes = 10
	const numIterations = 5
	
	var wg sync.WaitGroup
	var RWLock sync.RWMutex
	
	// Run multiple iterations to increase chance of detecting race conditions
	for iteration := 0; iteration < numIterations; iteration++ {
		wg.Add(numNodes)
		
		for i := 0; i < numNodes; i++ {
			go func(nodeIndex int) {
				defer wg.Done()
				
				// Create unique node for this goroutine
				nodeId := idwrap.NewNow()
				nextId := idwrap.NewNow()
				
				// Setup API endpoint with variables
				api := mitemapi.ItemApi{
					Method: "GET",
					Url:    "{{baseUrl}}/{{version}}/users/{{userId}}",
				}
				
				example := mitemapiexample.ItemApiExample{
					ID:       idwrap.NewNow(),
					Name:     "example",
					BodyType: mitemapiexample.BodyTypeRaw,
				}
				
				// Setup headers with variables
				headers := []mexampleheader.Header{
					{HeaderKey: "Authorization", Value: "{{token}}", Enable: true},
					{HeaderKey: "X-Node-ID", Value: string(rune('A' + nodeIndex)), Enable: true},
				}
				
				// Setup queries with variables
				queries := []mexamplequery.Query{
					{QueryKey: "limit", Value: "{{limit}}", Enable: true},
					{QueryKey: "offset", Value: "{{offset}}", Enable: true},
				}
				
				// Setup mock HTTP response
				mockResp := &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewBuffer([]byte(`{"result": "ok"}`))),
				}
				mockHttpClient := httpmockclient.NewMockHttpClient(mockResp)
				
				requestNodeRespChan := make(chan nrequest.NodeRequestSideResp, 1)
				
				requestNode := nrequest.New(
					nodeId, 
					"node-"+string(rune('A'+nodeIndex)),
					api, 
					example, 
					queries, 
					headers,
					mbodyraw.ExampleBodyRaw{},
					[]mbodyform.BodyForm{},
					[]mbodyurl.BodyURLEncoded{},
					mexampleresp.ExampleResp{},
					[]mexamplerespheader.ExampleRespHeader{},
					[]massert.Assert{},
					mockHttpClient,
					requestNodeRespChan,
				)
				
				edge1 := edge.NewEdge(idwrap.NewNow(), nodeId, nextId, edge.HandleUnspecified, edge.EdgeKindUnspecified)
				edges := []edge.Edge{edge1}
				edgesMap := edge.NewEdgesMap(edges)
				
				req := &node.FlowNodeRequest{
					VarMap:          varMap,
					ReadWriteLock:   &RWLock,
					EdgeSourceMap:   edgesMap,
					VariableTracker: sharedTracker, // All nodes share the same tracker
				}
				
				ctx := context.Background()
				
				// Randomly choose between sync and async execution
				if nodeIndex%2 == 0 {
					// RunSync
					_ = requestNode.RunSync(ctx, req)
				} else {
					// RunAsync
					resChan := make(chan node.FlowNodeResult, 1)
					go requestNode.RunAsync(ctx, req, resChan)
					<-resChan
				}
			}(i)
		}
		
		wg.Wait()
	}
	
	// Verify that the tracker has captured variables from all nodes
	readVars := sharedTracker.GetReadVars()
	
	// Check that expected variables were tracked
	expectedKeys := []string{"baseUrl", "version", "token", "limit", "offset", "userId"}
	for _, key := range expectedKeys {
		if _, exists := readVars[key]; !exists {
			t.Errorf("Expected variable '%s' to be tracked, but it wasn't", key)
		}
	}
	
	// Verify values match what we expect
	if readVars["baseUrl"] != "https://api.example.com" {
		t.Errorf("Unexpected value for baseUrl: %v", readVars["baseUrl"])
	}
	if readVars["token"] != "bearer-token-123" {
		t.Errorf("Unexpected value for token: %v", readVars["token"])
	}
}

func TestNodeRequest_MassiveConcurrentTracking(t *testing.T) {
	// Stress test with many goroutines
	sharedTracker := tracking.NewVariableTracker()
	
	const numGoroutines = 100
	const numVars = 50
	
	// Create a large variable map
	varMap := make(map[string]interface{})
	for i := 0; i < numVars; i++ {
		varMap["var_"+string(rune('a'+i%26))+string(rune('0'+i/26))] = "value_" + string(rune('a'+i%26))
	}
	
	var wg sync.WaitGroup
	var RWLock sync.RWMutex
	
	wg.Add(numGoroutines)
	
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			
			nodeId := idwrap.NewNow()
			nextId := idwrap.NewNow()
			
			// Use different variables in URL for each goroutine
			varKey := "var_" + string(rune('a'+id%26)) + string(rune('0'+id/26))
			api := mitemapi.ItemApi{
				Method: "GET",
				Url:    "https://api.example.com/{{" + varKey + "}}",
			}
			
			example := mitemapiexample.ItemApiExample{
				ID:       idwrap.NewNow(),
				Name:     "example",
				BodyType: mitemapiexample.BodyTypeRaw,
			}
			
			mockResp := &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBuffer([]byte("OK"))),
			}
			mockHttpClient := httpmockclient.NewMockHttpClient(mockResp)
			
			requestNodeRespChan := make(chan nrequest.NodeRequestSideResp, 1)
			
			requestNode := nrequest.New(
				nodeId,
				"node-"+string(rune('0'+id)),
				api,
				example,
				[]mexamplequery.Query{},
				[]mexampleheader.Header{},
				mbodyraw.ExampleBodyRaw{},
				[]mbodyform.BodyForm{},
				[]mbodyurl.BodyURLEncoded{},
				mexampleresp.ExampleResp{},
				[]mexamplerespheader.ExampleRespHeader{},
				[]massert.Assert{},
				mockHttpClient,
				requestNodeRespChan,
			)
			
			edge1 := edge.NewEdge(idwrap.NewNow(), nodeId, nextId, edge.HandleUnspecified, edge.EdgeKindUnspecified)
			edges := []edge.Edge{edge1}
			edgesMap := edge.NewEdgesMap(edges)
			
			req := &node.FlowNodeRequest{
				VarMap:          varMap,
				ReadWriteLock:   &RWLock,
				EdgeSourceMap:   edgesMap,
				VariableTracker: sharedTracker,
			}
			
			ctx := context.Background()
			_ = requestNode.RunSync(ctx, req)
		}(i)
	}
	
	wg.Wait()
	
	// Verify tracking worked
	readVars := sharedTracker.GetReadVars()
	if len(readVars) == 0 {
		t.Error("Expected tracked variables, but found none")
	}
}