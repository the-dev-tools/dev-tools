//nolint:revive // exported
package rhttp

import (
	"context"
	"errors"
	"sync"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/mutation"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/patch"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/shttp"
	apiv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/http/v1"
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
				if param.DeltaDisplayOrder != nil {
					order := float32(*param.DeltaDisplayOrder)
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

	// FETCH: Gather data and check permissions OUTSIDE transaction
	type insertItem struct {
		httpID      idwrap.IDWrap
		newID       idwrap.IDWrap
		parentID    idwrap.IDWrap
		workspaceID idwrap.IDWrap
		baseParam   mhttp.HTTPSearchParam
		item        *apiv1.HttpSearchParamDeltaInsert
	}
	insertData := make([]insertItem, 0, len(req.Msg.Items))

	for _, item := range req.Msg.Items {
		if len(item.HttpId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_id is required for each delta item"))
		}

		httpID, err := idwrap.NewFromBytes(item.HttpId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		httpEntry, err := h.hs.Get(ctx, httpID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHTTPFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if !httpEntry.IsDelta {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("specified HTTP entry is not a delta"))
		}

		if err := h.checkWorkspaceWriteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		if len(item.HttpSearchParamId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_search_param_id is required"))
		}

		parentParamID, err := idwrap.NewFromBytes(item.HttpSearchParamId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		baseParam, err := h.httpSearchParamService.GetByID(ctx, parentParamID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHttpSearchParamFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		newID := idwrap.NewNow()
		if len(item.DeltaHttpSearchParamId) > 0 {
			newID, err = idwrap.NewFromBytes(item.DeltaHttpSearchParamId)
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, err)
			}
		}

		insertData = append(insertData, insertItem{
			httpID:      httpID,
			newID:       newID,
			parentID:    parentParamID,
			workspaceID: httpEntry.WorkspaceID,
			baseParam:   *baseParam,
			item:        item,
		})
	}

	// ACT: Insert new delta records using mutation context
	mut := mutation.New(h.DB, mutation.WithPublisher(h.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	now := time.Now().UnixMilli()
	for _, data := range insertData {
		var deltaOrder *float64
		if data.item.Order != nil {
			order := float64(*data.item.Order)
			deltaOrder = &order
		}

		params := gen.CreateHTTPSearchParamParams{
			ID:                      data.newID,
			HttpID:                  data.httpID,
			Key:                     data.baseParam.Key,
			Value:                   data.baseParam.Value,
			Description:             data.baseParam.Description,
			Enabled:                 data.baseParam.Enabled,
			DisplayOrder:            float64(data.baseParam.DisplayOrder),
			ParentHttpSearchParamID: data.parentID.Bytes(),
			IsDelta:                 true,
			DeltaKey:                ptrToNullString(data.item.Key),
			DeltaValue:              ptrToNullString(data.item.Value),
			DeltaDescription:        data.item.Description,
			DeltaEnabled:            data.item.Enabled,
			DeltaDisplayOrder:       ptrToNullFloat64(ptrToFloat32(deltaOrder)),
			CreatedAt:               now,
			UpdatedAt:               now,
		}

		if err := mut.InsertHTTPSearchParam(ctx, mutation.HTTPSearchParamInsertItem{
			ID:          data.newID,
			HttpID:      data.httpID,
			WorkspaceID: data.workspaceID,
			IsDelta:     true,
			Params:      params,
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		paramService := h.httpSearchParamService.TX(mut.TX())
		updated, err := paramService.GetByID(ctx, data.newID)
		if err == nil {
			mut.UpdateLastEventPayload(*updated)
		}
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func ptrToFloat32(f *float64) *float32 {
	if f == nil {
		return nil
	}
	v := float32(*f)
	return &v
}

func (h *HttpServiceRPC) HttpSearchParamDeltaUpdate(ctx context.Context, req *connect.Request[apiv1.HttpSearchParamDeltaUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP search param delta must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	type updateItem struct {
		deltaID       idwrap.IDWrap
		existingParam mhttp.HTTPSearchParam
		workspaceID   idwrap.IDWrap
		item          *apiv1.HttpSearchParamDeltaUpdate
	}
	updateData := make([]updateItem, 0, len(req.Msg.Items))

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

		updateData = append(updateData, updateItem{
			deltaID:       deltaID,
			existingParam: *existingParam,
			workspaceID:   httpEntry.WorkspaceID,
			item:          item,
		})
	}

	// ACT: Update using mutation context
	mut := mutation.New(h.DB, mutation.WithPublisher(h.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	for _, data := range updateData {
		item := data.item
		deltaKey := data.existingParam.DeltaKey
		deltaValue := data.existingParam.DeltaValue
		deltaDescription := data.existingParam.DeltaDescription
		deltaEnabled := data.existingParam.DeltaEnabled
		deltaOrder := data.existingParam.DeltaDisplayOrder
		var patchData patch.HTTPSearchParamPatch

		if item.Key != nil {
			switch item.Key.GetKind() {
			case apiv1.HttpSearchParamDeltaUpdate_KeyUnion_KIND_UNSET:
				deltaKey = nil
				patchData.Key = patch.Unset[string]()
			case apiv1.HttpSearchParamDeltaUpdate_KeyUnion_KIND_VALUE:
				keyStr := item.Key.GetValue()
				deltaKey = &keyStr
				patchData.Key = patch.NewOptional(keyStr)
			}
		}
		if item.Value != nil {
			switch item.Value.GetKind() {
			case apiv1.HttpSearchParamDeltaUpdate_ValueUnion_KIND_UNSET:
				deltaValue = nil
				patchData.Value = patch.Unset[string]()
			case apiv1.HttpSearchParamDeltaUpdate_ValueUnion_KIND_VALUE:
				valueStr := item.Value.GetValue()
				deltaValue = &valueStr
				patchData.Value = patch.NewOptional(valueStr)
			}
		}
		if item.Enabled != nil {
			switch item.Enabled.GetKind() {
			case apiv1.HttpSearchParamDeltaUpdate_EnabledUnion_KIND_UNSET:
				deltaEnabled = nil
				patchData.Enabled = patch.Unset[bool]()
			case apiv1.HttpSearchParamDeltaUpdate_EnabledUnion_KIND_VALUE:
				enabledBool := item.Enabled.GetValue()
				deltaEnabled = &enabledBool
				patchData.Enabled = patch.NewOptional(enabledBool)
			}
		}
		if item.Description != nil {
			switch item.Description.GetKind() {
			case apiv1.HttpSearchParamDeltaUpdate_DescriptionUnion_KIND_UNSET:
				deltaDescription = nil
				patchData.Description = patch.Unset[string]()
			case apiv1.HttpSearchParamDeltaUpdate_DescriptionUnion_KIND_VALUE:
				descStr := item.Description.GetValue()
				deltaDescription = &descStr
				patchData.Description = patch.NewOptional(descStr)
			}
		}
		if item.Order != nil {
			switch item.Order.GetKind() {
			case apiv1.HttpSearchParamDeltaUpdate_OrderUnion_KIND_UNSET:
				deltaOrder = nil
				patchData.Order = patch.Unset[float32]()
			case apiv1.HttpSearchParamDeltaUpdate_OrderUnion_KIND_VALUE:
				orderFloat := float64(item.Order.GetValue())
				deltaOrder = &orderFloat
				// Store as float32 in patch for sync converter compatibility
				orderFloat32 := float32(orderFloat)
				patchData.Order = patch.NewOptional(orderFloat32)
			}
		}

		paramService := h.httpSearchParamService.TX(mut.TX())
		if err := mut.UpdateHTTPSearchParamDelta(ctx, mutation.HTTPSearchParamDeltaUpdateItem{
			ID:          data.deltaID,
			HttpID:      data.existingParam.HttpID,
			WorkspaceID: data.workspaceID,
			Params: gen.UpdateHTTPSearchParamDeltaParams{
				ID:                data.deltaID,
				DeltaKey:          ptrToNullString(deltaKey),
				DeltaValue:        ptrToNullString(deltaValue),
				DeltaEnabled:      deltaEnabled,
				DeltaDescription:  deltaDescription,
				DeltaDisplayOrder: ptrToNullFloat64(ptrToFloat32(deltaOrder)),
			},
			Patch: patchData,
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Update payload in tracked event
		updated, err := paramService.GetByID(ctx, data.deltaID)
		if err == nil {
			mut.UpdateLastEventPayload(*updated)
		}
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpSearchParamDeltaDelete(ctx context.Context, req *connect.Request[apiv1.HttpSearchParamDeltaDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP search param delta must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	type deleteItem struct {
		deltaID     idwrap.IDWrap
		httpID      idwrap.IDWrap
		workspaceID idwrap.IDWrap
		param       mhttp.HTTPSearchParam
	}
	deleteData := make([]deleteItem, 0, len(req.Msg.Items))

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

		deleteData = append(deleteData, deleteItem{
			deltaID:     deltaID,
			httpID:      existingParam.HttpID,
			workspaceID: httpEntry.WorkspaceID,
			param:       *existingParam,
		})
	}

	// Step 2: Execute deletes in transaction
	mut := mutation.New(h.DB, mutation.WithPublisher(h.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	for _, data := range deleteData {
		mut.Track(mutation.Event{
			Entity:      mutation.EntityHTTPParam,
			Op:          mutation.OpDelete,
			ID:          data.deltaID,
			ParentID:    data.httpID,
			WorkspaceID: data.workspaceID,
			IsDelta:     true,
			Payload:     data.param,
		})
		if err := mut.Queries().DeleteHTTPSearchParam(ctx, data.deltaID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
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
	events, err := h.streamers.HttpSearchParam.Subscribe(ctx, filter)
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
