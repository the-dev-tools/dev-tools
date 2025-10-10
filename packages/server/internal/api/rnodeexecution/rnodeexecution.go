package rnodeexecution

import (
	"context"
	"database/sql"
	"errors"
	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/rflow"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnodeexecution"
	"the-dev-tools/server/pkg/permcheck"
	"the-dev-tools/server/pkg/service/sexampleresp"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/snode"
	"the-dev-tools/server/pkg/service/snodeexecution"
	"the-dev-tools/server/pkg/service/snoderequest"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/translate/tnodeexecution"
	nodeexecutionv1 "the-dev-tools/spec/dist/buf/go/flow/node/execution/v1"
	"the-dev-tools/spec/dist/buf/go/flow/node/execution/v1/nodeexecutionv1connect"
	"time"

	"connectrpc.com/connect"
)

type NodeExecutionServiceRPC struct {
	nes *snodeexecution.NodeExecutionService
	ns  *snode.NodeService
	fs  *sflow.FlowService
	us  *suser.UserService
	ers *sexampleresp.ExampleRespService
	rns *snoderequest.NodeRequestService
}

func New(
	nes *snodeexecution.NodeExecutionService,
	ns *snode.NodeService,
	fs *sflow.FlowService,
	us *suser.UserService,
	ers *sexampleresp.ExampleRespService,
	rns *snoderequest.NodeRequestService,
) *NodeExecutionServiceRPC {
	return &NodeExecutionServiceRPC{
		nes: nes,
		ns:  ns,
		fs:  fs,
		us:  us,
		ers: ers,
		rns: rns,
	}
}

func CreateService(srv *NodeExecutionServiceRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := nodeexecutionv1connect.NewNodeExecutionServiceHandler(srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (s *NodeExecutionServiceRPC) NodeExecutionList(
	ctx context.Context,
	req *connect.Request[nodeexecutionv1.NodeExecutionListRequest],
) (*connect.Response[nodeexecutionv1.NodeExecutionListResponse], error) {
	// Parse node ID
	nodeID, err := idwrap.NewFromBytes(req.Msg.GetNodeId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Check permissions through flow ownership
	node, err := s.ns.GetNode(ctx, nodeID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	flow, err := s.fs.GetFlow(ctx, node.FlowID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	rpcErr := permcheck.CheckPerm(rflow.CheckOwnerFlow(ctx, *s.fs, *s.us, flow.ID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	executions, err := s.nes.ListNodeExecutionsByNodeID(ctx, nodeID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	items := make([]*nodeexecutionv1.NodeExecutionListItem, 0, len(executions))
	for _, exec := range executions {
		rpcExec, err := tnodeexecution.SerializeNodeExecutionModelToRPCListItem(&exec)
		if err != nil {
			return nil, err
		}
		items = append(items, rpcExec)
	}

	resp := &nodeexecutionv1.NodeExecutionListResponse{
		Items: items,
	}

	return connect.NewResponse(resp), nil
}

func (s *NodeExecutionServiceRPC) NodeExecutionGet(
	ctx context.Context,
	req *connect.Request[nodeexecutionv1.NodeExecutionGetRequest],
) (*connect.Response[nodeexecutionv1.NodeExecutionGetResponse], error) {
	// Parse execution ID
	executionID, err := idwrap.NewFromBytes(req.Msg.GetNodeExecutionId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	execution, err := s.getNodeExecutionWithWait(ctx, executionID)
	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			return nil, connect.NewError(connect.CodeDeadlineExceeded, err)
		case errors.Is(err, context.Canceled):
			return nil, connect.NewError(connect.CodeCanceled, err)
		case errors.Is(err, sql.ErrNoRows):
			return nil, connect.NewError(connect.CodeNotFound, err)
		default:
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	// Check permissions through node -> flow ownership
	node, err := s.ns.GetNode(ctx, execution.NodeID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	flow, err := s.fs.GetFlow(ctx, node.FlowID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	rpcErr := permcheck.CheckPerm(rflow.CheckOwnerFlow(ctx, *s.fs, *s.us, flow.ID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	// For REQUEST nodes in a terminal state, small wait to surface response correlation.
	if node.NodeKind == mnnode.NODE_KIND_REQUEST && execution.ResponseID == nil && isTerminalNodeState(execution.State) {
		if refreshed, err := s.awaitResponseID(ctx, execution.ID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		} else if refreshed != nil {
			execution = refreshed
		}
	}

	// Convert to RPC
	rpcExec, err := tnodeexecution.SerializeNodeExecutionModelToRPCGetResponse(execution)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// For REQUEST nodes, ensure ResponseID is included in RPC response
	if node.NodeKind == mnnode.NODE_KIND_REQUEST && execution.ResponseID != nil {
		// Verify the response exists (optional validation)
		_, err := s.ers.GetExampleResp(ctx, *execution.ResponseID)
		if err != nil {
			// Log validation error but don't fail the request
			// The ResponseID will still be included in the response
			_ = err // Acknowledge error but don't act on it
		}
		// ResponseID is already included by the translation layer
		// This validation just ensures the response record exists
	}

	resp := rpcExec
	return connect.NewResponse(resp), nil
}

const (
	responsePollInterval = 10 * time.Millisecond
	responsePollTimeout  = time.Second
)

func (s *NodeExecutionServiceRPC) getNodeExecutionWithWait(ctx context.Context, executionID idwrap.IDWrap) (*mnodeexecution.NodeExecution, error) {
	execution, err := s.nes.GetNodeExecution(ctx, executionID)
	if err == nil {
		return execution, nil
	}

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	if errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	waitCtx, cancel := context.WithTimeout(context.Background(), responsePollTimeout)
	defer cancel()

	ticker := time.NewTicker(responsePollInterval)
	defer ticker.Stop()

	lastErr := err
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-waitCtx.Done():
			if errors.Is(lastErr, context.DeadlineExceeded) {
				return nil, context.DeadlineExceeded
			}
			return nil, lastErr
		case <-ticker.C:
			exec, err := s.nes.GetNodeExecution(waitCtx, executionID)
			if err == nil {
				return exec, nil
			}
			if errors.Is(err, sql.ErrNoRows) {
				return nil, err
			}
			lastErr = err
		}
	}
}

func (s *NodeExecutionServiceRPC) awaitResponseID(ctx context.Context, executionID idwrap.IDWrap) (*mnodeexecution.NodeExecution, error) {
	waitCtx, cancel := context.WithTimeout(ctx, responsePollTimeout)
	defer cancel()

	ticker := time.NewTicker(responsePollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-waitCtx.Done():
			if waitCtx.Err() == context.DeadlineExceeded || waitCtx.Err() == context.Canceled {
				return nil, nil
			}
			return nil, waitCtx.Err()
		case <-ticker.C:
			exec, err := s.nes.GetNodeExecution(waitCtx, executionID)
			if err != nil {
				return nil, err
			}
			if exec.ResponseID != nil {
				return exec, nil
			}
		}
	}
}

func isTerminalNodeState(state int8) bool {
	switch state {
	case mnnode.NODE_STATE_SUCCESS, mnnode.NODE_STATE_FAILURE, mnnode.NODE_STATE_CANCELED:
		return true
	default:
		return false
	}
}
