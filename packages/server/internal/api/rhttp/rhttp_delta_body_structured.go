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

	// Process each delta item
	for _, item := range req.Msg.Items {
		if len(item.HttpBodyFormDataId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_body_form_id is required for each delta item"))
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

		// Update delta fields
		err = h.httpBodyFormService.UpdateDelta(ctx, bodyFormID, item.Key, item.Value, item.Enabled, item.Description, item.Order)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyFormDataDeltaUpdate(ctx context.Context, req *connect.Request[apiv1.HttpBodyFormDataDeltaUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP body form delta must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var updateData []struct {
		deltaID          idwrap.IDWrap
		existingBodyForm *mhttp.HTTPBodyForm
		item             *apiv1.HttpBodyFormDataDeltaUpdate
	}

	for _, item := range req.Msg.Items {
		if len(item.DeltaHttpBodyFormDataId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("delta_http_body_form_data_id is required"))
		}

		deltaID, err := idwrap.NewFromBytes(item.DeltaHttpBodyFormDataId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing delta body form - use pool service
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

		// Check write access to the workspace
		if err := h.checkWorkspaceWriteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		updateData = append(updateData, struct {
			deltaID          idwrap.IDWrap
			existingBodyForm *mhttp.HTTPBodyForm
			item             *apiv1.HttpBodyFormDataDeltaUpdate
		}{
			deltaID:          deltaID,
			existingBodyForm: existingBodyForm,
			item:             item,
		})
	}

	// Step 2: Prepare updates (in memory)
	var preparedUpdates []struct {
		deltaID          idwrap.IDWrap
		deltaKey         *string
		deltaValue       *string
		deltaEnabled     *bool
		deltaDescription *string
		deltaOrder       *float32
	}
	var patches []DeltaPatch

	for _, data := range updateData {
		item := data.item
		deltaKey := data.existingBodyForm.DeltaKey
		deltaValue := data.existingBodyForm.DeltaValue
		deltaDescription := data.existingBodyForm.DeltaDescription
		deltaEnabled := data.existingBodyForm.DeltaEnabled
		deltaOrder := data.existingBodyForm.DeltaDisplayOrder
		patch := make(DeltaPatch)

		if item.Key != nil {
			switch item.Key.GetKind() {
			case apiv1.HttpBodyFormDataDeltaUpdate_KeyUnion_KIND_UNSET:
				deltaKey = nil
				patch["key"] = nil
			case apiv1.HttpBodyFormDataDeltaUpdate_KeyUnion_KIND_VALUE:
				keyStr := item.Key.GetValue()
				deltaKey = &keyStr
				patch["key"] = &keyStr
			}
		}
		if item.Value != nil {
			switch item.Value.GetKind() {
			case apiv1.HttpBodyFormDataDeltaUpdate_ValueUnion_KIND_UNSET:
				deltaValue = nil
				patch["value"] = nil
			case apiv1.HttpBodyFormDataDeltaUpdate_ValueUnion_KIND_VALUE:
				valueStr := item.Value.GetValue()
				deltaValue = &valueStr
				patch["value"] = &valueStr
			}
		}
		if item.Enabled != nil {
			switch item.Enabled.GetKind() {
			case apiv1.HttpBodyFormDataDeltaUpdate_EnabledUnion_KIND_UNSET:
				deltaEnabled = nil
				patch["enabled"] = nil
			case apiv1.HttpBodyFormDataDeltaUpdate_EnabledUnion_KIND_VALUE:
				enabledBool := item.Enabled.GetValue()
				deltaEnabled = &enabledBool
				patch["enabled"] = &enabledBool
			}
		}
		if item.Description != nil {
			switch item.Description.GetKind() {
			case apiv1.HttpBodyFormDataDeltaUpdate_DescriptionUnion_KIND_UNSET:
				deltaDescription = nil
				patch["description"] = nil
			case apiv1.HttpBodyFormDataDeltaUpdate_DescriptionUnion_KIND_VALUE:
				descStr := item.Description.GetValue()
				deltaDescription = &descStr
				patch["description"] = &descStr
			}
		}
		if item.Order != nil {
			switch item.Order.GetKind() {
			case apiv1.HttpBodyFormDataDeltaUpdate_OrderUnion_KIND_UNSET:
				deltaOrder = nil
				patch["order"] = nil
			case apiv1.HttpBodyFormDataDeltaUpdate_OrderUnion_KIND_VALUE:
				orderVal := item.Order.GetValue()
				deltaOrder = &orderVal
				patch["order"] = &orderVal
			}
		}

		patches = append(patches, patch)
		preparedUpdates = append(preparedUpdates, struct {
			deltaID          idwrap.IDWrap
			deltaKey         *string
			deltaValue       *string
			deltaEnabled     *bool
			deltaDescription *string
			deltaOrder       *float32
		}{
			deltaID:          data.deltaID,
			deltaKey:         deltaKey,
			deltaValue:       deltaValue,
			deltaEnabled:     deltaEnabled,
			deltaDescription: deltaDescription,
			deltaOrder:       deltaOrder,
		})
	}

	// Step 3: Execute updates in transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	httpBodyFormService := h.httpBodyFormService.TX(tx)
	var updatedBodyForms []mhttp.HTTPBodyForm

	for _, update := range preparedUpdates {
		if err := httpBodyFormService.UpdateDelta(ctx, update.deltaID, update.deltaKey, update.deltaValue, update.deltaEnabled, update.deltaDescription, update.deltaOrder); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Get updated body form for event publishing (from TX service)
		updatedBodyForm, err := httpBodyFormService.GetByID(ctx, update.deltaID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		updatedBodyForms = append(updatedBodyForms, *updatedBodyForm)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish update events for real-time sync after successful commit
	for i, bodyForm := range updatedBodyForms {
		// Get workspace ID for the HTTP entry
		httpEntry, err := h.hs.Get(ctx, bodyForm.HttpID)
		if err != nil {
			continue
		}
		h.streamers.HttpBodyForm.Publish(HttpBodyFormTopic{WorkspaceID: httpEntry.WorkspaceID}, HttpBodyFormEvent{
			Type:         eventTypeUpdate,
			IsDelta:      true,
			Patch:        patches[i],
			HttpBodyForm: converter.ToAPIHttpBodyFormData(bodyForm),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyFormDataDeltaDelete(ctx context.Context, req *connect.Request[apiv1.HttpBodyFormDataDeltaDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP body form delta must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var deleteData []struct {
		deltaID          idwrap.IDWrap
		existingBodyForm *mhttp.HTTPBodyForm
		workspaceID      idwrap.IDWrap
	}

	for _, item := range req.Msg.Items {
		if len(item.DeltaHttpBodyFormDataId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("delta_http_body_form_data_id is required"))
		}

		deltaID, err := idwrap.NewFromBytes(item.DeltaHttpBodyFormDataId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing delta body form - use pool service
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

		// Check delete access to the workspace
		if err := h.checkWorkspaceDeleteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		deleteData = append(deleteData, struct {
			deltaID          idwrap.IDWrap
			existingBodyForm *mhttp.HTTPBodyForm
			workspaceID      idwrap.IDWrap
		}{
			deltaID:          deltaID,
			existingBodyForm: existingBodyForm,
			workspaceID:      httpEntry.WorkspaceID,
		})
	}

	// Step 2: Execute deletes in transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	httpBodyFormService := h.httpBodyFormService.TX(tx)
	var deletedBodyForms []mhttp.HTTPBodyForm
	var deletedWorkspaceIDs []idwrap.IDWrap

	for _, data := range deleteData {
		// Delete the delta record
		if err := httpBodyFormService.Delete(ctx, data.deltaID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		deletedBodyForms = append(deletedBodyForms, *data.existingBodyForm)
		deletedWorkspaceIDs = append(deletedWorkspaceIDs, data.workspaceID)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish delete events for real-time sync after successful commit
	for i, bodyForm := range deletedBodyForms {
		h.streamers.HttpBodyForm.Publish(HttpBodyFormTopic{WorkspaceID: deletedWorkspaceIDs[i]}, HttpBodyFormEvent{
			Type:         eventTypeDelete,
			HttpBodyForm: converter.ToAPIHttpBodyFormData(bodyForm),
		})
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

// streamHttpAssertDeltaSync streams HTTP assert delta events to the client
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

	// Process each delta item
	for _, item := range req.Msg.Items {
		if len(item.HttpBodyUrlEncodedId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_body_url_encoded_id is required for each delta item"))
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

		// Update delta fields
		err = h.httpBodyUrlEncodedService.UpdateDelta(ctx, bodyUrlEncodedID, item.Key, item.Value, item.Enabled, item.Description, item.Order)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyUrlEncodedDeltaUpdate(ctx context.Context, req *connect.Request[apiv1.HttpBodyUrlEncodedDeltaUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP body URL encoded delta must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var updateData []struct {
		deltaID                idwrap.IDWrap
		existingBodyUrlEncoded *mhttp.HTTPBodyUrlencoded
		item                   *apiv1.HttpBodyUrlEncodedDeltaUpdate
	}

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

		// Check write access to the workspace
		if err := h.checkWorkspaceWriteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		updateData = append(updateData, struct {
			deltaID                idwrap.IDWrap
			existingBodyUrlEncoded *mhttp.HTTPBodyUrlencoded
			item                   *apiv1.HttpBodyUrlEncodedDeltaUpdate
		}{
			deltaID:                deltaID,
			existingBodyUrlEncoded: existingBodyUrlEncoded,
			item:                   item,
		})
	}

	// Step 2: Prepare updates (in memory)
	var preparedUpdates []struct {
		deltaID          idwrap.IDWrap
		deltaKey         *string
		deltaValue       *string
		deltaEnabled     *bool
		deltaDescription *string
		deltaOrder       *float32
	}
	var patches []DeltaPatch

	for _, data := range updateData {
		item := data.item
		deltaKey := data.existingBodyUrlEncoded.DeltaKey
		deltaValue := data.existingBodyUrlEncoded.DeltaValue
		deltaDescription := data.existingBodyUrlEncoded.DeltaDescription
		deltaEnabled := data.existingBodyUrlEncoded.DeltaEnabled
		deltaOrder := data.existingBodyUrlEncoded.DeltaDisplayOrder
		patch := make(DeltaPatch)

		if item.Key != nil {
			switch item.Key.GetKind() {
			case apiv1.HttpBodyUrlEncodedDeltaUpdate_KeyUnion_KIND_UNSET:
				deltaKey = nil
				patch["key"] = nil
			case apiv1.HttpBodyUrlEncodedDeltaUpdate_KeyUnion_KIND_VALUE:
				keyStr := item.Key.GetValue()
				deltaKey = &keyStr
				patch["key"] = &keyStr
			}
		}
		if item.Value != nil {
			switch item.Value.GetKind() {
			case apiv1.HttpBodyUrlEncodedDeltaUpdate_ValueUnion_KIND_UNSET:
				deltaValue = nil
				patch["value"] = nil
			case apiv1.HttpBodyUrlEncodedDeltaUpdate_ValueUnion_KIND_VALUE:
				valueStr := item.Value.GetValue()
				deltaValue = &valueStr
				patch["value"] = &valueStr
			}
		}
		if item.Enabled != nil {
			switch item.Enabled.GetKind() {
			case apiv1.HttpBodyUrlEncodedDeltaUpdate_EnabledUnion_KIND_UNSET:
				deltaEnabled = nil
				patch["enabled"] = nil
			case apiv1.HttpBodyUrlEncodedDeltaUpdate_EnabledUnion_KIND_VALUE:
				enabledBool := item.Enabled.GetValue()
				deltaEnabled = &enabledBool
				patch["enabled"] = &enabledBool
			}
		}
		if item.Description != nil {
			switch item.Description.GetKind() {
			case apiv1.HttpBodyUrlEncodedDeltaUpdate_DescriptionUnion_KIND_UNSET:
				deltaDescription = nil
				patch["description"] = nil
			case apiv1.HttpBodyUrlEncodedDeltaUpdate_DescriptionUnion_KIND_VALUE:
				descStr := item.Description.GetValue()
				deltaDescription = &descStr
				patch["description"] = &descStr
			}
		}
		if item.Order != nil {
			switch item.Order.GetKind() {
			case apiv1.HttpBodyUrlEncodedDeltaUpdate_OrderUnion_KIND_UNSET:
				deltaOrder = nil
				patch["order"] = nil
			case apiv1.HttpBodyUrlEncodedDeltaUpdate_OrderUnion_KIND_VALUE:
				orderVal := item.Order.GetValue()
				deltaOrder = &orderVal
				patch["order"] = &orderVal
			}
		}

		patches = append(patches, patch)
		preparedUpdates = append(preparedUpdates, struct {
			deltaID          idwrap.IDWrap
			deltaKey         *string
			deltaValue       *string
			deltaEnabled     *bool
			deltaDescription *string
			deltaOrder       *float32
		}{
			deltaID:          data.deltaID,
			deltaKey:         deltaKey,
			deltaValue:       deltaValue,
			deltaEnabled:     deltaEnabled,
			deltaDescription: deltaDescription,
			deltaOrder:       deltaOrder,
		})
	}

	// Step 3: Execute updates in transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	httpBodyUrlEncodedService := h.httpBodyUrlEncodedService.TX(tx)
	var updatedBodyUrlEncodeds []mhttp.HTTPBodyUrlencoded

	for _, update := range preparedUpdates {
		if err := httpBodyUrlEncodedService.UpdateDelta(ctx, update.deltaID, update.deltaKey, update.deltaValue, update.deltaEnabled, update.deltaDescription, update.deltaOrder); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Get updated body url encoded for event publishing (from TX service)
		updatedBodyUrlEncoded, err := httpBodyUrlEncodedService.GetByID(ctx, update.deltaID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		updatedBodyUrlEncodeds = append(updatedBodyUrlEncodeds, *updatedBodyUrlEncoded)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish update events for real-time sync after successful commit
	for i, bodyUrlEncoded := range updatedBodyUrlEncodeds {
		// Get workspace ID for the HTTP entry
		httpEntry, err := h.hs.Get(ctx, bodyUrlEncoded.HttpID)
		if err != nil {
			continue
		}
		h.streamers.HttpBodyUrlEncoded.Publish(HttpBodyUrlEncodedTopic{WorkspaceID: httpEntry.WorkspaceID}, HttpBodyUrlEncodedEvent{
			Type:               eventTypeUpdate,
			IsDelta:            true,
			Patch:              patches[i],
			HttpBodyUrlEncoded: converter.ToAPIHttpBodyUrlEncoded(bodyUrlEncoded),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyUrlEncodedDeltaDelete(ctx context.Context, req *connect.Request[apiv1.HttpBodyUrlEncodedDeltaDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP body URL encoded delta must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var deleteData []struct {
		deltaID                idwrap.IDWrap
		existingBodyUrlEncoded *mhttp.HTTPBodyUrlencoded
		workspaceID            idwrap.IDWrap
	}

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

		deleteData = append(deleteData, struct {
			deltaID                idwrap.IDWrap
			existingBodyUrlEncoded *mhttp.HTTPBodyUrlencoded
			workspaceID            idwrap.IDWrap
		}{
			deltaID:                deltaID,
			existingBodyUrlEncoded: existingBodyUrlEncoded,
			workspaceID:            httpEntry.WorkspaceID,
		})
	}

	// Step 2: Execute deletes in transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	httpBodyUrlEncodedService := h.httpBodyUrlEncodedService.TX(tx)
	var deletedBodyUrlEncodeds []mhttp.HTTPBodyUrlencoded
	var deletedWorkspaceIDs []idwrap.IDWrap

	for _, data := range deleteData {
		// Delete the delta record
		if err := httpBodyUrlEncodedService.Delete(ctx, data.deltaID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		deletedBodyUrlEncodeds = append(deletedBodyUrlEncodeds, *data.existingBodyUrlEncoded)
		deletedWorkspaceIDs = append(deletedWorkspaceIDs, data.workspaceID)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish delete events for real-time sync after successful commit
	for i, bodyUrlEncoded := range deletedBodyUrlEncodeds {
		h.streamers.HttpBodyUrlEncoded.Publish(HttpBodyUrlEncodedTopic{WorkspaceID: deletedWorkspaceIDs[i]}, HttpBodyUrlEncodedEvent{
			Type:               eventTypeDelete,
			HttpBodyUrlEncoded: converter.ToAPIHttpBodyUrlEncoded(bodyUrlEncoded),
		})
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
