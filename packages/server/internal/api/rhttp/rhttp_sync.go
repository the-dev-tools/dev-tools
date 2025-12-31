//nolint:revive // exported
package rhttp

import (
	"context"
	"sync"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/converter"
	"the-dev-tools/server/pkg/eventstream"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/patch"
	"the-dev-tools/server/pkg/txutil"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
)

func (h *HttpServiceRPC) publishInsertEvent(http mhttp.HTTP) {
	topic := HttpTopic{WorkspaceID: http.WorkspaceID}
	event := HttpEvent{
		Type:    eventTypeInsert,
		IsDelta: http.IsDelta,
		Http:    converter.ToAPIHttp(http),
	}
	h.streamers.Http.Publish(topic, event)
}

// publishUpdateEvent publishes an update event for real-time sync
func (h *HttpServiceRPC) publishUpdateEvent(http mhttp.HTTP, p patch.HTTPDeltaPatch) {
	topic := HttpTopic{WorkspaceID: http.WorkspaceID}
	event := HttpEvent{
		Type:    eventTypeUpdate,
		IsDelta: http.IsDelta,
		Patch:   p,
		Http:    converter.ToAPIHttp(http),
	}
	h.streamers.Http.Publish(topic, event)
}

// publishVersionInsertEvent publishes an insert event for real-time sync
func (h *HttpServiceRPC) publishVersionInsertEvent(version mhttp.HttpVersion, workspaceID idwrap.IDWrap) {
	topic := HttpVersionTopic{WorkspaceID: workspaceID}
	event := HttpVersionEvent{
		Type:        eventTypeInsert,
		HttpVersion: converter.ToAPIHttpVersion(version),
	}
	h.streamers.HttpVersion.Publish(topic, event)
}

// publishBulkHttpUpdate publishes multiple HTTP update events in bulk.
// Items are already grouped by HttpTopic by the BulkSyncTxUpdate wrapper.
func (h *HttpServiceRPC) publishBulkHttpUpdate(
	topic HttpTopic,
	events []txutil.UpdateEvent[mhttp.HTTP, patch.HTTPDeltaPatch],
) {
	httpEvents := make([]HttpEvent, len(events))
	for i, evt := range events {
		httpEvents[i] = HttpEvent{
			Type:    eventTypeUpdate,
			IsDelta: evt.Item.IsDelta,
			Patch:   evt.Patch, // Partial updates preserved!
			Http:    converter.ToAPIHttp(evt.Item),
		}
	}
	h.streamers.Http.Publish(topic, httpEvents...)
}

// publishBulkVersionInsert publishes multiple version insert events in bulk.
// Items are already grouped by HttpVersionTopic by the BulkSyncTxInsert wrapper.
func (h *HttpServiceRPC) publishBulkVersionInsert(
	topic HttpVersionTopic,
	items []versionWithWorkspace,
) {
	versionEvents := make([]HttpVersionEvent, len(items))
	for i, item := range items {
		versionEvents[i] = HttpVersionEvent{
			Type:        eventTypeInsert,
			HttpVersion: converter.ToAPIHttpVersion(item.version),
		}
	}
	h.streamers.HttpVersion.Publish(topic, versionEvents...)
}

// publishBulkHttpDelete publishes multiple HTTP delete events in bulk.
// Items are already grouped by HttpTopic by the BulkSyncTxDelete wrapper.
func (h *HttpServiceRPC) publishBulkHttpDelete(
	topic HttpTopic,
	events []txutil.DeleteEvent[idwrap.IDWrap],
) {
	httpEvents := make([]HttpEvent, len(events))
	for i, evt := range events {
		httpEvents[i] = HttpEvent{
			Type:    eventTypeDelete,
			IsDelta: evt.IsDelta,
			Http: &apiv1.Http{
				HttpId: evt.ID.Bytes(),
			},
		}
	}
	h.streamers.Http.Publish(topic, httpEvents...)
}

// listUserHttp retrieves all HTTP entries accessible to the user
func (h *HttpServiceRPC) HttpSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return h.streamHttpSync(ctx, userID, stream.Send)
}

