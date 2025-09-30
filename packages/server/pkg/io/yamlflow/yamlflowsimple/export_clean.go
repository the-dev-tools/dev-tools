package yamlflowsimple

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/http/request"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/ioworkspace"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/model/mbodyform"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mbodyurl"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mexamplequery"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"the-dev-tools/server/pkg/model/mnnode/mnforeach"
	"the-dev-tools/server/pkg/model/mnnode/mnif"
	"the-dev-tools/server/pkg/model/mnnode/mnjs"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/model/mvar"
	"the-dev-tools/server/pkg/overlay/merge"
	"the-dev-tools/server/pkg/overlay/resolve"
)

// ExportYamlFlowYAML converts ioworkspace.WorkspaceData to simplified yamlflow YAML.
// Overlay manager may be nil when delta resolution is unnecessary.
func ExportYamlFlowYAML(ctx context.Context, workspaceData *ioworkspace.WorkspaceData, overlayMgr *merge.Manager) ([]byte, error) {
	if workspaceData == nil {
		return nil, fmt.Errorf("workspace data cannot be nil")
	}

	if len(workspaceData.Flows) == 0 {
		return nil, fmt.Errorf("no flows to export")
	}

	// Build request definitions from all endpoints across all flows
	requests, err := buildRequestDefinitions(ctx, workspaceData, overlayMgr)
	if err != nil {
		return nil, err
	}

	// Build flows
	flows := []map[string]any{}
	for _, flow := range workspaceData.Flows {
		flowData, err := exportFlow(ctx, flow, workspaceData, requests, overlayMgr)
		if err != nil {
			return nil, err
		}
		if flowData != nil {
			flows = append(flows, flowData)
		}
	}

	// Build final YAML structure using ordered approach
	var doc yaml.Node
	doc.Kind = yaml.DocumentNode

	var root yaml.Node
	root.Kind = yaml.MappingNode

	// Add workspace_name
	root.Content = append(root.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "workspace_name"},
		&yaml.Node{Kind: yaml.ScalarNode, Value: workspaceData.Workspace.Name})

	activeEnvName, globalEnvName, environmentsNode, err := buildEnvironmentNodes(workspaceData)
	if err != nil {
		return nil, err
	}

	if activeEnvName != "" {
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "active_environment"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: activeEnvName},
		)
	}

	if globalEnvName != "" {
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "global_environment"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: globalEnvName},
		)
	}

	if environmentsNode != nil && len(environmentsNode.Content) > 0 {
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "environments"},
			environmentsNode,
		)
	}

	// Add run field with all exported flows
	runEntries := buildRunEntries(workspaceData)
	if len(runEntries) > 0 {
		var runNode yaml.Node
		runNode.Kind = yaml.SequenceNode
		for _, entry := range runEntries {
			runNode.Content = append(runNode.Content, createRunEntryNode(entry))
		}
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "run"},
			&runNode)
	}

	// Add requests section if not empty
	if len(requests) > 0 {
		requestList := make([]map[string]any, 0, len(requests))
		for _, req := range requests {
			requestList = append(requestList, req)
		}
		// Sort by name for consistent output
		sort.Slice(requestList, func(i, j int) bool {
			nameI, _ := requestList[i]["name"].(string)
			nameJ, _ := requestList[j]["name"].(string)
			return nameI < nameJ
		})

		// Create requests array node
		var requestsNode yaml.Node
		requestsNode.Kind = yaml.SequenceNode
		for _, req := range requestList {
			requestsNode.Content = append(requestsNode.Content, createOrderedRequestNode(req))
		}

		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "requests"},
			&requestsNode)
	}

	// Add flows
	var flowsNode yaml.Node
	flowsNode.Kind = yaml.SequenceNode
	for _, flow := range flows {
		flowsNode.Content = append(flowsNode.Content, createOrderedFlowNode(flow))
	}
	root.Content = append(root.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "flows"},
		&flowsNode)

	doc.Content = append(doc.Content, &root)
	return yaml.Marshal(&doc)
}

func buildEnvironmentNodes(workspaceData *ioworkspace.WorkspaceData) (activeEnvName string, globalEnvName string, environmentsNode *yaml.Node, err error) {
	if len(workspaceData.Environments) == 0 {
		return "", "", nil, nil
	}

	envByID := make(map[idwrap.IDWrap]menv.Env, len(workspaceData.Environments))
	envs := make([]menv.Env, len(workspaceData.Environments))
	copy(envs, workspaceData.Environments)
	for _, env := range envs {
		envByID[env.ID] = env
	}

	sort.Slice(envs, func(i, j int) bool {
		if envs[i].Type != envs[j].Type {
			return envs[i].Type < envs[j].Type
		}
		return strings.ToLower(envs[i].Name) < strings.ToLower(envs[j].Name)
	})

	varsByEnv := make(map[idwrap.IDWrap][]mvar.Var)
	for _, v := range workspaceData.Variables {
		varsByEnv[v.EnvID] = append(varsByEnv[v.EnvID], v)
	}

	environmentsNode = &yaml.Node{Kind: yaml.SequenceNode}
	for _, env := range envs {
		if env.ID == workspaceData.Workspace.ActiveEnv {
			activeEnvName = env.Name
		}
		if env.ID == workspaceData.Workspace.GlobalEnv {
			globalEnvName = env.Name
		}

		envNode := &yaml.Node{Kind: yaml.MappingNode}
		envNode.Content = append(envNode.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "name"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: env.Name},
		)

		typeValue := exportEnvType(env.Type)
		if typeValue != "" {
			envNode.Content = append(envNode.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: "type"},
				&yaml.Node{Kind: yaml.ScalarNode, Value: typeValue},
			)
		}

		if env.Description != "" {
			envNode.Content = append(envNode.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: "description"},
				&yaml.Node{Kind: yaml.ScalarNode, Value: env.Description},
			)
		}

		vars := varsByEnv[env.ID]
		if len(vars) > 0 {
			sort.Slice(vars, func(i, j int) bool {
				return strings.ToLower(vars[i].VarKey) < strings.ToLower(vars[j].VarKey)
			})

			varSeq := &yaml.Node{Kind: yaml.SequenceNode}
			for _, variable := range vars {
				varNode := &yaml.Node{Kind: yaml.MappingNode}
				varNode.Content = append(varNode.Content,
					&yaml.Node{Kind: yaml.ScalarNode, Value: "key"},
					&yaml.Node{Kind: yaml.ScalarNode, Value: variable.VarKey},
				)
				varNode.Content = append(varNode.Content,
					&yaml.Node{Kind: yaml.ScalarNode, Value: "value"},
					&yaml.Node{Kind: yaml.ScalarNode, Value: variable.Value},
				)
				if !variable.Enabled {
					varNode.Content = append(varNode.Content,
						&yaml.Node{Kind: yaml.ScalarNode, Value: "enabled"},
						&yaml.Node{Kind: yaml.ScalarNode, Value: strconv.FormatBool(variable.Enabled)},
					)
				}
				if variable.Description != "" {
					varNode.Content = append(varNode.Content,
						&yaml.Node{Kind: yaml.ScalarNode, Value: "description"},
						&yaml.Node{Kind: yaml.ScalarNode, Value: variable.Description},
					)
				}
				varSeq.Content = append(varSeq.Content, varNode)
			}
			envNode.Content = append(envNode.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: "variables"},
				varSeq,
			)
		}

		environmentsNode.Content = append(environmentsNode.Content, envNode)
	}

	if activeEnvName == "" {
		if env, ok := envByID[workspaceData.Workspace.ActiveEnv]; ok {
			activeEnvName = env.Name
		}
	}

	if globalEnvName == "" {
		if env, ok := envByID[workspaceData.Workspace.GlobalEnv]; ok {
			globalEnvName = env.Name
		}
	}

	return activeEnvName, globalEnvName, environmentsNode, nil
}

