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

type nodeSubFlowTriggerWithFlow struct {
	nodeSubFlowTrigger mflow.NodeSubFlowTrigger
	flowID             idwrap.IDWrap
	baseNode           *mflow.Node
}

func (s *FlowServiceV2RPC) NodeSubFlowTriggerCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[flowv1.NodeSubFlowTriggerCollectionResponse], error) {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	var items []*flowv1.NodeSubFlowTrigger
	for _, flow := range flows {
		nodes, err := s.nsReader.GetNodesByFlowID(ctx, flow.ID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, node := range nodes {
			if node.NodeKind != mflow.NODE_KIND_SUB_FLOW_TRIGGER {
				continue
			}
			nodeSubFlowTrigger, err := s.nsfts.GetNodeSubFlowTrigger(ctx, node.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			if nodeSubFlowTrigger == nil {
				continue
			}
			items = append(items, serializeNodeSubFlowTrigger(*nodeSubFlowTrigger))
		}
	}

	return connect.NewResponse(&flowv1.NodeSubFlowTriggerCollectionResponse{Items: items}), nil
}

func (s *FlowServiceV2RPC) NodeSubFlowTriggerInsert(
	ctx context.Context,
	req *connect.Request[flowv1.NodeSubFlowTriggerInsertRequest],
) (*connect.Response[emptypb.Empty], error) {
	type insertData struct {
		nodeID      idwrap.IDWrap
		params      []mflow.SubFlowParam
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
			params:      protoToSubFlowParams(item.GetParams()),
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

	nsftsWriter := s.nsfts.TX(mut.TX())

	for _, data := range validatedItems {
		nodeSubFlowTrigger := mflow.NodeSubFlowTrigger{
			FlowNodeID: data.nodeID,
			Params:     data.params,
		}

		if err := nsftsWriter.CreateNodeSubFlowTrigger(ctx, nodeSubFlowTrigger); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if data.baseNode != nil {
			mut.Track(mutation.Event{
				Entity:      mutation.EntityFlowNodeSubFlowTrigger,
				Op:          mutation.OpInsert,
				ID:          data.nodeID,
				WorkspaceID: data.workspaceID,
				ParentID:    data.flowID,
				Payload: nodeSubFlowTriggerWithFlow{
					nodeSubFlowTrigger: nodeSubFlowTrigger,
					flowID:             data.flowID,
					baseNode:           data.baseNode,
				},
			})
		}
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeSubFlowTriggerUpdate(
	ctx context.Context,
	req *connect.Request[flowv1.NodeSubFlowTriggerUpdateRequest],
) (*connect.Response[emptypb.Empty], error) {
	type updateData struct {
		nodeID      idwrap.IDWrap
		params      []mflow.SubFlowParam
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

		existing, err := s.nsfts.GetNodeSubFlowTrigger(ctx, nodeID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		params := existing.Params
		if item.Params != nil {
			params = protoToSubFlowParams(item.Params)
		}

		validatedItems = append(validatedItems, updateData{
			nodeID:      nodeID,
			params:      params,
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

	nsftsWriter := s.nsfts.TX(mut.TX())

	for _, data := range validatedItems {
		nodeSubFlowTrigger := mflow.NodeSubFlowTrigger{
			FlowNodeID: data.nodeID,
			Params:     data.params,
		}

		if err := nsftsWriter.UpdateNodeSubFlowTrigger(ctx, nodeSubFlowTrigger); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		mut.Track(mutation.Event{
			Entity:      mutation.EntityFlowNodeSubFlowTrigger,
			Op:          mutation.OpUpdate,
			ID:          data.nodeID,
			WorkspaceID: data.workspaceID,
			ParentID:    data.baseNode.FlowID,
			Payload: nodeSubFlowTriggerWithFlow{
				nodeSubFlowTrigger: nodeSubFlowTrigger,
				flowID:             data.baseNode.FlowID,
				baseNode:           data.baseNode,
			},
		})
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeSubFlowTriggerDelete(
	ctx context.Context,
	req *connect.Request[flowv1.NodeSubFlowTriggerDeleteRequest],
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
			Entity:   mutation.EntityFlowNodeSubFlowTrigger,
			Op:       mutation.OpDelete,
			ID:       data.nodeID,
			ParentID: data.flowID,
		})
		if err := mut.Queries().DeleteFlowNodeSubFlowTrigger(ctx, data.nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeSubFlowTriggerSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[flowv1.NodeSubFlowTriggerSyncResponse],
) error {
	if stream == nil {
		return connect.NewError(connect.CodeInternal, errors.New("stream is required"))
	}
	return s.streamNodeSubFlowTriggerSync(ctx, func(resp *flowv1.NodeSubFlowTriggerSyncResponse) error {
		return stream.Send(resp)
	})
}

func (s *FlowServiceV2RPC) streamNodeSubFlowTriggerSync(
	ctx context.Context,
	send func(*flowv1.NodeSubFlowTriggerSyncResponse) error,
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
			resp, err := s.nodeSubFlowTriggerEventToSyncResponse(ctx, evt.Payload)
			if err != nil {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to convert sub flow trigger node event: %w", err))
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

func (s *FlowServiceV2RPC) nodeSubFlowTriggerEventToSyncResponse(
	ctx context.Context,
	evt NodeEvent,
) (*flowv1.NodeSubFlowTriggerSyncResponse, error) {
	if evt.Node == nil {
		return nil, nil
	}

	if evt.Node.GetKind() != flowv1.NodeKind_NODE_KIND_SUB_FLOW_TRIGGER {
		return nil, nil
	}

	nodeID, err := idwrap.NewFromBytes(evt.Node.GetNodeId())
	if err != nil {
		return nil, fmt.Errorf("invalid node id: %w", err)
	}

	nodeSubFlowTrigger, err := s.nsfts.GetNodeSubFlowTrigger(ctx, nodeID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	var syncEvent *flowv1.NodeSubFlowTriggerSync
	switch evt.Type {
	case nodeEventInsert:
		if nodeSubFlowTrigger == nil {
			return nil, nil
		}
		syncEvent = &flowv1.NodeSubFlowTriggerSync{
			Value: &flowv1.NodeSubFlowTriggerSync_ValueUnion{
				Kind: flowv1.NodeSubFlowTriggerSync_ValueUnion_KIND_INSERT,
				Insert: &flowv1.NodeSubFlowTriggerSyncInsert{
					NodeId: nodeID.Bytes(),
					Params: subFlowParamsToProto(nodeSubFlowTrigger.Params),
				},
			},
		}
	case nodeEventUpdate:
		if nodeSubFlowTrigger == nil {
			return nil, nil
		}
		syncEvent = &flowv1.NodeSubFlowTriggerSync{
			Value: &flowv1.NodeSubFlowTriggerSync_ValueUnion{
				Kind: flowv1.NodeSubFlowTriggerSync_ValueUnion_KIND_UPDATE,
				Update: &flowv1.NodeSubFlowTriggerSyncUpdate{
					NodeId: nodeID.Bytes(),
					Params: subFlowParamsToProto(nodeSubFlowTrigger.Params),
				},
			},
		}
	case nodeEventDelete:
		syncEvent = &flowv1.NodeSubFlowTriggerSync{
			Value: &flowv1.NodeSubFlowTriggerSync_ValueUnion{
				Kind: flowv1.NodeSubFlowTriggerSync_ValueUnion_KIND_DELETE,
				Delete: &flowv1.NodeSubFlowTriggerSyncDelete{
					NodeId: nodeID.Bytes(),
				},
			},
		}
	default:
		return nil, nil
	}

	return &flowv1.NodeSubFlowTriggerSyncResponse{
		Items: []*flowv1.NodeSubFlowTriggerSync{syncEvent},
	}, nil
}

func serializeNodeSubFlowTrigger(n mflow.NodeSubFlowTrigger) *flowv1.NodeSubFlowTrigger {
	return &flowv1.NodeSubFlowTrigger{
		NodeId: n.FlowNodeID.Bytes(),
		Params: subFlowParamsToProto(n.Params),
	}
}

func subFlowParamsToProto(params []mflow.SubFlowParam) []*flowv1.SubFlowParam {
	if len(params) == 0 {
		return nil
	}
	result := make([]*flowv1.SubFlowParam, len(params))
	for i, p := range params {
		result[i] = &flowv1.SubFlowParam{
			Name:         p.Name,
			Type:         p.Type,
			DefaultValue: p.DefaultValue,
			Required:     p.Required,
		}
	}
	return result
}

func protoToSubFlowParams(params []*flowv1.SubFlowParam) []mflow.SubFlowParam {
	if len(params) == 0 {
		return nil
	}
	result := make([]mflow.SubFlowParam, len(params))
	for i, p := range params {
		result[i] = mflow.SubFlowParam{
			Name:         p.GetName(),
			Type:         p.GetType(),
			DefaultValue: p.GetDefaultValue(),
			Required:     p.GetRequired(),
		}
	}
	return result
}
