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

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/mutation"
	flowv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/flow/v1"
)

// NodeWsSendTopic identifies the flow whose WS Send nodes are being published.
type NodeWsSendTopic struct {
	FlowID idwrap.IDWrap
}

// NodeWsSendEvent describes a WS Send node change for sync streaming.
type NodeWsSendEvent struct {
	Type   string
	FlowID idwrap.IDWrap
	Node   *flowv1.NodeWsSend
}

func (s *FlowServiceV2RPC) NodeWsSendCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[flowv1.NodeWsSendCollectionResponse], error) {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	var items []*flowv1.NodeWsSend
	for _, flow := range flows {
		nodes, err := s.nsReader.GetNodesByFlowID(ctx, flow.ID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, node := range nodes {
			if node.NodeKind != mflow.NODE_KIND_WS_SEND {
				continue
			}
			nodeWsSend, err := s.nwss.GetNodeWsSend(ctx, node.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			if nodeWsSend == nil {
				continue
			}
			items = append(items, serializeNodeWsSend(*nodeWsSend))
		}
	}

	return connect.NewResponse(&flowv1.NodeWsSendCollectionResponse{Items: items}), nil
}

func (s *FlowServiceV2RPC) NodeWsSendInsert(
	ctx context.Context,
	req *connect.Request[flowv1.NodeWsSendInsertRequest],
) (*connect.Response[emptypb.Empty], error) {
	type insertData struct {
		nodeID               idwrap.IDWrap
		wsConnectionNodeName string
		message              string
		baseNode             *mflow.Node
		flowID               idwrap.IDWrap
		workspaceID          idwrap.IDWrap
	}
	var validatedItems []insertData

	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		baseNode, _ := s.ns.GetNode(ctx, nodeID)

		var flowID idwrap.IDWrap
		var workspaceID idwrap.IDWrap
		if baseNode != nil {
			flowID = baseNode.FlowID
			flow, err := s.fsReader.GetFlow(ctx, flowID)
			if err == nil {
				workspaceID = flow.WorkspaceID
			}
		}

		validatedItems = append(validatedItems, insertData{
			nodeID:               nodeID,
			wsConnectionNodeName: item.GetWsConnectionNodeName(),
			message:              item.GetMessage(),
			baseNode:             baseNode,
			flowID:               flowID,
			workspaceID:          workspaceID,
		})
	}

	if len(validatedItems) == 0 {
		return connect.NewResponse(&emptypb.Empty{}), nil
	}

	mut := mutation.New(s.DB, mutation.WithPublisher(s.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	nwssWriter := s.nwss.TX(mut.TX())

	for _, data := range validatedItems {
		nodeWsSend := mflow.NodeWsSend{
			FlowNodeID:           data.nodeID,
			WsConnectionNodeName: data.wsConnectionNodeName,
			Message:              data.message,
		}

		if err := nwssWriter.CreateNodeWsSend(ctx, nodeWsSend); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if data.baseNode != nil {
			mut.Track(mutation.Event{
				Entity:      mutation.EntityFlowNodeWsSend,
				Op:          mutation.OpInsert,
				ID:          data.nodeID,
				WorkspaceID: data.workspaceID,
				ParentID:    data.flowID,
				Payload: nodeWsSendWithFlow{
					nodeWsSend: nodeWsSend,
					flowID:     data.flowID,
					baseNode:   data.baseNode,
				},
			})
		}
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeWsSendUpdate(
	ctx context.Context,
	req *connect.Request[flowv1.NodeWsSendUpdateRequest],
) (*connect.Response[emptypb.Empty], error) {
	type updateData struct {
		nodeID               idwrap.IDWrap
		wsConnectionNodeName string
		message              string
		baseNode             *mflow.Node
		workspaceID          idwrap.IDWrap
	}
	var validatedItems []updateData

	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		nodeModel, err := s.ensureNodeAccess(ctx, nodeID)
		if err != nil {
			return nil, err
		}

		flow, err := s.fsReader.GetFlow(ctx, nodeModel.FlowID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Get existing values to merge partial updates
		existing, err := s.nwss.GetNodeWsSend(ctx, nodeID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		wsConnName := existing.WsConnectionNodeName
		if item.WsConnectionNodeName != nil {
			wsConnName = *item.WsConnectionNodeName
		}
		msg := existing.Message
		if item.Message != nil {
			msg = *item.Message
		}

		validatedItems = append(validatedItems, updateData{
			nodeID:               nodeID,
			wsConnectionNodeName: wsConnName,
			message:              msg,
			baseNode:             nodeModel,
			workspaceID:          flow.WorkspaceID,
		})
	}

	if len(validatedItems) == 0 {
		return connect.NewResponse(&emptypb.Empty{}), nil
	}

	mut := mutation.New(s.DB, mutation.WithPublisher(s.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	nwssWriter := s.nwss.TX(mut.TX())

	for _, data := range validatedItems {
		nodeWsSend := mflow.NodeWsSend{
			FlowNodeID:           data.nodeID,
			WsConnectionNodeName: data.wsConnectionNodeName,
			Message:              data.message,
		}

		if err := nwssWriter.UpdateNodeWsSend(ctx, nodeWsSend); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		mut.Track(mutation.Event{
			Entity:      mutation.EntityFlowNodeWsSend,
			Op:          mutation.OpUpdate,
			ID:          data.nodeID,
			WorkspaceID: data.workspaceID,
			ParentID:    data.baseNode.FlowID,
			Payload: nodeWsSendWithFlow{
				nodeWsSend: nodeWsSend,
				flowID:     data.baseNode.FlowID,
				baseNode:   data.baseNode,
			},
		})
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeWsSendDelete(
	ctx context.Context,
	req *connect.Request[flowv1.NodeWsSendDeleteRequest],
) (*connect.Response[emptypb.Empty], error) {
	type deleteData struct {
		nodeID idwrap.IDWrap
		flowID idwrap.IDWrap
	}
	var validatedItems []deleteData

	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		nodeModel, err := s.ensureNodeAccess(ctx, nodeID)
		if err != nil {
			return nil, err
		}

		validatedItems = append(validatedItems, deleteData{
			nodeID: nodeID,
			flowID: nodeModel.FlowID,
		})
	}

	if len(validatedItems) == 0 {
		return connect.NewResponse(&emptypb.Empty{}), nil
	}

	mut := mutation.New(s.DB, mutation.WithPublisher(s.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	for _, data := range validatedItems {
		mut.Track(mutation.Event{
			Entity:   mutation.EntityFlowNodeWsSend,
			Op:       mutation.OpDelete,
			ID:       data.nodeID,
			ParentID: data.flowID,
		})
		if err := mut.Queries().DeleteFlowNodeWsSend(ctx, data.nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeWsSendSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[flowv1.NodeWsSendSyncResponse],
) error {
	if stream == nil {
		return connect.NewError(connect.CodeInternal, errors.New("stream is required"))
	}
	return s.streamNodeWsSendSync(ctx, func(resp *flowv1.NodeWsSendSyncResponse) error {
		return stream.Send(resp)
	})
}

func (s *FlowServiceV2RPC) streamNodeWsSendSync(
	ctx context.Context,
	send func(*flowv1.NodeWsSendSyncResponse) error,
) error {
	if s.nodeStream == nil {
		return connect.NewError(connect.CodeUnavailable, errors.New("node stream not configured"))
	}

	var flowSet sync.Map

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

	events, err := s.nodeStream.Subscribe(ctx, filter)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	for {
		select {
		case evt, ok := <-events:
			if !ok {
				return nil
			}
			resp, err := s.nodeWsSendEventToSyncResponse(ctx, evt.Payload)
			if err != nil {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to convert WS send node event: %w", err))
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

func (s *FlowServiceV2RPC) nodeWsSendEventToSyncResponse(
	ctx context.Context,
	evt NodeEvent,
) (*flowv1.NodeWsSendSyncResponse, error) {
	if evt.Node == nil {
		return nil, nil
	}

	if evt.Node.GetKind() != flowv1.NodeKind_NODE_KIND_WS_SEND {
		return nil, nil
	}

	nodeID, err := idwrap.NewFromBytes(evt.Node.GetNodeId())
	if err != nil {
		return nil, fmt.Errorf("invalid node id: %w", err)
	}

	nodeWsSend, err := s.nwss.GetNodeWsSend(ctx, nodeID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	var syncEvent *flowv1.NodeWsSendSync
	switch evt.Type {
	case nodeEventInsert:
		insert := &flowv1.NodeWsSendSyncInsert{
			NodeId: nodeID.Bytes(),
		}
		if nodeWsSend != nil {
			insert.WsConnectionNodeName = nodeWsSend.WsConnectionNodeName
			insert.Message = nodeWsSend.Message
		}
		syncEvent = &flowv1.NodeWsSendSync{
			Value: &flowv1.NodeWsSendSync_ValueUnion{
				Kind:   flowv1.NodeWsSendSync_ValueUnion_KIND_INSERT,
				Insert: insert,
			},
		}
	case nodeEventUpdate:
		update := &flowv1.NodeWsSendSyncUpdate{
			NodeId: nodeID.Bytes(),
		}
		if nodeWsSend != nil {
			update.WsConnectionNodeName = &nodeWsSend.WsConnectionNodeName
			update.Message = &nodeWsSend.Message
		}
		syncEvent = &flowv1.NodeWsSendSync{
			Value: &flowv1.NodeWsSendSync_ValueUnion{
				Kind:   flowv1.NodeWsSendSync_ValueUnion_KIND_UPDATE,
				Update: update,
			},
		}
	case nodeEventDelete:
		syncEvent = &flowv1.NodeWsSendSync{
			Value: &flowv1.NodeWsSendSync_ValueUnion{
				Kind: flowv1.NodeWsSendSync_ValueUnion_KIND_DELETE,
				Delete: &flowv1.NodeWsSendSyncDelete{
					NodeId: nodeID.Bytes(),
				},
			},
		}
	default:
		return nil, nil
	}

	return &flowv1.NodeWsSendSyncResponse{
		Items: []*flowv1.NodeWsSendSync{syncEvent},
	}, nil
}

func serializeNodeWsSend(n mflow.NodeWsSend) *flowv1.NodeWsSend {
	return &flowv1.NodeWsSend{
		NodeId:               n.FlowNodeID.Bytes(),
		WsConnectionNodeName: n.WsConnectionNodeName,
		Message:              n.Message,
	}
}
