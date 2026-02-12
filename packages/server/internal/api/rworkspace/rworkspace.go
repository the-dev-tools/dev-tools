//nolint:revive // exported
package rworkspace

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	devtoolsdb "github.com/the-dev-tools/dev-tools/packages/db"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/renv"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/converter"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/dbtime"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/menv"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/mutation"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/senv"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/suser"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
	apiv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/workspace/v1"
	"github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/workspace/v1/workspacev1connect"
)

var ErrWorkspaceNotFound = errors.New("workspace not found")

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
	wus sworkspace.UserService
	us  suser.UserService
	es  senv.EnvService

	wsReader   *sworkspace.WorkspaceReader
	userReader *sworkspace.UserReader

	stream    eventstream.SyncStreamer[WorkspaceTopic, WorkspaceEvent]
	envStream eventstream.SyncStreamer[renv.EnvironmentTopic, renv.EnvironmentEvent]
	publisher mutation.Publisher // Unified publisher for cascade delete events
}

type WorkspaceServiceRPCServices struct {
	Workspace     sworkspace.WorkspaceService
	WorkspaceUser sworkspace.UserService
	User          suser.UserService
	Env           senv.EnvService
}

func (s *WorkspaceServiceRPCServices) Validate() error {
	return nil
}

type WorkspaceServiceRPCReaders struct {
	Workspace *sworkspace.WorkspaceReader
	User      *sworkspace.UserReader
}

func (r *WorkspaceServiceRPCReaders) Validate() error {
	if r.Workspace == nil {
		return fmt.Errorf("workspace reader is required")
	}
	if r.User == nil {
		return fmt.Errorf("user reader is required")
	}
	return nil
}

type WorkspaceServiceRPCStreamers struct {
	Workspace   eventstream.SyncStreamer[WorkspaceTopic, WorkspaceEvent]
	Environment eventstream.SyncStreamer[renv.EnvironmentTopic, renv.EnvironmentEvent]
}

func (s *WorkspaceServiceRPCStreamers) Validate() error {
	if s.Workspace == nil {
		return fmt.Errorf("workspace stream is required")
	}
	if s.Environment == nil {
		return fmt.Errorf("environment stream is required")
	}
	return nil
}

type WorkspaceServiceRPCDeps struct {
	DB        *sql.DB
	Services  WorkspaceServiceRPCServices
	Readers   WorkspaceServiceRPCReaders
	Streamers WorkspaceServiceRPCStreamers
	Publisher mutation.Publisher // Unified publisher for cascade delete events
}

func (d *WorkspaceServiceRPCDeps) Validate() error {
	if d.DB == nil {
		return fmt.Errorf("db is required")
	}
	if err := d.Services.Validate(); err != nil {
		return err
	}
	if err := d.Readers.Validate(); err != nil {
		return err
	}
	if err := d.Streamers.Validate(); err != nil {
		return err
	}
	return nil
}

func New(deps WorkspaceServiceRPCDeps) WorkspaceServiceRPC {
	if err := deps.Validate(); err != nil {
		panic(fmt.Sprintf("WorkspaceServiceRPC Deps validation failed: %v", err))
	}

	return WorkspaceServiceRPC{
		DB:         deps.DB,
		ws:         deps.Services.Workspace,
		wus:        deps.Services.WorkspaceUser,
		us:         deps.Services.User,
		es:         deps.Services.Env,
		wsReader:   deps.Readers.Workspace,
		userReader: deps.Readers.User,
		stream:     deps.Streamers.Workspace,
		envStream:  deps.Streamers.Environment,
		publisher:  deps.Publisher,
	}
}

