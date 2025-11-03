package rhttp

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"strings"
	"sync"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rworkspace"
	"the-dev-tools/server/pkg/eventstream"
	"the-dev-tools/server/pkg/httpclient"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mexamplequery"
	"the-dev-tools/server/pkg/model/mexampleresp"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mhttpassert"
	"the-dev-tools/server/pkg/model/mhttpbodyform"
	"the-dev-tools/server/pkg/model/mhttpbodyurlencoded"
	"the-dev-tools/server/pkg/model/mhttpheader"
	"the-dev-tools/server/pkg/model/mhttpsearchparam"
	"the-dev-tools/server/pkg/model/mworkspaceuser"
	"the-dev-tools/server/pkg/service/sbodyraw"
	"the-dev-tools/server/pkg/service/sexampleheader"
	"the-dev-tools/server/pkg/service/sexamplequery"
	"the-dev-tools/server/pkg/service/sexampleresp"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/shttpassert"
	"the-dev-tools/server/pkg/service/shttpbodyform"
	"the-dev-tools/server/pkg/service/shttpbodyurlencoded"
	"the-dev-tools/server/pkg/service/shttpheader"
	"the-dev-tools/server/pkg/service/shttpsearchparam"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/service/sworkspacesusers"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
	httpv1connect "the-dev-tools/spec/dist/buf/go/api/http/v1/httpv1connect"
)

const (
	eventTypeCreate = "create"
	eventTypeUpdate = "update"
	eventTypeDelete = "delete"
)

// isForeignKeyConstraintError checks if the error is a foreign key constraint violation
func isForeignKeyConstraintError(err error) bool {
	if err == nil {
		return false
	}

	// SQLite foreign key constraint error patterns
	errStr := err.Error()
	return contains(errStr, "FOREIGN KEY constraint failed") ||
		contains(errStr, "foreign key constraint") ||
		contains(errStr, "constraint violation")
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					containsSubstring(s, substr)))
}

// containsSubstring performs a simple substring search
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// HttpTopic defines the streaming topic for HTTP events
type HttpTopic struct {
	WorkspaceID idwrap.IDWrap
}

// HttpEvent defines the event payload for HTTP streaming
type HttpEvent struct {
	Type string
	Http *apiv1.Http
}

// HttpHeaderTopic defines the streaming topic for HTTP header events
type HttpHeaderTopic struct {
	WorkspaceID idwrap.IDWrap
}

// HttpHeaderEvent defines the event payload for HTTP header streaming
type HttpHeaderEvent struct {
	Type       string
	HttpHeader *apiv1.HttpHeader
}

// HttpSearchParamTopic defines the streaming topic for HTTP search param events
type HttpSearchParamTopic struct {
	WorkspaceID idwrap.IDWrap
}

// HttpSearchParamEvent defines the event payload for HTTP search param streaming
type HttpSearchParamEvent struct {
	Type            string
	HttpSearchParam *apiv1.HttpSearchParam
}

// HttpBodyFormTopic defines the streaming topic for HTTP body form events
type HttpBodyFormTopic struct {
	WorkspaceID idwrap.IDWrap
}

// HttpBodyFormEvent defines the event payload for HTTP body form streaming
type HttpBodyFormEvent struct {
	Type         string
	HttpBodyForm *apiv1.HttpBodyForm
}

// HttpBodyUrlEncodedTopic defines the streaming topic for HTTP body URL encoded events
type HttpBodyUrlEncodedTopic struct {
	WorkspaceID idwrap.IDWrap
}

// HttpBodyUrlEncodedEvent defines the event payload for HTTP body URL encoded streaming
type HttpBodyUrlEncodedEvent struct {
	Type               string
	HttpBodyUrlEncoded *apiv1.HttpBodyUrlEncoded
}

// HttpAssertTopic defines the streaming topic for HTTP assert events
type HttpAssertTopic struct {
	WorkspaceID idwrap.IDWrap
}

// HttpAssertEvent defines the event payload for HTTP assert streaming
type HttpAssertEvent struct {
	Type       string
	HttpAssert *apiv1.HttpAssert
}

// HttpServiceRPC handles HTTP RPC operations with streaming support
type HttpServiceRPC struct {
	DB *sql.DB

	hs  shttp.HTTPService
	us  suser.UserService
	ws  sworkspace.WorkspaceService
	wus sworkspacesusers.WorkspaceUserService

	// Additional services for HTTP components
	headerService sexampleheader.HeaderService
	queryService  sexamplequery.ExampleQueryService
	bodyService   sbodyraw.BodyRawService
	respService   sexampleresp.ExampleRespService

	// Child entity services
	httpHeaderService         shttpheader.HttpHeaderService
	httpSearchParamService    shttpsearchparam.HttpSearchParamService
	httpBodyFormService       shttpbodyform.HttpBodyFormService
	httpBodyUrlEncodedService shttpbodyurlencoded.HttpBodyUrlEncodedService
	httpAssertService         shttpassert.HttpAssertService

	// Streamers for child entities
	stream                   eventstream.SyncStreamer[HttpTopic, HttpEvent]
	httpHeaderStream         eventstream.SyncStreamer[HttpHeaderTopic, HttpHeaderEvent]
	httpSearchParamStream    eventstream.SyncStreamer[HttpSearchParamTopic, HttpSearchParamEvent]
	httpBodyFormStream       eventstream.SyncStreamer[HttpBodyFormTopic, HttpBodyFormEvent]
	httpBodyUrlEncodedStream eventstream.SyncStreamer[HttpBodyUrlEncodedTopic, HttpBodyUrlEncodedEvent]
	httpAssertStream         eventstream.SyncStreamer[HttpAssertTopic, HttpAssertEvent]
}

// New creates a new HttpServiceRPC instance
func New(
	db *sql.DB,
	hs shttp.HTTPService,
	us suser.UserService,
	ws sworkspace.WorkspaceService,
	wus sworkspacesusers.WorkspaceUserService,
	headerService sexampleheader.HeaderService,
	queryService sexamplequery.ExampleQueryService,
	bodyService sbodyraw.BodyRawService,
	respService sexampleresp.ExampleRespService,
	httpHeaderService shttpheader.HttpHeaderService,
	httpSearchParamService shttpsearchparam.HttpSearchParamService,
	httpBodyFormService shttpbodyform.HttpBodyFormService,
	httpBodyUrlEncodedService shttpbodyurlencoded.HttpBodyUrlEncodedService,
	httpAssertService shttpassert.HttpAssertService,
	stream eventstream.SyncStreamer[HttpTopic, HttpEvent],
	httpHeaderStream eventstream.SyncStreamer[HttpHeaderTopic, HttpHeaderEvent],
	httpSearchParamStream eventstream.SyncStreamer[HttpSearchParamTopic, HttpSearchParamEvent],
	httpBodyFormStream eventstream.SyncStreamer[HttpBodyFormTopic, HttpBodyFormEvent],
	httpBodyUrlEncodedStream eventstream.SyncStreamer[HttpBodyUrlEncodedTopic, HttpBodyUrlEncodedEvent],
	httpAssertStream eventstream.SyncStreamer[HttpAssertTopic, HttpAssertEvent],
) HttpServiceRPC {
	return HttpServiceRPC{
		DB:                        db,
		hs:                        hs,
		us:                        us,
		ws:                        ws,
		wus:                       wus,
		headerService:             headerService,
		queryService:              queryService,
		bodyService:               bodyService,
		respService:               respService,
		httpHeaderService:         httpHeaderService,
		httpSearchParamService:    httpSearchParamService,
		httpBodyFormService:       httpBodyFormService,
		httpBodyUrlEncodedService: httpBodyUrlEncodedService,
		httpAssertService:         httpAssertService,
		stream:                    stream,
		httpHeaderStream:          httpHeaderStream,
		httpSearchParamStream:     httpSearchParamStream,
		httpBodyFormStream:        httpBodyFormStream,
		httpBodyUrlEncodedStream:  httpBodyUrlEncodedStream,
		httpAssertStream:          httpAssertStream,
	}
}

