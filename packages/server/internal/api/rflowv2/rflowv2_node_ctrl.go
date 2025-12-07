package rflowv2

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"

	"connectrpc.com/connect"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

	"the-dev-tools/server/internal/converter"
	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/eventstream"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"the-dev-tools/server/pkg/model/mnnode/mnforeach"
	"the-dev-tools/server/pkg/model/mnnode/mnif"
	"the-dev-tools/server/pkg/model/mnnode/mnjs"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
)

// --- NoOp Node ---

func (s *FlowServiceV2RPC) NodeNoOpCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[flowv1.NodeNoOpCollectionResponse], error) {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]*flowv1.NodeNoOp, 0)

	for _, flow := range flows {
		// Get all nodes in the flow and filter for NoOp nodes
		nodes, err := s.ns.GetNodesByFlowID(ctx, flow.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		for _, node := range nodes {
			// Only process NoOp nodes
			if node.NodeKind != mnnode.NODE_KIND_NO_OP {
				continue
			}

			// Get the NoOp configuration for this node
			noopNode, err := s.nnos.GetNodeNoop(ctx, node.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			if noopNode == nil {
				continue
			}

			items = append(items, serializeNodeNoop(*noopNode))
		}
	}

	return connect.NewResponse(&flowv1.NodeNoOpCollectionResponse{Items: items}), nil
}

func (s *FlowServiceV2RPC) NodeNoOpInsert(ctx context.Context, req *connect.Request[flowv1.NodeNoOpInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if err := s.nnos.DeleteNodeNoop(ctx, nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		noop := mnnoop.NoopNode{
			FlowNodeID: nodeID,
			Type:       mnnoop.NoopTypes(item.GetKind()), // nolint:gosec // G115
		}
		if err := s.nnos.CreateNodeNoop(ctx, noop); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Publish insert event if base node exists
		if baseNode, err := s.ns.GetNode(ctx, nodeID); err == nil {
			s.publishNoOpEvent(noopEventInsert, baseNode.FlowID, noop)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeNoOpUpdate(ctx context.Context, req *connect.Request[flowv1.NodeNoOpUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		baseNode, err := s.ensureNodeAccess(ctx, nodeID)
		if err != nil {
			return nil, err
		}

		if item.Kind == nil {
			continue
		}

		// Get existing NoOp node to publish delete event
		existingNoOp, err := s.nnos.GetNodeNoop(ctx, nodeID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := s.nnos.DeleteNodeNoop(ctx, nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		noop := mnnoop.NoopNode{
			FlowNodeID: nodeID,
			Type:       mnnoop.NoopTypes(item.GetKind()), // nolint:gosec // G115
		}
		if err := s.nnos.CreateNodeNoop(ctx, noop); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Publish events
		if existingNoOp != nil {
			s.publishNoOpEvent(noopEventDelete, baseNode.FlowID, *existingNoOp)
		}
		s.publishNoOpEvent(noopEventInsert, baseNode.FlowID, noop)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeNoOpDelete(ctx context.Context, req *connect.Request[flowv1.NodeNoOpDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		baseNode, err := s.ensureNodeAccess(ctx, nodeID)
		if err != nil {
			return nil, err
		}

		// Get existing NoOp node to publish delete event
		existingNoOp, err := s.nnos.GetNodeNoop(ctx, nodeID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := s.nnos.DeleteNodeNoop(ctx, nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Publish delete event
		if existingNoOp != nil {
			s.publishNoOpEvent(noopEventDelete, baseNode.FlowID, *existingNoOp)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeNoOpSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[flowv1.NodeNoOpSyncResponse],
) error {
	if stream == nil {
		return connect.NewError(connect.CodeInternal, errors.New("stream is required"))
	}
	return s.streamNoOpSync(ctx, func(resp *flowv1.NodeNoOpSyncResponse) error {
		return stream.Send(resp)
	})
}

func (s *FlowServiceV2RPC) streamNoOpSync(
	ctx context.Context,
	send func(*flowv1.NodeNoOpSyncResponse) error,
) error {
	if s.noopStream == nil {
		return connect.NewError(connect.CodeUnavailable, errors.New("noop stream not configured"))
	}

	var flowSet sync.Map

	snapshot := func(ctx context.Context) ([]eventstream.Event[NoOpTopic, NoOpEvent], error) {
		flows, err := s.listAccessibleFlows(ctx)
		if err != nil {
			return nil, err
		}

		events := make([]eventstream.Event[NoOpTopic, NoOpEvent], 0)

		for _, flow := range flows {
			flowSet.Store(flow.ID.String(), struct{}{})

			// Get all nodes in the flow and filter for NoOp nodes
			nodes, err := s.ns.GetNodesByFlowID(ctx, flow.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return nil, err
			}

			for _, node := range nodes {
				// Only process NoOp nodes
				if node.NodeKind != mnnode.NODE_KIND_NO_OP {
					continue
				}

				// Get the NoOp configuration for this node
				noopNode, err := s.nnos.GetNodeNoop(ctx, node.ID)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						continue
					}
					return nil, err
				}

				if noopNode == nil {
					continue
				}

				noopPB := serializeNodeNoop(*noopNode)
				events = append(events, eventstream.Event[NoOpTopic, NoOpEvent]{
					Topic: NoOpTopic{FlowID: flow.ID},
					Payload: NoOpEvent{
						Type:   noopEventInsert,
						FlowID: flow.ID,
						Node:   noopPB,
					},
				})
			}
		}

		return events, nil
	}

	filter := func(topic NoOpTopic) bool {
		if _, ok := flowSet.Load(topic.FlowID.String()); ok {
			return true
		}
		if err := s.ensureFlowAccess(ctx, topic.FlowID); err != nil {
			return false
		}
		flowSet.Store(topic.FlowID.String(), struct{}{})
		return true
	}

	events, err := s.noopStream.Subscribe(ctx, filter, eventstream.WithSnapshot(snapshot))
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	for {
		select {
		case evt, ok := <-events:
			if !ok {
				return nil
			}
			resp := noopEventToSyncResponse(evt.Payload)
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

func (s *FlowServiceV2RPC) publishNoOpEvent(eventType string, flowID idwrap.IDWrap, node mnnoop.NoopNode) {
	if s.noopStream == nil {
		return
	}

	nodePB := serializeNodeNoop(node)
	s.noopStream.Publish(NoOpTopic{FlowID: flowID}, NoOpEvent{
		Type:   eventType,
		FlowID: flowID,
		Node:   nodePB,
	})
}

func noopEventToSyncResponse(evt NoOpEvent) *flowv1.NodeNoOpSyncResponse {
	if evt.Node == nil {
		return nil
	}

	node := evt.Node

	switch evt.Type {
	case noopEventInsert:
		insert := &flowv1.NodeNoOpSyncInsert{
			NodeId: node.GetNodeId(),
			Kind:   node.GetKind(),
		}
		return &flowv1.NodeNoOpSyncResponse{
			Items: []*flowv1.NodeNoOpSync{{
				Value: &flowv1.NodeNoOpSync_ValueUnion{
					Kind:   flowv1.NodeNoOpSync_ValueUnion_KIND_INSERT,
					Insert: insert,
				},
			}},
		}
	case noopEventUpdate:
		update := &flowv1.NodeNoOpSyncUpdate{
			NodeId: node.GetNodeId(),
		}
		if kind := node.GetKind(); kind != flowv1.NodeNoOpKind_NODE_NO_OP_KIND_UNSPECIFIED {
			k := kind
			update.Kind = &k
		}
		return &flowv1.NodeNoOpSyncResponse{
			Items: []*flowv1.NodeNoOpSync{{
				Value: &flowv1.NodeNoOpSync_ValueUnion{
					Kind:   flowv1.NodeNoOpSync_ValueUnion_KIND_UPDATE,
					Update: update,
				},
			}},
		}
	case noopEventDelete:
		return &flowv1.NodeNoOpSyncResponse{
			Items: []*flowv1.NodeNoOpSync{{
				Value: &flowv1.NodeNoOpSync_ValueUnion{
					Kind: flowv1.NodeNoOpSync_ValueUnion_KIND_DELETE,
					Delete: &flowv1.NodeNoOpSyncDelete{
						NodeId: node.GetNodeId(),
					},
				},
			}},
		}
	default:
		return nil
	}
}

// --- For Node ---

func (s *FlowServiceV2RPC) NodeForCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[flowv1.NodeForCollectionResponse], error) {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]*flowv1.NodeFor, 0)

	for _, flow := range flows {
		nodes, err := s.ns.GetNodesByFlowID(ctx, flow.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, n := range nodes {
			if n.NodeKind != mnnode.NODE_KIND_FOR {
				continue
			}
			nodeFor, err := s.nfs.GetNodeFor(ctx, n.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			if nodeFor == nil {
				continue
			}
			items = append(items, serializeNodeFor(*nodeFor))
		}
	}

	return connect.NewResponse(&flowv1.NodeForCollectionResponse{Items: items}), nil
}

func (s *FlowServiceV2RPC) NodeForInsert(ctx context.Context, req *connect.Request[flowv1.NodeForInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		model := mnfor.MNFor{
			FlowNodeID:    nodeID,
			IterCount:     int64(item.GetIterations()),
			Condition:     buildCondition(item.GetCondition()),
			ErrorHandling: mnfor.ErrorHandling(item.GetErrorHandling()), // nolint:gosec // G115
		}

		if err := s.nfs.CreateNodeFor(ctx, model); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Publish insert event if base node exists
		if baseNode, err := s.ns.GetNode(ctx, nodeID); err == nil {
			s.publishForEvent(forEventInsert, baseNode.FlowID, model)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeForUpdate(ctx context.Context, req *connect.Request[flowv1.NodeForUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		baseNode, err := s.ensureNodeAccess(ctx, nodeID)
		if err != nil {
			return nil, err
		}

		existing, err := s.nfs.GetNodeFor(ctx, nodeID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("node %s does not have FOR config", nodeID.String()))
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if item.ErrorHandling != nil {
			existing.ErrorHandling = mnfor.ErrorHandling(item.GetErrorHandling()) // nolint:gosec // G115
		}

		if err := s.nfs.UpdateNodeFor(ctx, *existing); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Publish update event
		s.publishForEvent(forEventUpdate, baseNode.FlowID, *existing)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeForDelete(ctx context.Context, req *connect.Request[flowv1.NodeForDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		baseNode, err := s.ensureNodeAccess(ctx, nodeID)
		if err != nil {
			return nil, err
		}

		// Get existing For node to publish delete event
		existingFor, err := s.nfs.GetNodeFor(ctx, nodeID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := s.nfs.DeleteNodeFor(ctx, nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Publish delete event
		if existingFor != nil {
			s.publishForEvent(forEventDelete, baseNode.FlowID, *existingFor)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeForSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[flowv1.NodeForSyncResponse],
) error {
	if stream == nil {
		return connect.NewError(connect.CodeInternal, errors.New("stream is required"))
	}
	return s.streamNodeForSync(ctx, func(resp *flowv1.NodeForSyncResponse) error {
		return stream.Send(resp)
	})
}

func (s *FlowServiceV2RPC) streamNodeForSync(
	ctx context.Context,
	send func(*flowv1.NodeForSyncResponse) error,
) error {
	if s.forStream == nil {
		return connect.NewError(connect.CodeUnavailable, errors.New("for stream not configured"))
	}

	var flowSet sync.Map

	snapshot := func(ctx context.Context) ([]eventstream.Event[ForTopic, ForEvent], error) {
		flows, err := s.listAccessibleFlows(ctx)
		if err != nil {
			return nil, err
		}

		events := make([]eventstream.Event[ForTopic, ForEvent], 0)

		for _, flow := range flows {
			flowSet.Store(flow.ID.String(), struct{}{})

			// Get all nodes in the flow and filter for For nodes
			nodes, err := s.ns.GetNodesByFlowID(ctx, flow.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return nil, err
			}

			for _, node := range nodes {
				// Only process For nodes
				if node.NodeKind != mnnode.NODE_KIND_FOR {
					continue
				}

				// Skip start nodes (For nodes shouldn't be start nodes, but just in case)
				if isStartNode(node) {
					continue
				}

				// Get the For configuration for this node
				forNode, err := s.nfs.GetNodeFor(ctx, node.ID)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						continue
					}
					return nil, err
				}

				if forNode == nil {
					continue
				}

				forPB := serializeNodeFor(*forNode)
				events = append(events, eventstream.Event[ForTopic, ForEvent]{
					Topic: ForTopic{FlowID: flow.ID},
					Payload: ForEvent{
						Type:   forEventInsert,
						FlowID: flow.ID,
						Node:   forPB,
					},
				})
			}
		}

		return events, nil
	}

	filter := func(topic ForTopic) bool {
		if _, ok := flowSet.Load(topic.FlowID.String()); ok {
			return true
		}
		if err := s.ensureFlowAccess(ctx, topic.FlowID); err != nil {
			return false
		}
		flowSet.Store(topic.FlowID.String(), struct{}{})
		return true
	}

	events, err := s.forStream.Subscribe(ctx, filter, eventstream.WithSnapshot(snapshot))
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	for {
		select {
		case evt, ok := <-events:
			if !ok {
				return nil
			}
			resp := forEventToSyncResponse(evt.Payload)
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

func (s *FlowServiceV2RPC) publishForEvent(eventType string, flowID idwrap.IDWrap, node mnfor.MNFor) {
	if s.forStream == nil {
		return
	}

	nodePB := serializeNodeFor(node)
	s.forStream.Publish(ForTopic{FlowID: flowID}, ForEvent{
		Type:   eventType,
		FlowID: flowID,
		Node:   nodePB,
	})
}

func forEventToSyncResponse(evt ForEvent) *flowv1.NodeForSyncResponse {
	if evt.Node == nil {
		return nil
	}

	node := evt.Node

	switch evt.Type {
	case forEventInsert:
		insert := &flowv1.NodeForSyncInsert{
			NodeId:        node.GetNodeId(),
			Iterations:    node.GetIterations(),
			Condition:     node.GetCondition(),
			ErrorHandling: node.GetErrorHandling(),
		}
		return &flowv1.NodeForSyncResponse{
			Items: []*flowv1.NodeForSync{{
				Value: &flowv1.NodeForSync_ValueUnion{
					Kind:   flowv1.NodeForSync_ValueUnion_KIND_INSERT,
					Insert: insert,
				},
			}},
		}
	case forEventUpdate:
		update := &flowv1.NodeForSyncUpdate{
			NodeId: node.GetNodeId(),
		}
		if iterations := node.GetIterations(); iterations != 0 {
			update.Iterations = &iterations
		}
		if condition := node.GetCondition(); condition != "" {
			update.Condition = &condition
		}
		if errorHandling := node.GetErrorHandling(); errorHandling != flowv1.ErrorHandling_ERROR_HANDLING_UNSPECIFIED {
			update.ErrorHandling = &errorHandling
		}
		return &flowv1.NodeForSyncResponse{
			Items: []*flowv1.NodeForSync{{
				Value: &flowv1.NodeForSync_ValueUnion{
					Kind:   flowv1.NodeForSync_ValueUnion_KIND_UPDATE,
					Update: update,
				},
			}},
		}
	case forEventDelete:
		return &flowv1.NodeForSyncResponse{
			Items: []*flowv1.NodeForSync{{
				Value: &flowv1.NodeForSync_ValueUnion{
					Kind: flowv1.NodeForSync_ValueUnion_KIND_DELETE,
					Delete: &flowv1.NodeForSyncDelete{
						NodeId: node.GetNodeId(),
					},
				},
			}},
		}
	default:
		return nil
	}
}

// --- ForEach Node ---

func (s *FlowServiceV2RPC) NodeForEachCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[flowv1.NodeForEachCollectionResponse], error) {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]*flowv1.NodeForEach, 0)

	for _, flow := range flows {
		nodes, err := s.ns.GetNodesByFlowID(ctx, flow.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, n := range nodes {
			if n.NodeKind != mnnode.NODE_KIND_FOR_EACH {
				continue
			}
			nodeForEach, err := s.nfes.GetNodeForEach(ctx, n.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			if nodeForEach == nil {
				continue
			}
			items = append(items, serializeNodeForEach(*nodeForEach))
		}
	}

	return connect.NewResponse(&flowv1.NodeForEachCollectionResponse{Items: items}), nil
}

func (s *FlowServiceV2RPC) NodeForEachInsert(ctx context.Context, req *connect.Request[flowv1.NodeForEachInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		model := mnforeach.MNForEach{
			FlowNodeID:     nodeID,
			IterExpression: item.GetPath(),
			Condition:      buildCondition(item.GetCondition()),
			ErrorHandling:  mnfor.ErrorHandling(item.GetErrorHandling()), // nolint:gosec // G115
		}

		if err := s.nfes.CreateNodeForEach(ctx, model); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeForEachUpdate(ctx context.Context, req *connect.Request[flowv1.NodeForEachUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		existing, err := s.nfes.GetNodeForEach(ctx, nodeID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("node %s does not have FOREACH config", nodeID.String()))
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if item.Path != nil {
			existing.IterExpression = item.GetPath()
		}
		if item.Condition != nil {
			existing.Condition = buildCondition(item.GetCondition())
		}
		if item.ErrorHandling != nil {
			existing.ErrorHandling = mnfor.ErrorHandling(item.GetErrorHandling()) // nolint:gosec // G115
		}

		if err := s.nfes.UpdateNodeForEach(ctx, *existing); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeForEachDelete(ctx context.Context, req *connect.Request[flowv1.NodeForEachDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		if err := s.nfes.DeleteNodeForEach(ctx, nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeForEachSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[flowv1.NodeForEachSyncResponse],
) error {
	if stream == nil {
		return connect.NewError(connect.CodeInternal, errors.New("stream is required"))
	}
	return s.streamNodeForEachSync(ctx, func(resp *flowv1.NodeForEachSyncResponse) error {
		return stream.Send(resp)
	})
}

func (s *FlowServiceV2RPC) streamNodeForEachSync(
	ctx context.Context,
	send func(*flowv1.NodeForEachSyncResponse) error,
) error {
	if s.forEachStream == nil {
		return connect.NewError(connect.CodeUnavailable, errors.New("forEach stream not configured"))
	}

	var flowSet sync.Map

	snapshot := func(ctx context.Context) ([]eventstream.Event[NodeTopic, NodeEvent], error) {
		flows, err := s.listAccessibleFlows(ctx)
		if err != nil {
			return nil, err
		}

		events := make([]eventstream.Event[NodeTopic, NodeEvent], 0)

		for _, flow := range flows {
			flowSet.Store(flow.ID.String(), struct{}{})

			nodes, err := s.ns.GetNodesByFlowID(ctx, flow.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return nil, err
			}

			for _, nodeModel := range nodes {
				// Filter for ForEach nodes
				if nodeModel.NodeKind != mnnode.NODE_KIND_FOR_EACH {
					continue
				}

				nodeForEach, err := s.nfes.GetNodeForEach(ctx, nodeModel.ID)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						continue
					}
					return nil, err
				}
				if nodeForEach == nil {
					continue
				}

				// Create a custom NodeEvent that includes ForEach node data
				events = append(events, eventstream.Event[NodeTopic, NodeEvent]{
					Topic: NodeTopic{FlowID: flow.ID},
					Payload: NodeEvent{
						Type:   nodeEventInsert,
						FlowID: flow.ID,
						Node: &flowv1.Node{
							NodeId: nodeForEach.FlowNodeID.Bytes(),
							Kind:   flowv1.NodeKind_NODE_KIND_FOR_EACH,
						},
					},
				})
			}
		}

		return events, nil
	}

	filter := func(topic NodeTopic) bool {
		if _, ok := flowSet.Load(topic.FlowID.String()); ok {
			return true
		}
		if err := s.ensureFlowAccess(ctx, topic.FlowID); err != nil {
			return false
		}
		flowSet.Store(topic.FlowID.String(), struct{}{})
		return true
	}

	events, err := s.nodeStream.Subscribe(ctx, filter, eventstream.WithSnapshot(snapshot))
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	for {
		select {
		case evt, ok := <-events:
			if !ok {
				return nil
			}
			resp, err := s.forEachEventToSyncResponse(ctx, evt.Payload)
			if err != nil {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to convert ForEach node event: %w", err))
			}
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

func (s *FlowServiceV2RPC) forEachEventToSyncResponse(
	ctx context.Context,
	evt NodeEvent,
) (*flowv1.NodeForEachSyncResponse, error) {
	if evt.Node == nil {
		return nil, nil
	}

	// Only process ForEach nodes
	if evt.Node.GetKind() != flowv1.NodeKind_NODE_KIND_FOR_EACH {
		return nil, nil
	}

	nodeID, err := idwrap.NewFromBytes(evt.Node.GetNodeId())
	if err != nil {
		return nil, fmt.Errorf("invalid node id: %w", err)
	}

	// Fetch the ForEach configuration for this node
	nodeForEach, err := s.nfes.GetNodeForEach(ctx, nodeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Node exists but doesn't have ForEach config, skip
			return nil, nil
		}
		return nil, err
	}
	if nodeForEach == nil {
		return nil, nil
	}

	var syncEvent *flowv1.NodeForEachSync
	switch evt.Type {
	case nodeEventInsert:
		syncEvent = &flowv1.NodeForEachSync{
			Value: &flowv1.NodeForEachSync_ValueUnion{
				Kind: flowv1.NodeForEachSync_ValueUnion_KIND_INSERT,
				Insert: &flowv1.NodeForEachSyncInsert{
					NodeId:        nodeForEach.FlowNodeID.Bytes(),
					Path:          nodeForEach.IterExpression,
					Condition:     nodeForEach.Condition.Comparisons.Expression,
					ErrorHandling: converter.ToAPIErrorHandling(nodeForEach.ErrorHandling),
				},
			},
		}
	case nodeEventUpdate:
		syncEvent = &flowv1.NodeForEachSync{
			Value: &flowv1.NodeForEachSync_ValueUnion{
				Kind: flowv1.NodeForEachSync_ValueUnion_KIND_UPDATE,
				Update: &flowv1.NodeForEachSyncUpdate{
					NodeId: nodeForEach.FlowNodeID.Bytes(),
				},
			},
		}
	case nodeEventDelete:
		syncEvent = &flowv1.NodeForEachSync{
			Value: &flowv1.NodeForEachSync_ValueUnion{
				Kind: flowv1.NodeForEachSync_ValueUnion_KIND_DELETE,
				Delete: &flowv1.NodeForEachSyncDelete{
					NodeId: nodeForEach.FlowNodeID.Bytes(),
				},
			},
		}
	default:
		return nil, nil
	}

	return &flowv1.NodeForEachSyncResponse{
		Items: []*flowv1.NodeForEachSync{syncEvent},
	}, nil
}

// --- Condition Node ---

func (s *FlowServiceV2RPC) NodeConditionCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[flowv1.NodeConditionCollectionResponse], error) {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]*flowv1.NodeCondition, 0)

	for _, flow := range flows {
		nodes, err := s.ns.GetNodesByFlowID(ctx, flow.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, n := range nodes {
			if n.NodeKind != mnnode.NODE_KIND_CONDITION {
				continue
			}
			nodeCondition, err := s.nifs.GetNodeIf(ctx, n.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			if nodeCondition == nil {
				continue
			}
			items = append(items, serializeNodeCondition(*nodeCondition))
		}
	}

	return connect.NewResponse(&flowv1.NodeConditionCollectionResponse{Items: items}), nil
}

func (s *FlowServiceV2RPC) NodeConditionInsert(ctx context.Context, req *connect.Request[flowv1.NodeConditionInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		model := mnif.MNIF{
			FlowNodeID: nodeID,
			Condition:  buildCondition(item.GetCondition()),
		}

		if err := s.nifs.CreateNodeIf(ctx, model); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeConditionUpdate(ctx context.Context, req *connect.Request[flowv1.NodeConditionUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		existing, err := s.nifs.GetNodeIf(ctx, nodeID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("node %s does not have CONDITION config", nodeID.String()))
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if item.Condition != nil {
			existing.Condition = buildCondition(item.GetCondition())
		}

		if err := s.nifs.UpdateNodeIf(ctx, *existing); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeConditionDelete(ctx context.Context, req *connect.Request[flowv1.NodeConditionDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		if err := s.nifs.DeleteNodeIf(ctx, nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeConditionSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[flowv1.NodeConditionSyncResponse],
) error {
	if stream == nil {
		return connect.NewError(connect.CodeInternal, errors.New("stream is required"))
	}
	return s.streamNodeConditionSync(ctx, func(resp *flowv1.NodeConditionSyncResponse) error {
		return stream.Send(resp)
	})
}

func (s *FlowServiceV2RPC) streamNodeConditionSync(
	ctx context.Context,
	send func(*flowv1.NodeConditionSyncResponse) error,
) error {
	if s.conditionStream == nil {
		return connect.NewError(connect.CodeUnavailable, errors.New("condition stream not configured"))
	}

	var flowSet sync.Map

	snapshot := func(ctx context.Context) ([]eventstream.Event[NodeTopic, NodeEvent], error) {
		flows, err := s.listAccessibleFlows(ctx)
		if err != nil {
			return nil, err
		}

		events := make([]eventstream.Event[NodeTopic, NodeEvent], 0)

		for _, flow := range flows {
			flowSet.Store(flow.ID.String(), struct{}{})

			nodes, err := s.ns.GetNodesByFlowID(ctx, flow.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return nil, err
			}

			for _, nodeModel := range nodes {
				// Filter for Condition nodes
				if nodeModel.NodeKind != mnnode.NODE_KIND_CONDITION {
					continue
				}

				nodeCondition, err := s.nifs.GetNodeIf(ctx, nodeModel.ID)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						continue
					}
					return nil, err
				}
				if nodeCondition == nil {
					continue
				}

				// Create a custom NodeEvent that includes Condition node data
				events = append(events, eventstream.Event[NodeTopic, NodeEvent]{
					Topic: NodeTopic{FlowID: flow.ID},
					Payload: NodeEvent{
						Type:   nodeEventInsert,
						FlowID: flow.ID,
						Node: &flowv1.Node{
							NodeId: nodeCondition.FlowNodeID.Bytes(),
							Kind:   flowv1.NodeKind_NODE_KIND_CONDITION,
						},
					},
				})
			}
		}

		return events, nil
	}

	filter := func(topic NodeTopic) bool {
		if _, ok := flowSet.Load(topic.FlowID.String()); ok {
			return true
		}
		if err := s.ensureFlowAccess(ctx, topic.FlowID); err != nil {
			return false
		}
		flowSet.Store(topic.FlowID.String(), struct{}{})
		return true
	}

	events, err := s.nodeStream.Subscribe(ctx, filter, eventstream.WithSnapshot(snapshot))
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	for {
		select {
		case evt, ok := <-events:
			if !ok {
				return nil
			}
			resp, err := s.conditionEventToSyncResponse(ctx, evt.Payload)
			if err != nil {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to convert Condition node event: %w", err))
			}
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

func (s *FlowServiceV2RPC) conditionEventToSyncResponse(
	ctx context.Context,
	evt NodeEvent,
) (*flowv1.NodeConditionSyncResponse, error) {
	if evt.Node == nil {
		return nil, nil
	}

	// Only process Condition nodes
	if evt.Node.GetKind() != flowv1.NodeKind_NODE_KIND_CONDITION {
		return nil, nil
	}

	nodeID, err := idwrap.NewFromBytes(evt.Node.GetNodeId())
	if err != nil {
		return nil, fmt.Errorf("invalid node id: %w", err)
	}

	// Fetch the Condition configuration for this node
	nodeCondition, err := s.nifs.GetNodeIf(ctx, nodeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Node exists but doesn't have Condition config, skip
			return nil, nil
		}
		return nil, err
	}
	if nodeCondition == nil {
		return nil, nil
	}

	var syncEvent *flowv1.NodeConditionSync
	switch evt.Type {
	case nodeEventInsert:
		syncEvent = &flowv1.NodeConditionSync{
			Value: &flowv1.NodeConditionSync_ValueUnion{
				Kind: flowv1.NodeConditionSync_ValueUnion_KIND_INSERT,
				Insert: &flowv1.NodeConditionSyncInsert{
					NodeId:    nodeCondition.FlowNodeID.Bytes(),
					Condition: nodeCondition.Condition.Comparisons.Expression,
				},
			},
		}
	case nodeEventUpdate:
		syncEvent = &flowv1.NodeConditionSync{
			Value: &flowv1.NodeConditionSync_ValueUnion{
				Kind: flowv1.NodeConditionSync_ValueUnion_KIND_UPDATE,
				Update: &flowv1.NodeConditionSyncUpdate{
					NodeId: nodeCondition.FlowNodeID.Bytes(),
				},
			},
		}
	case nodeEventDelete:
		syncEvent = &flowv1.NodeConditionSync{
			Value: &flowv1.NodeConditionSync_ValueUnion{
				Kind: flowv1.NodeConditionSync_ValueUnion_KIND_DELETE,
				Delete: &flowv1.NodeConditionSyncDelete{
					NodeId: nodeCondition.FlowNodeID.Bytes(),
				},
			},
		}
	default:
		return nil, nil
	}

	return &flowv1.NodeConditionSyncResponse{
		Items: []*flowv1.NodeConditionSync{syncEvent},
	}, nil
}

// --- JS Node ---

func (s *FlowServiceV2RPC) NodeJsCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[flowv1.NodeJsCollectionResponse], error) {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]*flowv1.NodeJs, 0)

	for _, flow := range flows {
		nodes, err := s.ns.GetNodesByFlowID(ctx, flow.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, n := range nodes {
			if n.NodeKind != mnnode.NODE_KIND_JS {
				continue
			}
			nodeJs, err := s.njss.GetNodeJS(ctx, n.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			items = append(items, serializeNodeJs(nodeJs))
		}
	}

	return connect.NewResponse(&flowv1.NodeJsCollectionResponse{Items: items}), nil
}

func (s *FlowServiceV2RPC) NodeJsInsert(ctx context.Context, req *connect.Request[flowv1.NodeJsInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		model := mnjs.MNJS{
			FlowNodeID:       nodeID,
			Code:             []byte(item.GetCode()),
			CodeCompressType: compress.CompressTypeNone,
		}

		if err := s.njss.CreateNodeJS(ctx, model); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeJsUpdate(ctx context.Context, req *connect.Request[flowv1.NodeJsUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		existing, err := s.njss.GetNodeJS(ctx, nodeID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("node %s does not have JS config", nodeID.String()))
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if item.Code != nil {
			existing.Code = []byte(item.GetCode())
		}

		if err := s.njss.UpdateNodeJS(ctx, existing); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeJsDelete(ctx context.Context, req *connect.Request[flowv1.NodeJsDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		if err := s.njss.DeleteNodeJS(ctx, nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeJsSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[flowv1.NodeJsSyncResponse],
) error {
	if stream == nil {
		return connect.NewError(connect.CodeInternal, errors.New("stream is required"))
	}
	return s.streamNodeJsSync(ctx, func(resp *flowv1.NodeJsSyncResponse) error {
		return stream.Send(resp)
	})
}

func (s *FlowServiceV2RPC) streamNodeJsSync(
	ctx context.Context,
	send func(*flowv1.NodeJsSyncResponse) error,
) error {
	if s.jsStream == nil {
		return connect.NewError(connect.CodeUnavailable, errors.New("js stream not configured"))
	}

	var flowSet sync.Map

	snapshot := func(ctx context.Context) ([]eventstream.Event[NodeTopic, NodeEvent], error) {
		flows, err := s.listAccessibleFlows(ctx)
		if err != nil {
			return nil, err
		}

		events := make([]eventstream.Event[NodeTopic, NodeEvent], 0)

		for _, flow := range flows {
			flowSet.Store(flow.ID.String(), struct{}{})

			nodes, err := s.ns.GetNodesByFlowID(ctx, flow.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return nil, err
			}

			for _, nodeModel := range nodes {
				// Filter for JS nodes
				if nodeModel.NodeKind != mnnode.NODE_KIND_JS {
					continue
				}

				nodeJs, err := s.njss.GetNodeJS(ctx, nodeModel.ID)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						continue
					}
					return nil, err
				}

				// Create a custom NodeEvent that includes JS node data
				events = append(events, eventstream.Event[NodeTopic, NodeEvent]{
					Topic: NodeTopic{FlowID: flow.ID},
					Payload: NodeEvent{
						Type:   nodeEventInsert,
						FlowID: flow.ID,
						Node: &flowv1.Node{
							NodeId: nodeJs.FlowNodeID.Bytes(),
							Kind:   flowv1.NodeKind_NODE_KIND_JS,
						},
					},
				})
			}
		}

		return events, nil
	}

	filter := func(topic NodeTopic) bool {
		if _, ok := flowSet.Load(topic.FlowID.String()); ok {
			return true
		}
		if err := s.ensureFlowAccess(ctx, topic.FlowID); err != nil {
			return false
		}
		flowSet.Store(topic.FlowID.String(), struct{}{})
		return true
	}

	events, err := s.nodeStream.Subscribe(ctx, filter, eventstream.WithSnapshot(snapshot))
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	for {
		select {
		case evt, ok := <-events:
			if !ok {
				return nil
			}
			resp, err := s.jsEventToSyncResponse(ctx, evt.Payload)
			if err != nil {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to convert JS node event: %w", err))
			}
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

func (s *FlowServiceV2RPC) publishJsEvent(eventType string, flowID idwrap.IDWrap, node mnjs.MNJS) {
	if s.jsStream == nil {
		return
	}

	nodePB := serializeNodeJs(node)
	s.jsStream.Publish(JsTopic{FlowID: flowID}, JsEvent{
		Type:   eventType,
		FlowID: flowID,
		Node:   nodePB,
	})
}

func (s *FlowServiceV2RPC) jsEventToSyncResponse(
	ctx context.Context,
	evt NodeEvent,
) (*flowv1.NodeJsSyncResponse, error) {
	if evt.Node == nil {
		return nil, nil
	}

	// Only process JS nodes
	if evt.Node.GetKind() != flowv1.NodeKind_NODE_KIND_JS {
		return nil, nil
	}

	nodeID, err := idwrap.NewFromBytes(evt.Node.GetNodeId())
	if err != nil {
		return nil, fmt.Errorf("invalid node id: %w", err)
	}

	// Fetch the JavaScript configuration for this node
	nodeJs, err := s.njss.GetNodeJS(ctx, nodeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Node exists but doesn't have JS config, skip
			return nil, nil
		}
		return nil, err
	}

	var syncEvent *flowv1.NodeJsSync
	switch evt.Type {
	case nodeEventInsert:
		syncEvent = &flowv1.NodeJsSync{
			Value: &flowv1.NodeJsSync_ValueUnion{
				Kind: flowv1.NodeJsSync_ValueUnion_KIND_INSERT,
				Insert: &flowv1.NodeJsSyncInsert{
					NodeId: nodeJs.FlowNodeID.Bytes(),
					Code:   string(nodeJs.Code),
				},
			},
		}
	case nodeEventUpdate:
		syncEvent = &flowv1.NodeJsSync{
			Value: &flowv1.NodeJsSync_ValueUnion{
				Kind: flowv1.NodeJsSync_ValueUnion_KIND_UPDATE,
				Update: &flowv1.NodeJsSyncUpdate{
					NodeId: nodeJs.FlowNodeID.Bytes(),
				},
			},
		}
	case nodeEventDelete:
		syncEvent = &flowv1.NodeJsSync{
			Value: &flowv1.NodeJsSync_ValueUnion{
				Kind: flowv1.NodeJsSync_ValueUnion_KIND_DELETE,
				Delete: &flowv1.NodeJsSyncDelete{
					NodeId: nodeJs.FlowNodeID.Bytes(),
				},
			},
		}
	default:
		return nil, nil
	}

	return &flowv1.NodeJsSyncResponse{
		Items: []*flowv1.NodeJsSync{syncEvent},
	}, nil
}
