package yamlflowsimplev2

import (
	"encoding/json"
	"fmt"
	"sort"

	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/ioworkspace"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"the-dev-tools/server/pkg/model/mnnode/mnforeach"
	"the-dev-tools/server/pkg/model/mnnode/mnif"
	"the-dev-tools/server/pkg/model/mnnode/mnjs"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"

	"gopkg.in/yaml.v3"
)

// MarshalSimplifiedYAML converts resolved data structures back to the simplified YAML format
func MarshalSimplifiedYAML(data *ioworkspace.WorkspaceBundle) ([]byte, error) {
	if data == nil {
		return nil, fmt.Errorf("input data is nil")
	}

	// Build maps for efficient lookup
	nodeMap := make(map[idwrap.IDWrap]mnnode.MNode)
	for _, n := range data.FlowNodes {
		nodeMap[n.ID] = n
	}

	// HTTP Maps
	httpMap := make(map[idwrap.IDWrap]mhttp.HTTP)
	for _, h := range data.HTTPRequests {
		httpMap[h.ID] = h
	}

	// HTTP Related Data Maps
	headersMap := make(map[idwrap.IDWrap][]mhttp.HTTPHeader)
	for _, h := range data.HTTPHeaders {
		headersMap[h.HttpID] = append(headersMap[h.HttpID], h)
	}

	paramsMap := make(map[idwrap.IDWrap][]mhttp.HTTPSearchParam)
	for _, p := range data.HTTPSearchParams {
		paramsMap[p.HttpID] = append(paramsMap[p.HttpID], p)
	}

	bodyRawMap := make(map[idwrap.IDWrap]mhttp.HTTPBodyRaw)
	for _, b := range data.HTTPBodyRaw {
		bodyRawMap[b.HttpID] = b
	}

	bodyFormMap := make(map[idwrap.IDWrap][]mhttp.HTTPBodyForm)
	for _, f := range data.HTTPBodyForms {
		bodyFormMap[f.HttpID] = append(bodyFormMap[f.HttpID], f)
	}

	bodyUrlMap := make(map[idwrap.IDWrap][]mhttp.HTTPBodyUrlencoded)
	for _, u := range data.HTTPBodyUrlencoded {
		bodyUrlMap[u.HttpID] = append(bodyUrlMap[u.HttpID], u)
	}

	assertsMap := make(map[idwrap.IDWrap][]mhttp.HTTPAssert)
	for _, a := range data.HTTPAsserts {
		assertsMap[a.HttpID] = append(assertsMap[a.HttpID], a)
	}

	// Build lookup context for delta merging
	deltaCtx := &deltaLookupContext{
		httpMap:     httpMap,
		headersMap:  headersMap,
		paramsMap:   paramsMap,
		bodyRawMap:  bodyRawMap,
		bodyFormMap: bodyFormMap,
		bodyUrlMap:  bodyUrlMap,
		assertsMap:  assertsMap,
	}

	// Node Specific Maps
	reqNodeMap := make(map[idwrap.IDWrap]mnrequest.MNRequest)
	for _, n := range data.FlowRequestNodes {
		reqNodeMap[n.FlowNodeID] = n
	}

	ifNodeMap := make(map[idwrap.IDWrap]mnif.MNIF)
	for _, n := range data.FlowConditionNodes {
		ifNodeMap[n.FlowNodeID] = n
	}

	forNodeMap := make(map[idwrap.IDWrap]mnfor.MNFor)
	for _, n := range data.FlowForNodes {
		forNodeMap[n.FlowNodeID] = n
	}

	forEachNodeMap := make(map[idwrap.IDWrap]mnforeach.MNForEach)
	for _, n := range data.FlowForEachNodes {
		forEachNodeMap[n.FlowNodeID] = n
	}

	jsNodeMap := make(map[idwrap.IDWrap]mnjs.MNJS)
	for _, n := range data.FlowJSNodes {
		jsNodeMap[n.FlowNodeID] = n
	}

	// Edges Map (Source -> []Edge)
	edgesBySource := make(map[idwrap.IDWrap][]edge.Edge)
	edgesByTarget := make(map[idwrap.IDWrap][]edge.Edge)
	for _, e := range data.FlowEdges {
		edgesBySource[e.SourceID] = append(edgesBySource[e.SourceID], e)
		edgesByTarget[e.TargetID] = append(edgesByTarget[e.TargetID], e)
	}

	// 1. Construct the root YAML structure using the workspace name from the bundle
	wsName := data.Workspace.Name
	if wsName == "" {
		wsName = "Exported Workspace"
	}

	yamlFormat := YamlFlowFormatV2{
		WorkspaceName: wsName,
		Flows:         make([]YamlFlowFlowV2, 0),
	}

	// 2. Build top-level requests section from HTTP requests
	// Track which HTTP IDs we've already added to avoid duplicates
	httpIDToRequestName := make(map[idwrap.IDWrap]string)
	requestNameUsed := make(map[string]bool)
	// Track which base HTTP IDs have delta overrides (base HTTP ID -> delta HTTP ID)
	httpIDToDeltaID := make(map[idwrap.IDWrap]idwrap.IDWrap)

	// First pass: collect all HTTP requests used in flows and create unique names
	for _, flow := range data.Flows {
		for _, n := range data.FlowNodes {
			if n.FlowID != flow.ID || n.NodeKind != mnnode.NODE_KIND_REQUEST {
				continue
			}
			reqNode, ok := reqNodeMap[n.ID]
			if !ok || reqNode.HttpID == nil {
				continue
			}
			httpReq, ok := httpMap[*reqNode.HttpID]
			if !ok {
				continue
			}

			// Skip if already added
			if _, exists := httpIDToRequestName[httpReq.ID]; exists {
				continue
			}

			// Use HTTP request name as the request template name
			reqName := httpReq.Name
			if reqName == "" {
				reqName = "Request"
			}

			// Ensure unique name
			baseName := reqName
			counter := 1
			for requestNameUsed[reqName] {
				reqName = fmt.Sprintf("%s_%d", baseName, counter)
				counter++
			}
			requestNameUsed[reqName] = true
			httpIDToRequestName[httpReq.ID] = reqName

			// Track delta HTTP ID if present
			if reqNode.DeltaHttpID != nil {
				httpIDToDeltaID[httpReq.ID] = *reqNode.DeltaHttpID
			}
		}
	}

	// Second pass: build the requests section
	var requests []YamlRequestDefV2
	// Sort HTTP IDs for deterministic output
	var httpIDs []idwrap.IDWrap
	for httpID := range httpIDToRequestName {
		httpIDs = append(httpIDs, httpID)
	}
	sort.Slice(httpIDs, func(i, j int) bool {
		return httpIDToRequestName[httpIDs[i]] < httpIDToRequestName[httpIDs[j]]
	})

	for _, httpID := range httpIDs {
		reqName := httpIDToRequestName[httpID]
		httpReq := httpMap[httpID]

		// Check if this request has a delta override
		var deltaHttpID *idwrap.IDWrap
		if did, hasDelta := httpIDToDeltaID[httpID]; hasDelta {
			deltaHttpID = &did
		}

		// Build the request definition with delta merging
		reqDef := buildRequestDefWithDelta(reqName, httpReq, deltaHttpID, deltaCtx)
		requests = append(requests, reqDef)
	}

	if len(requests) > 0 {
		yamlFormat.Requests = requests
	}

	// 3. Process each Flow
	flowNameUsed := make(map[string]bool)
	for _, flow := range data.Flows {
		// Ensure unique flow name
		flowName := flow.Name
		if flowName == "" {
			flowName = "Flow"
		}
		
		baseName := flowName
		counter := 1
		for flowNameUsed[flowName] {
			flowName = fmt.Sprintf("%s_%d", baseName, counter)
			counter++
		}
		flowNameUsed[flowName] = true

		flowYaml := YamlFlowFlowV2{
			Name:      flowName,
			Variables: make([]YamlFlowVariableV2, 0),
			Steps:     make([]map[string]any, 0),
		}

		// Flow Variables
		for _, fv := range data.FlowVariables {
			if fv.FlowID == flow.ID {
				flowYaml.Variables = append(flowYaml.Variables, YamlFlowVariableV2{
					Name:  fv.Name,
					Value: fv.Value,
				})
			}
		}

		// Get all nodes for this flow
		var flowNodes []mnnode.MNode
		var startNodeID idwrap.IDWrap
		for _, n := range data.FlowNodes {
			if n.FlowID == flow.ID {
				flowNodes = append(flowNodes, n)
				// Check if it's a start node
				if n.NodeKind == mnnode.NODE_KIND_NO_OP {
					// Verify if it's actually the start node
					for _, noop := range data.FlowNoopNodes {
						if noop.FlowNodeID == n.ID && noop.Type == mnnoop.NODE_NO_OP_KIND_START {
							startNodeID = n.ID
							break
						}
					}
				}
			}
		}

		// Sort nodes topologically-ish to form a linear sequence for "steps"
		// We start BFS from StartNode
		orderedNodes := linearizeNodes(startNodeID, flowNodes, edgesBySource)

		// Process ordered nodes into steps
		for i, node := range orderedNodes {
			if node.ID == startNodeID {
				continue // Skip start node in output
			}

			stepMap := make(map[string]any)
			baseStep := map[string]any{
				"name": node.Name,
			}

			// Determine Dependencies
			// Find incoming edges that are NOT control flow (loop/then/else)
			var explicitDeps []string
			incoming := edgesByTarget[node.ID]
			for _, e := range incoming {
				// Filter out control flow edges from parents (handled by parent's 'then'/'loop' fields)
				// We only care about standard dependencies here.
				// But wait, we can't easily know if an incoming edge was a 'then' edge just by looking at the edge itself
				// if we didn't store that info. Fortunately Edge struct has SourceHandler.

				if e.SourceHandler == edge.HandleUnspecified {
					// This is a potential dependency
					sourceNode, ok := nodeMap[e.SourceID]
					if !ok || sourceNode.ID == startNodeID {
						continue
					}

					// Check if this is an implicit sequential dependency
					// i.e., is sourceNode the immediate predecessor in our ordered list?
					isPredecessor := false
					if i > 0 && orderedNodes[i-1].ID == sourceNode.ID {
						isPredecessor = true
					}

					if !isPredecessor {
						explicitDeps = append(explicitDeps, sourceNode.Name)
					}
				}
			}

			if len(explicitDeps) > 0 {
				// Sort for deterministic output
				sort.Strings(explicitDeps)
				baseStep["depends_on"] = explicitDeps
			}

			// Node Specific Logic
			switch node.NodeKind {
			case mnnode.NODE_KIND_REQUEST:
				reqNode, ok := reqNodeMap[node.ID]
				if !ok || reqNode.HttpID == nil {
					continue
				}
				httpReq, ok := httpMap[*reqNode.HttpID]
				if !ok {
					continue
				}

				// Use use_request to reference the top-level request
				if reqName, exists := httpIDToRequestName[httpReq.ID]; exists {
					baseStep["use_request"] = reqName
				} else {
					// Fallback to inline (shouldn't happen)
					baseStep["method"] = httpReq.Method
					baseStep["url"] = httpReq.Url
				}

				stepMap["request"] = baseStep

			case mnnode.NODE_KIND_CONDITION:
				ifNode, ok := ifNodeMap[node.ID]
				if !ok {
					continue
				}

				baseStep["condition"] = ifNode.Condition.Comparisons.Expression

				// Find targets
				outgoing := edgesBySource[node.ID]
				for _, e := range outgoing {
					targetNode, found := nodeMap[e.TargetID]
					if !found {
						continue
					}

					if e.SourceHandler == edge.HandleThen {
						baseStep["then"] = targetNode.Name
					} else if e.SourceHandler == edge.HandleElse {
						baseStep["else"] = targetNode.Name
					}
				}
				stepMap["if"] = baseStep

			case mnnode.NODE_KIND_FOR:
				forNode, ok := forNodeMap[node.ID]
				if !ok {
					continue
				}

				baseStep["iter_count"] = forNode.IterCount

				// Find loop target
				outgoing := edgesBySource[node.ID]
				for _, e := range outgoing {
					targetNode, found := nodeMap[e.TargetID]
					if !found {
						continue
					}

					if e.SourceHandler == edge.HandleLoop {
						baseStep["loop"] = targetNode.Name
					}
				}
				stepMap["for"] = baseStep

			case mnnode.NODE_KIND_FOR_EACH:
				forEachNode, ok := forEachNodeMap[node.ID]
				if !ok {
					continue
				}

				baseStep["items"] = forEachNode.IterExpression

				// Find loop target
				outgoing := edgesBySource[node.ID]
				for _, e := range outgoing {
					targetNode, found := nodeMap[e.TargetID]
					if !found {
						continue
					}

					if e.SourceHandler == edge.HandleLoop {
						baseStep["loop"] = targetNode.Name
					}
				}
				stepMap["for_each"] = baseStep

			case mnnode.NODE_KIND_JS:
				jsNode, ok := jsNodeMap[node.ID]
				if !ok {
					continue
				}

				baseStep["code"] = string(jsNode.Code)
				stepMap["js"] = baseStep

			case mnnode.NODE_KIND_NO_OP:
				// Skip other no-ops
				continue
			}

			if len(stepMap) > 0 {
				flowYaml.Steps = append(flowYaml.Steps, stepMap)
			}
		}

		yamlFormat.Flows = append(yamlFormat.Flows, flowYaml)
	}

	// 4. Export Environments
	if len(data.Environments) > 0 {
		envMap := make(map[idwrap.IDWrap]*YamlEnvironmentV2)
		
		// Initialize environments
		for _, env := range data.Environments {
			envMap[env.ID] = &YamlEnvironmentV2{
				Name:      env.Name,
				Variables: make(map[string]string),
			}
		}

		// Add variables
		for _, v := range data.EnvironmentVars {
			if env, ok := envMap[v.EnvID]; ok {
				env.Variables[v.VarKey] = v.Value
			}
		}

		// Convert map to slice
		for _, env := range data.Environments {
			if yamlEnv, ok := envMap[env.ID]; ok {
				yamlFormat.Environments = append(yamlFormat.Environments, *yamlEnv)
			}
		}
	}

	// 5. Generate default Run configuration
	// Since the database doesn't store the 'run' configuration explicitly,
	// we generate a default linear run sequence based on the exported flows.
	// This ensures the exported YAML is valid and runnable.
	if len(yamlFormat.Flows) > 0 {
		yamlFormat.Run = make([]map[string]any, 0, len(yamlFormat.Flows))
		for _, flow := range yamlFormat.Flows {
			yamlFormat.Run = append(yamlFormat.Run, map[string]any{
				"flow": flow.Name,
			})
		}
	}

	return yaml.Marshal(yamlFormat)
}

