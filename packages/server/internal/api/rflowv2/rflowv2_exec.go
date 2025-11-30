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
	"the-dev-tools/server/pkg/model/mnnode"
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

	// Mark flow as running
	flow.Running = true
	if err := s.fs.UpdateFlow(ctx, flow); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to mark flow as running: %w", err))
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
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			flowNodeMap[nodeModel.ID] = nnoop.New(nodeModel.ID, nodeModel.Name)
			if noopModel.Type == mnnoop.NODE_NO_OP_KIND_START {
				startNodeID = nodeModel.ID
			}
		case mnnode.NODE_KIND_REQUEST:
			requestCfg, err := s.nrs.GetNodeRequest(ctx, nodeModel.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			if requestCfg == nil || requestCfg.HttpID == nil || isZeroID(*requestCfg.HttpID) {
				return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("request node %s missing http configuration", nodeModel.ID.String()))
			}
			requestNode, err := s.buildRequestFlowNode(ctx, flow, nodeModel, *requestCfg, sharedHTTPClient, requestRespChan)
			if err != nil {
				return nil, err
			}
			flowNodeMap[nodeModel.ID] = requestNode
		case mnnode.NODE_KIND_FOR:
			forCfg, err := s.nfs.GetNodeFor(ctx, nodeModel.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			if forCfg.Condition.Comparisons.Expression != "" {
				flowNodeMap[nodeModel.ID] = nfor.NewWithCondition(nodeModel.ID, nodeModel.Name, forCfg.IterCount, timeoutDuration, forCfg.ErrorHandling, forCfg.Condition)
			} else {
				flowNodeMap[nodeModel.ID] = nfor.New(nodeModel.ID, nodeModel.Name, forCfg.IterCount, timeoutDuration, forCfg.ErrorHandling)
			}
		case mnnode.NODE_KIND_FOR_EACH:
			forEachCfg, err := s.nfes.GetNodeForEach(ctx, nodeModel.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			flowNodeMap[nodeModel.ID] = nforeach.New(nodeModel.ID, nodeModel.Name, forEachCfg.IterExpression, timeoutDuration, forEachCfg.Condition, forEachCfg.ErrorHandling)
		case mnnode.NODE_KIND_CONDITION:
			condCfg, err := s.nifs.GetNodeIf(ctx, nodeModel.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			flowNodeMap[nodeModel.ID] = nif.New(nodeModel.ID, nodeModel.Name, condCfg.Condition)
		case mnnode.NODE_KIND_JS:
			jsCfg, err := s.njss.GetNodeJS(ctx, nodeModel.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			codeBytes := jsCfg.Code
			if jsCfg.CodeCompressType != compress.CompressTypeNone {
				codeBytes, err = compress.Decompress(jsCfg.Code, jsCfg.CodeCompressType)
				if err != nil {
					return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("decompress js code: %w", err))
				}
			}
			flowNodeMap[nodeModel.ID] = njs.New(nodeModel.ID, nodeModel.Name, string(codeBytes), nil)
		default:
			return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("node kind %d not supported in FlowRun", nodeModel.NodeKind))
		}
	}

	if startNodeID == (idwrap.IDWrap{}) {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("flow missing start node"))
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
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	stateDrain.Wait()
	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) FlowStop(ctx context.Context, req *connect.Request[flowv1.FlowStopRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
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
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("request node %s missing http_id", nodeModel.Name))
	}

	resolved, err := s.resolver.Resolve(ctx, *cfg.HttpID, cfg.DeltaHttpID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("resolve http %s: %w", cfg.HttpID.String(), err))
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
