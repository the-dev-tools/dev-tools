package workflowsimple

import (
	"testing"
	"gopkg.in/yaml.v3"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/ioworkspace"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/model/mworkspace"
)

func TestExportHARImportedRequestsWithDifferentHeaders(t *testing.T) {
	// Simulate HAR imported data where each request has different headers
	// login endpoint has no auth header, categories has auth header
	flowID := idwrap.NewNow()
	startNodeID := idwrap.NewNow()
	loginNodeID := idwrap.NewNow()
	categoriesNodeID := idwrap.NewNow()
	
	// Each HAR request creates its own endpoint (simulating duplicate endpoints)
	loginEndpointID := idwrap.NewNow()
	categoriesEndpointID := idwrap.NewNow()
	
	// Base examples with hardcoded values
	loginExampleID := idwrap.NewNow()
	categoriesExampleID := idwrap.NewNow()
	
	// Delta examples with variable references
	loginDeltaExampleID := idwrap.NewNow()
	categoriesDeltaExampleID := idwrap.NewNow()

	workspaceData := &ioworkspace.WorkspaceData{
		Workspace: mworkspace.Workspace{
			ID:   idwrap.NewNow(),
			Name: "HAR Import Test",
		},
		Flows: []mflow.Flow{
			{
				ID:   flowID,
				Name: "HAR Flow",
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
				ID:       loginNodeID,
				FlowID:   flowID,
				Name:     "request_0", // HAR import naming
				NodeKind: mnnode.NODE_KIND_REQUEST,
			},
			{
				ID:       categoriesNodeID,
				FlowID:   flowID,
				Name:     "request_1", // HAR import naming
				NodeKind: mnnode.NODE_KIND_REQUEST,
			},
		},
		FlowEdges: []edge.Edge{
			{
				ID:            idwrap.NewNow(),
				FlowID:        flowID,
				SourceID:      startNodeID,
				TargetID:      loginNodeID,
				SourceHandler: edge.HandleUnspecified,
			},
			{
				ID:            idwrap.NewNow(),
				FlowID:        flowID,
				SourceID:      loginNodeID,
				TargetID:      categoriesNodeID,
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
				FlowNodeID:     loginNodeID,
				EndpointID:     &loginEndpointID,
				ExampleID:      &loginExampleID,
				DeltaExampleID: &loginDeltaExampleID,
			},
			{
				FlowNodeID:     categoriesNodeID,
				EndpointID:     &categoriesEndpointID,
				ExampleID:      &categoriesExampleID,
				DeltaExampleID: &categoriesDeltaExampleID,
			},
		},
		Endpoints: []mitemapi.ItemApi{
			{
				ID:     loginEndpointID,
				Name:   "login",
				Url:    "https://api.example.com/auth/login",
				Method: "POST",
			},
			{
				ID:     categoriesEndpointID,
				Name:   "categories",
				Url:    "https://api.example.com/api/categories",
				Method: "GET",
			},
		},
		Examples: []mitemapiexample.ItemApiExample{
			{
				ID:        loginExampleID,
				Name:      "login",
				ItemApiID: loginEndpointID,
			},
			{
				ID:        categoriesExampleID,
				Name:      "categories",
				ItemApiID: categoriesEndpointID,
			},
			{
				ID:        loginDeltaExampleID,
				Name:      "login (Delta)",
				ItemApiID: loginEndpointID,
				VersionParentID: &loginExampleID,
			},
			{
				ID:        categoriesDeltaExampleID,
				Name:      "categories (Delta)",
				ItemApiID: categoriesEndpointID,
				VersionParentID: &categoriesExampleID,
			},
		},
		ExampleHeaders: []mexampleheader.Header{
			// Login base example headers (no auth)
			{
				ID:        idwrap.NewNow(),
				ExampleID: loginExampleID,
				HeaderKey: "Content-Type",
				Value:     "application/json",
				Enable:    true,
			},
			{
				ID:        idwrap.NewNow(),
				ExampleID: loginExampleID,
				HeaderKey: "Accept",
				Value:     "*/*",
				Enable:    true,
			},
			// Categories base example headers (with hardcoded auth and all browser headers)
			{
				ID:        idwrap.NewNow(),
				ExampleID: categoriesExampleID,
				HeaderKey: "Content-Type",
				Value:     "application/json",
				Enable:    true,
			},
			{
				ID:        idwrap.NewNow(),
				ExampleID: categoriesExampleID,
				HeaderKey: "Accept",
				Value:     "*/*",
				Enable:    true,
			},
			{
				ID:        idwrap.NewNow(),
				ExampleID: categoriesExampleID,
				HeaderKey: "Accept-Encoding",
				Value:     "gzip, deflate, br, zstd",
				Enable:    true,
			},
			{
				ID:        idwrap.NewNow(),
				ExampleID: categoriesExampleID,
				HeaderKey: "Accept-Language",
				Value:     "en-US,en;q=0.9",
				Enable:    true,
			},
			{
				ID:        idwrap.NewNow(),
				ExampleID: categoriesExampleID,
				HeaderKey: "Authorization",
				Value:     "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6IjU5MmFiNzc0LWU3ODMtNDQ5NS05Yjc3LWFkODU2ODlmODRkNyIsImlhdCI6MTc0ODQzODkyMSwiZXhwIjoxNzQ4NTI1MzIxfQ.nRJ8x6ItgC8aOXj8P8jonmjwwOgs2lVTCOd7-KbYlxQ",
				Enable:    true,
			},
			{
				ID:        idwrap.NewNow(),
				ExampleID: categoriesExampleID,
				HeaderKey: "User-Agent",
				Value:     "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36",
				Enable:    true,
			},
			// Login delta headers (same as base)
			{
				ID:        idwrap.NewNow(),
				ExampleID: loginDeltaExampleID,
				HeaderKey: "Content-Type",
				Value:     "application/json",
				Enable:    true,
			},
			{
				ID:        idwrap.NewNow(),
				ExampleID: loginDeltaExampleID,
				HeaderKey: "Accept",
				Value:     "*/*",
				Enable:    true,
			},
			// Categories delta headers (with variable reference)
			{
				ID:        idwrap.NewNow(),
				ExampleID: categoriesDeltaExampleID,
				HeaderKey: "Content-Type",
				Value:     "application/json",
				Enable:    true,
			},
			{
				ID:        idwrap.NewNow(),
				ExampleID: categoriesDeltaExampleID,
				HeaderKey: "Accept",
				Value:     "*/*",
				Enable:    true,
			},
			{
				ID:        idwrap.NewNow(),
				ExampleID: categoriesDeltaExampleID,
				HeaderKey: "Authorization",
				Value:     "Bearer {{ request_0.response.body.token }}",
				Enable:    true,
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
	if !ok {
		t.Fatal("requests section not found or not an array")
	}

	if len(requests) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(requests))
	}

	// Find login and categories requests
	var loginRequest, categoriesRequest map[string]any
	for _, req := range requests {
		reqMap, ok := req.(map[string]any)
		if !ok {
			continue
		}
		name, _ := reqMap["name"].(string)
		switch name {
		case "request_0":
			loginRequest = reqMap
		case "request_1":
			categoriesRequest = reqMap
		}
	}

	if loginRequest == nil {
		t.Fatal("login request (request_0) not found")
	}
	if categoriesRequest == nil {
		t.Fatal("categories request (request_1) not found")
	}

	// Verify login has no Authorization header
	loginHeaders, ok := loginRequest["headers"].(map[string]any)
	if !ok {
		t.Fatal("login headers not found or not a map")
	}
	if _, hasAuth := loginHeaders["Authorization"]; hasAuth {
		t.Error("login request should NOT have Authorization header")
	}
	if loginHeaders["Content-Type"] != "application/json" {
		t.Error("login request should have Content-Type header")
	}

	// Verify categories has all headers including Authorization with variable reference
	categoriesHeaders, ok := categoriesRequest["headers"].(map[string]any)
	if !ok {
		t.Fatal("categories headers not found or not a map")
	}
	
	// Check Authorization header has hardcoded value (not variable reference)
	authHeader, hasAuth := categoriesHeaders["Authorization"].(string)
	if !hasAuth {
		t.Error("categories request should have Authorization header")
	}
	// Should have the hardcoded token from the base example
	expectedAuthToken := "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6IjU5MmFiNzc0LWU3ODMtNDQ5NS05Yjc3LWFkODU2ODlmODRkNyIsImlhdCI6MTc0ODQzODkyMSwiZXhwIjoxNzQ4NTI1MzIxfQ.nRJ8x6ItgC8aOXj8P8jonmjwwOgs2lVTCOd7-KbYlxQ"
	if authHeader != expectedAuthToken {
		t.Errorf("categories Authorization header should have hardcoded value, got: %s", authHeader)
	}
	
	// Check other important headers are present
	expectedHeaders := map[string]string{
		"Content-Type": "application/json",
		"Accept": "*/*",
		"Accept-Encoding": "gzip, deflate, br, zstd",
		"Accept-Language": "en-US,en;q=0.9",
		"User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36",
	}
	
	for key, expectedValue := range expectedHeaders {
		if value, exists := categoriesHeaders[key]; !exists {
			t.Errorf("categories request missing header: %s", key)
		} else if value != expectedValue {
			t.Errorf("categories header %s: expected '%s', got '%v'", key, expectedValue, value)
		}
	}
	
	// Verify we have more than just a few headers
	if len(categoriesHeaders) < 5 {
		t.Errorf("categories request should have many headers, got only %d", len(categoriesHeaders))
	}

	// Verify flows section
	flows, ok := exported["flows"].([]any)
	if !ok || len(flows) != 1 {
		t.Fatal("flows section not found or incorrect")
	}

	flow := flows[0].(map[string]any)
	steps, ok := flow["steps"].([]any)
	if !ok || len(steps) != 2 {
		t.Fatal("flow steps not found or incorrect count")
	}

	// Verify each step references its own request
	for i, step := range steps {
		stepMap := step.(map[string]any)
		reqStep, ok := stepMap["request"].(map[string]any)
		if !ok {
			t.Errorf("step %d is not a request", i)
			continue
		}
		
		expectedName := "request_" + string(rune('0' + i))
		if reqStep["name"] != expectedName {
			t.Errorf("step %d name mismatch: expected %s, got %v", i, expectedName, reqStep["name"])
		}
		if reqStep["use_request"] != expectedName {
			t.Errorf("step %d use_request mismatch: expected %s, got %v", i, expectedName, reqStep["use_request"])
		}
		
		// Step 1 (categories) should have headers override with variable reference
		// Step 0 (login) should not have any overrides
		if i == 1 {
			// request_1 (categories) should have header override
			if headers, hasHeaders := reqStep["headers"].(map[string]any); hasHeaders {
				if authOverride, hasAuth := headers["Authorization"].(string); hasAuth {
					if authOverride != "Bearer {{ request_0.response.body.token }}" {
						t.Errorf("step %d Authorization override should have variable reference, got: %s", i, authOverride)
					}
				} else {
					t.Errorf("step %d should have Authorization header override", i)
				}
			} else {
				t.Errorf("step %d should have headers field with Authorization override", i)
			}
		} else {
			// request_0 (login) should not have any overrides
			if _, hasHeaders := reqStep["headers"]; hasHeaders {
				t.Errorf("step %d should not have headers field", i)
			}
			if _, hasBody := reqStep["body"]; hasBody {
				t.Errorf("step %d should not have body field", i)
			}
			if _, hasQuery := reqStep["query_params"]; hasQuery {
				t.Errorf("step %d should not have query_params field", i)
			}
		}
	}
}