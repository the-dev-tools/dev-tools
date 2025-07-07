package workflowsimple_test

import (
	"testing"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/io/workflow/workflowsimple"
	"the-dev-tools/server/pkg/ioworkspace"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnif"
	"the-dev-tools/server/pkg/model/mnnode/mnjs"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/model/mworkspace"

	"github.com/stretchr/testify/require"
)

func TestImportExportRoundTrip(t *testing.T) {
	// Test simple workflow
	yamlData := `
workspace_name: Test Workspace

flows:
  - name: SimpleFlow
    variables:
      - name: base_url
        value: "https://api.example.com"
      - name: token
        value: "bearer123"
    steps:
      - request:
          name: GetUser
          url: "{{base_url}}/users/1"
          method: GET
          headers:
            - name: Authorization
              value: "Bearer {{token}}"
`

	// Import from simplified format
	imported, err := workflowsimple.ImportWorkflowYAML([]byte(yamlData))
	require.NoError(t, err)
	require.NotNil(t, imported)

	// Verify imported data
	require.Equal(t, "Test Workspace", imported.Workspace.Name)
	require.Len(t, imported.Flows, 1)
	require.Equal(t, "SimpleFlow", imported.Flows[0].Name)
	require.Len(t, imported.FlowVariables, 2)

	// Export back to simplified format
	exported, err := workflowsimple.ExportWorkflowYAML(imported)
	require.NoError(t, err)

	// Parse exported YAML
	reimported, err := workflowsimple.ImportWorkflowYAML(exported)
	require.NoError(t, err)

	// Verify round-trip preservation
	require.Equal(t, imported.Workspace.Name, reimported.Workspace.Name)
	require.Len(t, reimported.Flows, len(imported.Flows))
	require.Equal(t, imported.Flows[0].Name, reimported.Flows[0].Name)
	require.Len(t, reimported.FlowVariables, len(imported.FlowVariables))
}

func TestImportComplexWorkflow(t *testing.T) {
	yamlData := `
workspace_name: Complex Workflow Test

request_templates:
  api_base:
    method: POST
    url: "{{base_url}}/api/v1"
    headers:
      - name: Content-Type
        value: application/json

flows:
  - name: ComplexFlow
    variables:
      - name: base_url
        value: "https://api.example.com"
    steps:
      - request:
          name: InitialRequest
          use_request: api_base
          headers:
            - name: X-Custom
              value: custom-value
      
      - if:
          name: CheckStatus
          condition: "InitialRequest.response.status == 200"
          then: ProcessSuccess
          else: HandleError
      
      - request:
          name: ProcessSuccess
          url: "{{base_url}}/success"
          method: POST
      
      - js:
          name: HandleError
          code: |
            console.error("Request failed");
            return { error: true };
      
      - for:
          name: ProcessItems
          iter_count: 3
          loop: ProcessItem
          depends_on:
            - ProcessSuccess
      
      - request:
          name: ProcessItem
          url: "{{base_url}}/item"
          method: GET
`

	// Import
	imported, err := workflowsimple.ImportWorkflowYAML([]byte(yamlData))
	require.NoError(t, err)

	// Verify imported structure
	require.Len(t, imported.Flows, 1)
	require.Equal(t, "ComplexFlow", imported.Flows[0].Name)
	
	// Count node types
	requestNodeCount := 0
	conditionNodeCount := 0
	jsNodeCount := 0
	forNodeCount := 0
	
	for _, node := range imported.FlowNodes {
		switch node.NodeKind {
		case mnnode.NODE_KIND_REQUEST:
			requestNodeCount++
		case mnnode.NODE_KIND_CONDITION:
			conditionNodeCount++
		case mnnode.NODE_KIND_JS:
			jsNodeCount++
		case mnnode.NODE_KIND_FOR:
			forNodeCount++
		}
	}
	
	require.Equal(t, 3, requestNodeCount)
	require.Equal(t, 1, conditionNodeCount)
	require.Equal(t, 1, jsNodeCount)
	require.Equal(t, 1, forNodeCount)

	// Verify request nodes have delta endpoints
	require.Greater(t, len(imported.FlowRequestNodes), 0)
	for _, reqNode := range imported.FlowRequestNodes {
		require.NotNil(t, reqNode.DeltaEndpointID)
		require.NotNil(t, reqNode.DeltaExampleID)
	}
}

