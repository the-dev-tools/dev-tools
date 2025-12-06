package cmd

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/node/nfor"
	"the-dev-tools/server/pkg/flow/node/nforeach"
	"the-dev-tools/server/pkg/flow/node/nif"
	"the-dev-tools/server/pkg/flow/node/njs"
	"the-dev-tools/server/pkg/flow/node/nnoop"
	"the-dev-tools/server/pkg/flow/node/nrequest"
	"the-dev-tools/server/pkg/httpclient"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"the-dev-tools/server/pkg/model/mnnode/mnforeach"
	"the-dev-tools/server/pkg/model/mnnode/mnif"
	"the-dev-tools/server/pkg/model/mnnode/mnjs"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/spec/dist/buf/go/api/node_js_executor/v1/node_js_executorv1connect"

	"connectrpc.com/connect"
)

// buildFlowNodeMap initializes all flow nodes and returns the map and start node ID
func buildFlowNodeMap(
	ctx context.Context,
	c FlowServiceLocal,
	nodes []mnnode.MNode,
	nodeTimeout time.Duration,
	jsClient node_js_executorv1connect.NodeJsExecutorServiceClient,
) (map[idwrap.IDWrap]node.FlowNode, idwrap.IDWrap, int, error) {

	var requestNodes []mnrequest.MNRequest
	var forNodes []mnfor.MNFor
	var forEachNodes []mnforeach.MNForEach
	var ifNodes []mnif.MNIF
	var noopNodes []mnnoop.NoopNode
	var jsNodes []mnjs.MNJS
	var startNodeID idwrap.IDWrap

	nodeNameMap := make(map[idwrap.IDWrap]string, len(nodes))

	// 1. Fetch specific node data based on kind
	for _, node := range nodes {
		nodeNameMap[node.ID] = node.Name

		switch node.NodeKind {
		case mnnode.NODE_KIND_REQUEST:
			rn, err := c.rns.GetNodeRequest(ctx, node.ID)
			if err != nil {
				return nil, idwrap.IDWrap{}, 0, connect.NewError(connect.CodeInternal, fmt.Errorf("get node request: %w", err))
			}
			requestNodes = append(requestNodes, *rn)
		case mnnode.NODE_KIND_FOR:
			fn, err := c.fns.GetNodeFor(ctx, node.ID)
			if err != nil {
				return nil, idwrap.IDWrap{}, 0, connect.NewError(connect.CodeInternal, fmt.Errorf("get node for: %w", err))
			}
			forNodes = append(forNodes, *fn)
		case mnnode.NODE_KIND_FOR_EACH:
			fen, err := c.fens.GetNodeForEach(ctx, node.ID)
			if err != nil {
				return nil, idwrap.IDWrap{}, 0, connect.NewError(connect.CodeInternal, fmt.Errorf("get node for each: %w", err))
			}
			forEachNodes = append(forEachNodes, *fen)
		case mnnode.NODE_KIND_NO_OP:
			sn, err := c.sns.GetNodeNoop(ctx, node.ID)
			if err != nil {
				return nil, idwrap.IDWrap{}, 0, connect.NewError(connect.CodeInternal, fmt.Errorf("get node start: %w", err))
			}
			noopNodes = append(noopNodes, *sn)
		case mnnode.NODE_KIND_CONDITION:
			in, err := c.ins.GetNodeIf(ctx, node.ID)
			if err != nil {
				return nil, idwrap.IDWrap{}, 0, connect.NewError(connect.CodeInternal, errors.New("get node if"))
			}
			ifNodes = append(ifNodes, *in)
		case mnnode.NODE_KIND_JS:
			jsn, err := c.jsns.GetNodeJS(ctx, node.ID)
			if err != nil {
				return nil, idwrap.IDWrap{}, 0, connect.NewError(connect.CodeInternal, fmt.Errorf("get node js: %w", err))
			}
			jsNodes = append(jsNodes, jsn)
		default:
			return nil, idwrap.IDWrap{}, 0, connect.NewError(connect.CodeInternal, errors.New("not supported node"))
		}
	}

	// 2. Find start node
	var foundStartNode bool
	for _, node := range noopNodes {
		if node.Type == mnnoop.NODE_NO_OP_KIND_START {
			if foundStartNode {
				return nil, idwrap.IDWrap{}, 0, connect.NewError(connect.CodeInternal, errors.New("multiple start nodes"))
			}
			foundStartNode = true
			startNodeID = node.FlowNodeID
		}
	}
	if !foundStartNode {
		return nil, idwrap.IDWrap{}, 0, connect.NewError(connect.CodeInternal, errors.New("no start node"))
	}

	flowNodeMap := make(map[idwrap.IDWrap]node.FlowNode, 0)

	// 3. Initialize For Nodes
	for _, forNode := range forNodes {
		name := nodeNameMap[forNode.FlowNodeID]
		flowNodeMap[forNode.FlowNodeID] = nfor.New(forNode.FlowNodeID, name, forNode.IterCount, nodeTimeout, forNode.ErrorHandling)
	}

	// 4. Initialize Request Nodes
	// Calculate buffer size for request responses based on flow complexity
	requestBufferSize := len(requestNodes) * 100
	if forNodeCount := len(forNodes); forNodeCount > 0 {
		// For flows with iterations, we need larger buffers
		var maxIterations int64
		for _, fn := range forNodes {
			if fn.IterCount > maxIterations {
				maxIterations = fn.IterCount
			}
		}
		// Estimate requests per iteration
		estimatedRequests := int(maxIterations) * len(requestNodes) * 2
		if estimatedRequests > requestBufferSize {
			requestBufferSize = estimatedRequests
		}
	}
	httpClient := httpclient.New()
	requestNodeRespChan := make(chan nrequest.NodeRequestSideResp, requestBufferSize)

	// Start a goroutine to consume request responses and signal completion
	// This is necessary because nrequest nodes block waiting for Done to be closed
	// Note: This goroutine will leak if the flow runs indefinitely, but for CLI it's fine as the process exits
	// Ideally we should pass a context or have a cleanup mechanism
	go func() {
		for resp := range requestNodeRespChan {
			// Signal that we've processed the response
			if resp.Done != nil {
				close(resp.Done)
			}
		}
	}()

	for _, requestNode := range requestNodes {
		if requestNode.HttpID == nil {
			return nil, idwrap.IDWrap{}, 0, connect.NewError(connect.CodeInternal, fmt.Errorf("request node %s has no http id", requestNode.FlowNodeID))
		}

		httpRecord, err := c.hs.Get(ctx, *requestNode.HttpID)
		if err != nil {
			return nil, idwrap.IDWrap{}, 0, connect.NewError(connect.CodeInternal, fmt.Errorf("load http %s: %w", requestNode.HttpID.String(), err))
		}

		headers, err := c.hh.GetByHttpID(ctx, *requestNode.HttpID)
		if err != nil {
			return nil, idwrap.IDWrap{}, 0, connect.NewError(connect.CodeInternal, fmt.Errorf("load http headers: %w", err))
		}

		queries, err := c.hsp.GetByHttpID(ctx, *requestNode.HttpID)
		if err != nil {
			return nil, idwrap.IDWrap{}, 0, connect.NewError(connect.CodeInternal, fmt.Errorf("load http queries: %w", err))
		}

		forms, err := c.hbf.GetByHttpID(ctx, *requestNode.HttpID)
		if err != nil {
			return nil, idwrap.IDWrap{}, 0, connect.NewError(connect.CodeInternal, fmt.Errorf("load http body forms: %w", err))
		}

		urlEncoded, err := c.hbu.GetByHttpID(ctx, *requestNode.HttpID)
		if err != nil {
			return nil, idwrap.IDWrap{}, 0, connect.NewError(connect.CodeInternal, fmt.Errorf("load http body urlencoded: %w", err))
		}
		urlEncodedVals := urlEncoded

		rawBody, err := c.hbr.GetByHttpID(ctx, *requestNode.HttpID)
		if err != nil && !errors.Is(err, shttp.ErrNoHttpBodyRawFound) && !errors.Is(err, sql.ErrNoRows) {
			return nil, idwrap.IDWrap{}, 0, connect.NewError(connect.CodeInternal, fmt.Errorf("load http body raw: %w", err))
		}

		asserts, err := c.has.GetByHttpID(ctx, *requestNode.HttpID)
		if err != nil {
			return nil, idwrap.IDWrap{}, 0, connect.NewError(connect.CodeInternal, fmt.Errorf("load http asserts: %w", err))
		}

		name := nodeNameMap[requestNode.FlowNodeID]
		flowNodeMap[requestNode.FlowNodeID] = nrequest.New(
			requestNode.FlowNodeID,
			name,
			*httpRecord,
			headers,
			queries,
			rawBody,
			forms,
			urlEncodedVals,
			asserts,
			httpClient,
			requestNodeRespChan,
			c.logger,
		)
	}

	// 5. Initialize If Nodes
	for _, ifNode := range ifNodes {
		comp := ifNode.Condition
		name := nodeNameMap[ifNode.FlowNodeID]
		flowNodeMap[ifNode.FlowNodeID] = nif.New(ifNode.FlowNodeID, name, comp)
	}

	// 6. Initialize NoOp Nodes
	for _, noopNode := range noopNodes {
		name := nodeNameMap[noopNode.FlowNodeID]
		flowNodeMap[noopNode.FlowNodeID] = nnoop.New(noopNode.FlowNodeID, name)
	}

	// 7. Initialize ForEach Nodes
	for _, forEachNode := range forEachNodes {
		name := nodeNameMap[forEachNode.FlowNodeID]
		flowNodeMap[forEachNode.FlowNodeID] = nforeach.New(forEachNode.FlowNodeID, name, forEachNode.IterExpression, nodeTimeout,
			forEachNode.Condition, forEachNode.ErrorHandling)
	}

	// 8. Initialize JS Nodes
	for _, jsNode := range jsNodes {
		name := nodeNameMap[jsNode.FlowNodeID]
		flowNodeMap[jsNode.FlowNodeID] = njs.New(jsNode.FlowNodeID, name, string(jsNode.Code), jsClient)
	}

	// Calculate buffer size based on expected load
	bufferSize := 10000
	if len(forNodes) > 0 {
		var maxIterations int64
		for _, fn := range forNodes {
			if fn.IterCount > maxIterations {
				maxIterations = fn.IterCount
			}
		}
		estimatedSize := int(maxIterations) * len(flowNodeMap) * 2
		if estimatedSize > bufferSize {
			bufferSize = estimatedSize
		}
	}

	return flowNodeMap, startNodeID, bufferSize, nil
}
