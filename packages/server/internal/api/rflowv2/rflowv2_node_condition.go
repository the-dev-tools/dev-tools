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
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnif"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
)

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
