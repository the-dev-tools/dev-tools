//nolint:revive // exported
package yamlflowsimplev2

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/ioworkspace"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/varsystem"
)

// createStartNodeWithID creates a default start node with a specific ID
func createStartNodeWithID(nodeID, flowID idwrap.IDWrap, result *ioworkspace.WorkspaceBundle) {
	startNode := mflow.Node{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     "Start",
		NodeKind: mflow.NODE_KIND_NO_OP,
	}
	result.FlowNodes = append(result.FlowNodes, startNode)

	noopNode := mflow.NodeNoop{
		FlowNodeID: nodeID,
		Type:       mflow.NODE_NO_OP_KIND_START,
	}
	result.FlowNoopNodes = append(result.FlowNoopNodes, noopNode)
}

// processSteps processes all steps in a flow
func processSteps(flowEntry YamlFlowFlowV2, templates map[string]YamlRequestDefV2, varMap varsystem.VarMap, flowID, startNodeID idwrap.IDWrap, opts ConvertOptionsV2, result *ioworkspace.WorkspaceBundle) (*StepProcessingResult, error) {
	nodeInfoMap := make(map[string]*nodeInfo)
	nodeList := make([]*nodeInfo, 0)
	startNodeFound := false
	steps := flowEntry.Steps

	for i, stepWrapper := range steps {
		var nodeName string
		var dependsOn []string
		var nodeID = idwrap.NewNow()
		var info *nodeInfo

		switch {
		case stepWrapper.Request != nil:
			nodeName = stepWrapper.Request.Name
			dependsOn = stepWrapper.Request.DependsOn
		case stepWrapper.If != nil:
			nodeName = stepWrapper.If.Name
			dependsOn = stepWrapper.If.DependsOn
		case stepWrapper.For != nil:
			nodeName = stepWrapper.For.Name
			dependsOn = stepWrapper.For.DependsOn
		case stepWrapper.ForEach != nil:
			nodeName = stepWrapper.ForEach.Name
			dependsOn = stepWrapper.ForEach.DependsOn
		case stepWrapper.JS != nil:
			nodeName = stepWrapper.JS.Name
			dependsOn = stepWrapper.JS.DependsOn
		case stepWrapper.Noop != nil:
			nodeName = stepWrapper.Noop.Name
			dependsOn = stepWrapper.Noop.DependsOn
		default:
			return nil, NewYamlFlowErrorV2("empty step definition", "step", i)
		}

		if nodeName == "" {
			return nil, NewYamlFlowErrorV2("missing step name", "step", i)
		}

		info = &nodeInfo{
			id:        nodeID,
			name:      nodeName,
			index:     i,
			dependsOn: dependsOn,
		}

		switch {
		case stepWrapper.Request != nil:
			httpReq, associated, err := processRequestStep(nodeName, nodeID, flowID, stepWrapper.Request, templates, varMap, opts)
			if err != nil {
				return nil, err
			}
			info.httpReq = httpReq
			info.associated = associated
			result.HTTPRequests = append(result.HTTPRequests, *httpReq)
			if associated != nil {
				mergeAssociatedData(result, associated)
			}
			if opts.GenerateFiles {
				file := createFileForHTTP(*httpReq, opts)
				result.Files = append(result.Files, file)
			}
		case stepWrapper.If != nil:
			if stepWrapper.If.Condition == "" {
				return nil, NewYamlFlowErrorV2("missing required condition", "if", i)
			}
			if err := processIfStructStep(stepWrapper.If, nodeID, flowID, result); err != nil {
				return nil, err
			}
		case stepWrapper.For != nil:
			if err := processForStructStep(stepWrapper.For, nodeID, flowID, result); err != nil {
				return nil, err
			}
		case stepWrapper.ForEach != nil:
			if err := processForEachStructStep(stepWrapper.ForEach, nodeID, flowID, result); err != nil {
				return nil, err
			}
		case stepWrapper.JS != nil:
			if strings.TrimSpace(stepWrapper.JS.Code) == "" {
				return nil, NewYamlFlowErrorV2("missing required code", "js", i)
			}
			if err := processJSStructStep(stepWrapper.JS, nodeID, flowID, result); err != nil {
				return nil, err
			}
		case stepWrapper.Noop != nil:
			if stepWrapper.Noop.Type == "start" {
				info.id = startNodeID
				createStartNodeWithID(startNodeID, flowID, result)
				lastIdx := len(result.FlowNodes) - 1
				result.FlowNodes[lastIdx].Name = nodeName
				startNodeFound = true
				nodeInfoMap[nodeName] = info
				nodeList = append(nodeList, info)
				continue
			}
		}

		nodeInfoMap[nodeName] = info
		nodeList = append(nodeList, info)
	}

	return &StepProcessingResult{
		NodeInfoMap:    nodeInfoMap,
		NodeList:       nodeList,
		StartNodeFound: startNodeFound,
	}, nil
}

