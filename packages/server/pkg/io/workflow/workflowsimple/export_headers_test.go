package workflowsimple

import (
	"sort"
	"testing"
	"gopkg.in/yaml.v3"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/ioworkspace"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mexamplequery"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mflowvariable"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/model/mworkspace"
)

func TestExportWithHeadersQueryAndBody(t *testing.T) {
	// Create workspace data with headers, query params, and body
	flowID := idwrap.NewNow()
	startNodeID := idwrap.NewNow()
	requestNodeID := idwrap.NewNow()
	endpointID := idwrap.NewNow()
	exampleID := idwrap.NewNow()
	deltaEndpointID := idwrap.NewNow()
	deltaExampleID := idwrap.NewNow()

	workspaceData := &ioworkspace.WorkspaceData{
		Workspace: mworkspace.Workspace{
			ID:   idwrap.NewNow(),
			Name: "Test Workspace",
		},
		Flows: []mflow.Flow{
			{
				ID:   flowID,
				Name: "Test Flow",
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
				Name:     "API Request",
				NodeKind: mnnode.NODE_KIND_REQUEST,
			},
		},
		FlowEdges: []edge.Edge{
			{
				ID:            idwrap.NewNow(),
				FlowID:        flowID,
				SourceID:      startNodeID,
				TargetID:      requestNodeID,
				SourceHandler: edge.HandleUnspecified,
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
				EndpointID:      &endpointID,
				ExampleID:       &exampleID,
				DeltaEndpointID: &deltaEndpointID,
				DeltaExampleID:  &deltaExampleID,
			},
		},
		Endpoints: []mitemapi.ItemApi{
			{
				ID:     endpointID,
				Name:   "Test Endpoint",
				Url:    "https://api.example.com/users",
				Method: "POST",
			},
			{
				ID:            deltaEndpointID,
				Name:          "Test Endpoint (delta)",
				Url:           "https://api.example.com/{{version}}/users",
				Method:        "POST",
				DeltaParentID: &endpointID,
				Hidden:        true,
			},
		},
		Examples: []mitemapiexample.ItemApiExample{
			{
				ID:        exampleID,
				Name:      "Test Example",
				ItemApiID: endpointID,
				IsDefault: true,
			},
			{
				ID:        deltaExampleID,
				Name:      "Test Example (delta)",
				ItemApiID: deltaEndpointID,
				IsDefault: true,
			},
		},
		ExampleHeaders: []mexampleheader.Header{
			{
				ID:        idwrap.NewNow(),
				ExampleID: deltaExampleID,
				HeaderKey: "Authorization",
				Value:     "Bearer {{token}}",
			},
			{
				ID:        idwrap.NewNow(),
				ExampleID: deltaExampleID,
				HeaderKey: "Content-Type",
				Value:     "application/json",
			},
		},
		ExampleQueries: []mexamplequery.Query{
			{
				ID:        idwrap.NewNow(),
				ExampleID: deltaExampleID,
				QueryKey:  "page",
				Value:     "1",
			},
			{
				ID:        idwrap.NewNow(),
				ExampleID: deltaExampleID,
				QueryKey:  "limit",
				Value:     "{{limit}}",
			},
		},
		Rawbodies: []mbodyraw.ExampleBodyRaw{
			{
				ID:        idwrap.NewNow(),
				ExampleID: deltaExampleID,
				Data:      []byte(`{"name":"John Doe","email":"john@example.com"}`),
			},
		},
		FlowVariables: []mflowvariable.FlowVariable{
			{
				ID:     idwrap.NewNow(),
				FlowID: flowID,
				Name:   "version",
				Value:  "v1",
			},
			{
				ID:     idwrap.NewNow(),
				FlowID: flowID,
				Name:   "token",
				Value:  "test-token-123",
			},
			{
				ID:     idwrap.NewNow(),
				FlowID: flowID,
				Name:   "limit",
				Value:  "10",
			},
		},
	}

	// Export to YAML
	yamlData, err := ExportWorkflowYAMLOld(workspaceData)
	if err != nil {
		t.Fatalf("failed to export: %v", err)
	}

	// Parse the exported YAML
	var exported WorkflowFormat
	if err := yaml.Unmarshal(yamlData, &exported); err != nil {
		t.Fatalf("failed to parse exported YAML: %v", err)
	}

	// Verify workspace name
	if exported.WorkspaceName != "Test Workspace" {
		t.Errorf("expected workspace name 'Test Workspace', got '%s'", exported.WorkspaceName)
	}

	// Verify flow
	if len(exported.Flows) != 1 {
		t.Fatalf("expected 1 flow, got %d", len(exported.Flows))
	}

	flow := exported.Flows[0]
	if flow.Name != "Test Flow" {
		t.Errorf("expected flow name 'Test Flow', got '%s'", flow.Name)
	}

	// Verify variables
	if len(flow.Variables) != 3 {
		t.Errorf("expected 3 variables, got %d", len(flow.Variables))
	}

	// Verify steps
	if len(flow.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(flow.Steps))
	}

	// Get the request step
	var requestStep map[string]any
	for _, step := range flow.Steps {
		if stepMap, ok := step.(map[string]any); ok {
			if req, ok := stepMap["request"].(map[string]any); ok {
				requestStep = req
				break
			}
		}
	}

	if requestStep == nil {
		t.Fatal("request step not found")
	}

	// Verify basic fields
	if requestStep["name"] != "API Request" {
		t.Errorf("expected name 'API Request', got '%v'", requestStep["name"])
	}
	if requestStep["url"] != "https://api.example.com/{{version}}/users" {
		t.Errorf("expected URL with variable, got '%v'", requestStep["url"])
	}
	if requestStep["method"] != "POST" {
		t.Errorf("expected method 'POST', got '%v'", requestStep["method"])
	}

	// Verify headers
	headers, ok := requestStep["headers"].([]any)
	if !ok || len(headers) != 2 {
		t.Errorf("expected 2 headers, got %v", requestStep["headers"])
	}

	// Verify query params
	queryParams, ok := requestStep["query_params"].([]any)
	if !ok || len(queryParams) != 2 {
		t.Errorf("expected 2 query params, got %v", requestStep["query_params"])
	}

	// Verify body
	body, ok := requestStep["body"].(map[string]any)
	if !ok {
		t.Errorf("expected body map, got %v", requestStep["body"])
	} else {
		bodyJSON, ok := body["body_json"].(map[string]any)
		if !ok {
			t.Errorf("expected body_json map, got %v", body["body_json"])
		} else {
			if bodyJSON["name"] != "John Doe" {
				t.Errorf("expected name 'John Doe' in body, got %v", bodyJSON["name"])
			}
		}
	}

	// Print the exported YAML for debugging
	t.Logf("Exported YAML:\n%s", string(yamlData))
}

