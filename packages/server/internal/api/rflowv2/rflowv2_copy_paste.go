//nolint:revive // exported
package rflowv2

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"gopkg.in/yaml.v3"

	devtoolsdb "github.com/the-dev-tools/dev-tools/packages/db"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rhttp"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/converter"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/ioworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/menv"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/shttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/translate/yamlflowsimplev2"
	flowv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/flow/v1"
)

// FlowNodesCopy serializes selected nodes to YAML for clipboard copy.
func (s *FlowServiceV2RPC) FlowNodesCopy(
	ctx context.Context,
	req *connect.Request[flowv1.FlowNodesCopyRequest],
) (*connect.Response[flowv1.FlowNodesCopyResponse], error) {
	if len(req.Msg.GetFlowId()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow id is required"))
	}
	if len(req.Msg.GetNodeIds()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one node id is required"))
	}

	flowID, err := idwrap.NewFromBytes(req.Msg.GetFlowId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid flow id: %w", err))
	}

	if err := s.ensureFlowAccess(ctx, flowID); err != nil {
		return nil, err
	}

	sourceFlow, err := s.fsReader.GetFlow(ctx, flowID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("flow not found: %w", err))
	}

	// Parse requested node IDs
	selectedIDs := make(map[idwrap.IDWrap]bool, len(req.Msg.GetNodeIds()))
	for _, rawID := range req.Msg.GetNodeIds() {
		nodeID, err := idwrap.NewFromBytes(rawID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}
		selectedIDs[nodeID] = true
	}

	// Fetch all nodes in the flow
	allNodes, err := s.nsReader.GetNodesByFlowID(ctx, flowID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Filter to selected nodes, skip ManualStart
	var selectedNodes []mflow.Node
	for _, n := range allNodes {
		if selectedIDs[n.ID] && n.NodeKind != mflow.NODE_KIND_MANUAL_START {
			selectedNodes = append(selectedNodes, n)
		}
	}

	if len(selectedNodes) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("no valid nodes selected (ManualStart nodes are excluded)"))
	}

	// Build WorkspaceBundle for selected nodes
	bundle := &ioworkspace.WorkspaceBundle{
		Workspace: mworkspace.Workspace{Name: "_clipboard"},
		Flows: []mflow.Flow{{
			ID:          flowID,
			WorkspaceID: sourceFlow.WorkspaceID,
			Name:        "_clipboard",
		}},
	}

	selectedNodeIDs := make(map[idwrap.IDWrap]bool, len(selectedNodes))
	for _, n := range selectedNodes {
		selectedNodeIDs[n.ID] = true
		bundle.FlowNodes = append(bundle.FlowNodes, n)

		// Fetch type-specific data
		switch n.NodeKind {
		case mflow.NODE_KIND_REQUEST:
			if d, err := s.nrs.GetNodeRequest(ctx, n.ID); err == nil && d != nil {
				bundle.FlowRequestNodes = append(bundle.FlowRequestNodes, *d)
				// Fetch HTTP request and all associated data for the exporter
				if d.HttpID != nil {
					if h, err := s.hsReader.Get(ctx, *d.HttpID); err == nil {
						bundle.HTTPRequests = append(bundle.HTTPRequests, *h)
						s.populateHTTPBundle(ctx, h.ID, bundle)
					}
					// If there's a delta, fetch it too for the exporter's delta resolution
					if d.DeltaHttpID != nil {
						if dh, err := s.hsReader.Get(ctx, *d.DeltaHttpID); err == nil {
							bundle.HTTPRequests = append(bundle.HTTPRequests, *dh)
							s.populateHTTPBundle(ctx, dh.ID, bundle)
						}
					}
				}
			}
		case mflow.NODE_KIND_FOR:
			if d, err := s.nfs.GetNodeFor(ctx, n.ID); err == nil {
				bundle.FlowForNodes = append(bundle.FlowForNodes, *d)
			}
		case mflow.NODE_KIND_FOR_EACH:
			if d, err := s.nfes.GetNodeForEach(ctx, n.ID); err == nil {
				bundle.FlowForEachNodes = append(bundle.FlowForEachNodes, *d)
			}
		case mflow.NODE_KIND_CONDITION:
			if d, err := s.nifs.GetNodeIf(ctx, n.ID); err == nil {
				bundle.FlowConditionNodes = append(bundle.FlowConditionNodes, *d)
			}
		case mflow.NODE_KIND_JS:
			if d, err := s.njss.GetNodeJS(ctx, n.ID); err == nil {
				bundle.FlowJSNodes = append(bundle.FlowJSNodes, *d)
			}
		case mflow.NODE_KIND_AI:
			if s.nais != nil {
				if d, err := s.nais.GetNodeAI(ctx, n.ID); err == nil {
					bundle.FlowAINodes = append(bundle.FlowAINodes, *d)
				}
			}
		case mflow.NODE_KIND_AI_PROVIDER:
			if s.naps != nil {
				if d, err := s.naps.GetNodeAiProvider(ctx, n.ID); err == nil {
					bundle.FlowAIProviderNodes = append(bundle.FlowAIProviderNodes, *d)
				}
			}
		case mflow.NODE_KIND_AI_MEMORY:
			if s.nmems != nil {
				if d, err := s.nmems.GetNodeMemory(ctx, n.ID); err == nil {
					bundle.FlowAIMemoryNodes = append(bundle.FlowAIMemoryNodes, *d)
				}
			}
		}
	}

	// Fetch edges — keep only edges where both source and target are in the selected set
	allEdges, err := s.es.GetEdgesByFlowID(ctx, flowID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	for _, e := range allEdges {
		if selectedNodeIDs[e.SourceID] && selectedNodeIDs[e.TargetID] {
			bundle.FlowEdges = append(bundle.FlowEdges, e)
		}
	}

	// Serialize to YAML
	yamlBytes, err := yamlflowsimplev2.MarshalSimplifiedYAML(bundle)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to serialize nodes to YAML: %w", err))
	}

	return connect.NewResponse(&flowv1.FlowNodesCopyResponse{
		Yaml: string(yamlBytes),
	}), nil
}

