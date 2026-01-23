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

// --- AI Node ---

func (s *FlowServiceV2RPC) NodeAiCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[flowv1.NodeAiCollectionResponse], error) {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	var items []*flowv1.NodeAi
	for _, flow := range flows {
		nodes, err := s.nsReader.GetNodesByFlowID(ctx, flow.ID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, node := range nodes {
			if node.NodeKind != mflow.NODE_KIND_AI {
				continue
			}
			nodeAI, err := s.nais.GetNodeAI(ctx, node.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			items = append(items, serializeNodeAI(*nodeAI))
		}
	}

	return connect.NewResponse(&flowv1.NodeAiCollectionResponse{Items: items}), nil
}

func (s *FlowServiceV2RPC) NodeAiInsert(
	ctx context.Context,
	req *connect.Request[flowv1.NodeAiInsertRequest],
) (*connect.Response[emptypb.Empty], error) {
	type insertData struct {
		nodeID      idwrap.IDWrap
		model       mflow.NodeAI
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

		model := mflow.NodeAI{
			FlowNodeID:    nodeID,
			Prompt:        item.GetPrompt(),
			MaxIterations: item.GetMaxIterations(),
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

	naisWriter := s.nais.TX(mut.TX())

	for _, data := range validatedItems {
		if err := naisWriter.CreateNodeAI(ctx, data.model); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if data.baseNode != nil {
			mut.Track(mutation.Event{
				Entity:      mutation.EntityFlowNodeAI,
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

func (s *FlowServiceV2RPC) NodeAiUpdate(
	ctx context.Context,
	req *connect.Request[flowv1.NodeAiUpdateRequest],
) (*connect.Response[emptypb.Empty], error) {
	type updateData struct {
		nodeID      idwrap.IDWrap
		updated     mflow.NodeAI
		baseNode    *mflow.Node
		workspaceID idwrap.IDWrap
	}
	var validatedItems []updateData

	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		updated := mflow.NodeAI{
			FlowNodeID:    nodeID,
			Prompt:        item.GetPrompt(),
			MaxIterations: item.GetMaxIterations(),
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

	naisWriter := s.nais.TX(mut.TX())

	for _, data := range validatedItems {
		if err := naisWriter.UpdateNodeAI(ctx, data.updated); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if data.baseNode != nil {
			mut.Track(mutation.Event{
				Entity:      mutation.EntityFlowNodeAI,
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

func (s *FlowServiceV2RPC) NodeAiDelete(
	ctx context.Context,
	req *connect.Request[flowv1.NodeAiDeleteRequest],
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

	naisWriter := s.nais.TX(mut.TX())

	for _, data := range validatedItems {
		if err := naisWriter.DeleteNodeAI(ctx, data.nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		mut.Track(mutation.Event{
			Entity:      mutation.EntityFlowNodeAI,
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

// AiTopic identifies the flow whose AI nodes are being published.
type AiTopic struct {
	FlowID idwrap.IDWrap
}

// AiEvent describes an AI node change for sync streaming.
type AiEvent struct {
	Type   string
	FlowID idwrap.IDWrap
	Node   *flowv1.NodeAi
}

func (s *FlowServiceV2RPC) NodeAiSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[flowv1.NodeAiSyncResponse],
) error {
	return s.streamNodeAISync(ctx, stream.Send)
}

func (s *FlowServiceV2RPC) streamNodeAISync(
	ctx context.Context,
	send func(*flowv1.NodeAiSyncResponse) error,
) error {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return err
	}

	// Real-time streaming: subscribe to AI node events
	if s.aiStream == nil {
		// No streamer available, wait for context cancellation
		<-ctx.Done()
		return nil
	}

	// Build set of accessible flow IDs for filtering
	flowIDSet := make(map[string]bool, len(flows))
	for _, flow := range flows {
		flowIDSet[flow.ID.String()] = true
	}

	// Subscribe to AI node changes
	eventCh, err := s.aiStream.Subscribe(ctx, func(topic AiTopic) bool {
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

			var syncItem *flowv1.NodeAiSync
			switch evt.Payload.Type {
			case aiEventInsert, aiEventUpdate:
				syncItem = &flowv1.NodeAiSync{
					Value: &flowv1.NodeAiSync_ValueUnion{
						Kind: flowv1.NodeAiSync_ValueUnion_KIND_UPSERT,
						Upsert: &flowv1.NodeAiSyncUpsert{
							NodeId:        evt.Payload.Node.NodeId,
							Prompt:        evt.Payload.Node.Prompt,
							MaxIterations: evt.Payload.Node.MaxIterations,
						},
					},
				}
			case aiEventDelete:
				syncItem = &flowv1.NodeAiSync{
					Value: &flowv1.NodeAiSync_ValueUnion{
						Kind:   flowv1.NodeAiSync_ValueUnion_KIND_DELETE,
						Delete: &flowv1.NodeAiSyncDelete{NodeId: evt.Payload.Node.NodeId},
					},
				}
			}

			if syncItem != nil {
				if err := send(&flowv1.NodeAiSyncResponse{
					Items: []*flowv1.NodeAiSync{syncItem},
				}); err != nil {
					return err
				}
			}
		}
	}
}