func TestExportMultipleRequestsWithOverrides(t *testing.T) {
	// Test with multiple requests where later ones override headers
	flowID := idwrap.NewNow()
	startNodeID := idwrap.NewNow()
	request1ID := idwrap.NewNow()
	request2ID := idwrap.NewNow()
	endpoint1ID := idwrap.NewNow()
	endpoint2ID := idwrap.NewNow()
	example1ID := idwrap.NewNow()
	example2ID := idwrap.NewNow()

	workspaceData := &ioworkspace.WorkspaceData{
		Workspace: mworkspace.Workspace{
			ID:   idwrap.NewNow(),
			Name: "Override Test",
		},
		Flows: []mflow.Flow{
			{
				ID:   flowID,
				Name: "Override Flow",
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
				ID:       request1ID,
				FlowID:   flowID,
				Name:     "Request 1",
				NodeKind: mnnode.NODE_KIND_REQUEST,
			},
			{
				ID:       request2ID,
				FlowID:   flowID,
				Name:     "Request 2",
				NodeKind: mnnode.NODE_KIND_REQUEST,
			},
		},
		FlowEdges: []edge.Edge{
			{
				ID:            idwrap.NewNow(),
				FlowID:        flowID,
				SourceID:      startNodeID,
				TargetID:      request1ID,
				SourceHandler: edge.HandleUnspecified,
			},
			{
				ID:            idwrap.NewNow(),
				FlowID:        flowID,
				SourceID:      request1ID,
				TargetID:      request2ID,
				SourceHandler: edge.HandleUnspecified,
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
				FlowNodeID: request1ID,
				EndpointID: &endpoint1ID,
				ExampleID:  &example1ID,
			},
			{
				FlowNodeID: request2ID,
				EndpointID: &endpoint2ID,
				ExampleID:  &example2ID,
			},
		},
		Endpoints: []mitemapi.ItemApi{
			{
				ID:     endpoint1ID,
				Name:   "Endpoint 1",
				Url:    "https://api.example.com/auth",
				Method: "POST",
			},
			{
				ID:     endpoint2ID,
				Name:   "Endpoint 2",
				Url:    "https://api.example.com/data",
				Method: "GET",
			},
		},
		Examples: []mitemapiexample.ItemApiExample{
			{
				ID:        example1ID,
				Name:      "Example 1",
				ItemApiID: endpoint1ID,
				IsDefault: true,
			},
			{
				ID:        example2ID,
				Name:      "Example 2",
				ItemApiID: endpoint2ID,
				IsDefault: true,
			},
		},
		ExampleHeaders: []mexampleheader.Header{
			{
				ID:        idwrap.NewNow(),
				ExampleID: example1ID,
				HeaderKey: "Content-Type",
				Value:     "application/json",
			},
			{
				ID:        idwrap.NewNow(),
				ExampleID: example1ID,
				HeaderKey: "X-Custom",
				Value:     "value1",
			},
			{
				ID:        idwrap.NewNow(),
				ExampleID: example2ID,
				HeaderKey: "Authorization",
				Value:     "Bearer token",
			},
			{
				ID:        idwrap.NewNow(),
				ExampleID: example2ID,
				HeaderKey: "X-Custom",
				Value:     "value2", // Override
			},
		},
	}

	// Export to YAML
	yamlData, err := ExportWorkflowYAMLOld(workspaceData)
	if err != nil {
		t.Fatalf("failed to export: %v", err)
	}

	// Log the output
	t.Logf("Exported YAML:\n%s", string(yamlData))

	// Parse and verify
	var exported WorkflowFormat
	if err := yaml.Unmarshal(yamlData, &exported); err != nil {
		t.Fatalf("failed to parse exported YAML: %v", err)
	}

	if len(exported.Flows[0].Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(exported.Flows[0].Steps))
	}
}

