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

// headerWithWorkspace is a context carrier that pairs a header with its workspace ID.
// This is needed because HTTPHeader doesn't store WorkspaceID directly, but we need it
// for topic extraction during bulk sync event publishing.
type headerWithWorkspace struct {
	header      mhttp.HTTPHeader
	workspaceID idwrap.IDWrap
}

// publishBulkHeaderInsert publishes multiple header insert events in bulk.
// Items are already grouped by HttpHeaderTopic by the BulkSyncTxInsert wrapper.
func (h *HttpServiceRPC) publishBulkHeaderInsert(
	topic HttpHeaderTopic,
	items []headerWithWorkspace,
) {
	// Convert to event slice for variadic publish
	events := make([]HttpHeaderEvent, len(items))
	for i, item := range items {
		events[i] = HttpHeaderEvent{
			Type:       eventTypeInsert,
			IsDelta:    item.header.IsDelta,
			HttpHeader: converter.ToAPIHttpHeader(item.header),
		}
	}

	// Single bulk publish for entire batch
	h.streamers.HttpHeader.Publish(topic, events...)
}

// publishBulkHeaderUpdate publishes multiple header update events in bulk.
// Items are already grouped by HttpHeaderTopic by the BulkSyncTxUpdate wrapper.
func (h *HttpServiceRPC) publishBulkHeaderUpdate(
	topic HttpHeaderTopic,
	events []txutil.UpdateEvent[headerWithWorkspace, patch.HTTPHeaderPatch],
) {
	headerEvents := make([]HttpHeaderEvent, len(events))
	for i, evt := range events {
		headerEvents[i] = HttpHeaderEvent{
			Type:       eventTypeUpdate,
			IsDelta:    evt.Item.header.IsDelta,
			HttpHeader: converter.ToAPIHttpHeader(evt.Item.header),
			Patch:      evt.Patch, // Partial updates preserved!
		}
	}
	h.streamers.HttpHeader.Publish(topic, headerEvents...)
}