func exportEnvType(t menv.EnvType) string {
	switch t {
	case menv.EnvGlobal:
		return "global"
	case menv.EnvNormal:
		return "normal"
	default:
		return ""
	}
}

// createOrderedRequestNode creates a YAML node with fields in the desired order
func createOrderedRequestNode(req map[string]any) *yaml.Node {
	node := &yaml.Node{Kind: yaml.MappingNode}

	// Add fields in desired order: name first
	if name, ok := req["name"]; ok {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "name"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%v", name)})
	}

	// Then method
	if method, ok := req["method"]; ok {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "method"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%v", method)})
	}

	// Then url
	if url, ok := req["url"]; ok {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "url"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%v", url)})
	}

	// Then headers
	if headers, ok := req["headers"]; ok {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "headers"},
			createMapNode(headers))
	}

	// Then query_params
	if queryParams, ok := req["query_params"]; ok {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "query_params"},
			createMapNode(queryParams))
	}

	// Finally body
	if body, ok := req["body"]; ok {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "body"},
			createAnyNode(body))
	}

	if assertions, ok := req["assertions"].([]map[string]any); ok && len(assertions) > 0 {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "assertions"},
			createAssertionsNode(assertions))
	}

	return node
}

func createAssertionsNode(assertions []map[string]any) *yaml.Node {
	seq := &yaml.Node{Kind: yaml.SequenceNode}
	for _, assertion := range assertions {
		item := &yaml.Node{Kind: yaml.MappingNode}
		if expression, ok := assertion["expression"].(string); ok {
			item.Content = append(item.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: "expression"},
				&yaml.Node{Kind: yaml.ScalarNode, Value: expression},
			)
		}

		enabled := true
		switch v := assertion["enabled"].(type) {
		case bool:
			enabled = v
		case string:
			enabled = v != "false" && v != "0"
		case int:
			enabled = v != 0
		case int64:
			enabled = v != 0
		case nil:
			// leave default true
		default:
			// leave default true
		}

		enabledNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(enabled)}
		item.Content = append(item.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "enabled"},
			enabledNode,
		)

		seq.Content = append(seq.Content, item)
	}

	return seq
}

// createOrderedFlowNode creates a YAML node for flow with proper field ordering
func createOrderedFlowNode(flow map[string]any) *yaml.Node {
	node := &yaml.Node{Kind: yaml.MappingNode}

	// Add name first
	if name, ok := flow["name"]; ok {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "name"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%v", name)})
	}

	// Then variables
	if variables, ok := flow["variables"]; ok {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "variables"},
			createAnyNode(variables))
	}

	// Then steps
	if steps, ok := flow["steps"]; ok {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "steps"},
			createStepsNode(steps))
	}

	return node
}

// createStepsNode creates ordered step nodes
func createStepsNode(steps any) *yaml.Node {
	stepsSlice, ok := steps.([]map[string]any)
	if !ok {
		return createAnyNode(steps)
	}

	node := &yaml.Node{Kind: yaml.SequenceNode}
	for _, step := range stepsSlice {
		node.Content = append(node.Content, createOrderedStepNode(step))
	}
	return node
}

// createOrderedStepNode creates a step node with proper ordering
func createOrderedStepNode(step map[string]any) *yaml.Node {
	node := &yaml.Node{Kind: yaml.MappingNode}

	// Handle different step types
	for stepType, stepData := range step {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: stepType},
			createOrderedStepDataNode(stepData))
	}

	return node
}

// createOrderedStepDataNode creates step data with name first
func createOrderedStepDataNode(data any) *yaml.Node {
	dataMap, ok := data.(map[string]any)
	if !ok {
		return createAnyNode(data)
	}

	node := &yaml.Node{Kind: yaml.MappingNode}

	// Add name first
	if name, ok := dataMap["name"]; ok {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "name"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%v", name)})
	}

	// Then add other fields in a logical order
	fieldOrder := []string{"use_request", "method", "url", "headers", "query_params", "body",
		"condition", "code", "iter_count", "items", "then", "else", "loop", "depends_on"}

	for _, field := range fieldOrder {
		if val, ok := dataMap[field]; ok {
			node.Content = append(node.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: field},
				createAnyNode(val))
		}
	}

	// Add any remaining fields not in our order list
	for key, val := range dataMap {
		if key == "name" {
			continue // Already added
		}
		found := false
		for _, field := range fieldOrder {
			if key == field {
				found = true
				break
			}
		}
		if !found {
			node.Content = append(node.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: key},
				createAnyNode(val))
		}
	}

	return node
}

// createMapNode creates a YAML mapping node from a map
func createMapNode(data any) *yaml.Node {
	dataMap, ok := data.(map[string]string)
	if !ok {
		return createAnyNode(data)
	}

	node := &yaml.Node{Kind: yaml.MappingNode}

	// Sort keys for consistent output
	keys := make([]string, 0, len(dataMap))
	for k := range dataMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: k},
			&yaml.Node{Kind: yaml.ScalarNode, Value: dataMap[k]})
	}

	return node
}

