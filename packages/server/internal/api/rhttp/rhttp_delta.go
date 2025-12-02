package rhttp

import (
	"context"
	"errors"
	"sync"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/converter"
	"the-dev-tools/server/pkg/eventstream"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mhttpassert"

	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/shttpassert"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
)

func (h *HttpServiceRPC) HttpDeltaCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpDeltaCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Get user's workspaces
	workspaces, err := h.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var allDeltas []*apiv1.HttpDelta
	for _, workspace := range workspaces {
		// Get HTTP entries for this workspace
		httpList, err := h.hs.GetDeltasByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Convert to delta format
		for _, http := range httpList {
			delta := &apiv1.HttpDelta{
				DeltaHttpId: http.ID.Bytes(),
			}

			if http.ParentHttpID != nil {
				delta.HttpId = http.ParentHttpID.Bytes()
			}

			// Only include delta fields if they exist
			if http.DeltaName != nil {
				delta.Name = http.DeltaName
			}
			if http.DeltaMethod != nil {
				method := parseHttpMethod(*http.DeltaMethod)
				delta.Method = &method
			}
			if http.DeltaUrl != nil {
				delta.Url = http.DeltaUrl
			}

			allDeltas = append(allDeltas, delta)
		}
	}

	return connect.NewResponse(&apiv1.HttpDeltaCollectionResponse{
		Items: allDeltas,
	}), nil
}