// publishBulkHeaderDelete publishes multiple header delete events in bulk.
// Items are already grouped by HttpHeaderTopic by the BulkSyncTxDelete wrapper.
func (h *HttpServiceRPC) publishBulkHeaderDelete(
	topic HttpHeaderTopic,
	events []txutil.DeleteEvent[idwrap.IDWrap],
) {
	headerEvents := make([]HttpHeaderEvent, len(events))
	for i, evt := range events {
		headerEvents[i] = HttpHeaderEvent{
			Type:    eventTypeDelete,
			IsDelta: evt.IsDelta,
			HttpHeader: &apiv1.HttpHeader{
				HttpHeaderId: evt.ID.Bytes(),
			},
		}
	}
	h.streamers.HttpHeader.Publish(topic, headerEvents...)
}

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

	// Step 1: Process request data and perform all reads/checks OUTSIDE transaction
	var insertData []struct {
		headerID    idwrap.IDWrap
		httpID      idwrap.IDWrap
		key         string
		value       string
		enabled     bool
		description string
		order       float64
		workspaceID idwrap.IDWrap
	}

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

		// Check write access to the workspace
		if err := h.checkWorkspaceWriteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		insertData = append(insertData, struct {
			headerID    idwrap.IDWrap
			httpID      idwrap.IDWrap
			key         string
			value       string
			enabled     bool
			description string
			order       float64
			workspaceID idwrap.IDWrap
		}{
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

	// Step 2: Minimal write transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	// Create bulk sync wrapper with topic extractor
	syncTx := txutil.NewBulkInsertTx[headerWithWorkspace, HttpHeaderTopic](
		tx,
		func(hww headerWithWorkspace) HttpHeaderTopic {
			return HttpHeaderTopic{WorkspaceID: hww.workspaceID}
		},
	)

	httpHeaderService := h.httpHeaderService.TX(tx)

	for _, data := range insertData {
		// Create the header
		headerModel := &mhttp.HTTPHeader{
			ID:           data.headerID,
			HttpID:       data.httpID,
			Key:          data.key,
			Value:        data.value,
			Enabled:      data.enabled,
			Description:  data.description,
			DisplayOrder: float32(data.order),
		}

		if err := httpHeaderService.Create(ctx, headerModel); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Track with workspace context for bulk sync
		syncTx.Track(headerWithWorkspace{
			header:      *headerModel,
			workspaceID: data.workspaceID,
		})
	}

	// Step 3: Commit and bulk publish sync events (grouped by topic)
	if err := syncTx.CommitAndPublish(ctx, h.publishBulkHeaderInsert); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpHeaderUpdate(ctx context.Context, req *connect.Request[apiv1.HttpHeaderUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP header must be provided"))
	}

	// Step 1: Process request data and perform all reads/checks OUTSIDE transaction
	var updateData []struct {
		existingHeader mhttp.HTTPHeader
		key            *string
		value          *string
		enabled        *bool
		description    *string
		order          *float32
		workspaceID    idwrap.IDWrap
	}

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

		// Check write access to the workspace
		if err := h.checkWorkspaceWriteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		updateData = append(updateData, struct {
			existingHeader mhttp.HTTPHeader
			key            *string
			value          *string
			enabled        *bool
			description    *string
			order          *float32
			workspaceID    idwrap.IDWrap
		}{
			existingHeader: existingHeader,
			key:            item.Key,
			value:          item.Value,
			enabled:        item.Enabled,
			description:    item.Description,
			order:          item.Order,
			workspaceID:    httpEntry.WorkspaceID,
		})
	}

	// Step 2: Minimal write transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	// Create bulk sync wrapper with topic extractor
	syncTx := txutil.NewBulkUpdateTx[headerWithWorkspace, patch.HTTPHeaderPatch, HttpHeaderTopic](
		tx,
		func(hww headerWithWorkspace) HttpHeaderTopic {
			return HttpHeaderTopic{WorkspaceID: hww.workspaceID}
		},
	)

	httpHeaderService := h.httpHeaderService.TX(tx)

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

		if err := httpHeaderService.Update(ctx, &header); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Track with workspace context and patch
		syncTx.Track(
			headerWithWorkspace{
				header:      header,
				workspaceID: data.workspaceID,
			},
			headerPatch,
		)
	}

	// Step 3: Commit and bulk publish (grouped by workspace)
	if err := syncTx.CommitAndPublish(ctx, h.publishBulkHeaderUpdate); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpHeaderDelete(ctx context.Context, req *connect.Request[apiv1.HttpHeaderDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP header must be provided"))
	}

	// Step 1: Process request data and perform all reads/checks OUTSIDE transaction
	var deleteData []struct {
		headerID    idwrap.IDWrap
		workspaceID idwrap.IDWrap
		isDelta     bool
	}

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

		// Check delete access to the workspace
		if err := h.checkWorkspaceDeleteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		deleteData = append(deleteData, struct {
			headerID    idwrap.IDWrap
			workspaceID idwrap.IDWrap
			isDelta     bool
		}{
			headerID:    headerID,
			workspaceID: httpEntry.WorkspaceID,
			isDelta:     existingHeader.IsDelta,
		})
	}

	// Step 2: Minimal write transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	// Create bulk sync wrapper with topic extractor
	syncTx := txutil.NewBulkDeleteTx[idwrap.IDWrap, HttpHeaderTopic](
		tx,
		func(evt txutil.DeleteEvent[idwrap.IDWrap]) HttpHeaderTopic {
			return HttpHeaderTopic{WorkspaceID: evt.WorkspaceID}
		},
	)

	httpHeaderService := h.httpHeaderService.TX(tx)

	for _, data := range deleteData {
		if err := httpHeaderService.Delete(ctx, data.headerID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		syncTx.Track(data.headerID, data.workspaceID, data.isDelta)
	}

	// Step 3: Commit and bulk publish (grouped by workspace)
	if err := syncTx.CommitAndPublish(ctx, h.publishBulkHeaderDelete); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}
