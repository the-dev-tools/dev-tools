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

type nodeWaitWithFlow struct {
	nodeWait mflow.NodeWait
	flowID   idwrap.IDWrap
	baseNode *mflow.Node
}

func (s *FlowServiceV2RPC) NodeWaitCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[flowv1.NodeWaitCollectionResponse], error) {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	var items []*flowv1.NodeWait
	for _, flow := range flows {
		nodes, err := s.nsReader.GetNodesByFlowID(ctx, flow.ID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, node := range nodes {
			if node.NodeKind != mflow.NODE_KIND_WAIT {
				continue
			}
			nodeWait, err := s.nwaits.GetNodeWait(ctx, node.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			if nodeWait == nil {
				continue
			}
			items = append(items, serializeNodeWait(*nodeWait))
		}
	}

	return connect.NewResponse(&flowv1.NodeWaitCollectionResponse{Items: items}), nil
}

func (s *FlowServiceV2RPC) NodeWaitInsert(
	ctx context.Context,
	req *connect.Request[flowv1.NodeWaitInsertRequest],
) (*connect.Response[emptypb.Empty], error) {
	type insertData struct {
		nodeID     idwrap.IDWrap
		durationMs int64
		baseNode   *mflow.Node
		flowID     idwrap.IDWrap
		workspaceID idwrap.IDWrap
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
			nodeID:      nodeID,
			durationMs:  item.GetDurationMs(),
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

	nwaitsWriter := s.nwaits.TX(mut.TX())

	for _, data := range validatedItems {
		nodeWait := mflow.NodeWait{
			FlowNodeID: data.nodeID,
			DurationMs: data.durationMs,
		}

		if err := nwaitsWriter.CreateNodeWait(ctx, nodeWait); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if data.baseNode != nil {
			mut.Track(mutation.Event{
				Entity:      mutation.EntityFlowNodeWait,
				Op:          mutation.OpInsert,
				ID:          data.nodeID,
				WorkspaceID: data.workspaceID,
				ParentID:    data.flowID,
				Payload: nodeWaitWithFlow{
					nodeWait: nodeWait,
					flowID:   data.flowID,
					baseNode: data.baseNode,
				},
			})
		}
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeWaitUpdate(
	ctx context.Context,
	req *connect.Request[flowv1.NodeWaitUpdateRequest],
) (*connect.Response[emptypb.Empty], error) {
	type updateData struct {
		nodeID      idwrap.IDWrap
		durationMs  int64
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

		existing, err := s.nwaits.GetNodeWait(ctx, nodeID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		durationMs := existing.DurationMs
		if item.DurationMs != nil {
			durationMs = *item.DurationMs
		}

		validatedItems = append(validatedItems, updateData{
			nodeID:      nodeID,
			durationMs:  durationMs,
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

	nwaitsWriter := s.nwaits.TX(mut.TX())

	for _, data := range validatedItems {
		nodeWait := mflow.NodeWait{
			FlowNodeID: data.nodeID,
			DurationMs: data.durationMs,
		}

		if err := nwaitsWriter.UpdateNodeWait(ctx, nodeWait); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		mut.Track(mutation.Event{
			Entity:      mutation.EntityFlowNodeWait,
			Op:          mutation.OpUpdate,
			ID:          data.nodeID,
			WorkspaceID: data.workspaceID,
			ParentID:    data.baseNode.FlowID,
			Payload: nodeWaitWithFlow{
				nodeWait: nodeWait,
				flowID:   data.baseNode.FlowID,
				baseNode: data.baseNode,
			},
		})
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeWaitDelete(
	ctx context.Context,
	req *connect.Request[flowv1.NodeWaitDeleteRequest],
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
			Entity:   mutation.EntityFlowNodeWait,
			Op:       mutation.OpDelete,
			ID:       data.nodeID,
			ParentID: data.flowID,
		})
		if err := mut.Queries().DeleteFlowNodeWait(ctx, data.nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeWaitSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[flowv1.NodeWaitSyncResponse],
) error {
	if stream == nil {
		return connect.NewError(connect.CodeInternal, errors.New("stream is required"))
	}
	return s.streamNodeWaitSync(ctx, func(resp *flowv1.NodeWaitSyncResponse) error {
		return stream.Send(resp)
	})
}

func (s *FlowServiceV2RPC) streamNodeWaitSync(
	ctx context.Context,
	send func(*flowv1.NodeWaitSyncResponse) error,
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
			resp, err := s.nodeWaitEventToSyncResponse(ctx, evt.Payload)
			if err != nil {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to convert wait node event: %w", err))
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

func (s *FlowServiceV2RPC) nodeWaitEventToSyncResponse(
	ctx context.Context,
	evt NodeEvent,
) (*flowv1.NodeWaitSyncResponse, error) {
	if evt.Node == nil {
		return nil, nil
	}

	if evt.Node.GetKind() != flowv1.NodeKind_NODE_KIND_WAIT {
		return nil, nil
	}

	nodeID, err := idwrap.NewFromBytes(evt.Node.GetNodeId())
	if err != nil {
		return nil, fmt.Errorf("invalid node id: %w", err)
	}

	nodeWait, err := s.nwaits.GetNodeWait(ctx, nodeID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	var syncEvent *flowv1.NodeWaitSync
	switch evt.Type {
	case nodeEventInsert:
		insert := &flowv1.NodeWaitSyncInsert{
			NodeId: nodeID.Bytes(),
		}
		if nodeWait != nil {
			insert.DurationMs = nodeWait.DurationMs
		}
		syncEvent = &flowv1.NodeWaitSync{
			Value: &flowv1.NodeWaitSync_ValueUnion{
				Kind:   flowv1.NodeWaitSync_ValueUnion_KIND_INSERT,
				Insert: insert,
			},
		}
	case nodeEventUpdate:
		update := &flowv1.NodeWaitSyncUpdate{
			NodeId: nodeID.Bytes(),
		}
		if nodeWait != nil {
			update.DurationMs = &nodeWait.DurationMs
		}
		syncEvent = &flowv1.NodeWaitSync{
			Value: &flowv1.NodeWaitSync_ValueUnion{
				Kind:   flowv1.NodeWaitSync_ValueUnion_KIND_UPDATE,
				Update: update,
			},
		}
	case nodeEventDelete:
		syncEvent = &flowv1.NodeWaitSync{
			Value: &flowv1.NodeWaitSync_ValueUnion{
				Kind: flowv1.NodeWaitSync_ValueUnion_KIND_DELETE,
				Delete: &flowv1.NodeWaitSyncDelete{
					NodeId: nodeID.Bytes(),
				},
			},
		}
	default:
		return nil, nil
	}

	return &flowv1.NodeWaitSyncResponse{
		Items: []*flowv1.NodeWaitSync{syncEvent},
	}, nil
}

func serializeNodeWait(n mflow.NodeWait) *flowv1.NodeWait {
	return &flowv1.NodeWait{
		NodeId:     n.FlowNodeID.Bytes(),
		DurationMs: n.DurationMs,
	}
}
