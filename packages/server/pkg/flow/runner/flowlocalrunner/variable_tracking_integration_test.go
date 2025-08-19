package flowlocalrunner_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/node/nrequest"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/flow/runner/flowlocalrunner"
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
	"the-dev-tools/server/pkg/model/mnnode"
)

func TestVariableTrackingIntegration_RequestNodeWithVariables(t *testing.T) {
	// Create a REQUEST node that uses multiple variables
	nodeID := idwrap.NewNow()

	api := mitemapi.ItemApi{
		Method: "POST",
		Url:    "{{baseUrl}}/{{version}}/users",
	}

	example := mitemapiexample.ItemApiExample{
		ID:       idwrap.NewNow(),
		Name:     "test-example",
		BodyType: mitemapiexample.BodyTypeRaw,
	}

	// Headers with variables
	headers := []mexampleheader.Header{
		{HeaderKey: "Authorization", Value: "Bearer {{token}}", Enable: true},
		{HeaderKey: "Content-Type", Value: "application/json", Enable: true},
	}

	// Queries with variables
	queries := []mexamplequery.Query{
		{QueryKey: "limit", Value: "{{limit}}", Enable: true},
		{QueryKey: "include", Value: "{{include}}", Enable: true},
	}

	// Body with variables
	bodyData := `{"name": "{{userName}}", "email": "{{userEmail}}", "role": "{{userRole}}"}`
	rawBody := mbodyraw.ExampleBodyRaw{
		Data: []byte(bodyData),
	}

	formBody := []mbodyform.BodyForm{}
	urlBody := []mbodyurl.BodyURLEncoded{}

	exampleResp := mexampleresp.ExampleResp{}
	exampleRespHeader := []mexamplerespheader.ExampleRespHeader{}
	asserts := []massert.Assert{}

	// Mock HTTP client
	mockResp := &http.Response{
		StatusCode: 201,
		Body:       io.NopCloser(bytes.NewBufferString(`{"id": 123, "status": "created"}`)),
	}
	mockHttpClient := httpmockclient.NewMockHttpClient(mockResp)

	requestNodeRespChan := make(chan nrequest.NodeRequestSideResp, 1)
	requestNode := nrequest.New(nodeID, "TestRequest", api, example, queries, headers, rawBody, formBody, urlBody,
		exampleResp, exampleRespHeader, asserts,
		mockHttpClient, requestNodeRespChan)

	// Create flow node map
	flowNodeMap := map[idwrap.IDWrap]node.FlowNode{
		nodeID: requestNode,
	}

	// No edges for this simple single-node test
	edgesMap := edge.EdgesMap{}

	// Create flow runner
	flowID := idwrap.NewNow()
	runnerID := idwrap.NewNow()
	flowRunner := flowlocalrunner.CreateFlowRunner(runnerID, flowID, nodeID, flowNodeMap, edgesMap, 0)

	// Setup variables that will be used by the REQUEST node
	baseVars := map[string]any{
		"baseUrl":   "https://api.example.com",
		"version":   "v1",
		"token":     "abc123xyz",
		"limit":     "50",
		"include":   "profile,permissions",
		"userName":  "john_doe",
		"userEmail": "john@example.com",
		"userRole":  "admin",
	}

	// Channels to capture status
	statusChan := make(chan runner.FlowNodeStatus, 10)
	flowStatusChan := make(chan runner.FlowStatus, 10)

	// Run the flow
	ctx := context.TODO()
	err := flowRunner.Run(ctx, statusChan, flowStatusChan, baseVars)
	if err != nil {
		t.Fatalf("Flow execution failed: %v", err)
	}

	// Collect all statuses
	var allStatuses []runner.FlowNodeStatus
	for status := range statusChan {
		allStatuses = append(allStatuses, status)
	}

	// Find the success status for our REQUEST node
	var successStatus *runner.FlowNodeStatus
	for _, status := range allStatuses {
		if status.NodeID == nodeID && status.State == mnnode.NODE_STATE_SUCCESS {
			successStatus = &status
			break
		}
	}

	if successStatus == nil {
		t.Fatal("Did not find successful execution status for REQUEST node")
	}

	// Verify that InputData contains the tracked variables
	if successStatus.InputData == nil {
		t.Fatal("InputData is nil, expected tracked variables")
	}

	inputData, ok := successStatus.InputData.(map[string]any)
	if !ok {
		t.Fatalf("InputData is not a map, got %T", successStatus.InputData)
	}

	// InputData now contains variables directly (no 'variables' wrapper)
	// Verify all expected variables were tracked
	expectedVars := map[string]string{
		"baseUrl":   "https://api.example.com",
		"version":   "v1",
		"token":     "abc123xyz",
		"limit":     "50",
		"include":   "profile,permissions",
		"userName":  "john_doe",
		"userEmail": "john@example.com",
		"userRole":  "admin",
	}

	if len(inputData) != len(expectedVars) {
		t.Errorf("Expected %d tracked variables, got %d", len(expectedVars), len(inputData))
		t.Logf("Tracked variables: %v", inputData)
	}

	for key, expectedValue := range expectedVars {
		actualValue, found := inputData[key]
		if !found {
			t.Errorf("Expected variable '%s' was not tracked", key)
			continue
		}
		if actualValue != expectedValue {
			t.Errorf("Variable '%s': expected '%s', got '%v'", key, expectedValue, actualValue)
		}
	}

	t.Logf("✅ Successfully tracked %d variables in REQUEST node InputData", len(inputData))

	// Pretty print the InputData for verification
	inputJSON, _ := json.MarshalIndent(inputData, "", "  ")
	t.Logf("📊 Complete InputData:\n%s", string(inputJSON))
}

