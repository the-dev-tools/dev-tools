package workflowsimple

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/ioworkspace"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mexamplequery"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/model/mworkspace"
)

func TestExportWorkflowClean_WithDeltaOverrides(t *testing.T) {
	// Create IDs
	workspaceID := idwrap.NewNow()
	flowID := idwrap.NewNow()
	
	// Node IDs
	startNodeID := idwrap.NewNow()
	requestNodeID := idwrap.NewNow()
	
	// Base endpoint and example
	baseEndpointID := idwrap.NewNow()
	baseExampleID := idwrap.NewNow()
	
	// Delta endpoint and example
	deltaEndpointID := idwrap.NewNow()
	deltaExampleID := idwrap.NewNow()

	// Create workspace data with delta overrides
	workspaceData := &ioworkspace.WorkspaceData{
		Workspace: mworkspace.Workspace{
			ID:   workspaceID,
			Name: "Test Workspace with Deltas",
		},
		Flows: []mflow.Flow{
			{
				ID:          flowID,
				WorkspaceID: workspaceID,
				Name:        "Test Flow",
			},
		},
		FlowNodes: []mnnode.MNode{
			{
				ID:       startNodeID,
				FlowID:   flowID,
				Name:     "Start",
				NodeKind: mnnode.NODE_KIND_NO_OP,
			},
			{
				ID:       requestNodeID,
				FlowID:   flowID,
				Name:     "request_0",
				NodeKind: mnnode.NODE_KIND_REQUEST,
			},
		},
		FlowNoopNodes: []mnnoop.NoopNode{
			{
				FlowNodeID: startNodeID,
				Type:       mnnoop.NODE_NO_OP_KIND_START,
			},
		},
		FlowRequestNodes: []mnrequest.MNRequest{
			{
				FlowNodeID:      requestNodeID,
				EndpointID:      &baseEndpointID,
				ExampleID:       &baseExampleID,
				DeltaEndpointID: &deltaEndpointID,
				DeltaExampleID:  &deltaExampleID,
			},
		},
		FlowEdges: []edge.Edge{
			{
				FlowID:   flowID,
				SourceID: startNodeID,
				TargetID: requestNodeID,
			},
		},
		// Base endpoint
		Endpoints: []mitemapi.ItemApi{
			{
				ID:           baseEndpointID,
				CollectionID: workspaceID,
				Name:         "Base API",
				Method:       "GET",
				Url:          "https://api.example.com/users",
				Hidden:       false,
			},
			// Delta endpoint with overrides
			{
				ID:           deltaEndpointID,
				CollectionID: workspaceID,
				Name:         "Delta API",
				Method:       "POST", // Changed method
				Url:          "https://api.example.com/users/{{userId}}", // Changed URL
				Hidden:       true,
			},
		},
		// Base example
		Examples: []mitemapiexample.ItemApiExample{
			{
				ID:        baseExampleID,
				ItemApiID: baseEndpointID,
				Name:      "Base Example",
			},
			// Delta example
			{
				ID:        deltaExampleID,
				ItemApiID: deltaEndpointID,
				Name:      "Delta Example",
			},
		},
		// Base headers
		ExampleHeaders: []mexampleheader.Header{
			{
				ID:        idwrap.NewNow(),
				ExampleID: baseExampleID,
				HeaderKey: "Content-Type",
				Value:     "application/json",
				Enable:    true,
			},
			{
				ID:        idwrap.NewNow(),
				ExampleID: baseExampleID,
				HeaderKey: "Accept",
				Value:     "application/json",
				Enable:    true,
			},
			// Delta headers with overrides
			{
				ID:        idwrap.NewNow(),
				ExampleID: deltaExampleID,
				HeaderKey: "Authorization",
				Value:     "Bearer {{token}}", // New header
				Enable:    true,
			},
			{
				ID:        idwrap.NewNow(),
				ExampleID: deltaExampleID,
				HeaderKey: "Content-Type",
				Value:     "application/xml", // Override base header
				Enable:    true,
			},
		},
		// Base queries
		ExampleQueries: []mexamplequery.Query{
			{
				ID:        idwrap.NewNow(),
				ExampleID: baseExampleID,
				QueryKey:  "page",
				Value:     "1",
				Enable:    true,
			},
			// Delta queries with overrides
			{
				ID:        idwrap.NewNow(),
				ExampleID: deltaExampleID,
				QueryKey:  "page",
				Value:     "{{page}}", // Override with variable
				Enable:    true,
			},
			{
				ID:        idwrap.NewNow(),
				ExampleID: deltaExampleID,
				QueryKey:  "limit",
				Value:     "50", // New query param
				Enable:    true,
			},
		},
		// Bodies
		Rawbodies: []mbodyraw.ExampleBodyRaw{
			{
				ID:        idwrap.NewNow(),
				ExampleID: baseExampleID,
				Data:      []byte(`{"name":"John"}`),
			},
			{
				ID:        idwrap.NewNow(),
				ExampleID: deltaExampleID,
				Data:      []byte(`{"name":"{{userName}}","age":30}`), // Different body
			},
		},
	}

	// Export the workflow
	exported, err := ExportWorkflowClean(workspaceData)
	require.NoError(t, err)

	// Parse the exported YAML
	var result map[string]any
	err = yaml.Unmarshal(exported, &result)
	require.NoError(t, err)

	// Verify basic structure
	assert.Equal(t, "Test Workspace with Deltas", result["workspace_name"])

	// Check requests section
	requests, ok := result["requests"].([]any)
	require.True(t, ok)
	require.Len(t, requests, 1)

	// Verify the base request definition
	req := requests[0].(map[string]any)
	assert.Equal(t, "request_0", req["name"])
	assert.Equal(t, "GET", req["method"])
	assert.Equal(t, "https://api.example.com/users", req["url"])
	
	// Base headers should be in request definition
	headers, ok := req["headers"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "application/json", headers["Accept"])
	assert.Equal(t, "application/json", headers["Content-Type"])
	
	// Base queries
	queries, ok := req["query_params"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "1", queries["page"])
	
	// Base body
	body, ok := req["body"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "John", body["name"])

	// Check flows section
	flows, ok := result["flows"].([]any)
	require.True(t, ok)
	require.Len(t, flows, 1)

	flow := flows[0].(map[string]any)
	assert.Equal(t, "Test Flow", flow["name"])

	// Check steps
	steps, ok := flow["steps"].([]any)
	require.True(t, ok)
	require.Len(t, steps, 1)

	// Verify the request step with delta overrides
	step := steps[0].(map[string]any)
	requestStep, ok := step["request"].(map[string]any)
	require.True(t, ok)
	
	assert.Equal(t, "request_0", requestStep["name"])
	assert.Equal(t, "request_0", requestStep["use_request"])
	
	// Check delta overrides in the step
	assert.Equal(t, "POST", requestStep["method"]) // Method override
	assert.Equal(t, "https://api.example.com/users/{{userId}}", requestStep["url"]) // URL override
	
	// Check header overrides (only different/new headers)
	stepHeaders, ok := requestStep["headers"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Bearer {{token}}", stepHeaders["Authorization"]) // New header
	assert.Equal(t, "application/xml", stepHeaders["Content-Type"]) // Override header
	assert.NotContains(t, stepHeaders, "Accept") // Base header not included (not overridden)
	
	// Check query param overrides
	stepQueries, ok := requestStep["query_params"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "{{page}}", stepQueries["page"]) // Override with variable
	assert.Equal(t, "50", stepQueries["limit"]) // New query param
	
	// Check body override
	bodyData, ok := requestStep["body"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "{{userName}}", bodyData["name"])
	// Handle both int and float64 since YAML unmarshalling can produce either
	if ageFloat, ok := bodyData["age"].(float64); ok {
		assert.Equal(t, 30, int(ageFloat))
	} else if ageInt, ok := bodyData["age"].(int); ok {
		assert.Equal(t, 30, ageInt)
	} else {
		t.Fatalf("Unexpected type for age: %T", bodyData["age"])
	}
}

func TestExportWorkflowClean_NoDeltaOverrides(t *testing.T) {
	// Create IDs
	workspaceID := idwrap.NewNow()
	flowID := idwrap.NewNow()
	
	// Node IDs
	startNodeID := idwrap.NewNow()
	requestNodeID := idwrap.NewNow()
	
	// Base endpoint and example
	baseEndpointID := idwrap.NewNow()
	baseExampleID := idwrap.NewNow()

	// Create workspace data without delta overrides
	workspaceData := &ioworkspace.WorkspaceData{
		Workspace: mworkspace.Workspace{
			ID:   workspaceID,
			Name: "Test Workspace No Deltas",
		},
		Flows: []mflow.Flow{
			{
				ID:          flowID,
				WorkspaceID: workspaceID,
				Name:        "Test Flow",
			},
		},
		FlowNodes: []mnnode.MNode{
			{
				ID:       startNodeID,
				FlowID:   flowID,
				Name:     "Start",
				NodeKind: mnnode.NODE_KIND_NO_OP,
			},
			{
				ID:       requestNodeID,
				FlowID:   flowID,
				Name:     "get_user",
				NodeKind: mnnode.NODE_KIND_REQUEST,
			},
		},
		FlowNoopNodes: []mnnoop.NoopNode{
			{
				FlowNodeID: startNodeID,
				Type:       mnnoop.NODE_NO_OP_KIND_START,
			},
		},
		FlowRequestNodes: []mnrequest.MNRequest{
			{
				FlowNodeID: requestNodeID,
				EndpointID: &baseEndpointID,
				ExampleID:  &baseExampleID,
				// No delta IDs
			},
		},
		FlowEdges: []edge.Edge{
			{
				FlowID:   flowID,
				SourceID: startNodeID,
				TargetID: requestNodeID,
			},
		},
		// Base endpoint
		Endpoints: []mitemapi.ItemApi{
			{
				ID:           baseEndpointID,
				CollectionID: workspaceID,
				Name:         "Get User",
				Method:       "GET",
				Url:          "https://api.example.com/users/{{userId}}",
				Hidden:       false,
			},
		},
		// Base example
		Examples: []mitemapiexample.ItemApiExample{
			{
				ID:        baseExampleID,
				ItemApiID: baseEndpointID,
				Name:      "Example",
			},
		},
		// Base headers
		ExampleHeaders: []mexampleheader.Header{
			{
				ID:        idwrap.NewNow(),
				ExampleID: baseExampleID,
				HeaderKey: "Authorization",
				Value:     "Bearer {{token}}",
				Enable:    true,
			},
		},
	}

	// Export the workflow
	exported, err := ExportWorkflowClean(workspaceData)
	require.NoError(t, err)

	// Parse the exported YAML
	var result map[string]any
	err = yaml.Unmarshal(exported, &result)
	require.NoError(t, err)

	// Check flows section
	flows, ok := result["flows"].([]any)
	require.True(t, ok)
	require.Len(t, flows, 1)

	flow := flows[0].(map[string]any)
	steps, ok := flow["steps"].([]any)
	require.True(t, ok)
	require.Len(t, steps, 1)

	// Verify the request step has no overrides
	step := steps[0].(map[string]any)
	requestStep, ok := step["request"].(map[string]any)
	require.True(t, ok)
	
	assert.Equal(t, "get_user", requestStep["name"])
	assert.Equal(t, "get_user", requestStep["use_request"])
	
	// Should NOT have any override fields
	assert.NotContains(t, requestStep, "method")
	assert.NotContains(t, requestStep, "url")
	assert.NotContains(t, requestStep, "headers")
	assert.NotContains(t, requestStep, "query_params")
	assert.NotContains(t, requestStep, "body")
}

func TestExportWorkflowClean_PartialDeltaOverrides(t *testing.T) {
	// Create IDs
	workspaceID := idwrap.NewNow()
	flowID := idwrap.NewNow()
	
	// Node IDs
	startNodeID := idwrap.NewNow()
	requestNodeID := idwrap.NewNow()
	
	// Base endpoint and example
	baseEndpointID := idwrap.NewNow()
	baseExampleID := idwrap.NewNow()
	
	// Delta example only (no delta endpoint)
	deltaExampleID := idwrap.NewNow()

	// Create workspace data with only delta example (headers override)
	workspaceData := &ioworkspace.WorkspaceData{
		Workspace: mworkspace.Workspace{
			ID:   workspaceID,
			Name: "Test Workspace Partial Deltas",
		},
		Flows: []mflow.Flow{
			{
				ID:          flowID,
				WorkspaceID: workspaceID,
				Name:        "Test Flow",
			},
		},
		FlowNodes: []mnnode.MNode{
			{
				ID:       startNodeID,
				FlowID:   flowID,
				Name:     "Start",
				NodeKind: mnnode.NODE_KIND_NO_OP,
			},
			{
				ID:       requestNodeID,
				FlowID:   flowID,
				Name:     "api_call",
				NodeKind: mnnode.NODE_KIND_REQUEST,
			},
		},
		FlowNoopNodes: []mnnoop.NoopNode{
			{
				FlowNodeID: startNodeID,
				Type:       mnnoop.NODE_NO_OP_KIND_START,
			},
		},
		FlowRequestNodes: []mnrequest.MNRequest{
			{
				FlowNodeID:     requestNodeID,
				EndpointID:     &baseEndpointID,
				ExampleID:      &baseExampleID,
				DeltaExampleID: &deltaExampleID, // Only delta example, no delta endpoint
			},
		},
		FlowEdges: []edge.Edge{
			{
				FlowID:   flowID,
				SourceID: startNodeID,
				TargetID: requestNodeID,
			},
		},
		// Base endpoint
		Endpoints: []mitemapi.ItemApi{
			{
				ID:           baseEndpointID,
				CollectionID: workspaceID,
				Name:         "API Call",
				Method:       "POST",
				Url:          "https://api.example.com/data",
				Hidden:       false,
			},
		},
		// Base example
		Examples: []mitemapiexample.ItemApiExample{
			{
				ID:        baseExampleID,
				ItemApiID: baseEndpointID,
				Name:      "Base Example",
			},
			// Delta example
			{
				ID:        deltaExampleID,
				ItemApiID: baseEndpointID,
				Name:      "Delta Example",
			},
		},
		// Base headers
		ExampleHeaders: []mexampleheader.Header{
			{
				ID:        idwrap.NewNow(),
				ExampleID: baseExampleID,
				HeaderKey: "Content-Type",
				Value:     "application/json",
				Enable:    true,
			},
			// Delta headers - only add new header
			{
				ID:        idwrap.NewNow(),
				ExampleID: deltaExampleID,
				HeaderKey: "X-Custom-Header",
				Value:     "{{customValue}}",
				Enable:    true,
			},
		},
		// Base body
		Rawbodies: []mbodyraw.ExampleBodyRaw{
			{
				ID:        idwrap.NewNow(),
				ExampleID: baseExampleID,
				Data:      []byte(`{"action":"create"}`),
			},
			// Delta has same body (should not be included in override)
			{
				ID:        idwrap.NewNow(),
				ExampleID: deltaExampleID,
				Data:      []byte(`{"action":"create"}`),
			},
		},
	}

	// Export the workflow
	exported, err := ExportWorkflowClean(workspaceData)
	require.NoError(t, err)

	// Parse the exported YAML
	var result map[string]any
	err = yaml.Unmarshal(exported, &result)
	require.NoError(t, err)

	// Check flows section
	flows, ok := result["flows"].([]any)
	require.True(t, ok)
	require.Len(t, flows, 1)

	flow := flows[0].(map[string]any)
	steps, ok := flow["steps"].([]any)
	require.True(t, ok)
	require.Len(t, steps, 1)

	// Verify the request step with partial overrides
	step := steps[0].(map[string]any)
	requestStep, ok := step["request"].(map[string]any)
	require.True(t, ok)
	
	assert.Equal(t, "api_call", requestStep["name"])
	assert.Equal(t, "api_call", requestStep["use_request"])
	
	// Should NOT have method/url overrides (no delta endpoint)
	assert.NotContains(t, requestStep, "method")
	assert.NotContains(t, requestStep, "url")
	
	// Should have header override (only new header)
	stepHeaders, ok := requestStep["headers"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "{{customValue}}", stepHeaders["X-Custom-Header"])
	assert.NotContains(t, stepHeaders, "Content-Type") // Base header not overridden
	
	// Should NOT have body override (same as base)
	assert.NotContains(t, requestStep, "body")
}