package yamlflowsimplev2

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/ioworkspace"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mflowvariable"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"the-dev-tools/server/pkg/model/mnnode/mnforeach"
	"the-dev-tools/server/pkg/model/mnnode/mnif"
	"the-dev-tools/server/pkg/model/mnnode/mnjs"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/varsystem"
)

// ConvertSimplifiedYAML converts simplified YAML to modern HTTP and flow models
func ConvertSimplifiedYAML(data []byte, opts ConvertOptionsV2) (*ioworkspace.WorkspaceBundle, error) {
	// Validate options
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid options: %w", err)
	}

	// Parse YAML to get structured data
	yamlFormat, err := parseYAMLData(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Validate YAML structure
	if err := yamlFormat.Validate(); err != nil {
		return nil, fmt.Errorf("invalid YAML structure: %w", err)
	}

	// Validate references via utility
	if err := ValidateYAMLStructure(yamlFormat); err != nil {
		return nil, fmt.Errorf("invalid YAML semantics: %w", err)
	}

	// Initialize resolved data structure with workspace metadata
	result := &ioworkspace.WorkspaceBundle{
		Workspace: mworkspace.Workspace{
			ID:   opts.WorkspaceID,
			Name: yamlFormat.WorkspaceName,
		},
	}

	// Prepare request templates map from both Sources
	requestTemplates := make(map[string]YamlRequestDefV2)
	for k, v := range yamlFormat.RequestTemplates {
		requestTemplates[k] = v
	}
	for _, req := range yamlFormat.Requests {
		if req.Name != "" {
			requestTemplates[req.Name] = req
		}
	}

	// Process flows and generate HTTP requests
	for _, flowEntry := range yamlFormat.Flows {
		flowData, err := processFlow(flowEntry, yamlFormat.Run, requestTemplates, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to process flow '%s': %w", flowEntry.Name, err)
		}

		// Merge flow data into result
		mergeFlowData(result, flowData, opts)
	}

	// Ensure all flows have proper structure (start nodes, edges, positioning)
	if err := result.EnsureFlowStructure(); err != nil {
		return nil, fmt.Errorf("failed to ensure flow structure: %w", err)
	}

	return result, nil
}

// parseYAMLData parses YAML data into structured format
func parseYAMLData(data []byte) (*YamlFlowFormatV2, error) {
	var yamlFormat YamlFlowFormatV2
	if err := yaml.Unmarshal(data, &yamlFormat); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %w", err)
	}
	return &yamlFormat, nil
}

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
		flowVar := mflowvariable.FlowVariable{
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
	FlowNode       *mnnode.MNode
	RequestNode    *mnrequest.MNRequest
}

