package workflowsimple

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mexamplequery"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"the-dev-tools/server/pkg/model/mnnode/mnif"
	"the-dev-tools/server/pkg/model/mnnode/mnjs"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/model/mvar"
	"the-dev-tools/server/pkg/varsystem"
)

// Parse parses the workflow YAML and returns WorkflowData
func Parse(data []byte) (*WorkflowData, error) {
	var workflow WorkflowFormat
	var rawWorkflow map[string]any

	// First unmarshal to a generic map to handle step types properly
	if err := yaml.Unmarshal(data, &rawWorkflow); err != nil {
		return nil, fmt.Errorf("failed to unmarshal workflow format: %w", err)
	}

	// Then unmarshal to structured format
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		return nil, fmt.Errorf("failed to unmarshal workflow format: %w", err)
	}

	if workflow.WorkspaceName == "" {
		return nil, fmt.Errorf("workspace_name is required")
	}

	// Parse request templates (support both old and new format)
	var templates map[string]*requestTemplate
	if workflow.RequestTemplates != nil {
		templates = parseRequestTemplates(workflow.RequestTemplates)
	} else if workflow.Requests != nil {
		templates = parseRequests(workflow.Requests)
	} else {
		templates = make(map[string]*requestTemplate)
	}

	// Initialize workflow data
	workflowData := &WorkflowData{
		Nodes:          make([]mnnode.MNode, 0),
		Edges:          make([]edge.Edge, 0),
		Variables:      make([]mvar.Var, 0),
		NoopNodes:      make([]mnnoop.NoopNode, 0),
		RequestNodes:   make([]mnrequest.MNRequest, 0),
		ConditionNodes: make([]mnif.MNIF, 0),
		ForNodes:       make([]mnfor.MNFor, 0),
		JSNodes:        make([]mnjs.MNJS, 0),
		Endpoints:      make([]mitemapi.ItemApi, 0),
		Examples:       make([]mitemapiexample.ItemApiExample, 0),
		Headers:        make([]mexampleheader.Header, 0),
		Queries:        make([]mexamplequery.Query, 0),
		RawBodies:      make([]mbodyraw.ExampleBodyRaw, 0),
	}

	// Process first flow only (simplified version)
	if len(workflow.Flows) == 0 {
		return nil, fmt.Errorf("at least one flow is required")
	}

	flow := workflow.Flows[0]
	flowID := idwrap.NewNow()

	workflowData.Flow = mflow.Flow{
		ID:   flowID,
		Name: flow.Name,
	}

	// Process flow variables
	for _, v := range flow.Variables {
		workflowData.Variables = append(workflowData.Variables, mvar.Var{
			VarKey: v.Name,
			Value:  v.Value,
		})
	}

	// Create variable map for resolution
	varMap := varsystem.NewVarMap(workflowData.Variables)

	// Create start node
	startNodeID := idwrap.NewNow()
	startNode := mnnode.MNode{
		ID:       startNodeID,
		FlowID:   flowID,
		Name:     "Start",
		NodeKind: mnnode.NODE_KIND_NO_OP,
	}
	workflowData.Nodes = append(workflowData.Nodes, startNode)

	noopNode := mnnoop.NoopNode{
		FlowNodeID: startNodeID,
		Type:       mnnoop.NODE_NO_OP_KIND_START,
	}
	workflowData.NoopNodes = append(workflowData.NoopNodes, noopNode)

	// Get raw steps
	rawFlows, ok := rawWorkflow["flows"].([]any)
	if !ok || len(rawFlows) == 0 {
		return nil, fmt.Errorf("invalid flows format")
	}

	var rawSteps []map[string]any
	for _, rf := range rawFlows {
		rfMap, ok := rf.(map[string]any)
		if !ok {
			continue
		}
		if name, ok := rfMap["name"].(string); ok && name == flow.Name {
			if steps, ok := rfMap["steps"].([]any); ok {
				for _, step := range steps {
					if stepMap, ok := step.(map[string]any); ok && len(stepMap) == 1 {
						rawSteps = append(rawSteps, stepMap)
					}
				}
			}
			break
		}
	}

	// Process steps
	nodeInfoMap := make(map[string]*nodeInfo)
	nodeList := make([]*nodeInfo, 0)

	for i, rawStep := range rawSteps {
		for stepType, stepData := range rawStep {
			dataMap, ok := stepData.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("invalid step data format")
			}

			nodeName, ok := dataMap["name"].(string)
			if !ok || nodeName == "" {
				return nil, fmt.Errorf("step missing required 'name' field")
			}

			nodeID := idwrap.NewNow()
			info := &nodeInfo{
				id:    nodeID,
				name:  nodeName,
				index: i,
			}

			// Get dependencies
			if deps, ok := dataMap["depends_on"].([]any); ok {
				for _, dep := range deps {
					if depStr, ok := dep.(string); ok {
						info.dependsOn = append(info.dependsOn, depStr)
					}
				}
			}

			nodeInfoMap[nodeName] = info
			nodeList = append(nodeList, info)

			// Process step based on type
			switch stepType {
			case "request":
				if err := processRequestStep(workflowData, flowID, nodeID, nodeName, dataMap, templates, varMap); err != nil {
					return nil, err
				}
			case "if":
				if err := processIfStep(workflowData, flowID, nodeID, nodeName, dataMap); err != nil {
					return nil, err
				}
			case "for":
				if err := processForStep(workflowData, flowID, nodeID, nodeName, dataMap); err != nil {
					return nil, err
				}
			case "for_each":
				if err := processForEachStep(workflowData, flowID, nodeID, nodeName, dataMap); err != nil {
					return nil, err
				}
			case "js":
				if err := processJSStep(workflowData, flowID, nodeID, nodeName, dataMap); err != nil {
					return nil, err
				}
			default:
				return nil, fmt.Errorf("unknown step type: %s", stepType)
			}
		}
	}

	// Create edges
	createEdges(workflowData, flowID, startNodeID, nodeInfoMap, nodeList, rawSteps)

	// Position nodes
	if err := positionNodes(workflowData); err != nil {
		return nil, err
	}

	return workflowData, nil
}

