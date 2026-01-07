//nolint:revive // exported
package rflowv2

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"connectrpc.com/connect"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/mutation"
	"the-dev-tools/server/pkg/service/sflow"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
)

func (s *FlowServiceV2RPC) NodeCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[flowv1.NodeCollectionResponse], error) {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	var nodesPB []*flowv1.Node
	for _, flow := range flows {
		nodes, err := s.nsReader.GetNodesByFlowID(ctx, flow.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, node := range nodes {
			nodePB := serializeNode(node)

			exec, err := s.nes.GetLatestNodeExecutionByNodeID(ctx, node.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			if exec != nil {
				nodePB.State = flowv1.FlowItemState(exec.State)
				if exec.Error != nil {
					nodePB.Info = exec.Error
				}
			}

			nodesPB = append(nodesPB, nodePB)
		}
	}

	return connect.NewResponse(&flowv1.NodeCollectionResponse{Items: nodesPB}), nil
}

func (s *FlowServiceV2RPC) NodeInsert(
	ctx context.Context,
	req *connect.Request[flowv1.NodeInsertRequest],
) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one node is required"))
	}

	// 1. Move validation OUTSIDE transaction (before BeginTx)
	type insertData struct {
		node        mflow.Node
		flowID      idwrap.IDWrap
		workspaceID idwrap.IDWrap
	}
	var validatedItems []insertData

	for _, item := range req.Msg.GetItems() {
		nodeModel, err := s.deserializeNodeInsert(item)
		if err != nil {
			return nil, err
		}

		if err := s.ensureFlowAccess(ctx, nodeModel.FlowID); err != nil {
			return nil, err
		}

		// Get workspace ID for the flow
		flow, err := s.fsReader.GetFlow(ctx, nodeModel.FlowID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		validatedItems = append(validatedItems, insertData{
			node:        *nodeModel,
			flowID:      nodeModel.FlowID,
			workspaceID: flow.WorkspaceID,
		})
	}

	if len(validatedItems) == 0 {
		return connect.NewResponse(&emptypb.Empty{}), nil
	}

	// 2. Begin transaction with mutation context
	mut := mutation.New(s.DB, mutation.WithPublisher(s.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	nsWriter := sflow.NewNodeWriter(mut.TX())

	// 3. Execute all inserts in transaction
	for _, data := range validatedItems {
		if err := nsWriter.CreateNode(ctx, data.node); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		mut.Track(mutation.Event{
			Entity:      mutation.EntityFlowNode,
			Op:          mutation.OpInsert,
			ID:          data.node.ID,
			WorkspaceID: data.workspaceID,
			ParentID:    data.flowID,
			Payload:     data.node,
		})
	}

	// 4. Commit transaction (auto-publishes events)
	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeUpdate(
	ctx context.Context,
	req *connect.Request[flowv1.NodeUpdateRequest],
) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one node is required"))
	}

	// 1. Move validation OUTSIDE transaction (before BeginTx)
	type updateData struct {
		node        mflow.Node
		flowID      idwrap.IDWrap
		workspaceID idwrap.IDWrap
	}
	var validatedUpdates []updateData

	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		existing, err := s.ensureNodeAccess(ctx, nodeID)
		if err != nil {
			return nil, err
		}

		if item.Kind != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node kind updates are not supported"))
		}
		if len(item.GetFlowId()) != 0 {
			requestedFlowID, err := idwrap.NewFromBytes(item.GetFlowId())
			if err != nil || requestedFlowID != existing.FlowID {
				return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node flow reassignment is not supported"))
			}
		}

		// Get workspace ID for the flow
		flow, err := s.fsReader.GetFlow(ctx, existing.FlowID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Apply updates
		if item.Name != nil {
			existing.Name = item.GetName()
		}

		if item.Position != nil {
			existing.PositionX = float64(item.Position.GetX())
			existing.PositionY = float64(item.Position.GetY())
		}

		validatedUpdates = append(validatedUpdates, updateData{
			node:        *existing,
			flowID:      existing.FlowID,
			workspaceID: flow.WorkspaceID,
		})
	}

	if len(validatedUpdates) == 0 {
		return connect.NewResponse(&emptypb.Empty{}), nil
	}

	// 2. Begin transaction with mutation context
	mut := mutation.New(s.DB, mutation.WithPublisher(s.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	nsWriter := sflow.NewNodeWriter(mut.TX())

	// 3. Execute all updates in transaction
	for _, data := range validatedUpdates {
		if err := nsWriter.UpdateNode(ctx, data.node); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		mut.Track(mutation.Event{
			Entity:      mutation.EntityFlowNode,
			Op:          mutation.OpUpdate,
			ID:          data.node.ID,
			WorkspaceID: data.workspaceID,
			ParentID:    data.flowID,
			Payload:     data.node,
		})
	}

	// 4. Commit transaction (auto-publishes events)
	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeDelete(
	ctx context.Context,
	req *connect.Request[flowv1.NodeDeleteRequest],
) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one node is required"))
	}

	// 1. Move validation OUTSIDE transaction (before BeginTx)
	type deleteData struct {
		nodeID idwrap.IDWrap
		flowID idwrap.IDWrap
	}
	var validatedItems []deleteData

	for _, item := range req.Msg.Items {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		existing, err := s.ensureNodeAccess(ctx, nodeID)
		if err != nil {
			return nil, err
		}

		validatedItems = append(validatedItems, deleteData{
			nodeID: existing.ID,
			flowID: existing.FlowID,
		})
	}

	if len(validatedItems) == 0 {
		return connect.NewResponse(&emptypb.Empty{}), nil
	}

	// 2. Begin transaction with mutation context
	mut := mutation.New(s.DB, mutation.WithPublisher(s.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	// 3. Execute all deletes in transaction
	for _, data := range validatedItems {
		mut.Track(mutation.Event{
			Entity:   mutation.EntityFlowNode,
			Op:       mutation.OpDelete,
			ID:       data.nodeID,
			ParentID: data.flowID,
		})
		if err := mut.Queries().DeleteFlowNode(ctx, data.nodeID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	// 4. Commit transaction (auto-publishes events)
	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[flowv1.NodeSyncResponse],
) error {
	if stream == nil {
		return connect.NewError(connect.CodeInternal, errors.New("stream is required"))
	}
	return s.streamNodeSync(ctx, func(resp *flowv1.NodeSyncResponse) error {
		return stream.Send(resp)
	})
}

func (s *FlowServiceV2RPC) streamNodeSync(
	ctx context.Context,
	send func(*flowv1.NodeSyncResponse) error,
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
			resp := nodeEventToSyncResponse(evt.Payload)
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

func (s *FlowServiceV2RPC) publishNodeEvent(eventType string, model mflow.Node) {
	if s.nodeStream == nil {
		return
	}
	nodePB := serializeNode(model)
	s.nodeStream.Publish(NodeTopic{FlowID: model.FlowID}, NodeEvent{
		Type:   eventType,
		FlowID: model.FlowID,
		Node:   nodePB,
	})
}

func nodeEventToSyncResponse(evt NodeEvent) *flowv1.NodeSyncResponse {
	if evt.Node == nil {
		return nil
	}

	node := evt.Node

	switch evt.Type {
	case nodeEventInsert:
		insert := &flowv1.NodeSyncInsert{
			NodeId:   node.GetNodeId(),
			FlowId:   node.GetFlowId(),
			Kind:     node.GetKind(),
			Name:     node.GetName(),
			Position: node.GetPosition(),
			State:    node.GetState(),
		}
		if info := node.GetInfo(); info != "" {
			insert.Info = &info
		}
		return &flowv1.NodeSyncResponse{
			Items: []*flowv1.NodeSync{{
				Value: &flowv1.NodeSync_ValueUnion{
					Kind:   flowv1.NodeSync_ValueUnion_KIND_INSERT,
					Insert: insert,
				},
			}},
		}
	case nodeEventUpdate:
		update := &flowv1.NodeSyncUpdate{
			NodeId: node.GetNodeId(),
		}
		if flowID := node.GetFlowId(); len(flowID) > 0 {
			update.FlowId = flowID
		}
		if kind := node.GetKind(); kind != flowv1.NodeKind_NODE_KIND_UNSPECIFIED {
			k := kind
			update.Kind = &k
		}
		if name := node.GetName(); name != "" {
			update.Name = &name
		}
		if pos := node.GetPosition(); pos != nil {
			update.Position = pos
		}
		// Always include state to support resetting to UNSPECIFIED
		st := node.GetState()
		update.State = &st
		if info := node.GetInfo(); info != "" {
			update.Info = &flowv1.NodeSyncUpdate_InfoUnion{
				Kind:  flowv1.NodeSyncUpdate_InfoUnion_KIND_VALUE,
				Value: &info,
			}
		}
		return &flowv1.NodeSyncResponse{
			Items: []*flowv1.NodeSync{{
				Value: &flowv1.NodeSync_ValueUnion{
					Kind:   flowv1.NodeSync_ValueUnion_KIND_UPDATE,
					Update: update,
				},
			}},
		}
	case nodeEventDelete:
		return &flowv1.NodeSyncResponse{
			Items: []*flowv1.NodeSync{{
				Value: &flowv1.NodeSync_ValueUnion{
					Kind: flowv1.NodeSync_ValueUnion_KIND_DELETE,
					Delete: &flowv1.NodeSyncDelete{
						NodeId: node.GetNodeId(),
					},
				},
			}},
		}
	default:
		return nil
	}
}
