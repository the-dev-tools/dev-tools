//nolint:revive // exported
package yamlflowsimplev2

import (
	"fmt"
	"strings"
	"time"

	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/ioworkspace"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/varsystem"
)

// processFlow processes a single flow and returns the generated data
func processFlow(flowEntry YamlFlowFlowV2, runEntries []YamlRunEntryV2, templates map[string]YamlRequestDefV2, opts ConvertOptionsV2) (*ioworkspace.WorkspaceBundle, error) {
	result := &ioworkspace.WorkspaceBundle{}

	flowID := idwrap.NewNow()

	flow := mflow.Flow{
		ID:          flowID,
		Name:        flowEntry.Name,
		WorkspaceID: opts.WorkspaceID,
	}
	result.Flows = append(result.Flows, flow)

	// Create file entries if generating files
	if opts.GenerateFiles {
		// Create file for the flow
		flowFile := mfile.File{
			ID:          flowID,
			WorkspaceID: opts.WorkspaceID,
			ParentID:    opts.FolderID,
			ContentID:   &flowID,
			ContentType: mfile.ContentTypeFlow,
			Name:        flowEntry.Name,
			Order:       float64(opts.FileOrder),
			UpdatedAt:   time.Now(),
		}
		result.Files = append(result.Files, flowFile)

		// Create folder for the flow's HTTP requests
		folderID := idwrap.NewNow()
		folderFile := mfile.File{
			ID:          folderID,
			WorkspaceID: opts.WorkspaceID,
			ParentID:    opts.FolderID,
			ContentID:   &folderID,
			ContentType: mfile.ContentTypeFolder,
			Name:        flowEntry.Name,
			Order:       float64(opts.FileOrder) + 1,
			UpdatedAt:   time.Now(),
		}
		result.Files = append(result.Files, folderFile)
		// Update opts to use this folder as parent for HTTP files
		opts.FolderID = &folderID
	}

	// Process flow variables
	varMap, err := processFlowVariables(flowEntry, flowID, result)
	if err != nil {
		return nil, fmt.Errorf("failed to process flow variables: %w", err)
	}

	startNodeID := idwrap.NewNow()

	// Process steps
	processRes, err := processSteps(flowEntry, templates, varMap, flowID, startNodeID, opts, result)
	if err != nil {
		return nil, fmt.Errorf("failed to process steps: %w", err)
	}

	// Create edges
	if err := createEdges(flowID, startNodeID, processRes.NodeInfoMap, processRes.NodeList, flowEntry.Steps, processRes.StartNodeFound, result); err != nil {
		return nil, fmt.Errorf("failed to create edges: %w", err)
	}

	return result, nil
}

// StepProcessingResult contains the result of processing flow steps
type StepProcessingResult struct {
	NodeInfoMap    map[string]*nodeInfo
	NodeList       []*nodeInfo
	StartNodeFound bool
}

// processFlowVariables processes flow variables and returns a variable map
func processFlowVariables(flowEntry YamlFlowFlowV2, flowID idwrap.IDWrap, result *ioworkspace.WorkspaceBundle) (varsystem.VarMap, error) {
	for _, variable := range flowEntry.Variables {
		flowVar := mflow.FlowVariable{
			ID:      idwrap.NewNow(),
			FlowID:  flowID,
			Name:    variable.Name,
			Value:   variable.Value,
			Enabled: true,
		}
		result.FlowVariables = append(result.FlowVariables, flowVar)
	}
	return varsystem.NewVarMap(nil), nil
}

// nodeInfo tracks information about a flow node
type nodeInfo struct {
	id         idwrap.IDWrap
	name       string
	index      int
	dependsOn  []string
	httpReq    *mhttp.HTTP
	associated *HTTPAssociatedData
}

// HTTPAssociatedData holds HTTP-related data
type HTTPAssociatedData struct {
	Headers        []mhttp.HTTPHeader
	SearchParams   []mhttp.HTTPSearchParam
	BodyRaw        mhttp.HTTPBodyRaw
	BodyForms      []mhttp.HTTPBodyForm
	BodyUrlencoded []mhttp.HTTPBodyUrlencoded
	FlowNode       *mflow.Node
	RequestNode    *mflow.NodeRequest
}

