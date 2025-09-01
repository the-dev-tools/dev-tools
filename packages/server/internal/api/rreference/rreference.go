package rreference

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/rworkspace"
	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/httpclient"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/model/mexampleresp"
	"the-dev-tools/server/pkg/model/mflowvariable"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnodeexecution"
	"the-dev-tools/server/pkg/model/mvar"
	"the-dev-tools/server/pkg/permcheck"
	"the-dev-tools/server/pkg/reference"
	"the-dev-tools/server/pkg/referencecompletion"
	"the-dev-tools/server/pkg/service/flow/sedge"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/sexampleresp"
	"the-dev-tools/server/pkg/service/sexamplerespheader"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sflowvariable"
	"the-dev-tools/server/pkg/service/snode"
	"the-dev-tools/server/pkg/service/snodeexecution"
	"the-dev-tools/server/pkg/service/snoderequest"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/svar"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/sort/sortenabled"
	"the-dev-tools/server/pkg/zstdcompress"
	referencev1 "the-dev-tools/spec/dist/buf/go/reference/v1"
	"the-dev-tools/spec/dist/buf/go/reference/v1/referencev1connect"

	"connectrpc.com/connect"
)

type ReferenceServiceRPC struct {
	DB *sql.DB

	us suser.UserService
	ws sworkspace.WorkspaceService

	// env
	es senv.EnvService
	vs svar.VarService

	// resp
	ers  sexampleresp.ExampleRespService
	erhs sexamplerespheader.ExampleRespHeaderService

	// flow
	fs                   sflow.FlowService
	fns                  snode.NodeService
	frns                 snoderequest.NodeRequestService
	flowVariableService  sflowvariable.FlowVariableService
	flowEdgeService      sedge.EdgeService
	nodeExecutionService snodeexecution.NodeExecutionService
}

func NewNodeServiceRPC(db *sql.DB, us suser.UserService, ws sworkspace.WorkspaceService,
	es senv.EnvService, vs svar.VarService,
	ers sexampleresp.ExampleRespService, erhs sexamplerespheader.ExampleRespHeaderService,
	fs sflow.FlowService, fns snode.NodeService, frns snoderequest.NodeRequestService,
	flowVariableService sflowvariable.FlowVariableService,
	edgeService sedge.EdgeService,
	nodeExecutionService snodeexecution.NodeExecutionService,
) *ReferenceServiceRPC {
	return &ReferenceServiceRPC{
		DB: db,

		us: us,
		ws: ws,

		es: es,
		vs: vs,

		ers:  ers,
		erhs: erhs,

		fs:                  fs,
		fns:                 fns,
		frns:                frns,
		flowVariableService: flowVariableService,

		flowEdgeService:      edgeService,
		nodeExecutionService: nodeExecutionService,
	}
}

