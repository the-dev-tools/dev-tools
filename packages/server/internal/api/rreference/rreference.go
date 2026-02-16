//nolint:revive // exported
package rreference

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/compress"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/menv"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/permcheck"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/reference"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/referencecompletion"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/senv"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/sort/sortenabled"
	referencev1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/reference/v1"
	"github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/reference/v1/referencev1connect"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sgraphql"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/shttp"

	"connectrpc.com/connect"
)

type ReferenceServiceRPC struct {
	DB *sql.DB

	userReader      *sworkspace.UserReader
	workspaceReader *sworkspace.WorkspaceReader

	// env
	envReader *senv.EnvReader
	varReader *senv.VariableReader

	// flow
	flowReader          *sflow.FlowReader
	nodeReader          *sflow.NodeReader
	nodeRequestReader   *sflow.NodeRequestReader
	flowVariableReader  *sflow.FlowVariableReader
	flowEdgeReader      *sflow.EdgeReader
	nodeExecutionReader *sflow.NodeExecutionReader

	// http
	httpResponseReader *shttp.HttpResponseReader

	// graphql
	graphqlResponseReader *sgraphql.GraphQLResponseService
}

type ReferenceServiceRPCReaders struct {
	User          *sworkspace.UserReader
	Workspace     *sworkspace.WorkspaceReader
	Env           *senv.EnvReader
	Variable      *senv.VariableReader
	Flow          *sflow.FlowReader
	Node          *sflow.NodeReader
	NodeRequest   *sflow.NodeRequestReader
	FlowVariable  *sflow.FlowVariableReader
	FlowEdge      *sflow.EdgeReader
	NodeExecution     *sflow.NodeExecutionReader
	HttpResponse      *shttp.HttpResponseReader
	GraphQLResponse   *sgraphql.GraphQLResponseService
}

func (r *ReferenceServiceRPCReaders) Validate() error {
	if r.User == nil {
		return fmt.Errorf("user reader is required")
	}
	if r.Workspace == nil {
		return fmt.Errorf("workspace reader is required")
	}
	if r.Env == nil {
		return fmt.Errorf("env reader is required")
	}
	if r.Variable == nil {
		return fmt.Errorf("variable reader is required")
	}
	if r.Flow == nil {
		return fmt.Errorf("flow reader is required")
	}
	if r.Node == nil {
		return fmt.Errorf("node reader is required")
	}
	if r.NodeRequest == nil {
		return fmt.Errorf("node request reader is required")
	}
	if r.FlowVariable == nil {
		return fmt.Errorf("flow variable reader is required")
	}
	if r.FlowEdge == nil {
		return fmt.Errorf("flow edge reader is required")
	}
	if r.NodeExecution == nil {
		return fmt.Errorf("node execution reader is required")
	}
	if r.HttpResponse == nil {
		return fmt.Errorf("http response reader is required")
	}
	if r.GraphQLResponse == nil {
		return fmt.Errorf("graphql response reader is required")
	}
	return nil
}

type ReferenceServiceRPCDeps struct {
	DB      *sql.DB
	Readers ReferenceServiceRPCReaders
}

func (d *ReferenceServiceRPCDeps) Validate() error {
	if d.DB == nil {
		return fmt.Errorf("db is required")
	}
	if err := d.Readers.Validate(); err != nil {
		return err
	}
	return nil
}

