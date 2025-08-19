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
	"the-dev-tools/server/pkg/testutil"
)

func TestNodeRequest_RunSync_VariableTracking(t *testing.T) {
	id := idwrap.NewNow()
	next := idwrap.NewNow()

	// Setup API endpoint with variables
	api := mitemapi.ItemApi{
		Method: "GET",
		Url:    "{{baseUrl}}/{{version}}/users",
	}

	example := mitemapiexample.ItemApiExample{
		ID:       idwrap.NewNow(),
		Name:     "example",
		BodyType: mitemapiexample.BodyTypeRaw,
	}

	// Setup headers with variables
	headers := []mexampleheader.Header{
		{HeaderKey: "Authorization", Value: "Bearer {{token}}", Enable: true},
	}

	// Setup queries with variables
	queries := []mexamplequery.Query{
		{QueryKey: "limit", Value: "{{limit}}", Enable: true},
	}

	rawBody := mbodyraw.ExampleBodyRaw{}
	formBody := []mbodyform.BodyForm{}
	urlBody := []mbodyurl.BodyURLEncoded{}

	exampleResp := mexampleresp.ExampleResp{}
	exampleRespHeader := []mexamplerespheader.ExampleRespHeader{}
	asserts := []massert.Assert{}

	// Setup mock HTTP client
	expectedBody := []byte("Hello, World!")
	buf := bytes.NewBuffer(expectedBody)
	readCloser := io.NopCloser(buf)

	mockResp := &http.Response{
		StatusCode: 200,
		Body:       readCloser,
	}
	mockHttpClient := httpmockclient.NewMockHttpClient(mockResp)

	name := "example"

	requestNodeRespChan := make(chan nrequest.NodeRequestSideResp, 1)
	requestNode := nrequest.New(id, name, api, example, queries, headers, rawBody, formBody, urlBody,
		exampleResp, exampleRespHeader, asserts,
		mockHttpClient, requestNodeRespChan)

	edge1 := edge.NewEdge(idwrap.NewNow(), id, next, edge.HandleUnspecified, edge.EdgeKindUnspecified)
	edges := []edge.Edge{edge1}
	edgesMap := edge.NewEdgesMap(edges)

	var RWLock sync.RWMutex

	// Create variable tracker
	tracker := tracking.NewVariableTracker()

	// Setup variables that will be resolved
	varMap := map[string]interface{}{
		"baseUrl": "https://api.example.com",
		"version": "v1",
		"token":   "abc123",
		"limit":   "10",
	}

	req := &node.FlowNodeRequest{
		VarMap:          varMap,
		ReadWriteLock:   &RWLock,
		EdgeSourceMap:   edgesMap,
		VariableTracker: tracker,
	}

	ctx := context.TODO()
	result := requestNode.RunSync(ctx, req)

	testutil.Assert(t, next, result.NextNodeID[0])
	testutil.Assert(t, nil, result.Err)
	testutil.Assert(t, id, requestNode.GetID())

	// Check that variables were tracked
	readVars := tracker.GetReadVars()

	expectedVars := map[string]interface{}{
		"baseUrl": "https://api.example.com",
		"version": "v1",
		"token":   "abc123",
		"limit":   "10",
	}

	if len(readVars) != len(expectedVars) {
		t.Errorf("Expected %d tracked variables, got %d", len(expectedVars), len(readVars))
		t.Logf("Tracked variables: %v", readVars)
	}

	for key, expectedValue := range expectedVars {
		if readVars[key] != expectedValue {
			t.Errorf("Expected tracked %s value '%v', got '%v'", key, expectedValue, readVars[key])
		}
	}
}

