//nolint:revive // exported
package rhttp

import (
	"context"
	"errors"
	"sync"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/converter"
	"the-dev-tools/server/pkg/idwrap"

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
		bodyRaw, err := h.bodyService.CreateDelta(ctx, httpID, []byte(data))
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		h.streamers.HttpBodyRaw.Publish(HttpBodyRawTopic{WorkspaceID: httpEntry.WorkspaceID}, HttpBodyRawEvent{
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
		deltaData := bodyRaw.DeltaRawData
		patch := make(DeltaPatch)

		if item.Data != nil {
			switch item.Data.GetKind() {
			case apiv1.HttpBodyRawDeltaUpdate_DataUnion_KIND_UNSET:
				deltaData = nil
				patch["data"] = nil
			case apiv1.HttpBodyRawDeltaUpdate_DataUnion_KIND_VALUE:
				strVal := item.Data.GetValue()
				deltaData = []byte(strVal)
				patch["data"] = &strVal
			}
		}

		// Update using UpdateDelta
		updatedBody, err := h.bodyService.UpdateDelta(ctx, bodyRaw.ID, deltaData)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		h.streamers.HttpBodyRaw.Publish(HttpBodyRawTopic{WorkspaceID: httpEntry.WorkspaceID}, HttpBodyRawEvent{
			Type:        eventTypeUpdate,
			IsDelta:     true,
			Patch:       patch,
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

		h.streamers.HttpBodyRaw.Publish(HttpBodyRawTopic{WorkspaceID: httpEntry.WorkspaceID}, HttpBodyRawEvent{
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
				if evt.Payload.Patch != nil {
					if val, ok := evt.Payload.Patch["data"]; ok {
						if strPtr, ok := val.(*string); ok && strPtr != nil {
							syncItem.Value.Update.Data = &apiv1.HttpBodyRawDeltaSyncUpdate_DataUnion{
								Kind:  apiv1.HttpBodyRawDeltaSyncUpdate_DataUnion_KIND_VALUE,
								Value: strPtr,
							}
						} else {
							syncItem.Value.Update.Data = &apiv1.HttpBodyRawDeltaSyncUpdate_DataUnion{
								Kind:  apiv1.HttpBodyRawDeltaSyncUpdate_DataUnion_KIND_UNSET,
								Unset: globalv1.Unset_UNSET.Enum(),
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
