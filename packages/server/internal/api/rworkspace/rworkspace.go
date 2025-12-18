//nolint:revive // exported
package rworkspace

import (
	"context"
	"database/sql"
	"errors"
	"sync"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	devtoolsdb "the-dev-tools/db"
	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/eventstream"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/model/mworkspaceuser"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/service/sworkspacesusers"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/workspace/v1"
	"the-dev-tools/spec/dist/buf/go/api/workspace/v1/workspacev1connect"
)

const (
	eventTypeInsert = "insert"
	eventTypeUpdate = "update"
	eventTypeDelete = "delete"
)

type WorkspaceTopic struct {
	WorkspaceID idwrap.IDWrap
}

type WorkspaceEvent struct {
	Type      string
	Workspace *apiv1.Workspace
}

type WorkspaceServiceRPC struct {
	DB *sql.DB

	ws  sworkspace.WorkspaceService
	wus sworkspacesusers.WorkspaceUserService
	us  suser.UserService
	es  senv.EnvService

	stream eventstream.SyncStreamer[WorkspaceTopic, WorkspaceEvent]
}

func New(
	db *sql.DB,
	ws sworkspace.WorkspaceService,
	wus sworkspacesusers.WorkspaceUserService,
	us suser.UserService,
	es senv.EnvService,
	stream eventstream.SyncStreamer[WorkspaceTopic, WorkspaceEvent],
) WorkspaceServiceRPC {
	return WorkspaceServiceRPC{
		DB:     db,
		ws:     ws,
		wus:    wus,
		us:     us,
		es:     es,
		stream: stream,
	}
}

func CreateService(srv WorkspaceServiceRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := workspacev1connect.NewWorkspaceServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func stringPtr(s string) *string { return &s }

func float32Ptr(f float32) *float32 { return &f }

func workspaceUpdatedUnion(ts *timestamppb.Timestamp) *apiv1.WorkspaceSyncUpdate_UpdatedUnion {
	if ts == nil {
		return nil
	}
	return &apiv1.WorkspaceSyncUpdate_UpdatedUnion{
		Kind:  apiv1.WorkspaceSyncUpdate_UpdatedUnion_KIND_VALUE,
		Value: ts,
	}
}

func toAPIWorkspace(ws mworkspace.Workspace) *apiv1.Workspace {
	apiWorkspace := &apiv1.Workspace{
		WorkspaceId:           ws.ID.Bytes(),
		SelectedEnvironmentId: ws.ActiveEnv.Bytes(),
		Name:                  ws.Name,
		Order:                 float32(ws.Order),
	}
	if !ws.Updated.IsZero() {
		apiWorkspace.Updated = timestamppb.New(ws.Updated)
	}
	return apiWorkspace
}

func workspaceSyncResponseFrom(evt WorkspaceEvent) *apiv1.WorkspaceSyncResponse {
	if evt.Workspace == nil {
		return nil
	}

	switch evt.Type {
	case eventTypeInsert:
		msg := &apiv1.WorkspaceSync{
			Value: &apiv1.WorkspaceSync_ValueUnion{
				Kind: apiv1.WorkspaceSync_ValueUnion_KIND_INSERT,
				Insert: &apiv1.WorkspaceSyncInsert{
					WorkspaceId:           evt.Workspace.WorkspaceId,
					SelectedEnvironmentId: evt.Workspace.SelectedEnvironmentId,
					Name:                  evt.Workspace.Name,
					Updated:               evt.Workspace.Updated,
					Order:                 evt.Workspace.Order,
				},
			},
		}
		return &apiv1.WorkspaceSyncResponse{Items: []*apiv1.WorkspaceSync{msg}}
	case eventTypeUpdate:
		update := &apiv1.WorkspaceSyncUpdate{
			WorkspaceId: evt.Workspace.WorkspaceId,
			Name:        stringPtr(evt.Workspace.Name),
			Order:       float32Ptr(evt.Workspace.Order),
			Updated:     workspaceUpdatedUnion(evt.Workspace.Updated),
		}
		if len(evt.Workspace.SelectedEnvironmentId) > 0 {
			update.SelectedEnvironmentId = evt.Workspace.SelectedEnvironmentId
		}
		msg := &apiv1.WorkspaceSync{
			Value: &apiv1.WorkspaceSync_ValueUnion{
				Kind:   apiv1.WorkspaceSync_ValueUnion_KIND_UPDATE,
				Update: update,
			},
		}
		return &apiv1.WorkspaceSyncResponse{Items: []*apiv1.WorkspaceSync{msg}}
	case eventTypeDelete:
		msg := &apiv1.WorkspaceSync{
			Value: &apiv1.WorkspaceSync_ValueUnion{
				Kind: apiv1.WorkspaceSync_ValueUnion_KIND_DELETE,
				Delete: &apiv1.WorkspaceSyncDelete{
					WorkspaceId: evt.Workspace.WorkspaceId,
				},
			},
		}
		return &apiv1.WorkspaceSyncResponse{Items: []*apiv1.WorkspaceSync{msg}}
	default:
		return nil
	}
}

func (c *WorkspaceServiceRPC) listUserWorkspaces(ctx context.Context, userID idwrap.IDWrap) ([]mworkspace.Workspace, error) {
	workspaces, err := c.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		if errors.Is(err, sworkspace.ErrNoWorkspaceFound) {
			return nil, nil
		}
		return nil, err
	}
	return workspaces, nil
}

