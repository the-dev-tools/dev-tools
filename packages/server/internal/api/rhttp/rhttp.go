//nolint:revive // exported
package rhttp

import (
	"database/sql"
	"fmt"

	"connectrpc.com/connect"

	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/rlog"
	"the-dev-tools/server/pkg/eventstream"
	"the-dev-tools/server/pkg/http/resolver"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/patch"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	httpv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
	"the-dev-tools/spec/dist/buf/go/api/http/v1/httpv1connect"
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
	Patch   patch.HTTPDeltaPatch
	Http    *httpv1.Http
}

// HttpHeaderTopic defines the streaming topic for HTTP header events
type HttpHeaderTopic struct {
	WorkspaceID idwrap.IDWrap
}

// HttpHeaderEvent defines the event payload for HTTP header streaming
type HttpHeaderEvent struct {
	Type       string
	IsDelta    bool
	Patch      patch.HTTPHeaderPatch
	HttpHeader *httpv1.HttpHeader
}

// HttpSearchParamTopic defines the streaming topic for HTTP search param events
type HttpSearchParamTopic struct {
	WorkspaceID idwrap.IDWrap
}

// HttpSearchParamEvent defines the event payload for HTTP search param streaming
type HttpSearchParamEvent struct {
	Type            string
	IsDelta         bool
	Patch           patch.HTTPSearchParamPatch
	HttpSearchParam *httpv1.HttpSearchParam
}

// HttpBodyFormTopic defines the streaming topic for HTTP body form events
type HttpBodyFormTopic struct {
	WorkspaceID idwrap.IDWrap
}

// HttpBodyFormEvent defines the event payload for HTTP body form streaming
type HttpBodyFormEvent struct {
	Type         string
	IsDelta      bool
	Patch        patch.HTTPBodyFormPatch
	HttpBodyForm *httpv1.HttpBodyFormData
}

// HttpBodyUrlEncodedTopic defines the streaming topic for HTTP body URL encoded events
type HttpBodyUrlEncodedTopic struct {
	WorkspaceID idwrap.IDWrap
}

// HttpBodyUrlEncodedEvent defines the event payload for HTTP body URL encoded streaming
type HttpBodyUrlEncodedEvent struct {
	Type               string
	IsDelta            bool
	Patch              patch.HTTPBodyUrlEncodedPatch
	HttpBodyUrlEncoded *httpv1.HttpBodyUrlEncoded
}

// HttpAssertTopic defines the streaming topic for HTTP assert events
type HttpAssertTopic struct {
	WorkspaceID idwrap.IDWrap
}

// HttpAssertEvent defines the event payload for HTTP assert streaming
type HttpAssertEvent struct {
	Type       string
	IsDelta    bool
	Patch      patch.HTTPAssertPatch
	HttpAssert *httpv1.HttpAssert
}

// HttpVersionTopic defines the streaming topic for HTTP version events
type HttpVersionTopic struct {
	WorkspaceID idwrap.IDWrap
}

// HttpVersionEvent defines the event payload for HTTP version streaming
type HttpVersionEvent struct {
	Type        string
	HttpVersion *httpv1.HttpVersion
}

// HttpResponseTopic defines the streaming topic for HTTP response events
type HttpResponseTopic struct {
	WorkspaceID idwrap.IDWrap
}

// HttpResponseEvent defines the event payload for HTTP response streaming
type HttpResponseEvent struct {
	Type         string
	HttpResponse *httpv1.HttpResponse
}

// HttpResponseHeaderTopic defines the streaming topic for HTTP response header events
type HttpResponseHeaderTopic struct {
	WorkspaceID idwrap.IDWrap
}

// HttpResponseHeaderEvent defines the event payload for HTTP response header streaming
type HttpResponseHeaderEvent struct {
	Type               string
	HttpResponseHeader *httpv1.HttpResponseHeader
}

// HttpResponseAssertTopic defines the streaming topic for HTTP response assert events
type HttpResponseAssertTopic struct {
	WorkspaceID idwrap.IDWrap
}

// HttpResponseAssertEvent defines the event payload for HTTP response assert streaming
type HttpResponseAssertEvent struct {
	Type               string
	HttpResponseAssert *httpv1.HttpResponseAssert
}

// HttpBodyRawTopic defines the streaming topic for HTTP body raw events
type HttpBodyRawTopic struct {
	WorkspaceID idwrap.IDWrap
}

// HttpBodyRawEvent defines the event payload for HTTP body raw streaming
type HttpBodyRawEvent struct {
	Type        string
	IsDelta     bool
	Patch       patch.HTTPBodyRawPatch
	HttpBodyRaw *httpv1.HttpBodyRaw
}

// HttpStreamers groups all event streams used by the HTTP service
type HttpStreamers struct {
	Http               eventstream.SyncStreamer[HttpTopic, HttpEvent]
	HttpHeader         eventstream.SyncStreamer[HttpHeaderTopic, HttpHeaderEvent]
	HttpSearchParam    eventstream.SyncStreamer[HttpSearchParamTopic, HttpSearchParamEvent]
	HttpBodyForm       eventstream.SyncStreamer[HttpBodyFormTopic, HttpBodyFormEvent]
	HttpBodyUrlEncoded eventstream.SyncStreamer[HttpBodyUrlEncodedTopic, HttpBodyUrlEncodedEvent]
	HttpAssert         eventstream.SyncStreamer[HttpAssertTopic, HttpAssertEvent]
	HttpVersion        eventstream.SyncStreamer[HttpVersionTopic, HttpVersionEvent]
	HttpResponse       eventstream.SyncStreamer[HttpResponseTopic, HttpResponseEvent]
	HttpResponseHeader eventstream.SyncStreamer[HttpResponseHeaderTopic, HttpResponseHeaderEvent]
	HttpResponseAssert eventstream.SyncStreamer[HttpResponseAssertTopic, HttpResponseAssertEvent]
	HttpBodyRaw        eventstream.SyncStreamer[HttpBodyRawTopic, HttpBodyRawEvent]
	Log                eventstream.SyncStreamer[rlog.LogTopic, rlog.LogEvent]
}

