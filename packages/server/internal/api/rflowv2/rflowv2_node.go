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

	devtoolsdb "the-dev-tools/db"
	"the-dev-tools/server/pkg/eventstream"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
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

	// Step 1: FETCH/CHECK (Outside transaction)
	var nodeModels []*mflow.Node
	for _, item := range req.Msg.GetItems() {
		nodeModel, err := s.deserializeNodeInsert(item)
		if err != nil {
			return nil, err
		}

		if err := s.ensureFlowAccess(ctx, nodeModel.FlowID); err != nil {
			return nil, err
		}
		nodeModels = append(nodeModels, nodeModel)
	}

	// Step 2: ACT (Inside transaction)
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	nsWriter := sflow.NewNodeWriter(tx)

	for _, nodeModel := range nodeModels {
		if err := nsWriter.CreateNode(ctx, *nodeModel); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Step 3: NOTIFY (Outside transaction)
	for _, nodeModel := range nodeModels {
		s.publishNodeEvent(nodeEventInsert, *nodeModel)
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

	// Step 1: FETCH/CHECK (Outside transaction)
	var updateData []struct {
		existing *mflow.Node
		item     *flowv1.NodeUpdate
	}

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
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node flow reassignment is not supported"))
		}

		if item.Name != nil {
			existing.Name = item.GetName()
		}

		if item.Position != nil {
			existing.PositionX = float64(item.Position.GetX())
			existing.PositionY = float64(item.Position.GetY())
		}

		updateData = append(updateData, struct {
			existing *mflow.Node
			item     *flowv1.NodeUpdate
		}{existing: existing, item: item})
	}

	// Step 2: ACT (Inside transaction)
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	nsWriter := sflow.NewNodeWriter(tx)

	for _, data := range updateData {
		if err := nsWriter.UpdateNode(ctx, *data.existing); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Step 3: NOTIFY (Outside transaction)
	for _, data := range updateData {
		s.publishNodeEvent(nodeEventUpdate, *data.existing)
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

	// Step 1: FETCH/CHECK (Outside transaction)
	var deleteData []struct {
		existing *mflow.Node
	}

	for _, item := range req.Msg.Items {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		existing, err := s.ensureNodeAccess(ctx, nodeID)
		if err != nil {
			return nil, err
		}
		deleteData = append(deleteData, struct{ existing *mflow.Node }{existing: existing})
	}

	// Step 2: ACT (Inside transaction)
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	nsWriter := sflow.NewNodeWriter(tx)

	for _, data := range deleteData {
		if err := nsWriter.DeleteNode(ctx, data.existing.ID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Step 3: NOTIFY (Outside transaction)
	for _, data := range deleteData {
		s.publishNodeEvent(nodeEventDelete, *data.existing)
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
				nodePB := serializeNode(nodeModel)
				exec, err := s.nes.GetLatestNodeExecutionByNodeID(ctx, nodeModel.ID)
				if err == nil && exec != nil {
					nodePB.State = flowv1.FlowItemState(exec.State)
					if exec.Error != nil {
						nodePB.Info = exec.Error
					}
				}

				events = append(events, eventstream.Event[NodeTopic, NodeEvent]{
					Topic: NodeTopic{FlowID: flow.ID},
					Payload: NodeEvent{
						Type:   nodeEventInsert,
						FlowID: flow.ID,
						Node:   nodePB,
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
		if state := node.GetState(); state != flowv1.FlowItemState_FLOW_ITEM_STATE_UNSPECIFIED {
			st := state
			update.State = &st
		}
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
