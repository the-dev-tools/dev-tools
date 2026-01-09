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

	"github.com/the-dev-tools/dev-tools/packages/server/internal/converter"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/mutation"
	flowv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/flow/v1"
)

// --- ForEach Node ---

func (s *FlowServiceV2RPC) NodeForEachCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[flowv1.NodeForEachCollectionResponse], error) {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]*flowv1.NodeForEach, 0)

	for _, flow := range flows {
		nodes, err := s.ns.GetNodesByFlowID(ctx, flow.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, n := range nodes {
			if n.NodeKind != mflow.NODE_KIND_FOR_EACH {
				continue
			}
			nodeForEach, err := s.nfes.GetNodeForEach(ctx, n.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			if nodeForEach == nil {
				continue
			}
			items = append(items, serializeNodeForEach(*nodeForEach))
		}
	}

	return connect.NewResponse(&flowv1.NodeForEachCollectionResponse{Items: items}), nil
}

func (s *FlowServiceV2RPC) NodeForEachInsert(ctx context.Context, req *connect.Request[flowv1.NodeForEachInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	// 1. Move validation OUTSIDE transaction (before BeginTx)
	type insertData struct {
		nodeID      idwrap.IDWrap
		model       mflow.NodeForEach
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

		model := mflow.NodeForEach{
			FlowNodeID:     nodeID,
			IterExpression: item.GetPath(),
			Condition:      buildCondition(item.GetCondition()),
			ErrorHandling:  mflow.ErrorHandling(item.GetErrorHandling()), // nolint:gosec // G115
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

	nfesWriter := s.nfes.TX(mut.TX())

	// 3. Execute all inserts in transaction
	for _, data := range validatedItems {
		if err := nfesWriter.CreateNodeForEach(ctx, data.model); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Only track for event publishing if base node exists
		if data.baseNode != nil {
			mut.Track(mutation.Event{
				Entity:      mutation.EntityFlowNodeForEach,
				Op:          mutation.OpInsert,
				ID:          data.nodeID,
				WorkspaceID: data.workspaceID,
				ParentID:    data.flowID,
				Payload: nodeForEachWithFlow{
					nodeForEach: data.model,
					flowID:      data.flowID,
					baseNode:    data.baseNode,
				},
			})
		}
	}

	// 4. Commit transaction (auto-publishes events)
	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeForEachUpdate(ctx context.Context, req *connect.Request[flowv1.NodeForEachUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	// 1. Move validation OUTSIDE transaction (before BeginTx)
	type updateData struct {
		nodeID      idwrap.IDWrap
		updated     mflow.NodeForEach
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

		existing, err := s.nfes.GetNodeForEach(ctx, nodeID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if existing == nil {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("node %s does not have FOREACH config", nodeID.String()))
		}

		if item.Path != nil {
			existing.IterExpression = item.GetPath()
		}
		if item.Condition != nil {
			existing.Condition = buildCondition(item.GetCondition())
		}
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

	nfesWriter := s.nfes.TX(mut.TX())

	// 3. Execute all updates in transaction
	for _, data := range validatedItems {
		if err := nfesWriter.UpdateNodeForEach(ctx, data.updated); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		mut.Track(mutation.Event{
			Entity:      mutation.EntityFlowNodeForEach,
			Op:          mutation.OpUpdate,
			ID:          data.nodeID,
			WorkspaceID: data.workspaceID,
			ParentID:    data.baseNode.FlowID,
			Payload: nodeForEachWithFlow{
				nodeForEach: data.updated,
				flowID:      data.baseNode.FlowID,
				baseNode:    data.baseNode,
			},
		})
	}

	// 4. Commit transaction (auto-publishes events)
	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeForEachDelete(ctx context.Context, req *connect.Request[flowv1.NodeForEachDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
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
			Entity:   mutation.EntityFlowNodeForEach,
			Op:       mutation.OpDelete,
			ID:       data.nodeID,
			ParentID: data.flowID,
		})
		if err := mut.Queries().DeleteFlowNodeForEach(ctx, data.nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	// 4. Commit transaction (auto-publishes events)
	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeForEachSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[flowv1.NodeForEachSyncResponse],
) error {
	if stream == nil {
		return connect.NewError(connect.CodeInternal, errors.New("stream is required"))
	}
	return s.streamNodeForEachSync(ctx, func(resp *flowv1.NodeForEachSyncResponse) error {
		return stream.Send(resp)
	})
}

func (s *FlowServiceV2RPC) streamNodeForEachSync(
	ctx context.Context,
	send func(*flowv1.NodeForEachSyncResponse) error,
) error {
	if s.forEachStream == nil {
		return connect.NewError(connect.CodeUnavailable, errors.New("forEach stream not configured"))
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
			resp, err := s.forEachEventToSyncResponse(ctx, evt.Payload)
			if err != nil {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to convert ForEach node event: %w", err))
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

func (s *FlowServiceV2RPC) forEachEventToSyncResponse(
	ctx context.Context,
	evt NodeEvent,
) (*flowv1.NodeForEachSyncResponse, error) {
	if evt.Node == nil {
		return nil, nil
	}

	// Only process ForEach nodes
	if evt.Node.GetKind() != flowv1.NodeKind_NODE_KIND_FOR_EACH {
		return nil, nil
	}

	nodeID, err := idwrap.NewFromBytes(evt.Node.GetNodeId())
	if err != nil {
		return nil, fmt.Errorf("invalid node id: %w", err)
	}

	// Fetch the ForEach configuration for this node
	nodeForEach, err := s.nfes.GetNodeForEach(ctx, nodeID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	var syncEvent *flowv1.NodeForEachSync
	switch evt.Type {
	case nodeEventInsert:
		if nodeForEach == nil {
			return nil, nil
		}
		syncEvent = &flowv1.NodeForEachSync{
			Value: &flowv1.NodeForEachSync_ValueUnion{
				Kind: flowv1.NodeForEachSync_ValueUnion_KIND_INSERT,
				Insert: &flowv1.NodeForEachSyncInsert{
					NodeId:        nodeForEach.FlowNodeID.Bytes(),
					Path:          nodeForEach.IterExpression,
					Condition:     nodeForEach.Condition.Comparisons.Expression,
					ErrorHandling: converter.ToAPIErrorHandling(nodeForEach.ErrorHandling),
				},
			},
		}
	case nodeEventUpdate:
		update := &flowv1.NodeForEachSyncUpdate{
			NodeId: nodeID.Bytes(),
		}
		if nodeForEach != nil {
			// Include all fields in the update
			if path := nodeForEach.IterExpression; path != "" {
				update.Path = &path
			}
			if condition := nodeForEach.Condition.Comparisons.Expression; condition != "" {
				update.Condition = &condition
			}
			if errorHandling := converter.ToAPIErrorHandling(nodeForEach.ErrorHandling); errorHandling != flowv1.ErrorHandling_ERROR_HANDLING_UNSPECIFIED {
				update.ErrorHandling = &errorHandling
			}
		}
		syncEvent = &flowv1.NodeForEachSync{
			Value: &flowv1.NodeForEachSync_ValueUnion{
				Kind:   flowv1.NodeForEachSync_ValueUnion_KIND_UPDATE,
				Update: update,
			},
		}
	case nodeEventDelete:
		syncEvent = &flowv1.NodeForEachSync{
			Value: &flowv1.NodeForEachSync_ValueUnion{
				Kind: flowv1.NodeForEachSync_ValueUnion_KIND_DELETE,
				Delete: &flowv1.NodeForEachSyncDelete{
					NodeId: nodeID.Bytes(),
				},
			},
		}
	default:
		return nil, nil
	}

	return &flowv1.NodeForEachSyncResponse{
		Items: []*flowv1.NodeForEachSync{syncEvent},
	}, nil
}
