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
	"the-dev-tools/server/pkg/model/mexampleresp"
	"the-dev-tools/server/pkg/model/mexamplerespheader"
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
	// Generate workspace ID first
	workspaceID := idwrap.NewNow()
	collectionID := idwrap.NewNow()

	// Use the new ConvertSimplifiedYAML function
	resolved, err := ConvertSimplifiedYAML(data, collectionID, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to convert simplified workflow: %w", err)
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
	// Parse the data to extract variables
	workflowData, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse workflow: %w", err)
	}

	// Extract all variable references from the workflow
	variableRefs := ExtractVariableReferences(workflowData)
	
	// Separate into flow and environment variables
	_, envVarsToCreate := SeparateVariablesByType(variableRefs)
	
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
			ID:   workspaceID,
			Name: workspaceName,
		},
		Collections:            resolved.Collections,
		Folders:                make([]mitemfolder.ItemFolder, 0),
		Endpoints:              resolved.Endpoints,
		Examples:               resolved.Examples,
		ExampleHeaders:         resolved.Headers,
		ExampleQueries:         resolved.Queries,
		ExampleAsserts:         make([]massert.Assert, 0),
		Rawbodies:              resolved.RawBodies,
		FormBodies:             make([]mbodyform.BodyForm, 0),
		UrlBodies:              make([]mbodyurl.BodyURLEncoded, 0),
		ExampleResponses:       make([]mexampleresp.ExampleResp, 0),
		ExampleResponseHeaders: make([]mexamplerespheader.ExampleRespHeader, 0),
		ExampleResponseAsserts: make([]massertres.AssertResult, 0),
		Flows:                  resolved.Flows,
		FlowNodes:              resolved.FlowNodes,
		FlowEdges:              resolved.FlowEdges,
		FlowVariables:          resolved.FlowVariables,
		FlowRequestNodes:       resolved.FlowRequestNodes,
		FlowConditionNodes:     resolved.FlowConditionNodes,
		FlowNoopNodes:          resolved.FlowNoopNodes,
		FlowForNodes:           resolved.FlowForNodes,
		FlowForEachNodes:       make([]mnforeach.MNForEach, 0),
		FlowJSNodes:            resolved.FlowJSNodes,
		Environments:           []menv.Env{defaultEnv},
		Variables:              environmentVariables,
	}

	// Separate for and for_each nodes
	forNodes := make([]mnfor.MNFor, 0)
	forEachNodes := make([]mnforeach.MNForEach, 0)
	
	for _, node := range resolved.FlowNodes {
		switch node.NodeKind {
		case mnnode.NODE_KIND_FOR_EACH:
			// Find the corresponding for node data
			for _, fn := range resolved.FlowForNodes {
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
			for _, fn := range resolved.FlowForNodes {
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