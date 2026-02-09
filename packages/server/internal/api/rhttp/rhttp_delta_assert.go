//nolint:revive // exported
package rhttp

import (
	"context"
	"database/sql"
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

	// FETCH: Gather data and check permissions OUTSIDE transaction
	type insertItem struct {
		httpID      idwrap.IDWrap
		newID       idwrap.IDWrap
		parentID    idwrap.IDWrap
		workspaceID idwrap.IDWrap
		baseAssert  mhttp.HTTPAssert
		item        *apiv1.HttpAssertDeltaInsert
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

		if len(item.HttpAssertId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_assert_id is required"))
		}

		parentAssertID, err := idwrap.NewFromBytes(item.HttpAssertId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		baseAssert, err := h.httpAssertService.GetByID(ctx, parentAssertID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHttpAssertFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		newID := idwrap.NewNow()
		if len(item.DeltaHttpAssertId) > 0 {
			newID, err = idwrap.NewFromBytes(item.DeltaHttpAssertId)
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, err)
			}
		}

		insertData = append(insertData, insertItem{
			httpID:      httpID,
			newID:       newID,
			parentID:    parentAssertID,
			workspaceID: httpEntry.WorkspaceID,
			baseAssert:  *baseAssert,
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
		params := gen.CreateHTTPAssertParams{
			ID:                 data.newID,
			HttpID:             data.httpID,
			Value:              data.baseAssert.Value,
			Enabled:            data.baseAssert.Enabled,
			Description:        data.baseAssert.Description,
			DisplayOrder:       float64(data.baseAssert.DisplayOrder),
			ParentHttpAssertID: data.parentID.Bytes(),
			IsDelta:            true,
			DeltaValue:         ptrToNullString(data.item.Value),
			DeltaEnabled:       data.item.Enabled,
			DeltaDescription:   sql.NullString{},
			DeltaDisplayOrder:  ptrToNullFloat64(data.item.Order),
			CreatedAt:          now,
			UpdatedAt:          now,
		}

		if err := mut.InsertHTTPAssert(ctx, mutation.HTTPAssertInsertItem{
			ID:          data.newID,
			HttpID:      data.httpID,
			WorkspaceID: data.workspaceID,
			IsDelta:     true,
			Params:      params,
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		assertService := h.httpAssertService.TX(mut.TX())
		updated, err := assertService.GetByID(ctx, data.newID)
		if err == nil {
			mut.UpdateLastEventPayload(*updated)
		}
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpAssertDeltaUpdate(ctx context.Context, req *connect.Request[apiv1.HttpAssertDeltaUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP assert delta must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	type updateItem struct {
		deltaID        idwrap.IDWrap
		existingAssert mhttp.HTTPAssert
		workspaceID    idwrap.IDWrap
		item           *apiv1.HttpAssertDeltaUpdate
	}
	updateData := make([]updateItem, 0, len(req.Msg.Items))

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

		updateData = append(updateData, updateItem{
			deltaID:        deltaID,
			existingAssert: *existingAssert,
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
		deltaValue := data.existingAssert.DeltaValue
		deltaEnabled := data.existingAssert.DeltaEnabled
		deltaOrder := data.existingAssert.DeltaDisplayOrder
		var patchData patch.HTTPAssertPatch

		if item.Value != nil {
			switch item.Value.GetKind() {
			case apiv1.HttpAssertDeltaUpdate_ValueUnion_KIND_UNSET:
				deltaValue = nil
				patchData.Value = patch.Unset[string]()
			case apiv1.HttpAssertDeltaUpdate_ValueUnion_KIND_VALUE:
				valueStr := item.Value.GetValue()
				deltaValue = &valueStr
				patchData.Value = patch.NewOptional(valueStr)
			}
		}
		if item.Enabled != nil {
			switch item.Enabled.GetKind() {
			case apiv1.HttpAssertDeltaUpdate_EnabledUnion_KIND_UNSET:
				deltaEnabled = nil
				patchData.Enabled = patch.Unset[bool]()
			case apiv1.HttpAssertDeltaUpdate_EnabledUnion_KIND_VALUE:
				enabledBool := item.Enabled.GetValue()
				deltaEnabled = &enabledBool
				patchData.Enabled = patch.NewOptional(enabledBool)
			}
		}
		if item.Order != nil {
			switch item.Order.GetKind() {
			case apiv1.HttpAssertDeltaUpdate_OrderUnion_KIND_UNSET:
				deltaOrder = nil
				patchData.Order = patch.Unset[float32]()
			case apiv1.HttpAssertDeltaUpdate_OrderUnion_KIND_VALUE:
				orderFloat := item.Order.GetValue()
				deltaOrder = &orderFloat
				patchData.Order = patch.NewOptional(orderFloat)
			}
		}

		assertService := h.httpAssertService.TX(mut.TX())
		if err := mut.UpdateHTTPAssertDelta(ctx, mutation.HTTPAssertDeltaUpdateItem{
			ID:          data.deltaID,
			HttpID:      data.existingAssert.HttpID,
			WorkspaceID: data.workspaceID,
			Params: gen.UpdateHTTPAssertDeltaParams{
				ID:                data.deltaID,
				DeltaValue:        ptrToNullString(deltaValue),
				DeltaEnabled:      deltaEnabled,
				DeltaDisplayOrder: ptrToNullFloat64(deltaOrder),
			},
			Patch: patchData,
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Update payload in tracked event
		updated, err := assertService.GetByID(ctx, data.deltaID)
		if err == nil {
			mut.UpdateLastEventPayload(*updated)
		}
	}

	if err := mut.Commit(ctx); err != nil { // Auto-publishes events!
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpAssertDeltaDelete(ctx context.Context, req *connect.Request[apiv1.HttpAssertDeltaDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP assert delta must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	type deleteItem struct {
		deltaID     idwrap.IDWrap
		httpID      idwrap.IDWrap
		workspaceID idwrap.IDWrap
		assert      mhttp.HTTPAssert
	}
	deleteData := make([]deleteItem, 0, len(req.Msg.Items))

	for _, item := range req.Msg.Items {
		if len(item.DeltaHttpAssertId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("delta_http_assert_id is required"))
		}

		deltaID, err := idwrap.NewFromBytes(item.DeltaHttpAssertId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing delta assert
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

		// Get the HTTP entry to check workspace access
		httpEntry, err := h.hs.Get(ctx, existingAssert.HttpID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Check delete access to the workspace
		if err := h.checkWorkspaceDeleteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		deleteData = append(deleteData, deleteItem{
			deltaID:     deltaID,
			httpID:      existingAssert.HttpID,
			workspaceID: httpEntry.WorkspaceID,
			assert:      *existingAssert,
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
			Entity:      mutation.EntityHTTPAssert,
			Op:          mutation.OpDelete,
			ID:          data.deltaID,
			ParentID:    data.httpID,
			WorkspaceID: data.workspaceID,
			IsDelta:     true,
			Payload:     data.assert,
		})
		if err := mut.Queries().DeleteHTTPAssert(ctx, data.deltaID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
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
