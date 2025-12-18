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
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/service/flow/sedge"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sflowvariable"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/snode"
	"the-dev-tools/server/pkg/service/snodefor"
	"the-dev-tools/server/pkg/service/snodeforeach"
	"the-dev-tools/server/pkg/service/snodeif"
	"the-dev-tools/server/pkg/service/snodejs"
	"the-dev-tools/server/pkg/service/snodenoop"
	"the-dev-tools/server/pkg/service/snoderequest"
	"the-dev-tools/server/pkg/service/sworkspace"
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

	// Step 1: FETCH (outside transaction)
	workspaceUpdates := make(map[idwrap.IDWrap]*mworkspace.Workspace)
	for _, item := range req.Msg.GetItems() {
		workspaceID, err := idwrap.NewFromBytes(item.GetWorkspaceId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid workspace id: %w", err))
		}

		if err := s.ensureWorkspaceAccess(ctx, workspaceID); err != nil {
			return nil, err
		}

		if _, exists := workspaceUpdates[workspaceID]; !exists {
			workspace, err := s.wsReader.Get(ctx, workspaceID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			workspaceUpdates[workspaceID] = workspace
		}
	}

	// Step 2: ACT (Inside transaction)
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	wsWriter := sworkspace.NewWriter(tx)
	fsWriter := sflow.NewWriter(tx)
	nsWriter := snode.NewWriter(tx)
	nnosWriter := snodenoop.NewWriter(tx)

	var createdFlows []mflow.Flow

	for _, item := range req.Msg.GetItems() {
		workspaceID, _ := idwrap.NewFromBytes(item.GetWorkspaceId())
		workspace := workspaceUpdates[workspaceID]

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

		if err := fsWriter.CreateFlow(ctx, flow); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Seed start node
		startNodeID := idwrap.NewNow()
		startNode := mflow.Node{
			ID:        startNodeID,
			FlowID:    flowID,
			Name:      "Start",
			NodeKind:  mflow.NODE_KIND_NO_OP,
			PositionX: 0,
			PositionY: 0,
		}
		if err := nsWriter.CreateNode(ctx, startNode); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		startNoop := mflow.NodeNoop{
			FlowNodeID: startNodeID,
			Type:       mflow.NODE_NO_OP_KIND_START,
		}
		if err := nnosWriter.CreateNodeNoop(ctx, startNoop); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		createdFlows = append(createdFlows, flow)
		workspace.FlowCount++
	}

	// Update all workspaces flow counts
	for _, workspace := range workspaceUpdates {
		workspace.Updated = dbtime.DBNow()
		if err := wsWriter.Update(ctx, workspace); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Step 3: NOTIFY (Outside transaction)
	for _, flow := range createdFlows {
		// Re-fetch to get any auto-populated fields if needed, or just use what we have
		s.publishFlowEvent(flowEventInsert, flow)
		// We skipped seeding sync events for start node in TX for brevity,
		// but ideally we should notify about it too.
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) FlowUpdate(ctx context.Context, req *connect.Request[flowv1.FlowUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one flow is required"))
	}

	// Step 1: FETCH/CHECK (Outside transaction)
	var updateData []struct {
		flow mflow.Flow
		item *flowv1.FlowUpdate
	}

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
			flow.Name = strings.TrimSpace(item.GetName())
			if flow.Name == "" {
				return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow name cannot be empty"))
			}
		}

		if du := item.GetDuration(); du != nil {
			switch du.GetKind() {
			case flowv1.FlowUpdate_DurationUnion_KIND_UNSET:
				flow.Duration = 0
			case flowv1.FlowUpdate_DurationUnion_KIND_VALUE:
				flow.Duration = du.GetValue()
			}
		}

		updateData = append(updateData, struct {
			flow mflow.Flow
			item *flowv1.FlowUpdate
		}{flow: flow, item: item})
	}

	// Step 2: ACT (Inside transaction)
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	fsWriter := sflow.NewWriter(tx)

	for _, data := range updateData {
		if err := fsWriter.UpdateFlow(ctx, data.flow); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Step 3: NOTIFY (Outside transaction)
	for _, data := range updateData {
		s.publishFlowEvent(flowEventUpdate, data.flow)
		if data.flow.VersionParentID != nil {
			s.publishFlowVersionEvent(flowVersionEventUpdate, data.flow)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) FlowDelete(ctx context.Context, req *connect.Request[flowv1.FlowDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one flow is required"))
	}

	// Step 1: FETCH/CHECK (Outside transaction)
	var deleteData []struct {
		flow mflow.Flow
	}

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

		deleteData = append(deleteData, struct{ flow mflow.Flow }{flow: flow})
	}

	// Step 2: ACT (Inside transaction)
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	fsWriter := sflow.NewWriter(tx)
	wsWriter := sworkspace.NewWriter(tx)

	for _, data := range deleteData {
		flow := data.flow
		if err := fsWriter.DeleteFlow(ctx, flow.ID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		workspace, err := s.wsReader.Get(ctx, flow.WorkspaceID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if workspace.FlowCount > 0 {
			workspace.FlowCount--
		}
		workspace.Updated = dbtime.DBNow()
		if err := wsWriter.Update(ctx, workspace); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Step 3: NOTIFY (Outside transaction)
	for _, data := range deleteData {
		s.publishFlowEvent(flowEventDelete, data.flow)
		if data.flow.VersionParentID != nil {
			s.publishFlowVersionEvent(flowVersionEventDelete, data.flow)
		}
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
		noop    *mflow.NodeNoop
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
		case mflow.NODE_KIND_NO_OP:
			if d, err := s.nnos.GetNodeNoop(ctx, n.ID); err == nil {
				detail.noop = d
			}
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
				detail.jsNode = &d
			}
		}
		details = append(details, detail)
	}

	sourceEdges, err := s.es.GetEdgesByFlowID(ctx, sourceFlowID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	sourceVariables, err := s.fvs.GetFlowVariablesByFlowID(ctx, sourceFlowID)
	if err != nil && !errors.Is(err, sflowvariable.ErrNoFlowVariableFound) {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Step 2: ACT (Inside transaction)
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	fsWriter := sflow.NewWriter(tx)
	wsWriter := sworkspace.NewWriter(tx)
	nsWriter := snode.NewWriter(tx)
	nnosWriter := snodenoop.NewWriter(tx)
	nrsWriter := snoderequest.NewWriter(tx)
	hsWriter := shttp.NewWriter(tx)
	nfsWriter := snodefor.NewWriter(tx)
	nfesWriter := snodeforeach.NewWriter(tx)
	nifsWriter := snodeif.NewWriter(tx)
	njssWriter := snodejs.NewWriter(tx)
	esWriter := sedge.NewWriter(tx)
	fvsWriter := sflowvariable.NewWriter(tx)

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

		if d.noop != nil {
			node := *d.noop
			node.FlowNodeID = newNodeID
			if err := nnosWriter.CreateNodeNoop(ctx, node); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
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
		newEdge := edge.Edge{
			ID:            idwrap.NewNow(),
			FlowID:        newFlowID,
			SourceID:      newSourceID,
			TargetID:      newTargetID,
			SourceHandler: e.SourceHandler,
			Kind:          e.Kind,
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

	return connect.NewResponse(&emptypb.Empty{}), nil
}
