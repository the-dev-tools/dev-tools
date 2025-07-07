package workflowsimple_test

import (
	"encoding/json"
	"strings"
	"testing"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/io/workflow/workflowsimple"
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

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestExportCleanPreservesVariables(t *testing.T) {
	// Test that export uses delta examples to preserve variable placeholders
	
	// Create workspace data
	workspaceData := &ioworkspace.WorkspaceData{
		Workspace: mworkspace.Workspace{
			ID:   idwrap.NewNow(),
			Name: "Variable Preservation Test",
		},
		Flows: []mflow.Flow{
			{
				ID:   idwrap.NewNow(),
				Name: "TestFlow",
			},
		},
	}
	
	flowID := workspaceData.Flows[0].ID
	
	// Create nodes
	startNodeID := idwrap.NewNow()
	requestNodeID := idwrap.NewNow()
	
	workspaceData.FlowNodes = []mnnode.MNode{
		{
			ID:       startNodeID,
			FlowID:   flowID,
			Name:     "Start",
			NodeKind: mnnode.NODE_KIND_NO_OP,
		},
		{
			ID:       requestNodeID,
			FlowID:   flowID,
			Name:     "TestRequest",
			NodeKind: mnnode.NODE_KIND_REQUEST,
		},
	}
	
	// Add noop node
	workspaceData.FlowNoopNodes = []mnnoop.NoopNode{
		{
			FlowNodeID: startNodeID,
			Type:       mnnoop.NODE_NO_OP_KIND_START,
		},
	}
	
	// Create endpoint
	endpointID := idwrap.NewNow()
	deltaEndpointID := idwrap.NewNow()
	workspaceData.Endpoints = []mitemapi.ItemApi{
		{
			ID:     endpointID,
			Name:   "TestEndpoint",
			Url:    "https://api.example.com/users",
			Method: "POST",
		},
		{
			ID:            deltaEndpointID,
			Name:          "TestEndpoint (delta)",
			Url:           "https://{{base_url}}/users", // Original URL with variable
			Method:        "POST",
			DeltaParentID: &endpointID,
			Hidden:        true,
		},
	}
	
	// Create examples
	baseExampleID := idwrap.NewNow()
	defaultExampleID := idwrap.NewNow()
	deltaExampleID := idwrap.NewNow()
	
	workspaceData.Examples = []mitemapiexample.ItemApiExample{
		{
			ID:        baseExampleID,
			Name:      "TestExample",
			ItemApiID: endpointID,
			IsDefault: true,
		},
		{
			ID:        defaultExampleID,
			Name:      "TestExample (default)",
			ItemApiID: endpointID,
			IsDefault: false,
		},
		{
			ID:        deltaExampleID,
			Name:      "TestExample (delta)",
			ItemApiID: deltaEndpointID,
			IsDefault: true,
		},
	}
	
	// Create headers - base has variable, default has resolved value, delta preserves variable
	baseHeaderID := idwrap.NewNow()
	defaultHeaderID := idwrap.NewNow()
	deltaHeaderID := idwrap.NewNow()
	
	workspaceData.ExampleHeaders = []mexampleheader.Header{
		{
			ID:        baseHeaderID,
			ExampleID: baseExampleID,
			HeaderKey: "Authorization",
			Value:     "Bearer {{token}}",
			Enable:    true,
		},
		{
			ID:        defaultHeaderID,
			ExampleID: defaultExampleID,
			HeaderKey: "Authorization",
			Value:     "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...", // Resolved value
			Enable:    true,
		},
		{
			ID:            deltaHeaderID,
			ExampleID:     deltaExampleID,
			HeaderKey:     "Authorization",
			Value:         "Bearer {{token}}", // Preserved variable
			DeltaParentID: &defaultHeaderID,
			Enable:        true,
		},
	}
	
	// Create query parameters
	baseQueryID := idwrap.NewNow()
	defaultQueryID := idwrap.NewNow()
	deltaQueryID := idwrap.NewNow()
	
	workspaceData.ExampleQueries = []mexamplequery.Query{
		{
			ID:        baseQueryID,
			ExampleID: baseExampleID,
			QueryKey:  "apiKey",
			Value:     "{{api_key}}",
			Enable:    true,
		},
		{
			ID:        defaultQueryID,
			ExampleID: defaultExampleID,
			QueryKey:  "apiKey",
			Value:     "sk-1234567890abcdef", // Resolved value
			Enable:    true,
		},
		{
			ID:            deltaQueryID,
			ExampleID:     deltaExampleID,
			QueryKey:      "apiKey",
			Value:         "{{api_key}}", // Preserved variable
			DeltaParentID: &defaultQueryID,
			Enable:        true,
		},
	}
	
	// Create body with variables
	bodyData := map[string]any{
		"username": "{{username}}",
		"email":    "{{email}}",
	}
	bodyJSON, _ := json.Marshal(bodyData)
	
	workspaceData.Rawbodies = []mbodyraw.ExampleBodyRaw{
		{
			ID:        idwrap.NewNow(),
			ExampleID: baseExampleID,
			Data:      bodyJSON,
		},
		{
			ID:        idwrap.NewNow(),
			ExampleID: defaultExampleID,
			Data:      bodyJSON, // In real scenario, this might have resolved values
		},
		{
			ID:        idwrap.NewNow(),
			ExampleID: deltaExampleID,
			Data:      bodyJSON, // Delta preserves variables
		},
	}
	
	// Create request node
	workspaceData.FlowRequestNodes = []mnrequest.MNRequest{
		{
			FlowNodeID:      requestNodeID,
			EndpointID:      &endpointID,
			ExampleID:       &baseExampleID,
			DeltaEndpointID: &deltaEndpointID,
			DeltaExampleID:  &deltaExampleID,
		},
	}
	
	// Create flow variables
	workspaceData.FlowVariables = []mflowvariable.FlowVariable{
		{
			ID:      idwrap.NewNow(),
			FlowID:  flowID,
			Name:    "base_url",
			Value:   "api.example.com",
			Enabled: true,
		},
		{
			ID:      idwrap.NewNow(),
			FlowID:  flowID,
			Name:    "token",
			Value:   "test-token-123",
			Enabled: true,
		},
		{
			ID:      idwrap.NewNow(),
			FlowID:  flowID,
			Name:    "api_key",
			Value:   "test-key",
			Enabled: true,
		},
		{
			ID:      idwrap.NewNow(),
			FlowID:  flowID,
			Name:    "username",
			Value:   "testuser",
			Enabled: true,
		},
		{
			ID:      idwrap.NewNow(),
			FlowID:  flowID,
			Name:    "email",
			Value:   "test@example.com",
			Enabled: true,
		},
	}
	
	// Export using clean format
	exported, err := workflowsimple.ExportWorkflowClean(workspaceData)
	require.NoError(t, err)
	
	// Parse the exported YAML
	var exportedData map[string]any
	err = yaml.Unmarshal(exported, &exportedData)
	require.NoError(t, err)
	
	// Check that requests section exists
	requests, ok := exportedData["requests"].([]any)
	require.True(t, ok, "Expected 'requests' section in export")
	require.Len(t, requests, 1)
	
	// Get the first request
	request := requests[0].(map[string]any)
	
	// Verify URL has variable placeholder (from delta endpoint)
	// Since delta endpoint has the variable URL, this shouldn't be used in global requests
	// But the endpoint used should be the base one
	require.Equal(t, "https://api.example.com/users", request["url"])
	
	// Verify headers have variable placeholders (from delta examples)
	headers, ok := request["headers"].(map[string]any)
	require.True(t, ok, "Headers should be a map")
	require.Len(t, headers, 1)
	
	authValue, ok := headers["Authorization"].(string)
	require.True(t, ok, "Authorization header should exist")
	require.Equal(t, "Bearer {{token}}", authValue, "Header should preserve variable placeholder")
	
	// Verify query params have variable placeholders
	queryParams, ok := request["query_params"].(map[string]any)
	require.True(t, ok, "Query params should be a map")
	require.Len(t, queryParams, 1)
	
	apiKeyValue, ok := queryParams["apiKey"].(string)
	require.True(t, ok, "apiKey query param should exist")
	require.Equal(t, "{{api_key}}", apiKeyValue, "Query param should preserve variable placeholder")
	
	// Verify body has variable placeholders
	body, ok := request["body"].(map[string]any)
	require.True(t, ok, "Body should be a map")
	require.Equal(t, "{{username}}", body["username"], "Body should preserve username variable")
	require.Equal(t, "{{email}}", body["email"], "Body should preserve email variable")
	
	// Verify the exported YAML string contains variable placeholders
	exportedStr := string(exported)
	require.True(t, strings.Contains(exportedStr, "Bearer {{token}}"), "Export should contain token variable")
	require.True(t, strings.Contains(exportedStr, "{{api_key}}"), "Export should contain api_key variable")
	require.True(t, strings.Contains(exportedStr, "{{username}}"), "Export should contain username variable")
	require.True(t, strings.Contains(exportedStr, "{{email}}"), "Export should contain email variable")
	
	// Ensure resolved values are NOT in the export
	require.False(t, strings.Contains(exportedStr, "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"), "Export should not contain resolved JWT token")
	require.False(t, strings.Contains(exportedStr, "sk-1234567890abcdef"), "Export should not contain resolved API key")
}

func TestExportCleanWithMultipleRequests(t *testing.T) {
	// Test export with multiple requests that share common patterns
	
	workspaceData := &ioworkspace.WorkspaceData{
		Workspace: mworkspace.Workspace{
			ID:   idwrap.NewNow(),
			Name: "Multi Request Test",
		},
		Flows: []mflow.Flow{
			{
				ID:   idwrap.NewNow(),
				Name: "MultiFlow",
			},
		},
	}
	
	flowID := workspaceData.Flows[0].ID
	
	// Create start node
	startNodeID := idwrap.NewNow()
	workspaceData.FlowNodes = []mnnode.MNode{
		{
			ID:       startNodeID,
			FlowID:   flowID,
			Name:     "Start",
			NodeKind: mnnode.NODE_KIND_NO_OP,
		},
	}
	
	workspaceData.FlowNoopNodes = []mnnoop.NoopNode{
		{
			FlowNodeID: startNodeID,
			Type:       mnnoop.NODE_NO_OP_KIND_START,
		},
	}
	
	// Create two request nodes
	for i := 1; i <= 2; i++ {
		nodeID := idwrap.NewNow()
		endpointID := idwrap.NewNow()
		exampleID := idwrap.NewNow()
		
		workspaceData.FlowNodes = append(workspaceData.FlowNodes, mnnode.MNode{
			ID:       nodeID,
			FlowID:   flowID,
			Name:     "Request" + string(rune('0'+i)),
			NodeKind: mnnode.NODE_KIND_REQUEST,
		})
		
		workspaceData.Endpoints = append(workspaceData.Endpoints, mitemapi.ItemApi{
			ID:     endpointID,
			Name:   "Endpoint" + string(rune('0'+i)),
			Url:    "https://api.example.com/endpoint" + string(rune('0'+i)),
			Method: "GET",
		})
		
		workspaceData.Examples = append(workspaceData.Examples, mitemapiexample.ItemApiExample{
			ID:        exampleID,
			Name:      "Example" + string(rune('0'+i)),
			ItemApiID: endpointID,
			IsDefault: true,
		})
		
		workspaceData.FlowRequestNodes = append(workspaceData.FlowRequestNodes, mnrequest.MNRequest{
			FlowNodeID: nodeID,
			EndpointID: &endpointID,
			ExampleID:  &exampleID,
		})
		
		// Add common header
		workspaceData.ExampleHeaders = append(workspaceData.ExampleHeaders, mexampleheader.Header{
			ID:        idwrap.NewNow(),
			ExampleID: exampleID,
			HeaderKey: "Authorization",
			Value:     "Bearer {{token}}",
			Enable:    true,
		})
	}
	
	// Export
	exported, err := workflowsimple.ExportWorkflowClean(workspaceData)
	require.NoError(t, err)
	
	// Parse and verify
	var exportedData map[string]any
	err = yaml.Unmarshal(exported, &exportedData)
	require.NoError(t, err)
	
	requests, ok := exportedData["requests"].([]any)
	require.True(t, ok)
	require.Len(t, requests, 2, "Should have 2 request definitions")
	
	// Both requests should be properly formatted
	for i, req := range requests {
		reqMap := req.(map[string]any)
		require.Equal(t, "Request"+string(rune('1'+i)), reqMap["name"])
		require.Equal(t, "GET", reqMap["method"])
		require.Contains(t, reqMap["url"].(string), "endpoint"+string(rune('1'+i)))
		
		headers, ok := reqMap["headers"].(map[string]any)
		require.True(t, ok, "Headers should be a map")
		require.Len(t, headers, 1)
		authValue, ok := headers["Authorization"].(string)
		require.True(t, ok)
		require.Equal(t, "Bearer {{token}}", authValue)
	}
}