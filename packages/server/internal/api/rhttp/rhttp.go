package rhttp

import (
	"database/sql"

	"connectrpc.com/connect"

	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/pkg/eventstream"
	"the-dev-tools/server/pkg/http/resolver"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/shttpassert"
	"the-dev-tools/server/pkg/service/shttpbodyform"
	"the-dev-tools/server/pkg/service/shttpbodyurlencoded"
	"the-dev-tools/server/pkg/service/shttpheader"
	"the-dev-tools/server/pkg/service/shttpsearchparam"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/svar"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/service/sworkspacesusers"

	apiv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
	httpv1connect "the-dev-tools/spec/dist/buf/go/api/http/v1/httpv1connect"
)

const (
	eventTypeInsert = "insert"
	eventTypeUpdate = "update"
	eventTypeDelete = "delete"
)

// HttpTopic defines the streaming topic for HTTP events
type HttpTopic struct {
	WorkspaceID idwrap.IDWrap
}

// HttpEvent defines the event payload for HTTP streaming
type HttpEvent struct {
	Type    string
	IsDelta bool
	Http    *apiv1.Http
}

// HttpHeaderTopic defines the streaming topic for HTTP header events
type HttpHeaderTopic struct {
	WorkspaceID idwrap.IDWrap
}

// HttpHeaderEvent defines the event payload for HTTP header streaming
type HttpHeaderEvent struct {
	Type       string
	IsDelta    bool
	HttpHeader *apiv1.HttpHeader
}

// HttpSearchParamTopic defines the streaming topic for HTTP search param events
type HttpSearchParamTopic struct {
	WorkspaceID idwrap.IDWrap
}

// HttpSearchParamEvent defines the event payload for HTTP search param streaming
type HttpSearchParamEvent struct {
	Type            string
	IsDelta         bool
	HttpSearchParam *apiv1.HttpSearchParam
}

// HttpBodyFormTopic defines the streaming topic for HTTP body form events
type HttpBodyFormTopic struct {
	WorkspaceID idwrap.IDWrap
}

// HttpBodyFormEvent defines the event payload for HTTP body form streaming
type HttpBodyFormEvent struct {
	Type         string
	IsDelta      bool
	HttpBodyForm *apiv1.HttpBodyFormData
}

// HttpBodyUrlEncodedTopic defines the streaming topic for HTTP body URL encoded events
type HttpBodyUrlEncodedTopic struct {
	WorkspaceID idwrap.IDWrap
}

// HttpBodyUrlEncodedEvent defines the event payload for HTTP body URL encoded streaming
type HttpBodyUrlEncodedEvent struct {
	Type               string
	IsDelta            bool
	HttpBodyUrlEncoded *apiv1.HttpBodyUrlEncoded
}

// HttpAssertTopic defines the streaming topic for HTTP assert events
type HttpAssertTopic struct {
	WorkspaceID idwrap.IDWrap
}

// HttpAssertEvent defines the event payload for HTTP assert streaming
type HttpAssertEvent struct {
	Type       string
	IsDelta    bool
	HttpAssert *apiv1.HttpAssert
}

// HttpVersionTopic defines the streaming topic for HTTP version events
type HttpVersionTopic struct {
	WorkspaceID idwrap.IDWrap
}

// HttpVersionEvent defines the event payload for HTTP version streaming
type HttpVersionEvent struct {
	Type        string
	HttpVersion *apiv1.HttpVersion
}

// HttpResponseTopic defines the streaming topic for HTTP response events
type HttpResponseTopic struct {
	WorkspaceID idwrap.IDWrap
}

// HttpResponseEvent defines the event payload for HTTP response streaming
type HttpResponseEvent struct {
	Type         string
	HttpResponse *apiv1.HttpResponse
}

// HttpResponseHeaderTopic defines the streaming topic for HTTP response header events
type HttpResponseHeaderTopic struct {
	WorkspaceID idwrap.IDWrap
}

// HttpResponseHeaderEvent defines the event payload for HTTP response header streaming
type HttpResponseHeaderEvent struct {
	Type               string
	HttpResponseHeader *apiv1.HttpResponseHeader
}

// HttpResponseAssertTopic defines the streaming topic for HTTP response assert events
type HttpResponseAssertTopic struct {
	WorkspaceID idwrap.IDWrap
}

// HttpResponseAssertEvent defines the event payload for HTTP response assert streaming
type HttpResponseAssertEvent struct {
	Type               string
	HttpResponseAssert *apiv1.HttpResponseAssert
}

// HttpBodyRawTopic defines the streaming topic for HTTP body raw events
type HttpBodyRawTopic struct {
	WorkspaceID idwrap.IDWrap
}

// HttpBodyRawEvent defines the event payload for HTTP body raw streaming
type HttpBodyRawEvent struct {
	Type        string
	IsDelta     bool
	HttpBodyRaw *apiv1.HttpBodyRaw
}

