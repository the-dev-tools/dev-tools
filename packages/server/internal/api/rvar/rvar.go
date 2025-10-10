package rvar

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"sort"
	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/renv"
	"the-dev-tools/server/internal/api/rworkspace"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mvar"
	"the-dev-tools/server/pkg/permcheck"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/svar"
	"the-dev-tools/server/pkg/translate/tgeneric"
	"the-dev-tools/server/pkg/translate/tvar"
	resourcesv1 "the-dev-tools/spec/dist/buf/go/resource/v1"
	variablev1 "the-dev-tools/spec/dist/buf/go/variable/v1"
	"the-dev-tools/spec/dist/buf/go/variable/v1/variablev1connect"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"
)

type VarRPC struct {
	DB *sql.DB

	us suser.UserService

	es senv.EnvService
	vs svar.VarService
}

func New(db *sql.DB, us suser.UserService, es senv.EnvService, vs svar.VarService) VarRPC {
	return VarRPC{
		DB: db,
		us: us,
		es: es,
		vs: vs,
	}
}

func CreateService(srv VarRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := variablev1connect.NewVariableServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (v *VarRPC) VariableList(ctx context.Context, req *connect.Request[variablev1.VariableListRequest]) (*connect.Response[variablev1.VariableListResponse], error) {
	envIDRaw, workspaceIDRaw := req.Msg.GetEnvironmentId(), req.Msg.GetWorkspaceId()
	if len(envIDRaw) != 0 {
		envID, err := idwrap.NewFromBytes(req.Msg.EnvironmentId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		rpcErr := permcheck.CheckPerm(renv.CheckOwnerEnv(ctx, v.us, v.es, envID))
		if rpcErr != nil {
			return nil, rpcErr
		}
		vars, err := v.vs.GetVariablesByEnvIDOrdered(ctx, envID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		rpcVars := tgeneric.MassConvert(vars, tvar.SerializeModelToRPCItem)
		return connect.NewResponse(&variablev1.VariableListResponse{Items: rpcVars, EnvironmentId: envIDRaw}), nil

	} else if len(workspaceIDRaw) != 0 {
		workspaceID, err := idwrap.NewFromBytes(req.Msg.WorkspaceId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		rpcErr := permcheck.CheckPerm(rworkspace.CheckOwnerWorkspace(ctx, v.us, workspaceID))
		if rpcErr != nil {
			return nil, rpcErr
		}
		envs, err := v.es.GetByWorkspace(ctx, workspaceID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		var vars []mvar.Var
		for _, env := range envs {
			envVars, err := v.vs.GetVariablesByEnvIDOrdered(ctx, env.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			vars = append(vars, envVars...)
		}

		sort.Slice(vars, func(i, j int) bool {
			return bytes.Compare(vars[i].EnvID.Bytes(), vars[j].EnvID.Bytes()) < 0
		})

		rpcVars := tgeneric.MassConvert(vars, tvar.SerializeModelToRPCItem)

		return connect.NewResponse(&variablev1.VariableListResponse{
			EnvironmentId: envIDRaw,
			Items:         rpcVars,
		}), nil
	}
	return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace id or env ID is required"))
}

func (v *VarRPC) VariableGet(ctx context.Context, req *connect.Request[variablev1.VariableGetRequest]) (*connect.Response[variablev1.VariableGetResponse], error) {
	id, err := idwrap.NewFromBytes(req.Msg.VariableId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerVar(ctx, v.us, v.vs, v.es, id))
	if rpcErr != nil {
		return nil, rpcErr
	}
	varible, err := v.vs.Get(ctx, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	rpcVar := tvar.SerializeModelToRPC(*varible)
	rpcRawResp := &variablev1.VariableGetResponse{
		VariableId:  rpcVar.VariableId,
		Name:        rpcVar.Name,
		Value:       rpcVar.Value,
		Enabled:     rpcVar.Enabled,
		Description: rpcVar.Description,
	}
	return connect.NewResponse(rpcRawResp), nil
}

func (v *VarRPC) VariableCreate(ctx context.Context, req *connect.Request[variablev1.VariableCreateRequest]) (*connect.Response[variablev1.VariableCreateResponse], error) {
	envID, err := idwrap.NewFromBytes(req.Msg.GetEnvironmentId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(renv.CheckOwnerEnv(ctx, v.us, v.es, envID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	rpcVar := variablev1.Variable{
		Name:        req.Msg.Name,
		Value:       req.Msg.Value,
		Enabled:     req.Msg.Enabled,
		Description: req.Msg.Description,
	}

	varReq := tvar.DeserializeRPCToModelWithID(idwrap.NewNow(), envID, &rpcVar)
	err = v.vs.Create(ctx, varReq)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	varReq.EnvID = envID

	return connect.NewResponse(&variablev1.VariableCreateResponse{VariableId: varReq.ID.Bytes()}), nil
}

func (c *VarRPC) VariableUpdate(ctx context.Context, req *connect.Request[variablev1.VariableUpdateRequest]) (*connect.Response[variablev1.VariableUpdateResponse], error) {
	msg := req.Msg

	var name string
	var value string
	var enabled bool
	var description string
	if msg.Name != nil {
		name = *msg.Name
	}
	if msg.Value != nil {
		value = *msg.Value
	}
	if msg.Enabled != nil {
		enabled = *msg.Enabled
	}
	if msg.Description != nil {
		description = *msg.Description
	}

	varConverted := &variablev1.Variable{
		VariableId:  msg.GetVariableId(),
		Name:        name,
		Value:       value,
		Enabled:     enabled,
		Description: description,
	}
	varReq, err := tvar.DeserializeRPCToModel(varConverted)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerVar(ctx, c.us, c.vs, c.es, varReq.ID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	err = c.vs.Update(ctx, &varReq)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&variablev1.VariableUpdateResponse{}), nil
}

func (c *VarRPC) VariableDelete(ctx context.Context, req *connect.Request[variablev1.VariableDeleteRequest]) (*connect.Response[variablev1.VariableDeleteResponse], error) {
	id, err := idwrap.NewFromBytes(req.Msg.GetVariableId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerVar(ctx, c.us, c.vs, c.es, id))
	if rpcErr != nil {
		return nil, rpcErr
	}
	err = c.vs.Delete(ctx, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&variablev1.VariableDeleteResponse{}), nil
}

func CheckOwnerVar(ctx context.Context, us suser.UserService, vs svar.VarService, es senv.EnvService, varID idwrap.IDWrap) (bool, error) {
	variable, err := vs.Get(ctx, varID)
	if err != nil {
		return false, err
	}
	return renv.CheckOwnerEnv(ctx, us, es, variable.EnvID)
}

func (c *VarRPC) VariableMove(ctx context.Context, req *connect.Request[variablev1.VariableMoveRequest]) (*connect.Response[emptypb.Empty], error) {
	// Validate variable ID
	variableID, err := idwrap.NewFromBytes(req.Msg.GetVariableId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Validate target variable ID
	targetVariableID, err := idwrap.NewFromBytes(req.Msg.GetTargetVariableId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Check permissions for the variable being moved
	rpcErr := permcheck.CheckPerm(CheckOwnerVar(ctx, c.us, c.vs, c.es, variableID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	// Check permissions for the target variable
	rpcErr = permcheck.CheckPerm(CheckOwnerVar(ctx, c.us, c.vs, c.es, targetVariableID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	// Validate position
	position := req.Msg.GetPosition()
	if position == resourcesv1.MovePosition_MOVE_POSITION_UNSPECIFIED {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("position must be specified"))
	}

	// Prevent moving variable relative to itself
	if variableID.Compare(targetVariableID) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("cannot move variable relative to itself"))
	}

	// Verify both variables are in the same environment
	sourceEnvID, err := c.vs.GetEnvID(ctx, variableID)
	if err != nil {
		if err == svar.ErrNoVarFound {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("variable not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	targetEnvID, err := c.vs.GetEnvID(ctx, targetVariableID)
	if err != nil {
		if err == svar.ErrNoVarFound {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("target variable not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if sourceEnvID.Compare(targetEnvID) != 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("variables must be in the same environment"))
	}

	// Add debug logging for move operations
	slog.DebugContext(ctx, "VariableMove request",
		"variable_id", variableID.String(),
		"target_variable_id", targetVariableID.String(),
		"position", position.String(),
		"environment_id", sourceEnvID.String())

	// Execute the move operation
	switch position {
	case resourcesv1.MovePosition_MOVE_POSITION_AFTER:
		err = c.vs.MoveVariableAfter(ctx, variableID, targetVariableID)
	case resourcesv1.MovePosition_MOVE_POSITION_BEFORE:
		err = c.vs.MoveVariableBefore(ctx, variableID, targetVariableID)
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid position"))
	}

	if err != nil {
		// Map service-level errors to appropriate Connect error codes
		if err == svar.ErrNoVarFound {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		if err == svar.ErrSelfReferentialMove {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		if err == svar.ErrEnvironmentBoundaryViolation {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}