// createAnyNode creates a YAML node from any value
func createAnyNode(data any) *yaml.Node {
	node := &yaml.Node{}
	if err := node.Encode(data); err != nil {
		// If encoding fails, return a string representation as fallback
		node.Kind = yaml.ScalarNode
		node.Value = fmt.Sprintf("%v", data)
	}
	return node
}

// buildRequestDefinitions creates global request definitions from endpoints
func buildRequestDefinitions(ctx context.Context, workspaceData *ioworkspace.WorkspaceData, overlayMgr *merge.Manager) (map[string]map[string]any, error) {
	requests := make(map[string]map[string]any)

	nodeNameToRequestNode := make(map[string]*mnrequest.MNRequest)
	for i := range workspaceData.FlowRequestNodes {
		reqNode := &workspaceData.FlowRequestNodes[i]
		for _, node := range workspaceData.FlowNodes {
			if node.ID == reqNode.FlowNodeID {
				nodeNameToRequestNode[node.Name] = reqNode
				break
			}
		}
	}

	reachableNodes, flowsWithStart := computeReachableNodesForWorkspace(workspaceData)

	processedNodes := make(map[string]bool)

	for _, node := range workspaceData.FlowNodes {
		if flowsWithStart[node.FlowID] && !reachableNodes[node.ID] {
			continue
		}
		if node.NodeKind != mnnode.NODE_KIND_REQUEST {
			continue
		}

		reqNode, exists := nodeNameToRequestNode[node.Name]
		if !exists || reqNode.EndpointID == nil || reqNode.ExampleID == nil {
			continue
		}

		if processedNodes[node.Name] {
			continue
		}
		processedNodes[node.Name] = true

		endpoint := findEndpointByID(workspaceData, *reqNode.EndpointID)
		if endpoint == nil {
			continue
		}

		requestEntry := map[string]any{
			"name": node.Name,
		}

		if endpoint.Method != "" {
			requestEntry["method"] = endpoint.Method
		}
		if endpoint.Url != "" {
			requestEntry["url"] = endpoint.Url
		}

		if reqNode.DeltaEndpointID != nil {
			if deltaEndpoint := findEndpointByID(workspaceData, *reqNode.DeltaEndpointID); deltaEndpoint != nil {
				if deltaEndpoint.Method != "" {
					requestEntry["method"] = deltaEndpoint.Method
				}
				if deltaEndpoint.Url != "" {
					requestEntry["url"] = deltaEndpoint.Url
				}
			}
		}

		resolveInput, ok := requestInputForNode(workspaceData, reqNode)
		if !ok {
			continue
		}

		mergeOutput, err := resolve.Request(ctx, overlayMgr, resolveInput, reqNode.DeltaExampleID)
		if err != nil {
			return nil, err
		}

		headerMap := headersToMap(mergeOutput.MergeHeaders)
		if len(headerMap) > 0 {
			requestEntry["headers"] = headerMap
		}

		queryMap := queriesToMap(mergeOutput.MergeQueries)
		if len(queryMap) > 0 {
			requestEntry["query_params"] = queryMap
		}

		if bodyEntry, ok := encodeMergedBody(mergeOutput); ok {
			requestEntry["body"] = bodyEntry
		}

		if assertionEntries := assertsToEntries(mergeOutput.MergeAsserts); len(assertionEntries) > 0 {
			requestEntry["assertions"] = assertionEntries
		}

		requests[node.Name] = requestEntry
	}

	return requests, nil
}

func requestInputForNode(workspaceData *ioworkspace.WorkspaceData, reqNode *mnrequest.MNRequest) (resolve.RequestInput, bool) {
	if reqNode.ExampleID == nil {
		return resolve.RequestInput{}, false
	}

	baseExample := findExampleByID(workspaceData, *reqNode.ExampleID)
	if baseExample == nil {
		return resolve.RequestInput{}, false
	}

	baseHeaders := collectHeadersByExampleID(workspaceData, baseExample.ID)
	baseQueries := collectQueriesByExampleID(workspaceData, baseExample.ID)
	baseRawBody, baseRawFound := collectRawBodyByExampleID(workspaceData, baseExample.ID)
	baseFormBody := collectFormBodiesByExampleID(workspaceData, baseExample.ID)
	baseURLBody := collectUrlBodiesByExampleID(workspaceData, baseExample.ID)
	baseAsserts := collectOrderedAssertsForExample(workspaceData, baseExample.ID)

	var baseRawPtr *mbodyraw.ExampleBodyRaw
	if baseRawFound {
		copyRaw := baseRawBody
		baseRawPtr = &copyRaw
	}

	input := resolve.RequestInput{
		BaseExample:  *baseExample,
		BaseHeaders:  baseHeaders,
		BaseQueries:  baseQueries,
		BaseRawBody:  baseRawPtr,
		BaseFormBody: baseFormBody,
		BaseURLBody:  baseURLBody,
		BaseAsserts:  baseAsserts,
	}

	if reqNode.DeltaExampleID != nil {
		deltaExample := findExampleByID(workspaceData, *reqNode.DeltaExampleID)
		if deltaExample != nil {
			copyDelta := *deltaExample

			deltaHeaders := collectHeadersByExampleID(workspaceData, deltaExample.ID)
			deltaQueries := collectQueriesByExampleID(workspaceData, deltaExample.ID)
			deltaRawBody, deltaRawFound := collectRawBodyByExampleID(workspaceData, deltaExample.ID)
			deltaFormBody := collectFormBodiesByExampleID(workspaceData, deltaExample.ID)
			deltaURLBody := collectUrlBodiesByExampleID(workspaceData, deltaExample.ID)
			deltaAsserts := collectOrderedAssertsForExample(workspaceData, deltaExample.ID)

			var deltaRawPtr *mbodyraw.ExampleBodyRaw
			if deltaRawFound {
				copyRaw := deltaRawBody
				deltaRawPtr = &copyRaw
			}

			input.DeltaExample = &copyDelta
			input.DeltaHeaders = deltaHeaders
			input.DeltaQueries = deltaQueries
			input.DeltaRawBody = deltaRawPtr
			input.DeltaFormBody = deltaFormBody
			input.DeltaURLBody = deltaURLBody
			input.DeltaAsserts = deltaAsserts
		}
	}

	return input, true
}

func diffHeaderOverrides(base []mexampleheader.Header, final []mexampleheader.Header) map[string]string {
	if len(final) == 0 {
		return nil
	}
	baseMap := make(map[string]string, len(base))
	for _, h := range base {
		if h.Enable {
			baseMap[h.HeaderKey] = h.Value
		}
	}

	overrides := make(map[string]string)
	for _, h := range final {
		if !h.Enable {
			continue
		}
		if baseVal, ok := baseMap[h.HeaderKey]; !ok || baseVal != h.Value {
			overrides[h.HeaderKey] = h.Value
		}
	}
	if len(overrides) == 0 {
		return nil
	}
	return overrides
}

