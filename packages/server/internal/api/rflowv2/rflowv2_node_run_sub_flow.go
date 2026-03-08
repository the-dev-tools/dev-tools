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

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/mutation"
	flowv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/flow/v1"
)

type nodeRunSubFlowWithFlow struct {
	nodeRunSubFlow mflow.NodeRunSubFlow
	flowID         idwrap.IDWrap
	baseNode       *mflow.Node
}

func (s *FlowServiceV2RPC) NodeRunSubFlowCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[flowv1.NodeRunSubFlowCollectionResponse], error) {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	var items []*flowv1.NodeRunSubFlow
	for _, flow := range flows {
		nodes, err := s.nsReader.GetNodesByFlowID(ctx, flow.ID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, node := range nodes {
			if node.NodeKind != mflow.NODE_KIND_RUN_SUB_FLOW {
				continue
			}
			nodeRunSubFlow, err := s.nrsfs.GetNodeRunSubFlow(ctx, node.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			if nodeRunSubFlow == nil {
				continue
			}
			items = append(items, serializeNodeRunSubFlow(*nodeRunSubFlow))
		}
	}

	return connect.NewResponse(&flowv1.NodeRunSubFlowCollectionResponse{Items: items}), nil
}

func (s *FlowServiceV2RPC) NodeRunSubFlowInsert(
	ctx context.Context,
	req *connect.Request[flowv1.NodeRunSubFlowInsertRequest],
) (*connect.Response[emptypb.Empty], error) {
	type insertData struct {
		nodeID         idwrap.IDWrap
		targetFlowID   *idwrap.IDWrap
		targetFlowName string
		inputs         []mflow.SubFlowInputMapping
		baseNode       *mflow.Node
		flowID         idwrap.IDWrap
		workspaceID    idwrap.IDWrap
	}
	var validatedItems []insertData

	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
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

		var targetFlowID *idwrap.IDWrap
		if targetBytes := item.GetTargetFlowId(); len(targetBytes) > 0 {
			id, err := idwrap.NewFromBytes(targetBytes)
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid target flow id: %w", err))
			}
			targetFlowID = &id
		}

		validatedItems = append(validatedItems, insertData{
			nodeID:         nodeID,
			targetFlowID:   targetFlowID,
			targetFlowName: item.GetTargetFlowName(),
			inputs:         protoToSubFlowInputMappings(item.GetInputs()),
			baseNode:       baseNode,
			flowID:         flowID,
			workspaceID:    workspaceID,
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

	nrsfsWriter := s.nrsfs.TX(mut.TX())

	for _, data := range validatedItems {
		nodeRunSubFlow := mflow.NodeRunSubFlow{
			FlowNodeID:     data.nodeID,
			TargetFlowID:   data.targetFlowID,
			TargetFlowName: data.targetFlowName,
			Inputs:         data.inputs,
		}

		if err := nrsfsWriter.CreateNodeRunSubFlow(ctx, nodeRunSubFlow); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if data.baseNode != nil {
			mut.Track(mutation.Event{
				Entity:      mutation.EntityFlowNodeRunSubFlow,
				Op:          mutation.OpInsert,
				ID:          data.nodeID,
				WorkspaceID: data.workspaceID,
				ParentID:    data.flowID,
				Payload: nodeRunSubFlowWithFlow{
					nodeRunSubFlow: nodeRunSubFlow,
					flowID:         data.flowID,
					baseNode:       data.baseNode,
				},
			})
		}
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeRunSubFlowUpdate(
	ctx context.Context,
	req *connect.Request[flowv1.NodeRunSubFlowUpdateRequest],
) (*connect.Response[emptypb.Empty], error) {
	type updateData struct {
		nodeID         idwrap.IDWrap
		targetFlowID   *idwrap.IDWrap
		targetFlowName string
		inputs         []mflow.SubFlowInputMapping
		baseNode       *mflow.Node
		workspaceID    idwrap.IDWrap
	}
	var validatedItems []updateData

	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		nodeModel, err := s.ensureNodeAccess(ctx, nodeID)
		if err != nil {
			return nil, err
		}

		flow, err := s.fsReader.GetFlow(ctx, nodeModel.FlowID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		existing, err := s.nrsfs.GetNodeRunSubFlow(ctx, nodeID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		targetFlowID := existing.TargetFlowID
		if union := item.GetTargetFlowId(); union != nil {
			switch union.GetKind() {
			case flowv1.NodeRunSubFlowUpdate_TargetFlowIdUnion_KIND_UNSET:
				targetFlowID = nil
			case flowv1.NodeRunSubFlowUpdate_TargetFlowIdUnion_KIND_VALUE:
				if valueBytes := union.GetValue(); len(valueBytes) > 0 {
					id, err := idwrap.NewFromBytes(valueBytes)
					if err != nil {
						return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid target flow id: %w", err))
					}
					targetFlowID = &id
				}
			}
		}

		targetFlowName := existing.TargetFlowName
		if item.TargetFlowName != nil {
			targetFlowName = *item.TargetFlowName
		}

		inputs := existing.Inputs
		if item.Inputs != nil {
			inputs = protoToSubFlowInputMappings(item.Inputs)
		}

		validatedItems = append(validatedItems, updateData{
			nodeID:         nodeID,
			targetFlowID:   targetFlowID,
			targetFlowName: targetFlowName,
			inputs:         inputs,
			baseNode:       nodeModel,
			workspaceID:    flow.WorkspaceID,
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

	nrsfsWriter := s.nrsfs.TX(mut.TX())

	for _, data := range validatedItems {
		nodeRunSubFlow := mflow.NodeRunSubFlow{
			FlowNodeID:     data.nodeID,
			TargetFlowID:   data.targetFlowID,
			TargetFlowName: data.targetFlowName,
			Inputs:         data.inputs,
		}

		if err := nrsfsWriter.UpdateNodeRunSubFlow(ctx, nodeRunSubFlow); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		mut.Track(mutation.Event{
			Entity:      mutation.EntityFlowNodeRunSubFlow,
			Op:          mutation.OpUpdate,
			ID:          data.nodeID,
			WorkspaceID: data.workspaceID,
			ParentID:    data.baseNode.FlowID,
			Payload: nodeRunSubFlowWithFlow{
				nodeRunSubFlow: nodeRunSubFlow,
				flowID:         data.baseNode.FlowID,
				baseNode:       data.baseNode,
			},
		})
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeRunSubFlowDelete(
	ctx context.Context,
	req *connect.Request[flowv1.NodeRunSubFlowDeleteRequest],
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

		nodeModel, err := s.ensureNodeAccess(ctx, nodeID)
		if err != nil {
			return nil, err
		}

		validatedItems = append(validatedItems, deleteData{
			nodeID: nodeID,
			flowID: nodeModel.FlowID,
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

	for _, data := range validatedItems {
		mut.Track(mutation.Event{
			Entity:   mutation.EntityFlowNodeRunSubFlow,
			Op:       mutation.OpDelete,
			ID:       data.nodeID,
			ParentID: data.flowID,
		})
		if err := mut.Queries().DeleteFlowNodeRunSubFlow(ctx, data.nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeRunSubFlowSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[flowv1.NodeRunSubFlowSyncResponse],
) error {
	if stream == nil {
		return connect.NewError(connect.CodeInternal, errors.New("stream is required"))
	}
	return s.streamNodeRunSubFlowSync(ctx, func(resp *flowv1.NodeRunSubFlowSyncResponse) error {
		return stream.Send(resp)
	})
}

func (s *FlowServiceV2RPC) streamNodeRunSubFlowSync(
	ctx context.Context,
	send func(*flowv1.NodeRunSubFlowSyncResponse) error,
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
			resp, err := s.nodeRunSubFlowEventToSyncResponse(ctx, evt.Payload)
			if err != nil {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to convert run sub flow node event: %w", err))
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

func (s *FlowServiceV2RPC) nodeRunSubFlowEventToSyncResponse(
	ctx context.Context,
	evt NodeEvent,
) (*flowv1.NodeRunSubFlowSyncResponse, error) {
	if evt.Node == nil {
		return nil, nil
	}

	if evt.Node.GetKind() != flowv1.NodeKind_NODE_KIND_RUN_SUB_FLOW {
		return nil, nil
	}

	nodeID, err := idwrap.NewFromBytes(evt.Node.GetNodeId())
	if err != nil {
		return nil, fmt.Errorf("invalid node id: %w", err)
	}

	nodeRunSubFlow, err := s.nrsfs.GetNodeRunSubFlow(ctx, nodeID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	var syncEvent *flowv1.NodeRunSubFlowSync
	switch evt.Type {
	case nodeEventInsert:
		if nodeRunSubFlow == nil {
			return nil, nil
		}
		syncEvent = &flowv1.NodeRunSubFlowSync{
			Value: &flowv1.NodeRunSubFlowSync_ValueUnion{
				Kind: flowv1.NodeRunSubFlowSync_ValueUnion_KIND_INSERT,
				Insert: &flowv1.NodeRunSubFlowSyncInsert{
					NodeId:         nodeID.Bytes(),
					TargetFlowId:   idwrapPtrToBytes(nodeRunSubFlow.TargetFlowID),
					TargetFlowName: nodeRunSubFlow.TargetFlowName,
					Inputs:         subFlowInputMappingsToProto(nodeRunSubFlow.Inputs),
				},
			},
		}
	case nodeEventUpdate:
		if nodeRunSubFlow == nil {
			return nil, nil
		}
		var targetFlowIDUnion *flowv1.NodeRunSubFlowSyncUpdate_TargetFlowIdUnion
		if nodeRunSubFlow.TargetFlowID != nil {
			targetFlowIDUnion = &flowv1.NodeRunSubFlowSyncUpdate_TargetFlowIdUnion{
				Kind:  flowv1.NodeRunSubFlowSyncUpdate_TargetFlowIdUnion_KIND_VALUE,
				Value: nodeRunSubFlow.TargetFlowID.Bytes(),
			}
		}
		syncEvent = &flowv1.NodeRunSubFlowSync{
			Value: &flowv1.NodeRunSubFlowSync_ValueUnion{
				Kind: flowv1.NodeRunSubFlowSync_ValueUnion_KIND_UPDATE,
				Update: &flowv1.NodeRunSubFlowSyncUpdate{
					NodeId:         nodeID.Bytes(),
					TargetFlowId:   targetFlowIDUnion,
					TargetFlowName: &nodeRunSubFlow.TargetFlowName,
					Inputs:         subFlowInputMappingsToProto(nodeRunSubFlow.Inputs),
				},
			},
		}
	case nodeEventDelete:
		syncEvent = &flowv1.NodeRunSubFlowSync{
			Value: &flowv1.NodeRunSubFlowSync_ValueUnion{
				Kind: flowv1.NodeRunSubFlowSync_ValueUnion_KIND_DELETE,
				Delete: &flowv1.NodeRunSubFlowSyncDelete{
					NodeId: nodeID.Bytes(),
				},
			},
		}
	default:
		return nil, nil
	}

	return &flowv1.NodeRunSubFlowSyncResponse{
		Items: []*flowv1.NodeRunSubFlowSync{syncEvent},
	}, nil
}

func serializeNodeRunSubFlow(n mflow.NodeRunSubFlow) *flowv1.NodeRunSubFlow {
	return &flowv1.NodeRunSubFlow{
		NodeId:         n.FlowNodeID.Bytes(),
		TargetFlowId:   idwrapPtrToBytes(n.TargetFlowID),
		TargetFlowName: n.TargetFlowName,
		Inputs:         subFlowInputMappingsToProto(n.Inputs),
	}
}

func idwrapPtrToBytes(id *idwrap.IDWrap) []byte {
	if id == nil {
		return nil
	}
	return id.Bytes()
}

func subFlowInputMappingsToProto(inputs []mflow.SubFlowInputMapping) []*flowv1.SubFlowInputMapping {
	if len(inputs) == 0 {
		return nil
	}
	result := make([]*flowv1.SubFlowInputMapping, len(inputs))
	for i, m := range inputs {
		result[i] = &flowv1.SubFlowInputMapping{
			ParamName:  m.ParamName,
			Expression: m.Expression,
		}
	}
	return result
}

func protoToSubFlowInputMappings(inputs []*flowv1.SubFlowInputMapping) []mflow.SubFlowInputMapping {
	if len(inputs) == 0 {
		return nil
	}
	result := make([]mflow.SubFlowInputMapping, len(inputs))
	for i, m := range inputs {
		result[i] = mflow.SubFlowInputMapping{
			ParamName:  m.GetParamName(),
			Expression: m.GetExpression(),
		}
	}
	return result
}
