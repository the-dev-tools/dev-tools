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

// --- Model Node ---

func serializeNodeModel(m mflow.NodeModel) *flowv1.NodeAiModel {
	return &flowv1.NodeAiModel{
		NodeId:       m.FlowNodeID.Bytes(),
		CredentialId: m.CredentialID.Bytes(),
		Model:        flowv1.AiModel(m.Model),
		Temperature:  m.Temperature,
		MaxTokens:    m.MaxTokens,
	}
}

func (s *FlowServiceV2RPC) NodeAiModelCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[flowv1.NodeAiModelCollectionResponse], error) {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	var items []*flowv1.NodeAiModel
	for _, flow := range flows {
		nodes, err := s.nsReader.GetNodesByFlowID(ctx, flow.ID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, node := range nodes {
			if node.NodeKind != mflow.NODE_KIND_AI_MODEL {
				continue
			}
			nodeModel, err := s.nms.GetNodeModel(ctx, node.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			items = append(items, serializeNodeModel(*nodeModel))
		}
	}

	return connect.NewResponse(&flowv1.NodeAiModelCollectionResponse{Items: items}), nil
}

func (s *FlowServiceV2RPC) NodeAiModelInsert(
	ctx context.Context,
	req *connect.Request[flowv1.NodeAiModelInsertRequest],
) (*connect.Response[emptypb.Empty], error) {
	type insertData struct {
		nodeID      idwrap.IDWrap
		model       mflow.NodeModel
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

		credID, err := idwrap.NewFromBytes(item.GetCredentialId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid credential id: %w", err))
		}

		model := mflow.NodeModel{
			FlowNodeID:   nodeID,
			CredentialID: credID,
			Model:        mflow.AiModel(int8(item.GetModel())), //nolint:gosec // G115: Model is a small enum
			Temperature:  item.Temperature,
			MaxTokens:    item.MaxTokens,
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

	nmsWriter := s.nms.TX(mut.TX())

	for _, data := range validatedItems {
		if err := nmsWriter.CreateNodeModel(ctx, data.model); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if data.baseNode != nil {
			mut.Track(mutation.Event{
				Entity:      mutation.EntityFlowNodeModel,
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

func (s *FlowServiceV2RPC) NodeAiModelUpdate(
	ctx context.Context,
	req *connect.Request[flowv1.NodeAiModelUpdateRequest],
) (*connect.Response[emptypb.Empty], error) {
	type updateData struct {
		nodeID      idwrap.IDWrap
		updated     mflow.NodeModel
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
		existing, err := s.nms.GetNodeModel(ctx, nodeID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		updated := *existing

		// Apply optional updates
		if item.CredentialId != nil {
			credID, err := idwrap.NewFromBytes(item.CredentialId)
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid credential id: %w", err))
			}
			updated.CredentialID = credID
		}

		if item.Model != nil {
			updated.Model = mflow.AiModel(int8(*item.Model)) //nolint:gosec // G115: Model is a small enum
		}

		// Handle temperature union
		if item.Temperature != nil {
			switch item.Temperature.Kind {
			case flowv1.NodeAiModelUpdate_TemperatureUnion_KIND_VALUE:
				updated.Temperature = item.Temperature.Value
			case flowv1.NodeAiModelUpdate_TemperatureUnion_KIND_UNSET:
				updated.Temperature = nil
			}
		}

		// Handle max_tokens union
		if item.MaxTokens != nil {
			switch item.MaxTokens.Kind {
			case flowv1.NodeAiModelUpdate_MaxTokensUnion_KIND_VALUE:
				updated.MaxTokens = item.MaxTokens.Value
			case flowv1.NodeAiModelUpdate_MaxTokensUnion_KIND_UNSET:
				updated.MaxTokens = nil
			}
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

	nmsWriter := s.nms.TX(mut.TX())

	for _, data := range validatedItems {
		if err := nmsWriter.UpdateNodeModel(ctx, data.updated); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if data.baseNode != nil {
			mut.Track(mutation.Event{
				Entity:      mutation.EntityFlowNodeModel,
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

func (s *FlowServiceV2RPC) NodeAiModelDelete(
	ctx context.Context,
	req *connect.Request[flowv1.NodeAiModelDeleteRequest],
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

		baseNode, _ := s.ns.GetNode(ctx, nodeID)
		var flowID idwrap.IDWrap
		if baseNode != nil {
			flowID = baseNode.FlowID
		}

		validatedItems = append(validatedItems, deleteData{
			nodeID: nodeID,
			flowID: flowID,
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

	nmsWriter := s.nms.TX(mut.TX())

	for _, data := range validatedItems {
		if err := nmsWriter.DeleteNodeModel(ctx, data.nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		mut.Track(mutation.Event{
			Entity:   mutation.EntityFlowNodeModel,
			Op:       mutation.OpDelete,
			ID:       data.nodeID,
			ParentID: data.flowID,
		})
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// ModelTopic identifies the flow whose Model nodes are being published.
type ModelTopic struct {
	FlowID idwrap.IDWrap
}

// ModelEvent describes a Model node change for sync streaming.
type ModelEvent struct {
	Type   string
	FlowID idwrap.IDWrap
	Node   *flowv1.NodeAiModel
}

func (s *FlowServiceV2RPC) NodeAiModelSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[flowv1.NodeAiModelSyncResponse],
) error {
	return s.streamNodeModelSync(ctx, stream.Send)
}

func (s *FlowServiceV2RPC) streamNodeModelSync(
	ctx context.Context,
	send func(*flowv1.NodeAiModelSyncResponse) error,
) error {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return err
	}

	// Build initial collection
	var items []*flowv1.NodeAiModel
	for _, flow := range flows {
		nodes, err := s.nsReader.GetNodesByFlowID(ctx, flow.ID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return connect.NewError(connect.CodeInternal, err)
		}
		for _, node := range nodes {
			if node.NodeKind != mflow.NODE_KIND_AI_MODEL {
				continue
			}
			nodeModel, err := s.nms.GetNodeModel(ctx, node.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return connect.NewError(connect.CodeInternal, err)
			}
			items = append(items, serializeNodeModel(*nodeModel))
		}
	}

	// Convert to sync items format
	syncItems := make([]*flowv1.NodeAiModelSync, 0, len(items))
	for _, item := range items {
		syncItems = append(syncItems, &flowv1.NodeAiModelSync{
			Value: &flowv1.NodeAiModelSync_ValueUnion{
				Kind: flowv1.NodeAiModelSync_ValueUnion_KIND_UPSERT,
				Upsert: &flowv1.NodeAiModelSyncUpsert{
					NodeId:       item.NodeId,
					CredentialId: item.CredentialId,
					Model:        item.Model,
					Temperature:  item.Temperature,
					MaxTokens:    item.MaxTokens,
				},
			},
		})
	}

	// Send initial collection as upsert items
	if err := send(&flowv1.NodeAiModelSyncResponse{
		Items: syncItems,
	}); err != nil {
		return err
	}

	// Real-time streaming: subscribe to Model node events
	if s.modelStream == nil {
		// No streamer available, wait for context cancellation
		<-ctx.Done()
		return nil
	}

	// Build set of accessible flow IDs for filtering
	flowIDSet := make(map[string]bool, len(flows))
	for _, flow := range flows {
		flowIDSet[flow.ID.String()] = true
	}

	// Subscribe to Model node changes
	eventCh, err := s.modelStream.Subscribe(ctx, func(topic ModelTopic) bool {
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

			var syncItem *flowv1.NodeAiModelSync
			switch evt.Payload.Type {
			case eventTypeInsert, eventTypeUpdate:
				syncItem = &flowv1.NodeAiModelSync{
					Value: &flowv1.NodeAiModelSync_ValueUnion{
						Kind: flowv1.NodeAiModelSync_ValueUnion_KIND_UPSERT,
						Upsert: &flowv1.NodeAiModelSyncUpsert{
							NodeId:       evt.Payload.Node.NodeId,
							CredentialId: evt.Payload.Node.CredentialId,
							Model:        evt.Payload.Node.Model,
							Temperature:  evt.Payload.Node.Temperature,
							MaxTokens:    evt.Payload.Node.MaxTokens,
						},
					},
				}
			case eventTypeDelete:
				syncItem = &flowv1.NodeAiModelSync{
					Value: &flowv1.NodeAiModelSync_ValueUnion{
						Kind:   flowv1.NodeAiModelSync_ValueUnion_KIND_DELETE,
						Delete: &flowv1.NodeAiModelSyncDelete{NodeId: evt.Payload.Node.NodeId},
					},
				}
			}

			if syncItem != nil {
				if err := send(&flowv1.NodeAiModelSyncResponse{
					Items: []*flowv1.NodeAiModelSync{syncItem},
				}); err != nil {
					return err
				}
			}
		}
	}
}
