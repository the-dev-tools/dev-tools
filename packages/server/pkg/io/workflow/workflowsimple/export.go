package workflowsimple

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"sort"
	"strings"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/ioworkspace"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"the-dev-tools/server/pkg/model/mnnode/mnif"
	"the-dev-tools/server/pkg/model/mnnode/mnjs"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
)

// ExportWorkflowYAML converts ioworkspace.WorkspaceData to simplified workflow YAML
func ExportWorkflowYAML(workspaceData *ioworkspace.WorkspaceData) ([]byte, error) {
	// Use the clean export format
	return ExportWorkflowClean(workspaceData)
}

// ExportWorkflowYAMLOld is the original export function (kept for reference)
func ExportWorkflowYAMLOld(workspaceData *ioworkspace.WorkspaceData) ([]byte, error) {
	if workspaceData == nil {
		return nil, fmt.Errorf("workspace data cannot be nil")
	}

	if len(workspaceData.Flows) == 0 {
		return nil, fmt.Errorf("no flows to export")
	}
	

	// For simplicity, export only the first flow
	flow := workspaceData.Flows[0]

	// Build node map
	nodeMap := make(map[idwrap.IDWrap]mnnode.MNode)
	for _, node := range workspaceData.FlowNodes {
		if node.FlowID == flow.ID {
			nodeMap[node.ID] = node
		}
	}

	// Build edge map (target -> sources)
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
			startNodeID = noop.FlowNodeID
			break
		}
	}

	// Build variables list
	var variables []map[string]string
	for _, v := range workspaceData.FlowVariables {
		if v.FlowID == flow.ID {
			variables = append(variables, map[string]string{
				"name":  v.Name,
				"value": v.Value,
			})
		}
	}

	// Extract request templates by analyzing common patterns
	requestTemplates := extractRequestTemplates(workspaceData, nodeMap)

	// Process nodes in dependency order
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
		step := convertNodeToStep(node, nodeMap, incomingEdges, outgoingEdges, workspaceData, startNodeID)
		if step != nil {
			steps = append(steps, step)
		}
	}

	// Process all nodes
	for nodeID := range nodeMap {
		processNode(nodeID)
	}

	// Sort steps by their original order (if possible)
	sort.Slice(steps, func(i, j int) bool {
		// Try to maintain a sensible order
		nameI := getStepName(steps[i])
		nameJ := getStepName(steps[j])
		return nameI < nameJ
	})

	// Build flow data
	flowData := map[string]any{
		"name":  flow.Name,
		"steps": steps,
	}
	
	// Only add variables if not empty
	if len(variables) > 0 {
		flowData["variables"] = variables
	}
	
	// Convert to YAML
	yamlData := map[string]any{
		"workspace_name": workspaceData.Workspace.Name,
		"flows":          []map[string]any{flowData},
	}

	// Add request templates if any were found
	if len(requestTemplates) > 0 {
		yamlData["request_templates"] = requestTemplates
	}

	return yaml.Marshal(yamlData)
}

// convertNodeToStep converts a node to a simplified step format
func convertNodeToStep(node mnnode.MNode, nodeMap map[idwrap.IDWrap]mnnode.MNode, 
	incomingEdges map[idwrap.IDWrap][]edge.Edge, outgoingEdges map[idwrap.IDWrap][]edge.Edge,
	workspaceData *ioworkspace.WorkspaceData, startNodeID idwrap.IDWrap) map[string]any {

	// Build dependencies list
	var dependencies []string
	hasSequentialDependency := false
	
	for _, e := range incomingEdges[node.ID] {
		if e.SourceID != startNodeID {
			if sourceNode, exists := nodeMap[e.SourceID]; exists {
				// Check if this is a sequential dependency (would be implicit in simplified format)
				if e.SourceHandler == edge.HandleUnspecified {
					hasSequentialDependency = true
				}
				dependencies = append(dependencies, sourceNode.Name)
			}
		}
	}

	// Remove sequential dependencies if there are explicit ones
	if len(dependencies) > 1 && hasSequentialDependency {
		// Filter out sequential dependencies
		explicitDeps := make([]string, 0)
		for i, dep := range dependencies {
			if i == 0 || !hasSequentialDependency {
				explicitDeps = append(explicitDeps, dep)
			}
		}
		dependencies = explicitDeps
	}

	switch node.NodeKind {
	case mnnode.NODE_KIND_REQUEST:
		return convertRequestNode(node, dependencies, workspaceData)
	case mnnode.NODE_KIND_CONDITION:
		return convertConditionNode(node, dependencies, outgoingEdges, nodeMap, workspaceData)
	case mnnode.NODE_KIND_FOR:
		return convertForNode(node, dependencies, outgoingEdges, nodeMap, workspaceData)
	case mnnode.NODE_KIND_FOR_EACH:
		return convertForEachNode(node, dependencies, outgoingEdges, nodeMap, workspaceData)
	case mnnode.NODE_KIND_JS:
		return convertJSNode(node, dependencies, workspaceData)
	case mnnode.NODE_KIND_NO_OP:
		// Skip no-op nodes
		return nil
	default:
		return nil
	}
}

