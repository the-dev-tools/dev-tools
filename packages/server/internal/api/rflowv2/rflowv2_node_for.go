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

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/mutation"
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
		nodeID      idwrap.IDWrap
		model       mflow.NodeFor
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
			model:       model,
			baseNode:    baseNode,
			flowID:      flowID,
			workspaceID: workspaceID,
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

	nfsWriter := s.nfs.TX(mut.TX())

	// 3. Execute all inserts in transaction
	for _, data := range validatedItems {
		if err := nfsWriter.CreateNodeFor(ctx, data.model); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Only track for event publishing if base node exists
		if data.baseNode != nil {
			mut.Track(mutation.Event{
				Entity:      mutation.EntityFlowNodeFor,
				Op:          mutation.OpInsert,
				ID:          data.nodeID,
				WorkspaceID: data.workspaceID,
				ParentID:    data.flowID,
				Payload:     data.model,
			})
		}
	}

	// 4. Commit transaction (auto-publishes events)
	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeForUpdate(ctx context.Context, req *connect.Request[flowv1.NodeForUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	// 1. Move validation OUTSIDE transaction (before BeginTx)
	type updateData struct {
		nodeID      idwrap.IDWrap
		updated     mflow.NodeFor
		baseNode    *mflow.Node
		workspaceID idwrap.IDWrap
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

		// Get workspace ID for the flow
		flow, err := s.fsReader.GetFlow(ctx, baseNode.FlowID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
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
			nodeID:      nodeID,
			updated:     *existing,
			baseNode:    baseNode,
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

	nfsWriter := s.nfs.TX(mut.TX())

	// 3. Execute all updates in transaction
	for _, data := range validatedItems {
		if err := nfsWriter.UpdateNodeFor(ctx, data.updated); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		mut.Track(mutation.Event{
			Entity:      mutation.EntityFlowNodeFor,
			Op:          mutation.OpUpdate,
			ID:          data.nodeID,
			WorkspaceID: data.workspaceID,
			ParentID:    data.baseNode.FlowID,
			Payload:     data.updated,
		})
	}

	// 4. Commit transaction (auto-publishes events)
	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeForDelete(ctx context.Context, req *connect.Request[flowv1.NodeForDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	// 1. Move validation OUTSIDE transaction (before BeginTx)
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

		baseNode, err := s.ensureNodeAccess(ctx, nodeID)
		if err != nil {
			return nil, err
		}

		validatedItems = append(validatedItems, deleteData{
			nodeID: nodeID,
			flowID: baseNode.FlowID,
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
			Entity:   mutation.EntityFlowNodeFor,
			Op:       mutation.OpDelete,
			ID:       data.nodeID,
			ParentID: data.flowID,
		})
		if err := mut.Queries().DeleteFlowNodeFor(ctx, data.nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	// 4. Commit transaction (auto-publishes events)
	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
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

	events, err := s.forStream.Subscribe(ctx, filter)
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