// linearizeNodes attempts to create a linear sequence of nodes starting from startNode.
// It basically performs a BFS/topological traversal to order nodes in a way that makes sense for a YAML list.
func linearizeNodes(startNodeID idwrap.IDWrap, allNodes []mnnode.MNode, edgesBySource map[idwrap.IDWrap][]edge.Edge) []mnnode.MNode {
	nodeMap := make(map[idwrap.IDWrap]mnnode.MNode)
	for _, n := range allNodes {
		nodeMap[n.ID] = n
	}

	visited := make(map[idwrap.IDWrap]bool)
	var result []mnnode.MNode
	queue := []idwrap.IDWrap{startNodeID}
	visited[startNodeID] = true

	// NOTE: This is a simplified BFS. For a perfect reproduction of the original "steps" list order,
	// we would need to preserve the index order if available.
	// Since we don't have the original index, BFS is a reasonable approximation for execution order.
	// A pure dependency topological sort might be better, but BFS handles the "flow" visualization better.

	for len(queue) > 0 {
		currentID := queue[0]
		queue = queue[1:]

		if n, ok := nodeMap[currentID]; ok {
			result = append(result, n)
		}

		// Find neighbors
		edges := edgesBySource[currentID]

		// Sort edges to be deterministic?
		// In graph theory, the order of edges doesn't matter, but for stability it's nice.
		// We can't easily sort edges without looking up target names.

		var neighbors []mnnode.MNode
		for _, e := range edges {
			if target, ok := nodeMap[e.TargetID]; ok {
				neighbors = append(neighbors, target)
			}
		}

		// Sort neighbors by name to ensure deterministic output
		sort.Slice(neighbors, func(i, j int) bool {
			return neighbors[i].Name < neighbors[j].Name
		})

		for _, neighbor := range neighbors {
			if !visited[neighbor.ID] {
				visited[neighbor.ID] = true
				queue = append(queue, neighbor.ID)
			}
		}
	}

	// Add any disconnected nodes (shouldn't happen in valid flows, but good for robustness)
	var disconnected []mnnode.MNode
	for _, n := range allNodes {
		if !visited[n.ID] {
			disconnected = append(disconnected, n)
		}
	}
	// Sort disconnected nodes
	sort.Slice(disconnected, func(i, j int) bool {
		return disconnected[i].Name < disconnected[j].Name
	})
	result = append(result, disconnected...)

	return result
}

