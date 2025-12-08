//nolint:revive // exported
package renv

import (
	"context"
	"database/sql"
	"errors"
	"sync"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	devtoolsdb "the-dev-tools/db"
	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rworkspace"
	"the-dev-tools/server/pkg/eventstream"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/model/mvar"
	"the-dev-tools/server/pkg/permcheck"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/svar"
	"the-dev-tools/server/pkg/service/sworkspace"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/environment/v1"
	"the-dev-tools/spec/dist/buf/go/api/environment/v1/environmentv1connect"
)

type EnvRPC struct {
	DB *sql.DB

	es        senv.EnvService
	vs        svar.VarService
	us        suser.UserService
	ws        sworkspace.WorkspaceService
	envStream eventstream.SyncStreamer[EnvironmentTopic, EnvironmentEvent]
	varStream eventstream.SyncStreamer[EnvironmentVariableTopic, EnvironmentVariableEvent]
}

const (
	eventTypeInsert = "insert"
	eventTypeUpdate = "update"
	eventTypeDelete = "delete"
)

type EnvironmentTopic struct {
	WorkspaceID idwrap.IDWrap
}

type EnvironmentEvent struct {
	Type        string
	Environment *apiv1.Environment
}

type EnvironmentVariableTopic struct {
	WorkspaceID   idwrap.IDWrap
	EnvironmentID idwrap.IDWrap
}

type EnvironmentVariableEvent struct {
	Type     string
	Variable *apiv1.EnvironmentVariable
}

func New(
	db *sql.DB,
	es senv.EnvService,
	vs svar.VarService,
	us suser.UserService,
	ws sworkspace.WorkspaceService,
	envStream eventstream.SyncStreamer[EnvironmentTopic, EnvironmentEvent],
	varStream eventstream.SyncStreamer[EnvironmentVariableTopic, EnvironmentVariableEvent],
) EnvRPC {
	return EnvRPC{
		DB:        db,
		es:        es,
		vs:        vs,
		us:        us,
		ws:        ws,
		envStream: envStream,
		varStream: varStream,
	}
}

