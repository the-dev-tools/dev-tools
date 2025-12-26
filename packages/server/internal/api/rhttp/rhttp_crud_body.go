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

// bodyFormWithWorkspace is a context carrier that pairs a body form with its workspace ID.
type bodyFormWithWorkspace struct {
	bodyForm    mhttp.HTTPBodyForm
	workspaceID idwrap.IDWrap
}

// publishBulkBodyFormInsert publishes multiple body form insert events in bulk.
func (h *HttpServiceRPC) publishBulkBodyFormInsert(
	topic HttpBodyFormTopic,
	items []bodyFormWithWorkspace,
) {
	events := make([]HttpBodyFormEvent, len(items))
	for i, item := range items {
		events[i] = HttpBodyFormEvent{
			Type:         eventTypeInsert,
			IsDelta:      item.bodyForm.IsDelta,
			HttpBodyForm: converter.ToAPIHttpBodyFormData(item.bodyForm),
		}
	}
	h.streamers.HttpBodyForm.Publish(topic, events...)
}

// publishBulkBodyFormUpdate publishes multiple body form update events in bulk.
func (h *HttpServiceRPC) publishBulkBodyFormUpdate(
	topic HttpBodyFormTopic,
	events []txutil.UpdateEvent[bodyFormWithWorkspace, patch.HTTPBodyFormPatch],
) {
	bodyFormEvents := make([]HttpBodyFormEvent, len(events))
	for i, evt := range events {
		bodyFormEvents[i] = HttpBodyFormEvent{
			Type:         eventTypeUpdate,
			IsDelta:      evt.Item.bodyForm.IsDelta,
			HttpBodyForm: converter.ToAPIHttpBodyFormData(evt.Item.bodyForm),
			Patch:        evt.Patch,
		}
	}
	h.streamers.HttpBodyForm.Publish(topic, bodyFormEvents...)
}

// publishBulkBodyFormDelete publishes multiple body form delete events in bulk.
func (h *HttpServiceRPC) publishBulkBodyFormDelete(
	topic HttpBodyFormTopic,
	events []txutil.DeleteEvent[idwrap.IDWrap],
) {
	bodyFormEvents := make([]HttpBodyFormEvent, len(events))
	for i, evt := range events {
		bodyFormEvents[i] = HttpBodyFormEvent{
			Type:    eventTypeDelete,
			IsDelta: evt.IsDelta,
			HttpBodyForm: &apiv1.HttpBodyFormData{
				HttpBodyFormDataId: evt.ID.Bytes(),
			},
		}
	}
	h.streamers.HttpBodyForm.Publish(topic, bodyFormEvents...)
}

// bodyUrlEncodedWithWorkspace is a context carrier that pairs a body URL encoded with its workspace ID.
type bodyUrlEncodedWithWorkspace struct {
	bodyUrlEncoded mhttp.HTTPBodyUrlencoded
	workspaceID    idwrap.IDWrap
}

// publishBulkBodyUrlEncodedInsert publishes multiple body URL encoded insert events in bulk.
func (h *HttpServiceRPC) publishBulkBodyUrlEncodedInsert(
	topic HttpBodyUrlEncodedTopic,
	items []bodyUrlEncodedWithWorkspace,
) {
	events := make([]HttpBodyUrlEncodedEvent, len(items))
	for i, item := range items {
		events[i] = HttpBodyUrlEncodedEvent{
			Type:               eventTypeInsert,
			IsDelta:            item.bodyUrlEncoded.IsDelta,
			HttpBodyUrlEncoded: converter.ToAPIHttpBodyUrlEncoded(item.bodyUrlEncoded),
		}
	}
	h.streamers.HttpBodyUrlEncoded.Publish(topic, events...)
}

