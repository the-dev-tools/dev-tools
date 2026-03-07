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

// NodeWsConnectionTopic identifies the flow whose WS Connection nodes are being published.
type NodeWsConnectionTopic struct {
	FlowID idwrap.IDWrap
}

// NodeWsConnectionEvent describes a WS Connection node change for sync streaming.
type NodeWsConnectionEvent struct {
	Type   string
	FlowID idwrap.IDWrap
	Node   *flowv1.NodeWsConnection
}

func (s *FlowServiceV2RPC) NodeWsConnectionCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[flowv1.NodeWsConnectionCollectionResponse], error) {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	var items []*flowv1.NodeWsConnection
	for _, flow := range flows {
		nodes, err := s.nsReader.GetNodesByFlowID(ctx, flow.ID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, node := range nodes {
			if node.NodeKind != mflow.NODE_KIND_WS_CONNECTION {
				continue
			}
			nodeWsConn, err := s.nwcs.GetNodeWsConnection(ctx, node.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			if nodeWsConn == nil {
				continue
			}
			items = append(items, serializeNodeWsConnection(*nodeWsConn))
		}
	}

	return connect.NewResponse(&flowv1.NodeWsConnectionCollectionResponse{Items: items}), nil
}

func (s *FlowServiceV2RPC) NodeWsConnectionInsert(
	ctx context.Context,
	req *connect.Request[flowv1.NodeWsConnectionInsertRequest],
) (*connect.Response[emptypb.Empty], error) {
	type insertData struct {
		nodeID      idwrap.IDWrap
		wsID        *idwrap.IDWrap
		baseNode    *mflow.Node
		flowID      idwrap.IDWrap
		workspaceID idwrap.IDWrap
	}
	var validatedItems []insertData

	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		var wsID *idwrap.IDWrap
		if len(item.GetWebsocketId()) > 0 {
			parsedID, err := idwrap.NewFromBytes(item.GetWebsocketId())
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid websocket id: %w", err))
			}
			if !isZeroID(parsedID) {
				wsID = &parsedID
			}
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
			nodeID:      nodeID,
			wsID:        wsID,
			baseNode:    baseNode,
			flowID:      flowID,
			workspaceID: workspaceID,
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

	nwcsWriter := s.nwcs.TX(mut.TX())

	for _, data := range validatedItems {
		nodeWsConn := mflow.NodeWsConnection{
			FlowNodeID:  data.nodeID,
			WebSocketID: data.wsID,
		}

		if err := nwcsWriter.CreateNodeWsConnection(ctx, nodeWsConn); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if data.baseNode != nil {
			mut.Track(mutation.Event{
				Entity:      mutation.EntityFlowNodeWsConnection,
				Op:          mutation.OpInsert,
				ID:          data.nodeID,
				WorkspaceID: data.workspaceID,
				ParentID:    data.flowID,
				Payload: nodeWsConnectionWithFlow{
					nodeWsConnection: nodeWsConn,
					flowID:           data.flowID,
					baseNode:         data.baseNode,
				},
			})
		}
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeWsConnectionUpdate(
	ctx context.Context,
	req *connect.Request[flowv1.NodeWsConnectionUpdateRequest],
) (*connect.Response[emptypb.Empty], error) {
	type updateData struct {
		nodeID      idwrap.IDWrap
		wsID        *idwrap.IDWrap
		baseNode    *mflow.Node
		workspaceID idwrap.IDWrap
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

		var wsID *idwrap.IDWrap
		if wsUnion := item.GetWebsocketId(); wsUnion != nil {
			if wsUnion.GetKind() == flowv1.NodeWsConnectionUpdate_WebsocketIdUnion_KIND_VALUE {
				if len(wsUnion.GetValue()) > 0 {
					parsedID, err := idwrap.NewFromBytes(wsUnion.GetValue())
					if err != nil {
						return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid websocket id: %w", err))
					}
					if !isZeroID(parsedID) {
						wsID = &parsedID
					}
				}
			}
			// KIND_UNSET leaves wsID as nil (clears it)
		} else {
			// No update to websocket_id — preserve existing
			existing, err := s.nwcs.GetNodeWsConnection(ctx, nodeID)
			if err == nil {
				wsID = existing.WebSocketID
			}
		}

		validatedItems = append(validatedItems, updateData{
			nodeID:      nodeID,
			wsID:        wsID,
			baseNode:    nodeModel,
			workspaceID: flow.WorkspaceID,
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

	nwcsWriter := s.nwcs.TX(mut.TX())

	for _, data := range validatedItems {
		nodeWsConn := mflow.NodeWsConnection{
			FlowNodeID:  data.nodeID,
			WebSocketID: data.wsID,
		}

		if err := nwcsWriter.UpdateNodeWsConnection(ctx, nodeWsConn); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		mut.Track(mutation.Event{
			Entity:      mutation.EntityFlowNodeWsConnection,
			Op:          mutation.OpUpdate,
			ID:          data.nodeID,
			WorkspaceID: data.workspaceID,
			ParentID:    data.baseNode.FlowID,
			Payload: nodeWsConnectionWithFlow{
				nodeWsConnection: nodeWsConn,
				flowID:           data.baseNode.FlowID,
				baseNode:         data.baseNode,
			},
		})
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeWsConnectionDelete(
	ctx context.Context,
	req *connect.Request[flowv1.NodeWsConnectionDeleteRequest],
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
			Entity:   mutation.EntityFlowNodeWsConnection,
			Op:       mutation.OpDelete,
			ID:       data.nodeID,
			ParentID: data.flowID,
		})
		if err := mut.Queries().DeleteFlowNodeWsConnection(ctx, data.nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeWsConnectionSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[flowv1.NodeWsConnectionSyncResponse],
) error {
	if stream == nil {
		return connect.NewError(connect.CodeInternal, errors.New("stream is required"))
	}
	return s.streamNodeWsConnectionSync(ctx, func(resp *flowv1.NodeWsConnectionSyncResponse) error {
		return stream.Send(resp)
	})
}

func (s *FlowServiceV2RPC) streamNodeWsConnectionSync(
	ctx context.Context,
	send func(*flowv1.NodeWsConnectionSyncResponse) error,
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
			resp, err := s.nodeWsConnectionEventToSyncResponse(ctx, evt.Payload)
			if err != nil {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to convert WS connection node event: %w", err))
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

func (s *FlowServiceV2RPC) nodeWsConnectionEventToSyncResponse(
	ctx context.Context,
	evt NodeEvent,
) (*flowv1.NodeWsConnectionSyncResponse, error) {
	if evt.Node == nil {
		return nil, nil
	}

	if evt.Node.GetKind() != flowv1.NodeKind_NODE_KIND_WS_CONNECTION {
		return nil, nil
	}

	nodeID, err := idwrap.NewFromBytes(evt.Node.GetNodeId())
	if err != nil {
		return nil, fmt.Errorf("invalid node id: %w", err)
	}

	nodeWsConn, err := s.nwcs.GetNodeWsConnection(ctx, nodeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Skip — version nodes don't have WsConnection records
		}
		return nil, err
	}

	var syncEvent *flowv1.NodeWsConnectionSync
	switch evt.Type {
	case nodeEventInsert:
		insert := &flowv1.NodeWsConnectionSyncInsert{
			NodeId: nodeID.Bytes(),
		}
		if nodeWsConn != nil && nodeWsConn.WebSocketID != nil && !isZeroID(*nodeWsConn.WebSocketID) {
			insert.WebsocketId = nodeWsConn.WebSocketID.Bytes()
		}
		syncEvent = &flowv1.NodeWsConnectionSync{
			Value: &flowv1.NodeWsConnectionSync_ValueUnion{
				Kind:   flowv1.NodeWsConnectionSync_ValueUnion_KIND_INSERT,
				Insert: insert,
			},
		}
	case nodeEventUpdate:
		update := &flowv1.NodeWsConnectionSyncUpdate{
			NodeId: nodeID.Bytes(),
		}
		if nodeWsConn != nil && nodeWsConn.WebSocketID != nil && !isZeroID(*nodeWsConn.WebSocketID) {
			update.WebsocketId = &flowv1.NodeWsConnectionSyncUpdate_WebsocketIdUnion{
				Kind:  flowv1.NodeWsConnectionSyncUpdate_WebsocketIdUnion_KIND_VALUE,
				Value: nodeWsConn.WebSocketID.Bytes(),
			}
		}
		syncEvent = &flowv1.NodeWsConnectionSync{
			Value: &flowv1.NodeWsConnectionSync_ValueUnion{
				Kind:   flowv1.NodeWsConnectionSync_ValueUnion_KIND_UPDATE,
				Update: update,
			},
		}
	case nodeEventDelete:
		syncEvent = &flowv1.NodeWsConnectionSync{
			Value: &flowv1.NodeWsConnectionSync_ValueUnion{
				Kind: flowv1.NodeWsConnectionSync_ValueUnion_KIND_DELETE,
				Delete: &flowv1.NodeWsConnectionSyncDelete{
					NodeId: nodeID.Bytes(),
				},
			},
		}
	default:
		return nil, nil
	}

	return &flowv1.NodeWsConnectionSyncResponse{
		Items: []*flowv1.NodeWsConnectionSync{syncEvent},
	}, nil
}

func serializeNodeWsConnection(n mflow.NodeWsConnection) *flowv1.NodeWsConnection {
	msg := &flowv1.NodeWsConnection{
		NodeId: n.FlowNodeID.Bytes(),
	}
	if n.WebSocketID != nil && !isZeroID(*n.WebSocketID) {
		msg.WebsocketId = n.WebSocketID.Bytes()
	}
	return msg
}