func CreateService(srv EnvRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := environmentv1connect.NewEnvironmentServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func toAPIEnvironment(env menv.Env) *apiv1.Environment {
	return &apiv1.Environment{
		EnvironmentId: env.ID.Bytes(),
		WorkspaceId:   env.WorkspaceID.Bytes(),
		Name:          env.Name,
		Description:   env.Description,
		IsGlobal:      env.Type == menv.EnvGlobal,
		Order:         float32(env.Order),
	}
}

func toAPIEnvironmentVariable(v mvar.Var) *apiv1.EnvironmentVariable {
	return &apiv1.EnvironmentVariable{
		EnvironmentVariableId: v.ID.Bytes(),
		EnvironmentId:         v.EnvID.Bytes(),
		Key:                   v.VarKey,
		Enabled:               v.Enabled,
		Value:                 v.Value,
		Description:           v.Description,
		Order:                 float32(v.Order),
	}
}

func stringPtr(s string) *string { return &s }

func boolPtr(b bool) *bool { return &b }

func float32Ptr(f float32) *float32 { return &f }

func environmentSyncResponseFrom(evt EnvironmentEvent) *apiv1.EnvironmentSyncResponse {
	if evt.Environment == nil {
		return nil
	}

	switch evt.Type {
	case eventTypeInsert:
		msg := &apiv1.EnvironmentSync{
			Value: &apiv1.EnvironmentSync_ValueUnion{
				Kind: apiv1.EnvironmentSync_ValueUnion_KIND_INSERT,
				Insert: &apiv1.EnvironmentSyncInsert{
					EnvironmentId: evt.Environment.EnvironmentId,
					WorkspaceId:   evt.Environment.WorkspaceId,
					Name:          evt.Environment.Name,
					Description:   evt.Environment.Description,
					IsGlobal:      evt.Environment.IsGlobal,
					Order:         evt.Environment.Order,
				},
			},
		}
		return &apiv1.EnvironmentSyncResponse{Items: []*apiv1.EnvironmentSync{msg}}
	case eventTypeUpdate:
		msg := &apiv1.EnvironmentSync{
			Value: &apiv1.EnvironmentSync_ValueUnion{
				Kind: apiv1.EnvironmentSync_ValueUnion_KIND_UPDATE,
				Update: &apiv1.EnvironmentSyncUpdate{
					EnvironmentId: evt.Environment.EnvironmentId,
					WorkspaceId:   evt.Environment.WorkspaceId,
					Name:          stringPtr(evt.Environment.Name),
					Description:   stringPtr(evt.Environment.Description),
					IsGlobal:      boolPtr(evt.Environment.IsGlobal),
					Order:         float32Ptr(evt.Environment.Order),
				},
			},
		}
		return &apiv1.EnvironmentSyncResponse{Items: []*apiv1.EnvironmentSync{msg}}
	case eventTypeDelete:
		msg := &apiv1.EnvironmentSync{
			Value: &apiv1.EnvironmentSync_ValueUnion{
				Kind: apiv1.EnvironmentSync_ValueUnion_KIND_DELETE,
				Delete: &apiv1.EnvironmentSyncDelete{
					EnvironmentId: evt.Environment.EnvironmentId,
				},
			},
		}
		return &apiv1.EnvironmentSyncResponse{Items: []*apiv1.EnvironmentSync{msg}}
	default:
		return nil
	}
}

func environmentVariableSyncResponseFrom(evt EnvironmentVariableEvent) *apiv1.EnvironmentVariableSyncResponse {
	if evt.Variable == nil {
		return nil
	}

	switch evt.Type {
	case eventTypeInsert:
		msg := &apiv1.EnvironmentVariableSync{
			Value: &apiv1.EnvironmentVariableSync_ValueUnion{
				Kind: apiv1.EnvironmentVariableSync_ValueUnion_KIND_INSERT,
				Insert: &apiv1.EnvironmentVariableSyncInsert{
					EnvironmentVariableId: evt.Variable.EnvironmentVariableId,
					EnvironmentId:         evt.Variable.EnvironmentId,
					Key:                   evt.Variable.Key,
					Enabled:               evt.Variable.Enabled,
					Value:                 evt.Variable.Value,
					Description:           evt.Variable.Description,
					Order:                 evt.Variable.Order,
				},
			},
		}
		return &apiv1.EnvironmentVariableSyncResponse{Items: []*apiv1.EnvironmentVariableSync{msg}}
	case eventTypeUpdate:
		msg := &apiv1.EnvironmentVariableSync{
			Value: &apiv1.EnvironmentVariableSync_ValueUnion{
				Kind: apiv1.EnvironmentVariableSync_ValueUnion_KIND_UPDATE,
				Update: &apiv1.EnvironmentVariableSyncUpdate{
					EnvironmentVariableId: evt.Variable.EnvironmentVariableId,
					EnvironmentId:         evt.Variable.EnvironmentId,
					Key:                   stringPtr(evt.Variable.Key),
					Enabled:               boolPtr(evt.Variable.Enabled),
					Value:                 stringPtr(evt.Variable.Value),
					Description:           stringPtr(evt.Variable.Description),
					Order:                 float32Ptr(evt.Variable.Order),
				},
			},
		}
		return &apiv1.EnvironmentVariableSyncResponse{Items: []*apiv1.EnvironmentVariableSync{msg}}
	case eventTypeDelete:
		msg := &apiv1.EnvironmentVariableSync{
			Value: &apiv1.EnvironmentVariableSync_ValueUnion{
				Kind: apiv1.EnvironmentVariableSync_ValueUnion_KIND_DELETE,
				Delete: &apiv1.EnvironmentVariableSyncDelete{
					EnvironmentVariableId: evt.Variable.EnvironmentVariableId,
				},
			},
		}
		return &apiv1.EnvironmentVariableSyncResponse{Items: []*apiv1.EnvironmentVariableSync{msg}}
	default:
		return nil
	}
}

func (e *EnvRPC) listUserEnvironments(ctx context.Context) ([]menv.Env, error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, err
	}

	workspaces, err := e.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		if errors.Is(err, sworkspace.ErrNoWorkspaceFound) {
			return []menv.Env{}, nil
		}
		return nil, err
	}

	var environments []menv.Env
	for _, workspace := range workspaces {
		envs, err := e.es.ListEnvironments(ctx, workspace.ID)
		if err != nil {
			if errors.Is(err, senv.ErrNoEnvironmentFound) {
				continue
			}
			return nil, err
		}
		environments = append(environments, envs...)
	}
	return environments, nil
}