// HttpServiceRPC handles HTTP RPC operations with streaming support
type HttpServiceRPC struct {
	DB *sql.DB

	hs  shttp.HTTPService
	us  suser.UserService
	ws  sworkspace.WorkspaceService
	wus sworkspacesusers.WorkspaceUserService

	// Environment and variable services
	es senv.EnvService
	vs svar.VarService

	// Additional services for HTTP components
	bodyService         *shttp.HttpBodyRawService
	httpResponseService shttp.HttpResponseService

	// Resolver for delta request resolution
	resolver resolver.RequestResolver

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
	httpVersionStream        eventstream.SyncStreamer[HttpVersionTopic, HttpVersionEvent]

	// Streamers for response entities
	httpResponseStream       eventstream.SyncStreamer[HttpResponseTopic, HttpResponseEvent]
	httpResponseHeaderStream eventstream.SyncStreamer[HttpResponseHeaderTopic, HttpResponseHeaderEvent]
	httpResponseAssertStream eventstream.SyncStreamer[HttpResponseAssertTopic, HttpResponseAssertEvent]
	httpBodyRawStream        eventstream.SyncStreamer[HttpBodyRawTopic, HttpBodyRawEvent]
}

// New creates a new HttpServiceRPC instance
func New(
	db *sql.DB,
	hs shttp.HTTPService,
	us suser.UserService,
	ws sworkspace.WorkspaceService,
	wus sworkspacesusers.WorkspaceUserService,
	es senv.EnvService,
	vs svar.VarService,
	bodyService *shttp.HttpBodyRawService,
	httpHeaderService shttpheader.HttpHeaderService,
	httpSearchParamService shttpsearchparam.HttpSearchParamService,
	httpBodyFormService shttpbodyform.HttpBodyFormService,
	httpBodyUrlEncodedService shttpbodyurlencoded.HttpBodyUrlEncodedService,
	httpAssertService shttpassert.HttpAssertService,
	httpResponseService shttp.HttpResponseService,
	requestResolver resolver.RequestResolver,
	stream eventstream.SyncStreamer[HttpTopic, HttpEvent],
	httpHeaderStream eventstream.SyncStreamer[HttpHeaderTopic, HttpHeaderEvent],
	httpSearchParamStream eventstream.SyncStreamer[HttpSearchParamTopic, HttpSearchParamEvent],
	httpBodyFormStream eventstream.SyncStreamer[HttpBodyFormTopic, HttpBodyFormEvent],
	httpBodyUrlEncodedStream eventstream.SyncStreamer[HttpBodyUrlEncodedTopic, HttpBodyUrlEncodedEvent],
	httpAssertStream eventstream.SyncStreamer[HttpAssertTopic, HttpAssertEvent],
	httpVersionStream eventstream.SyncStreamer[HttpVersionTopic, HttpVersionEvent],
	httpResponseStream eventstream.SyncStreamer[HttpResponseTopic, HttpResponseEvent],
	httpResponseHeaderStream eventstream.SyncStreamer[HttpResponseHeaderTopic, HttpResponseHeaderEvent],
	httpResponseAssertStream eventstream.SyncStreamer[HttpResponseAssertTopic, HttpResponseAssertEvent],
	httpBodyRawStream eventstream.SyncStreamer[HttpBodyRawTopic, HttpBodyRawEvent],
) HttpServiceRPC {
	return HttpServiceRPC{
		DB:                        db,
		hs:                        hs,
		us:                        us,
		ws:                        ws,
		wus:                       wus,
		es:                        es,
		vs:                        vs,
		bodyService:               bodyService,
		httpHeaderService:         httpHeaderService,
		httpSearchParamService:    httpSearchParamService,
		httpBodyFormService:       httpBodyFormService,
		httpBodyUrlEncodedService: httpBodyUrlEncodedService,
		httpAssertService:         httpAssertService,
		httpResponseService:       httpResponseService,
		resolver:                  requestResolver,
		stream:                    stream,
		httpHeaderStream:          httpHeaderStream,
		httpSearchParamStream:     httpSearchParamStream,
		httpBodyFormStream:        httpBodyFormStream,
		httpBodyUrlEncodedStream:  httpBodyUrlEncodedStream,
		httpAssertStream:          httpAssertStream,
		httpVersionStream:         httpVersionStream,
		httpResponseStream:        httpResponseStream,
		httpResponseHeaderStream:  httpResponseHeaderStream,
		httpResponseAssertStream:  httpResponseAssertStream,
		httpBodyRawStream:         httpBodyRawStream,
	}
}

// CreateService creates the HTTP service with Connect handler
func CreateService(srv HttpServiceRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := httpv1connect.NewHttpServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

// publishInsertEvent publishes an insert event for real-time sync
