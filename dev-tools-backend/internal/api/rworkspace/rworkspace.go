package rworkspace

import (
	"context"
	"database/sql"
	"dev-tools-backend/internal/api"
	"dev-tools-backend/internal/api/middleware/mwauth"
	"dev-tools-backend/internal/api/middleware/mwcompress"
	"dev-tools-backend/pkg/dbtime"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/menv"
	"dev-tools-backend/pkg/model/muser"
	"dev-tools-backend/pkg/model/mworkspace"
	"dev-tools-backend/pkg/model/mworkspaceuser"
	"dev-tools-backend/pkg/service/senv"
	"dev-tools-backend/pkg/service/suser"
	"dev-tools-backend/pkg/service/sworkspace"
	"dev-tools-backend/pkg/service/sworkspacesusers"
	"dev-tools-backend/pkg/translate/tworkspace"
	"dev-tools-mail/pkg/emailclient"
	"dev-tools-mail/pkg/emailinvite"
	workspacev1 "dev-tools-services/gen/workspace/v1"
	"dev-tools-services/gen/workspace/v1/workspacev1connect"
	"errors"
	"log"
	"os"

	"connectrpc.com/connect"
)

var ErrWorkspaceNotFound = errors.New("workspace not found")

type WorkspaceServiceRPC struct {
	DB  *sql.DB
	sw  sworkspace.WorkspaceService
	swu sworkspacesusers.WorkspaceUserService
	su  suser.UserService

	// env
	es senv.EnvService

	// email
	ec  emailclient.EmailClient
	eim *emailinvite.EmailTemplateManager
}

