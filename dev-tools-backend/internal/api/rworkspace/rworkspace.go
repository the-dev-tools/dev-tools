package rworkspace

import (
	"context"
	"database/sql"
	"dev-tools-backend/internal/api"
	"dev-tools-backend/internal/api/middleware/mwauth"
	"dev-tools-backend/pkg/model/mworkspace"
	"dev-tools-backend/pkg/model/mworkspaceuser"
	"dev-tools-backend/pkg/service/sworkspace"
	"dev-tools-backend/pkg/service/sworkspacesusers"
	workspacev1 "dev-tools-services/gen/workspace/v1"
	"dev-tools-services/gen/workspace/v1/workspacev1connect"
	"errors"

	"connectrpc.com/connect"
	"github.com/oklog/ulid/v2"
)

type WorkspaceService struct{}

func (c *WorkspaceService) GetWorkspace(ctx context.Context, req *connect.Request[workspacev1.GetWorkspaceRequest]) (*connect.Response[workspacev1.GetWorkspaceResponse], error) {
	orgID, err := ulid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user id not found"))
	}

	org, err := sworkspace.GetByIDandUserID(orgID, userID)
	if err != nil {
		if errors.Is(err, sworkspace.ErrOrgNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &workspacev1.GetWorkspaceResponse{
		Workspace: &workspacev1.Workspace{
			Id:   org.ID.String(),
			Name: org.Name,
		},
	}

	return connect.NewResponse(resp), nil
}

func (c *WorkspaceService) GetWorkspaces(ctx context.Context, req *connect.Request[workspacev1.GetWorkspacesRequest]) (*connect.Response[workspacev1.GetWorkspacesResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user id not found"))
	}

	workspaces, err := sworkspace.GetMultiByUserID(userID)
	if err != nil {
		if errors.Is(err, sworkspace.ErrOrgNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	respOrgs := make([]*workspacev1.Workspace, len(workspaces))
	for i, org := range workspaces {
		respOrgs[i] = &workspacev1.Workspace{
			Id:   org.ID.String(),
			Name: org.Name,
		}
	}

	resp := &workspacev1.GetWorkspacesResponse{
		Workspaces: respOrgs,
	}
	return connect.NewResponse(resp), nil
}

func (c *WorkspaceService) CreateWorkspace(ctx context.Context, req *connect.Request[workspacev1.CreateWorkspaceRequest]) (*connect.Response[workspacev1.CreateWorkspaceResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user id not found"))
	}

	ulidID := ulid.Make()

	org := &mworkspace.Workspace{
		ID:   ulidID,
		Name: req.Msg.Name,
	}

	// TODO: add transaction
	err = sworkspace.Create(org)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	orgUser := &mworkspaceuser.WorkspaceUser{
		ID:     ulid.Make(),
		OrgID:  org.ID,
		UserID: userID,
	}

	err = sworkspacesusers.CreateOrgUser(orgUser)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &workspacev1.CreateWorkspaceResponse{
		Workspace: &workspacev1.Workspace{
			Id:   org.ID.String(),
			Name: org.Name,
		},
	}
	return connect.NewResponse(resp), nil
}

func (c *WorkspaceService) UpdateWorkspace(ctx context.Context, req *connect.Request[workspacev1.UpdateWorkspaceRequest]) (*connect.Response[workspacev1.UpdateWorkspaceResponse], error) {
	userUlid, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user id not found"))
	}

	orgUlid, err := ulid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if req.Msg.Name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}

	org, err := sworkspace.GetByIDandUserID(orgUlid, userUlid)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("organization not found"))
		}
	}

	org.Name = req.Msg.Name
	err = sworkspace.Update(org)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&workspacev1.UpdateWorkspaceResponse{}), nil
}

func (c *WorkspaceService) DeleteWorkspace(ctx context.Context, req *connect.Request[workspacev1.DeleteWorkspaceRequest]) (*connect.Response[workspacev1.DeleteWorkspaceResponse], error) {
	userUlid, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user id not found"))
	}

	orgUlid, err := ulid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	org, err := sworkspace.GetByIDandUserID(orgUlid, userUlid)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("organization not found"))
		}
	}

	err = sworkspace.Delete(org.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&workspacev1.DeleteWorkspaceResponse{}), nil
}

func CreateService(secret []byte) (*api.Service, error) {
	AuthInterceptorFunc := mwauth.NewAuthInterceptor(secret)
	server := &WorkspaceService{}
	path, handler := workspacev1connect.NewWorkspaceServiceHandler(server, connect.WithInterceptors(AuthInterceptorFunc))
	return &api.Service{Path: path, Handler: handler}, nil
}