// Helper functions for converting specific node types
func convertRequestNode(node mnnode.MNode, dependencies []string, workspaceData *ioworkspace.WorkspaceData) map[string]any {
	// Find the request node data
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

	// Find the endpoint
	var endpoint *mitemapi.ItemApi
	if requestNode.DeltaEndpointID != nil {
		for i := range workspaceData.Endpoints {
			if workspaceData.Endpoints[i].ID == *requestNode.DeltaEndpointID {
				endpoint = &workspaceData.Endpoints[i]
				break
			}
		}
	}
	if endpoint == nil && requestNode.EndpointID != nil {
		for i := range workspaceData.Endpoints {
			if workspaceData.Endpoints[i].ID == *requestNode.EndpointID {
				endpoint = &workspaceData.Endpoints[i]
				break
			}
		}
	}
	if endpoint == nil {
		return nil
	}

	step := map[string]any{
		"name":   node.Name,
		"url":    endpoint.Url,
		"method": endpoint.Method,
	}

	// Add headers (prefer delta example to preserve variables)
	var headers []map[string]string
	headerMap := make(map[string]string) // Use map to avoid duplicates
	
	// Try delta example first, then fall back to base example
	var searchExampleIDs []idwrap.IDWrap
	if requestNode.DeltaExampleID != nil {
		searchExampleIDs = append(searchExampleIDs, *requestNode.DeltaExampleID)
	}
	if requestNode.ExampleID != nil {
		searchExampleIDs = append(searchExampleIDs, *requestNode.ExampleID)
	}
	
	
	// Search through both delta and base example IDs
	for _, searchExampleID := range searchExampleIDs {
		for _, h := range workspaceData.ExampleHeaders {
			if h.ExampleID == searchExampleID {
				// Only add if not already present (delta takes precedence)
				if _, exists := headerMap[h.HeaderKey]; !exists {
					headerMap[h.HeaderKey] = h.Value
				}
			}
		}
	}
	
	// Convert map to list
	for name, value := range headerMap {
		headers = append(headers, map[string]string{
			"name":  name,
			"value": value,
		})
	}
	
	// Sort headers for consistent output
	sort.Slice(headers, func(i, j int) bool {
		return headers[i]["name"] < headers[j]["name"]
	})
	
	if len(headers) > 0 {
		step["headers"] = headers
	}

	// Add query params (prefer delta example to preserve variables)
	var queryParams []map[string]string
	queryMap := make(map[string]string) // Use map to avoid duplicates
	
	
	// Search through both delta and base example IDs
	for _, searchExampleID := range searchExampleIDs {
		for _, q := range workspaceData.ExampleQueries {
			if q.ExampleID == searchExampleID {
				// Only add if not already present (delta takes precedence)
				if _, exists := queryMap[q.QueryKey]; !exists {
					queryMap[q.QueryKey] = q.Value
				}
			}
		}
	}
	
	// Convert map to list
	for name, value := range queryMap {
		queryParams = append(queryParams, map[string]string{
			"name":  name,
			"value": value,
		})
	}
	
	// Sort query params for consistent output
	sort.Slice(queryParams, func(i, j int) bool {
		return queryParams[i]["name"] < queryParams[j]["name"]
	})
	
	if len(queryParams) > 0 {
		step["query_params"] = queryParams
	}

	// Add body if exists (prefer delta example to preserve variables)
	
	// Search through both delta and base example IDs
	for _, searchExampleID := range searchExampleIDs {
		for _, b := range workspaceData.Rawbodies {
			if b.ExampleID == searchExampleID && len(b.Data) > 0 {
				// Try to unmarshal as JSON
				var bodyData any
				if err := json.Unmarshal(b.Data, &bodyData); err == nil {
					step["body"] = map[string]any{
						"body_json": bodyData,
					}
					// Found body, break out of both loops
					goto bodyFound
				}
			}
		}
	}
	bodyFound:

	if len(dependencies) > 0 {
		step["depends_on"] = dependencies
	}
	

	return map[string]any{"request": step}
}

