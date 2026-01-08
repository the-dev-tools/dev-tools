//nolint:revive // exported
package rhttp

import (
	"context"
	"errors"
	"sync"

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

func (h *HttpServiceRPC) HttpHeaderDeltaCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpHeaderDeltaCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Get user's workspaces
	workspaces, err := h.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var allDeltas []*apiv1.HttpHeaderDelta
	for _, workspace := range workspaces {
		// Get HTTP entries for this workspace
		httpList, err := h.hs.GetDeltasByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Get headers for each HTTP entry
		for _, http := range httpList {
			headers, err := h.httpHeaderService.GetByHttpID(ctx, http.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			// Convert to delta format
			for _, header := range headers {
				if !header.IsDelta {
					continue
				}

				delta := &apiv1.HttpHeaderDelta{
					DeltaHttpHeaderId: header.ID.Bytes(),
					HttpId:            header.HttpID.Bytes(),
				}

				if header.ParentHttpHeaderID != nil {
					delta.HttpHeaderId = header.ParentHttpHeaderID.Bytes()
				}

				// Only include delta fields if they exist
				if header.DeltaKey != nil {
					delta.Key = header.DeltaKey
				}
				if header.DeltaValue != nil {
					delta.Value = header.DeltaValue
				}
				if header.DeltaEnabled != nil {
					delta.Enabled = header.DeltaEnabled
				}
				if header.DeltaDescription != nil {
					delta.Description = header.DeltaDescription
				}
				if header.DeltaDisplayOrder != nil {
					delta.Order = header.DeltaDisplayOrder
				}

				allDeltas = append(allDeltas, delta)
			}
		}
	}

	return connect.NewResponse(&apiv1.HttpHeaderDeltaCollectionResponse{
		Items: allDeltas,
	}), nil
}

func (h *HttpServiceRPC) HttpHeaderDeltaInsert(ctx context.Context, req *connect.Request[apiv1.HttpHeaderDeltaInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.Items) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one delta item is required"))
	}

	// FETCH: Gather data and check permissions OUTSIDE transaction
	type insertItem struct {
		headerID    idwrap.IDWrap
		workspaceID idwrap.IDWrap
		item        *apiv1.HttpHeaderDeltaInsert
	}
	insertData := make([]insertItem, 0, len(req.Msg.Items))

	for _, item := range req.Msg.Items {
		if len(item.HttpHeaderId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_header_id is required"))
		}

		headerID, err := idwrap.NewFromBytes(item.HttpHeaderId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		header, err := h.httpHeaderService.GetByID(ctx, headerID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHttpHeaderFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		httpEntry, err := h.hs.Get(ctx, header.HttpID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := h.checkWorkspaceWriteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		insertData = append(insertData, insertItem{
			headerID:    headerID,
			workspaceID: httpEntry.WorkspaceID,
			item:        item,
		})
	}

	// ACT: Update delta fields using mutation context
	mut := mutation.New(h.DB, mutation.WithPublisher(h.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	for _, data := range insertData {
		// Use TX service to read current state within TX
		headerService := h.httpHeaderService.TX(mut.TX())
		header, err := headerService.GetByID(ctx, data.headerID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := mut.UpdateHTTPHeaderDelta(ctx, mutation.HTTPHeaderDeltaUpdateItem{
			ID:          data.headerID,
			HttpID:      header.HttpID,
			WorkspaceID: data.workspaceID,
			Params: gen.UpdateHTTPHeaderDeltaParams{
				ID:                data.headerID,
				DeltaHeaderKey:    data.item.Key,
				DeltaHeaderValue:  data.item.Value,
				DeltaDescription:  data.item.Description,
				DeltaEnabled:      data.item.Enabled,
				DeltaDisplayOrder: ptrToNullFloat64(data.item.Order),
			},
			// Fetch updated model for publisher
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Update payload in tracked event
		updated, err := headerService.GetByID(ctx, data.headerID)
		if err == nil {
			mut.UpdateLastEventPayload(updated)
		}
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpHeaderDeltaUpdate(ctx context.Context, req *connect.Request[apiv1.HttpHeaderDeltaUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP header delta must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	type updateItem struct {
		deltaID        idwrap.IDWrap
		existingHeader mhttp.HTTPHeader
		workspaceID    idwrap.IDWrap
		item           *apiv1.HttpHeaderDeltaUpdate
	}
	updateData := make([]updateItem, 0, len(req.Msg.Items))

	for _, item := range req.Msg.Items {
		if len(item.DeltaHttpHeaderId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("delta_http_header_id is required"))
		}

		deltaID, err := idwrap.NewFromBytes(item.DeltaHttpHeaderId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing delta header
		existingHeader, err := h.httpHeaderService.GetByID(ctx, deltaID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHttpHeaderFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Verify this is actually a delta record
		if !existingHeader.IsDelta {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("specified HTTP header is not a delta"))
		}

		// Get the HTTP entry to check workspace access
		httpEntry, err := h.hs.Get(ctx, existingHeader.HttpID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := h.checkWorkspaceWriteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		updateData = append(updateData, updateItem{
			deltaID:        deltaID,
			existingHeader: existingHeader,
			workspaceID:    httpEntry.WorkspaceID,
			item:           item,
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
		deltaKey := data.existingHeader.DeltaKey
		deltaValue := data.existingHeader.DeltaValue
		deltaDescription := data.existingHeader.DeltaDescription
		deltaEnabled := data.existingHeader.DeltaEnabled
		deltaOrder := data.existingHeader.DeltaDisplayOrder
		var patchData patch.HTTPHeaderPatch

		if item.Key != nil {
			switch item.Key.GetKind() {
			case apiv1.HttpHeaderDeltaUpdate_KeyUnion_KIND_UNSET:
				deltaKey = nil
				patchData.Key = patch.Unset[string]()
			case apiv1.HttpHeaderDeltaUpdate_KeyUnion_KIND_VALUE:
				keyStr := item.Key.GetValue()
				deltaKey = &keyStr
				patchData.Key = patch.NewOptional(keyStr)
			}
		}
		if item.Value != nil {
			switch item.Value.GetKind() {
			case apiv1.HttpHeaderDeltaUpdate_ValueUnion_KIND_UNSET:
				deltaValue = nil
				patchData.Value = patch.Unset[string]()
			case apiv1.HttpHeaderDeltaUpdate_ValueUnion_KIND_VALUE:
				valueStr := item.Value.GetValue()
				deltaValue = &valueStr
				patchData.Value = patch.NewOptional(valueStr)
			}
		}
		if item.Enabled != nil {
			switch item.Enabled.GetKind() {
			case apiv1.HttpHeaderDeltaUpdate_EnabledUnion_KIND_UNSET:
				deltaEnabled = nil
				patchData.Enabled = patch.Unset[bool]()
			case apiv1.HttpHeaderDeltaUpdate_EnabledUnion_KIND_VALUE:
				enabledBool := item.Enabled.GetValue()
				deltaEnabled = &enabledBool
				patchData.Enabled = patch.NewOptional(enabledBool)
			}
		}
		if item.Description != nil {
			switch item.Description.GetKind() {
			case apiv1.HttpHeaderDeltaUpdate_DescriptionUnion_KIND_UNSET:
				deltaDescription = nil
				patchData.Description = patch.Unset[string]()
			case apiv1.HttpHeaderDeltaUpdate_DescriptionUnion_KIND_VALUE:
				descStr := item.Description.GetValue()
				deltaDescription = &descStr
				patchData.Description = patch.NewOptional(descStr)
			}
		}
		if item.Order != nil {
			switch item.Order.GetKind() {
			case apiv1.HttpHeaderDeltaUpdate_OrderUnion_KIND_UNSET:
				deltaOrder = nil
				patchData.Order = patch.Unset[float32]()
			case apiv1.HttpHeaderDeltaUpdate_OrderUnion_KIND_VALUE:
				orderFloat := item.Order.GetValue()
				deltaOrder = &orderFloat
				patchData.Order = patch.NewOptional(orderFloat)
			}
		}

		headerService := h.httpHeaderService.TX(mut.TX())
		if err := mut.UpdateHTTPHeaderDelta(ctx, mutation.HTTPHeaderDeltaUpdateItem{
			ID:          data.deltaID,
			HttpID:      data.existingHeader.HttpID,
			WorkspaceID: data.workspaceID,
			Params: gen.UpdateHTTPHeaderDeltaParams{
				ID:                data.deltaID,
				DeltaHeaderKey:    deltaKey,
				DeltaHeaderValue:  deltaValue,
				DeltaDescription:  deltaDescription,
				DeltaEnabled:      deltaEnabled,
				DeltaDisplayOrder: ptrToNullFloat64(deltaOrder),
			},
			Patch: patchData,
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Update payload in tracked event
		updated, err := headerService.GetByID(ctx, data.deltaID)
		if err == nil {
			mut.UpdateLastEventPayload(updated)
		}
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpHeaderDeltaDelete(ctx context.Context, req *connect.Request[apiv1.HttpHeaderDeltaDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP header delta must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	type deleteItem struct {
		deltaID     idwrap.IDWrap
		httpID      idwrap.IDWrap
		workspaceID idwrap.IDWrap
		header      mhttp.HTTPHeader
	}
	deleteData := make([]deleteItem, 0, len(req.Msg.Items))

	for _, item := range req.Msg.Items {
		if len(item.DeltaHttpHeaderId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("delta_http_header_id is required"))
		}

		deltaID, err := idwrap.NewFromBytes(item.DeltaHttpHeaderId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing delta header
		existingHeader, err := h.httpHeaderService.GetByID(ctx, deltaID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHttpHeaderFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Verify this is actually a delta record
		if !existingHeader.IsDelta {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("specified HTTP header is not a delta"))
		}

		// Get the HTTP entry to check workspace access
		httpEntry, err := h.hs.Get(ctx, existingHeader.HttpID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := h.checkWorkspaceDeleteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		deleteData = append(deleteData, deleteItem{
			deltaID:     deltaID,
			httpID:      existingHeader.HttpID,
			workspaceID: httpEntry.WorkspaceID,
			header:      existingHeader,
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
			Entity:      mutation.EntityHTTPHeader,
			Op:          mutation.OpDelete,
			ID:          data.deltaID,
			ParentID:    data.httpID,
			WorkspaceID: data.workspaceID,
			IsDelta:     true,
			Payload:     data.header,
		})
		if err := mut.Queries().DeleteHTTPHeader(ctx, data.deltaID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpHeaderDeltaSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpHeaderDeltaSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return h.streamHttpHeaderDeltaSync(ctx, userID, stream.Send)
}

func (h *HttpServiceRPC) streamHttpHeaderDeltaSync(ctx context.Context, userID idwrap.IDWrap, send func(*apiv1.HttpHeaderDeltaSyncResponse) error) error {
	var workspaceSet sync.Map

	// Filter for workspace-based access control
	filter := func(topic HttpHeaderTopic) bool {
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
	events, err := h.streamers.HttpHeader.Subscribe(ctx, filter)
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
			// Get the full header record for delta sync response
			headerID, err := idwrap.NewFromBytes(evt.Payload.HttpHeader.GetHttpHeaderId())
			if err != nil {
				continue // Skip if can't parse ID
			}
			headerRecord, err := h.httpHeaderService.GetByID(ctx, headerID)
			if err != nil {
				continue // Skip if can't get the record
			}
			if !headerRecord.IsDelta {
				continue
			}
			resp := httpHeaderDeltaSyncResponseFrom(evt.Payload, headerRecord)
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