// parseRequestTemplates parses request templates into a map
func parseRequestTemplates(templates map[string]map[string]any) map[string]*requestTemplate {
	result := make(map[string]*requestTemplate)
	for name, tmpl := range templates {
		rt := &requestTemplate{}
		if method, ok := tmpl["method"].(string); ok {
			rt.method = method
		}
		if url, ok := tmpl["url"].(string); ok {
			rt.url = url
		}
		if headers, ok := tmpl["headers"].([]any); ok {
			for _, h := range headers {
				if hMap, ok := h.(map[string]any); ok {
					// Convert map[string]any to map[string]string
					strMap := make(map[string]string)
					for k, v := range hMap {
						if str, ok := v.(string); ok {
							strMap[k] = str
						}
					}
					rt.headers = append(rt.headers, strMap)
				} else if hMap, ok := h.(map[string]string); ok {
					rt.headers = append(rt.headers, hMap)
				}
			}
		}
		if queryParams, ok := tmpl["query_params"].([]any); ok {
			for _, q := range queryParams {
				if qMap, ok := q.(map[string]string); ok {
					rt.queryParams = append(rt.queryParams, qMap)
				}
			}
		}
		if body, ok := tmpl["body"].(map[string]any); ok {
			rt.body = body
		}
		result[name] = rt
	}
	return result
}

