package workflowsimple_test

import (
	"testing"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/io/workflow/workflowsimple"
	"the-dev-tools/server/pkg/ioworkspace"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/model/mworkspace"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

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
	exported, err := workflowsimple.ExportWorkflowClean(workspaceData)
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
	exported, err := workflowsimple.ExportWorkflowClean(workspaceData)
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