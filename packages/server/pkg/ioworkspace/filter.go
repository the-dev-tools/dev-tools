package ioworkspace

import (
	"fmt"

	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/model/massertres"
	"the-dev-tools/server/pkg/model/mbodyform"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mbodyurl"
	"the-dev-tools/server/pkg/model/mcollection"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mexamplequery"
	"the-dev-tools/server/pkg/model/mexampleresp"
	"the-dev-tools/server/pkg/model/mexamplerespheader"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mflowvariable"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mitemfolder"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
)

// FilterWorkspaceDataByExamples mutates the workspace data so that only flows and
// request nodes tied to the provided examples remain. The resulting workspace keeps
// a minimal graph per flow (start node plus the matched requests) which allows downstream
// exporters to emit focused YAML output.
func FilterWorkspaceDataByExamples(data *WorkspaceData, requested []idwrap.IDWrap) error {
	if data == nil {
		return fmt.Errorf("workspace data is nil")
	}
	if len(requested) == 0 {
		return nil
	}

	type void struct{}

	requestedSet := make(map[idwrap.IDWrap]struct{}, len(requested))
	for _, id := range requested {
		requestedSet[id] = void{}
	}

	// Map flow nodes by ID for quick lookup.
	nodeByID := make(map[idwrap.IDWrap]mnnode.MNode, len(data.FlowNodes))
	for _, node := range data.FlowNodes {
		nodeByID[node.ID] = node
	}

	// Build lookup for start nodes (per flow) to keep flows runnable after trimming.
	flowStartNode := make(map[idwrap.IDWrap]mnnode.MNode)
	flowStartNoop := make(map[idwrap.IDWrap]mnnoop.NoopNode)
	for _, noop := range data.FlowNoopNodes {
		if noop.Type != mnnoop.NODE_NO_OP_KIND_START {
			continue
		}
		node, ok := nodeByID[noop.FlowNodeID]
		if !ok {
			continue
		}
		flowStartNode[node.FlowID] = node
		flowStartNoop[node.FlowID] = noop
	}

	// Track request nodes we must keep and group them by flow.
	requestsByFlow := make(map[idwrap.IDWrap][]mnrequest.MNRequest)
	keptRequestNodes := make(map[idwrap.IDWrap]mnrequest.MNRequest)
	for _, req := range data.FlowRequestNodes {
		match := false
		if req.ExampleID != nil {
			if _, ok := requestedSet[*req.ExampleID]; ok {
				match = true
			}
		}
		if !match && req.DeltaExampleID != nil {
			if _, ok := requestedSet[*req.DeltaExampleID]; ok {
				match = true
			}
		}
		if !match {
			continue
		}

		node, ok := nodeByID[req.FlowNodeID]
		if !ok {
			continue
		}

		keptRequestNodes[req.FlowNodeID] = req
		requestsByFlow[node.FlowID] = append(requestsByFlow[node.FlowID], req)
	}

	if len(keptRequestNodes) == 0 {
		return filterExamplesWithoutFlows(data, requestedSet)
	}

	// Determine flows that remain after trimming and ensure each has a start node.
	keptFlows := make(map[idwrap.IDWrap]struct{})
	for flowID := range requestsByFlow {
		if _, ok := flowStartNode[flowID]; !ok {
			return fmt.Errorf("flow %s is missing start node", flowID.String())
		}
		keptFlows[flowID] = void{}
	}

	// Filter flows to only those referenced by kept requests.
	trimmedFlows := make([]mflow.Flow, 0, len(keptFlows))
	for _, flow := range data.Flows {
		if _, ok := keptFlows[flow.ID]; ok {
			trimmedFlows = append(trimmedFlows, flow)
		}
	}
	data.Flows = trimmedFlows

	// Rebuild flow level structures keeping only start + matched request nodes.
	keptNodeIDs := make(map[idwrap.IDWrap]struct{}, len(keptRequestNodes)*2)

	newFlowNodes := make([]mnnode.MNode, 0, len(keptRequestNodes)*2)
	newFlowEdges := make([]edge.Edge, 0, len(keptRequestNodes))
	newFlowNoops := make([]mnnoop.NoopNode, 0, len(keptFlows))
	newFlowRequests := make([]mnrequest.MNRequest, 0, len(keptRequestNodes))

	for _, flow := range data.Flows {
		reqs := requestsByFlow[flow.ID]
		if len(reqs) == 0 {
			continue
		}

		startNode := flowStartNode[flow.ID]
		if _, already := keptNodeIDs[startNode.ID]; !already {
			newFlowNodes = append(newFlowNodes, startNode)
			keptNodeIDs[startNode.ID] = void{}
			newFlowNoops = append(newFlowNoops, flowStartNoop[flow.ID])
		}

		for _, req := range reqs {
			node := nodeByID[req.FlowNodeID]
			if _, ok := keptNodeIDs[node.ID]; !ok {
				newFlowNodes = append(newFlowNodes, node)
				keptNodeIDs[node.ID] = void{}
			}
			newFlowRequests = append(newFlowRequests, req)
			newFlowEdges = append(newFlowEdges, edge.Edge{
				ID:            idwrap.NewNow(),
				FlowID:        flow.ID,
				SourceID:      startNode.ID,
				TargetID:      node.ID,
				SourceHandler: edge.HandleThen,
				Kind:          int32(edge.EdgeKindNoOp),
			})
		}
	}

	data.FlowNodes = newFlowNodes
	data.FlowEdges = newFlowEdges
	data.FlowNoopNodes = newFlowNoops
	data.FlowRequestNodes = newFlowRequests

	// Remove unused typed node collections after trimming.
	data.FlowConditionNodes = nil
	data.FlowForNodes = nil
	data.FlowForEachNodes = nil
	data.FlowJSNodes = nil

	// Retain flow variables only for the surviving flows.
	filteredVars := make([]mflowvariable.FlowVariable, 0, len(data.FlowVariables))
	for _, v := range data.FlowVariables {
		if _, ok := keptFlows[v.FlowID]; ok {
			filteredVars = append(filteredVars, v)
		}
	}
	data.FlowVariables = filteredVars

	// Collect example and endpoint IDs that must remain.
	exampleIDsToKeep := make(map[idwrap.IDWrap]struct{}, len(requestedSet))
	for id := range requestedSet {
		exampleIDsToKeep[id] = void{}
	}
	endpointIDsToKeep := make(map[idwrap.IDWrap]struct{})
	for _, req := range data.FlowRequestNodes {
		if req.ExampleID != nil {
			exampleIDsToKeep[*req.ExampleID] = void{}
		}
		if req.DeltaExampleID != nil {
			exampleIDsToKeep[*req.DeltaExampleID] = void{}
		}
		if req.EndpointID != nil {
			endpointIDsToKeep[*req.EndpointID] = void{}
		}
		if req.DeltaEndpointID != nil {
			endpointIDsToKeep[*req.DeltaEndpointID] = void{}
		}
	}

	// Ensure every requested example is still present via FlowRequestNodes.
	for id := range requestedSet {
		if _, ok := exampleIDsToKeep[id]; !ok {
			return fmt.Errorf("example %s is not referenced by any request node", id.String())
		}
	}

	// Filter examples and accumulate collection IDs.
	collectionIDsToKeep := make(map[idwrap.IDWrap]struct{})
	trimmedExamples := make([]mitemapiexample.ItemApiExample, 0, len(data.Examples))
	for _, ex := range data.Examples {
		if _, ok := exampleIDsToKeep[ex.ID]; !ok {
			continue
		}
		trimmedExamples = append(trimmedExamples, ex)
		collectionIDsToKeep[ex.CollectionID] = void{}
		endpointIDsToKeep[ex.ItemApiID] = void{}
	}
	data.Examples = trimmedExamples

	// Filter example-related children.
	data.ExampleHeaders = filterHeaders(data.ExampleHeaders, exampleIDsToKeep)
	data.ExampleQueries = filterQueries(data.ExampleQueries, exampleIDsToKeep)
	data.ExampleAsserts = filterAsserts(data.ExampleAsserts, exampleIDsToKeep)
	data.Rawbodies = filterRawBodies(data.Rawbodies, exampleIDsToKeep)
	data.FormBodies = filterFormBodies(data.FormBodies, exampleIDsToKeep)
	data.UrlBodies = filterURLBodies(data.UrlBodies, exampleIDsToKeep)

	// Filter responses and related artifacts.
	responseIDsToKeep := make(map[idwrap.IDWrap]struct{})
	trimmedResponses := make([]mexampleresp.ExampleResp, 0, len(data.ExampleResponses))
	for _, resp := range data.ExampleResponses {
		if _, ok := exampleIDsToKeep[resp.ExampleID]; !ok {
			continue
		}
		trimmedResponses = append(trimmedResponses, resp)
		responseIDsToKeep[resp.ID] = void{}
	}
	data.ExampleResponses = trimmedResponses

	data.ExampleResponseHeaders = filterRespHeaders(data.ExampleResponseHeaders, responseIDsToKeep)
	data.ExampleResponseAsserts = filterRespAsserts(data.ExampleResponseAsserts, responseIDsToKeep)

	// Filter endpoints, collections, folders based on kept examples.
	trimmedEndpoints := make([]mitemapi.ItemApi, 0, len(data.Endpoints))
	for _, ep := range data.Endpoints {
		if _, ok := endpointIDsToKeep[ep.ID]; ok {
			trimmedEndpoints = append(trimmedEndpoints, ep)
			collectionIDsToKeep[ep.CollectionID] = void{}
		}
	}
	data.Endpoints = trimmedEndpoints

	trimmedCollections := make([]mcollection.Collection, 0, len(data.Collections))
	for _, col := range data.Collections {
		if _, ok := collectionIDsToKeep[col.ID]; ok {
			trimmedCollections = append(trimmedCollections, col)
		}
	}
	data.Collections = trimmedCollections

	trimmedFolders := make([]mitemfolder.ItemFolder, 0, len(data.Folders))
	for _, folder := range data.Folders {
		if _, ok := collectionIDsToKeep[folder.CollectionID]; ok {
			trimmedFolders = append(trimmedFolders, folder)
		}
	}
	data.Folders = trimmedFolders

	return nil
}

