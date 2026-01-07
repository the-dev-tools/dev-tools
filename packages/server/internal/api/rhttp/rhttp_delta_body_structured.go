//nolint:revive // exported
package rhttp

import (
	"context"
	"errors"
	"sync"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/mutation"
	"the-dev-tools/server/pkg/patch"

	"the-dev-tools/server/pkg/service/shttp"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
)

// streamHttpBodyFormDeltaSync streams HTTP body form delta events to the client
func (h *HttpServiceRPC) HttpBodyFormDataDeltaCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpBodyFormDataDeltaCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Get user's workspaces
	workspaces, err := h.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var allDeltas []*apiv1.HttpBodyFormDataDelta
	for _, workspace := range workspaces {
		// Get HTTP entries for this workspace
		httpList, err := h.hs.GetDeltasByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Get body forms for each HTTP entry
		for _, http := range httpList {
			bodyForms, err := h.httpBodyFormService.GetByHttpID(ctx, http.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			// Convert to delta format
			for _, bodyForm := range bodyForms {
				if !bodyForm.IsDelta {
					continue
				}

				delta := &apiv1.HttpBodyFormDataDelta{
					DeltaHttpBodyFormDataId: bodyForm.ID.Bytes(),
					HttpId:                  bodyForm.HttpID.Bytes(),
				}

				if bodyForm.ParentHttpBodyFormID != nil {
					delta.HttpBodyFormDataId = bodyForm.ParentHttpBodyFormID.Bytes()
				}

				// Only include delta fields if they exist
				if bodyForm.DeltaKey != nil {
					delta.Key = bodyForm.DeltaKey
				}
				if bodyForm.DeltaValue != nil {
					delta.Value = bodyForm.DeltaValue
				}
				if bodyForm.DeltaEnabled != nil {
					delta.Enabled = bodyForm.DeltaEnabled
				}
				if bodyForm.DeltaDescription != nil {
					delta.Description = bodyForm.DeltaDescription
				}
				if bodyForm.DeltaDisplayOrder != nil {
					delta.Order = bodyForm.DeltaDisplayOrder
				}

				allDeltas = append(allDeltas, delta)
			}
		}
	}

	return connect.NewResponse(&apiv1.HttpBodyFormDataDeltaCollectionResponse{
		Items: allDeltas,
	}), nil
}

