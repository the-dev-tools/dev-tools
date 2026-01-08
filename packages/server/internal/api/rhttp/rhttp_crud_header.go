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

func (h *HttpServiceRPC) HttpHeaderCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpHeaderCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Get user's workspaces
	workspaces, err := h.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var allHeaders []*apiv1.HttpHeader
	for _, workspace := range workspaces {
		// Get HTTP entries for this workspace
		httpList, err := h.hs.GetByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Get headers for each HTTP entry
		for _, http := range httpList {
			headers, err := h.httpHeaderService.GetByHttpID(ctx, http.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			// Convert to API format
			for _, header := range headers {
				apiHeader := converter.ToAPIHttpHeader(header)
				allHeaders = append(allHeaders, apiHeader)
			}
		}
	}

	return connect.NewResponse(&apiv1.HttpHeaderCollectionResponse{Items: allHeaders}), nil
}

func (h *HttpServiceRPC) HttpHeaderInsert(ctx context.Context, req *connect.Request[apiv1.HttpHeaderInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP header must be provided"))
	}

	// FETCH: Process request data and perform all reads/checks OUTSIDE transaction
	type insertItem struct {
		headerID    idwrap.IDWrap
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
		if len(item.HttpHeaderId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_header_id is required"))
		}
		if len(item.HttpId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_id is required"))
		}

		headerID, err := idwrap.NewFromBytes(item.HttpHeaderId)
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
			headerID:    headerID,
			httpID:      httpID,
			key:         item.Key,
			value:       item.Value,
			enabled:     item.Enabled,
			description: item.Description,
			order:       float64(item.Order),
			workspaceID: httpEntry.WorkspaceID,
		})
	}

	// ACT: Insert headers using mutation context with auto-publish
	mut := mutation.New(h.DB, mutation.WithPublisher(h.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	now := time.Now().UnixMilli()
	for _, data := range insertData {
		header := mhttp.HTTPHeader{
			ID:           data.headerID,
			HttpID:       data.httpID,
			Key:          data.key,
			Value:        data.value,
			Enabled:      data.enabled,
			Description:  data.description,
			DisplayOrder: float32(data.order),
		}

		if err := mut.InsertHTTPHeader(ctx, mutation.HTTPHeaderInsertItem{
			ID:          data.headerID,
			HttpID:      data.httpID,
			WorkspaceID: data.workspaceID,
			IsDelta:     false,
			Params: gen.CreateHTTPHeaderParams{
				ID:           data.headerID,
				HttpID:       data.httpID,
				HeaderKey:    data.key,
				HeaderValue:  data.value,
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
			Entity:      mutation.EntityHTTPHeader,
			Op:          mutation.OpInsert,
			ID:          data.headerID,
			ParentID:    data.httpID,
			WorkspaceID: data.workspaceID,
			Payload:     header,
		})
	}

	if err := mut.Commit(ctx); err != nil { // Auto-publishes events!
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpHeaderUpdate(ctx context.Context, req *connect.Request[apiv1.HttpHeaderUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP header must be provided"))
	}

	// FETCH: Process request data and perform all reads/checks OUTSIDE transaction
	type updateItem struct {
		existingHeader mhttp.HTTPHeader
		key            *string
		value          *string
		enabled        *bool
		description    *string
		order          *float32
		workspaceID    idwrap.IDWrap
	}
	updateData := make([]updateItem, 0, len(req.Msg.Items))

	for _, item := range req.Msg.Items {
		if len(item.HttpHeaderId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_header_id is required"))
		}

		headerID, err := idwrap.NewFromBytes(item.HttpHeaderId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing header - use pool service
		existingHeader, err := h.httpHeaderService.GetByID(ctx, headerID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHttpHeaderFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Get the HTTP entry to check workspace access - use pool service
		httpEntry, err := h.hs.Get(ctx, existingHeader.HttpID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// CHECK: Validate write access to the workspace
		if err := h.checkWorkspaceWriteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		updateData = append(updateData, updateItem{
			existingHeader: existingHeader,
			key:            item.Key,
			value:          item.Value,
			enabled:        item.Enabled,
			description:    item.Description,
			order:          item.Order,
			workspaceID:    httpEntry.WorkspaceID,
		})
	}

	// ACT: Update headers using mutation context with auto-publish
	mut := mutation.New(h.DB, mutation.WithPublisher(h.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	for _, data := range updateData {
		header := data.existingHeader

		// Build patch with only changed fields
		headerPatch := patch.HTTPHeaderPatch{}

		// Update fields if provided and track in patch
		if data.key != nil {
			header.Key = *data.key
			headerPatch.Key = patch.NewOptional(*data.key)
		}
		if data.value != nil {
			header.Value = *data.value
			headerPatch.Value = patch.NewOptional(*data.value)
		}
		if data.enabled != nil {
			header.Enabled = *data.enabled
			headerPatch.Enabled = patch.NewOptional(*data.enabled)
		}
		if data.description != nil {
			header.Description = *data.description
			headerPatch.Description = patch.NewOptional(*data.description)
		}
		if data.order != nil {
			header.DisplayOrder = *data.order
			headerPatch.Order = patch.NewOptional(*data.order)
		}

		if err := mut.UpdateHTTPHeader(ctx, mutation.HTTPHeaderUpdateItem{
			ID:          header.ID,
			HttpID:      header.HttpID,
			WorkspaceID: data.workspaceID,
			IsDelta:     header.IsDelta,
			Params: gen.UpdateHTTPHeaderParams{
				ID:           header.ID,
				HeaderKey:    header.Key,
				HeaderValue:  header.Value,
				Description:  header.Description,
				Enabled:      header.Enabled,
				DisplayOrder: float64(header.DisplayOrder),
			},
			Patch: headerPatch,
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		mut.Track(mutation.Event{
			Entity:      mutation.EntityHTTPHeader,
			Op:          mutation.OpUpdate,
			ID:          header.ID,
			ParentID:    header.HttpID,
			WorkspaceID: data.workspaceID,
			Payload:     header,
			Patch:       headerPatch,
		})
	}

	if err := mut.Commit(ctx); err != nil { // Auto-publishes events!
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpHeaderDelete(ctx context.Context, req *connect.Request[apiv1.HttpHeaderDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP header must be provided"))
	}

	// FETCH: Process request data and perform all reads/checks OUTSIDE transaction
	type deleteItem struct {
		headerID    idwrap.IDWrap
		httpID      idwrap.IDWrap
		workspaceID idwrap.IDWrap
		isDelta     bool
	}
	deleteItems := make([]deleteItem, 0, len(req.Msg.Items))

	for _, item := range req.Msg.Items {
		if len(item.HttpHeaderId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_header_id is required"))
		}

		headerID, err := idwrap.NewFromBytes(item.HttpHeaderId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing header - use pool service
		existingHeader, err := h.httpHeaderService.GetByID(ctx, headerID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHttpHeaderFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Get the HTTP entry to check workspace access - use pool service
		httpEntry, err := h.hs.Get(ctx, existingHeader.HttpID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// CHECK: Validate delete access to the workspace
		if err := h.checkWorkspaceDeleteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		deleteItems = append(deleteItems, deleteItem{
			headerID:    headerID,
			httpID:      existingHeader.HttpID,
			workspaceID: httpEntry.WorkspaceID,
			isDelta:     existingHeader.IsDelta,
		})
	}

	// ACT: Delete headers using mutation context with auto-publish
	mut := mutation.New(h.DB, mutation.WithPublisher(h.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	for _, item := range deleteItems {
		mut.Track(mutation.Event{
			Entity:      mutation.EntityHTTPHeader,
			Op:          mutation.OpDelete,
			ID:          item.headerID,
			ParentID:    item.httpID,
			WorkspaceID: item.workspaceID,
			IsDelta:     item.isDelta,
		})
		if err := mut.Queries().DeleteHTTPHeader(ctx, item.headerID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := mut.Commit(ctx); err != nil { // Auto-publishes events!
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}