// publishBulkBodyUrlEncodedUpdate publishes multiple body URL encoded update events in bulk.
func (h *HttpServiceRPC) publishBulkBodyUrlEncodedUpdate(
	topic HttpBodyUrlEncodedTopic,
	events []txutil.UpdateEvent[bodyUrlEncodedWithWorkspace, patch.HTTPBodyUrlEncodedPatch],
) {
	bodyUrlEncodedEvents := make([]HttpBodyUrlEncodedEvent, len(events))
	for i, evt := range events {
		bodyUrlEncodedEvents[i] = HttpBodyUrlEncodedEvent{
			Type:               eventTypeUpdate,
			IsDelta:            evt.Item.bodyUrlEncoded.IsDelta,
			HttpBodyUrlEncoded: converter.ToAPIHttpBodyUrlEncoded(evt.Item.bodyUrlEncoded),
			Patch:              evt.Patch,
		}
	}
	h.streamers.HttpBodyUrlEncoded.Publish(topic, bodyUrlEncodedEvents...)
}

// publishBulkBodyUrlEncodedDelete publishes multiple body URL encoded delete events in bulk.
func (h *HttpServiceRPC) publishBulkBodyUrlEncodedDelete(
	topic HttpBodyUrlEncodedTopic,
	events []txutil.DeleteEvent[idwrap.IDWrap],
) {
	bodyUrlEncodedEvents := make([]HttpBodyUrlEncodedEvent, len(events))
	for i, evt := range events {
		bodyUrlEncodedEvents[i] = HttpBodyUrlEncodedEvent{
			Type:    eventTypeDelete,
			IsDelta: evt.IsDelta,
			HttpBodyUrlEncoded: &apiv1.HttpBodyUrlEncoded{
				HttpBodyUrlEncodedId: evt.ID.Bytes(),
			},
		}
	}
	h.streamers.HttpBodyUrlEncoded.Publish(topic, bodyUrlEncodedEvents...)
}

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
		workspaceID   idwrap.IDWrap
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
			workspaceID   idwrap.IDWrap
		}{
			bodyFormModel: bodyFormModel,
			workspaceID:   httpEntry.WorkspaceID,
		})
	}

	// Step 2: Minimal write transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	// Create bulk sync wrapper with topic extractor
	syncTx := txutil.NewBulkInsertTx[bodyFormWithWorkspace, HttpBodyFormTopic](
		tx,
		func(bfw bodyFormWithWorkspace) HttpBodyFormTopic {
			return HttpBodyFormTopic{WorkspaceID: bfw.workspaceID}
		},
	)

	bodyFormWriter := shttp.NewBodyFormWriter(tx)

	for _, data := range insertData {
		if err := bodyFormWriter.Create(ctx, data.bodyFormModel); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Track with workspace context for bulk sync
		syncTx.Track(bodyFormWithWorkspace{
			bodyForm:    *data.bodyFormModel,
			workspaceID: data.workspaceID,
		})
	}

	// Step 3: Commit and bulk publish sync events (grouped by topic)
	if err := syncTx.CommitAndPublish(ctx, h.publishBulkBodyFormInsert); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
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
		key              *string
		value            *string
		enabled          *bool
		description      *string
		order            *float32
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

		// Check write access to the workspace
		if err := h.checkWorkspaceWriteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		updateData = append(updateData, struct {
			existingBodyForm *mhttp.HTTPBodyForm
			key              *string
			value            *string
			enabled          *bool
			description      *string
			order            *float32
			workspaceID      idwrap.IDWrap
		}{
			existingBodyForm: existingBodyForm,
			key:              item.Key,
			value:            item.Value,
			enabled:          item.Enabled,
			description:      item.Description,
			order:            item.Order,
			workspaceID:      httpEntry.WorkspaceID,
		})
	}

	// Step 2: Minimal write transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	// Create bulk sync wrapper with topic extractor
	syncTx := txutil.NewBulkUpdateTx[bodyFormWithWorkspace, patch.HTTPBodyFormPatch, HttpBodyFormTopic](
		tx,
		func(bfw bodyFormWithWorkspace) HttpBodyFormTopic {
			return HttpBodyFormTopic{WorkspaceID: bfw.workspaceID}
		},
	)

	bodyFormWriter := shttp.NewBodyFormWriter(tx)

	for _, data := range updateData {
		bodyForm := *data.existingBodyForm

		// Build patch with only changed fields
		bodyFormPatch := patch.HTTPBodyFormPatch{}

		// Update fields if provided and track in patch
		if data.key != nil {
			bodyForm.Key = *data.key
			bodyFormPatch.Key = patch.NewOptional(*data.key)
		}
		if data.value != nil {
			bodyForm.Value = *data.value
			bodyFormPatch.Value = patch.NewOptional(*data.value)
		}
		if data.enabled != nil {
			bodyForm.Enabled = *data.enabled
			bodyFormPatch.Enabled = patch.NewOptional(*data.enabled)
		}
		if data.description != nil {
			bodyForm.Description = *data.description
			bodyFormPatch.Description = patch.NewOptional(*data.description)
		}
		if data.order != nil {
			bodyForm.DisplayOrder = *data.order
			bodyFormPatch.Order = patch.NewOptional(*data.order)
		}

		if err := bodyFormWriter.Update(ctx, &bodyForm); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Track with workspace context and patch
		syncTx.Track(
			bodyFormWithWorkspace{
				bodyForm:    bodyForm,
				workspaceID: data.workspaceID,
			},
			bodyFormPatch,
		)
	}

	// Step 3: Commit and bulk publish (grouped by workspace)
	if err := syncTx.CommitAndPublish(ctx, h.publishBulkBodyFormUpdate); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
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

	// Step 2: Minimal write transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	// Create bulk sync wrapper with topic extractor
	syncTx := txutil.NewBulkDeleteTx[idwrap.IDWrap, HttpBodyFormTopic](
		tx,
		func(evt txutil.DeleteEvent[idwrap.IDWrap]) HttpBodyFormTopic {
			return HttpBodyFormTopic{WorkspaceID: evt.WorkspaceID}
		},
	)

	bodyFormWriter := shttp.NewBodyFormWriter(tx)

	for _, data := range deleteData {
		if err := bodyFormWriter.Delete(ctx, data.bodyFormID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		syncTx.Track(data.bodyFormID, data.workspaceID, data.existingBodyForm.IsDelta)
	}

	// Step 3: Commit and bulk publish (grouped by workspace)
	if err := syncTx.CommitAndPublish(ctx, h.publishBulkBodyFormDelete); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
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
		workspaceID         idwrap.IDWrap
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
			workspaceID         idwrap.IDWrap
		}{
			bodyUrlEncodedModel: bodyUrlEncodedModel,
			workspaceID:         httpEntry.WorkspaceID,
		})
	}

	// Step 2: Minimal write transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	// Create bulk sync wrapper with topic extractor
	syncTx := txutil.NewBulkInsertTx[bodyUrlEncodedWithWorkspace, HttpBodyUrlEncodedTopic](
		tx,
		func(buw bodyUrlEncodedWithWorkspace) HttpBodyUrlEncodedTopic {
			return HttpBodyUrlEncodedTopic{WorkspaceID: buw.workspaceID}
		},
	)

	bodyUrlEncodedWriter := shttp.NewBodyUrlEncodedWriter(tx)

	for _, data := range insertData {
		if err := bodyUrlEncodedWriter.Create(ctx, data.bodyUrlEncodedModel); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Track with workspace context for bulk sync
		syncTx.Track(bodyUrlEncodedWithWorkspace{
			bodyUrlEncoded: *data.bodyUrlEncodedModel,
			workspaceID:    data.workspaceID,
		})
	}

	// Step 3: Commit and bulk publish sync events (grouped by topic)
	if err := syncTx.CommitAndPublish(ctx, h.publishBulkBodyUrlEncodedInsert); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
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
		key                    *string
		value                  *string
		enabled                *bool
		description            *string
		order                  *float32
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

		// Check write access to the workspace
		if err := h.checkWorkspaceWriteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		updateData = append(updateData, struct {
			existingBodyUrlEncoded *mhttp.HTTPBodyUrlencoded
			key                    *string
			value                  *string
			enabled                *bool
			description            *string
			order                  *float32
			workspaceID            idwrap.IDWrap
		}{
			existingBodyUrlEncoded: existingBodyUrlEncoded,
			key:                    item.Key,
			value:                  item.Value,
			enabled:                item.Enabled,
			description:            item.Description,
			order:                  item.Order,
			workspaceID:            httpEntry.WorkspaceID,
		})
	}

	// Step 2: Minimal write transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	// Create bulk sync wrapper with topic extractor
	syncTx := txutil.NewBulkUpdateTx[bodyUrlEncodedWithWorkspace, patch.HTTPBodyUrlEncodedPatch, HttpBodyUrlEncodedTopic](
		tx,
		func(buw bodyUrlEncodedWithWorkspace) HttpBodyUrlEncodedTopic {
			return HttpBodyUrlEncodedTopic{WorkspaceID: buw.workspaceID}
		},
	)

	bodyUrlEncodedWriter := shttp.NewBodyUrlEncodedWriter(tx)

	for _, data := range updateData {
		bodyUrlEncoded := *data.existingBodyUrlEncoded

		// Build patch with only changed fields
		bodyUrlEncodedPatch := patch.HTTPBodyUrlEncodedPatch{}

		// Update fields if provided and track in patch
		if data.key != nil {
			bodyUrlEncoded.Key = *data.key
			bodyUrlEncodedPatch.Key = patch.NewOptional(*data.key)
		}
		if data.value != nil {
			bodyUrlEncoded.Value = *data.value
			bodyUrlEncodedPatch.Value = patch.NewOptional(*data.value)
		}
		if data.enabled != nil {
			bodyUrlEncoded.Enabled = *data.enabled
			bodyUrlEncodedPatch.Enabled = patch.NewOptional(*data.enabled)
		}
		if data.description != nil {
			bodyUrlEncoded.Description = *data.description
			bodyUrlEncodedPatch.Description = patch.NewOptional(*data.description)
		}
		if data.order != nil {
			bodyUrlEncoded.DisplayOrder = *data.order
			bodyUrlEncodedPatch.Order = patch.NewOptional(*data.order)
		}

		if err := bodyUrlEncodedWriter.Update(ctx, &bodyUrlEncoded); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Track with workspace context and patch
		syncTx.Track(
			bodyUrlEncodedWithWorkspace{
				bodyUrlEncoded: bodyUrlEncoded,
				workspaceID:    data.workspaceID,
			},
			bodyUrlEncodedPatch,
		)
	}

	// Step 3: Commit and bulk publish (grouped by workspace)
	if err := syncTx.CommitAndPublish(ctx, h.publishBulkBodyUrlEncodedUpdate); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
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

	// Step 2: Minimal write transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	// Create bulk sync wrapper with topic extractor
	syncTx := txutil.NewBulkDeleteTx[idwrap.IDWrap, HttpBodyUrlEncodedTopic](
		tx,
		func(evt txutil.DeleteEvent[idwrap.IDWrap]) HttpBodyUrlEncodedTopic {
			return HttpBodyUrlEncodedTopic{WorkspaceID: evt.WorkspaceID}
		},
	)

	bodyUrlEncodedWriter := shttp.NewBodyUrlEncodedWriter(tx)

	for _, data := range deleteData {
		if err := bodyUrlEncodedWriter.Delete(ctx, data.bodyUrlEncodedID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		syncTx.Track(data.bodyUrlEncodedID, data.workspaceID, data.existingBodyUrlEncoded.IsDelta)
	}

	// Step 3: Commit and bulk publish (grouped by workspace)
	if err := syncTx.CommitAndPublish(ctx, h.publishBulkBodyUrlEncodedDelete); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
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