func diffQueryOverrides(base []mexamplequery.Query, final []mexamplequery.Query) map[string]string {
	if len(final) == 0 {
		return nil
	}
	baseMap := make(map[string]string, len(base))
	for _, q := range base {
		baseMap[q.QueryKey] = q.Value
	}

	overrides := make(map[string]string)
	for _, q := range final {
		if baseVal, ok := baseMap[q.QueryKey]; !ok || baseVal != q.Value {
			overrides[q.QueryKey] = q.Value
		}
	}
	if len(overrides) == 0 {
		return nil
	}
	return overrides
}

func encodeMergedBody(mergeOutput request.MergeExamplesOutput) (any, bool) {
	if body, ok := encodeFormBody(mergeOutput.MergeFormBody); ok {
		return body, true
	}
	if body, ok := encodeURLEncodedBody(mergeOutput.MergeUrlEncodedBody); ok {
		return body, true
	}
	if mergeOutput.Merged.BodyType == mitemapiexample.BodyTypeForm {
		if body, ok := encodeFormBodyFromRaw(mergeOutput.MergeRawBody.Data); ok {
			return body, true
		}
	}
	if mergeOutput.Merged.BodyType == mitemapiexample.BodyTypeUrlencoded {
		if body, ok := encodeURLEncodedBodyFromRaw(mergeOutput.MergeRawBody.Data); ok {
			return body, true
		}
	}
	if len(mergeOutput.MergeRawBody.Data) == 0 {
		return nil, false
	}
	var bodyData any
	if err := json.Unmarshal(mergeOutput.MergeRawBody.Data, &bodyData); err != nil {
		return nil, false
	}
	if isEmptyBodyData(bodyData) {
		return nil, false
	}
	return bodyData, true
}

func baseMergeOutputFromInput(input resolve.RequestInput) request.MergeExamplesOutput {
	baseRaw := mbodyraw.ExampleBodyRaw{}
	if input.BaseRawBody != nil {
		baseRaw = *input.BaseRawBody
	}

	baseHeaders := make([]mexampleheader.Header, len(input.BaseHeaders))
	copy(baseHeaders, input.BaseHeaders)

	baseQueries := make([]mexamplequery.Query, len(input.BaseQueries))
	copy(baseQueries, input.BaseQueries)

	baseForm := make([]mbodyform.BodyForm, len(input.BaseFormBody))
	copy(baseForm, input.BaseFormBody)

	baseURL := make([]mbodyurl.BodyURLEncoded, len(input.BaseURLBody))
	copy(baseURL, input.BaseURLBody)

	baseAsserts := make([]massert.Assert, len(input.BaseAsserts))
	copy(baseAsserts, input.BaseAsserts)

	return request.MergeExamplesOutput{
		Merged:              input.BaseExample,
		MergeHeaders:        baseHeaders,
		MergeQueries:        baseQueries,
		MergeRawBody:        baseRaw,
		MergeFormBody:       baseForm,
		MergeUrlEncodedBody: baseURL,
		MergeAsserts:        baseAsserts,
	}
}

func encodeFormBody(items []mbodyform.BodyForm) (map[string]any, bool) {
	if len(items) == 0 {
		return nil, false
	}
	filtered := make([]mbodyform.BodyForm, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.BodyKey) == "" {
			continue
		}
		filtered = append(filtered, item)
	}
	if len(filtered) == 0 {
		return nil, false
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].BodyKey != filtered[j].BodyKey {
			return filtered[i].BodyKey < filtered[j].BodyKey
		}
		return filtered[i].Value < filtered[j].Value
	})
	encoded := make([]map[string]any, 0, len(filtered))
	for _, item := range filtered {
		entry := map[string]any{
			"name":    strings.TrimSpace(item.BodyKey),
			"value":   item.Value,
			"enabled": item.Enable,
		}
		if desc := strings.TrimSpace(item.Description); desc != "" {
			entry["description"] = desc
		}
		encoded = append(encoded, entry)
	}
	if len(encoded) == 0 {
		return nil, false
	}
	return map[string]any{
		"type":  "form-data",
		"items": encoded,
	}, true
}

func encodeURLEncodedBody(items []mbodyurl.BodyURLEncoded) (map[string]any, bool) {
	if len(items) == 0 {
		return nil, false
	}
	filtered := make([]mbodyurl.BodyURLEncoded, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.BodyKey) == "" {
			continue
		}
		filtered = append(filtered, item)
	}
	if len(filtered) == 0 {
		return nil, false
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].BodyKey != filtered[j].BodyKey {
			return filtered[i].BodyKey < filtered[j].BodyKey
		}
		return filtered[i].Value < filtered[j].Value
	})
	encoded := make([]map[string]any, 0, len(filtered))
	for _, item := range filtered {
		entry := map[string]any{
			"name":    strings.TrimSpace(item.BodyKey),
			"value":   item.Value,
			"enabled": item.Enable,
		}
		if desc := strings.TrimSpace(item.Description); desc != "" {
			entry["description"] = desc
		}
		encoded = append(encoded, entry)
	}
	if len(encoded) == 0 {
		return nil, false
	}
	return map[string]any{
		"type":  "x-www-form-urlencoded",
		"items": encoded,
	}, true
}

func isEmptyBodyData(body any) bool {
	switch v := body.(type) {
	case nil:
		return true
	case map[string]any:
		return len(v) == 0
	case []any:
		return len(v) == 0
	case string:
		return strings.TrimSpace(v) == ""
	default:
		return false
	}
}

func encodeFormBodyFromRaw(raw []byte) (map[string]any, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, false
	}
	if len(parsed) == 0 {
		return nil, false
	}
	keys := make([]string, 0, len(parsed))
	for k := range parsed {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	items := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		val := parsed[key]
		entry := map[string]any{
			"name":    key,
			"value":   fmt.Sprint(val),
			"enabled": true,
		}
		items = append(items, entry)
	}
	if len(items) == 0 {
		return nil, false
	}
	return map[string]any{
		"type":  "form-data",
		"items": items,
	}, true
}

