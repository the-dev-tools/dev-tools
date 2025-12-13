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
				if header.DeltaOrder != nil {
					delta.Order = header.DeltaOrder
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

	// Process each delta item
	for _, item := range req.Msg.Items {
		if len(item.HttpHeaderId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_header_id is required for each delta item"))
		}

		headerID, err := idwrap.NewFromBytes(item.HttpHeaderId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Check workspace write access
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

		// Update delta fields
		err = h.httpHeaderService.UpdateDelta(ctx, headerID, item.Key, item.Value, item.Description, item.Enabled)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpHeaderDeltaUpdate(ctx context.Context, req *connect.Request[apiv1.HttpHeaderDeltaUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP header delta must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var updateData []struct {
		deltaID        idwrap.IDWrap
		existingHeader mhttp.HTTPHeader
		item           *apiv1.HttpHeaderDeltaUpdate
	}

	for _, item := range req.Msg.Items {
		if len(item.DeltaHttpHeaderId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("delta_http_header_id is required"))
		}

		deltaID, err := idwrap.NewFromBytes(item.DeltaHttpHeaderId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing delta header - use pool service
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
			deltaID        idwrap.IDWrap
			existingHeader mhttp.HTTPHeader
			item           *apiv1.HttpHeaderDeltaUpdate
		}{
			deltaID:        deltaID,
			existingHeader: existingHeader,
			item:           item,
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
			case apiv1.HttpHeaderDeltaUpdate_KeyUnion_KIND_UNSET:
				deltaKey = nil
			case apiv1.HttpHeaderDeltaUpdate_KeyUnion_KIND_VALUE:
				keyStr := item.Key.GetValue()
				deltaKey = &keyStr
			}
		}
		if item.Value != nil {
			switch item.Value.GetKind() {
			case apiv1.HttpHeaderDeltaUpdate_ValueUnion_KIND_UNSET:
				deltaValue = nil
			case apiv1.HttpHeaderDeltaUpdate_ValueUnion_KIND_VALUE:
				valueStr := item.Value.GetValue()
				deltaValue = &valueStr
			}
		}
		if item.Enabled != nil {
			switch item.Enabled.GetKind() {
			case apiv1.HttpHeaderDeltaUpdate_EnabledUnion_KIND_UNSET:
				deltaEnabled = nil
			case apiv1.HttpHeaderDeltaUpdate_EnabledUnion_KIND_VALUE:
				enabledBool := item.Enabled.GetValue()
				deltaEnabled = &enabledBool
			}
		}
		if item.Description != nil {
			switch item.Description.GetKind() {
			case apiv1.HttpHeaderDeltaUpdate_DescriptionUnion_KIND_UNSET:
				deltaDescription = nil
			case apiv1.HttpHeaderDeltaUpdate_DescriptionUnion_KIND_VALUE:
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

	httpHeaderService := h.httpHeaderService.TX(tx)
	var updatedHeaders []mhttp.HTTPHeader

	for _, update := range preparedUpdates {
		if err := httpHeaderService.UpdateDelta(ctx, update.deltaID, update.deltaKey, update.deltaValue, update.deltaDescription, update.deltaEnabled); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Get updated header for event publishing (from TX service)
		updatedHeader, err := httpHeaderService.GetByID(ctx, update.deltaID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		updatedHeaders = append(updatedHeaders, updatedHeader)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish update events for real-time sync after successful commit
	for _, header := range updatedHeaders {
		// Get workspace ID for the HTTP entry
		httpEntry, err := h.hs.Get(ctx, header.HttpID)
		if err != nil {
			continue
		}
		h.streamers.HttpHeader.Publish(HttpHeaderTopic{WorkspaceID: httpEntry.WorkspaceID}, HttpHeaderEvent{
			Type:       eventTypeUpdate,
			HttpHeader: converter.ToAPIHttpHeader(header),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpHeaderDeltaDelete(ctx context.Context, req *connect.Request[apiv1.HttpHeaderDeltaDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP header delta must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var deleteData []struct {
		deltaID        idwrap.IDWrap
		existingHeader mhttp.HTTPHeader
		workspaceID    idwrap.IDWrap
	}

	for _, item := range req.Msg.Items {
		if len(item.DeltaHttpHeaderId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("delta_http_header_id is required"))
		}

		deltaID, err := idwrap.NewFromBytes(item.DeltaHttpHeaderId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing delta header - use pool service
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
			deltaID        idwrap.IDWrap
			existingHeader mhttp.HTTPHeader
			workspaceID    idwrap.IDWrap
		}{
			deltaID:        deltaID,
			existingHeader: existingHeader,
			workspaceID:    httpEntry.WorkspaceID,
		})
	}

	// Step 2: Execute deletes in transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	httpHeaderService := h.httpHeaderService.TX(tx)
	var deletedHeaders []mhttp.HTTPHeader
	var deletedWorkspaceIDs []idwrap.IDWrap

	for _, data := range deleteData {
		// Delete the delta record
		if err := httpHeaderService.Delete(ctx, data.deltaID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		deletedHeaders = append(deletedHeaders, data.existingHeader)
		deletedWorkspaceIDs = append(deletedWorkspaceIDs, data.workspaceID)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish delete events for real-time sync after successful commit
	for i, header := range deletedHeaders {
		h.streamers.HttpHeader.Publish(HttpHeaderTopic{WorkspaceID: deletedWorkspaceIDs[i]}, HttpHeaderEvent{
			Type:       eventTypeDelete,
			HttpHeader: converter.ToAPIHttpHeader(header),
		})
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