func CreateService(srv *ReferenceServiceRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := referencev1connect.NewReferenceServiceHandler(srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

var (
	ErrExampleNotFound   = errors.New("example not found")
	ErrNodeNotFound      = errors.New("node not found")
	ErrWorkspaceNotFound = errors.New("workspace not found")
	ErrEnvNotFound       = errors.New("env not found")
)

// isIterationExecution checks if an execution name indicates it's an iteration execution
// from a foreach/for node rather than the main node execution.
// Iteration executions have names like "Iteration 0", "Iteration 1", "Error Summary", etc.
func isIterationExecution(name string) bool {
	return strings.HasPrefix(name, "Iteration ") || name == "Error Summary"
}

func (c *ReferenceServiceRPC) ReferenceTree(ctx context.Context, req *connect.Request[referencev1.ReferenceTreeRequest]) (*connect.Response[referencev1.ReferenceTreeResponse], error) {
	var Items []*referencev1.ReferenceTreeItem

	var workspaceID, exampleID, nodeIDPtr *idwrap.IDWrap
	msg := req.Msg
	if msg.WorkspaceId != nil {
		tempID, err := idwrap.NewFromBytes(msg.WorkspaceId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		workspaceID = &tempID
	}
	if msg.ExampleId != nil {
		tempID, err := idwrap.NewFromBytes(msg.ExampleId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		exampleID = &tempID
	}
	if msg.NodeId != nil {
		tempID, err := idwrap.NewFromBytes(msg.NodeId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		nodeIDPtr = &tempID
	}

	// Workspace
	if workspaceID != nil {
		wsID := *workspaceID
		rpcErr := permcheck.CheckPerm(rworkspace.CheckOwnerWorkspace(ctx, c.us, wsID))
		if rpcErr != nil {
			return nil, rpcErr
		}
		envs, err := c.es.GetByWorkspace(ctx, wsID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, ErrWorkspaceNotFound)
		}

		present := make(map[string][]menv.Env)
		envMap := make([]*referencev1.ReferenceTreeItem, 0, len(envs))
		var allVars []mvar.Var

		for _, env := range envs {
			vars, err := c.vs.GetVariableByEnvID(ctx, env.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, ErrEnvNotFound)
			}
			// Filter to only include enabled variables
			sortenabled.GetAllWithState(&vars, true)
			for _, v := range vars {
				foundEnvs := present[v.VarKey]
				foundEnvs = append(foundEnvs, env)
				present[v.VarKey] = foundEnvs
			}
			allVars = append(allVars, vars...)
		}

		for _, v := range allVars {
			foundEnvs := present[v.VarKey]
			var containsEnv []string
			for _, env := range foundEnvs {
				containsEnv = append(containsEnv, env.Name)
			}

			envRef := &referencev1.ReferenceTreeItem{
				Key: &referencev1.ReferenceKey{
					Kind: referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY,
					Key:  &v.VarKey,
				},
				Kind:     referencev1.ReferenceKind_REFERENCE_KIND_VARIABLE,
				Variable: containsEnv,
			}
			envMap = append(envMap, envRef)
		}

		groupStr := "env"
		Items = append(Items, &referencev1.ReferenceTreeItem{
			Key: &referencev1.ReferenceKey{
				Kind:  referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_GROUP,
				Group: &groupStr,
			},
			Kind: referencev1.ReferenceKind_REFERENCE_KIND_MAP,
			Map:  envMap,
		})
	}

	// Example
	if exampleID != nil {
		exID := *exampleID

		respRef, err := GetExampleRespByExampleID(ctx, c.ers, c.erhs, exID)
		if err != nil {
			if err != sql.ErrNoRows {
				return nil, err
			}
		} else {
			Items = append(Items, reference.ConvertPkgToRpcTree(*respRef))
		}

	}

	// Node
	if nodeIDPtr != nil {
		refs, err := c.HandleNode(ctx, *nodeIDPtr)
		if err != nil {
			return nil, err
		}
		Items = append(Items, refs...)
	}

	response := &referencev1.ReferenceTreeResponse{
		Items: Items,
	}
	return connect.NewResponse(response), nil
}

func (c *ReferenceServiceRPC) HandleNode(ctx context.Context, nodeID idwrap.IDWrap) ([]*referencev1.ReferenceTreeItem, error) {
	nodeInst, err := c.fns.GetNode(ctx, nodeID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	flowID := nodeInst.FlowID
	nodes, err := c.fns.GetNodesByFlowID(ctx, flowID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var nodeRefs []*referencev1.ReferenceTreeItem
	flowVars, err := c.flowVariableService.GetFlowVariablesByFlowID(ctx, flowID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	sortenabled.GetAllWithState(&flowVars, true)
	for _, flowVar := range flowVars {
		flowVarRef := reference.NewReferenceFromInterfaceWithKey(flowVar.Value, flowVar.Name)
		nodeRefs = append(nodeRefs, reference.ConvertPkgToRpcTree(flowVarRef))
	}

	// Edges
	edges, err := c.flowEdgeService.GetEdgesByFlowID(ctx, flowID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	edgesMap := edge.NewEdgesMap(edges)

	beforeNodes := make([]mnnode.MNode, 0, len(nodes))
	for _, node := range nodes {
		if edge.IsNodeCheckTarget(edgesMap, node.ID, nodeID) == edge.NodeBefore {
			beforeNodes = append(beforeNodes, node)
		}
	}

	for _, node := range beforeNodes {
		// First, try to get execution data for ANY node type
		var nodeData interface{}
		hasExecutionData := false

		executions, err := c.nodeExecutionService.GetNodeExecutionsByNodeID(ctx, node.ID)
		if err == nil && len(executions) > 0 {
			// Filter out iteration executions from foreach/for nodes
			// These have names like "Iteration 0", "Iteration 1", "Error Summary" etc.
			// We want the main node execution which contains the actual written variables
			var latestExecution *mnodeexecution.NodeExecution
			for _, exec := range executions {
				if !isIterationExecution(exec.Name) {
					latestExecution = &exec
					break
				}
			}

			// If we found a non-iteration execution, use it
			if latestExecution != nil {
				// Decompress data if needed
				data := latestExecution.OutputData
				if latestExecution.OutputDataCompressType != compress.CompressTypeNone {
					decompressed, err := compress.Decompress(data, latestExecution.OutputDataCompressType)
					if err == nil {
						data = decompressed
					}
				}

				// Try to unmarshal as generic JSON
				var genericOutput interface{}
				if err := json.Unmarshal(data, &genericOutput); err == nil {
					nodeData = genericOutput
					hasExecutionData = true
				}
			}
		}

		// If we have execution data, use it
		if hasExecutionData && nodeData != nil {
			// The execution data contains the full tree structure from tracker.GetWrittenVarsAsTree()
			// which already includes node names as top-level keys
			// We need to extract just the data for this specific node
			if nodeMap, ok := nodeData.(map[string]interface{}); ok {
				// Check if the data contains this node's name as a key
				if nodeSpecificData, hasNodeKey := nodeMap[node.Name]; hasNodeKey {
					// Use the node-specific data
					nodeVarRef := reference.NewReferenceFromInterfaceWithKey(nodeSpecificData, node.Name)
					nodeRefs = append(nodeRefs, reference.ConvertPkgToRpcTree(nodeVarRef))
				} else {
					// Data doesn't have the expected structure, use it as-is
					nodeVarRef := reference.NewReferenceFromInterfaceWithKey(nodeData, node.Name)
					nodeRefs = append(nodeRefs, reference.ConvertPkgToRpcTree(nodeVarRef))
				}
			} else {
				// Not a map, use directly
				nodeVarRef := reference.NewReferenceFromInterfaceWithKey(nodeData, node.Name)
				nodeRefs = append(nodeRefs, reference.ConvertPkgToRpcTree(nodeVarRef))
			}
			continue
		}

		// Otherwise, provide schema for specific node types
		switch node.NodeKind {
		case mnnode.NODE_KIND_FOR_EACH:
			// For foreach loops, they write 'item' and 'key' variables
			nodeVarsMap := map[string]interface{}{
				"item": nil, // Can be any type from the iterated collection
				"key":  0,   // Index for arrays, string key for maps
			}
			nodeVarRef := reference.NewReferenceFromInterfaceWithKey(nodeVarsMap, node.Name)
			nodeRefs = append(nodeRefs, reference.ConvertPkgToRpcTree(nodeVarRef))

		case mnnode.NODE_KIND_FOR:
			// For for loops, they write 'index' variable
			nodeVarsMap := map[string]interface{}{
				"index": 0,
			}
			nodeVarRef := reference.NewReferenceFromInterfaceWithKey(nodeVarsMap, node.Name)
			nodeRefs = append(nodeRefs, reference.ConvertPkgToRpcTree(nodeVarRef))

		case mnnode.NODE_KIND_REQUEST:
			// For REQUEST nodes, provide the schema structure
			nodeVarsMap := map[string]interface{}{
				"request": map[string]interface{}{
					"headers": map[string]string{},
					"queries": map[string]string{},
					"body":    "string",
				},
				"response": map[string]interface{}{
					"status":   200,
					"body":     map[string]interface{}{},
					"headers":  map[string]string{},
					"duration": 0,
				},
			}
			nodeVarRef := reference.NewReferenceFromInterfaceWithKey(nodeVarsMap, node.Name)
			nodeRefs = append(nodeRefs, reference.ConvertPkgToRpcTree(nodeVarRef))
		}
		// Other node types (JS, CONDITION, etc.) don't have default schemas
	}

	return nodeRefs, nil
}

func GetExampleRespByExampleID(ctx context.Context, ers sexampleresp.ExampleRespService, erhs sexamplerespheader.ExampleRespHeaderService, exID idwrap.IDWrap) (*reference.ReferenceTreeItem, error) {
	resp, err := ers.GetExampleRespByExampleIDLatest(ctx, exID)
	if err != nil {
		return nil, err
	}

	respHeaders, err := erhs.GetHeaderByRespID(ctx, resp.ID)
	if err != nil {
		return nil, err
	}

	headerMap := make(map[string]string)
	for _, header := range respHeaders {
		headerVal, ok := headerMap[header.HeaderKey]
		if ok {
			headerMap[header.HeaderKey] = headerVal + ", " + header.Value
		} else {
			headerMap[header.HeaderKey] = header.Value
		}
	}

	if resp.BodyCompressType != mexampleresp.BodyCompressTypeNone {
		if resp.BodyCompressType == mexampleresp.BodyCompressTypeZstd {
			data, err := zstdcompress.Decompress(resp.Body)
			if err != nil {
				return nil, err
			}
			resp.Body = data
		}
	}

	// check if body seems like json; if so decode it into a map[string]interface{}, otherwise use a string.
	var body any
	if json.Valid(resp.Body) {
		var jsonBody map[string]any
		// If unmarshaling works, use the decoded JSON.
		if err := json.Unmarshal(resp.Body, &jsonBody); err == nil {
			body = jsonBody
		} else {
			body = string(resp.Body)
		}
	} else {
		body = string(resp.Body)
	}

	// check if body seems like json

	httpResp := httpclient.ResponseVar{
		StatusCode: int(resp.Status),
		Body:       body,
		Headers:    headerMap,
		Duration:   resp.Duration,
	}

	var m map[string]interface{}
	data, err := json.Marshal(httpResp)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(data, &m)
	if err != nil {
		return nil, err
	}

	localRef, err := reference.ConvertMapToReference(m, "response")
	if err != nil {
		return nil, err
	}
	return &localRef, nil
}

// ReferenceCompletion calls reference.v1.ReferenceService.ReferenceCompletion.
func (c *ReferenceServiceRPC) ReferenceCompletion(ctx context.Context, req *connect.Request[referencev1.ReferenceCompletionRequest]) (*connect.Response[referencev1.ReferenceCompletionResponse], error) {

	var workspaceID, exampleID, nodeIDPtr *idwrap.IDWrap
	msg := req.Msg
	if msg.WorkspaceId != nil {
		tempID, err := idwrap.NewFromBytes(msg.WorkspaceId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		workspaceID = &tempID
	}
	if msg.ExampleId != nil {
		tempID, err := idwrap.NewFromBytes(msg.ExampleId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		exampleID = &tempID
	}
	if msg.NodeId != nil {
		tempID, err := idwrap.NewFromBytes(msg.NodeId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		nodeIDPtr = &tempID
	}

	creator := referencecompletion.NewReferenceCompletionCreator()

	// Workspace
	if workspaceID != nil {
		wsID := *workspaceID
		rpcErr := permcheck.CheckPerm(rworkspace.CheckOwnerWorkspace(ctx, c.us, wsID))
		if rpcErr != nil {
			return nil, rpcErr
		}
		envs, err := c.es.GetByWorkspace(ctx, wsID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, ErrWorkspaceNotFound)
		}

		for _, env := range envs {
			vars, err := c.vs.GetVariableByEnvID(ctx, env.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, ErrEnvNotFound)
			}

			// Filter to only include enabled variables
			sortenabled.GetAllWithState(&vars, true)
			for _, v := range vars {
				creator.AddWithKey(v.VarKey, v.Value)
			}
		}
	}

	if exampleID != nil {
		exID := *exampleID
		resp, err := c.ers.GetExampleRespByExampleIDLatest(ctx, exID)
		if err != nil {
			if err == sexampleresp.ErrNoRespFound {
			} else {
				return nil, err
			}
		}

		if resp != nil {
			respHeaders, err := c.erhs.GetHeaderByRespID(ctx, resp.ID)
			if err != nil {
				return nil, err
			}

			headerMap := make(map[string]string)
			for _, header := range respHeaders {
				headerVal, ok := headerMap[header.HeaderKey]
				if ok {
					headerMap[header.HeaderKey] = headerVal + ", " + header.Value
				} else {
					headerMap[header.HeaderKey] = header.Value
				}
			}

			if resp.BodyCompressType != mexampleresp.BodyCompressTypeNone {
				if resp.BodyCompressType == mexampleresp.BodyCompressTypeZstd {
					data, err := zstdcompress.Decompress(resp.Body)
					if err != nil {
						return nil, err
					}
					resp.Body = data
				}
			}

			// check if body seems like json; if so decode it into a map[string]interface{}, otherwise use a string.
			var body any
			if json.Valid(resp.Body) {
				var jsonBody map[string]any
				// If unmarshaling works, use the decoded JSON.
				if err := json.Unmarshal(resp.Body, &jsonBody); err == nil {
					body = jsonBody
				} else {
					body = string(resp.Body)
				}
			} else {
				body = string(resp.Body)
			}

			// check if body seems like json

			httpResp := httpclient.ResponseVar{
				StatusCode: int(resp.Status),
				Body:       body,
				Headers:    headerMap,
				Duration:   resp.Duration,
			}

			var m map[string]any
			data, err := json.Marshal(httpResp)
			if err != nil {
				return nil, err
			}
			err = json.Unmarshal(data, &m)
			if err != nil {
				return nil, err
			}

			creator.AddWithKey("response", m)
		}

	}

	if nodeIDPtr != nil {
		nodeID := *nodeIDPtr
		nodeInst, err := c.fns.GetNode(ctx, nodeID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		flowID := nodeInst.FlowID
		nodes, err := c.fns.GetNodesByFlowID(ctx, flowID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		flowVars, err := c.flowVariableService.GetFlowVariablesByFlowID(ctx, flowID)
		if err != nil {
			if err != sflowvariable.ErrNoFlowVariableFound {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			flowVars = []mflowvariable.FlowVariable{}
		}

		sortenabled.GetAllWithState(&flowVars, true)
		for _, flowVar := range flowVars {
			creator.AddWithKey(flowVar.Name, flowVar.Value)
		}

		// Edges
		edges, err := c.flowEdgeService.GetEdgesByFlowID(ctx, flowID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		edgesMap := edge.NewEdgesMap(edges)

		beforeNodes := make([]mnnode.MNode, 0, len(nodes))
		for _, node := range nodes {
			if edge.IsNodeCheckTarget(edgesMap, node.ID, nodeID) == edge.NodeBefore {
				beforeNodes = append(beforeNodes, node)
			}
		}

		for _, node := range beforeNodes {
			// First, try to get execution data for ANY node type
			var nodeData interface{}
			hasExecutionData := false

			executions, err := c.nodeExecutionService.GetNodeExecutionsByNodeID(ctx, node.ID)
			if err == nil && len(executions) > 0 {
				// Use the latest execution (first one, as they're ordered by ID DESC)
				latestExecution := executions[0]

				// Decompress data if needed
				data := latestExecution.OutputData
				if latestExecution.OutputDataCompressType != compress.CompressTypeNone {
					decompressed, err := compress.Decompress(data, latestExecution.OutputDataCompressType)
					if err == nil {
						data = decompressed
					}
				}

				// Try to unmarshal as generic JSON
				var genericOutput interface{}
				if err := json.Unmarshal(data, &genericOutput); err == nil {
					nodeData = genericOutput
					hasExecutionData = true
				}
			}

			// If we have execution data, use it
			if hasExecutionData && nodeData != nil {
				// The execution data contains the full tree structure from tracker.GetWrittenVarsAsTree()
				// which already includes node names as top-level keys
				// We need to extract just the data for this specific node
				if nodeMap, ok := nodeData.(map[string]interface{}); ok {
					// Check if the data contains this node's name as a key
					if nodeSpecificData, hasNodeKey := nodeMap[node.Name]; hasNodeKey {
						// Use the node-specific data
						creator.AddWithKey(node.Name, nodeSpecificData)
					} else {
						// Data doesn't have the expected structure, use it as-is
						creator.AddWithKey(node.Name, nodeData)
					}
				} else {
					// Not a map, use directly
					creator.AddWithKey(node.Name, nodeData)
				}
				continue
			}

			// Otherwise, provide schema for specific node types
			switch node.NodeKind {
			case mnnode.NODE_KIND_FOR_EACH:
				// For foreach loops, they write 'item' and 'key' variables
				nodeVarsMap := map[string]interface{}{
					"item": nil, // Can be any type from the iterated collection
					"key":  0,   // Index for arrays, string key for maps
				}
				creator.AddWithKey(node.Name, nodeVarsMap)

			case mnnode.NODE_KIND_FOR:
				// For for loops, they write 'index' variable
				nodeVarsMap := map[string]interface{}{
					"index": 0,
				}
				creator.AddWithKey(node.Name, nodeVarsMap)

			case mnnode.NODE_KIND_REQUEST:
				// For REQUEST nodes, provide the schema structure
				nodeVarsMap := map[string]interface{}{
					"request": map[string]interface{}{
						"headers": map[string]string{},
						"queries": map[string]string{},
						"body":    "string",
					},
					"response": map[string]interface{}{
						"status":   200,
						"body":     map[string]interface{}{},
						"headers":  map[string]string{},
						"duration": 0,
					},
				}
				creator.AddWithKey(node.Name, nodeVarsMap)
			}
			// Other node types (JS, CONDITION, etc.) don't have default schemas
		}

		// Add self-reference for FOR, FOREACH, and REQUEST nodes so they can reference their own variables
		// This enables break conditions like "if foreach_8.index > 8" and request nodes to use "response.status"
		if nodeIDPtr != nil {
			currentNode, err := c.fns.GetNode(ctx, *nodeIDPtr)
			if err == nil {
				switch currentNode.NodeKind {
				case mnnode.NODE_KIND_FOR:
					// FOR nodes can reference their own index
					nodeVarsMap := map[string]interface{}{
						"index": 0,
					}
					creator.AddWithKey(currentNode.Name, nodeVarsMap)

				case mnnode.NODE_KIND_FOR_EACH:
					// FOREACH nodes can reference their own item and key
					nodeVarsMap := map[string]interface{}{
						"item": nil,
						"key":  0,
					}
					creator.AddWithKey(currentNode.Name, nodeVarsMap)

				case mnnode.NODE_KIND_REQUEST:
					// REQUEST nodes can reference their own response and request directly (without prefix)
					var nodeData interface{}
					hasExecutionData := false

					// Try to get the current node's execution data
					executions, err := c.nodeExecutionService.GetNodeExecutionsByNodeID(ctx, currentNode.ID)
					if err == nil && len(executions) > 0 {
						// Filter out iteration executions
						var latestExecution *mnodeexecution.NodeExecution
						for _, exec := range executions {
							if !isIterationExecution(exec.Name) {
								latestExecution = &exec
								break
							}
						}

						if latestExecution != nil {
							// Decompress data if needed
							data := latestExecution.OutputData
							if latestExecution.OutputDataCompressType != compress.CompressTypeNone {
								decompressed, err := compress.Decompress(data, latestExecution.OutputDataCompressType)
								if err == nil {
									data = decompressed
								}
							}

							// Try to unmarshal as generic JSON
							var genericOutput interface{}
							if err := json.Unmarshal(data, &genericOutput); err == nil {
								nodeData = genericOutput
								hasExecutionData = true
							}
						}
					}

					if hasExecutionData && nodeData != nil {
						// Extract the node-specific data
						if nodeMap, ok := nodeData.(map[string]interface{}); ok {
							if nodeSpecificData, hasNodeKey := nodeMap[currentNode.Name]; hasNodeKey {
								// Add response and request at ROOT level, not under node name
								if nodeVars, ok := nodeSpecificData.(map[string]interface{}); ok {
									// Add each variable directly without prefix
									if response, ok := nodeVars["response"]; ok {
										creator.AddWithKey("response", response)
									}
									if request, ok := nodeVars["request"]; ok {
										creator.AddWithKey("request", request)
									}
								}
							}
						}
					} else {
						// No execution data, provide the schema at root level
						creator.AddWithKey("request", map[string]interface{}{
							"headers": map[string]string{},
							"queries": map[string]string{},
							"body":    "string",
						})
						creator.AddWithKey("response", map[string]interface{}{
							"status":   200,
							"body":     map[string]interface{}{},
							"headers":  map[string]string{},
							"duration": 0,
						})
					}
				}
			}
		}
	}

	items := creator.FindMatchAndCalcCompletionData(req.Msg.Start)

	var Items []*referencev1.ReferenceCompletion

	for _, item := range items {
		Items = append(Items, &referencev1.ReferenceCompletion{
			Kind:         referencev1.ReferenceKind(item.Kind),
			EndToken:     item.EndToken,
			EndIndex:     item.EndIndex,
			ItemCount:    item.ItemCount,
			Environments: item.Environments,
		})
	}

	response := &referencev1.ReferenceCompletionResponse{
		Items: Items,
	}

	return connect.NewResponse(response), nil
}

// ReferenceValue calls reference.v1.ReferenceService.ReferenceValue.
func (c *ReferenceServiceRPC) ReferenceValue(ctx context.Context, req *connect.Request[referencev1.ReferenceValueRequest]) (*connect.Response[referencev1.ReferenceValueResponse], error) {
	var workspaceID, exampleID, nodeIDPtr *idwrap.IDWrap
	msg := req.Msg
	if msg.WorkspaceId != nil {
		tempID, err := idwrap.NewFromBytes(msg.WorkspaceId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		workspaceID = &tempID
	}
	if msg.ExampleId != nil {
		tempID, err := idwrap.NewFromBytes(msg.ExampleId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		exampleID = &tempID
	}
	if msg.NodeId != nil {
		tempID, err := idwrap.NewFromBytes(msg.NodeId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		nodeIDPtr = &tempID
	}

	lookup := referencecompletion.NewReferenceCompletionLookup()

	// Workspace
	if workspaceID != nil {
		wsID := *workspaceID
		rpcErr := permcheck.CheckPerm(rworkspace.CheckOwnerWorkspace(ctx, c.us, wsID))
		if rpcErr != nil {
			return nil, rpcErr
		}
		envs, err := c.es.GetByWorkspace(ctx, wsID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, ErrWorkspaceNotFound)
		}

		for _, env := range envs {
			vars, err := c.vs.GetVariableByEnvID(ctx, env.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, ErrEnvNotFound)
			}

			// Filter to only include enabled variables
			sortenabled.GetAllWithState(&vars, true)
			for _, v := range vars {
				lookup.AddWithKey(v.VarKey, v.Value)
			}
		}

	}

	if exampleID != nil {
		exID := *exampleID
		resp, err := c.ers.GetExampleRespByExampleIDLatest(ctx, exID)
		if err != nil {
			return nil, err
		}

		respHeaders, err := c.erhs.GetHeaderByRespID(ctx, resp.ID)
		if err != nil {
			return nil, err
		}

		headerMap := make(map[string]string)
		for _, header := range respHeaders {
			headerVal, ok := headerMap[header.HeaderKey]
			if ok {
				headerMap[header.HeaderKey] = headerVal + ", " + header.Value
			} else {
				headerMap[header.HeaderKey] = header.Value
			}
		}

		if resp.BodyCompressType != mexampleresp.BodyCompressTypeNone {
			if resp.BodyCompressType == mexampleresp.BodyCompressTypeZstd {
				data, err := zstdcompress.Decompress(resp.Body)
				if err != nil {
					return nil, err
				}
				resp.Body = data
			}
		}

		// check if body seems like json; if so decode it into a map[string]interface{}, otherwise use a string.
		var body any
		if json.Valid(resp.Body) {
			var jsonBody map[string]any
			// If unmarshaling works, use the decoded JSON.
			if err := json.Unmarshal(resp.Body, &jsonBody); err == nil {
				body = jsonBody
			} else {
				body = string(resp.Body)
			}
		} else {
			body = string(resp.Body)
		}

		// check if body seems like json

		httpResp := httpclient.ResponseVar{
			StatusCode: int(resp.Status),
			Body:       body,
			Headers:    headerMap,
			Duration:   resp.Duration,
		}

		var m map[string]any
		data, err := json.Marshal(httpResp)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(data, &m)
		if err != nil {
			return nil, err
		}

		lookup.AddWithKey("response", m)
	}

	if nodeIDPtr != nil {
		nodeID := *nodeIDPtr
		nodeInst, err := c.fns.GetNode(ctx, nodeID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		flowID := nodeInst.FlowID
		nodes, err := c.fns.GetNodesByFlowID(ctx, flowID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		flowVars, err := c.flowVariableService.GetFlowVariablesByFlowID(ctx, flowID)
		if err != nil {
			if err != sflowvariable.ErrNoFlowVariableFound {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			flowVars = []mflowvariable.FlowVariable{}
		}

		sortenabled.GetAllWithState(&flowVars, true)
		for _, flowVar := range flowVars {
			lookup.AddWithKey(flowVar.Name, flowVar.Value)
		}

		// Edges
		edges, err := c.flowEdgeService.GetEdgesByFlowID(ctx, flowID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		edgesMap := edge.NewEdgesMap(edges)

		beforeNodes := make([]mnnode.MNode, 0, len(nodes))
		for _, node := range nodes {
			if edge.IsNodeCheckTarget(edgesMap, node.ID, nodeID) == edge.NodeBefore {
				beforeNodes = append(beforeNodes, node)
			}
		}

		for _, node := range beforeNodes {
			// First, try to get execution data for ANY node type
			var nodeData interface{}
			hasExecutionData := false

			executions, err := c.nodeExecutionService.GetNodeExecutionsByNodeID(ctx, node.ID)
			if err == nil && len(executions) > 0 {
				// Filter out iteration executions from foreach/for nodes
				var latestExecution *mnodeexecution.NodeExecution
				for _, exec := range executions {
					if !isIterationExecution(exec.Name) {
						latestExecution = &exec
						break
					}
				}

				// Only proceed if we found a non-iteration execution
				if latestExecution != nil {
					// Decompress data if needed
					data := latestExecution.OutputData
					if latestExecution.OutputDataCompressType != compress.CompressTypeNone {
						decompressed, err := compress.Decompress(data, latestExecution.OutputDataCompressType)
						if err == nil {
							data = decompressed
						}
					}

					// Try to unmarshal as generic JSON
					var genericOutput interface{}
					if err := json.Unmarshal(data, &genericOutput); err == nil {
						nodeData = genericOutput
						hasExecutionData = true
					}
				}
			}

			// If we have execution data, use it
			if hasExecutionData && nodeData != nil {
				lookup.AddWithKey(node.Name, nodeData)
				continue
			}

			// Otherwise, provide schema for specific node types
			switch node.NodeKind {
			case mnnode.NODE_KIND_FOR_EACH:
				// For foreach loops, they write 'item' and 'key' variables
				nodeVarsMap := map[string]interface{}{
					"item": nil, // Can be any type from the iterated collection
					"key":  0,   // Index for arrays, string key for maps
				}
				lookup.AddWithKey(node.Name, nodeVarsMap)

			case mnnode.NODE_KIND_FOR:
				// For for loops, they write 'index' variable
				nodeVarsMap := map[string]interface{}{
					"index": 0,
				}
				lookup.AddWithKey(node.Name, nodeVarsMap)

			case mnnode.NODE_KIND_REQUEST:
				// For REQUEST nodes, provide the schema structure
				nodeVarsMap := map[string]interface{}{
					"request": map[string]interface{}{
						"headers": map[string]string{},
						"queries": map[string]string{},
						"body":    "string",
					},
					"response": map[string]interface{}{
						"status":   200,
						"body":     map[string]interface{}{},
						"headers":  map[string]string{},
						"duration": 0,
					},
				}
				lookup.AddWithKey(node.Name, nodeVarsMap)
			}
			// Other node types (JS, CONDITION, etc.) don't have default schemas
		}

		// Add self-reference for REQUEST nodes so they can reference their own variables
		// This allows REQUEST nodes to use "response.status" directly
		if nodeIDPtr != nil {
			currentNode, err := c.fns.GetNode(ctx, *nodeIDPtr)
			if err == nil && currentNode.NodeKind == mnnode.NODE_KIND_REQUEST {
				// REQUEST nodes can reference their own response and request directly (without prefix)
				var nodeData interface{}
				hasExecutionData := false

				// Try to get the current node's execution data
				executions, err := c.nodeExecutionService.GetNodeExecutionsByNodeID(ctx, currentNode.ID)
				if err == nil && len(executions) > 0 {
					// Filter out iteration executions
					var latestExecution *mnodeexecution.NodeExecution
					for _, exec := range executions {
						if !isIterationExecution(exec.Name) {
							latestExecution = &exec
							break
						}
					}

					if latestExecution != nil {
						// Decompress data if needed
						data := latestExecution.OutputData
						if latestExecution.OutputDataCompressType != compress.CompressTypeNone {
							decompressed, err := compress.Decompress(data, latestExecution.OutputDataCompressType)
							if err == nil {
								data = decompressed
							}
						}

						// Try to unmarshal as generic JSON
						var genericOutput interface{}
						if err := json.Unmarshal(data, &genericOutput); err == nil {
							nodeData = genericOutput
							hasExecutionData = true
						}
					}
				}

				if hasExecutionData && nodeData != nil {
					// Extract the node-specific data
					if nodeMap, ok := nodeData.(map[string]interface{}); ok {
						if nodeSpecificData, hasNodeKey := nodeMap[currentNode.Name]; hasNodeKey {
							// Add response and request at ROOT level, not under node name
							if nodeVars, ok := nodeSpecificData.(map[string]interface{}); ok {
								// Add each variable directly without prefix
								if response, ok := nodeVars["response"]; ok {
									lookup.AddWithKey("response", response)
								}
								if request, ok := nodeVars["request"]; ok {
									lookup.AddWithKey("request", request)
								}
							}
						}
					}
				} else {
					// No execution data, provide the schema at root level
					lookup.AddWithKey("request", map[string]interface{}{
						"headers": map[string]string{},
						"queries": map[string]string{},
						"body":    "string",
					})
					lookup.AddWithKey("response", map[string]interface{}{
						"status":   200,
						"body":     map[string]interface{}{},
						"headers":  map[string]string{},
						"duration": 0,
					})
				}
			}
		}
	}

	value, err := lookup.GetValue(req.Msg.Path)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	response := &referencev1.ReferenceValueResponse{
		Value: value,
	}

	return connect.NewResponse(response), nil
}
