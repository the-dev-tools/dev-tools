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

func (h *HttpServiceRPC) HttpBodyFormDataCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpBodyFormDataCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Get user's workspaces
	workspaces, err := h.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var allBodyForms []*apiv1.HttpBodyFormData
	for _, workspace := range workspaces {
		// Get HTTP entries for this workspace
		httpList, err := h.httpReader.GetByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Get body forms for each HTTP entry
		for _, http := range httpList {
			bodyForms, err := h.httpBodyFormService.GetByHttpID(ctx, http.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			// Convert to API format
			for _, bodyForm := range bodyForms {
				apiBodyForm := converter.ToAPIHttpBodyFormData(bodyForm)
				allBodyForms = append(allBodyForms, apiBodyForm)
			}
		}
	}

	return connect.NewResponse(&apiv1.HttpBodyFormDataCollectionResponse{Items: allBodyForms}), nil
}

func (h *HttpServiceRPC) HttpBodyFormDataInsert(ctx context.Context, req *connect.Request[apiv1.HttpBodyFormDataInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP body form must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var insertData []struct {
		bodyFormModel *mhttp.HTTPBodyForm
	}

	for _, item := range req.Msg.Items {
		if len(item.HttpBodyFormDataId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_body_form_data_id is required"))
		}
		if len(item.HttpId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_id is required"))
		}

		bodyFormID, err := idwrap.NewFromBytes(item.HttpBodyFormDataId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		httpID, err := idwrap.NewFromBytes(item.HttpId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Verify the HTTP entry exists and user has access - use pool service
		httpEntry, err := h.httpReader.Get(ctx, httpID)
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

		// Create the body form model
		bodyFormModel := &mhttp.HTTPBodyForm{
			ID:           bodyFormID,
			HttpID:       httpID,
			Key:          item.Key,
			Value:        item.Value,
			Enabled:      item.Enabled,
			Description:  item.Description,
			DisplayOrder: item.Order,
		}

		insertData = append(insertData, struct {
			bodyFormModel *mhttp.HTTPBodyForm
		}{
			bodyFormModel: bodyFormModel,
		})
	}

	// Step 2: Execute inserts in transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	bodyFormWriter := shttp.NewBodyFormWriter(tx)

	var createdBodyForms []mhttp.HTTPBodyForm

	for _, data := range insertData {
		if err := bodyFormWriter.Create(ctx, data.bodyFormModel); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		createdBodyForms = append(createdBodyForms, *data.bodyFormModel)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish create events for real-time sync
	for _, bodyForm := range createdBodyForms {
		// Get workspace ID for the HTTP entry
		httpEntry, err := h.httpReader.Get(ctx, bodyForm.HttpID)
		if err != nil {
			// Log error but continue - event publishing shouldn't fail the operation
			continue
		}
		h.streamers.HttpBodyForm.Publish(HttpBodyFormTopic{WorkspaceID: httpEntry.WorkspaceID}, HttpBodyFormEvent{
			Type:         eventTypeInsert,
			IsDelta:      bodyForm.IsDelta,
			HttpBodyForm: converter.ToAPIHttpBodyFormData(bodyForm),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyFormDataUpdate(ctx context.Context, req *connect.Request[apiv1.HttpBodyFormDataUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP body form must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var updateData []struct {
		existingBodyForm *mhttp.HTTPBodyForm
		item             *apiv1.HttpBodyFormDataUpdate
	}

	for _, item := range req.Msg.Items {
		if len(item.HttpBodyFormDataId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_body_form_data_id is required"))
		}

		bodyFormID, err := idwrap.NewFromBytes(item.HttpBodyFormDataId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing body form - use pool service
		existingBodyForm, err := h.httpBodyFormService.GetByID(ctx, bodyFormID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHttpBodyFormFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Verify the HTTP entry exists and user has access - use pool service
		httpEntry, err := h.httpReader.Get(ctx, existingBodyForm.HttpID)
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

		updateData = append(updateData, struct {
			existingBodyForm *mhttp.HTTPBodyForm
			item             *apiv1.HttpBodyFormDataUpdate
		}{
			existingBodyForm: existingBodyForm,
			item:             item,
		})
	}

	// Step 2: Prepare updates (in memory)
	for _, data := range updateData {
		item := data.item
		existingBodyForm := data.existingBodyForm

		if item.Key != nil {
			existingBodyForm.Key = *item.Key
		}
		if item.Value != nil {
			existingBodyForm.Value = *item.Value
		}
		if item.Enabled != nil {
			existingBodyForm.Enabled = *item.Enabled
		}
		if item.Description != nil {
			existingBodyForm.Description = *item.Description
		}
		if item.Order != nil {
			existingBodyForm.DisplayOrder = *item.Order
		}
	}

	// Step 3: Execute updates in transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	bodyFormWriter := shttp.NewBodyFormWriter(tx)
	var updatedBodyForms []mhttp.HTTPBodyForm

	for _, data := range updateData {
		if err := bodyFormWriter.Update(ctx, data.existingBodyForm); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		updatedBodyForms = append(updatedBodyForms, *data.existingBodyForm)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish update events for real-time sync
	for _, bodyForm := range updatedBodyForms {
		// Get workspace ID for the HTTP entry
		httpEntry, err := h.httpReader.Get(ctx, bodyForm.HttpID)
		if err != nil {
			continue
		}
		h.streamers.HttpBodyForm.Publish(HttpBodyFormTopic{WorkspaceID: httpEntry.WorkspaceID}, HttpBodyFormEvent{
			Type:         eventTypeUpdate,
			IsDelta:      bodyForm.IsDelta,
			HttpBodyForm: converter.ToAPIHttpBodyFormData(bodyForm),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyFormDataDelete(ctx context.Context, req *connect.Request[apiv1.HttpBodyFormDataDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP body form must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var deleteData []struct {
		bodyFormID       idwrap.IDWrap
		existingBodyForm *mhttp.HTTPBodyForm
		workspaceID      idwrap.IDWrap
	}

	for _, item := range req.Msg.Items {
		if len(item.HttpBodyFormDataId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_body_form_data_id is required"))
		}

		bodyFormID, err := idwrap.NewFromBytes(item.HttpBodyFormDataId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing body form - use pool service
		existingBodyForm, err := h.httpBodyFormService.GetByID(ctx, bodyFormID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHttpBodyFormFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Verify the HTTP entry exists and user has access - use pool service
		httpEntry, err := h.httpReader.Get(ctx, existingBodyForm.HttpID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHTTPFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Check delete access to the workspace
		if err := h.checkWorkspaceDeleteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		deleteData = append(deleteData, struct {
			bodyFormID       idwrap.IDWrap
			existingBodyForm *mhttp.HTTPBodyForm
			workspaceID      idwrap.IDWrap
		}{
			bodyFormID:       bodyFormID,
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

	bodyFormWriter := shttp.NewBodyFormWriter(tx)
	var deletedBodyForms []mhttp.HTTPBodyForm
	var deletedWorkspaceIDs []idwrap.IDWrap

	for _, data := range deleteData {
		if err := bodyFormWriter.Delete(ctx, data.bodyFormID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		deletedBodyForms = append(deletedBodyForms, *data.existingBodyForm)
		deletedWorkspaceIDs = append(deletedWorkspaceIDs, data.workspaceID)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish delete events for real-time sync
	for i, bodyForm := range deletedBodyForms {
		h.streamers.HttpBodyForm.Publish(HttpBodyFormTopic{WorkspaceID: deletedWorkspaceIDs[i]}, HttpBodyFormEvent{
			Type:         eventTypeDelete,
			IsDelta:      bodyForm.IsDelta,
			HttpBodyForm: converter.ToAPIHttpBodyFormData(bodyForm),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyUrlEncodedCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpBodyUrlEncodedCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Get user's workspaces
	workspaces, err := h.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var allBodyUrlEncodeds []*apiv1.HttpBodyUrlEncoded
	for _, workspace := range workspaces {
		// Get HTTP entries for this workspace
		httpList, err := h.httpReader.GetByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Get body URL encoded for each HTTP entry
		for _, http := range httpList {
			bodyUrlEncodeds, err := h.httpBodyUrlEncodedService.GetByHttpID(ctx, http.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			// Convert to API format
			for _, bodyUrlEncoded := range bodyUrlEncodeds {
				apiBodyUrlEncoded := converter.ToAPIHttpBodyUrlEncoded(bodyUrlEncoded)
				allBodyUrlEncodeds = append(allBodyUrlEncodeds, apiBodyUrlEncoded)
			}
		}
	}

	return connect.NewResponse(&apiv1.HttpBodyUrlEncodedCollectionResponse{Items: allBodyUrlEncodeds}), nil
}

func (h *HttpServiceRPC) HttpBodyUrlEncodedInsert(ctx context.Context, req *connect.Request[apiv1.HttpBodyUrlEncodedInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP body URL encoded must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var insertData []struct {
		bodyUrlEncodedModel *mhttp.HTTPBodyUrlencoded
	}

	for _, item := range req.Msg.Items {
		if len(item.HttpBodyUrlEncodedId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_body_url_encoded_id is required"))
		}
		if len(item.HttpId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_id is required"))
		}

		bodyUrlEncodedID, err := idwrap.NewFromBytes(item.HttpBodyUrlEncodedId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		httpID, err := idwrap.NewFromBytes(item.HttpId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Verify the HTTP entry exists and user has access - use pool service
		httpEntry, err := h.httpReader.Get(ctx, httpID)
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

		// Create the body URL encoded model
		bodyUrlEncodedModel := &mhttp.HTTPBodyUrlencoded{
			ID:           bodyUrlEncodedID,
			HttpID:       httpID,
			Key:          item.Key,
			Value:        item.Value,
			Enabled:      item.Enabled,
			Description:  item.Description,
			DisplayOrder: item.Order,
		}

		insertData = append(insertData, struct {
			bodyUrlEncodedModel *mhttp.HTTPBodyUrlencoded
		}{
			bodyUrlEncodedModel: bodyUrlEncodedModel,
		})
	}

	// Step 2: Execute inserts in transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	bodyUrlEncodedWriter := shttp.NewBodyUrlEncodedWriter(tx)
	var createdBodyUrlEncodeds []mhttp.HTTPBodyUrlencoded

	for _, data := range insertData {
		if err := bodyUrlEncodedWriter.Create(ctx, data.bodyUrlEncodedModel); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		createdBodyUrlEncodeds = append(createdBodyUrlEncodeds, *data.bodyUrlEncodedModel)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish create events for real-time sync
	for _, bodyUrlEncoded := range createdBodyUrlEncodeds {
		// Get workspace ID for the HTTP entry
		httpEntry, err := h.httpReader.Get(ctx, bodyUrlEncoded.HttpID)
		if err != nil {
			// Log error but continue - event publishing shouldn't fail the operation
			continue
		}
		h.streamers.HttpBodyUrlEncoded.Publish(HttpBodyUrlEncodedTopic{WorkspaceID: httpEntry.WorkspaceID}, HttpBodyUrlEncodedEvent{
			Type:               eventTypeInsert,
			IsDelta:            bodyUrlEncoded.IsDelta,
			HttpBodyUrlEncoded: converter.ToAPIHttpBodyUrlEncoded(bodyUrlEncoded),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyUrlEncodedUpdate(ctx context.Context, req *connect.Request[apiv1.HttpBodyUrlEncodedUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP body URL encoded must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var updateData []struct {
		existingBodyUrlEncoded *mhttp.HTTPBodyUrlencoded
		item                   *apiv1.HttpBodyUrlEncodedUpdate
	}

	for _, item := range req.Msg.Items {
		if len(item.HttpBodyUrlEncodedId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_body_url_encoded_id is required"))
		}

		bodyUrlEncodedID, err := idwrap.NewFromBytes(item.HttpBodyUrlEncodedId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing body URL encoded - use pool service
		existingBodyUrlEncoded, err := h.httpBodyUrlEncodedService.GetByID(ctx, bodyUrlEncodedID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHttpBodyUrlEncodedFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Verify the HTTP entry exists and user has access - use pool service
		httpEntry, err := h.httpReader.Get(ctx, existingBodyUrlEncoded.HttpID)
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

		updateData = append(updateData, struct {
			existingBodyUrlEncoded *mhttp.HTTPBodyUrlencoded
			item                   *apiv1.HttpBodyUrlEncodedUpdate
		}{
			existingBodyUrlEncoded: existingBodyUrlEncoded,
			item:                   item,
		})
	}

	// Step 2: Prepare updates (in memory)
	for _, data := range updateData {
		item := data.item
		existingBodyUrlEncoded := data.existingBodyUrlEncoded

		if item.Key != nil {
			existingBodyUrlEncoded.Key = *item.Key
		}
		if item.Value != nil {
			existingBodyUrlEncoded.Value = *item.Value
		}
		if item.Enabled != nil {
			existingBodyUrlEncoded.Enabled = *item.Enabled
		}
		if item.Description != nil {
			existingBodyUrlEncoded.Description = *item.Description
		}
		if item.Order != nil {
			existingBodyUrlEncoded.DisplayOrder = *item.Order
		}
	}

	// Step 3: Execute updates in transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	bodyUrlEncodedWriter := shttp.NewBodyUrlEncodedWriter(tx)
	var updatedBodyUrlEncodeds []mhttp.HTTPBodyUrlencoded

	for _, data := range updateData {
		if err := bodyUrlEncodedWriter.Update(ctx, data.existingBodyUrlEncoded); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		updatedBodyUrlEncodeds = append(updatedBodyUrlEncodeds, *data.existingBodyUrlEncoded)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish update events for real-time sync
	for _, bodyUrlEncoded := range updatedBodyUrlEncodeds {
		// Get workspace ID for the HTTP entry
		httpEntry, err := h.httpReader.Get(ctx, bodyUrlEncoded.HttpID)
		if err != nil {
			continue
		}
		h.streamers.HttpBodyUrlEncoded.Publish(HttpBodyUrlEncodedTopic{WorkspaceID: httpEntry.WorkspaceID}, HttpBodyUrlEncodedEvent{
			Type:               eventTypeUpdate,
			IsDelta:            bodyUrlEncoded.IsDelta,
			HttpBodyUrlEncoded: converter.ToAPIHttpBodyUrlEncoded(bodyUrlEncoded),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyUrlEncodedDelete(ctx context.Context, req *connect.Request[apiv1.HttpBodyUrlEncodedDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP body URL encoded must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var deleteData []struct {
		bodyUrlEncodedID       idwrap.IDWrap
		existingBodyUrlEncoded *mhttp.HTTPBodyUrlencoded
		workspaceID            idwrap.IDWrap
	}

	for _, item := range req.Msg.Items {
		if len(item.HttpBodyUrlEncodedId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_body_url_encoded_id is required"))
		}

		bodyUrlEncodedID, err := idwrap.NewFromBytes(item.HttpBodyUrlEncodedId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing body URL encoded - use pool service
		existingBodyUrlEncoded, err := h.httpBodyUrlEncodedService.GetByID(ctx, bodyUrlEncodedID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHttpBodyUrlEncodedFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Verify the HTTP entry exists and user has access - use pool service
		httpEntry, err := h.httpReader.Get(ctx, existingBodyUrlEncoded.HttpID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHTTPFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Check delete access to the workspace
		if err := h.checkWorkspaceDeleteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		deleteData = append(deleteData, struct {
			bodyUrlEncodedID       idwrap.IDWrap
			existingBodyUrlEncoded *mhttp.HTTPBodyUrlencoded
			workspaceID            idwrap.IDWrap
		}{
			bodyUrlEncodedID:       bodyUrlEncodedID,
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

	bodyUrlEncodedWriter := shttp.NewBodyUrlEncodedWriter(tx)
	var deletedBodyUrlEncodeds []mhttp.HTTPBodyUrlencoded
	var deletedWorkspaceIDs []idwrap.IDWrap

	for _, data := range deleteData {
		if err := bodyUrlEncodedWriter.Delete(ctx, data.bodyUrlEncodedID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		deletedBodyUrlEncodeds = append(deletedBodyUrlEncodeds, *data.existingBodyUrlEncoded)
		deletedWorkspaceIDs = append(deletedWorkspaceIDs, data.workspaceID)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish delete events for real-time sync
	for i, bodyUrlEncoded := range deletedBodyUrlEncodeds {
		h.streamers.HttpBodyUrlEncoded.Publish(HttpBodyUrlEncodedTopic{WorkspaceID: deletedWorkspaceIDs[i]}, HttpBodyUrlEncodedEvent{
			Type:               eventTypeDelete,
			IsDelta:            bodyUrlEncoded.IsDelta,
			HttpBodyUrlEncoded: converter.ToAPIHttpBodyUrlEncoded(bodyUrlEncoded),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}
func (h *HttpServiceRPC) HttpBodyRawCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpBodyRawCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Get user's workspaces
	workspaces, err := h.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var allBodies []*apiv1.HttpBodyRaw
	for _, workspace := range workspaces {
		// Get HTTP entries for this workspace
		httpList, err := h.httpReader.GetByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Get body for each HTTP entry
		for _, http := range httpList {
			body, err := h.bodyService.GetByHttpID(ctx, http.ID)
			if err != nil && !errors.Is(err, shttp.ErrNoHttpBodyRawFound) {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			if body != nil {
				allBodies = append(allBodies, converter.ToAPIHttpBodyRawFromMHttp(*body))
			}
		}
	}

	return connect.NewResponse(&apiv1.HttpBodyRawCollectionResponse{
		Items: allBodies,
	}), nil
}

func (h *HttpServiceRPC) HttpBodyRawInsert(ctx context.Context, req *connect.Request[apiv1.HttpBodyRawInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP body raw must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var insertData []struct {
		httpID idwrap.IDWrap
		data   []byte
	}

	for _, item := range req.Msg.Items {
		if len(item.HttpId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_id is required"))
		}

		httpID, err := idwrap.NewFromBytes(item.HttpId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Verify the HTTP entry exists and user has access - use pool service
		httpEntry, err := h.httpReader.Get(ctx, httpID)
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
			httpID idwrap.IDWrap
			data   []byte
		}{
			httpID: httpID,
			data:   []byte(item.Data),
		})
	}

	// Step 2: Execute inserts in transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	bodyRawWriter := shttp.NewBodyRawWriter(tx)

	for _, data := range insertData {
		// Create the body raw using the new service
		_, err = bodyRawWriter.Create(ctx, data.httpID, data.data)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyRawUpdate(ctx context.Context, req *connect.Request[apiv1.HttpBodyRawUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP body raw must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var updateData []struct {
		existingBodyID idwrap.IDWrap
		data           []byte
	}

	for _, item := range req.Msg.Items {
		if len(item.HttpId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_id is required"))
		}

		httpID, err := idwrap.NewFromBytes(item.HttpId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Verify the HTTP entry exists and user has access - use pool service
		httpEntry, err := h.httpReader.Get(ctx, httpID)
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

		// Get existing body raw to get its ID - use pool service
		existingBodyRaw, err := h.bodyService.GetByHttpID(ctx, httpID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHttpBodyRawFound) {
				return nil, connect.NewError(connect.CodeNotFound, errors.New("raw body not found for this HTTP entry"))
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Prepare update data if provided
		if item.Data != nil {
			updateData = append(updateData, struct {
				existingBodyID idwrap.IDWrap
				data           []byte
			}{
				existingBodyID: existingBodyRaw.ID,
				data:           []byte(*item.Data),
			})
		}
	}

	// Step 2: Execute updates in transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	bodyRawWriter := shttp.NewBodyRawWriter(tx)

	for _, data := range updateData {
		// Update using the new service
		_, err := bodyRawWriter.Update(ctx, data.existingBodyID, data.data)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}
