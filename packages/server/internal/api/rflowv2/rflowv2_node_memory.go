//nolint:revive // exported
package rflowv2

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/mutation"
	flowv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/flow/v1"
)

// --- Memory Node ---

func serializeNodeMemory(m mflow.NodeMemory) *flowv1.NodeAiMemory {
	return &flowv1.NodeAiMemory{
		NodeId:     m.FlowNodeID.Bytes(),
		MemoryType: flowv1.AiMemoryType(m.MemoryType),
		WindowSize: m.WindowSize,
	}
}

func (s *FlowServiceV2RPC) NodeAiMemoryCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[flowv1.NodeAiMemoryCollectionResponse], error) {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	var items []*flowv1.NodeAiMemory
	for _, flow := range flows {
		nodes, err := s.nsReader.GetNodesByFlowID(ctx, flow.ID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, node := range nodes {
			if node.NodeKind != mflow.NODE_KIND_AI_MEMORY {
				continue
			}
			nodeMemory, err := s.nmems.GetNodeMemory(ctx, node.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			items = append(items, serializeNodeMemory(*nodeMemory))
		}
	}

	return connect.NewResponse(&flowv1.NodeAiMemoryCollectionResponse{Items: items}), nil
}

func (s *FlowServiceV2RPC) NodeAiMemoryInsert(
	ctx context.Context,
	req *connect.Request[flowv1.NodeAiMemoryInsertRequest],
) (*connect.Response[emptypb.Empty], error) {
	type insertData struct {
		nodeID      idwrap.IDWrap
		model       mflow.NodeMemory
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

		model := mflow.NodeMemory{
			FlowNodeID: nodeID,
			MemoryType: mflow.AiMemoryType(int8(item.GetMemoryType())), //nolint:gosec // G115: MemoryType is a small enum
			WindowSize: item.GetWindowSize(),
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
			model:       model,
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

	nmemsWriter := s.nmems.TX(mut.TX())

	for _, data := range validatedItems {
		if err := nmemsWriter.CreateNodeMemory(ctx, data.model); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if data.baseNode != nil {
			mut.Track(mutation.Event{
				Entity:      mutation.EntityFlowNodeMemory,
				Op:          mutation.OpInsert,
				ID:          data.nodeID,
				WorkspaceID: data.workspaceID,
				ParentID:    data.flowID,
				Payload:     data.model,
			})
		}
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeAiMemoryUpdate(
	ctx context.Context,
	req *connect.Request[flowv1.NodeAiMemoryUpdateRequest],
) (*connect.Response[emptypb.Empty], error) {
	type updateData struct {
		nodeID      idwrap.IDWrap
		updated     mflow.NodeMemory
		baseNode    *mflow.Node
		workspaceID idwrap.IDWrap
	}
	var validatedItems []updateData

	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		// Get existing model
		existing, err := s.nmems.GetNodeMemory(ctx, nodeID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		updated := *existing

		// Apply optional updates
		if item.MemoryType != nil {
			updated.MemoryType = mflow.AiMemoryType(int8(*item.MemoryType)) //nolint:gosec // G115: MemoryType is a small enum
		}

		if item.WindowSize != nil {
			updated.WindowSize = *item.WindowSize
		}

		baseNode, _ := s.ns.GetNode(ctx, nodeID)

		var workspaceID idwrap.IDWrap
		if baseNode != nil {
			flow, err := s.fsReader.GetFlow(ctx, baseNode.FlowID)
			if err == nil {
				workspaceID = flow.WorkspaceID
			}
		}

		validatedItems = append(validatedItems, updateData{
			nodeID:      nodeID,
			updated:     updated,
			baseNode:    baseNode,
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

	nmemsWriter := s.nmems.TX(mut.TX())

	for _, data := range validatedItems {
		if err := nmemsWriter.UpdateNodeMemory(ctx, data.updated); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if data.baseNode != nil {
			mut.Track(mutation.Event{
				Entity:      mutation.EntityFlowNodeMemory,
				Op:          mutation.OpUpdate,
				ID:          data.nodeID,
				WorkspaceID: data.workspaceID,
				ParentID:    data.baseNode.FlowID,
				Payload:     data.updated,
			})
		}
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeAiMemoryDelete(
	ctx context.Context,
	req *connect.Request[flowv1.NodeAiMemoryDeleteRequest],
) (*connect.Response[emptypb.Empty], error) {
	type deleteData struct {
		nodeID      idwrap.IDWrap
		flowID      idwrap.IDWrap
		workspaceID idwrap.IDWrap
	}
	var validatedItems []deleteData

	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		baseNode, _ := s.ns.GetNode(ctx, nodeID)
		var flowID, workspaceID idwrap.IDWrap
		if baseNode != nil {
			flowID = baseNode.FlowID
			flow, err := s.fsReader.GetFlow(ctx, baseNode.FlowID)
			if err == nil {
				workspaceID = flow.WorkspaceID
			}
		}

		validatedItems = append(validatedItems, deleteData{
			nodeID:      nodeID,
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

	nmemsWriter := s.nmems.TX(mut.TX())

	for _, data := range validatedItems {
		if err := nmemsWriter.DeleteNodeMemory(ctx, data.nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		mut.Track(mutation.Event{
			Entity:      mutation.EntityFlowNodeMemory,
			Op:          mutation.OpDelete,
			ID:          data.nodeID,
			ParentID:    data.flowID,
			WorkspaceID: data.workspaceID,
		})
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// MemoryTopic identifies the flow whose Memory nodes are being published.
type MemoryTopic struct {
	FlowID idwrap.IDWrap
}

// MemoryEvent describes a Memory node change for sync streaming.
type MemoryEvent struct {
	Type   string
	FlowID idwrap.IDWrap
	Node   *flowv1.NodeAiMemory
}

func (s *FlowServiceV2RPC) NodeAiMemorySync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[flowv1.NodeAiMemorySyncResponse],
) error {
	return s.streamNodeMemorySync(ctx, stream.Send)
}

func (s *FlowServiceV2RPC) streamNodeMemorySync(
	ctx context.Context,
	send func(*flowv1.NodeAiMemorySyncResponse) error,
) error {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return err
	}

	// Build initial collection
	var items []*flowv1.NodeAiMemory
	for _, flow := range flows {
		nodes, err := s.nsReader.GetNodesByFlowID(ctx, flow.ID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return connect.NewError(connect.CodeInternal, err)
		}
		for _, node := range nodes {
			if node.NodeKind != mflow.NODE_KIND_AI_MEMORY {
				continue
			}
			nodeMemory, err := s.nmems.GetNodeMemory(ctx, node.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return connect.NewError(connect.CodeInternal, err)
			}
			items = append(items, serializeNodeMemory(*nodeMemory))
		}
	}

	// Convert to sync items format
	syncItems := make([]*flowv1.NodeAiMemorySync, 0, len(items))
	for _, item := range items {
		syncItems = append(syncItems, &flowv1.NodeAiMemorySync{
			Value: &flowv1.NodeAiMemorySync_ValueUnion{
				Kind: flowv1.NodeAiMemorySync_ValueUnion_KIND_UPSERT,
				Upsert: &flowv1.NodeAiMemorySyncUpsert{
					NodeId:     item.NodeId,
					MemoryType: item.MemoryType,
					WindowSize: item.WindowSize,
				},
			},
		})
	}

	// Send initial collection as upsert items
	if err := send(&flowv1.NodeAiMemorySyncResponse{
		Items: syncItems,
	}); err != nil {
		return err
	}

	// Real-time streaming: subscribe to Memory node events
	if s.memoryStream == nil {
		// No streamer available, wait for context cancellation
		<-ctx.Done()
		return nil
	}

	// Build set of accessible flow IDs for filtering
	flowIDSet := make(map[string]bool, len(flows))
	for _, flow := range flows {
		flowIDSet[flow.ID.String()] = true
	}

	// Subscribe to Memory node changes
	eventCh, err := s.memoryStream.Subscribe(ctx, func(topic MemoryTopic) bool {
		return flowIDSet[topic.FlowID.String()]
	})
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	// Stream events as they come
	for {
		select {
		case <-ctx.Done():
			return nil
		case evt, ok := <-eventCh:
			if !ok {
				return nil
			}

			var syncItem *flowv1.NodeAiMemorySync
			switch evt.Payload.Type {
			case eventTypeInsert, eventTypeUpdate:
				syncItem = &flowv1.NodeAiMemorySync{
					Value: &flowv1.NodeAiMemorySync_ValueUnion{
						Kind: flowv1.NodeAiMemorySync_ValueUnion_KIND_UPSERT,
						Upsert: &flowv1.NodeAiMemorySyncUpsert{
							NodeId:     evt.Payload.Node.NodeId,
							MemoryType: evt.Payload.Node.MemoryType,
							WindowSize: evt.Payload.Node.WindowSize,
						},
					},
				}
			case eventTypeDelete:
				syncItem = &flowv1.NodeAiMemorySync{
					Value: &flowv1.NodeAiMemorySync_ValueUnion{
						Kind:   flowv1.NodeAiMemorySync_ValueUnion_KIND_DELETE,
						Delete: &flowv1.NodeAiMemorySyncDelete{NodeId: evt.Payload.Node.NodeId},
					},
				}
			}

			if syncItem != nil {
				if err := send(&flowv1.NodeAiMemorySyncResponse{
					Items: []*flowv1.NodeAiMemorySync{syncItem},
				}); err != nil {
					return err
				}
			}
		}
	}
}
