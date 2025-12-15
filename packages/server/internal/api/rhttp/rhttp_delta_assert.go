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

// streamHttpHeaderDeltaSync streams HTTP header delta events to the client
func (h *HttpServiceRPC) HttpAssertDeltaCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpAssertDeltaCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Get user's workspaces
	workspaces, err := h.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var allDeltas []*apiv1.HttpAssertDelta
	for _, workspace := range workspaces {
		// Get HTTP entries for this workspace
		httpList, err := h.hs.GetDeltasByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Get asserts for each HTTP entry
		for _, http := range httpList {
			asserts, err := h.httpAssertService.GetByHttpID(ctx, http.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			// Convert to delta format
			for _, assert := range asserts {
				if !assert.IsDelta {
					continue
				}

				delta := &apiv1.HttpAssertDelta{
					DeltaHttpAssertId: assert.ID.Bytes(),
					HttpId:            assert.HttpID.Bytes(),
				}

				if assert.ParentHttpAssertID != nil {
					delta.HttpAssertId = assert.ParentHttpAssertID.Bytes()
				}

				// Only include delta fields if they exist
				if assert.DeltaValue != nil {
					delta.Value = assert.DeltaValue
				}
				if assert.DeltaEnabled != nil {
					delta.Enabled = assert.DeltaEnabled
				}
				if assert.DeltaDisplayOrder != nil {
					delta.Order = assert.DeltaDisplayOrder
				}

				allDeltas = append(allDeltas, delta)
			}
		}
	}

	return connect.NewResponse(&apiv1.HttpAssertDeltaCollectionResponse{
		Items: allDeltas,
	}), nil
}

