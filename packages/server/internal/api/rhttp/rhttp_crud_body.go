//nolint:revive // exported
package rhttp

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/converter"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/mutation"
	"the-dev-tools/server/pkg/patch"

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

	// FETCH: Gather data and check permissions OUTSIDE transaction
	type insertItem struct {
		bodyFormID  idwrap.IDWrap
		httpID      idwrap.IDWrap
		key         string
		value       string
		enabled     bool
		description string
		order       float32
		workspaceID idwrap.IDWrap
	}
	insertData := make([]insertItem, 0, len(req.Msg.Items))

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

		// CHECK: Validate write access to the workspace
		if err := h.checkWorkspaceWriteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		insertData = append(insertData, insertItem{
			bodyFormID:  bodyFormID,
			httpID:      httpID,
			key:         item.Key,
			value:       item.Value,
			enabled:     item.Enabled,
			description: item.Description,
			order:       item.Order,
			workspaceID: httpEntry.WorkspaceID,
		})
	}

	// ACT: Insert body forms using mutation context with auto-publish
	mut := mutation.New(h.DB, mutation.WithPublisher(h.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	now := time.Now().UnixMilli()
	for _, data := range insertData {
		bodyForm := mhttp.HTTPBodyForm{
			ID:           data.bodyFormID,
			HttpID:       data.httpID,
			Key:          data.key,
			Value:        data.value,
			Enabled:      data.enabled,
			Description:  data.description,
			DisplayOrder: data.order,
		}

		if err := mut.InsertHTTPBodyForm(ctx, mutation.HTTPBodyFormInsertItem{
			ID:          data.bodyFormID,
			HttpID:      data.httpID,
			WorkspaceID: data.workspaceID,
			IsDelta:     false,
			Params: gen.CreateHTTPBodyFormParams{
				ID:           data.bodyFormID,
				HttpID:       data.httpID,
				Key:          data.key,
				Value:        data.value,
				Description:  data.description,
				Enabled:      data.enabled,
				DisplayOrder: float64(data.order),
				IsDelta:      false,
				CreatedAt:    now,
			},
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		mut.Track(mutation.Event{
			Entity:      mutation.EntityHTTPBodyForm,
			Op:          mutation.OpInsert,
			ID:          data.bodyFormID,
			ParentID:    data.httpID,
			WorkspaceID: data.workspaceID,
			Payload:     bodyForm,
		})
	}

	if err := mut.Commit(ctx); err != nil { // Auto-publishes events!
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyFormDataUpdate(ctx context.Context, req *connect.Request[apiv1.HttpBodyFormDataUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP body form must be provided"))
	}

	// FETCH: Gather data and check permissions OUTSIDE transaction
	type updateItem struct {
		existingBodyForm mhttp.HTTPBodyForm
		key              *string
		value            *string
		enabled          *bool
		description      *string
		order            *float32
		workspaceID      idwrap.IDWrap
	}
	updateData := make([]updateItem, 0, len(req.Msg.Items))

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

		// CHECK: Validate write access to the workspace
		if err := h.checkWorkspaceWriteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		updateData = append(updateData, updateItem{
			existingBodyForm: *existingBodyForm,
			key:              item.Key,
			value:            item.Value,
			enabled:          item.Enabled,
			description:      item.Description,
			order:            item.Order,
			workspaceID:      httpEntry.WorkspaceID,
		})
	}

	// ACT: Update body forms using mutation context with auto-publish
	mut := mutation.New(h.DB, mutation.WithPublisher(h.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	for _, data := range updateData {
		bodyForm := data.existingBodyForm

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

		if err := mut.UpdateHTTPBodyForm(ctx, mutation.HTTPBodyFormUpdateItem{
			ID:          bodyForm.ID,
			HttpID:      bodyForm.HttpID,
			WorkspaceID: data.workspaceID,
			IsDelta:     bodyForm.IsDelta,
			Params: gen.UpdateHTTPBodyFormParams{
				ID:           bodyForm.ID,
				Key:          bodyForm.Key,
				Value:        bodyForm.Value,
				Description:  bodyForm.Description,
				Enabled:      bodyForm.Enabled,
				DisplayOrder: float64(bodyForm.DisplayOrder),
			},
			Patch: bodyFormPatch,
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		mut.Track(mutation.Event{
			Entity:      mutation.EntityHTTPBodyForm,
			Op:          mutation.OpUpdate,
			ID:          bodyForm.ID,
			ParentID:    bodyForm.HttpID,
			WorkspaceID: data.workspaceID,
			Payload:     bodyForm,
			Patch:       bodyFormPatch,
		})
	}

	if err := mut.Commit(ctx); err != nil { // Auto-publishes events!
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyFormDataDelete(ctx context.Context, req *connect.Request[apiv1.HttpBodyFormDataDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP body form must be provided"))
	}

	// FETCH: Gather data and check permissions OUTSIDE transaction
	type deleteItem struct {
		ID          idwrap.IDWrap
		HttpID      idwrap.IDWrap
		WorkspaceID idwrap.IDWrap
		IsDelta     bool
	}
	deleteItems := make([]deleteItem, 0, len(req.Msg.Items))

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

		// CHECK: Validate delete access to the workspace
		if err := h.checkWorkspaceDeleteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		deleteItems = append(deleteItems, deleteItem{
			ID:          bodyFormID,
			HttpID:      existingBodyForm.HttpID,
			WorkspaceID: httpEntry.WorkspaceID,
			IsDelta:     existingBodyForm.IsDelta,
		})
	}

	// ACT: Delete using mutation context with auto-publish
	mut := mutation.New(h.DB, mutation.WithPublisher(h.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	for _, item := range deleteItems {
		mut.Track(mutation.Event{
			Entity:      mutation.EntityHTTPBodyForm,
			Op:          mutation.OpDelete,
			ID:          item.ID,
			ParentID:    item.HttpID,
			WorkspaceID: item.WorkspaceID,
			IsDelta:     item.IsDelta,
		})
		if err := mut.Queries().DeleteHTTPBodyForm(ctx, item.ID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := mut.Commit(ctx); err != nil { // Auto-publishes events!
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

	// FETCH: Gather data and check permissions OUTSIDE transaction
	type insertItem struct {
		bodyUrlEncodedID idwrap.IDWrap
		httpID           idwrap.IDWrap
		key              string
		value            string
		enabled          bool
		description      string
		order            float32
		workspaceID      idwrap.IDWrap
	}
	insertData := make([]insertItem, 0, len(req.Msg.Items))

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

		// CHECK: Validate write access to the workspace
		if err := h.checkWorkspaceWriteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		insertData = append(insertData, insertItem{
			bodyUrlEncodedID: bodyUrlEncodedID,
			httpID:           httpID,
			key:              item.Key,
			value:            item.Value,
			enabled:          item.Enabled,
			description:      item.Description,
			order:            item.Order,
			workspaceID:      httpEntry.WorkspaceID,
		})
	}

	// ACT: Insert body URL encoded using mutation context with auto-publish
	mut := mutation.New(h.DB, mutation.WithPublisher(h.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	now := time.Now().UnixMilli()
	for _, data := range insertData {
		bodyUrlEnc := mhttp.HTTPBodyUrlencoded{
			ID:           data.bodyUrlEncodedID,
			HttpID:       data.httpID,
			Key:          data.key,
			Value:        data.value,
			Enabled:      data.enabled,
			Description:  data.description,
			DisplayOrder: data.order,
		}

		if err := mut.InsertHTTPBodyUrlEncoded(ctx, mutation.HTTPBodyUrlEncodedInsertItem{
			ID:          data.bodyUrlEncodedID,
			HttpID:      data.httpID,
			WorkspaceID: data.workspaceID,
			IsDelta:     false,
			Params: gen.CreateHTTPBodyUrlEncodedParams{
				ID:           data.bodyUrlEncodedID,
				HttpID:       data.httpID,
				Key:          data.key,
				Value:        data.value,
				Description:  data.description,
				Enabled:      data.enabled,
				DisplayOrder: float64(data.order),
				IsDelta:      false,
				CreatedAt:    now,
			},
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		mut.Track(mutation.Event{
			Entity:      mutation.EntityHTTPBodyURL,
			Op:          mutation.OpInsert,
			ID:          data.bodyUrlEncodedID,
			ParentID:    data.httpID,
			WorkspaceID: data.workspaceID,
			Payload:     bodyUrlEnc,
		})
	}

	if err := mut.Commit(ctx); err != nil { // Auto-publishes events!
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyUrlEncodedUpdate(ctx context.Context, req *connect.Request[apiv1.HttpBodyUrlEncodedUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP body URL encoded must be provided"))
	}

	// FETCH: Gather data and check permissions OUTSIDE transaction
	type updateItem struct {
		existingBodyUrlEncoded mhttp.HTTPBodyUrlencoded
		key                    *string
		value                  *string
		enabled                *bool
		description            *string
		order                  *float32
		workspaceID            idwrap.IDWrap
	}
	updateData := make([]updateItem, 0, len(req.Msg.Items))

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

		// CHECK: Validate write access to the workspace
		if err := h.checkWorkspaceWriteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		updateData = append(updateData, updateItem{
			existingBodyUrlEncoded: *existingBodyUrlEncoded,
			key:                    item.Key,
			value:                  item.Value,
			enabled:                item.Enabled,
			description:            item.Description,
			order:                  item.Order,
			workspaceID:            httpEntry.WorkspaceID,
		})
	}

	// ACT: Update body URL encoded using mutation context with auto-publish
	mut := mutation.New(h.DB, mutation.WithPublisher(h.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	for _, data := range updateData {
		bodyUrlEncoded := data.existingBodyUrlEncoded

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

		if err := mut.UpdateHTTPBodyUrlEncoded(ctx, mutation.HTTPBodyUrlEncodedUpdateItem{
			ID:          bodyUrlEncoded.ID,
			HttpID:      bodyUrlEncoded.HttpID,
			WorkspaceID: data.workspaceID,
			IsDelta:     bodyUrlEncoded.IsDelta,
			Params: gen.UpdateHTTPBodyUrlEncodedParams{
				ID:           bodyUrlEncoded.ID,
				Key:          bodyUrlEncoded.Key,
				Value:        bodyUrlEncoded.Value,
				Description:  bodyUrlEncoded.Description,
				Enabled:      bodyUrlEncoded.Enabled,
				DisplayOrder: float64(bodyUrlEncoded.DisplayOrder),
			},
			Patch: bodyUrlEncodedPatch,
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		mut.Track(mutation.Event{
			Entity:      mutation.EntityHTTPBodyURL,
			Op:          mutation.OpUpdate,
			ID:          bodyUrlEncoded.ID,
			ParentID:    bodyUrlEncoded.HttpID,
			WorkspaceID: data.workspaceID,
			Payload:     bodyUrlEncoded,
			Patch:       bodyUrlEncodedPatch,
		})
	}

	if err := mut.Commit(ctx); err != nil { // Auto-publishes events!
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyUrlEncodedDelete(ctx context.Context, req *connect.Request[apiv1.HttpBodyUrlEncodedDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP body URL encoded must be provided"))
	}

	// FETCH: Gather data and check permissions OUTSIDE transaction
	type deleteItem struct {
		ID          idwrap.IDWrap
		HttpID      idwrap.IDWrap
		WorkspaceID idwrap.IDWrap
		IsDelta     bool
	}
	deleteItems := make([]deleteItem, 0, len(req.Msg.Items))

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

		// CHECK: Validate delete access to the workspace
		if err := h.checkWorkspaceDeleteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		deleteItems = append(deleteItems, deleteItem{
			ID:          bodyUrlEncodedID,
			HttpID:      existingBodyUrlEncoded.HttpID,
			WorkspaceID: httpEntry.WorkspaceID,
			IsDelta:     existingBodyUrlEncoded.IsDelta,
		})
	}

	// ACT: Delete using mutation context with auto-publish
	mut := mutation.New(h.DB, mutation.WithPublisher(h.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	for _, item := range deleteItems {
		mut.Track(mutation.Event{
			Entity:      mutation.EntityHTTPBodyURL,
			Op:          mutation.OpDelete,
			ID:          item.ID,
			ParentID:    item.HttpID,
			WorkspaceID: item.WorkspaceID,
			IsDelta:     item.IsDelta,
		})
		if err := mut.Queries().DeleteHTTPBodyUrlEncoded(ctx, item.ID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := mut.Commit(ctx); err != nil { // Auto-publishes events!
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

	// FETCH: Gather data and check permissions OUTSIDE transaction
	type insertItem struct {
		httpID      idwrap.IDWrap
		data        []byte
		workspaceID idwrap.IDWrap
	}
	insertData := make([]insertItem, 0, len(req.Msg.Items))

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

		// CHECK: Validate write access to the workspace
		if err := h.checkWorkspaceWriteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		insertData = append(insertData, insertItem{
			httpID:      httpID,
			data:        []byte(item.Data),
			workspaceID: httpEntry.WorkspaceID,
		})
	}

	// ACT: Insert body raw using mutation context with auto-publish
	mut := mutation.New(h.DB, mutation.WithPublisher(h.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	now := time.Now().UnixMilli()
	for _, data := range insertData {
		bodyRawID := idwrap.NewNow()
		bodyRaw := mhttp.HTTPBodyRaw{
			ID:      bodyRawID,
			HttpID:  data.httpID,
			RawData: data.data,
		}

		if err := mut.InsertHTTPBodyRaw(ctx, mutation.HTTPBodyRawInsertItem{
			ID:          bodyRawID,
			HttpID:      data.httpID,
			WorkspaceID: data.workspaceID,
			IsDelta:     false,
			Params: gen.CreateHTTPBodyRawParams{
				ID:        bodyRawID,
				HttpID:    data.httpID,
				RawData:   data.data,
				IsDelta:   false,
				CreatedAt: now,
				UpdatedAt: now,
			},
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		mut.Track(mutation.Event{
			Entity:      mutation.EntityHTTPBodyRaw,
			Op:          mutation.OpInsert,
			ID:          bodyRawID,
			ParentID:    data.httpID,
			WorkspaceID: data.workspaceID,
			Payload:     bodyRaw,
		})
	}

	if err := mut.Commit(ctx); err != nil { // Auto-publishes events!
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyRawUpdate(ctx context.Context, req *connect.Request[apiv1.HttpBodyRawUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP body raw must be provided"))
	}

	// FETCH: Gather data and check permissions OUTSIDE transaction
	type updateItem struct {
		existingBodyRaw mhttp.HTTPBodyRaw
		data            *string
		workspaceID     idwrap.IDWrap
	}
	updateData := make([]updateItem, 0, len(req.Msg.Items))

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

		// CHECK: Validate write access to the workspace
		if err := h.checkWorkspaceWriteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		// Get existing body raw - use pool service
		existingBodyRaw, err := h.bodyService.GetByHttpID(ctx, httpID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHttpBodyRawFound) {
				return nil, connect.NewError(connect.CodeNotFound, errors.New("raw body not found for this HTTP entry"))
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		updateData = append(updateData, updateItem{
			existingBodyRaw: *existingBodyRaw,
			data:            item.Data,
			workspaceID:     httpEntry.WorkspaceID,
		})
	}

	// ACT: Update body raw using mutation context with auto-publish
	mut := mutation.New(h.DB, mutation.WithPublisher(h.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	now := time.Now().UnixMilli()
	for _, data := range updateData {
		bodyRaw := data.existingBodyRaw

		// Build patch with only changed fields
		bodyRawPatch := patch.HTTPBodyRawPatch{}

		// Update data if provided and track in patch
		if data.data != nil {
			bodyRaw.RawData = []byte(*data.data)
			bodyRawPatch.Data = patch.NewOptional(*data.data)
		}

		if err := mut.UpdateHTTPBodyRaw(ctx, mutation.HTTPBodyRawUpdateItem{
			ID:          bodyRaw.ID,
			HttpID:      bodyRaw.HttpID,
			WorkspaceID: data.workspaceID,
			IsDelta:     bodyRaw.IsDelta,
			Params: gen.UpdateHTTPBodyRawParams{
				RawData:   bodyRaw.RawData,
				UpdatedAt: now,
				ID:        bodyRaw.ID,
			},
			Patch: bodyRawPatch,
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		mut.Track(mutation.Event{
			Entity:      mutation.EntityHTTPBodyRaw,
			Op:          mutation.OpUpdate,
			ID:          bodyRaw.ID,
			ParentID:    bodyRaw.HttpID,
			WorkspaceID: data.workspaceID,
			Payload:     bodyRaw,
			Patch:       bodyRawPatch,
		})
	}

	if err := mut.Commit(ctx); err != nil { // Auto-publishes events!
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}