func convertConditionNode(node mnnode.MNode, dependencies []string, outgoingEdges map[idwrap.IDWrap][]edge.Edge, 
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
		"name":      node.Name,
		"condition": condNode.Condition.Comparisons.Expression,
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

	if len(dependencies) > 0 {
		step["depends_on"] = dependencies
	}

	return map[string]any{"if": step}
}

func convertForNode(node mnnode.MNode, dependencies []string, outgoingEdges map[idwrap.IDWrap][]edge.Edge, 
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

	if len(dependencies) > 0 {
		step["depends_on"] = dependencies
	}

	return map[string]any{"for": step}
}

func convertForEachNode(node mnnode.MNode, dependencies []string, outgoingEdges map[idwrap.IDWrap][]edge.Edge, 
	nodeMap map[idwrap.IDWrap]mnnode.MNode, workspaceData *ioworkspace.WorkspaceData) map[string]any {
	
	step := map[string]any{
		"name":  node.Name,
		"items": "response.items", // Default value since we don't store it
	}

	// Find loop target
	for _, e := range outgoingEdges[node.ID] {
		if e.SourceHandler == edge.HandleLoop {
			if targetNode, exists := nodeMap[e.TargetID]; exists {
				step["loop"] = targetNode.Name
			}
		}
	}

	if len(dependencies) > 0 {
		step["depends_on"] = dependencies
	}

	return map[string]any{"for_each": step}
}

func convertJSNode(node mnnode.MNode, dependencies []string, workspaceData *ioworkspace.WorkspaceData) map[string]any {
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

	if len(dependencies) > 0 {
		step["depends_on"] = dependencies
	}

	return map[string]any{"js": step}
}

// getStepName extracts the name from a step map
func getStepName(step map[string]any) string {
	for _, v := range step {
		if m, ok := v.(map[string]any); ok {
			if name, ok := m["name"].(string); ok {
				return name
			}
		}
	}
	return ""
}

// extractRequestTemplates analyzes request nodes to find common patterns
func extractRequestTemplates(workspaceData *ioworkspace.WorkspaceData, nodeMap map[idwrap.IDWrap]mnnode.MNode) map[string]map[string]any {
	templates := make(map[string]map[string]any)
	
	// Group requests by common headers
	type requestPattern struct {
		headers []string
		count   int
		nodes   []mnrequest.MNRequest
	}
	
	patterns := make(map[string]*requestPattern)
	
	// Analyze all request nodes
	for _, reqNode := range workspaceData.FlowRequestNodes {
		if node, exists := nodeMap[reqNode.FlowNodeID]; exists && node.NodeKind == mnnode.NODE_KIND_REQUEST {
			// Get headers for this request
			var headers []string
			exampleID := reqNode.ExampleID
			if reqNode.DeltaExampleID != nil {
				exampleID = reqNode.DeltaExampleID // Prefer delta to preserve variables
			}
			
			if exampleID != nil {
				for _, h := range workspaceData.ExampleHeaders {
					if h.ExampleID == *exampleID {
						headers = append(headers, fmt.Sprintf("%s:%s", h.HeaderKey, h.Value))
					}
				}
			}
			
			// Sort headers for consistent comparison
			sort.Strings(headers)
			headerKey := strings.Join(headers, "|")
			
			if pattern, exists := patterns[headerKey]; exists {
				pattern.count++
				pattern.nodes = append(pattern.nodes, reqNode)
			} else {
				patterns[headerKey] = &requestPattern{
					headers: headers,
					count:   1,
					nodes:   []mnrequest.MNRequest{reqNode},
				}
			}
		}
	}
	
	// Create templates for patterns that appear multiple times
	templateIndex := 1
	for _, pattern := range patterns {
		if pattern.count >= 2 && len(pattern.headers) > 0 {
			templateName := fmt.Sprintf("common_headers_%d", templateIndex)
			templateIndex++
			
			// Build template
			template := make(map[string]any)
			
			// Add headers
			var headerList []map[string]string
			for _, h := range pattern.headers {
				parts := strings.SplitN(h, ":", 2)
				if len(parts) == 2 {
					headerList = append(headerList, map[string]string{
						"name":  parts[0],
						"value": parts[1],
					})
				}
			}
			if len(headerList) > 0 {
				template["headers"] = headerList
			}
			
			templates[templateName] = template
		}
	}
	
	return templates
}