// populateHTTPBundle fetches headers, params, body, and assertions for an HTTP request
// and adds them to the workspace bundle.
func (s *FlowServiceV2RPC) populateHTTPBundle(ctx context.Context, httpID idwrap.IDWrap, bundle *ioworkspace.WorkspaceBundle) {
	if headers, err := s.hs.GetHeadersByHttpID(ctx, httpID); err == nil {
		bundle.HTTPHeaders = append(bundle.HTTPHeaders, headers...)
	}
	if params, err := s.hs.GetSearchParamsByHttpID(ctx, httpID); err == nil {
		bundle.HTTPSearchParams = append(bundle.HTTPSearchParams, params...)
	}
	if bodyRaw, err := s.hbr.GetByHttpID(ctx, httpID); err == nil && bodyRaw != nil {
		bundle.HTTPBodyRaw = append(bundle.HTTPBodyRaw, *bodyRaw)
	}
	if bodyForms, err := s.hs.GetBodyFormsByHttpID(ctx, httpID); err == nil {
		bundle.HTTPBodyForms = append(bundle.HTTPBodyForms, bodyForms...)
	}
	if bodyUrl, err := s.hs.GetBodyUrlEncodedByHttpID(ctx, httpID); err == nil {
		bundle.HTTPBodyUrlencoded = append(bundle.HTTPBodyUrlencoded, bodyUrl...)
	}
	if asserts, err := s.hs.GetAssertsByHttpID(ctx, httpID); err == nil {
		bundle.HTTPAsserts = append(bundle.HTTPAsserts, asserts...)
	}
}

