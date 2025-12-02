package rhttp

import (
	"context"
	"encoding/json"
	"errors"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/converter"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"

	"the-dev-tools/server/pkg/service/shttp"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
)

func (h *HttpServiceRPC) HttpSearchParamCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpSearchParamCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Get user's workspaces
	workspaces, err := h.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var allParams []*apiv1.HttpSearchParam
	for _, workspace := range workspaces {
		// Get HTTP entries for this workspace
		httpList, err := h.hs.GetByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Get search params for each HTTP entry
		for _, http := range httpList {
			params, err := h.httpSearchParamService.GetByHttpIDOrdered(ctx, http.ID)
			if err != nil {
				if errors.Is(err, shttp.ErrNoHttpSearchParamFound) {
					continue
				}
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			// Convert to API format
			for _, param := range params {
				apiParam := converter.ToAPIHttpSearchParam(param)
				allParams = append(allParams, apiParam)
			}
		}
	}

	return connect.NewResponse(&apiv1.HttpSearchParamCollectionResponse{Items: allParams}), nil
}

func (h *HttpServiceRPC) HttpSearchParamInsert(ctx context.Context, req *connect.Request[apiv1.HttpSearchParamInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP search param must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var insertData []struct {
		paramModel *mhttp.HTTPSearchParam
	}

	for _, item := range req.Msg.Items {
		if len(item.HttpSearchParamId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_search_param_id is required"))
		}
		if len(item.HttpId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_id is required"))
		}

		paramID, err := idwrap.NewFromBytes(item.HttpSearchParamId)
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

		// Create the param model
		paramModel := &mhttp.HTTPSearchParam{
			ID:          paramID,
			HttpID:      httpID,
			Key:         item.Key,
			Value:       item.Value,
			Enabled:     item.Enabled,
			Description: item.Description,
			Order:       float64(item.Order),
		}

		insertData = append(insertData, struct {
			paramModel *mhttp.HTTPSearchParam
		}{
			paramModel: paramModel,
		})
	}

	// Step 2: Execute inserts in transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	httpSearchParamService := h.httpSearchParamService.TX(tx)
	var createdParams []mhttp.HTTPSearchParam

	for _, data := range insertData {
		if err := httpSearchParamService.Create(ctx, data.paramModel); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		createdParams = append(createdParams, *data.paramModel)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish create events for real-time sync
	for _, param := range createdParams {
		// Get workspace ID for the HTTP entry (we can reuse pool read here as it's after commit)
		httpEntry, err := h.hs.Get(ctx, param.HttpID)
		if err != nil {
			// Log error but continue - event publishing shouldn't fail the operation
			continue
		}
		h.httpSearchParamStream.Publish(HttpSearchParamTopic{WorkspaceID: httpEntry.WorkspaceID}, HttpSearchParamEvent{
			Type:            eventTypeInsert,
			IsDelta:         param.IsDelta,
			HttpSearchParam: converter.ToAPIHttpSearchParam(param),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpSearchParamUpdate(ctx context.Context, req *connect.Request[apiv1.HttpSearchParamUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP search param must be provided"))
	}

	// Step 1: Pre-process and check permissions OUTSIDE transaction
	var updateData []struct {
		paramID       idwrap.IDWrap
		existingParam *mhttp.HTTPSearchParam
		item          *apiv1.HttpSearchParamUpdate
	}

	for _, item := range req.Msg.Items {
		if len(item.HttpSearchParamId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_search_param_id is required"))
		}

		paramID, err := idwrap.NewFromBytes(item.HttpSearchParamId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing param - use pool service
		existingParam, err := h.httpSearchParamService.GetByID(ctx, paramID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHttpSearchParamFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Get the HTTP entry to check workspace access
		httpEntry, err := h.hs.Get(ctx, existingParam.HttpID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Check write access to the workspace
		if err := h.checkWorkspaceWriteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		updateData = append(updateData, struct {
			paramID       idwrap.IDWrap
			existingParam *mhttp.HTTPSearchParam
			item          *apiv1.HttpSearchParamUpdate
		}{
			paramID:       paramID,
			existingParam: existingParam,
			item:          item,
		})
	}

	// Step 2: Prepare updates (in memory)
	for _, data := range updateData {
		item := data.item
		existingParam := data.existingParam

		if item.Key != nil {
			existingParam.Key = *item.Key
		}
		if item.Value != nil {
			existingParam.Value = *item.Value
		}
		if item.Enabled != nil {
			existingParam.Enabled = *item.Enabled
		}
		if item.Description != nil {
			existingParam.Description = *item.Description
		}
		if item.Order != nil {
			existingParam.Order = float64(*item.Order)
		}
	}

	// Step 3: Execute updates in transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	httpSearchParamService := h.httpSearchParamService.TX(tx)
	var updatedParams []mhttp.HTTPSearchParam

	for _, data := range updateData {
		if err := httpSearchParamService.Update(ctx, data.existingParam); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		updatedParams = append(updatedParams, *data.existingParam)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish update events for real-time sync
	for _, param := range updatedParams {
		// Get workspace ID for the HTTP entry
		httpEntry, err := h.hs.Get(ctx, param.HttpID)
		if err != nil {
			continue
		}
		h.httpSearchParamStream.Publish(HttpSearchParamTopic{WorkspaceID: httpEntry.WorkspaceID}, HttpSearchParamEvent{
			Type:            eventTypeUpdate,
			IsDelta:         param.IsDelta,
			HttpSearchParam: converter.ToAPIHttpSearchParam(param),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpSearchParamDelete(ctx context.Context, req *connect.Request[apiv1.HttpSearchParamDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP search param must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var deleteData []struct {
		paramID       idwrap.IDWrap
		existingParam *mhttp.HTTPSearchParam
		workspaceID   idwrap.IDWrap
	}

	for _, item := range req.Msg.Items {
		if len(item.HttpSearchParamId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_search_param_id is required"))
		}

		paramID, err := idwrap.NewFromBytes(item.HttpSearchParamId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing param - use pool service
		existingParam, err := h.httpSearchParamService.GetByID(ctx, paramID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHttpSearchParamFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Get the HTTP entry to check workspace access
		httpEntry, err := h.hs.Get(ctx, existingParam.HttpID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Check delete access to the workspace
		if err := h.checkWorkspaceDeleteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		deleteData = append(deleteData, struct {
			paramID       idwrap.IDWrap
			existingParam *mhttp.HTTPSearchParam
			workspaceID   idwrap.IDWrap
		}{
			paramID:       paramID,
			existingParam: existingParam,
			workspaceID:   httpEntry.WorkspaceID,
		})
	}

	// Step 2: Execute deletes in transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	httpSearchParamService := h.httpSearchParamService.TX(tx)
	var deletedParams []mhttp.HTTPSearchParam
	var deletedWorkspaceIDs []idwrap.IDWrap

	for _, data := range deleteData {
		// Delete the param
		if err := httpSearchParamService.Delete(ctx, data.paramID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		deletedParams = append(deletedParams, *data.existingParam)
		deletedWorkspaceIDs = append(deletedWorkspaceIDs, data.workspaceID)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish delete events for real-time sync
	for i, param := range deletedParams {
		h.httpSearchParamStream.Publish(HttpSearchParamTopic{WorkspaceID: deletedWorkspaceIDs[i]}, HttpSearchParamEvent{
			Type:            eventTypeDelete,
			IsDelta:         param.IsDelta,
			HttpSearchParam: converter.ToAPIHttpSearchParam(param),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpAssertCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpAssertCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Get user's workspaces
	workspaces, err := h.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var allAsserts []*apiv1.HttpAssert
	for _, workspace := range workspaces {
		// Get HTTP entries for this workspace
		httpList, err := h.hs.GetByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Get asserts for each HTTP entry
		for _, http := range httpList {
			asserts, err := h.httpAssertService.GetByHttpID(ctx, http.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			// Convert to API format
			for _, assert := range asserts {
				apiAssert := converter.ToAPIHttpAssert(assert)
				allAsserts = append(allAsserts, apiAssert)
			}
		}
	}

	return connect.NewResponse(&apiv1.HttpAssertCollectionResponse{Items: allAsserts}), nil
}

func (h *HttpServiceRPC) HttpAssertInsert(ctx context.Context, req *connect.Request[apiv1.HttpAssertInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP assert must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var insertData []struct {
		assertModel *mhttp.HTTPAssert
	}

	for _, item := range req.Msg.Items {
		if len(item.HttpAssertId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_assert_id is required"))
		}
		if len(item.HttpId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_id is required"))
		}

		assertID, err := idwrap.NewFromBytes(item.HttpAssertId)
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

		// Create the assert model
		assertModel := &mhttp.HTTPAssert{
			ID:          assertID,
			HttpID:      httpID,
			Value:       item.Value,
			Enabled:     true, // Assertions are always active
			Description: "",   // No description in API
			Order:       0,    // No order in API
		}

		insertData = append(insertData, struct {
			assertModel *mhttp.HTTPAssert
		}{
			assertModel: assertModel,
		})
	}

	// Step 2: Execute inserts in transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	httpAssertService := h.httpAssertService.TX(tx)
	var createdAsserts []mhttp.HTTPAssert

	for _, data := range insertData {
		if err := httpAssertService.Create(ctx, data.assertModel); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		createdAsserts = append(createdAsserts, *data.assertModel)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish create events for real-time sync
	for _, assert := range createdAsserts {
		// Get workspace ID for the HTTP entry
		httpEntry, err := h.hs.Get(ctx, assert.HttpID)
		if err != nil {
			// Log error but continue - event publishing shouldn't fail the operation
			continue
		}
		h.httpAssertStream.Publish(HttpAssertTopic{WorkspaceID: httpEntry.WorkspaceID}, HttpAssertEvent{
			Type:       eventTypeInsert,
			IsDelta:    assert.IsDelta,
			HttpAssert: converter.ToAPIHttpAssert(assert),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpAssertUpdate(ctx context.Context, req *connect.Request[apiv1.HttpAssertUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP assert must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var updateData []struct {
		existingAssert *mhttp.HTTPAssert
		item           *apiv1.HttpAssertUpdate
	}

	for _, item := range req.Msg.Items {
		if len(item.HttpAssertId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_assert_id is required"))
		}

		assertID, err := idwrap.NewFromBytes(item.HttpAssertId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing assert - use pool service
		existingAssert, err := h.httpAssertService.GetByID(ctx, assertID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHttpAssertFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Verify the HTTP entry exists and user has access - use pool service
		httpEntry, err := h.hs.Get(ctx, existingAssert.HttpID)
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
			existingAssert *mhttp.HTTPAssert
			item           *apiv1.HttpAssertUpdate
		}{
			existingAssert: existingAssert,
			item:           item,
		})
	}

	// Step 2: Prepare updates (in memory)
	for _, data := range updateData {
		item := data.item
		existingAssert := data.existingAssert

		if item.Value != nil {
			existingAssert.Value = *item.Value
		}
	}

	// Step 3: Execute updates in transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	httpAssertService := h.httpAssertService.TX(tx)
	var updatedAsserts []mhttp.HTTPAssert

	for _, data := range updateData {
		if err := httpAssertService.Update(ctx, data.existingAssert); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		updatedAsserts = append(updatedAsserts, *data.existingAssert)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish update events for real-time sync
	for _, assert := range updatedAsserts {
		// Get workspace ID for the HTTP entry
		httpEntry, err := h.hs.Get(ctx, assert.HttpID)
		if err != nil {
			// Log error but continue - event publishing shouldn't fail the operation
			continue
		}
		h.httpAssertStream.Publish(HttpAssertTopic{WorkspaceID: httpEntry.WorkspaceID}, HttpAssertEvent{
			Type:       eventTypeUpdate,
			IsDelta:    assert.IsDelta,
			HttpAssert: converter.ToAPIHttpAssert(assert),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpAssertDelete(ctx context.Context, req *connect.Request[apiv1.HttpAssertDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP assert must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var deleteData []struct {
		assertID       idwrap.IDWrap
		existingAssert *mhttp.HTTPAssert
		workspaceID    idwrap.IDWrap
	}

	for _, item := range req.Msg.Items {
		if len(item.HttpAssertId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_assert_id is required"))
		}

		assertID, err := idwrap.NewFromBytes(item.HttpAssertId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing assert - use pool service
		existingAssert, err := h.httpAssertService.GetByID(ctx, assertID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHttpAssertFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Verify the HTTP entry exists and user has access - use pool service
		httpEntry, err := h.hs.Get(ctx, existingAssert.HttpID)
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
			assertID       idwrap.IDWrap
			existingAssert *mhttp.HTTPAssert
			workspaceID    idwrap.IDWrap
		}{
			assertID:       assertID,
			existingAssert: existingAssert,
			workspaceID:    httpEntry.WorkspaceID,
		})
	}

	// Step 2: Execute deletes in transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	httpAssertService := h.httpAssertService.TX(tx)
	var deletedAsserts []mhttp.HTTPAssert
	var deletedWorkspaceIDs []idwrap.IDWrap

	for _, data := range deleteData {
		if err := httpAssertService.Delete(ctx, data.assertID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		deletedAsserts = append(deletedAsserts, *data.existingAssert)
		deletedWorkspaceIDs = append(deletedWorkspaceIDs, data.workspaceID)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish delete events for real-time sync
	for i, assert := range deletedAsserts {
		h.httpAssertStream.Publish(HttpAssertTopic{WorkspaceID: deletedWorkspaceIDs[i]}, HttpAssertEvent{
			Type:       eventTypeDelete,
			IsDelta:    assert.IsDelta,
			HttpAssert: converter.ToAPIHttpAssert(assert),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpResponseCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpResponseCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Get user's workspaces
	workspaces, err := h.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var allResponses []*apiv1.HttpResponse
	for _, workspace := range workspaces {
		// Get base HTTP entries for this workspace
		httpList, err := h.hs.GetByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Also get delta HTTP entries (responses can be stored against delta IDs)
		deltaList, err := h.hs.GetDeltasByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Combine base and delta entries
		allHTTPs := append(httpList, deltaList...)

		// Get responses for each HTTP entry
		for _, http := range allHTTPs {
			responses, err := h.httpResponseService.GetByHttpID(ctx, http.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			for _, response := range responses {
				apiResponse := converter.ToAPIHttpResponse(response)
				allResponses = append(allResponses, apiResponse)
			}
		}
	}

	return connect.NewResponse(&apiv1.HttpResponseCollectionResponse{Items: allResponses}), nil
}

func (h *HttpServiceRPC) HttpResponseHeaderCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpResponseHeaderCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Get user's workspaces
	workspaces, err := h.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var allHeaders []*apiv1.HttpResponseHeader
	for _, workspace := range workspaces {
		// Get base HTTP entries for this workspace
		httpList, err := h.hs.GetByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Also get delta HTTP entries (response headers can be stored against delta IDs)
		deltaList, err := h.hs.GetDeltasByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Combine base and delta entries
		allHTTPs := append(httpList, deltaList...)

		// Get response headers for each HTTP entry
		for _, http := range allHTTPs {
			headers, err := h.httpResponseService.GetHeadersByHttpID(ctx, http.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			for _, header := range headers {
				apiHeader := converter.ToAPIHttpResponseHeader(header)
				allHeaders = append(allHeaders, apiHeader)
			}
		}
	}

	return connect.NewResponse(&apiv1.HttpResponseHeaderCollectionResponse{Items: allHeaders}), nil
}

func (h *HttpServiceRPC) HttpResponseAssertCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpResponseAssertCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Get user's workspaces
	workspaces, err := h.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var allAsserts []*apiv1.HttpResponseAssert
	for _, workspace := range workspaces {
		// Get base HTTP entries for this workspace
		httpList, err := h.hs.GetByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Also get delta HTTP entries (response asserts can be stored against delta IDs)
		deltaList, err := h.hs.GetDeltasByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Combine base and delta entries
		allHTTPs := append(httpList, deltaList...)

		// Get response asserts for each HTTP entry
		for _, http := range allHTTPs {
			asserts, err := h.httpResponseService.GetAssertsByHttpID(ctx, http.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			for _, assert := range asserts {
				apiAssert := converter.ToAPIHttpResponseAssert(assert)
				allAsserts = append(allAsserts, apiAssert)
			}
		}
	}

	return connect.NewResponse(&apiv1.HttpResponseAssertCollectionResponse{Items: allAsserts}), nil
}

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
	defer tx.Rollback()

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
		h.httpHeaderStream.Publish(HttpHeaderTopic{WorkspaceID: insertData[i].workspaceID}, HttpHeaderEvent{
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
	defer tx.Rollback()

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
		h.httpHeaderStream.Publish(HttpHeaderTopic{WorkspaceID: updateData[i].workspaceID}, HttpHeaderEvent{
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
	defer tx.Rollback()

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
		h.httpHeaderStream.Publish(HttpHeaderTopic{WorkspaceID: deleteData[i].workspaceID}, HttpHeaderEvent{
			Type:    eventTypeDelete,
			IsDelta: deleteData[i].isDelta,
			HttpHeader: &apiv1.HttpHeader{
				HttpHeaderId: headerID.Bytes(),
			},
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// HttpHeaderSync handles real-time synchronization for HTTP header entries
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
		httpList, err := h.hs.GetByWorkspaceID(ctx, workspace.ID)
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

		// Create the body form model
		bodyFormModel := &mhttp.HTTPBodyForm{
			ID:          bodyFormID,
			HttpID:      httpID,
			Key:         item.Key,
			Value:       item.Value,
			Enabled:     item.Enabled,
			Description: item.Description,
			Order:       item.Order,
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
	defer tx.Rollback()

	httpBodyFormService := h.httpBodyFormService.TX(tx)

	var createdBodyForms []mhttp.HTTPBodyForm

	for _, data := range insertData {
		if err := httpBodyFormService.Create(ctx, data.bodyFormModel); err != nil {
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
		httpEntry, err := h.hs.Get(ctx, bodyForm.HttpID)
		if err != nil {
			// Log error but continue - event publishing shouldn't fail the operation
			continue
		}
		h.httpBodyFormStream.Publish(HttpBodyFormTopic{WorkspaceID: httpEntry.WorkspaceID}, HttpBodyFormEvent{
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
		httpEntry, err := h.hs.Get(ctx, existingBodyForm.HttpID)
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
			existingBodyForm.Order = *item.Order
		}
	}

	// Step 3: Execute updates in transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	httpBodyFormService := h.httpBodyFormService.TX(tx)
	var updatedBodyForms []mhttp.HTTPBodyForm

	for _, data := range updateData {
		if err := httpBodyFormService.Update(ctx, data.existingBodyForm); err != nil {
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
		httpEntry, err := h.hs.Get(ctx, bodyForm.HttpID)
		if err != nil {
			continue
		}
		h.httpBodyFormStream.Publish(HttpBodyFormTopic{WorkspaceID: httpEntry.WorkspaceID}, HttpBodyFormEvent{
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
		httpEntry, err := h.hs.Get(ctx, existingBodyForm.HttpID)
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
	defer tx.Rollback()

	httpBodyFormService := h.httpBodyFormService.TX(tx)
	var deletedBodyForms []mhttp.HTTPBodyForm
	var deletedWorkspaceIDs []idwrap.IDWrap

	for _, data := range deleteData {
		if err := httpBodyFormService.Delete(ctx, data.bodyFormID); err != nil {
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
		h.httpBodyFormStream.Publish(HttpBodyFormTopic{WorkspaceID: deletedWorkspaceIDs[i]}, HttpBodyFormEvent{
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
		httpList, err := h.hs.GetByWorkspaceID(ctx, workspace.ID)
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

		// Create the body URL encoded model
		bodyUrlEncodedModel := &mhttp.HTTPBodyUrlencoded{
			ID:          bodyUrlEncodedID,
			HttpID:      httpID,
			Key:         item.Key,
			Value:       item.Value,
			Enabled:     item.Enabled,
			Description: item.Description,
			Order:       item.Order,
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
	defer tx.Rollback()

	httpBodyUrlEncodedService := h.httpBodyUrlEncodedService.TX(tx)
	var createdBodyUrlEncodeds []mhttp.HTTPBodyUrlencoded

	for _, data := range insertData {
		if err := httpBodyUrlEncodedService.Create(ctx, data.bodyUrlEncodedModel); err != nil {
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
		httpEntry, err := h.hs.Get(ctx, bodyUrlEncoded.HttpID)
		if err != nil {
			// Log error but continue - event publishing shouldn't fail the operation
			continue
		}
		h.httpBodyUrlEncodedStream.Publish(HttpBodyUrlEncodedTopic{WorkspaceID: httpEntry.WorkspaceID}, HttpBodyUrlEncodedEvent{
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
		httpEntry, err := h.hs.Get(ctx, existingBodyUrlEncoded.HttpID)
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
			existingBodyUrlEncoded.Order = *item.Order
		}
	}

	// Step 3: Execute updates in transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	httpBodyUrlEncodedService := h.httpBodyUrlEncodedService.TX(tx)
	var updatedBodyUrlEncodeds []mhttp.HTTPBodyUrlencoded

	for _, data := range updateData {
		if err := httpBodyUrlEncodedService.Update(ctx, data.existingBodyUrlEncoded); err != nil {
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
		httpEntry, err := h.hs.Get(ctx, bodyUrlEncoded.HttpID)
		if err != nil {
			continue
		}
		h.httpBodyUrlEncodedStream.Publish(HttpBodyUrlEncodedTopic{WorkspaceID: httpEntry.WorkspaceID}, HttpBodyUrlEncodedEvent{
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
		httpEntry, err := h.hs.Get(ctx, existingBodyUrlEncoded.HttpID)
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
	defer tx.Rollback()

	httpBodyUrlEncodedService := h.httpBodyUrlEncodedService.TX(tx)
	var deletedBodyUrlEncodeds []mhttp.HTTPBodyUrlencoded
	var deletedWorkspaceIDs []idwrap.IDWrap

	for _, data := range deleteData {
		if err := httpBodyUrlEncodedService.Delete(ctx, data.bodyUrlEncodedID); err != nil {
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
		h.httpBodyUrlEncodedStream.Publish(HttpBodyUrlEncodedTopic{WorkspaceID: deletedWorkspaceIDs[i]}, HttpBodyUrlEncodedEvent{
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
		httpList, err := h.hs.GetByWorkspaceID(ctx, workspace.ID)
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
		httpID      idwrap.IDWrap
		data        []byte
		contentType string
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

		// Determine content type based on content
		contentType := "text/plain"
		if json.Valid([]byte(item.Data)) {
			contentType = "application/json"
		}

		insertData = append(insertData, struct {
			httpID      idwrap.IDWrap
			data        []byte
			contentType string
		}{
			httpID:      httpID,
			data:        []byte(item.Data),
			contentType: contentType,
		})
	}

	// Step 2: Execute inserts in transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	bodyRawService := h.bodyService.TX(tx)

	for _, data := range insertData {
		// Create the body raw using the new service
		_, err = bodyRawService.Create(ctx, data.httpID, data.data, data.contentType)
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
		contentType    string
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
			// Determine content type based on new content
			contentType := "text/plain"
			if json.Valid([]byte(*item.Data)) {
				contentType = "application/json"
			}

			updateData = append(updateData, struct {
				existingBodyID idwrap.IDWrap
				data           []byte
				contentType    string
			}{
				existingBodyID: existingBodyRaw.ID,
				data:           []byte(*item.Data),
				contentType:    contentType,
			})
		}
	}

	// Step 2: Execute updates in transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	bodyRawService := h.bodyService.TX(tx)

	for _, data := range updateData {
		// Update using the new service
		_, err := bodyRawService.Update(ctx, data.existingBodyID, data.data, data.contentType)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}