func TestExportPreservesStructure(t *testing.T) {
	// Create a workspace data structure manually
	workspaceData := &ioworkspace.WorkspaceData{
		Workspace: mworkspace.Workspace{
			ID:   idwrap.NewNow(),
			Name: "Export Test",
		},
		Flows: []mflow.Flow{
			{
				ID:   idwrap.NewNow(),
				Name: "TestFlow",
			},
		},
		FlowNodes: []mnnode.MNode{
			{
				ID:       idwrap.NewNow(),
				Name:     "Start",
				NodeKind: mnnode.NODE_KIND_NO_OP,
			},
			{
				ID:       idwrap.NewNow(),
				Name:     "GetData",
				NodeKind: mnnode.NODE_KIND_REQUEST,
			},
		},
		FlowEdges: []edge.Edge{
			{
				ID:            idwrap.NewNow(),
				SourceID:      idwrap.NewNow(), // Will be fixed after nodes are created
				TargetID:      idwrap.NewNow(), // Will be fixed after nodes are created
				SourceHandler: edge.HandleUnspecified,
			},
		},
	}

	// Fix edge IDs
	workspaceData.FlowEdges[0].SourceID = workspaceData.FlowNodes[0].ID
	workspaceData.FlowEdges[0].TargetID = workspaceData.FlowNodes[1].ID

	// Set flow IDs
	for i := range workspaceData.FlowNodes {
		workspaceData.FlowNodes[i].FlowID = workspaceData.Flows[0].ID
	}
	for i := range workspaceData.FlowEdges {
		workspaceData.FlowEdges[i].FlowID = workspaceData.Flows[0].ID
	}

	// Add a noop node for start
	workspaceData.FlowNoopNodes = []mnnoop.NoopNode{
		{
			FlowNodeID: workspaceData.FlowNodes[0].ID,
			Type:       mnnoop.NODE_NO_OP_KIND_START,
		},
	}

	// Add request node data
	workspaceData.FlowRequestNodes = []mnrequest.MNRequest{
		{
			FlowNodeID: workspaceData.FlowNodes[1].ID,
		},
	}

	// Add endpoint and example
	endpointID := idwrap.NewNow()
	exampleID := idwrap.NewNow()
	workspaceData.Endpoints = []mitemapi.ItemApi{
		{
			ID:     endpointID,
			Name:   "GetData",
			Url:    "https://api.example.com/data",
			Method: "GET",
		},
	}
	workspaceData.Examples = []mitemapiexample.ItemApiExample{
		{
			ID:        exampleID,
			Name:      "GetData",
			ItemApiID: endpointID,
		},
	}
	
	// Update request node with IDs
	workspaceData.FlowRequestNodes[0].EndpointID = &endpointID
	workspaceData.FlowRequestNodes[0].ExampleID = &exampleID

	// Export to simplified format
	exported, err := workflowsimple.ExportWorkflowYAML(workspaceData)
	require.NoError(t, err)
	require.NotNil(t, exported)

	// Verify exported content
	exportedStr := string(exported)
	require.Contains(t, exportedStr, "workspace_name: Export Test")
	require.Contains(t, exportedStr, "name: TestFlow")
	require.Contains(t, exportedStr, "name: GetData")
	require.Contains(t, exportedStr, "url: https://api.example.com/data")
	require.Contains(t, exportedStr, "method: GET")
}

func TestImportWithDependencies(t *testing.T) {
	yamlData := `
workspace_name: Dependency Test

flows:
  - name: DependencyFlow
    steps:
      - request:
          name: Step1
          url: "https://api.example.com/1"
          method: GET
      
      - request:
          name: Step2
          url: "https://api.example.com/2"
          method: GET
      
      - request:
          name: Step3
          url: "https://api.example.com/3"
          method: GET
          depends_on:
            - Step1
            - Step2
`

	// Import
	imported, err := workflowsimple.ImportWorkflowYAML([]byte(yamlData))
	require.NoError(t, err)

	// Verify edges were created correctly
	edgeCount := 0
	for _, edge := range imported.FlowEdges {
		// Find source and target nodes
		var sourceNode, targetNode *mnnode.MNode
		for i := range imported.FlowNodes {
			if imported.FlowNodes[i].ID == edge.SourceID {
				sourceNode = &imported.FlowNodes[i]
			}
			if imported.FlowNodes[i].ID == edge.TargetID {
				targetNode = &imported.FlowNodes[i]
			}
		}
		
		// Check if this is a dependency edge to Step3
		if targetNode != nil && targetNode.Name == "Step3" {
			if sourceNode != nil && (sourceNode.Name == "Step1" || sourceNode.Name == "Step2") {
				edgeCount++
			}
		}
	}
	
	require.Equal(t, 2, edgeCount, "Step3 should have edges from both Step1 and Step2")
}

func TestImportExportWithVariables(t *testing.T) {
	yamlData := `
workspace_name: Variable Test

flows:
  - name: VarFlow
    variables:
      - name: host
        value: "api.example.com"
      - name: version
        value: "v2"
    steps:
      - request:
          name: TestVars
          url: "https://{{host}}/{{version}}/users"
          method: GET
          headers:
            - name: Authorization
              value: "Bearer {{auth}}"
`

	// Import
	imported, err := workflowsimple.ImportWorkflowYAML([]byte(yamlData))
	require.NoError(t, err)

	// Verify variables
	require.Len(t, imported.FlowVariables, 2)
	varMap := make(map[string]string)
	for _, v := range imported.FlowVariables {
		varMap[v.Name] = v.Value
	}
	require.Equal(t, "api.example.com", varMap["host"])
	require.Equal(t, "v2", varMap["version"])

	// Export and verify variables are preserved
	exported, err := workflowsimple.ExportWorkflowYAML(imported)
	require.NoError(t, err)
	
	exportedStr := string(exported)
	require.Contains(t, exportedStr, "name: host")
	require.Contains(t, exportedStr, "value: api.example.com")
	require.Contains(t, exportedStr, "name: version")
	require.Contains(t, exportedStr, "value: v2")
}

