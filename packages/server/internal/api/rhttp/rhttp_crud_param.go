//nolint:revive // exported
package rhttp

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	devtoolsdb "the-dev-tools/db"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/converter"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/patch"
	"the-dev-tools/server/pkg/txutil"

	"the-dev-tools/server/pkg/service/shttp"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
)

// paramWithWorkspace is a context carrier that pairs a search param with its workspace ID.
// This is needed because HTTPSearchParam doesn't store WorkspaceID directly, but we need it
// for topic extraction during bulk sync event publishing.
type paramWithWorkspace struct {
	param       mhttp.HTTPSearchParam
	workspaceID idwrap.IDWrap
}

// publishBulkSearchParamInsert publishes multiple search param insert events in bulk.
// Items are already grouped by HttpSearchParamTopic by the BulkSyncTxInsert wrapper.
func (h *HttpServiceRPC) publishBulkSearchParamInsert(
	topic HttpSearchParamTopic,
	items []paramWithWorkspace,
) {
	// Convert to event slice for variadic publish
	events := make([]HttpSearchParamEvent, len(items))
	for i, item := range items {
		events[i] = HttpSearchParamEvent{
			Type:            eventTypeInsert,
			IsDelta:         item.param.IsDelta,
			HttpSearchParam: converter.ToAPIHttpSearchParam(item.param),
		}
	}

	// Single bulk publish for entire batch
	h.streamers.HttpSearchParam.Publish(topic, events...)
}

// publishBulkSearchParamUpdate publishes multiple search param update events in bulk.
// Items are already grouped by HttpSearchParamTopic by the BulkSyncTxUpdate wrapper.
func (h *HttpServiceRPC) publishBulkSearchParamUpdate(
	topic HttpSearchParamTopic,
	events []txutil.UpdateEvent[paramWithWorkspace, patch.HTTPSearchParamPatch],
) {
	paramEvents := make([]HttpSearchParamEvent, len(events))
	for i, evt := range events {
		paramEvents[i] = HttpSearchParamEvent{
			Type:            eventTypeUpdate,
			IsDelta:         evt.Item.param.IsDelta,
			HttpSearchParam: converter.ToAPIHttpSearchParam(evt.Item.param),
			Patch:           evt.Patch, // Partial updates preserved!
		}
	}
	h.streamers.HttpSearchParam.Publish(topic, paramEvents...)
}

// publishBulkSearchParamDelete publishes multiple search param delete events in bulk.
// Items are already grouped by HttpSearchParamTopic by the BulkSyncTxDelete wrapper.
func (h *HttpServiceRPC) publishBulkSearchParamDelete(
	topic HttpSearchParamTopic,
	events []txutil.DeleteEvent[idwrap.IDWrap],
) {
	paramEvents := make([]HttpSearchParamEvent, len(events))
	for i, evt := range events {
		paramEvents[i] = HttpSearchParamEvent{
			Type:    eventTypeDelete,
			IsDelta: evt.IsDelta,
			HttpSearchParam: &apiv1.HttpSearchParam{
				HttpSearchParamId: evt.ID.Bytes(),
			},
		}
	}
	h.streamers.HttpSearchParam.Publish(topic, paramEvents...)
}

func (h *HttpServiceRPC) HttpSearchParamCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpSearchParamCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Get user's workspaces
	workspaces, err := h.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var allParams []*apiv1.HttpSearchParam
	for _, workspace := range workspaces {
		// Get HTTP entries for this workspace
		httpList, err := h.hs.GetByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Get search params for each HTTP entry
		for _, http := range httpList {
			params, err := h.httpSearchParamService.GetByHttpIDOrdered(ctx, http.ID)
			if err != nil {
				if errors.Is(err, shttp.ErrNoHttpSearchParamFound) {
					continue
				}
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			// Convert to API format
			for _, param := range params {
				apiParam := converter.ToAPIHttpSearchParam(param)
				allParams = append(allParams, apiParam)
			}
		}
	}

	return connect.NewResponse(&apiv1.HttpSearchParamCollectionResponse{Items: allParams}), nil
}

