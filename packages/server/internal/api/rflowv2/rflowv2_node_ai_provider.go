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

// --- AI Provider Node ---

func serializeNodeAiProvider(m mflow.NodeAiProvider) *flowv1.NodeAiProvider {
	return &flowv1.NodeAiProvider{
		NodeId:       m.FlowNodeID.Bytes(),
		CredentialId: m.CredentialID.Bytes(),
		Model:        flowv1.AiModel(m.Model),
		Temperature:  m.Temperature,
		MaxTokens:    m.MaxTokens,
	}
}

func (s *FlowServiceV2RPC) NodeAiProviderCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[flowv1.NodeAiProviderCollectionResponse], error) {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	var items []*flowv1.NodeAiProvider
	for _, flow := range flows {
		nodes, err := s.nsReader.GetNodesByFlowID(ctx, flow.ID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, node := range nodes {
			if node.NodeKind != mflow.NODE_KIND_AI_PROVIDER {
				continue
			}
			nodeAiProvider, err := s.naps.GetNodeAiProvider(ctx, node.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			items = append(items, serializeNodeAiProvider(*nodeAiProvider))
		}
	}

	return connect.NewResponse(&flowv1.NodeAiProviderCollectionResponse{Items: items}), nil
}

func (s *FlowServiceV2RPC) NodeAiProviderInsert(
	ctx context.Context,
	req *connect.Request[flowv1.NodeAiProviderInsertRequest],
) (*connect.Response[emptypb.Empty], error) {
	type insertData struct {
		nodeID      idwrap.IDWrap
		provider    mflow.NodeAiProvider
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

		provider := mflow.NodeAiProvider{
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
			provider:    provider,
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

	napsWriter := s.naps.TX(mut.TX())

	for _, data := range validatedItems {
		if err := napsWriter.CreateNodeAiProvider(ctx, data.provider); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if data.baseNode != nil {
			mut.Track(mutation.Event{
				Entity:      mutation.EntityFlowNodeAiProvider,
				Op:          mutation.OpInsert,
				ID:          data.nodeID,
				WorkspaceID: data.workspaceID,
				ParentID:    data.flowID,
				Payload:     data.provider,
			})
		}
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeAiProviderUpdate(
	ctx context.Context,
	req *connect.Request[flowv1.NodeAiProviderUpdateRequest],
) (*connect.Response[emptypb.Empty], error) {
	type updateData struct {
		nodeID      idwrap.IDWrap
		updated     mflow.NodeAiProvider
		baseNode    *mflow.Node
		workspaceID idwrap.IDWrap
	}
	var validatedItems []updateData

	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		// Get existing provider
		existing, err := s.naps.GetNodeAiProvider(ctx, nodeID)
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
			case flowv1.NodeAiProviderUpdate_TemperatureUnion_KIND_VALUE:
				updated.Temperature = item.Temperature.Value
			case flowv1.NodeAiProviderUpdate_TemperatureUnion_KIND_UNSET:
				updated.Temperature = nil
			}
		}

		// Handle max_tokens union
		if item.MaxTokens != nil {
			switch item.MaxTokens.Kind {
			case flowv1.NodeAiProviderUpdate_MaxTokensUnion_KIND_VALUE:
				updated.MaxTokens = item.MaxTokens.Value
			case flowv1.NodeAiProviderUpdate_MaxTokensUnion_KIND_UNSET:
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

	napsWriter := s.naps.TX(mut.TX())

	for _, data := range validatedItems {
		if err := napsWriter.UpdateNodeAiProvider(ctx, data.updated); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if data.baseNode != nil {
			mut.Track(mutation.Event{
				Entity:      mutation.EntityFlowNodeAiProvider,
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

func (s *FlowServiceV2RPC) NodeAiProviderDelete(
	ctx context.Context,
	req *connect.Request[flowv1.NodeAiProviderDeleteRequest],
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

	napsWriter := s.naps.TX(mut.TX())

	for _, data := range validatedItems {
		if err := napsWriter.DeleteNodeAiProvider(ctx, data.nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		mut.Track(mutation.Event{
			Entity:      mutation.EntityFlowNodeAiProvider,
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

// AiProviderTopic identifies the flow whose AI Provider nodes are being published.
type AiProviderTopic struct {
	FlowID idwrap.IDWrap
}

// AiProviderEvent describes an AI Provider node change for sync streaming.
type AiProviderEvent struct {
	Type   string
	FlowID idwrap.IDWrap
	Node   *flowv1.NodeAiProvider
}

func (s *FlowServiceV2RPC) NodeAiProviderSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[flowv1.NodeAiProviderSyncResponse],
) error {
	return s.streamNodeAiProviderSync(ctx, stream.Send)
}

func (s *FlowServiceV2RPC) streamNodeAiProviderSync(
	ctx context.Context,
	send func(*flowv1.NodeAiProviderSyncResponse) error,
) error {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return err
	}

	// Build initial collection
	var items []*flowv1.NodeAiProvider
	for _, flow := range flows {
		nodes, err := s.nsReader.GetNodesByFlowID(ctx, flow.ID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return connect.NewError(connect.CodeInternal, err)
		}
		for _, node := range nodes {
			if node.NodeKind != mflow.NODE_KIND_AI_PROVIDER {
				continue
			}
			nodeAiProvider, err := s.naps.GetNodeAiProvider(ctx, node.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return connect.NewError(connect.CodeInternal, err)
			}
			items = append(items, serializeNodeAiProvider(*nodeAiProvider))
		}
	}

	// Convert to sync items format
	syncItems := make([]*flowv1.NodeAiProviderSync, 0, len(items))
	for _, item := range items {
		syncItems = append(syncItems, &flowv1.NodeAiProviderSync{
			Value: &flowv1.NodeAiProviderSync_ValueUnion{
				Kind: flowv1.NodeAiProviderSync_ValueUnion_KIND_UPSERT,
				Upsert: &flowv1.NodeAiProviderSyncUpsert{
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
	if err := send(&flowv1.NodeAiProviderSyncResponse{
		Items: syncItems,
	}); err != nil {
		return err
	}

	// Real-time streaming: subscribe to AI Provider node events
	if s.aiProviderStream == nil {
		// No streamer available, wait for context cancellation
		<-ctx.Done()
		return nil
	}

	// Build set of accessible flow IDs for filtering
	flowIDSet := make(map[string]bool, len(flows))
	for _, flow := range flows {
		flowIDSet[flow.ID.String()] = true
	}

	// Subscribe to AI Provider node changes
	eventCh, err := s.aiProviderStream.Subscribe(ctx, func(topic AiProviderTopic) bool {
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

			var syncItem *flowv1.NodeAiProviderSync
			switch evt.Payload.Type {
			case eventTypeInsert, eventTypeUpdate:
				syncItem = &flowv1.NodeAiProviderSync{
					Value: &flowv1.NodeAiProviderSync_ValueUnion{
						Kind: flowv1.NodeAiProviderSync_ValueUnion_KIND_UPSERT,
						Upsert: &flowv1.NodeAiProviderSyncUpsert{
							NodeId:       evt.Payload.Node.NodeId,
							CredentialId: evt.Payload.Node.CredentialId,
							Model:        evt.Payload.Node.Model,
							Temperature:  evt.Payload.Node.Temperature,
							MaxTokens:    evt.Payload.Node.MaxTokens,
						},
					},
				}
			case eventTypeDelete:
				syncItem = &flowv1.NodeAiProviderSync{
					Value: &flowv1.NodeAiProviderSync_ValueUnion{
						Kind:   flowv1.NodeAiProviderSync_ValueUnion_KIND_DELETE,
						Delete: &flowv1.NodeAiProviderSyncDelete{NodeId: evt.Payload.Node.NodeId},
					},
				}
			}

			if syncItem != nil {
				if err := send(&flowv1.NodeAiProviderSyncResponse{
					Items: []*flowv1.NodeAiProviderSync{syncItem},
				}); err != nil {
					return err
				}
			}
		}
	}
}
