//nolint:revive // exported
package rhttp

import (
	"context"
	"errors"
	"sync"
	"time"

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
	globalv1 "the-dev-tools/spec/dist/buf/go/global/v1"
)

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
				// For delta bodies, the override content is stored in DeltaRawData
				// Only use RawData as a fallback if DeltaRawData is empty
				var data string
				if len(body.DeltaRawData) > 0 {
					data = string(body.DeltaRawData)
				} else {
					data = string(body.RawData)
				}

				// Use the delta HTTP's own ID - this matches what frontend queries by (deltaHttpId)
				// NOT the ParentHttpID, which would cause a key mismatch
				httpId := http.ID.Bytes()

				allDeltas = append(allDeltas, &apiv1.HttpBodyRawDelta{
					HttpId:      httpId,
					DeltaHttpId: httpId,
					Data:        &data,
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

	// FETCH: Gather data and check permissions OUTSIDE transaction
	type insertItem struct {
		httpID          idwrap.IDWrap
		workspaceID     idwrap.IDWrap
		parentBodyRawID *idwrap.IDWrap
		data            string
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

		// Find parent body raw ID (required by schema constraint for deltas)
		if httpEntry.ParentHttpID == nil {
			return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("delta HTTP entry must have a parent"))
		}

		parentBody, err := h.bodyService.GetByHttpID(ctx, *httpEntry.ParentHttpID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHttpBodyRawFound) {
				return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("parent HTTP entry must have a body"))
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		data := ""
		if item.Data != nil {
			data = *item.Data
		}

		insertData = append(insertData, insertItem{
			httpID:          httpID,
			workspaceID:     httpEntry.WorkspaceID,
			parentBodyRawID: &parentBody.ID,
			data:            data,
		})
	}

	// ACT: Insert using mutation context
	mut := mutation.New(h.DB, mutation.WithPublisher(h.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	now := time.Now().UnixMilli()
	for _, data := range insertData {
		newID := idwrap.NewNow()
		params := gen.CreateHTTPBodyRawParams{
			ID:              newID,
			HttpID:          data.httpID,
			RawData:         []byte(""), // Base data empty for delta
			ParentBodyRawID: data.parentBodyRawID,
			IsDelta:         true,
			DeltaRawData:    []byte(data.data),
			CreatedAt:       now,
			UpdatedAt:       now,
		}

		if err := mut.InsertHTTPBodyRaw(ctx, mutation.HTTPBodyRawInsertItem{
			ID:          newID,
			HttpID:      data.httpID,
			WorkspaceID: data.workspaceID,
			IsDelta:     true,
			Params:      params,
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Fetch the full model for publisher
		bodyService := h.bodyService.TX(mut.TX())
		updated, err := bodyService.GetByHttpID(ctx, data.httpID)
		if err == nil {
			mut.UpdateLastEventPayload(*updated)
		}
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyRawDeltaUpdate(ctx context.Context, req *connect.Request[apiv1.HttpBodyRawDeltaUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP body raw delta must be provided"))
	}

	// FETCH: Gather data and check permissions OUTSIDE transaction
	type updateItem struct {
		httpID      idwrap.IDWrap
		workspaceID idwrap.IDWrap
		bodyRaw     mhttp.HTTPBodyRaw
		item        *apiv1.HttpBodyRawDeltaUpdate
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

		// Check workspace write access
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

		updateData = append(updateData, updateItem{
			httpID:      httpID,
			workspaceID: httpEntry.WorkspaceID,
			bodyRaw:     *bodyRaw,
			item:        item,
		})
	}

	// ACT: Update using mutation context
	mut := mutation.New(h.DB, mutation.WithPublisher(h.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	for _, data := range updateData {
		deltaData := data.bodyRaw.DeltaRawData
		var patchData patch.HTTPBodyRawPatch

		if data.item.Data != nil {
			switch data.item.Data.GetKind() {
			case apiv1.HttpBodyRawDeltaUpdate_DataUnion_KIND_UNSET:
				deltaData = nil
				patchData.Data = patch.Unset[string]()
			case apiv1.HttpBodyRawDeltaUpdate_DataUnion_KIND_VALUE:
				strVal := data.item.Data.GetValue()
				deltaData = []byte(strVal)
				patchData.Data = patch.NewOptional(strVal)
			}
		}

		bodyService := h.bodyService.TX(mut.TX())
		if err := mut.UpdateHTTPBodyRawDelta(ctx, mutation.HTTPBodyRawDeltaUpdateItem{
			ID:          data.bodyRaw.ID,
			HttpID:      data.httpID,
			WorkspaceID: data.workspaceID,
			Params: gen.UpdateHTTPBodyRawDeltaParams{
				ID:           data.bodyRaw.ID,
				DeltaRawData: deltaData,
			},
			Patch: patchData,
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Update payload in tracked event
		updated, err := bodyService.GetByHttpID(ctx, data.httpID)
		if err == nil {
			mut.UpdateLastEventPayload(*updated)
		}
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyRawDeltaDelete(ctx context.Context, req *connect.Request[apiv1.HttpBodyRawDeltaDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP body raw delta must be provided"))
	}

	// FETCH: Gather data and check permissions OUTSIDE transaction
	type deleteItem struct {
		httpID      idwrap.IDWrap
		workspaceID idwrap.IDWrap
		bodyRawID   idwrap.IDWrap
		bodyRaw     mhttp.HTTPBodyRaw
	}
	deleteData := make([]deleteItem, 0, len(req.Msg.Items))

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

		bodyRaw, err := h.bodyService.GetByHttpID(ctx, httpID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHttpBodyRawFound) {
				continue
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		deleteData = append(deleteData, deleteItem{
			httpID:      httpID,
			workspaceID: httpEntry.WorkspaceID,
			bodyRawID:   bodyRaw.ID,
			bodyRaw:     *bodyRaw,
		})
	}

	// ACT: Delete using mutation context
	mut := mutation.New(h.DB, mutation.WithPublisher(h.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	for _, data := range deleteData {
		mut.Track(mutation.Event{
			Entity:      mutation.EntityHTTPBodyRaw,
			Op:          mutation.OpDelete,
			ID:          data.bodyRawID,
			ParentID:    data.httpID,
			WorkspaceID: data.workspaceID,
			IsDelta:     true,
			Payload:     data.bodyRaw,
		})
		if err := mut.Queries().DeleteHTTPBodyRaw(ctx, data.bodyRawID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
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
	events, err := h.streamers.HttpBodyRaw.Subscribe(ctx, filter)
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
				syncItem = &apiv1.HttpBodyRawDeltaSync{
					Value: &apiv1.HttpBodyRawDeltaSync_ValueUnion{
						Kind: apiv1.HttpBodyRawDeltaSync_ValueUnion_KIND_UPDATE,
						Update: &apiv1.HttpBodyRawDeltaSyncUpdate{
							DeltaHttpId: evt.Payload.HttpBodyRaw.HttpId,
							HttpId:      evt.Payload.HttpBodyRaw.HttpId,
						},
					},
				}

				// Populate Data based on Patch if available, else Full State
				if evt.Payload.Patch.HasChanges() {
					if evt.Payload.Patch.Data.IsSet() {
						if evt.Payload.Patch.Data.IsUnset() {
							syncItem.Value.Update.Data = &apiv1.HttpBodyRawDeltaSyncUpdate_DataUnion{
								Kind:  apiv1.HttpBodyRawDeltaSyncUpdate_DataUnion_KIND_UNSET,
								Unset: globalv1.Unset_UNSET.Enum(),
							}
						} else {
							syncItem.Value.Update.Data = &apiv1.HttpBodyRawDeltaSyncUpdate_DataUnion{
								Kind:  apiv1.HttpBodyRawDeltaSyncUpdate_DataUnion_KIND_VALUE,
								Value: evt.Payload.Patch.Data.Value(),
							}
						}
					}
				} else {
					// Fallback to existing behavior (Always Value)
					data := evt.Payload.HttpBodyRaw.Data
					syncItem.Value.Update.Data = &apiv1.HttpBodyRawDeltaSyncUpdate_DataUnion{
						Kind:  apiv1.HttpBodyRawDeltaSyncUpdate_DataUnion_KIND_VALUE,
						Value: &data,
					}
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