// deltaLookupContext holds all the maps needed to look up delta values
type deltaLookupContext struct {
	httpMap     map[idwrap.IDWrap]mhttp.HTTP
	headersMap  map[idwrap.IDWrap][]mhttp.HTTPHeader
	paramsMap   map[idwrap.IDWrap][]mhttp.HTTPSearchParam
	bodyRawMap  map[idwrap.IDWrap]mhttp.HTTPBodyRaw
	bodyFormMap map[idwrap.IDWrap][]mhttp.HTTPBodyForm
	bodyUrlMap  map[idwrap.IDWrap][]mhttp.HTTPBodyUrlencoded
	assertsMap  map[idwrap.IDWrap][]mhttp.HTTPAssert
}

// buildRequestDefWithDelta builds a YamlRequestDefV2 by merging base HTTP with delta overrides
func buildRequestDefWithDelta(reqName string, baseHttp mhttp.HTTP, deltaHttpID *idwrap.IDWrap, ctx *deltaLookupContext) YamlRequestDefV2 {
	// Start with base values
	method := baseHttp.Method
	url := baseHttp.Url

	// If there's a delta HTTP, check for overrides on the delta HTTP itself
	if deltaHttpID != nil {
		if deltaHttp, ok := ctx.httpMap[*deltaHttpID]; ok {
			// Delta HTTP can override URL and Method via Delta* fields
			if deltaHttp.DeltaUrl != nil && *deltaHttp.DeltaUrl != "" {
				url = *deltaHttp.DeltaUrl
			}
			if deltaHttp.DeltaMethod != nil && *deltaHttp.DeltaMethod != "" {
				method = *deltaHttp.DeltaMethod
			}
		}
	}

	reqDef := YamlRequestDefV2{
		Name:   reqName,
		Method: method,
		URL:    url,
	}

	// Merge headers
	reqDef.Headers = mergeHeaders(baseHttp.ID, deltaHttpID, ctx)

	// Merge query params
	reqDef.QueryParams = mergeQueryParams(baseHttp.ID, deltaHttpID, ctx)

	// Merge body
	reqDef.Body = mergeBody(baseHttp.ID, deltaHttpID, ctx)

	// Merge assertions
	reqDef.Assertions = mergeAssertions(baseHttp.ID, deltaHttpID, ctx)

	return reqDef
}

