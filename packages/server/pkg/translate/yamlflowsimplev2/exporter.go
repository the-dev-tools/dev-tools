package yamlflowsimplev2

import (
	"encoding/json"
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"
	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"the-dev-tools/server/pkg/model/mnnode/mnforeach"
	"the-dev-tools/server/pkg/model/mnnode/mnif"
	"the-dev-tools/server/pkg/model/mnnode/mnjs"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
)

// MarshalSimplifiedYAML converts resolved data structures back to the simplified YAML format
func MarshalSimplifiedYAML(data *SimplifiedYAMLResolvedV2) ([]byte, error) {
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
	for _, h := range data.Headers {
		headersMap[h.HttpID] = append(headersMap[h.HttpID], h)
	}

	paramsMap := make(map[idwrap.IDWrap][]mhttp.HTTPSearchParam)
	for _, p := range data.SearchParams {
		paramsMap[p.HttpID] = append(paramsMap[p.HttpID], p)
	}

	bodyRawMap := make(map[idwrap.IDWrap]*mhttp.HTTPBodyRaw)
	for _, b := range data.BodyRaw {
		if b != nil {
			bodyRawMap[b.HttpID] = b
		}
	}

	bodyFormMap := make(map[idwrap.IDWrap][]mhttp.HTTPBodyForm)
	for _, f := range data.BodyForms {
		bodyFormMap[f.HttpID] = append(bodyFormMap[f.HttpID], f)
	}

	bodyUrlMap := make(map[idwrap.IDWrap][]mhttp.HTTPBodyUrlencoded)
	for _, u := range data.BodyUrlencoded {
		bodyUrlMap[u.HttpID] = append(bodyUrlMap[u.HttpID], u)
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

	// 1. Construct the root YAML structure
	// We'll try to infer workspace name from the first flow or file if available, mostly placeholder since source is resolved data
	wsName := "Exported Workspace"
	if len(data.Files) > 0 {
		// Try to find a collection name if files were structured that way?
		// Since SimplifiedYAMLResolvedV2 doesn't have Workspace info directly, we use a default
	}

	yamlFormat := YamlFlowFormatV2{
		WorkspaceName: wsName,
		Flows:         make([]YamlFlowFlowV2, 0),
	}

	// 2. Process each Flow
	for _, flow := range data.Flows {
		flowYaml := YamlFlowFlowV2{
			Name:      flow.Name,
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

				baseStep["method"] = httpReq.Method
				baseStep["url"] = httpReq.Url

				// Headers
				if hdrs, ok := headersMap[httpReq.ID]; ok && len(hdrs) > 0 {
					// To match v2 schema tests which use map format often, or list format.
					// v2 schema supports list of objects with name/value/enabled.
					hList := make([]map[string]any, 0)
					for _, h := range hdrs {
						if !h.Enabled { continue }
						hList = append(hList, map[string]any{
							"name":  h.HeaderKey,
							"value": h.HeaderValue,
						})
					}
					if len(hList) > 0 {
						baseStep["headers"] = hList
					}
				}

				// Query Params
				if params, ok := paramsMap[httpReq.ID]; ok && len(params) > 0 {
					pList := make([]map[string]any, 0)
					for _, p := range params {
						if !p.Enabled { continue }
						pList = append(pList, map[string]any{
							"name":  p.ParamKey,
							"value": p.ParamValue,
						})
					}
					if len(pList) > 0 {
						baseStep["query_params"] = pList
					}
				}

				// Body
				bodyData := make(map[string]any)
				if forms, ok := bodyFormMap[httpReq.ID]; ok && len(forms) > 0 {
					bodyData["type"] = "form-data"
					fList := make([]map[string]any, 0)
					for _, f := range forms {
						if !f.Enabled { continue }
						fList = append(fList, map[string]any{
							"name": f.FormKey,
							"value": f.FormValue,
						})
					}
					bodyData["form_data"] = fList
				} else if urls, ok := bodyUrlMap[httpReq.ID]; ok && len(urls) > 0 {
					bodyData["type"] = "urlencoded"
					uList := make([]map[string]any, 0)
					for _, u := range urls {
						if !u.Enabled { continue }
						uList = append(uList, map[string]any{
							"name": u.UrlencodedKey,
							"value": u.UrlencodedValue,
						})
					}
					bodyData["urlencoded"] = uList
				} else if raw, ok := bodyRawMap[httpReq.ID]; ok {
					// Decompress if needed
					dataBytes := raw.RawData
					if raw.CompressionType != int8(compress.CompressTypeNone) {
						decompressed, err := compress.Decompress(dataBytes, compress.CompressType(raw.CompressionType))
						if err == nil {
							dataBytes = decompressed
						}
					}

					// Try JSON
					var jsonObj any
					if json.Unmarshal(dataBytes, &jsonObj) == nil {
						bodyData["type"] = "json"
						bodyData["json"] = jsonObj
					} else {
						bodyData["type"] = "raw"
						bodyData["raw"] = string(dataBytes)
					}
				}

				if len(bodyData) > 0 {
					baseStep["body"] = bodyData
				}

				stepMap["request"] = baseStep

			case mnnode.NODE_KIND_CONDITION:
				ifNode, ok := ifNodeMap[node.ID]
				if !ok { continue }
				
				baseStep["condition"] = ifNode.Condition.Comparisons.Expression
				
				// Find targets
				outgoing := edgesBySource[node.ID]
				for _, e := range outgoing {
					targetNode, found := nodeMap[e.TargetID]
					if !found { continue }
					
					if e.SourceHandler == edge.HandleThen {
						baseStep["then"] = targetNode.Name
					} else if e.SourceHandler == edge.HandleElse {
						baseStep["else"] = targetNode.Name
					}
				}
				stepMap["if"] = baseStep

			case mnnode.NODE_KIND_FOR:
				forNode, ok := forNodeMap[node.ID]
				if !ok { continue }

				baseStep["iter_count"] = forNode.IterCount
				
				// Find loop target
				outgoing := edgesBySource[node.ID]
				for _, e := range outgoing {
					targetNode, found := nodeMap[e.TargetID]
					if !found { continue }
					
					if e.SourceHandler == edge.HandleLoop {
						baseStep["loop"] = targetNode.Name
					}
				}
				stepMap["for"] = baseStep

			case mnnode.NODE_KIND_FOR_EACH:
				forEachNode, ok := forEachNodeMap[node.ID]
				if !ok { continue }

				baseStep["items"] = forEachNode.IterExpression

				// Find loop target
				outgoing := edgesBySource[node.ID]
				for _, e := range outgoing {
					targetNode, found := nodeMap[e.TargetID]
					if !found { continue }
					
					if e.SourceHandler == edge.HandleLoop {
						baseStep["loop"] = targetNode.Name
					}
				}
				stepMap["for_each"] = baseStep

			case mnnode.NODE_KIND_JS:
				jsNode, ok := jsNodeMap[node.ID]
				if !ok { continue }

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
