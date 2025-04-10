package rflowvariable

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/anypb"

	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/rflow"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/permcheck"

	"the-dev-tools/server/pkg/model/mflowvariable"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sflowvariable"
	"the-dev-tools/server/pkg/service/suser"

	"the-dev-tools/server/pkg/translate/tflowvariable"

	changev1 "the-dev-tools/spec/dist/buf/go/change/v1"
	flowvariablev1 "the-dev-tools/spec/dist/buf/go/flowvariable/v1"
	"the-dev-tools/spec/dist/buf/go/flowvariable/v1/flowvariablev1connect"
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

	variables, err := c.fvs.GetFlowVariablesByFlowID(ctx, flowID)
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

	response := &flowvariablev1.FlowVariableCreateResponse{
		VariableId: variableID.Bytes(),
		Changes:    changes,
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

	response := &flowvariablev1.FlowVariableUpdateResponse{
		Changes: changes,
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

	response := &flowvariablev1.FlowVariableDeleteResponse{
		Changes: changes,
	}

	return connect.NewResponse(response), nil
}