func createEdges(flowID, startNodeID idwrap.IDWrap, nodeInfoMap map[string]*nodeInfo, nodeList []*nodeInfo, steps []YamlStepWrapper, startNodeFound bool, result *ioworkspace.WorkspaceBundle) error {
	for _, node := range nodeList {
		for _, depName := range node.dependsOn {
			sourceName := depName
			handler := edge.HandleUnspecified

			// Check for dot notation (e.g., "Check.then")
			if strings.Contains(depName, ".") {
				parts := strings.Split(depName, ".")
				if len(parts) == 2 {
					sourceName = parts[0]
					switch parts[1] {
					case "then":
						handler = edge.HandleThen
					case "else":
						handler = edge.HandleElse
					case "loop":
						handler = edge.HandleLoop
					}
				}
			}

			targetInfo, ok := nodeInfoMap[sourceName]
			if !ok {
				return NewYamlFlowErrorV2(fmt.Sprintf("step '%s' depends on unknown step '%s'", node.name, sourceName), "depends_on", sourceName)
			}
			result.FlowEdges = append(result.FlowEdges, createEdge(targetInfo.id, node.id, flowID, handler))
		}

		step := steps[node.index]

		if step.If != nil {
			if step.If.Then != "" {
				target, ok := nodeInfoMap[step.If.Then]
				if !ok {
					return NewYamlFlowErrorV2("if 'then' target not found", "then", step.If.Then)
				}
				result.FlowEdges = append(result.FlowEdges, createEdge(node.id, target.id, flowID, edge.HandleThen))
			}
			if step.If.Else != "" {
				target, ok := nodeInfoMap[step.If.Else]
				if !ok {
					return NewYamlFlowErrorV2("if 'else' target not found", "else", step.If.Else)
				}
				result.FlowEdges = append(result.FlowEdges, createEdge(node.id, target.id, flowID, edge.HandleElse))
			}
		}

		if step.For != nil {
			if step.For.Loop != "" {
				target, ok := nodeInfoMap[step.For.Loop]
				if !ok {
					return NewYamlFlowErrorV2("for 'loop' target not found", "loop", step.For.Loop)
				}
				result.FlowEdges = append(result.FlowEdges, createEdge(node.id, target.id, flowID, edge.HandleLoop))
			}
		}

		if step.ForEach != nil {
			if step.ForEach.Loop != "" {
				target, ok := nodeInfoMap[step.ForEach.Loop]
				if !ok {
					return NewYamlFlowErrorV2("for_each 'loop' target not found", "loop", step.ForEach.Loop)
				}
				result.FlowEdges = append(result.FlowEdges, createEdge(node.id, target.id, flowID, edge.HandleLoop))
			}
		}

		// Only auto-connect nodes to start if there's no explicit start node in the YAML
		// When an explicit start node exists, disconnected nodes should remain disconnected (won't run)
		if len(node.dependsOn) == 0 {
			if node.id != startNodeID && !startNodeFound {
				result.FlowEdges = append(result.FlowEdges, createEdge(startNodeID, node.id, flowID, edge.HandleUnspecified))
			}
		}
	}
	return nil
}

func createEdge(source, target, flowID idwrap.IDWrap, handler edge.EdgeHandle) edge.Edge {
	return edge.Edge{
		ID:            idwrap.NewNow(),
		FlowID:        flowID,
		SourceID:      source,
		TargetID:      target,
		SourceHandler: handler,
	}
}

// Helpers for data merging (restored)

func mergeFlowData(result *ioworkspace.WorkspaceBundle, flowData *ioworkspace.WorkspaceBundle, opts ConvertOptionsV2) {
	result.Flows = append(result.Flows, flowData.Flows...)
	result.FlowNodes = append(result.FlowNodes, flowData.FlowNodes...)
	result.FlowEdges = append(result.FlowEdges, flowData.FlowEdges...)
	result.FlowVariables = append(result.FlowVariables, flowData.FlowVariables...)
	result.Files = append(result.Files, flowData.Files...)

	result.HTTPRequests = append(result.HTTPRequests, flowData.HTTPRequests...)
	result.HTTPHeaders = append(result.HTTPHeaders, flowData.HTTPHeaders...)
	result.HTTPSearchParams = append(result.HTTPSearchParams, flowData.HTTPSearchParams...)
	result.HTTPBodyRaw = append(result.HTTPBodyRaw, flowData.HTTPBodyRaw...)
	result.HTTPBodyForms = append(result.HTTPBodyForms, flowData.HTTPBodyForms...)
	result.HTTPBodyUrlencoded = append(result.HTTPBodyUrlencoded, flowData.HTTPBodyUrlencoded...)
	result.HTTPAsserts = append(result.HTTPAsserts, flowData.HTTPAsserts...)

	result.FlowConditionNodes = append(result.FlowConditionNodes, flowData.FlowConditionNodes...)
	result.FlowForNodes = append(result.FlowForNodes, flowData.FlowForNodes...)
	result.FlowForEachNodes = append(result.FlowForEachNodes, flowData.FlowForEachNodes...)
	result.FlowJSNodes = append(result.FlowJSNodes, flowData.FlowJSNodes...)
	result.FlowNoopNodes = append(result.FlowNoopNodes, flowData.FlowNoopNodes...)
	result.FlowRequestNodes = append(result.FlowRequestNodes, flowData.FlowRequestNodes...)
}

func mergeAssociatedData(result *ioworkspace.WorkspaceBundle, assoc *HTTPAssociatedData) {
	if assoc == nil {
		return
	}
	result.HTTPHeaders = append(result.HTTPHeaders, assoc.Headers...)
	result.HTTPSearchParams = append(result.HTTPSearchParams, assoc.SearchParams...)
	if assoc.BodyRaw.ID != (idwrap.IDWrap{}) {
		result.HTTPBodyRaw = append(result.HTTPBodyRaw, assoc.BodyRaw)
	}
	result.HTTPBodyForms = append(result.HTTPBodyForms, assoc.BodyForms...)
	result.HTTPBodyUrlencoded = append(result.HTTPBodyUrlencoded, assoc.BodyUrlencoded...)

	if assoc.FlowNode != nil {
		result.FlowNodes = append(result.FlowNodes, *assoc.FlowNode)
	}
	if assoc.RequestNode != nil {
		result.FlowRequestNodes = append(result.FlowRequestNodes, *assoc.RequestNode)
	}
}

func createFileForHTTP(httpReq mhttp.HTTP, opts ConvertOptionsV2) mfile.File {
	return mfile.File{
		ID:          httpReq.ID,
		WorkspaceID: opts.WorkspaceID,
		ParentID:    opts.FolderID,
		ContentID:   &httpReq.ID,
		ContentType: mfile.ContentTypeHTTP,
		Name:        httpReq.Name,
		Order:       GenerateFileOrder(nil), // Should track order properly if strict
		UpdatedAt:   time.Now(),
	}
}