// FlowNodesPaste parses YAML from clipboard and creates nodes in the target flow.
func (s *FlowServiceV2RPC) FlowNodesPaste(
	ctx context.Context,
	req *connect.Request[flowv1.FlowNodesPasteRequest],
) (*connect.Response[flowv1.FlowNodesPasteResponse], error) {
	if len(req.Msg.GetFlowId()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow id is required"))
	}
	if req.Msg.GetYaml() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("yaml is required"))
	}

	flowID, err := idwrap.NewFromBytes(req.Msg.GetFlowId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid flow id: %w", err))
	}

	if err := s.ensureFlowAccess(ctx, flowID); err != nil {
		return nil, err
	}

	targetFlow, err := s.fsReader.GetFlow(ctx, flowID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("flow not found: %w", err))
	}

	if err := s.ensureWorkspaceAccess(ctx, targetFlow.WorkspaceID); err != nil {
		return nil, err
	}

	// Parse the YAML
	opts := yamlflowsimplev2.GetDefaultOptions(targetFlow.WorkspaceID)
	opts.GenerateFiles = false // Don't create sidebar files for pasted nodes

	parsed, err := yamlflowsimplev2.ConvertSimplifiedYAML([]byte(req.Msg.GetYaml()), opts)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("failed to parse YAML: %w", err))
	}

	if len(parsed.FlowNodes) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("no nodes found in YAML"))
	}

	// Get existing node names in target flow for deduplication
	existingNodes, err := s.nsReader.GetNodesByFlowID(ctx, flowID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	existingNames := make(map[string]bool, len(existingNodes))
	for _, n := range existingNodes {
		existingNames[n.Name] = true
	}

	// For USE_EXISTING reference mode, look up existing HTTP requests by name
	referenceMode := req.Msg.GetReferenceMode()
	existingHTTPByName := make(map[string]*idwrap.IDWrap)
	if referenceMode == flowv1.ReferenceMode_REFERENCE_MODE_USE_EXISTING {
		existingHTTPs, err := s.hs.GetByWorkspaceID(ctx, targetFlow.WorkspaceID)
		if err == nil {
			for _, h := range existingHTTPs {
				id := h.ID
				existingHTTPByName[h.Name] = &id
			}
		}
	}

	// Apply offset and deduplicate names
	offsetX := float64(req.Msg.GetOffsetX())
	offsetY := float64(req.Msg.GetOffsetY())

	// Filter out ManualStart nodes first
	filteredNodes := parsed.FlowNodes[:0]
	for _, n := range parsed.FlowNodes {
		if n.NodeKind != mflow.NODE_KIND_MANUAL_START {
			filteredNodes = append(filteredNodes, n)
		}
	}
	parsed.FlowNodes = filteredNodes

	// Build node ID mapping (old parsed ID -> new ID) and name mapping for variable remapping
	nodeIDMapping := make(map[idwrap.IDWrap]idwrap.IDWrap, len(parsed.FlowNodes))
	nameMapping := make(map[string]string) // oldName -> newName (only for renamed nodes)
	for i := range parsed.FlowNodes {
		oldID := parsed.FlowNodes[i].ID
		newID := idwrap.NewNow()
		nodeIDMapping[oldID] = newID

		// Update the node
		parsed.FlowNodes[i].ID = newID
		parsed.FlowNodes[i].FlowID = flowID
		parsed.FlowNodes[i].PositionX += offsetX
		parsed.FlowNodes[i].PositionY += offsetY
		parsed.FlowNodes[i].State = mflow.NODE_STATE_UNSPECIFIED

		// Deduplicate names
		originalName := parsed.FlowNodes[i].Name
		name := originalName
		if existingNames[name] {
			counter := 1
			for existingNames[fmt.Sprintf("%s_%d", name, counter)] {
				counter++
			}
			name = fmt.Sprintf("%s_%d", name, counter)
			parsed.FlowNodes[i].Name = name
		}
		if name != originalName {
			nameMapping[originalName] = name
		}
		existingNames[name] = true
	}

	// Remap type-specific node IDs
	for i := range parsed.FlowRequestNodes {
		if newID, ok := nodeIDMapping[parsed.FlowRequestNodes[i].FlowNodeID]; ok {
			parsed.FlowRequestNodes[i].FlowNodeID = newID
		}
	}
	for i := range parsed.FlowConditionNodes {
		if newID, ok := nodeIDMapping[parsed.FlowConditionNodes[i].FlowNodeID]; ok {
			parsed.FlowConditionNodes[i].FlowNodeID = newID
		}
	}
	for i := range parsed.FlowForNodes {
		if newID, ok := nodeIDMapping[parsed.FlowForNodes[i].FlowNodeID]; ok {
			parsed.FlowForNodes[i].FlowNodeID = newID
		}
	}
	for i := range parsed.FlowForEachNodes {
		if newID, ok := nodeIDMapping[parsed.FlowForEachNodes[i].FlowNodeID]; ok {
			parsed.FlowForEachNodes[i].FlowNodeID = newID
		}
	}
	for i := range parsed.FlowJSNodes {
		if newID, ok := nodeIDMapping[parsed.FlowJSNodes[i].FlowNodeID]; ok {
			parsed.FlowJSNodes[i].FlowNodeID = newID
		}
	}
	for i := range parsed.FlowAINodes {
		if newID, ok := nodeIDMapping[parsed.FlowAINodes[i].FlowNodeID]; ok {
			parsed.FlowAINodes[i].FlowNodeID = newID
		}
	}
	for i := range parsed.FlowAIProviderNodes {
		if newID, ok := nodeIDMapping[parsed.FlowAIProviderNodes[i].FlowNodeID]; ok {
			parsed.FlowAIProviderNodes[i].FlowNodeID = newID
		}
	}
	for i := range parsed.FlowAIMemoryNodes {
		if newID, ok := nodeIDMapping[parsed.FlowAIMemoryNodes[i].FlowNodeID]; ok {
			parsed.FlowAIMemoryNodes[i].FlowNodeID = newID
		}
	}

	// Remap variable references in expression fields when node names changed
	if len(nameMapping) > 0 {
		for i := range parsed.FlowConditionNodes {
			parsed.FlowConditionNodes[i].Condition.Comparisons.Expression = remapVarRefs(
				parsed.FlowConditionNodes[i].Condition.Comparisons.Expression, nameMapping)
		}
		for i := range parsed.FlowForNodes {
			parsed.FlowForNodes[i].Condition.Comparisons.Expression = remapVarRefs(
				parsed.FlowForNodes[i].Condition.Comparisons.Expression, nameMapping)
		}
		for i := range parsed.FlowForEachNodes {
			parsed.FlowForEachNodes[i].IterExpression = remapVarRefs(
				parsed.FlowForEachNodes[i].IterExpression, nameMapping)
			parsed.FlowForEachNodes[i].Condition.Comparisons.Expression = remapVarRefs(
				parsed.FlowForEachNodes[i].Condition.Comparisons.Expression, nameMapping)
		}
		for i := range parsed.FlowJSNodes {
			parsed.FlowJSNodes[i].Code = remapAllRefsBytes(parsed.FlowJSNodes[i].Code, nameMapping)
		}
		for i := range parsed.FlowAINodes {
			parsed.FlowAINodes[i].Prompt = remapVarRefs(parsed.FlowAINodes[i].Prompt, nameMapping)
		}
		for i := range parsed.HTTPRequests {
			parsed.HTTPRequests[i].Url = remapVarRefs(parsed.HTTPRequests[i].Url, nameMapping)
		}
		for i := range parsed.HTTPHeaders {
			parsed.HTTPHeaders[i].Value = remapVarRefs(parsed.HTTPHeaders[i].Value, nameMapping)
		}
		for i := range parsed.HTTPSearchParams {
			parsed.HTTPSearchParams[i].Value = remapVarRefs(parsed.HTTPSearchParams[i].Value, nameMapping)
		}
		for i := range parsed.HTTPBodyRaw {
			parsed.HTTPBodyRaw[i].RawData = remapVarRefsBytes(parsed.HTTPBodyRaw[i].RawData, nameMapping)
		}
		for i := range parsed.HTTPBodyForms {
			parsed.HTTPBodyForms[i].Value = remapVarRefs(parsed.HTTPBodyForms[i].Value, nameMapping)
		}
		for i := range parsed.HTTPBodyUrlencoded {
			parsed.HTTPBodyUrlencoded[i].Value = remapVarRefs(parsed.HTTPBodyUrlencoded[i].Value, nameMapping)
		}
		for i := range parsed.HTTPAsserts {
			parsed.HTTPAsserts[i].Value = remapVarRefs(parsed.HTTPAsserts[i].Value, nameMapping)
		}
	}

	// Remap edges
	var validEdges []mflow.Edge
	for _, e := range parsed.FlowEdges {
		newSourceID, sourceOK := nodeIDMapping[e.SourceID]
		newTargetID, targetOK := nodeIDMapping[e.TargetID]
		if sourceOK && targetOK {
			e.ID = idwrap.NewNow()
			e.FlowID = flowID
			e.SourceID = newSourceID
			e.TargetID = newTargetID
			validEdges = append(validEdges, e)
		}
	}

	// Handle HTTP requests — resolve references based on referenceMode
	httpIDMapping := make(map[idwrap.IDWrap]idwrap.IDWrap) // parsed HTTP ID -> actual HTTP ID
	httpIDsToCreate := make(map[idwrap.IDWrap]bool)        // new HTTP IDs that need creation
	for i := range parsed.HTTPRequests {
		httpReq := &parsed.HTTPRequests[i]
		oldID := httpReq.ID
		if referenceMode == flowv1.ReferenceMode_REFERENCE_MODE_USE_EXISTING {
			if existingID, ok := existingHTTPByName[httpReq.Name]; ok {
				httpIDMapping[oldID] = *existingID
				continue
			}
		}
		// CREATE_COPY or not found: create new HTTP request
		newHTTPID := idwrap.NewNow()
		httpIDMapping[oldID] = newHTTPID
		httpReq.ID = newHTTPID
		httpReq.WorkspaceID = targetFlow.WorkspaceID
		httpReq.IsDelta = false
		httpReq.ParentHttpID = nil
		httpIDsToCreate[newHTTPID] = true
	}

	// Update request node HTTP references
	for i := range parsed.FlowRequestNodes {
		rn := &parsed.FlowRequestNodes[i]
		if rn.HttpID != nil {
			if newID, ok := httpIDMapping[*rn.HttpID]; ok {
				rn.HttpID = &newID
			}
		}
		// Clear delta reference — paste always uses resolved (base) requests
		rn.DeltaHttpID = nil
	}

	// Remap HTTP children's HttpID fields and filter to only those needing creation
	var headersToCreate []mhttp.HTTPHeader
	for i := range parsed.HTTPHeaders {
		h := &parsed.HTTPHeaders[i]
		if newID, ok := httpIDMapping[h.HttpID]; ok {
			h.HttpID = newID
			h.ID = idwrap.NewNow()
			h.IsDelta = false
			h.ParentHttpHeaderID = nil
			if httpIDsToCreate[newID] {
				headersToCreate = append(headersToCreate, *h)
			}
		}
	}
	var paramsToCreate []mhttp.HTTPSearchParam
	for i := range parsed.HTTPSearchParams {
		p := &parsed.HTTPSearchParams[i]
		if newID, ok := httpIDMapping[p.HttpID]; ok {
			p.HttpID = newID
			p.ID = idwrap.NewNow()
			p.IsDelta = false
			p.ParentHttpSearchParamID = nil
			if httpIDsToCreate[newID] {
				paramsToCreate = append(paramsToCreate, *p)
			}
		}
	}
	var bodyFormsToCreate []mhttp.HTTPBodyForm
	for i := range parsed.HTTPBodyForms {
		bf := &parsed.HTTPBodyForms[i]
		if newID, ok := httpIDMapping[bf.HttpID]; ok {
			bf.HttpID = newID
			bf.ID = idwrap.NewNow()
			bf.IsDelta = false
			bf.ParentHttpBodyFormID = nil
			if httpIDsToCreate[newID] {
				bodyFormsToCreate = append(bodyFormsToCreate, *bf)
			}
		}
	}
	var bodyUrlToCreate []mhttp.HTTPBodyUrlencoded
	for i := range parsed.HTTPBodyUrlencoded {
		bu := &parsed.HTTPBodyUrlencoded[i]
		if newID, ok := httpIDMapping[bu.HttpID]; ok {
			bu.HttpID = newID
			bu.ID = idwrap.NewNow()
			bu.IsDelta = false
			bu.ParentHttpBodyUrlEncodedID = nil
			if httpIDsToCreate[newID] {
				bodyUrlToCreate = append(bodyUrlToCreate, *bu)
			}
		}
	}
	var bodyRawToCreate []mhttp.HTTPBodyRaw
	for i := range parsed.HTTPBodyRaw {
		br := &parsed.HTTPBodyRaw[i]
		if newID, ok := httpIDMapping[br.HttpID]; ok {
			br.HttpID = newID
			br.ID = idwrap.NewNow()
			br.IsDelta = false
			br.ParentBodyRawID = nil
			if httpIDsToCreate[newID] {
				bodyRawToCreate = append(bodyRawToCreate, *br)
			}
		}
	}
	var assertsToCreate []mhttp.HTTPAssert
	for i := range parsed.HTTPAsserts {
		a := &parsed.HTTPAsserts[i]
		if newID, ok := httpIDMapping[a.HttpID]; ok {
			a.HttpID = newID
			a.ID = idwrap.NewNow()
			if httpIDsToCreate[newID] {
				assertsToCreate = append(assertsToCreate, *a)
			}
		}
	}

	// Begin transaction for creating all entities
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	hsWriter := shttp.NewWriter(tx)
	headerWriter := shttp.NewHeaderWriter(tx)
	paramWriter := shttp.NewSearchParamWriter(tx)
	bodyFormWriter := shttp.NewBodyFormWriter(tx)
	bodyUrlWriter := shttp.NewBodyUrlEncodedWriter(tx)
	bodyRawWriter := shttp.NewBodyRawWriter(tx)
	assertWriter := shttp.NewAssertWriter(tx)
	nsWriter := sflow.NewNodeWriter(tx)
	nrsWriter := sflow.NewNodeRequestWriter(tx)
	nfsWriter := sflow.NewNodeForWriter(tx)
	nfesWriter := sflow.NewNodeForEachWriter(tx)
	nifsWriter := sflow.NewNodeIfWriter(tx)
	njssWriter := sflow.NewNodeJsWriter(tx)
	esWriter := sflow.NewEdgeWriter(tx)

	// Create HTTP requests that need creation (not USE_EXISTING)
	for i := range parsed.HTTPRequests {
		if httpIDsToCreate[parsed.HTTPRequests[i].ID] {
			if err := hsWriter.Create(ctx, &parsed.HTTPRequests[i]); err != nil {
				return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create HTTP request: %w", err))
			}
		}
	}

	// Create HTTP children
	for i := range headersToCreate {
		if err := headerWriter.Create(ctx, &headersToCreate[i]); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create HTTP header: %w", err))
		}
	}
	for i := range paramsToCreate {
		if err := paramWriter.Create(ctx, &paramsToCreate[i]); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create HTTP search param: %w", err))
		}
	}
	for i := range bodyFormsToCreate {
		if err := bodyFormWriter.Create(ctx, &bodyFormsToCreate[i]); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create HTTP body form: %w", err))
		}
	}
	for i := range bodyUrlToCreate {
		if err := bodyUrlWriter.Create(ctx, &bodyUrlToCreate[i]); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create HTTP body urlencoded: %w", err))
		}
	}
	for i := range bodyRawToCreate {
		if _, err := bodyRawWriter.CreateFull(ctx, &bodyRawToCreate[i]); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create HTTP body raw: %w", err))
		}
	}
	for i := range assertsToCreate {
		if err := assertWriter.Create(ctx, &assertsToCreate[i]); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create HTTP assert: %w", err))
		}
	}

	// Create nodes
	var createdNodeIDs [][]byte
	for _, n := range parsed.FlowNodes {
		if err := nsWriter.CreateNode(ctx, n); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create node: %w", err))
		}
		createdNodeIDs = append(createdNodeIDs, n.ID.Bytes())
	}

	// Create type-specific node records
	for _, rn := range parsed.FlowRequestNodes {
		if err := nrsWriter.CreateNodeRequest(ctx, rn); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create request node: %w", err))
		}
	}
	for _, ifn := range parsed.FlowConditionNodes {
		if err := nifsWriter.CreateNodeIf(ctx, ifn); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create condition node: %w", err))
		}
	}
	for _, fn := range parsed.FlowForNodes {
		if err := nfsWriter.CreateNodeFor(ctx, fn); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create for node: %w", err))
		}
	}
	for _, fen := range parsed.FlowForEachNodes {
		if err := nfesWriter.CreateNodeForEach(ctx, fen); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create foreach node: %w", err))
		}
	}
	for _, jsn := range parsed.FlowJSNodes {
		if err := njssWriter.CreateNodeJS(ctx, jsn); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create js node: %w", err))
		}
	}
	if s.nais != nil {
		for _, ain := range parsed.FlowAINodes {
			naisWriter := s.nais.TX(tx)
			if err := naisWriter.CreateNodeAI(ctx, ain); err != nil {
				return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create ai node: %w", err))
			}
		}
	}
	if s.naps != nil {
		for _, apn := range parsed.FlowAIProviderNodes {
			napsWriter := s.naps.TX(tx)
			if err := napsWriter.CreateNodeAiProvider(ctx, apn); err != nil {
				return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create ai provider node: %w", err))
			}
		}
	}
	if s.nmems != nil {
		for _, mn := range parsed.FlowAIMemoryNodes {
			nmemsWriter := s.nmems.TX(tx)
			if err := nmemsWriter.CreateNodeMemory(ctx, mn); err != nil {
				return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create memory node: %w", err))
			}
		}
	}

	// Create edges
	for _, e := range validEdges {
		if err := esWriter.CreateEdge(ctx, e); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create edge: %w", err))
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to commit: %w", err))
	}

	// Publish events for sync
	for _, n := range parsed.FlowNodes {
		s.nodeStream.Publish(NodeTopic{FlowID: flowID}, NodeEvent{
			Type:   nodeEventInsert,
			FlowID: flowID,
			Node:   serializeNode(n),
		})
	}
	for _, e := range validEdges {
		s.edgeStream.Publish(EdgeTopic{FlowID: flowID}, EdgeEvent{
			Type:   edgeEventInsert,
			FlowID: flowID,
			Edge:   serializeEdge(e),
		})
	}

	// Publish HTTP events for newly created requests so the client's HttpCollectionSchema stays in sync
	for i := range parsed.HTTPRequests {
		if httpIDsToCreate[parsed.HTTPRequests[i].ID] {
			s.httpStream.Publish(rhttp.HttpTopic{WorkspaceID: targetFlow.WorkspaceID}, rhttp.HttpEvent{
				Type: eventTypeInsert,
				Http: converter.ToAPIHttp(parsed.HTTPRequests[i]),
			})
		}
	}

	return connect.NewResponse(&flowv1.FlowNodesPasteResponse{
		NodeIds: createdNodeIDs,
	}), nil
}