func encodeURLEncodedBodyFromRaw(raw []byte) (map[string]any, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, false
	}
	if len(parsed) == 0 {
		return nil, false
	}
	keys := make([]string, 0, len(parsed))
	for k := range parsed {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	items := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		val := parsed[key]
		entry := map[string]any{
			"name":    key,
			"value":   fmt.Sprint(val),
			"enabled": true,
		}
		items = append(items, entry)
	}
	if len(items) == 0 {
		return nil, false
	}
	return map[string]any{
		"type":  "x-www-form-urlencoded",
		"items": items,
	}, true
}

func diffBody(base *mbodyraw.ExampleBodyRaw, final mbodyraw.ExampleBodyRaw) (any, bool) {
	if len(final.Data) == 0 {
		return nil, false
	}
	var baseData []byte
	if base != nil {
		baseData = base.Data
	}
	if bytes.Equal(baseData, final.Data) {
		return nil, false
	}
	var body any
	if err := json.Unmarshal(final.Data, &body); err != nil {
		return nil, false
	}
	return body, true
}

func findEndpointByID(workspaceData *ioworkspace.WorkspaceData, id idwrap.IDWrap) *mitemapi.ItemApi {
	for i := range workspaceData.Endpoints {
		if workspaceData.Endpoints[i].ID == id {
			return &workspaceData.Endpoints[i]
		}
	}
	return nil
}

func findExampleByID(workspaceData *ioworkspace.WorkspaceData, id idwrap.IDWrap) *mitemapiexample.ItemApiExample {
	for i := range workspaceData.Examples {
		if workspaceData.Examples[i].ID == id {
			return &workspaceData.Examples[i]
		}
	}
	return nil
}

func collectHeadersByExampleID(workspaceData *ioworkspace.WorkspaceData, exampleID idwrap.IDWrap) []mexampleheader.Header {
	headers := make([]mexampleheader.Header, 0)
	for _, header := range workspaceData.ExampleHeaders {
		if header.ExampleID == exampleID {
			headers = append(headers, header)
		}
	}
	return headers
}

func collectQueriesByExampleID(workspaceData *ioworkspace.WorkspaceData, exampleID idwrap.IDWrap) []mexamplequery.Query {
	queries := make([]mexamplequery.Query, 0)
	for _, query := range workspaceData.ExampleQueries {
		if query.ExampleID == exampleID {
			queries = append(queries, query)
		}
	}
	return queries
}

func collectRawBodyByExampleID(workspaceData *ioworkspace.WorkspaceData, exampleID idwrap.IDWrap) (mbodyraw.ExampleBodyRaw, bool) {
	for _, body := range workspaceData.Rawbodies {
		if body.ExampleID == exampleID {
			return body, true
		}
	}
	return mbodyraw.ExampleBodyRaw{}, false
}

func collectFormBodiesByExampleID(workspaceData *ioworkspace.WorkspaceData, exampleID idwrap.IDWrap) []mbodyform.BodyForm {
	forms := make([]mbodyform.BodyForm, 0)
	for _, body := range workspaceData.FormBodies {
		if body.ExampleID == exampleID {
			forms = append(forms, body)
		}
	}
	return forms
}

func collectUrlBodiesByExampleID(workspaceData *ioworkspace.WorkspaceData, exampleID idwrap.IDWrap) []mbodyurl.BodyURLEncoded {
	encoded := make([]mbodyurl.BodyURLEncoded, 0)
	for _, body := range workspaceData.UrlBodies {
		if body.ExampleID == exampleID {
			encoded = append(encoded, body)
		}
	}
	return encoded
}

func collectOrderedAssertsForExample(workspaceData *ioworkspace.WorkspaceData, exampleID idwrap.IDWrap) []massert.Assert {
	filtered := make([]massert.Assert, 0)
	for _, assert := range workspaceData.ExampleAsserts {
		if assert.ExampleID == exampleID {
			filtered = append(filtered, assert)
		}
	}
	if len(filtered) == 0 {
		return filtered
	}
	return dedupAssertionsOrdered(orderAssertions(filtered))
}

func orderAssertions(asserts []massert.Assert) []massert.Assert {
	if len(asserts) <= 1 {
		return append([]massert.Assert(nil), asserts...)
	}

	byID := make(map[idwrap.IDWrap]*massert.Assert, len(asserts))
	var head *massert.Assert
	for i := range asserts {
		assert := &asserts[i]
		byID[assert.ID] = assert
		if assert.Prev == nil {
			head = assert
		}
	}

	ordered := make([]massert.Assert, 0, len(asserts))
	visited := make(map[idwrap.IDWrap]bool, len(asserts))

	for current := head; current != nil; {
		if visited[current.ID] {
			break
		}
		ordered = append(ordered, *current)
		visited[current.ID] = true
		if current.Next == nil {
			break
		}
		next, ok := byID[*current.Next]
		if !ok {
			break
		}
		current = next
	}

	if len(ordered) < len(asserts) {
		remaining := make([]massert.Assert, 0, len(asserts)-len(ordered))
		for _, assert := range asserts {
			if !visited[assert.ID] {
				remaining = append(remaining, assert)
			}
		}
		sort.Slice(remaining, func(i, j int) bool {
			return remaining[i].Condition.Comparisons.Expression < remaining[j].Condition.Comparisons.Expression
		})
		ordered = append(ordered, remaining...)
	}

	return ordered
}

func dedupAssertionsOrdered(asserts []massert.Assert) []massert.Assert {
	if len(asserts) <= 1 {
		return asserts
	}

	seen := make(map[string]struct{}, len(asserts))
	result := make([]massert.Assert, 0, len(asserts))
	for _, assert := range asserts {
		key := assert.Condition.Comparisons.Expression + "|" + strconv.FormatBool(assert.Enable)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, assert)
	}
	return result
}

