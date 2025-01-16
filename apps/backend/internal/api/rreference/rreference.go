package rreference

import (
	"context"
	"database/sql"
	"fmt"
	"the-dev-tools/backend/internal/api"
	"the-dev-tools/backend/internal/api/rworkspace"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/menv"
	"the-dev-tools/backend/pkg/model/mnode"
	"the-dev-tools/backend/pkg/model/mnode/mnrequest"
	"the-dev-tools/backend/pkg/model/mvar"
	"the-dev-tools/backend/pkg/permcheck"
	"the-dev-tools/backend/pkg/service/senv"
	"the-dev-tools/backend/pkg/service/sexampleresp"
	"the-dev-tools/backend/pkg/service/sexamplerespheader"
	"the-dev-tools/backend/pkg/service/sflow"
	"the-dev-tools/backend/pkg/service/snode"
	"the-dev-tools/backend/pkg/service/snoderequest"
	"the-dev-tools/backend/pkg/service/suser"
	"the-dev-tools/backend/pkg/service/svar"
	"the-dev-tools/backend/pkg/service/sworkspace"
	referencev1 "the-dev-tools/spec/dist/buf/go/reference/v1"
	"the-dev-tools/spec/dist/buf/go/reference/v1/referencev1connect"

	"connectrpc.com/connect"
)

type NodeServiceRPC struct {
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
	fs   sflow.FlowService
	fns  snode.NodeService
	frns snoderequest.NodeRequestService
}

func NewNodeServiceRPC(db *sql.DB, us suser.UserService, ws sworkspace.WorkspaceService,
	es senv.EnvService, vs svar.VarService,
	ers sexampleresp.ExampleRespService, erhs sexamplerespheader.ExampleRespHeaderService,
	fs sflow.FlowService, fns snode.NodeService, frns snoderequest.NodeRequestService,
) *NodeServiceRPC {
	return &NodeServiceRPC{
		DB: db,

		us: us,
		ws: ws,

		es: es,
		vs: vs,

		ers:  ers,
		erhs: erhs,

		fs:   fs,
		fns:  fns,
		frns: frns,
	}
}

func CreateService(srv *NodeServiceRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := referencev1connect.NewReferenceServiceHandler(srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (c *NodeServiceRPC) ReferenceGet(ctx context.Context, req *connect.Request[referencev1.ReferenceGetRequest]) (*connect.Response[referencev1.ReferenceGetResponse], error) {
	var Items []*referencev1.Reference

	var workspaceID, exampleID, nodeID *idwrap.IDWrap
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
		nodeID = &tempID
	}

	if workspaceID != nil {
		wsID := *workspaceID
		rpcErr := permcheck.CheckPerm(rworkspace.CheckOwnerWorkspace(ctx, c.us, wsID))
		if rpcErr != nil {
			return nil, rpcErr
		}
		envs, err := c.es.GetByWorkspace(ctx, wsID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		present := make(map[string][]menv.Env)
		envMap := make([]*referencev1.Reference, 0, len(envs))
		var allVars []mvar.Var

		for _, env := range envs {
			vars, err := c.vs.GetVariableByEnvID(ctx, env.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
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

			envRef := &referencev1.Reference{
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
		Items = append(Items, &referencev1.Reference{
			Key: &referencev1.ReferenceKey{
				Kind:  referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_GROUP,
				Group: &groupStr,
			},
			Kind: referencev1.ReferenceKind_REFERENCE_KIND_MAP,
			Map:  envMap,
		})
	}
	if exampleID != nil {
		exID := *exampleID
		resp, err := c.ers.GetExampleRespByExampleID(ctx, exID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		respHeaders, err := c.erhs.GetHeaderByRespID(ctx, resp.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		headersSub := make([]*referencev1.Reference, 0, len(respHeaders))
		for _, header := range respHeaders {
			headersSub = append(headersSub, &referencev1.Reference{
				Key: &referencev1.ReferenceKey{
					Kind: referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY,
					Key:  &header.HeaderKey,
				},
				Kind:  referencev1.ReferenceKind_REFERENCE_KIND_VALUE,
				Value: &header.Value,
			})
		}

		headerKey := "headers"
		Items = append(Items, &referencev1.Reference{
			Key: &referencev1.ReferenceKey{
				Kind: referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY,
				Key:  &headerKey,
			},
			Kind: referencev1.ReferenceKind_REFERENCE_KIND_MAP,
			Map:  headersSub,
		})

	}
	if nodeID != nil {
		node, err := c.fns.GetNode(ctx, *nodeID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		flowID := node.FlowID
		nodes, err := c.fns.GetNodesByFlowID(ctx, flowID)

		var reqNodeIDs []idwrap.IDWrap
		for _, n := range nodes {
			if n.NodeKind == mnode.NODE_KIND_REQUEST {
				reqNodeIDs = append(reqNodeIDs, n.ID)
			}
		}

		// Get All Request
		var reqs []mnrequest.MNRequest
		for _, reqNodeID := range reqNodeIDs {
			req, err := c.frns.GetNodeRequest(ctx, reqNodeID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			reqs = append(reqs, *req)
		}

		// Get All Responses

		var nodeRefs []*referencev1.Reference
		for _, req := range reqs {
			if req.ExampleID != nil {
				continue
			}
			resp, err := c.ers.GetExampleRespByExampleID(ctx, *req.ExampleID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			var headersSubRefs []*referencev1.Reference
			subRespHeaders, err := c.erhs.GetHeaderByRespID(ctx, resp.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			for _, header := range subRespHeaders {
				headersSubRefs = append(headersSubRefs, &referencev1.Reference{
					Key: &referencev1.ReferenceKey{
						Kind: referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY,
						Key:  &header.HeaderKey,
					},
					Kind:  referencev1.ReferenceKind_REFERENCE_KIND_VALUE,
					Value: &header.Value,
				})
			}

			flowNodeIDStr := req.FlowNodeID.String()
			nodeRefs = append(nodeRefs, &referencev1.Reference{
				Key: &referencev1.ReferenceKey{
					Kind: referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY,
					Key:  &flowNodeIDStr,
				},
				Kind: referencev1.ReferenceKind_REFERENCE_KIND_MAP,
				Map:  headersSubRefs,
			})
		}

		refGroupVarStr := "var"
		Items = append(Items, &referencev1.Reference{
			Key: &referencev1.ReferenceKey{
				Kind:  referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_GROUP,
				Group: &refGroupVarStr,
			},
			Kind: referencev1.ReferenceKind_REFERENCE_KIND_MAP,
			Map:  nodeRefs,
		})
		fmt.Println("nodeID")
	}

	response := &referencev1.ReferenceGetResponse{
		Items: Items,
	}
	return connect.NewResponse(response), nil
}