// CreateService creates the HTTP service with Connect handler
func CreateService(srv HttpServiceRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := httpv1connect.NewHttpServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

// toAPIHttp converts model HTTP to API HTTP
func toAPIHttp(http mhttp.HTTP) *apiv1.Http {
	apiHttp := &apiv1.Http{
		HttpId:   http.ID.Bytes(),
		Name:     http.Name,
		Url:      http.Url,
		Method:   toAPIHttpMethod(http.Method),
		BodyKind: apiv1.HttpBodyKind_HTTP_BODY_KIND_UNSPECIFIED, // Default value
	}

	if http.FolderID != nil {
		// Note: FolderId field may need to be added to API proto if not present
		// apiHttp.FolderId = http.FolderID.Bytes()
	}

	return apiHttp
}

// toAPIHttpMethod converts string method to API HttpMethod
func toAPIHttpMethod(method string) apiv1.HttpMethod {
	switch method {
	case "GET":
		return apiv1.HttpMethod_HTTP_METHOD_GET
	case "POST":
		return apiv1.HttpMethod_HTTP_METHOD_POST
	case "PUT":
		return apiv1.HttpMethod_HTTP_METHOD_PUT
	case "PATCH":
		return apiv1.HttpMethod_HTTP_METHOD_PATCH
	case "DELETE":
		return apiv1.HttpMethod_HTTP_METHOD_DELETE
	case "HEAD":
		return apiv1.HttpMethod_HTTP_METHOD_HEAD
	case "OPTION":
		return apiv1.HttpMethod_HTTP_METHOD_OPTION
	case "CONNECT":
		return apiv1.HttpMethod_HTTP_METHOD_CONNECT
	default:
		return apiv1.HttpMethod_HTTP_METHOD_UNSPECIFIED
	}
}

// toAPIHttpHeader converts model HttpHeader to API HttpHeader
func toAPIHttpHeader(header mhttpheader.HttpHeader) *apiv1.HttpHeader {
	return &apiv1.HttpHeader{
		HttpHeaderId: header.ID.Bytes(),
		HttpId:       header.HttpID.Bytes(),
		Key:          header.Key,
		Value:        header.Value,
		Enabled:      header.Enabled,
		Description:  header.Description,
		Order:        float32(header.Order),
	}
}

// toAPIHttpSearchParam converts model HttpSearchParam to API HttpSearchParam
func toAPIHttpSearchParam(param mhttpsearchparam.HttpSearchParam) *apiv1.HttpSearchParam {
	return &apiv1.HttpSearchParam{
		HttpSearchParamId: param.ID.Bytes(),
		HttpId:            param.HttpID.Bytes(),
		Key:               param.Key,
		Value:             param.Value,
		Enabled:           param.Enabled,
		Description:       param.Description,
		Order:             float32(param.Order),
	}
}

// toAPIHttpBodyForm converts model HttpBodyForm to API HttpBodyForm
func toAPIHttpBodyForm(form mhttpbodyform.HttpBodyForm) *apiv1.HttpBodyForm {
	return &apiv1.HttpBodyForm{
		HttpBodyFormId: form.ID.Bytes(),
		HttpId:         form.HttpID.Bytes(),
		Key:            form.Key,
		Value:          form.Value,
		Enabled:        form.Enabled,
		Description:    form.Description,
	}
}

// toAPIHttpBodyUrlEncoded converts model HttpBodyUrlEncoded to API HttpBodyUrlEncoded
func toAPIHttpBodyUrlEncoded(urlEncoded mhttpbodyurlencoded.HttpBodyUrlEncoded) *apiv1.HttpBodyUrlEncoded {
	return &apiv1.HttpBodyUrlEncoded{
		HttpBodyUrlEncodedId: urlEncoded.ID.Bytes(),
		HttpId:               urlEncoded.HttpID.Bytes(),
		Key:                  urlEncoded.Key,
		Value:                urlEncoded.Value,
		Enabled:              urlEncoded.Enabled,
		Description:          urlEncoded.Description,
	}
}

// toAPIHttpAssert converts model HttpAssert to API HttpAssert
func toAPIHttpAssert(assert mhttpassert.HttpAssert) *apiv1.HttpAssert {
	return &apiv1.HttpAssert{
		HttpAssertId: assert.ID.Bytes(),
		HttpId:       assert.HttpID.Bytes(),
		Value:        assert.Value,
	}
}

// fromAPIHttpMethod converts API HttpMethod to string
func fromAPIHttpMethod(method apiv1.HttpMethod) string {
	switch method {
	case apiv1.HttpMethod_HTTP_METHOD_GET:
		return "GET"
	case apiv1.HttpMethod_HTTP_METHOD_POST:
		return "POST"
	case apiv1.HttpMethod_HTTP_METHOD_PUT:
		return "PUT"
	case apiv1.HttpMethod_HTTP_METHOD_PATCH:
		return "PATCH"
	case apiv1.HttpMethod_HTTP_METHOD_DELETE:
		return "DELETE"
	case apiv1.HttpMethod_HTTP_METHOD_HEAD:
		return "HEAD"
	case apiv1.HttpMethod_HTTP_METHOD_OPTION:
		return "OPTION"
	case apiv1.HttpMethod_HTTP_METHOD_CONNECT:
		return "CONNECT"
	default:
		return ""
	}
}

// publishCreateEvent publishes a create event for real-time sync
func (h *HttpServiceRPC) publishCreateEvent(http mhttp.HTTP) {
	h.stream.Publish(HttpTopic{WorkspaceID: http.WorkspaceID}, HttpEvent{
		Type: eventTypeCreate,
		Http: toAPIHttp(http),
	})
}

// publishUpdateEvent publishes an update event for real-time sync
func (h *HttpServiceRPC) publishUpdateEvent(http mhttp.HTTP) {
	h.stream.Publish(HttpTopic{WorkspaceID: http.WorkspaceID}, HttpEvent{
		Type: eventTypeUpdate,
		Http: toAPIHttp(http),
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

// listUserHttp retrieves all HTTP entries accessible to the user
func (h *HttpServiceRPC) listUserHttp(ctx context.Context) ([]mhttp.HTTP, error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, err
	}

	// Get user's workspaces
	workspaces, err := h.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, err
	}

	var allHttp []mhttp.HTTP
	for _, workspace := range workspaces {
		httpList, err := h.hs.GetByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, err
		}
		allHttp = append(allHttp, httpList...)
	}

	return allHttp, nil
}

// httpSyncResponseFrom converts HttpEvent to HttpSync response
func httpSyncResponseFrom(event HttpEvent) *apiv1.HttpSyncResponse {
	var value *apiv1.HttpSync_ValueUnion

	switch event.Type {
	case eventTypeCreate:
		name := event.Http.GetName()
		method := event.Http.GetMethod()
		url := event.Http.GetUrl()
		bodyKind := event.Http.GetBodyKind()
		value = &apiv1.HttpSync_ValueUnion{
			Kind: apiv1.HttpSync_ValueUnion_KIND_CREATE,
			Create: &apiv1.HttpSyncCreate{
				HttpId:   event.Http.GetHttpId(),
				Name:     name,
				Method:   method,
				Url:      url,
				BodyKind: bodyKind,
			},
		}
	case eventTypeUpdate:
		name := event.Http.GetName()
		method := event.Http.GetMethod()
		url := event.Http.GetUrl()
		bodyKind := event.Http.GetBodyKind()
		value = &apiv1.HttpSync_ValueUnion{
			Kind: apiv1.HttpSync_ValueUnion_KIND_UPDATE,
			Update: &apiv1.HttpSyncUpdate{
				HttpId:   event.Http.GetHttpId(),
				Name:     &name,
				Method:   &method,
				Url:      &url,
				BodyKind: &bodyKind,
			},
		}
	case eventTypeDelete:
		value = &apiv1.HttpSync_ValueUnion{
			Kind: apiv1.HttpSync_ValueUnion_KIND_DELETE,
			Delete: &apiv1.HttpSyncDelete{
				HttpId: event.Http.GetHttpId(),
			},
		}
	}

	return &apiv1.HttpSyncResponse{
		Items: []*apiv1.HttpSync{
			{
				Value: value,
			},
		},
	}
}

// httpHeaderSyncResponseFrom converts HttpHeaderEvent to HttpHeaderSync response
func httpHeaderSyncResponseFrom(event HttpHeaderEvent) *apiv1.HttpHeaderSyncResponse {
	var value *apiv1.HttpHeaderSync_ValueUnion

	switch event.Type {
	case eventTypeCreate:
		key := event.HttpHeader.GetKey()
		value_ := event.HttpHeader.GetValue()
		enabled := event.HttpHeader.GetEnabled()
		description := event.HttpHeader.GetDescription()
		order := event.HttpHeader.GetOrder()
		value = &apiv1.HttpHeaderSync_ValueUnion{
			Kind: apiv1.HttpHeaderSync_ValueUnion_KIND_CREATE,
			Create: &apiv1.HttpHeaderSyncCreate{
				HttpHeaderId: event.HttpHeader.GetHttpHeaderId(),
				HttpId:       event.HttpHeader.GetHttpId(),
				Key:          key,
				Value:        value_,
				Enabled:      enabled,
				Description:  description,
				Order:        order,
			},
		}
	case eventTypeUpdate:
		key := event.HttpHeader.GetKey()
		value_ := event.HttpHeader.GetValue()
		enabled := event.HttpHeader.GetEnabled()
		description := event.HttpHeader.GetDescription()
		order := event.HttpHeader.GetOrder()
		value = &apiv1.HttpHeaderSync_ValueUnion{
			Kind: apiv1.HttpHeaderSync_ValueUnion_KIND_UPDATE,
			Update: &apiv1.HttpHeaderSyncUpdate{
				HttpHeaderId: event.HttpHeader.GetHttpHeaderId(),
				Key:          &key,
				Value:        &value_,
				Enabled:      &enabled,
				Description:  &description,
				Order:        &order,
			},
		}
	case eventTypeDelete:
		value = &apiv1.HttpHeaderSync_ValueUnion{
			Kind: apiv1.HttpHeaderSync_ValueUnion_KIND_DELETE,
			Delete: &apiv1.HttpHeaderSyncDelete{
				HttpHeaderId: event.HttpHeader.GetHttpHeaderId(),
			},
		}
	}

	return &apiv1.HttpHeaderSyncResponse{
		Items: []*apiv1.HttpHeaderSync{
			{
				Value: value,
			},
		},
	}
}

// httpSearchParamSyncResponseFrom converts HttpSearchParamEvent to HttpSearchParamSync response
func httpSearchParamSyncResponseFrom(event HttpSearchParamEvent) *apiv1.HttpSearchParamSyncResponse {
	var value *apiv1.HttpSearchParamSync_ValueUnion

	switch event.Type {
	case eventTypeCreate:
		key := event.HttpSearchParam.GetKey()
		value_ := event.HttpSearchParam.GetValue()
		enabled := event.HttpSearchParam.GetEnabled()
		description := event.HttpSearchParam.GetDescription()
		order := event.HttpSearchParam.GetOrder()
		value = &apiv1.HttpSearchParamSync_ValueUnion{
			Kind: apiv1.HttpSearchParamSync_ValueUnion_KIND_CREATE,
			Create: &apiv1.HttpSearchParamSyncCreate{
				HttpSearchParamId: event.HttpSearchParam.GetHttpSearchParamId(),
				HttpId:            event.HttpSearchParam.GetHttpId(),
				Key:               key,
				Value:             value_,
				Enabled:           enabled,
				Description:       description,
				Order:             order,
			},
		}
	case eventTypeUpdate:
		key := event.HttpSearchParam.GetKey()
		value_ := event.HttpSearchParam.GetValue()
		enabled := event.HttpSearchParam.GetEnabled()
		description := event.HttpSearchParam.GetDescription()
		order := event.HttpSearchParam.GetOrder()
		value = &apiv1.HttpSearchParamSync_ValueUnion{
			Kind: apiv1.HttpSearchParamSync_ValueUnion_KIND_UPDATE,
			Update: &apiv1.HttpSearchParamSyncUpdate{
				HttpSearchParamId: event.HttpSearchParam.GetHttpSearchParamId(),
				Key:               &key,
				Value:             &value_,
				Enabled:           &enabled,
				Description:       &description,
				Order:             &order,
			},
		}
	case eventTypeDelete:
		value = &apiv1.HttpSearchParamSync_ValueUnion{
			Kind: apiv1.HttpSearchParamSync_ValueUnion_KIND_DELETE,
			Delete: &apiv1.HttpSearchParamSyncDelete{
				HttpSearchParamId: event.HttpSearchParam.GetHttpSearchParamId(),
			},
		}
	}

	return &apiv1.HttpSearchParamSyncResponse{
		Items: []*apiv1.HttpSearchParamSync{
			{
				Value: value,
			},
		},
	}
}

// httpAssertSyncResponseFrom converts HttpAssertEvent to HttpAssertSync response
func httpAssertSyncResponseFrom(event HttpAssertEvent) *apiv1.HttpAssertSyncResponse {
	var value *apiv1.HttpAssertSync_ValueUnion

	switch event.Type {
	case eventTypeCreate:
		value_ := event.HttpAssert.GetValue()
		value = &apiv1.HttpAssertSync_ValueUnion{
			Kind: apiv1.HttpAssertSync_ValueUnion_KIND_CREATE,
			Create: &apiv1.HttpAssertSyncCreate{
				HttpAssertId: event.HttpAssert.GetHttpAssertId(),
				HttpId:       event.HttpAssert.GetHttpId(),
				Value:        value_,
			},
		}
	case eventTypeUpdate:
		value_ := event.HttpAssert.GetValue()
		value = &apiv1.HttpAssertSync_ValueUnion{
			Kind: apiv1.HttpAssertSync_ValueUnion_KIND_UPDATE,
			Update: &apiv1.HttpAssertSyncUpdate{
				HttpAssertId: event.HttpAssert.GetHttpAssertId(),
				Value:        &value_,
			},
		}
	case eventTypeDelete:
		value = &apiv1.HttpAssertSync_ValueUnion{
			Kind: apiv1.HttpAssertSync_ValueUnion_KIND_DELETE,
			Delete: &apiv1.HttpAssertSyncDelete{
				HttpAssertId: event.HttpAssert.GetHttpAssertId(),
			},
		}
	}

	return &apiv1.HttpAssertSyncResponse{
		Items: []*apiv1.HttpAssertSync{
			{
				Value: value,
			},
		},
	}
}

// HttpSync handles real-time synchronization for HTTP entries
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

	// Snapshot provider for initial state
	snapshot := func(ctx context.Context) ([]eventstream.Event[HttpTopic, HttpEvent], error) {
		httpList, err := h.listUserHttp(ctx)
		if err != nil {
			return nil, err
		}

		events := make([]eventstream.Event[HttpTopic, HttpEvent], 0, len(httpList))
		for _, http := range httpList {
			workspaceSet.Store(http.WorkspaceID.String(), struct{}{})
			events = append(events, eventstream.Event[HttpTopic, HttpEvent]{
				Topic: HttpTopic{WorkspaceID: http.WorkspaceID},
				Payload: HttpEvent{
					Type: eventTypeCreate,
					Http: toAPIHttp(http),
				},
			})
		}
		return events, nil
	}

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

	// Subscribe to events with snapshot
	events, err := h.stream.Subscribe(ctx, filter, eventstream.WithSnapshot(snapshot))
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
			resp := httpSyncResponseFrom(evt.Payload)
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

// CheckOwnerHttp verifies if a user owns an HTTP entry via workspace membership
func CheckOwnerHttp(ctx context.Context, hs shttp.HTTPService, us suser.UserService, httpID idwrap.IDWrap) (bool, error) {
	workspaceID, err := hs.GetWorkspaceID(ctx, httpID)
	if err != nil {
		return false, err
	}
	return rworkspace.CheckOwnerWorkspace(ctx, us, workspaceID)
}

// checkWorkspaceReadAccess verifies if user has read access to workspace (any role)
func (h *HttpServiceRPC) checkWorkspaceReadAccess(ctx context.Context, workspaceID idwrap.IDWrap) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	wsUser, err := h.wus.GetWorkspaceUsersByWorkspaceIDAndUserID(ctx, workspaceID, userID)
	if err != nil {
		if errors.Is(err, sworkspacesusers.ErrWorkspaceUserNotFound) {
			return connect.NewError(connect.CodeNotFound, errors.New("workspace not found or access denied"))
		}
		return connect.NewError(connect.CodeInternal, err)
	}

	// Any role provides read access
	if wsUser.Role < mworkspaceuser.RoleUser {
		return connect.NewError(connect.CodePermissionDenied, errors.New("permission denied"))
	}

	return nil
}