// parseRequests parses the new requests format into templates
func parseRequests(requests []map[string]any) map[string]*requestTemplate {
	result := make(map[string]*requestTemplate)
	
	for _, req := range requests {
		// Get the request name
		name, ok := req["name"].(string)
		if !ok || name == "" {
			continue
		}
		
		rt := &requestTemplate{}
		
		if method, ok := req["method"].(string); ok {
			rt.method = method
		}
		if url, ok := req["url"].(string); ok {
			rt.url = url
		}
		
		// Parse headers
		if headers, ok := req["headers"].(map[string]any); ok {
			// Headers as a map (new format)
			strMap := make(map[string]string)
			for k, v := range headers {
				if str, ok := v.(string); ok {
					strMap[k] = str
				}
			}
			rt.headers = append(rt.headers, strMap)
		} else if headers, ok := req["headers"].([]any); ok {
			// Headers as an array (old format)
			for _, h := range headers {
				if hMap, ok := h.(map[string]any); ok {
					strMap := make(map[string]string)
					for k, v := range hMap {
						if str, ok := v.(string); ok {
							strMap[k] = str
						}
					}
					rt.headers = append(rt.headers, strMap)
				} else if hMap, ok := h.(map[string]string); ok {
					rt.headers = append(rt.headers, hMap)
				}
			}
		}
		
		// Parse query params
		if queryParams, ok := req["query_params"].(map[string]any); ok {
			// Query params as a map (new format)
			strMap := make(map[string]string)
			for k, v := range queryParams {
				if str, ok := v.(string); ok {
					strMap[k] = str
				}
			}
			rt.queryParams = append(rt.queryParams, strMap)
		} else if queryParams, ok := req["query_params"].([]any); ok {
			// Query params as an array (old format)
			for _, q := range queryParams {
				if qMap, ok := q.(map[string]any); ok {
					strMap := make(map[string]string)
					for k, v := range qMap {
						if str, ok := v.(string); ok {
							strMap[k] = str
						}
					}
					rt.queryParams = append(rt.queryParams, strMap)
				} else if qMap, ok := q.(map[string]string); ok {
					rt.queryParams = append(rt.queryParams, qMap)
				}
			}
		}
		
		// Parse body
		if body, ok := req["body"].(map[string]any); ok {
			rt.body = body
		}
		
		result[name] = rt
	}
	
	return result
}