func filterExamplesWithoutFlows(data *WorkspaceData, requestedSet map[idwrap.IDWrap]struct{}) error {
	if len(requestedSet) == 0 {
		return nil
	}

	type void struct{}

	requestedExamples := make([]mitemapiexample.ItemApiExample, 0, len(requestedSet))
	endpointIDsToKeep := make(map[idwrap.IDWrap]struct{}, len(requestedSet))
	collectionIDsToKeep := make(map[idwrap.IDWrap]struct{}, len(requestedSet))

	for _, ex := range data.Examples {
		if _, ok := requestedSet[ex.ID]; !ok {
			continue
		}
		requestedExamples = append(requestedExamples, ex)
		endpointIDsToKeep[ex.ItemApiID] = void{}
		collectionIDsToKeep[ex.CollectionID] = void{}
	}

	if len(requestedExamples) == 0 {
		return fmt.Errorf("requested examples not found in workspace")
	}

	exampleIDsToKeep := make(map[idwrap.IDWrap]struct{}, len(requestedExamples))
	for _, ex := range requestedExamples {
		exampleIDsToKeep[ex.ID] = void{}
	}

	data.Examples = requestedExamples
	data.ExampleHeaders = filterHeaders(data.ExampleHeaders, exampleIDsToKeep)
	data.ExampleQueries = filterQueries(data.ExampleQueries, exampleIDsToKeep)
	data.ExampleAsserts = filterAsserts(data.ExampleAsserts, exampleIDsToKeep)
	data.Rawbodies = filterRawBodies(data.Rawbodies, exampleIDsToKeep)
	data.FormBodies = filterFormBodies(data.FormBodies, exampleIDsToKeep)
	data.UrlBodies = filterURLBodies(data.UrlBodies, exampleIDsToKeep)

	responseIDsToKeep := make(map[idwrap.IDWrap]struct{})
	trimmedResponses := make([]mexampleresp.ExampleResp, 0, len(data.ExampleResponses))
	for _, resp := range data.ExampleResponses {
		if _, ok := exampleIDsToKeep[resp.ExampleID]; ok {
			trimmedResponses = append(trimmedResponses, resp)
			responseIDsToKeep[resp.ID] = void{}
		}
	}
	data.ExampleResponses = trimmedResponses
	data.ExampleResponseHeaders = filterRespHeaders(data.ExampleResponseHeaders, responseIDsToKeep)
	data.ExampleResponseAsserts = filterRespAsserts(data.ExampleResponseAsserts, responseIDsToKeep)

	trimmedEndpoints := make([]mitemapi.ItemApi, 0, len(data.Endpoints))
	for _, ep := range data.Endpoints {
		if _, ok := endpointIDsToKeep[ep.ID]; ok {
			trimmedEndpoints = append(trimmedEndpoints, ep)
			collectionIDsToKeep[ep.CollectionID] = void{}
		}
	}
	if len(trimmedEndpoints) == 0 {
		return fmt.Errorf("requested examples lack associated endpoints")
	}
	data.Endpoints = trimmedEndpoints

	trimmedCollections := make([]mcollection.Collection, 0, len(data.Collections))
	for _, col := range data.Collections {
		if _, ok := collectionIDsToKeep[col.ID]; ok {
			trimmedCollections = append(trimmedCollections, col)
		}
	}
	data.Collections = trimmedCollections

	trimmedFolders := make([]mitemfolder.ItemFolder, 0, len(data.Folders))
	for _, folder := range data.Folders {
		if _, ok := collectionIDsToKeep[folder.CollectionID]; ok {
			trimmedFolders = append(trimmedFolders, folder)
		}
	}
	data.Folders = trimmedFolders

	buildFlowsForExamples(data, requestedExamples)

	return nil
}

