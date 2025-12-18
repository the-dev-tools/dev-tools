//nolint:revive // exported
package rflowv2

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"connectrpc.com/connect"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

	"the-dev-tools/server/pkg/eventstream"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/service/sflowvariable"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
)

func (s *FlowServiceV2RPC) FlowVariableCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[flowv1.FlowVariableCollectionResponse], error) {
	// Get all accessible flows
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	// Collect all variables from all flows
	var allVariables []*flowv1.FlowVariable
	for _, flow := range flows {
		variables, err := s.fvs.GetFlowVariablesByFlowIDOrdered(ctx, flow.ID)
		if err != nil {
			if errors.Is(err, sflowvariable.ErrNoFlowVariableFound) {
				continue // No variables for this flow, continue to next
			} else {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		}

		for _, variable := range variables {
			allVariables = append(allVariables, serializeFlowVariable(variable))
		}
	}

	return connect.NewResponse(&flowv1.FlowVariableCollectionResponse{Items: allVariables}), nil
}

func (s *FlowServiceV2RPC) FlowVariableInsert(ctx context.Context, req *connect.Request[flowv1.FlowVariableInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		if len(item.GetFlowId()) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow id is required"))
		}

		flowID, err := idwrap.NewFromBytes(item.GetFlowId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid flow id: %w", err))
		}

		if err := s.ensureFlowAccess(ctx, flowID); err != nil {
			return nil, err
		}

		variableID := idwrap.NewNow()
		if len(item.GetFlowVariableId()) != 0 {
			variableID, err = idwrap.NewFromBytes(item.GetFlowVariableId())
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid flow variable id: %w", err))
			}
		}

		key := strings.TrimSpace(item.GetKey())
		if key == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow variable key is required"))
		}

		variable := mflow.FlowVariable{
			ID:          variableID,
			FlowID:      flowID,
			Name:        key,
			Value:       item.GetValue(),
			Enabled:     item.GetEnabled(),
			Description: item.GetDescription(),
			Order:       float64(item.GetOrder()),
		}

		if err := s.fvs.CreateFlowVariable(ctx, variable); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		s.publishFlowVariableEvent(flowVarEventInsert, variable)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) FlowVariableUpdate(ctx context.Context, req *connect.Request[flowv1.FlowVariableUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		if len(item.GetFlowVariableId()) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow variable id is required"))
		}

		variableID, err := idwrap.NewFromBytes(item.GetFlowVariableId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid flow variable id: %w", err))
		}

		variable, err := s.fvs.GetFlowVariable(ctx, variableID)
		if err != nil {
			if errors.Is(err, sflowvariable.ErrNoFlowVariableFound) {
				return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("flow variable %s not found", variableID.String()))
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := s.ensureFlowAccess(ctx, variable.FlowID); err != nil {
			return nil, err
		}

		if len(item.GetFlowId()) != 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow reassignment is not supported"))
		}

		if item.Key != nil {
			key := strings.TrimSpace(item.GetKey())
			if key == "" {
				return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow variable key cannot be empty"))
			}
			variable.Name = key
		}

		if item.Value != nil {
			variable.Value = item.GetValue()
		}

		if item.Enabled != nil {
			variable.Enabled = item.GetEnabled()
		}

		if item.Description != nil {
			variable.Description = item.GetDescription()
		}

		if item.Order != nil {
			variable.Order = float64(item.GetOrder())
			if err := s.fvs.UpdateFlowVariableOrder(ctx, variable.ID, variable.Order); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		}

		if err := s.fvs.UpdateFlowVariable(ctx, variable); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		s.publishFlowVariableEvent(flowVarEventUpdate, variable)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) FlowVariableDelete(ctx context.Context, req *connect.Request[flowv1.FlowVariableDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		variableID, err := idwrap.NewFromBytes(item.GetFlowVariableId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid flow variable id: %w", err))
		}

		variable, err := s.fvs.GetFlowVariable(ctx, variableID)
		if err != nil {
			if errors.Is(err, sflowvariable.ErrNoFlowVariableFound) {
				continue
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := s.ensureFlowAccess(ctx, variable.FlowID); err != nil {
			return nil, err
		}

		if err := s.fvs.DeleteFlowVariable(ctx, variableID); err != nil && !errors.Is(err, sflowvariable.ErrNoFlowVariableFound) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		s.publishFlowVariableEvent(flowVarEventDelete, variable)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) FlowVariableSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[flowv1.FlowVariableSyncResponse],
) error {
	if stream == nil {
		return connect.NewError(connect.CodeInternal, errors.New("stream is required"))
	}
	return s.streamFlowVariableSync(ctx, func(resp *flowv1.FlowVariableSyncResponse) error {
		return stream.Send(resp)
	})
}

func (s *FlowServiceV2RPC) streamFlowVariableSync(
	ctx context.Context,
	send func(*flowv1.FlowVariableSyncResponse) error,
) error {
	if s.varStream == nil {
		return connect.NewError(connect.CodeUnavailable, errors.New("flow variable stream not configured"))
	}

	var flowSet sync.Map

	snapshot := func(ctx context.Context) ([]eventstream.Event[FlowVariableTopic, FlowVariableEvent], error) {
		flows, err := s.listAccessibleFlows(ctx)
		if err != nil {
			return nil, err
		}

		events := make([]eventstream.Event[FlowVariableTopic, FlowVariableEvent], 0)

		for _, flow := range flows {
			flowSet.Store(flow.ID.String(), struct{}{})

			variables, err := s.fvs.GetFlowVariablesByFlowIDOrdered(ctx, flow.ID)
			if err != nil {
				if errors.Is(err, sflowvariable.ErrNoFlowVariableFound) {
					continue
				}
				return nil, err
			}

			for _, variable := range variables {
				events = append(events, eventstream.Event[FlowVariableTopic, FlowVariableEvent]{
					Topic: FlowVariableTopic{FlowID: flow.ID},
					Payload: FlowVariableEvent{
						Type:     flowVarEventInsert,
						FlowID:   flow.ID,
						Variable: variable,
					},
				})
			}
		}

		return events, nil
	}

	filter := func(topic FlowVariableTopic) bool {
		if _, ok := flowSet.Load(topic.FlowID.String()); ok {
			return true
		}
		if err := s.ensureFlowAccess(ctx, topic.FlowID); err != nil {
			return false
		}
		flowSet.Store(topic.FlowID.String(), struct{}{})
		return true
	}

	events, err := s.varStream.Subscribe(ctx, filter, eventstream.WithSnapshot(snapshot))
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	for {
		select {
		case evt, ok := <-events:
			if !ok {
				return nil
			}
			resp := flowVariableEventToSyncResponse(evt.Payload)
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

func (s *FlowServiceV2RPC) publishFlowVariableEvent(eventType string, variable mflow.FlowVariable) {
	if s.varStream == nil {
		return
	}
	s.varStream.Publish(FlowVariableTopic{FlowID: variable.FlowID}, FlowVariableEvent{
		Type:     eventType,
		FlowID:   variable.FlowID,
		Variable: variable,
	})
}

func flowVariableEventToSyncResponse(evt FlowVariableEvent) *flowv1.FlowVariableSyncResponse {
	variable := evt.Variable

	switch evt.Type {
	case flowVarEventInsert:
		insert := &flowv1.FlowVariableSyncInsert{
			FlowVariableId: variable.ID.Bytes(),
			FlowId:         variable.FlowID.Bytes(),
			Key:            variable.Name,
			Enabled:        variable.Enabled,
			Value:          variable.Value,
			Description:    variable.Description,
			Order:          float32(variable.Order),
		}
		return &flowv1.FlowVariableSyncResponse{
			Items: []*flowv1.FlowVariableSync{{
				Value: &flowv1.FlowVariableSync_ValueUnion{
					Kind:   flowv1.FlowVariableSync_ValueUnion_KIND_INSERT,
					Insert: insert,
				},
			}},
		}
	case flowVarEventUpdate:
		update := &flowv1.FlowVariableSyncUpdate{
			FlowVariableId: variable.ID.Bytes(),
		}
		if flowID := variable.FlowID.Bytes(); len(flowID) > 0 {
			update.FlowId = flowID
		}
		key := variable.Name
		update.Key = &key
		enabled := variable.Enabled
		update.Enabled = &enabled
		value := variable.Value
		update.Value = &value
		description := variable.Description
		update.Description = &description
		order := float32(variable.Order)
		update.Order = &order

		return &flowv1.FlowVariableSyncResponse{
			Items: []*flowv1.FlowVariableSync{{
				Value: &flowv1.FlowVariableSync_ValueUnion{
					Kind:   flowv1.FlowVariableSync_ValueUnion_KIND_UPDATE,
					Update: update,
				},
			}},
		}
	case flowVarEventDelete:
		return &flowv1.FlowVariableSyncResponse{
			Items: []*flowv1.FlowVariableSync{{
				Value: &flowv1.FlowVariableSync_ValueUnion{
					Kind: flowv1.FlowVariableSync_ValueUnion_KIND_DELETE,
					Delete: &flowv1.FlowVariableSyncDelete{
						FlowVariableId: variable.ID.Bytes(),
					},
				},
			}},
		}
	default:
		return nil
	}
}
