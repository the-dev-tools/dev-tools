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

	"the-dev-tools/server/pkg/service/shttp"
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
			Type:    eventTypeUpdate,
			IsDelta: true,
			Http:    converter.ToAPIHttp(delta),
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
			Type:    eventTypeDelete,
			IsDelta: true,
			Http:    converter.ToAPIHttp(delta),
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
