//nolint:revive // exported
package rhttp

import (
	"context"
	"errors"
	"sync"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	devtoolsdb "the-dev-tools/db"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/converter"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"

	"the-dev-tools/server/pkg/service/shttp"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
)

func (h *HttpServiceRPC) HttpSearchParamDeltaCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpSearchParamDeltaCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Get user's workspaces
	workspaces, err := h.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var allDeltas []*apiv1.HttpSearchParamDelta
	for _, workspace := range workspaces {
		// Get HTTP entries for this workspace
		httpList, err := h.hs.GetDeltasByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Get params for each HTTP entry
		for _, http := range httpList {
			params, err := h.httpSearchParamService.GetByHttpIDOrdered(ctx, http.ID)
			if err != nil {
				if errors.Is(err, shttp.ErrNoHttpSearchParamFound) {
					continue
				}
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			// Convert to delta format
			for _, param := range params {
				if !param.IsDelta {
					continue
				}

				delta := &apiv1.HttpSearchParamDelta{
					DeltaHttpSearchParamId: param.ID.Bytes(),
					HttpId:                 param.HttpID.Bytes(),
				}

				if param.ParentHttpSearchParamID != nil {
					delta.HttpSearchParamId = param.ParentHttpSearchParamID.Bytes()
				}

				// Only include delta fields if they exist
				if param.DeltaKey != nil {
					delta.Key = param.DeltaKey
				}
				if param.DeltaValue != nil {
					delta.Value = param.DeltaValue
				}
				if param.DeltaEnabled != nil {
					delta.Enabled = param.DeltaEnabled
				}
				if param.DeltaDescription != nil {
					delta.Description = param.DeltaDescription
				}
				if param.DeltaOrder != nil {
					order := float32(*param.DeltaOrder)
					delta.Order = &order
				}

				allDeltas = append(allDeltas, delta)
			}
		}
	}

	return connect.NewResponse(&apiv1.HttpSearchParamDeltaCollectionResponse{
		Items: allDeltas,
	}), nil
}