// streamHttpSync streams HTTP events to the client
func (h *HttpServiceRPC) streamHttpSync(ctx context.Context, userID idwrap.IDWrap, send func(*apiv1.HttpSyncResponse) error) error {
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

	converter := func(events []HttpEvent) *apiv1.HttpSyncResponse {
		var items []*apiv1.HttpSync
		for _, event := range events {
			if event.IsDelta {
				continue
			}
			if resp := httpSyncResponseFrom(event); resp != nil && len(resp.Items) > 0 {
				items = append(items, resp.Items...)
			}
		}
		if len(items) == 0 {
			return nil
		}
		return &apiv1.HttpSyncResponse{Items: items}
	}

	return eventstream.StreamToClient(
		ctx,
		h.streamers.Http,
		nil,
		filter,
		converter,
		send,
		nil, // Use default batching options
	)
}

// CheckOwnerHttp verifies if a user owns an HTTP entry via workspace membership
func (h *HttpServiceRPC) HttpVersionSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpVersionSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return h.streamHttpVersionSync(ctx, userID, stream.Send)
}

func (h *HttpServiceRPC) HttpSearchParamSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpSearchParamSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return h.streamHttpSearchParamSync(ctx, userID, stream.Send)
}

// streamHttpSearchParamSync streams HTTP search param events to the client
func (h *HttpServiceRPC) streamHttpSearchParamSync(ctx context.Context, userID idwrap.IDWrap, send func(*apiv1.HttpSearchParamSyncResponse) error) error {
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

	converter := func(events []HttpSearchParamEvent) *apiv1.HttpSearchParamSyncResponse {
		var items []*apiv1.HttpSearchParamSync
		for _, event := range events {
			if event.IsDelta {
				continue
			}
			if resp := httpSearchParamSyncResponseFrom(event); resp != nil && len(resp.Items) > 0 {
				items = append(items, resp.Items...)
			}
		}
		if len(items) == 0 {
			return nil
		}
		return &apiv1.HttpSearchParamSyncResponse{Items: items}
	}

	return eventstream.StreamToClient(
		ctx,
		h.streamers.HttpSearchParam,
		nil,
		filter,
		converter,
		send,
		nil,
	)
}

// streamHttpAssertSync streams HTTP assert events to the client
func (h *HttpServiceRPC) streamHttpAssertSync(ctx context.Context, userID idwrap.IDWrap, send func(*apiv1.HttpAssertSyncResponse) error) error {
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

	converter := func(events []HttpAssertEvent) *apiv1.HttpAssertSyncResponse {
		var items []*apiv1.HttpAssertSync
		for _, event := range events {
			if event.IsDelta {
				continue
			}
			if resp := httpAssertSyncResponseFrom(event); resp != nil && len(resp.Items) > 0 {
				items = append(items, resp.Items...)
			}
		}
		if len(items) == 0 {
			return nil
		}
		return &apiv1.HttpAssertSyncResponse{Items: items}
	}

	return eventstream.StreamToClient(
		ctx,
		h.streamers.HttpAssert,
		nil,
		filter,
		converter,
		send,
		nil,
	)
}

// streamHttpVersionSync streams HTTP version events to the client
func (h *HttpServiceRPC) streamHttpVersionSync(ctx context.Context, userID idwrap.IDWrap, send func(*apiv1.HttpVersionSyncResponse) error) error {
	var workspaceSet sync.Map

	// Filter for workspace-based access control
	filter := func(topic HttpVersionTopic) bool {
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

	converter := func(events []HttpVersionEvent) *apiv1.HttpVersionSyncResponse {
		var items []*apiv1.HttpVersionSync
		for _, event := range events {
			if resp := httpVersionSyncResponseFrom(event); resp != nil && len(resp.Items) > 0 {
				items = append(items, resp.Items...)
			}
		}
		if len(items) == 0 {
			return nil
		}
		return &apiv1.HttpVersionSyncResponse{Items: items}
	}

	return eventstream.StreamToClient(
		ctx,
		h.streamers.HttpVersion,
		nil,
		filter,
		converter,
		send,
		nil,
	)
}

func (h *HttpServiceRPC) HttpAssertSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpAssertSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return h.streamHttpAssertSync(ctx, userID, stream.Send)
}

