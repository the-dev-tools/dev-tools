package yamlflowsimple

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"sort"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/ioworkspace"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"the-dev-tools/server/pkg/model/mnnode/mnforeach"
	"the-dev-tools/server/pkg/model/mnnode/mnif"
	"the-dev-tools/server/pkg/model/mnnode/mnjs"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
)

// ExportYamlFlowYAML converts ioworkspace.WorkspaceData to simplified yamlflow YAML
func ExportYamlFlowYAML(workspaceData *ioworkspace.WorkspaceData) ([]byte, error) {
	if workspaceData == nil {
		return nil, fmt.Errorf("workspace data cannot be nil")
	}

	if len(workspaceData.Flows) == 0 {
		return nil, fmt.Errorf("no flows to export")
	}

	// Build request definitions from all endpoints across all flows
	requests := buildRequestDefinitions(workspaceData)

	// Build flows
	flows := []map[string]any{}
	for _, flow := range workspaceData.Flows {
		flowData := exportFlow(flow, workspaceData, requests)
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

	return node
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
func buildRequestDefinitions(workspaceData *ioworkspace.WorkspaceData) map[string]map[string]any {
	requests := make(map[string]map[string]any)

	// Map node names to their request nodes for direct lookup
	nodeNameToRequestNode := make(map[string]*mnrequest.MNRequest)
	for i := range workspaceData.FlowRequestNodes {
		reqNode := &workspaceData.FlowRequestNodes[i]
		// Find the node name for this request node
		for _, node := range workspaceData.FlowNodes {
			if node.ID == reqNode.FlowNodeID {
				nodeNameToRequestNode[node.Name] = reqNode
				break
			}
		}
	}

	// Create a unique request definition for each request node
	// This ensures each request has its own headers/params/body
	processedNodes := make(map[string]bool)

	for _, node := range workspaceData.FlowNodes {
		if node.NodeKind != mnnode.NODE_KIND_REQUEST {
			continue
		}

		reqNode, exists := nodeNameToRequestNode[node.Name]
		if !exists || reqNode.EndpointID == nil {
			continue
		}

		// Skip if already processed
		if processedNodes[node.Name] {
			continue
		}
		processedNodes[node.Name] = true

		// Find the endpoint
		var endpoint *mitemapi.ItemApi
		for i := range workspaceData.Endpoints {
			if workspaceData.Endpoints[i].ID == *reqNode.EndpointID {
				endpoint = &workspaceData.Endpoints[i]
				break
			}
		}
		if endpoint == nil {
			continue
		}

		// Build request definition for this specific node
		req := map[string]any{
			"name": node.Name,
		}

		// Only add method if not empty
		if endpoint.Method != "" {
			req["method"] = endpoint.Method
		}

		// Only add url if not empty
		if endpoint.Url != "" {
			req["url"] = endpoint.Url
		}

		// Check if there's a delta endpoint with overrides
		if reqNode.DeltaEndpointID != nil {
			for _, deltaEndpoint := range workspaceData.Endpoints {
				if deltaEndpoint.ID == *reqNode.DeltaEndpointID {
					// Use delta endpoint's method/URL if different
					if deltaEndpoint.Method != endpoint.Method && deltaEndpoint.Method != "" {
						req["method"] = deltaEndpoint.Method
					}
					if deltaEndpoint.Url != endpoint.Url && deltaEndpoint.Url != "" {
						req["url"] = deltaEndpoint.Url
					}
					break
				}
			}
		}

		// Collect headers - use base example only (has hardcoded values)
		headerMap := make(map[string]string)
		if reqNode.ExampleID != nil {
			for _, h := range workspaceData.ExampleHeaders {
				if h.ExampleID == *reqNode.ExampleID && h.Enable {
					headerMap[h.HeaderKey] = h.Value
				}
			}
		}

		if len(headerMap) > 0 {
			// Create sorted headers for consistent output
			headerKeys := make([]string, 0, len(headerMap))
			for key := range headerMap {
				headerKeys = append(headerKeys, key)
			}
			sort.Strings(headerKeys)

			// Build ordered header map
			orderedHeaders := make(map[string]string)
			for _, key := range headerKeys {
				orderedHeaders[key] = headerMap[key]
			}
			req["headers"] = orderedHeaders
		}

		// Collect query params - use base example only
		queryMap := make(map[string]string)
		if reqNode.ExampleID != nil {
			for _, q := range workspaceData.ExampleQueries {
				if q.ExampleID == *reqNode.ExampleID {
					queryMap[q.QueryKey] = q.Value
				}
			}
		}

		if len(queryMap) > 0 {
			// Create sorted query params for consistent output
			queryKeys := make([]string, 0, len(queryMap))
			for key := range queryMap {
				queryKeys = append(queryKeys, key)
			}
			sort.Strings(queryKeys)

			// Build ordered query map
			orderedQueries := make(map[string]string)
			for _, key := range queryKeys {
				orderedQueries[key] = queryMap[key]
			}
			req["query_params"] = orderedQueries
		}

		// Collect body - use base example only
		if reqNode.ExampleID != nil {
			for _, b := range workspaceData.Rawbodies {
				if b.ExampleID == *reqNode.ExampleID && len(b.Data) > 0 {
					var bodyData any
					if err := json.Unmarshal(b.Data, &bodyData); err == nil {
						req["body"] = bodyData
						break
					}
				}
			}
		}

		// Store with node name as key
		requests[node.Name] = req
	}

	return requests
}

// exportFlow exports a single flow
func exportFlow(flow mflow.Flow, workspaceData *ioworkspace.WorkspaceData, requests map[string]map[string]any) map[string]any {
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
	steps := processFlowNodes(nodeMap, incomingEdges, outgoingEdges, startNodeID, workspaceData, nodeToRequest)

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

	return flowData
}

// processFlowNodes processes all nodes in a flow and returns steps
func processFlowNodes(nodeMap map[idwrap.IDWrap]mnnode.MNode, incomingEdges map[idwrap.IDWrap][]edge.Edge,
	outgoingEdges map[idwrap.IDWrap][]edge.Edge, startNodeID idwrap.IDWrap,
	workspaceData *ioworkspace.WorkspaceData, nodeToRequest map[string]string) []map[string]any {

	processed := make(map[idwrap.IDWrap]bool)
	steps := make([]map[string]any, 0)

	var processNode func(nodeID idwrap.IDWrap)
	processNode = func(nodeID idwrap.IDWrap) {
		if processed[nodeID] || nodeID == startNodeID {
			return
		}
		processed[nodeID] = true

		node, exists := nodeMap[nodeID]
		if !exists {
			return
		}

		// Process dependencies first
		for _, e := range incomingEdges[nodeID] {
			if e.SourceID != startNodeID {
				processNode(e.SourceID)
			}
		}

		// Convert node to step
		var step map[string]any
		switch node.NodeKind {
		case mnnode.NODE_KIND_REQUEST:
			step = convertRequestNodeClean(node, incomingEdges, outgoingEdges, startNodeID,
				nodeMap, workspaceData, nodeToRequest)
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

		if step != nil {
			steps = append(steps, step)
		}
	}

	// Process all nodes
	for nodeID := range nodeMap {
		processNode(nodeID)
	}

	return steps
}

// convertRequestNodeClean converts a request node to clean format
func convertRequestNodeClean(node mnnode.MNode, incomingEdges map[idwrap.IDWrap][]edge.Edge,
	outgoingEdges map[idwrap.IDWrap][]edge.Edge, startNodeID idwrap.IDWrap,
	nodeMap map[idwrap.IDWrap]mnnode.MNode, workspaceData *ioworkspace.WorkspaceData,
	nodeToRequest map[string]string) map[string]any {

	// Find request node data
	var requestNode *mnrequest.MNRequest
	for i := range workspaceData.FlowRequestNodes {
		if workspaceData.FlowRequestNodes[i].FlowNodeID == node.ID {
			requestNode = &workspaceData.FlowRequestNodes[i]
			break
		}
	}
	if requestNode == nil {
		return nil
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
			// Add method override if different and not empty
			if deltaEndpoint.Method != baseEndpoint.Method && deltaEndpoint.Method != "" {
				step["method"] = deltaEndpoint.Method
			}
			// Add URL override if different and not empty
			if deltaEndpoint.Url != baseEndpoint.Url && deltaEndpoint.Url != "" {
				step["url"] = deltaEndpoint.Url
			}
		}
	}

	if requestNode.DeltaExampleID != nil {
		// Check for header overrides
		headerOverrides := make(map[string]string)
		for _, h := range workspaceData.ExampleHeaders {
			if h.ExampleID == *requestNode.DeltaExampleID && h.Enable {
				// Only include if different from base
				var baseValue string
				if requestNode.ExampleID != nil {
					for _, baseH := range workspaceData.ExampleHeaders {
						if baseH.ExampleID == *requestNode.ExampleID && baseH.HeaderKey == h.HeaderKey && baseH.Enable {
							baseValue = baseH.Value
							break
						}
					}
				}
				if h.Value != baseValue {
					headerOverrides[h.HeaderKey] = h.Value
				}
			}
		}
		if len(headerOverrides) > 0 {
			step["headers"] = headerOverrides
		}

		// Check for query param overrides
		queryOverrides := make(map[string]string)
		for _, q := range workspaceData.ExampleQueries {
			if q.ExampleID == *requestNode.DeltaExampleID {
				// Only include if different from base
				var baseValue string
				if requestNode.ExampleID != nil {
					for _, baseQ := range workspaceData.ExampleQueries {
						if baseQ.ExampleID == *requestNode.ExampleID && baseQ.QueryKey == q.QueryKey {
							baseValue = baseQ.Value
							break
						}
					}
				}
				if q.Value != baseValue {
					queryOverrides[q.QueryKey] = q.Value
				}
			}
		}
		if len(queryOverrides) > 0 {
			step["query_params"] = queryOverrides
		}

		// Check for body overrides
		for _, b := range workspaceData.Rawbodies {
			if b.ExampleID == *requestNode.DeltaExampleID && len(b.Data) > 0 {
				var deltaBodyData any
				if err := json.Unmarshal(b.Data, &deltaBodyData); err == nil {
					// Check if different from base
					var baseBodyData any
					if requestNode.ExampleID != nil {
						for _, baseB := range workspaceData.Rawbodies {
							if baseB.ExampleID == *requestNode.ExampleID && len(baseB.Data) > 0 {
								if err := json.Unmarshal(baseB.Data, &baseBodyData); err != nil {
									// If base body can't be unmarshaled, treat as different
									baseBodyData = nil
								}
								break
							}
						}
					}
					// Simple comparison - if they're different, include the override
					if fmt.Sprintf("%v", deltaBodyData) != fmt.Sprintf("%v", baseBodyData) {
						step["body"] = deltaBodyData
					}
				}
				break
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

	return map[string]any{"request": step}
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
