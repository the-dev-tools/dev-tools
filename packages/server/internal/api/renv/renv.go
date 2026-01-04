//nolint:revive // exported
package renv

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	devtoolsdb "the-dev-tools/db"
	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/converter"
	"the-dev-tools/server/pkg/eventstream"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/permcheck"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/environment/v1"
	"the-dev-tools/spec/dist/buf/go/api/environment/v1/environmentv1connect"
)

type EnvRPC struct {
	DB *sql.DB

	es senv.EnvService
	vs senv.VariableService
	us suser.UserService
	ws sworkspace.WorkspaceService

	envReader *senv.EnvReader
	varReader *senv.VariableReader

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

type EnvRPCServices struct {
	Env       senv.EnvService
	Variable  senv.VariableService
	User      suser.UserService
	Workspace sworkspace.WorkspaceService
}

func (s *EnvRPCServices) Validate() error {
	return nil
}

type EnvRPCReaders struct {
	Env      *senv.EnvReader
	Variable *senv.VariableReader
}

func (r *EnvRPCReaders) Validate() error {
	if r.Env == nil {
		return fmt.Errorf("env reader is required")
	}
	if r.Variable == nil {
		return fmt.Errorf("variable reader is required")
	}
	return nil
}

type EnvRPCStreamers struct {
	Env      eventstream.SyncStreamer[EnvironmentTopic, EnvironmentEvent]
	Variable eventstream.SyncStreamer[EnvironmentVariableTopic, EnvironmentVariableEvent]
}

func (s *EnvRPCStreamers) Validate() error {
	if s.Env == nil {
		return fmt.Errorf("env stream is required")
	}
	if s.Variable == nil {
		return fmt.Errorf("variable stream is required")
	}
	return nil
}

type EnvRPCDeps struct {
	DB        *sql.DB
	Services  EnvRPCServices
	Readers   EnvRPCReaders
	Streamers EnvRPCStreamers
}

func (d *EnvRPCDeps) Validate() error {
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

func New(deps EnvRPCDeps) EnvRPC {
	if err := deps.Validate(); err != nil {
		panic(fmt.Sprintf("EnvRPC Deps validation failed: %v", err))
	}

	return EnvRPC{
		DB:        deps.DB,
		es:        deps.Services.Env,
		vs:        deps.Services.Variable,
		us:        deps.Services.User,
		ws:        deps.Services.Workspace,
		envReader: deps.Readers.Env,
		varReader: deps.Readers.Variable,
		envStream: deps.Streamers.Env,
		varStream: deps.Streamers.Variable,
	}
}

func CreateService(srv EnvRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := environmentv1connect.NewEnvironmentServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
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
		envs, err := e.envReader.ListEnvironments(ctx, workspace.ID)
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
		items = append(items, converter.ToAPIEnvironment(env))
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
		rpcErr := permcheck.CheckPerm(mwauth.CheckOwnerWorkspace(ctx, e.us, workspaceID))
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

	envWriter := senv.NewEnvWriter(tx)
	var createdEnvs []menv.Env

	// Fast inserts inside minimal transaction
	for _, envReq := range envModels {
		if err := envWriter.CreateEnvironment(ctx, &envReq); err != nil {
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
			Environment: converter.ToAPIEnvironment(env),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// EnvironmentUpdate updates an existing environment
func (e *EnvRPC) EnvironmentUpdate(ctx context.Context, req *connect.Request[apiv1.EnvironmentUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.Items) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one environment must be provided"))
	}

	// Step 1: FETCH and CHECK (Outside transaction)
	var validatedUpdates []*menv.Env

	for _, envUpdate := range req.Msg.Items {
		envID, err := idwrap.NewFromBytes(envUpdate.EnvironmentId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Use global service (Reader) for checks
		rpcErr := permcheck.CheckPerm(CheckOwnerEnv(ctx, e.us, e.es, envID))
		if rpcErr != nil {
			return nil, rpcErr
		}

		// Use global service (Reader) for Fetch
		env, err := e.envReader.GetEnvironment(ctx, envID)
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

		validatedUpdates = append(validatedUpdates, env)
	}

	// Step 2: ACT (Inside transaction)
	tx, err := e.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	envWriter := senv.NewEnvWriter(tx)

	for _, env := range validatedUpdates {
		if err := envWriter.UpdateEnvironment(ctx, env); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Step 3: NOTIFY
	for _, env := range validatedUpdates {
		if env == nil {
			continue
		}
		e.envStream.Publish(EnvironmentTopic{WorkspaceID: env.WorkspaceID}, EnvironmentEvent{
			Type:        eventTypeUpdate,
			Environment: converter.ToAPIEnvironment(*env),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// EnvironmentDelete deletes an environment
func (e *EnvRPC) EnvironmentDelete(ctx context.Context, req *connect.Request[apiv1.EnvironmentDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.Items) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one environment must be provided"))
	}

	// Step 1: FETCH and CHECK
	var validatedDeletes []menv.Env

	for _, envDelete := range req.Msg.Items {
		envID, err := idwrap.NewFromBytes(envDelete.EnvironmentId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		rpcErr := permcheck.CheckPerm(CheckOwnerEnv(ctx, e.us, e.es, envID))
		if rpcErr != nil {
			return nil, rpcErr
		}

		env, err := e.envReader.GetEnvironment(ctx, envID)
		if err != nil {
			if errors.Is(err, senv.ErrNoEnvironmentFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		validatedDeletes = append(validatedDeletes, *env)
	}

	// Step 2: ACT
	tx, err := e.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	envWriter := senv.NewEnvWriter(tx)

	for _, env := range validatedDeletes {
		if err := envWriter.DeleteEnvironment(ctx, env.ID); err != nil {
			if errors.Is(err, senv.ErrNoEnvironmentFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Step 3: NOTIFY
	for _, env := range validatedDeletes {
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

	events, err := e.envStream.Subscribe(ctx, filter)
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
		vars, err := e.varReader.GetVariableByEnvID(ctx, env.ID)
		if err != nil {
			if errors.Is(err, senv.ErrNoVarFound) {
				continue
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, v := range vars {
			items = append(items, converter.ToAPIEnvironmentVariable(v))
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
			env, err := e.envReader.GetEnvironment(ctx, envID)
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

	varWriter := senv.NewVariableWriter(tx)
	createdVars := []struct {
		variable    menv.Variable
		workspaceID idwrap.IDWrap
	}{}

	// Fast inserts inside minimal transaction
	for _, data := range varModels {
		varReq := menv.Variable{
			ID:          data.varID,
			EnvID:       data.envID,
			VarKey:      data.key,
			Value:       data.value,
			Enabled:     data.enabled,
			Description: data.description,
			Order:       data.order,
		}

		if err := varWriter.Create(ctx, varReq); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		createdVars = append(createdVars, struct {
			variable    menv.Variable
			workspaceID idwrap.IDWrap
		}{variable: varReq, workspaceID: data.workspaceID})
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for _, evt := range createdVars {
		e.varStream.Publish(EnvironmentVariableTopic{WorkspaceID: evt.workspaceID, EnvironmentID: evt.variable.EnvID}, EnvironmentVariableEvent{
			Type:     eventTypeInsert,
			Variable: converter.ToAPIEnvironmentVariable(evt.variable),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// EnvironmentVariableUpdate updates existing environment variables
func (e *EnvRPC) EnvironmentVariableUpdate(ctx context.Context, req *connect.Request[apiv1.EnvironmentVariableUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.Items) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one environment variable must be provided"))
	}

	// Step 1: FETCH and CHECK (Outside transaction)
	type varUpdateData struct {
		variable    menv.Variable
		workspaceID idwrap.IDWrap
	}

	var varModels []varUpdateData
	workspaceCache := map[string]idwrap.IDWrap{}

	for _, item := range req.Msg.Items {
		varID, err := idwrap.NewFromBytes(item.EnvironmentVariableId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		rpcErr := permcheck.CheckPerm(CheckOwnerVar(ctx, e.us, e.vs, e.es, varID))
		if rpcErr != nil {
			return nil, rpcErr
		}

		variable, err := e.varReader.Get(ctx, varID)
		if err != nil {
			if errors.Is(err, senv.ErrNoVarFound) {
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

		workspaceID := workspaceCache[variable.EnvID.String()]
		if workspaceID == (idwrap.IDWrap{}) {
			env, err := e.es.GetEnvironment(ctx, variable.EnvID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			workspaceID = env.WorkspaceID
			workspaceCache[variable.EnvID.String()] = workspaceID
		}

		varModels = append(varModels, varUpdateData{
			variable:    *variable,
			workspaceID: workspaceID,
		})
	}

	// Step 2: ACT (Inside transaction)
	tx, err := e.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	varWriter := senv.NewVariableWriter(tx)

	for i := range varModels {
		if err := varWriter.Update(ctx, &varModels[i].variable); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Step 3: NOTIFY
	for _, data := range varModels {
		e.varStream.Publish(EnvironmentVariableTopic{WorkspaceID: data.workspaceID, EnvironmentID: data.variable.EnvID}, EnvironmentVariableEvent{
			Type:     eventTypeUpdate,
			Variable: converter.ToAPIEnvironmentVariable(data.variable),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// EnvironmentVariableDelete deletes environment variables
func (e *EnvRPC) EnvironmentVariableDelete(ctx context.Context, req *connect.Request[apiv1.EnvironmentVariableDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.Items) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one environment variable must be provided"))
	}

	// Step 1: FETCH and CHECK
	type varDeleteData struct {
		varID       idwrap.IDWrap
		variable    menv.Variable
		workspaceID idwrap.IDWrap
	}

	var varModels []varDeleteData
	workspaceCache := map[string]idwrap.IDWrap{}

	for _, item := range req.Msg.Items {
		varID, err := idwrap.NewFromBytes(item.EnvironmentVariableId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		rpcErr := permcheck.CheckPerm(CheckOwnerVar(ctx, e.us, e.vs, e.es, varID))
		if rpcErr != nil {
			return nil, rpcErr
		}

		variable, err := e.varReader.Get(ctx, varID)
		if err != nil {
			if errors.Is(err, senv.ErrNoVarFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		workspaceID := workspaceCache[variable.EnvID.String()]
		if workspaceID == (idwrap.IDWrap{}) {
			env, err := e.envReader.GetEnvironment(ctx, variable.EnvID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			workspaceID = env.WorkspaceID
			workspaceCache[variable.EnvID.String()] = workspaceID
		}

		varModels = append(varModels, varDeleteData{
			varID:       varID,
			variable:    *variable,
			workspaceID: workspaceID,
		})
	}

	// Step 2: ACT
	tx, err := e.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	varWriter := senv.NewVariableWriter(tx)

	for _, data := range varModels {
		if err := varWriter.Delete(ctx, data.varID); err != nil {
			if errors.Is(err, senv.ErrNoVarFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Step 3: NOTIFY
	for _, data := range varModels {
		e.varStream.Publish(EnvironmentVariableTopic{WorkspaceID: data.workspaceID, EnvironmentID: data.variable.EnvID}, EnvironmentVariableEvent{
			Type: eventTypeDelete,
			Variable: &apiv1.EnvironmentVariable{
				EnvironmentVariableId: data.variable.ID.Bytes(),
				EnvironmentId:         data.variable.EnvID.Bytes(),
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

	events, err := e.varStream.Subscribe(ctx, filter)
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
func CheckOwnerVar(ctx context.Context, su suser.UserService, vs senv.VariableService, es senv.EnvService, varID idwrap.IDWrap) (bool, error) {
	variable, err := vs.Get(ctx, varID)
	if err != nil {
		return false, err
	}
	return CheckOwnerEnv(ctx, su, es, variable.EnvID)
}