// remapVarRefs replaces node name references inside {{ }} variable expressions.
// For example, if nameMapping = {"GetUsers": "GetUsers_1"}, then
// "{{ GetUsers.response.body }}" becomes "{{ GetUsers_1.response.body }}".
func remapVarRefs(s string, nameMapping map[string]string) string {
	if len(nameMapping) == 0 || s == "" {
		return s
	}

	var result strings.Builder
	remaining := s

	for {
		startIdx := strings.Index(remaining, menv.Prefix)
		if startIdx == -1 {
			result.WriteString(remaining)
			break
		}

		endIdx := strings.Index(remaining[startIdx:], menv.Suffix)
		if endIdx == -1 {
			result.WriteString(remaining)
			break
		}

		// Write everything before this {{ block
		result.WriteString(remaining[:startIdx])

		// Extract the content between {{ and }}
		innerStart := startIdx + menv.PrefixSize
		innerEnd := startIdx + endIdx
		inner := remaining[innerStart:innerEnd]

		// Try to match a node name at the start of the inner content
		trimmedInner := strings.TrimSpace(inner)
		replaced := false
		for oldName, newName := range nameMapping {
			// Match "oldName.something" or "oldName" exactly
			if strings.HasPrefix(trimmedInner, oldName) {
				rest := trimmedInner[len(oldName):]
				if rest == "" || rest[0] == '.' {
					// Preserve original whitespace by replacing within the trimmed portion
					newInner := strings.Replace(inner, oldName, newName, 1)
					result.WriteString(menv.Prefix)
					result.WriteString(newInner)
					result.WriteString(menv.Suffix)
					replaced = true
					break
				}
			}
		}

		if !replaced {
			// Write the original {{ ... }} block unchanged
			result.WriteString(remaining[startIdx : startIdx+endIdx+menv.SuffixSize])
		}

		remaining = remaining[startIdx+endIdx+menv.SuffixSize:]
	}

	return result.String()
}