func (h *HttpServiceRPC) HttpResponseSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpResponseSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return h.streamHttpResponseSync(ctx, userID, stream.Send)
}

// streamHttpResponseSync streams HTTP response events to the client
func (h *HttpServiceRPC) streamHttpResponseSync(ctx context.Context, userID idwrap.IDWrap, send func(*apiv1.HttpResponseSyncResponse) error) error {
	var workspaceSet sync.Map

	// Filter for workspace-based access control
	filter := func(topic HttpResponseTopic) bool {
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

	converter := func(events []HttpResponseEvent) *apiv1.HttpResponseSyncResponse {
		var items []*apiv1.HttpResponseSync
		for _, event := range events {
			if resp := httpResponseSyncResponseFrom(event); resp != nil && len(resp.Items) > 0 {
				items = append(items, resp.Items...)
			}
		}
		if len(items) == 0 {
			return nil
		}
		return &apiv1.HttpResponseSyncResponse{Items: items}
	}

	return eventstream.StreamToClient(
		ctx,
		h.streamers.HttpResponse,
		nil,
		filter,
		converter,
		send,
		nil,
	)
}

func (h *HttpServiceRPC) HttpResponseHeaderSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpResponseHeaderSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return h.streamHttpResponseHeaderSync(ctx, userID, stream.Send)
}

// streamHttpResponseHeaderSync streams HTTP response header events to the client
func (h *HttpServiceRPC) streamHttpResponseHeaderSync(ctx context.Context, userID idwrap.IDWrap, send func(*apiv1.HttpResponseHeaderSyncResponse) error) error {
	var workspaceSet sync.Map

	// Filter for workspace-based access control
	filter := func(topic HttpResponseHeaderTopic) bool {
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

	converter := func(events []HttpResponseHeaderEvent) *apiv1.HttpResponseHeaderSyncResponse {
		var items []*apiv1.HttpResponseHeaderSync
		for _, event := range events {
			if resp := httpResponseHeaderSyncResponseFrom(event); resp != nil && len(resp.Items) > 0 {
				items = append(items, resp.Items...)
			}
		}
		if len(items) == 0 {
			return nil
		}
		return &apiv1.HttpResponseHeaderSyncResponse{Items: items}
	}

	return eventstream.StreamToClient(
		ctx,
		h.streamers.HttpResponseHeader,
		nil,
		filter,
		converter,
		send,
		nil,
	)
}

func (h *HttpServiceRPC) HttpResponseAssertSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpResponseAssertSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return h.streamHttpResponseAssertSync(ctx, userID, stream.Send)
}

// streamHttpResponseAssertSync streams HTTP response assert events to the client
func (h *HttpServiceRPC) streamHttpResponseAssertSync(ctx context.Context, userID idwrap.IDWrap, send func(*apiv1.HttpResponseAssertSyncResponse) error) error {
	var workspaceSet sync.Map

	// Filter for workspace-based access control
	filter := func(topic HttpResponseAssertTopic) bool {
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

	converter := func(events []HttpResponseAssertEvent) *apiv1.HttpResponseAssertSyncResponse {
		var items []*apiv1.HttpResponseAssertSync
		for _, event := range events {
			if resp := httpResponseAssertSyncResponseFrom(event); resp != nil && len(resp.Items) > 0 {
				items = append(items, resp.Items...)
			}
		}
		if len(items) == 0 {
			return nil
		}
		return &apiv1.HttpResponseAssertSyncResponse{Items: items}
	}

	return eventstream.StreamToClient(
		ctx,
		h.streamers.HttpResponseAssert,
		nil,
		filter,
		converter,
		send,
		nil,
	)
}

func (h *HttpServiceRPC) HttpHeaderSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpHeaderSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return h.streamHttpHeaderSync(ctx, userID, stream.Send)
}

