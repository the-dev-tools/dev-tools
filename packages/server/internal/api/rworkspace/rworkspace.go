package rworkspace

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/model/mworkspaceuser"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/service/sworkspacesusers"
	"the-dev-tools/server/pkg/translate/tworkspace"
	workspacev1 "the-dev-tools/spec/dist/buf/go/workspace/v1"
	"the-dev-tools/spec/dist/buf/go/workspace/v1/workspacev1connect"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var ErrWorkspaceNotFound = errors.New("workspace not found")

type WorkspaceServiceRPC struct {
	DB  *sql.DB
	ws  sworkspace.WorkspaceService
	wus sworkspacesusers.WorkspaceUserService
	us  suser.UserService

	// env
	es senv.EnvService

	// TODO: @Ege move to private repo
	// email
	// ec  emailclient.EmailClient
	// eim *emailinvite.EmailTemplateManager
}

func New(db *sql.DB, ws sworkspace.WorkspaceService, wus sworkspacesusers.WorkspaceUserService, us suser.UserService, es senv.EnvService) WorkspaceServiceRPC {
	return WorkspaceServiceRPC{
		DB:  db,
		ws:  ws,
		wus: wus,
		us:  us,
		es:  es,
	}
}

func CreateService(srv WorkspaceServiceRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := workspacev1connect.NewWorkspaceServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (c *WorkspaceServiceRPC) GetWorkspace(ctx context.Context, req *connect.Request[workspacev1.WorkspaceGetRequest]) (*connect.Response[workspacev1.WorkspaceGetResponse], error) {
	wsID, err := idwrap.NewFromBytes(req.Msg.WorkspaceId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user id not found"))
	}

	ws, err := c.ws.GetByIDandUserID(ctx, wsID, userID)
	if err != nil {
		if errors.Is(err, sworkspace.ErrNoWorkspaceFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	env, err := c.es.Get(ctx, ws.ActiveEnv)
	if err != nil {
		if !errors.Is(err, senv.ErrNoEnvFound) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	resp := &workspacev1.WorkspaceGetResponse{
		WorkspaceId:           ws.ID.Bytes(),
		Name:                  ws.Name,
		Updated:               timestamppb.New(ws.Updated),
		SelectedEnvironmentId: env.ID.Bytes(),
	}

	return connect.NewResponse(resp), nil
}

func (c *WorkspaceServiceRPC) WorkspaceList(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[workspacev1.WorkspaceListResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user id not found"))
	}

	workspaces, err := c.ws.GetMultiByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, sworkspace.ErrNoWorkspaceFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var rpcWorkspaces []*workspacev1.WorkspaceListItem
	for _, workspace := range workspaces {
		env, err := c.es.Get(ctx, workspace.ActiveEnv)
		if err != nil {
			if !errors.Is(err, senv.ErrNoEnvFound) {
				return nil, err
			}
		}
		rpcworkspace := tworkspace.SeralizeWorkspaceItem(workspace, env)
		rpcWorkspaces = append(rpcWorkspaces, rpcworkspace)

	}
	resp := &workspacev1.WorkspaceListResponse{
		Items: rpcWorkspaces,
	}
	return connect.NewResponse(resp), nil
}

func (c *WorkspaceServiceRPC) WorkspaceGet(ctx context.Context, req *connect.Request[workspacev1.WorkspaceGetRequest]) (*connect.Response[workspacev1.WorkspaceGetResponse], error) {
	workspaceID, err := idwrap.NewFromBytes(req.Msg.GetWorkspaceId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user id not found"))
	}

	permc, err := c.us.CheckUserBelongsToWorkspace(ctx, userID, workspaceID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !permc {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("permission denied"))
	}

	ws, err := c.ws.Get(ctx, workspaceID)
	if err != nil {
		if errors.Is(err, sworkspace.ErrNoWorkspaceFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	env, err := c.es.Get(ctx, ws.ActiveEnv)
	if err != nil {
		if !errors.Is(err, senv.ErrNoEnvFound) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	rpcWs := tworkspace.SeralizeWorkspace(*ws, env)

	resp := &workspacev1.WorkspaceGetResponse{
		WorkspaceId:           rpcWs.WorkspaceId,
		Name:                  rpcWs.Name,
		Updated:               rpcWs.Updated,
		SelectedEnvironmentId: rpcWs.SelectedEnvironmentId,
		CollectionCount:       rpcWs.CollectionCount,
		FlowCount:             rpcWs.FlowCount,
	}
	return connect.NewResponse(resp), nil
}

func (c *WorkspaceServiceRPC) WorkspaceCreate(ctx context.Context, req *connect.Request[workspacev1.WorkspaceCreateRequest]) (*connect.Response[workspacev1.WorkspaceCreateResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user id not found"))
	}
	name := req.Msg.GetName()
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}

	envID := idwrap.NewNow()

	workspaceidWrap := idwrap.NewNow()
	ws := &mworkspace.Workspace{
		ID:        workspaceidWrap,
		Name:      name,
		Updated:   dbtime.DBNow(),
		ActiveEnv: envID,
		GlobalEnv: envID,
	}

	wsEnv := menv.Env{
		ID:          envID,
		WorkspaceID: workspaceidWrap,
		Name:        "default",
		Type:        menv.EnvGlobal,
	}

	wsUser := &mworkspaceuser.WorkspaceUser{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceidWrap,
		UserID:      userID,
		Role:        mworkspaceuser.RoleOwner,
	}

	tx, err := c.DB.Begin()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer func() {
		localErr := tx.Rollback()
		if localErr != nil {
			fmt.Println(localErr)
		}
	}()
	workspaceServiceTX, err := sworkspace.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = workspaceServiceTX.Create(ctx, ws)
	if err != nil {
		return nil, err
	}

	envServiceTX, err := senv.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = envServiceTX.Create(ctx, wsEnv)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	workspaceUserServiceTX, err := sworkspacesusers.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = workspaceUserServiceTX.CreateWorkspaceUser(ctx, wsUser)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	err = tx.Commit()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &workspacev1.WorkspaceCreateResponse{
		WorkspaceId: workspaceidWrap.Bytes(),
	}
	return connect.NewResponse(resp), nil
}

func (c *WorkspaceServiceRPC) WorkspaceUpdate(ctx context.Context, req *connect.Request[workspacev1.WorkspaceUpdateRequest]) (*connect.Response[workspacev1.WorkspaceUpdateResponse], error) {
	workspaceUlid, err := idwrap.NewFromBytes(req.Msg.GetWorkspaceId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	userUlid, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user id not found"))
	}
	wsUser, err := c.wus.GetWorkspaceUsersByWorkspaceIDAndUserID(ctx, workspaceUlid, userUlid)
	if err != nil {
		if errors.Is(err, sworkspacesusers.ErrWorkspaceUserNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("workspace not found"))
		}
	}
	// TODO: role system will change later
	if wsUser.Role < mworkspaceuser.RoleAdmin {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("permission denied"))
	}
	ws, err := c.ws.Get(ctx, workspaceUlid)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("workspace not found"))
		}
	}

	envID := req.Msg.SelectedEnvironmentId
	if len(envID) != 0 {
		tempEnvID, err := idwrap.NewFromBytes(envID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		ws.ActiveEnv = tempEnvID
	}

	name := req.Msg.GetName()
	if name != "" {
		ws.Name = name
	}
	err = c.ws.Update(ctx, ws)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&workspacev1.WorkspaceUpdateResponse{}), nil
}

func (c *WorkspaceServiceRPC) WorkspaceDelete(ctx context.Context, req *connect.Request[workspacev1.WorkspaceDeleteRequest]) (*connect.Response[workspacev1.WorkspaceDeleteResponse], error) {
	userUlid, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user id not found"))
	}

	workspaceUlid, err := idwrap.NewFromBytes(req.Msg.GetWorkspaceId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	wsUser, err := c.wus.GetWorkspaceUsersByWorkspaceIDAndUserID(ctx, workspaceUlid, userUlid)
	if err != nil {
		if errors.Is(err, sworkspacesusers.ErrWorkspaceUserNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("workspace not found"))
		}
	}

	if wsUser.Role != mworkspaceuser.RoleOwner {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("permission denied"))
	}

	err = c.ws.Delete(ctx, workspaceUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&workspacev1.WorkspaceDeleteResponse{}), nil
}