// createStartNodeWithID creates a default start node with a specific ID
func createStartNodeWithID(nodeID, flowID idwrap.IDWrap, result *ioworkspace.WorkspaceBundle) {
	startNode := mnnode.MNode{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     "Start",
		NodeKind: mnnode.NODE_KIND_NO_OP,
	}
	result.FlowNodes = append(result.FlowNodes, startNode)

	noopNode := mnnoop.NoopNode{
		FlowNodeID: nodeID,
		Type:       mnnoop.NODE_NO_OP_KIND_START,
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
		var nodeID idwrap.IDWrap = idwrap.NewNow()
		var info *nodeInfo

		if stepWrapper.Request != nil {
			nodeName = stepWrapper.Request.Name
			dependsOn = stepWrapper.Request.DependsOn
		} else if stepWrapper.If != nil {
			nodeName = stepWrapper.If.Name
			dependsOn = stepWrapper.If.DependsOn
		} else if stepWrapper.For != nil {
			nodeName = stepWrapper.For.Name
			dependsOn = stepWrapper.For.DependsOn
		} else if stepWrapper.ForEach != nil {
			nodeName = stepWrapper.ForEach.Name
			dependsOn = stepWrapper.ForEach.DependsOn
		} else if stepWrapper.JS != nil {
			nodeName = stepWrapper.JS.Name
			dependsOn = stepWrapper.JS.DependsOn
		} else if stepWrapper.Noop != nil {
			nodeName = stepWrapper.Noop.Name
			dependsOn = stepWrapper.Noop.DependsOn
		} else {
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

		if stepWrapper.Request != nil {
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
		} else if stepWrapper.If != nil {
			if stepWrapper.If.Condition == "" {
				return nil, NewYamlFlowErrorV2("missing required condition", "if", i)
			}
			if err := processIfStructStep(stepWrapper.If, nodeID, flowID, result); err != nil {
				return nil, err
			}
		} else if stepWrapper.For != nil {
			if err := processForStructStep(stepWrapper.For, nodeID, flowID, result); err != nil {
				return nil, err
			}
		} else if stepWrapper.ForEach != nil {
			if err := processForEachStructStep(stepWrapper.ForEach, nodeID, flowID, result); err != nil {
				return nil, err
			}
		} else if stepWrapper.JS != nil {
			if strings.TrimSpace(stepWrapper.JS.Code) == "" {
				return nil, NewYamlFlowErrorV2("missing required code", "js", i)
			}
			if err := processJSStructStep(stepWrapper.JS, nodeID, flowID, result); err != nil {
				return nil, err
			}
		} else if stepWrapper.Noop != nil {
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

	requestNode := mnrequest.MNRequest{
		FlowNodeID:       nodeID,
		HttpID:           &httpID,
		HasRequestConfig: true,
	}

	flowNode := mnnode.MNode{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     nodeName,
		NodeKind: mnnode.NODE_KIND_REQUEST,
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
	flowNode := mnnode.MNode{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     step.Name,
		NodeKind: mnnode.NODE_KIND_CONDITION,
	}
	result.FlowNodes = append(result.FlowNodes, flowNode)

	cond := mnif.MNIF{
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
	flowNode := mnnode.MNode{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     step.Name,
		NodeKind: mnnode.NODE_KIND_FOR,
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

	forNode := mnfor.MNFor{
		FlowNodeID: nodeID,
		IterCount:  iterCount,
	}
	result.FlowForNodes = append(result.FlowForNodes, forNode)
	return nil
}

func processForEachStructStep(step *YamlStepForEach, nodeID, flowID idwrap.IDWrap, result *ioworkspace.WorkspaceBundle) error {
	flowNode := mnnode.MNode{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     step.Name,
		NodeKind: mnnode.NODE_KIND_FOR_EACH,
	}
	result.FlowNodes = append(result.FlowNodes, flowNode)

	forEachNode := mnforeach.MNForEach{
		FlowNodeID:     nodeID,
		IterExpression: step.Items,
	}
	result.FlowForEachNodes = append(result.FlowForEachNodes, forEachNode)
	return nil
}

func processJSStructStep(step *YamlStepJS, nodeID, flowID idwrap.IDWrap, result *ioworkspace.WorkspaceBundle) error {
	flowNode := mnnode.MNode{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     step.Name,
		NodeKind: mnnode.NODE_KIND_JS,
	}
	result.FlowNodes = append(result.FlowNodes, flowNode)

	jsNode := mnjs.MNJS{
		FlowNodeID: nodeID,
		Code:       []byte(strings.TrimSpace(step.Code)),
	}
	result.FlowJSNodes = append(result.FlowJSNodes, jsNode)
	return nil
}

func mergeHTTPRequestDataStruct(base, override YamlRequestDefV2, usingTemplate bool) YamlRequestDefV2 {
	if !usingTemplate {
		return override
	}
	merged := base
	if override.Method != "" {
		merged.Method = override.Method
	}
	if override.URL != "" {
		merged.URL = override.URL
	}
	if override.Description != "" {
		merged.Description = override.Description
	}
	if override.Body != nil {
		merged.Body = override.Body
	}

	if len(override.Headers) > 0 {
		merged.Headers = append(merged.Headers, override.Headers...)
	}
	if len(override.QueryParams) > 0 {
		merged.QueryParams = append(merged.QueryParams, override.QueryParams...)
	}
	if len(override.Assertions) > 0 {
		merged.Assertions = append(merged.Assertions, override.Assertions...)
	}
	return merged
}

func convertToHTTPHeaders(yamlHeaders []YamlNameValuePairV2, httpID idwrap.IDWrap) []mhttp.HTTPHeader {
	var headers []mhttp.HTTPHeader
	for _, h := range yamlHeaders {
		headers = append(headers, mhttp.HTTPHeader{
			ID:      idwrap.NewNow(),
			HttpID:  httpID,
			Key:     h.Name,
			Value:   h.Value,
			Enabled: h.Enabled,
		})
	}
	return headers
}

func convertToHTTPSearchParams(yamlParams []YamlNameValuePairV2, httpID idwrap.IDWrap) []mhttp.HTTPSearchParam {
	var params []mhttp.HTTPSearchParam
	for _, p := range yamlParams {
		params = append(params, mhttp.HTTPSearchParam{
			ID:      idwrap.NewNow(),
			HttpID:  httpID,
			Key:     p.Name,
			Value:   p.Value,
			Enabled: p.Enabled,
		})
	}
	return params
}

func convertBodyStruct(body *YamlBodyUnion, httpID idwrap.IDWrap, opts ConvertOptionsV2) (mhttp.HTTPBodyRaw, []mhttp.HTTPBodyForm, []mhttp.HTTPBodyUrlencoded, mhttp.HttpBodyKind) {
	bodyRaw := mhttp.HTTPBodyRaw{
		ID:     idwrap.NewNow(),
		HttpID: httpID,
	}
	var bodyForms []mhttp.HTTPBodyForm
	var bodyUrlencoded []mhttp.HTTPBodyUrlencoded
	bodyKind := mhttp.HttpBodyKindRaw

	if body == nil {
		return bodyRaw, nil, nil, bodyKind
	}

	switch strings.ToLower(body.Type) {
	case "form-data":
		bodyKind = mhttp.HttpBodyKindFormData
		for _, form := range body.Form {
			bodyForms = append(bodyForms, mhttp.HTTPBodyForm{
				ID:      idwrap.NewNow(),
				HttpID:  httpID,
				Key:     form.Name,
				Value:   form.Value,
				Enabled: form.Enabled,
			})
		}
	case "urlencoded":
		bodyKind = mhttp.HttpBodyKindUrlEncoded
		for _, urlEncoded := range body.UrlEncoded {
			bodyUrlencoded = append(bodyUrlencoded, mhttp.HTTPBodyUrlencoded{
				ID:      idwrap.NewNow(),
				HttpID:  httpID,
				Key:     urlEncoded.Name,
				Value:   urlEncoded.Value,
				Enabled: urlEncoded.Enabled,
			})
		}
	case "json":
		bodyKind = mhttp.HttpBodyKindRaw
		if body.JSON != nil {
			jb, _ := json.Marshal(body.JSON)
			bodyRaw.RawData = jb
			bodyRaw.ContentType = "application/json"
		}
	case "raw":
		bodyKind = mhttp.HttpBodyKindRaw
		bodyRaw.RawData = []byte(body.Raw)
	default:
		bodyKind = mhttp.HttpBodyKindRaw
		bodyRaw.RawData = []byte(body.Raw)
	}

	if body.Compression != "" {
		if ct, ok := compress.CompressLockupMap[body.Compression]; ok {
			bodyRaw.CompressionType = ct
		}
	} else if opts.EnableCompression && len(bodyRaw.RawData) > 1024 {
		// Auto-compress only if larger than threshold
		compressed, err := compress.Compress(bodyRaw.RawData, opts.CompressionType)
		if err == nil {
			bodyRaw.RawData = compressed
			bodyRaw.CompressionType = opts.CompressionType
		}
	}

	return bodyRaw, bodyForms, bodyUrlencoded, bodyKind
}

func createEdges(flowID, startNodeID idwrap.IDWrap, nodeInfoMap map[string]*nodeInfo, nodeList []*nodeInfo, steps []YamlStepWrapper, startNodeFound bool, result *ioworkspace.WorkspaceBundle) error {
	for _, node := range nodeList {
		for _, depName := range node.dependsOn {
			targetInfo, ok := nodeInfoMap[depName]
			if !ok {
				return NewYamlFlowErrorV2(fmt.Sprintf("step '%s' depends on unknown step '%s'", node.name, depName), "depends_on", depName)
			}
			result.FlowEdges = append(result.FlowEdges, createEdge(targetInfo.id, node.id, flowID, edge.HandleUnspecified))
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

		if len(node.dependsOn) == 0 && node.index > 0 {
			prevNode := nodeList[node.index-1]
			result.FlowEdges = append(result.FlowEdges, createEdge(prevNode.id, node.id, flowID, edge.HandleUnspecified))
		} else if node.index == 0 && !startNodeFound {
			if node.id != startNodeID {
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
