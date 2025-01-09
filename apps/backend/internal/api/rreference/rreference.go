package rreference

import (
	"context"
	"database/sql"
	"the-dev-tools/backend/internal/api"
	"the-dev-tools/backend/internal/api/rworkspace"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/permcheck"
	"the-dev-tools/backend/pkg/service/senv"
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
}

func NewNodeServiceRPC(db *sql.DB, us suser.UserService, ws sworkspace.WorkspaceService,
	es senv.EnvService, vs svar.VarService,
) *NodeServiceRPC {
	return &NodeServiceRPC{
		DB: db,

		us: us,
		ws: ws,

		es: es,
		vs: vs,
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
		var envNames []string

		for _, env := range envs {
			envNames = append(envNames, env.Name)
		}

		Items = append(Items, &referencev1.Reference{
			Variable: envNames,
		})
	}
	if exampleID != nil {
	}
	if nodeID != nil {
	}

	response := &referencev1.ReferenceGetResponse{
		Items: Items,
	}
	return connect.NewResponse(response), nil
}