// EnvironmentCollection returns all environments the user has access to
func (e *EnvRPC) EnvironmentCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.EnvironmentCollectionResponse], error) {
	environments, err := e.listUserEnvironments(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	items := make([]*apiv1.Environment, 0, len(environments))
	for _, env := range environments {
		items = append(items, toAPIEnvironment(env))
	}

	return connect.NewResponse(&apiv1.EnvironmentCollectionResponse{Items: items}), nil
}

// EnvironmentInsert creates a new environment
func (e *EnvRPC) EnvironmentInsert(ctx context.Context, req *connect.Request[apiv1.EnvironmentInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.Items) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one environment must be provided"))
	}

	// Step 1: Process request data and create environment models OUTSIDE transaction
	var envModels []menv.Env
	for _, envCreate := range req.Msg.Items {
		workspaceID, err := idwrap.NewFromBytes(envCreate.WorkspaceId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Check workspace permissions OUTSIDE transaction
		rpcErr := permcheck.CheckPerm(rworkspace.CheckOwnerWorkspace(ctx, e.us, workspaceID))
		if rpcErr != nil {
			return nil, rpcErr
		}

		var envID idwrap.IDWrap
		if len(envCreate.EnvironmentId) > 0 {
			envID, err = idwrap.NewFromBytes(envCreate.EnvironmentId)
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, err)
			}
		} else {
			envID = idwrap.NewNow()
		}

		envReq := menv.Env{
			ID:          envID,
			WorkspaceID: workspaceID,
			Type:        menv.EnvNormal,
			Description: envCreate.Description,
			Name:        envCreate.Name,
			Order:       float64(envCreate.Order),
		}

		envModels = append(envModels, envReq)
	}

	// Step 2: Minimal write transaction for fast inserts only
	tx, err := e.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	envService := e.es.TX(tx)
	var createdEnvs []menv.Env

	// Fast inserts inside minimal transaction
	for _, envReq := range envModels {
		if err := envService.CreateEnvironment(ctx, &envReq); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		createdEnvs = append(createdEnvs, envReq)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for _, env := range createdEnvs {
		e.envStream.Publish(EnvironmentTopic{WorkspaceID: env.WorkspaceID}, EnvironmentEvent{
			Type:        eventTypeInsert,
			Environment: toAPIEnvironment(env),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// EnvironmentUpdate updates an existing environment
func (e *EnvRPC) EnvironmentUpdate(ctx context.Context, req *connect.Request[apiv1.EnvironmentUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.Items) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one environment must be provided"))
	}

	tx, err := e.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	envService := e.es.TX(tx)
	var updatedEnvs []*menv.Env

	for _, envUpdate := range req.Msg.Items {
		envID, err := idwrap.NewFromBytes(envUpdate.EnvironmentId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		rpcErr := permcheck.CheckPerm(CheckOwnerEnv(ctx, e.us, envService, envID))
		if rpcErr != nil {
			return nil, rpcErr
		}

		env, err := envService.GetEnvironment(ctx, envID)
		if err != nil {
			if errors.Is(err, senv.ErrNoEnvironmentFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if len(envUpdate.WorkspaceId) > 0 {
			newWorkspaceID, err := idwrap.NewFromBytes(envUpdate.WorkspaceId)
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, err)
			}
			if newWorkspaceID.Compare(env.WorkspaceID) != 0 {
				return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("moving environments across workspaces is not supported"))
			}
		}

		if envUpdate.Name != nil {
			env.Name = *envUpdate.Name
		}
		if envUpdate.Description != nil {
			env.Description = *envUpdate.Description
		}
		if envUpdate.IsGlobal != nil {
			if *envUpdate.IsGlobal {
				env.Type = menv.EnvGlobal
			} else {
				env.Type = menv.EnvNormal
			}
		}
		if envUpdate.Order != nil {
			env.Order = float64(*envUpdate.Order)
		}

		if err := envService.UpdateEnvironment(ctx, env); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		updatedEnvs = append(updatedEnvs, env)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for _, env := range updatedEnvs {
		if env == nil {
			continue
		}
		e.envStream.Publish(EnvironmentTopic{WorkspaceID: env.WorkspaceID}, EnvironmentEvent{
			Type:        eventTypeUpdate,
			Environment: toAPIEnvironment(*env),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// EnvironmentDelete deletes an environment
func (e *EnvRPC) EnvironmentDelete(ctx context.Context, req *connect.Request[apiv1.EnvironmentDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.Items) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one environment must be provided"))
	}

	tx, err := e.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	envService := e.es.TX(tx)
	var deletedEnvs []menv.Env

	for _, envDelete := range req.Msg.Items {
		envID, err := idwrap.NewFromBytes(envDelete.EnvironmentId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		rpcErr := permcheck.CheckPerm(CheckOwnerEnv(ctx, e.us, envService, envID))
		if rpcErr != nil {
			return nil, rpcErr
		}

		env, err := envService.GetEnvironment(ctx, envID)
		if err != nil {
			if errors.Is(err, senv.ErrNoEnvironmentFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := envService.DeleteEnvironment(ctx, envID); err != nil {
			if errors.Is(err, senv.ErrNoEnvironmentFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		deletedEnvs = append(deletedEnvs, *env)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for _, env := range deletedEnvs {
		e.envStream.Publish(EnvironmentTopic{WorkspaceID: env.WorkspaceID}, EnvironmentEvent{
			Type: eventTypeDelete,
			Environment: &apiv1.Environment{
				EnvironmentId: env.ID.Bytes(),
				WorkspaceId:   env.WorkspaceID.Bytes(),
			},
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// EnvironmentSync handles real-time synchronization for environments
func (e *EnvRPC) EnvironmentSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.EnvironmentSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return e.streamEnvironmentSync(ctx, userID, stream.Send)
}

func (e *EnvRPC) streamEnvironmentSync(ctx context.Context, userID idwrap.IDWrap, send func(*apiv1.EnvironmentSyncResponse) error) error {
	var workspaceSet sync.Map

	snapshot := func(ctx context.Context) ([]eventstream.Event[EnvironmentTopic, EnvironmentEvent], error) {
		envs, err := e.listUserEnvironments(ctx)
		if err != nil {
			return nil, err
		}

		events := make([]eventstream.Event[EnvironmentTopic, EnvironmentEvent], 0, len(envs))
		for _, env := range envs {
			workspaceSet.Store(env.WorkspaceID.String(), struct{}{})
			events = append(events, eventstream.Event[EnvironmentTopic, EnvironmentEvent]{
				Topic: EnvironmentTopic{WorkspaceID: env.WorkspaceID},
				Payload: EnvironmentEvent{
					Type:        eventTypeInsert,
					Environment: toAPIEnvironment(env),
				},
			})
		}
		return events, nil
	}

	filter := func(topic EnvironmentTopic) bool {
		if _, ok := workspaceSet.Load(topic.WorkspaceID.String()); ok {
			return true
		}
		belongs, err := e.us.CheckUserBelongsToWorkspace(ctx, userID, topic.WorkspaceID)
		if err != nil || !belongs {
			return false
		}
		workspaceSet.Store(topic.WorkspaceID.String(), struct{}{})
		return true
	}

	events, err := e.envStream.Subscribe(ctx, filter, eventstream.WithSnapshot(snapshot))
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	for {
		select {
		case evt, ok := <-events:
			if !ok {
				return nil
			}
			resp := environmentSyncResponseFrom(evt.Payload)
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

// EnvironmentVariableCollection returns all environment variables for environments the user has access to
func (e *EnvRPC) EnvironmentVariableCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.EnvironmentVariableCollectionResponse], error) {
	environments, err := e.listUserEnvironments(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var items []*apiv1.EnvironmentVariable
	for _, env := range environments {
		vars, err := e.vs.GetVariableByEnvID(ctx, env.ID)
		if err != nil {
			if errors.Is(err, svar.ErrNoVarFound) {
				continue
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, v := range vars {
			items = append(items, toAPIEnvironmentVariable(v))
		}
	}

	return connect.NewResponse(&apiv1.EnvironmentVariableCollectionResponse{Items: items}), nil
}

// EnvironmentVariableInsert creates new environment variables
func (e *EnvRPC) EnvironmentVariableInsert(ctx context.Context, req *connect.Request[apiv1.EnvironmentVariableInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.Items) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one environment variable must be provided"))
	}

	// Step 1: Process request data and build cache OUTSIDE transaction
	type varData struct {
		envID       idwrap.IDWrap
		workspaceID idwrap.IDWrap
		varID       idwrap.IDWrap
		key         string
		value       string
		enabled     bool
		description string
		order       float64
	}

	var varModels []varData
	workspaceCache := map[string]idwrap.IDWrap{}

	for _, item := range req.Msg.Items {
		envID, err := idwrap.NewFromBytes(item.EnvironmentId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Check permissions OUTSIDE transaction
		rpcErr := permcheck.CheckPerm(CheckOwnerEnv(ctx, e.us, e.es, envID))
		if rpcErr != nil {
			return nil, rpcErr
		}

		// Build cache OUTSIDE transaction
		workspaceID := workspaceCache[envID.String()]
		if workspaceID == (idwrap.IDWrap{}) {
			env, err := e.es.GetEnvironment(ctx, envID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			workspaceID = env.WorkspaceID
			workspaceCache[envID.String()] = workspaceID
		}

		varID := idwrap.NewNow()
		if len(item.EnvironmentVariableId) > 0 {
			varID, err = idwrap.NewFromBytes(item.EnvironmentVariableId)
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, err)
			}
		}

		varModels = append(varModels, varData{
			envID:       envID,
			workspaceID: workspaceID,
			varID:       varID,
			key:         item.Key,
			value:       item.Value,
			enabled:     item.Enabled,
			description: item.Description,
			order:       float64(item.Order),
		})
	}

	// Step 2: Minimal write transaction for fast inserts only
	tx, err := e.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	varService := e.vs.TX(tx)
	createdVars := []struct {
		variable    mvar.Var
		workspaceID idwrap.IDWrap
	}{}

	// Fast inserts inside minimal transaction
	for _, data := range varModels {
		varReq := mvar.Var{
			ID:          data.varID,
			EnvID:       data.envID,
			VarKey:      data.key,
			Value:       data.value,
			Enabled:     data.enabled,
			Description: data.description,
			Order:       data.order,
		}

		if err := varService.Create(ctx, varReq); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		createdVars = append(createdVars, struct {
			variable    mvar.Var
			workspaceID idwrap.IDWrap
		}{variable: varReq, workspaceID: data.workspaceID})
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for _, evt := range createdVars {
		e.varStream.Publish(EnvironmentVariableTopic{WorkspaceID: evt.workspaceID, EnvironmentID: evt.variable.EnvID}, EnvironmentVariableEvent{
			Type:     eventTypeInsert,
			Variable: toAPIEnvironmentVariable(evt.variable),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// EnvironmentVariableUpdate updates existing environment variables
func (e *EnvRPC) EnvironmentVariableUpdate(ctx context.Context, req *connect.Request[apiv1.EnvironmentVariableUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.Items) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one environment variable must be provided"))
	}

	tx, err := e.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	varService := e.vs.TX(tx)
	var updatedVars []struct {
		variable    *mvar.Var
		workspaceID idwrap.IDWrap
	}
	workspaceCache := map[string]idwrap.IDWrap{}

	for _, item := range req.Msg.Items {
		varID, err := idwrap.NewFromBytes(item.EnvironmentVariableId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		rpcErr := permcheck.CheckPerm(CheckOwnerVar(ctx, e.us, varService, e.es, varID))
		if rpcErr != nil {
			return nil, rpcErr
		}

		variable, err := varService.Get(ctx, varID)
		if err != nil {
			if errors.Is(err, svar.ErrNoVarFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if len(item.EnvironmentId) > 0 {
			newEnvID, err := idwrap.NewFromBytes(item.EnvironmentId)
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, err)
			}

			rpcErr := permcheck.CheckPerm(CheckOwnerEnv(ctx, e.us, e.es, newEnvID))
			if rpcErr != nil {
				return nil, rpcErr
			}

			variable.EnvID = newEnvID
		}

		if item.Key != nil {
			variable.VarKey = *item.Key
		}
		if item.Value != nil {
			variable.Value = *item.Value
		}
		if item.Enabled != nil {
			variable.Enabled = *item.Enabled
		}
		if item.Description != nil {
			variable.Description = *item.Description
		}
		if item.Order != nil {
			variable.Order = float64(*item.Order)
		}

		if err := varService.Update(ctx, variable); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		workspaceID := workspaceCache[variable.EnvID.String()]
		if workspaceID == (idwrap.IDWrap{}) {
			env, err := e.es.GetEnvironment(ctx, variable.EnvID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			workspaceID = env.WorkspaceID
			workspaceCache[variable.EnvID.String()] = workspaceID
		}
		updatedVars = append(updatedVars, struct {
			variable    *mvar.Var
			workspaceID idwrap.IDWrap
		}{variable: variable, workspaceID: workspaceID})
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for _, evt := range updatedVars {
		e.varStream.Publish(EnvironmentVariableTopic{WorkspaceID: evt.workspaceID, EnvironmentID: evt.variable.EnvID}, EnvironmentVariableEvent{
			Type:     eventTypeUpdate,
			Variable: toAPIEnvironmentVariable(*evt.variable),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// EnvironmentVariableDelete deletes environment variables
func (e *EnvRPC) EnvironmentVariableDelete(ctx context.Context, req *connect.Request[apiv1.EnvironmentVariableDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.Items) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one environment variable must be provided"))
	}

	tx, err := e.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	varService := e.vs.TX(tx)
	deletedVars := []struct {
		variable    mvar.Var
		workspaceID idwrap.IDWrap
	}{}
	workspaceCache := map[string]idwrap.IDWrap{}

	for _, item := range req.Msg.Items {
		varID, err := idwrap.NewFromBytes(item.EnvironmentVariableId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		rpcErr := permcheck.CheckPerm(CheckOwnerVar(ctx, e.us, varService, e.es, varID))
		if rpcErr != nil {
			return nil, rpcErr
		}

		variable, err := varService.Get(ctx, varID)
		if err != nil {
			if errors.Is(err, svar.ErrNoVarFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := varService.Delete(ctx, varID); err != nil {
			if errors.Is(err, svar.ErrNoVarFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		workspaceID := workspaceCache[variable.EnvID.String()]
		if workspaceID == (idwrap.IDWrap{}) {
			env, err := e.es.GetEnvironment(ctx, variable.EnvID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			workspaceID = env.WorkspaceID
			workspaceCache[variable.EnvID.String()] = workspaceID
		}
		deletedVars = append(deletedVars, struct {
			variable    mvar.Var
			workspaceID idwrap.IDWrap
		}{variable: *variable, workspaceID: workspaceID})
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for _, evt := range deletedVars {
		e.varStream.Publish(EnvironmentVariableTopic{WorkspaceID: evt.workspaceID, EnvironmentID: evt.variable.EnvID}, EnvironmentVariableEvent{
			Type: eventTypeDelete,
			Variable: &apiv1.EnvironmentVariable{
				EnvironmentVariableId: evt.variable.ID.Bytes(),
				EnvironmentId:         evt.variable.EnvID.Bytes(),
			},
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// EnvironmentVariableSync handles real-time synchronization for environment variables
func (e *EnvRPC) EnvironmentVariableSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.EnvironmentVariableSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return e.streamEnvironmentVariableSync(ctx, userID, stream.Send)
}

func (e *EnvRPC) streamEnvironmentVariableSync(ctx context.Context, userID idwrap.IDWrap, send func(*apiv1.EnvironmentVariableSyncResponse) error) error {
	var workspaceSet sync.Map

	snapshot := func(ctx context.Context) ([]eventstream.Event[EnvironmentVariableTopic, EnvironmentVariableEvent], error) {
		envs, err := e.listUserEnvironments(ctx)
		if err != nil {
			return nil, err
		}

		var events []eventstream.Event[EnvironmentVariableTopic, EnvironmentVariableEvent]
		for _, env := range envs {
			workspaceSet.Store(env.WorkspaceID.String(), struct{}{})
			vars, err := e.vs.GetVariableByEnvID(ctx, env.ID)
			if err != nil {
				if errors.Is(err, svar.ErrNoVarFound) {
					continue
				}
				return nil, err
			}
			for _, v := range vars {
				copyVar := v
				events = append(events, eventstream.Event[EnvironmentVariableTopic, EnvironmentVariableEvent]{
					Topic: EnvironmentVariableTopic{WorkspaceID: env.WorkspaceID, EnvironmentID: env.ID},
					Payload: EnvironmentVariableEvent{
						Type:     eventTypeInsert,
						Variable: toAPIEnvironmentVariable(copyVar),
					},
				})
			}
		}
		return events, nil
	}

	filter := func(topic EnvironmentVariableTopic) bool {
		if _, ok := workspaceSet.Load(topic.WorkspaceID.String()); ok {
			return true
		}
		belongs, err := e.us.CheckUserBelongsToWorkspace(ctx, userID, topic.WorkspaceID)
		if err != nil || !belongs {
			return false
		}
		workspaceSet.Store(topic.WorkspaceID.String(), struct{}{})
		return true
	}

	events, err := e.varStream.Subscribe(ctx, filter, eventstream.WithSnapshot(snapshot))
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	for {
		select {
		case evt, ok := <-events:
			if !ok {
				return nil
			}
			resp := environmentVariableSyncResponseFrom(evt.Payload)
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

// Helper function to check environment ownership
func CheckOwnerEnv(ctx context.Context, su suser.UserService, es senv.EnvService, envid idwrap.IDWrap) (bool, error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return false, err
	}
	env, err := es.Get(ctx, envid)
	if err != nil {
		return false, err
	}
	return su.CheckUserBelongsToWorkspace(ctx, userID, env.WorkspaceID)
}

// Helper function to check environment variable ownership
func CheckOwnerVar(ctx context.Context, su suser.UserService, vs svar.VarService, es senv.EnvService, varID idwrap.IDWrap) (bool, error) {
	variable, err := vs.Get(ctx, varID)
	if err != nil {
		return false, err
	}
	return CheckOwnerEnv(ctx, su, es, variable.EnvID)
}