// checkWorkspaceWriteAccess verifies if user has write access to workspace (Admin or Owner)
func (h *HttpServiceRPC) checkWorkspaceWriteAccess(ctx context.Context, workspaceID idwrap.IDWrap) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	wsUser, err := h.wus.GetWorkspaceUsersByWorkspaceIDAndUserID(ctx, workspaceID, userID)
	if err != nil {
		if errors.Is(err, sworkspacesusers.ErrWorkspaceUserNotFound) {
			return connect.NewError(connect.CodeNotFound, errors.New("workspace not found or access denied"))
		}
		return connect.NewError(connect.CodeInternal, err)
	}

	// Write access requires Admin or Owner role
	if wsUser.Role < mworkspaceuser.RoleAdmin {
		return connect.NewError(connect.CodePermissionDenied, errors.New("permission denied"))
	}

	return nil
}

// checkWorkspaceDeleteAccess verifies if user has delete access to workspace (Owner only)
func (h *HttpServiceRPC) checkWorkspaceDeleteAccess(ctx context.Context, workspaceID idwrap.IDWrap) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	wsUser, err := h.wus.GetWorkspaceUsersByWorkspaceIDAndUserID(ctx, workspaceID, userID)
	if err != nil {
		if errors.Is(err, sworkspacesusers.ErrWorkspaceUserNotFound) {
			return connect.NewError(connect.CodeNotFound, errors.New("workspace not found or access denied"))
		}
		return connect.NewError(connect.CodeInternal, err)
	}

	// Delete access requires Owner role only
	if wsUser.Role != mworkspaceuser.RoleOwner {
		return connect.NewError(connect.CodePermissionDenied, errors.New("permission denied"))
	}

	return nil
}

// executeHTTPRequest performs the actual HTTP request execution
func (h *HttpServiceRPC) executeHTTPRequest(ctx context.Context, httpEntry *mhttp.HTTP) error {
	// Create HTTP client with timeout
	client := httpclient.New()

	// Load HTTP headers, queries, and body from related services
	headers, err := h.loadHttpHeaders(ctx, httpEntry.ID)
	if err != nil {
		log.Printf("Failed to load HTTP headers: %v", err)
		// Continue with empty headers rather than failing
		headers = []mexampleheader.Header{}
	}

	queries, err := h.loadHttpQueries(ctx, httpEntry.ID)
	if err != nil {
		log.Printf("Failed to load HTTP queries: %v", err)
		// Continue with empty queries rather than failing
		queries = []mexamplequery.Query{}
	}

	body, err := h.loadHttpBody(ctx, httpEntry.ID)
	if err != nil {
		log.Printf("Failed to load HTTP body: %v", err)
		// Continue with empty body rather than failing
		body = nil
	}

	// Prepare the HTTP request using existing utilities
	httpReq := &httpclient.Request{
		Method:  httpEntry.Method,
		URL:     httpEntry.Url,
		Body:    body,
		Headers: headers,
		Queries: queries,
	}

	// Execute the request with context and convert to Response struct
	httpResp, err := httpclient.SendRequestAndConvertWithContext(ctx, client, httpReq, httpEntry.ID)
	if err != nil {
		// Handle different types of HTTP errors with proper Connect error codes
		if netErr, ok := err.(net.Error); ok {
			if netErr.Timeout() {
				return connect.NewError(connect.CodeDeadlineExceeded, fmt.Errorf("request timeout: %w", err))
			}
			if netErr.Temporary() {
				return connect.NewError(connect.CodeUnavailable, fmt.Errorf("temporary network error: %w", err))
			}
		}

		// Handle DNS resolution errors
		if strings.Contains(err.Error(), "no such host") || strings.Contains(err.Error(), "dns") {
			return connect.NewError(connect.CodeUnavailable, fmt.Errorf("DNS resolution failed: %w", err))
		}

		// Handle connection refused errors
		if strings.Contains(err.Error(), "connection refused") {
			return connect.NewError(connect.CodeUnavailable, fmt.Errorf("connection refused: %w", err))
		}

		// Handle SSL/TLS errors
		if strings.Contains(err.Error(), "certificate") || strings.Contains(err.Error(), "tls") {
			return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("TLS/SSL error: %w", err))
		}

		// Generic HTTP execution error
		return connect.NewError(connect.CodeInternal, fmt.Errorf("HTTP request failed: %w", err))
	}

	// Store HTTP response in database
	if err := h.storeHttpResponse(ctx, httpEntry.ID, httpResp); err != nil {
		// Log error but don't fail the request
		log.Printf("Failed to store HTTP response: %v", err)
	}

	// TODO: Load and evaluate assertions when assertion system is implemented
	// For now, we skip assertion evaluation

	return nil
}

// isNetworkError checks if the error is a network-related error
func isNetworkError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	return strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "network is unreachable") ||
		strings.Contains(errStr, "no such host") ||
		isDNSError(err)
}

// isTimeoutError checks if the error is a timeout error
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "deadline exceeded") ||
		errors.Is(err, context.DeadlineExceeded)
}

// isDNSError checks if the error is a DNS resolution error
func isDNSError(err error) bool {
	if err == nil {
		return false
	}

	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		var netErr *net.DNSError
		if errors.As(urlErr.Err, &netErr) {
			return true
		}
	}

	errStr := err.Error()
	return strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "name resolution failed")
}

// Stub methods to satisfy HttpServiceHandler interface
// These will be implemented in future phases