func headersToMap(headers []mexampleheader.Header) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	result := make(map[string]string)
	for _, header := range headers {
		if !header.Enable {
			continue
		}
		result[header.HeaderKey] = header.Value
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func queriesToMap(queries []mexamplequery.Query) map[string]string {
	if len(queries) == 0 {
		return nil
	}
	result := make(map[string]string)
	for _, query := range queries {
		result[query.QueryKey] = query.Value
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func assertsToEntries(asserts []massert.Assert) []map[string]any {
	if len(asserts) == 0 {
		return nil
	}

	ordered := dedupAssertionsOrdered(orderAssertions(asserts))
	entries := make([]map[string]any, 0, len(ordered))
	for _, assertModel := range ordered {
		expr := assertModel.Condition.Comparisons.Expression
		if expr == "" {
			continue
		}
		entries = append(entries, map[string]any{
			"expression": expr,
			"enabled":    assertModel.Enable,
		})
	}

	if len(entries) == 0 {
		return nil
	}
	return entries
}

func assertionsEqual(a, b []massert.Assert) bool {
	a = dedupAssertionsOrdered(orderAssertions(a))
	b = dedupAssertionsOrdered(orderAssertions(b))
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i].Condition.Comparisons.Expression != b[i].Condition.Comparisons.Expression {
			return false
		}
		if a[i].Enable != b[i].Enable {
			return false
		}
	}

	return true
}

// exportFlow exports a single flow
func exportFlow(ctx context.Context, flow mflow.Flow, workspaceData *ioworkspace.WorkspaceData, requests map[string]map[string]any, overlayMgr *merge.Manager) (map[string]any, error) {
	// Build node map for this flow
	nodeMap := make(map[idwrap.IDWrap]mnnode.MNode)
	for _, node := range workspaceData.FlowNodes {
		if node.FlowID == flow.ID {
			nodeMap[node.ID] = node
		}
	}

	// Build edge maps
	incomingEdges := make(map[idwrap.IDWrap][]edge.Edge)
	outgoingEdges := make(map[idwrap.IDWrap][]edge.Edge)
	for _, e := range workspaceData.FlowEdges {
		if e.FlowID == flow.ID {
			incomingEdges[e.TargetID] = append(incomingEdges[e.TargetID], e)
			outgoingEdges[e.SourceID] = append(outgoingEdges[e.SourceID], e)
		}
	}

	// Find start node
	var startNodeID idwrap.IDWrap
	for _, noop := range workspaceData.FlowNoopNodes {
		if noop.Type == mnnoop.NODE_NO_OP_KIND_START {
			// Check if this noop belongs to our flow
			if node, exists := nodeMap[noop.FlowNodeID]; exists && node.FlowID == flow.ID {
				startNodeID = noop.FlowNodeID
				break
			}
		}
	}

	// Build variables
	var variables []map[string]string
	for _, v := range workspaceData.FlowVariables {
		if v.FlowID == flow.ID && v.Enabled {
			variables = append(variables, map[string]string{
				"name":  v.Name,
				"value": v.Value,
			})
		}
	}

	// Build node name to request name mapping (which are now the same)
	nodeToRequest := make(map[string]string)
	for nodeName := range requests {
		nodeToRequest[nodeName] = nodeName
	}

	// Process nodes to create steps
	steps, err := processFlowNodes(ctx, nodeMap, incomingEdges, outgoingEdges, startNodeID, workspaceData, nodeToRequest, overlayMgr)
	if err != nil {
		return nil, err
	}

	// Build flow data
	flowData := map[string]any{
		"name": flow.Name,
	}

	if len(variables) > 0 {
		flowData["variables"] = variables
	}

	if len(steps) > 0 {
		flowData["steps"] = steps
	}

	return flowData, nil
}

// processFlowNodes processes all nodes in a flow and returns steps

func processFlowNodes(ctx context.Context, nodeMap map[idwrap.IDWrap]mnnode.MNode, incomingEdges map[idwrap.IDWrap][]edge.Edge,
	outgoingEdges map[idwrap.IDWrap][]edge.Edge, startNodeID idwrap.IDWrap,
	workspaceData *ioworkspace.WorkspaceData, nodeToRequest map[string]string, overlayMgr *merge.Manager) ([]map[string]any, error) {

	reachable := computeReachableNodeSet(outgoingEdges, startNodeID)
	checkReachable := len(reachable) > 0

	processed := make(map[idwrap.IDWrap]bool)
	steps := make([]map[string]any, 0)

	var processNode func(nodeID idwrap.IDWrap) error
	processNode = func(nodeID idwrap.IDWrap) error {
		if checkReachable && !reachable[nodeID] {
			return nil
		}
		if processed[nodeID] || nodeID == startNodeID {
			return nil
		}
		processed[nodeID] = true

		node, exists := nodeMap[nodeID]
		if !exists {
			return nil
		}

		for _, e := range incomingEdges[nodeID] {
			if e.SourceID != startNodeID {
				if err := processNode(e.SourceID); err != nil {
					return err
				}
			}
		}

		var (
			step map[string]any
			err  error
		)

		switch node.NodeKind {
		case mnnode.NODE_KIND_REQUEST:
			step, err = convertRequestNodeClean(ctx, node, incomingEdges, outgoingEdges, startNodeID,
				nodeMap, workspaceData, nodeToRequest, overlayMgr)
		case mnnode.NODE_KIND_JS:
			step = convertJSNodeClean(node, incomingEdges, startNodeID, nodeMap, workspaceData)
		case mnnode.NODE_KIND_CONDITION:
			step = convertConditionNodeClean(node, incomingEdges, outgoingEdges, startNodeID,
				nodeMap, workspaceData)
		case mnnode.NODE_KIND_FOR:
			step = convertForNodeClean(node, incomingEdges, outgoingEdges, startNodeID,
				nodeMap, workspaceData)
		case mnnode.NODE_KIND_FOR_EACH:
			step = convertForEachNodeClean(node, incomingEdges, outgoingEdges, startNodeID,
				nodeMap, workspaceData)
		}

		if err != nil {
			return err
		}

		if step != nil {
			steps = append(steps, step)
		}

		return nil
	}

	for nodeID := range nodeMap {
		if checkReachable && !reachable[nodeID] {
			continue
		}
		if err := processNode(nodeID); err != nil {
			return nil, err
		}
	}

	return steps, nil
}

func computeReachableNodesForWorkspace(workspaceData *ioworkspace.WorkspaceData) (map[idwrap.IDWrap]bool, map[idwrap.IDWrap]bool) {
	reachable := make(map[idwrap.IDWrap]bool)
	flowsWithStart := make(map[idwrap.IDWrap]bool)

	nodeByID := make(map[idwrap.IDWrap]mnnode.MNode)
	for _, node := range workspaceData.FlowNodes {
		nodeByID[node.ID] = node
	}

	outgoingByFlow := make(map[idwrap.IDWrap]map[idwrap.IDWrap][]edge.Edge)
	for _, e := range workspaceData.FlowEdges {
		perFlow := outgoingByFlow[e.FlowID]
		if perFlow == nil {
			perFlow = make(map[idwrap.IDWrap][]edge.Edge)
			outgoingByFlow[e.FlowID] = perFlow
		}
		perFlow[e.SourceID] = append(perFlow[e.SourceID], e)
	}

	startByFlow := make(map[idwrap.IDWrap]idwrap.IDWrap)
	for _, noop := range workspaceData.FlowNoopNodes {
		if noop.Type != mnnoop.NODE_NO_OP_KIND_START {
			continue
		}
		node, ok := nodeByID[noop.FlowNodeID]
		if !ok {
			continue
		}
		flowsWithStart[node.FlowID] = true
		if _, exists := startByFlow[node.FlowID]; !exists {
			startByFlow[node.FlowID] = noop.FlowNodeID
		}
	}

	for flowID, start := range startByFlow {
		for nodeID := range computeReachableNodeSet(outgoingByFlow[flowID], start) {
			reachable[nodeID] = true
		}
	}

	return reachable, flowsWithStart
}

func computeReachableNodeSet(outgoingEdges map[idwrap.IDWrap][]edge.Edge, startNodeID idwrap.IDWrap) map[idwrap.IDWrap]bool {
	if startNodeID == (idwrap.IDWrap{}) {
		return nil
	}

	reachable := make(map[idwrap.IDWrap]bool)
	reachable[startNodeID] = true
	queue := []idwrap.IDWrap{startNodeID}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, e := range outgoingEdges[current] {
			target := e.TargetID
			if target == (idwrap.IDWrap{}) {
				continue
			}
			if !reachable[target] {
				reachable[target] = true
				queue = append(queue, target)
			}
		}
	}

	return reachable
}

