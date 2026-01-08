//nolint:revive // exported
package rhttp

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/api"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rfile"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rlog"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/converter"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/http/resolver"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/mutation"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/patch"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/senv"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sfile"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/shttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/suser"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
	httpv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/http/v1"
	"github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/http/v1/httpv1connect"
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
	File               eventstream.SyncStreamer[rfile.FileTopic, rfile.FileEvent]
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

	// File service and stream for sidebar integration
	fileService *sfile.FileService
	fileStream  eventstream.SyncStreamer[rfile.FileTopic, rfile.FileEvent]

	// Streamers
	streamers *HttpStreamers
}

type HttpServiceRPCReaders struct {
	Http      *shttp.Reader
	User      *sworkspace.UserReader
	Workspace *sworkspace.WorkspaceReader
}

func (r *HttpServiceRPCReaders) Validate() error {
	if r.Http == nil {
		return fmt.Errorf("http reader is required")
	}
	if r.User == nil {
		return fmt.Errorf("user reader is required")
	}
	if r.Workspace == nil {
		return fmt.Errorf("workspace reader is required")
	}
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
	File               *sfile.FileService
}

func (s *HttpServiceRPCServices) Validate() error {
	if s.HttpBodyRaw == nil {
		return fmt.Errorf("http body raw service is required")
	}
	if s.HttpSearchParam == nil {
		return fmt.Errorf("http search param service is required")
	}
	if s.HttpBodyForm == nil {
		return fmt.Errorf("http body form service is required")
	}
	if s.HttpBodyUrlEncoded == nil {
		return fmt.Errorf("http body url encoded service is required")
	}
	if s.HttpAssert == nil {
		return fmt.Errorf("http assert service is required")
	}
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
	if d.DB == nil {
		return fmt.Errorf("db is required")
	}
	if err := d.Readers.Validate(); err != nil {
		return err
	}
	if err := d.Services.Validate(); err != nil {
		return err
	}
	if d.Resolver == nil {
		return fmt.Errorf("resolver is required")
	}
	if d.Streamers == nil {
		return fmt.Errorf("streamers is required")
	}
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
		fileService:               deps.Services.File,
		fileStream:                deps.Streamers.File,
		streamers:                 deps.Streamers,
	}
}

