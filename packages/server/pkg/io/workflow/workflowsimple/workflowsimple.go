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
	"the-dev-tools/server/pkg/model/mnnode/mnforeach"
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
	result.FlowForEachNodes = workflowData.ForEachNodes
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
		ForEachNodes:   make([]mnforeach.MNForEach, 0),
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
					if depStr, ok := dep.(string); ok && depStr != "" {
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

// parseRequestData parses a single request template from data
func parseRequestData(data map[string]any) *requestTemplate {
	rt := &requestTemplate{}
	if method, ok := data["method"].(string); ok && method != "" {
		rt.method = method
	}
	if url, ok := data["url"].(string); ok && url != "" {
		rt.url = url
	}

	// Parse headers - support both map and array formats
	if headers, ok := data["headers"].(map[string]any); ok {
		// Map format: {"X-Header": "value"}
		for k, v := range headers {
			if str, ok := v.(string); ok {
				rt.headers = append(rt.headers, map[string]string{
					"name":  k,
					"value": str,
				})
			}
		}
	} else if headers, ok := data["headers"].([]any); ok {
		// Array format (for parseRequests compatibility)
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

	// Parse query params - support both map and array formats
	if queryParams, ok := data["query_params"].(map[string]any); ok {
		// Map format: {"param": "value"}
		for k, v := range queryParams {
			if str, ok := v.(string); ok {
				rt.queryParams = append(rt.queryParams, map[string]string{
					"name":  k,
					"value": str,
				})
			}
		}
	} else if queryParams, ok := data["query_params"].([]any); ok {
		// Array format (for parseRequests compatibility)
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

	if body, ok := data["body"].(map[string]any); ok {
		rt.body = body
	}

	return rt
}

// parseRequestTemplates parses request templates into a map
func parseRequestTemplates(templates map[string]map[string]any) map[string]*requestTemplate {
	result := make(map[string]*requestTemplate)
	for name, tmpl := range templates {
		result[name] = parseRequestData(tmpl)
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

		result[name] = parseRequestData(req)
	}

	return result
}

// requestContext holds all the IDs and data needed for request processing
type requestContext struct {
	nodeID           idwrap.IDWrap
	endpointID       idwrap.IDWrap
	deltaEndpointID  idwrap.IDWrap
	exampleID        idwrap.IDWrap
	defaultExampleID idwrap.IDWrap
	deltaExampleID   idwrap.IDWrap
}

// createRequestEntities creates all the entities needed for a request node
func createRequestEntities(data *WorkflowData, flowID, nodeID idwrap.IDWrap, nodeName, url, method string) *requestContext {
	ctx := &requestContext{
		nodeID:           nodeID,
		endpointID:       idwrap.NewNow(),
		deltaEndpointID:  idwrap.NewNow(),
		exampleID:        idwrap.NewNow(),
		defaultExampleID: idwrap.NewNow(),
		deltaExampleID:   idwrap.NewNow(),
	}

	// Add node
	data.Nodes = append(data.Nodes, mnnode.MNode{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     nodeName,
		NodeKind: mnnode.NODE_KIND_REQUEST,
	})

	// Add endpoints
	data.Endpoints = append(data.Endpoints,
		mitemapi.ItemApi{ID: ctx.endpointID, Name: nodeName, Url: url, Method: method},
		mitemapi.ItemApi{
			ID:            ctx.deltaEndpointID,
			Name:          fmt.Sprintf("%s (delta)", nodeName),
			Url:           url,
			Method:        method,
			DeltaParentID: &ctx.endpointID,
			Hidden:        true,
		},
	)

	// Add examples
	data.Examples = append(data.Examples,
		mitemapiexample.ItemApiExample{
			ID:        ctx.exampleID,
			Name:      nodeName,
			ItemApiID: ctx.endpointID,
			IsDefault: true,
			BodyType:  mitemapiexample.BodyTypeRaw,
		},
		mitemapiexample.ItemApiExample{
			ID:        ctx.defaultExampleID,
			Name:      fmt.Sprintf("%s (default)", nodeName),
			ItemApiID: ctx.endpointID,
			IsDefault: false,
			BodyType:  mitemapiexample.BodyTypeRaw,
		},
		mitemapiexample.ItemApiExample{
			ID:              ctx.deltaExampleID,
			Name:            fmt.Sprintf("%s (delta)", nodeName),
			ItemApiID:       ctx.deltaEndpointID,
			IsDefault:       true,
			VersionParentID: &ctx.defaultExampleID,
			BodyType:        mitemapiexample.BodyTypeRaw,
		},
	)

	// Add request node
	data.RequestNodes = append(data.RequestNodes, mnrequest.MNRequest{
		FlowNodeID:      nodeID,
		EndpointID:      &ctx.endpointID,
		ExampleID:       &ctx.exampleID,
		DeltaEndpointID: &ctx.deltaEndpointID,
		DeltaExampleID:  &ctx.deltaExampleID,
	})

	return ctx
}

// addBody adds body data for all three examples
func addBody(data *WorkflowData, ctx *requestContext, bodyData []byte) {
	if bodyData == nil {
		bodyData = []byte("{}")
	}

	visualMode := mbodyraw.VisualizeModeJSON
	if !json.Valid(bodyData) {
		visualMode = mbodyraw.VisualizeModeText
	}

	for _, exampleID := range []idwrap.IDWrap{ctx.exampleID, ctx.defaultExampleID, ctx.deltaExampleID} {
		data.RawBodies = append(data.RawBodies, mbodyraw.ExampleBodyRaw{
			ID:            idwrap.NewNow(),
			ExampleID:     exampleID,
			Data:          bodyData,
			CompressType:  compress.CompressTypeNone,
			VisualizeMode: visualMode,
		})
	}
}

// processNameValuePairs processes headers or query parameters generically
func processNameValuePairs(
	data *WorkflowData,
	exampleID, defaultExampleID, deltaExampleID idwrap.IDWrap,
	templatePairs, overridePairs []map[string]string,
	usingTemplate bool,
	varMap varsystem.VarMap,
	createFunc func(id idwrap.IDWrap, exampleID idwrap.IDWrap, name, value string, deltaParentID *idwrap.IDWrap) interface{},
	appendFunc func(interface{}),
) {
	// Convert to common format
	toNameValueMap := func(pairs []map[string]string) map[string]string {
		result := make(map[string]string)
		for _, p := range pairs {
			if name, ok := p["name"]; ok {
				if value, ok := p["value"]; ok {
					result[name] = value
				}
			} else {
				// Direct map format
				for k, v := range p {
					if k != "name" && k != "value" {
						result[k] = v
					}
				}
			}
		}
		return result
	}

	if usingTemplate {
		templateMap := toNameValueMap(templatePairs)
		overrideMap := toNameValueMap(overridePairs)
		processedNames := make(map[string]bool)

		// Process template items
		for name, templateValue := range templateMap {
			processedNames[name] = true

			// Base item
			baseItem := createFunc(idwrap.NewNow(), exampleID, name, templateValue, nil)
			appendFunc(baseItem)

			// Default item with resolved value
			resolvedValue, _ := varMap.ReplaceVars(templateValue)
			defaultItem := createFunc(idwrap.NewNow(), defaultExampleID, name, resolvedValue, nil)
			appendFunc(defaultItem)

			// Check if overridden
			if overrideValue, isOverridden := overrideMap[name]; isOverridden {
				// Need to get ID from the actual type
				var defaultID idwrap.IDWrap
				switch v := defaultItem.(type) {
				case mexampleheader.Header:
					defaultID = v.ID
				case mexamplequery.Query:
					defaultID = v.ID
				}
				deltaItem := createFunc(idwrap.NewNow(), deltaExampleID, name, overrideValue, &defaultID)
				appendFunc(deltaItem)
			}
		}

		// Process override-only items
		for name, overrideValue := range overrideMap {
			if !processedNames[name] {
				// Base item
				baseItem := createFunc(idwrap.NewNow(), exampleID, name, overrideValue, nil)
				appendFunc(baseItem)

				// Default item
				resolvedValue, _ := varMap.ReplaceVars(overrideValue)
				defaultItem := createFunc(idwrap.NewNow(), defaultExampleID, name, resolvedValue, nil)
				appendFunc(defaultItem)

				// Delta item
				var defaultID idwrap.IDWrap
				switch v := defaultItem.(type) {
				case mexampleheader.Header:
					defaultID = v.ID
				case mexamplequery.Query:
					defaultID = v.ID
				}
				deltaItem := createFunc(idwrap.NewNow(), deltaExampleID, name, overrideValue, &defaultID)
				appendFunc(deltaItem)
			}
		}
	} else {
		// No template - process directly
		directMap := toNameValueMap(overridePairs)
		for name, value := range directMap {
			// Base item
			baseItem := createFunc(idwrap.NewNow(), exampleID, name, value, nil)
			appendFunc(baseItem)

			// Default item
			resolvedValue, _ := varMap.ReplaceVars(value)
			defaultItem := createFunc(idwrap.NewNow(), defaultExampleID, name, resolvedValue, nil)
			appendFunc(defaultItem)

			// Delta item if has variables
			if varsystem.CheckStringHasAnyVarKey(value) {
				var defaultID idwrap.IDWrap
				switch v := defaultItem.(type) {
				case mexampleheader.Header:
					defaultID = v.ID
				case mexamplequery.Query:
					defaultID = v.ID
				}
				deltaItem := createFunc(idwrap.NewNow(), deltaExampleID, name, value, &defaultID)
				appendFunc(deltaItem)
			}
		}
	}
}

// processRequestStep processes a request step
func processRequestStep(data *WorkflowData, flowID, nodeID idwrap.IDWrap, nodeName string, stepData map[string]any, templates map[string]*requestTemplate, varMap varsystem.VarMap) error {
	// Get method and URL
	method, url := "GET", ""
	var templateHeaders, templateQueries, stepHeaderOverrides, stepQueryOverrides []map[string]string
	var templateBody, stepBodyOverride map[string]any
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
		}
	}

	// Override with step-specific data
	if m, ok := stepData["method"].(string); ok && m != "" {
		method = m
	}
	if u, ok := stepData["url"].(string); ok && u != "" {
		url = u
	}
	// Only require URL if not using a template or if template didn't provide one
	if url == "" && !usingTemplate {
		return fmt.Errorf("request step '%s' missing required url", nodeName)
	}

	// Parse step overrides
	if h, ok := stepData["headers"].(map[string]any); ok {
		for k, v := range h {
			if vs, ok := v.(string); ok {
				stepHeaderOverrides = append(stepHeaderOverrides, map[string]string{"name": k, "value": vs})
			}
		}
	}
	if q, ok := stepData["query_params"].(map[string]any); ok {
		for k, v := range q {
			if vs, ok := v.(string); ok {
				stepQueryOverrides = append(stepQueryOverrides, map[string]string{"name": k, "value": vs})
			}
		}
	}
	if b, ok := stepData["body"].(map[string]any); ok {
		stepBodyOverride = b
	}

	// Create all request entities
	ctx := createRequestEntities(data, flowID, nodeID, nodeName, url, method)

	// Process headers
	processNameValuePairs(
		data,
		ctx.exampleID, ctx.defaultExampleID, ctx.deltaExampleID,
		templateHeaders, stepHeaderOverrides,
		usingTemplate,
		varMap,
		func(id, exampleID idwrap.IDWrap, name, value string, deltaParentID *idwrap.IDWrap) interface{} {
			return mexampleheader.Header{
				ID:            id,
				ExampleID:     exampleID,
				HeaderKey:     name,
				Value:         value,
				DeltaParentID: deltaParentID,
				Enable:        true,
			}
		},
		func(item interface{}) {
			data.Headers = append(data.Headers, item.(mexampleheader.Header))
		},
	)

	// Process query parameters
	processNameValuePairs(
		data,
		ctx.exampleID, ctx.defaultExampleID, ctx.deltaExampleID,
		templateQueries, stepQueryOverrides,
		usingTemplate,
		varMap,
		func(id, exampleID idwrap.IDWrap, name, value string, deltaParentID *idwrap.IDWrap) interface{} {
			return mexamplequery.Query{
				ID:            id,
				ExampleID:     exampleID,
				QueryKey:      name,
				Value:         value,
				DeltaParentID: deltaParentID,
				Enable:        true,
			}
		},
		func(item interface{}) {
			data.Queries = append(data.Queries, item.(mexamplequery.Query))
		},
	)

	// Process body
	var bodyData []byte
	if usingTemplate && templateBody != nil {
		// Use template body for base
		bodyData, _ = json.Marshal(templateBody)
		addBody(data, ctx, bodyData)

		// If there's an override, update delta only
		if stepBodyOverride != nil {
			overrideData, err := json.Marshal(stepBodyOverride)
			if err != nil {
				return fmt.Errorf("failed to marshal body: %w", err)
			}
			// Update only the delta body
			for i := range data.RawBodies {
				if data.RawBodies[i].ExampleID == ctx.deltaExampleID {
					data.RawBodies[i].Data = overrideData
					break
				}
			}
		}
	} else if stepBodyOverride != nil {
		// No template, use step body for all
		bodyData, _ = json.Marshal(stepBodyOverride)
		addBody(data, ctx, bodyData)
	} else {
		// No body at all
		addBody(data, ctx, nil)
	}

	return nil
}

// addNode adds a flow node
func addNode(data *WorkflowData, flowID, nodeID idwrap.IDWrap, nodeName string, kind mnnode.NodeKind) {
	data.Nodes = append(data.Nodes, mnnode.MNode{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     nodeName,
		NodeKind: kind,
	})
}

// processIfStep processes an if step
func processIfStep(data *WorkflowData, flowID, nodeID idwrap.IDWrap, nodeName string, stepData map[string]any) error {
	addNode(data, flowID, nodeID, nodeName, mnnode.NODE_KIND_CONDITION)

	condition, ok := stepData["condition"].(string)
	if !ok || condition == "" {
		return fmt.Errorf("if step '%s' missing required condition", nodeName)
	}

	data.ConditionNodes = append(data.ConditionNodes, mnif.MNIF{
		FlowNodeID: nodeID,
		Condition: mcondition.Condition{
			Comparisons: mcondition.Comparison{Expression: condition},
		},
	})
	return nil
}

// processForStep processes a for step
func processForStep(data *WorkflowData, flowID, nodeID idwrap.IDWrap, nodeName string, stepData map[string]any) error {
	addNode(data, flowID, nodeID, nodeName, mnnode.NODE_KIND_FOR)

	iterCount := 1 // Default to 1 if not specified
	if val, ok := stepData["iter_count"]; ok {
		if i, ok := val.(int); ok {
			iterCount = i
		} else if f, ok := val.(float64); ok {
			iterCount = int(f)
		}
	}

	data.ForNodes = append(data.ForNodes, mnfor.MNFor{
		FlowNodeID: nodeID,
		IterCount:  int64(iterCount),
	})
	return nil
}

// processForEachStep processes a for_each step
func processForEachStep(data *WorkflowData, flowID, nodeID idwrap.IDWrap, nodeName string, stepData map[string]any) error {
	addNode(data, flowID, nodeID, nodeName, mnnode.NODE_KIND_FOR_EACH)

	items, ok := stepData["items"].(string)
	if !ok || items == "" {
		return fmt.Errorf("for_each step '%s' missing required items", nodeName)
	}

	data.ForEachNodes = append(data.ForEachNodes, mnforeach.MNForEach{
		FlowNodeID:     nodeID,
		IterExpression: items,
	})
	return nil
}

// processJSStep processes a JavaScript step
func processJSStep(data *WorkflowData, flowID, nodeID idwrap.IDWrap, nodeName string, stepData map[string]any) error {
	addNode(data, flowID, nodeID, nodeName, mnnode.NODE_KIND_JS)

	code, ok := stepData["code"].(string)
	if !ok || code == "" {
		return fmt.Errorf("js step '%s' missing required code", nodeName)
	}

	data.JSNodes = append(data.JSNodes, mnjs.MNJS{
		FlowNodeID: nodeID,
		Code:       []byte(code),
	})
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
