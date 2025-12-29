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

	devtoolsdb "the-dev-tools/db"
	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/txutil"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
)

// --- JS Node ---

func (s *FlowServiceV2RPC) NodeJsCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[flowv1.NodeJsCollectionResponse], error) {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]*flowv1.NodeJs, 0)

	for _, flow := range flows {
		nodes, err := s.ns.GetNodesByFlowID(ctx, flow.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, n := range nodes {
			if n.NodeKind != mflow.NODE_KIND_JS {
				continue
			}
			nodeJs, err := s.njss.GetNodeJS(ctx, n.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			if nodeJs == nil {
				continue
			}
			items = append(items, serializeNodeJs(*nodeJs))
		}
	}

	return connect.NewResponse(&flowv1.NodeJsCollectionResponse{Items: items}), nil
}

func (s *FlowServiceV2RPC) NodeJsInsert(ctx context.Context, req *connect.Request[flowv1.NodeJsInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	// 1. Move validation OUTSIDE transaction (before BeginTx)
	type insertData struct {
		nodeID   idwrap.IDWrap
		model    mflow.NodeJS
		baseNode *mflow.Node
		flowID   idwrap.IDWrap
	}
	var validatedItems []insertData

	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		model := mflow.NodeJS{
			FlowNodeID:       nodeID,
			Code:             []byte(item.GetCode()),
			CodeCompressType: compress.CompressTypeNone,
		}

		// CRITICAL FIX: Get base node BEFORE transaction to avoid SQLite deadlock
		// Allow nil baseNode to support out-of-order message arrival
		baseNode, _ := s.ns.GetNode(ctx, nodeID)

		var flowID idwrap.IDWrap
		if baseNode != nil {
			flowID = baseNode.FlowID
		}

		validatedItems = append(validatedItems, insertData{
			nodeID:   nodeID,
			model:    model,
			baseNode: baseNode,
			flowID:   flowID,
		})
	}

	// 2. Begin transaction with bulk sync wrapper
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	syncTx := txutil.NewBulkInsertTx[nodeJsWithFlow, NodeTopic](
		tx,
		func(njwf nodeJsWithFlow) NodeTopic {
			return NodeTopic{FlowID: njwf.flowID}
		},
	)

	njssWriter := s.njss.TX(tx)

	// 3. Execute all inserts in transaction
	for _, data := range validatedItems {
		if err := njssWriter.CreateNodeJS(ctx, data.model); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Only track for event publishing if base node exists
		if data.baseNode != nil {
			syncTx.Track(nodeJsWithFlow{
				nodeJS:   data.model,
				flowID:   data.flowID,
				baseNode: data.baseNode,
			})
		}
	}

	// 4. Commit transaction and publish events in bulk
	if err := syncTx.CommitAndPublish(ctx, s.publishBulkNodeJsInsert); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeJsUpdate(ctx context.Context, req *connect.Request[flowv1.NodeJsUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	// 1. Move validation OUTSIDE transaction (before BeginTx)
	type updateData struct {
		nodeID   idwrap.IDWrap
		updated  mflow.NodeJS
		baseNode *mflow.Node
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

		existing, err := s.njss.GetNodeJS(ctx, nodeID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if existing == nil {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("node %s does not have JS config", nodeID.String()))
		}

		if item.Code != nil {
			existing.Code = []byte(item.GetCode())
		}

		validatedItems = append(validatedItems, updateData{
			nodeID:   nodeID,
			updated:  *existing,
			baseNode: baseNode,
		})
	}

	// 2. Begin transaction with bulk sync wrapper
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	syncTx := txutil.NewBulkUpdateTx[nodeJsWithFlow, nodeJsPatch, NodeTopic](
		tx,
		func(njwf nodeJsWithFlow) NodeTopic {
			return NodeTopic{FlowID: njwf.flowID}
		},
	)

	njssWriter := s.njss.TX(tx)

	// 3. Execute all updates in transaction
	for _, data := range validatedItems {
		if err := njssWriter.UpdateNodeJS(ctx, data.updated); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		syncTx.Track(
			nodeJsWithFlow{
				nodeJS:   data.updated,
				flowID:   data.baseNode.FlowID,
				baseNode: data.baseNode,
			},
			nodeJsPatch{}, // Empty patch - not used for NodeJS
		)
	}

	// 4. Commit transaction and publish events in bulk
	if err := syncTx.CommitAndPublish(ctx, s.publishBulkNodeJsUpdate); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeJsDelete(ctx context.Context, req *connect.Request[flowv1.NodeJsDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	// 1. Move validation OUTSIDE transaction (before BeginTx)
	type deleteData struct {
		nodeID   idwrap.IDWrap
		baseNode *mflow.Node
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
			nodeID:   nodeID,
			baseNode: baseNode,
		})
	}

	// 2. Begin transaction
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	njssWriter := s.njss.TX(tx)
	var deletedNodes []*mflow.Node

	// 3. Execute all deletes in transaction
	for _, data := range validatedItems {
		if err := njssWriter.DeleteNodeJS(ctx, data.nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		deletedNodes = append(deletedNodes, data.baseNode)
	}

	// 4. Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// 5. Publish events AFTER successful commit
	for _, node := range deletedNodes {
		s.publishNodeEvent(nodeEventUpdate, *node)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeJsSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[flowv1.NodeJsSyncResponse],
) error {
	if stream == nil {
		return connect.NewError(connect.CodeInternal, errors.New("stream is required"))
	}
	return s.streamNodeJsSync(ctx, func(resp *flowv1.NodeJsSyncResponse) error {
		return stream.Send(resp)
	})
}

func (s *FlowServiceV2RPC) streamNodeJsSync(
	ctx context.Context,
	send func(*flowv1.NodeJsSyncResponse) error,
) error {
	if s.jsStream == nil {
		return connect.NewError(connect.CodeUnavailable, errors.New("js stream not configured"))
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
			resp, err := s.jsEventToSyncResponse(ctx, evt.Payload)
			if err != nil {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to convert JS node event: %w", err))
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

func (s *FlowServiceV2RPC) jsEventToSyncResponse(
	ctx context.Context,
	evt NodeEvent,
) (*flowv1.NodeJsSyncResponse, error) {
	if evt.Node == nil {
		return nil, nil
	}

	// Only process JS nodes
	if evt.Node.GetKind() != flowv1.NodeKind_NODE_KIND_JS {
		return nil, nil
	}

	nodeID, err := idwrap.NewFromBytes(evt.Node.GetNodeId())
	if err != nil {
		return nil, fmt.Errorf("invalid node id: %w", err)
	}

	// Fetch the JavaScript configuration for this node
	nodeJs, err := s.njss.GetNodeJS(ctx, nodeID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	var syncEvent *flowv1.NodeJsSync
	switch evt.Type {
	case nodeEventInsert:
		if nodeJs == nil {
			return nil, nil
		}
		syncEvent = &flowv1.NodeJsSync{
			Value: &flowv1.NodeJsSync_ValueUnion{
				Kind: flowv1.NodeJsSync_ValueUnion_KIND_INSERT,
				Insert: &flowv1.NodeJsSyncInsert{
					NodeId: nodeJs.FlowNodeID.Bytes(),
					Code:   string(nodeJs.Code),
				},
			},
		}
	case nodeEventUpdate:
		update := &flowv1.NodeJsSyncUpdate{
			NodeId: nodeID.Bytes(),
		}
		if nodeJs != nil {
			code := string(nodeJs.Code)
			update.Code = &code
		}
		syncEvent = &flowv1.NodeJsSync{
			Value: &flowv1.NodeJsSync_ValueUnion{
				Kind:   flowv1.NodeJsSync_ValueUnion_KIND_UPDATE,
				Update: update,
			},
		}
	case nodeEventDelete:
		syncEvent = &flowv1.NodeJsSync{
			Value: &flowv1.NodeJsSync_ValueUnion{
				Kind: flowv1.NodeJsSync_ValueUnion_KIND_DELETE,
				Delete: &flowv1.NodeJsSyncDelete{
					NodeId: nodeID.Bytes(),
				},
			},
		}
	default:
		return nil, nil
	}

	return &flowv1.NodeJsSyncResponse{
		Items: []*flowv1.NodeJsSync{syncEvent},
	}, nil
}