func (h *HttpServiceRPC) HttpDeltaInsert(ctx context.Context, req *connect.Request[apiv1.HttpDeltaInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.Items) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one delta item is required"))
	}

	// Process each delta item
	for _, item := range req.Msg.Items {
		if len(item.HttpId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_id is required for each delta item"))
		}

		httpID, err := idwrap.NewFromBytes(item.HttpId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Check workspace write access
		httpEntry, err := h.hs.Get(ctx, httpID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHTTPFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := h.checkWorkspaceWriteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		var deltaID idwrap.IDWrap
		if len(item.DeltaHttpId) > 0 {
			var err error
			deltaID, err = idwrap.NewFromBytes(item.DeltaHttpId)
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, err)
			}
		} else {
			deltaID = idwrap.NewNow()
		}

		// Create delta HTTP entry
		deltaHttp := &mhttp.HTTP{
			ID:           deltaID,
			WorkspaceID:  httpEntry.WorkspaceID,
			FolderID:     httpEntry.FolderID,
			Name:         httpEntry.Name,
			Url:          httpEntry.Url,
			Method:       httpEntry.Method,
			Description:  httpEntry.Description,
			ParentHttpID: &httpID,
			IsDelta:      true,
			DeltaName:    item.Name,
			DeltaUrl:     item.Url,
			DeltaMethod: func() *string {
				if item.Method != nil {
					methodStr := converter.FromAPIHttpMethod(*item.Method)
					return &methodStr
				}
				return nil
			}(),
			CreatedAt: 0, // Will be set by service
			UpdatedAt: 0, // Will be set by service
		}

		// Create in database
		err = h.hs.Create(ctx, deltaHttp)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		h.publishInsertEvent(*deltaHttp)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpDeltaUpdate(ctx context.Context, req *connect.Request[apiv1.HttpDeltaUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP delta must be provided"))
	}

	// Step 1: Pre-process and check permissions OUTSIDE transaction
	var updateData []struct {
		deltaID       idwrap.IDWrap
		existingDelta *mhttp.HTTP
		item          *apiv1.HttpDeltaUpdate
	}

	for _, item := range req.Msg.Items {
		if len(item.DeltaHttpId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("delta_http_id is required"))
		}

		deltaID, err := idwrap.NewFromBytes(item.DeltaHttpId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing delta HTTP entry
		existingDelta, err := h.hs.Get(ctx, deltaID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHTTPFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Verify this is actually a delta record
		if !existingDelta.IsDelta {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("specified HTTP entry is not a delta"))
		}

		// Check write access to the workspace
		if err := h.checkWorkspaceWriteAccess(ctx, existingDelta.WorkspaceID); err != nil {
			return nil, err
		}

		updateData = append(updateData, struct {
			deltaID       idwrap.IDWrap
			existingDelta *mhttp.HTTP
			item          *apiv1.HttpDeltaUpdate
		}{
			deltaID:       deltaID,
			existingDelta: existingDelta,
			item:          item,
		})
	}

	// Step 2: Prepare updates (in memory modifications)
	for _, data := range updateData {
		item := data.item
		existingDelta := data.existingDelta

		if item.Name != nil {
			switch item.Name.GetKind() {
			case apiv1.HttpDeltaUpdate_NameUnion_KIND_UNSET:
				existingDelta.DeltaName = nil
			case apiv1.HttpDeltaUpdate_NameUnion_KIND_VALUE:
				nameStr := item.Name.GetValue()
				existingDelta.DeltaName = &nameStr
			}
		}
		if item.Method != nil {
			switch item.Method.GetKind() {
			case apiv1.HttpDeltaUpdate_MethodUnion_KIND_UNSET:
				existingDelta.DeltaMethod = nil
			case apiv1.HttpDeltaUpdate_MethodUnion_KIND_VALUE:
				method := item.Method.GetValue()
				existingDelta.DeltaMethod = httpMethodToString(&method)
			}
		}
		if item.Url != nil {
			switch item.Url.GetKind() {
			case apiv1.HttpDeltaUpdate_UrlUnion_KIND_UNSET:
				existingDelta.DeltaUrl = nil
			case apiv1.HttpDeltaUpdate_UrlUnion_KIND_VALUE:
				urlStr := item.Url.GetValue()
				existingDelta.DeltaUrl = &urlStr
			}
		}
		if item.BodyKind != nil {
			// Note: BodyKind is not currently in the mhttp.HTTP model delta fields
			// This would need to be added to the model and database schema if needed
		}
	}

	// Step 3: Execute updates in transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	hsService := h.hs.TX(tx)
	var updatedDeltas []mhttp.HTTP

	for _, data := range updateData {
		if err := hsService.Update(ctx, data.existingDelta); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		updatedDeltas = append(updatedDeltas, *data.existingDelta)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish update events for real-time sync after successful commit
	for _, delta := range updatedDeltas {
		h.stream.Publish(HttpTopic{WorkspaceID: delta.WorkspaceID}, HttpEvent{
			Type: eventTypeUpdate,
			Http: converter.ToAPIHttp(delta),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpDeltaDelete(ctx context.Context, req *connect.Request[apiv1.HttpDeltaDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP delta must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var deleteData []struct {
		deltaID       idwrap.IDWrap
		existingDelta *mhttp.HTTP
	}

	for _, item := range req.Msg.Items {
		if len(item.DeltaHttpId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("delta_http_id is required"))
		}

		deltaID, err := idwrap.NewFromBytes(item.DeltaHttpId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing delta HTTP entry - use pool service
		existingDelta, err := h.hs.Get(ctx, deltaID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHTTPFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Verify this is actually a delta record
		if !existingDelta.IsDelta {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("specified HTTP entry is not a delta"))
		}

		// Check delete access to the workspace
		if err := h.checkWorkspaceDeleteAccess(ctx, existingDelta.WorkspaceID); err != nil {
			return nil, err
		}

		deleteData = append(deleteData, struct {
			deltaID       idwrap.IDWrap
			existingDelta *mhttp.HTTP
		}{
			deltaID:       deltaID,
			existingDelta: existingDelta,
		})
	}

	// Step 2: Execute deletes in transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	hsService := h.hs.TX(tx)
	var deletedDeltas []mhttp.HTTP
	var deletedWorkspaceIDs []idwrap.IDWrap

	for _, data := range deleteData {
		// Delete the delta record
		if err := hsService.Delete(ctx, data.deltaID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		deletedDeltas = append(deletedDeltas, *data.existingDelta)
		deletedWorkspaceIDs = append(deletedWorkspaceIDs, data.existingDelta.WorkspaceID)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish delete events for real-time sync after successful commit
	for i, delta := range deletedDeltas {
		h.stream.Publish(HttpTopic{WorkspaceID: deletedWorkspaceIDs[i]}, HttpEvent{
			Type: eventTypeDelete,
			Http: converter.ToAPIHttp(delta),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpDeltaSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpDeltaSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return h.streamHttpDeltaSync(ctx, userID, stream.Send)
}

// streamHttpDeltaSync streams HTTP delta events to the client
func (h *HttpServiceRPC) streamHttpDeltaSync(ctx context.Context, userID idwrap.IDWrap, send func(*apiv1.HttpDeltaSyncResponse) error) error {
	var workspaceSet sync.Map

	// Filter for workspace-based access control
	filter := func(topic HttpTopic) bool {
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

	// Converter with data fetching logic
	converter := func(event HttpEvent) *apiv1.HttpDeltaSyncResponse {
		// Get the full HTTP record for delta sync response
		httpID, err := idwrap.NewFromBytes(event.Http.HttpId)
		if err != nil {
			return nil // Skip if can't parse ID
		}
		httpRecord, err := h.hs.Get(ctx, httpID)
		if err != nil {
			return nil // Skip if can't get the record
		}

		// Filter: Only process actual Delta records
		if !httpRecord.IsDelta {
			return nil
		}

		return httpDeltaSyncResponseFrom(event, *httpRecord)
	}

	return eventstream.StreamToClient(
		ctx,
		h.stream,
		nil,
		filter,
		converter,
		send,
	)
}

func (h *HttpServiceRPC) HttpSearchParamDeltaCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpSearchParamDeltaCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Get user's workspaces
	workspaces, err := h.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var allDeltas []*apiv1.HttpSearchParamDelta
	for _, workspace := range workspaces {
		// Get HTTP entries for this workspace
		httpList, err := h.hs.GetDeltasByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Get params for each HTTP entry
		for _, http := range httpList {
			params, err := h.httpSearchParamService.GetByHttpIDOrdered(ctx, http.ID)
			if err != nil {
				if errors.Is(err, shttp.ErrNoHttpSearchParamFound) {
					continue
				}
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			// Convert to delta format
			for _, param := range params {
				if !param.IsDelta {
					continue
				}

				delta := &apiv1.HttpSearchParamDelta{
					DeltaHttpSearchParamId: param.ID.Bytes(),
					HttpId:                 param.HttpID.Bytes(),
				}

				if param.ParentHttpSearchParamID != nil {
					delta.HttpSearchParamId = param.ParentHttpSearchParamID.Bytes()
				}

				// Only include delta fields if they exist
				if param.DeltaKey != nil {
					delta.Key = param.DeltaKey
				}
				if param.DeltaValue != nil {
					delta.Value = param.DeltaValue
				}
				if param.DeltaEnabled != nil {
					delta.Enabled = param.DeltaEnabled
				}
				if param.DeltaDescription != nil {
					delta.Description = param.DeltaDescription
				}
				if param.DeltaOrder != nil {
					order := float32(*param.DeltaOrder)
					delta.Order = &order
				}

				allDeltas = append(allDeltas, delta)
			}
		}
	}

	return connect.NewResponse(&apiv1.HttpSearchParamDeltaCollectionResponse{
		Items: allDeltas,
	}), nil
}

func (h *HttpServiceRPC) HttpSearchParamDeltaInsert(ctx context.Context, req *connect.Request[apiv1.HttpSearchParamDeltaInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.Items) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one delta item is required"))
	}

	// Process each delta item
	for _, item := range req.Msg.Items {
		if len(item.HttpSearchParamId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_search_param_id is required for each delta item"))
		}

		paramID, err := idwrap.NewFromBytes(item.HttpSearchParamId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Check workspace write access
		param, err := h.httpSearchParamService.GetByID(ctx, paramID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHttpSearchParamFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		httpEntry, err := h.hs.Get(ctx, param.HttpID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := h.checkWorkspaceWriteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		// Update delta fields
		var deltaOrder *float64
		if item.Order != nil {
			order := float64(*item.Order)
			deltaOrder = &order
		}
		err = h.httpSearchParamService.UpdateDelta(ctx, paramID, item.Key, item.Value, item.Enabled, item.Description, deltaOrder)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpSearchParamDeltaUpdate(ctx context.Context, req *connect.Request[apiv1.HttpSearchParamDeltaUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP search param delta must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var updateData []struct {
		deltaID       idwrap.IDWrap
		existingParam *mhttp.HTTPSearchParam
		item          *apiv1.HttpSearchParamDeltaUpdate
	}

	for _, item := range req.Msg.Items {
		if len(item.DeltaHttpSearchParamId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("delta_http_search_param_id is required"))
		}

		deltaID, err := idwrap.NewFromBytes(item.DeltaHttpSearchParamId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing delta param - use pool service
		existingParam, err := h.httpSearchParamService.GetByID(ctx, deltaID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHttpSearchParamFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Verify this is actually a delta record
		if !existingParam.IsDelta {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("specified HTTP search param is not a delta"))
		}

		// Get the HTTP entry to check workspace access - use pool service
		httpEntry, err := h.hs.Get(ctx, existingParam.HttpID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Check write access to the workspace
		if err := h.checkWorkspaceWriteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		updateData = append(updateData, struct {
			deltaID       idwrap.IDWrap
			existingParam *mhttp.HTTPSearchParam
			item          *apiv1.HttpSearchParamDeltaUpdate
		}{
			deltaID:       deltaID,
			existingParam: existingParam,
			item:          item,
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
			case apiv1.HttpSearchParamDeltaUpdate_KeyUnion_KIND_UNSET:
				deltaKey = nil
			case apiv1.HttpSearchParamDeltaUpdate_KeyUnion_KIND_VALUE:
				keyStr := item.Key.GetValue()
				deltaKey = &keyStr
			}
		}
		if item.Value != nil {
			switch item.Value.GetKind() {
			case apiv1.HttpSearchParamDeltaUpdate_ValueUnion_KIND_UNSET:
				deltaValue = nil
			case apiv1.HttpSearchParamDeltaUpdate_ValueUnion_KIND_VALUE:
				valueStr := item.Value.GetValue()
				deltaValue = &valueStr
			}
		}
		if item.Enabled != nil {
			switch item.Enabled.GetKind() {
			case apiv1.HttpSearchParamDeltaUpdate_EnabledUnion_KIND_UNSET:
				deltaEnabled = nil
			case apiv1.HttpSearchParamDeltaUpdate_EnabledUnion_KIND_VALUE:
				enabledBool := item.Enabled.GetValue()
				deltaEnabled = &enabledBool
			}
		}
		if item.Description != nil {
			switch item.Description.GetKind() {
			case apiv1.HttpSearchParamDeltaUpdate_DescriptionUnion_KIND_UNSET:
				deltaDescription = nil
			case apiv1.HttpSearchParamDeltaUpdate_DescriptionUnion_KIND_VALUE:
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
	defer tx.Rollback()

	httpSearchParamService := h.httpSearchParamService.TX(tx)
	var updatedParams []mhttp.HTTPSearchParam

	for _, update := range preparedUpdates {
		if err := httpSearchParamService.UpdateDelta(ctx, update.deltaID, update.deltaKey, update.deltaValue, update.deltaEnabled, update.deltaDescription, nil); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Get updated param for event publishing (must get from TX service to see changes)
		updatedParam, err := httpSearchParamService.GetByID(ctx, update.deltaID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		updatedParams = append(updatedParams, *updatedParam)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish update events for real-time sync after successful commit
	for _, param := range updatedParams {
		// Get workspace ID for the HTTP entry
		httpEntry, err := h.hs.Get(ctx, param.HttpID)
		if err != nil {
			continue
		}
		h.httpSearchParamStream.Publish(HttpSearchParamTopic{WorkspaceID: httpEntry.WorkspaceID}, HttpSearchParamEvent{
			Type:            eventTypeUpdate,
			HttpSearchParam: converter.ToAPIHttpSearchParam(param),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpSearchParamDeltaDelete(ctx context.Context, req *connect.Request[apiv1.HttpSearchParamDeltaDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP search param delta must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var deleteData []struct {
		deltaID       idwrap.IDWrap
		existingParam *mhttp.HTTPSearchParam
		workspaceID   idwrap.IDWrap
	}

	for _, item := range req.Msg.Items {
		if len(item.DeltaHttpSearchParamId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("delta_http_search_param_id is required"))
		}

		deltaID, err := idwrap.NewFromBytes(item.DeltaHttpSearchParamId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing delta param - use pool service
		existingParam, err := h.httpSearchParamService.GetByID(ctx, deltaID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHttpSearchParamFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Verify this is actually a delta record
		if !existingParam.IsDelta {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("specified HTTP search param is not a delta"))
		}

		// Get the HTTP entry to check workspace access - use pool service
		httpEntry, err := h.hs.Get(ctx, existingParam.HttpID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Check delete access to the workspace
		if err := h.checkWorkspaceDeleteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		deleteData = append(deleteData, struct {
			deltaID       idwrap.IDWrap
			existingParam *mhttp.HTTPSearchParam
			workspaceID   idwrap.IDWrap
		}{
			deltaID:       deltaID,
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
		// Delete the delta record
		if err := httpSearchParamService.Delete(ctx, data.deltaID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		deletedParams = append(deletedParams, *data.existingParam)
		deletedWorkspaceIDs = append(deletedWorkspaceIDs, data.workspaceID)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish delete events for real-time sync after successful commit
	for i, param := range deletedParams {
		h.httpSearchParamStream.Publish(HttpSearchParamTopic{WorkspaceID: deletedWorkspaceIDs[i]}, HttpSearchParamEvent{
			Type:            eventTypeDelete,
			HttpSearchParam: converter.ToAPIHttpSearchParam(param),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpSearchParamDeltaSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpSearchParamDeltaSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return h.streamHttpSearchParamDeltaSync(ctx, userID, stream.Send)
}

func (h *HttpServiceRPC) streamHttpSearchParamDeltaSync(ctx context.Context, userID idwrap.IDWrap, send func(*apiv1.HttpSearchParamDeltaSyncResponse) error) error {
	var workspaceSet sync.Map

	// Filter for workspace-based access control
	filter := func(topic HttpSearchParamTopic) bool {
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
	events, err := h.httpSearchParamStream.Subscribe(ctx, filter)
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
			// Get the full param record for delta sync response
			paramID, err := idwrap.NewFromBytes(evt.Payload.HttpSearchParam.GetHttpSearchParamId())
			if err != nil {
				continue // Skip if can't parse ID
			}
			paramRecord, err := h.httpSearchParamService.GetByID(ctx, paramID)
			if err != nil {
				continue // Skip if can't get the record
			}
			if !paramRecord.IsDelta {
				continue
			}
			resp := httpSearchParamDeltaSyncResponseFrom(evt.Payload, *paramRecord)
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
			asserts, err := h.httpAssertService.GetHttpAssertsByHttpID(ctx, http.ID)
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
		assert, err := h.httpAssertService.GetHttpAssert(ctx, assertID)
		if err != nil {
			if errors.Is(err, shttpassert.ErrNoHttpAssertFound) {
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
		err = h.httpAssertService.UpdateHttpAssertDelta(ctx, assertID, nil, item.Value, nil, nil, nil)
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
		existingAssert *mhttpassert.HttpAssert
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
		existingAssert, err := h.httpAssertService.GetHttpAssert(ctx, deltaID)
		if err != nil {
			if errors.Is(err, shttpassert.ErrNoHttpAssertFound) {
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
			existingAssert *mhttpassert.HttpAssert
			item           *apiv1.HttpAssertDeltaUpdate
		}{
			deltaID:        deltaID,
			existingAssert: existingAssert,
			item:           item,
		})
	}

	// Step 2: Prepare updates (in memory)
	var preparedUpdates []struct {
		deltaID    idwrap.IDWrap
		deltaValue *string
	}

	for _, data := range updateData {
		item := data.item
		var deltaValue *string

		if item.Value != nil {
			switch item.Value.GetKind() {
			case apiv1.HttpAssertDeltaUpdate_ValueUnion_KIND_UNSET:
				deltaValue = nil
			case apiv1.HttpAssertDeltaUpdate_ValueUnion_KIND_VALUE:
				valueStr := item.Value.GetValue()
				deltaValue = &valueStr
			}
		}

		preparedUpdates = append(preparedUpdates, struct {
			deltaID    idwrap.IDWrap
			deltaValue *string
		}{
			deltaID:    data.deltaID,
			deltaValue: deltaValue,
		})
	}

	// Step 3: Execute updates in transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	httpAssertService := h.httpAssertService.TX(tx)
	var updatedAsserts []mhttpassert.HttpAssert

	for _, update := range preparedUpdates {
		// HttpAssert only supports updating Value delta currently (based on Insert implementation)
		if err := httpAssertService.UpdateHttpAssertDelta(ctx, update.deltaID, nil, update.deltaValue, nil, nil, nil); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Get updated assert for event publishing (from TX service)
		updatedAssert, err := httpAssertService.GetHttpAssert(ctx, update.deltaID)
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
		h.httpAssertStream.Publish(HttpAssertTopic{WorkspaceID: httpEntry.WorkspaceID}, HttpAssertEvent{
			Type:       eventTypeUpdate,
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
		existingAssert *mhttpassert.HttpAssert
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
		existingAssert, err := h.httpAssertService.GetHttpAssert(ctx, deltaID)
		if err != nil {
			if errors.Is(err, shttpassert.ErrNoHttpAssertFound) {
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
			existingAssert *mhttpassert.HttpAssert
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
	defer tx.Rollback()

	httpAssertService := h.httpAssertService.TX(tx)
	var deletedAsserts []mhttpassert.HttpAssert
	var deletedWorkspaceIDs []idwrap.IDWrap

	for _, data := range deleteData {
		// Delete the delta record
		if err := httpAssertService.DeleteHttpAssert(ctx, data.deltaID); err != nil {
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
		h.httpAssertStream.Publish(HttpAssertTopic{WorkspaceID: deletedWorkspaceIDs[i]}, HttpAssertEvent{
			Type:       eventTypeDelete,
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
	events, err := h.httpAssertStream.Subscribe(ctx, filter)
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
			assertRecord, err := h.httpAssertService.GetHttpAssert(ctx, assertID)
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

// streamHttpBodyFormSync streams HTTP body form events to the client
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
	defer tx.Rollback()

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
		h.httpHeaderStream.Publish(HttpHeaderTopic{WorkspaceID: httpEntry.WorkspaceID}, HttpHeaderEvent{
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
	defer tx.Rollback()

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
		h.httpHeaderStream.Publish(HttpHeaderTopic{WorkspaceID: deletedWorkspaceIDs[i]}, HttpHeaderEvent{
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
	events, err := h.httpHeaderStream.Subscribe(ctx, filter)
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
				if bodyForm.DeltaOrder != nil {
					delta.Order = bodyForm.DeltaOrder
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

	for _, data := range updateData {
		item := data.item
		var deltaKey, deltaValue, deltaDescription *string
		var deltaEnabled *bool
		var deltaOrder *float32

		if item.Key != nil {
			switch item.Key.GetKind() {
			case apiv1.HttpBodyFormDataDeltaUpdate_KeyUnion_KIND_UNSET:
				deltaKey = nil
			case apiv1.HttpBodyFormDataDeltaUpdate_KeyUnion_KIND_VALUE:
				keyStr := item.Key.GetValue()
				deltaKey = &keyStr
			}
		}
		if item.Value != nil {
			switch item.Value.GetKind() {
			case apiv1.HttpBodyFormDataDeltaUpdate_ValueUnion_KIND_UNSET:
				deltaValue = nil
			case apiv1.HttpBodyFormDataDeltaUpdate_ValueUnion_KIND_VALUE:
				valueStr := item.Value.GetValue()
				deltaValue = &valueStr
			}
		}
		if item.Enabled != nil {
			switch item.Enabled.GetKind() {
			case apiv1.HttpBodyFormDataDeltaUpdate_EnabledUnion_KIND_UNSET:
				deltaEnabled = nil
			case apiv1.HttpBodyFormDataDeltaUpdate_EnabledUnion_KIND_VALUE:
				enabledBool := item.Enabled.GetValue()
				deltaEnabled = &enabledBool
			}
		}
		if item.Description != nil {
			switch item.Description.GetKind() {
			case apiv1.HttpBodyFormDataDeltaUpdate_DescriptionUnion_KIND_UNSET:
				deltaDescription = nil
			case apiv1.HttpBodyFormDataDeltaUpdate_DescriptionUnion_KIND_VALUE:
				descStr := item.Description.GetValue()
				deltaDescription = &descStr
			}
		}
		if item.Order != nil {
			switch item.Order.GetKind() {
			case apiv1.HttpBodyFormDataDeltaUpdate_OrderUnion_KIND_UNSET:
				deltaOrder = nil
			case apiv1.HttpBodyFormDataDeltaUpdate_OrderUnion_KIND_VALUE:
				orderVal := item.Order.GetValue()
				deltaOrder = &orderVal
			}
		}

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
	defer tx.Rollback()

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
	for _, bodyForm := range updatedBodyForms {
		// Get workspace ID for the HTTP entry
		httpEntry, err := h.hs.Get(ctx, bodyForm.HttpID)
		if err != nil {
			continue
		}
		h.httpBodyFormStream.Publish(HttpBodyFormTopic{WorkspaceID: httpEntry.WorkspaceID}, HttpBodyFormEvent{
			Type:         eventTypeUpdate,
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
	defer tx.Rollback()

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
		h.httpBodyFormStream.Publish(HttpBodyFormTopic{WorkspaceID: deletedWorkspaceIDs[i]}, HttpBodyFormEvent{
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
	events, err := h.httpBodyFormStream.Subscribe(ctx, filter)
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
				if bodyUrlEncoded.DeltaOrder != nil {
					delta.Order = bodyUrlEncoded.DeltaOrder
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

	for _, data := range updateData {
		item := data.item
		var deltaKey, deltaValue, deltaDescription *string
		var deltaEnabled *bool
		var deltaOrder *float32

		if item.Key != nil {
			switch item.Key.GetKind() {
			case apiv1.HttpBodyUrlEncodedDeltaUpdate_KeyUnion_KIND_UNSET:
				deltaKey = nil
			case apiv1.HttpBodyUrlEncodedDeltaUpdate_KeyUnion_KIND_VALUE:
				keyStr := item.Key.GetValue()
				deltaKey = &keyStr
			}
		}
		if item.Value != nil {
			switch item.Value.GetKind() {
			case apiv1.HttpBodyUrlEncodedDeltaUpdate_ValueUnion_KIND_UNSET:
				deltaValue = nil
			case apiv1.HttpBodyUrlEncodedDeltaUpdate_ValueUnion_KIND_VALUE:
				valueStr := item.Value.GetValue()
				deltaValue = &valueStr
			}
		}
		if item.Enabled != nil {
			switch item.Enabled.GetKind() {
			case apiv1.HttpBodyUrlEncodedDeltaUpdate_EnabledUnion_KIND_UNSET:
				deltaEnabled = nil
			case apiv1.HttpBodyUrlEncodedDeltaUpdate_EnabledUnion_KIND_VALUE:
				enabledBool := item.Enabled.GetValue()
				deltaEnabled = &enabledBool
			}
		}
		if item.Description != nil {
			switch item.Description.GetKind() {
			case apiv1.HttpBodyUrlEncodedDeltaUpdate_DescriptionUnion_KIND_UNSET:
				deltaDescription = nil
			case apiv1.HttpBodyUrlEncodedDeltaUpdate_DescriptionUnion_KIND_VALUE:
				descStr := item.Description.GetValue()
				deltaDescription = &descStr
			}
		}
		if item.Order != nil {
			switch item.Order.GetKind() {
			case apiv1.HttpBodyUrlEncodedDeltaUpdate_OrderUnion_KIND_UNSET:
				deltaOrder = nil
			case apiv1.HttpBodyUrlEncodedDeltaUpdate_OrderUnion_KIND_VALUE:
				orderVal := item.Order.GetValue()
				deltaOrder = &orderVal
			}
		}

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
	defer tx.Rollback()

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
	for _, bodyUrlEncoded := range updatedBodyUrlEncodeds {
		// Get workspace ID for the HTTP entry
		httpEntry, err := h.hs.Get(ctx, bodyUrlEncoded.HttpID)
		if err != nil {
			continue
		}
		h.httpBodyUrlEncodedStream.Publish(HttpBodyUrlEncodedTopic{WorkspaceID: httpEntry.WorkspaceID}, HttpBodyUrlEncodedEvent{
			Type:               eventTypeUpdate,
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
	defer tx.Rollback()

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
		h.httpBodyUrlEncodedStream.Publish(HttpBodyUrlEncodedTopic{WorkspaceID: deletedWorkspaceIDs[i]}, HttpBodyUrlEncodedEvent{
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
	events, err := h.httpBodyUrlEncodedStream.Subscribe(ctx, filter)
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

// streamHttpBodyUrlEncodedSync streams HTTP body URL encoded events to the client
func (h *HttpServiceRPC) HttpBodyRawDeltaCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpBodyRawDeltaCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	workspaces, err := h.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var allDeltas []*apiv1.HttpBodyRawDelta
	for _, workspace := range workspaces {
		httpList, err := h.hs.GetDeltasByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		for _, http := range httpList {
			body, err := h.bodyService.GetByHttpID(ctx, http.ID)
			if err != nil && !errors.Is(err, shttp.ErrNoHttpBodyRawFound) {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			if body != nil {
				data := string(body.RawData)
				httpId := body.HttpID.Bytes()
				if http.ParentHttpID != nil {
					httpId = http.ParentHttpID.Bytes()
				}

				allDeltas = append(allDeltas, &apiv1.HttpBodyRawDelta{
					HttpId: httpId,
					Data:   &data,
				})
			}
		}
	}

	return connect.NewResponse(&apiv1.HttpBodyRawDeltaCollectionResponse{
		Items: allDeltas,
	}), nil
}

func (h *HttpServiceRPC) HttpBodyRawDeltaInsert(ctx context.Context, req *connect.Request[apiv1.HttpBodyRawDeltaInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.Items) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one delta item is required"))
	}

	for _, item := range req.Msg.Items {
		if len(item.HttpId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_id is required for each delta item"))
		}

		httpID, err := idwrap.NewFromBytes(item.HttpId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Check workspace write access
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

		// Create delta body raw
		// Data is optional (can be empty string)
		data := ""
		if item.Data != nil {
			data = *item.Data
		}

		// Use CreateDelta from body service
		// We assume default content type or empty for now, as API doesn't seem to pass it in Insert
		bodyRaw, err := h.bodyService.CreateDelta(ctx, httpID, []byte(data), "")
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		h.httpBodyRawStream.Publish(HttpBodyRawTopic{WorkspaceID: httpEntry.WorkspaceID}, HttpBodyRawEvent{
			Type:        eventTypeInsert,
			IsDelta:     true,
			HttpBodyRaw: converter.ToAPIHttpBodyRawFromMHttp(*bodyRaw),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyRawDeltaUpdate(ctx context.Context, req *connect.Request[apiv1.HttpBodyRawDeltaUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP body raw delta must be provided"))
	}

	for _, item := range req.Msg.Items {
		if len(item.HttpId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_id is required"))
		}

		httpID, err := idwrap.NewFromBytes(item.HttpId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Check workspace write access and if body exists
		httpEntry, err := h.hs.Get(ctx, httpID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if !httpEntry.IsDelta {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("specified HTTP entry is not a delta"))
		}

		if err := h.checkWorkspaceWriteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		bodyRaw, err := h.bodyService.GetByHttpID(ctx, httpID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHttpBodyRawFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Prepare update
		var deltaData []byte
		var deltaContentType *string

		if item.Data != nil {
			switch item.Data.GetKind() {
			case apiv1.HttpBodyRawDeltaUpdate_DataUnion_KIND_UNSET:
				deltaData = nil
			case apiv1.HttpBodyRawDeltaUpdate_DataUnion_KIND_VALUE:
				deltaData = []byte(item.Data.GetValue())
			}
		}

		// Update using UpdateDelta
		updatedBody, err := h.bodyService.UpdateDelta(ctx, bodyRaw.ID, deltaData, deltaContentType)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		h.httpBodyRawStream.Publish(HttpBodyRawTopic{WorkspaceID: httpEntry.WorkspaceID}, HttpBodyRawEvent{
			Type:        eventTypeUpdate,
			IsDelta:     true,
			HttpBodyRaw: converter.ToAPIHttpBodyRawFromMHttp(*updatedBody),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyRawDeltaDelete(ctx context.Context, req *connect.Request[apiv1.HttpBodyRawDeltaDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP body raw delta must be provided"))
	}

	for _, item := range req.Msg.Items {
		if len(item.DeltaHttpId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("delta_http_id is required"))
		}

		httpID, err := idwrap.NewFromBytes(item.DeltaHttpId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Check workspace delete access
		httpEntry, err := h.hs.Get(ctx, httpID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if !httpEntry.IsDelta {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("specified HTTP entry is not a delta"))
		}

		if err := h.checkWorkspaceDeleteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		// Delete by HTTP ID
		if err := h.bodyService.DeleteByHttpID(ctx, httpID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Publish delete event
		// We need to construct a minimal object for the event since we deleted it
		deletedBody := &apiv1.HttpBodyRaw{
			HttpId: item.DeltaHttpId,
		}

		h.httpBodyRawStream.Publish(HttpBodyRawTopic{WorkspaceID: httpEntry.WorkspaceID}, HttpBodyRawEvent{
			Type:        eventTypeDelete,
			IsDelta:     true,
			HttpBodyRaw: deletedBody,
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyRawDeltaSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpBodyRawDeltaSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return h.streamHttpBodyRawDeltaSync(ctx, userID, stream.Send)
}

func (h *HttpServiceRPC) streamHttpBodyRawDeltaSync(ctx context.Context, userID idwrap.IDWrap, send func(*apiv1.HttpBodyRawDeltaSyncResponse) error) error {
	var workspaceSet sync.Map

	// Filter for workspace-based access control
	filter := func(topic HttpBodyRawTopic) bool {
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
	events, err := h.httpBodyRawStream.Subscribe(ctx, filter)
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
			// Only stream delta events
			if !evt.Payload.IsDelta {
				continue
			}

			var syncItem *apiv1.HttpBodyRawDeltaSync

			switch evt.Payload.Type {
			case eventTypeInsert:
				data := evt.Payload.HttpBodyRaw.Data
				syncItem = &apiv1.HttpBodyRawDeltaSync{
					Value: &apiv1.HttpBodyRawDeltaSync_ValueUnion{
						Kind: apiv1.HttpBodyRawDeltaSync_ValueUnion_KIND_INSERT,
						Insert: &apiv1.HttpBodyRawDeltaSyncInsert{
							DeltaHttpId: evt.Payload.HttpBodyRaw.HttpId,
							HttpId:      evt.Payload.HttpBodyRaw.HttpId,
							Data:        &data,
						},
					},
				}
			case eventTypeUpdate:
				data := evt.Payload.HttpBodyRaw.Data
				syncItem = &apiv1.HttpBodyRawDeltaSync{
					Value: &apiv1.HttpBodyRawDeltaSync_ValueUnion{
						Kind: apiv1.HttpBodyRawDeltaSync_ValueUnion_KIND_UPDATE,
						Update: &apiv1.HttpBodyRawDeltaSyncUpdate{
							DeltaHttpId: evt.Payload.HttpBodyRaw.HttpId,
							HttpId:      evt.Payload.HttpBodyRaw.HttpId,
							Data: &apiv1.HttpBodyRawDeltaSyncUpdate_DataUnion{
								Kind:  apiv1.HttpBodyRawDeltaSyncUpdate_DataUnion_KIND_VALUE,
								Value: &data,
							},
						},
					},
				}
			case eventTypeDelete:
				syncItem = &apiv1.HttpBodyRawDeltaSync{
					Value: &apiv1.HttpBodyRawDeltaSync_ValueUnion{
						Kind: apiv1.HttpBodyRawDeltaSync_ValueUnion_KIND_DELETE,
						Delete: &apiv1.HttpBodyRawDeltaSyncDelete{
							DeltaHttpId: evt.Payload.HttpBodyRaw.HttpId,
						},
					},
				}
			}

			if syncItem != nil {
				resp := &apiv1.HttpBodyRawDeltaSyncResponse{
					Items: []*apiv1.HttpBodyRawDeltaSync{syncItem},
				}
				if err := send(resp); err != nil {
					return err
				}
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
