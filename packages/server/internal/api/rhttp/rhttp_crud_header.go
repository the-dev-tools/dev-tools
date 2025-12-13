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

	"the-dev-tools/server/pkg/service/shttp"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
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

	httpHeaderService := h.httpHeaderService.TX(tx)

	var createdHeaders []mhttp.HTTPHeader

	for _, data := range insertData {
		// Create the header
		headerModel := &mhttp.HTTPHeader{
			ID:          data.headerID,
			HttpID:      data.httpID,
			Key:         data.key,
			Value:       data.value,
			Enabled:     data.enabled,
			Description: data.description,
			Order:       float32(data.order),
		}

		if err := httpHeaderService.Create(ctx, headerModel); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		createdHeaders = append(createdHeaders, *headerModel)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Step 3: Publish create events for real-time sync
	for i, header := range createdHeaders {
		h.streamers.HttpHeader.Publish(HttpHeaderTopic{WorkspaceID: insertData[i].workspaceID}, HttpHeaderEvent{
			Type:       eventTypeInsert,
			IsDelta:    header.IsDelta,
			HttpHeader: converter.ToAPIHttpHeader(header),
		})
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

	httpHeaderService := h.httpHeaderService.TX(tx)

	var updatedHeaders []mhttp.HTTPHeader

	for _, data := range updateData {
		header := data.existingHeader

		// Update fields if provided
		if data.key != nil {
			header.Key = *data.key
		}
		if data.value != nil {
			header.Value = *data.value
		}
		if data.enabled != nil {
			header.Enabled = *data.enabled
		}
		if data.description != nil {
			header.Description = *data.description
		}
		if data.order != nil {
			header.Order = *data.order
		}

		if err := httpHeaderService.Update(ctx, &header); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		updatedHeaders = append(updatedHeaders, header)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Step 3: Publish update events for real-time sync
	for i, header := range updatedHeaders {
		h.streamers.HttpHeader.Publish(HttpHeaderTopic{WorkspaceID: updateData[i].workspaceID}, HttpHeaderEvent{
			Type:       eventTypeUpdate,
			IsDelta:    header.IsDelta,
			HttpHeader: converter.ToAPIHttpHeader(header),
		})
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

	httpHeaderService := h.httpHeaderService.TX(tx)

	var deletedHeaders []idwrap.IDWrap

	for _, data := range deleteData {
		if err := httpHeaderService.Delete(ctx, data.headerID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		deletedHeaders = append(deletedHeaders, data.headerID)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Step 3: Publish delete events for real-time sync
	for i, headerID := range deletedHeaders {
		h.streamers.HttpHeader.Publish(HttpHeaderTopic{WorkspaceID: deleteData[i].workspaceID}, HttpHeaderEvent{
			Type:    eventTypeDelete,
			IsDelta: deleteData[i].isDelta,
			HttpHeader: &apiv1.HttpHeader{
				HttpHeaderId: headerID.Bytes(),
			},
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}
