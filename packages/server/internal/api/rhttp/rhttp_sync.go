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
	apiv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
)

func (h *HttpServiceRPC) publishInsertEvent(http mhttp.HTTP) {
	h.stream.Publish(HttpTopic{WorkspaceID: http.WorkspaceID}, HttpEvent{
		Type: eventTypeInsert,
		Http: converter.ToAPIHttp(http),
	})
}

// publishUpdateEvent publishes an update event for real-time sync
func (h *HttpServiceRPC) publishUpdateEvent(http mhttp.HTTP) {
	h.stream.Publish(HttpTopic{WorkspaceID: http.WorkspaceID}, HttpEvent{
		Type: eventTypeUpdate,
		Http: converter.ToAPIHttp(http),
	})
}

// publishDeleteEvent publishes a delete event for real-time sync
func (h *HttpServiceRPC) publishDeleteEvent(httpID, workspaceID idwrap.IDWrap) {
	h.stream.Publish(HttpTopic{WorkspaceID: workspaceID}, HttpEvent{
		Type: eventTypeDelete,
		Http: &apiv1.Http{
			HttpId: httpID.Bytes(),
		},
	})
}

// publishVersionInsertEvent publishes an insert event for real-time sync
func (h *HttpServiceRPC) publishVersionInsertEvent(version mhttp.HttpVersion, workspaceID idwrap.IDWrap) {
	h.httpVersionStream.Publish(HttpVersionTopic{WorkspaceID: workspaceID}, HttpVersionEvent{
		Type:        eventTypeInsert,
		HttpVersion: converter.ToAPIHttpVersion(version),
	})
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

	return eventstream.StreamToClient(
		ctx,
		h.stream,
		nil,
		filter,
		httpSyncResponseFrom,
		send,
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

	return eventstream.StreamToClient(
		ctx,
		h.httpSearchParamStream,
		nil,
		filter,
		httpSearchParamSyncResponseFrom,
		send,
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

	return eventstream.StreamToClient(
		ctx,
		h.httpAssertStream,
		nil,
		filter,
		httpAssertSyncResponseFrom,
		send,
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

	return eventstream.StreamToClient(
		ctx,
		h.httpVersionStream,
		nil,
		filter,
		httpVersionSyncResponseFrom,
		send,
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

	return eventstream.StreamToClient(
		ctx,
		h.httpResponseStream,
		nil,
		filter,
		httpResponseSyncResponseFrom,
		send,
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

	return eventstream.StreamToClient(
		ctx,
		h.httpResponseHeaderStream,
		nil,
		filter,
		httpResponseHeaderSyncResponseFrom,
		send,
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

	return eventstream.StreamToClient(
		ctx,
		h.httpResponseAssertStream,
		nil,
		filter,
		httpResponseAssertSyncResponseFrom,
		send,
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

	// Subscribe to events with snapshot
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
			resp := httpHeaderSyncResponseFrom(evt.Payload)
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

	return eventstream.StreamToClient(
		ctx,
		h.httpBodyFormStream,
		nil,
		filter,
		httpBodyFormDataSyncResponseFrom,
		send,
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

	return eventstream.StreamToClient(
		ctx,
		h.httpBodyUrlEncodedStream,
		nil,
		filter,
		httpBodyUrlEncodedSyncResponseFrom,
		send,
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

	return eventstream.StreamToClient(
		ctx,
		h.httpBodyRawStream,
		nil,
		filter,
		httpBodyRawSyncResponseFrom,
		send,
	)
}

// Helper methods for HTTP request execution

// parseHttpMethod converts string method to HttpMethod enum
