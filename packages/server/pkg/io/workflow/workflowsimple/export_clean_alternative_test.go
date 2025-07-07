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

func TestAlternativeExportFormat(t *testing.T) {
	// Maybe we should just inline all request details instead of using a global requests section?
	// This test explores what the export would look like without the requests section
	
	workspaceData := &ioworkspace.WorkspaceData{
		Workspace: mworkspace.Workspace{
			ID:   idwrap.NewNow(),
			Name: "Alternative Format Test",
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
			Name:     "request_0",
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
	
	// Export with the current system
	exported, err := workflowsimple.ExportWorkflowClean(workspaceData)
	require.NoError(t, err)
	
	t.Logf("Current export format:\n%s", string(exported))
	
	// Parse to check structure
	var exportedData map[string]any
	err = yaml.Unmarshal(exported, &exportedData)
	require.NoError(t, err)
	
	// Alternative: what if we used node names as request definition names?
	// Or what if we didn't use a global requests section at all?
	
	// Expected alternative format 1: Use node names
	// requests:
	//   - name: request_0  # matches the node name
	//     method: GET
	//     url: https://api.example.com/test
	// flows:
	//   - steps:
	//       - request:
	//           name: request_0
	//           use_request: request_0  # same as node name
	
	// Expected alternative format 2: Inline everything
	// flows:
	//   - steps:
	//       - request:
	//           name: request_0
	//           method: GET
	//           url: https://api.example.com/test
	//           headers: ...
	
	// Current format has the mismatch:
	// Step name: request_0
	// use_request: request_1
	
	flows := exportedData["flows"].([]any)
	flow := flows[0].(map[string]any)
	steps := flow["steps"].([]any)
	step := steps[0].(map[string]any)
	reqStep := step["request"].(map[string]any)
	
	t.Logf("Step name: %s", reqStep["name"])
	t.Logf("use_request: %s", reqStep["use_request"])
	
	// The issue is that node name is request_0 but use_request is request_1
}