func TestNodeRequest_RunAsync_VariableTracking(t *testing.T) {
	id := idwrap.NewNow()
	next := idwrap.NewNow()

	// Setup API endpoint with variables
	api := mitemapi.ItemApi{
		Method: "POST",
		Url:    "{{baseUrl}}/users",
	}

	example := mitemapiexample.ItemApiExample{
		ID:       idwrap.NewNow(),
		Name:     "example",
		BodyType: mitemapiexample.BodyTypeRaw,
	}

	// Setup body with variables
	bodyData := `{"name": "{{userName}}", "email": "{{userEmail}}"}`
	rawBody := mbodyraw.ExampleBodyRaw{
		Data: []byte(bodyData),
	}

	queries := []mexamplequery.Query{}
	headers := []mexampleheader.Header{}
	formBody := []mbodyform.BodyForm{}
	urlBody := []mbodyurl.BodyURLEncoded{}

	exampleResp := mexampleresp.ExampleResp{}
	exampleRespHeader := []mexamplerespheader.ExampleRespHeader{}
	asserts := []massert.Assert{}

	// Setup mock HTTP client
	expectedBody := []byte("Created")
	buf := bytes.NewBuffer(expectedBody)
	readCloser := io.NopCloser(buf)

	mockResp := &http.Response{
		StatusCode: 201,
		Body:       readCloser,
	}
	mockHttpClient := httpmockclient.NewMockHttpClient(mockResp)

	requestNodeRespChan := make(chan nrequest.NodeRequestSideResp, 1)
	nodeName := "test-node"

	requestNode := nrequest.New(id, nodeName, api, example, queries, headers, rawBody, formBody, urlBody,
		exampleResp, exampleRespHeader, asserts,
		mockHttpClient, requestNodeRespChan)
	edge1 := edge.NewEdge(idwrap.NewNow(), id, next, edge.HandleUnspecified, edge.EdgeKindUnspecified)
	edges := []edge.Edge{edge1}
	edgesMap := edge.NewEdgesMap(edges)

	var RWLock sync.RWMutex

	// Create variable tracker
	tracker := tracking.NewVariableTracker()

	// Setup variables that will be resolved
	varMap := map[string]interface{}{
		"baseUrl":   "https://api.example.com",
		"userName":  "john_doe",
		"userEmail": "john@example.com",
	}

	req := &node.FlowNodeRequest{
		VarMap:          varMap,
		ReadWriteLock:   &RWLock,
		EdgeSourceMap:   edgesMap,
		VariableTracker: tracker,
	}

	ctx := context.TODO()
	resChan := make(chan node.FlowNodeResult, 1)
	go requestNode.RunAsync(ctx, req, resChan)
	result := <-resChan

	testutil.Assert(t, next, result.NextNodeID[0])
	testutil.Assert(t, nil, result.Err)
	testutil.Assert(t, id, requestNode.GetID())

	// Check that variables were tracked
	readVars := tracker.GetReadVars()

	expectedVars := map[string]interface{}{
		"baseUrl":   "https://api.example.com",
		"userName":  "john_doe",
		"userEmail": "john@example.com",
	}

	if len(readVars) != len(expectedVars) {
		t.Errorf("Expected %d tracked variables, got %d", len(expectedVars), len(readVars))
		t.Logf("Tracked variables: %v", readVars)
	}

	for key, expectedValue := range expectedVars {
		if readVars[key] != expectedValue {
			t.Errorf("Expected tracked %s value '%v', got '%v'", key, expectedValue, readVars[key])
		}
	}
}

