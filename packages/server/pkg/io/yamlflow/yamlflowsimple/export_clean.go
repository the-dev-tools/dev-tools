package yamlflowsimple

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"sort"
	"strconv"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/http/request"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/ioworkspace"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/model/mbodyform"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mbodyurl"
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
func buildRequestDefinitions(workspaceData *ioworkspace.WorkspaceData) map[string]map[string]any {
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

	processedNodes := make(map[string]bool)

	for _, node := range workspaceData.FlowNodes {
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

		baseExample := findExampleByID(workspaceData, *reqNode.ExampleID)
		if baseExample == nil {
			continue
		}

		baseHeaders := collectHeadersByExampleID(workspaceData, baseExample.ID)
		baseQueries := collectQueriesByExampleID(workspaceData, baseExample.ID)
		baseRawBody, baseRawFound := collectRawBodyByExampleID(workspaceData, baseExample.ID)
		baseFormBody := collectFormBodiesByExampleID(workspaceData, baseExample.ID)
		baseUrlBody := collectUrlBodiesByExampleID(workspaceData, baseExample.ID)
		baseAsserts := collectOrderedAssertsForExample(workspaceData, baseExample.ID)

		finalHeaders := baseHeaders
		finalQueries := baseQueries
		finalRawBody := baseRawBody
		finalAsserts := baseAsserts

		if reqNode.DeltaExampleID != nil {
			deltaExample := findExampleByID(workspaceData, *reqNode.DeltaExampleID)
			if deltaExample != nil {
				deltaHeaders := collectHeadersByExampleID(workspaceData, deltaExample.ID)
				deltaQueries := collectQueriesByExampleID(workspaceData, deltaExample.ID)
				deltaRawBody, deltaRawFound := collectRawBodyByExampleID(workspaceData, deltaExample.ID)
				deltaFormBody := collectFormBodiesByExampleID(workspaceData, deltaExample.ID)
				deltaUrlBody := collectUrlBodiesByExampleID(workspaceData, deltaExample.ID)
				deltaAsserts := collectOrderedAssertsForExample(workspaceData, deltaExample.ID)

				mergeInput := request.MergeExamplesInput{
					Base:                *baseExample,
					Delta:               *deltaExample,
					BaseQueries:         baseQueries,
					DeltaQueries:        deltaQueries,
					BaseHeaders:         baseHeaders,
					DeltaHeaders:        deltaHeaders,
					BaseRawBody:         baseRawBody,
					DeltaRawBody:        deltaRawBody,
					BaseFormBody:        baseFormBody,
					DeltaFormBody:       deltaFormBody,
					BaseUrlEncodedBody:  baseUrlBody,
					DeltaUrlEncodedBody: deltaUrlBody,
					BaseAsserts:         baseAsserts,
					DeltaAsserts:        deltaAsserts,
				}

				if !baseRawFound {
					mergeInput.BaseRawBody = mbodyraw.ExampleBodyRaw{}
				}
				if !deltaRawFound {
					mergeInput.DeltaRawBody = mbodyraw.ExampleBodyRaw{}
				}

				mergeOutput := request.MergeExamples(mergeInput)
				finalHeaders = mergeOutput.MergeHeaders
				finalQueries = mergeOutput.MergeQueries
				finalRawBody = mergeOutput.MergeRawBody
				finalAsserts = mergeOutput.MergeAsserts
			}
		}

		headerMap := headersToMap(finalHeaders)
		if len(headerMap) > 0 {
			requestEntry["headers"] = headerMap
		}

		queryMap := queriesToMap(finalQueries)
		if len(queryMap) > 0 {
			requestEntry["query_params"] = queryMap
		}

		if len(finalRawBody.Data) > 0 {
			var bodyData any
			if err := json.Unmarshal(finalRawBody.Data, &bodyData); err == nil {
				requestEntry["body"] = bodyData
			}
		}

		if assertionEntries := assertsToEntries(finalAsserts); len(assertionEntries) > 0 {
			requestEntry["assertions"] = assertionEntries
		}

		requests[node.Name] = requestEntry
	}

	return requests
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

		deltaAsserts := collectOrderedAssertsForExample(workspaceData, *requestNode.DeltaExampleID)
		if len(deltaAsserts) > 0 {
			var baseAsserts []massert.Assert
			if requestNode.ExampleID != nil {
				baseAsserts = collectOrderedAssertsForExample(workspaceData, *requestNode.ExampleID)
			}
			if !assertionsEqual(baseAsserts, deltaAsserts) {
				if entries := assertsToEntries(deltaAsserts); len(entries) > 0 {
					step["assertions"] = entries
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