// remapVarRefsBytes is a convenience wrapper for []byte fields.
func remapVarRefsBytes(b []byte, nameMapping map[string]string) []byte {
	if len(nameMapping) == 0 || len(b) == 0 {
		return b
	}
	return []byte(remapVarRefs(string(b), nameMapping))
}

// remapJSBracketRefs replaces ["NodeName"] and ['NodeName'] references in JS code.
// The variable name before the bracket (ctx, context, etc.) doesn't matter —
// we match the bracket pattern directly since node names are known.
func remapJSBracketRefs(s string, nameMapping map[string]string) string {
	if len(nameMapping) == 0 || s == "" {
		return s
	}
	for oldName, newName := range nameMapping {
		s = strings.ReplaceAll(s, `["`+oldName+`"]`, `["`+newName+`"]`)
		s = strings.ReplaceAll(s, `['`+oldName+`']`, `['`+newName+`']`)
	}
	return s
}

// remapAllRefs applies both {{ }} variable remapping and JS ctx[] remapping.
func remapAllRefs(s string, nameMapping map[string]string) string {
	s = remapVarRefs(s, nameMapping)
	s = remapJSBracketRefs(s, nameMapping)
	return s
}

// remapAllRefsBytes is a convenience wrapper for []byte fields.
func remapAllRefsBytes(b []byte, nameMapping map[string]string) []byte {
	if len(nameMapping) == 0 || len(b) == 0 {
		return b
	}
	return []byte(remapAllRefs(string(b), nameMapping))
}

