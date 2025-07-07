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
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
)

// RequestInfo holds all information about a request for analysis
type RequestInfo struct {
	Node        mnnode.MNode
	RequestNode *mnrequest.MNRequest
	Endpoint    *mitemapi.ItemApi
	Headers     map[string]string
	QueryParams map[string]string
	Body        map[string]any
	Dependencies []string
}

// ExportWorkflowYAMLV2 converts ioworkspace.WorkspaceData to simplified workflow YAML with better template extraction
func ExportWorkflowYAMLV2(workspaceData *ioworkspace.WorkspaceData) ([]byte, error) {
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

	// Collect all request information
	requestInfos := collectRequestInfo(workspaceData, nodeMap, incomingEdges, startNodeID)

	// Extract common patterns and create templates
	templates, templateAssignments := extractSmartTemplates(requestInfos)

	// Process nodes in dependency order to create steps
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
		if node.NodeKind == mnnode.NODE_KIND_REQUEST {
			// Use template-aware conversion for requests
			if reqInfo := findRequestInfo(requestInfos, node.ID); reqInfo != nil {
				templateName := templateAssignments[node.ID]
				step = convertRequestWithTemplate(*reqInfo, templateName, templates[templateName])
			}
		} else {
			// Use original conversion for non-request nodes
			step = convertNodeToStep(node, nodeMap, incomingEdges, outgoingEdges, workspaceData, startNodeID)
		}
		
		if step != nil {
			steps = append(steps, step)
		}
	}

	// Process all nodes
	for nodeID := range nodeMap {
		processNode(nodeID)
	}

	// Sort steps by name for consistent output
	sort.Slice(steps, func(i, j int) bool {
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

	// Add request templates if any were created
	if len(templates) > 0 {
		yamlData["request_templates"] = templates
	}

	return yaml.Marshal(yamlData)
}

// collectRequestInfo gathers all information about request nodes
func collectRequestInfo(workspaceData *ioworkspace.WorkspaceData, nodeMap map[idwrap.IDWrap]mnnode.MNode, 
	incomingEdges map[idwrap.IDWrap][]edge.Edge, startNodeID idwrap.IDWrap) []RequestInfo {
	
	var infos []RequestInfo

	for _, reqNode := range workspaceData.FlowRequestNodes {
		node, exists := nodeMap[reqNode.FlowNodeID]
		if !exists || node.NodeKind != mnnode.NODE_KIND_REQUEST {
			continue
		}

		info := RequestInfo{
			Node:        node,
			RequestNode: &reqNode,
			Headers:     make(map[string]string),
			QueryParams: make(map[string]string),
		}

		// Find endpoint
		if reqNode.DeltaEndpointID != nil {
			for i := range workspaceData.Endpoints {
				if workspaceData.Endpoints[i].ID == *reqNode.DeltaEndpointID {
					info.Endpoint = &workspaceData.Endpoints[i]
					break
				}
			}
		}
		if info.Endpoint == nil && reqNode.EndpointID != nil {
			for i := range workspaceData.Endpoints {
				if workspaceData.Endpoints[i].ID == *reqNode.EndpointID {
					info.Endpoint = &workspaceData.Endpoints[i]
					break
				}
			}
		}

		if info.Endpoint == nil {
			continue
		}

		// Collect headers
		exampleIDs := []idwrap.IDWrap{}
		if reqNode.ExampleID != nil {
			exampleIDs = append(exampleIDs, *reqNode.ExampleID)
		}
		if reqNode.DeltaExampleID != nil {
			exampleIDs = append(exampleIDs, *reqNode.DeltaExampleID)
		}

		for _, exID := range exampleIDs {
			for _, h := range workspaceData.ExampleHeaders {
				if h.ExampleID == exID {
					info.Headers[h.HeaderKey] = h.Value
				}
			}
		}

		// Collect query params
		for _, exID := range exampleIDs {
			for _, q := range workspaceData.ExampleQueries {
				if q.ExampleID == exID {
					info.QueryParams[q.QueryKey] = q.Value
				}
			}
		}

		// Collect body
		for _, exID := range exampleIDs {
			for _, b := range workspaceData.Rawbodies {
				if b.ExampleID == exID && len(b.Data) > 0 {
					var bodyData any
					if err := json.Unmarshal(b.Data, &bodyData); err == nil {
						info.Body = map[string]any{"body_json": bodyData}
					}
					break
				}
			}
			if info.Body != nil {
				break
			}
		}

		// Collect dependencies
		for _, e := range incomingEdges[node.ID] {
			if e.SourceID != startNodeID {
				if sourceNode, exists := nodeMap[e.SourceID]; exists {
					info.Dependencies = append(info.Dependencies, sourceNode.Name)
				}
			}
		}

		infos = append(infos, info)
	}

	return infos
}

// extractSmartTemplates analyzes requests and creates templates for common patterns
func extractSmartTemplates(requests []RequestInfo) (templates map[string]map[string]any, assignments map[idwrap.IDWrap]string) {
	templates = make(map[string]map[string]any)
	assignments = make(map[idwrap.IDWrap]string)

	// Group by base URL pattern
	urlGroups := make(map[string][]RequestInfo)
	for _, req := range requests {
		if req.Endpoint == nil {
			continue
		}
		
		// Extract base URL (up to the third slash)
		parts := strings.Split(req.Endpoint.Url, "/")
		if len(parts) >= 3 {
			baseURL := strings.Join(parts[:3], "/")
			urlGroups[baseURL] = append(urlGroups[baseURL], req)
		}
	}

	templateIndex := 1
	
	// Create templates for each URL group with common headers
	for baseURL, groupReqs := range urlGroups {
		if len(groupReqs) < 2 {
			continue // Skip single requests
		}

		// Find common headers
		commonHeaders := findCommonHeaders(groupReqs)
		if len(commonHeaders) == 0 {
			continue // No common headers
		}

		// Create template name based on URL
		templateName := fmt.Sprintf("api_%d", templateIndex)
		if strings.Contains(baseURL, "auth") {
			templateName = fmt.Sprintf("auth_api_%d", templateIndex)
		} else if strings.Contains(baseURL, "api") {
			templateName = fmt.Sprintf("api_%d", templateIndex)
		}
		templateIndex++

		// Build template
		template := make(map[string]any)
		
		// Add common headers
		var headerList []map[string]string
		for name, value := range commonHeaders {
			headerList = append(headerList, map[string]string{
				"name":  name,
				"value": value,
			})
		}
		// Sort headers for consistent output
		sort.Slice(headerList, func(i, j int) bool {
			return headerList[i]["name"] < headerList[j]["name"]
		})
		template["headers"] = headerList

		// Check if all requests have the same method
		commonMethod := groupReqs[0].Endpoint.Method
		allSameMethod := true
		for _, req := range groupReqs[1:] {
			if req.Endpoint.Method != commonMethod {
				allSameMethod = false
				break
			}
		}
		if allSameMethod {
			template["method"] = commonMethod
		}

		templates[templateName] = template

		// Assign template to requests
		for _, req := range groupReqs {
			assignments[req.Node.ID] = templateName
		}
	}

	return templates, assignments
}

// findCommonHeaders finds headers that appear in all requests
func findCommonHeaders(requests []RequestInfo) map[string]string {
	if len(requests) == 0 {
		return nil
	}

	// Start with headers from first request
	common := make(map[string]string)
	for k, v := range requests[0].Headers {
		common[k] = v
	}

	// Remove any that don't match in other requests
	for _, req := range requests[1:] {
		for k, v := range common {
			if reqValue, exists := req.Headers[k]; !exists || reqValue != v {
				delete(common, k)
			}
		}
	}

	return common
}

// convertRequestWithTemplate converts a request node using template information
func convertRequestWithTemplate(info RequestInfo, templateName string, template map[string]any) map[string]any {
	step := map[string]any{
		"name": info.Node.Name,
	}

	// Add template reference if assigned
	if templateName != "" {
		step["use_request"] = templateName
	}

	// Add URL (always include as it's usually unique)
	step["url"] = info.Endpoint.Url

	// Add method if not in template or different from template
	includeMethod := true
	if template != nil {
		if tplMethod, ok := template["method"].(string); ok && tplMethod == info.Endpoint.Method {
			includeMethod = false
		}
	}
	if includeMethod {
		step["method"] = info.Endpoint.Method
	}

	// Add headers that are not in the template
	var extraHeaders []map[string]string
	templateHeaders := make(map[string]string)
	if template != nil && template["headers"] != nil {
		if headers, ok := template["headers"].([]map[string]string); ok {
			for _, h := range headers {
				templateHeaders[h["name"]] = h["value"]
			}
		}
	}

	for name, value := range info.Headers {
		if tplValue, exists := templateHeaders[name]; !exists || tplValue != value {
			extraHeaders = append(extraHeaders, map[string]string{
				"name":  name,
				"value": value,
			})
		}
	}

	// Sort headers for consistent output
	sort.Slice(extraHeaders, func(i, j int) bool {
		return extraHeaders[i]["name"] < extraHeaders[j]["name"]
	})

	if len(extraHeaders) > 0 {
		step["headers"] = extraHeaders
	}

	// Add query params if any
	if len(info.QueryParams) > 0 {
		var queryList []map[string]string
		for name, value := range info.QueryParams {
			queryList = append(queryList, map[string]string{
				"name":  name,
				"value": value,
			})
		}
		sort.Slice(queryList, func(i, j int) bool {
			return queryList[i]["name"] < queryList[j]["name"]
		})
		step["query_params"] = queryList
	}

	// Add body if exists
	if info.Body != nil {
		step["body"] = info.Body
	}

	// Add dependencies
	if len(info.Dependencies) > 0 {
		step["depends_on"] = info.Dependencies
	}

	return map[string]any{"request": step}
}

// findRequestInfo finds the RequestInfo for a given node ID
func findRequestInfo(infos []RequestInfo, nodeID idwrap.IDWrap) *RequestInfo {
	for i := range infos {
		if infos[i].Node.ID == nodeID {
			return &infos[i]
		}
	}
	return nil
}