func (h *HttpServiceRPC) HttpSearchParamInsert(ctx context.Context, req *connect.Request[apiv1.HttpSearchParamInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP search param must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var insertData []struct {
		paramModel  *mhttp.HTTPSearchParam
		workspaceID idwrap.IDWrap
	}

	for _, item := range req.Msg.Items {
		if len(item.HttpSearchParamId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_search_param_id is required"))
		}
		if len(item.HttpId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_id is required"))
		}

		paramID, err := idwrap.NewFromBytes(item.HttpSearchParamId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		httpID, err := idwrap.NewFromBytes(item.HttpId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Verify the HTTP entry exists and user has access - use pool service
		httpEntry, err := h.hs.Get(ctx, httpID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHTTPFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Check write access to the workspace
		if err := h.checkWorkspaceWriteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		// Create the param model
		paramModel := &mhttp.HTTPSearchParam{
			ID:           paramID,
			HttpID:       httpID,
			Key:          item.Key,
			Value:        item.Value,
			Enabled:      item.Enabled,
			Description:  item.Description,
			DisplayOrder: float64(item.Order),
		}

		insertData = append(insertData, struct {
			paramModel  *mhttp.HTTPSearchParam
			workspaceID idwrap.IDWrap
		}{
			paramModel:  paramModel,
			workspaceID: httpEntry.WorkspaceID,
		})
	}

	// Step 2: Minimal write transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	// Create bulk sync wrapper with topic extractor
	syncTx := txutil.NewBulkInsertTx[paramWithWorkspace, HttpSearchParamTopic](
		tx,
		func(pww paramWithWorkspace) HttpSearchParamTopic {
			return HttpSearchParamTopic{WorkspaceID: pww.workspaceID}
		},
	)

	httpSearchParamService := h.httpSearchParamService.TX(tx)

	for _, data := range insertData {
		if err := httpSearchParamService.Create(ctx, data.paramModel); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Track with workspace context for bulk sync
		syncTx.Track(paramWithWorkspace{
			param:       *data.paramModel,
			workspaceID: data.workspaceID,
		})
	}

	// Step 3: Commit and bulk publish sync events (grouped by topic)
	if err := syncTx.CommitAndPublish(ctx, h.publishBulkSearchParamInsert); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpSearchParamUpdate(ctx context.Context, req *connect.Request[apiv1.HttpSearchParamUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP search param must be provided"))
	}

	// Step 1: Process request data and perform all reads/checks OUTSIDE transaction
	var updateData []struct {
		existingParam *mhttp.HTTPSearchParam
		key           *string
		value         *string
		enabled       *bool
		description   *string
		order         *float32
		workspaceID   idwrap.IDWrap
	}

	for _, item := range req.Msg.Items {
		if len(item.HttpSearchParamId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_search_param_id is required"))
		}

		paramID, err := idwrap.NewFromBytes(item.HttpSearchParamId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing param - use pool service
		existingParam, err := h.httpSearchParamService.GetByID(ctx, paramID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHttpSearchParamFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Get the HTTP entry to check workspace access
		httpEntry, err := h.hs.Get(ctx, existingParam.HttpID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Check write access to the workspace
		if err := h.checkWorkspaceWriteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		updateData = append(updateData, struct {
			existingParam *mhttp.HTTPSearchParam
			key           *string
			value         *string
			enabled       *bool
			description   *string
			order         *float32
			workspaceID   idwrap.IDWrap
		}{
			existingParam: existingParam,
			key:           item.Key,
			value:         item.Value,
			enabled:       item.Enabled,
			description:   item.Description,
			order:         item.Order,
			workspaceID:   httpEntry.WorkspaceID,
		})
	}

	// Step 2: Minimal write transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	// Create bulk sync wrapper with topic extractor
	syncTx := txutil.NewBulkUpdateTx[paramWithWorkspace, patch.HTTPSearchParamPatch, HttpSearchParamTopic](
		tx,
		func(pww paramWithWorkspace) HttpSearchParamTopic {
			return HttpSearchParamTopic{WorkspaceID: pww.workspaceID}
		},
	)

	httpSearchParamService := h.httpSearchParamService.TX(tx)

	for _, data := range updateData {
		param := *data.existingParam

		// Build patch with only changed fields
		paramPatch := patch.HTTPSearchParamPatch{}

		// Update fields if provided and track in patch
		if data.key != nil {
			param.Key = *data.key
			paramPatch.Key = patch.NewOptional(*data.key)
		}
		if data.value != nil {
			param.Value = *data.value
			paramPatch.Value = patch.NewOptional(*data.value)
		}
		if data.enabled != nil {
			param.Enabled = *data.enabled
			paramPatch.Enabled = patch.NewOptional(*data.enabled)
		}
		if data.description != nil {
			param.Description = *data.description
			paramPatch.Description = patch.NewOptional(*data.description)
		}
		if data.order != nil {
			param.DisplayOrder = float64(*data.order)
			paramPatch.Order = patch.NewOptional(*data.order)
		}

		if err := httpSearchParamService.Update(ctx, &param); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Update order if changed
		if data.order != nil {
			if err := httpSearchParamService.UpdateOrder(ctx, param.ID, param.HttpID, param.DisplayOrder); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		}

		// Track with workspace context and patch
		syncTx.Track(
			paramWithWorkspace{
				param:       param,
				workspaceID: data.workspaceID,
			},
			paramPatch,
		)
	}

	// Step 3: Commit and bulk publish (grouped by workspace)
	if err := syncTx.CommitAndPublish(ctx, h.publishBulkSearchParamUpdate); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpSearchParamDelete(ctx context.Context, req *connect.Request[apiv1.HttpSearchParamDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP search param must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var deleteData []struct {
		paramID       idwrap.IDWrap
		existingParam *mhttp.HTTPSearchParam
		workspaceID   idwrap.IDWrap
	}

	for _, item := range req.Msg.Items {
		if len(item.HttpSearchParamId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_search_param_id is required"))
		}

		paramID, err := idwrap.NewFromBytes(item.HttpSearchParamId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing param - use pool service
		existingParam, err := h.httpSearchParamService.GetByID(ctx, paramID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHttpSearchParamFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Get the HTTP entry to check workspace access
		httpEntry, err := h.hs.Get(ctx, existingParam.HttpID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Check delete access to the workspace
		if err := h.checkWorkspaceDeleteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		deleteData = append(deleteData, struct {
			paramID       idwrap.IDWrap
			existingParam *mhttp.HTTPSearchParam
			workspaceID   idwrap.IDWrap
		}{
			paramID:       paramID,
			existingParam: existingParam,
			workspaceID:   httpEntry.WorkspaceID,
		})
	}

	// Step 2: Minimal write transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	// Create bulk sync wrapper with topic extractor
	syncTx := txutil.NewBulkDeleteTx[idwrap.IDWrap, HttpSearchParamTopic](
		tx,
		func(evt txutil.DeleteEvent[idwrap.IDWrap]) HttpSearchParamTopic {
			return HttpSearchParamTopic{WorkspaceID: evt.WorkspaceID}
		},
	)

	httpSearchParamService := h.httpSearchParamService.TX(tx)

	for _, data := range deleteData {
		if err := httpSearchParamService.Delete(ctx, data.paramID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		syncTx.Track(data.paramID, data.workspaceID, data.existingParam.IsDelta)
	}

	// Step 3: Commit and bulk publish (grouped by workspace)
	if err := syncTx.CommitAndPublish(ctx, h.publishBulkSearchParamDelete); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}