func buildFlowsForExamples(data *WorkspaceData, examples []mitemapiexample.ItemApiExample) {
	if len(examples) == 0 {
		data.Flows = nil
		data.FlowNodes = nil
		data.FlowEdges = nil
		data.FlowRequestNodes = nil
		data.FlowNoopNodes = nil
		data.FlowVariables = nil
		data.FlowConditionNodes = nil
		data.FlowForNodes = nil
		data.FlowForEachNodes = nil
		data.FlowJSNodes = nil
		return
	}

	newFlows := make([]mflow.Flow, 0, len(examples))
	newFlowNodes := make([]mnnode.MNode, 0, len(examples)*2)
	newFlowEdges := make([]edge.Edge, 0, len(examples))
	newFlowRequests := make([]mnrequest.MNRequest, 0, len(examples))
	newFlowNoops := make([]mnnoop.NoopNode, 0, len(examples))

	for idx, ex := range examples {
		flowID := idwrap.NewNow()
		flowName := ex.Name
		if flowName == "" {
			flowName = fmt.Sprintf("Example %d", idx+1)
		}

		newFlows = append(newFlows, mflow.Flow{
			ID:          flowID,
			WorkspaceID: data.Workspace.ID,
			Name:        flowName,
		})

		startNodeID := idwrap.NewNow()
		requestNodeID := idwrap.NewNow()

		startNodeName := fmt.Sprintf("Start %s", flowName)
		requestNodeName := ex.Name
		if requestNodeName == "" {
			requestNodeName = flowName
		}

		newFlowNodes = append(newFlowNodes,
			mnnode.MNode{ID: startNodeID, FlowID: flowID, Name: startNodeName, NodeKind: mnnode.NODE_KIND_NO_OP},
			mnnode.MNode{ID: requestNodeID, FlowID: flowID, Name: requestNodeName, NodeKind: mnnode.NODE_KIND_REQUEST},
		)

		newFlowNoops = append(newFlowNoops, mnnoop.NoopNode{FlowNodeID: startNodeID, Type: mnnoop.NODE_NO_OP_KIND_START})

		endpointID := ex.ItemApiID
		exampleID := ex.ID
		newFlowRequests = append(newFlowRequests, mnrequest.MNRequest{
			FlowNodeID:       requestNodeID,
			EndpointID:       &endpointID,
			ExampleID:        &exampleID,
			HasRequestConfig: true,
		})

		newFlowEdges = append(newFlowEdges, edge.Edge{
			ID:            idwrap.NewNow(),
			FlowID:        flowID,
			SourceID:      startNodeID,
			TargetID:      requestNodeID,
			SourceHandler: edge.HandleThen,
			Kind:          int32(edge.EdgeKindNoOp),
		})
	}

	data.Flows = newFlows
	data.FlowNodes = newFlowNodes
	data.FlowEdges = newFlowEdges
	data.FlowRequestNodes = newFlowRequests
	data.FlowNoopNodes = newFlowNoops
	data.FlowVariables = nil
	data.FlowConditionNodes = nil
	data.FlowForNodes = nil
	data.FlowForEachNodes = nil
	data.FlowJSNodes = nil
}