// HttpServiceRPC handles HTTP RPC operations with streaming support
type HttpServiceRPC struct {
	DB *sql.DB

	httpReader *shttp.Reader
	hs         shttp.HTTPService
	us         suser.UserService
	ws         sworkspace.WorkspaceService
	wus        sworkspace.UserService
	userReader *sworkspace.UserReader
	wsReader   *sworkspace.WorkspaceReader

	es senv.EnvService
	vs senv.VariableService

	bodyService               *shttp.HttpBodyRawService
	httpHeaderService         shttp.HttpHeaderService
	httpSearchParamService    *shttp.HttpSearchParamService
	httpBodyFormService       *shttp.HttpBodyFormService
	httpBodyUrlEncodedService *shttp.HttpBodyUrlEncodedService
	httpAssertService         *shttp.HttpAssertService
	httpResponseService       shttp.HttpResponseService

	resolver resolver.RequestResolver

	// Streamers
	streamers *HttpStreamers
}

type HttpServiceRPCReaders struct {
	Http      *shttp.Reader
	User      *sworkspace.UserReader
	Workspace *sworkspace.WorkspaceReader
}

func (r *HttpServiceRPCReaders) Validate() error {
	if r.Http == nil { return fmt.Errorf("http reader is required") }
	if r.User == nil { return fmt.Errorf("user reader is required") }
	if r.Workspace == nil { return fmt.Errorf("workspace reader is required") }
	return nil
}

type HttpServiceRPCServices struct {
	Http               shttp.HTTPService
	User               suser.UserService
	Workspace          sworkspace.WorkspaceService
	WorkspaceUser      sworkspace.UserService
	Env                senv.EnvService
	Variable           senv.VariableService
	HttpBodyRaw        *shttp.HttpBodyRawService
	HttpHeader         shttp.HttpHeaderService
	HttpSearchParam    *shttp.HttpSearchParamService
	HttpBodyForm       *shttp.HttpBodyFormService
	HttpBodyUrlEncoded *shttp.HttpBodyUrlEncodedService
	HttpAssert         *shttp.HttpAssertService
	HttpResponse       shttp.HttpResponseService
}

func (s *HttpServiceRPCServices) Validate() error {
	if s.HttpBodyRaw == nil { return fmt.Errorf("http body raw service is required") }
	if s.HttpSearchParam == nil { return fmt.Errorf("http search param service is required") }
	if s.HttpBodyForm == nil { return fmt.Errorf("http body form service is required") }
	if s.HttpBodyUrlEncoded == nil { return fmt.Errorf("http body url encoded service is required") }
	if s.HttpAssert == nil { return fmt.Errorf("http assert service is required") }
	return nil
}

type HttpServiceRPCDeps struct {
	DB        *sql.DB
	Readers   HttpServiceRPCReaders
	Services  HttpServiceRPCServices
	Resolver  resolver.RequestResolver
	Streamers *HttpStreamers
}

func (d *HttpServiceRPCDeps) Validate() error {
	if d.DB == nil { return fmt.Errorf("db is required") }
	if err := d.Readers.Validate(); err != nil { return err }
	if err := d.Services.Validate(); err != nil { return err }
	if d.Resolver == nil { return fmt.Errorf("resolver is required") }
	if d.Streamers == nil { return fmt.Errorf("streamers is required") }
	return nil
}

// New creates a new HttpServiceRPC instance
func New(deps HttpServiceRPCDeps) HttpServiceRPC {
	if err := deps.Validate(); err != nil {
		panic(fmt.Sprintf("HttpServiceRPC Deps validation failed: %v", err))
	}

	return HttpServiceRPC{
		DB:                        deps.DB,
		httpReader:                deps.Readers.Http,
		hs:                        deps.Services.Http,
		us:                        deps.Services.User,
		ws:                        deps.Services.Workspace,
		wus:                       deps.Services.WorkspaceUser,
		userReader:                deps.Readers.User,
		wsReader:                  deps.Readers.Workspace,
		es:                        deps.Services.Env,
		vs:                        deps.Services.Variable,
		bodyService:               deps.Services.HttpBodyRaw,
		httpHeaderService:         deps.Services.HttpHeader,
		httpSearchParamService:    deps.Services.HttpSearchParam,
		httpBodyFormService:       deps.Services.HttpBodyForm,
		httpBodyUrlEncodedService: deps.Services.HttpBodyUrlEncoded,
		httpAssertService:         deps.Services.HttpAssert,
		httpResponseService:       deps.Services.HttpResponse,
		resolver:                  deps.Resolver,
		streamers:                 deps.Streamers,
	}
}

// CreateService creates the HTTP service with Connect handler
func CreateService(srv HttpServiceRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := httpv1connect.NewHttpServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}