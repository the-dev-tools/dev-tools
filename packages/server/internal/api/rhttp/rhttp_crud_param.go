//nolint:revive // exported
package rhttp

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/converter"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/mutation"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/patch"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/shttp"
	apiv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/http/v1"
)

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

	// FETCH: Gather data and check permissions OUTSIDE transaction
	type insertItem struct {
		paramID     idwrap.IDWrap
		httpID      idwrap.IDWrap
		key         string
		value       string
		enabled     bool
		description string
		order       float64
		workspaceID idwrap.IDWrap
	}
	insertData := make([]insertItem, 0, len(req.Msg.Items))

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

		// CHECK: Validate write access to the workspace
		if err := h.checkWorkspaceWriteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		insertData = append(insertData, insertItem{
			paramID:     paramID,
			httpID:      httpID,
			key:         item.Key,
			value:       item.Value,
			enabled:     item.Enabled,
			description: item.Description,
			order:       float64(item.Order),
			workspaceID: httpEntry.WorkspaceID,
		})
	}

	// ACT: Insert search params using mutation context with auto-publish
	mut := mutation.New(h.DB, mutation.WithPublisher(h.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	now := time.Now().UnixMilli()
	for _, data := range insertData {
		param := mhttp.HTTPSearchParam{
			ID:           data.paramID,
			HttpID:       data.httpID,
			Key:          data.key,
			Value:        data.value,
			Enabled:      data.enabled,
			Description:  data.description,
			DisplayOrder: data.order,
		}

		if err := mut.InsertHTTPSearchParam(ctx, mutation.HTTPSearchParamInsertItem{
			ID:          data.paramID,
			HttpID:      data.httpID,
			WorkspaceID: data.workspaceID,
			IsDelta:     false,
			Params: gen.CreateHTTPSearchParamParams{
				ID:           data.paramID,
				HttpID:       data.httpID,
				Key:          data.key,
				Value:        data.value,
				Description:  data.description,
				Enabled:      data.enabled,
				DisplayOrder: data.order,
				IsDelta:      false,
				CreatedAt:    now,
				UpdatedAt:    now,
			},
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		mut.Track(mutation.Event{
			Entity:      mutation.EntityHTTPParam,
			Op:          mutation.OpInsert,
			ID:          data.paramID,
			ParentID:    data.httpID,
			WorkspaceID: data.workspaceID,
			Payload:     param,
		})
	}

	if err := mut.Commit(ctx); err != nil { // Auto-publishes events!
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpSearchParamUpdate(ctx context.Context, req *connect.Request[apiv1.HttpSearchParamUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP search param must be provided"))
	}

	// FETCH: Process request data and perform all reads/checks OUTSIDE transaction
	type updateItem struct {
		existingParam mhttp.HTTPSearchParam
		key           *string
		value         *string
		enabled       *bool
		description   *string
		order         *float32
		workspaceID   idwrap.IDWrap
	}
	updateData := make([]updateItem, 0, len(req.Msg.Items))

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

		// CHECK: Validate write access to the workspace
		if err := h.checkWorkspaceWriteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		updateData = append(updateData, updateItem{
			existingParam: *existingParam,
			key:           item.Key,
			value:         item.Value,
			enabled:       item.Enabled,
			description:   item.Description,
			order:         item.Order,
			workspaceID:   httpEntry.WorkspaceID,
		})
	}

	// ACT: Update search params using mutation context with auto-publish
	mut := mutation.New(h.DB, mutation.WithPublisher(h.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	for _, data := range updateData {
		param := data.existingParam

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

		if err := mut.UpdateHTTPSearchParam(ctx, mutation.HTTPSearchParamUpdateItem{
			ID:          param.ID,
			HttpID:      param.HttpID,
			WorkspaceID: data.workspaceID,
			IsDelta:     param.IsDelta,
			Params: gen.UpdateHTTPSearchParamParams{
				ID:          param.ID,
				Key:         param.Key,
				Value:       param.Value,
				Description: param.Description,
				Enabled:     param.Enabled,
			},
			Patch: paramPatch,
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Update order separately if provided (order is stored in a separate column)
		if data.order != nil {
			if err := mut.Queries().UpdateHTTPSearchParamOrder(ctx, gen.UpdateHTTPSearchParamOrderParams{
				DisplayOrder: param.DisplayOrder,
				ID:           param.ID,
				HttpID:       param.HttpID,
			}); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		}

		mut.Track(mutation.Event{
			Entity:      mutation.EntityHTTPParam,
			Op:          mutation.OpUpdate,
			ID:          param.ID,
			ParentID:    param.HttpID,
			WorkspaceID: data.workspaceID,
			Payload:     param,
			Patch:       paramPatch,
		})
	}

	if err := mut.Commit(ctx); err != nil { // Auto-publishes events!
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpSearchParamDelete(ctx context.Context, req *connect.Request[apiv1.HttpSearchParamDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP search param must be provided"))
	}

	// FETCH: Gather data and check permissions OUTSIDE transaction
	type deleteItem struct {
		paramID     idwrap.IDWrap
		httpID      idwrap.IDWrap
		workspaceID idwrap.IDWrap
		isDelta     bool
	}
	deleteItems := make([]deleteItem, 0, len(req.Msg.Items))

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

		// CHECK: Validate delete access to the workspace
		if err := h.checkWorkspaceDeleteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		deleteItems = append(deleteItems, deleteItem{
			paramID:     paramID,
			httpID:      existingParam.HttpID,
			workspaceID: httpEntry.WorkspaceID,
			isDelta:     existingParam.IsDelta,
		})
	}

	// ACT: Delete search params using mutation context with auto-publish
	mut := mutation.New(h.DB, mutation.WithPublisher(h.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	for _, item := range deleteItems {
		mut.Track(mutation.Event{
			Entity:      mutation.EntityHTTPParam,
			Op:          mutation.OpDelete,
			ID:          item.paramID,
			ParentID:    item.httpID,
			WorkspaceID: item.workspaceID,
			IsDelta:     item.isDelta,
		})
		if err := mut.Queries().DeleteHTTPSearchParam(ctx, item.paramID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := mut.Commit(ctx); err != nil { // Auto-publishes events!
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}