// convertRequestNodeClean converts a request node to clean format
func convertRequestNodeClean(ctx context.Context, node mnnode.MNode, incomingEdges map[idwrap.IDWrap][]edge.Edge,
	outgoingEdges map[idwrap.IDWrap][]edge.Edge, startNodeID idwrap.IDWrap,
	nodeMap map[idwrap.IDWrap]mnnode.MNode, workspaceData *ioworkspace.WorkspaceData,
	nodeToRequest map[string]string, overlayMgr *merge.Manager) (map[string]any, error) {

	// Find request node data
	var requestNode *mnrequest.MNRequest
	for i := range workspaceData.FlowRequestNodes {
		if workspaceData.FlowRequestNodes[i].FlowNodeID == node.ID {
			requestNode = &workspaceData.FlowRequestNodes[i]
			break
		}
	}
	if requestNode == nil {
		return nil, nil
	}

	step := map[string]any{
		"name": node.Name,
	}

	// Find the request reference using node name
	if requestName, ok := nodeToRequest[node.Name]; ok {
		step["use_request"] = requestName
	}

	// Add override values from delta endpoint/examples (with variable references)
	// Check for method/URL overrides from delta endpoint
	if requestNode.DeltaEndpointID != nil && requestNode.EndpointID != nil {
		// Find base and delta endpoints
		var baseEndpoint, deltaEndpoint *mitemapi.ItemApi
		for i := range workspaceData.Endpoints {
			if workspaceData.Endpoints[i].ID == *requestNode.EndpointID {
				baseEndpoint = &workspaceData.Endpoints[i]
			}
			if workspaceData.Endpoints[i].ID == *requestNode.DeltaEndpointID {
				deltaEndpoint = &workspaceData.Endpoints[i]
			}
		}

		if baseEndpoint != nil && deltaEndpoint != nil {
			if deltaEndpoint.Method != baseEndpoint.Method && deltaEndpoint.Method != "" {
				step["method"] = deltaEndpoint.Method
			}
			if deltaEndpoint.Url != baseEndpoint.Url && deltaEndpoint.Url != "" {
				step["url"] = deltaEndpoint.Url
			}
		}
	}

	if requestNode.DeltaExampleID != nil {
		input, ok := requestInputForNode(workspaceData, requestNode)
		if ok {
			merged, err := resolve.Request(ctx, overlayMgr, input, requestNode.DeltaExampleID)
			if err != nil {
				return nil, err
			}

			baseOutput := baseMergeOutputFromInput(input)

			headerOverrides := diffHeaderOverrides(input.BaseHeaders, merged.MergeHeaders)
			if len(headerOverrides) > 0 {
				step["headers"] = headerOverrides
			}

			queryOverrides := diffQueryOverrides(input.BaseQueries, merged.MergeQueries)
			if len(queryOverrides) > 0 {
				step["query_params"] = queryOverrides
			}

			if mergedBody, ok := encodeMergedBody(merged); ok {
				if baseBody, baseOK := encodeMergedBody(baseOutput); !baseOK || !reflect.DeepEqual(baseBody, mergedBody) {
					step["body"] = mergedBody
				}
			}

			if !assertionsEqual(input.BaseAsserts, merged.MergeAsserts) {
				if assertionEntries := assertsToEntries(merged.MergeAsserts); len(assertionEntries) > 0 {
					step["assertions"] = assertionEntries
				}
			}
		}
	}

	// Add dependencies
	var dependencies []string
	for _, e := range incomingEdges[node.ID] {
		if e.SourceID != startNodeID {
			if sourceNode, exists := nodeMap[e.SourceID]; exists && sourceNode.Name != "" {
				dependencies = append(dependencies, sourceNode.Name)
			}
		}
	}
	if len(dependencies) > 0 {
		step["depends_on"] = dependencies
	}

	return map[string]any{"request": step}, nil
}

// convertJSNodeClean converts a JS node to clean format
func convertJSNodeClean(node mnnode.MNode, incomingEdges map[idwrap.IDWrap][]edge.Edge,
	startNodeID idwrap.IDWrap, nodeMap map[idwrap.IDWrap]mnnode.MNode,
	workspaceData *ioworkspace.WorkspaceData) map[string]any {

	// Find JS node data
	var jsNode *mnjs.MNJS
	for i := range workspaceData.FlowJSNodes {
		if workspaceData.FlowJSNodes[i].FlowNodeID == node.ID {
			jsNode = &workspaceData.FlowJSNodes[i]
			break
		}
	}
	if jsNode == nil {
		return nil
	}

	step := map[string]any{
		"name": node.Name,
		"code": string(jsNode.Code),
	}

	// Add dependencies
	var dependencies []string
	for _, e := range incomingEdges[node.ID] {
		if e.SourceID != startNodeID {
			if sourceNode, exists := nodeMap[e.SourceID]; exists && sourceNode.Name != "" {
				dependencies = append(dependencies, sourceNode.Name)
			}
		}
	}
	if len(dependencies) > 0 {
		step["depends_on"] = dependencies
	}

	return map[string]any{"js": step}
}

