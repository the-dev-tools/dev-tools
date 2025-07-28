package yamlflowsimple

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/ioworkspace"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/model/massertres"
	"the-dev-tools/server/pkg/model/mbodyform"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mbodyurl"
	"the-dev-tools/server/pkg/model/mcollection"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mexamplequery"
	"the-dev-tools/server/pkg/model/mexampleresp"
	"the-dev-tools/server/pkg/model/mexamplerespheader"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mflowvariable"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mitemfolder"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"the-dev-tools/server/pkg/model/mnnode/mnforeach"
	"the-dev-tools/server/pkg/model/mnnode/mnif"
	"the-dev-tools/server/pkg/model/mnnode/mnjs"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/model/mvar"
	"the-dev-tools/server/pkg/model/mworkspace"
	"time"
)

// ImportYamlFlowYAML converts simplified yamlflow YAML to ioworkspace.WorkspaceData
func ImportYamlFlowYAML(data []byte) (*ioworkspace.WorkspaceData, error) {
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

	// Copy for and for_each nodes directly
	workspaceData.FlowForNodes = resolved.FlowForNodes
	workspaceData.FlowForEachNodes = resolved.FlowForEachNodes

	return workspaceData, nil
}

// ImportYamlFlowYAMLMultiFlow converts yamlflow YAML with multiple flows to ioworkspace.WorkspaceData
func ImportYamlFlowYAMLMultiFlow(data []byte) (*ioworkspace.WorkspaceData, error) {
	// Generate workspace ID first
	workspaceID := idwrap.NewNow()

	// Parse the workflow to get basic structure
	var workflow YamlFlowFormat
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		return nil, fmt.Errorf("failed to unmarshal yamlflow format: %w", err)
	}

	if workflow.WorkspaceName == "" {
		return nil, fmt.Errorf("workspace_name is required")
	}

	// Create workspace data
	workspaceData := &ioworkspace.WorkspaceData{
		Workspace: mworkspace.Workspace{
			ID:   workspaceID,
			Name: workflow.WorkspaceName,
		},
		Collections:            make([]mcollection.Collection, 0),
		Folders:                make([]mitemfolder.ItemFolder, 0),
		Endpoints:              make([]mitemapi.ItemApi, 0),
		Examples:               make([]mitemapiexample.ItemApiExample, 0),
		ExampleHeaders:         make([]mexampleheader.Header, 0),
		ExampleQueries:         make([]mexamplequery.Query, 0),
		ExampleAsserts:         make([]massert.Assert, 0),
		Rawbodies:              make([]mbodyraw.ExampleBodyRaw, 0),
		FormBodies:             make([]mbodyform.BodyForm, 0),
		UrlBodies:              make([]mbodyurl.BodyURLEncoded, 0),
		ExampleResponses:       make([]mexampleresp.ExampleResp, 0),
		ExampleResponseHeaders: make([]mexamplerespheader.ExampleRespHeader, 0),
		ExampleResponseAsserts: make([]massertres.AssertResult, 0),
		Flows:                  make([]mflow.Flow, 0),
		FlowNodes:              make([]mnnode.MNode, 0),
		FlowEdges:              make([]edge.Edge, 0),
		FlowVariables:          make([]mflowvariable.FlowVariable, 0),
		FlowRequestNodes:       make([]mnrequest.MNRequest, 0),
		FlowConditionNodes:     make([]mnif.MNIF, 0),
		FlowNoopNodes:          make([]mnnoop.NoopNode, 0),
		FlowForNodes:           make([]mnfor.MNFor, 0),
		FlowForEachNodes:       make([]mnforeach.MNForEach, 0),
		FlowJSNodes:            make([]mnjs.MNJS, 0),
		Environments:           make([]menv.Env, 0),
		Variables:              make([]mvar.Var, 0),
	}

	// Create a default environment
	defaultEnv := menv.Env{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		Type:        menv.EnvNormal,
		Name:        "Default Environment",
		Description: "Default environment for imported workflows",
		Updated:     time.Now(),
	}
	workspaceData.Environments = append(workspaceData.Environments, defaultEnv)

	// Create one collection for all flows
	collectionID := idwrap.NewNow()
	collection := mcollection.Collection{
		ID:          collectionID,
		Name:        "Workflow Collection",
		WorkspaceID: workspaceID,
	}
	workspaceData.Collections = append(workspaceData.Collections, collection)

	// Process each flow
	for _, flowDef := range workflow.Flows {
		// Use ConvertSimplifiedYAML for each flow by creating a temporary workflow with single flow
		tempWorkflow := YamlFlowFormat{
			WorkspaceName:    workflow.WorkspaceName,
			RequestTemplates: workflow.RequestTemplates,
			Requests:         workflow.Requests,
			Flows:            []YamlFlowFlow{flowDef},
		}

		tempData, err := yaml.Marshal(tempWorkflow)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal temp workflow for flow %s: %w", flowDef.Name, err)
		}

		resolved, err := ConvertSimplifiedYAML(tempData, collectionID, workspaceID)
		if err != nil {
			return nil, fmt.Errorf("failed to convert flow %s: %w", flowDef.Name, err)
		}

		// Merge the resolved data into workspace data
		workspaceData.Flows = append(workspaceData.Flows, resolved.Flows...)
		workspaceData.FlowNodes = append(workspaceData.FlowNodes, resolved.FlowNodes...)
		workspaceData.FlowEdges = append(workspaceData.FlowEdges, resolved.FlowEdges...)
		workspaceData.FlowVariables = append(workspaceData.FlowVariables, resolved.FlowVariables...)
		workspaceData.FlowRequestNodes = append(workspaceData.FlowRequestNodes, resolved.FlowRequestNodes...)
		workspaceData.FlowConditionNodes = append(workspaceData.FlowConditionNodes, resolved.FlowConditionNodes...)
		workspaceData.FlowNoopNodes = append(workspaceData.FlowNoopNodes, resolved.FlowNoopNodes...)
		workspaceData.FlowForNodes = append(workspaceData.FlowForNodes, resolved.FlowForNodes...)
		workspaceData.FlowForEachNodes = append(workspaceData.FlowForEachNodes, resolved.FlowForEachNodes...)
		workspaceData.FlowJSNodes = append(workspaceData.FlowJSNodes, resolved.FlowJSNodes...)

		// Merge endpoints and examples, avoiding duplicates
		endpointMap := make(map[string]bool)
		for _, e := range workspaceData.Endpoints {
			endpointMap[e.Name] = true
		}

		for _, e := range resolved.Endpoints {
			if !endpointMap[e.Name] {
				workspaceData.Endpoints = append(workspaceData.Endpoints, e)
				endpointMap[e.Name] = true
			}
		}

		// Merge examples
		workspaceData.Examples = append(workspaceData.Examples, resolved.Examples...)
		workspaceData.ExampleHeaders = append(workspaceData.ExampleHeaders, resolved.Headers...)
		workspaceData.ExampleQueries = append(workspaceData.ExampleQueries, resolved.Queries...)
		workspaceData.Rawbodies = append(workspaceData.Rawbodies, resolved.RawBodies...)
	}

	// Set Prev/Next for endpoints
	for i := range workspaceData.Endpoints {
		if i > 0 {
			prevID := &workspaceData.Endpoints[i-1].ID
			workspaceData.Endpoints[i].Prev = prevID
		}
		if i < len(workspaceData.Endpoints)-1 {
			nextID := &workspaceData.Endpoints[i+1].ID
			workspaceData.Endpoints[i].Next = nextID
		}
	}

	// Set Prev/Next for examples
	for i := range workspaceData.Examples {
		if i > 0 {
			prevID := &workspaceData.Examples[i-1].ID
			workspaceData.Examples[i].Prev = prevID
		}
		if i < len(workspaceData.Examples)-1 {
			nextID := &workspaceData.Examples[i+1].ID
			workspaceData.Examples[i].Next = nextID
		}
	}

	return workspaceData, nil
}
