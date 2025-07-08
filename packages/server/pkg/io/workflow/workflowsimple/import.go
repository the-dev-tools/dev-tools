package workflowsimple

import (
	"fmt"
	"time"
	"gopkg.in/yaml.v3"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/ioworkspace"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/model/massertres"
	"the-dev-tools/server/pkg/model/mbodyform"
	"the-dev-tools/server/pkg/model/mbodyurl"
	"the-dev-tools/server/pkg/model/mcollection"
	"the-dev-tools/server/pkg/model/mexampleresp"
	"the-dev-tools/server/pkg/model/mexamplerespheader"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mflowvariable"
	"the-dev-tools/server/pkg/model/mitemfolder"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"the-dev-tools/server/pkg/model/mnnode/mnforeach"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/model/mvar"
)

// ImportWorkflowYAML converts simplified workflow YAML to ioworkspace.WorkspaceData
func ImportWorkflowYAML(data []byte) (*ioworkspace.WorkspaceData, error) {
	// Parse the simplified format
	workflowData, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse simplified workflow: %w", err)
	}

	// Extract workspace name from the workflow
	workspaceName := "Imported Workspace"
	// Check for workspace name in the original YAML
	var yamlCheck map[string]any
	if err := yaml.Unmarshal(data, &yamlCheck); err == nil {
		if name, ok := yamlCheck["workspace_name"].(string); ok && name != "" {
			workspaceName = name
		}
	}

	// Generate workspace ID first
	workspaceID := idwrap.NewNow()

	// Create a default collection for the imported workflows
	collectionID := idwrap.NewNow()
	defaultCollection := mcollection.Collection{
		ID:          collectionID,
		Name:        "Imported Workflows",
		WorkspaceID: workspaceID,
	}

	// Update all endpoints and examples to have the collection ID
	for i := range workflowData.Endpoints {
		workflowData.Endpoints[i].CollectionID = collectionID
	}
	for i := range workflowData.Examples {
		workflowData.Examples[i].CollectionID = collectionID
	}

	// Update the flow with the workspace ID
	workflowData.Flow.WorkspaceID = workspaceID
	
	// Extract all variable references from the workflow
	variableRefs := ExtractVariableReferences(workflowData)
	
	// Separate into flow and environment variables
	flowVarsFromYAML, envVarsToCreate := SeparateVariablesByType(variableRefs)
	
	// Create a default environment for the workspace
	defaultEnv := menv.Env{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		Type:        menv.EnvNormal,
		Name:        "Default Environment",
		Description: "Default environment for imported workflows",
		Updated:     time.Now(),
	}
	
	// Convert environment variables to proper format with EnvID
	var environmentVariables []mvar.Var
	for _, v := range envVarsToCreate {
		envVar := mvar.Var{
			ID:          idwrap.NewNow(),
			EnvID:       defaultEnv.ID,
			VarKey:      v.VarKey,
			Value:       v.Value,
			Enabled:     true,
			Description: "Imported from workflow",
		}
		environmentVariables = append(environmentVariables, envVar)
	}

	// Create workspace data
	workspaceData := &ioworkspace.WorkspaceData{
		Workspace: mworkspace.Workspace{
			ID:   workspaceID, // Use the workspace ID we generated above
			Name: workspaceName,
		},
		Collections:            []mcollection.Collection{defaultCollection},
		Folders:                make([]mitemfolder.ItemFolder, 0),
		Endpoints:              workflowData.Endpoints,
		Examples:               workflowData.Examples,
		ExampleHeaders:         workflowData.Headers,
		ExampleQueries:         workflowData.Queries,
		ExampleAsserts:         make([]massert.Assert, 0),
		Rawbodies:              workflowData.RawBodies,
		FormBodies:             make([]mbodyform.BodyForm, 0),
		UrlBodies:              make([]mbodyurl.BodyURLEncoded, 0),
		ExampleResponses:       make([]mexampleresp.ExampleResp, 0),
		ExampleResponseHeaders: make([]mexamplerespheader.ExampleRespHeader, 0),
		ExampleResponseAsserts: make([]massertres.AssertResult, 0),
		Flows:                  []mflow.Flow{workflowData.Flow},
		FlowNodes:              workflowData.Nodes,
		FlowEdges:              workflowData.Edges,
		FlowVariables:          make([]mflowvariable.FlowVariable, 0),
		FlowRequestNodes:       workflowData.RequestNodes,
		FlowConditionNodes:     workflowData.ConditionNodes,
		FlowNoopNodes:          workflowData.NoopNodes,
		FlowForNodes:           workflowData.ForNodes,
		FlowForEachNodes:       make([]mnforeach.MNForEach, 0), // Convert from ForNodes if needed
		FlowJSNodes:            workflowData.JSNodes,
		Environments:           []menv.Env{defaultEnv},
		Variables:              environmentVariables,
	}

	// Convert flow variables (only those defined in the YAML with values)
	for _, v := range flowVarsFromYAML {
		flowVar := mflowvariable.FlowVariable{
			ID:      idwrap.NewNow(),
			FlowID:  workflowData.Flow.ID,
			Name:    v.VarKey,
			Value:   v.Value,
			Enabled: true,
		}
		workspaceData.FlowVariables = append(workspaceData.FlowVariables, flowVar)
	}

	// Separate for and for_each nodes
	forNodes := make([]mnfor.MNFor, 0)
	forEachNodes := make([]mnforeach.MNForEach, 0)
	
	for _, node := range workflowData.Nodes {
		switch node.NodeKind {
		case mnnode.NODE_KIND_FOR_EACH:
			// Find the corresponding for node data
			for _, fn := range workflowData.ForNodes {
				if fn.FlowNodeID == node.ID {
					// Create a for_each node
					forEachNode := mnforeach.MNForEach{
						FlowNodeID: fn.FlowNodeID,
						// Note: The simplified format doesn't capture the items expression
						// In a real implementation, we'd need to store this somewhere
					}
					forEachNodes = append(forEachNodes, forEachNode)
				}
			}
		case mnnode.NODE_KIND_FOR:
			// Keep regular for nodes
			for _, fn := range workflowData.ForNodes {
				if fn.FlowNodeID == node.ID {
					forNodes = append(forNodes, fn)
				}
			}
		}
	}

	workspaceData.FlowForNodes = forNodes
	workspaceData.FlowForEachNodes = forEachNodes

	return workspaceData, nil
}