// convertConditionNodeClean converts a condition node to clean format
func convertConditionNodeClean(node mnnode.MNode, incomingEdges map[idwrap.IDWrap][]edge.Edge,
	outgoingEdges map[idwrap.IDWrap][]edge.Edge, startNodeID idwrap.IDWrap,
	nodeMap map[idwrap.IDWrap]mnnode.MNode, workspaceData *ioworkspace.WorkspaceData) map[string]any {

	// Find condition node data
	var condNode *mnif.MNIF
	for i := range workspaceData.FlowConditionNodes {
		if workspaceData.FlowConditionNodes[i].FlowNodeID == node.ID {
			condNode = &workspaceData.FlowConditionNodes[i]
			break
		}
	}
	if condNode == nil {
		return nil
	}

	step := map[string]any{
		"name":       node.Name,
		"expression": condNode.Condition.Comparisons.Expression,
	}

	// Find then/else targets
	for _, e := range outgoingEdges[node.ID] {
		if targetNode, exists := nodeMap[e.TargetID]; exists {
			switch e.SourceHandler {
			case edge.HandleThen:
				step["then"] = targetNode.Name
			case edge.HandleElse:
				step["else"] = targetNode.Name
			}
		}
	}

	// Add dependencies
	var dependencies []string
	for _, e := range incomingEdges[node.ID] {
		if e.SourceID != startNodeID {
			if sourceNode, exists := nodeMap[e.SourceID]; exists && sourceNode.Name != "" {
				dependencies = append(dependencies, sourceNode.Name)
			}
		}
	}
	if len(dependencies) > 0 {
		step["depends_on"] = dependencies
	}

	return map[string]any{"if": step}
}

// convertForNodeClean converts a for node to clean format
func convertForNodeClean(node mnnode.MNode, incomingEdges map[idwrap.IDWrap][]edge.Edge,
	outgoingEdges map[idwrap.IDWrap][]edge.Edge, startNodeID idwrap.IDWrap,
	nodeMap map[idwrap.IDWrap]mnnode.MNode, workspaceData *ioworkspace.WorkspaceData) map[string]any {

	// Find for node data
	var forNode *mnfor.MNFor
	for i := range workspaceData.FlowForNodes {
		if workspaceData.FlowForNodes[i].FlowNodeID == node.ID {
			forNode = &workspaceData.FlowForNodes[i]
			break
		}
	}
	if forNode == nil {
		return nil
	}

	step := map[string]any{
		"name": node.Name,
	}

	// Only add iter_count if it's non-zero
	if forNode.IterCount > 0 {
		step["iter_count"] = forNode.IterCount
	}

	// Find loop target
	for _, e := range outgoingEdges[node.ID] {
		if e.SourceHandler == edge.HandleLoop {
			if targetNode, exists := nodeMap[e.TargetID]; exists && targetNode.Name != "" {
				step["loop"] = targetNode.Name
			}
		}
	}

	// Add dependencies
	var dependencies []string
	for _, e := range incomingEdges[node.ID] {
		if e.SourceID != startNodeID {
			if sourceNode, exists := nodeMap[e.SourceID]; exists && sourceNode.Name != "" {
				dependencies = append(dependencies, sourceNode.Name)
			}
		}
	}
	if len(dependencies) > 0 {
		step["depends_on"] = dependencies
	}

	return map[string]any{"for": step}
}

// convertForEachNodeClean converts a for_each node to clean format
func convertForEachNodeClean(node mnnode.MNode, incomingEdges map[idwrap.IDWrap][]edge.Edge,
	outgoingEdges map[idwrap.IDWrap][]edge.Edge, startNodeID idwrap.IDWrap,
	nodeMap map[idwrap.IDWrap]mnnode.MNode, workspaceData *ioworkspace.WorkspaceData) map[string]any {

	// Find ForEach node data
	var forEachNode *mnforeach.MNForEach
	for i := range workspaceData.FlowForEachNodes {
		if workspaceData.FlowForEachNodes[i].FlowNodeID == node.ID {
			forEachNode = &workspaceData.FlowForEachNodes[i]
			break
		}
	}

	step := map[string]any{
		"name": node.Name,
	}

	// Add items expression
	if forEachNode != nil && forEachNode.IterExpression != "" {
		step["items"] = forEachNode.IterExpression
	} else {
		step["items"] = "response.items" // Default fallback
	}

	// Find loop target
	for _, e := range outgoingEdges[node.ID] {
		if e.SourceHandler == edge.HandleLoop {
			if targetNode, exists := nodeMap[e.TargetID]; exists && targetNode.Name != "" {
				step["loop"] = targetNode.Name
			}
		}
	}

	// Add dependencies
	var dependencies []string
	for _, e := range incomingEdges[node.ID] {
		if e.SourceID != startNodeID {
			if sourceNode, exists := nodeMap[e.SourceID]; exists && sourceNode.Name != "" {
				dependencies = append(dependencies, sourceNode.Name)
			}
		}
	}
	if len(dependencies) > 0 {
		step["depends_on"] = dependencies
	}

	return map[string]any{"for_each": step}
}

// buildRunEntries analyzes flows and their dependencies to build run entries
func buildRunEntries(workspaceData *ioworkspace.WorkspaceData) []RunEntry {
	// Create a run entry for each flow that was exported
	// This enables the exported YAML to specify which flows should be executed
	entries := make([]RunEntry, 0, len(workspaceData.Flows))

	for _, flow := range workspaceData.Flows {
		entry := RunEntry{
			Flow: flow.Name,
		}
		entries = append(entries, entry)
	}

	return entries
}

// createRunEntryNode creates a YAML node for a run entry
func createRunEntryNode(entry RunEntry) *yaml.Node {
	node := &yaml.Node{Kind: yaml.MappingNode}

	// Add flow field
	node.Content = append(node.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "flow"},
		&yaml.Node{Kind: yaml.ScalarNode, Value: entry.Flow})

	// Add depends_on if present
	if len(entry.DependsOn) > 0 {
		if len(entry.DependsOn) == 1 {
			// Single dependency as string
			node.Content = append(node.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: "depends_on"},
				&yaml.Node{Kind: yaml.ScalarNode, Value: entry.DependsOn[0]})
		} else {
			// Multiple dependencies as array
			var depsNode yaml.Node
			depsNode.Kind = yaml.SequenceNode
			for _, dep := range entry.DependsOn {
				depsNode.Content = append(depsNode.Content,
					&yaml.Node{Kind: yaml.ScalarNode, Value: dep})
			}
			node.Content = append(node.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: "depends_on"},
				&depsNode)
		}
	}

	return node
}
