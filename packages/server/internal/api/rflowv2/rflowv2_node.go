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
	"the-dev-tools/server/pkg/patch"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/txutil"
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
		node   mflow.Node
		flowID idwrap.IDWrap
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

		validatedItems = append(validatedItems, insertData{
			node:   *nodeModel,
			flowID: nodeModel.FlowID,
		})
	}

	// 2. Begin transaction with bulk sync wrapper
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	syncTx := txutil.NewBulkInsertTx[nodeWithFlow, NodeTopic](
		tx,
		func(nwf nodeWithFlow) NodeTopic {
			return NodeTopic{FlowID: nwf.flowID}
		},
	)

	nsWriter := sflow.NewNodeWriter(tx)

	// 3. Execute all inserts in transaction
	for _, data := range validatedItems {
		if err := nsWriter.CreateNode(ctx, data.node); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		syncTx.Track(nodeWithFlow(data))
	}

	// 4. Commit transaction and publish events in bulk
	if err := syncTx.CommitAndPublish(ctx, s.publishBulkNodeInsert); err != nil {
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
		node      mflow.Node
		nodePatch patch.NodePatch
		flowID    idwrap.IDWrap
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
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node flow reassignment is not supported"))
		}

		// Build patch
		var nodePatch patch.NodePatch
		if item.Name != nil {
			existing.Name = item.GetName()
			nodePatch.Name = patch.NewOptional(item.GetName())
		}

		if item.Position != nil {
			existing.PositionX = float64(item.Position.GetX())
			existing.PositionY = float64(item.Position.GetY())
			nodePatch.PositionX = patch.NewOptional(float64(item.Position.GetX()))
			nodePatch.PositionY = patch.NewOptional(float64(item.Position.GetY()))
		}

		validatedUpdates = append(validatedUpdates, updateData{
			node:      *existing,
			nodePatch: nodePatch,
			flowID:    existing.FlowID,
		})
	}

	// 2. Begin transaction with bulk sync wrapper
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	syncTx := txutil.NewBulkUpdateTx[nodeWithFlow, patch.NodePatch, NodeTopic](
		tx,
		func(nwf nodeWithFlow) NodeTopic {
			return NodeTopic{FlowID: nwf.flowID}
		},
	)

	nsWriter := sflow.NewNodeWriter(tx)

	// 3. Execute all updates in transaction
	for _, data := range validatedUpdates {
		if err := nsWriter.UpdateNode(ctx, data.node); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		syncTx.Track(
			nodeWithFlow{
				node:   data.node,
				flowID: data.flowID,
			},
			data.nodePatch,
		)
	}

	// 4. Commit transaction and publish events in bulk
	if err := syncTx.CommitAndPublish(ctx, s.publishBulkNodeUpdate); err != nil {
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

	// 2. Begin transaction with bulk sync wrapper
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	syncTx := txutil.NewBulkDeleteTx[idwrap.IDWrap, NodeTopic](
		tx,
		func(evt txutil.DeleteEvent[idwrap.IDWrap]) NodeTopic {
			return NodeTopic{FlowID: evt.WorkspaceID} // WorkspaceID field is reused for FlowID
		},
	)

	nsWriter := sflow.NewNodeWriter(tx)

	// 3. Execute all deletes in transaction
	for _, data := range validatedItems {
		if err := nsWriter.DeleteNode(ctx, data.nodeID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		syncTx.Track(data.nodeID, data.flowID, false)
	}

	// 4. Commit transaction and publish events in bulk
	if err := syncTx.CommitAndPublish(ctx, s.publishBulkNodeDelete); err != nil {
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