// processRequestStep processes a request step
func processRequestStep(data *WorkflowData, flowID, nodeID idwrap.IDWrap, nodeName string, stepData map[string]any, templates map[string]*requestTemplate, varMap varsystem.VarMap) error {
	// Create base node
	node := mnnode.MNode{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     nodeName,
		NodeKind: mnnode.NODE_KIND_REQUEST,
	}
	data.Nodes = append(data.Nodes, node)

	// Get request data
	var method, url string
	var headers []map[string]string
	var queryParams []map[string]string
	var body map[string]any

	// Check if using template
	if useRequest, ok := stepData["use_request"].(string); ok && useRequest != "" {
		if tmpl, exists := templates[useRequest]; exists {
			method = tmpl.method
			url = tmpl.url
			headers = append(headers, tmpl.headers...)
			queryParams = append(queryParams, tmpl.queryParams...)
			body = tmpl.body
		}
	}

	// Override with step-specific data
	if m, ok := stepData["method"].(string); ok && m != "" {
		method = m
	}
	if u, ok := stepData["url"].(string); ok && u != "" {
		url = u
	}
	if h, ok := stepData["headers"].(map[string]any); ok {
		// Headers as a map (new format)
		headerMap := make(map[string]string)
		for k, v := range h {
			if vs, ok := v.(string); ok {
				headerMap[k] = vs
			}
		}
		headers = append(headers, headerMap)
	} else if h, ok := stepData["headers"].([]any); ok {
		// Headers as an array (old format)
		for _, header := range h {
			if hMap, ok := header.(map[string]any); ok {
				headerMap := make(map[string]string)
				for k, v := range hMap {
					if vs, ok := v.(string); ok {
						headerMap[k] = vs
					}
				}
				headers = append(headers, headerMap)
			}
		}
	}
	if q, ok := stepData["query_params"].([]any); ok {
		// Append to existing query params from template
		for _, query := range q {
			if qMap, ok := query.(map[string]any); ok {
				queryMap := make(map[string]string)
				for k, v := range qMap {
					if vs, ok := v.(string); ok {
						queryMap[k] = vs
					}
				}
				queryParams = append(queryParams, queryMap)
			}
		}
	}
	if b, ok := stepData["body"].(map[string]any); ok {
		body = b
	}

	// Set defaults
	if method == "" {
		method = "GET"
	}
	if url == "" {
		return fmt.Errorf("request step '%s' missing required url", nodeName)
	}

	// Create base endpoint
	endpointID := idwrap.NewNow()
	endpoint := mitemapi.ItemApi{
		ID:     endpointID,
		Name:   nodeName,
		Url:    url,
		Method: method,
	}
	data.Endpoints = append(data.Endpoints, endpoint)

	// Create base example (marked as default)
	exampleID := idwrap.NewNow()
	example := mitemapiexample.ItemApiExample{
		ID:        exampleID,
		Name:      nodeName,
		ItemApiID: endpointID,
		IsDefault: true,
	}
	data.Examples = append(data.Examples, example)

	// Create default example with resolved variables
	defaultExampleID := idwrap.NewNow()
	defaultExample := mitemapiexample.ItemApiExample{
		ID:        defaultExampleID,
		Name:      fmt.Sprintf("%s (default)", nodeName),
		ItemApiID: endpointID,
		IsDefault: false,
	}
	data.Examples = append(data.Examples, defaultExample)

	// Create delta endpoint (keeps original URL with variables)
	deltaEndpointID := idwrap.NewNow()
	deltaEndpoint := mitemapi.ItemApi{
		ID:            deltaEndpointID,
		Name:          fmt.Sprintf("%s (delta)", nodeName),
		Url:           url, // Keep original URL with variables
		Method:        method,
		DeltaParentID: &endpointID,
		Hidden:        true,
	}
	data.Endpoints = append(data.Endpoints, deltaEndpoint)

	// Create delta example (marked as default for the delta endpoint)
	deltaExampleID := idwrap.NewNow()
	deltaExample := mitemapiexample.ItemApiExample{
		ID:        deltaExampleID,
		Name:      fmt.Sprintf("%s (delta)", nodeName),
		ItemApiID: deltaEndpointID,
		IsDefault: true,
	}
	data.Examples = append(data.Examples, deltaExample)

	// Process headers
	for _, h := range headers {
		// Headers are in format: {"name": "HeaderName", "value": "HeaderValue"}
		headerName, nameOk := h["name"]
		headerValue, valueOk := h["value"]
		if !nameOk || !valueOk {
			continue
		}
		
		// Base header
		header := mexampleheader.Header{
			ID:        idwrap.NewNow(),
			ExampleID: exampleID,
			HeaderKey: headerName,
			Value:     headerValue,
			Enable:    true,
		}
		data.Headers = append(data.Headers, header)

		// Default header with resolved value
		resolvedValue, _ := varMap.ReplaceVars(headerValue)
		defaultHeader := mexampleheader.Header{
			ID:        idwrap.NewNow(),
			ExampleID: defaultExampleID,
			HeaderKey: headerName,
			Value:     resolvedValue,
			Enable:    true,
		}
		data.Headers = append(data.Headers, defaultHeader)

		// Delta header (only if contains variables)
		if varsystem.CheckStringHasAnyVarKey(headerValue) {
			deltaHeader := mexampleheader.Header{
				ID:            idwrap.NewNow(),
				ExampleID:     deltaExampleID,
				HeaderKey:     headerName,
				Value:         headerValue, // Keep original value with variables
				DeltaParentID: &defaultHeader.ID,
				Enable:        true,
			}
			data.Headers = append(data.Headers, deltaHeader)
		}
	}

	// Process query parameters
	for _, q := range queryParams {
		// Query params can be in format: {"name": "ParamName", "value": "ParamValue"}
		// or direct key-value pairs
		queryName := ""
		queryValue := ""
		
		if name, nameOk := q["name"]; nameOk {
			// Format: {"name": "ParamName", "value": "ParamValue"}
			queryName = name
			queryValue = q["value"]
		} else {
			// Direct key-value format
			for k, v := range q {
				queryName = k
				queryValue = v
				break // Only process first key-value pair
			}
		}
		
		if queryName == "" {
			continue
		}
		
		// Base query
		query := mexamplequery.Query{
			ID:        idwrap.NewNow(),
			ExampleID: exampleID,
			QueryKey:  queryName,
			Value:     queryValue,
			Enable:    true,
		}
		data.Queries = append(data.Queries, query)

		// Default query with resolved value
		resolvedValue, _ := varMap.ReplaceVars(queryValue)
		defaultQuery := mexamplequery.Query{
			ID:        idwrap.NewNow(),
			ExampleID: defaultExampleID,
			QueryKey:  queryName,
			Value:     resolvedValue,
			Enable:    true,
		}
		data.Queries = append(data.Queries, defaultQuery)

		// Delta query (only if contains variables)
		if varsystem.CheckStringHasAnyVarKey(queryValue) {
			deltaQuery := mexamplequery.Query{
				ID:            idwrap.NewNow(),
				ExampleID:     deltaExampleID,
				QueryKey:      queryName,
				Value:         queryValue, // Keep original value with variables
				DeltaParentID: &defaultQuery.ID,
				Enable:        true,
			}
			data.Queries = append(data.Queries, deltaQuery)
		}
	}

	// Process body
	if body != nil {
		// Body can be a direct map or wrapped in body_json
		var bodyData map[string]any
		if bodyJSON, ok := body["body_json"].(map[string]any); ok {
			bodyData = bodyJSON
		} else {
			// Direct body format from templates
			bodyData = body
		}
		
		if bodyData != nil {
			jsonData, err := json.Marshal(bodyData)
			if err != nil {
				return fmt.Errorf("failed to marshal body: %w", err)
			}

			// Base body
			rawBody := mbodyraw.ExampleBodyRaw{
				ID:        idwrap.NewNow(),
				ExampleID: exampleID,
				Data:      jsonData,
			}
			data.RawBodies = append(data.RawBodies, rawBody)

			// Default body
			defaultBody := mbodyraw.ExampleBodyRaw{
				ID:        idwrap.NewNow(),
				ExampleID: defaultExampleID,
				Data:      jsonData,
			}
			data.RawBodies = append(data.RawBodies, defaultBody)

			// Delta body
			deltaBody := mbodyraw.ExampleBodyRaw{
				ID:        idwrap.NewNow(),
				ExampleID: deltaExampleID,
				Data:      jsonData,
			}
			data.RawBodies = append(data.RawBodies, deltaBody)
		}
	} else {
		// Create empty bodies for requests without body (like GET requests)
		emptyData := []byte("{}")
		
		// Base body
		rawBody := mbodyraw.ExampleBodyRaw{
			ID:        idwrap.NewNow(),
			ExampleID: exampleID,
			Data:      emptyData,
		}
		data.RawBodies = append(data.RawBodies, rawBody)

		// Default body
		defaultBody := mbodyraw.ExampleBodyRaw{
			ID:        idwrap.NewNow(),
			ExampleID: defaultExampleID,
			Data:      emptyData,
		}
		data.RawBodies = append(data.RawBodies, defaultBody)

		// Delta body
		deltaBody := mbodyraw.ExampleBodyRaw{
			ID:        idwrap.NewNow(),
			ExampleID: deltaExampleID,
			Data:      emptyData,
		}
		data.RawBodies = append(data.RawBodies, deltaBody)
	}

	// Create request node
	requestNode := mnrequest.MNRequest{
		FlowNodeID:      nodeID,
		EndpointID:      &endpointID,
		ExampleID:       &exampleID,
		DeltaEndpointID: &deltaEndpointID,
		DeltaExampleID:  &deltaExampleID,
	}
	data.RequestNodes = append(data.RequestNodes, requestNode)

	return nil
}

