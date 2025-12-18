//nolint:revive // exported
package rflowv2

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"

	"connectrpc.com/connect"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

	"the-dev-tools/server/pkg/eventstream"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
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
			if node.NodeKind != mflow.NODE_KIND_NO_OP {
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

		noop := mflow.NodeNoop{
			FlowNodeID: nodeID,
			Type:       mflow.NoopTypes(item.GetKind()), // nolint:gosec // G115
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

		noop := mflow.NodeNoop{
			FlowNodeID: nodeID,
			Type:       mflow.NoopTypes(item.GetKind()), // nolint:gosec // G115
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
				if node.NodeKind != mflow.NODE_KIND_NO_OP {
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

func (s *FlowServiceV2RPC) publishNoOpEvent(eventType string, flowID idwrap.IDWrap, node mflow.NodeNoop) {
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