func TestNodeRequest_RunSync_NoVariables(t *testing.T) {
	id := idwrap.NewNow()
	next := idwrap.NewNow()

	// Setup API endpoint without variables
	api := mitemapi.ItemApi{
		Method: "GET",
		Url:    "https://api.example.com/users",
	}

	example := mitemapiexample.ItemApiExample{
		ID:       idwrap.NewNow(),
		Name:     "example",
		BodyType: mitemapiexample.BodyTypeRaw,
	}

	queries := []mexamplequery.Query{}
	headers := []mexampleheader.Header{}

	rawBody := mbodyraw.ExampleBodyRaw{}
	formBody := []mbodyform.BodyForm{}
	urlBody := []mbodyurl.BodyURLEncoded{}

	exampleResp := mexampleresp.ExampleResp{}
	exampleRespHeader := []mexamplerespheader.ExampleRespHeader{}
	asserts := []massert.Assert{}

	// Setup mock HTTP client
	expectedBody := []byte("Hello, World!")
	buf := bytes.NewBuffer(expectedBody)
	readCloser := io.NopCloser(buf)

	mockResp := &http.Response{
		StatusCode: 200,
		Body:       readCloser,
	}
	mockHttpClient := httpmockclient.NewMockHttpClient(mockResp)

	name := "example"

	requestNodeRespChan := make(chan nrequest.NodeRequestSideResp, 1)
	requestNode := nrequest.New(id, name, api, example, queries, headers, rawBody, formBody, urlBody,
		exampleResp, exampleRespHeader, asserts,
		mockHttpClient, requestNodeRespChan)

	edge1 := edge.NewEdge(idwrap.NewNow(), id, next, edge.HandleUnspecified, edge.EdgeKindUnspecified)
	edges := []edge.Edge{edge1}
	edgesMap := edge.NewEdgesMap(edges)

	var RWLock sync.RWMutex

	// Create variable tracker
	tracker := tracking.NewVariableTracker()

	req := &node.FlowNodeRequest{
		VarMap:          map[string]interface{}{},
		ReadWriteLock:   &RWLock,
		EdgeSourceMap:   edgesMap,
		VariableTracker: tracker,
	}

	ctx := context.TODO()
	result := requestNode.RunSync(ctx, req)

	testutil.Assert(t, next, result.NextNodeID[0])
	testutil.Assert(t, nil, result.Err)
	testutil.Assert(t, id, requestNode.GetID())

	// Check that no variables were tracked
	readVars := tracker.GetReadVars()
	if len(readVars) != 0 {
		t.Errorf("Expected 0 tracked variables, got %d", len(readVars))
		t.Logf("Tracked variables: %v", readVars)
	}
}

func TestNodeRequest_RunSync_NoTracker(t *testing.T) {
	id := idwrap.NewNow()
	next := idwrap.NewNow()

	// Setup API endpoint with variables
	api := mitemapi.ItemApi{
		Method: "GET",
		Url:    "{{baseUrl}}/users",
	}

	example := mitemapiexample.ItemApiExample{
		ID:       idwrap.NewNow(),
		Name:     "example",
		BodyType: mitemapiexample.BodyTypeRaw,
	}

	queries := []mexamplequery.Query{}
	headers := []mexampleheader.Header{}
	rawBody := mbodyraw.ExampleBodyRaw{}
	formBody := []mbodyform.BodyForm{}
	urlBody := []mbodyurl.BodyURLEncoded{}

	exampleResp := mexampleresp.ExampleResp{}
	exampleRespHeader := []mexamplerespheader.ExampleRespHeader{}
	asserts := []massert.Assert{}

	// Setup mock HTTP client
	expectedBody := []byte("Hello, World!")
	buf := bytes.NewBuffer(expectedBody)
	readCloser := io.NopCloser(buf)

	mockResp := &http.Response{
		StatusCode: 200,
		Body:       readCloser,
	}
	mockHttpClient := httpmockclient.NewMockHttpClient(mockResp)

	name := "example"

	requestNodeRespChan := make(chan nrequest.NodeRequestSideResp, 1)
	requestNode := nrequest.New(id, name, api, example, queries, headers, rawBody, formBody, urlBody,
		exampleResp, exampleRespHeader, asserts,
		mockHttpClient, requestNodeRespChan)

	edge1 := edge.NewEdge(idwrap.NewNow(), id, next, edge.HandleUnspecified, edge.EdgeKindUnspecified)
	edges := []edge.Edge{edge1}
	edgesMap := edge.NewEdgesMap(edges)

	var RWLock sync.RWMutex

	// Setup variables but NO tracker
	varMap := map[string]interface{}{
		"baseUrl": "https://api.example.com",
	}

	req := &node.FlowNodeRequest{
		VarMap:          varMap,
		ReadWriteLock:   &RWLock,
		EdgeSourceMap:   edgesMap,
		VariableTracker: nil, // No tracker provided
	}

	ctx := context.TODO()
	result := requestNode.RunSync(ctx, req)

	// Should still work without errors even when no tracker is provided
	testutil.Assert(t, next, result.NextNodeID[0])
	testutil.Assert(t, nil, result.Err)
	testutil.Assert(t, id, requestNode.GetID())
}
