//nolint:revive // exported
package rflowv2

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

	devtoolsdb "github.com/the-dev-tools/dev-tools/packages/db"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rfile"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/converter"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/dbtime"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mfile"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/mutation"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sfile"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
	flowv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/flow/v1"
)

func (s *FlowServiceV2RPC) FlowCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[flowv1.FlowCollectionResponse], error) {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]*flowv1.Flow, 0, len(flows))
	for _, flow := range flows {
		items = append(items, serializeFlow(flow))
	}

	return connect.NewResponse(&flowv1.FlowCollectionResponse{Items: items}), nil
}

func (s *FlowServiceV2RPC) FlowSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[flowv1.FlowSyncResponse],
) error {
	if stream == nil {
		return connect.NewError(connect.CodeInternal, errors.New("stream is required"))
	}
	return s.streamFlowSync(ctx, func(resp *flowv1.FlowSyncResponse) error {
		return stream.Send(resp)
	})
}

func (s *FlowServiceV2RPC) streamFlowSync(
	ctx context.Context,
	send func(*flowv1.FlowSyncResponse) error,
) error {
	if s.flowStream == nil {
		return connect.NewError(connect.CodeUnavailable, errors.New("flow stream not configured"))
	}

	var workspaceSet sync.Map

	filter := func(topic FlowTopic) bool {
		if _, ok := workspaceSet.Load(topic.WorkspaceID.String()); ok {
			return true
		}
		if err := s.ensureWorkspaceAccess(ctx, topic.WorkspaceID); err != nil {
			return false
		}
		workspaceSet.Store(topic.WorkspaceID.String(), struct{}{})
		return true
	}

	events, err := s.flowStream.Subscribe(ctx, filter)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	for {
		select {
		case evt, ok := <-events:
			if !ok {
				return nil
			}
			resp := flowEventToSyncResponse(evt.Payload)
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

func (s *FlowServiceV2RPC) publishFlowEvent(eventType string, flow mflow.Flow) {
	if s.flowStream == nil {
		return
	}
	s.flowStream.Publish(FlowTopic{WorkspaceID: flow.WorkspaceID}, FlowEvent{
		Type: eventType,
		Flow: serializeFlow(flow),
	})
}

func (s *FlowServiceV2RPC) publishFileEvent(file mfile.File) {
	if s.fileStream == nil {
		return
	}
	s.fileStream.Publish(rfile.FileTopic{WorkspaceID: file.WorkspaceID}, rfile.FileEvent{
		Type: "create",
		File: converter.ToAPIFile(file),
		Name: file.Name,
	})
}

func flowEventToSyncResponse(evt FlowEvent) *flowv1.FlowSyncResponse {
	if evt.Flow == nil {
		return nil
	}

	var syncEvent *flowv1.FlowSync
	switch evt.Type {
	case flowEventInsert:
		insert := &flowv1.FlowSyncInsert{
			FlowId:      evt.Flow.FlowId,
			WorkspaceId: evt.Flow.WorkspaceId,
			Name:        evt.Flow.Name,
			Running:     evt.Flow.Running,
		}
		if evt.Flow.Duration != nil {
			insert.Duration = evt.Flow.Duration
		}
		syncEvent = &flowv1.FlowSync{
			Value: &flowv1.FlowSync_ValueUnion{
				Kind:   flowv1.FlowSync_ValueUnion_KIND_INSERT,
				Insert: insert,
			},
		}
	case flowEventUpdate:
		update := &flowv1.FlowSyncUpdate{
			FlowId:  evt.Flow.FlowId,
			Running: &evt.Flow.Running,
		}
		if evt.Flow.Name != "" {
			update.Name = &evt.Flow.Name
		}
		if evt.Flow.Duration != nil {
			update.Duration = &flowv1.FlowSyncUpdate_DurationUnion{
				Kind:  flowv1.FlowSyncUpdate_DurationUnion_KIND_VALUE,
				Value: evt.Flow.Duration,
			}
		}
		syncEvent = &flowv1.FlowSync{
			Value: &flowv1.FlowSync_ValueUnion{
				Kind:   flowv1.FlowSync_ValueUnion_KIND_UPDATE,
				Update: update,
			},
		}
	case flowEventDelete:
		syncEvent = &flowv1.FlowSync{
			Value: &flowv1.FlowSync_ValueUnion{
				Kind: flowv1.FlowSync_ValueUnion_KIND_DELETE,
				Delete: &flowv1.FlowSyncDelete{
					FlowId: evt.Flow.FlowId,
				},
			},
		}
	default:
		return nil
	}

	return &flowv1.FlowSyncResponse{
		Items: []*flowv1.FlowSync{syncEvent},
	}
}

func (s *FlowServiceV2RPC) FlowInsert(ctx context.Context, req *connect.Request[flowv1.FlowInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one flow is required"))
	}

	_, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// 1. Move validation OUTSIDE transaction (before BeginTx)
	type insertData struct {
		flow        mflow.Flow
		startNode   mflow.Node
		workspaceID idwrap.IDWrap
	}
	var validatedItems []insertData

	workspaceUpdates := make(map[idwrap.IDWrap]*mworkspace.Workspace)
	for _, item := range req.Msg.GetItems() {
		workspaceID, err := idwrap.NewFromBytes(item.GetWorkspaceId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid workspace id: %w", err))
		}

		if err := s.ensureWorkspaceAccess(ctx, workspaceID); err != nil {
			return nil, err
		}

		// Fetch workspace if not already loaded
		if _, exists := workspaceUpdates[workspaceID]; !exists {
			workspace, err := s.wsReader.Get(ctx, workspaceID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			workspaceUpdates[workspaceID] = workspace
		}

		name := strings.TrimSpace(item.GetName())
		flowID := idwrap.NewNow()
		if len(item.GetFlowId()) != 0 {
			flowID, _ = idwrap.NewFromBytes(item.GetFlowId())
		}

		flow := mflow.Flow{
			ID:          flowID,
			WorkspaceID: workspaceID,
			Name:        name,
		}

		startNode := mflow.Node{
			ID:        idwrap.NewNow(),
			FlowID:    flowID,
			Name:      "Start",
			NodeKind:  mflow.NODE_KIND_MANUAL_START,
			PositionX: 0,
			PositionY: 0,
		}

		validatedItems = append(validatedItems, insertData{
			flow:        flow,
			startNode:   startNode,
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

	wsWriter := sworkspace.NewWorkspaceWriter(mut.TX())
	fsWriter := sflow.NewFlowWriter(mut.TX())
	nsWriter := sflow.NewNodeWriter(mut.TX())

	// 3. Execute all inserts in transaction
	for _, data := range validatedItems {
		if err := fsWriter.CreateFlow(ctx, data.flow); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := nsWriter.CreateNode(ctx, data.startNode); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Track flow insert
		mut.Track(mutation.Event{
			Entity:      mutation.EntityFlow,
			Op:          mutation.OpInsert,
			ID:          data.flow.ID,
			WorkspaceID: data.workspaceID,
			Payload:     flowNodePair(data),
		})

		// Increment workspace flow count
		workspaceUpdates[data.workspaceID].FlowCount++
	}

	// Update all workspaces flow counts
	for _, workspace := range workspaceUpdates {
		workspace.Updated = dbtime.DBNow()
		if err := wsWriter.Update(ctx, workspace); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	// 4. Commit transaction (auto-publishes events)
	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) FlowUpdate(ctx context.Context, req *connect.Request[flowv1.FlowUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one flow is required"))
	}

	// 1. Move validation OUTSIDE transaction (before BeginTx)
	type updateData struct {
		flow        mflow.Flow
		workspaceID idwrap.IDWrap
	}
	var validatedUpdates []updateData

	for _, item := range req.Msg.GetItems() {
		if len(item.GetFlowId()) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow id is required"))
		}

		flowID, err := idwrap.NewFromBytes(item.GetFlowId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid flow id: %w", err))
		}

		if err := s.ensureFlowAccess(ctx, flowID); err != nil {
			return nil, err
		}

		flow, err := s.fsReader.GetFlow(ctx, flowID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("flow %s not found", flowID.String()))
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if item.Name != nil {
			name := strings.TrimSpace(item.GetName())
			if name == "" {
				return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow name cannot be empty"))
			}
			flow.Name = name
		}

		if du := item.GetDuration(); du != nil {
			switch du.GetKind() {
			case flowv1.FlowUpdate_DurationUnion_KIND_UNSET:
				flow.Duration = 0
			case flowv1.FlowUpdate_DurationUnion_KIND_VALUE:
				flow.Duration = du.GetValue()
			}
		}

		validatedUpdates = append(validatedUpdates, updateData{
			flow:        flow,
			workspaceID: flow.WorkspaceID,
		})
	}

	if len(validatedUpdates) == 0 {
		return connect.NewResponse(&emptypb.Empty{}), nil
	}

	// 2. Begin transaction with mutation context
	mut := mutation.New(s.DB, mutation.WithPublisher(s.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	fsWriter := sflow.NewFlowWriter(mut.TX())

	// 3. Execute all updates in transaction
	for _, data := range validatedUpdates {
		if err := fsWriter.UpdateFlow(ctx, data.flow); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		mut.Track(mutation.Event{
			Entity:      mutation.EntityFlow,
			Op:          mutation.OpUpdate,
			ID:          data.flow.ID,
			WorkspaceID: data.workspaceID,
			Payload:     data.flow,
		})
	}

	// 4. Commit transaction (auto-publishes events)
	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) FlowDelete(ctx context.Context, req *connect.Request[flowv1.FlowDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one flow is required"))
	}

	// FETCH: Get flow data and build delete items (outside transaction)
	deleteItems := make([]mutation.FlowDeleteItem, 0, len(req.Msg.GetItems()))
	workspaceUpdates := make(map[idwrap.IDWrap]*mworkspace.Workspace)

	for _, item := range req.Msg.GetItems() {
		flowID, err := idwrap.NewFromBytes(item.GetFlowId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid flow id: %w", err))
		}

		flow, err := s.fsReader.GetFlow(ctx, flowID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// CHECK: Validate permissions
		if err := s.ensureFlowAccess(ctx, flowID); err != nil {
			return nil, err
		}

		// Fetch workspace OUTSIDE transaction (SQLite best practice)
		if _, exists := workspaceUpdates[flow.WorkspaceID]; !exists {
			workspace, err := s.wsReader.Get(ctx, flow.WorkspaceID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			workspaceUpdates[flow.WorkspaceID] = workspace
		}

		deleteItems = append(deleteItems, mutation.FlowDeleteItem{
			ID:          flow.ID,
			WorkspaceID: flow.WorkspaceID,
		})
	}

	if len(deleteItems) == 0 {
		return connect.NewResponse(&emptypb.Empty{}), nil
	}

	// ACT: Delete flows using mutation context with auto-publish
	mut := mutation.New(s.DB, mutation.WithPublisher(s.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	if err := mut.DeleteFlowBatch(ctx, deleteItems); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Update workspace flow counts inside the transaction
	wsWriter := sworkspace.NewWorkspaceWriter(mut.TX())
	for _, item := range deleteItems {
		workspace := workspaceUpdates[item.WorkspaceID]
		if workspace.FlowCount > 0 {
			workspace.FlowCount--
		}
	}

	for _, workspace := range workspaceUpdates {
		workspace.Updated = dbtime.DBNow()
		if err := wsWriter.Update(ctx, workspace); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := mut.Commit(ctx); err != nil { // Auto-publishes events!
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) FlowDuplicate(ctx context.Context, req *connect.Request[flowv1.FlowDuplicateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetFlowId()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow id is required"))
	}

	sourceFlowID, err := idwrap.NewFromBytes(req.Msg.GetFlowId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid flow id: %w", err))
	}

	// Step 1: FETCH/CHECK (Outside transaction)
	if err := s.ensureFlowAccess(ctx, sourceFlowID); err != nil {
		return nil, err
	}

	sourceFlow, err := s.fsReader.GetFlow(ctx, sourceFlowID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("flow %s not found", sourceFlowID.String()))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if err := s.ensureWorkspaceAccess(ctx, sourceFlow.WorkspaceID); err != nil {
		return nil, err
	}

	workspace, err := s.wsReader.Get(ctx, sourceFlow.WorkspaceID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	sourceNodes, err := s.nsReader.GetNodesByFlowID(ctx, sourceFlowID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Collect node details outside TX
	type nodeDetail struct {
		node    mflow.Node
		request *mflow.NodeRequest
		http    *mhttp.HTTP
		forNode *mflow.NodeFor
		forEach *mflow.NodeForEach
		ifNode  *mflow.NodeIf
		jsNode  *mflow.NodeJS
	}
	details := make([]nodeDetail, 0, len(sourceNodes))
	for _, n := range sourceNodes {
		detail := nodeDetail{node: n}
		switch n.NodeKind {
		case mflow.NODE_KIND_REQUEST:
			if d, err := s.nrs.GetNodeRequest(ctx, n.ID); err == nil && d != nil {
				detail.request = d
				if d.HttpID != nil {
					if h, err := s.hsReader.Get(ctx, *d.HttpID); err == nil {
						detail.http = h
					}
				}
			}
		case mflow.NODE_KIND_FOR:
			if d, err := s.nfs.GetNodeFor(ctx, n.ID); err == nil {
				detail.forNode = d
			}
		case mflow.NODE_KIND_FOR_EACH:
			if d, err := s.nfes.GetNodeForEach(ctx, n.ID); err == nil {
				detail.forEach = d
			}
		case mflow.NODE_KIND_CONDITION:
			if d, err := s.nifs.GetNodeIf(ctx, n.ID); err == nil {
				detail.ifNode = d
			}
		case mflow.NODE_KIND_JS:
			if d, err := s.njss.GetNodeJS(ctx, n.ID); err == nil {
				detail.jsNode = d
			}
		}
		details = append(details, detail)
	}

	sourceEdges, err := s.es.GetEdgesByFlowID(ctx, sourceFlowID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	sourceVariables, err := s.fvs.GetFlowVariablesByFlowID(ctx, sourceFlowID)
	if err != nil && !errors.Is(err, sflow.ErrNoFlowVariableFound) {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Step 2: ACT (Inside transaction)
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	fsWriter := sflow.NewFlowWriter(tx)
	wsWriter := sworkspace.NewWorkspaceWriter(tx)
	nsWriter := sflow.NewNodeWriter(tx)
	nrsWriter := sflow.NewNodeRequestWriter(tx)
	nfsWriter := sflow.NewNodeForWriter(tx)
	nfesWriter := sflow.NewNodeForEachWriter(tx)
	nifsWriter := sflow.NewNodeIfWriter(tx)
	njssWriter := sflow.NewNodeJsWriter(tx)
	esWriter := sflow.NewEdgeWriter(tx)
	fvsWriter := sflow.NewFlowVariableWriter(tx)

	newFlowID := idwrap.NewNow()
	newFlow := mflow.Flow{
		ID:          newFlowID,
		WorkspaceID: sourceFlow.WorkspaceID,
		Name:        fmt.Sprintf("Copy of %s", sourceFlow.Name),
	}
	if err := fsWriter.CreateFlow(ctx, newFlow); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Create file entry for the duplicated flow (for sidebar integration)
	fileWriter := sfile.NewWriter(tx, nil)
	newFlowFile := mfile.File{
		ID:          newFlowID,
		WorkspaceID: sourceFlow.WorkspaceID,
		ContentID:   &newFlowID,
		ContentType: mfile.ContentTypeFlow,
		Name:        newFlow.Name,
		Order:       float64(time.Now().UnixMilli()),
		UpdatedAt:   time.Now(),
	}
	if err := fileWriter.CreateFile(ctx, &newFlowFile); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	nodeIDMapping := make(map[string]idwrap.IDWrap)
	for _, d := range details {
		newNodeID := idwrap.NewNow()
		nodeIDMapping[d.node.ID.String()] = newNodeID

		newNode := d.node
		newNode.ID = newNodeID
		newNode.FlowID = newFlowID
		if err := nsWriter.CreateNode(ctx, newNode); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if d.request != nil {
			// Reuse the same HTTP reference - don't duplicate HTTP requests
			node := mflow.NodeRequest{
				FlowNodeID:       newNodeID,
				HttpID:           d.request.HttpID,
				HasRequestConfig: d.request.HasRequestConfig,
			}
			if err := nrsWriter.CreateNodeRequest(ctx, node); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		}
		if d.forNode != nil {
			node := *d.forNode
			node.FlowNodeID = newNodeID
			if err := nfsWriter.CreateNodeFor(ctx, node); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		}
		if d.forEach != nil {
			node := *d.forEach
			node.FlowNodeID = newNodeID
			if err := nfesWriter.CreateNodeForEach(ctx, node); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		}
		if d.ifNode != nil {
			node := *d.ifNode
			node.FlowNodeID = newNodeID
			if err := nifsWriter.CreateNodeIf(ctx, node); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		}
		if d.jsNode != nil {
			node := *d.jsNode
			node.FlowNodeID = newNodeID
			if err := njssWriter.CreateNodeJS(ctx, node); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		}
	}

	// Track created edges for event publishing
	createdEdges := make([]mflow.Edge, 0, len(sourceEdges))
	for _, e := range sourceEdges {
		newSourceID, sourceOK := nodeIDMapping[e.SourceID.String()]
		newTargetID, targetOK := nodeIDMapping[e.TargetID.String()]
		if !sourceOK || !targetOK {
			continue
		}
		newEdge := mflow.Edge{
			ID:            idwrap.NewNow(),
			FlowID:        newFlowID,
			SourceID:      newSourceID,
			TargetID:      newTargetID,
			SourceHandler: e.SourceHandler,
		}
		if err := esWriter.CreateEdge(ctx, newEdge); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		createdEdges = append(createdEdges, newEdge)
	}

	// Track created variables for event publishing
	createdVariables := make([]mflow.FlowVariable, 0, len(sourceVariables))
	for _, v := range sourceVariables {
		newVar := v
		newVar.ID = idwrap.NewNow()
		newVar.FlowID = newFlowID
		if err := fvsWriter.CreateFlowVariable(ctx, newVar); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		createdVariables = append(createdVariables, newVar)
	}

	workspace.FlowCount++
	workspace.Updated = dbtime.DBNow()
	if err := wsWriter.Update(ctx, workspace); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Step 3: NOTIFY (Outside transaction) - Publish events for all created entities
	s.publishFlowEvent(flowEventInsert, newFlow)

	// Publish file event for sidebar integration
	s.publishFileEvent(newFlowFile)

	// Publish node events for all duplicated nodes
	for _, d := range details {
		newNodeID, ok := nodeIDMapping[d.node.ID.String()]
		if !ok {
			continue
		}
		duplicatedNode := d.node
		duplicatedNode.ID = newNodeID
		duplicatedNode.FlowID = newFlowID
		s.publishNodeEvent(nodeEventInsert, duplicatedNode)
	}

	// Publish edge events for all duplicated edges (use tracked IDs)
	for _, edge := range createdEdges {
		s.publishEdgeEvent(edgeEventInsert, edge)
	}

	// Publish variable events for all duplicated variables (use tracked IDs)
	for _, variable := range createdVariables {
		s.publishFlowVariableEvent(flowVarEventInsert, variable)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}