func NewReferenceServiceRPC(deps ReferenceServiceRPCDeps) *ReferenceServiceRPC {
	if err := deps.Validate(); err != nil {
		panic(fmt.Sprintf("ReferenceServiceRPC Deps validation failed: %v", err))
	}

	return &ReferenceServiceRPC{
		DB: deps.DB,

		userReader:      deps.Readers.User,
		workspaceReader: deps.Readers.Workspace,

		envReader: deps.Readers.Env,
		varReader: deps.Readers.Variable,

		flowReader:          deps.Readers.Flow,
		nodeReader:          deps.Readers.Node,
		nodeRequestReader:   deps.Readers.NodeRequest,
		flowVariableReader:  deps.Readers.FlowVariable,
		flowEdgeReader:      deps.Readers.FlowEdge,
		nodeExecutionReader: deps.Readers.NodeExecution,
		httpResponseReader:  deps.Readers.HttpResponse,
		graphqlResponseReader: deps.Readers.GraphQLResponse,
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

// referenceKindProtoFallback is emitted when a completion kind is unknown.
const referenceKindProtoFallback = referencev1.ReferenceKind_REFERENCE_KIND_UNSPECIFIED

func referenceKindToProto(kind reference.ReferenceKind) (referencev1.ReferenceKind, error) {
	switch kind {
	case reference.ReferenceKind_REFERENCE_KIND_UNSPECIFIED:
		return referencev1.ReferenceKind_REFERENCE_KIND_UNSPECIFIED, nil
	case reference.ReferenceKind_REFERENCE_KIND_MAP:
		return referencev1.ReferenceKind_REFERENCE_KIND_MAP, nil
	case reference.ReferenceKind_REFERENCE_KIND_ARRAY:
		return referencev1.ReferenceKind_REFERENCE_KIND_ARRAY, nil
	case reference.ReferenceKind_REFERENCE_KIND_VALUE:
		return referencev1.ReferenceKind_REFERENCE_KIND_VALUE, nil
	case reference.ReferenceKind_REFERENCE_KIND_VARIABLE:
		return referencev1.ReferenceKind_REFERENCE_KIND_VARIABLE, nil
	default:
		return referenceKindProtoFallback, fmt.Errorf("unknown reference kind: %d", kind)
	}
}

var convertReferenceCompletionItemsFn = convertReferenceCompletionItems

func convertReferenceCompletionItems(items []referencecompletion.ReferenceCompletionItem) ([]*referencev1.ReferenceCompletion, error) {
	if len(items) == 0 {
		return nil, nil
	}

	converted := make([]*referencev1.ReferenceCompletion, 0, len(items))
	for _, item := range items {
		kind, err := referenceKindToProto(item.Kind)
		if err != nil {
			return nil, fmt.Errorf("reference kind to proto: %w", err)
		}

		converted = append(converted, &referencev1.ReferenceCompletion{
			Kind:         kind,
			EndToken:     item.EndToken,
			EndIndex:     item.EndIndex,
			ItemCount:    item.ItemCount,
			Environments: item.Environments,
		})
	}

	return converted, nil
}

func (c *ReferenceServiceRPC) getLatestResponse(ctx context.Context, httpID idwrap.IDWrap) (map[string]interface{}, error) {
	responses, err := c.httpResponseReader.GetByHttpID(ctx, httpID)
	if err != nil {
		return nil, err
	}
	if len(responses) == 0 {
		return nil, nil
	}

	// Find latest response
	latest := responses[0]
	for _, r := range responses {
		if r.CreatedAt > latest.CreatedAt {
			latest = r
		}
	}

	// Parse body
	var body interface{} = string(latest.Body)
	if len(latest.Body) > 0 {
		var jsonBody interface{}
		if err := json.Unmarshal(latest.Body, &jsonBody); err == nil {
			body = jsonBody
		}
	}

	return map[string]interface{}{
		"status":   latest.Status,
		"body":     body,
		"headers":  map[string]string{}, // Headers not currently linkable to specific response
		"duration": latest.Duration,
		"size":     latest.Size,
	}, nil
}

func (c *ReferenceServiceRPC) getLatestGraphQLResponse(ctx context.Context, graphqlID idwrap.IDWrap) (map[string]interface{}, error) {
	responses, err := c.graphqlResponseReader.GetByGraphQLID(ctx, graphqlID)
	if err != nil {
		return nil, err
	}
	if len(responses) == 0 {
		return nil, nil
	}

	// Find latest response
	latest := responses[0]
	for _, r := range responses {
		if r.Time > latest.Time {
			latest = r
		}
	}

	// Parse body
	var body interface{} = string(latest.Body)
	var bodyMap map[string]interface{}
	if len(latest.Body) > 0 {
		var jsonBody interface{}
		if err := json.Unmarshal(latest.Body, &jsonBody); err == nil {
			body = jsonBody
			if m, ok := jsonBody.(map[string]interface{}); ok {
				bodyMap = m
			}
		}
	}

	// Extract GraphQL-specific fields (data and errors)
	var data interface{}
	var errors interface{}
	if bodyMap != nil {
		if d, ok := bodyMap["data"]; ok {
			data = d
		}
		if e, ok := bodyMap["errors"]; ok {
			errors = e
		}
	}

	return map[string]interface{}{
		"status":   latest.Status,
		"body":     body,
		"data":     data,
		"errors":   errors,
		"headers":  map[string]string{}, // Headers not currently linkable to specific response
		"duration": latest.Duration,
		"size":     latest.Size,
	}, nil
}

func (c *ReferenceServiceRPC) ReferenceTree(ctx context.Context, req *connect.Request[referencev1.ReferenceTreeRequest]) (*connect.Response[referencev1.ReferenceTreeResponse], error) {
	var Items []*referencev1.ReferenceTreeItem

	var workspaceID, httpID, flowNodeID *idwrap.IDWrap
	msg := req.Msg
	if msg.WorkspaceId != nil {
		tempID, err := idwrap.NewFromBytes(msg.WorkspaceId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		workspaceID = &tempID
	}
	if msg.HttpId != nil {
		tempID, err := idwrap.NewFromBytes(msg.HttpId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		httpID = &tempID
	}
	if msg.FlowNodeId != nil {
		tempID, err := idwrap.NewFromBytes(msg.FlowNodeId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		flowNodeID = &tempID
	}

	// Workspace
	if workspaceID != nil {
		wsID := *workspaceID
		rpcErr := permcheck.CheckPerm(true, mwauth.CheckOwnerWorkspaceWithReader(ctx, c.userReader, wsID))
		if rpcErr != nil {
			return nil, rpcErr
		}
		envs, err := c.envReader.ListEnvironments(ctx, wsID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, ErrWorkspaceNotFound)
		}

		present := make(map[string][]menv.Env)
		envMap := make([]*referencev1.ReferenceTreeItem, 0, len(envs))
		var allVars []menv.Variable

		for _, env := range envs {
			vars, err := c.varReader.GetVariableByEnvID(ctx, env.ID)
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
	if httpID != nil {
		resp, err := c.getLatestResponse(ctx, *httpID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if resp != nil {
			respRef := reference.NewReferenceFromInterfaceWithKey(resp, "response")
			converted, err := reference.ConvertPkgToRpcTree(respRef)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			Items = append(Items, converted)
		}
	}

	// Node
	if flowNodeID != nil {
		refs, err := c.HandleNode(ctx, *flowNodeID)
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
	nodeInst, err := c.nodeReader.GetNode(ctx, nodeID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	flowID := nodeInst.FlowID
	nodes, err := c.nodeReader.GetNodesByFlowID(ctx, flowID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var nodeRefs []*referencev1.ReferenceTreeItem
	appendNodeRef := func(item reference.ReferenceTreeItem, context string) error {
		converted, err := reference.ConvertPkgToRpcTree(item)
		if err != nil {
			return fmt.Errorf("convert %s: %w", context, err)
		}
		nodeRefs = append(nodeRefs, converted)
		return nil
	}

	flowVars, err := c.flowVariableReader.GetFlowVariablesByFlowID(ctx, flowID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	sortenabled.GetAllWithState(&flowVars, true)
	for _, flowVar := range flowVars {
		flowVarRef := reference.NewReferenceFromInterfaceWithKey(flowVar.Value, flowVar.Name)
		if err := appendNodeRef(flowVarRef, fmt.Sprintf("flow variable %q", flowVar.Name)); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	// Edges
	edges, err := c.flowEdgeReader.GetEdgesByFlowID(ctx, flowID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	edgesMap := mflow.NewEdgesMap(edges)

	beforeNodes := make([]mflow.Node, 0, len(nodes))
	for _, node := range nodes {
		if mflow.IsNodeCheckTarget(edgesMap, node.ID, nodeID) == mflow.NodeBefore {
			beforeNodes = append(beforeNodes, node)
		}
	}

	for _, node := range beforeNodes {
		// First, try to get execution data for ANY node type
		var nodeData interface{}
		hasExecutionData := false

		executions, err := c.nodeExecutionReader.GetNodeExecutionsByNodeID(ctx, node.ID)
		if err == nil && len(executions) > 0 {
			// Use the latest execution (first one, as they're ordered by ID DESC)
			// This includes iteration executions which now contain the actual values
			latestExecution := &executions[0]

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
					nodeVarRef := reference.NewReferenceFromInterfaceWithKey(nodeSpecificData, node.Name)
					if err := appendNodeRef(nodeVarRef, fmt.Sprintf("node %q execution data", node.Name)); err != nil {
						return nil, connect.NewError(connect.CodeInternal, err)
					}
				} else {
					// Data doesn't have the expected structure, use it as-is
					nodeVarRef := reference.NewReferenceFromInterfaceWithKey(nodeData, node.Name)
					if err := appendNodeRef(nodeVarRef, fmt.Sprintf("node %q execution data fallback", node.Name)); err != nil {
						return nil, connect.NewError(connect.CodeInternal, err)
					}
				}
			} else {
				// Not a map, use directly
				nodeVarRef := reference.NewReferenceFromInterfaceWithKey(nodeData, node.Name)
				if err := appendNodeRef(nodeVarRef, fmt.Sprintf("node %q execution data direct", node.Name)); err != nil {
					return nil, connect.NewError(connect.CodeInternal, err)
				}
			}
			continue
		}

		// Otherwise, provide schema for specific node types
		switch node.NodeKind {
		case mflow.NODE_KIND_FOR_EACH:
			// For foreach loops, they write 'item' and 'key' variables
			nodeVarsMap := map[string]interface{}{
				"item": nil, // Can be any type from the iterated collection
				"key":  0,   // Index for arrays, string key for maps
			}
			nodeVarRef := reference.NewReferenceFromInterfaceWithKey(nodeVarsMap, node.Name)
			if err := appendNodeRef(nodeVarRef, fmt.Sprintf("node %q foreach schema", node.Name)); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

		case mflow.NODE_KIND_FOR:
			// For for loops, they write 'index' variable
			nodeVarsMap := map[string]interface{}{
				"index": 0,
			}
			nodeVarRef := reference.NewReferenceFromInterfaceWithKey(nodeVarsMap, node.Name)
			if err := appendNodeRef(nodeVarRef, fmt.Sprintf("node %q for schema", node.Name)); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

		case mflow.NODE_KIND_REQUEST:
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
			if err := appendNodeRef(nodeVarRef, fmt.Sprintf("node %q request schema", node.Name)); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		}
		// Other node types (JS, CONDITION, etc.) don't have default schemas
	}

	return nodeRefs, nil
}

// ReferenceCompletion calls reference.v1.ReferenceService.ReferenceCompletion.
func (c *ReferenceServiceRPC) ReferenceCompletion(ctx context.Context, req *connect.Request[referencev1.ReferenceCompletionRequest]) (*connect.Response[referencev1.ReferenceCompletionResponse], error) {
	var workspaceID, httpID, graphqlID, flowNodeID *idwrap.IDWrap
	msg := req.Msg
	if msg.WorkspaceId != nil {
		tempID, err := idwrap.NewFromBytes(msg.WorkspaceId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		workspaceID = &tempID
	}
	if msg.HttpId != nil {
		tempID, err := idwrap.NewFromBytes(msg.HttpId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		httpID = &tempID
	}
	if msg.GraphqlId != nil {
		tempID, err := idwrap.NewFromBytes(msg.GraphqlId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		graphqlID = &tempID
	}
	if msg.FlowNodeId != nil {
		tempID, err := idwrap.NewFromBytes(msg.FlowNodeId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		flowNodeID = &tempID
	}

	creator := referencecompletion.NewReferenceCompletionCreator()

	// Environment variables namespace - collect all env vars under "env" key
	envVarsMap := make(map[string]any)

	// Workspace environment variables
	if workspaceID != nil {
		wsID := *workspaceID
		rpcErr := permcheck.CheckPerm(true, mwauth.CheckOwnerWorkspaceWithReader(ctx, c.userReader, wsID))
		if rpcErr != nil {
			return nil, rpcErr
		}
		envs, err := c.envReader.ListEnvironments(ctx, wsID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, ErrWorkspaceNotFound)
		}

		for _, env := range envs {
			vars, err := c.varReader.GetVariableByEnvID(ctx, env.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, ErrEnvNotFound)
			}

			// Filter to only include enabled variables
			sortenabled.GetAllWithState(&vars, true)
			for _, v := range vars {
				// Add to env vars map
				envVarsMap[v.VarKey] = v.Value
			}
		}
	}

	if httpID != nil {
		resp, err := c.getLatestResponse(ctx, *httpID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if resp != nil {
			creator.AddWithKey("response", resp)
		} else {
			// Fallback schema
			creator.AddWithKey("response", map[string]interface{}{
				"status":   200,
				"body":     map[string]interface{}{},
				"headers":  map[string]string{},
				"duration": 0,
			})
		}

		// Request schema (always present for now as we don't fetch actual request config yet)
		creator.AddWithKey("request", map[string]interface{}{
			"headers": map[string]string{},
			"queries": map[string]string{},
			"body":    "string",
		})
	}

	if graphqlID != nil {
		resp, err := c.getLatestGraphQLResponse(ctx, *graphqlID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if resp != nil {
			// Add full response object
			creator.AddWithKey("response", resp)

			// Add GraphQL-specific top-level fields for convenience
			if data, ok := resp["data"]; ok && data != nil {
				creator.AddWithKey("data", data)
			}
			if errors, ok := resp["errors"]; ok && errors != nil {
				creator.AddWithKey("errors", errors)
			}

			// Add convenience variables
			status := int(0)
			if s, ok := resp["status"].(int32); ok {
				status = int(s)
			}
			creator.AddWithKey("status", status)
			creator.AddWithKey("success", status >= 200 && status < 300)
			creator.AddWithKey("has_data", resp["data"] != nil)
			creator.AddWithKey("has_errors", resp["errors"] != nil)
		} else {
			// Fallback schema for GraphQL
			creator.AddWithKey("response", map[string]interface{}{
				"status":   200,
				"body":     map[string]interface{}{},
				"data":     map[string]interface{}{},
				"errors":   nil,
				"headers":  map[string]string{},
				"duration": 0,
			})
			creator.AddWithKey("data", map[string]interface{}{})
			creator.AddWithKey("status", 200)
			creator.AddWithKey("success", true)
			creator.AddWithKey("has_data", false)
			creator.AddWithKey("has_errors", false)
		}
	}

	if flowNodeID != nil {
		nodeID := *flowNodeID
		nodeInst, err := c.nodeReader.GetNode(ctx, nodeID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		flowID := nodeInst.FlowID
		nodes, err := c.nodeReader.GetNodesByFlowID(ctx, flowID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		flowVars, err := c.flowVariableReader.GetFlowVariablesByFlowID(ctx, flowID)
		if err != nil {
			if !errors.Is(err, sflow.ErrNoFlowVariableFound) {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			flowVars = []mflow.FlowVariable{}
		}

		sortenabled.GetAllWithState(&flowVars, true)
		for _, flowVar := range flowVars {
			// Add flow variables (same as workspace env vars)
			envVarsMap[flowVar.Name] = flowVar.Value
		}

		// Edges
		edges, err := c.flowEdgeReader.GetEdgesByFlowID(ctx, flowID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		edgesMap := mflow.NewEdgesMap(edges)

		beforeNodes := make([]mflow.Node, 0, len(nodes))
		for _, node := range nodes {
			if mflow.IsNodeCheckTarget(edgesMap, node.ID, nodeID) == mflow.NodeBefore {
				beforeNodes = append(beforeNodes, node)
			}
		}

		for _, node := range beforeNodes {
			// First, try to get execution data for ANY node type
			var nodeData interface{}
			hasExecutionData := false

			executions, err := c.nodeExecutionReader.GetNodeExecutionsByNodeID(ctx, node.ID)
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
			case mflow.NODE_KIND_FOR_EACH:
				// For foreach loops, they write 'item' and 'key' variables
				nodeVarsMap := map[string]interface{}{
					"item": nil, // Can be any type from the iterated collection
					"key":  0,   // Index for arrays, string key for maps
				}
				creator.AddWithKey(node.Name, nodeVarsMap)

			case mflow.NODE_KIND_FOR:
				// For for loops, they write 'index' variable
				nodeVarsMap := map[string]interface{}{
					"index": 0,
				}
				creator.AddWithKey(node.Name, nodeVarsMap)

			case mflow.NODE_KIND_REQUEST:
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

			case mflow.NODE_KIND_AI:
				// For AI nodes, provide the output schema
				nodeVarsMap := map[string]interface{}{
					"text":          "",
					"total_metrics": map[string]interface{}{},
					"iteration":     0,
				}
				creator.AddWithKey(node.Name, nodeVarsMap)

			case mflow.NODE_KIND_JS:
				// For JS nodes, the node itself is the reference (js_5, not js_5.result)
				// JS returns dynamic output, so we provide an empty map as placeholder
				nodeVarsMap := map[string]interface{}{}
				creator.AddWithKey(node.Name, nodeVarsMap)

			case mflow.NODE_KIND_CONDITION:
				// For condition/IF nodes, provide the output schema
				nodeVarsMap := map[string]interface{}{
					"condition": "",
					"result":    false,
				}
				creator.AddWithKey(node.Name, nodeVarsMap)

			case mflow.NODE_KIND_AI_PROVIDER:
				// For AI Provider nodes, provide the output schema
				nodeVarsMap := map[string]interface{}{
					"text":       "",
					"tool_calls": []interface{}{},
					"metrics":    map[string]interface{}{},
				}
				creator.AddWithKey(node.Name, nodeVarsMap)
			}
		}

		// Add self-reference for FOR, FOREACH, and REQUEST nodes so they can reference their own variables
		// This enables break conditions like "if foreach_8.index > 8" and request nodes to use "response.status"
		if true {
			currentNode, err := c.nodeReader.GetNode(ctx, *flowNodeID)
			if err == nil {
				switch currentNode.NodeKind {
				case mflow.NODE_KIND_FOR:
					// FOR nodes can reference their own index from execution data
					var nodeData interface{}
					hasExecutionData := false

					// Try to get the current node's execution data
					executions, err := c.nodeExecutionReader.GetNodeExecutionsByNodeID(ctx, currentNode.ID)
					if err == nil && len(executions) > 0 {
						// Use the latest execution (first one, as they're ordered by ID DESC)
						latestExecution := &executions[0]

						if true {
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
						// Use the actual execution data
						creator.AddWithKey(currentNode.Name, nodeData)
					} else {
						// No execution data, provide the schema
						nodeVarsMap := map[string]interface{}{
							"index": 0,
						}
						creator.AddWithKey(currentNode.Name, nodeVarsMap)
					}

				case mflow.NODE_KIND_FOR_EACH:
					// FOREACH nodes can reference their own item and key from execution data
					var nodeData interface{}
					hasExecutionData := false

					// Try to get the current node's execution data
					executions, err := c.nodeExecutionReader.GetNodeExecutionsByNodeID(ctx, currentNode.ID)
					if err == nil && len(executions) > 0 {
						// Use the latest execution (first one, as they're ordered by ID DESC)
						latestExecution := &executions[0]

						if true {
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
						// Use the actual execution data
						creator.AddWithKey(currentNode.Name, nodeData)
					} else {
						// No execution data, provide the schema
						nodeVarsMap := map[string]interface{}{
							"item": nil,
							"key":  0,
						}
						creator.AddWithKey(currentNode.Name, nodeVarsMap)
					}

				case mflow.NODE_KIND_REQUEST:
					// REQUEST nodes can reference their own response and request directly (without prefix)
					var nodeData interface{}
					hasExecutionData := false

					// Try to get the current node's execution data
					executions, err := c.nodeExecutionReader.GetNodeExecutionsByNodeID(ctx, currentNode.ID)
					if err == nil && len(executions) > 0 {
						// Use the latest execution (first one, as they're ordered by ID DESC)
						latestExecution := &executions[0]

						if true {
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

					dataAdded := false
					if hasExecutionData && nodeData != nil {
						// Extract the node-specific data
						if nodeMap, ok := nodeData.(map[string]interface{}); ok {
							if nodeSpecificData, hasNodeKey := nodeMap[currentNode.Name]; hasNodeKey {
								// Add the entire node data at ROOT level
								// This allows direct access to response.* and request.*
								if nodeVars, ok := nodeSpecificData.(map[string]interface{}); ok {
									// Add all variables from the node directly at root
									for key, value := range nodeVars {
										creator.AddWithKey(key, value)
									}
									dataAdded = true
								}
							}
						}
					}

					if !dataAdded {
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

	// Add all environment variables at root level
	// Access via {{ apiKey }} or {{ varName }}
	for k, v := range envVarsMap {
		creator.AddWithKey(k, v)
	}

	items := creator.FindMatchAndCalcCompletionData(req.Msg.Start)

	completions, err := convertReferenceCompletionItemsFn(items)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("convert reference completion items: %w", err))
	}

	response := &referencev1.ReferenceCompletionResponse{
		Items: completions,
	}

	return connect.NewResponse(response), nil
}

// ReferenceValue calls reference.v1.ReferenceService.ReferenceValue.
func (c *ReferenceServiceRPC) ReferenceValue(ctx context.Context, req *connect.Request[referencev1.ReferenceValueRequest]) (*connect.Response[referencev1.ReferenceValueResponse], error) {
	var workspaceID, httpID, graphqlID, flowNodeID *idwrap.IDWrap
	msg := req.Msg
	if msg.WorkspaceId != nil {
		tempID, err := idwrap.NewFromBytes(msg.WorkspaceId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		workspaceID = &tempID
	}
	if msg.HttpId != nil {
		tempID, err := idwrap.NewFromBytes(msg.HttpId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		httpID = &tempID
	}
	if msg.GraphqlId != nil {
		tempID, err := idwrap.NewFromBytes(msg.GraphqlId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		graphqlID = &tempID
	}
	if msg.FlowNodeId != nil {
		tempID, err := idwrap.NewFromBytes(msg.FlowNodeId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		flowNodeID = &tempID
	}

	lookup := referencecompletion.NewReferenceCompletionLookup()

	// Environment variables namespace - collect all env vars under "env" key
	envVarsMapLookup := make(map[string]any)

	// Workspace environment variables
	if workspaceID != nil {
		wsID := *workspaceID
		rpcErr := permcheck.CheckPerm(true, mwauth.CheckOwnerWorkspaceWithReader(ctx, c.userReader, wsID))
		if rpcErr != nil {
			return nil, rpcErr
		}
		envs, err := c.envReader.ListEnvironments(ctx, wsID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, ErrWorkspaceNotFound)
		}

		for _, env := range envs {
			vars, err := c.varReader.GetVariableByEnvID(ctx, env.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, ErrEnvNotFound)
			}

			// Filter to only include enabled variables
			sortenabled.GetAllWithState(&vars, true)
			for _, v := range vars {
				// Add to env vars map
				envVarsMapLookup[v.VarKey] = v.Value
			}
		}
	}

	if httpID != nil {
		resp, err := c.getLatestResponse(ctx, *httpID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if resp != nil {
			lookup.AddWithKey("response", resp)
		} else {
			// Fallback schema
			lookup.AddWithKey("response", map[string]interface{}{
				"status":   200,
				"body":     map[string]interface{}{},
				"headers":  map[string]string{},
				"duration": 0,
			})
		}

		// Request schema
		lookup.AddWithKey("request", map[string]interface{}{
			"headers": map[string]string{},
			"queries": map[string]string{},
			"body":    "string",
		})
	}

	if graphqlID != nil {
		resp, err := c.getLatestGraphQLResponse(ctx, *graphqlID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if resp != nil {
			// Add full response object
			lookup.AddWithKey("response", resp)

			// Add GraphQL-specific top-level fields for convenience
			if data, ok := resp["data"]; ok && data != nil {
				lookup.AddWithKey("data", data)
			}
			if errors, ok := resp["errors"]; ok && errors != nil {
				lookup.AddWithKey("errors", errors)
			}

			// Add convenience variables
			status := int(0)
			if s, ok := resp["status"].(int32); ok {
				status = int(s)
			}
			lookup.AddWithKey("status", status)
			lookup.AddWithKey("success", status >= 200 && status < 300)
			lookup.AddWithKey("has_data", resp["data"] != nil)
			lookup.AddWithKey("has_errors", resp["errors"] != nil)
		} else {
			// Fallback schema for GraphQL
			lookup.AddWithKey("response", map[string]interface{}{
				"status":   200,
				"body":     map[string]interface{}{},
				"data":     map[string]interface{}{},
				"errors":   nil,
				"headers":  map[string]string{},
				"duration": 0,
			})
			lookup.AddWithKey("data", map[string]interface{}{})
			lookup.AddWithKey("status", 200)
			lookup.AddWithKey("success", true)
			lookup.AddWithKey("has_data", false)
			lookup.AddWithKey("has_errors", false)
		}
	}

	if flowNodeID != nil {
		nodeID := *flowNodeID
		nodeInst, err := c.nodeReader.GetNode(ctx, nodeID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		flowID := nodeInst.FlowID
		nodes, err := c.nodeReader.GetNodesByFlowID(ctx, flowID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		flowVars, err := c.flowVariableReader.GetFlowVariablesByFlowID(ctx, flowID)
		if err != nil {
			if !errors.Is(err, sflow.ErrNoFlowVariableFound) {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			flowVars = []mflow.FlowVariable{}
		}

		sortenabled.GetAllWithState(&flowVars, true)
		for _, flowVar := range flowVars {
			// Add flow variables (same as workspace env vars)
			envVarsMapLookup[flowVar.Name] = flowVar.Value
		}

		// Edges
		edges, err := c.flowEdgeReader.GetEdgesByFlowID(ctx, flowID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		edgesMap := mflow.NewEdgesMap(edges)

		beforeNodes := make([]mflow.Node, 0, len(nodes))
		for _, node := range nodes {
			if mflow.IsNodeCheckTarget(edgesMap, node.ID, nodeID) == mflow.NodeBefore {
				beforeNodes = append(beforeNodes, node)
			}
		}

		for _, node := range beforeNodes {
			// First, try to get execution data for ANY node type
			var nodeData interface{}
			hasExecutionData := false

			executions, err := c.nodeExecutionReader.GetNodeExecutionsByNodeID(ctx, node.ID)
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
						lookup.AddWithKey(node.Name, nodeSpecificData)
					} else {
						// Data doesn't have the expected structure, use it as-is
						lookup.AddWithKey(node.Name, nodeData)
					}
				} else {
					// Not a map, use directly
					lookup.AddWithKey(node.Name, nodeData)
				}
				continue
			}

			// Otherwise, provide schema for specific node types
			switch node.NodeKind {
			case mflow.NODE_KIND_FOR_EACH:
				// For foreach loops, they write 'item' and 'key' variables
				nodeVarsMap := map[string]interface{}{
					"item": nil, // Can be any type from the iterated collection
					"key":  0,   // Index for arrays, string key for maps
				}
				lookup.AddWithKey(node.Name, nodeVarsMap)

			case mflow.NODE_KIND_FOR:
				// For for loops, they write 'index' variable
				nodeVarsMap := map[string]interface{}{
					"index": 0,
				}
				lookup.AddWithKey(node.Name, nodeVarsMap)

			case mflow.NODE_KIND_REQUEST:
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

		// Add self-reference for REQUEST, FOR, and FOREACH nodes so they can reference their own variables
		// This allows these nodes to use their own variables directly
		if true {
			currentNode, err := c.nodeReader.GetNode(ctx, *flowNodeID)
			if err == nil {
				switch currentNode.NodeKind {
				case mflow.NODE_KIND_FOR, mflow.NODE_KIND_FOR_EACH:
					// FOR and FOREACH nodes can reference their own index/item/key from execution data
					var nodeData interface{}
					hasExecutionData := false

					// Try to get the current node's execution data
					executions, err := c.nodeExecutionReader.GetNodeExecutionsByNodeID(ctx, currentNode.ID)
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

					if hasExecutionData && nodeData != nil {
						// Add the execution data for self-reference
						lookup.AddWithKey(currentNode.Name, nodeData)
					} else {
						// No execution data, provide the schema
						if currentNode.NodeKind == mflow.NODE_KIND_FOR {
							lookup.AddWithKey(currentNode.Name, map[string]interface{}{
								"index": 0,
							})
						} else {
							lookup.AddWithKey(currentNode.Name, map[string]interface{}{
								"item": nil,
								"key":  0,
							})
						}
					}

				case mflow.NODE_KIND_REQUEST:
					// REQUEST nodes can reference their own response and request directly (without prefix)
					var nodeData interface{}
					hasExecutionData := false

					// Try to get the current node's execution data
					executions, err := c.nodeExecutionReader.GetNodeExecutionsByNodeID(ctx, currentNode.ID)
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

					dataAdded := false
					if hasExecutionData && nodeData != nil {
						// Extract the node-specific data
						if nodeMap, ok := nodeData.(map[string]interface{}); ok {
							if nodeSpecificData, hasNodeKey := nodeMap[currentNode.Name]; hasNodeKey {
								// Add the entire node data at ROOT level
								// This allows direct access to response.* and request.*
								if nodeVars, ok := nodeSpecificData.(map[string]interface{}); ok {
									// Add all variables from the node directly at root
									for key, value := range nodeVars {
										lookup.AddWithKey(key, value)
									}
									dataAdded = true
								}
							}
						}
					}

					if !dataAdded {
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
	}

	// Add all environment variables at root level
	// Access via {{ apiKey }} or {{ varName }}
	for k, v := range envVarsMapLookup {
		lookup.AddWithKey(k, v)
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