func (h *HttpServiceRPC) HttpBodyFormDataDeltaInsert(ctx context.Context, req *connect.Request[apiv1.HttpBodyFormDataDeltaInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.Items) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one delta item is required"))
	}

	// FETCH: Gather data and check permissions OUTSIDE transaction
	type insertItem struct {
		bodyFormID  idwrap.IDWrap
		workspaceID idwrap.IDWrap
		item        *apiv1.HttpBodyFormDataDeltaInsert
	}
	insertData := make([]insertItem, 0, len(req.Msg.Items))

	for _, item := range req.Msg.Items {
		if len(item.HttpBodyFormDataId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_body_form_id is required"))
		}

		bodyFormID, err := idwrap.NewFromBytes(item.HttpBodyFormDataId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Check workspace write access
		bodyForm, err := h.httpBodyFormService.GetByID(ctx, bodyFormID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHttpBodyFormFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		httpEntry, err := h.hs.Get(ctx, bodyForm.HttpID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := h.checkWorkspaceWriteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		insertData = append(insertData, insertItem{
			bodyFormID:  bodyFormID,
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
		bodyFormService := h.httpBodyFormService.TX(mut.TX())
		bodyForm, err := bodyFormService.GetByID(ctx, data.bodyFormID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := mut.UpdateHTTPBodyFormDelta(ctx, mutation.HTTPBodyFormDeltaUpdateItem{
			ID:          data.bodyFormID,
			HttpID:      bodyForm.HttpID,
			WorkspaceID: data.workspaceID,
			Params: gen.UpdateHTTPBodyFormDeltaParams{
				ID:                data.bodyFormID,
				DeltaKey:          ptrToNullString(data.item.Key),
				DeltaValue:        ptrToNullString(data.item.Value),
				DeltaEnabled:      data.item.Enabled,
				DeltaDescription:  data.item.Description,
				DeltaDisplayOrder: ptrToNullFloat64(data.item.Order),
			},
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Update payload in tracked event
		updated, err := bodyFormService.GetByID(ctx, data.bodyFormID)
		if err == nil {
			mut.UpdateLastEventPayload(*updated)
		}
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyFormDataDeltaUpdate(ctx context.Context, req *connect.Request[apiv1.HttpBodyFormDataDeltaUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP body form delta must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	type updateItem struct {
		deltaID          idwrap.IDWrap
		existingBodyForm mhttp.HTTPBodyForm
		workspaceID      idwrap.IDWrap
		item             *apiv1.HttpBodyFormDataDeltaUpdate
	}
	updateData := make([]updateItem, 0, len(req.Msg.Items))

	for _, item := range req.Msg.Items {
		if len(item.DeltaHttpBodyFormDataId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("delta_http_body_form_data_id is required"))
		}

		deltaID, err := idwrap.NewFromBytes(item.DeltaHttpBodyFormDataId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing delta body form
		existingBodyForm, err := h.httpBodyFormService.GetByID(ctx, deltaID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHttpBodyFormFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Verify this is actually a delta record
		if !existingBodyForm.IsDelta {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("specified HTTP body form is not a delta"))
		}

		// Get the HTTP entry to check workspace access - use pool service
		httpEntry, err := h.hs.Get(ctx, existingBodyForm.HttpID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := h.checkWorkspaceWriteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		updateData = append(updateData, updateItem{
			deltaID:          deltaID,
			existingBodyForm: *existingBodyForm,
			workspaceID:      httpEntry.WorkspaceID,
			item:             item,
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
		deltaKey := data.existingBodyForm.DeltaKey
		deltaValue := data.existingBodyForm.DeltaValue
		deltaDescription := data.existingBodyForm.DeltaDescription
		deltaEnabled := data.existingBodyForm.DeltaEnabled
		deltaOrder := data.existingBodyForm.DeltaDisplayOrder
		var patchData patch.HTTPBodyFormPatch

		if item.Key != nil {
			switch item.Key.GetKind() {
			case apiv1.HttpBodyFormDataDeltaUpdate_KeyUnion_KIND_UNSET:
				deltaKey = nil
				patchData.Key = patch.Unset[string]()
			case apiv1.HttpBodyFormDataDeltaUpdate_KeyUnion_KIND_VALUE:
				keyStr := item.Key.GetValue()
				deltaKey = &keyStr
				patchData.Key = patch.NewOptional(keyStr)
			}
		}
		if item.Value != nil {
			switch item.Value.GetKind() {
			case apiv1.HttpBodyFormDataDeltaUpdate_ValueUnion_KIND_UNSET:
				deltaValue = nil
				patchData.Value = patch.Unset[string]()
			case apiv1.HttpBodyFormDataDeltaUpdate_ValueUnion_KIND_VALUE:
				valueStr := item.Value.GetValue()
				deltaValue = &valueStr
				patchData.Value = patch.NewOptional(valueStr)
			}
		}
		if item.Enabled != nil {
			switch item.Enabled.GetKind() {
			case apiv1.HttpBodyFormDataDeltaUpdate_EnabledUnion_KIND_UNSET:
				deltaEnabled = nil
				patchData.Enabled = patch.Unset[bool]()
			case apiv1.HttpBodyFormDataDeltaUpdate_EnabledUnion_KIND_VALUE:
				enabledBool := item.Enabled.GetValue()
				deltaEnabled = &enabledBool
				patchData.Enabled = patch.NewOptional(enabledBool)
			}
		}
		if item.Description != nil {
			switch item.Description.GetKind() {
			case apiv1.HttpBodyFormDataDeltaUpdate_DescriptionUnion_KIND_UNSET:
				deltaDescription = nil
				patchData.Description = patch.Unset[string]()
			case apiv1.HttpBodyFormDataDeltaUpdate_DescriptionUnion_KIND_VALUE:
				descStr := item.Description.GetValue()
				deltaDescription = &descStr
				patchData.Description = patch.NewOptional(descStr)
			}
		}
		if item.Order != nil {
			switch item.Order.GetKind() {
			case apiv1.HttpBodyFormDataDeltaUpdate_OrderUnion_KIND_UNSET:
				deltaOrder = nil
				patchData.Order = patch.Unset[float32]()
			case apiv1.HttpBodyFormDataDeltaUpdate_OrderUnion_KIND_VALUE:
				orderVal := item.Order.GetValue()
				deltaOrder = &orderVal
				patchData.Order = patch.NewOptional(orderVal)
			}
		}

		bodyFormService := h.httpBodyFormService.TX(mut.TX())
		if err := mut.UpdateHTTPBodyFormDelta(ctx, mutation.HTTPBodyFormDeltaUpdateItem{
			ID:          data.deltaID,
			HttpID:      data.existingBodyForm.HttpID,
			WorkspaceID: data.workspaceID,
			Params: gen.UpdateHTTPBodyFormDeltaParams{
				ID:                data.deltaID,
				DeltaKey:          ptrToNullString(deltaKey),
				DeltaValue:        ptrToNullString(deltaValue),
				DeltaEnabled:      deltaEnabled,
				DeltaDescription:  deltaDescription,
				DeltaDisplayOrder: ptrToNullFloat64(deltaOrder),
			},
			Patch: patchData,
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Update payload in tracked event
		updated, err := bodyFormService.GetByID(ctx, data.deltaID)
		if err == nil {
			mut.UpdateLastEventPayload(*updated)
		}
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyFormDataDeltaDelete(ctx context.Context, req *connect.Request[apiv1.HttpBodyFormDataDeltaDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP body form delta must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	type deleteItem struct {
		deltaID     idwrap.IDWrap
		httpID      idwrap.IDWrap
		workspaceID idwrap.IDWrap
		bodyForm    mhttp.HTTPBodyForm
	}
	deleteData := make([]deleteItem, 0, len(req.Msg.Items))

	for _, item := range req.Msg.Items {
		if len(item.DeltaHttpBodyFormDataId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("delta_http_body_form_data_id is required"))
		}

		deltaID, err := idwrap.NewFromBytes(item.DeltaHttpBodyFormDataId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing delta body form
		existingBodyForm, err := h.httpBodyFormService.GetByID(ctx, deltaID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHttpBodyFormFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Verify this is actually a delta record
		if !existingBodyForm.IsDelta {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("specified HTTP body form is not a delta"))
		}

		// Get the HTTP entry to check workspace access
		httpEntry, err := h.hs.Get(ctx, existingBodyForm.HttpID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := h.checkWorkspaceDeleteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		deleteData = append(deleteData, deleteItem{
			deltaID:     deltaID,
			httpID:      existingBodyForm.HttpID,
			workspaceID: httpEntry.WorkspaceID,
			bodyForm:    *existingBodyForm,
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
			Entity:      mutation.EntityHTTPBodyForm,
			Op:          mutation.OpDelete,
			ID:          data.deltaID,
			ParentID:    data.httpID,
			WorkspaceID: data.workspaceID,
			IsDelta:     true,
			Payload:     data.bodyForm,
		})
		if err := mut.Queries().DeleteHTTPBodyForm(ctx, data.deltaID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyFormDataDeltaSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpBodyFormDataDeltaSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return h.streamHttpBodyFormDeltaSync(ctx, userID, stream.Send)
}

func (h *HttpServiceRPC) streamHttpBodyFormDeltaSync(ctx context.Context, userID idwrap.IDWrap, send func(*apiv1.HttpBodyFormDataDeltaSyncResponse) error) error {
	var workspaceSet sync.Map

	// Filter for workspace-based access control
	filter := func(topic HttpBodyFormTopic) bool {
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
	events, err := h.streamers.HttpBodyForm.Subscribe(ctx, filter)
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
			// Get the full body form record for delta sync response
			bodyFormID, err := idwrap.NewFromBytes(evt.Payload.HttpBodyForm.GetHttpBodyFormDataId())
			if err != nil {
				continue // Skip if can't parse ID
			}
			bodyFormRecord, err := h.httpBodyFormService.GetByID(ctx, bodyFormID)
			if err != nil {
				continue // Skip if can't get the record
			}
			if !bodyFormRecord.IsDelta {
				continue
			}
			resp := httpBodyFormDataDeltaSyncResponseFrom(evt.Payload, *bodyFormRecord)
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

// HttpBodyUrlEncodedDeltaCollection returns all body URL encoded deltas
func (h *HttpServiceRPC) HttpBodyUrlEncodedDeltaCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpBodyUrlEncodedDeltaCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Get user's workspaces
	workspaces, err := h.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var allDeltas []*apiv1.HttpBodyUrlEncodedDelta
	for _, workspace := range workspaces {
		// Get HTTP entries for this workspace
		httpList, err := h.hs.GetDeltasByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Get body URL encoded for each HTTP entry
		for _, http := range httpList {
			bodyUrlEncodeds, err := h.httpBodyUrlEncodedService.GetByHttpID(ctx, http.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			// Convert to delta format
			for _, bodyUrlEncoded := range bodyUrlEncodeds {
				if !bodyUrlEncoded.IsDelta {
					continue
				}

				delta := &apiv1.HttpBodyUrlEncodedDelta{
					DeltaHttpBodyUrlEncodedId: bodyUrlEncoded.ID.Bytes(),
					HttpId:                    bodyUrlEncoded.HttpID.Bytes(),
				}

				if bodyUrlEncoded.ParentHttpBodyUrlEncodedID != nil {
					delta.HttpBodyUrlEncodedId = bodyUrlEncoded.ParentHttpBodyUrlEncodedID.Bytes()
				}

				// Only include delta fields if they exist
				if bodyUrlEncoded.DeltaKey != nil {
					delta.Key = bodyUrlEncoded.DeltaKey
				}
				if bodyUrlEncoded.DeltaValue != nil {
					delta.Value = bodyUrlEncoded.DeltaValue
				}
				if bodyUrlEncoded.DeltaEnabled != nil {
					delta.Enabled = bodyUrlEncoded.DeltaEnabled
				}
				if bodyUrlEncoded.DeltaDescription != nil {
					delta.Description = bodyUrlEncoded.DeltaDescription
				}
				if bodyUrlEncoded.DeltaDisplayOrder != nil {
					delta.Order = bodyUrlEncoded.DeltaDisplayOrder
				}

				allDeltas = append(allDeltas, delta)
			}
		}
	}

	return connect.NewResponse(&apiv1.HttpBodyUrlEncodedDeltaCollectionResponse{
		Items: allDeltas,
	}), nil
}

func (h *HttpServiceRPC) HttpBodyUrlEncodedDeltaInsert(ctx context.Context, req *connect.Request[apiv1.HttpBodyUrlEncodedDeltaInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.Items) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one delta item is required"))
	}

	// FETCH: Gather data and check permissions OUTSIDE transaction
	type insertItem struct {
		bodyUrlEncodedID idwrap.IDWrap
		workspaceID      idwrap.IDWrap
		item             *apiv1.HttpBodyUrlEncodedDeltaInsert
	}
	insertData := make([]insertItem, 0, len(req.Msg.Items))

	for _, item := range req.Msg.Items {
		if len(item.HttpBodyUrlEncodedId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_body_url_encoded_id is required"))
		}

		bodyUrlEncodedID, err := idwrap.NewFromBytes(item.HttpBodyUrlEncodedId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Check workspace write access
		bodyUrlEncoded, err := h.httpBodyUrlEncodedService.GetByID(ctx, bodyUrlEncodedID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHttpBodyUrlEncodedFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		httpEntry, err := h.hs.Get(ctx, bodyUrlEncoded.HttpID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := h.checkWorkspaceWriteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		insertData = append(insertData, insertItem{
			bodyUrlEncodedID: bodyUrlEncodedID,
			workspaceID:      httpEntry.WorkspaceID,
			item:             item,
		})
	}

	// ACT: Update delta fields using mutation context
	mut := mutation.New(h.DB, mutation.WithPublisher(h.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	for _, data := range insertData {
		bodyUrlEncodedService := h.httpBodyUrlEncodedService.TX(mut.TX())
		bodyUrlEncoded, err := bodyUrlEncodedService.GetByID(ctx, data.bodyUrlEncodedID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := mut.UpdateHTTPBodyUrlEncodedDelta(ctx, mutation.HTTPBodyUrlEncodedDeltaUpdateItem{
			ID:          data.bodyUrlEncodedID,
			HttpID:      bodyUrlEncoded.HttpID,
			WorkspaceID: data.workspaceID,
			Params: gen.UpdateHTTPBodyUrlEncodedDeltaParams{
				ID:                data.bodyUrlEncodedID,
				DeltaKey:          ptrToNullString(data.item.Key),
				DeltaValue:        ptrToNullString(data.item.Value),
				DeltaDescription:  data.item.Description,
				DeltaEnabled:      data.item.Enabled,
				DeltaDisplayOrder: ptrToNullFloat64(data.item.Order),
			},
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Update payload in tracked event
		updated, err := bodyUrlEncodedService.GetByID(ctx, data.bodyUrlEncodedID)
		if err == nil {
			mut.UpdateLastEventPayload(*updated)
		}
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyUrlEncodedDeltaUpdate(ctx context.Context, req *connect.Request[apiv1.HttpBodyUrlEncodedDeltaUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP body URL encoded delta must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	type updateItem struct {
		deltaID                idwrap.IDWrap
		existingBodyUrlEncoded mhttp.HTTPBodyUrlencoded
		workspaceID            idwrap.IDWrap
		item                   *apiv1.HttpBodyUrlEncodedDeltaUpdate
	}
	updateData := make([]updateItem, 0, len(req.Msg.Items))

	for _, item := range req.Msg.Items {
		if len(item.DeltaHttpBodyUrlEncodedId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("delta_http_body_url_encoded_id is required"))
		}

		deltaID, err := idwrap.NewFromBytes(item.DeltaHttpBodyUrlEncodedId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing delta body url encoded
		existingBodyUrlEncoded, err := h.httpBodyUrlEncodedService.GetByID(ctx, deltaID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHttpBodyUrlEncodedFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Verify this is actually a delta record
		if !existingBodyUrlEncoded.IsDelta {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("specified HTTP body URL encoded is not a delta"))
		}

		// Get the HTTP entry to check workspace access - use pool service
		httpEntry, err := h.hs.Get(ctx, existingBodyUrlEncoded.HttpID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Check write access to the workspace
		if err := h.checkWorkspaceWriteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		updateData = append(updateData, updateItem{
			deltaID:                deltaID,
			existingBodyUrlEncoded: *existingBodyUrlEncoded,
			workspaceID:            httpEntry.WorkspaceID,
			item:                   item,
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
		deltaKey := data.existingBodyUrlEncoded.DeltaKey
		deltaValue := data.existingBodyUrlEncoded.DeltaValue
		deltaDescription := data.existingBodyUrlEncoded.DeltaDescription
		deltaEnabled := data.existingBodyUrlEncoded.DeltaEnabled
		deltaOrder := data.existingBodyUrlEncoded.DeltaDisplayOrder
		var patchData patch.HTTPBodyUrlEncodedPatch

		if item.Key != nil {
			switch item.Key.GetKind() {
			case apiv1.HttpBodyUrlEncodedDeltaUpdate_KeyUnion_KIND_UNSET:
				deltaKey = nil
				patchData.Key = patch.Unset[string]()
			case apiv1.HttpBodyUrlEncodedDeltaUpdate_KeyUnion_KIND_VALUE:
				keyStr := item.Key.GetValue()
				deltaKey = &keyStr
				patchData.Key = patch.NewOptional(keyStr)
			}
		}
		if item.Value != nil {
			switch item.Value.GetKind() {
			case apiv1.HttpBodyUrlEncodedDeltaUpdate_ValueUnion_KIND_UNSET:
				deltaValue = nil
				patchData.Value = patch.Unset[string]()
			case apiv1.HttpBodyUrlEncodedDeltaUpdate_ValueUnion_KIND_VALUE:
				valueStr := item.Value.GetValue()
				deltaValue = &valueStr
				patchData.Value = patch.NewOptional(valueStr)
			}
		}
		if item.Enabled != nil {
			switch item.Enabled.GetKind() {
			case apiv1.HttpBodyUrlEncodedDeltaUpdate_EnabledUnion_KIND_UNSET:
				deltaEnabled = nil
				patchData.Enabled = patch.Unset[bool]()
			case apiv1.HttpBodyUrlEncodedDeltaUpdate_EnabledUnion_KIND_VALUE:
				enabledBool := item.Enabled.GetValue()
				deltaEnabled = &enabledBool
				patchData.Enabled = patch.NewOptional(enabledBool)
			}
		}
		if item.Description != nil {
			switch item.Description.GetKind() {
			case apiv1.HttpBodyUrlEncodedDeltaUpdate_DescriptionUnion_KIND_UNSET:
				deltaDescription = nil
				patchData.Description = patch.Unset[string]()
			case apiv1.HttpBodyUrlEncodedDeltaUpdate_DescriptionUnion_KIND_VALUE:
				descStr := item.Description.GetValue()
				deltaDescription = &descStr
				patchData.Description = patch.NewOptional(descStr)
			}
		}
		if item.Order != nil {
			switch item.Order.GetKind() {
			case apiv1.HttpBodyUrlEncodedDeltaUpdate_OrderUnion_KIND_UNSET:
				deltaOrder = nil
				patchData.Order = patch.Unset[float32]()
			case apiv1.HttpBodyUrlEncodedDeltaUpdate_OrderUnion_KIND_VALUE:
				orderVal := item.Order.GetValue()
				deltaOrder = &orderVal
				patchData.Order = patch.NewOptional(orderVal)
			}
		}

		bodyUrlEncodedService := h.httpBodyUrlEncodedService.TX(mut.TX())
		if err := mut.UpdateHTTPBodyUrlEncodedDelta(ctx, mutation.HTTPBodyUrlEncodedDeltaUpdateItem{
			ID:          data.deltaID,
			HttpID:      data.existingBodyUrlEncoded.HttpID,
			WorkspaceID: data.workspaceID,
			Params: gen.UpdateHTTPBodyUrlEncodedDeltaParams{
				ID:                data.deltaID,
				DeltaKey:          ptrToNullString(deltaKey),
				DeltaValue:        ptrToNullString(deltaValue),
				DeltaEnabled:      deltaEnabled,
				DeltaDescription:  deltaDescription,
				DeltaDisplayOrder: ptrToNullFloat64(deltaOrder),
			},
			Patch: patchData,
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Update payload in tracked event
		updated, err := bodyUrlEncodedService.GetByID(ctx, data.deltaID)
		if err == nil {
			mut.UpdateLastEventPayload(*updated)
		}
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyUrlEncodedDeltaDelete(ctx context.Context, req *connect.Request[apiv1.HttpBodyUrlEncodedDeltaDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP body URL encoded delta must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	type deleteItem struct {
		deltaID     idwrap.IDWrap
		httpID      idwrap.IDWrap
		workspaceID idwrap.IDWrap
		bodyUrl     mhttp.HTTPBodyUrlencoded
	}
	deleteData := make([]deleteItem, 0, len(req.Msg.Items))

	for _, item := range req.Msg.Items {
		if len(item.DeltaHttpBodyUrlEncodedId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("delta_http_body_url_encoded_id is required"))
		}

		deltaID, err := idwrap.NewFromBytes(item.DeltaHttpBodyUrlEncodedId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing delta body url encoded - use pool service
		existingBodyUrlEncoded, err := h.httpBodyUrlEncodedService.GetByID(ctx, deltaID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHttpBodyUrlEncodedFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Verify this is actually a delta record
		if !existingBodyUrlEncoded.IsDelta {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("specified HTTP body URL encoded is not a delta"))
		}

		// Get the HTTP entry to check workspace access - use pool service
		httpEntry, err := h.hs.Get(ctx, existingBodyUrlEncoded.HttpID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Check delete access to the workspace
		if err := h.checkWorkspaceDeleteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		deleteData = append(deleteData, deleteItem{
			deltaID:     deltaID,
			httpID:      existingBodyUrlEncoded.HttpID,
			workspaceID: httpEntry.WorkspaceID,
			bodyUrl:     *existingBodyUrlEncoded,
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
			Entity:      mutation.EntityHTTPBodyURL,
			Op:          mutation.OpDelete,
			ID:          data.deltaID,
			ParentID:    data.httpID,
			WorkspaceID: data.workspaceID,
			IsDelta:     true,
			Payload:     data.bodyUrl,
		})
		if err := mut.Queries().DeleteHTTPBodyUrlEncoded(ctx, data.deltaID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := mut.Commit(ctx); err != nil { // Auto-publishes events!
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyUrlEncodedDeltaSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpBodyUrlEncodedDeltaSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return h.streamHttpBodyUrlEncodedDeltaSync(ctx, userID, stream.Send)
}

func (h *HttpServiceRPC) streamHttpBodyUrlEncodedDeltaSync(ctx context.Context, userID idwrap.IDWrap, send func(*apiv1.HttpBodyUrlEncodedDeltaSyncResponse) error) error {
	var workspaceSet sync.Map

	// Filter for workspace-based access control
	filter := func(topic HttpBodyUrlEncodedTopic) bool {
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
	events, err := h.streamers.HttpBodyUrlEncoded.Subscribe(ctx, filter)
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
			// Get the full body URL encoded record for delta sync response
			bodyID, err := idwrap.NewFromBytes(evt.Payload.HttpBodyUrlEncoded.GetHttpBodyUrlEncodedId())
			if err != nil {
				continue // Skip if can't parse ID
			}
			bodyRecord, err := h.httpBodyUrlEncodedService.GetByID(ctx, bodyID)
			if err != nil {
				continue // Skip if can't get the record
			}
			if !bodyRecord.IsDelta {
				continue
			}
			resp := httpBodyUrlEncodedDeltaSyncResponseFrom(evt.Payload, *bodyRecord)
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