func (c *WorkspaceServiceRPC) WorkspaceCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.WorkspaceCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	ordered, err := c.listUserWorkspaces(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	items := make([]*apiv1.Workspace, 0, len(ordered))
	for _, item := range ordered {
		items = append(items, toAPIWorkspace(item))
	}

	return connect.NewResponse(&apiv1.WorkspaceCollectionResponse{Items: items}), nil
}

func (c *WorkspaceServiceRPC) WorkspaceInsert(ctx context.Context, req *connect.Request[apiv1.WorkspaceInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one workspace must be provided"))
	}

	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	tx, err := c.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	wsWriter := sworkspace.NewWriter(tx)
	wusWriter := sworkspacesusers.NewWriter(tx)
	envWriter := senv.NewWriter(tx)

	var createdIDs []idwrap.IDWrap

	for _, item := range req.Msg.Items {
		workspaceID := idwrap.NewNow()
		if len(item.WorkspaceId) > 0 {
			workspaceID, err = idwrap.NewFromBytes(item.WorkspaceId)
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, err)
			}
		}

		name := item.GetName()
		if name == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
		}

		envID := idwrap.NewNow()
		if len(item.SelectedEnvironmentId) > 0 {
			envID, err = idwrap.NewFromBytes(item.SelectedEnvironmentId)
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, err)
			}
		}

		ws := &mworkspace.Workspace{
			ID:        workspaceID,
			Name:      name,
			Updated:   dbtime.DBNow(),
			ActiveEnv: envID,
			GlobalEnv: envID,
			Order:     float64(item.Order),
		}

		if err := wsWriter.Create(ctx, ws); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		defaultEnv := menv.Env{
			ID:          envID,
			WorkspaceID: workspaceID,
			Name:        "default",
			Type:        menv.EnvGlobal,
		}
		if err := envWriter.CreateEnvironment(ctx, &defaultEnv); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		workspaceUser := &mworkspaceuser.WorkspaceUser{
			ID:          idwrap.NewNow(),
			WorkspaceID: workspaceID,
			UserID:      userID,
			Role:        mworkspaceuser.RoleOwner,
		}
		if err := wusWriter.CreateWorkspaceUser(ctx, workspaceUser); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		createdIDs = append(createdIDs, workspaceID)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for _, workspaceID := range createdIDs {
		ws, err := c.ws.Get(ctx, workspaceID)
		if err != nil {
			continue
		}
		c.stream.Publish(WorkspaceTopic{WorkspaceID: workspaceID}, WorkspaceEvent{
			Type:      eventTypeInsert,
			Workspace: toAPIWorkspace(*ws),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (c *WorkspaceServiceRPC) WorkspaceUpdate(ctx context.Context, req *connect.Request[apiv1.WorkspaceUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one workspace must be provided"))
	}

	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	tx, err := c.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	wsWriter := sworkspace.NewWriter(tx)

	var updatedIDs []idwrap.IDWrap

	for _, item := range req.Msg.Items {
		if len(item.WorkspaceId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id is required"))
		}

		workspaceID, err := idwrap.NewFromBytes(item.WorkspaceId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		wsUser, err := c.wus.GetWorkspaceUsersByWorkspaceIDAndUserID(ctx, workspaceID, userID)
		if err != nil {
			if errors.Is(err, sworkspacesusers.ErrWorkspaceUserNotFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if wsUser.Role < mworkspaceuser.RoleAdmin {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New("permission denied"))
		}

		ws, err := c.ws.Get(ctx, workspaceID)
		if err != nil {
			if errors.Is(err, sworkspace.ErrNoWorkspaceFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if len(item.SelectedEnvironmentId) > 0 {
			newEnvID, err := idwrap.NewFromBytes(item.SelectedEnvironmentId)
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, err)
			}
			ws.ActiveEnv = newEnvID
		}

		if item.Name != nil {
			ws.Name = *item.Name
		}

		if item.Order != nil {
			ws.Order = float64(*item.Order)
		}

		ws.Updated = dbtime.DBNow()

		if err := wsWriter.Update(ctx, ws); err != nil {
			if errors.Is(err, sworkspace.ErrNoWorkspaceFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		updatedIDs = append(updatedIDs, workspaceID)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for _, workspaceID := range updatedIDs {
		ws, err := c.ws.Get(ctx, workspaceID)
		if err != nil {
			continue
		}
		c.stream.Publish(WorkspaceTopic{WorkspaceID: workspaceID}, WorkspaceEvent{
			Type:      eventTypeUpdate,
			Workspace: toAPIWorkspace(*ws),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (c *WorkspaceServiceRPC) WorkspaceDelete(ctx context.Context, req *connect.Request[apiv1.WorkspaceDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one workspace must be provided"))
	}

	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	tx, err := c.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	wsWriter := sworkspace.NewWriter(tx)

	var deletedIDs []idwrap.IDWrap

	for _, item := range req.Msg.Items {
		if len(item.WorkspaceId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id is required"))
		}

		workspaceID, err := idwrap.NewFromBytes(item.WorkspaceId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		wsUser, err := c.wus.GetWorkspaceUsersByWorkspaceIDAndUserID(ctx, workspaceID, userID)
		if err != nil {
			if errors.Is(err, sworkspacesusers.ErrWorkspaceUserNotFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if wsUser.Role != mworkspaceuser.RoleOwner {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New("permission denied"))
		}

		if err := wsWriter.Delete(ctx, workspaceID); err != nil {
			if errors.Is(err, sworkspace.ErrNoWorkspaceFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		deletedIDs = append(deletedIDs, workspaceID)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for _, workspaceID := range deletedIDs {
		c.stream.Publish(WorkspaceTopic{WorkspaceID: workspaceID}, WorkspaceEvent{
			Type: eventTypeDelete,
			Workspace: &apiv1.Workspace{
				WorkspaceId: workspaceID.Bytes(),
			},
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (c *WorkspaceServiceRPC) WorkspaceSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.WorkspaceSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return c.streamWorkspaceSync(ctx, userID, stream.Send)
}

func (c *WorkspaceServiceRPC) streamWorkspaceSync(ctx context.Context, userID idwrap.IDWrap, send func(*apiv1.WorkspaceSyncResponse) error) error {
	var workspaceSet sync.Map

	snapshot := func(ctx context.Context) ([]eventstream.Event[WorkspaceTopic, WorkspaceEvent], error) {
		ordered, err := c.listUserWorkspaces(ctx, userID)
		if err != nil {
			return nil, err
		}

		events := make([]eventstream.Event[WorkspaceTopic, WorkspaceEvent], 0, len(ordered))
		for _, item := range ordered {
			workspaceSet.Store(item.ID.String(), struct{}{})
			events = append(events, eventstream.Event[WorkspaceTopic, WorkspaceEvent]{
				Topic: WorkspaceTopic{WorkspaceID: item.ID},
				Payload: WorkspaceEvent{
					Type:      eventTypeInsert,
					Workspace: toAPIWorkspace(item),
				},
			})
		}
		return events, nil
	}

	filter := func(topic WorkspaceTopic) bool {
		if _, ok := workspaceSet.Load(topic.WorkspaceID.String()); ok {
			return true
		}
		belongs, err := c.us.CheckUserBelongsToWorkspace(ctx, userID, topic.WorkspaceID)
		if err != nil || !belongs {
			return false
		}
		workspaceSet.Store(topic.WorkspaceID.String(), struct{}{})
		return true
	}

	events, err := c.stream.Subscribe(ctx, filter, eventstream.WithSnapshot(snapshot))
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	for {
		select {
		case evt, ok := <-events:
			if !ok {
				return nil
			}
			resp := workspaceSyncResponseFrom(evt.Payload)
			if resp == nil {
				continue
			}
			if err := send(resp); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func CheckOwnerWorkspace(ctx context.Context, su suser.UserService, workspaceID idwrap.IDWrap) (bool, error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return false, err
	}
	return su.CheckUserBelongsToWorkspace(ctx, userID, workspaceID)
}

func CheckOwnerWorkspaceWithReader(ctx context.Context, su *suser.Reader, workspaceID idwrap.IDWrap) (bool, error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return false, err
	}
	return su.CheckUserBelongsToWorkspace(ctx, userID, workspaceID)
}
