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

	"the-dev-tools/server/internal/converter"
	"the-dev-tools/server/pkg/eventstream"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"the-dev-tools/server/pkg/model/mnnode/mnforeach"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
)

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

		// Get node model to publish event later
		nodeModel, err := s.ns.GetNode(ctx, nodeID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get node: %w", err))
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

		// Publish node event so NodeForEachSync can pick up the config
		if nodeModel != nil {
			s.publishNodeEvent(nodeEventUpdate, *nodeModel)
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

		nodeModel, err := s.ensureNodeAccess(ctx, nodeID)
		if err != nil {
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

		// Publish node event so NodeForEachSync can pick up the updated config
		s.publishNodeEvent(nodeEventUpdate, *nodeModel)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeForEachDelete(ctx context.Context, req *connect.Request[flowv1.NodeForEachDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		nodeModel, err := s.ensureNodeAccess(ctx, nodeID)
		if err != nil {
			return nil, err
		}

		if err := s.nfes.DeleteNodeForEach(ctx, nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Publish node event so NodeForEachSync can pick up the deletion
		s.publishNodeEvent(nodeEventUpdate, *nodeModel)
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
