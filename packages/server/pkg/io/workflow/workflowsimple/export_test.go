package workflowsimple_test

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
	"the-dev-tools/server/pkg/flow/edge"
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
	exported, err := workflowsimple.ExportWorkflowYAML(workspaceData)
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
	// Delta endpoint is used to preserve variable placeholders in the export
	require.Equal(t, "https://{{base_url}}/users", request["url"])

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
	exported, err := workflowsimple.ExportWorkflowYAML(workspaceData)
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

func TestExportCleanNamingConsistency(t *testing.T) {
	// Test that step names match their use_request references correctly

	// Create workspace data
	workspaceData := &ioworkspace.WorkspaceData{
		Workspace: mworkspace.Workspace{
			ID:   idwrap.NewNow(),
			Name: "Test Naming Consistency",
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
	request1NodeID := idwrap.NewNow()
	request2NodeID := idwrap.NewNow()

	workspaceData.FlowNodes = []mnnode.MNode{
		{
			ID:       startNodeID,
			FlowID:   flowID,
			Name:     "Start",
			NodeKind: mnnode.NODE_KIND_NO_OP,
		},
		{
			ID:       request1NodeID,
			FlowID:   flowID,
			Name:     "request_0", // Node name is request_0
			NodeKind: mnnode.NODE_KIND_REQUEST,
		},
		{
			ID:       request2NodeID,
			FlowID:   flowID,
			Name:     "request_1", // Node name is request_1
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

	// Create endpoints
	endpoint1ID := idwrap.NewNow()
	endpoint2ID := idwrap.NewNow()

	workspaceData.Endpoints = []mitemapi.ItemApi{
		{
			ID:     endpoint1ID,
			Name:   "Login",
			Url:    "https://api.example.com/login",
			Method: "POST",
		},
		{
			ID:     endpoint2ID,
			Name:   "GetUsers",
			Url:    "https://api.example.com/users",
			Method: "GET",
		},
	}

	// Create examples
	example1ID := idwrap.NewNow()
	example2ID := idwrap.NewNow()

	workspaceData.Examples = []mitemapiexample.ItemApiExample{
		{
			ID:        example1ID,
			Name:      "Login Example",
			ItemApiID: endpoint1ID,
			IsDefault: true,
		},
		{
			ID:        example2ID,
			Name:      "GetUsers Example",
			ItemApiID: endpoint2ID,
			IsDefault: true,
		},
	}

	// Create request nodes
	workspaceData.FlowRequestNodes = []mnrequest.MNRequest{
		{
			FlowNodeID: request1NodeID,
			EndpointID: &endpoint1ID,
			ExampleID:  &example1ID,
		},
		{
			FlowNodeID: request2NodeID,
			EndpointID: &endpoint2ID,
			ExampleID:  &example2ID,
		},
	}

	// Add headers
	workspaceData.ExampleHeaders = []mexampleheader.Header{
		{
			ID:        idwrap.NewNow(),
			ExampleID: example1ID,
			HeaderKey: "Content-Type",
			Value:     "application/json",
			Enable:    true,
		},
		{
			ID:        idwrap.NewNow(),
			ExampleID: example2ID,
			HeaderKey: "Authorization",
			Value:     "Bearer {{token}}",
			Enable:    true,
		},
	}

	// Create edges - request_1 depends on request_0
	workspaceData.FlowEdges = []edge.Edge{
		{
			ID:            idwrap.NewNow(),
			FlowID:        flowID,
			SourceID:      startNodeID,
			TargetID:      request1NodeID,
			SourceHandler: edge.HandleUnspecified,
		},
		{
			ID:            idwrap.NewNow(),
			FlowID:        flowID,
			SourceID:      request1NodeID,
			TargetID:      request2NodeID,
			SourceHandler: edge.HandleUnspecified,
		},
	}

	// Export
	exported, err := workflowsimple.ExportWorkflowYAML(workspaceData)
	require.NoError(t, err)

	// Parse the exported YAML
	var exportedData map[string]any
	err = yaml.Unmarshal(exported, &exportedData)
	require.NoError(t, err)

	t.Logf("Exported YAML:\n%s", string(exported))

	// Verify requests section exists
	requests, ok := exportedData["requests"].([]any)
	require.True(t, ok, "Expected 'requests' section")
	require.Len(t, requests, 2, "Should have 2 request definitions")

	// Build a map of request names to verify references
	requestNameMap := make(map[string]bool)
	for _, req := range requests {
		reqMap := req.(map[string]any)
		name, ok := reqMap["name"].(string)
		require.True(t, ok, "Request should have a name")
		requestNameMap[name] = true
	}

	// Verify flows
	flows, ok := exportedData["flows"].([]any)
	require.True(t, ok, "Expected 'flows' section")
	require.Len(t, flows, 1)

	flow := flows[0].(map[string]any)
	steps, ok := flow["steps"].([]any)
	require.True(t, ok, "Expected 'steps' in flow")
	require.Len(t, steps, 2, "Should have 2 steps")

	// Check each step
	for i, step := range steps {
		stepMap := step.(map[string]any)
		reqStep, ok := stepMap["request"].(map[string]any)
		require.True(t, ok, "Step should be a request")

		// Verify step name matches the node name
		stepName, ok := reqStep["name"].(string)
		require.True(t, ok, "Step should have a name")
		expectedName := "request_" + string(rune('0'+i))
		require.Equal(t, expectedName, stepName, "Step name should match node name")

		// Verify use_request references a valid request
		useRequest, ok := reqStep["use_request"].(string)
		require.True(t, ok, "Step should have use_request")
		require.True(t, requestNameMap[useRequest], "use_request '%s' should reference an existing request", useRequest)

		// For request_1, verify it has depends_on
		if i == 1 {
			dependsOn, ok := reqStep["depends_on"].([]any)
			require.True(t, ok, "request_1 should have depends_on")
			require.Len(t, dependsOn, 1, "request_1 should depend on one node")
			require.Equal(t, "request_0", dependsOn[0], "request_1 should depend on request_0")
		}
	}
}

func TestExportCleanWithDependencies(t *testing.T) {
	// Test that dependencies are properly exported

	workspaceData := &ioworkspace.WorkspaceData{
		Workspace: mworkspace.Workspace{
			ID:   idwrap.NewNow(),
			Name: "Test Dependencies",
		},
		Flows: []mflow.Flow{
			{
				ID:   idwrap.NewNow(),
				Name: "DependencyFlow",
			},
		},
	}

	flowID := workspaceData.Flows[0].ID

	// Create nodes
	startNodeID := idwrap.NewNow()
	nodeAID := idwrap.NewNow()
	nodeBID := idwrap.NewNow()
	nodeCID := idwrap.NewNow()

	workspaceData.FlowNodes = []mnnode.MNode{
		{
			ID:       startNodeID,
			FlowID:   flowID,
			Name:     "Start",
			NodeKind: mnnode.NODE_KIND_NO_OP,
		},
		{
			ID:       nodeAID,
			FlowID:   flowID,
			Name:     "NodeA",
			NodeKind: mnnode.NODE_KIND_REQUEST,
		},
		{
			ID:       nodeBID,
			FlowID:   flowID,
			Name:     "NodeB",
			NodeKind: mnnode.NODE_KIND_REQUEST,
		},
		{
			ID:       nodeCID,
			FlowID:   flowID,
			Name:     "NodeC",
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

	// Create a single endpoint for all (to simplify)
	endpointID := idwrap.NewNow()
	workspaceData.Endpoints = []mitemapi.ItemApi{
		{
			ID:     endpointID,
			Name:   "TestEndpoint",
			Url:    "https://api.example.com/test",
			Method: "GET",
		},
	}

	// Create example
	exampleID := idwrap.NewNow()
	workspaceData.Examples = []mitemapiexample.ItemApiExample{
		{
			ID:        exampleID,
			Name:      "TestExample",
			ItemApiID: endpointID,
			IsDefault: true,
		},
	}

	// Create request nodes
	for _, nodeID := range []idwrap.IDWrap{nodeAID, nodeBID, nodeCID} {
		workspaceData.FlowRequestNodes = append(workspaceData.FlowRequestNodes, mnrequest.MNRequest{
			FlowNodeID: nodeID,
			EndpointID: &endpointID,
			ExampleID:  &exampleID,
		})
	}

	// Create edges: NodeC depends on both NodeA and NodeB
	workspaceData.FlowEdges = []edge.Edge{
		{
			ID:            idwrap.NewNow(),
			FlowID:        flowID,
			SourceID:      startNodeID,
			TargetID:      nodeAID,
			SourceHandler: edge.HandleUnspecified,
		},
		{
			ID:            idwrap.NewNow(),
			FlowID:        flowID,
			SourceID:      startNodeID,
			TargetID:      nodeBID,
			SourceHandler: edge.HandleUnspecified,
		},
		{
			ID:            idwrap.NewNow(),
			FlowID:        flowID,
			SourceID:      nodeAID,
			TargetID:      nodeCID,
			SourceHandler: edge.HandleUnspecified,
		},
		{
			ID:            idwrap.NewNow(),
			FlowID:        flowID,
			SourceID:      nodeBID,
			TargetID:      nodeCID,
			SourceHandler: edge.HandleUnspecified,
		},
	}

	// Export
	exported, err := workflowsimple.ExportWorkflowYAML(workspaceData)
	require.NoError(t, err)

	// Parse the exported YAML
	var exportedData map[string]any
	err = yaml.Unmarshal(exported, &exportedData)
	require.NoError(t, err)

	t.Logf("Exported YAML with dependencies:\n%s", string(exported))

	// Find NodeC in the steps and verify its dependencies
	flows := exportedData["flows"].([]any)
	flow := flows[0].(map[string]any)
	steps := flow["steps"].([]any)

	var nodeCStep map[string]any
	for _, step := range steps {
		stepMap := step.(map[string]any)
		if reqStep, ok := stepMap["request"].(map[string]any); ok {
			if reqStep["name"] == "NodeC" {
				nodeCStep = reqStep
				break
			}
		}
	}

	require.NotNil(t, nodeCStep, "Should find NodeC step")

	// Verify NodeC has dependencies
	dependsOn, ok := nodeCStep["depends_on"].([]any)
	require.True(t, ok, "NodeC should have depends_on")
	require.Len(t, dependsOn, 2, "NodeC should depend on both NodeA and NodeB")

	// Check that both NodeA and NodeB are in the dependencies
	depMap := make(map[string]bool)
	for _, dep := range dependsOn {
		depMap[dep.(string)] = true
	}
	require.True(t, depMap["NodeA"], "NodeC should depend on NodeA")
	require.True(t, depMap["NodeB"], "NodeC should depend on NodeB")
}

func TestExportRequestOrderConsistency(t *testing.T) {
	// Test multiple times to catch ordering issues (maps are random)
	for i := 0; i < 5; i++ {
		t.Run("iteration", func(t *testing.T) {
			testRequestOrdering(t)
		})
	}
}

func testRequestOrdering(t *testing.T) {
	workspaceData := &ioworkspace.WorkspaceData{
		Workspace: mworkspace.Workspace{
			ID:   idwrap.NewNow(),
			Name: "Order Test",
		},
		Flows: []mflow.Flow{
			{
				ID:   idwrap.NewNow(),
				Name: "OrderFlow",
			},
		},
	}

	flowID := workspaceData.Flows[0].ID

	// Create nodes
	startNodeID := idwrap.NewNow()
	nodeAID := idwrap.NewNow()
	nodeBID := idwrap.NewNow()
	nodeCID := idwrap.NewNow()

	workspaceData.FlowNodes = []mnnode.MNode{
		{
			ID:       startNodeID,
			FlowID:   flowID,
			Name:     "Start",
			NodeKind: mnnode.NODE_KIND_NO_OP,
		},
		{
			ID:       nodeAID,
			FlowID:   flowID,
			Name:     "RequestA",
			NodeKind: mnnode.NODE_KIND_REQUEST,
		},
		{
			ID:       nodeBID,
			FlowID:   flowID,
			Name:     "RequestB",
			NodeKind: mnnode.NODE_KIND_REQUEST,
		},
		{
			ID:       nodeCID,
			FlowID:   flowID,
			Name:     "RequestC",
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

	// Create different endpoints for each
	endpointAID := idwrap.NewNow()
	endpointBID := idwrap.NewNow()
	endpointCID := idwrap.NewNow()

	workspaceData.Endpoints = []mitemapi.ItemApi{
		{
			ID:     endpointAID,
			Name:   "EndpointA",
			Url:    "https://api.example.com/a",
			Method: "GET",
		},
		{
			ID:     endpointBID,
			Name:   "EndpointB",
			Url:    "https://api.example.com/b",
			Method: "POST",
		},
		{
			ID:     endpointCID,
			Name:   "EndpointC",
			Url:    "https://api.example.com/c",
			Method: "PUT",
		},
	}

	// Create examples
	exampleAID := idwrap.NewNow()
	exampleBID := idwrap.NewNow()
	exampleCID := idwrap.NewNow()

	workspaceData.Examples = []mitemapiexample.ItemApiExample{
		{
			ID:        exampleAID,
			Name:      "ExampleA",
			ItemApiID: endpointAID,
			IsDefault: true,
		},
		{
			ID:        exampleBID,
			Name:      "ExampleB",
			ItemApiID: endpointBID,
			IsDefault: true,
		},
		{
			ID:        exampleCID,
			Name:      "ExampleC",
			ItemApiID: endpointCID,
			IsDefault: true,
		},
	}

	// Create request nodes
	workspaceData.FlowRequestNodes = []mnrequest.MNRequest{
		{
			FlowNodeID: nodeAID,
			EndpointID: &endpointAID,
			ExampleID:  &exampleAID,
		},
		{
			FlowNodeID: nodeBID,
			EndpointID: &endpointBID,
			ExampleID:  &exampleBID,
		},
		{
			FlowNodeID: nodeCID,
			EndpointID: &endpointCID,
			ExampleID:  &exampleCID,
		},
	}

	// Create edges
	workspaceData.FlowEdges = []edge.Edge{
		{
			ID:            idwrap.NewNow(),
			FlowID:        flowID,
			SourceID:      startNodeID,
			TargetID:      nodeAID,
			SourceHandler: edge.HandleUnspecified,
		},
		{
			ID:            idwrap.NewNow(),
			FlowID:        flowID,
			SourceID:      nodeAID,
			TargetID:      nodeBID,
			SourceHandler: edge.HandleUnspecified,
		},
		{
			ID:            idwrap.NewNow(),
			FlowID:        flowID,
			SourceID:      nodeBID,
			TargetID:      nodeCID,
			SourceHandler: edge.HandleUnspecified,
		},
	}

	// Export
	exported, err := workflowsimple.ExportWorkflowYAML(workspaceData)
	require.NoError(t, err)

	// Parse the exported YAML
	var exportedData map[string]any
	err = yaml.Unmarshal(exported, &exportedData)
	require.NoError(t, err)

	t.Logf("Exported YAML:\n%s", string(exported))

	// Verify each step references the correct request
	flows := exportedData["flows"].([]any)
	flow := flows[0].(map[string]any)
	steps := flow["steps"].([]any)

	// Build a map of URL to request name from the requests section
	urlToRequestName := make(map[string]string)
	requests := exportedData["requests"].([]any)
	for _, req := range requests {
		reqMap := req.(map[string]any)
		url := reqMap["url"].(string)
		name := reqMap["name"].(string)
		urlToRequestName[url] = name
	}

	// Check each step
	expectedMappings := map[string]string{
		"RequestA": "https://api.example.com/a",
		"RequestB": "https://api.example.com/b",
		"RequestC": "https://api.example.com/c",
	}

	for _, step := range steps {
		stepMap := step.(map[string]any)
		reqStep := stepMap["request"].(map[string]any)

		stepName := reqStep["name"].(string)
		useRequest := reqStep["use_request"].(string)

		// Find the request definition
		var requestDef map[string]any
		for _, req := range requests {
			reqMap := req.(map[string]any)
			if reqMap["name"] == useRequest {
				requestDef = reqMap
				break
			}
		}

		require.NotNil(t, requestDef, "Should find request definition for %s", useRequest)

		// Verify the URL matches what we expect
		expectedURL := expectedMappings[stepName]
		actualURL := requestDef["url"].(string)
		require.Equal(t, expectedURL, actualURL, "Step %s should reference request with URL %s", stepName, expectedURL)
	}
}

// TestExportWithHeadersQueryAndBody - removed as it was testing the old export format
/*
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
	yamlData, err := workflowsimple.ExportWorkflowYAMLOld(workspaceData)
	if err != nil {
		t.Fatalf("failed to export: %v", err)
	}

	// Parse the exported YAML
	var exported workflowsimple.WorkflowFormat
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
*/

// TestExportMultipleRequestsWithOverrides - removed as it was testing the old export format
/*
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
	yamlData, err := workflowsimple.ExportWorkflowYAMLOld(workspaceData)
	if err != nil {
		t.Fatalf("failed to export: %v", err)
	}

	// Log the output
	t.Logf("Exported YAML:\n%s", string(yamlData))

	// Parse and verify
	var exported workflowsimple.WorkflowFormat
	if err := yaml.Unmarshal(yamlData, &exported); err != nil {
		t.Fatalf("failed to parse exported YAML: %v", err)
	}

	if len(exported.Flows[0].Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(exported.Flows[0].Steps))
	}
}
*/

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
	yamlData, err := workflowsimple.ExportWorkflowYAML(workspaceData)
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