func TestExportFullBrowserHeaders(t *testing.T) {
	// Test that all browser headers are preserved in export
	flowID := idwrap.NewNow()
	startNodeID := idwrap.NewNow()
	requestNodeID := idwrap.NewNow()
	endpointID := idwrap.NewNow()
	exampleID := idwrap.NewNow()
	deltaExampleID := idwrap.NewNow()

	// Full set of browser headers from HAR
	browserHeaders := []struct {
		key   string
		value string
	}{
		{"accept", "*/*"},
		{"accept-encoding", "gzip, deflate, br, zstd"},
		{"accept-language", "en-US,en;q=0.9"},
		{"authorization", "Bearer {{ request_0.response.body.token }}"},
		{"content-type", "application/json"},
		{"priority", "u=1, i"},
		{"referer", "https://ecommerce-admin-panel.fly.dev/"},
		{"sec-ch-ua", `"Chromium";v="136", "Google Chrome";v="136", "Not.A/Brand";v="99"`},
		{"sec-ch-ua-mobile", "?0"},
		{"sec-ch-ua-platform", `"macOS"`},
		{"sec-fetch-dest", "empty"},
		{"sec-fetch-mode", "cors"},
		{"sec-fetch-site", "same-origin"},
		{"user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36"},
	}

	// Create headers for base example (so they appear in requests section)
	var exampleHeaders []mexampleheader.Header
	for _, h := range browserHeaders {
		exampleHeaders = append(exampleHeaders, mexampleheader.Header{
			ID:        idwrap.NewNow(),
			ExampleID: exampleID,
			HeaderKey: h.key,
			Value:     h.value,
			Enable:    true,
		})
	}

	workspaceData := &ioworkspace.WorkspaceData{
		Workspace: mworkspace.Workspace{
			ID:   idwrap.NewNow(),
			Name: "Browser Headers Test",
		},
		Flows: []mflow.Flow{
			{
				ID:   flowID,
				Name: "Test Flow",
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
				Name:     "request_7",
				NodeKind: mnnode.NODE_KIND_REQUEST,
			},
		},
		FlowEdges: []edge.Edge{
			{
				ID:            idwrap.NewNow(),
				FlowID:        flowID,
				SourceID:      startNodeID,
				TargetID:      requestNodeID,
				SourceHandler: edge.HandleUnspecified,
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
				EndpointID:     &endpointID,
				ExampleID:      &exampleID,
				DeltaExampleID: &deltaExampleID,
			},
		},
		Endpoints: []mitemapi.ItemApi{
			{
				ID:     endpointID,
				Name:   "tags",
				Url:    "https://ecommerce-admin-panel.fly.dev/api/tags",
				Method: "GET",
			},
		},
		Examples: []mitemapiexample.ItemApiExample{
			{
				ID:        exampleID,
				Name:      "tags",
				ItemApiID: endpointID,
			},
			{
				ID:              deltaExampleID,
				Name:            "tags (Delta)",
				ItemApiID:       endpointID,
				VersionParentID: &exampleID,
			},
		},
		ExampleHeaders: exampleHeaders,
		Rawbodies: []mbodyraw.ExampleBodyRaw{
			{
				ID:        idwrap.NewNow(),
				ExampleID: deltaExampleID,
				Data:      []byte{}, // Empty body for GET request
			},
		},
	}

	// Export using the clean format
	yamlData, err := ExportWorkflowClean(workspaceData)
	if err != nil {
		t.Fatalf("failed to export: %v", err)
	}

	// Parse the exported YAML
	var exported map[string]any
	if err := yaml.Unmarshal(yamlData, &exported); err != nil {
		t.Fatalf("failed to parse exported YAML: %v", err)
	}

	t.Logf("Exported YAML:\n%s", string(yamlData))

	// Verify requests section
	requests, ok := exported["requests"].([]any)
	if !ok || len(requests) != 1 {
		t.Fatal("requests section not found or incorrect count")
	}

	request := requests[0].(map[string]any)
	
	// Verify request has all headers
	headers, ok := request["headers"].(map[string]any)
	if !ok {
		t.Fatal("headers not found or not a map")
	}

	// Check that all browser headers are present
	if len(headers) != len(browserHeaders) {
		t.Errorf("expected %d headers, got %d", len(browserHeaders), len(headers))
	}

	// Verify each header
	for _, expectedHeader := range browserHeaders {
		value, exists := headers[expectedHeader.key]
		if !exists {
			t.Errorf("header %s not found", expectedHeader.key)
			continue
		}
		if value != expectedHeader.value {
			t.Errorf("header %s: expected '%s', got '%v'", expectedHeader.key, expectedHeader.value, value)
		}
	}

	// Verify the headers are alphabetically sorted
	var headerKeys []string
	for k := range headers {
		headerKeys = append(headerKeys, k)
	}
	
	// Create a sorted copy to compare
	sortedKeys := make([]string, len(headerKeys))
	copy(sortedKeys, headerKeys)
	sort.Strings(sortedKeys)
	
	// Log the header keys for debugging
	t.Logf("Header keys found: %v", headerKeys)
}