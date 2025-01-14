package rreference

import (
	"context"
	"database/sql"
	"fmt"
	"the-dev-tools/backend/internal/api"
	"the-dev-tools/backend/internal/api/rworkspace"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/menv"
	"the-dev-tools/backend/pkg/model/mvar"
	"the-dev-tools/backend/pkg/permcheck"
	"the-dev-tools/backend/pkg/service/senv"
	"the-dev-tools/backend/pkg/service/sexampleresp"
	"the-dev-tools/backend/pkg/service/sexamplerespheader"
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

	es senv.EnvService
	vs svar.VarService

	ers  sexampleresp.ExampleRespService
	erhs sexamplerespheader.ExampleRespHeaderService
}

func NewNodeServiceRPC(db *sql.DB, us suser.UserService, ws sworkspace.WorkspaceService,
	es senv.EnvService, vs svar.VarService,
	ers sexampleresp.ExampleRespService, erhs sexamplerespheader.ExampleRespHeaderService,
) *NodeServiceRPC {
	return &NodeServiceRPC{
		DB: db,

		us: us,
		ws: ws,

		es: es,
		vs: vs,

		ers:  ers,
		erhs: erhs,
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
					Key:  v.VarKey,
				},
				Kind:     referencev1.ReferenceKind_REFERENCE_KIND_VARIABLE,
				Variable: containsEnv,
			}
			envMap = append(envMap, envRef)
		}

		Items = append(Items, &referencev1.Reference{
			Key: &referencev1.ReferenceKey{
				Kind:  referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_GROUP,
				Group: "env",
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
					Key:  header.HeaderKey,
				},
				Kind:  referencev1.ReferenceKind_REFERENCE_KIND_VALUE,
				Value: header.Value,
			})
		}

		Items = append(Items, &referencev1.Reference{
			Key: &referencev1.ReferenceKey{
				Kind: referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY,
				Key:  "headers",
			},
			Kind: referencev1.ReferenceKind_REFERENCE_KIND_MAP,
			Map:  headersSub,
		})

		fmt.Println("exampleID")
	}
	if nodeID != nil {
		fmt.Println("nodeID")
	}

	response := &referencev1.ReferenceGetResponse{
		Items: Items,
	}
	return connect.NewResponse(response), nil
}
