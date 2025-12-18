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

	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/eventstream"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
)

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
			if n.NodeKind != mflow.NODE_KIND_JS {
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

		// Get node model to publish event later
		nodeModel, err := s.ns.GetNode(ctx, nodeID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get node: %w", err))
		}

		model := mflow.NodeJS{
			FlowNodeID:       nodeID,
			Code:             []byte(item.GetCode()),
			CodeCompressType: compress.CompressTypeNone,
		}

		if err := s.njss.CreateNodeJS(ctx, model); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Publish node event so NodeJsSync can pick up the code
		if nodeModel != nil {
			s.publishNodeEvent(nodeEventUpdate, *nodeModel)
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

		nodeModel, err := s.ensureNodeAccess(ctx, nodeID)
		if err != nil {
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

		// Publish node event so NodeJsSync can pick up the updated code
		s.publishNodeEvent(nodeEventUpdate, *nodeModel)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeJsDelete(ctx context.Context, req *connect.Request[flowv1.NodeJsDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		nodeModel, err := s.ensureNodeAccess(ctx, nodeID)
		if err != nil {
			return nil, err
		}

		if err := s.njss.DeleteNodeJS(ctx, nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Publish node event so NodeJsSync can pick up the deletion
		s.publishNodeEvent(nodeEventUpdate, *nodeModel)
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
				if nodeModel.NodeKind != mflow.NODE_KIND_JS {
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