func filterHeaders(headers []mexampleheader.Header, allowed map[idwrap.IDWrap]struct{}) []mexampleheader.Header {
	if len(headers) == 0 {
		return headers
	}
	filtered := make([]mexampleheader.Header, 0, len(headers))
	for _, h := range headers {
		if _, ok := allowed[h.ExampleID]; ok {
			filtered = append(filtered, h)
		}
	}
	return filtered
}

func filterQueries(queries []mexamplequery.Query, allowed map[idwrap.IDWrap]struct{}) []mexamplequery.Query {
	if len(queries) == 0 {
		return queries
	}
	filtered := make([]mexamplequery.Query, 0, len(queries))
	for _, q := range queries {
		if _, ok := allowed[q.ExampleID]; ok {
			filtered = append(filtered, q)
		}
	}
	return filtered
}

func filterAsserts(asserts []massert.Assert, allowed map[idwrap.IDWrap]struct{}) []massert.Assert {
	if len(asserts) == 0 {
		return asserts
	}
	filtered := make([]massert.Assert, 0, len(asserts))
	for _, a := range asserts {
		if _, ok := allowed[a.ExampleID]; ok {
			filtered = append(filtered, a)
		}
	}
	return filtered
}

func filterRawBodies(bodies []mbodyraw.ExampleBodyRaw, allowed map[idwrap.IDWrap]struct{}) []mbodyraw.ExampleBodyRaw {
	if len(bodies) == 0 {
		return bodies
	}
	filtered := make([]mbodyraw.ExampleBodyRaw, 0, len(bodies))
	for _, b := range bodies {
		if _, ok := allowed[b.ExampleID]; ok {
			filtered = append(filtered, b)
		}
	}
	return filtered
}

