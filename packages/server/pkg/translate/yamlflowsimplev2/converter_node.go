//nolint:revive // exported
package yamlflowsimplev2

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/ioworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mcondition"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mgraphql"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/varsystem"
)

// createStartNodeWithID creates a default start node with a specific ID
func createStartNodeWithID(nodeID, flowID idwrap.IDWrap, result *ioworkspace.WorkspaceBundle) {
	startNode := mflow.Node{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     "Start",
		NodeKind: mflow.NODE_KIND_MANUAL_START,
	}
	result.FlowNodes = append(result.FlowNodes, startNode)
}

// processSteps processes all steps in a flow
func processSteps(flowEntry YamlFlowFlowV2, templates map[string]YamlRequestDefV2, graphqlTemplates map[string]YamlGraphQLDefV2, varMap varsystem.VarMap, flowID, startNodeID idwrap.IDWrap, opts ConvertOptionsV2, result *ioworkspace.WorkspaceBundle) (*StepProcessingResult, error) {
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
		case stepWrapper.GraphQL != nil:
			nodeName = stepWrapper.GraphQL.Name
			dependsOn = stepWrapper.GraphQL.DependsOn
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
		case stepWrapper.AI != nil:
			nodeName = stepWrapper.AI.Name
			dependsOn = stepWrapper.AI.DependsOn
		case stepWrapper.AIProvider != nil:
			nodeName = stepWrapper.AIProvider.Name
			dependsOn = stepWrapper.AIProvider.DependsOn
		case stepWrapper.AIMemory != nil:
			nodeName = stepWrapper.AIMemory.Name
			dependsOn = stepWrapper.AIMemory.DependsOn
		case stepWrapper.ManualStart != nil:
			nodeName = stepWrapper.ManualStart.Name
			dependsOn = stepWrapper.ManualStart.DependsOn
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
		case stepWrapper.GraphQL != nil:
			if err := processGraphQLStructStep(stepWrapper.GraphQL, nodeID, flowID, graphqlTemplates, opts, result); err != nil {
				return nil, err
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
		case stepWrapper.AI != nil:
			if strings.TrimSpace(stepWrapper.AI.Prompt) == "" {
				return nil, NewYamlFlowErrorV2("missing required prompt", "ai", i)
			}
			if err := processAIStructStep(stepWrapper.AI, nodeID, flowID, opts, result); err != nil {
				return nil, err
			}
			// Store AI-specific references for edge creation
			info.aiProvider = stepWrapper.AI.Provider
			info.aiMemory = stepWrapper.AI.Memory
			info.aiTools = stepWrapper.AI.Tools
		case stepWrapper.AIProvider != nil:
			if stepWrapper.AIProvider.Credential == "" {
				return nil, NewYamlFlowErrorV2("missing required credential", "ai_provider", i)
			}
			if stepWrapper.AIProvider.Model == "" {
				return nil, NewYamlFlowErrorV2("missing required model", "ai_provider", i)
			}
			if err := processAIProviderStructStep(stepWrapper.AIProvider, nodeID, flowID, opts, result); err != nil {
				return nil, err
			}
		case stepWrapper.AIMemory != nil:
			if err := processAIMemoryStructStep(stepWrapper.AIMemory, nodeID, flowID, result); err != nil {
				return nil, err
			}
		case stepWrapper.ManualStart != nil:
			info.id = startNodeID
			createStartNodeWithID(startNodeID, flowID, result)
			lastIdx := len(result.FlowNodes) - 1
			result.FlowNodes[lastIdx].Name = nodeName
			startNodeFound = true
			nodeInfoMap[nodeName] = info
			nodeList = append(nodeList, info)
			continue
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

func processAIStructStep(step *YamlStepAI, nodeID, flowID idwrap.IDWrap, _ ConvertOptionsV2, result *ioworkspace.WorkspaceBundle) error {
	flowNode := mflow.Node{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     step.Name,
		NodeKind: mflow.NODE_KIND_AI,
	}
	result.FlowNodes = append(result.FlowNodes, flowNode)

	// Default max iterations to 5 if not specified
	maxIterations := step.MaxIterations
	if maxIterations <= 0 {
		maxIterations = 5
	}
	// Cap max iterations to prevent overflow
	if maxIterations > 100 {
		maxIterations = 100
	}

	aiNode := mflow.NodeAI{
		FlowNodeID:    nodeID,
		Prompt:        step.Prompt,
		MaxIterations: int32(maxIterations), //nolint:gosec // validated above
	}
	result.FlowAINodes = append(result.FlowAINodes, aiNode)
	return nil
}

func processAIProviderStructStep(step *YamlStepAIProvider, nodeID, flowID idwrap.IDWrap, opts ConvertOptionsV2, result *ioworkspace.WorkspaceBundle) error {
	flowNode := mflow.Node{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     step.Name,
		NodeKind: mflow.NODE_KIND_AI_PROVIDER,
	}
	result.FlowNodes = append(result.FlowNodes, flowNode)

	// Resolve credential name to ID
	var credentialID *idwrap.IDWrap
	if opts.CredentialMap != nil {
		if id, ok := opts.CredentialMap[step.Credential]; ok {
			credentialID = &id
		}
	}

	// Parse model string to AiModel enum
	model := mflow.AiModelFromString(step.Model)
	if model == mflow.AiModelCustom && step.CustomModel == "" && step.Model != "custom" {
		// Model string didn't match known models, use it as custom model
		model = mflow.AiModelCustom
	}

	// Convert temperature from float64 to float32
	var temperature *float32
	if step.Temperature != nil {
		t := float32(*step.Temperature)
		temperature = &t
	}

	providerNode := mflow.NodeAiProvider{
		FlowNodeID:   nodeID,
		CredentialID: credentialID,
		Model:        model,
		Temperature:  temperature,
		MaxTokens:    step.MaxTokens,
	}
	result.FlowAIProviderNodes = append(result.FlowAIProviderNodes, providerNode)
	return nil
}

func processAIMemoryStructStep(step *YamlStepAIMemory, nodeID, flowID idwrap.IDWrap, result *ioworkspace.WorkspaceBundle) error {
	flowNode := mflow.Node{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     step.Name,
		NodeKind: mflow.NODE_KIND_AI_MEMORY,
	}
	result.FlowNodes = append(result.FlowNodes, flowNode)

	// Parse memory type
	memoryType := mflow.AiMemoryTypeWindowBuffer // default
	if step.Type == MemoryTypeSummary {
		// Future: add summary memory type when supported
		memoryType = mflow.AiMemoryTypeWindowBuffer
	}

	// Default window size to 10 if not specified
	windowSize := step.WindowSize
	if windowSize <= 0 {
		windowSize = 10
	}

	memoryNode := mflow.NodeMemory{
		FlowNodeID: nodeID,
		MemoryType: memoryType,
		WindowSize: int32(windowSize), //nolint:gosec // validated above
	}
	result.FlowAIMemoryNodes = append(result.FlowAIMemoryNodes, memoryNode)
	return nil
}

func processGraphQLStructStep(step *YamlStepGraphQL, nodeID, flowID idwrap.IDWrap, templates map[string]YamlGraphQLDefV2, opts ConvertOptionsV2, result *ioworkspace.WorkspaceBundle) error {
	url := step.URL
	query := step.Query
	variables := step.Variables
	var headers HeaderMapOrSlice

	if step.UseRequest != "" {
		if tmpl, ok := templates[step.UseRequest]; ok {
			if tmpl.URL != "" {
				url = tmpl.URL
			}
			if tmpl.Query != "" {
				query = tmpl.Query
			}
			if tmpl.Variables != "" {
				variables = tmpl.Variables
			}
			headers = tmpl.Headers
		} else {
			return NewYamlFlowErrorV2(fmt.Sprintf("graphql step '%s' references unknown template '%s'", step.Name, step.UseRequest), "use_request", step.UseRequest)
		}
	}

	// Step-level values override template
	if step.URL != "" {
		url = step.URL
	}
	if step.Query != "" {
		query = step.Query
	}
	if step.Variables != "" {
		variables = step.Variables
	}
	if len(step.Headers) > 0 {
		headers = append(headers, step.Headers...)
	}

	if url == "" {
		return NewYamlFlowErrorV2(fmt.Sprintf("graphql step '%s' missing required url", step.Name), "url", nil)
	}

	gqlID := idwrap.NewNow()
	now := time.Now().UnixMilli()

	gqlReq := mgraphql.GraphQL{
		ID:          gqlID,
		WorkspaceID: opts.WorkspaceID,
		FolderID:    opts.FolderID,
		Name:        step.Name,
		Url:         url,
		Query:       query,
		Variables:   variables,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	result.GraphQLRequests = append(result.GraphQLRequests, gqlReq)

	// Create headers
	for i, h := range headers {
		header := mgraphql.GraphQLHeader{
			ID:           idwrap.NewNow(),
			GraphQLID:    gqlID,
			Key:          h.Name,
			Value:        h.Value,
			Enabled:      h.Enabled,
			DisplayOrder: float32(i),
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		result.GraphQLHeaders = append(result.GraphQLHeaders, header)
	}

	// Create flow node
	flowNode := mflow.Node{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     step.Name,
		NodeKind: mflow.NODE_KIND_GRAPHQL,
	}
	result.FlowNodes = append(result.FlowNodes, flowNode)

	// Create GraphQL node linking flow node to GraphQL entity
	graphqlNode := mflow.NodeGraphQL{
		FlowNodeID: nodeID,
		GraphQLID:  &gqlID,
	}
	result.FlowGraphQLNodes = append(result.FlowGraphQLNodes, graphqlNode)

	return nil
}
