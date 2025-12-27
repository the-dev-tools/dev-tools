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

	devtoolsdb "the-dev-tools/db"
	"the-dev-tools/server/pkg/eventstream"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/patch"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/txutil"
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
			if errors.Is(err, sflow.ErrNoFlowVariableFound) {
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
	// 1. Move validation OUTSIDE transaction (before BeginTx)
	var validatedVariables []mflow.FlowVariable

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

		validatedVariables = append(validatedVariables, variable)
	}

	// 2. Begin transaction with bulk sync wrapper
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	syncTx := txutil.NewBulkInsertTx[variableWithFlow, FlowVariableTopic](
		tx,
		func(vwf variableWithFlow) FlowVariableTopic {
			return FlowVariableTopic{FlowID: vwf.flowID}
		},
	)

	varWriter := s.fvs.TX(tx)

	// 3. Execute all inserts in transaction
	for _, variable := range validatedVariables {
		if err := varWriter.CreateFlowVariable(ctx, variable); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		syncTx.Track(variableWithFlow{
			variable: variable,
			flowID:   variable.FlowID,
		})
	}

	// 4. Commit transaction and publish events in bulk
	if err := syncTx.CommitAndPublish(ctx, s.publishBulkFlowVariableInsert); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) FlowVariableUpdate(ctx context.Context, req *connect.Request[flowv1.FlowVariableUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	// 1. Move validation OUTSIDE transaction (before BeginTx)
	type updateData struct {
		variable      mflow.FlowVariable
		variablePatch patch.FlowVariablePatch
		updateOrder   bool
	}
	var validatedUpdates []updateData

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
			if errors.Is(err, sflow.ErrNoFlowVariableFound) {
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

		variablePatch := patch.FlowVariablePatch{}

		if item.Key != nil {
			key := strings.TrimSpace(item.GetKey())
			if key == "" {
				return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow variable key cannot be empty"))
			}
			variable.Name = key
			variablePatch.Name = patch.NewOptional(key)
		}

		if item.Value != nil {
			variable.Value = item.GetValue()
			variablePatch.Value = patch.NewOptional(item.GetValue())
		}

		if item.Enabled != nil {
			variable.Enabled = item.GetEnabled()
			variablePatch.Enabled = patch.NewOptional(item.GetEnabled())
		}

		if item.Description != nil {
			variable.Description = item.GetDescription()
			variablePatch.Description = patch.NewOptional(item.GetDescription())
		}

		updateOrder := false
		if item.Order != nil {
			variable.Order = float64(item.GetOrder())
			variablePatch.Order = patch.NewOptional(variable.Order)
			updateOrder = true
		}

		validatedUpdates = append(validatedUpdates, updateData{
			variable:      variable,
			variablePatch: variablePatch,
			updateOrder:   updateOrder,
		})
	}

	// 2. Begin transaction with bulk sync wrapper
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	syncTx := txutil.NewBulkUpdateTx[variableWithFlow, patch.FlowVariablePatch, FlowVariableTopic](
		tx,
		func(vwf variableWithFlow) FlowVariableTopic {
			return FlowVariableTopic{FlowID: vwf.flowID}
		},
	)

	varWriter := s.fvs.TX(tx)

	// 3. Execute all updates in transaction
	for _, data := range validatedUpdates {
		if data.updateOrder {
			if err := varWriter.UpdateFlowVariableOrder(ctx, data.variable.ID, data.variable.Order); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		}

		if err := varWriter.UpdateFlowVariable(ctx, data.variable); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		syncTx.Track(
			variableWithFlow{
				variable: data.variable,
				flowID:   data.variable.FlowID,
			},
			data.variablePatch,
		)
	}

	// 4. Commit transaction and publish events in bulk
	if err := syncTx.CommitAndPublish(ctx, s.publishBulkFlowVariableUpdate); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) FlowVariableDelete(ctx context.Context, req *connect.Request[flowv1.FlowVariableDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	// 1. Move validation OUTSIDE transaction (before BeginTx)
	type deleteData struct {
		variableID idwrap.IDWrap
		variable   mflow.FlowVariable
	}
	var validatedDeletes []deleteData

	for _, item := range req.Msg.GetItems() {
		variableID, err := idwrap.NewFromBytes(item.GetFlowVariableId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid flow variable id: %w", err))
		}

		variable, err := s.fvs.GetFlowVariable(ctx, variableID)
		if err != nil {
			if errors.Is(err, sflow.ErrNoFlowVariableFound) {
				continue
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := s.ensureFlowAccess(ctx, variable.FlowID); err != nil {
			return nil, err
		}

		validatedDeletes = append(validatedDeletes, deleteData{
			variableID: variableID,
			variable:   variable,
		})
	}

	// 2. Begin transaction
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	varWriter := s.fvs.TX(tx)
	var deletedVariables []mflow.FlowVariable

	// 3. Execute all deletes in transaction
	for _, data := range validatedDeletes {
		if err := varWriter.DeleteFlowVariable(ctx, data.variableID); err != nil && !errors.Is(err, sflow.ErrNoFlowVariableFound) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		deletedVariables = append(deletedVariables, data.variable)
	}

	// 4. Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// 5. Publish events AFTER successful commit
	for _, variable := range deletedVariables {
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
				if errors.Is(err, sflow.ErrNoFlowVariableFound) {
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