func TestExportWithControlFlow(t *testing.T) {
	// Create workspace with control flow nodes
	workspaceData := &ioworkspace.WorkspaceData{
		Workspace: mworkspace.Workspace{
			ID:   idwrap.NewNow(),
			Name: "Control Flow Export",
		},
		Flows: []mflow.Flow{
			{
				ID:   idwrap.NewNow(),
				Name: "ControlFlow",
			},
		},
	}

	// Create nodes
	startNodeID := idwrap.NewNow()
	checkNodeID := idwrap.NewNow()
	successNodeID := idwrap.NewNow()
	errorNodeID := idwrap.NewNow()

	workspaceData.FlowNodes = []mnnode.MNode{
		{
			ID:       startNodeID,
			FlowID:   workspaceData.Flows[0].ID,
			Name:     "Start",
			NodeKind: mnnode.NODE_KIND_NO_OP,
		},
		{
			ID:       checkNodeID,
			FlowID:   workspaceData.Flows[0].ID,
			Name:     "CheckStatus",
			NodeKind: mnnode.NODE_KIND_CONDITION,
		},
		{
			ID:       successNodeID,
			FlowID:   workspaceData.Flows[0].ID,
			Name:     "HandleSuccess",
			NodeKind: mnnode.NODE_KIND_JS,
		},
		{
			ID:       errorNodeID,
			FlowID:   workspaceData.Flows[0].ID,
			Name:     "HandleError",
			NodeKind: mnnode.NODE_KIND_JS,
		},
	}

	// Add edges
	workspaceData.FlowEdges = []edge.Edge{
		{
			ID:            idwrap.NewNow(),
			FlowID:        workspaceData.Flows[0].ID,
			SourceID:      startNodeID,
			TargetID:      checkNodeID,
			SourceHandler: edge.HandleUnspecified,
		},
		{
			ID:            idwrap.NewNow(),
			FlowID:        workspaceData.Flows[0].ID,
			SourceID:      checkNodeID,
			TargetID:      successNodeID,
			SourceHandler: edge.HandleThen,
		},
		{
			ID:            idwrap.NewNow(),
			FlowID:        workspaceData.Flows[0].ID,
			SourceID:      checkNodeID,
			TargetID:      errorNodeID,
			SourceHandler: edge.HandleElse,
		},
	}

	// Add node data
	workspaceData.FlowNoopNodes = []mnnoop.NoopNode{
		{
			FlowNodeID: startNodeID,
			Type:       mnnoop.NODE_NO_OP_KIND_START,
		},
	}

	workspaceData.FlowConditionNodes = []mnif.MNIF{
		{
			FlowNodeID: checkNodeID,
			Condition: mcondition.Condition{
				Comparisons: mcondition.Comparison{
					Expression: "response.status == 200",
				},
			},
		},
	}

	workspaceData.FlowJSNodes = []mnjs.MNJS{
		{
			FlowNodeID: successNodeID,
			Code:       []byte("return { success: true };"),
		},
		{
			FlowNodeID: errorNodeID,
			Code:       []byte("return { error: true };"),
		},
	}

	// Export
	exported, err := workflowsimple.ExportWorkflowYAML(workspaceData)
	require.NoError(t, err)

	exportedStr := string(exported)
	
	// Verify control flow structure
	require.Contains(t, exportedStr, "name: CheckStatus")
	require.Contains(t, exportedStr, "expression: response.status == 200")
	require.Contains(t, exportedStr, "then: HandleSuccess")
	require.Contains(t, exportedStr, "else: HandleError")
	require.Contains(t, exportedStr, "code: 'return { success: true };'")
	require.Contains(t, exportedStr, "code: 'return { error: true };'")
}

func TestImportErrorHandling(t *testing.T) {
	tests := []struct {
		name     string
		yamlData string
		wantErr  string
	}{
		{
			name: "missing workspace name",
			yamlData: `
flows:
  - name: TestFlow
    steps:
      - request:
          name: Test
          url: "https://example.com"
`,
			wantErr: "workspace_name is required",
		},
		{
			name: "no flows",
			yamlData: `
workspace_name: Test
`,
			wantErr: "at least one flow is required",
		},
		{
			name: "invalid YAML",
			yamlData: `
workspace_name: Test
flows: [
  invalid yaml
`,
			wantErr: "failed to unmarshal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := workflowsimple.ImportWorkflowYAML([]byte(tt.yamlData))
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestExportErrorHandling(t *testing.T) {
	// Test nil workspace data
	_, err := workflowsimple.ExportWorkflowYAML(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "workspace data cannot be nil")

	// Test workspace with no flows
	workspaceData := &ioworkspace.WorkspaceData{
		Workspace: mworkspace.Workspace{
			ID:   idwrap.NewNow(),
			Name: "Empty",
		},
		Flows: []mflow.Flow{},
	}
	_, err = workflowsimple.ExportWorkflowYAML(workspaceData)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no flows to export")
}