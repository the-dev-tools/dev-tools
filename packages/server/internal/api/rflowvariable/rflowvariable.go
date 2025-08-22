package rflowvariable

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"connectrpc.com/connect"

	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/rflow"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/permcheck"

	"the-dev-tools/server/pkg/model/mflowvariable"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sflowvariable"
	"the-dev-tools/server/pkg/service/suser"

	"the-dev-tools/server/pkg/translate/tflowvariable"

	flowvariablev1 "the-dev-tools/spec/dist/buf/go/flowvariable/v1"
	"the-dev-tools/spec/dist/buf/go/flowvariable/v1/flowvariablev1connect"
	resourcesv1 "the-dev-tools/spec/dist/buf/go/resources/v1"
)

type FlowVariableServiceRPC struct {
	DB  *sql.DB
	fs  sflow.FlowService
	us  suser.UserService
	fvs sflowvariable.FlowVariableService
}

func New(db *sql.DB, fs sflow.FlowService, us suser.UserService, fvs sflowvariable.FlowVariableService) FlowVariableServiceRPC {
	return FlowVariableServiceRPC{
		DB:  db,
		fs:  fs,
		us:  us,
		fvs: fvs,
	}
}

func CreateService(srv FlowVariableServiceRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := flowvariablev1connect.NewFlowVariableServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (c *FlowVariableServiceRPC) FlowVariableList(ctx context.Context, req *connect.Request[flowvariablev1.FlowVariableListRequest]) (*connect.Response[flowvariablev1.FlowVariableListResponse], error) {
	flowID, err := idwrap.NewFromBytes(req.Msg.FlowId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	rpcErr := permcheck.CheckPerm(rflow.CheckOwnerFlow(ctx, c.fs, c.us, flowID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	variables, err := c.fvs.GetFlowVariablesByFlowIDOrdered(ctx, flowID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var items []*flowvariablev1.FlowVariableListItem
	for _, variable := range variables {
		items = append(items, tflowvariable.ModelToRPCListItem(variable))
	}

	response := &flowvariablev1.FlowVariableListResponse{
		FlowId: flowID.Bytes(),
		Items:  items,
	}

	return connect.NewResponse(response), nil
}

func (c *FlowVariableServiceRPC) FlowVariableGet(ctx context.Context, req *connect.Request[flowvariablev1.FlowVariableGetRequest]) (*connect.Response[flowvariablev1.FlowVariableGetResponse], error) {
	variableID, err := idwrap.NewFromBytes(req.Msg.VariableId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	variable, err := c.fvs.GetFlowVariable(ctx, variableID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	rpcErr := permcheck.CheckPerm(rflow.CheckOwnerFlow(ctx, c.fs, c.us, variable.FlowID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	response := &flowvariablev1.FlowVariableGetResponse{
		VariableId:  variableID.Bytes(),
		Name:        variable.Name,
		Value:       variable.Value,
		Enabled:     variable.Enabled,
		Description: variable.Description,
	}

	return connect.NewResponse(response), nil
}

func (c *FlowVariableServiceRPC) FlowVariableCreate(ctx context.Context, req *connect.Request[flowvariablev1.FlowVariableCreateRequest]) (*connect.Response[flowvariablev1.FlowVariableCreateResponse], error) {
	flowID, err := idwrap.NewFromBytes(req.Msg.FlowId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	rpcErr := permcheck.CheckPerm(rflow.CheckOwnerFlow(ctx, c.fs, c.us, flowID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	variableID := idwrap.NewNow()
	if len(req.Msg.VariableId) > 0 {
		variableID, err = idwrap.NewFromBytes(req.Msg.VariableId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
	}

	if req.Msg.Name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}

	variable := mflowvariable.FlowVariable{
		ID:          variableID,
		FlowID:      flowID,
		Name:        req.Msg.Name,
		Value:       req.Msg.Value,
		Enabled:     req.Msg.Enabled,
		Description: req.Msg.Description,
	}

	err = c.fvs.CreateFlowVariable(ctx, variable)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create flow variable: %w", err))
	}
	/*

		// Create an invalidation change for the flow variables list
		service := "flowvariable.v1.FlowVariableService"
		method := "FlowVariableList"
		changeKind := changev1.ChangeKind_CHANGE_KIND_INVALIDATE
		listRequest, err := anypb.New(&flowvariablev1.FlowVariableListRequest{
			FlowId: flowID.Bytes(),
		})
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

			changes := []*changev1.Change{
				{
					Kind:    &changeKind,
					Service: &service,
					Method:  &method,
					Data:    listRequest,
				},
			}
	*/

	response := &flowvariablev1.FlowVariableCreateResponse{
		VariableId: variableID.Bytes(),
		// Changes:    changes,
	}

	return connect.NewResponse(response), nil
}

func (c *FlowVariableServiceRPC) FlowVariableUpdate(ctx context.Context, req *connect.Request[flowvariablev1.FlowVariableUpdateRequest]) (*connect.Response[flowvariablev1.FlowVariableUpdateResponse], error) {
	variableID, err := idwrap.NewFromBytes(req.Msg.VariableId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	variable, err := c.fvs.GetFlowVariable(ctx, variableID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	rpcErr := permcheck.CheckPerm(rflow.CheckOwnerFlow(ctx, c.fs, c.us, variable.FlowID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	if req.Msg.Name != nil {
		variable.Name = *req.Msg.Name
	}
	if req.Msg.Value != nil {
		variable.Value = *req.Msg.Value
	}
	if req.Msg.Enabled != nil {
		variable.Enabled = *req.Msg.Enabled
	}
	if req.Msg.Description != nil {
		variable.Description = *req.Msg.Description
	}

	err = c.fvs.UpdateFlowVariable(ctx, variable)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update flow variable: %w", err))
	}

	/*

		// TODO: should be just update one not invalidation all list
		// Create an invalidation change for the list and get
		service := "flowvariable.v1.FlowVariableService"
		listMethod := "FlowVariableList"
		changeKind := changev1.ChangeKind_CHANGE_KIND_INVALIDATE
		listRequest, err := anypb.New(&flowvariablev1.FlowVariableListRequest{
			FlowId: variable.FlowID.Bytes(),
		})
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		getMethod := "FlowVariableGet"
		getRequest, err := anypb.New(&flowvariablev1.FlowVariableGetRequest{
			VariableId: variableID.Bytes(),
		})
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		changes := []*changev1.Change{
			{
				Kind:    &changeKind,
				Service: &service,
				Method:  &listMethod,
				Data:    listRequest,
			},
			{
				Kind:    &changeKind,
				Service: &service,
				Method:  &getMethod,
				Data:    getRequest,
			},
		}

	*/
	response := &flowvariablev1.FlowVariableUpdateResponse{
		// 	Changes: changes,
	}

	return connect.NewResponse(response), nil
}

func (c *FlowVariableServiceRPC) FlowVariableDelete(ctx context.Context, req *connect.Request[flowvariablev1.FlowVariableDeleteRequest]) (*connect.Response[flowvariablev1.FlowVariableDeleteResponse], error) {
	variableID, err := idwrap.NewFromBytes(req.Msg.VariableId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	variable, err := c.fvs.GetFlowVariable(ctx, variableID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	rpcErr := permcheck.CheckPerm(rflow.CheckOwnerFlow(ctx, c.fs, c.us, variable.FlowID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	err = c.fvs.DeleteFlowVariable(ctx, variableID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete flow variable: %w", err))
	}

	/*

		// Create an invalidation change for the flow variables list
		service := "flowvariable.v1.FlowVariableService"
		method := "FlowVariableList"
		changeKind := changev1.ChangeKind_CHANGE_KIND_INVALIDATE
		listRequest, err := anypb.New(&flowvariablev1.FlowVariableListRequest{
			FlowId: variable.FlowID.Bytes(),
		})
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		changes := []*changev1.Change{
			{
				Kind:    &changeKind,
				Service: &service,
				Method:  &method,
				Data:    listRequest,
			},
		}
	*/

	response := &flowvariablev1.FlowVariableDeleteResponse{
		// Changes: changes,
	}

	return connect.NewResponse(response), nil
}

func (c *FlowVariableServiceRPC) FlowVariableMove(ctx context.Context, req *connect.Request[flowvariablev1.FlowVariableMoveRequest]) (*connect.Response[flowvariablev1.FlowVariableMoveResponse], error) {
	// Validate flow ID
	flowID, err := idwrap.NewFromBytes(req.Msg.GetFlowId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

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

	// Check permissions for the flow
	rpcErr := permcheck.CheckPerm(rflow.CheckOwnerFlow(ctx, c.fs, c.us, flowID))
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
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("cannot move flow variable relative to itself"))
	}

	// Verify both variables exist and are in the same flow
	sourceVariable, err := c.fvs.GetFlowVariable(ctx, variableID)
	if err != nil {
		if err == sflowvariable.ErrNoFlowVariableFound {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("flow variable not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	targetVariable, err := c.fvs.GetFlowVariable(ctx, targetVariableID)
	if err != nil {
		if err == sflowvariable.ErrNoFlowVariableFound {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("target flow variable not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Verify both variables are in the specified flow
	if sourceVariable.FlowID.Compare(flowID) != 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow variable does not belong to the specified flow"))
	}

	if targetVariable.FlowID.Compare(flowID) != 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("target flow variable does not belong to the specified flow"))
	}

	if sourceVariable.FlowID.Compare(targetVariable.FlowID) != 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow variables must be in the same flow"))
	}

	// Execute the move operation
	switch position {
	case resourcesv1.MovePosition_MOVE_POSITION_AFTER:
		err = c.fvs.MoveFlowVariableAfter(ctx, variableID, targetVariableID)
	case resourcesv1.MovePosition_MOVE_POSITION_BEFORE:
		err = c.fvs.MoveFlowVariableBefore(ctx, variableID, targetVariableID)
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid position"))
	}

	if err != nil {
		// Map service-level errors to appropriate Connect error codes
		if err == sflowvariable.ErrNoFlowVariableFound {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&flowvariablev1.FlowVariableMoveResponse{}), nil
}
