package rflowv2

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"

	"connectrpc.com/connect"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/eventstream"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mflowvariable"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"the-dev-tools/server/pkg/model/mnnode/mnforeach"
	"the-dev-tools/server/pkg/model/mnnode/mnif"
	"the-dev-tools/server/pkg/model/mnnode/mnjs"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/service/sflowvariable"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
)

func (s *FlowServiceV2RPC) FlowCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[flowv1.FlowCollectionResponse], error) {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]*flowv1.Flow, 0, len(flows))
	for _, flow := range flows {
		items = append(items, serializeFlow(flow))
	}

	return connect.NewResponse(&flowv1.FlowCollectionResponse{Items: items}), nil
}

func (s *FlowServiceV2RPC) FlowSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[flowv1.FlowSyncResponse],
) error {
	if stream == nil {
		return connect.NewError(connect.CodeInternal, errors.New("stream is required"))
	}
	return s.streamFlowSync(ctx, func(resp *flowv1.FlowSyncResponse) error {
		return stream.Send(resp)
	})
}

func (s *FlowServiceV2RPC) streamFlowSync(
	ctx context.Context,
	send func(*flowv1.FlowSyncResponse) error,
) error {
	if s.flowStream == nil {
		return connect.NewError(connect.CodeUnavailable, errors.New("flow stream not configured"))
	}

	var workspaceSet sync.Map

	snapshot := func(ctx context.Context) ([]eventstream.Event[FlowTopic, FlowEvent], error) {
		flows, err := s.listAccessibleFlows(ctx)
		if err != nil {
			return nil, err
		}

		events := make([]eventstream.Event[FlowTopic, FlowEvent], 0)

		for _, flow := range flows {
			workspaceSet.Store(flow.WorkspaceID.String(), struct{}{})

			events = append(events, eventstream.Event[FlowTopic, FlowEvent]{
				Topic: FlowTopic{WorkspaceID: flow.WorkspaceID},
				Payload: FlowEvent{
					Type: flowEventInsert,
					Flow: serializeFlow(flow),
				},
			})
		}

		return events, nil
	}

	filter := func(topic FlowTopic) bool {
		if _, ok := workspaceSet.Load(topic.WorkspaceID.String()); ok {
			return true
		}
		if err := s.ensureWorkspaceAccess(ctx, topic.WorkspaceID); err != nil {
			return false
		}
		workspaceSet.Store(topic.WorkspaceID.String(), struct{}{})
		return true
	}

	events, err := s.flowStream.Subscribe(ctx, filter, eventstream.WithSnapshot(snapshot))
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	for {
		select {
		case evt, ok := <-events:
			if !ok {
				return nil
			}
			resp := flowEventToSyncResponse(evt.Payload)
			if resp == nil {
				continue
			}
			if err := send(resp); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (s *FlowServiceV2RPC) publishFlowEvent(eventType string, flow mflow.Flow) {
	if s.flowStream == nil {
		return
	}
	s.flowStream.Publish(FlowTopic{WorkspaceID: flow.WorkspaceID}, FlowEvent{
		Type: eventType,
		Flow: serializeFlow(flow),
	})
}

func flowEventToSyncResponse(evt FlowEvent) *flowv1.FlowSyncResponse {
	if evt.Flow == nil {
		return nil
	}

	var syncEvent *flowv1.FlowSync
	switch evt.Type {
	case flowEventInsert:
		insert := &flowv1.FlowSyncInsert{
			FlowId:  evt.Flow.FlowId,
			Name:    evt.Flow.Name,
			Running: evt.Flow.Running,
		}
		if evt.Flow.Duration != nil {
			insert.Duration = evt.Flow.Duration
		}
		syncEvent = &flowv1.FlowSync{
			Value: &flowv1.FlowSync_ValueUnion{
				Kind:   flowv1.FlowSync_ValueUnion_KIND_INSERT,
				Insert: insert,
			},
		}
	case flowEventUpdate:
		update := &flowv1.FlowSyncUpdate{
			FlowId:  evt.Flow.FlowId,
			Running: &evt.Flow.Running,
		}
		if evt.Flow.Name != "" {
			update.Name = &evt.Flow.Name
		}
		if evt.Flow.Duration != nil {
			update.Duration = &flowv1.FlowSyncUpdate_DurationUnion{
				Kind:  flowv1.FlowSyncUpdate_DurationUnion_KIND_VALUE,
				Value: evt.Flow.Duration,
			}
		}
		syncEvent = &flowv1.FlowSync{
			Value: &flowv1.FlowSync_ValueUnion{
				Kind:   flowv1.FlowSync_ValueUnion_KIND_UPDATE,
				Update: update,
			},
		}
	case flowEventDelete:
		syncEvent = &flowv1.FlowSync{
			Value: &flowv1.FlowSync_ValueUnion{
				Kind: flowv1.FlowSync_ValueUnion_KIND_DELETE,
				Delete: &flowv1.FlowSyncDelete{
					FlowId: evt.Flow.FlowId,
				},
			},
		}
	default:
		return nil
	}

	return &flowv1.FlowSyncResponse{
		Items: []*flowv1.FlowSync{syncEvent},
	}
}

func (s *FlowServiceV2RPC) FlowInsert(ctx context.Context, req *connect.Request[flowv1.FlowInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one flow is required"))
	}

	// Track workspaces to update their flow counts
	workspaceUpdates := make(map[idwrap.IDWrap]*mworkspace.Workspace)

	for _, item := range req.Msg.GetItems() {
		if len(item.GetWorkspaceId()) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace id is required"))
		}

		workspaceID, err := idwrap.NewFromBytes(item.GetWorkspaceId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid workspace id: %w", err))
		}

		if err := s.ensureWorkspaceAccess(ctx, workspaceID); err != nil {
			return nil, err
		}

		workspace, exists := workspaceUpdates[workspaceID]
		if !exists {
			workspace, err = s.ws.Get(ctx, workspaceID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			workspaceUpdates[workspaceID] = workspace
		}

		name := strings.TrimSpace(item.GetName())
		if name == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow name is required"))
		}

		flowID := idwrap.NewNow()
		if len(item.GetFlowId()) != 0 {
			flowID, err = idwrap.NewFromBytes(item.GetFlowId())
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid flow id: %w", err))
			}
		}

		flow := mflow.Flow{
			ID:          flowID,
			WorkspaceID: workspaceID,
			Name:        name,
		}

		if err := s.fs.CreateFlow(ctx, flow); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Seed start node so the flow is immediately runnable.
		startNodeID := idwrap.NewNow()
		startNode := mnnode.MNode{
			ID:        startNodeID,
			FlowID:    flowID,
			Name:      "Start",
			NodeKind:  mnnode.NODE_KIND_NO_OP,
			PositionX: 0,
			PositionY: 0,
		}
		if err := s.ns.CreateNode(ctx, startNode); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		startNoop := mnnoop.NoopNode{
			FlowNodeID: startNodeID,
			Type:       mnnoop.NODE_NO_OP_KIND_START,
		}
		if err := s.nnos.CreateNodeNoop(ctx, startNoop); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Publish node events for the start node so clients receive it via sync streams.
		// NoOp event must be published before the base node event so the client has the
		// sub-node data available when it receives and renders the base node.
		s.publishNoOpEvent(noopEventInsert, flowID, startNoop)
		s.publishNodeEvent(nodeEventInsert, startNode)

		if created, err := s.fs.GetFlow(ctx, flowID); err == nil {
			s.publishFlowEvent(flowEventInsert, created)
			if created.VersionParentID != nil {
				s.publishFlowVersionEvent(flowVersionEventInsert, created)
			}
		}

		workspace.FlowCount++
	}

	// Update all workspaces that had flows added
	for _, workspace := range workspaceUpdates {
		workspace.Updated = dbtime.DBNow()
		if err := s.ws.Update(ctx, workspace); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) FlowUpdate(ctx context.Context, req *connect.Request[flowv1.FlowUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		if len(item.GetFlowId()) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow id is required"))
		}

		flowID, err := idwrap.NewFromBytes(item.GetFlowId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid flow id: %w", err))
		}

		flow, err := s.fs.GetFlow(ctx, flowID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("flow %s not found", flowID.String()))
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := s.ensureFlowAccess(ctx, flowID); err != nil {
			return nil, err
		}

		if item.Name != nil {
			flow.Name = strings.TrimSpace(item.GetName())
			if flow.Name == "" {
				return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow name cannot be empty"))
			}
		}

		if du := item.GetDuration(); du != nil {
			switch du.GetKind() {
			case flowv1.FlowUpdate_DurationUnion_KIND_UNSET:
				flow.Duration = 0
			case flowv1.FlowUpdate_DurationUnion_KIND_VALUE:
				flow.Duration = du.GetValue()
			}
		}

		if err := s.fs.UpdateFlow(ctx, flow); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		s.publishFlowEvent(flowEventUpdate, flow)

		if flow.VersionParentID != nil {
			s.publishFlowVersionEvent(flowVersionEventUpdate, flow)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) FlowDelete(ctx context.Context, req *connect.Request[flowv1.FlowDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		flowID, err := idwrap.NewFromBytes(item.GetFlowId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid flow id: %w", err))
		}

		flow, err := s.fs.GetFlow(ctx, flowID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := s.ensureFlowAccess(ctx, flowID); err != nil {
			return nil, err
		}

		if err := s.fs.DeleteFlow(ctx, flowID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		s.publishFlowEvent(flowEventDelete, flow)

		if flow.VersionParentID != nil {
			s.publishFlowVersionEvent(flowVersionEventDelete, flow)
		}

		workspace, err := s.ws.Get(ctx, flow.WorkspaceID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if workspace.FlowCount > 0 {
			workspace.FlowCount--
		}
		workspace.Updated = dbtime.DBNow()
		if err := s.ws.Update(ctx, workspace); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) FlowDuplicate(ctx context.Context, req *connect.Request[flowv1.FlowDuplicateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetFlowId()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow id is required"))
	}

	sourceFlowID, err := idwrap.NewFromBytes(req.Msg.GetFlowId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid flow id: %w", err))
	}

	// Validate access to source flow
	if err := s.ensureFlowAccess(ctx, sourceFlowID); err != nil {
		return nil, err
	}

	// Get source flow details
	sourceFlow, err := s.fs.GetFlow(ctx, sourceFlowID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("flow %s not found", sourceFlowID.String()))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Get workspace access for creating the new flow
	if err := s.ensureWorkspaceAccess(ctx, sourceFlow.WorkspaceID); err != nil {
		return nil, err
	}

	// Get workspace to update flow count
	workspace, err := s.ws.Get(ctx, sourceFlow.WorkspaceID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Create new flow with duplicated name
	newFlowID := idwrap.NewNow()
	duplicatedName := fmt.Sprintf("Copy of %s", sourceFlow.Name)

	newFlow := mflow.Flow{
		ID:          newFlowID,
		WorkspaceID: sourceFlow.WorkspaceID,
		Name:        duplicatedName,
	}

	if err := s.fs.CreateFlow(ctx, newFlow); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Create a mapping from old node IDs to new node IDs for edge remapping
	nodeIDMapping := make(map[string]string)

	// Duplicate all nodes
	sourceNodes, err := s.ns.GetNodesByFlowID(ctx, sourceFlowID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for _, sourceNode := range sourceNodes {
		newNodeID := idwrap.NewNow()
		nodeIDMapping[sourceNode.ID.String()] = newNodeID.String()

		// Create the basic node
		newNode := mnnode.MNode{
			ID:        newNodeID,
			FlowID:    newFlowID,
			Name:      sourceNode.Name,
			NodeKind:  sourceNode.NodeKind,
			PositionX: sourceNode.PositionX,
			PositionY: sourceNode.PositionY,
		}

		if err := s.ns.CreateNode(ctx, newNode); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Duplicate node-type specific data
		switch sourceNode.NodeKind {
		case mnnode.NODE_KIND_NO_OP:
			noopData, err := s.nnos.GetNodeNoop(ctx, sourceNode.ID)
			if err == nil {
				newNoopData := mnnoop.NoopNode{
					FlowNodeID: newNodeID,
					Type:       noopData.Type,
				}
				if err := s.nnos.CreateNodeNoop(ctx, newNoopData); err != nil {
					return nil, connect.NewError(connect.CodeInternal, err)
				}
			}

		case mnnode.NODE_KIND_REQUEST:
			requestData, err := s.nrs.GetNodeRequest(ctx, sourceNode.ID)
			if err == nil && requestData.HttpID != nil {
				// Get the original HTTP data
				httpData, err := s.hs.Get(ctx, *requestData.HttpID)
				if err == nil {
					// Create a new HTTP record for the duplicated node
					newHttpID := idwrap.NewNow()
					duplicatedHttp := *httpData
					duplicatedHttp.ID = newHttpID
					duplicatedHttp.Name = fmt.Sprintf("Copy of %s", httpData.Name)

					if err := s.hs.Create(ctx, &duplicatedHttp); err != nil {
						return nil, connect.NewError(connect.CodeInternal, err)
					}

					newRequestData := mnrequest.MNRequest{
						FlowNodeID:       newNodeID,
						HttpID:           &newHttpID,
						HasRequestConfig: requestData.HasRequestConfig,
					}
					if err := s.nrs.CreateNodeRequest(ctx, newRequestData); err != nil {
						return nil, connect.NewError(connect.CodeInternal, err)
					}
				}
			}

		case mnnode.NODE_KIND_FOR:
			forData, err := s.nfs.GetNodeFor(ctx, sourceNode.ID)
			if err == nil {
				newForData := mnfor.MNFor{
					FlowNodeID:    newNodeID,
					IterCount:     forData.IterCount,
					Condition:     forData.Condition,
					ErrorHandling: forData.ErrorHandling,
				}
				if err := s.nfs.CreateNodeFor(ctx, newForData); err != nil {
					return nil, connect.NewError(connect.CodeInternal, err)
				}
			}

		case mnnode.NODE_KIND_FOR_EACH:
			forEachData, err := s.nfes.GetNodeForEach(ctx, sourceNode.ID)
			if err == nil {
				newForEachData := mnforeach.MNForEach{
					FlowNodeID:     newNodeID,
					IterExpression: forEachData.IterExpression,
					Condition:      forEachData.Condition,
					ErrorHandling:  forEachData.ErrorHandling,
				}
				if err := s.nfes.CreateNodeForEach(ctx, newForEachData); err != nil {
					return nil, connect.NewError(connect.CodeInternal, err)
				}
			}

		case mnnode.NODE_KIND_CONDITION:
			conditionData, err := s.nifs.GetNodeIf(ctx, sourceNode.ID)
			if err == nil {
				newConditionData := mnif.MNIF{
					FlowNodeID: newNodeID,
					Condition:  conditionData.Condition,
				}
				if err := s.nifs.CreateNodeIf(ctx, newConditionData); err != nil {
					return nil, connect.NewError(connect.CodeInternal, err)
				}
			}

		case mnnode.NODE_KIND_JS:
			jsData, err := s.njss.GetNodeJS(ctx, sourceNode.ID)
			if err == nil {
				newJsData := mnjs.MNJS{
					FlowNodeID: newNodeID,
					Code:       jsData.Code,
				}
				if err := s.njss.CreateNodeJS(ctx, newJsData); err != nil {
					return nil, connect.NewError(connect.CodeInternal, err)
				}
			}
		}
	}

	// Duplicate all edges with remapped node IDs
	sourceEdges, err := s.es.GetEdgesByFlowID(ctx, sourceFlowID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for _, sourceEdge := range sourceEdges {
		newEdgeID := idwrap.NewNow()

		// Map old node IDs to new node IDs
		newSourceIDStr, sourceOK := nodeIDMapping[sourceEdge.SourceID.String()]
		newTargetIDStr, targetOK := nodeIDMapping[sourceEdge.TargetID.String()]

		if !sourceOK || !targetOK {
			// This should not happen in normal circumstances, but skip invalid edges
			continue
		}

		newSourceID, err := idwrap.NewText(newSourceIDStr)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("invalid new source node id: %w", err))
		}

		newTargetID, err := idwrap.NewText(newTargetIDStr)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("invalid new target node id: %w", err))
		}

		newEdge := edge.Edge{
			ID:            newEdgeID,
			FlowID:        newFlowID,
			SourceID:      newSourceID,
			TargetID:      newTargetID,
			SourceHandler: sourceEdge.SourceHandler,
			Kind:          sourceEdge.Kind,
		}

		if err := s.es.CreateEdge(ctx, newEdge); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	// Duplicate all flow variables
	sourceVariables, err := s.fvs.GetFlowVariablesByFlowID(ctx, sourceFlowID)
	if err != nil && !errors.Is(err, sflowvariable.ErrNoFlowVariableFound) {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for _, sourceVariable := range sourceVariables {
		newVariableID := idwrap.NewNow()
		newVariable := mflowvariable.FlowVariable{
			ID:          newVariableID,
			FlowID:      newFlowID,
			Name:        sourceVariable.Name,
			Value:       sourceVariable.Value,
			Enabled:     sourceVariable.Enabled,
			Description: sourceVariable.Description,
		}

		if err := s.fvs.CreateFlowVariable(ctx, newVariable); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	// Update workspace flow count
	workspace.FlowCount++
	workspace.Updated = dbtime.DBNow()
	if err := s.ws.Update(ctx, workspace); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}