// processIfStep processes an if step
func processIfStep(data *WorkflowData, flowID, nodeID idwrap.IDWrap, nodeName string, stepData map[string]any) error {
	node := mnnode.MNode{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     nodeName,
		NodeKind: mnnode.NODE_KIND_CONDITION,
	}
	data.Nodes = append(data.Nodes, node)

	condition, ok := stepData["condition"].(string)
	if !ok || condition == "" {
		return fmt.Errorf("if step '%s' missing required condition", nodeName)
	}

	ifNode := mnif.MNIF{
		FlowNodeID: nodeID,
		Condition: mcondition.Condition{
			Comparisons: mcondition.Comparison{
				Expression: condition,
			},
		},
	}

	// Store then/else targets for later edge creation
	// Note: The MNIF model doesn't have ThenID/ElseID fields,
	// so we'll handle the edges separately in createEdges

	data.ConditionNodes = append(data.ConditionNodes, ifNode)
	return nil
}

// processForStep processes a for step
func processForStep(data *WorkflowData, flowID, nodeID idwrap.IDWrap, nodeName string, stepData map[string]any) error {
	node := mnnode.MNode{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     nodeName,
		NodeKind: mnnode.NODE_KIND_FOR,
	}
	data.Nodes = append(data.Nodes, node)

	iterCount, ok := stepData["iter_count"].(int)
	if !ok {
		// Try float64 (YAML numbers are often parsed as float64)
		if f, ok := stepData["iter_count"].(float64); ok {
			iterCount = int(f)
		} else {
			return fmt.Errorf("for step '%s' missing required iter_count", nodeName)
		}
	}

	forNode := mnfor.MNFor{
		FlowNodeID: nodeID,
		IterCount:  int64(iterCount),
	}

	// Note: The MNFor model doesn't have LoopID field,
	// so we'll handle the loop edges separately in createEdges

	data.ForNodes = append(data.ForNodes, forNode)
	return nil
}

