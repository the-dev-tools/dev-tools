package workflowsimple

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mcollection"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mexamplequery"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mflowvariable"
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

// ConvertSimplifiedYAML converts simplified YAML to all the entities needed for import
func ConvertSimplifiedYAML(data []byte, collectionID, workspaceID idwrap.IDWrap) (SimplifiedYAMLResolved, error) {
	result := SimplifiedYAMLResolved{}

	// Parse the YAML
	workflowData, err := Parse(data)
	if err != nil {
		return result, err
	}

	// Create collection
	collection := mcollection.Collection{
		ID:          collectionID,
		Name:        "Workflow Collection",
		WorkspaceID: workspaceID,
	}
	result.Collections = append(result.Collections, collection)

	// Convert flow data
	flow := workflowData.Flow
	flow.WorkspaceID = workspaceID
	result.Flows = append(result.Flows, flow)

	// Copy all flow nodes
	result.FlowNodes = workflowData.Nodes

	// Copy all edges
	result.FlowEdges = workflowData.Edges

	// Convert variables to flow variables
	for _, v := range workflowData.Variables {
		flowVar := mflowvariable.FlowVariable{
			ID:      idwrap.NewNow(),
			FlowID:  flow.ID,
			Name:    v.VarKey,
			Value:   v.Value,
			Enabled: true,
		}
		result.FlowVariables = append(result.FlowVariables, flowVar)
	}

	// Copy node implementations
	result.FlowRequestNodes = workflowData.RequestNodes
	result.FlowConditionNodes = workflowData.ConditionNodes
	result.FlowNoopNodes = workflowData.NoopNodes
	result.FlowForNodes = workflowData.ForNodes
	result.FlowJSNodes = workflowData.JSNodes

	// Process endpoints and examples
	for _, endpoint := range workflowData.Endpoints {
		// Set collection ID
		endpoint.CollectionID = collectionID
		result.Endpoints = append(result.Endpoints, endpoint)
	}

	// Process examples
	for _, example := range workflowData.Examples {
		// Set collection ID
		example.CollectionID = collectionID
		result.Examples = append(result.Examples, example)
	}

	// Copy headers, queries, and bodies
	result.Headers = workflowData.Headers
	result.Queries = workflowData.Queries
	result.RawBodies = workflowData.RawBodies

	// Set Prev/Next for endpoints
	for i := range result.Endpoints {
		if i > 0 {
			prevID := &result.Endpoints[i-1].ID
			result.Endpoints[i].Prev = prevID
		}
		if i < len(result.Endpoints)-1 {
			nextID := &result.Endpoints[i+1].ID
			result.Endpoints[i].Next = nextID
		}
	}

	// Set Prev/Next for examples
	for i := range result.Examples {
		if i > 0 {
			prevID := &result.Examples[i-1].ID
			result.Examples[i].Prev = prevID
		}
		if i < len(result.Examples)-1 {
			nextID := &result.Examples[i+1].ID
			result.Examples[i].Next = nextID
		}
	}

	return result, nil
}


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
	// Note: You can set a "timeout" variable to control flow execution timeout (in seconds)
	// Default is 60 seconds if not specified. Example: - name: timeout, value: "300"
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
		// Headers only support map format
		if headers, ok := tmpl["headers"].(map[string]any); ok {
			// Map format: {"X-Header": "value"}
			// Convert to array of name/value maps for internal consistency
			for k, v := range headers {
				if str, ok := v.(string); ok {
					headerMap := map[string]string{
						"name": k,
						"value": str,
					}
					rt.headers = append(rt.headers, headerMap)
				}
			}
		}
		// Query params only support map format
		if queryParams, ok := tmpl["query_params"].(map[string]any); ok {
			// Map format: {"param": "value"}
			// Convert to array of name/value maps for internal consistency
			for k, v := range queryParams {
				if str, ok := v.(string); ok {
					queryMap := map[string]string{
						"name": k,
						"value": str,
					}
					rt.queryParams = append(rt.queryParams, queryMap)
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
			// Convert to array of name/value maps for internal consistency
			for k, v := range headers {
				if str, ok := v.(string); ok {
					headerMap := map[string]string{
						"name": k,
						"value": str,
					}
					rt.headers = append(rt.headers, headerMap)
				}
			}
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
			// Convert to array of name/value maps for internal consistency
			for k, v := range queryParams {
				if str, ok := v.(string); ok {
					queryMap := map[string]string{
						"name": k,
						"value": str,
					}
					rt.queryParams = append(rt.queryParams, queryMap)
				}
			}
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

	// Track template data for comparison
	var templateHeaders []map[string]string
	var templateQueries []map[string]string
	var templateBody map[string]any
	var usingTemplate bool

	// Check if using template
	if useRequest, ok := stepData["use_request"].(string); ok && useRequest != "" {
		if tmpl, exists := templates[useRequest]; exists {
			usingTemplate = true
			templateHeaders = tmpl.headers
			templateQueries = tmpl.queryParams
			templateBody = tmpl.body
			
			method = tmpl.method
			url = tmpl.url
			// Don't append template data to headers/queryParams/body yet
			// We'll handle them separately to preserve original values
		}
	}

	// Collect step overrides separately
	var stepHeaderOverrides []map[string]string
	var stepQueryOverrides []map[string]string
	var stepBodyOverride map[string]any

	// Override with step-specific data
	if m, ok := stepData["method"].(string); ok && m != "" {
		method = m
	}
	if u, ok := stepData["url"].(string); ok && u != "" {
		url = u
	}
	if h, ok := stepData["headers"].(map[string]any); ok {
		// Headers only support map format
		// Convert to name/value format for internal consistency
		for k, v := range h {
			if vs, ok := v.(string); ok {
				headerMap := map[string]string{
					"name": k,
					"value": vs,
				}
				stepHeaderOverrides = append(stepHeaderOverrides, headerMap)
			}
		}
	}
	// Query params only support map format
	if q, ok := stepData["query_params"].(map[string]any); ok {
		// Map format: {"param": "value"}
		// Convert to name/value format for internal consistency
		for k, v := range q {
			if vs, ok := v.(string); ok {
				queryMap := map[string]string{
					"name": k,
					"value": vs,
				}
				stepQueryOverrides = append(stepQueryOverrides, queryMap)
			}
		}
	}
	if b, ok := stepData["body"].(map[string]any); ok {
		stepBodyOverride = b
	}

	// Now merge template and step data intelligently
	if usingTemplate {
		// Start with template data for base values
		headers = append(headers, templateHeaders...)
		queryParams = append(queryParams, templateQueries...)
		
		// For the actual request execution, we need merged values
		// But we'll track what's an override for delta creation
	} else {
		// No template, use step data directly
		headers = stepHeaderOverrides
		queryParams = stepQueryOverrides
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
		BodyType:  mitemapiexample.BodyTypeRaw,
	}
	data.Examples = append(data.Examples, example)

	// Create default example with resolved variables
	defaultExampleID := idwrap.NewNow()
	defaultExample := mitemapiexample.ItemApiExample{
		ID:        defaultExampleID,
		Name:      fmt.Sprintf("%s (default)", nodeName),
		ItemApiID: endpointID,
		IsDefault: false,
		BodyType:  mitemapiexample.BodyTypeRaw,
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
		ID:              deltaExampleID,
		Name:            fmt.Sprintf("%s (delta)", nodeName),
		ItemApiID:       deltaEndpointID,
		IsDefault:       true,
		VersionParentID: &defaultExampleID, // Link to default example
		BodyType:        mitemapiexample.BodyTypeRaw,
	}
	data.Examples = append(data.Examples, deltaExample)

	// Process headers with proper base/delta separation
	if usingTemplate {
		// Build maps for easier lookup
		templateHeaderMap := make(map[string]string)
		for _, h := range templateHeaders {
			if name, ok := h["name"]; ok {
				if value, ok := h["value"]; ok {
					templateHeaderMap[name] = value
				}
			}
		}
		
		// Check for direct map format too
		if len(templateHeaderMap) == 0 && len(templateHeaders) > 0 {
			// Maybe it's in direct map format
			for _, h := range templateHeaders {
				for k, v := range h {
					if k != "name" && k != "value" {
						templateHeaderMap[k] = v
					}
				}
			}
		}
		
		overrideHeaderMap := make(map[string]string)
		for _, h := range stepHeaderOverrides {
			if name, ok := h["name"]; ok {
				if value, ok := h["value"]; ok {
					overrideHeaderMap[name] = value
				}
			}
		}
		
		// Process all unique headers
		processedHeaders := make(map[string]bool)
		
		// First, process template headers
		for headerName, templateValue := range templateHeaderMap {
			processedHeaders[headerName] = true
			
			// Base header - always use template value
			baseHeader := mexampleheader.Header{
				ID:        idwrap.NewNow(),
				ExampleID: exampleID,
				HeaderKey: headerName,
				Value:     templateValue, // Always use template value for base
				Enable:    true,
			}
			data.Headers = append(data.Headers, baseHeader)
			
			// Default header with resolved template value
			resolvedValue, _ := varMap.ReplaceVars(templateValue)
			defaultHeader := mexampleheader.Header{
				ID:        idwrap.NewNow(),
				ExampleID: defaultExampleID,
				HeaderKey: headerName,
				Value:     resolvedValue,
				Enable:    true,
			}
			data.Headers = append(data.Headers, defaultHeader)
			
			// Check if this header is overridden in the step
			if overrideValue, isOverridden := overrideHeaderMap[headerName]; isOverridden {
				// Create delta with the override value
				deltaHeader := mexampleheader.Header{
					ID:            idwrap.NewNow(),
					ExampleID:     deltaExampleID,
					HeaderKey:     headerName,
					Value:         overrideValue, // Use override value for delta
					DeltaParentID: &defaultHeader.ID,
					Enable:        true,
				}
				data.Headers = append(data.Headers, deltaHeader)
			}
		}
		
		// Process headers that only exist in overrides (additions)
		for headerName, overrideValue := range overrideHeaderMap {
			if !processedHeaders[headerName] {
				// This is a new header not in template
				// For additions, base gets the override value (no template to use)
				baseHeader := mexampleheader.Header{
					ID:        idwrap.NewNow(),
					ExampleID: exampleID,
					HeaderKey: headerName,
					Value:     overrideValue,
					Enable:    true,
				}
				data.Headers = append(data.Headers, baseHeader)
				
				// Default header
				resolvedValue, _ := varMap.ReplaceVars(overrideValue)
				defaultHeader := mexampleheader.Header{
					ID:        idwrap.NewNow(),
					ExampleID: defaultExampleID,
					HeaderKey: headerName,
					Value:     resolvedValue,
					Enable:    true,
				}
				data.Headers = append(data.Headers, defaultHeader)
				
				// Delta header
				deltaHeader := mexampleheader.Header{
					ID:            idwrap.NewNow(),
					ExampleID:     deltaExampleID,
					HeaderKey:     headerName,
					Value:         overrideValue,
					DeltaParentID: &defaultHeader.ID,
					Enable:        true,
				}
				data.Headers = append(data.Headers, deltaHeader)
			}
		}
	} else {
		// No template - process step headers directly
		for _, h := range headers {
			headerName, nameOk := h["name"]
			headerValue, valueOk := h["value"]
			if !nameOk || !valueOk {
				continue
			}
			
			// Base header
			baseHeader := mexampleheader.Header{
				ID:        idwrap.NewNow(),
				ExampleID: exampleID,
				HeaderKey: headerName,
				Value:     headerValue,
				Enable:    true,
			}
			data.Headers = append(data.Headers, baseHeader)
			
			// Default header
			resolvedValue, _ := varMap.ReplaceVars(headerValue)
			defaultHeader := mexampleheader.Header{
				ID:        idwrap.NewNow(),
				ExampleID: defaultExampleID,
				HeaderKey: headerName,
				Value:     resolvedValue,
				Enable:    true,
			}
			data.Headers = append(data.Headers, defaultHeader)
			
			// Delta header if has variables
			if varsystem.CheckStringHasAnyVarKey(headerValue) {
				deltaHeader := mexampleheader.Header{
					ID:            idwrap.NewNow(),
					ExampleID:     deltaExampleID,
					HeaderKey:     headerName,
					Value:         headerValue,
					DeltaParentID: &defaultHeader.ID,
					Enable:        true,
				}
				data.Headers = append(data.Headers, deltaHeader)
			}
		}
	}

	// Process query parameters with proper base/delta separation
	if usingTemplate {
		// Build maps for easier lookup
		templateQueryMap := make(map[string]string)
		for _, q := range templateQueries {
			if name, ok := q["name"]; ok {
				if value, ok := q["value"]; ok {
					templateQueryMap[name] = value
				}
			}
		}
		
		overrideQueryMap := make(map[string]string)
		for _, q := range stepQueryOverrides {
			if name, ok := q["name"]; ok {
				if value, ok := q["value"]; ok {
					overrideQueryMap[name] = value
				}
			}
		}
		
		// Process all unique query params
		processedQueries := make(map[string]bool)
		
		// First, process template queries
		for queryName, templateValue := range templateQueryMap {
			processedQueries[queryName] = true
			
			// Base query - always use template value
			baseQuery := mexamplequery.Query{
				ID:        idwrap.NewNow(),
				ExampleID: exampleID,
				QueryKey:  queryName,
				Value:     templateValue, // Always use template value for base
				Enable:    true,
			}
			data.Queries = append(data.Queries, baseQuery)
			
			// Default query with resolved template value
			resolvedValue, _ := varMap.ReplaceVars(templateValue)
			defaultQuery := mexamplequery.Query{
				ID:        idwrap.NewNow(),
				ExampleID: defaultExampleID,
				QueryKey:  queryName,
				Value:     resolvedValue,
				Enable:    true,
			}
			data.Queries = append(data.Queries, defaultQuery)
			
			// Check if this query is overridden in the step
			if overrideValue, isOverridden := overrideQueryMap[queryName]; isOverridden {
				// Create delta with the override value
				deltaQuery := mexamplequery.Query{
					ID:            idwrap.NewNow(),
					ExampleID:     deltaExampleID,
					QueryKey:      queryName,
					Value:         overrideValue, // Use override value for delta
					DeltaParentID: &defaultQuery.ID,
					Enable:        true,
				}
				data.Queries = append(data.Queries, deltaQuery)
			}
		}
		
		// Process queries that only exist in overrides (additions)
		for queryName, overrideValue := range overrideQueryMap {
			if !processedQueries[queryName] {
				// This is a new query not in template
				// For additions, base gets the override value (no template to use)
				baseQuery := mexamplequery.Query{
					ID:        idwrap.NewNow(),
					ExampleID: exampleID,
					QueryKey:  queryName,
					Value:     overrideValue,
					Enable:    true,
				}
				data.Queries = append(data.Queries, baseQuery)
				
				// Default query
				resolvedValue, _ := varMap.ReplaceVars(overrideValue)
				defaultQuery := mexamplequery.Query{
					ID:        idwrap.NewNow(),
					ExampleID: defaultExampleID,
					QueryKey:  queryName,
					Value:     resolvedValue,
					Enable:    true,
				}
				data.Queries = append(data.Queries, defaultQuery)
				
				// Delta query
				deltaQuery := mexamplequery.Query{
					ID:            idwrap.NewNow(),
					ExampleID:     deltaExampleID,
					QueryKey:      queryName,
					Value:         overrideValue,
					DeltaParentID: &defaultQuery.ID,
					Enable:        true,
				}
				data.Queries = append(data.Queries, deltaQuery)
			}
		}
	} else {
		// No template - process step queries directly
		for _, q := range queryParams {
			var queryName, queryValue string
			
			if name, nameOk := q["name"]; nameOk {
				queryName = name
				queryValue = q["value"]
			} else {
				// Direct key-value format
				for k, v := range q {
					queryName = k
					queryValue = v
					break
				}
			}
			
			if queryName == "" {
				continue
			}
			
			// Base query
			baseQuery := mexamplequery.Query{
				ID:        idwrap.NewNow(),
				ExampleID: exampleID,
				QueryKey:  queryName,
				Value:     queryValue,
				Enable:    true,
			}
			data.Queries = append(data.Queries, baseQuery)
			
			// Default query
			resolvedValue, _ := varMap.ReplaceVars(queryValue)
			defaultQuery := mexamplequery.Query{
				ID:        idwrap.NewNow(),
				ExampleID: defaultExampleID,
				QueryKey:  queryName,
				Value:     resolvedValue,
				Enable:    true,
			}
			data.Queries = append(data.Queries, defaultQuery)
			
			// Delta query if has variables
			if varsystem.CheckStringHasAnyVarKey(queryValue) {
				deltaQuery := mexamplequery.Query{
					ID:            idwrap.NewNow(),
					ExampleID:     deltaExampleID,
					QueryKey:      queryName,
					Value:         queryValue,
					DeltaParentID: &defaultQuery.ID,
					Enable:        true,
				}
				data.Queries = append(data.Queries, deltaQuery)
			}
		}
	}

	// Process body with proper base/delta separation
	var baseBodyData, deltaBodyData []byte
	
	if usingTemplate {
		// Handle template body
		if templateBody != nil {
			jsonData, err := json.Marshal(templateBody)
			if err != nil {
				return fmt.Errorf("failed to marshal template body: %w", err)
			}
			baseBodyData = jsonData // Base uses template
		}
		
		// Handle override body
		if stepBodyOverride != nil {
			jsonData, err := json.Marshal(stepBodyOverride)
			if err != nil {
				return fmt.Errorf("failed to marshal override body: %w", err)
			}
			deltaBodyData = jsonData // Delta uses override
		} else if templateBody != nil {
			// No override, delta uses template too
			deltaBodyData = baseBodyData
		}
	} else {
		// No template - use step body for everything
		if stepBodyOverride != nil {
			jsonData, err := json.Marshal(stepBodyOverride)
			if err != nil {
				return fmt.Errorf("failed to marshal body: %w", err)
			}
			baseBodyData = jsonData
			deltaBodyData = jsonData
		}
	}
	
	// Create bodies
	if baseBodyData == nil {
		baseBodyData = []byte("{}")
	}
	if deltaBodyData == nil {
		deltaBodyData = []byte("{}")
	}
	
	// Base body
	rawBody := mbodyraw.ExampleBodyRaw{
		ID:            idwrap.NewNow(),
		ExampleID:     exampleID,
		Data:          baseBodyData,
		CompressType:  compress.CompressTypeNone,
		VisualizeMode: mbodyraw.VisualizeModeJSON, // Default to JSON since we're creating JSON bodies
	}
	// Check if it's actually valid JSON
	if !json.Valid(baseBodyData) {
		rawBody.VisualizeMode = mbodyraw.VisualizeModeText
	}
	data.RawBodies = append(data.RawBodies, rawBody)

	// Default body (uses base data)
	defaultBody := mbodyraw.ExampleBodyRaw{
		ID:            idwrap.NewNow(),
		ExampleID:     defaultExampleID,
		Data:          baseBodyData,
		CompressType:  compress.CompressTypeNone,
		VisualizeMode: rawBody.VisualizeMode, // Use same mode as base body
	}
	data.RawBodies = append(data.RawBodies, defaultBody)

	// Delta body - ALWAYS create when we have a delta example
	// Flow execution expects delta raw body to exist for all delta examples
	deltaBody := mbodyraw.ExampleBodyRaw{
		ID:            idwrap.NewNow(),
		ExampleID:     deltaExampleID,
		Data:          deltaBodyData,
		CompressType:  compress.CompressTypeNone,
		VisualizeMode: mbodyraw.VisualizeModeJSON, // Default to JSON
	}
	// Check if delta body is valid JSON
	if !json.Valid(deltaBodyData) {
		deltaBody.VisualizeMode = mbodyraw.VisualizeModeText
	}
	data.RawBodies = append(data.RawBodies, deltaBody)

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