func CreateService(srv WorkspaceServiceRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := workspacev1connect.NewWorkspaceServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func stringPtr(s string) *string { return &s }

func float32Ptr(f float32) *float32 { return &f }

func boolPtr(b bool) *bool { return &b }

func workspaceUpdatedUnion(ts *timestamppb.Timestamp) *apiv1.WorkspaceSyncUpdate_UpdatedUnion {
	if ts == nil {
		return nil
	}
	return &apiv1.WorkspaceSyncUpdate_UpdatedUnion{
		Kind:  apiv1.WorkspaceSyncUpdate_UpdatedUnion_KIND_VALUE,
		Value: ts,
	}
}

func syncPathSyncUnion(s *string) *apiv1.WorkspaceSyncUpdate_SyncPathUnion {
	if s == nil {
		return nil
	}
	return &apiv1.WorkspaceSyncUpdate_SyncPathUnion{
		Kind:  apiv1.WorkspaceSyncUpdate_SyncPathUnion_KIND_VALUE,
		Value: s,
	}
}

func syncFormatSyncUnion(s *string) *apiv1.WorkspaceSyncUpdate_SyncFormatUnion {
	if s == nil {
		return nil
	}
	return &apiv1.WorkspaceSyncUpdate_SyncFormatUnion{
		Kind:  apiv1.WorkspaceSyncUpdate_SyncFormatUnion_KIND_VALUE,
		Value: s,
	}
}

func toAPIWorkspace(ws mworkspace.Workspace) *apiv1.Workspace {
	apiWorkspace := &apiv1.Workspace{
		WorkspaceId:           ws.ID.Bytes(),
		SelectedEnvironmentId: ws.ActiveEnv.Bytes(),
		Name:                  ws.Name,
		Order:                 float32(ws.Order),
		SyncEnabled:           ws.SyncEnabled,
	}
	if !ws.Updated.IsZero() {
		apiWorkspace.Updated = timestamppb.New(ws.Updated)
	}
	if ws.SyncPath != nil {
		apiWorkspace.SyncPath = ws.SyncPath
	}
	if ws.SyncFormat != nil {
		apiWorkspace.SyncFormat = ws.SyncFormat
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
					SyncPath:              evt.Workspace.SyncPath,
					SyncFormat:            evt.Workspace.SyncFormat,
					SyncEnabled:           evt.Workspace.SyncEnabled,
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
			SyncPath:    syncPathSyncUnion(evt.Workspace.SyncPath),
			SyncFormat:  syncFormatSyncUnion(evt.Workspace.SyncFormat),
			SyncEnabled: boolPtr(evt.Workspace.SyncEnabled),
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
	workspaces, err := c.wsReader.GetWorkspacesByUserIDOrdered(ctx, userID)
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

	wsWriter := sworkspace.NewWorkspaceWriter(tx)
	wusWriter := sworkspace.NewUserWriter(tx)
	envWriter := senv.NewEnvWriter(tx)

	var createdIDs []idwrap.IDWrap
	var createdEnvs []menv.Env

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
			ID:          workspaceID,
			Name:        name,
			Updated:     dbtime.DBNow(),
			ActiveEnv:   envID,
			GlobalEnv:   envID,
			Order:       float64(item.Order),
			SyncPath:    item.SyncPath,
			SyncFormat:  item.SyncFormat,
			SyncEnabled: item.SyncEnabled,
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

		workspaceUser := &mworkspace.WorkspaceUser{
			ID:          idwrap.NewNow(),
			WorkspaceID: workspaceID,
			UserID:      userID,
			Role:        mworkspace.RoleOwner,
		}
		if err := wusWriter.CreateWorkspaceUser(ctx, workspaceUser); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		createdIDs = append(createdIDs, workspaceID)
		createdEnvs = append(createdEnvs, defaultEnv)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for _, workspaceID := range createdIDs {
		ws, err := c.wsReader.Get(ctx, workspaceID)
		if err != nil {
			continue
		}
		c.stream.Publish(WorkspaceTopic{WorkspaceID: workspaceID}, WorkspaceEvent{
			Type:      eventTypeInsert,
			Workspace: toAPIWorkspace(*ws),
		})
	}

	for _, env := range createdEnvs {
		c.envStream.Publish(renv.EnvironmentTopic{WorkspaceID: env.WorkspaceID}, renv.EnvironmentEvent{
			Type:        eventTypeInsert,
			Environment: converter.ToAPIEnvironment(env),
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

	// Step 1: FETCH and CHECK (Outside transaction)
	type updateData struct {
		workspaceID idwrap.IDWrap
		workspace   *mworkspace.Workspace
		activeEnv   *idwrap.IDWrap
	}
	var validatedUpdates []updateData

	for _, item := range req.Msg.Items {
		if len(item.WorkspaceId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id is required"))
		}

		workspaceID, err := idwrap.NewFromBytes(item.WorkspaceId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		wsUser, err := c.userReader.GetWorkspaceUsersByWorkspaceIDAndUserID(ctx, workspaceID, userID)
		if err != nil {
			if errors.Is(err, sworkspace.ErrWorkspaceUserNotFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if wsUser.Role < mworkspace.RoleAdmin {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New("permission denied"))
		}

		ws, err := c.wsReader.Get(ctx, workspaceID)
		if err != nil {
			if errors.Is(err, sworkspace.ErrNoWorkspaceFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		var activeEnv *idwrap.IDWrap
		if len(item.SelectedEnvironmentId) > 0 {
			newEnvID, err := idwrap.NewFromBytes(item.SelectedEnvironmentId)
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, err)
			}
			activeEnv = &newEnvID
		}

		validatedUpdates = append(validatedUpdates, updateData{
			workspaceID: workspaceID,
			workspace:   ws,
			activeEnv:   activeEnv,
		})
	}

	// Step 2: ACT (Inside transaction)
	tx, err := c.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	wsWriter := sworkspace.NewWorkspaceWriter(tx)
	var updatedIDs []idwrap.IDWrap

	for _, data := range validatedUpdates {
		ws := data.workspace
		if data.activeEnv != nil {
			ws.ActiveEnv = *data.activeEnv
		}

		for _, item := range req.Msg.Items {
			// Find the corresponding request item
			wID, _ := idwrap.NewFromBytes(item.WorkspaceId)
			if wID.Compare(data.workspaceID) != 0 {
				continue
			}

			if item.Name != nil {
				ws.Name = *item.Name
			}
			if item.Order != nil {
				ws.Order = float64(*item.Order)
			}
			if item.SyncPath != nil {
				switch item.SyncPath.Kind {
				case apiv1.WorkspaceUpdate_SyncPathUnion_KIND_VALUE:
					ws.SyncPath = item.SyncPath.Value
				case apiv1.WorkspaceUpdate_SyncPathUnion_KIND_UNSET:
					ws.SyncPath = nil
				}
			}
			if item.SyncFormat != nil {
				switch item.SyncFormat.Kind {
				case apiv1.WorkspaceUpdate_SyncFormatUnion_KIND_VALUE:
					ws.SyncFormat = item.SyncFormat.Value
				case apiv1.WorkspaceUpdate_SyncFormatUnion_KIND_UNSET:
					ws.SyncFormat = nil
				}
			}
			if item.SyncEnabled != nil {
				ws.SyncEnabled = *item.SyncEnabled
			}
			break
		}

		ws.Updated = dbtime.DBNow()

		if err := wsWriter.Update(ctx, ws); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		updatedIDs = append(updatedIDs, data.workspaceID)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Step 3: NOTIFY
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

	// FETCH and CHECK: Validate permissions (Owner role required)
	var validatedDeletes []idwrap.IDWrap

	for _, item := range req.Msg.Items {
		if len(item.WorkspaceId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id is required"))
		}

		workspaceID, err := idwrap.NewFromBytes(item.WorkspaceId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		wsUser, err := c.userReader.GetWorkspaceUsersByWorkspaceIDAndUserID(ctx, workspaceID, userID)
		if err != nil {
			if errors.Is(err, sworkspace.ErrWorkspaceUserNotFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if wsUser.Role != mworkspace.RoleOwner {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New("permission denied"))
		}

		validatedDeletes = append(validatedDeletes, workspaceID)
	}

	// ACT: Delete workspaces using mutation context with unified publisher
	var opts []mutation.Option
	if c.publisher != nil {
		opts = append(opts, mutation.WithPublisher(c.publisher))
	}
	mut := mutation.New(c.DB, opts...)
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	for _, wsID := range validatedDeletes {
		if err := mut.DeleteWorkspace(ctx, wsID); err != nil {
			if errors.Is(err, sworkspace.ErrNoWorkspaceFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := mut.Commit(ctx); err != nil { // Auto-publishes events!
		return nil, connect.NewError(connect.CodeInternal, err)
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

	events, err := c.stream.Subscribe(ctx, filter)
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