func (h *HttpServiceRPC) HttpSearchParamDeltaInsert(ctx context.Context, req *connect.Request[apiv1.HttpSearchParamDeltaInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.Items) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one delta item is required"))
	}

	// Process each delta item
	for _, item := range req.Msg.Items {
		if len(item.HttpSearchParamId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_search_param_id is required for each delta item"))
		}

		paramID, err := idwrap.NewFromBytes(item.HttpSearchParamId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Check workspace write access
		param, err := h.httpSearchParamService.GetByID(ctx, paramID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHttpSearchParamFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		httpEntry, err := h.hs.Get(ctx, param.HttpID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := h.checkWorkspaceWriteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		// Update delta fields
		var deltaOrder *float64
		if item.Order != nil {
			order := float64(*item.Order)
			deltaOrder = &order
		}
		err = h.httpSearchParamService.UpdateDelta(ctx, paramID, item.Key, item.Value, item.Enabled, item.Description, deltaOrder)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpSearchParamDeltaUpdate(ctx context.Context, req *connect.Request[apiv1.HttpSearchParamDeltaUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP search param delta must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var updateData []struct {
		deltaID       idwrap.IDWrap
		existingParam *mhttp.HTTPSearchParam
		item          *apiv1.HttpSearchParamDeltaUpdate
	}

	for _, item := range req.Msg.Items {
		if len(item.DeltaHttpSearchParamId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("delta_http_search_param_id is required"))
		}

		deltaID, err := idwrap.NewFromBytes(item.DeltaHttpSearchParamId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing delta param - use pool service
		existingParam, err := h.httpSearchParamService.GetByID(ctx, deltaID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHttpSearchParamFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Verify this is actually a delta record
		if !existingParam.IsDelta {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("specified HTTP search param is not a delta"))
		}

		// Get the HTTP entry to check workspace access - use pool service
		httpEntry, err := h.hs.Get(ctx, existingParam.HttpID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Check write access to the workspace
		if err := h.checkWorkspaceWriteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		updateData = append(updateData, struct {
			deltaID       idwrap.IDWrap
			existingParam *mhttp.HTTPSearchParam
			item          *apiv1.HttpSearchParamDeltaUpdate
		}{
			deltaID:       deltaID,
			existingParam: existingParam,
			item:          item,
		})
	}

	// Step 2: Prepare updates (in memory)
	var preparedUpdates []struct {
		deltaID          idwrap.IDWrap
		deltaKey         *string
		deltaValue       *string
		deltaEnabled     *bool
		deltaDescription *string
	}

	for _, data := range updateData {
		item := data.item
		var deltaKey, deltaValue, deltaDescription *string
		var deltaEnabled *bool

		if item.Key != nil {
			switch item.Key.GetKind() {
			case apiv1.HttpSearchParamDeltaUpdate_KeyUnion_KIND_UNSET:
				deltaKey = nil
			case apiv1.HttpSearchParamDeltaUpdate_KeyUnion_KIND_VALUE:
				keyStr := item.Key.GetValue()
				deltaKey = &keyStr
			}
		}
		if item.Value != nil {
			switch item.Value.GetKind() {
			case apiv1.HttpSearchParamDeltaUpdate_ValueUnion_KIND_UNSET:
				deltaValue = nil
			case apiv1.HttpSearchParamDeltaUpdate_ValueUnion_KIND_VALUE:
				valueStr := item.Value.GetValue()
				deltaValue = &valueStr
			}
		}
		if item.Enabled != nil {
			switch item.Enabled.GetKind() {
			case apiv1.HttpSearchParamDeltaUpdate_EnabledUnion_KIND_UNSET:
				deltaEnabled = nil
			case apiv1.HttpSearchParamDeltaUpdate_EnabledUnion_KIND_VALUE:
				enabledBool := item.Enabled.GetValue()
				deltaEnabled = &enabledBool
			}
		}
		if item.Description != nil {
			switch item.Description.GetKind() {
			case apiv1.HttpSearchParamDeltaUpdate_DescriptionUnion_KIND_UNSET:
				deltaDescription = nil
			case apiv1.HttpSearchParamDeltaUpdate_DescriptionUnion_KIND_VALUE:
				descStr := item.Description.GetValue()
				deltaDescription = &descStr
			}
		}

		preparedUpdates = append(preparedUpdates, struct {
			deltaID          idwrap.IDWrap
			deltaKey         *string
			deltaValue       *string
			deltaEnabled     *bool
			deltaDescription *string
		}{
			deltaID:          data.deltaID,
			deltaKey:         deltaKey,
			deltaValue:       deltaValue,
			deltaEnabled:     deltaEnabled,
			deltaDescription: deltaDescription,
		})
	}

	// Step 3: Execute updates in transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	httpSearchParamService := h.httpSearchParamService.TX(tx)
	var updatedParams []mhttp.HTTPSearchParam

	for _, update := range preparedUpdates {
		if err := httpSearchParamService.UpdateDelta(ctx, update.deltaID, update.deltaKey, update.deltaValue, update.deltaEnabled, update.deltaDescription, nil); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Get updated param for event publishing (must get from TX service to see changes)
		updatedParam, err := httpSearchParamService.GetByID(ctx, update.deltaID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		updatedParams = append(updatedParams, *updatedParam)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish update events for real-time sync after successful commit
	for _, param := range updatedParams {
		// Get workspace ID for the HTTP entry
		httpEntry, err := h.hs.Get(ctx, param.HttpID)
		if err != nil {
			continue
		}
		h.httpSearchParamStream.Publish(HttpSearchParamTopic{WorkspaceID: httpEntry.WorkspaceID}, HttpSearchParamEvent{
			Type:            eventTypeUpdate,
			HttpSearchParam: converter.ToAPIHttpSearchParam(param),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpSearchParamDeltaDelete(ctx context.Context, req *connect.Request[apiv1.HttpSearchParamDeltaDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP search param delta must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var deleteData []struct {
		deltaID       idwrap.IDWrap
		existingParam *mhttp.HTTPSearchParam
		workspaceID   idwrap.IDWrap
	}

	for _, item := range req.Msg.Items {
		if len(item.DeltaHttpSearchParamId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("delta_http_search_param_id is required"))
		}

		deltaID, err := idwrap.NewFromBytes(item.DeltaHttpSearchParamId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing delta param - use pool service
		existingParam, err := h.httpSearchParamService.GetByID(ctx, deltaID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHttpSearchParamFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Verify this is actually a delta record
		if !existingParam.IsDelta {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("specified HTTP search param is not a delta"))
		}

		// Get the HTTP entry to check workspace access - use pool service
		httpEntry, err := h.hs.Get(ctx, existingParam.HttpID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Check delete access to the workspace
		if err := h.checkWorkspaceDeleteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		deleteData = append(deleteData, struct {
			deltaID       idwrap.IDWrap
			existingParam *mhttp.HTTPSearchParam
			workspaceID   idwrap.IDWrap
		}{
			deltaID:       deltaID,
			existingParam: existingParam,
			workspaceID:   httpEntry.WorkspaceID,
		})
	}

	// Step 2: Execute deletes in transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	httpSearchParamService := h.httpSearchParamService.TX(tx)
	var deletedParams []mhttp.HTTPSearchParam
	var deletedWorkspaceIDs []idwrap.IDWrap

	for _, data := range deleteData {
		// Delete the delta record
		if err := httpSearchParamService.Delete(ctx, data.deltaID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		deletedParams = append(deletedParams, *data.existingParam)
		deletedWorkspaceIDs = append(deletedWorkspaceIDs, data.workspaceID)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish delete events for real-time sync after successful commit
	for i, param := range deletedParams {
		h.httpSearchParamStream.Publish(HttpSearchParamTopic{WorkspaceID: deletedWorkspaceIDs[i]}, HttpSearchParamEvent{
			Type:            eventTypeDelete,
			HttpSearchParam: converter.ToAPIHttpSearchParam(param),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpSearchParamDeltaSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpSearchParamDeltaSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return h.streamHttpSearchParamDeltaSync(ctx, userID, stream.Send)
}

func (h *HttpServiceRPC) streamHttpSearchParamDeltaSync(ctx context.Context, userID idwrap.IDWrap, send func(*apiv1.HttpSearchParamDeltaSyncResponse) error) error {
	var workspaceSet sync.Map

	// Filter for workspace-based access control
	filter := func(topic HttpSearchParamTopic) bool {
		if _, ok := workspaceSet.Load(topic.WorkspaceID.String()); ok {
			return true
		}
		belongs, err := h.us.CheckUserBelongsToWorkspace(ctx, userID, topic.WorkspaceID)
		if err != nil || !belongs {
			return false
		}
		workspaceSet.Store(topic.WorkspaceID.String(), struct{}{})
		return true
	}

	// Subscribe to events without snapshot
	events, err := h.httpSearchParamStream.Subscribe(ctx, filter)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	// Stream events to client
	for {
		select {
		case evt, ok := <-events:
			if !ok {
				return nil
			}
			// Get the full param record for delta sync response
			paramID, err := idwrap.NewFromBytes(evt.Payload.HttpSearchParam.GetHttpSearchParamId())
			if err != nil {
				continue // Skip if can't parse ID
			}
			paramRecord, err := h.httpSearchParamService.GetByID(ctx, paramID)
			if err != nil {
				continue // Skip if can't get the record
			}
			if !paramRecord.IsDelta {
				continue
			}
			resp := httpSearchParamDeltaSyncResponseFrom(evt.Payload, *paramRecord)
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