// processForEachStep processes a for_each step
func processForEachStep(data *WorkflowData, flowID, nodeID idwrap.IDWrap, nodeName string, stepData map[string]any) error {
	node := mnnode.MNode{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     nodeName,
		NodeKind: mnnode.NODE_KIND_FOR_EACH,
	}
	data.Nodes = append(data.Nodes, node)

	items, ok := stepData["items"].(string)
	if !ok || items == "" {
		return fmt.Errorf("for_each step '%s' missing required items", nodeName)
	}

	// For for_each, we'll also use MNFor but without iteration count
	// The actual implementation might need a separate type or field
	forNode := mnfor.MNFor{
		FlowNodeID: nodeID,
		// Note: MNFor doesn't have ForEachItems field, so this is simplified
		// In a real implementation, you might need to store the items expression elsewhere
	}

	data.ForNodes = append(data.ForNodes, forNode)
	return nil
}

// processJSStep processes a JavaScript step
func processJSStep(data *WorkflowData, flowID, nodeID idwrap.IDWrap, nodeName string, stepData map[string]any) error {
	node := mnnode.MNode{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     nodeName,
		NodeKind: mnnode.NODE_KIND_JS,
	}
	data.Nodes = append(data.Nodes, node)

	code, ok := stepData["code"].(string)
	if !ok || code == "" {
		return fmt.Errorf("js step '%s' missing required code", nodeName)
	}

	jsNode := mnjs.MNJS{
		FlowNodeID: nodeID,
		Code:       []byte(code),
	}
	data.JSNodes = append(data.JSNodes, jsNode)
	return nil
}