// CreateService creates the HTTP service with Connect handler
func CreateService(srv HttpServiceRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := httpv1connect.NewHttpServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

// mutationPublisher returns a unified publisher for HTTP-related mutation events.
func (s *HttpServiceRPC) mutationPublisher() mutation.Publisher {
	return &rhttpPublisher{
		streamers: s.streamers,
	}
}

type rhttpPublisher struct {
	streamers *HttpStreamers
}

func (p *rhttpPublisher) PublishAll(events []mutation.Event) {
	for _, evt := range events {
		//nolint:exhaustive
		switch evt.Entity {
		case mutation.EntityHTTP:
			p.publishHTTP(evt)
		case mutation.EntityHTTPHeader:
			p.publishHeader(evt)
		case mutation.EntityHTTPParam:
			p.publishParam(evt)
		case mutation.EntityHTTPAssert:
			p.publishAssert(evt)
		case mutation.EntityHTTPBodyRaw:
			p.publishBodyRaw(evt)
		case mutation.EntityHTTPBodyForm:
			p.publishBodyForm(evt)
		case mutation.EntityHTTPBodyURL:
			p.publishBodyUrlEncoded(evt)
		case mutation.EntityHTTPVersion:
			p.publishVersion(evt)
		}
	}
}

func (p *rhttpPublisher) publishHTTP(evt mutation.Event) {
	if p.streamers.Http == nil {
		return
	}
	var httpModel *httpv1.Http
	var eventType string

	switch evt.Op {
	case mutation.OpInsert, mutation.OpUpdate:
		if evt.Op == mutation.OpInsert {
			eventType = eventTypeInsert
		} else {
			eventType = eventTypeUpdate
		}
		if h, ok := evt.Payload.(mhttp.HTTP); ok {
			httpModel = converter.ToAPIHttp(h)
		} else if hp, ok := evt.Payload.(*mhttp.HTTP); ok {
			httpModel = converter.ToAPIHttp(*hp)
		}
	case mutation.OpDelete:
		eventType = eventTypeDelete
		httpModel = &httpv1.Http{
			HttpId: evt.ID.Bytes(),
		}
	}

	if httpModel != nil {
		event := HttpEvent{
			Type:    eventType,
			IsDelta: evt.IsDelta,
			Http:    httpModel,
		}
		if p, ok := evt.Patch.(patch.HTTPDeltaPatch); ok {
			event.Patch = p
		}
		p.streamers.Http.Publish(HttpTopic{WorkspaceID: evt.WorkspaceID}, event)
	}
}

func (p *rhttpPublisher) publishHeader(evt mutation.Event) {
	if p.streamers.HttpHeader == nil {
		return
	}
	var headerModel *httpv1.HttpHeader
	var eventType string

	switch evt.Op {
	case mutation.OpInsert, mutation.OpUpdate:
		eventType = eventTypeInsert
		if evt.Op == mutation.OpUpdate {
			eventType = eventTypeUpdate
		}
		if h, ok := evt.Payload.(mhttp.HTTPHeader); ok {
			headerModel = converter.ToAPIHttpHeader(h)
		} else if hp, ok := evt.Payload.(*mhttp.HTTPHeader); ok {
			headerModel = converter.ToAPIHttpHeader(*hp)
		}
	case mutation.OpDelete:
		eventType = eventTypeDelete
		headerModel = &httpv1.HttpHeader{
			HttpHeaderId: evt.ID.Bytes(),
			HttpId:       evt.ParentID.Bytes(),
		}
	}

	if headerModel != nil {
		event := HttpHeaderEvent{
			Type:       eventType,
			IsDelta:    evt.IsDelta,
			HttpHeader: headerModel,
		}
		if patch, ok := evt.Patch.(patch.HTTPHeaderPatch); ok {
			event.Patch = patch
		}
		p.streamers.HttpHeader.Publish(HttpHeaderTopic{WorkspaceID: evt.WorkspaceID}, event)
	}
}

func (p *rhttpPublisher) publishParam(evt mutation.Event) {
	if p.streamers.HttpSearchParam == nil {
		return
	}
	var paramModel *httpv1.HttpSearchParam
	var eventType string

	switch evt.Op {
	case mutation.OpInsert, mutation.OpUpdate:
		eventType = eventTypeInsert
		if evt.Op == mutation.OpUpdate {
			eventType = eventTypeUpdate
		}
		if pr, ok := evt.Payload.(mhttp.HTTPSearchParam); ok {
			paramModel = converter.ToAPIHttpSearchParam(pr)
		} else if prp, ok := evt.Payload.(*mhttp.HTTPSearchParam); ok {
			paramModel = converter.ToAPIHttpSearchParam(*prp)
		}
	case mutation.OpDelete:
		eventType = eventTypeDelete
		paramModel = &httpv1.HttpSearchParam{
			HttpSearchParamId: evt.ID.Bytes(),
			HttpId:            evt.ParentID.Bytes(),
		}
	}

	if paramModel != nil {
		event := HttpSearchParamEvent{
			Type:            eventType,
			IsDelta:         evt.IsDelta,
			HttpSearchParam: paramModel,
		}
		if patch, ok := evt.Patch.(patch.HTTPSearchParamPatch); ok {
			event.Patch = patch
		}
		p.streamers.HttpSearchParam.Publish(HttpSearchParamTopic{WorkspaceID: evt.WorkspaceID}, event)
	}
}

func (p *rhttpPublisher) publishAssert(evt mutation.Event) {
	if p.streamers.HttpAssert == nil {
		return
	}
	var assertModel *httpv1.HttpAssert
	var eventType string

	switch evt.Op {
	case mutation.OpInsert, mutation.OpUpdate:
		eventType = eventTypeInsert
		if evt.Op == mutation.OpUpdate {
			eventType = eventTypeUpdate
		}
		if a, ok := evt.Payload.(mhttp.HTTPAssert); ok {
			assertModel = converter.ToAPIHttpAssert(a)
		} else if ap, ok := evt.Payload.(*mhttp.HTTPAssert); ok {
			assertModel = converter.ToAPIHttpAssert(*ap)
		}
	case mutation.OpDelete:
		eventType = eventTypeDelete
		assertModel = &httpv1.HttpAssert{
			HttpAssertId: evt.ID.Bytes(),
			HttpId:       evt.ParentID.Bytes(),
		}
	}

	if assertModel != nil {
		event := HttpAssertEvent{
			Type:       eventType,
			IsDelta:    evt.IsDelta,
			HttpAssert: assertModel,
		}
		if patch, ok := evt.Patch.(patch.HTTPAssertPatch); ok {
			event.Patch = patch
		}
		p.streamers.HttpAssert.Publish(HttpAssertTopic{WorkspaceID: evt.WorkspaceID}, event)
	}
}

func (p *rhttpPublisher) publishBodyRaw(evt mutation.Event) {
	if p.streamers.HttpBodyRaw == nil {
		return
	}
	var bodyModel *httpv1.HttpBodyRaw
	var eventType string

	switch evt.Op {
	case mutation.OpInsert, mutation.OpUpdate:
		eventType = eventTypeInsert
		if evt.Op == mutation.OpUpdate {
			eventType = eventTypeUpdate
		}
		if b, ok := evt.Payload.(mhttp.HTTPBodyRaw); ok {
			bodyModel = converter.ToAPIHttpBodyRawFromMHttp(b)
		} else if bp, ok := evt.Payload.(*mhttp.HTTPBodyRaw); ok {
			bodyModel = converter.ToAPIHttpBodyRawFromMHttp(*bp)
		}
	case mutation.OpDelete:
		eventType = eventTypeDelete
		bodyModel = &httpv1.HttpBodyRaw{
			HttpId: evt.ParentID.Bytes(),
		}
	}

	if bodyModel != nil {
		event := HttpBodyRawEvent{
			Type:        eventType,
			IsDelta:     evt.IsDelta,
			HttpBodyRaw: bodyModel,
		}
		if patch, ok := evt.Patch.(patch.HTTPBodyRawPatch); ok {
			event.Patch = patch
		}
		p.streamers.HttpBodyRaw.Publish(HttpBodyRawTopic{WorkspaceID: evt.WorkspaceID}, event)
	}
}

func (p *rhttpPublisher) publishBodyForm(evt mutation.Event) {
	if p.streamers.HttpBodyForm == nil {
		return
	}
	var formModel *httpv1.HttpBodyFormData
	var eventType string

	switch evt.Op {
	case mutation.OpInsert, mutation.OpUpdate:
		eventType = eventTypeInsert
		if evt.Op == mutation.OpUpdate {
			eventType = eventTypeUpdate
		}
		if f, ok := evt.Payload.(mhttp.HTTPBodyForm); ok {
			formModel = converter.ToAPIHttpBodyFormData(f)
		} else if fp, ok := evt.Payload.(*mhttp.HTTPBodyForm); ok {
			formModel = converter.ToAPIHttpBodyFormData(*fp)
		}
	case mutation.OpDelete:
		eventType = eventTypeDelete
		formModel = &httpv1.HttpBodyFormData{
			HttpBodyFormDataId: evt.ID.Bytes(),
			HttpId:             evt.ParentID.Bytes(),
		}
	}

	if formModel != nil {
		event := HttpBodyFormEvent{
			Type:         eventType,
			IsDelta:      evt.IsDelta,
			HttpBodyForm: formModel,
		}
		if patch, ok := evt.Patch.(patch.HTTPBodyFormPatch); ok {
			event.Patch = patch
		}
		p.streamers.HttpBodyForm.Publish(HttpBodyFormTopic{WorkspaceID: evt.WorkspaceID}, event)
	}
}

func (p *rhttpPublisher) publishBodyUrlEncoded(evt mutation.Event) {
	if p.streamers.HttpBodyUrlEncoded == nil {
		return
	}
	var urlModel *httpv1.HttpBodyUrlEncoded
	var eventType string

	switch evt.Op {
	case mutation.OpInsert, mutation.OpUpdate:
		eventType = eventTypeInsert
		if evt.Op == mutation.OpUpdate {
			eventType = eventTypeUpdate
		}
		if u, ok := evt.Payload.(mhttp.HTTPBodyUrlencoded); ok {
			urlModel = converter.ToAPIHttpBodyUrlEncoded(u)
		} else if up, ok := evt.Payload.(*mhttp.HTTPBodyUrlencoded); ok {
			urlModel = converter.ToAPIHttpBodyUrlEncoded(*up)
		}
	case mutation.OpDelete:
		eventType = eventTypeDelete
		urlModel = &httpv1.HttpBodyUrlEncoded{
			HttpBodyUrlEncodedId: evt.ID.Bytes(),
			HttpId:               evt.ParentID.Bytes(),
		}
	}

	if urlModel != nil {
		event := HttpBodyUrlEncodedEvent{
			Type:               eventType,
			IsDelta:            evt.IsDelta,
			HttpBodyUrlEncoded: urlModel,
		}
		if patch, ok := evt.Patch.(patch.HTTPBodyUrlEncodedPatch); ok {
			event.Patch = patch
		}
		p.streamers.HttpBodyUrlEncoded.Publish(HttpBodyUrlEncodedTopic{WorkspaceID: evt.WorkspaceID}, event)
	}
}

func (p *rhttpPublisher) publishVersion(evt mutation.Event) {
	if p.streamers.HttpVersion == nil {
		return
	}
	var versionModel *httpv1.HttpVersion
	var eventType string

	switch evt.Op {
	case mutation.OpInsert, mutation.OpUpdate:
		if evt.Op == mutation.OpInsert {
			eventType = eventTypeInsert
		} else {
			eventType = eventTypeUpdate
		}
		if v, ok := evt.Payload.(mhttp.HttpVersion); ok {
			versionModel = converter.ToAPIHttpVersion(v)
		} else if vp, ok := evt.Payload.(*mhttp.HttpVersion); ok {
			versionModel = converter.ToAPIHttpVersion(*vp)
		}
	case mutation.OpDelete:
		eventType = eventTypeDelete
		versionModel = &httpv1.HttpVersion{
			HttpVersionId: evt.ID.Bytes(),
		}
	}

	if versionModel != nil {
		p.streamers.HttpVersion.Publish(HttpVersionTopic{WorkspaceID: evt.WorkspaceID}, HttpVersionEvent{
			Type:        eventType,
			HttpVersion: versionModel,
		})
	}
}

func (h *HttpServiceRPC) HttpAssertSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[httpv1.HttpAssertSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return h.streamHttpAssertSync(ctx, userID, stream.Send)
}

// streamHttpAssertSync streams HTTP assert events to the client
func (h *HttpServiceRPC) streamHttpAssertSync(ctx context.Context, userID idwrap.IDWrap, send func(*httpv1.HttpAssertSyncResponse) error) error {
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

	converter := func(events []HttpAssertEvent) *httpv1.HttpAssertSyncResponse {
		var items []*httpv1.HttpAssertSync
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
		return &httpv1.HttpAssertSyncResponse{Items: items}
	}

	return eventstream.StreamToClient(
		ctx,
		h.streamers.HttpAssert,
		filter,
		converter,
		send,
		nil,
	)
}

func (h *HttpServiceRPC) HttpBodyFormDataSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[httpv1.HttpBodyFormDataSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return h.streamHttpBodyFormSync(ctx, userID, stream.Send)
}

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

func (h *HttpServiceRPC) HttpBodyRawSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[httpv1.HttpBodyRawSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return h.streamHttpBodyRawSync(ctx, userID, stream.Send)
}

// streamHttpBodyRawSync streams HTTP body raw events to the client
func (h *HttpServiceRPC) streamHttpBodyRawSync(ctx context.Context, userID idwrap.IDWrap, send func(*httpv1.HttpBodyRawSyncResponse) error) error {
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

	converter := func(events []HttpBodyRawEvent) *httpv1.HttpBodyRawSyncResponse {
		var items []*httpv1.HttpBodyRawSync
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
		return &httpv1.HttpBodyRawSyncResponse{Items: items}
	}

	return eventstream.StreamToClient(
		ctx,
		h.streamers.HttpBodyRaw,
		filter,
		converter,
		send,
		nil,
	)
}

func (h *HttpServiceRPC) streamHttpBodyFormSync(ctx context.Context, userID idwrap.IDWrap, send func(*httpv1.HttpBodyFormDataSyncResponse) error) error {
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

	converter := func(events []HttpBodyFormEvent) *httpv1.HttpBodyFormDataSyncResponse {
		var items []*httpv1.HttpBodyFormDataSync
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
		return &httpv1.HttpBodyFormDataSyncResponse{Items: items}
	}

	return eventstream.StreamToClient(
		ctx,
		h.streamers.HttpBodyForm,
		filter,
		converter,
		send,
		nil,
	)
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

func (h *HttpServiceRPC) HttpBodyUrlEncodedSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[httpv1.HttpBodyUrlEncodedSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return h.streamHttpBodyUrlEncodedSync(ctx, userID, stream.Send)
}

func (h *HttpServiceRPC) streamHttpBodyUrlEncodedSync(ctx context.Context, userID idwrap.IDWrap, send func(*httpv1.HttpBodyUrlEncodedSyncResponse) error) error {
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

	converter := func(events []HttpBodyUrlEncodedEvent) *httpv1.HttpBodyUrlEncodedSyncResponse {
		var items []*httpv1.HttpBodyUrlEncodedSync
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
		return &httpv1.HttpBodyUrlEncodedSyncResponse{Items: items}
	}

	return eventstream.StreamToClient(
		ctx,
		h.streamers.HttpBodyUrlEncoded,
		filter,
		converter,
		send,
		nil,
	)
}

func (h *HttpServiceRPC) HttpHeaderSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[httpv1.HttpHeaderSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return h.streamHttpHeaderSync(ctx, userID, stream.Send)
}

func (h *HttpServiceRPC) streamHttpHeaderSync(ctx context.Context, userID idwrap.IDWrap, send func(*httpv1.HttpHeaderSyncResponse) error) error {
	var workspaceSet sync.Map

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

	converter := func(events []HttpHeaderEvent) *httpv1.HttpHeaderSyncResponse {
		var items []*httpv1.HttpHeaderSync
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
		return &httpv1.HttpHeaderSyncResponse{Items: items}
	}

	return eventstream.StreamToClient(
		ctx,
		h.streamers.HttpHeader,
		filter,
		converter,
		send,
		nil,
	)
}

func (h *HttpServiceRPC) streamHttpResponseSync(ctx context.Context, userID idwrap.IDWrap, send func(*httpv1.HttpResponseSyncResponse) error) error {
	var workspaceSet sync.Map

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

	converter := func(events []HttpResponseEvent) *httpv1.HttpResponseSyncResponse {
		var items []*httpv1.HttpResponseSync
		for _, event := range events {
			if resp := httpResponseSyncResponseFrom(event); resp != nil && len(resp.Items) > 0 {
				items = append(items, resp.Items...)
			}
		}
		if len(items) == 0 {
			return nil
		}
		return &httpv1.HttpResponseSyncResponse{Items: items}
	}

	return eventstream.StreamToClient(
		ctx,
		h.streamers.HttpResponse,
		filter,
		converter,
		send,
		nil,
	)
}

func (h *HttpServiceRPC) streamHttpVersionSync(ctx context.Context, userID idwrap.IDWrap, send func(*httpv1.HttpVersionSyncResponse) error) error {
	var workspaceSet sync.Map

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

	converter := func(events []HttpVersionEvent) *httpv1.HttpVersionSyncResponse {
		var items []*httpv1.HttpVersionSync
		for _, event := range events {
			if resp := httpVersionSyncResponseFrom(event); resp != nil && len(resp.Items) > 0 {
				items = append(items, resp.Items...)
			}
		}
		if len(items) == 0 {
			return nil
		}
		return &httpv1.HttpVersionSyncResponse{Items: items}
	}

	return eventstream.StreamToClient(
		ctx,
		h.streamers.HttpVersion,
		filter,
		converter,
		send,
		nil,
	)
}

func (h *HttpServiceRPC) streamHttpSync(ctx context.Context, userID idwrap.IDWrap, send func(*httpv1.HttpSyncResponse) error) error {
	return h.streamHttpSyncWithOptions(ctx, userID, send, nil)
}

func (h *HttpServiceRPC) streamHttpSyncWithOptions(ctx context.Context, userID idwrap.IDWrap, send func(*httpv1.HttpSyncResponse) error, opts *eventstream.BulkOptions) error {
	var workspaceSet sync.Map

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

	converter := func(events []HttpEvent) *httpv1.HttpSyncResponse {
		var items []*httpv1.HttpSync
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
		return &httpv1.HttpSyncResponse{Items: items}
	}

	return eventstream.StreamToClient(
		ctx,
		h.streamers.Http,
		filter,
		converter,
		send,
		opts,
	)
}

func (h *HttpServiceRPC) HttpResponseSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[httpv1.HttpResponseSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return h.streamHttpResponseSync(ctx, userID, stream.Send)
}

func (h *HttpServiceRPC) HttpResponseHeaderSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[httpv1.HttpResponseHeaderSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return h.streamHttpResponseHeaderSync(ctx, userID, stream.Send)
}

func (h *HttpServiceRPC) HttpResponseAssertSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[httpv1.HttpResponseAssertSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return h.streamHttpResponseAssertSync(ctx, userID, stream.Send)
}

func (h *HttpServiceRPC) streamHttpSearchParamSync(ctx context.Context, userID idwrap.IDWrap, send func(*httpv1.HttpSearchParamSyncResponse) error) error {
	var workspaceSet sync.Map

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

	converter := func(events []HttpSearchParamEvent) *httpv1.HttpSearchParamSyncResponse {
		var items []*httpv1.HttpSearchParamSync
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
		return &httpv1.HttpSearchParamSyncResponse{Items: items}
	}

	return eventstream.StreamToClient(
		ctx,
		h.streamers.HttpSearchParam,
		filter,
		converter,
		send,
		nil,
	)
}

func (h *HttpServiceRPC) streamHttpResponseHeaderSync(ctx context.Context, userID idwrap.IDWrap, send func(*httpv1.HttpResponseHeaderSyncResponse) error) error {
	var workspaceSet sync.Map

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

	converter := func(events []HttpResponseHeaderEvent) *httpv1.HttpResponseHeaderSyncResponse {
		var items []*httpv1.HttpResponseHeaderSync
		for _, event := range events {
			if resp := httpResponseHeaderSyncResponseFrom(event); resp != nil && len(resp.Items) > 0 {
				items = append(items, resp.Items...)
			}
		}
		if len(items) == 0 {
			return nil
		}
		return &httpv1.HttpResponseHeaderSyncResponse{Items: items}
	}

	return eventstream.StreamToClient(
		ctx,
		h.streamers.HttpResponseHeader,
		filter,
		converter,
		send,
		nil,
	)
}

func (h *HttpServiceRPC) streamHttpResponseAssertSync(ctx context.Context, userID idwrap.IDWrap, send func(*httpv1.HttpResponseAssertSyncResponse) error) error {
	var workspaceSet sync.Map

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

	converter := func(events []HttpResponseAssertEvent) *httpv1.HttpResponseAssertSyncResponse {
		var items []*httpv1.HttpResponseAssertSync
		for _, event := range events {
			if resp := httpResponseAssertSyncResponseFrom(event); resp != nil && len(resp.Items) > 0 {
				items = append(items, resp.Items...)
			}
		}
		if len(items) == 0 {
			return nil
		}
		return &httpv1.HttpResponseAssertSyncResponse{Items: items}
	}

	return eventstream.StreamToClient(
		ctx,
		h.streamers.HttpResponseAssert,
		filter,
		converter,
		send,
		nil,
	)
}

func (h *HttpServiceRPC) HttpSearchParamSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[httpv1.HttpSearchParamSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return h.streamHttpSearchParamSync(ctx, userID, stream.Send)
}

func (h *HttpServiceRPC) HttpSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[httpv1.HttpSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return h.streamHttpSync(ctx, userID, stream.Send)
}

func (h *HttpServiceRPC) HttpVersionSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[httpv1.HttpVersionSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return h.streamHttpVersionSync(ctx, userID, stream.Send)
}
