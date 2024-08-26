package rworkspace

import (
	"context"
	"database/sql"
	"dev-tools-backend/internal/api"
	"dev-tools-backend/internal/api/middleware/mwauth"
	"dev-tools-backend/pkg/model/muser"
	"dev-tools-backend/pkg/model/mworkspace"
	"dev-tools-backend/pkg/model/mworkspaceuser"
	"dev-tools-backend/pkg/service/suser"
	"dev-tools-backend/pkg/service/sworkspace"
	"dev-tools-backend/pkg/service/sworkspacesusers"
	"dev-tools-mail/pkg/emailclient"
	"dev-tools-mail/pkg/emailinvite"
	workspacev1 "dev-tools-services/gen/workspace/v1"
	"dev-tools-services/gen/workspace/v1/workspacev1connect"
	"errors"

	"connectrpc.com/connect"
	"github.com/oklog/ulid/v2"
)

type WorkspaceServiceRPC struct {
	sw  sworkspace.WorkspaceService
	swu sworkspacesusers.WorkspaceUserService
	su  suser.UserService
	ec  emailclient.EmailClient
}

func (c *WorkspaceServiceRPC) GetWorkspace(ctx context.Context, req *connect.Request[workspacev1.GetWorkspaceRequest]) (*connect.Response[workspacev1.GetWorkspaceResponse], error) {
	orgID, err := ulid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user id not found"))
	}

	org, err := c.sw.GetByIDandUserID(ctx, orgID, userID)
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

func (c *WorkspaceServiceRPC) GetWorkspaces(ctx context.Context, req *connect.Request[workspacev1.GetWorkspacesRequest]) (*connect.Response[workspacev1.GetWorkspacesResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user id not found"))
	}

	workspaces, err := c.sw.GetMultiByUserID(ctx, userID)
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

func (c *WorkspaceServiceRPC) CreateWorkspace(ctx context.Context, req *connect.Request[workspacev1.CreateWorkspaceRequest]) (*connect.Response[workspacev1.CreateWorkspaceResponse], error) {
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
	err = c.sw.Create(ctx, org)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	orgUser := &mworkspaceuser.WorkspaceUser{
		ID:          ulid.Make(),
		WorkspaceID: org.ID,
		UserID:      userID,
	}

	err = c.swu.CreateWorkspaceUser(ctx, orgUser)
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

func (c *WorkspaceServiceRPC) UpdateWorkspace(ctx context.Context, req *connect.Request[workspacev1.UpdateWorkspaceRequest]) (*connect.Response[workspacev1.UpdateWorkspaceResponse], error) {
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

	org, err := c.sw.GetByIDandUserID(ctx, orgUlid, userUlid)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("organization not found"))
		}
	}

	org.Name = req.Msg.Name
	err = c.sw.Update(ctx, org)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&workspacev1.UpdateWorkspaceResponse{}), nil
}

func (c *WorkspaceServiceRPC) DeleteWorkspace(ctx context.Context, req *connect.Request[workspacev1.DeleteWorkspaceRequest]) (*connect.Response[workspacev1.DeleteWorkspaceResponse], error) {
	userUlid, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user id not found"))
	}

	orgUlid, err := ulid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	org, err := c.sw.GetByIDandUserID(ctx, orgUlid, userUlid)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("organization not found"))
		}
	}

	err = c.sw.Delete(ctx, org.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&workspacev1.DeleteWorkspaceResponse{}), nil
}

// TODO: I'm not sure this is the correct implementation of this function
// Will talk with the team about this on the next meeting
func (c *WorkspaceServiceRPC) InviteUser(ctx context.Context, req *connect.Request[workspacev1.InviteUserRequest]) (*connect.Response[workspacev1.InviteUserResponse], error) {
	wid, err := ulid.Parse(req.Msg.WorkspaceId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	// check email
	if req.Msg.Email == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("email is required"))
	}
	// TODO: add more validation for email
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user id not found"))
	}

	// check if workspace has the user
	_, err = c.sw.GetByIDandUserID(ctx, wid, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("organization not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	inviterUser, err := c.su.GetUser(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	invitedUserUlid := ulid.Make()
	invitedUser, err := c.su.CreateUser(ctx, &muser.User{
		ID:           invitedUserUlid,
		Email:        req.Msg.Email,
		Password:     nil,
		ProviderType: muser.Unknown,
		ProviderID:   nil,
		Status:       muser.Pending,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	workspace, err := c.sw.Get(ctx, wid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	EmailInviteTemplateData := &emailinvite.EmailInviteTemplateData{
		WorkspaceName:     workspace.Name,
		InviteLink:        "https://dev.tools",
		InvitedByUsername: inviterUser.Email,
		Username:          invitedUser.Email,
	}

	err = emailinvite.SendEmailInvite(ctx, c.ec, req.Msg.Email, EmailInviteTemplateData)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&workspacev1.InviteUserResponse{}), nil
}

func CreateService(secret []byte, db *sql.DB) (*api.Service, error) {
	ctx := context.Background()
	sw, err := sworkspace.New(ctx, db)
	if err != nil {
		return nil, err
	}
	swu, err := sworkspacesusers.New(ctx, db)
	if err != nil {
		return nil, err
	}

	us, err := suser.New(ctx, db)
	if err != nil {
		return nil, err
	}

	AuthInterceptorFunc := mwauth.NewAuthInterceptor(secret)
	server := &WorkspaceServiceRPC{
		sw:  *sw,
		swu: *swu,
		su:  *us,
	}
	path, handler := workspacev1connect.NewWorkspaceServiceHandler(server, connect.WithInterceptors(AuthInterceptorFunc))
	return &api.Service{Path: path, Handler: handler}, nil
}
