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

func (s *FlowServiceV2RPC) NodeHttpCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[flowv1.NodeHttpCollectionResponse], error) {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]*flowv1.NodeHttp, 0)

	for _, flow := range flows {
		nodes, err := s.ns.GetNodesByFlowID(ctx, flow.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, n := range nodes {
			if n.NodeKind != mflow.NODE_KIND_REQUEST {
				continue
			}
			nodeReq, err := s.nrs.GetNodeRequest(ctx, n.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					// No flow_node_http record exists, return node with just nodeId
					items = append(items, &flowv1.NodeHttp{
						NodeId: n.ID.Bytes(),
					})
					continue
				}
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			if nodeReq == nil {
				// No record, return node with just nodeId
				items = append(items, &flowv1.NodeHttp{
					NodeId: n.ID.Bytes(),
				})
				continue
			}
			items = append(items, serializeNodeHTTP(*nodeReq))
		}
	}

	return connect.NewResponse(&flowv1.NodeHttpCollectionResponse{Items: items}), nil
}

func (s *FlowServiceV2RPC) NodeHttpInsert(ctx context.Context, req *connect.Request[flowv1.NodeHttpInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	// 1. Move validation OUTSIDE transaction (before BeginTx)
	type insertData struct {
		nodeID      idwrap.IDWrap
		httpID      *idwrap.IDWrap
		deltaHttpID *idwrap.IDWrap
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

		var httpID *idwrap.IDWrap
		if len(item.GetHttpId()) > 0 {
			parsedID, err := idwrap.NewFromBytes(item.GetHttpId())
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid http id: %w", err))
			}
			if !isZeroID(parsedID) {
				httpID = &parsedID
			}
		}

		var deltaHttpID *idwrap.IDWrap
		if len(item.GetDeltaHttpId()) > 0 {
			parsedID, err := idwrap.NewFromBytes(item.GetDeltaHttpId())
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid delta http id: %w", err))
			}
			if !isZeroID(parsedID) {
				deltaHttpID = &parsedID
			}
		}

		// CRITICAL FIX: Get base node BEFORE transaction to avoid SQLite deadlock
		// Allow nil baseNode to support out-of-order message arrival
		baseNode, _ := s.ns.GetNode(ctx, nodeID)

		var flowID idwrap.IDWrap
		var workspaceID idwrap.IDWrap
		if baseNode != nil {
			flowID = baseNode.FlowID
			// Get workspace ID for the flow
			flow, err := s.fsReader.GetFlow(ctx, flowID)
			if err == nil {
				workspaceID = flow.WorkspaceID
			}
		}

		validatedItems = append(validatedItems, insertData{
			nodeID:      nodeID,
			httpID:      httpID,
			deltaHttpID: deltaHttpID,
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

	nrsWriter := s.nrs.TX(mut.TX())

	// 3. Execute all inserts in transaction
	for _, data := range validatedItems {
		nodeRequest := mflow.NodeRequest{
			FlowNodeID:       data.nodeID,
			HttpID:           data.httpID,
			DeltaHttpID:      data.deltaHttpID,
			HasRequestConfig: data.httpID != nil,
		}

		if err := nrsWriter.CreateNodeRequest(ctx, nodeRequest); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Only track for event publishing if base node exists
		if data.baseNode != nil {
			mut.Track(mutation.Event{
				Entity:      mutation.EntityFlowNodeHTTP,
				Op:          mutation.OpInsert,
				ID:          data.nodeID,
				WorkspaceID: data.workspaceID,
				ParentID:    data.flowID,
				Payload: nodeHttpWithFlow{
					nodeRequest: nodeRequest,
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

func (s *FlowServiceV2RPC) NodeHttpUpdate(ctx context.Context, req *connect.Request[flowv1.NodeHttpUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	// 1. Move validation OUTSIDE transaction (before BeginTx)
	type updateData struct {
		nodeID      idwrap.IDWrap
		httpID      *idwrap.IDWrap
		deltaHttpID *idwrap.IDWrap
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

		// Get workspace ID for the flow
		flow, err := s.fsReader.GetFlow(ctx, nodeModel.FlowID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		var httpID *idwrap.IDWrap
		if httpBytes := item.GetHttpId(); len(httpBytes) > 0 {
			parsedID, err := idwrap.NewFromBytes(httpBytes)
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid http id: %w", err))
			}
			if !isZeroID(parsedID) {
				httpID = &parsedID
			}
		}

		var deltaHttpID *idwrap.IDWrap
		deltaUnion := item.GetDeltaHttpId()
		if deltaUnion != nil && deltaUnion.Kind == flowv1.NodeHttpUpdate_DeltaHttpIdUnion_KIND_VALUE {
			parsedID, err := idwrap.NewFromBytes(deltaUnion.GetValue())
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid delta http id: %w", err))
			}
			if !isZeroID(parsedID) {
				deltaHttpID = &parsedID
			}
		}

		validatedItems = append(validatedItems, updateData{
			nodeID:      nodeID,
			httpID:      httpID,
			deltaHttpID: deltaHttpID,
			baseNode:    nodeModel,
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

	nrsWriter := s.nrs.TX(mut.TX())

	// 3. Execute all updates in transaction
	for _, data := range validatedItems {
		nodeRequest := mflow.NodeRequest{
			FlowNodeID:       data.nodeID,
			HttpID:           data.httpID,
			DeltaHttpID:      data.deltaHttpID,
			HasRequestConfig: data.httpID != nil,
		}

		if err := nrsWriter.UpdateNodeRequest(ctx, nodeRequest); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		mut.Track(mutation.Event{
			Entity:      mutation.EntityFlowNodeHTTP,
			Op:          mutation.OpUpdate,
			ID:          data.nodeID,
			WorkspaceID: data.workspaceID,
			ParentID:    data.baseNode.FlowID,
			Payload: nodeHttpWithFlow{
				nodeRequest: nodeRequest,
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

func (s *FlowServiceV2RPC) NodeHttpDelete(ctx context.Context, req *connect.Request[flowv1.NodeHttpDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
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

	// 2. Begin transaction with mutation context
	mut := mutation.New(s.DB, mutation.WithPublisher(s.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	// 3. Execute all deletes in transaction
	for _, data := range validatedItems {
		mut.Track(mutation.Event{
			Entity:   mutation.EntityFlowNodeHTTP,
			Op:       mutation.OpDelete,
			ID:       data.nodeID,
			ParentID: data.flowID,
		})
		if err := mut.Queries().DeleteFlowNodeHTTP(ctx, data.nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	// 4. Commit transaction (auto-publishes events)
	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeHttpSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[flowv1.NodeHttpSyncResponse],
) error {
	if stream == nil {
		return connect.NewError(connect.CodeInternal, errors.New("stream is required"))
	}
	return s.streamNodeHttpSync(ctx, func(resp *flowv1.NodeHttpSyncResponse) error {
		return stream.Send(resp)
	})
}

func (s *FlowServiceV2RPC) streamNodeHttpSync(
	ctx context.Context,
	send func(*flowv1.NodeHttpSyncResponse) error,
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
			resp, err := s.nodeHttpEventToSyncResponse(ctx, evt.Payload)
			if err != nil {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to convert HTTP node event: %w", err))
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

func (s *FlowServiceV2RPC) nodeHttpEventToSyncResponse(
	ctx context.Context,
	evt NodeEvent,
) (*flowv1.NodeHttpSyncResponse, error) {
	if evt.Node == nil {
		return nil, nil
	}

	// Only process HTTP nodes (REQUEST nodes)
	if evt.Node.GetKind() != flowv1.NodeKind_NODE_KIND_HTTP {
		return nil, nil
	}

	nodeID, err := idwrap.NewFromBytes(evt.Node.GetNodeId())
	if err != nil {
		return nil, fmt.Errorf("invalid node id: %w", err)
	}

	// Fetch the HTTP configuration for this node (may not exist)
	nodeReq, err := s.nrs.GetNodeRequest(ctx, nodeID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	var syncEvent *flowv1.NodeHttpSync
	switch evt.Type {
	case nodeEventInsert:
		insert := &flowv1.NodeHttpSyncInsert{
			NodeId: nodeID.Bytes(),
		}
		if nodeReq != nil && nodeReq.HttpID != nil && !isZeroID(*nodeReq.HttpID) {
			insert.HttpId = nodeReq.HttpID.Bytes()
		}
		if nodeReq != nil && nodeReq.DeltaHttpID != nil && !isZeroID(*nodeReq.DeltaHttpID) {
			insert.DeltaHttpId = nodeReq.DeltaHttpID.Bytes()
		}
		syncEvent = &flowv1.NodeHttpSync{
			Value: &flowv1.NodeHttpSync_ValueUnion{
				Kind:   flowv1.NodeHttpSync_ValueUnion_KIND_INSERT,
				Insert: insert,
			},
		}
	case nodeEventUpdate:
		update := &flowv1.NodeHttpSyncUpdate{
			NodeId: nodeID.Bytes(),
		}
		if nodeReq != nil && nodeReq.HttpID != nil && !isZeroID(*nodeReq.HttpID) {
			update.HttpId = nodeReq.HttpID.Bytes()
		}
		if nodeReq != nil && nodeReq.DeltaHttpID != nil && !isZeroID(*nodeReq.DeltaHttpID) {
			update.DeltaHttpId = &flowv1.NodeHttpSyncUpdate_DeltaHttpIdUnion{
				Kind:  flowv1.NodeHttpSyncUpdate_DeltaHttpIdUnion_KIND_VALUE,
				Value: nodeReq.DeltaHttpID.Bytes(),
			}
		}
		syncEvent = &flowv1.NodeHttpSync{
			Value: &flowv1.NodeHttpSync_ValueUnion{
				Kind:   flowv1.NodeHttpSync_ValueUnion_KIND_UPDATE,
				Update: update,
			},
		}
	case nodeEventDelete:
		syncEvent = &flowv1.NodeHttpSync{
			Value: &flowv1.NodeHttpSync_ValueUnion{
				Kind: flowv1.NodeHttpSync_ValueUnion_KIND_DELETE,
				Delete: &flowv1.NodeHttpSyncDelete{
					NodeId: nodeID.Bytes(),
				},
			},
		}
	default:
		return nil, nil
	}

	return &flowv1.NodeHttpSyncResponse{
		Items: []*flowv1.NodeHttpSync{syncEvent},
	}, nil
}
