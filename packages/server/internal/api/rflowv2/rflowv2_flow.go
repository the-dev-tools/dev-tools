//nolint:revive // exported
package rflowv2

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"

	"connectrpc.com/connect"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

	devtoolsdb "the-dev-tools/db"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/eventstream"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/patch"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/txutil"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
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

	snapshot := func(ctx context.Context) ([]eventstream.Event[FlowTopic, FlowEvent], error) {
		flows, err := s.listAccessibleFlows(ctx)
		if err != nil {
			return nil, err
		}

		events := make([]eventstream.Event[FlowTopic, FlowEvent], 0)

		for _, flow := range flows {
			workspaceSet.Store(flow.WorkspaceID.String(), struct{}{})

			events = append(events, eventstream.Event[FlowTopic, FlowEvent]{
				Topic: FlowTopic{WorkspaceID: flow.WorkspaceID},
				Payload: FlowEvent{
					Type: flowEventInsert,
					Flow: serializeFlow(flow),
				},
			})
		}

		return events, nil
	}

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

	events, err := s.flowStream.Subscribe(ctx, filter, eventstream.WithSnapshot(snapshot))
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

	// 2. Begin transaction with bulk sync wrapper
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	syncTx := txutil.NewBulkInsertTx[flowNodePair, FlowTopic](
		tx,
		func(fnp flowNodePair) FlowTopic {
			return FlowTopic{WorkspaceID: fnp.workspaceID}
		},
	)

	wsWriter := sworkspace.NewWorkspaceWriter(tx)
	fsWriter := sflow.NewFlowWriter(tx)
	nsWriter := sflow.NewNodeWriter(tx)

	// 3. Execute all inserts in transaction
	for _, data := range validatedItems {
		if err := fsWriter.CreateFlow(ctx, data.flow); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := nsWriter.CreateNode(ctx, data.startNode); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Track for bulk event publishing
		syncTx.Track(flowNodePair{
			flow:        data.flow,
			startNode:   data.startNode,
			workspaceID: data.workspaceID,
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

	// 4. Commit transaction and publish events in bulk
	if err := syncTx.CommitAndPublish(ctx, s.publishBulkFlowInsert); err != nil {
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
		flowPatch   patch.FlowPatch
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

		flowPatch := patch.FlowPatch{}

		if item.Name != nil {
			name := strings.TrimSpace(item.GetName())
			if name == "" {
				return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow name cannot be empty"))
			}
			flow.Name = name
			flowPatch.Name = patch.NewOptional(name)
		}

		if du := item.GetDuration(); du != nil {
			switch du.GetKind() {
			case flowv1.FlowUpdate_DurationUnion_KIND_UNSET:
				flow.Duration = 0
				flowPatch.Duration = patch.NewOptional(uint64(0))
			case flowv1.FlowUpdate_DurationUnion_KIND_VALUE:
				flow.Duration = du.GetValue()
				flowPatch.Duration = patch.NewOptional(uint64(du.GetValue()))
			}
		}

		validatedUpdates = append(validatedUpdates, updateData{
			flow:        flow,
			flowPatch:   flowPatch,
			workspaceID: flow.WorkspaceID,
		})
	}

	// 2. Begin transaction with bulk sync wrapper
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	syncTx := txutil.NewBulkUpdateTx[flowWithWorkspace, patch.FlowPatch, FlowTopic](
		tx,
		func(fww flowWithWorkspace) FlowTopic {
			return FlowTopic{WorkspaceID: fww.workspaceID}
		},
	)

	fsWriter := sflow.NewFlowWriter(tx)

	// 3. Execute all updates in transaction
	for _, data := range validatedUpdates {
		if err := fsWriter.UpdateFlow(ctx, data.flow); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		syncTx.Track(
			flowWithWorkspace{
				flow:        data.flow,
				workspaceID: data.workspaceID,
			},
			data.flowPatch,
		)
	}

	// 4. Commit transaction and publish events in bulk
	if err := syncTx.CommitAndPublish(ctx, s.publishBulkFlowUpdate); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) FlowDelete(ctx context.Context, req *connect.Request[flowv1.FlowDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one flow is required"))
	}

	// 1. Move validation OUTSIDE transaction (before BeginTx)
	type deleteData struct {
		flow        mflow.Flow
		workspaceID idwrap.IDWrap
	}
	var validatedDeletes []deleteData

	// Track workspace updates needed (aggregated by workspace ID)
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

		if err := s.ensureFlowAccess(ctx, flowID); err != nil {
			return nil, err
		}

		// Fetch workspace OUTSIDE transaction (fix for SQLite best practices)
		if _, exists := workspaceUpdates[flow.WorkspaceID]; !exists {
			workspace, err := s.wsReader.Get(ctx, flow.WorkspaceID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			workspaceUpdates[flow.WorkspaceID] = workspace
		}

		validatedDeletes = append(validatedDeletes, deleteData{
			flow:        flow,
			workspaceID: flow.WorkspaceID,
		})
	}

	// 2. Begin transaction
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	fsWriter := sflow.NewFlowWriter(tx)
	wsWriter := sworkspace.NewWorkspaceWriter(tx)

	var deletedFlows []flowWithWorkspace

	// 3. Execute all deletes in transaction
	for _, data := range validatedDeletes {
		if err := fsWriter.DeleteFlow(ctx, data.flow.ID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Decrement workspace flow count
		workspace := workspaceUpdates[data.workspaceID]
		if workspace.FlowCount > 0 {
			workspace.FlowCount--
		}

		deletedFlows = append(deletedFlows, flowWithWorkspace{
			flow:        data.flow,
			workspaceID: data.workspaceID,
		})
	}

	// Update all workspaces
	for _, workspace := range workspaceUpdates {
		workspace.Updated = dbtime.DBNow()
		if err := wsWriter.Update(ctx, workspace); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	// 4. Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// 5. Publish events AFTER successful commit (group by workspace)
	// Group by workspace ID for bulk publishing
	grouped := make(map[idwrap.IDWrap][]flowWithWorkspace)
	for _, fww := range deletedFlows {
		grouped[fww.workspaceID] = append(grouped[fww.workspaceID], fww)
	}

	// Publish bulk delete events per workspace
	for workspaceID, items := range grouped {
		s.publishBulkFlowDelete(FlowTopic{WorkspaceID: workspaceID}, items)
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
			if d, err := s.nrs.GetNodeRequest(ctx, n.ID); err == nil {
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
	hsWriter := shttp.NewWriter(tx)
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
			newHttpID := idwrap.IDWrap{}
			if d.http != nil {
				newHttpID = idwrap.NewNow()
				duplicatedHttp := *d.http
				duplicatedHttp.ID = newHttpID
				duplicatedHttp.Name = fmt.Sprintf("Copy of %s", d.http.Name)
				if err := hsWriter.Create(ctx, &duplicatedHttp); err != nil {
					return nil, connect.NewError(connect.CodeInternal, err)
				}
			}
			node := mflow.NodeRequest{
				FlowNodeID:       newNodeID,
				HttpID:           nil,
				HasRequestConfig: d.request.HasRequestConfig,
			}
			if !isZeroID(newHttpID) {
				node.HttpID = &newHttpID
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
	}

	for _, v := range sourceVariables {
		newVar := v
		newVar.ID = idwrap.NewNow()
		newVar.FlowID = newFlowID
		if err := fvsWriter.CreateFlowVariable(ctx, newVar); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
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

	// Publish edge events for all duplicated edges
	for _, e := range sourceEdges {
		newSourceID, sourceOK := nodeIDMapping[e.SourceID.String()]
		newTargetID, targetOK := nodeIDMapping[e.TargetID.String()]
		if !sourceOK || !targetOK {
			continue
		}
		duplicatedEdge := mflow.Edge{
			ID:            idwrap.NewNow(), // Event system will handle ID reconciliation
			FlowID:        newFlowID,
			SourceID:      newSourceID,
			TargetID:      newTargetID,
			SourceHandler: e.SourceHandler,
		}
		s.publishEdgeEvent(edgeEventInsert, duplicatedEdge)
	}

	// Publish variable events for all duplicated variables
	for _, v := range sourceVariables {
		duplicatedVar := v
		duplicatedVar.ID = idwrap.NewNow() // Event system will handle ID reconciliation
		duplicatedVar.FlowID = newFlowID
		s.publishFlowVariableEvent(flowVarEventInsert, duplicatedVar)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}