func CreateService(ctx context.Context, secret []byte, db *sql.DB) (*api.Service, error) {
	AWS_ACCESS_KEY := os.Getenv("AWS_ACCESS_KEY")
	if AWS_ACCESS_KEY == "" {
		log.Fatalf("AWS_ACCESS_KEY is empty")
	}
	AWS_SECRET_KEY := os.Getenv("AWS_SECRET_KEY")
	if AWS_SECRET_KEY == "" {
		log.Fatalf("AWS_SECRET_KEY is empty")
	}

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

	es, err := senv.New(ctx, db)
	if err != nil {
		return nil, err
	}

	client, err := emailclient.NewClient(AWS_ACCESS_KEY, AWS_SECRET_KEY, "")
	if err != nil {
		log.Fatalf("failed to create email client: %v", err)
	}

	path := os.Getenv("EMAIL_INVITE_TEMPLATE_PATH")
	if path == "" {
		return nil, errors.New("EMAIL_INVITE_TEMPLATE_PATH env var is required")
	}
	emailInviteManager, err := emailinvite.NewEmailTemplateFile(path, client)
	if err != nil {
		return nil, err
	}

	var options []connect.HandlerOption
	options = append(options, connect.WithCompression("zstd", mwcompress.NewDecompress, mwcompress.NewCompress))
	options = append(options, connect.WithCompression("gzip", nil, nil))
	options = append(options, connect.WithInterceptors(mwauth.NewAuthInterceptor(secret)))
	server := &WorkspaceServiceRPC{
		DB:  db,
		sw:  *sw,
		swu: *swu,
		su:  *us,
		ec:  *client,
		eim: emailInviteManager,
		es:  es,
	}
	path, handler := workspacev1connect.NewWorkspaceServiceHandler(server, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (c *WorkspaceServiceRPC) GetWorkspace(ctx context.Context, req *connect.Request[workspacev1.GetWorkspaceRequest]) (*connect.Response[workspacev1.GetWorkspaceResponse], error) {
	orgID, err := idwrap.NewWithParse(req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user id not found"))
	}

	ws, err := c.sw.GetByIDandUserID(ctx, orgID, userID)
	if err != nil {
		if errors.Is(err, sworkspace.ErrNoWorkspaceFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	env, err := c.es.GetActiveByWorkspace(ctx, ws.ID)
	if err != nil {
		if !errors.Is(err, senv.ErrNoEnvFound) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	resp := &workspacev1.GetWorkspaceResponse{
		Workspace: tworkspace.SeralizeWorkspace(*ws, env),
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
		if errors.Is(err, sworkspace.ErrNoWorkspaceFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	respOrgs := make([]*workspacev1.Workspace, len(workspaces))
	for i, ws := range workspaces {
		env, err := c.es.GetActiveByWorkspace(ctx, ws.ID)
		if err != nil {
			if !errors.Is(err, senv.ErrNoEnvFound) {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		}
		respOrgs[i] = tworkspace.SeralizeWorkspace(ws, env)
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
	name := req.Msg.GetName()
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}

	workspaceUlid := idwrap.NewNow()
	ws := &mworkspace.Workspace{
		ID:      workspaceUlid,
		Name:    name,
		Updated: dbtime.DBNow(),
	}

	wsEnv := menv.Env{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceUlid,
		Name:        "default",
		Type:        menv.EnvGlobal,
	}

	orgUser := &mworkspaceuser.WorkspaceUser{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceUlid,
		UserID:      userID,
		Role:        mworkspaceuser.RoleOwner,
	}

	tx, err := c.DB.Begin()
	defer tx.Rollback()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
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
	err = workspaceUserServiceTX.CreateWorkspaceUser(ctx, orgUser)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	err = tx.Commit()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &workspacev1.CreateWorkspaceResponse{
		Workspace: tworkspace.SeralizeWorkspace(*ws, nil),
	}
	return connect.NewResponse(resp), nil
}

func (c *WorkspaceServiceRPC) UpdateWorkspace(ctx context.Context, req *connect.Request[workspacev1.UpdateWorkspaceRequest]) (*connect.Response[workspacev1.UpdateWorkspaceResponse], error) {
	workspaceUlid, err := idwrap.NewWithParse(req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if req.Msg.GetName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}
	userUlid, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user id not found"))
	}
	wsUser, err := c.swu.GetWorkspaceUsersByWorkspaceIDAndUserID(ctx, workspaceUlid, userUlid)
	if err != nil {
		if errors.Is(err, sworkspacesusers.ErrWorkspaceUserNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("workspace not found"))
		}
	}
	// TODO: role system will change later
	if wsUser.Role < mworkspaceuser.RoleAdmin {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("permission denied"))
	}
	ws, err := c.sw.Get(ctx, workspaceUlid)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("workspace not found"))
		}
	}

	ws.Name = req.Msg.GetName()
	err = c.sw.Update(ctx, ws)
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

	workspaceUlid, err := idwrap.NewWithParse(req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	wsUser, err := c.swu.GetWorkspaceUsersByWorkspaceIDAndUserID(ctx, workspaceUlid, userUlid)
	if err != nil {
		if errors.Is(err, sworkspacesusers.ErrWorkspaceUserNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("workspace not found"))
		}
	}

	if wsUser.Role != mworkspaceuser.RoleOwner {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("permission denied"))
	}

	err = c.sw.Delete(ctx, workspaceUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&workspacev1.DeleteWorkspaceResponse{}), nil
}

func (c *WorkspaceServiceRPC) ListUsers(ctx context.Context, req *connect.Request[workspacev1.ListUsersRequest]) (*connect.Response[workspacev1.ListUsersResponse], error) {
	actionUserUlid, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user id not found"))
	}
	workspaceUlid, err := idwrap.NewWithParse(req.Msg.GetWorkspaceId())
	if err != nil {
	}
	ok, err := c.su.CheckUserBelongsToWorkspace(ctx, actionUserUlid, workspaceUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !ok {
		// TODO: remove perm error for information leak
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("permission denied"))
	}
	wsUsers, err := c.swu.GetWorkspaceUserByWorkspaceID(ctx, workspaceUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	rpcUser := make([]*workspacev1.User, len(wsUsers))
	for i, wsUser := range wsUsers {
		user, err := c.su.GetUser(ctx, wsUser.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		rpcUser[i] = &workspacev1.User{
			Id:    wsUser.UserID.String(),
			Email: user.Email,
			Role:  workspacev1.Role(wsUser.Role),
		}
	}

	return connect.NewResponse(&workspacev1.ListUsersResponse{Users: rpcUser}), nil
}

// TODO: I'm not sure this is the correct implementation of this function
// Will talk with the team about this on the next meeting
func (c *WorkspaceServiceRPC) InviteUser(ctx context.Context, req *connect.Request[workspacev1.InviteUserRequest]) (*connect.Response[workspacev1.InviteUserResponse], error) {
	wid, err := idwrap.NewWithParse(req.Msg.GetWorkspaceId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	// check email
	if req.Msg.GetEmail() == "" {
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
			return nil, connect.NewError(connect.CodeNotFound, errors.New("workspace not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	inviterUser, err := c.su.GetUser(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	invitedUser, err := c.su.GetUserByEmail(ctx, req.Msg.GetEmail())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			invitedUser, err = c.su.CreateUser(ctx, &muser.User{
				ID:           idwrap.NewNow(),
				Email:        req.Msg.GetEmail(),
				Password:     nil,
				ProviderType: muser.Unknown,
				ProviderID:   nil,
				Status:       muser.Pending,
			})
		}
		return nil, err
	}

	err = c.swu.CreateWorkspaceUser(ctx, &mworkspaceuser.WorkspaceUser{
		ID:          idwrap.NewNow(),
		WorkspaceID: wid,
		UserID:      invitedUser.ID,
		Role:        mworkspaceuser.RoleUser,
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

	// TODO: add limit for sending email
	err = c.eim.SendEmailInvite(ctx, req.Msg.GetEmail(), EmailInviteTemplateData)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&workspacev1.InviteUserResponse{
		UserId: invitedUser.ID.String(),
	}), nil
}

func (c *WorkspaceServiceRPC) RemoveUser(ctx context.Context, req *connect.Request[workspacev1.RemoveUserRequest]) (*connect.Response[workspacev1.RemoveUserResponse], error) {
	workspaceULID, err := idwrap.NewWithParse(req.Msg.GetWorkspaceId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	actionUserUlid, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user id not found"))
	}
	targetUserUlid, err := idwrap.NewWithParse(req.Msg.GetUserId())
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user id not found"))
	}

	ActionUser, err := c.swu.GetWorkspaceUsersByWorkspaceIDAndUserID(ctx, workspaceULID, actionUserUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	TargetUser, err := c.swu.GetWorkspaceUsersByWorkspaceIDAndUserID(ctx, workspaceULID, targetUserUlid)
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
	err = c.swu.DeleteWorkspaceUser(ctx, targetUserUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&workspacev1.RemoveUserResponse{}), nil
}

func (c *WorkspaceServiceRPC) UpdateUserRole(ctx context.Context, req *connect.Request[workspacev1.UpdateUserRoleRequest]) (*connect.Response[workspacev1.UpdateUserRoleResponse], error) {
	workspaceULID, err := idwrap.NewWithParse(req.Msg.GetWorkspaceId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	actionUserUlid, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user id not found"))
	}
	targetUserUlid, err := idwrap.NewWithParse(req.Msg.GetUserId())
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user id not found"))
	}

	ActionUser, err := c.swu.GetWorkspaceUsersByWorkspaceIDAndUserID(ctx, workspaceULID, actionUserUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	TargetUser, err := c.swu.GetWorkspaceUsersByWorkspaceIDAndUserID(ctx, workspaceULID, targetUserUlid)
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
	err = c.swu.UpdateWorkspaceUser(ctx, TargetUser)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&workspacev1.UpdateUserRoleResponse{}), nil
}

func CheckOwnerWorkspace(ctx context.Context, su suser.UserService, workspaceID idwrap.IDWrap) (bool, error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return false, err
	}
	return su.CheckUserBelongsToWorkspace(ctx, userID, workspaceID)
}