// processRequestStep processes a request step using struct
func processRequestStep(nodeName string, nodeID, flowID idwrap.IDWrap, step *YamlStepRequest, templates map[string]YamlRequestDefV2, varMap varsystem.VarMap, opts ConvertOptionsV2) (*mhttp.HTTP, *HTTPAssociatedData, error) {
	method := "GET"
	url := ""

	var templateDef YamlRequestDefV2
	usingTemplate := false

	if step.UseRequest != "" {
		if tmpl, ok := templates[step.UseRequest]; ok {
			templateDef = tmpl
			usingTemplate = true
			if tmpl.Method != "" {
				method = tmpl.Method
			}
			if tmpl.URL != "" {
				url = tmpl.URL
			}
		} else {
			return nil, nil, NewYamlFlowErrorV2(fmt.Sprintf("request step '%s' references unknown template '%s'", nodeName, step.UseRequest), "use_request", step.UseRequest)
		}
	}

	if step.Method != "" {
		method = step.Method
	}
	if step.URL != "" {
		url = step.URL
	}

	if url == "" {
		return nil, nil, NewYamlFlowErrorV2(fmt.Sprintf("request step '%s' missing required url", nodeName), "url", nil)
	}

	stepOverrides := YamlRequestDefV2{
		Method:      step.Method,
		URL:         step.URL,
		Headers:     step.Headers,
		QueryParams: step.QueryParams,
		Body:        step.Body,
		Assertions:  step.Assertions,
	}

	finalReq := mergeHTTPRequestDataStruct(templateDef, stepOverrides, usingTemplate)

	httpID := idwrap.NewNow()
	now := time.Now().UnixMilli()

	httpReq := &mhttp.HTTP{
		ID:           httpID,
		WorkspaceID:  opts.WorkspaceID,
		FolderID:     opts.FolderID,
		Name:         nodeName,
		Url:          url,
		Method:       method,
		Description:  finalReq.Description,
		ParentHttpID: opts.ParentHttpID,
		IsDelta:      opts.IsDelta,
		DeltaName:    opts.DeltaName,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	requestNode := mflow.NodeRequest{
		FlowNodeID:       nodeID,
		HttpID:           &httpID,
		HasRequestConfig: true,
	}

	flowNode := mflow.Node{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     nodeName,
		NodeKind: mflow.NODE_KIND_REQUEST,
	}

	associated := &HTTPAssociatedData{
		Headers:      convertToHTTPHeaders(finalReq.Headers, httpID),
		SearchParams: convertToHTTPSearchParams(finalReq.QueryParams, httpID),
		FlowNode:     &flowNode,
		RequestNode:  &requestNode,
	}

	if finalReq.Body != nil {
		bodyRaw, bodyForms, bodyUrlencoded, bodyKind := convertBodyStruct(finalReq.Body, httpID, opts)
		associated.BodyRaw = bodyRaw
		associated.BodyForms = bodyForms
		associated.BodyUrlencoded = bodyUrlencoded
		httpReq.BodyKind = bodyKind
	}

	return httpReq, associated, nil
}

func processIfStructStep(step *YamlStepIf, nodeID, flowID idwrap.IDWrap, result *ioworkspace.WorkspaceBundle) error {
	flowNode := mflow.Node{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     step.Name,
		NodeKind: mflow.NODE_KIND_CONDITION,
	}
	result.FlowNodes = append(result.FlowNodes, flowNode)

	cond := mflow.NodeIf{
		FlowNodeID: nodeID,
		Condition: mcondition.Condition{
			Comparisons: mcondition.Comparison{
				Expression: step.Condition,
			},
		},
	}
	result.FlowConditionNodes = append(result.FlowConditionNodes, cond)
	return nil
}

func processForStructStep(step *YamlStepFor, nodeID, flowID idwrap.IDWrap, result *ioworkspace.WorkspaceBundle) error {
	flowNode := mflow.Node{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     step.Name,
		NodeKind: mflow.NODE_KIND_FOR,
	}
	result.FlowNodes = append(result.FlowNodes, flowNode)

	// Parse iter count
	var iterCount int64
	if step.IterCount != "" {
		count, err := strconv.ParseInt(step.IterCount, 10, 64)
		if err != nil {
			return NewYamlFlowErrorV2(fmt.Sprintf("invalid iter_count value '%s': %v", step.IterCount, err), "iter_count", step.IterCount)
		}
		iterCount = count
	}

	forNode := mflow.NodeFor{
		FlowNodeID: nodeID,
		IterCount:  iterCount,
	}
	result.FlowForNodes = append(result.FlowForNodes, forNode)
	return nil
}

func processForEachStructStep(step *YamlStepForEach, nodeID, flowID idwrap.IDWrap, result *ioworkspace.WorkspaceBundle) error {
	flowNode := mflow.Node{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     step.Name,
		NodeKind: mflow.NODE_KIND_FOR_EACH,
	}
	result.FlowNodes = append(result.FlowNodes, flowNode)

	forEachNode := mflow.NodeForEach{
		FlowNodeID:     nodeID,
		IterExpression: step.Items,
	}
	result.FlowForEachNodes = append(result.FlowForEachNodes, forEachNode)
	return nil
}

func processJSStructStep(step *YamlStepJS, nodeID, flowID idwrap.IDWrap, result *ioworkspace.WorkspaceBundle) error {
	flowNode := mflow.Node{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     step.Name,
		NodeKind: mflow.NODE_KIND_JS,
	}
	result.FlowNodes = append(result.FlowNodes, flowNode)

	jsNode := mflow.NodeJS{
		FlowNodeID: nodeID,
		Code:       []byte(strings.TrimSpace(step.Code)),
	}
	result.FlowJSNodes = append(result.FlowJSNodes, jsNode)
	return nil
}
