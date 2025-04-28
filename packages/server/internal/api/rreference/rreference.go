package rreference

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/rworkspace"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/httpclient"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/model/mexampleresp"
	"the-dev-tools/server/pkg/model/mnnode"
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
	fs                  sflow.FlowService
	fns                 snode.NodeService
	frns                snoderequest.NodeRequestService
	flowVariableService sflowvariable.FlowVariableService
	flowEdgeService     sedge.EdgeService
}

func NewNodeServiceRPC(db *sql.DB, us suser.UserService, ws sworkspace.WorkspaceService,
	es senv.EnvService, vs svar.VarService,
	ers sexampleresp.ExampleRespService, erhs sexamplerespheader.ExampleRespHeaderService,
	fs sflow.FlowService, fns snode.NodeService, frns snoderequest.NodeRequestService,
	flowVariableService sflowvariable.FlowVariableService,
	edgeService sedge.EdgeService,
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

		flowEdgeService: edgeService,
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
		stateData := node.StateData
		if json.Valid(stateData) {
			var anyStateData any
			err = json.Unmarshal(stateData, &anyStateData)
			if err != nil {
				return nil, err
			}

			ref := reference.NewReferenceFromInterfaceWithKey(anyStateData, node.Name)
			nodeRefs = append(nodeRefs, reference.ConvertPkgToRpcTree(ref))
		}
	}

	return nodeRefs, nil
}

func GetExampleRespByExampleID(ctx context.Context, ers sexampleresp.ExampleRespService, erhs sexamplerespheader.ExampleRespHeaderService, exID idwrap.IDWrap) (*reference.ReferenceTreeItem, error) {
	resp, err := ers.GetExampleRespByExampleID(ctx, exID)
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
	var body interface{}
	if json.Valid(resp.Body) {
		var jsonBody map[string]interface{}
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

	fmt.Println(workspaceID, exampleID, nodeIDPtr)

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

		envKeyValueMap := make(map[string]string, len(envs))

		for _, env := range envs {
			vars, err := c.vs.GetVariableByEnvID(ctx, env.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, ErrEnvNotFound)
			}

			for _, v := range vars {
				envKeyValueMap[v.VarKey] = v.Value
			}
		}

		if len(envKeyValueMap) > 0 {
			creator.Add(envKeyValueMap)
			fmt.Println(envKeyValueMap)
		}

	}

	fmt.Println(req.Msg.Start)
	items := creator.FindMatchAndCalcCompletionData(req.Msg.Start)
	fmt.Println(items)

	var Items []*referencev1.ReferenceCompletion

	for _, item := range items {
		Items = append(Items, &referencev1.ReferenceCompletion{
			Kind:         referencev1.ReferenceKind(item.Kind),
			EndToken:     item.EndToken,
			ItemCount:    item.ItemCount,
			Environments: item.Environments,
		})
		fmt.Println(item.EndToken)
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

	fmt.Println(workspaceID, exampleID, nodeIDPtr)

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

		envKeyValueMap := make(map[string]string, len(envs))

		for _, env := range envs {
			vars, err := c.vs.GetVariableByEnvID(ctx, env.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, ErrEnvNotFound)
			}

			for _, v := range vars {
				envKeyValueMap[v.VarKey] = v.Value
			}
		}

		fmt.Println(envKeyValueMap)
		lookup.Add(envKeyValueMap)
	}

	fmt.Println(req.Msg.Path)

	response := &referencev1.ReferenceValueResponse{
		Value: fmt.Sprint(lookup.GetValue(req.Msg.Path)),
	}

	return connect.NewResponse(response), nil
}
