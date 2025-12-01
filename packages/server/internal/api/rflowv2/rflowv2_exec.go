package rflowv2

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"connectrpc.com/connect"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/node/nfor"
	"the-dev-tools/server/pkg/flow/node/nforeach"
	"the-dev-tools/server/pkg/flow/node/nif"
	"the-dev-tools/server/pkg/flow/node/njs"
	"the-dev-tools/server/pkg/flow/node/nnoop"
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
	version, err := s.createFlowVersionSnapshot(ctx, flow, nodes, edges, flowVars)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create flow version: %w", err))
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

		if err := s.executeFlow(bgCtx, flow, nodes, edges, flowVars); err != nil {
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
) error {
	// Mark flow as running
	flow.Running = true
	if err := s.fs.UpdateFlow(ctx, flow); err != nil {
		return fmt.Errorf("failed to mark flow as running: %w", err)
	}
	s.publishFlowEvent(flowEventUpdate, flow)

	defer func() {
		// Mark flow as not running when done
		flow.Running = false
		if err := s.fs.UpdateFlow(context.Background(), flow); err != nil {
			s.logger.Error("failed to mark flow as not running", "error", err)
		}
		s.publishFlowEvent(flowEventUpdate, flow)
	}()

	baseVars := make(map[string]any, len(flowVars))
	for _, variable := range flowVars {
		if variable.Enabled {
			baseVars[variable.Name] = variable.Value
		}
	}

	requestRespChan := make(chan nrequest.NodeRequestSideResp, len(nodes)*2+1)
	var respDrain sync.WaitGroup
	respDrain.Add(1)
	go func() {
		defer respDrain.Done()
		for resp := range requestRespChan {
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
	flowNodeMap := make(map[idwrap.IDWrap]node.FlowNode, len(nodes))

	var startNodeID idwrap.IDWrap
	const defaultNodeTimeout = 60 // seconds
	timeoutDuration := time.Duration(defaultNodeTimeout) * time.Second

	for _, nodeModel := range nodes {
		switch nodeModel.NodeKind {
		case mnnode.NODE_KIND_NO_OP:
			noopModel, err := s.nnos.GetNodeNoop(ctx, nodeModel.ID)
			if err != nil {
				return err
			}
			flowNodeMap[nodeModel.ID] = nnoop.New(nodeModel.ID, nodeModel.Name)
			if noopModel.Type == mnnoop.NODE_NO_OP_KIND_START {
				startNodeID = nodeModel.ID
			}
		case mnnode.NODE_KIND_REQUEST:
			requestCfg, err := s.nrs.GetNodeRequest(ctx, nodeModel.ID)
			if err != nil {
				return err
			}
			if requestCfg == nil || requestCfg.HttpID == nil || isZeroID(*requestCfg.HttpID) {
				return fmt.Errorf("request node %s missing http configuration", nodeModel.ID.String())
			}
			requestNode, err := s.buildRequestFlowNode(ctx, flow, nodeModel, *requestCfg, sharedHTTPClient, requestRespChan)
			if err != nil {
				return err
			}
			flowNodeMap[nodeModel.ID] = requestNode
		case mnnode.NODE_KIND_FOR:
			forCfg, err := s.nfs.GetNodeFor(ctx, nodeModel.ID)
			if err != nil {
				return err
			}
			if forCfg.Condition.Comparisons.Expression != "" {
				flowNodeMap[nodeModel.ID] = nfor.NewWithCondition(nodeModel.ID, nodeModel.Name, forCfg.IterCount, timeoutDuration, forCfg.ErrorHandling, forCfg.Condition)
			} else {
				flowNodeMap[nodeModel.ID] = nfor.New(nodeModel.ID, nodeModel.Name, forCfg.IterCount, timeoutDuration, forCfg.ErrorHandling)
			}
		case mnnode.NODE_KIND_FOR_EACH:
			forEachCfg, err := s.nfes.GetNodeForEach(ctx, nodeModel.ID)
			if err != nil {
				return err
			}
			flowNodeMap[nodeModel.ID] = nforeach.New(nodeModel.ID, nodeModel.Name, forEachCfg.IterExpression, timeoutDuration, forEachCfg.Condition, forEachCfg.ErrorHandling)
		case mnnode.NODE_KIND_CONDITION:
			condCfg, err := s.nifs.GetNodeIf(ctx, nodeModel.ID)
			if err != nil {
				return err
			}
			flowNodeMap[nodeModel.ID] = nif.New(nodeModel.ID, nodeModel.Name, condCfg.Condition)
		case mnnode.NODE_KIND_JS:
			jsCfg, err := s.njss.GetNodeJS(ctx, nodeModel.ID)
			if err != nil {
				return err
			}
			codeBytes := jsCfg.Code
			if jsCfg.CodeCompressType != compress.CompressTypeNone {
				codeBytes, err = compress.Decompress(jsCfg.Code, jsCfg.CodeCompressType)
				if err != nil {
					return fmt.Errorf("decompress js code: %w", err)
				}
			}
			flowNodeMap[nodeModel.ID] = njs.New(nodeModel.ID, nodeModel.Name, string(codeBytes), nil)
		default:
			return fmt.Errorf("node kind %d not supported in FlowRun", nodeModel.NodeKind)
		}
	}

	if startNodeID == (idwrap.IDWrap{}) {
		return errors.New("flow missing start node")
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
				State:      int8(status.State),
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

			// Publish execution event
			s.publishExecutionEvent(eventType, model, flow.ID)

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
		}
	}()

	if err := flowRunner.RunWithEvents(ctx, runner.FlowEventChannels{
		NodeStates: nodeStateChan,
	}, baseVars); err != nil {
		stateDrain.Wait()
		return err
	}

	stateDrain.Wait()
	return nil
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

func (s *FlowServiceV2RPC) buildRequestFlowNode(
	ctx context.Context,
	flow mflow.Flow,
	nodeModel mnnode.MNode,
	cfg mnrequest.MNRequest,
	client httpclient.HttpClient,
	respChan chan nrequest.NodeRequestSideResp,
) (*nrequest.NodeRequest, error) {
	if cfg.HttpID == nil {
		return nil, fmt.Errorf("request node %s missing http_id", nodeModel.Name)
	}

	resolved, err := s.resolver.Resolve(ctx, *cfg.HttpID, cfg.DeltaHttpID)
	if err != nil {
		return nil, fmt.Errorf("resolve http %s: %w", cfg.HttpID.String(), err)
	}

	requestNode := nrequest.New(
		nodeModel.ID,
		nodeModel.Name,
		resolved.Resolved,
		resolved.ResolvedHeaders,
		resolved.ResolvedQueries,
		&resolved.ResolvedRawBody,
		resolved.ResolvedFormBody,
		resolved.ResolvedUrlEncodedBody,
		resolved.ResolvedAsserts,
		client,
		respChan,
		s.logger,
	)
	return requestNode, nil
}

// createFlowVersionSnapshot creates a complete snapshot of the flow including all nodes, edges, sub-nodes, and variables.
// It also publishes sync events for all created entities so clients receive the full flow data.
func (s *FlowServiceV2RPC) createFlowVersionSnapshot(
	ctx context.Context,
	sourceFlow mflow.Flow,
	sourceNodes []mnnode.MNode,
	sourceEdges []edge.Edge,
	sourceVars []mflowvariable.FlowVariable,
) (mflow.Flow, error) {
	// Create the version flow record
	version, err := s.fs.CreateFlowVersion(ctx, sourceFlow)
	if err != nil {
		return mflow.Flow{}, fmt.Errorf("create flow version: %w", err)
	}

	versionFlowID := version.ID

	// Create a mapping from old node IDs to new node IDs for edge remapping
	nodeIDMapping := make(map[string]idwrap.IDWrap, len(sourceNodes))

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
			return mflow.Flow{}, fmt.Errorf("create node %s: %w", sourceNode.Name, err)
		}

		// Duplicate node-type specific data and publish events
		// Sub-node events must be published before base node events
		switch sourceNode.NodeKind {
		case mnnode.NODE_KIND_NO_OP:
			noopData, err := s.nnos.GetNodeNoop(ctx, sourceNode.ID)
			if err == nil {
				newNoopData := mnnoop.NoopNode{
					FlowNodeID: newNodeID,
					Type:       noopData.Type,
				}
				if err := s.nnos.CreateNodeNoop(ctx, newNoopData); err != nil {
					return mflow.Flow{}, fmt.Errorf("create noop node: %w", err)
				}
				s.publishNoOpEvent(noopEventInsert, versionFlowID, newNoopData)
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
					return mflow.Flow{}, fmt.Errorf("create request node: %w", err)
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
					return mflow.Flow{}, fmt.Errorf("create for node: %w", err)
				}
				s.publishForEvent(forEventInsert, versionFlowID, newForData)
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
					return mflow.Flow{}, fmt.Errorf("create foreach node: %w", err)
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
					return mflow.Flow{}, fmt.Errorf("create condition node: %w", err)
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
					return mflow.Flow{}, fmt.Errorf("create js node: %w", err)
				}
				s.publishJsEvent(jsEventInsert, versionFlowID, newJsData)
			}
		}

		// Publish the base node event after sub-node event
		s.publishNodeEvent(nodeEventInsert, newNode)
	}

	// Duplicate all edges with remapped node IDs
	for _, sourceEdge := range sourceEdges {
		newSourceID, sourceOK := nodeIDMapping[sourceEdge.SourceID.String()]
		newTargetID, targetOK := nodeIDMapping[sourceEdge.TargetID.String()]

		if !sourceOK || !targetOK {
			// Skip invalid edges
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
			return mflow.Flow{}, fmt.Errorf("create edge: %w", err)
		}
		s.publishEdgeEvent(edgeEventInsert, newEdge)
	}

	// Duplicate all flow variables
	for _, sourceVar := range sourceVars {
		newVar := mflowvariable.FlowVariable{
			ID:          idwrap.NewNow(),
			FlowID:      versionFlowID,
			Name:        sourceVar.Name,
			Value:       sourceVar.Value,
			Enabled:     sourceVar.Enabled,
			Description: sourceVar.Description,
		}

		if err := s.fvs.CreateFlowVariable(ctx, newVar); err != nil {
			return mflow.Flow{}, fmt.Errorf("create flow variable: %w", err)
		}
		s.publishFlowVariableEvent(flowVarEventInsert, newVar, 0)
	}

	return version, nil
}