func TestVariableTrackingIntegration_RequestNodeWithNoVariables(t *testing.T) {
	// Create a REQUEST node that uses no variables
	nodeID := idwrap.NewNow()

	api := mitemapi.ItemApi{
		Method: "GET",
		Url:    "https://api.example.com/static/endpoint",
	}

	example := mitemapiexample.ItemApiExample{
		ID:       idwrap.NewNow(),
		Name:     "static-example",
		BodyType: mitemapiexample.BodyTypeRaw,
	}

	// Static headers, queries, body
	headers := []mexampleheader.Header{
		{HeaderKey: "Content-Type", Value: "application/json", Enable: true},
	}

	queries := []mexamplequery.Query{
		{QueryKey: "format", Value: "json", Enable: true},
	}

	rawBody := mbodyraw.ExampleBodyRaw{}
	formBody := []mbodyform.BodyForm{}
	urlBody := []mbodyurl.BodyURLEncoded{}

	exampleResp := mexampleresp.ExampleResp{}
	exampleRespHeader := []mexamplerespheader.ExampleRespHeader{}
	asserts := []massert.Assert{}

	// Mock HTTP client
	mockResp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewBufferString(`{"message": "success"}`)),
	}
	mockHttpClient := httpmockclient.NewMockHttpClient(mockResp)

	requestNodeRespChan := make(chan nrequest.NodeRequestSideResp, 1)
	requestNode := nrequest.New(nodeID, "StaticRequest", api, example, queries, headers, rawBody, formBody, urlBody,
		exampleResp, exampleRespHeader, asserts,
		mockHttpClient, requestNodeRespChan)

	// Create flow node map
	flowNodeMap := map[idwrap.IDWrap]node.FlowNode{
		nodeID: requestNode,
	}

	// No edges for this simple single-node test
	edgesMap := edge.EdgesMap{}

	// Create flow runner
	flowID := idwrap.NewNow()
	runnerID := idwrap.NewNow()
	flowRunner := flowlocalrunner.CreateFlowRunner(runnerID, flowID, nodeID, flowNodeMap, edgesMap, 0)

	// Setup variables (but node won't use them)
	baseVars := map[string]any{
		"unused": "value",
	}

	// Channels to capture status
	statusChan := make(chan runner.FlowNodeStatus, 10)
	flowStatusChan := make(chan runner.FlowStatus, 10)

	// Run the flow
	ctx := context.TODO()
	err := flowRunner.Run(ctx, statusChan, flowStatusChan, baseVars)
	if err != nil {
		t.Fatalf("Flow execution failed: %v", err)
	}

	// Collect all statuses
	var allStatuses []runner.FlowNodeStatus
	for status := range statusChan {
		allStatuses = append(allStatuses, status)
	}

	// Find the success status for our REQUEST node
	var successStatus *runner.FlowNodeStatus
	for _, status := range allStatuses {
		if status.NodeID == nodeID && status.State == mnnode.NODE_STATE_SUCCESS {
			successStatus = &status
			break
		}
	}

	if successStatus == nil {
		t.Fatal("Did not find successful execution status for REQUEST node")
	}

	// Verify that InputData doesn't contain a variables section or it's empty
	if successStatus.InputData != nil {
		inputData, ok := successStatus.InputData.(map[string]any)
		if ok {
			variablesData, hasVariables := inputData["variables"]
			if hasVariables {
				variables, ok := variablesData.(map[string]any)
				if ok && len(variables) > 0 {
					t.Errorf("Expected no tracked variables, but found: %v", variables)
				}
			}
		}
	}

	t.Logf("✅ Correctly handled REQUEST node with no variables")
}

