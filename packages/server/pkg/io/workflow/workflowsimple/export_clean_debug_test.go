package workflowsimple_test

import (
	"testing"
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

func TestDebugRequestMapping(t *testing.T) {
	// Create a simple test to understand the mapping issue
	
	workspaceData := &ioworkspace.WorkspaceData{
		Workspace: mworkspace.Workspace{
			ID:   idwrap.NewNow(),
			Name: "Debug Test",
		},
		Flows: []mflow.Flow{
			{
				ID:   idwrap.NewNow(),
				Name: "DebugFlow",
			},
		},
	}
	
	flowID := workspaceData.Flows[0].ID
	
	// Create nodes
	startNodeID := idwrap.NewNow()
	nodeID := idwrap.NewNow()
	
	workspaceData.FlowNodes = []mnnode.MNode{
		{
			ID:       startNodeID,
			FlowID:   flowID,
			Name:     "Start",
			NodeKind: mnnode.NODE_KIND_NO_OP,
		},
		{
			ID:       nodeID,
			FlowID:   flowID,
			Name:     "MyRequestNode",
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
	t.Logf("Endpoint ID: %s", endpointID.String())
	
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
	
	// Create request node
	workspaceData.FlowRequestNodes = []mnrequest.MNRequest{
		{
			FlowNodeID: nodeID,
			EndpointID: &endpointID,
			ExampleID:  &exampleID,
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
	
	// Check the request definition
	requests := exportedData["requests"].([]any)
	require.Len(t, requests, 1)
	
	reqDef := requests[0].(map[string]any)
	t.Logf("Request definition name: %s", reqDef["name"])
	
	// Check the step
	flows := exportedData["flows"].([]any)
	flow := flows[0].(map[string]any)
	steps := flow["steps"].([]any)
	require.Len(t, steps, 1)
	
	step := steps[0].(map[string]any)
	reqStep := step["request"].(map[string]any)
	t.Logf("Step name: %s", reqStep["name"])
	t.Logf("Step use_request: %s", reqStep["use_request"])
	
	// They should match somehow
	require.Equal(t, "MyRequestNode", reqStep["name"], "Step name should be the node name")
	require.Equal(t, reqDef["name"], reqStep["use_request"], "use_request should reference the request definition")
}