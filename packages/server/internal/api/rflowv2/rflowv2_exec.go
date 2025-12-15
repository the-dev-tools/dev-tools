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
	"google.golang.org/protobuf/types/known/structpb"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rlog"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node/nrequest"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/flow/runner/flowlocalrunner"
	"the-dev-tools/server/pkg/httpclient"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mflowvariable"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"the-dev-tools/server/pkg/model/mnnode/mnforeach"
	"the-dev-tools/server/pkg/model/mnnode/mnif"
	"the-dev-tools/server/pkg/model/mnnode/mnjs"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/model/mnodeexecution"
	"the-dev-tools/server/pkg/service/sflowvariable"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
	logv1 "the-dev-tools/spec/dist/buf/go/api/log/v1"
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
	if err != nil && !errors.Is(err, sflowvariable.ErrNoFlowVariableFound) {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Create a new flow version for this run (snapshot of the flow with all nodes, edges, etc.)
	version, nodeIDMapping, err := s.createFlowVersionSnapshot(ctx, flow, nodes, edges, flowVars)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create flow version: %w", err))
	}

	// Publish flow insert event so clients receive the version in FlowSync
	// (the version is a flow record that clients need to query)
	s.publishFlowEvent(flowEventInsert, version)

	// Publish version insert event for real-time sync
	s.publishFlowVersionEvent(flowVersionEventInsert, version)

	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

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

		if err := s.executeFlow(bgCtx, flow, nodes, edges, flowVars, version.ID, nodeIDMapping, userID); err != nil {
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
	nodes []mnnode.MNode,
	edges []edge.Edge,
	flowVars []mflowvariable.FlowVariable,
	versionFlowID idwrap.IDWrap,
	nodeIDMapping map[string]idwrap.IDWrap,
	userID idwrap.IDWrap,
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

	sharedHTTPClient := httpclient.New()
	edgeMap := edge.NewEdgesMap(edges)

	const defaultNodeTimeout = 60 // seconds
	timeoutDuration := time.Duration(defaultNodeTimeout) * time.Second

	flowNodeMap, startNodeID, err := s.builder.BuildNodes(
		ctx,
		flow,
		nodes,
		timeoutDuration,
		sharedHTTPClient,
		requestRespChan,
		s.jsClient,
	)
	if err != nil {
		return err
	}

	flowRunner := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), flow.ID, startNodeID, flowNodeMap, edgeMap, 0, nil)

	nodeStateChan := make(chan runner.FlowNodeStatus, len(nodes)*2+1)
	var stateDrain sync.WaitGroup
	stateDrain.Add(1)
	go func() {
		defer stateDrain.Done()
		for status := range nodeStateChan {
			// Persist execution state
			execID := status.ExecutionID
			if isZeroID(execID) {
				execID = idwrap.NewNow()
			}

			model := mnodeexecution.NodeExecution{
				ID:         execID,
				NodeID:     status.NodeID,
				Name:       status.Name,
				State:      status.State,
				ResponseID: status.AuxiliaryID,
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

			eventType := executionEventInsert
			if status.State == mnnode.NODE_STATE_SUCCESS ||
				status.State == mnnode.NODE_STATE_FAILURE ||
				status.State == mnnode.NODE_STATE_CANCELED {
				now := time.Now().Unix()
				model.CompletedAt = &now
				eventType = executionEventUpdate
			}

			if err := s.nes.UpsertNodeExecution(ctx, model); err != nil {
				s.logger.Error("failed to persist node execution", "error", err)
			}

			// If this execution has a ResponseID, wait for the response to be published first
			// This ensures frontend receives HttpResponse before NodeExecution
			if status.AuxiliaryID != nil {
				respIDStr := status.AuxiliaryID.String()
				responsePublishedMu.Lock()
				publishedChan, ok := responsePublished[respIDStr]
				responsePublishedMu.Unlock()
				if ok {
					select {
					case <-publishedChan:
						// Response published, safe to continue
					case <-ctx.Done():
						// Context cancelled, continue anyway
					}
					// Clean up map entry to prevent memory leak
					responsePublishedMu.Lock()
					delete(responsePublished, respIDStr)
					responsePublishedMu.Unlock()
				}
			}

			// Publish execution event for original node
			s.publishExecutionEvent(eventType, model, flow.ID)

			// Also create execution for the version node so history shows correct state
			if versionNodeID, ok := nodeIDMapping[status.NodeID.String()]; ok {
				versionModel := mnodeexecution.NodeExecution{
					ID:                     idwrap.NewNow(),
					NodeID:                 versionNodeID,
					Name:                   model.Name,
					State:                  model.State,
					Error:                  model.Error,
					InputData:              model.InputData,
					InputDataCompressType:  model.InputDataCompressType,
					OutputData:             model.OutputData,
					OutputDataCompressType: model.OutputDataCompressType,
					ResponseID:             model.ResponseID,
					CompletedAt:            model.CompletedAt,
				}
				if err := s.nes.UpsertNodeExecution(ctx, versionModel); err != nil {
					s.logger.Error("failed to persist version node execution", "error", err)
				}
				// Publish execution event for version node - always INSERT since these are new records
				// (the frontend needs INSERT before any UPDATE can be applied)
				s.publishExecutionEvent(executionEventInsert, versionModel, versionFlowID)

				// Also publish node state update for version node
				if s.nodeStream != nil {
					var info string
					if status.Error != nil {
						info = status.Error.Error()
					}
					versionNodePB := &flowv1.Node{
						NodeId: versionNodeID.Bytes(),
						FlowId: versionFlowID.Bytes(),
						State:  flowv1.FlowItemState(status.State),
					}
					if info != "" {
						versionNodePB.Info = &info
					}
					s.nodeStream.Publish(NodeTopic{FlowID: versionFlowID}, NodeEvent{
						Type:   nodeEventUpdate,
						FlowID: versionFlowID,
						Node:   versionNodePB,
					})
				}
			}

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

				nodePB := &flowv1.Node{
					NodeId: status.NodeID.Bytes(),
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

			if s.logStream != nil && status.State != mnnode.NODE_STATE_RUNNING {
				idStr := status.NodeID.String()
				stateStr := mnnode.StringNodeState(status.State)
				msg := fmt.Sprintf("Node %s: %s", idStr, stateStr)

				var logLevel logv1.LogLevel
				switch status.State {
				case mnnode.NODE_STATE_FAILURE:
					logLevel = logv1.LogLevel_LOG_LEVEL_ERROR
				case mnnode.NODE_STATE_CANCELED:
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

				val, err := structpb.NewValue(logData)
				if err != nil {
					s.logger.Error("failed to create log value", "error", err)
				}

				s.logStream.Publish(rlog.LogTopic{UserID: userID}, rlog.LogEvent{
					Type: rlog.EventTypeInsert,
					Log: &logv1.Log{
						LogId: idwrap.NewNow().Bytes(),
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
func (s *FlowServiceV2RPC) createFlowVersionSnapshot(
	ctx context.Context,
	sourceFlow mflow.Flow,
	sourceNodes []mnnode.MNode,
	sourceEdges []edge.Edge,
	sourceVars []mflowvariable.FlowVariable,
) (mflow.Flow, map[string]idwrap.IDWrap, error) {
	// Create the version flow record
	version, err := s.fs.CreateFlowVersion(ctx, sourceFlow)
	if err != nil {
		return mflow.Flow{}, nil, fmt.Errorf("create flow version: %w", err)
	}

	versionFlowID := version.ID

	// Create a mapping from old node IDs to new node IDs for edge remapping
	nodeIDMapping := make(map[string]idwrap.IDWrap, len(sourceNodes))

	// Events collections for bulk publishing
	nodeEvents := make([]NodeEvent, 0, len(sourceNodes))
	noOpEvents := make([]NoOpEvent, 0)
	jsEvents := make([]JsEvent, 0)
	forEvents := make([]ForEvent, 0)

	// Duplicate all nodes and their sub-node data
	for _, sourceNode := range sourceNodes {
		newNodeID := idwrap.NewNow()
		nodeIDMapping[sourceNode.ID.String()] = newNodeID

		// Create the base node
		newNode := mnnode.MNode{
			ID:        newNodeID,
			FlowID:    versionFlowID,
			Name:      sourceNode.Name,
			NodeKind:  sourceNode.NodeKind,
			PositionX: sourceNode.PositionX,
			PositionY: sourceNode.PositionY,
		}

		if err := s.ns.CreateNode(ctx, newNode); err != nil {
			return mflow.Flow{}, nil, fmt.Errorf("create node %s: %w", sourceNode.Name, err)
		}

		// Duplicate node-type specific data and collect events
		switch sourceNode.NodeKind {
		case mnnode.NODE_KIND_NO_OP:
			noopData, err := s.nnos.GetNodeNoop(ctx, sourceNode.ID)
			if err == nil {
				newNoopData := mnnoop.NoopNode{
					FlowNodeID: newNodeID,
					Type:       noopData.Type,
				}
				if err := s.nnos.CreateNodeNoop(ctx, newNoopData); err != nil {
					return mflow.Flow{}, nil, fmt.Errorf("create noop node: %w", err)
				}
				noOpEvents = append(noOpEvents, NoOpEvent{
					Type:   noopEventInsert,
					FlowID: versionFlowID,
					Node:   serializeNodeNoop(newNoopData),
				})
			}

		case mnnode.NODE_KIND_REQUEST:
			requestData, err := s.nrs.GetNodeRequest(ctx, sourceNode.ID)
			if err == nil {
				// Copy the request node config (referencing same HTTP, not duplicating)
				newRequestData := mnrequest.MNRequest{
					FlowNodeID:       newNodeID,
					HttpID:           requestData.HttpID,
					DeltaHttpID:      requestData.DeltaHttpID,
					HasRequestConfig: requestData.HasRequestConfig,
				}
				if err := s.nrs.CreateNodeRequest(ctx, newRequestData); err != nil {
					return mflow.Flow{}, nil, fmt.Errorf("create request node: %w", err)
				}
				// Request node events are handled through nodeStream subscription
			}

		case mnnode.NODE_KIND_FOR:
			forData, err := s.nfs.GetNodeFor(ctx, sourceNode.ID)
			if err == nil {
				newForData := mnfor.MNFor{
					FlowNodeID:    newNodeID,
					IterCount:     forData.IterCount,
					Condition:     forData.Condition,
					ErrorHandling: forData.ErrorHandling,
				}
				if err := s.nfs.CreateNodeFor(ctx, newForData); err != nil {
					return mflow.Flow{}, nil, fmt.Errorf("create for node: %w", err)
				}
				forEvents = append(forEvents, ForEvent{
					Type:   forEventInsert,
					FlowID: versionFlowID,
					Node:   serializeNodeFor(newForData),
				})
			}

		case mnnode.NODE_KIND_FOR_EACH:
			forEachData, err := s.nfes.GetNodeForEach(ctx, sourceNode.ID)
			if err == nil {
				newForEachData := mnforeach.MNForEach{
					FlowNodeID:     newNodeID,
					IterExpression: forEachData.IterExpression,
					Condition:      forEachData.Condition,
					ErrorHandling:  forEachData.ErrorHandling,
				}
				if err := s.nfes.CreateNodeForEach(ctx, newForEachData); err != nil {
					return mflow.Flow{}, nil, fmt.Errorf("create foreach node: %w", err)
				}
				// ForEach node events are handled through nodeStream subscription
			}

		case mnnode.NODE_KIND_CONDITION:
			conditionData, err := s.nifs.GetNodeIf(ctx, sourceNode.ID)
			if err == nil {
				newConditionData := mnif.MNIF{
					FlowNodeID: newNodeID,
					Condition:  conditionData.Condition,
				}
				if err := s.nifs.CreateNodeIf(ctx, newConditionData); err != nil {
					return mflow.Flow{}, nil, fmt.Errorf("create condition node: %w", err)
				}
				// Condition node events are handled through nodeStream subscription
			}

		case mnnode.NODE_KIND_JS:
			jsData, err := s.njss.GetNodeJS(ctx, sourceNode.ID)
			if err == nil {
				newJsData := mnjs.MNJS{
					FlowNodeID:       newNodeID,
					Code:             jsData.Code,
					CodeCompressType: jsData.CodeCompressType,
				}
				if err := s.njss.CreateNodeJS(ctx, newJsData); err != nil {
					return mflow.Flow{}, nil, fmt.Errorf("create js node: %w", err)
				}
				jsEvents = append(jsEvents, JsEvent{
					Type:   jsEventInsert,
					FlowID: versionFlowID,
					Node:   serializeNodeJs(newJsData),
				})
			}
		}

		// Collect base node event
		nodeEvents = append(nodeEvents, NodeEvent{
			Type:   nodeEventInsert,
			FlowID: versionFlowID,
			Node:   serializeNode(newNode),
		})
	}

	// Bulk publish sub-node events first
	if len(noOpEvents) > 0 && s.noopStream != nil {
		s.noopStream.Publish(NoOpTopic{FlowID: versionFlowID}, noOpEvents...)
	}
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

	// Duplicate all edges with remapped node IDs
	edgeEvents := make([]EdgeEvent, 0, len(sourceEdges))
	for _, sourceEdge := range sourceEdges {
		newSourceID, sourceOK := nodeIDMapping[sourceEdge.SourceID.String()]
		newTargetID, targetOK := nodeIDMapping[sourceEdge.TargetID.String()]

		if !sourceOK || !targetOK {
			continue
		}

		newEdge := edge.Edge{
			ID:            idwrap.NewNow(),
			FlowID:        versionFlowID,
			SourceID:      newSourceID,
			TargetID:      newTargetID,
			SourceHandler: sourceEdge.SourceHandler,
			Kind:          sourceEdge.Kind,
		}

		if err := s.es.CreateEdge(ctx, newEdge); err != nil {
			return mflow.Flow{}, nil, fmt.Errorf("create edge: %w", err)
		}
		edgeEvents = append(edgeEvents, EdgeEvent{
			Type:   edgeEventInsert,
			FlowID: versionFlowID,
			Edge:   serializeEdge(newEdge),
		})
	}

	// Bulk publish edge events
	if len(edgeEvents) > 0 && s.edgeStream != nil {
		s.edgeStream.Publish(EdgeTopic{FlowID: versionFlowID}, edgeEvents...)
	}

	// Duplicate all flow variables
	varEvents := make([]FlowVariableEvent, 0, len(sourceVars))
	for _, sourceVar := range sourceVars {
		newVar := mflowvariable.FlowVariable{
			ID:          idwrap.NewNow(),
			FlowID:      versionFlowID,
			Name:        sourceVar.Name,
			Value:       sourceVar.Value,
			Enabled:     sourceVar.Enabled,
			Description: sourceVar.Description,
			Order:       sourceVar.Order,
		}

		if err := s.fvs.CreateFlowVariable(ctx, newVar); err != nil {
			return mflow.Flow{}, nil, fmt.Errorf("create flow variable: %w", err)
		}
		varEvents = append(varEvents, FlowVariableEvent{
			Type:     flowVarEventInsert,
			FlowID:   versionFlowID,
			Variable: newVar,
		})
	}

	// Bulk publish variable events
	if len(varEvents) > 0 && s.varStream != nil {
		s.varStream.Publish(FlowVariableTopic{FlowID: versionFlowID}, varEvents...)
	}

	return version, nodeIDMapping, nil
}