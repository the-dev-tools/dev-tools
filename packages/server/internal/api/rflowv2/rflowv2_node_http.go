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
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
)

func (s *FlowServiceV2RPC) NodeHttpCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[flowv1.NodeHttpCollectionResponse], error) {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]*flowv1.NodeHttp, 0)

	for _, flow := range flows {
		nodes, err := s.ns.GetNodesByFlowID(ctx, flow.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, n := range nodes {
			if n.NodeKind != mnnode.NODE_KIND_REQUEST {
				continue
			}
			nodeReq, err := s.nrs.GetNodeRequest(ctx, n.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			if nodeReq == nil || nodeReq.HttpID == nil || isZeroID(*nodeReq.HttpID) {
				continue
			}
			items = append(items, serializeNodeHTTP(*nodeReq))
		}
	}

	return connect.NewResponse(&flowv1.NodeHttpCollectionResponse{Items: items}), nil
}

func (s *FlowServiceV2RPC) NodeHttpInsert(ctx context.Context, req *connect.Request[flowv1.NodeHttpInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		var httpID idwrap.IDWrap
		if len(item.GetHttpId()) > 0 {
			httpID, err = idwrap.NewFromBytes(item.GetHttpId())
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid http id: %w", err))
			}
		}

		if err := s.nrs.CreateNodeRequest(ctx, mnrequest.MNRequest{
			FlowNodeID:       nodeID,
			HttpID:           &httpID,
			HasRequestConfig: !isZeroID(httpID),
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeHttpUpdate(ctx context.Context, req *connect.Request[flowv1.NodeHttpUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		var httpID idwrap.IDWrap
		union := item.GetHttpId()
		if union != nil && union.Kind == flowv1.NodeHttpUpdate_HttpIdUnion_KIND_VALUE {
			httpID, err = idwrap.NewFromBytes(union.GetValue())
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid http id: %w", err))
			}
		}

		if err := s.nrs.UpdateNodeRequest(ctx, mnrequest.MNRequest{
			FlowNodeID:       nodeID,
			HttpID:           &httpID,
			HasRequestConfig: !isZeroID(httpID),
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeHttpDelete(ctx context.Context, req *connect.Request[flowv1.NodeHttpDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		if err := s.nrs.DeleteNodeRequest(ctx, nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeHttpSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[flowv1.NodeHttpSyncResponse],
) error {
	if stream == nil {
		return connect.NewError(connect.CodeInternal, errors.New("stream is required"))
	}
	return s.streamNodeHttpSync(ctx, func(resp *flowv1.NodeHttpSyncResponse) error {
		return stream.Send(resp)
	})
}

func (s *FlowServiceV2RPC) streamNodeHttpSync(
	ctx context.Context,
	send func(*flowv1.NodeHttpSyncResponse) error,
) error {
	if s.nodeStream == nil {
		return connect.NewError(connect.CodeUnavailable, errors.New("node stream not configured"))
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
				// Filter for HTTP nodes (REQUEST nodes)
				if nodeModel.NodeKind != mnnode.NODE_KIND_REQUEST {
					continue
				}

				nodeReq, err := s.nrs.GetNodeRequest(ctx, nodeModel.ID)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						continue
					}
					return nil, err
				}
				if nodeReq == nil || nodeReq.HttpID == nil || isZeroID(*nodeReq.HttpID) {
					continue
				}

				// Create a custom NodeEvent that includes HTTP node data
				events = append(events, eventstream.Event[NodeTopic, NodeEvent]{
					Topic: NodeTopic{FlowID: flow.ID},
					Payload: NodeEvent{
						Type:   nodeEventInsert,
						FlowID: flow.ID,
						Node:   serializeNode(nodeModel), // Pass regular node for compatibility
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
			resp, err := s.nodeHttpEventToSyncResponse(ctx, evt.Payload)
			if err != nil {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to convert HTTP node event: %w", err))
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

func (s *FlowServiceV2RPC) nodeHttpEventToSyncResponse(
	ctx context.Context,
	evt NodeEvent,
) (*flowv1.NodeHttpSyncResponse, error) {
	if evt.Node == nil {
		return nil, nil
	}

	// Only process HTTP nodes (REQUEST nodes)
	if evt.Node.GetKind() != flowv1.NodeKind_NODE_KIND_HTTP {
		return nil, nil
	}

	nodeID, err := idwrap.NewFromBytes(evt.Node.GetNodeId())
	if err != nil {
		return nil, fmt.Errorf("invalid node id: %w", err)
	}

	// Fetch the HTTP configuration for this node
	nodeReq, err := s.nrs.GetNodeRequest(ctx, nodeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Node exists but doesn't have HTTP config, skip
			return nil, nil
		}
		return nil, err
	}
	if nodeReq == nil || nodeReq.HttpID == nil || isZeroID(*nodeReq.HttpID) {
		return nil, nil
	}

	var syncEvent *flowv1.NodeHttpSync
	switch evt.Type {
	case nodeEventInsert:
		insert := &flowv1.NodeHttpSyncInsert{
			NodeId: nodeReq.FlowNodeID.Bytes(),
			HttpId: nodeReq.HttpID.Bytes(),
		}
		if nodeReq.DeltaHttpID != nil {
			insert.DeltaHttpId = nodeReq.DeltaHttpID.Bytes()
		}
		syncEvent = &flowv1.NodeHttpSync{
			Value: &flowv1.NodeHttpSync_ValueUnion{
				Kind:   flowv1.NodeHttpSync_ValueUnion_KIND_INSERT,
				Insert: insert,
			},
		}
	case nodeEventUpdate:
		update := &flowv1.NodeHttpSyncUpdate{
			NodeId: nodeReq.FlowNodeID.Bytes(),
			HttpId: &flowv1.NodeHttpSyncUpdate_HttpIdUnion{
				Kind:  flowv1.NodeHttpSyncUpdate_HttpIdUnion_KIND_VALUE,
				Value: nodeReq.HttpID.Bytes(),
			},
		}
		if nodeReq.DeltaHttpID != nil {
			update.DeltaHttpId = &flowv1.NodeHttpSyncUpdate_DeltaHttpIdUnion{
				Kind:  flowv1.NodeHttpSyncUpdate_DeltaHttpIdUnion_KIND_VALUE,
				Value: nodeReq.DeltaHttpID.Bytes(),
			}
		}
		syncEvent = &flowv1.NodeHttpSync{
			Value: &flowv1.NodeHttpSync_ValueUnion{
				Kind:   flowv1.NodeHttpSync_ValueUnion_KIND_UPDATE,
				Update: update,
			},
		}
	case nodeEventDelete:
		syncEvent = &flowv1.NodeHttpSync{
			Value: &flowv1.NodeHttpSync_ValueUnion{
				Kind: flowv1.NodeHttpSync_ValueUnion_KIND_DELETE,
				Delete: &flowv1.NodeHttpSyncDelete{
					NodeId: nodeReq.FlowNodeID.Bytes(),
				},
			},
		}
	default:
		return nil, nil
	}

	return &flowv1.NodeHttpSyncResponse{
		Items: []*flowv1.NodeHttpSync{syncEvent},
	}, nil
}
