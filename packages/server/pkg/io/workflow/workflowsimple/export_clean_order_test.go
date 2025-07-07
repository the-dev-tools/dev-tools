package workflowsimple_test

import (
	"testing"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/io/workflow/workflowsimple"
	"the-dev-tools/server/pkg/ioworkspace"
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
	exported, err := workflowsimple.ExportWorkflowClean(workspaceData)
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