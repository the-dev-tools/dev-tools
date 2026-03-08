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

type nodeSubFlowReturnWithFlow struct {
	nodeSubFlowReturn mflow.NodeSubFlowReturn
	flowID            idwrap.IDWrap
	baseNode          *mflow.Node
}

func (s *FlowServiceV2RPC) NodeSubFlowReturnCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[flowv1.NodeSubFlowReturnCollectionResponse], error) {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	var items []*flowv1.NodeSubFlowReturn
	for _, flow := range flows {
		nodes, err := s.nsReader.GetNodesByFlowID(ctx, flow.ID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, node := range nodes {
			if node.NodeKind != mflow.NODE_KIND_SUB_FLOW_RETURN {
				continue
			}
			nodeSubFlowReturn, err := s.nsfrs.GetNodeSubFlowReturn(ctx, node.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			if nodeSubFlowReturn == nil {
				continue
			}
			items = append(items, serializeNodeSubFlowReturn(*nodeSubFlowReturn))
		}
	}

	return connect.NewResponse(&flowv1.NodeSubFlowReturnCollectionResponse{Items: items}), nil
}

func (s *FlowServiceV2RPC) NodeSubFlowReturnInsert(
	ctx context.Context,
	req *connect.Request[flowv1.NodeSubFlowReturnInsertRequest],
) (*connect.Response[emptypb.Empty], error) {
	type insertData struct {
		nodeID      idwrap.IDWrap
		outputs     []mflow.SubFlowOutput
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
			outputs:     protoToSubFlowOutputs(item.GetOutputs()),
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

	nsfrsWriter := s.nsfrs.TX(mut.TX())

	for _, data := range validatedItems {
		nodeSubFlowReturn := mflow.NodeSubFlowReturn{
			FlowNodeID: data.nodeID,
			Outputs:    data.outputs,
		}

		if err := nsfrsWriter.CreateNodeSubFlowReturn(ctx, nodeSubFlowReturn); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if data.baseNode != nil {
			mut.Track(mutation.Event{
				Entity:      mutation.EntityFlowNodeSubFlowReturn,
				Op:          mutation.OpInsert,
				ID:          data.nodeID,
				WorkspaceID: data.workspaceID,
				ParentID:    data.flowID,
				Payload: nodeSubFlowReturnWithFlow{
					nodeSubFlowReturn: nodeSubFlowReturn,
					flowID:            data.flowID,
					baseNode:          data.baseNode,
				},
			})
		}
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeSubFlowReturnUpdate(
	ctx context.Context,
	req *connect.Request[flowv1.NodeSubFlowReturnUpdateRequest],
) (*connect.Response[emptypb.Empty], error) {
	type updateData struct {
		nodeID      idwrap.IDWrap
		outputs     []mflow.SubFlowOutput
		baseNode    *mflow.Node
		workspaceID idwrap.IDWrap
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

		existing, err := s.nsfrs.GetNodeSubFlowReturn(ctx, nodeID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		outputs := existing.Outputs
		if item.Outputs != nil {
			outputs = protoToSubFlowOutputs(item.Outputs)
		}

		validatedItems = append(validatedItems, updateData{
			nodeID:      nodeID,
			outputs:     outputs,
			baseNode:    nodeModel,
			workspaceID: flow.WorkspaceID,
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

	nsfrsWriter := s.nsfrs.TX(mut.TX())

	for _, data := range validatedItems {
		nodeSubFlowReturn := mflow.NodeSubFlowReturn{
			FlowNodeID: data.nodeID,
			Outputs:    data.outputs,
		}

		if err := nsfrsWriter.UpdateNodeSubFlowReturn(ctx, nodeSubFlowReturn); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		mut.Track(mutation.Event{
			Entity:      mutation.EntityFlowNodeSubFlowReturn,
			Op:          mutation.OpUpdate,
			ID:          data.nodeID,
			WorkspaceID: data.workspaceID,
			ParentID:    data.baseNode.FlowID,
			Payload: nodeSubFlowReturnWithFlow{
				nodeSubFlowReturn: nodeSubFlowReturn,
				flowID:            data.baseNode.FlowID,
				baseNode:          data.baseNode,
			},
		})
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeSubFlowReturnDelete(
	ctx context.Context,
	req *connect.Request[flowv1.NodeSubFlowReturnDeleteRequest],
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
			Entity:   mutation.EntityFlowNodeSubFlowReturn,
			Op:       mutation.OpDelete,
			ID:       data.nodeID,
			ParentID: data.flowID,
		})
		if err := mut.Queries().DeleteFlowNodeSubFlowReturn(ctx, data.nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeSubFlowReturnSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[flowv1.NodeSubFlowReturnSyncResponse],
) error {
	if stream == nil {
		return connect.NewError(connect.CodeInternal, errors.New("stream is required"))
	}
	return s.streamNodeSubFlowReturnSync(ctx, func(resp *flowv1.NodeSubFlowReturnSyncResponse) error {
		return stream.Send(resp)
	})
}

func (s *FlowServiceV2RPC) streamNodeSubFlowReturnSync(
	ctx context.Context,
	send func(*flowv1.NodeSubFlowReturnSyncResponse) error,
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
			resp, err := s.nodeSubFlowReturnEventToSyncResponse(ctx, evt.Payload)
			if err != nil {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to convert sub flow return node event: %w", err))
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

func (s *FlowServiceV2RPC) nodeSubFlowReturnEventToSyncResponse(
	ctx context.Context,
	evt NodeEvent,
) (*flowv1.NodeSubFlowReturnSyncResponse, error) {
	if evt.Node == nil {
		return nil, nil
	}

	if evt.Node.GetKind() != flowv1.NodeKind_NODE_KIND_SUB_FLOW_RETURN {
		return nil, nil
	}

	nodeID, err := idwrap.NewFromBytes(evt.Node.GetNodeId())
	if err != nil {
		return nil, fmt.Errorf("invalid node id: %w", err)
	}

	nodeSubFlowReturn, err := s.nsfrs.GetNodeSubFlowReturn(ctx, nodeID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	var syncEvent *flowv1.NodeSubFlowReturnSync
	switch evt.Type {
	case nodeEventInsert:
		if nodeSubFlowReturn == nil {
			return nil, nil
		}
		syncEvent = &flowv1.NodeSubFlowReturnSync{
			Value: &flowv1.NodeSubFlowReturnSync_ValueUnion{
				Kind: flowv1.NodeSubFlowReturnSync_ValueUnion_KIND_INSERT,
				Insert: &flowv1.NodeSubFlowReturnSyncInsert{
					NodeId:  nodeID.Bytes(),
					Outputs: subFlowOutputsToProto(nodeSubFlowReturn.Outputs),
				},
			},
		}
	case nodeEventUpdate:
		if nodeSubFlowReturn == nil {
			return nil, nil
		}
		syncEvent = &flowv1.NodeSubFlowReturnSync{
			Value: &flowv1.NodeSubFlowReturnSync_ValueUnion{
				Kind: flowv1.NodeSubFlowReturnSync_ValueUnion_KIND_UPDATE,
				Update: &flowv1.NodeSubFlowReturnSyncUpdate{
					NodeId:  nodeID.Bytes(),
					Outputs: subFlowOutputsToProto(nodeSubFlowReturn.Outputs),
				},
			},
		}
	case nodeEventDelete:
		syncEvent = &flowv1.NodeSubFlowReturnSync{
			Value: &flowv1.NodeSubFlowReturnSync_ValueUnion{
				Kind: flowv1.NodeSubFlowReturnSync_ValueUnion_KIND_DELETE,
				Delete: &flowv1.NodeSubFlowReturnSyncDelete{
					NodeId: nodeID.Bytes(),
				},
			},
		}
	default:
		return nil, nil
	}

	return &flowv1.NodeSubFlowReturnSyncResponse{
		Items: []*flowv1.NodeSubFlowReturnSync{syncEvent},
	}, nil
}

func serializeNodeSubFlowReturn(n mflow.NodeSubFlowReturn) *flowv1.NodeSubFlowReturn {
	return &flowv1.NodeSubFlowReturn{
		NodeId:  n.FlowNodeID.Bytes(),
		Outputs: subFlowOutputsToProto(n.Outputs),
	}
}

func subFlowOutputsToProto(outputs []mflow.SubFlowOutput) []*flowv1.SubFlowOutput {
	if len(outputs) == 0 {
		return nil
	}
	result := make([]*flowv1.SubFlowOutput, len(outputs))
	for i, o := range outputs {
		result[i] = &flowv1.SubFlowOutput{
			Name:       o.Name,
			Expression: o.Expression,
		}
	}
	return result
}

func protoToSubFlowOutputs(outputs []*flowv1.SubFlowOutput) []mflow.SubFlowOutput {
	if len(outputs) == 0 {
		return nil
	}
	result := make([]mflow.SubFlowOutput, len(outputs))
	for i, o := range outputs {
		result[i] = mflow.SubFlowOutput{
			Name:       o.GetName(),
			Expression: o.GetExpression(),
		}
	}
	return result
}