// createEdges creates edges based on dependencies and sequential order
func createEdges(data *WorkflowData, flowID, startNodeID idwrap.IDWrap, nodeInfoMap map[string]*nodeInfo, nodeList []*nodeInfo, rawSteps []map[string]any) {
	// Track which nodes have incoming edges
	hasIncoming := make(map[idwrap.IDWrap]bool)

	// Create edges for explicit dependencies
	for _, info := range nodeList {
		for _, depName := range info.dependsOn {
			if depInfo, exists := nodeInfoMap[depName]; exists {
				edge := edge.Edge{
					ID:            idwrap.NewNow(),
					FlowID:        flowID,
					SourceID:      depInfo.id,
					TargetID:      info.id,
					SourceHandler: edge.HandleUnspecified,
				}
				data.Edges = append(data.Edges, edge)
				hasIncoming[info.id] = true
			}
		}
	}

	// Create edges for sequential steps (implicit dependencies)
	for i := 0; i < len(nodeList)-1; i++ {
		current := nodeList[i]
		next := nodeList[i+1]

		// Only create sequential edge if next node doesn't have explicit dependencies
		if len(next.dependsOn) == 0 && !hasIncoming[next.id] {
			edge := edge.Edge{
				ID:            idwrap.NewNow(),
				FlowID:        flowID,
				SourceID:      current.id,
				TargetID:      next.id,
				SourceHandler: edge.HandleUnspecified,
			}
			data.Edges = append(data.Edges, edge)
			hasIncoming[next.id] = true
		}
	}

	// Connect nodes without incoming edges to start node
	for _, info := range nodeList {
		if !hasIncoming[info.id] {
			edge := edge.Edge{
				ID:            idwrap.NewNow(),
				FlowID:        flowID,
				SourceID:      startNodeID,
				TargetID:      info.id,
				SourceHandler: edge.HandleUnspecified,
			}
			data.Edges = append(data.Edges, edge)
		}
	}

	// Create edges for control flow nodes
	// We need to parse the original step data to get then/else/loop targets
	for _, rawStep := range rawSteps {
		for stepType, stepData := range rawStep {
			dataMap, _ := stepData.(map[string]any)
			nodeName, _ := dataMap["name"].(string)
			nodeID := nodeInfoMap[nodeName].id

			switch stepType {
			case "if":
				// Create edges for then/else branches
				if thenTarget, ok := dataMap["then"].(string); ok && thenTarget != "" {
					if targetID, exists := nodeInfoMap[thenTarget]; exists {
						edge := edge.Edge{
							ID:            idwrap.NewNow(),
							FlowID:        flowID,
							SourceID:      nodeID,
							TargetID:      targetID.id,
							SourceHandler: edge.HandleThen,
						}
						data.Edges = append(data.Edges, edge)
						hasIncoming[targetID.id] = true
					}
				}

				if elseTarget, ok := dataMap["else"].(string); ok && elseTarget != "" {
					if targetID, exists := nodeInfoMap[elseTarget]; exists {
						edge := edge.Edge{
							ID:            idwrap.NewNow(),
							FlowID:        flowID,
							SourceID:      nodeID,
							TargetID:      targetID.id,
							SourceHandler: edge.HandleElse,
						}
						data.Edges = append(data.Edges, edge)
						hasIncoming[targetID.id] = true
					}
				}

			case "for", "for_each":
				// Create edge for loop body
				if loopTarget, ok := dataMap["loop"].(string); ok && loopTarget != "" {
					if targetID, exists := nodeInfoMap[loopTarget]; exists {
						edge := edge.Edge{
							ID:            idwrap.NewNow(),
							FlowID:        flowID,
							SourceID:      nodeID,
							TargetID:      targetID.id,
							SourceHandler: edge.HandleLoop,
						}
						data.Edges = append(data.Edges, edge)
						hasIncoming[targetID.id] = true
					}
				}
			}
		}
	}
}