func TestVariableTrackingIntegration_RequestNodeWithPartialVariables(t *testing.T) {
	// Create a REQUEST node that uses some variables and some static values
	nodeID := idwrap.NewNow()

	api := mitemapi.ItemApi{
		Method: "PUT",
		Url:    "{{baseUrl}}/static/{{resourceId}}",
	}

	example := mitemapiexample.ItemApiExample{
		ID:       idwrap.NewNow(),
		Name:     "partial-example",
		BodyType: mitemapiexample.BodyTypeRaw,
	}

	// Mix of variable and static headers
	headers := []mexampleheader.Header{
		{HeaderKey: "Authorization", Value: "Bearer {{token}}", Enable: true},
		{HeaderKey: "Content-Type", Value: "application/json", Enable: true},
		{HeaderKey: "X-Static", Value: "static-value", Enable: true},
	}

	// Mix of variable and static queries
	queries := []mexamplequery.Query{
		{QueryKey: "dynamic", Value: "{{dynamicValue}}", Enable: true},
		{QueryKey: "static", Value: "constant", Enable: true},
	}

	// Body with both variables and static content
	bodyData := `{"dynamic": "{{dynamicField}}", "static": "constant-value"}`
	rawBody := mbodyraw.ExampleBodyRaw{
		Data: []byte(bodyData),
	}

	formBody := []mbodyform.BodyForm{}
	urlBody := []mbodyurl.BodyURLEncoded{}

	exampleResp := mexampleresp.ExampleResp{}
	exampleRespHeader := []mexamplerespheader.ExampleRespHeader{}
	asserts := []massert.Assert{}

	// Mock HTTP client
	mockResp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewBufferString(`{"updated": true}`)),
	}
	mockHttpClient := httpmockclient.NewMockHttpClient(mockResp)

	requestNodeRespChan := make(chan nrequest.NodeRequestSideResp, 1)
	requestNode := nrequest.New(nodeID, "PartialRequest", api, example, queries, headers, rawBody, formBody, urlBody,
		exampleResp, exampleRespHeader, asserts,
		mockHttpClient, requestNodeRespChan)

	// Create flow node map
	flowNodeMap := map[idwrap.IDWrap]node.FlowNode{
		nodeID: requestNode,
	}

	// No edges for this simple single-node test
	edgesMap := edge.EdgesMap{}

	// Create flow runner
	flowID := idwrap.NewNow()
	runnerID := idwrap.NewNow()
	flowRunner := flowlocalrunner.CreateFlowRunner(runnerID, flowID, nodeID, flowNodeMap, edgesMap, 0)

	// Setup variables (some used, some not)
	baseVars := map[string]any{
		"baseUrl":      "https://api.example.com",
		"resourceId":   "123",
		"token":        "secret-token",
		"dynamicValue": "runtime-value",
		"dynamicField": "runtime-field",
		"unusedVar":    "not-used",
	}

	// Channels to capture status
	statusChan := make(chan runner.FlowNodeStatus, 10)
	flowStatusChan := make(chan runner.FlowStatus, 10)

	// Run the flow
	ctx := context.TODO()
	err := flowRunner.Run(ctx, statusChan, flowStatusChan, baseVars)
	if err != nil {
		t.Fatalf("Flow execution failed: %v", err)
	}

	// Collect all statuses
	var allStatuses []runner.FlowNodeStatus
	for status := range statusChan {
		allStatuses = append(allStatuses, status)
	}

	// Find the success status for our REQUEST node
	var successStatus *runner.FlowNodeStatus
	for _, status := range allStatuses {
		if status.NodeID == nodeID && status.State == mnnode.NODE_STATE_SUCCESS {
			successStatus = &status
			break
		}
	}

	if successStatus == nil {
		t.Fatal("Did not find successful execution status for REQUEST node")
	}

	// Verify that InputData contains only the used variables
	if successStatus.InputData == nil {
		t.Fatal("InputData is nil, expected tracked variables")
	}

	inputData, ok := successStatus.InputData.(map[string]any)
	if !ok {
		t.Fatalf("InputData is not a map, got %T", successStatus.InputData)
	}

	// InputData now contains variables directly (no 'variables' wrapper)
	// Verify only the used variables were tracked
	expectedVars := map[string]string{
		"baseUrl":      "https://api.example.com",
		"resourceId":   "123",
		"token":        "secret-token",
		"dynamicValue": "runtime-value",
		"dynamicField": "runtime-field",
	}

	if len(inputData) != len(expectedVars) {
		t.Errorf("Expected %d tracked variables, got %d", len(expectedVars), len(inputData))
		t.Logf("Tracked variables: %v", inputData)
	}

	for key, expectedValue := range expectedVars {
		actualValue, found := inputData[key]
		if !found {
			t.Errorf("Expected variable '%s' was not tracked", key)
			continue
		}
		if actualValue != expectedValue {
			t.Errorf("Variable '%s': expected '%s', got '%v'", key, expectedValue, actualValue)
		}
	}

	// Verify unused variable was not tracked
	if _, found := inputData["unusedVar"]; found {
		t.Error("Unused variable 'unusedVar' should not have been tracked")
	}

	t.Logf("✅ Successfully tracked only the used variables (%d out of %d available)", len(inputData), len(baseVars))
}
