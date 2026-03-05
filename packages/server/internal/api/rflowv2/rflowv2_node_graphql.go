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

// NodeGraphQLTopic identifies the flow whose GraphQL nodes are being published.
type NodeGraphQLTopic struct {
	FlowID idwrap.IDWrap
}

// NodeGraphQLEvent describes a GraphQL node change for sync streaming.
type NodeGraphQLEvent struct {
	Type   string
	FlowID idwrap.IDWrap
	Node   *flowv1.NodeGraphQL
}

func (s *FlowServiceV2RPC) NodeGraphQLCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[flowv1.NodeGraphQLCollectionResponse], error) {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	var items []*flowv1.NodeGraphQL
	for _, flow := range flows {
		nodes, err := s.nsReader.GetNodesByFlowID(ctx, flow.ID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, node := range nodes {
			if node.NodeKind != mflow.NODE_KIND_GRAPHQL {
				continue
			}
			nodeGQL, err := s.ngqs.GetNodeGraphQL(ctx, node.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			items = append(items, serializeNodeGraphQL(*nodeGQL))
		}
	}

	return connect.NewResponse(&flowv1.NodeGraphQLCollectionResponse{Items: items}), nil
}

func (s *FlowServiceV2RPC) NodeGraphQLInsert(
	ctx context.Context,
	req *connect.Request[flowv1.NodeGraphQLInsertRequest],
) (*connect.Response[emptypb.Empty], error) {
	type insertData struct {
		nodeID      idwrap.IDWrap
		graphqlID   *idwrap.IDWrap
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

		var graphqlID *idwrap.IDWrap
		if len(item.GetGraphqlId()) > 0 {
			parsedID, err := idwrap.NewFromBytes(item.GetGraphqlId())
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid graphql id: %w", err))
			}
			if !isZeroID(parsedID) {
				graphqlID = &parsedID
			}
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
			graphqlID:   graphqlID,
			baseNode:    baseNode,
			flowID:      flowID,
			workspaceID: workspaceID,
		})
	}

	if len(validatedItems) == 0 {
		return connect.NewResponse(&emptypb.Empty{}), nil
	}

	// Begin transaction with mutation context
	mut := mutation.New(s.DB, mutation.WithPublisher(s.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	ngqsWriter := s.ngqs.TX(mut.TX())

	for _, data := range validatedItems {
		nodeGraphQL := mflow.NodeGraphQL{
			FlowNodeID: data.nodeID,
			GraphQLID:  data.graphqlID,
		}

		if err := ngqsWriter.CreateNodeGraphQL(ctx, nodeGraphQL); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Only track for event publishing if base node exists
		if data.baseNode != nil {
			mut.Track(mutation.Event{
				Entity:      mutation.EntityFlowNodeGraphQL,
				Op:          mutation.OpInsert,
				ID:          data.nodeID,
				WorkspaceID: data.workspaceID,
				ParentID:    data.flowID,
				Payload: nodeGraphQLWithFlow{
					nodeGraphQL: nodeGraphQL,
					flowID:      data.flowID,
					baseNode:    data.baseNode,
				},
			})
		}
	}

	// Commit transaction (auto-publishes events)
	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeGraphQLUpdate(
	ctx context.Context,
	req *connect.Request[flowv1.NodeGraphQLUpdateRequest],
) (*connect.Response[emptypb.Empty], error) {
	type updateData struct {
		nodeID      idwrap.IDWrap
		graphqlID   *idwrap.IDWrap
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

		var graphqlID *idwrap.IDWrap
		if graphqlBytes := item.GetGraphqlId(); len(graphqlBytes) > 0 {
			parsedID, err := idwrap.NewFromBytes(graphqlBytes)
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid graphql id: %w", err))
			}
			if !isZeroID(parsedID) {
				graphqlID = &parsedID
			}
		}

		validatedItems = append(validatedItems, updateData{
			nodeID:      nodeID,
			graphqlID:   graphqlID,
			baseNode:    nodeModel,
			workspaceID: flow.WorkspaceID,
		})
	}

	if len(validatedItems) == 0 {
		return connect.NewResponse(&emptypb.Empty{}), nil
	}

	// Begin transaction with mutation context
	mut := mutation.New(s.DB, mutation.WithPublisher(s.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	ngqsWriter := s.ngqs.TX(mut.TX())

	for _, data := range validatedItems {
		nodeGraphQL := mflow.NodeGraphQL{
			FlowNodeID: data.nodeID,
			GraphQLID:  data.graphqlID,
		}

		if err := ngqsWriter.UpdateNodeGraphQL(ctx, nodeGraphQL); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		mut.Track(mutation.Event{
			Entity:      mutation.EntityFlowNodeGraphQL,
			Op:          mutation.OpUpdate,
			ID:          data.nodeID,
			WorkspaceID: data.workspaceID,
			ParentID:    data.baseNode.FlowID,
			Payload: nodeGraphQLWithFlow{
				nodeGraphQL: nodeGraphQL,
				flowID:      data.baseNode.FlowID,
				baseNode:    data.baseNode,
			},
		})
	}

	// Commit transaction (auto-publishes events)
	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeGraphQLDelete(
	ctx context.Context,
	req *connect.Request[flowv1.NodeGraphQLDeleteRequest],
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

	// Begin transaction with mutation context
	mut := mutation.New(s.DB, mutation.WithPublisher(s.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	for _, data := range validatedItems {
		mut.Track(mutation.Event{
			Entity:   mutation.EntityFlowNodeGraphQL,
			Op:       mutation.OpDelete,
			ID:       data.nodeID,
			ParentID: data.flowID,
		})
		if err := mut.Queries().DeleteFlowNodeGraphQL(ctx, data.nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	// Commit transaction (auto-publishes events)
	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeGraphQLSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[flowv1.NodeGraphQLSyncResponse],
) error {
	if stream == nil {
		return connect.NewError(connect.CodeInternal, errors.New("stream is required"))
	}
	return s.streamNodeGraphQLSync(ctx, func(resp *flowv1.NodeGraphQLSyncResponse) error {
		return stream.Send(resp)
	})
}

func (s *FlowServiceV2RPC) streamNodeGraphQLSync(
	ctx context.Context,
	send func(*flowv1.NodeGraphQLSyncResponse) error,
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
			resp, err := s.nodeGraphQLEventToSyncResponse(ctx, evt.Payload)
			if err != nil {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to convert GraphQL node event: %w", err))
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

func (s *FlowServiceV2RPC) nodeGraphQLEventToSyncResponse(
	ctx context.Context,
	evt NodeEvent,
) (*flowv1.NodeGraphQLSyncResponse, error) {
	if evt.Node == nil {
		return nil, nil
	}

	// Only process GraphQL nodes
	if evt.Node.GetKind() != flowv1.NodeKind_NODE_KIND_GRAPH_Q_L {
		return nil, nil
	}

	nodeID, err := idwrap.NewFromBytes(evt.Node.GetNodeId())
	if err != nil {
		return nil, fmt.Errorf("invalid node id: %w", err)
	}

	// Fetch the GraphQL configuration for this node (may not exist)
	nodeGQL, err := s.ngqs.GetNodeGraphQL(ctx, nodeID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	var syncEvent *flowv1.NodeGraphQLSync
	switch evt.Type {
	case nodeEventInsert:
		insert := &flowv1.NodeGraphQLSyncInsert{
			NodeId: nodeID.Bytes(),
		}
		if nodeGQL != nil && nodeGQL.GraphQLID != nil && !isZeroID(*nodeGQL.GraphQLID) {
			insert.GraphqlId = nodeGQL.GraphQLID.Bytes()
		}
		syncEvent = &flowv1.NodeGraphQLSync{
			Value: &flowv1.NodeGraphQLSync_ValueUnion{
				Kind:   flowv1.NodeGraphQLSync_ValueUnion_KIND_INSERT,
				Insert: insert,
			},
		}
	case nodeEventUpdate:
		update := &flowv1.NodeGraphQLSyncUpdate{
			NodeId: nodeID.Bytes(),
		}
		if nodeGQL != nil && nodeGQL.GraphQLID != nil && !isZeroID(*nodeGQL.GraphQLID) {
			update.GraphqlId = nodeGQL.GraphQLID.Bytes()
		}
		syncEvent = &flowv1.NodeGraphQLSync{
			Value: &flowv1.NodeGraphQLSync_ValueUnion{
				Kind:   flowv1.NodeGraphQLSync_ValueUnion_KIND_UPDATE,
				Update: update,
			},
		}
	case nodeEventDelete:
		syncEvent = &flowv1.NodeGraphQLSync{
			Value: &flowv1.NodeGraphQLSync_ValueUnion{
				Kind: flowv1.NodeGraphQLSync_ValueUnion_KIND_DELETE,
				Delete: &flowv1.NodeGraphQLSyncDelete{
					NodeId: nodeID.Bytes(),
				},
			},
		}
	default:
		return nil, nil
	}

	return &flowv1.NodeGraphQLSyncResponse{
		Items: []*flowv1.NodeGraphQLSync{syncEvent},
	}, nil
}