// streamHttpHeaderSync streams HTTP header events to the client
func (h *HttpServiceRPC) streamHttpHeaderSync(ctx context.Context, userID idwrap.IDWrap, send func(*apiv1.HttpHeaderSyncResponse) error) error {
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

	converter := func(events []HttpHeaderEvent) *apiv1.HttpHeaderSyncResponse {
		var items []*apiv1.HttpHeaderSync
		for _, event := range events {
			if event.IsDelta {
				continue
			}
			if resp := httpHeaderSyncResponseFrom(event); resp != nil && len(resp.Items) > 0 {
				items = append(items, resp.Items...)
			}
		}
		if len(items) == 0 {
			return nil
		}
		return &apiv1.HttpHeaderSyncResponse{Items: items}
	}

	// Subscribe to events with snapshot
	return eventstream.StreamToClient(
		ctx,
		h.streamers.HttpHeader,
		nil,
		filter,
		converter,
		send,
		nil,
	)
}

func (h *HttpServiceRPC) HttpBodyFormDataSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpBodyFormDataSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return h.streamHttpBodyFormSync(ctx, userID, stream.Send)
}

func (h *HttpServiceRPC) streamHttpBodyFormSync(ctx context.Context, userID idwrap.IDWrap, send func(*apiv1.HttpBodyFormDataSyncResponse) error) error {
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

	converter := func(events []HttpBodyFormEvent) *apiv1.HttpBodyFormDataSyncResponse {
		var items []*apiv1.HttpBodyFormDataSync
		for _, event := range events {
			if event.IsDelta {
				continue
			}
			if resp := httpBodyFormDataSyncResponseFrom(event); resp != nil && len(resp.Items) > 0 {
				items = append(items, resp.Items...)
			}
		}
		if len(items) == 0 {
			return nil
		}
		return &apiv1.HttpBodyFormDataSyncResponse{Items: items}
	}

	return eventstream.StreamToClient(
		ctx,
		h.streamers.HttpBodyForm,
		nil,
		filter,
		converter,
		send,
		nil,
	)
}

func (h *HttpServiceRPC) HttpBodyUrlEncodedSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpBodyUrlEncodedSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return h.streamHttpBodyUrlEncodedSync(ctx, userID, stream.Send)
}

func (h *HttpServiceRPC) streamHttpBodyUrlEncodedSync(ctx context.Context, userID idwrap.IDWrap, send func(*apiv1.HttpBodyUrlEncodedSyncResponse) error) error {
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

	converter := func(events []HttpBodyUrlEncodedEvent) *apiv1.HttpBodyUrlEncodedSyncResponse {
		var items []*apiv1.HttpBodyUrlEncodedSync
		for _, event := range events {
			if event.IsDelta {
				continue
			}
			if resp := httpBodyUrlEncodedSyncResponseFrom(event); resp != nil && len(resp.Items) > 0 {
				items = append(items, resp.Items...)
			}
		}
		if len(items) == 0 {
			return nil
		}
		return &apiv1.HttpBodyUrlEncodedSyncResponse{Items: items}
	}

	return eventstream.StreamToClient(
		ctx,
		h.streamers.HttpBodyUrlEncoded,
		nil,
		filter,
		converter,
		send,
		nil,
	)
}

func (h *HttpServiceRPC) HttpBodyRawSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpBodyRawSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return h.streamHttpBodyRawSync(ctx, userID, stream.Send)
}

// streamHttpBodyRawSync streams HTTP body raw events to the client
func (h *HttpServiceRPC) streamHttpBodyRawSync(ctx context.Context, userID idwrap.IDWrap, send func(*apiv1.HttpBodyRawSyncResponse) error) error {
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

	converter := func(events []HttpBodyRawEvent) *apiv1.HttpBodyRawSyncResponse {
		var items []*apiv1.HttpBodyRawSync
		for _, event := range events {
			if event.IsDelta {
				continue
			}
			if resp := httpBodyRawSyncResponseFrom(event); resp != nil && len(resp.Items) > 0 {
				items = append(items, resp.Items...)
			}
		}
		if len(items) == 0 {
			return nil
		}
		return &apiv1.HttpBodyRawSyncResponse{Items: items}
	}

	return eventstream.StreamToClient(
		ctx,
		h.streamers.HttpBodyRaw,
		nil,
		filter,
		converter,
		send,
		nil,
	)
}