// FlowNodesPastePreview checks which HTTP requests from clipboard YAML already exist in the target workspace.
func (s *FlowServiceV2RPC) FlowNodesPastePreview(
	ctx context.Context,
	req *connect.Request[flowv1.FlowNodesPastePreviewRequest],
) (*connect.Response[flowv1.FlowNodesPastePreviewResponse], error) {
	if len(req.Msg.GetFlowId()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow id is required"))
	}
	if req.Msg.GetYaml() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("yaml is required"))
	}

	flowID, err := idwrap.NewFromBytes(req.Msg.GetFlowId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid flow id: %w", err))
	}

	if err := s.ensureFlowAccess(ctx, flowID); err != nil {
		return nil, err
	}

	targetFlow, err := s.fsReader.GetFlow(ctx, flowID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("flow not found: %w", err))
	}

	// Parse the YAML to extract request names
	var yamlFormat yamlflowsimplev2.YamlFlowFormatV2
	if err := yaml.Unmarshal([]byte(req.Msg.GetYaml()), &yamlFormat); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid YAML: %w", err))
	}

	// Collect request names from the YAML
	requestNames := make(map[string]bool)
	for _, r := range yamlFormat.Requests {
		if r.Name != "" {
			requestNames[r.Name] = true
		}
	}

	if len(requestNames) == 0 {
		return connect.NewResponse(&flowv1.FlowNodesPastePreviewResponse{}), nil
	}

	// Check which exist in the target workspace
	existingHTTPs, err := s.hs.GetByWorkspaceID(ctx, targetFlow.WorkspaceID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var existingRequests []string
	for _, h := range existingHTTPs {
		if requestNames[h.Name] {
			existingRequests = append(existingRequests, h.Name)
		}
	}

	return connect.NewResponse(&flowv1.FlowNodesPastePreviewResponse{
		ExistingRequests: existingRequests,
	}), nil
}