func (h *HttpServiceRPC) HttpCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpCollectionResponse], error) {
	_, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	httpList, err := h.listUserHttp(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	items := make([]*apiv1.Http, 0, len(httpList))
	for _, http := range httpList {
		items = append(items, toAPIHttp(http))
	}

	return connect.NewResponse(&apiv1.HttpCollectionResponse{Items: items}), nil
}

func (h *HttpServiceRPC) HttpCreate(ctx context.Context, req *connect.Request[apiv1.HttpCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP entry must be provided"))
	}

	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	hsService := h.hs.TX(tx)

	var createdHTTPs []mhttp.HTTP

	for _, item := range req.Msg.Items {
		if len(item.HttpId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_id is required"))
		}

		httpID, err := idwrap.NewFromBytes(item.HttpId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// For now, we'll use the first workspace the user has access to
		// In a real implementation, workspace_id should be in the API request
		workspaces, err := h.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if len(workspaces) == 0 {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("user has no workspaces"))
		}

		// Check write access to the workspace (Admin or Owner role required)
		if err := h.checkWorkspaceWriteAccess(ctx, workspaces[0].ID); err != nil {
			return nil, err
		}

		// Create the HTTP entry
		httpModel := &mhttp.HTTP{
			ID:          httpID,
			WorkspaceID: workspaces[0].ID,
			Name:        item.Name,
			Url:         item.Url,
			Method:      fromAPIHttpMethod(item.Method),
			Description: "", // Description field not available in API yet
		}

		if err := hsService.Create(ctx, httpModel); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		createdHTTPs = append(createdHTTPs, *httpModel)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish create events for real-time sync after successful commit
	for _, http := range createdHTTPs {
		h.publishCreateEvent(http)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpUpdate(ctx context.Context, req *connect.Request[apiv1.HttpUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP entry must be provided"))
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	hsService := h.hs.TX(tx)

	var updatedHTTPs []mhttp.HTTP

	for _, item := range req.Msg.Items {
		if len(item.HttpId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_id is required"))
		}

		httpID, err := idwrap.NewFromBytes(item.HttpId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing HTTP entry within transaction for consistency
		existingHttp, err := hsService.Get(ctx, httpID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHTTPFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Check write access (Admin or Owner role required)
		if err := h.checkWorkspaceWriteAccess(ctx, existingHttp.WorkspaceID); err != nil {
			return nil, err
		}

		// Update fields if provided - use transaction-scoped service
		if item.Name != nil {
			existingHttp.Name = *item.Name
		}
		if item.Url != nil {
			existingHttp.Url = *item.Url
		}
		if item.Method != nil {
			existingHttp.Method = fromAPIHttpMethod(*item.Method)
		}
		if item.BodyKind != nil {
			// Note: BodyKind is not currently in the mhttp.HTTP model
			// This would need to be added to the model and database schema
		}

		if err := hsService.Update(ctx, existingHttp); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		updatedHTTPs = append(updatedHTTPs, *existingHttp)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish update events for real-time sync after successful commit
	for _, http := range updatedHTTPs {
		h.publishUpdateEvent(http)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpDelete(ctx context.Context, req *connect.Request[apiv1.HttpDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP entry must be provided"))
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	hsService := h.hs.TX(tx)

	var deletedIDs []idwrap.IDWrap
	var deletedWorkspaceIDs []idwrap.IDWrap

	for _, item := range req.Msg.Items {
		if len(item.HttpId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_id is required"))
		}

		httpID, err := idwrap.NewFromBytes(item.HttpId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing HTTP entry within transaction for consistency
		existingHttp, err := hsService.Get(ctx, httpID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHTTPFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Check delete access (Owner role only)
		if err := h.checkWorkspaceDeleteAccess(ctx, existingHttp.WorkspaceID); err != nil {
			return nil, err
		}

		// Perform cascade delete - the database schema should handle foreign key constraints
		// This includes: http_search_param, http_header, http_body_form, http_body_urlencoded,
		// http_body_raw, http_assert, http_response, etc.
		if err := hsService.Delete(ctx, httpID); err != nil {
			// Handle foreign key constraint violations gracefully
			if isForeignKeyConstraintError(err) {
				return nil, connect.NewError(connect.CodeFailedPrecondition,
					errors.New("cannot delete HTTP entry with dependent records"))
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		deletedIDs = append(deletedIDs, httpID)
		deletedWorkspaceIDs = append(deletedWorkspaceIDs, existingHttp.WorkspaceID)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish delete events for real-time sync after successful commit
	for i, httpID := range deletedIDs {
		h.publishDeleteEvent(httpID, deletedWorkspaceIDs[i])
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

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
		httpList, err := h.hs.GetByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Convert to delta format
		for _, http := range httpList {
			delta := &apiv1.HttpDelta{
				HttpId: http.ID.Bytes(),
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

func (h *HttpServiceRPC) HttpDeltaCreate(ctx context.Context, req *connect.Request[apiv1.HttpDeltaCreateRequest]) (*connect.Response[emptypb.Empty], error) {
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

		// Create delta HTTP entry
		deltaHttp := &mhttp.HTTP{
			ID:           idwrap.NewNow(),
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
			DeltaMethod:  httpMethodToString(item.Method),
			CreatedAt:    0, // Will be set by service
			UpdatedAt:    0, // Will be set by service
		}

		// Create in database
		err = h.hs.Create(ctx, deltaHttp)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpDeltaUpdate(ctx context.Context, req *connect.Request[apiv1.HttpDeltaUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpDeltaDelete(ctx context.Context, req *connect.Request[apiv1.HttpDeltaDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpDeltaSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpDeltaSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpRun(ctx context.Context, req *connect.Request[apiv1.HttpRunRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.HttpId) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_id is required"))
	}

	httpID, err := idwrap.NewFromBytes(req.Msg.HttpId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Get HTTP entry to check workspace permissions
	httpEntry, err := h.hs.Get(ctx, httpID)
	if err != nil {
		if errors.Is(err, shttp.ErrNoHTTPFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Check read access (any role in workspace)
	if err := h.checkWorkspaceReadAccess(ctx, httpEntry.WorkspaceID); err != nil {
		return nil, err
	}

	// Execute HTTP request with proper error handling
	if err := h.executeHTTPRequest(ctx, httpEntry); err != nil {
		// Handle different types of errors appropriately
		if isNetworkError(err) {
			return nil, connect.NewError(connect.CodeUnavailable, err)
		}
		if isTimeoutError(err) {
			return nil, connect.NewError(connect.CodeDeadlineExceeded, err)
		}
		if isDNSError(err) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpVersionCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpVersionCollectionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpVersionSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpVersionSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

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
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			// Convert to API format
			for _, param := range params {
				apiParam := toAPIHttpSearchParam(param)
				allParams = append(allParams, apiParam)
			}
		}
	}

	return connect.NewResponse(&apiv1.HttpSearchParamCollectionResponse{Items: allParams}), nil
}

func (h *HttpServiceRPC) HttpSearchParamCreate(ctx context.Context, req *connect.Request[apiv1.HttpSearchParamCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP search param must be provided"))
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	httpSearchParamService := h.httpSearchParamService.TX(tx)

	var createdParams []mhttpsearchparam.HttpSearchParam

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

		// Verify the HTTP entry exists and user has access
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

		// Create the param
		paramModel := &mhttpsearchparam.HttpSearchParam{
			ID:          paramID,
			HttpID:      httpID,
			Key:         item.Key,
			Value:       item.Value,
			Enabled:     item.Enabled,
			Description: item.Description,
			Order:       float64(item.Order),
		}

		if err := httpSearchParamService.Create(ctx, paramModel); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		createdParams = append(createdParams, *paramModel)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish create events for real-time sync
	for _, param := range createdParams {
		// Get workspace ID for the HTTP entry
		httpEntry, err := h.hs.Get(ctx, param.HttpID)
		if err != nil {
			// Log error but continue - event publishing shouldn't fail the operation
			continue
		}
		h.httpSearchParamStream.Publish(HttpSearchParamTopic{WorkspaceID: httpEntry.WorkspaceID}, HttpSearchParamEvent{
			Type:            eventTypeCreate,
			HttpSearchParam: toAPIHttpSearchParam(param),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpSearchParamUpdate(ctx context.Context, req *connect.Request[apiv1.HttpSearchParamUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP search param must be provided"))
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	httpSearchParamService := h.httpSearchParamService.TX(tx)

	var updatedParams []mhttpsearchparam.HttpSearchParam

	for _, item := range req.Msg.Items {
		if len(item.HttpSearchParamId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_search_param_id is required"))
		}

		paramID, err := idwrap.NewFromBytes(item.HttpSearchParamId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing param within transaction for consistency
		existingParam, err := httpSearchParamService.GetHttpSearchParam(ctx, paramID)
		if err != nil {
			if errors.Is(err, shttpsearchparam.ErrNoHttpSearchParamFound) {
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

		// Update fields if provided
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

		if err := httpSearchParamService.Update(ctx, existingParam); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		updatedParams = append(updatedParams, *existingParam)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish update events for real-time sync
	for _, param := range updatedParams {
		// Get workspace ID for the HTTP entry
		httpEntry, err := h.hs.Get(ctx, param.HttpID)
		if err != nil {
			// Log error but continue - event publishing shouldn't fail the operation
			continue
		}
		h.httpSearchParamStream.Publish(HttpSearchParamTopic{WorkspaceID: httpEntry.WorkspaceID}, HttpSearchParamEvent{
			Type:            eventTypeUpdate,
			HttpSearchParam: toAPIHttpSearchParam(param),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpSearchParamDelete(ctx context.Context, req *connect.Request[apiv1.HttpSearchParamDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP search param must be provided"))
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	httpSearchParamService := h.httpSearchParamService.TX(tx)

	var deletedParams []mhttpsearchparam.HttpSearchParam
	var deletedWorkspaceIDs []idwrap.IDWrap

	for _, item := range req.Msg.Items {
		if len(item.HttpSearchParamId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_search_param_id is required"))
		}

		paramID, err := idwrap.NewFromBytes(item.HttpSearchParamId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing param within transaction for consistency
		existingParam, err := httpSearchParamService.GetHttpSearchParam(ctx, paramID)
		if err != nil {
			if errors.Is(err, shttpsearchparam.ErrNoHttpSearchParamFound) {
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

		// Delete the param
		if err := httpSearchParamService.Delete(ctx, paramID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		deletedParams = append(deletedParams, *existingParam)
		deletedWorkspaceIDs = append(deletedWorkspaceIDs, httpEntry.WorkspaceID)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish delete events for real-time sync
	for i, param := range deletedParams {
		h.httpSearchParamStream.Publish(HttpSearchParamTopic{WorkspaceID: deletedWorkspaceIDs[i]}, HttpSearchParamEvent{
			Type:            eventTypeDelete,
			HttpSearchParam: toAPIHttpSearchParam(param),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
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

	// Snapshot provider for initial state
	snapshot := func(ctx context.Context) ([]eventstream.Event[HttpSearchParamTopic, HttpSearchParamEvent], error) {
		// Get all HTTP entries for user
		httpList, err := h.listUserHttp(ctx)
		if err != nil {
			return nil, err
		}

		events := make([]eventstream.Event[HttpSearchParamTopic, HttpSearchParamEvent], 0)
		for _, http := range httpList {
			workspaceSet.Store(http.WorkspaceID.String(), struct{}{})

			// Get params for this HTTP entry
			params, err := h.httpSearchParamService.GetByHttpIDOrdered(ctx, http.ID)
			if err != nil {
				return nil, err
			}

			for _, param := range params {
				events = append(events, eventstream.Event[HttpSearchParamTopic, HttpSearchParamEvent]{
					Topic: HttpSearchParamTopic{WorkspaceID: http.WorkspaceID},
					Payload: HttpSearchParamEvent{
						Type:            eventTypeCreate,
						HttpSearchParam: toAPIHttpSearchParam(param),
					},
				})
			}
		}
		return events, nil
	}

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

	// Subscribe to events with snapshot
	events, err := h.httpSearchParamStream.Subscribe(ctx, filter, eventstream.WithSnapshot(snapshot))
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
			resp := httpSearchParamSyncResponseFrom(evt.Payload)
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

// streamHttpAssertSync streams HTTP assert events to the client
func (h *HttpServiceRPC) streamHttpAssertSync(ctx context.Context, userID idwrap.IDWrap, send func(*apiv1.HttpAssertSyncResponse) error) error {
	var workspaceSet sync.Map

	// Snapshot provider for initial state
	snapshot := func(ctx context.Context) ([]eventstream.Event[HttpAssertTopic, HttpAssertEvent], error) {
		// Get all HTTP entries for user
		httpList, err := h.listUserHttp(ctx)
		if err != nil {
			return nil, err
		}

		events := make([]eventstream.Event[HttpAssertTopic, HttpAssertEvent], 0)
		for _, http := range httpList {
			workspaceSet.Store(http.WorkspaceID.String(), struct{}{})

			// Get asserts for this HTTP entry
			asserts, err := h.httpAssertService.GetHttpAssertsByHttpID(ctx, http.ID)
			if err != nil {
				return nil, err
			}

			for _, assert := range asserts {
				events = append(events, eventstream.Event[HttpAssertTopic, HttpAssertEvent]{
					Topic: HttpAssertTopic{WorkspaceID: http.WorkspaceID},
					Payload: HttpAssertEvent{
						Type:       eventTypeCreate,
						HttpAssert: toAPIHttpAssert(assert),
					},
				})
			}
		}
		return events, nil
	}

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

	// Subscribe to events with snapshot
	events, err := h.httpAssertStream.Subscribe(ctx, filter, eventstream.WithSnapshot(snapshot))
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
			resp := httpAssertSyncResponseFrom(evt.Payload)
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
		httpList, err := h.hs.GetByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Get params for each HTTP entry
		for _, http := range httpList {
			params, err := h.httpSearchParamService.GetByHttpIDOrdered(ctx, http.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			// Convert to delta format
			for _, param := range params {
				delta := &apiv1.HttpSearchParamDelta{
					DeltaHttpSearchParamId: param.ID.Bytes(),
					HttpSearchParamId:      param.ID.Bytes(),
					HttpId:                 param.HttpID.Bytes(),
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

func (h *HttpServiceRPC) HttpSearchParamDeltaCreate(ctx context.Context, req *connect.Request[apiv1.HttpSearchParamDeltaCreateRequest]) (*connect.Response[emptypb.Empty], error) {
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
		param, err := h.httpSearchParamService.GetHttpSearchParam(ctx, paramID)
		if err != nil {
			if errors.Is(err, shttpsearchparam.ErrNoHttpSearchParamFound) {
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
		err = h.httpSearchParamService.UpdateHttpSearchParamDelta(ctx, paramID, item.Key, item.Value, item.Enabled, item.Description, deltaOrder)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpSearchParamDeltaUpdate(ctx context.Context, req *connect.Request[apiv1.HttpSearchParamDeltaUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("HttpSearchParamDeltaUpdate not implemented yet"))
}

func (h *HttpServiceRPC) HttpSearchParamDeltaDelete(ctx context.Context, req *connect.Request[apiv1.HttpSearchParamDeltaDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("HttpSearchParamDeltaDelete not implemented yet"))
}

func (h *HttpServiceRPC) HttpSearchParamDeltaSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpSearchParamDeltaSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errors.New("HttpSearchParamDeltaSync not implemented yet"))
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
			asserts, err := h.httpAssertService.GetHttpAssertsByHttpID(ctx, http.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			// Convert to API format
			for _, assert := range asserts {
				apiAssert := toAPIHttpAssert(assert)
				allAsserts = append(allAsserts, apiAssert)
			}
		}
	}

	return connect.NewResponse(&apiv1.HttpAssertCollectionResponse{Items: allAsserts}), nil
}

func (h *HttpServiceRPC) HttpAssertCreate(ctx context.Context, req *connect.Request[apiv1.HttpAssertCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP assert must be provided"))
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	httpAssertService := h.httpAssertService.TX(tx)

	var createdAsserts []mhttpassert.HttpAssert

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

		// Verify the HTTP entry exists and user has access
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

		// Create the assert
		assertModel := &mhttpassert.HttpAssert{
			ID:          assertID,
			HttpID:      httpID,
			Key:         "", // HttpAssert doesn't use Key field
			Value:       item.Value,
			Enabled:     true, // Assertions are always active
			Description: "",   // No description in API
			Order:       0,    // No order in API
		}

		if err := httpAssertService.CreateHttpAssert(ctx, assertModel); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		createdAsserts = append(createdAsserts, *assertModel)
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
			Type:       eventTypeCreate,
			HttpAssert: toAPIHttpAssert(assert),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpAssertUpdate(ctx context.Context, req *connect.Request[apiv1.HttpAssertUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP assert must be provided"))
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	httpAssertService := h.httpAssertService.TX(tx)

	var updatedAsserts []mhttpassert.HttpAssert

	for _, item := range req.Msg.Items {
		if len(item.HttpAssertId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_assert_id is required"))
		}

		assertID, err := idwrap.NewFromBytes(item.HttpAssertId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing assert to verify ownership
		existingAssert, err := httpAssertService.GetHttpAssert(ctx, assertID)
		if err != nil {
			if errors.Is(err, shttpassert.ErrNoHttpAssertFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Verify the HTTP entry exists and user has access
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

		// Update fields if provided
		if item.Value != nil {
			existingAssert.Value = *item.Value
		}

		if err := httpAssertService.UpdateHttpAssert(ctx, existingAssert); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		updatedAsserts = append(updatedAsserts, *existingAssert)
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
			HttpAssert: toAPIHttpAssert(assert),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpAssertDelete(ctx context.Context, req *connect.Request[apiv1.HttpAssertDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP assert must be provided"))
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	httpAssertService := h.httpAssertService.TX(tx)

	var deletedAsserts []mhttpassert.HttpAssert
	var deletedWorkspaceIDs []idwrap.IDWrap

	for _, item := range req.Msg.Items {
		if len(item.HttpAssertId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_assert_id is required"))
		}

		assertID, err := idwrap.NewFromBytes(item.HttpAssertId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing assert to verify ownership
		existingAssert, err := httpAssertService.GetHttpAssert(ctx, assertID)
		if err != nil {
			if errors.Is(err, shttpassert.ErrNoHttpAssertFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Verify the HTTP entry exists and user has access
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

		if err := httpAssertService.DeleteHttpAssert(ctx, assertID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		deletedAsserts = append(deletedAsserts, *existingAssert)
		deletedWorkspaceIDs = append(deletedWorkspaceIDs, httpEntry.WorkspaceID)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish delete events for real-time sync
	for i, assert := range deletedAsserts {
		h.httpAssertStream.Publish(HttpAssertTopic{WorkspaceID: deletedWorkspaceIDs[i]}, HttpAssertEvent{
			Type:       eventTypeDelete,
			HttpAssert: toAPIHttpAssert(assert),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpAssertSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpAssertSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return h.streamHttpAssertSync(ctx, userID, stream.Send)
}

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
		httpList, err := h.hs.GetByWorkspaceID(ctx, workspace.ID)
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
				delta := &apiv1.HttpAssertDelta{
					DeltaHttpAssertId: assert.ID.Bytes(),
					HttpAssertId:      assert.ID.Bytes(),
					HttpId:            assert.HttpID.Bytes(),
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

func (h *HttpServiceRPC) HttpAssertDeltaCreate(ctx context.Context, req *connect.Request[apiv1.HttpAssertDeltaCreateRequest]) (*connect.Response[emptypb.Empty], error) {
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
	// Stub implementation - delta updates are handled via HttpAssertDeltaCreate
	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpAssertDeltaDelete(ctx context.Context, req *connect.Request[apiv1.HttpAssertDeltaDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	// Stub implementation - delta deletion is not supported
	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpAssertDeltaSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpAssertDeltaSyncResponse]) error {
	// Stub implementation - delta sync is not implemented
	return connect.NewError(connect.CodeUnimplemented, errors.New("HttpAssertDeltaSync not implemented yet"))
}

func (h *HttpServiceRPC) HttpResponseCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpResponseCollectionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpResponseSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpResponseSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpResponseHeaderCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpResponseHeaderCollectionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpResponseHeaderSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpResponseHeaderSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpResponseAssertCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpResponseAssertCollectionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpResponseAssertSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpResponseAssertSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
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
				apiHeader := toAPIHttpHeader(header)
				allHeaders = append(allHeaders, apiHeader)
			}
		}
	}

	return connect.NewResponse(&apiv1.HttpHeaderCollectionResponse{Items: allHeaders}), nil
}

func (h *HttpServiceRPC) HttpHeaderCreate(ctx context.Context, req *connect.Request[apiv1.HttpHeaderCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP header must be provided"))
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	httpHeaderService := h.httpHeaderService.TX(tx)

	var createdHeaders []mhttpheader.HttpHeader

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

		// Verify the HTTP entry exists and user has access
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

		// Create the header
		headerModel := &mhttpheader.HttpHeader{
			ID:          headerID,
			HttpID:      httpID,
			Key:         item.Key,
			Value:       item.Value,
			Enabled:     item.Enabled,
			Description: item.Description,
			Order:       item.Order,
		}

		if err := httpHeaderService.Create(ctx, headerModel); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		createdHeaders = append(createdHeaders, *headerModel)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish create events for real-time sync
	for _, header := range createdHeaders {
		// Get workspace ID for the HTTP entry
		httpEntry, err := h.hs.Get(ctx, header.HttpID)
		if err != nil {
			// Log error but continue - event publishing shouldn't fail the operation
			continue
		}
		h.httpHeaderStream.Publish(HttpHeaderTopic{WorkspaceID: httpEntry.WorkspaceID}, HttpHeaderEvent{
			Type:       eventTypeCreate,
			HttpHeader: toAPIHttpHeader(header),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpHeaderUpdate(ctx context.Context, req *connect.Request[apiv1.HttpHeaderUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP header must be provided"))
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	httpHeaderService := h.httpHeaderService.TX(tx)

	var updatedHeaders []mhttpheader.HttpHeader

	for _, item := range req.Msg.Items {
		if len(item.HttpHeaderId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_header_id is required"))
		}

		headerID, err := idwrap.NewFromBytes(item.HttpHeaderId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing header within transaction for consistency
		existingHeader, err := httpHeaderService.GetByID(ctx, headerID)
		if err != nil {
			if errors.Is(err, shttpheader.ErrNoHttpHeaderFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Get the HTTP entry to check workspace access
		httpEntry, err := h.hs.Get(ctx, existingHeader.HttpID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Check write access to the workspace
		if err := h.checkWorkspaceWriteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		// Update fields if provided
		if item.Key != nil {
			existingHeader.Key = *item.Key
		}
		if item.Value != nil {
			existingHeader.Value = *item.Value
		}
		if item.Enabled != nil {
			existingHeader.Enabled = *item.Enabled
		}
		if item.Description != nil {
			existingHeader.Description = *item.Description
		}
		if item.Order != nil {
			existingHeader.Order = *item.Order
		}

		if err := httpHeaderService.Update(ctx, &existingHeader); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		updatedHeaders = append(updatedHeaders, existingHeader)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish update events for real-time sync
	for _, header := range updatedHeaders {
		// Get workspace ID for the HTTP entry
		httpEntry, err := h.hs.Get(ctx, header.HttpID)
		if err != nil {
			// Log error but continue - event publishing shouldn't fail the operation
			continue
		}
		h.httpHeaderStream.Publish(HttpHeaderTopic{WorkspaceID: httpEntry.WorkspaceID}, HttpHeaderEvent{
			Type:       eventTypeUpdate,
			HttpHeader: toAPIHttpHeader(header),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpHeaderDelete(ctx context.Context, req *connect.Request[apiv1.HttpHeaderDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP header must be provided"))
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	httpHeaderService := h.httpHeaderService.TX(tx)

	var deletedHeaders []mhttpheader.HttpHeader
	var deletedWorkspaceIDs []idwrap.IDWrap

	for _, item := range req.Msg.Items {
		if len(item.HttpHeaderId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_header_id is required"))
		}

		headerID, err := idwrap.NewFromBytes(item.HttpHeaderId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing header within transaction for consistency
		existingHeader, err := httpHeaderService.GetByID(ctx, headerID)
		if err != nil {
			if errors.Is(err, shttpheader.ErrNoHttpHeaderFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Get the HTTP entry to check workspace access
		httpEntry, err := h.hs.Get(ctx, existingHeader.HttpID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Check delete access to the workspace
		if err := h.checkWorkspaceDeleteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		// Delete the header
		if err := httpHeaderService.Delete(ctx, headerID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		deletedHeaders = append(deletedHeaders, existingHeader)
		deletedWorkspaceIDs = append(deletedWorkspaceIDs, httpEntry.WorkspaceID)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish delete events for real-time sync
	for i, header := range deletedHeaders {
		h.httpHeaderStream.Publish(HttpHeaderTopic{WorkspaceID: deletedWorkspaceIDs[i]}, HttpHeaderEvent{
			Type:       eventTypeDelete,
			HttpHeader: toAPIHttpHeader(header),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// HttpHeaderSync handles real-time synchronization for HTTP header entries
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

	// Snapshot provider for initial state
	snapshot := func(ctx context.Context) ([]eventstream.Event[HttpHeaderTopic, HttpHeaderEvent], error) {
		// Get all HTTP entries for user
		httpList, err := h.listUserHttp(ctx)
		if err != nil {
			return nil, err
		}

		events := make([]eventstream.Event[HttpHeaderTopic, HttpHeaderEvent], 0)
		for _, http := range httpList {
			workspaceSet.Store(http.WorkspaceID.String(), struct{}{})

			// Get headers for this HTTP entry
			headers, err := h.httpHeaderService.GetByHttpID(ctx, http.ID)
			if err != nil {
				return nil, err
			}

			for _, header := range headers {
				events = append(events, eventstream.Event[HttpHeaderTopic, HttpHeaderEvent]{
					Topic: HttpHeaderTopic{WorkspaceID: http.WorkspaceID},
					Payload: HttpHeaderEvent{
						Type:       eventTypeCreate,
						HttpHeader: toAPIHttpHeader(header),
					},
				})
			}
		}
		return events, nil
	}

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
	events, err := h.httpHeaderStream.Subscribe(ctx, filter, eventstream.WithSnapshot(snapshot))
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

			// Convert to delta format
			for _, header := range headers {
				delta := &apiv1.HttpHeaderDelta{
					DeltaHttpHeaderId: header.ID.Bytes(),
					HttpHeaderId:      header.ID.Bytes(),
					HttpId:            header.HttpID.Bytes(),
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

func (h *HttpServiceRPC) HttpHeaderDeltaCreate(ctx context.Context, req *connect.Request[apiv1.HttpHeaderDeltaCreateRequest]) (*connect.Response[emptypb.Empty], error) {
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
			if errors.Is(err, shttpheader.ErrNoHttpHeaderFound) {
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
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("HttpHeaderDeltaUpdate not implemented yet"))
}

func (h *HttpServiceRPC) HttpHeaderDeltaDelete(ctx context.Context, req *connect.Request[apiv1.HttpHeaderDeltaDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("HttpHeaderDeltaDelete not implemented yet"))
}

func (h *HttpServiceRPC) HttpHeaderDeltaSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpHeaderDeltaSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errors.New("HttpHeaderDeltaSync not implemented yet"))
}

func (h *HttpServiceRPC) HttpBodyFormCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpBodyFormCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Get user's workspaces
	workspaces, err := h.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var allBodyForms []*apiv1.HttpBodyForm
	for _, workspace := range workspaces {
		// Get HTTP entries for this workspace
		httpList, err := h.hs.GetByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Get body forms for each HTTP entry
		for _, http := range httpList {
			bodyForms, err := h.httpBodyFormService.GetHttpBodyFormsByHttpID(ctx, http.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			// Convert to API format
			for _, bodyForm := range bodyForms {
				apiBodyForm := toAPIHttpBodyForm(bodyForm)
				allBodyForms = append(allBodyForms, apiBodyForm)
			}
		}
	}

	return connect.NewResponse(&apiv1.HttpBodyFormCollectionResponse{Items: allBodyForms}), nil
}

func (h *HttpServiceRPC) HttpBodyFormCreate(ctx context.Context, req *connect.Request[apiv1.HttpBodyFormCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP body form must be provided"))
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	httpBodyFormService := h.httpBodyFormService.TX(tx)

	var createdBodyForms []mhttpbodyform.HttpBodyForm

	for _, item := range req.Msg.Items {
		if len(item.HttpBodyFormId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_body_form_id is required"))
		}
		if len(item.HttpId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_id is required"))
		}

		bodyFormID, err := idwrap.NewFromBytes(item.HttpBodyFormId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		httpID, err := idwrap.NewFromBytes(item.HttpId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Verify the HTTP entry exists and user has access
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

		// Create the body form
		bodyFormModel := &mhttpbodyform.HttpBodyForm{
			ID:          bodyFormID,
			HttpID:      httpID,
			Key:         item.Key,
			Value:       item.Value,
			Enabled:     item.Enabled,
			Description: item.Description,
			Order:       item.Order,
		}

		if err := httpBodyFormService.CreateHttpBodyForm(ctx, bodyFormModel); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		createdBodyForms = append(createdBodyForms, *bodyFormModel)
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
			Type:         eventTypeCreate,
			HttpBodyForm: toAPIHttpBodyForm(bodyForm),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyFormUpdate(ctx context.Context, req *connect.Request[apiv1.HttpBodyFormUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP body form must be provided"))
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	httpBodyFormService := h.httpBodyFormService.TX(tx)

	var updatedBodyForms []mhttpbodyform.HttpBodyForm

	for _, item := range req.Msg.Items {
		if len(item.HttpBodyFormId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_body_form_id is required"))
		}

		bodyFormID, err := idwrap.NewFromBytes(item.HttpBodyFormId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing body form to verify ownership
		existingBodyForm, err := httpBodyFormService.GetHttpBodyForm(ctx, bodyFormID)
		if err != nil {
			if errors.Is(err, shttpbodyform.ErrNoHttpBodyFormFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Verify the HTTP entry exists and user has access
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

		// Update fields if provided
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

		if err := httpBodyFormService.UpdateHttpBodyForm(ctx, existingBodyForm); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		updatedBodyForms = append(updatedBodyForms, *existingBodyForm)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish update events for real-time sync
	for _, bodyForm := range updatedBodyForms {
		// Get workspace ID for the HTTP entry
		httpEntry, err := h.hs.Get(ctx, bodyForm.HttpID)
		if err != nil {
			// Log error but continue - event publishing shouldn't fail the operation
			continue
		}
		h.httpBodyFormStream.Publish(HttpBodyFormTopic{WorkspaceID: httpEntry.WorkspaceID}, HttpBodyFormEvent{
			Type:         eventTypeUpdate,
			HttpBodyForm: toAPIHttpBodyForm(bodyForm),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyFormDelete(ctx context.Context, req *connect.Request[apiv1.HttpBodyFormDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP body form must be provided"))
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	httpBodyFormService := h.httpBodyFormService.TX(tx)

	var deletedBodyForms []mhttpbodyform.HttpBodyForm

	for _, item := range req.Msg.Items {
		if len(item.HttpBodyFormId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_body_form_id is required"))
		}

		bodyFormID, err := idwrap.NewFromBytes(item.HttpBodyFormId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing body form to verify ownership
		existingBodyForm, err := httpBodyFormService.GetHttpBodyForm(ctx, bodyFormID)
		if err != nil {
			if errors.Is(err, shttpbodyform.ErrNoHttpBodyFormFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Verify the HTTP entry exists and user has access
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

		if err := httpBodyFormService.DeleteHttpBodyForm(ctx, bodyFormID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		deletedBodyForms = append(deletedBodyForms, *existingBodyForm)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish delete events for real-time sync
	for _, bodyForm := range deletedBodyForms {
		// Get workspace ID for the HTTP entry
		httpEntry, err := h.hs.Get(ctx, bodyForm.HttpID)
		if err != nil {
			// Log error but continue - event publishing shouldn't fail the operation
			continue
		}
		h.httpBodyFormStream.Publish(HttpBodyFormTopic{WorkspaceID: httpEntry.WorkspaceID}, HttpBodyFormEvent{
			Type:         eventTypeDelete,
			HttpBodyForm: toAPIHttpBodyForm(bodyForm),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyFormSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpBodyFormSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return h.streamHttpBodyFormSync(ctx, userID, stream.Send)
}

func httpBodyFormSyncResponseFrom(event HttpBodyFormEvent) *apiv1.HttpBodyFormSyncResponse {
	var value *apiv1.HttpBodyFormSync_ValueUnion

	switch event.Type {
	case eventTypeCreate:
		key := event.HttpBodyForm.GetKey()
		value_ := event.HttpBodyForm.GetValue()
		enabled := event.HttpBodyForm.GetEnabled()
		description := event.HttpBodyForm.GetDescription()
		order := event.HttpBodyForm.GetOrder()
		value = &apiv1.HttpBodyFormSync_ValueUnion{
			Kind: apiv1.HttpBodyFormSync_ValueUnion_KIND_CREATE,
			Create: &apiv1.HttpBodyFormSyncCreate{
				HttpBodyFormId: event.HttpBodyForm.GetHttpBodyFormId(),
				HttpId:         event.HttpBodyForm.GetHttpId(),
				Key:            key,
				Value:          value_,
				Enabled:        enabled,
				Description:    description,
				Order:          order,
			},
		}
	case eventTypeUpdate:
		value = &apiv1.HttpBodyFormSync_ValueUnion{
			Kind: apiv1.HttpBodyFormSync_ValueUnion_KIND_UPDATE,
			Update: &apiv1.HttpBodyFormSyncUpdate{
				HttpBodyFormId: event.HttpBodyForm.GetHttpBodyFormId(),
			},
		}
	case eventTypeDelete:
		value = &apiv1.HttpBodyFormSync_ValueUnion{
			Kind: apiv1.HttpBodyFormSync_ValueUnion_KIND_DELETE,
			Delete: &apiv1.HttpBodyFormSyncDelete{
				HttpBodyFormId: event.HttpBodyForm.GetHttpBodyFormId(),
			},
		}
	}

	return &apiv1.HttpBodyFormSyncResponse{
		Items: []*apiv1.HttpBodyFormSync{
			{
				Value: value,
			},
		},
	}
}

// streamHttpBodyFormSync streams HTTP body form events to the client
func (h *HttpServiceRPC) streamHttpBodyFormSync(ctx context.Context, userID idwrap.IDWrap, send func(*apiv1.HttpBodyFormSyncResponse) error) error {
	var workspaceSet sync.Map

	// Snapshot provider for initial state
	snapshot := func(ctx context.Context) ([]eventstream.Event[HttpBodyFormTopic, HttpBodyFormEvent], error) {
		// Get all HTTP entries for user
		httpList, err := h.listUserHttp(ctx)
		if err != nil {
			return nil, err
		}

		events := make([]eventstream.Event[HttpBodyFormTopic, HttpBodyFormEvent], 0)
		for _, http := range httpList {
			workspaceSet.Store(http.WorkspaceID.String(), struct{}{})

			// Get body forms for this HTTP entry
			bodyForms, err := h.httpBodyFormService.GetHttpBodyFormsByHttpID(ctx, http.ID)
			if err != nil {
				return nil, err
			}

			for _, bodyForm := range bodyForms {
				events = append(events, eventstream.Event[HttpBodyFormTopic, HttpBodyFormEvent]{
					Topic: HttpBodyFormTopic{WorkspaceID: http.WorkspaceID},
					Payload: HttpBodyFormEvent{
						Type:         eventTypeCreate,
						HttpBodyForm: toAPIHttpBodyForm(bodyForm),
					},
				})
			}
		}
		return events, nil
	}

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

	// Subscribe to events with snapshot
	events, err := h.httpBodyFormStream.Subscribe(ctx, filter, eventstream.WithSnapshot(snapshot))
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
			resp := httpBodyFormSyncResponseFrom(evt.Payload)
			if err := send(resp); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (h *HttpServiceRPC) HttpBodyFormDeltaCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpBodyFormDeltaCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Get user's workspaces
	workspaces, err := h.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var allDeltas []*apiv1.HttpBodyFormDelta
	for _, workspace := range workspaces {
		// Get HTTP entries for this workspace
		httpList, err := h.hs.GetByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Get body forms for each HTTP entry
		for _, http := range httpList {
			bodyForms, err := h.httpBodyFormService.GetHttpBodyFormsByHttpID(ctx, http.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			// Convert to delta format
			for _, bodyForm := range bodyForms {
				delta := &apiv1.HttpBodyFormDelta{
					DeltaHttpBodyFormId: bodyForm.ID.Bytes(),
					HttpBodyFormId:      bodyForm.ID.Bytes(),
					HttpId:              bodyForm.HttpID.Bytes(),
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

	return connect.NewResponse(&apiv1.HttpBodyFormDeltaCollectionResponse{
		Items: allDeltas,
	}), nil
}

func (h *HttpServiceRPC) HttpBodyFormDeltaCreate(ctx context.Context, req *connect.Request[apiv1.HttpBodyFormDeltaCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.Items) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one delta item is required"))
	}

	// Process each delta item
	for _, item := range req.Msg.Items {
		if len(item.HttpBodyFormId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_body_form_id is required for each delta item"))
		}

		bodyFormID, err := idwrap.NewFromBytes(item.HttpBodyFormId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Check workspace write access
		bodyForm, err := h.httpBodyFormService.GetHttpBodyForm(ctx, bodyFormID)
		if err != nil {
			if errors.Is(err, shttpbodyform.ErrNoHttpBodyFormFound) {
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
		err = h.httpBodyFormService.UpdateHttpBodyFormDelta(ctx, bodyFormID, item.Key, item.Value, item.Enabled, item.Description, item.Order)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyFormDeltaUpdate(ctx context.Context, req *connect.Request[apiv1.HttpBodyFormDeltaUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	// Stub implementation - delta updates are handled via HttpBodyFormDeltaCreate
	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyFormDeltaDelete(ctx context.Context, req *connect.Request[apiv1.HttpBodyFormDeltaDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	// Stub implementation - delta deletion is not supported
	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyFormDeltaSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpBodyFormDeltaSyncResponse]) error {
	// Stub implementation - delta sync is not implemented
	return connect.NewError(connect.CodeUnimplemented, errors.New("HttpBodyFormDeltaSync not implemented yet"))
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
			bodyUrlEncodeds, err := h.httpBodyUrlEncodedService.GetHttpBodyUrlEncodedByHttpID(ctx, http.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			// Convert to API format
			for _, bodyUrlEncoded := range bodyUrlEncodeds {
				apiBodyUrlEncoded := toAPIHttpBodyUrlEncoded(bodyUrlEncoded)
				allBodyUrlEncodeds = append(allBodyUrlEncodeds, apiBodyUrlEncoded)
			}
		}
	}

	return connect.NewResponse(&apiv1.HttpBodyUrlEncodedCollectionResponse{Items: allBodyUrlEncodeds}), nil
}

func (h *HttpServiceRPC) HttpBodyUrlEncodedCreate(ctx context.Context, req *connect.Request[apiv1.HttpBodyUrlEncodedCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP body URL encoded must be provided"))
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	httpBodyUrlEncodedService := h.httpBodyUrlEncodedService.TX(tx)

	var createdBodyUrlEncodeds []mhttpbodyurlencoded.HttpBodyUrlEncoded

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

		// Verify the HTTP entry exists and user has access
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

		// Create the body URL encoded
		bodyUrlEncodedModel := &mhttpbodyurlencoded.HttpBodyUrlEncoded{
			ID:          bodyUrlEncodedID,
			HttpID:      httpID,
			Key:         item.Key,
			Value:       item.Value,
			Enabled:     item.Enabled,
			Description: item.Description,
			Order:       item.Order,
		}

		if err := httpBodyUrlEncodedService.CreateHttpBodyUrlEncoded(ctx, bodyUrlEncodedModel); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		createdBodyUrlEncodeds = append(createdBodyUrlEncodeds, *bodyUrlEncodedModel)
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
			Type:               eventTypeCreate,
			HttpBodyUrlEncoded: toAPIHttpBodyUrlEncoded(bodyUrlEncoded),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyUrlEncodedUpdate(ctx context.Context, req *connect.Request[apiv1.HttpBodyUrlEncodedUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP body URL encoded must be provided"))
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	httpBodyUrlEncodedService := h.httpBodyUrlEncodedService.TX(tx)

	var updatedBodyUrlEncodeds []mhttpbodyurlencoded.HttpBodyUrlEncoded

	for _, item := range req.Msg.Items {
		if len(item.HttpBodyUrlEncodedId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_body_url_encoded_id is required"))
		}

		bodyUrlEncodedID, err := idwrap.NewFromBytes(item.HttpBodyUrlEncodedId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing body URL encoded to verify ownership
		existingBodyUrlEncoded, err := httpBodyUrlEncodedService.GetHttpBodyUrlEncoded(ctx, bodyUrlEncodedID)
		if err != nil {
			if errors.Is(err, shttpbodyurlencoded.ErrNoHttpBodyUrlEncodedFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Verify the HTTP entry exists and user has access
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

		// Update fields if provided
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

		if err := httpBodyUrlEncodedService.UpdateHttpBodyUrlEncoded(ctx, existingBodyUrlEncoded); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		updatedBodyUrlEncodeds = append(updatedBodyUrlEncodeds, *existingBodyUrlEncoded)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish update events for real-time sync
	for _, bodyUrlEncoded := range updatedBodyUrlEncodeds {
		// Get workspace ID for the HTTP entry
		httpEntry, err := h.hs.Get(ctx, bodyUrlEncoded.HttpID)
		if err != nil {
			// Log error but continue - event publishing shouldn't fail the operation
			continue
		}
		h.httpBodyUrlEncodedStream.Publish(HttpBodyUrlEncodedTopic{WorkspaceID: httpEntry.WorkspaceID}, HttpBodyUrlEncodedEvent{
			Type:               eventTypeUpdate,
			HttpBodyUrlEncoded: toAPIHttpBodyUrlEncoded(bodyUrlEncoded),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyUrlEncodedDelete(ctx context.Context, req *connect.Request[apiv1.HttpBodyUrlEncodedDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP body URL encoded must be provided"))
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	httpBodyUrlEncodedService := h.httpBodyUrlEncodedService.TX(tx)

	var deletedBodyUrlEncodeds []mhttpbodyurlencoded.HttpBodyUrlEncoded

	for _, item := range req.Msg.Items {
		if len(item.HttpBodyUrlEncodedId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_body_url_encoded_id is required"))
		}

		bodyUrlEncodedID, err := idwrap.NewFromBytes(item.HttpBodyUrlEncodedId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing body URL encoded to verify ownership
		existingBodyUrlEncoded, err := httpBodyUrlEncodedService.GetHttpBodyUrlEncoded(ctx, bodyUrlEncodedID)
		if err != nil {
			if errors.Is(err, shttpbodyurlencoded.ErrNoHttpBodyUrlEncodedFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Verify the HTTP entry exists and user has access
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

		if err := httpBodyUrlEncodedService.DeleteHttpBodyUrlEncoded(ctx, bodyUrlEncodedID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		deletedBodyUrlEncodeds = append(deletedBodyUrlEncodeds, *existingBodyUrlEncoded)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish delete events for real-time sync
	for _, bodyUrlEncoded := range deletedBodyUrlEncodeds {
		// Get workspace ID for the HTTP entry
		httpEntry, err := h.hs.Get(ctx, bodyUrlEncoded.HttpID)
		if err != nil {
			// Log error but continue - event publishing shouldn't fail the operation
			continue
		}
		h.httpBodyUrlEncodedStream.Publish(HttpBodyUrlEncodedTopic{WorkspaceID: httpEntry.WorkspaceID}, HttpBodyUrlEncodedEvent{
			Type:               eventTypeDelete,
			HttpBodyUrlEncoded: toAPIHttpBodyUrlEncoded(bodyUrlEncoded),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyUrlEncodedSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpBodyUrlEncodedSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return h.streamHttpBodyUrlEncodedSync(ctx, userID, stream.Send)
}

func httpBodyUrlEncodedSyncResponseFrom(event HttpBodyUrlEncodedEvent) *apiv1.HttpBodyUrlEncodedSyncResponse {
	var value *apiv1.HttpBodyUrlEncodedSync_ValueUnion

	switch event.Type {
	case eventTypeCreate:
		key := event.HttpBodyUrlEncoded.GetKey()
		value_ := event.HttpBodyUrlEncoded.GetValue()
		enabled := event.HttpBodyUrlEncoded.GetEnabled()
		description := event.HttpBodyUrlEncoded.GetDescription()
		order := event.HttpBodyUrlEncoded.GetOrder()
		value = &apiv1.HttpBodyUrlEncodedSync_ValueUnion{
			Kind: apiv1.HttpBodyUrlEncodedSync_ValueUnion_KIND_CREATE,
			Create: &apiv1.HttpBodyUrlEncodedSyncCreate{
				HttpBodyUrlEncodedId: event.HttpBodyUrlEncoded.GetHttpBodyUrlEncodedId(),
				HttpId:               event.HttpBodyUrlEncoded.GetHttpId(),
				Key:                  key,
				Value:                value_,
				Enabled:              enabled,
				Description:          description,
				Order:                order,
			},
		}
	case eventTypeUpdate:
		value = &apiv1.HttpBodyUrlEncodedSync_ValueUnion{
			Kind: apiv1.HttpBodyUrlEncodedSync_ValueUnion_KIND_UPDATE,
			Update: &apiv1.HttpBodyUrlEncodedSyncUpdate{
				HttpBodyUrlEncodedId: event.HttpBodyUrlEncoded.GetHttpBodyUrlEncodedId(),
			},
		}
	case eventTypeDelete:
		value = &apiv1.HttpBodyUrlEncodedSync_ValueUnion{
			Kind: apiv1.HttpBodyUrlEncodedSync_ValueUnion_KIND_DELETE,
			Delete: &apiv1.HttpBodyUrlEncodedSyncDelete{
				HttpBodyUrlEncodedId: event.HttpBodyUrlEncoded.GetHttpBodyUrlEncodedId(),
			},
		}
	}

	return &apiv1.HttpBodyUrlEncodedSyncResponse{
		Items: []*apiv1.HttpBodyUrlEncodedSync{
			{
				Value: value,
			},
		},
	}
}

// streamHttpBodyUrlEncodedSync streams HTTP body URL encoded events to the client
func (h *HttpServiceRPC) streamHttpBodyUrlEncodedSync(ctx context.Context, userID idwrap.IDWrap, send func(*apiv1.HttpBodyUrlEncodedSyncResponse) error) error {
	var workspaceSet sync.Map

	// Snapshot provider for initial state
	snapshot := func(ctx context.Context) ([]eventstream.Event[HttpBodyUrlEncodedTopic, HttpBodyUrlEncodedEvent], error) {
		// Get all HTTP entries for user
		httpList, err := h.listUserHttp(ctx)
		if err != nil {
			return nil, err
		}

		events := make([]eventstream.Event[HttpBodyUrlEncodedTopic, HttpBodyUrlEncodedEvent], 0)
		for _, http := range httpList {
			workspaceSet.Store(http.WorkspaceID.String(), struct{}{})

			// Get body URL encoded for this HTTP entry
			bodyUrlEncodeds, err := h.httpBodyUrlEncodedService.GetHttpBodyUrlEncodedByHttpID(ctx, http.ID)
			if err != nil {
				return nil, err
			}

			for _, bodyUrlEncoded := range bodyUrlEncodeds {
				events = append(events, eventstream.Event[HttpBodyUrlEncodedTopic, HttpBodyUrlEncodedEvent]{
					Topic: HttpBodyUrlEncodedTopic{WorkspaceID: http.WorkspaceID},
					Payload: HttpBodyUrlEncodedEvent{
						Type:               eventTypeCreate,
						HttpBodyUrlEncoded: toAPIHttpBodyUrlEncoded(bodyUrlEncoded),
					},
				})
			}
		}
		return events, nil
	}

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

	// Subscribe to events with snapshot
	events, err := h.httpBodyUrlEncodedStream.Subscribe(ctx, filter, eventstream.WithSnapshot(snapshot))
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
			resp := httpBodyUrlEncodedSyncResponseFrom(evt.Payload)
			if err := send(resp); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

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
		httpList, err := h.hs.GetByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Get body URL encoded for each HTTP entry
		for _, http := range httpList {
			bodyUrlEncodeds, err := h.httpBodyUrlEncodedService.GetHttpBodyUrlEncodedByHttpID(ctx, http.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			// Convert to delta format
			for _, bodyUrlEncoded := range bodyUrlEncodeds {
				delta := &apiv1.HttpBodyUrlEncodedDelta{
					DeltaHttpBodyUrlEncodedId: bodyUrlEncoded.ID.Bytes(),
					HttpBodyUrlEncodedId:      bodyUrlEncoded.ID.Bytes(),
					HttpId:                    bodyUrlEncoded.HttpID.Bytes(),
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

func (h *HttpServiceRPC) HttpBodyUrlEncodedDeltaCreate(ctx context.Context, req *connect.Request[apiv1.HttpBodyUrlEncodedDeltaCreateRequest]) (*connect.Response[emptypb.Empty], error) {
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
		bodyUrlEncoded, err := h.httpBodyUrlEncodedService.GetHttpBodyUrlEncoded(ctx, bodyUrlEncodedID)
		if err != nil {
			if errors.Is(err, shttpbodyurlencoded.ErrNoHttpBodyUrlEncodedFound) {
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
		err = h.httpBodyUrlEncodedService.UpdateHttpBodyUrlEncodedDelta(ctx, bodyUrlEncodedID, item.Key, item.Value, item.Enabled, item.Description, item.Order)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyUrlEncodedDeltaUpdate(ctx context.Context, req *connect.Request[apiv1.HttpBodyUrlEncodedDeltaUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	// Stub implementation - delta updates are handled via HttpBodyUrlEncodedDeltaCreate
	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyUrlEncodedDeltaDelete(ctx context.Context, req *connect.Request[apiv1.HttpBodyUrlEncodedDeltaDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	// Stub implementation - delta deletion is not supported
	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyUrlEncodedDeltaSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpBodyUrlEncodedDeltaSyncResponse]) error {
	// Stub implementation - delta sync is not implemented
	return connect.NewError(connect.CodeUnimplemented, errors.New("HttpBodyUrlEncodedDeltaSync not implemented yet"))
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
			body, err := h.bodyService.GetBodyRawByExampleID(ctx, http.ID)
			if err != nil && !errors.Is(err, sbodyraw.ErrNoBodyRawFound) {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			if body != nil {
				bodyRaw := &apiv1.HttpBodyRaw{
					HttpId: http.ID.Bytes(),
					Data:   string(body.Data), // Convert []byte to string
				}
				allBodies = append(allBodies, bodyRaw)
			}
		}
	}

	return connect.NewResponse(&apiv1.HttpBodyRawCollectionResponse{
		Items: allBodies,
	}), nil
}

func (h *HttpServiceRPC) HttpBodyRawCreate(ctx context.Context, req *connect.Request[apiv1.HttpBodyRawCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpBodyRawUpdate(ctx context.Context, req *connect.Request[apiv1.HttpBodyRawUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpBodyRawSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpBodyRawSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

// Helper methods for HTTP request execution

// parseHttpMethod converts string method to HttpMethod enum
func parseHttpMethod(method string) apiv1.HttpMethod {
	switch strings.ToUpper(method) {
	case "GET":
		return apiv1.HttpMethod_HTTP_METHOD_GET
	case "POST":
		return apiv1.HttpMethod_HTTP_METHOD_POST
	case "PUT":
		return apiv1.HttpMethod_HTTP_METHOD_PUT
	case "PATCH":
		return apiv1.HttpMethod_HTTP_METHOD_PATCH
	case "DELETE":
		return apiv1.HttpMethod_HTTP_METHOD_DELETE
	case "HEAD":
		return apiv1.HttpMethod_HTTP_METHOD_HEAD
	case "OPTION":
		return apiv1.HttpMethod_HTTP_METHOD_OPTION
	case "CONNECT":
		return apiv1.HttpMethod_HTTP_METHOD_CONNECT
	default:
		return apiv1.HttpMethod_HTTP_METHOD_UNSPECIFIED
	}
}

// httpMethodToString converts HttpMethod enum to string
func httpMethodToString(method *apiv1.HttpMethod) *string {
	if method == nil {
		return nil
	}

	var result string
	switch *method {
	case apiv1.HttpMethod_HTTP_METHOD_GET:
		result = "GET"
	case apiv1.HttpMethod_HTTP_METHOD_POST:
		result = "POST"
	case apiv1.HttpMethod_HTTP_METHOD_PUT:
		result = "PUT"
	case apiv1.HttpMethod_HTTP_METHOD_PATCH:
		result = "PATCH"
	case apiv1.HttpMethod_HTTP_METHOD_DELETE:
		result = "DELETE"
	case apiv1.HttpMethod_HTTP_METHOD_HEAD:
		result = "HEAD"
	case apiv1.HttpMethod_HTTP_METHOD_OPTION:
		result = "OPTION"
	case apiv1.HttpMethod_HTTP_METHOD_CONNECT:
		result = "CONNECT"
	default:
		result = ""
	}
	return &result
}

// loadHttpHeaders loads HTTP headers for the given HTTP ID
func (h *HttpServiceRPC) loadHttpHeaders(ctx context.Context, httpID idwrap.IDWrap) ([]mexampleheader.Header, error) {
	return h.headerService.GetHeaderByExampleID(ctx, httpID)
}

// loadHttpQueries loads HTTP queries for the given HTTP ID
func (h *HttpServiceRPC) loadHttpQueries(ctx context.Context, httpID idwrap.IDWrap) ([]mexamplequery.Query, error) {
	return h.queryService.GetExampleQueriesByExampleID(ctx, httpID)
}

// loadHttpBody loads HTTP body for the given HTTP ID
func (h *HttpServiceRPC) loadHttpBody(ctx context.Context, httpID idwrap.IDWrap) ([]byte, error) {
	body, err := h.bodyService.GetBodyRawByExampleID(ctx, httpID)
	if err != nil {
		if err == sbodyraw.ErrNoBodyRawFound {
			return nil, nil // No body is valid
		}
		return nil, err
	}
	return body.Data, nil
}

// storeHttpResponse stores the HTTP response in the database
func (h *HttpServiceRPC) storeHttpResponse(ctx context.Context, httpID idwrap.IDWrap, resp httpclient.Response) error {
	// Create response model
	responseModel := mexampleresp.ExampleResp{
		ID:               idwrap.NewNow(), // Generate new ID for response
		ExampleID:        httpID,
		Status:           uint16(resp.StatusCode),
		Body:             resp.Body,
		BodyCompressType: mexampleresp.BodyCompressTypeNone, // No compression for now
		Duration:         0,                                 // TODO: Capture actual request duration
	}

	// Store using response service
	err := h.respService.CreateExampleResp(ctx, responseModel)
	if err != nil {
		return fmt.Errorf("failed to store HTTP response: %w", err)
	}

	log.Printf("Stored HTTP Response for %s: Status=%d, Content-Length=%d", httpID.String(), resp.StatusCode, len(resp.Body))
	return nil
}