func (h *HttpServiceRPC) HttpAssertDeltaInsert(ctx context.Context, req *connect.Request[apiv1.HttpAssertDeltaInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.Items) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one delta item is required"))
	}

	// Process each delta item
	for _, item := range req.Msg.Items {
		if len(item.HttpAssertId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_assert_id is required for each delta item"))
		}

		assertID, err := idwrap.NewFromBytes(item.HttpAssertId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Check workspace write access
		assert, err := h.httpAssertService.GetByID(ctx, assertID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHttpAssertFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		httpEntry, err := h.hs.Get(ctx, assert.HttpID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := h.checkWorkspaceWriteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		// Update delta fields
		err = h.httpAssertService.UpdateDelta(ctx, assertID, item.Value, item.Enabled, nil, item.Order)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpAssertDeltaUpdate(ctx context.Context, req *connect.Request[apiv1.HttpAssertDeltaUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP assert delta must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var updateData []struct {
		deltaID        idwrap.IDWrap
		existingAssert *mhttp.HTTPAssert
		item           *apiv1.HttpAssertDeltaUpdate
	}

	for _, item := range req.Msg.Items {
		if len(item.DeltaHttpAssertId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("delta_http_assert_id is required"))
		}

		deltaID, err := idwrap.NewFromBytes(item.DeltaHttpAssertId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing delta assert - use pool service
		existingAssert, err := h.httpAssertService.GetByID(ctx, deltaID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHttpAssertFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Verify this is actually a delta record
		if !existingAssert.IsDelta {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("specified HTTP assert is not a delta"))
		}

		// Get the HTTP entry to check workspace access - use pool service
		httpEntry, err := h.hs.Get(ctx, existingAssert.HttpID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Check write access to the workspace
		if err := h.checkWorkspaceWriteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		updateData = append(updateData, struct {
			deltaID        idwrap.IDWrap
			existingAssert *mhttp.HTTPAssert
			item           *apiv1.HttpAssertDeltaUpdate
		}{
			deltaID:        deltaID,
			existingAssert: existingAssert,
			item:           item,
		})
	}

	// Step 2: Prepare updates (in memory)
	var preparedUpdates []struct {
		deltaID      idwrap.IDWrap
		deltaValue   *string
		deltaEnabled *bool
		deltaOrder   *float32
	}

	for _, data := range updateData {
		item := data.item
		var deltaValue *string
		var deltaEnabled *bool
		var deltaOrder *float32

		if item.Value != nil {
			switch item.Value.GetKind() {
			case apiv1.HttpAssertDeltaUpdate_ValueUnion_KIND_UNSET:
				deltaValue = nil
			case apiv1.HttpAssertDeltaUpdate_ValueUnion_KIND_VALUE:
				valueStr := item.Value.GetValue()
				deltaValue = &valueStr
			}
		}
		if item.Enabled != nil {
			switch item.Enabled.GetKind() {
			case apiv1.HttpAssertDeltaUpdate_EnabledUnion_KIND_UNSET:
				deltaEnabled = nil
			case apiv1.HttpAssertDeltaUpdate_EnabledUnion_KIND_VALUE:
				enabledBool := item.Enabled.GetValue()
				deltaEnabled = &enabledBool
			}
		}
		if item.Order != nil {
			switch item.Order.GetKind() {
			case apiv1.HttpAssertDeltaUpdate_OrderUnion_KIND_UNSET:
				deltaOrder = nil
			case apiv1.HttpAssertDeltaUpdate_OrderUnion_KIND_VALUE:
				orderFloat := item.Order.GetValue()
				deltaOrder = &orderFloat
			}
		}

		preparedUpdates = append(preparedUpdates, struct {
			deltaID      idwrap.IDWrap
			deltaValue   *string
			deltaEnabled *bool
			deltaOrder   *float32
		}{
			deltaID:      data.deltaID,
			deltaValue:   deltaValue,
			deltaEnabled: deltaEnabled,
			deltaOrder:   deltaOrder,
		})
	}

	// Step 3: Execute updates in transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	httpAssertService := h.httpAssertService.TX(tx)
	var updatedAsserts []mhttp.HTTPAssert

	for _, update := range preparedUpdates {
		if err := httpAssertService.UpdateDelta(ctx, update.deltaID, update.deltaValue, update.deltaEnabled, nil, update.deltaOrder); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Get updated assert for event publishing (from TX service)
		updatedAssert, err := httpAssertService.GetByID(ctx, update.deltaID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		updatedAsserts = append(updatedAsserts, *updatedAssert)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish update events for real-time sync after successful commit
	for _, assert := range updatedAsserts {
		// Get workspace ID for the HTTP entry
		httpEntry, err := h.hs.Get(ctx, assert.HttpID)
		if err != nil {
			continue
		}
		h.streamers.HttpAssert.Publish(HttpAssertTopic{WorkspaceID: httpEntry.WorkspaceID}, HttpAssertEvent{
			Type:       eventTypeUpdate,
			IsDelta:    assert.IsDelta,
			HttpAssert: converter.ToAPIHttpAssert(assert),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpAssertDeltaDelete(ctx context.Context, req *connect.Request[apiv1.HttpAssertDeltaDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP assert delta must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var deleteData []struct {
		deltaID        idwrap.IDWrap
		existingAssert *mhttp.HTTPAssert
		workspaceID    idwrap.IDWrap
	}

	for _, item := range req.Msg.Items {
		if len(item.DeltaHttpAssertId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("delta_http_assert_id is required"))
		}

		deltaID, err := idwrap.NewFromBytes(item.DeltaHttpAssertId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing delta assert - use pool service
		existingAssert, err := h.httpAssertService.GetByID(ctx, deltaID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHttpAssertFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Verify this is actually a delta record
		if !existingAssert.IsDelta {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("specified HTTP assert is not a delta"))
		}

		// Get the HTTP entry to check workspace access - use pool service
		httpEntry, err := h.hs.Get(ctx, existingAssert.HttpID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Check delete access to the workspace
		if err := h.checkWorkspaceDeleteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		deleteData = append(deleteData, struct {
			deltaID        idwrap.IDWrap
			existingAssert *mhttp.HTTPAssert
			workspaceID    idwrap.IDWrap
		}{
			deltaID:        deltaID,
			existingAssert: existingAssert,
			workspaceID:    httpEntry.WorkspaceID,
		})
	}

	// Step 2: Execute deletes in transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	httpAssertService := h.httpAssertService.TX(tx)
	var deletedAsserts []mhttp.HTTPAssert
	var deletedWorkspaceIDs []idwrap.IDWrap

	for _, data := range deleteData {
		// Delete the delta record
		if err := httpAssertService.Delete(ctx, data.deltaID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		deletedAsserts = append(deletedAsserts, *data.existingAssert)
		deletedWorkspaceIDs = append(deletedWorkspaceIDs, data.workspaceID)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish delete events for real-time sync after successful commit
	for i, assert := range deletedAsserts {
		h.streamers.HttpAssert.Publish(HttpAssertTopic{WorkspaceID: deletedWorkspaceIDs[i]}, HttpAssertEvent{
			Type:       eventTypeDelete,
			IsDelta:    assert.IsDelta,
			HttpAssert: converter.ToAPIHttpAssert(assert),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpAssertDeltaSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpAssertDeltaSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return h.streamHttpAssertDeltaSync(ctx, userID, stream.Send)
}

func (h *HttpServiceRPC) streamHttpAssertDeltaSync(ctx context.Context, userID idwrap.IDWrap, send func(*apiv1.HttpAssertDeltaSyncResponse) error) error {
	var workspaceSet sync.Map

	// Filter for workspace-based access control
	filter := func(topic HttpAssertTopic) bool {
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
	events, err := h.streamers.HttpAssert.Subscribe(ctx, filter)
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
			// Get the full assert record for delta sync response
			assertID, err := idwrap.NewFromBytes(evt.Payload.HttpAssert.GetHttpAssertId())
			if err != nil {
				continue // Skip if can't parse ID
			}
			assertRecord, err := h.httpAssertService.GetByID(ctx, assertID)
			if err != nil {
				continue // Skip if can't get the record
			}
			if !assertRecord.IsDelta {
				continue
			}
			resp := httpAssertDeltaSyncResponseFrom(evt.Payload, *assertRecord)
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