func (c *WorkspaceServiceRPC) WorkspaceMemberList(ctx context.Context, req *connect.Request[workspacev1.WorkspaceMemberListRequest]) (*connect.Response[workspacev1.WorkspaceMemberListResponse], error) {
	actionUserUlid, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user id not found"))
	}
	workspaceUlid, err := idwrap.NewFromBytes(req.Msg.GetWorkspaceId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	ok, err := c.us.CheckUserBelongsToWorkspace(ctx, actionUserUlid, workspaceUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !ok {
		// TODO: remove perm error for information leak
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("permission denied"))
	}
	wsUsers, err := c.wus.GetWorkspaceUserByWorkspaceID(ctx, workspaceUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	rpcUser := make([]*workspacev1.WorkspaceMemberListItem, len(wsUsers))
	for i, wsUser := range wsUsers {
		user, err := c.us.GetUser(ctx, wsUser.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		rpcUser[i] = &workspacev1.WorkspaceMemberListItem{
			UserID: wsUser.UserID.Bytes(),
			Email:  user.Email,
			Role:   workspacev1.MemberRole(wsUser.Role),
		}
	}

	return connect.NewResponse(&workspacev1.WorkspaceMemberListResponse{Items: rpcUser}), nil
}

func (c *WorkspaceServiceRPC) WorkspaceMemberCreate(ctx context.Context, req *connect.Request[workspacev1.WorkspaceMemberCreateRequest]) (*connect.Response[workspacev1.WorkspaceMemberCreateResponse], error) {
	// TODO: @Ege move to private repo
	// wid, err := idwrap.NewFromBytes(req.Msg.GetWorkspaceId())
	// if err != nil {
	// 	return nil, connect.NewError(connect.CodeInvalidArgument, err)
	// }
	// // check email
	// if req.Msg.GetEmail() == "" {
	// 	return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("email is required"))
	// }
	// // TODO: add more validation for email
	// userID, err := mwauth.GetContextUserID(ctx)
	// if err != nil {
	// 	return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user id not found"))
	// }

	// // check if workspace has the user
	// _, err = c.ws.GetByIDandUserID(ctx, wid, userID)
	// if err != nil {
	// 	if err == sql.ErrNoRows {
	// 		return nil, connect.NewError(connect.CodeNotFound, errors.New("workspace not found"))
	// 	}
	// 	return nil, connect.NewError(connect.CodeInternal, err)
	// }

	// inviterUser, err := c.us.GetUser(ctx, userID)
	// if err != nil {
	// 	return nil, connect.NewError(connect.CodeInternal, err)
	// }

	// invitedUser, err := c.us.GetUserByEmail(ctx, req.Msg.GetEmail())
	// if err != nil {
	// 	if errors.Is(err, sql.ErrNoRows) {
	// 		invitedUser = &muser.User{
	// 			ID:           idwrap.NewNow(),
	// 			Email:        req.Msg.GetEmail(),
	// 			Password:     nil,
	// 			ProviderType: muser.Unknown,
	// 			ProviderID:   nil,
	// 			Status:       muser.Pending,
	// 		}
	// 		err = c.us.CreateUser(ctx, invitedUser)
	// 	}
	// 	return nil, err
	// }

	// err = c.wus.CreateWorkspaceUser(ctx, &mworkspaceuser.WorkspaceUser{
	// 	ID:          idwrap.NewNow(),
	// 	WorkspaceID: wid,
	// 	UserID:      invitedUser.ID,
	// 	Role:        mworkspaceuser.RoleUser,
	// })
	// if err != nil {
	// 	return nil, connect.NewError(connect.CodeInternal, err)
	// }

	// workspace, err := c.ws.Get(ctx, wid)
	// if err != nil {
	// 	return nil, connect.NewError(connect.CodeInternal, err)
	// }

	// // TODO: @Ege move to private repo
	// EmailInviteTemplateData := &emailinvite.EmailInviteTemplateData{
	// 	WorkspaceName:     workspace.Name,
	// 	InviteLink:        "https://dev.tools",
	// 	InvitedByUsername: inviterUser.Email,
	// 	Username:          invitedUser.Email,
	// }

	// // TODO: add limit for sending email
	// err = c.eim.SendEmailInvite(ctx, req.Msg.GetEmail(), EmailInviteTemplateData)
	// if err != nil {
	// 	return nil, connect.NewError(connect.CodeInternal, err)
	// }

	// TODO: look into this with new spec member id return just not possible
	return connect.NewResponse(&workspacev1.WorkspaceMemberCreateResponse{}), nil
}

func (c *WorkspaceServiceRPC) WorkspaceMemberDelete(ctx context.Context, req *connect.Request[workspacev1.WorkspaceMemberDeleteRequest]) (*connect.Response[workspacev1.WorkspaceMemberDeleteResponse], error) {
	workspaceULID, err := idwrap.NewFromBytes(req.Msg.GetWorkspaceId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	actionUserUlid, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user id not found"))
	}
	targetUserUlid, err := idwrap.NewFromBytes(req.Msg.UserID)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user id not found"))
	}

	ActionUser, err := c.wus.GetWorkspaceUsersByWorkspaceIDAndUserID(ctx, workspaceULID, actionUserUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	TargetUser, err := c.wus.GetWorkspaceUsersByWorkspaceIDAndUserID(ctx, workspaceULID, targetUserUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	ok, err := sworkspacesusers.IsPermGreater(ActionUser, TargetUser)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !ok {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("permission denied"))
	}
	err = c.wus.DeleteWorkspaceUser(ctx, targetUserUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&workspacev1.WorkspaceMemberDeleteResponse{}), nil
}

func (c *WorkspaceServiceRPC) WorkspaceMemberUpdate(ctx context.Context, req *connect.Request[workspacev1.WorkspaceMemberUpdateRequest]) (*connect.Response[workspacev1.WorkspaceMemberUpdateResponse], error) {
	workspaceULID, err := idwrap.NewFromBytes(req.Msg.GetWorkspaceId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	actionUserUlid, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user id not found"))
	}
	targetUserUlid, err := idwrap.NewFromBytes(req.Msg.UserID)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user id not found"))
	}

	ActionUser, err := c.wus.GetWorkspaceUsersByWorkspaceIDAndUserID(ctx, workspaceULID, actionUserUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	TargetUser, err := c.wus.GetWorkspaceUsersByWorkspaceIDAndUserID(ctx, workspaceULID, targetUserUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	ok, err := sworkspacesusers.IsPermGreater(ActionUser, TargetUser)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !ok {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("permission denied"))
	}
	TargetUser.Role = mworkspaceuser.Role(req.Msg.GetRole())

	// TODO: add check for user role such bigger then enum etc
	err = c.wus.UpdateWorkspaceUser(ctx, TargetUser)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&workspacev1.WorkspaceMemberUpdateResponse{}), nil
}

func CheckOwnerWorkspace(ctx context.Context, su suser.UserService, workspaceID idwrap.IDWrap) (bool, error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return false, err
	}
	return su.CheckUserBelongsToWorkspace(ctx, userID, workspaceID)
}