func filterFormBodies(bodies []mbodyform.BodyForm, allowed map[idwrap.IDWrap]struct{}) []mbodyform.BodyForm {
	if len(bodies) == 0 {
		return bodies
	}
	filtered := make([]mbodyform.BodyForm, 0, len(bodies))
	for _, b := range bodies {
		if _, ok := allowed[b.ExampleID]; ok {
			filtered = append(filtered, b)
		}
	}
	return filtered
}

func filterURLBodies(bodies []mbodyurl.BodyURLEncoded, allowed map[idwrap.IDWrap]struct{}) []mbodyurl.BodyURLEncoded {
	if len(bodies) == 0 {
		return bodies
	}
	filtered := make([]mbodyurl.BodyURLEncoded, 0, len(bodies))
	for _, b := range bodies {
		if _, ok := allowed[b.ExampleID]; ok {
			filtered = append(filtered, b)
		}
	}
	return filtered
}

func filterRespHeaders(headers []mexamplerespheader.ExampleRespHeader, allowed map[idwrap.IDWrap]struct{}) []mexamplerespheader.ExampleRespHeader {
	if len(headers) == 0 {
		return headers
	}
	filtered := make([]mexamplerespheader.ExampleRespHeader, 0, len(headers))
	for _, h := range headers {
		if _, ok := allowed[h.ExampleRespID]; ok {
			filtered = append(filtered, h)
		}
	}
	return filtered
}

func filterRespAsserts(asserts []massertres.AssertResult, allowed map[idwrap.IDWrap]struct{}) []massertres.AssertResult {
	if len(asserts) == 0 {
		return asserts
	}
	filtered := make([]massertres.AssertResult, 0, len(asserts))
	for _, a := range asserts {
		if _, ok := allowed[a.ResponseID]; ok {
			filtered = append(filtered, a)
		}
	}
	return filtered
}
