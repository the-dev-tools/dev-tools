//nolint:revive // exported
package rflowv2

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"connectrpc.com/connect"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

	devtoolsdb "github.com/the-dev-tools/dev-tools/packages/db"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rlog"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/ngraphql"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/nrequest"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner/flowlocalrunner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/httpclient"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mcondition"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mgraphql"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	flowv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/flow/v1"
	logv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/log/v1"
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

		defer func() {
			// Cleanup
			s.runningFlowsMu.Lock()
			delete(s.runningFlows, flowID.String())
			s.runningFlowsMu.Unlock()
			cancel()
		}()

		if err := s.executeFlow(bgCtx, flow, nodes, edges, flowVars, version.ID, nodeIDMapping); err != nil {
			// Check if error is due to cancellation
			if errors.Is(err, context.Canceled) {
				s.logger.Info("flow execution canceled", "flow_id", flowID.String())
			} else {
				s.logger.Error("async flow execution failed", "flow_id", flowID.String(), "error", err)
			}
		}
	}()

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) executeFlow(
	ctx context.Context,
	flow mflow.Flow,
	nodes []mflow.Node,
	edges []mflow.Edge,
	flowVars []mflow.FlowVariable,
	versionFlowID idwrap.IDWrap,
	nodeIDMapping map[string]idwrap.IDWrap,
) error {
	flow.Running = true
	if err := s.fs.UpdateFlow(ctx, flow); err != nil {
		return fmt.Errorf("failed to mark flow as running: %w", err)
	}
	s.publishFlowEvent(flowEventUpdate, flow)

	// Build base variables by merging: GlobalEnv -> ActiveEnv -> FlowVars
	// Later values override earlier ones
	baseVars, err := s.builder.BuildVariables(ctx, flow.WorkspaceID, flowVars)
	if err != nil {
		return fmt.Errorf("failed to build execution variables: %w", err)
	}

	requestRespChan := make(chan nrequest.NodeRequestSideResp, len(nodes)*2+1)
	// Track when HTTP responses are published so node execution events can wait
	// This ensures frontend receives HttpResponse before NodeExecution with ResponseID
	responsePublished := make(map[string]chan struct{})
	var responsePublishedMu sync.Mutex
	var respDrain sync.WaitGroup
	respDrain.Add(1)
	go func() {
		defer respDrain.Done()
		for resp := range requestRespChan {
			responseID := resp.Resp.HTTPResponse.ID.String()

			// Register the channel before processing so nodeStateChan can find it
			responsePublishedMu.Lock()
			publishedChan := make(chan struct{})
			responsePublished[responseID] = publishedChan
			responsePublishedMu.Unlock()

			// Save HTTP Response
			if err := s.httpResponseService.Create(ctx, resp.Resp.HTTPResponse); err != nil {
				s.logger.Error("failed to save http response", "error", err)
			} else {
				s.publishHttpResponseEvent("insert", resp.Resp.HTTPResponse, flow.WorkspaceID)
			}

			// Save Headers
			for _, h := range resp.Resp.ResponseHeaders {
				if err := s.httpResponseService.CreateHeader(ctx, h); err != nil {
					s.logger.Error("failed to save http response header", "error", err)
				} else {
					s.publishHttpResponseHeaderEvent("insert", h, flow.WorkspaceID)
				}
			}

			// Save Asserts
			for _, a := range resp.Resp.ResponseAsserts {
				if err := s.httpResponseService.CreateAssert(ctx, a); err != nil {
					s.logger.Error("failed to save http response assert", "error", err)
				} else {
					s.publishHttpResponseAssertEvent("insert", a, flow.WorkspaceID)
				}
			}

			// Signal that response is published - nodeStateChan can now publish execution
			close(publishedChan)

			if resp.Done != nil {
				close(resp.Done)
			}
		}
	}()
	defer func() {
		close(requestRespChan)
		respDrain.Wait()
	}()

	gqlRespChan := make(chan ngraphql.NodeGraphQLSideResp, len(nodes)*2+1)
	gqlResponsePublished := make(map[string]chan struct{})
	var gqlResponsePublishedMu sync.Mutex
	var gqlRespDrain sync.WaitGroup
	gqlRespDrain.Add(1)
	go func() {
		defer gqlRespDrain.Done()
		for resp := range gqlRespChan {
			responseID := resp.Response.ID.String()

			gqlResponsePublishedMu.Lock()
			publishedChan := make(chan struct{})
			gqlResponsePublished[responseID] = publishedChan
			gqlResponsePublishedMu.Unlock()

			// Save all entities first, THEN publish events in batch
			// This ensures atomicity and ordering - the client can query for
			// child entities (headers/assertions) immediately after receiving
			// the response event, preventing race conditions in real-time updates

			// Save GraphQL Response
			responseSuccess := false
			if err := s.graphqlResponseService.Create(ctx, resp.Response); err != nil {
				s.logger.Error("failed to save graphql response", "error", err)
			} else {
				responseSuccess = true
			}

			// Save Response Headers
			var successHeaders []mgraphql.GraphQLResponseHeader
			for _, h := range resp.RespHeaders {
				if err := s.graphqlResponseService.CreateHeader(ctx, h); err != nil {
					s.logger.Error("failed to save graphql response header", "error", err)
				} else {
					successHeaders = append(successHeaders, h)
				}
			}

			// Save Asserts
			var successAsserts []mgraphql.GraphQLResponseAssert
			for _, a := range resp.RespAsserts {
				if err := s.graphqlResponseService.CreateAssert(ctx, a); err != nil {
					s.logger.Error("failed to save graphql response assert", "error", err)
				} else {
					successAsserts = append(successAsserts, a)
				}
			}

			// Publish all events atomically AFTER all saves complete
			// This guarantees the client receives events in the correct order:
			// 1. Response (parent)
			// 2. Headers (children)
			// 3. Assertions (children)
			if responseSuccess {
				// Publish response first
				s.publishGraphQLResponseEvent("insert", resp.Response, flow.WorkspaceID)

				// Then headers
				for _, h := range successHeaders {
					s.publishGraphQLResponseHeaderEvent("insert", h, flow.WorkspaceID)
				}

				// Then assertions
				for _, a := range successAsserts {
					s.publishGraphQLResponseAssertEvent("insert", a, flow.WorkspaceID)
				}
			}

			close(publishedChan)

			if resp.Done != nil {
				close(resp.Done)
			}
		}
	}()
	defer func() {
		close(gqlRespChan)
		gqlRespDrain.Wait()
	}()

	sharedHTTPClient := httpclient.New()
	edgeMap := mflow.NewEdgesMap(edges)
	// Build edgesBySource map for O(1) edge lookup by source node ID
	edgesBySource := make(map[idwrap.IDWrap][]mflow.Edge, len(edges))
	for _, edge := range edges {
		edgesBySource[edge.SourceID] = append(edgesBySource[edge.SourceID], edge)
	}

	const defaultNodeTimeout = 60 // seconds
	timeoutDuration := time.Duration(defaultNodeTimeout) * time.Second

	flowNodeMap, startNodeID, err := s.builder.BuildNodes(
		ctx,
		flow,
		nodes,
		timeoutDuration,
		sharedHTTPClient,
		requestRespChan,
		gqlRespChan,
		s.jsClient,
	)
	if err != nil {
		return err
	}

	flowRunner := flowlocalrunner.CreateFlowRunner(idwrap.NewMonotonic(), flow.ID, startNodeID, flowNodeMap, edgeMap, 0, nil)

	// Reset all node states to UNSPECIFIED before flow execution
	nodeResetEvents := make([]NodeEvent, 0, len(nodes))
	for _, node := range nodes {
		if err := s.ns.UpdateNodeState(ctx, node.ID, mflow.NODE_STATE_UNSPECIFIED); err != nil {
			s.logger.Error("failed to reset node state", "node_id", node.ID.String(), "error", err)
		} else {
			resetNode := node
			resetNode.State = mflow.NODE_STATE_UNSPECIFIED
			nodeResetEvents = append(nodeResetEvents, NodeEvent{
				Type:   nodeEventUpdate,
				FlowID: flow.ID,
				Node:   serializeNode(resetNode),
			})
		}
	}
	// Bulk publish node reset events for real-time sync
	if len(nodeResetEvents) > 0 && s.nodeStream != nil {
		s.nodeStream.Publish(NodeTopic{FlowID: flow.ID}, nodeResetEvents...)
	}

	// Reset all edge states to UNSPECIFIED before flow execution
	edgeResetEvents := make([]EdgeEvent, 0, len(edges))
	for _, edge := range edges {
		if err := s.es.UpdateEdgeState(ctx, edge.ID, mflow.NODE_STATE_UNSPECIFIED); err != nil {
			s.logger.Error("failed to reset edge state", "edge_id", edge.ID.String(), "error", err)
		} else {
			resetEdge := edge
			resetEdge.State = mflow.NODE_STATE_UNSPECIFIED
			edgeResetEvents = append(edgeResetEvents, EdgeEvent{
				Type:   edgeEventUpdate,
				FlowID: flow.ID,
				Edge:   serializeEdge(resetEdge),
			})
		}
	}
	// Bulk publish edge reset events for real-time sync
	if len(edgeResetEvents) > 0 && s.edgeStream != nil {
		s.edgeStream.Publish(EdgeTopic{FlowID: flow.ID}, edgeResetEvents...)
	}

	// Build nodeKindMap for O(1) lookup of node kinds
	// Used to skip NodeExecution creation for loop coordinator wrapper statuses
	nodeKindMap := make(map[idwrap.IDWrap]mflow.NodeKind, len(nodes))
	for _, node := range nodes {
		nodeKindMap[node.ID] = node.NodeKind
	}

	nodeStateChan := make(chan runner.FlowNodeStatus, len(nodes)*2+1)
	var stateDrain sync.WaitGroup
	stateDrain.Add(1)

	// Create inverse mapping to map versioned IDs back to original IDs for live sync
	inverseNodeIDMapping := make(map[string]idwrap.IDWrap, len(nodeIDMapping))
	for k, v := range nodeIDMapping {
		inverseNodeIDMapping[v.String()] = idwrap.NewTextMust(k)
	}

	go func() {
		defer stateDrain.Done()

		// Cache execution IDs to ensure stability across multiple events for the same execution
		// Key: NodeID + Iteration info
		executionCache := make(map[string]idwrap.IDWrap)

		for status := range nodeStateChan {
			// Find the original node ID if this is a versioned ID
			originalNodeID := status.NodeID
			if origID, ok := inverseNodeIDMapping[status.NodeID.String()]; ok {
				originalNodeID = origID
			}

			// Check if this is a loop coordinator (For/ForEach) wrapper status
			// Skip NodeExecution creation for these, but still update node visual state
			nodeKind := nodeKindMap[status.NodeID]
			isLoopNode := nodeKind == mflow.NODE_KIND_FOR || nodeKind == mflow.NODE_KIND_FOR_EACH
			skipExecution := isLoopNode && !status.IterationEvent

			// Persist execution state (skip for loop node wrapper statuses)
			if !skipExecution {
				execID := status.ExecutionID
				isNewExecution := false

				if isZeroID(execID) {
					// Construct cache key based on node and iteration context
					cacheKey := status.NodeID.String()
					if status.IterationContext != nil {
						// Use iteration path and index for uniqueness in loops
						cacheKey = fmt.Sprintf("%s:%v:%d", cacheKey, status.IterationContext.IterationPath, status.IterationContext.ExecutionIndex)
					} else if status.IterationIndex >= 0 {
						cacheKey = fmt.Sprintf("%s:%d", cacheKey, status.IterationIndex)
					}

					if cachedID, ok := executionCache[cacheKey]; ok {
						execID = cachedID
					} else {
						execID = idwrap.NewMonotonic()
						executionCache[cacheKey] = execID
						isNewExecution = true
					}
				}

				// Include timestamp in execution name for easy identification
				executionName := fmt.Sprintf("%s - %s", status.Name, time.Now().Format("2006-01-02 15:04"))

				// Debug: Log AuxiliaryID value being set
				if status.AuxiliaryID != nil {
					s.logger.Debug("Creating execution with AuxiliaryID",
						"exec_id", execID.String(),
						"node_id", status.NodeID.String(),
						"node_name", status.Name,
						"state", status.State,
						"auxiliary_id", status.AuxiliaryID.String(),
					)
				} else {
					s.logger.Debug("Creating execution without AuxiliaryID",
						"exec_id", execID.String(),
						"node_id", status.NodeID.String(),
						"node_name", status.Name,
						"state", status.State,
					)
				}

				model := mflow.NodeExecution{
					ID:     execID,
					NodeID: status.NodeID,
					Name:   executionName,
					State:  status.State,
				}

				// Set the appropriate response ID based on node kind
				nodeKindForAux := nodeKindMap[status.NodeID]
				if status.AuxiliaryID != nil {
					if nodeKindForAux == mflow.NODE_KIND_GRAPHQL {
						model.GraphQLResponseID = status.AuxiliaryID
					} else {
						model.ResponseID = status.AuxiliaryID
					}
				}

				if status.Error != nil {
					errStr := status.Error.Error()
					model.Error = &errStr
				}

				if status.InputData != nil {
					if b, err := json.Marshal(status.InputData); err == nil {
						_ = model.SetInputJSON(b)
					}
				}
				if status.OutputData != nil {
					if b, err := json.Marshal(status.OutputData); err == nil {
						_ = model.SetOutputJSON(b)
					}
				}

				// Set CompletedAt for terminal states
				if status.State == mflow.NODE_STATE_SUCCESS ||
					status.State == mflow.NODE_STATE_FAILURE ||
					status.State == mflow.NODE_STATE_CANCELED {
					now := time.Now().Unix()
					model.CompletedAt = &now
				}

				eventType := executionEventInsert
				// Only use UPDATE if it's NOT a new execution AND the state is terminal.
				// If it's a new execution (first time seeing this node run), we MUST send INSERT,
				// even if the state is already SUCCESS/FAILURE (instant execution).
				if !isNewExecution && (status.State == mflow.NODE_STATE_SUCCESS ||
					status.State == mflow.NODE_STATE_FAILURE ||
					status.State == mflow.NODE_STATE_CANCELED) {
					eventType = executionEventUpdate
				}

				if err := s.nes.UpsertNodeExecution(ctx, model); err != nil {
					s.logger.Error("failed to persist node execution", "error", err)
				}

				// If this execution has a ResponseID, wait for the response to be published first
				// This ensures frontend receives HttpResponse/GraphQLResponse before NodeExecution
				if status.AuxiliaryID != nil {
					respIDStr := status.AuxiliaryID.String()

					// Check HTTP response published map
					responsePublishedMu.Lock()
					publishedChan, ok := responsePublished[respIDStr]
					responsePublishedMu.Unlock()
					if ok {
						select {
						case <-publishedChan:
						case <-ctx.Done():
						}
						responsePublishedMu.Lock()
						delete(responsePublished, respIDStr)
						responsePublishedMu.Unlock()
					}

					// Check GraphQL response published map
					gqlResponsePublishedMu.Lock()
					gqlPublishedChan, gqlOK := gqlResponsePublished[respIDStr]
					gqlResponsePublishedMu.Unlock()
					if gqlOK {
						select {
						case <-gqlPublishedChan:
						case <-ctx.Done():
						}
						gqlResponsePublishedMu.Lock()
						delete(gqlResponsePublished, respIDStr)
						gqlResponsePublishedMu.Unlock()
					}
				}

				// Publish execution event
				s.publishExecutionEvent(eventType, model, flow.ID)
			}

			// Update node state in database (always use versioned ID for state persistence)
			if err := s.ns.UpdateNodeState(ctx, status.NodeID, status.State); err != nil {
				s.logger.Error("failed to update node state", "node_id", status.NodeID.String(), "error", err)
			}

			// Update edge states based on node execution state
			if status.State == mflow.NODE_STATE_SUCCESS || status.State == mflow.NODE_STATE_FAILURE {
				// Find edges that start from this node using O(1) map lookup
				edgesFromNode := edgesBySource[status.NodeID]
				edgeState := mflow.NODE_STATE_SUCCESS
				if status.State == mflow.NODE_STATE_FAILURE {
					edgeState = mflow.NODE_STATE_FAILURE
				}
				for _, edge := range edgesFromNode {
					if err := s.es.UpdateEdgeState(ctx, edge.ID, edgeState); err != nil {
						s.logger.Error("failed to update edge state", "edge_id", edge.ID.String(), "error", err)
					} else {
						// Publish edge state update event for real-time sync
						updatedEdge := edge
						updatedEdge.State = edgeState
						s.publishEdgeEvent(edgeEventUpdate, updatedEdge)
					}
				}
			}

			// Note: Version node executions are no longer created here.
			// Executions are moved from parent nodes to version nodes at the start of the next run.

			if s.nodeStream != nil {
				var info string
				if status.Error != nil {
					info = status.Error.Error()
				} else {
					iterIndex := -1
					if status.IterationEvent {
						iterIndex = status.IterationIndex
					} else if status.IterationContext != nil {
						iterIndex = status.IterationContext.ExecutionIndex
					}

					if iterIndex >= 0 {
						info = fmt.Sprintf("Iter: %d", iterIndex+1)
					}
				}

				// Map versioned node ID back to original node ID for live sync on current view
				nodePB := &flowv1.Node{
					NodeId: originalNodeID.Bytes(),
					FlowId: flow.ID.Bytes(),
					State:  flowv1.FlowItemState(status.State),
				}
				if info != "" {
					nodePB.Info = &info
				}

				s.nodeStream.Publish(NodeTopic{FlowID: flow.ID}, NodeEvent{
					Type:   nodeEventUpdate,
					FlowID: flow.ID,
					Node:   nodePB,
				})
			}

			if s.logStream != nil && status.State != mflow.NODE_STATE_RUNNING {
				idStr := status.NodeID.String()
				stateStr := mflow.StringNodeState(status.State)
				nodeName := status.Name
				if nodeName == "" {
					nodeName = idStr
				}
				msg := fmt.Sprintf("Node %s: %s", nodeName, stateStr)

				var logLevel logv1.LogLevel
				switch status.State {
				case mflow.NODE_STATE_FAILURE:
					logLevel = logv1.LogLevel_LOG_LEVEL_ERROR
				case mflow.NODE_STATE_CANCELED:
					logLevel = logv1.LogLevel_LOG_LEVEL_WARNING
				default:
					logLevel = logv1.LogLevel_LOG_LEVEL_UNSPECIFIED
				}

				// Create structured value with full node details
				logData := map[string]any{
					"node_id":     status.NodeID.String(),
					"node_name":   status.Name,
					"state":       stateStr,
					"flow_id":     flow.ID.String(),
					"duration_ms": status.RunDuration.Milliseconds(),
				}

				// Convert output/input to JSON-safe format via marshal/unmarshal
				// This ensures types like []byte are properly converted
				// Limit size to avoid very large log entries that could slow down the frontend
				const maxLogDataSize = 64 * 1024 // 64KB limit
				if status.OutputData != nil {
					if jsonBytes, err := json.Marshal(status.OutputData); err == nil {
						if len(jsonBytes) <= maxLogDataSize {
							var jsonSafe any
							if json.Unmarshal(jsonBytes, &jsonSafe) == nil {
								logData["output"] = jsonSafe
							}
						} else {
							logData["output"] = "(output too large to display)"
						}
					}
				}
				if status.InputData != nil {
					if jsonBytes, err := json.Marshal(status.InputData); err == nil {
						if len(jsonBytes) <= maxLogDataSize {
							var jsonSafe any
							if json.Unmarshal(jsonBytes, &jsonSafe) == nil {
								logData["input"] = jsonSafe
							}
						} else {
							logData["input"] = "(input too large to display)"
						}
					}
				}
				if status.Error != nil {
					logData["error"] = status.Error.Error()
				}
				if status.IterationContext != nil {
					logData["iteration_index"] = status.IterationContext.ExecutionIndex
					logData["iteration_path"] = status.IterationContext.IterationPath
				}

				val, err := rlog.NewLogValue(logData)
				if err != nil {
					s.logger.Error("failed to create log value", "error", err)
				}

				s.logStream.Publish(rlog.LogTopic{}, rlog.LogEvent{
					Type: rlog.EventTypeInsert,
					Log: &logv1.Log{
						LogId: idwrap.NewMonotonic().Bytes(),
						Name:  msg,
						Level: logLevel,
						Value: val,
					},
				})
			}
		}
	}()

	startTime := time.Now()
	runErr := flowRunner.RunWithEvents(ctx, runner.FlowEventChannels{
		NodeStates: nodeStateChan,
	}, baseVars)

	duration := time.Since(startTime).Milliseconds()
	if duration > math.MaxInt32 {
		duration = math.MaxInt32
	}
	//nolint:gosec // duration clamped to MaxInt32
	flow.Duration = int32(duration)

	flow.Running = false
	if err := s.fs.UpdateFlow(context.Background(), flow); err != nil {
		s.logger.Error("failed to mark flow as not running", "error", err)
	}
	s.publishFlowEvent(flowEventUpdate, flow)

	stateDrain.Wait()
	return runErr
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

	if !ok {
		// Flow is not running, which is fine for a stop request (idempotent)
		return connect.NewResponse(&emptypb.Empty{}), nil
	}

	// Cancel the flow
	cancel()

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
	// This follows SQLite best practices of keeping transactions short and doing reads outside when possible.

	type nodeConfig struct {
		sourceNode     mflow.Node
		requestData    *mflow.NodeRequest
		forData        *mflow.NodeFor
		forEachData    *mflow.NodeForEach
		conditionData  *mflow.NodeIf
		jsData         *mflow.NodeJS
		aiData         *mflow.NodeAI
		aiProviderData *mflow.NodeAiProvider
		memoryData     *mflow.NodeMemory
		graphqlData    *mflow.NodeGraphQL
	}

	nodeConfigs := make([]nodeConfig, 0, len(sourceNodes))

	for _, sourceNode := range sourceNodes {
		config := nodeConfig{sourceNode: sourceNode}

		switch sourceNode.NodeKind {
		case mflow.NODE_KIND_REQUEST:
			requestData, err := s.nrs.GetNodeRequest(ctx, sourceNode.ID)
			if err == nil && requestData != nil {
				config.requestData = requestData
			}

		case mflow.NODE_KIND_FOR:
			forData, err := s.nfs.GetNodeFor(ctx, sourceNode.ID)
			if err != nil {
				s.logger.Warn("failed to get for node config, using defaults", "node_id", sourceNode.ID.String(), "error", err)
			} else if forData != nil {
				config.forData = forData
			}

		case mflow.NODE_KIND_FOR_EACH:
			forEachData, err := s.nfes.GetNodeForEach(ctx, sourceNode.ID)
			if err != nil {
				s.logger.Warn("failed to get foreach node config, using defaults", "node_id", sourceNode.ID.String(), "error", err)
			} else if forEachData != nil {
				config.forEachData = forEachData
			}

		case mflow.NODE_KIND_CONDITION:
			conditionData, err := s.nifs.GetNodeIf(ctx, sourceNode.ID)
			if err != nil {
				s.logger.Warn("failed to get condition node config, using defaults", "node_id", sourceNode.ID.String(), "error", err)
			} else if conditionData != nil {
				config.conditionData = conditionData
			}

		case mflow.NODE_KIND_JS:
			jsData, err := s.njss.GetNodeJS(ctx, sourceNode.ID)
			if err != nil {
				s.logger.Warn("failed to get js node config, using defaults", "node_id", sourceNode.ID.String(), "error", err)
			} else if jsData != nil {
				config.jsData = jsData
			}

		case mflow.NODE_KIND_AI:
			aiData, err := s.nais.GetNodeAI(ctx, sourceNode.ID)
			if err != nil {
				s.logger.Warn("failed to get ai node config, using defaults", "node_id", sourceNode.ID.String(), "error", err)
			} else if aiData != nil {
				config.aiData = aiData
			}

		case mflow.NODE_KIND_AI_PROVIDER:
			aiProviderData, err := s.naps.GetNodeAiProvider(ctx, sourceNode.ID)
			if err != nil {
				s.logger.Warn("failed to get ai provider node config, using defaults", "node_id", sourceNode.ID.String(), "error", err)
			} else if aiProviderData != nil {
				config.aiProviderData = aiProviderData
			}

		case mflow.NODE_KIND_AI_MEMORY:
			memoryData, err := s.nmems.GetNodeMemory(ctx, sourceNode.ID)
			if err != nil {
				s.logger.Warn("failed to get memory node config, using defaults", "node_id", sourceNode.ID.String(), "error", err)
			} else if memoryData != nil {
				config.memoryData = memoryData
			}

		case mflow.NODE_KIND_GRAPHQL:
			graphqlData, err := s.ngqs.GetNodeGraphQL(ctx, sourceNode.ID)
			if err != nil {
				s.logger.Warn("failed to get graphql node config, using defaults", "node_id", sourceNode.ID.String(), "error", err)
			} else if graphqlData != nil {
				config.graphqlData = graphqlData
			}
		}

		nodeConfigs = append(nodeConfigs, config)
	}

	// === BEGIN TRANSACTION ===
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return mflow.Flow{}, nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer devtoolsdb.TxnRollback(tx)

	// Get TX-bound service writers
	flowWriter := s.fs.TX(tx)
	nodeWriter := s.ns.TX(tx)
	nrsWriter := s.nrs.TX(tx)
	nfsWriter := s.nfs.TX(tx)
	nfesWriter := s.nfes.TX(tx)
	nifsWriter := s.nifs.TX(tx)
	njssWriter := s.njss.TX(tx)
	var naisWriter *sflow.NodeAIService
	if s.nais != nil {
		txService := s.nais.TX(tx)
		naisWriter = &txService
	}
	var napsWriter *sflow.NodeAiProviderService
	if s.naps != nil {
		txService := s.naps.TX(tx)
		napsWriter = &txService
	}
	var nmemsWriter *sflow.NodeMemoryService
	if s.nmems != nil {
		txService := s.nmems.TX(tx)
		nmemsWriter = &txService
	}
	var ngqsWriter *sflow.NodeGraphQLService
	if s.ngqs != nil {
		txService := s.ngqs.TX(tx)
		ngqsWriter = &txService
	}
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
	jsEvents := make([]JsEvent, 0)
	forEvents := make([]ForEvent, 0)

	// Duplicate all nodes and their sub-node data
	for _, config := range nodeConfigs {
		sourceNode := config.sourceNode
		newNodeID := idwrap.NewMonotonic()
		nodeIDMapping[sourceNode.ID.String()] = newNodeID

		// Create the base node (including State to preserve execution status in snapshot)
		newNode := mflow.Node{
			ID:        newNodeID,
			FlowID:    versionFlowID,
			Name:      sourceNode.Name,
			NodeKind:  sourceNode.NodeKind,
			PositionX: sourceNode.PositionX,
			PositionY: sourceNode.PositionY,
			State:     sourceNode.State,
		}

		// Use CreateNodeWithState to preserve the execution state in the snapshot
		if err := nodeWriter.CreateNodeWithState(ctx, newNode); err != nil {
			return mflow.Flow{}, nil, fmt.Errorf("create node %s: %w", sourceNode.Name, err)
		}

		// Duplicate node-type specific data
		switch sourceNode.NodeKind {
		case mflow.NODE_KIND_REQUEST:
			if config.requestData != nil {
				// Copy the request node config (referencing same HTTP, not duplicating)
				newRequestData := mflow.NodeRequest{
					FlowNodeID:       newNodeID,
					HttpID:           config.requestData.HttpID,
					DeltaHttpID:      config.requestData.DeltaHttpID,
					HasRequestConfig: config.requestData.HasRequestConfig,
				}
				if err := nrsWriter.CreateNodeRequest(ctx, newRequestData); err != nil {
					return mflow.Flow{}, nil, fmt.Errorf("create request node: %w", err)
				}
				// Request node events are handled through nodeStream subscription
			}

		case mflow.NODE_KIND_FOR:
			// Always create For node config with defaults
			newForData := mflow.NodeFor{
				FlowNodeID:    newNodeID,
				IterCount:     1,                                        // default
				Condition:     mcondition.Condition{},                   // default empty
				ErrorHandling: mflow.ErrorHandling_ERROR_HANDLING_BREAK, // default
			}
			if config.forData != nil {
				// Override with actual values, but keep default of 1 if IterCount is 0
				if config.forData.IterCount > 0 {
					newForData.IterCount = config.forData.IterCount
				}
				newForData.Condition = config.forData.Condition
				newForData.ErrorHandling = config.forData.ErrorHandling
			}
			if err := nfsWriter.CreateNodeFor(ctx, newForData); err != nil {
				return mflow.Flow{}, nil, fmt.Errorf("create for node: %w", err)
			}
			forEvents = append(forEvents, ForEvent{
				Type:   forEventInsert,
				FlowID: versionFlowID,
				Node:   serializeNodeFor(newForData),
			})

		case mflow.NODE_KIND_FOR_EACH:
			// Always create ForEach node config with defaults
			newForEachData := mflow.NodeForEach{
				FlowNodeID:     newNodeID,
				IterExpression: "",                                       // default empty
				Condition:      mcondition.Condition{},                   // default empty
				ErrorHandling:  mflow.ErrorHandling_ERROR_HANDLING_BREAK, // default
			}
			if config.forEachData != nil {
				// Override with actual values
				newForEachData.IterExpression = config.forEachData.IterExpression
				newForEachData.Condition = config.forEachData.Condition
				newForEachData.ErrorHandling = config.forEachData.ErrorHandling
			}
			if err := nfesWriter.CreateNodeForEach(ctx, newForEachData); err != nil {
				return mflow.Flow{}, nil, fmt.Errorf("create foreach node: %w", err)
			}
			// ForEach node events are handled through nodeStream subscription

		case mflow.NODE_KIND_CONDITION:
			// Always create Condition node config with defaults
			newConditionData := mflow.NodeIf{
				FlowNodeID: newNodeID,
				Condition:  mcondition.Condition{}, // default empty
			}
			if config.conditionData != nil {
				// Override with actual values
				newConditionData.Condition = config.conditionData.Condition
			}
			if err := nifsWriter.CreateNodeIf(ctx, newConditionData); err != nil {
				return mflow.Flow{}, nil, fmt.Errorf("create condition node: %w", err)
			}
			// Condition node events are handled through nodeStream subscription

		case mflow.NODE_KIND_JS:
			// Always create JS node config with defaults
			newJsData := mflow.NodeJS{
				FlowNodeID:       newNodeID,
				Code:             nil, // default empty
				CodeCompressType: 0,   // default none
			}
			if config.jsData != nil {
				// Override with actual values
				newJsData.Code = config.jsData.Code
				newJsData.CodeCompressType = config.jsData.CodeCompressType
			}
			if err := njssWriter.CreateNodeJS(ctx, newJsData); err != nil {
				return mflow.Flow{}, nil, fmt.Errorf("create js node: %w", err)
			}
			jsEvents = append(jsEvents, JsEvent{
				Type:   jsEventInsert,
				FlowID: versionFlowID,
				Node:   serializeNodeJs(newJsData),
			})

		case mflow.NODE_KIND_AI:
			// Skip if AI service is not available
			if naisWriter == nil {
				s.logger.Warn("NodeAI service not available, skipping AI node config", "node_id", sourceNode.ID.String())
			} else {
				// Create AI node config (model/credential now via connected Model node)
				newAIData := mflow.NodeAI{
					FlowNodeID:    newNodeID,
					Prompt:        "",
					MaxIterations: 5,
				}
				if config.aiData != nil {
					newAIData.Prompt = config.aiData.Prompt
					newAIData.MaxIterations = config.aiData.MaxIterations
				}
				if err := naisWriter.CreateNodeAI(ctx, newAIData); err != nil {
					return mflow.Flow{}, nil, fmt.Errorf("create ai node: %w", err)
				}
				// AI node events are handled through nodeStream subscription
			}

		case mflow.NODE_KIND_AI_PROVIDER:
			// Skip if AI Provider service is not available
			if napsWriter == nil {
				s.logger.Warn("NodeAiProvider service not available, skipping AI Provider node config", "node_id", sourceNode.ID.String())
			} else {
				// Create AI Provider node config with defaults
				newAiProviderData := mflow.NodeAiProvider{
					FlowNodeID:   newNodeID,
					CredentialID: nil,
					Model:        mflow.AiModelUnspecified,
					Temperature:  nil,
					MaxTokens:    nil,
				}
				if config.aiProviderData != nil {
					newAiProviderData.CredentialID = config.aiProviderData.CredentialID
					newAiProviderData.Model = config.aiProviderData.Model
					newAiProviderData.Temperature = config.aiProviderData.Temperature
					newAiProviderData.MaxTokens = config.aiProviderData.MaxTokens
				}
				if err := napsWriter.CreateNodeAiProvider(ctx, newAiProviderData); err != nil {
					return mflow.Flow{}, nil, fmt.Errorf("create ai provider node: %w", err)
				}
				// AI Provider node events are handled through nodeStream subscription
			}

		case mflow.NODE_KIND_AI_MEMORY:
			// Skip if Memory service is not available
			if nmemsWriter == nil {
				s.logger.Warn("NodeMemory service not available, skipping Memory node config", "node_id", sourceNode.ID.String())
			} else {
				// Create Memory node config with defaults
				newMemoryData := mflow.NodeMemory{
					FlowNodeID: newNodeID,
					MemoryType: mflow.AiMemoryTypeWindowBuffer,
					WindowSize: 10, // Default window size
				}
				if config.memoryData != nil {
					newMemoryData.MemoryType = config.memoryData.MemoryType
					newMemoryData.WindowSize = config.memoryData.WindowSize
				}
				if err := nmemsWriter.CreateNodeMemory(ctx, newMemoryData); err != nil {
					return mflow.Flow{}, nil, fmt.Errorf("create memory node: %w", err)
				}
				// Memory node events are handled through nodeStream subscription
			}

		case mflow.NODE_KIND_GRAPHQL:
			if ngqsWriter == nil {
				s.logger.Warn("NodeGraphQL service not available, skipping GraphQL node config", "node_id", sourceNode.ID.String())
			} else if config.graphqlData != nil {
				newGraphQLData := mflow.NodeGraphQL{
					FlowNodeID: newNodeID,
					GraphQLID:  config.graphqlData.GraphQLID,
				}
				if err := ngqsWriter.CreateNodeGraphQL(ctx, newGraphQLData); err != nil {
					return mflow.Flow{}, nil, fmt.Errorf("create graphql node: %w", err)
				}
			}
		}

		// Collect base node event
		nodeEvents = append(nodeEvents, NodeEvent{
			Type:   nodeEventInsert,
			FlowID: versionFlowID,
			Node:   serializeNode(newNode),
		})
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

	// Move each parent node's executions to the previous version's corresponding node
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