// mergeHeaders merges base headers with delta header overrides
func mergeHeaders(baseHttpID idwrap.IDWrap, deltaHttpID *idwrap.IDWrap, ctx *deltaLookupContext) map[string]string {
	result := make(map[string]string)

	// Get base headers
	baseHeaders := ctx.headersMap[baseHttpID]

	// Build a map of base header ID -> delta header for quick lookup
	deltaByParentID := make(map[idwrap.IDWrap]mhttp.HTTPHeader)
	if deltaHttpID != nil {
		for _, dh := range ctx.headersMap[*deltaHttpID] {
			if dh.ParentHttpHeaderID != nil {
				deltaByParentID[*dh.ParentHttpHeaderID] = dh
			}
		}
	}

	// Process each base header
	for _, h := range baseHeaders {
		key := h.Key
		value := h.Value
		enabled := h.Enabled

		// Check if there's a delta override for this header
		if delta, hasDelta := deltaByParentID[h.ID]; hasDelta {
			// Apply delta overrides
			if delta.DeltaKey != nil {
				key = *delta.DeltaKey
			}
			if delta.DeltaValue != nil {
				value = *delta.DeltaValue
			}
			if delta.DeltaEnabled != nil {
				enabled = *delta.DeltaEnabled
			}
		}

		if enabled {
			result[key] = value
		}
	}

	// Also add any new headers from delta that don't have a parent (new headers added in delta)
	if deltaHttpID != nil {
		for _, dh := range ctx.headersMap[*deltaHttpID] {
			if dh.ParentHttpHeaderID == nil && dh.Enabled {
				// This is a new header added in the delta
				result[dh.Key] = dh.Value
			}
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

// mergeQueryParams merges base query params with delta overrides
func mergeQueryParams(baseHttpID idwrap.IDWrap, deltaHttpID *idwrap.IDWrap, ctx *deltaLookupContext) map[string]string {
	result := make(map[string]string)

	// Get base params
	baseParams := ctx.paramsMap[baseHttpID]

	// Build a map of base param ID -> delta param for quick lookup
	deltaByParentID := make(map[idwrap.IDWrap]mhttp.HTTPSearchParam)
	if deltaHttpID != nil {
		for _, dp := range ctx.paramsMap[*deltaHttpID] {
			if dp.ParentHttpSearchParamID != nil {
				deltaByParentID[*dp.ParentHttpSearchParamID] = dp
			}
		}
	}

	// Process each base param
	for _, p := range baseParams {
		key := p.Key
		value := p.Value
		enabled := p.Enabled

		// Check if there's a delta override
		if delta, hasDelta := deltaByParentID[p.ID]; hasDelta {
			if delta.DeltaKey != nil {
				key = *delta.DeltaKey
			}
			if delta.DeltaValue != nil {
				value = *delta.DeltaValue
			}
			if delta.DeltaEnabled != nil {
				enabled = *delta.DeltaEnabled
			}
		}

		if enabled {
			result[key] = value
		}
	}

	// Add new params from delta
	if deltaHttpID != nil {
		for _, dp := range ctx.paramsMap[*deltaHttpID] {
			if dp.ParentHttpSearchParamID == nil && dp.Enabled {
				result[dp.Key] = dp.Value
			}
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

// mergeBody merges base body with delta overrides
func mergeBody(baseHttpID idwrap.IDWrap, deltaHttpID *idwrap.IDWrap, ctx *deltaLookupContext) any {
	// Check for form data
	if forms := mergeBodyForms(baseHttpID, deltaHttpID, ctx); forms != nil {
		return forms
	}

	// Check for urlencoded
	if urlencoded := mergeBodyUrlencoded(baseHttpID, deltaHttpID, ctx); urlencoded != nil {
		return urlencoded
	}

	// Check for raw body
	return mergeBodyRaw(baseHttpID, deltaHttpID, ctx)
}

// mergeBodyForms merges base form data with delta overrides
func mergeBodyForms(baseHttpID idwrap.IDWrap, deltaHttpID *idwrap.IDWrap, ctx *deltaLookupContext) any {
	baseForms := ctx.bodyFormMap[baseHttpID]
	if len(baseForms) == 0 {
		// Check if delta has forms
		if deltaHttpID != nil {
			deltaForms := ctx.bodyFormMap[*deltaHttpID]
			if len(deltaForms) > 0 {
				bodyData := map[string]any{"type": "form-data"}
				fList := make([]map[string]any, 0)
				for _, f := range deltaForms {
					if f.Enabled {
						fList = append(fList, map[string]any{"name": f.Key, "value": f.Value})
					}
				}
				if len(fList) > 0 {
					bodyData["form_data"] = fList
					return bodyData
				}
			}
		}
		return nil
	}

	// Build delta lookup
	deltaByParentID := make(map[idwrap.IDWrap]mhttp.HTTPBodyForm)
	if deltaHttpID != nil {
		for _, df := range ctx.bodyFormMap[*deltaHttpID] {
			if df.ParentHttpBodyFormID != nil {
				deltaByParentID[*df.ParentHttpBodyFormID] = df
			}
		}
	}

	bodyData := map[string]any{"type": "form-data"}
	fList := make([]map[string]any, 0)

	for _, f := range baseForms {
		key := f.Key
		value := f.Value
		enabled := f.Enabled

		if delta, hasDelta := deltaByParentID[f.ID]; hasDelta {
			if delta.DeltaKey != nil {
				key = *delta.DeltaKey
			}
			if delta.DeltaValue != nil {
				value = *delta.DeltaValue
			}
			if delta.DeltaEnabled != nil {
				enabled = *delta.DeltaEnabled
			}
		}

		if enabled {
			fList = append(fList, map[string]any{"name": key, "value": value})
		}
	}

	// Add new form fields from delta
	if deltaHttpID != nil {
		for _, df := range ctx.bodyFormMap[*deltaHttpID] {
			if df.ParentHttpBodyFormID == nil && df.Enabled {
				fList = append(fList, map[string]any{"name": df.Key, "value": df.Value})
			}
		}
	}

	if len(fList) > 0 {
		bodyData["form_data"] = fList
		return bodyData
	}
	return nil
}

// mergeBodyUrlencoded merges base urlencoded data with delta overrides
func mergeBodyUrlencoded(baseHttpID idwrap.IDWrap, deltaHttpID *idwrap.IDWrap, ctx *deltaLookupContext) any {
	baseUrls := ctx.bodyUrlMap[baseHttpID]
	if len(baseUrls) == 0 {
		// Check if delta has urlencoded
		if deltaHttpID != nil {
			deltaUrls := ctx.bodyUrlMap[*deltaHttpID]
			if len(deltaUrls) > 0 {
				bodyData := map[string]any{"type": "urlencoded"}
				uList := make([]map[string]any, 0)
				for _, u := range deltaUrls {
					if u.Enabled {
						uList = append(uList, map[string]any{"name": u.Key, "value": u.Value})
					}
				}
				if len(uList) > 0 {
					bodyData["urlencoded"] = uList
					return bodyData
				}
			}
		}
		return nil
	}

	// Build delta lookup
	deltaByParentID := make(map[idwrap.IDWrap]mhttp.HTTPBodyUrlencoded)
	if deltaHttpID != nil {
		for _, du := range ctx.bodyUrlMap[*deltaHttpID] {
			if du.ParentHttpBodyUrlEncodedID != nil {
				deltaByParentID[*du.ParentHttpBodyUrlEncodedID] = du
			}
		}
	}

	bodyData := map[string]any{"type": "urlencoded"}
	uList := make([]map[string]any, 0)

	for _, u := range baseUrls {
		key := u.Key
		value := u.Value
		enabled := u.Enabled

		if delta, hasDelta := deltaByParentID[u.ID]; hasDelta {
			if delta.DeltaKey != nil {
				key = *delta.DeltaKey
			}
			if delta.DeltaValue != nil {
				value = *delta.DeltaValue
			}
			if delta.DeltaEnabled != nil {
				enabled = *delta.DeltaEnabled
			}
		}

		if enabled {
			uList = append(uList, map[string]any{"name": key, "value": value})
		}
	}

	// Add new urlencoded fields from delta
	if deltaHttpID != nil {
		for _, du := range ctx.bodyUrlMap[*deltaHttpID] {
			if du.ParentHttpBodyUrlEncodedID == nil && du.Enabled {
				uList = append(uList, map[string]any{"name": du.Key, "value": du.Value})
			}
		}
	}

	if len(uList) > 0 {
		bodyData["urlencoded"] = uList
		return bodyData
	}
	return nil
}

// mergeBodyRaw merges base raw body with delta override
func mergeBodyRaw(baseHttpID idwrap.IDWrap, deltaHttpID *idwrap.IDWrap, ctx *deltaLookupContext) any {
	baseRaw, hasBase := ctx.bodyRawMap[baseHttpID]

	// Check for delta raw body
	var deltaRaw *mhttp.HTTPBodyRaw
	if deltaHttpID != nil {
		if dr, ok := ctx.bodyRawMap[*deltaHttpID]; ok {
			deltaRaw = &dr
		}
	}

	// Determine which raw data to use
	var dataBytes []byte
	var compressionType int8

	if deltaRaw != nil && len(deltaRaw.DeltaRawData) > 0 {
		// Use delta raw data
		dataBytes = deltaRaw.DeltaRawData
		// Delta compression type handling
		if ct, ok := deltaRaw.DeltaCompressionType.(int8); ok {
			compressionType = ct
		} else {
			compressionType = deltaRaw.CompressionType
		}
	} else if hasBase {
		// Use base raw data
		dataBytes = baseRaw.RawData
		compressionType = baseRaw.CompressionType
	} else {
		return nil
	}

	// Decompress if needed
	if compressionType != int8(compress.CompressTypeNone) {
		decompressed, err := compress.Decompress(dataBytes, compress.CompressType(compressionType))
		if err == nil {
			dataBytes = decompressed
		}
	}

	if len(dataBytes) == 0 {
		return nil
	}

	// Try to parse as JSON
	var jsonObj any
	if json.Unmarshal(dataBytes, &jsonObj) == nil {
		return map[string]any{"type": "json", "json": jsonObj}
	}
	return map[string]any{"type": "raw", "raw": string(dataBytes)}
}

// mergeAssertions merges base assertions with delta overrides
func mergeAssertions(baseHttpID idwrap.IDWrap, deltaHttpID *idwrap.IDWrap, ctx *deltaLookupContext) []string {
	var result []string

	// Get base assertions
	baseAsserts := ctx.assertsMap[baseHttpID]

	// Build delta lookup
	deltaByParentID := make(map[idwrap.IDWrap]mhttp.HTTPAssert)
	if deltaHttpID != nil {
		for _, da := range ctx.assertsMap[*deltaHttpID] {
			if da.ParentHttpAssertID != nil {
				deltaByParentID[*da.ParentHttpAssertID] = da
			}
		}
	}

	// Process base assertions
	for _, a := range baseAsserts {
		value := a.Value
		enabled := a.Enabled

		if delta, hasDelta := deltaByParentID[a.ID]; hasDelta {
			if delta.DeltaValue != nil {
				value = *delta.DeltaValue
			}
			if delta.DeltaEnabled != nil {
				enabled = *delta.DeltaEnabled
			}
		}

		if enabled && value != "" {
			result = append(result, value)
		}
	}

	// Add new assertions from delta
	if deltaHttpID != nil {
		for _, da := range ctx.assertsMap[*deltaHttpID] {
			if da.ParentHttpAssertID == nil && da.Enabled && da.Value != "" {
				result = append(result, da.Value)
			}
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}
