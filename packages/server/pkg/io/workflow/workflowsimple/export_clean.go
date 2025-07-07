package workflowsimple

import (
	"bytes"
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

	// Track which endpoints are used in flows
	usedEndpoints := make(map[idwrap.IDWrap]bool)
	endpointToNodes := make(map[idwrap.IDWrap][]string)      // endpoint -> node names that use it
	nodeNameToEndpoint := make(map[string]idwrap.IDWrap)     // node name -> endpoint ID
	
	// First pass: collect endpoint usage and node associations
	for _, reqNode := range workspaceData.FlowRequestNodes {
		if reqNode.EndpointID != nil {
			usedEndpoints[*reqNode.EndpointID] = true
			
			// Find the node name for this request node
			for _, node := range workspaceData.FlowNodes {
				if node.ID == reqNode.FlowNodeID {
					endpointToNodes[*reqNode.EndpointID] = append(endpointToNodes[*reqNode.EndpointID], node.Name)
					nodeNameToEndpoint[node.Name] = *reqNode.EndpointID
					break
				}
			}
			
			if reqNode.DeltaEndpointID != nil {
				usedEndpoints[*reqNode.DeltaEndpointID] = true
			}
		}
	}

	// Collect endpoints to process in a deterministic order
	endpointList := make([]mitemapi.ItemApi, 0)
	for _, endpoint := range workspaceData.Endpoints {
		// Skip if not used in any flow
		if !usedEndpoints[endpoint.ID] {
			continue
		}

		// Skip hidden endpoints (they're usually deltas)
		if endpoint.Hidden {
			continue
		}
		
		endpointList = append(endpointList, endpoint)
	}
	
	// Sort endpoints by the first node name that uses them (for consistent ordering)
	sort.Slice(endpointList, func(i, j int) bool {
		nodesI := endpointToNodes[endpointList[i].ID]
		nodesJ := endpointToNodes[endpointList[j].ID]
		if len(nodesI) > 0 && len(nodesJ) > 0 {
			return nodesI[0] < nodesJ[0]
		}
		return endpointList[i].Name < endpointList[j].Name
	})

	// Track used request names to avoid conflicts
	usedRequestNames := make(map[string]bool)
	
	// Process endpoints in sorted order
	for _, endpoint := range endpointList {
		// Determine request name
		var requestName string
		
		// Check if any node using this endpoint has a suitable name
		nodeNames := endpointToNodes[endpoint.ID]
		if len(nodeNames) > 0 {
			// Use the first node name if it looks like a request name pattern
			firstNodeName := nodeNames[0]
			if isRequestNamePattern(firstNodeName) && !usedRequestNames[firstNodeName] {
				requestName = firstNodeName
			}
		}
		
		// If no suitable node name, generate a unique name
		if requestName == "" {
			for i := 1; ; i++ {
				candidateName := fmt.Sprintf("request_%d", i)
				if !usedRequestNames[candidateName] {
					requestName = candidateName
					break
				}
			}
		}
		
		usedRequestNames[requestName] = true

		// Build request definition
		req := map[string]any{
			"name":   requestName,
			"method": endpoint.Method,
			"url":    endpoint.Url,
		}

		// Find the base example for this endpoint
		var primaryExampleID *idwrap.IDWrap
		for _, example := range workspaceData.Examples {
			if example.ItemApiID == endpoint.ID {
				primaryExampleID = &example.ID
				break
			}
		}
		
		if primaryExampleID == nil {
			// No examples for this endpoint
			continue
		}

		// Collect headers from base example only
		headerMap := make(map[string]string)
		for _, h := range workspaceData.ExampleHeaders {
			if h.ExampleID == *primaryExampleID {
				headerMap[h.HeaderKey] = h.Value
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

		// Collect query params from base example only
		queryMap := make(map[string]string)
		for _, q := range workspaceData.ExampleQueries {
			if q.ExampleID == *primaryExampleID {
				queryMap[q.QueryKey] = q.Value
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

		// Collect body from base example only
		for _, b := range workspaceData.Rawbodies {
			if b.ExampleID == *primaryExampleID && len(b.Data) > 0 {
				var bodyData any
				if err := json.Unmarshal(b.Data, &bodyData); err == nil {
					req["body"] = bodyData
					break
				}
			}
		}

		// Store with endpoint ID for later reference
		requests[endpoint.ID.String()] = req
	}

	return requests
}

// isRequestNamePattern checks if a name follows common request naming patterns
func isRequestNamePattern(name string) bool {
	// Check for patterns like: request_0, request_1, api_call_1, etc.
	// But avoid overly generic names
	return len(name) > 0 && name != "request" && name != "api" && name != "call"
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

	// Build endpoint ID to request name mapping
	endpointToRequest := make(map[string]string)
	for endpointID, req := range requests {
		if name, ok := req["name"].(string); ok {
			endpointToRequest[endpointID] = name
		}
	}

	// Process nodes to create steps
	steps := processFlowNodes(nodeMap, incomingEdges, outgoingEdges, startNodeID, workspaceData, endpointToRequest)

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
	workspaceData *ioworkspace.WorkspaceData, endpointToRequest map[string]string) []map[string]any {

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
				nodeMap, workspaceData, endpointToRequest)
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
	endpointToRequest map[string]string) map[string]any {

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

	// Find the request reference
	if requestNode.EndpointID != nil {
		if requestName, ok := endpointToRequest[requestNode.EndpointID.String()]; ok {
			step["use_request"] = requestName
		}
	}

	// Check if this node has delta overrides
	if requestNode.DeltaEndpointID != nil || requestNode.DeltaExampleID != nil {
		// Check for delta endpoint (URL/method overrides)
		if requestNode.DeltaEndpointID != nil {
			// Find the delta endpoint
			for _, endpoint := range workspaceData.Endpoints {
				if endpoint.ID == *requestNode.DeltaEndpointID {
					// Find the base endpoint to compare
					var baseEndpoint *mitemapi.ItemApi
					if requestNode.EndpointID != nil {
						for _, ep := range workspaceData.Endpoints {
							if ep.ID == *requestNode.EndpointID {
								baseEndpoint = &ep
								break
							}
						}
					}
					
					// Add URL override if different from base
					if baseEndpoint != nil && endpoint.Url != baseEndpoint.Url {
						step["url"] = endpoint.Url
					}
					
					// Add method override if different from base
					if baseEndpoint != nil && endpoint.Method != baseEndpoint.Method {
						step["method"] = endpoint.Method
					}
					break
				}
			}
		}
		
		// Check for delta example (headers/query/body overrides)
		if requestNode.DeltaExampleID != nil {
			// Collect delta headers
			deltaHeaders := make(map[string]string)
			for _, h := range workspaceData.ExampleHeaders {
				if h.ExampleID == *requestNode.DeltaExampleID {
					deltaHeaders[h.HeaderKey] = h.Value
				}
			}
			
			// Compare with base headers (if we have base example)
			baseHeaders := make(map[string]string)
			if requestNode.ExampleID != nil {
				for _, h := range workspaceData.ExampleHeaders {
					if h.ExampleID == *requestNode.ExampleID {
						baseHeaders[h.HeaderKey] = h.Value
					}
				}
			}
			
			// Find headers that are different or new in delta
			overrideHeaders := make(map[string]string)
			for key, value := range deltaHeaders {
				if baseValue, exists := baseHeaders[key]; !exists || baseValue != value {
					overrideHeaders[key] = value
				}
			}
			
			if len(overrideHeaders) > 0 {
				// Sort keys for consistent output
				keys := make([]string, 0, len(overrideHeaders))
				for k := range overrideHeaders {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				
				sortedHeaders := make(map[string]string)
				for _, k := range keys {
					sortedHeaders[k] = overrideHeaders[k]
				}
				step["headers"] = sortedHeaders
			}
			
			// Collect delta query params
			deltaQueries := make(map[string]string)
			for _, q := range workspaceData.ExampleQueries {
				if q.ExampleID == *requestNode.DeltaExampleID {
					deltaQueries[q.QueryKey] = q.Value
				}
			}
			
			// Compare with base queries
			baseQueries := make(map[string]string)
			if requestNode.ExampleID != nil {
				for _, q := range workspaceData.ExampleQueries {
					if q.ExampleID == *requestNode.ExampleID {
						baseQueries[q.QueryKey] = q.Value
					}
				}
			}
			
			// Find queries that are different or new in delta
			overrideQueries := make(map[string]string)
			for key, value := range deltaQueries {
				if baseValue, exists := baseQueries[key]; !exists || baseValue != value {
					overrideQueries[key] = value
				}
			}
			
			if len(overrideQueries) > 0 {
				// Sort keys for consistent output
				keys := make([]string, 0, len(overrideQueries))
				for k := range overrideQueries {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				
				sortedQueries := make(map[string]string)
				for _, k := range keys {
					sortedQueries[k] = overrideQueries[k]
				}
				step["query_params"] = sortedQueries
			}
			
			// Check for body overrides
			var deltaBody []byte
			for _, b := range workspaceData.Rawbodies {
				if b.ExampleID == *requestNode.DeltaExampleID && len(b.Data) > 0 {
					deltaBody = b.Data
					break
				}
			}
			
			if len(deltaBody) > 0 {
				// Compare with base body if available
				var baseBody []byte
				if requestNode.ExampleID != nil {
					for _, b := range workspaceData.Rawbodies {
						if b.ExampleID == *requestNode.ExampleID && len(b.Data) > 0 {
							baseBody = b.Data
							break
						}
					}
				}
				
				// Only add body override if different from base
				if !bytes.Equal(deltaBody, baseBody) {
					var bodyData any
					if err := json.Unmarshal(deltaBody, &bodyData); err == nil {
						step["body"] = bodyData
					}
				}
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