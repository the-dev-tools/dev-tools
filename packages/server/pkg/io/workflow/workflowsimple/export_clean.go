package workflowsimple

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
	"the-dev-tools/server/pkg/model/mnnode/mnif"
	"the-dev-tools/server/pkg/model/mnnode/mnjs"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
)

// ExportWorkflowClean exports workspace data to the clean simplified format
func ExportWorkflowClean(workspaceData *ioworkspace.WorkspaceData) ([]byte, error) {
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

	// Build final YAML structure
	yamlData := map[string]any{
		"workspace_name": workspaceData.Workspace.Name,
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
		yamlData["requests"] = requestList
	}

	yamlData["flows"] = flows

	return yaml.Marshal(yamlData)
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
			"name":   node.Name,
			"method": endpoint.Method,
			"url":    endpoint.Url,
		}
		
		// Check if there's a delta endpoint with overrides
		if reqNode.DeltaEndpointID != nil {
			for _, deltaEndpoint := range workspaceData.Endpoints {
				if deltaEndpoint.ID == *reqNode.DeltaEndpointID {
					// Use delta endpoint's method/URL if different
					if deltaEndpoint.Method != endpoint.Method {
						req["method"] = deltaEndpoint.Method
					}
					if deltaEndpoint.Url != endpoint.Url {
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
			// Add method override if different
			if deltaEndpoint.Method != baseEndpoint.Method {
				step["method"] = deltaEndpoint.Method
			}
			// Add URL override if different
			if deltaEndpoint.Url != baseEndpoint.Url {
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
			if sourceNode, exists := nodeMap[e.SourceID]; exists {
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
			if sourceNode, exists := nodeMap[e.SourceID]; exists {
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
			if sourceNode, exists := nodeMap[e.SourceID]; exists {
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
		"name":       node.Name,
		"iter_count": forNode.IterCount,
	}

	// Find loop target
	for _, e := range outgoingEdges[node.ID] {
		if e.SourceHandler == edge.HandleLoop {
			if targetNode, exists := nodeMap[e.TargetID]; exists {
				step["loop"] = targetNode.Name
			}
		}
	}

	// Add dependencies
	var dependencies []string
	for _, e := range incomingEdges[node.ID] {
		if e.SourceID != startNodeID {
			if sourceNode, exists := nodeMap[e.SourceID]; exists {
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

	step := map[string]any{
		"name":  node.Name,
		"items": "response.items", // Default since we don't store it
	}

	// Find loop target
	for _, e := range outgoingEdges[node.ID] {
		if e.SourceHandler == edge.HandleLoop {
			if targetNode, exists := nodeMap[e.TargetID]; exists {
				step["loop"] = targetNode.Name
			}
		}
	}

	// Add dependencies
	var dependencies []string
	for _, e := range incomingEdges[node.ID] {
		if e.SourceID != startNodeID {
			if sourceNode, exists := nodeMap[e.SourceID]; exists {
				dependencies = append(dependencies, sourceNode.Name)
			}
		}
	}
	if len(dependencies) > 0 {
		step["depends_on"] = dependencies
	}

	return map[string]any{"for_each": step}
}