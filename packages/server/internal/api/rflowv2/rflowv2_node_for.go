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
	"the-dev-tools/server/pkg/txutil"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
)

// --- For Node ---

func (s *FlowServiceV2RPC) NodeForCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[flowv1.NodeForCollectionResponse], error) {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]*flowv1.NodeFor, 0)

	for _, flow := range flows {
		nodes, err := s.ns.GetNodesByFlowID(ctx, flow.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, n := range nodes {
			if n.NodeKind != mflow.NODE_KIND_FOR {
				continue
			}
			nodeFor, err := s.nfs.GetNodeFor(ctx, n.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			if nodeFor == nil {
				continue
			}
			items = append(items, serializeNodeFor(*nodeFor))
		}
	}

	return connect.NewResponse(&flowv1.NodeForCollectionResponse{Items: items}), nil
}

func (s *FlowServiceV2RPC) NodeForInsert(ctx context.Context, req *connect.Request[flowv1.NodeForInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	// 1. Move validation OUTSIDE transaction (before BeginTx)
	type insertData struct {
		nodeID   idwrap.IDWrap
		model    mflow.NodeFor
		baseNode *mflow.Node
		flowID   idwrap.IDWrap
	}
	var validatedItems []insertData

	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		model := mflow.NodeFor{
			FlowNodeID:    nodeID,
			IterCount:     int64(item.GetIterations()),
			Condition:     buildCondition(item.GetCondition()),
			ErrorHandling: mflow.ErrorHandling(item.GetErrorHandling()), // nolint:gosec // G115
		}

		// CRITICAL FIX: Get base node BEFORE transaction to avoid SQLite deadlock
		// Allow nil baseNode to support out-of-order message arrival
		baseNode, _ := s.ns.GetNode(ctx, nodeID)

		var flowID idwrap.IDWrap
		if baseNode != nil {
			flowID = baseNode.FlowID
		}

		validatedItems = append(validatedItems, insertData{
			nodeID:   nodeID,
			model:    model,
			baseNode: baseNode,
			flowID:   flowID,
		})
	}

	// 2. Begin transaction with bulk sync wrapper
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	syncTx := txutil.NewBulkInsertTx[nodeForWithFlow, ForTopic](
		tx,
		func(nfwf nodeForWithFlow) ForTopic {
			return ForTopic{FlowID: nfwf.flowID}
		},
	)

	nfsWriter := s.nfs.TX(tx)

	// 3. Execute all inserts in transaction
	for _, data := range validatedItems {
		if err := nfsWriter.CreateNodeFor(ctx, data.model); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Only track for event publishing if base node exists
		if data.baseNode != nil {
			syncTx.Track(nodeForWithFlow{
				nodeFor: data.model,
				flowID:  data.flowID,
			})
		}
	}

	// 4. Commit transaction and publish events in bulk
	if err := syncTx.CommitAndPublish(ctx, s.publishBulkNodeForInsert); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeForUpdate(ctx context.Context, req *connect.Request[flowv1.NodeForUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	// 1. Move validation OUTSIDE transaction (before BeginTx)
	type updateData struct {
		nodeID   idwrap.IDWrap
		updated  mflow.NodeFor
		baseNode *mflow.Node
	}
	var validatedItems []updateData

	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		baseNode, err := s.ensureNodeAccess(ctx, nodeID)
		if err != nil {
			return nil, err
		}

		existing, err := s.nfs.GetNodeFor(ctx, nodeID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if existing == nil {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("node %s does not have FOR config", nodeID.String()))
		}

		// Update iterations if provided
		if item.Iterations != nil {
			existing.IterCount = int64(item.GetIterations())
		}

		// Update condition if provided
		if item.Condition != nil {
			existing.Condition = buildCondition(item.GetCondition())
		}

		// Update error handling if provided
		if item.ErrorHandling != nil {
			existing.ErrorHandling = mflow.ErrorHandling(item.GetErrorHandling()) // nolint:gosec // G115
		}

		validatedItems = append(validatedItems, updateData{
			nodeID:   nodeID,
			updated:  *existing,
			baseNode: baseNode,
		})
	}

	// 2. Begin transaction with bulk sync wrapper
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	syncTx := txutil.NewBulkUpdateTx[nodeForWithFlow, nodeForPatch, ForTopic](
		tx,
		func(nfwf nodeForWithFlow) ForTopic {
			return ForTopic{FlowID: nfwf.flowID}
		},
	)

	nfsWriter := s.nfs.TX(tx)

	// 3. Execute all updates in transaction
	for _, data := range validatedItems {
		if err := nfsWriter.UpdateNodeFor(ctx, data.updated); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		syncTx.Track(
			nodeForWithFlow{
				nodeFor: data.updated,
				flowID:  data.baseNode.FlowID,
			},
			nodeForPatch{}, // Empty patch - not used for NodeFor
		)
	}

	// 4. Commit transaction and publish events in bulk
	if err := syncTx.CommitAndPublish(ctx, s.publishBulkNodeForUpdate); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeForDelete(ctx context.Context, req *connect.Request[flowv1.NodeForDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	// 1. Move validation OUTSIDE transaction (before BeginTx)
	type deleteData struct {
		nodeID      idwrap.IDWrap
		existingFor *mflow.NodeFor
		baseNode    *mflow.Node
	}
	var validatedItems []deleteData

	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		baseNode, err := s.ensureNodeAccess(ctx, nodeID)
		if err != nil {
			return nil, err
		}

		// Get existing For node to publish delete event
		existingFor, err := s.nfs.GetNodeFor(ctx, nodeID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		validatedItems = append(validatedItems, deleteData{
			nodeID:      nodeID,
			existingFor: existingFor,
			baseNode:    baseNode,
		})
	}

	// 2. Begin transaction
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	nfsWriter := s.nfs.TX(tx)
	var deletedNodes []nodeForWithFlow

	// 3. Execute all deletes in transaction
	for _, data := range validatedItems {
		if err := nfsWriter.DeleteNodeFor(ctx, data.nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if data.existingFor != nil {
			deletedNodes = append(deletedNodes, nodeForWithFlow{
				nodeFor: *data.existingFor,
				flowID:  data.baseNode.FlowID,
			})
		}
	}

	// 4. Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// 5. Publish events AFTER successful commit
	for _, node := range deletedNodes {
		s.publishForEvent(forEventDelete, node.flowID, node.nodeFor)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeForSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[flowv1.NodeForSyncResponse],
) error {
	if stream == nil {
		return connect.NewError(connect.CodeInternal, errors.New("stream is required"))
	}
	return s.streamNodeForSync(ctx, func(resp *flowv1.NodeForSyncResponse) error {
		return stream.Send(resp)
	})
}

func (s *FlowServiceV2RPC) streamNodeForSync(
	ctx context.Context,
	send func(*flowv1.NodeForSyncResponse) error,
) error {
	if s.forStream == nil {
		return connect.NewError(connect.CodeUnavailable, errors.New("for stream not configured"))
	}

	var flowSet sync.Map

	snapshot := func(ctx context.Context) ([]eventstream.Event[ForTopic, ForEvent], error) {
		flows, err := s.listAccessibleFlows(ctx)
		if err != nil {
			return nil, err
		}

		events := make([]eventstream.Event[ForTopic, ForEvent], 0)

		for _, flow := range flows {
			flowSet.Store(flow.ID.String(), struct{}{})

			// Get all nodes in the flow and filter for For nodes
			nodes, err := s.ns.GetNodesByFlowID(ctx, flow.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return nil, err
			}

			for _, node := range nodes {
				// Only process For nodes
				if node.NodeKind != mflow.NODE_KIND_FOR {
					continue
				}

				// Skip start nodes (For nodes shouldn't be start nodes, but just in case)
				if isStartNode(node) {
					continue
				}

				// Get the For configuration for this node
				forNode, err := s.nfs.GetNodeFor(ctx, node.ID)
				if err != nil {
					return nil, err
				}

				if forNode == nil {
					continue
				}

				forPB := serializeNodeFor(*forNode)
				events = append(events, eventstream.Event[ForTopic, ForEvent]{
					Topic: ForTopic{FlowID: flow.ID},
					Payload: ForEvent{
						Type:   forEventInsert,
						FlowID: flow.ID,
						Node:   forPB,
					},
				})
			}
		}

		return events, nil
	}

	filter := func(topic ForTopic) bool {
		if _, ok := flowSet.Load(topic.FlowID.String()); ok {
			return true
		}
		if err := s.ensureFlowAccess(ctx, topic.FlowID); err != nil {
			return false
		}
		flowSet.Store(topic.FlowID.String(), struct{}{})
		return true
	}

	events, err := s.forStream.Subscribe(ctx, filter, eventstream.WithSnapshot(snapshot))
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	for {
		select {
		case evt, ok := <-events:
			if !ok {
				return nil
			}
			resp := forEventToSyncResponse(evt.Payload)
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

func (s *FlowServiceV2RPC) publishForEvent(eventType string, flowID idwrap.IDWrap, node mflow.NodeFor) {
	if s.forStream == nil {
		return
	}

	nodePB := serializeNodeFor(node)
	s.forStream.Publish(ForTopic{FlowID: flowID}, ForEvent{
		Type:   eventType,
		FlowID: flowID,
		Node:   nodePB,
	})
}

func forEventToSyncResponse(evt ForEvent) *flowv1.NodeForSyncResponse {
	if evt.Node == nil {
		return nil
	}

	node := evt.Node

	switch evt.Type {
	case forEventInsert:
		insert := &flowv1.NodeForSyncInsert{
			NodeId:        node.GetNodeId(),
			Iterations:    node.GetIterations(),
			Condition:     node.GetCondition(),
			ErrorHandling: node.GetErrorHandling(),
		}
		return &flowv1.NodeForSyncResponse{
			Items: []*flowv1.NodeForSync{{
				Value: &flowv1.NodeForSync_ValueUnion{
					Kind:   flowv1.NodeForSync_ValueUnion_KIND_INSERT,
					Insert: insert,
				},
			}},
		}
	case forEventUpdate:
		update := &flowv1.NodeForSyncUpdate{
			NodeId: node.GetNodeId(),
		}
		// Always include iterations - zero is a valid value
		iterations := node.GetIterations()
		update.Iterations = &iterations
		if condition := node.GetCondition(); condition != "" {
			update.Condition = &condition
		}
		if errorHandling := node.GetErrorHandling(); errorHandling != flowv1.ErrorHandling_ERROR_HANDLING_UNSPECIFIED {
			update.ErrorHandling = &errorHandling
		}
		return &flowv1.NodeForSyncResponse{
			Items: []*flowv1.NodeForSync{{
				Value: &flowv1.NodeForSync_ValueUnion{
					Kind:   flowv1.NodeForSync_ValueUnion_KIND_UPDATE,
					Update: update,
				},
			}},
		}
	case forEventDelete:
		return &flowv1.NodeForSyncResponse{
			Items: []*flowv1.NodeForSync{{
				Value: &flowv1.NodeForSync_ValueUnion{
					Kind: flowv1.NodeForSync_ValueUnion_KIND_DELETE,
					Delete: &flowv1.NodeForSyncDelete{
						NodeId: node.GetNodeId(),
					},
				},
			}},
		}
	default:
		return nil
	}
}
