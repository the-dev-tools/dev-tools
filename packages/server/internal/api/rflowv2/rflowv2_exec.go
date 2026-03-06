//nolint:revive // exported
package rflowv2

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

	devtoolsdb "github.com/the-dev-tools/dev-tools/packages/db"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/flowexec"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/flowresult"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	flowv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/flow/v1"
)

func (s *FlowServiceV2RPC) FlowRun(ctx context.Context, req *connect.Request[flowv1.FlowRunRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetFlowId()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow id is required"))
	}

	flowID, err := idwrap.NewFromBytes(req.Msg.GetFlowId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid flow id: %w", err))
	}

	if err := s.ensureFlowAccess(ctx, flowID); err != nil {
		return nil, err
	}

	flow, err := s.fs.GetFlow(ctx, flowID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("flow %s not found", flowID.String()))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	nodes, err := s.ns.GetNodesByFlowID(ctx, flowID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	edges, err := s.es.GetEdgesByFlowID(ctx, flowID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	flowVars, err := s.fvs.GetFlowVariablesByFlowID(ctx, flowID)
	if err != nil && !errors.Is(err, sflow.ErrNoFlowVariableFound) {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Move existing parent node executions to the previous version before creating a new one
	if err := s.moveParentExecutionsToPreviousVersion(ctx, flow); err != nil {
		s.logger.Error("failed to move parent executions to previous version", "error", err)
		// Continue anyway - not a critical failure
	}

	// Create a new flow version for this run (snapshot of the flow with all nodes, edges, etc.)
	version, nodeIDMapping, err := s.createFlowVersionSnapshot(ctx, flow, nodes, edges, flowVars)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create flow version: %w", err))
	}

	// Save the nodeIDMapping to the version flow for future execution moves
	// Convert to string map for JSON serialization
	if len(nodeIDMapping) > 0 {
		stringMapping := make(map[string]string, len(nodeIDMapping))
		for k, v := range nodeIDMapping {
			stringMapping[k] = v.String()
		}
		mappingJSON, err := json.Marshal(stringMapping)
		if err != nil {
			s.logger.Error("failed to marshal nodeIDMapping", "error", err)
		} else if err := s.fs.UpdateFlowNodeIDMapping(ctx, version.ID, mappingJSON); err != nil {
			s.logger.Error("failed to save nodeIDMapping", "error", err)
		}
	}

	// Publish flow insert event so clients receive the version in FlowSync
	// (the version is a flow record that clients need to query)
	s.publishFlowEvent(flowEventInsert, version)

	// Publish version insert event for real-time sync
	s.publishFlowVersionEvent(flowVersionEventInsert, version)

	// Run execution asynchronously
	go func() {
		// Create a background context for execution with cancellation support
		bgCtx, cancel := context.WithCancel(context.Background())

		// Store cancel function
		s.runningFlowsMu.Lock()
		s.runningFlows[flowID.String()] = cancel
		s.runningFlowsMu.Unlock()

		// Mark flow as running before execution starts
		flow.Running = true
		flow.Error = nil
		if err := s.fs.UpdateFlow(bgCtx, flow); err != nil {
			s.logger.Error("failed to mark flow as running", "flow_id", flowID.String(), "error", err)
		}
		s.publishFlowEvent(flowEventUpdate, flow)

		defer func() {
			// Always mark flow as not running, regardless of how executeFlow returned
			flow.Running = false
			if err := s.fs.UpdateFlow(context.Background(), flow); err != nil {
				s.logger.Error("failed to mark flow as not running", "flow_id", flowID.String(), "error", err)
			}
			s.publishFlowEvent(flowEventUpdate, flow)

			// Cleanup running flows map
			s.runningFlowsMu.Lock()
			delete(s.runningFlows, flowID.String())
			s.runningFlowsMu.Unlock()
			cancel()
		}()

		duration, execErr := s.executeFlow(bgCtx, flow, nodes, edges, flowVars, nodeIDMapping)

		// Copy final node/edge states from parent to version (best-effort).
		// Use Background() because bgCtx may be cancelled on FlowStop.
		s.copyStatesToVersion(context.Background(), flow.ID, version.ID, nodeIDMapping)

		if execErr != nil {
			errMsg := execErr.Error()
			flow.Error = &errMsg
			if errors.Is(execErr, context.Canceled) {
				s.logger.Info("flow execution canceled", "flow_id", flowID.String())
			} else {
				s.logger.Error("async flow execution failed", "flow_id", flowID.String(), "error", execErr)
			}
		}

		// Update duration for the cleanup defer's UpdateFlow call
		flow.Duration = duration

		// Also update the version with duration and error
		version.Duration = duration
		version.Error = flow.Error
		if err := s.fs.UpdateFlow(context.Background(), version); err != nil {
			s.logger.Error("failed to update version with results", "version_id", version.ID.String(), "error", err)
		}
		s.publishFlowEvent(flowEventUpdate, version)
	}()

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) executeFlow(
	ctx context.Context,
	flow mflow.Flow,
	nodes []mflow.Node,
	edges []mflow.Edge,
	flowVars []mflow.FlowVariable,
	nodeIDMapping map[string]idwrap.IDWrap,
) (int32, error) {
	// Filter orphaned edges (source or target node missing)
	validEdges := filterValidEdges(nodes, edges)

	// Create result processor
	proc := flowresult.NewServerResultProcessor(flowresult.ServerResultProcessorOpts{
		FlowID:                 flow.ID,
		WorkspaceID:            flow.WorkspaceID,
		Nodes:                  nodes,
		Edges:                  validEdges,
		NodeIDMapping:          nodeIDMapping,
		HTTPResponseService:    s.httpResponseService,
		GraphQLResponseService: s.graphqlResponseService,
		NodeExecutionService:   s.nes,
		NodeService:            s.ns,
		EdgeService:            s.es,
		Publisher:              s.newExecEventPublisher(),
		Logger:                 s.logger,
	})

	// Create and prepare execution session
	session := s.sessionFactory.Create(proc)

	if err := session.Prepare(ctx, flowexec.ExecutionParams{
		Flow:     flow,
		Nodes:    nodes,
		Edges:    validEdges,
		FlowVars: flowVars,
	}); err != nil {
		return 0, err
	}

	// Reset node/edge states before execution (batch-publishes UI events)
	s.resetNodeStates(ctx, flow.ID, nodes)
	s.resetEdgeStates(ctx, flow.ID, edges)

	// Execute flow and wait for result processing
	result, err := session.Run(ctx)
	return result.Duration, err
}

// filterValidEdges removes edges whose source or target node is missing.
func filterValidEdges(nodes []mflow.Node, edges []mflow.Edge) []mflow.Edge {
	nodeIDSet := make(map[idwrap.IDWrap]struct{}, len(nodes))
	for _, n := range nodes {
		nodeIDSet[n.ID] = struct{}{}
	}
	validEdges := make([]mflow.Edge, 0, len(edges))
	for _, e := range edges {
		if _, srcOK := nodeIDSet[e.SourceID]; !srcOK {
			continue
		}
		if _, tgtOK := nodeIDSet[e.TargetID]; !tgtOK {
			continue
		}
		validEdges = append(validEdges, e)
	}
	return validEdges
}

// resetNodeStates sets all node states to UNSPECIFIED and batch-publishes events.
func (s *FlowServiceV2RPC) resetNodeStates(ctx context.Context, flowID idwrap.IDWrap, nodes []mflow.Node) {
	nodeResetEvents := make([]NodeEvent, 0, len(nodes))
	for _, node := range nodes {
		if err := s.ns.UpdateNodeState(ctx, node.ID, mflow.NODE_STATE_UNSPECIFIED); err != nil {
			s.logger.Error("failed to reset node state", "node_id", node.ID.String(), "error", err)
		} else {
			resetNode := node
			resetNode.State = mflow.NODE_STATE_UNSPECIFIED
			nodeResetEvents = append(nodeResetEvents, NodeEvent{
				Type:   nodeEventUpdate,
				FlowID: flowID,
				Node:   serializeNode(resetNode),
			})
		}
	}
	if len(nodeResetEvents) > 0 && s.nodeStream != nil {
		s.nodeStream.Publish(NodeTopic{FlowID: flowID}, nodeResetEvents...)
	}
}

// resetEdgeStates sets all edge states to UNSPECIFIED and batch-publishes events.
func (s *FlowServiceV2RPC) resetEdgeStates(ctx context.Context, flowID idwrap.IDWrap, edges []mflow.Edge) {
	edgeResetEvents := make([]EdgeEvent, 0, len(edges))
	for _, edge := range edges {
		if err := s.es.UpdateEdgeState(ctx, edge.ID, mflow.NODE_STATE_UNSPECIFIED); err != nil {
			s.logger.Error("failed to reset edge state", "edge_id", edge.ID.String(), "error", err)
		} else {
			resetEdge := edge
			resetEdge.State = mflow.NODE_STATE_UNSPECIFIED
			edgeResetEvents = append(edgeResetEvents, EdgeEvent{
				Type:   edgeEventUpdate,
				FlowID: flowID,
				Edge:   serializeEdge(resetEdge),
			})
		}
	}
	if len(edgeResetEvents) > 0 && s.edgeStream != nil {
		s.edgeStream.Publish(EdgeTopic{FlowID: flowID}, edgeResetEvents...)
	}
}

func (s *FlowServiceV2RPC) FlowStop(ctx context.Context, req *connect.Request[flowv1.FlowStopRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetFlowId()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow id is required"))
	}

	flowID, err := idwrap.NewFromBytes(req.Msg.GetFlowId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid flow id: %w", err))
	}

	if err := s.ensureFlowAccess(ctx, flowID); err != nil {
		return nil, err
	}

	s.runningFlowsMu.Lock()
	cancel, ok := s.runningFlows[flowID.String()]
	s.runningFlowsMu.Unlock()

	if ok {
		// Cancel the actively running flow
		cancel()
		return connect.NewResponse(&emptypb.Empty{}), nil
	}

	// No active goroutine — check if the DB has stale running state
	// (e.g., from a previous server crash or a goroutine that already exited).
	flow, err := s.fs.GetFlow(ctx, flowID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if flow.Running {
		flow.Running = false
		if err := s.fs.UpdateFlow(ctx, flow); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to reset stale running state: %w", err))
		}
		s.publishFlowEvent(flowEventUpdate, flow)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// createFlowVersionSnapshot creates a complete snapshot of the flow including all nodes, edges, sub-nodes, and variables.
// It also publishes sync events for all created entities so clients receive the full flow data.
// Returns the created version flow and a mapping from original node IDs to version node IDs.
//
// CRITICAL: This function uses a single transaction to ensure atomicity. If any creation fails,
// all changes are rolled back and no sync events are published. This prevents partial/corrupted
// flow version snapshots from being created.
func (s *FlowServiceV2RPC) createFlowVersionSnapshot(
	ctx context.Context,
	sourceFlow mflow.Flow,
	sourceNodes []mflow.Node,
	sourceEdges []mflow.Edge,
	sourceVars []mflow.FlowVariable,
) (mflow.Flow, map[string]idwrap.IDWrap, error) {
	// === PREPARATION PHASE (BEFORE TRANSACTION) ===
	// Read all sub-node configurations before starting the transaction to minimize transaction duration.
	nodeConfigs := s.snapshotRegistry.ReadAll(ctx, sourceNodes, s.logger)

	// === BEGIN TRANSACTION ===
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return mflow.Flow{}, nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer devtoolsdb.TxnRollback(tx)

	flowWriter := s.fs.TX(tx)
	nodeWriter := s.ns.TX(tx)
	edgeWriter := s.es.TX(tx)
	varWriter := s.fvs.TX(tx)

	// Create the version flow record
	version, err := flowWriter.CreateFlowVersion(ctx, sourceFlow)
	if err != nil {
		return mflow.Flow{}, nil, fmt.Errorf("create flow version: %w", err)
	}

	versionFlowID := version.ID

	// Create a mapping from old node IDs to new node IDs for edge remapping
	nodeIDMapping := make(map[string]idwrap.IDWrap, len(sourceNodes))

	// Events collections for bulk publishing (after commit)
	nodeEvents := make([]NodeEvent, 0, len(sourceNodes))

	// Duplicate all nodes
	for _, sourceNode := range sourceNodes {
		newNodeID := idwrap.NewMonotonic()
		nodeIDMapping[sourceNode.ID.String()] = newNodeID

		newNode := mflow.Node{
			ID:        newNodeID,
			FlowID:    versionFlowID,
			Name:      sourceNode.Name,
			NodeKind:  sourceNode.NodeKind,
			PositionX: sourceNode.PositionX,
			PositionY: sourceNode.PositionY,
			State:     mflow.NODE_STATE_UNSPECIFIED,
		}

		if err := nodeWriter.CreateNodeWithState(ctx, newNode); err != nil {
			return mflow.Flow{}, nil, fmt.Errorf("create node %s: %w", sourceNode.Name, err)
		}

		nodeEvents = append(nodeEvents, NodeEvent{
			Type:   nodeEventInsert,
			FlowID: versionFlowID,
			Node:   serializeNode(newNode),
		})
	}

	// Write type-specific node configs via snapshot registry
	configResults, err := s.snapshotRegistry.WriteAllTx(ctx, tx, sourceNodes, nodeIDMapping, nodeConfigs)
	if err != nil {
		return mflow.Flow{}, nil, err
	}

	// Collect type-specific events for publishing
	var jsEvents []JsEvent
	var forEvents []ForEvent
	for _, result := range configResults {
		switch result.NodeKind {
		case mflow.NODE_KIND_FOR:
			if data, ok := result.Config.(mflow.NodeFor); ok {
				forEvents = append(forEvents, ForEvent{
					Type:   forEventInsert,
					FlowID: versionFlowID,
					Node:   serializeNodeFor(data),
				})
			}
		case mflow.NODE_KIND_JS:
			if data, ok := result.Config.(mflow.NodeJS); ok {
				jsEvents = append(jsEvents, JsEvent{
					Type:   jsEventInsert,
					FlowID: versionFlowID,
					Node:   serializeNodeJs(data),
				})
			}
		default:
			// Other node kinds don't need type-specific events
		}
	}

	// Duplicate all edges with remapped node IDs
	edgeEvents := make([]EdgeEvent, 0, len(sourceEdges))
	for _, sourceEdge := range sourceEdges {
		newSourceID, sourceOK := nodeIDMapping[sourceEdge.SourceID.String()]
		newTargetID, targetOK := nodeIDMapping[sourceEdge.TargetID.String()]

		if !sourceOK || !targetOK {
			continue
		}

		newEdge := mflow.Edge{
			ID:            idwrap.NewMonotonic(),
			FlowID:        versionFlowID,
			SourceID:      newSourceID,
			TargetID:      newTargetID,
			SourceHandler: sourceEdge.SourceHandler,
		}

		if err := edgeWriter.CreateEdge(ctx, newEdge); err != nil {
			return mflow.Flow{}, nil, fmt.Errorf("create edge: %w", err)
		}
		edgeEvents = append(edgeEvents, EdgeEvent{
			Type:   edgeEventInsert,
			FlowID: versionFlowID,
			Edge:   serializeEdge(newEdge),
		})
	}

	// Duplicate all flow variables
	varEvents := make([]FlowVariableEvent, 0, len(sourceVars))
	for _, sourceVar := range sourceVars {
		newVar := mflow.FlowVariable{
			ID:          idwrap.NewMonotonic(),
			FlowID:      versionFlowID,
			Name:        sourceVar.Name,
			Value:       sourceVar.Value,
			Enabled:     sourceVar.Enabled,
			Description: sourceVar.Description,
			Order:       sourceVar.Order,
		}

		if err := varWriter.CreateFlowVariable(ctx, newVar); err != nil {
			return mflow.Flow{}, nil, fmt.Errorf("create flow variable: %w", err)
		}
		varEvents = append(varEvents, FlowVariableEvent{
			Type:     flowVarEventInsert,
			FlowID:   versionFlowID,
			Variable: newVar,
		})
	}

	// === COMMIT TRANSACTION ===
	if err := tx.Commit(); err != nil {
		return mflow.Flow{}, nil, fmt.Errorf("commit transaction: %w", err)
	}

	// === PUBLISH EVENTS (AFTER SUCCESSFUL COMMIT) ===
	// Bulk publish sub-node events first
	if len(jsEvents) > 0 && s.jsStream != nil {
		s.jsStream.Publish(JsTopic{FlowID: versionFlowID}, jsEvents...)
	}
	if len(forEvents) > 0 && s.forStream != nil {
		s.forStream.Publish(ForTopic{FlowID: versionFlowID}, forEvents...)
	}

	// Bulk publish base node events
	if len(nodeEvents) > 0 && s.nodeStream != nil {
		s.nodeStream.Publish(NodeTopic{FlowID: versionFlowID}, nodeEvents...)
	}

	// Bulk publish edge events
	if len(edgeEvents) > 0 && s.edgeStream != nil {
		s.edgeStream.Publish(EdgeTopic{FlowID: versionFlowID}, edgeEvents...)
	}

	// Bulk publish variable events
	if len(varEvents) > 0 && s.varStream != nil {
		s.varStream.Publish(FlowVariableTopic{FlowID: versionFlowID}, varEvents...)
	}

	return version, nodeIDMapping, nil
}

// moveParentExecutionsToPreviousVersion moves existing parent node executions
// to the previous version's corresponding nodes. This ensures parent nodes always
// show only the current/latest run's executions.
func (s *FlowServiceV2RPC) moveParentExecutionsToPreviousVersion(
	ctx context.Context,
	flow mflow.Flow,
) error {
	// Get the most recent version of this flow (will have the mapping we need)
	prevVersion, err := s.fs.GetLatestVersionByParentID(ctx, flow.ID)
	if err != nil {
		return fmt.Errorf("get latest version: %w", err)
	}
	if prevVersion == nil {
		// No previous version exists, nothing to move (first run)
		return nil
	}

	// Parse the stored nodeIDMapping from the previous version
	if len(prevVersion.NodeIDMapping) == 0 {
		// No mapping stored, nothing to move
		return nil
	}

	var nodeIDMapping map[string]string
	if err := json.Unmarshal(prevVersion.NodeIDMapping, &nodeIDMapping); err != nil {
		return fmt.Errorf("unmarshal nodeIDMapping: %w", err)
	}

	// Move each parent node's executions to the previous version's corresponding node.
	// Note: node/edge state copying is handled by copyStatesToVersion (called after execution).
	// This function only moves execution records as housekeeping.
	for parentNodeIDStr, versionNodeIDStr := range nodeIDMapping {
		parentNodeID, err := idwrap.NewText(parentNodeIDStr)
		if err != nil {
			s.logger.Warn("invalid parent node ID in mapping", "id", parentNodeIDStr, "error", err)
			continue
		}
		versionNodeID, err := idwrap.NewText(versionNodeIDStr)
		if err != nil {
			s.logger.Warn("invalid version node ID in mapping", "id", versionNodeIDStr, "error", err)
			continue
		}

		// Get existing executions for this parent node
		executions, err := s.nes.ListNodeExecutionsByNodeID(ctx, parentNodeID)
		if err != nil {
			s.logger.Warn("failed to list node executions", "node_id", parentNodeIDStr, "error", err)
			continue
		}
		if len(executions) == 0 {
			continue
		}

		// Move each execution to the version node
		for _, exec := range executions {
			if err := s.nes.UpdateNodeExecutionNodeID(ctx, exec.ID, versionNodeID); err != nil {
				s.logger.Error("failed to move execution to version node",
					"exec_id", exec.ID.String(),
					"from_node", parentNodeIDStr,
					"to_node", versionNodeIDStr,
					"error", err)
				continue
			}

			// Publish sync events: DELETE from parent flow, INSERT into version flow
			s.publishExecutionEvent(executionEventDelete, exec, flow.ID)
			exec.NodeID = versionNodeID
			s.publishExecutionEvent(executionEventInsert, exec, prevVersion.ID)
		}
	}

	return nil
}

// copyStatesToVersion copies the final execution states from parent nodes/edges
// to the corresponding version nodes/edges. This is called immediately after execution
// completes so that every version is viewable with correct results right away,
// without waiting for the next execution to trigger lazy migration.
func (s *FlowServiceV2RPC) copyStatesToVersion(
	ctx context.Context,
	parentFlowID idwrap.IDWrap,
	versionFlowID idwrap.IDWrap,
	nodeIDMapping map[string]idwrap.IDWrap,
) {
	// --- Copy node states ---
	nodeEvents := make([]NodeEvent, 0, len(nodeIDMapping))
	for parentNodeIDStr, versionNodeID := range nodeIDMapping {
		parentNodeID, err := idwrap.NewText(parentNodeIDStr)
		if err != nil {
			continue
		}

		parentNode, err := s.nsReader.GetNode(ctx, parentNodeID)
		if err != nil || parentNode == nil {
			s.logger.Warn("copyStatesToVersion: failed to read parent node", "node_id", parentNodeIDStr, "error", err)
			continue
		}

		if err := s.ns.UpdateNodeState(ctx, versionNodeID, parentNode.State); err != nil {
			s.logger.Warn("copyStatesToVersion: failed to update version node state", "node_id", versionNodeID.String(), "error", err)
			continue
		}

		versionNode := *parentNode
		versionNode.ID = versionNodeID
		versionNode.FlowID = versionFlowID
		nodeEvents = append(nodeEvents, NodeEvent{
			Type:   nodeEventUpdate,
			FlowID: versionFlowID,
			Node:   serializeNode(versionNode),
		})
	}
	if len(nodeEvents) > 0 && s.nodeStream != nil {
		s.nodeStream.Publish(NodeTopic{FlowID: versionFlowID}, nodeEvents...)
	}

	// --- Copy edge states ---
	parentEdges, err := s.es.GetEdgesByFlowID(ctx, parentFlowID)
	if err != nil {
		s.logger.Warn("copyStatesToVersion: failed to read parent edges", "error", err)
		return
	}
	versionEdges, err := s.es.GetEdgesByFlowID(ctx, versionFlowID)
	if err != nil {
		s.logger.Warn("copyStatesToVersion: failed to read version edges", "error", err)
		return
	}

	// Build lookup of version edges by (source, target) pair
	type edgeKey struct{ Source, Target string }
	versionEdgeMap := make(map[edgeKey]*mflow.Edge, len(versionEdges))
	for i := range versionEdges {
		ve := &versionEdges[i]
		versionEdgeMap[edgeKey{ve.SourceID.String(), ve.TargetID.String()}] = ve
	}

	edgeEvents := make([]EdgeEvent, 0, len(parentEdges))
	for _, pe := range parentEdges {
		vSourceID, ok1 := nodeIDMapping[pe.SourceID.String()]
		vTargetID, ok2 := nodeIDMapping[pe.TargetID.String()]
		if !ok1 || !ok2 {
			continue
		}

		ve, ok := versionEdgeMap[edgeKey{vSourceID.String(), vTargetID.String()}]
		if !ok {
			continue
		}

		if err := s.es.UpdateEdgeState(ctx, ve.ID, pe.State); err != nil {
			s.logger.Warn("copyStatesToVersion: failed to update version edge state", "edge_id", ve.ID.String(), "error", err)
			continue
		}

		updatedEdge := *ve
		updatedEdge.State = pe.State
		edgeEvents = append(edgeEvents, EdgeEvent{
			Type:   edgeEventUpdate,
			FlowID: versionFlowID,
			Edge:   serializeEdge(updatedEdge),
		})
	}
	if len(edgeEvents) > 0 && s.edgeStream != nil {
		s.edgeStream.Publish(EdgeTopic{FlowID: versionFlowID}, edgeEvents...)
	}
}
