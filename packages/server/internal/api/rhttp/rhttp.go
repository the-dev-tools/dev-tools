package rhttp

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	dbmodels "the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rworkspace"
	"the-dev-tools/server/pkg/eventstream"
	"the-dev-tools/server/pkg/expression"
	"the-dev-tools/server/pkg/http/request"
	"the-dev-tools/server/pkg/httpclient"
	"the-dev-tools/server/pkg/idwrap"
	// "the-dev-tools/server/pkg/model/mbodyform" // TODO: Use if needed
	// "the-dev-tools/server/pkg/model/mbodyurl" // TODO: Use if needed
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mhttpassert"
	"the-dev-tools/server/pkg/model/mhttpbodyform"
	"the-dev-tools/server/pkg/model/mhttpbodyurlencoded"
	"the-dev-tools/server/pkg/model/mhttpheader"
	"the-dev-tools/server/pkg/model/mhttpsearchparam"
	// "the-dev-tools/server/pkg/model/mitemapi" // TODO: Use if needed
	// "the-dev-tools/server/pkg/model/mitemapiexample" // TODO: Use if needed
	"the-dev-tools/server/pkg/model/mvar"
	"the-dev-tools/server/pkg/model/mworkspaceuser"
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
	// "the-dev-tools/server/pkg/sort/sortenabled" // TODO: Use if needed
	"the-dev-tools/server/pkg/varsystem"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
	httpv1connect "the-dev-tools/spec/dist/buf/go/api/http/v1/httpv1connect"
)

const (
	eventTypeInsert = "insert"
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

// bytesToIDWrap converts []byte to *idwrap.IDWrap safely
func bytesToIDWrap(b []byte) *idwrap.IDWrap {
	if b == nil || len(b) == 0 {
		return nil
	}
	id, err := idwrap.NewFromBytes(b)
	if err != nil {
		return nil
	}
	return &id
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
	HttpBodyForm *apiv1.HttpBodyFormData
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

// toAPIHttp converts model HTTP to API HTTP
func toAPIHttp(http mhttp.HTTP) *apiv1.Http {
	apiHttp := &apiv1.Http{
		HttpId:   http.ID.Bytes(),
		Name:     http.Name,
		Url:      http.Url,
		Method:   toAPIHttpMethod(http.Method),
		BodyKind: toAPIHttpBodyKind(http.BodyKind),
	}

	if http.LastRunAt != nil {
		apiHttp.LastRunAt = timestamppb.New(time.Unix(*http.LastRunAt, 0))
	}

	if http.FolderID != nil {
		// Note: FolderId field may need to be added to API proto if not present
		// apiHttp.FolderId = http.FolderID.Bytes()
	}

	return apiHttp
}

// toAPIHttpBodyKind converts model HttpBodyKind to API HttpBodyKind
func toAPIHttpBodyKind(kind mhttp.HttpBodyKind) apiv1.HttpBodyKind {
	switch kind {
	case mhttp.HttpBodyKindNone:
		return apiv1.HttpBodyKind_HTTP_BODY_KIND_UNSPECIFIED
	case mhttp.HttpBodyKindFormData:
		return apiv1.HttpBodyKind_HTTP_BODY_KIND_FORM_DATA
	case mhttp.HttpBodyKindUrlEncoded:
		return apiv1.HttpBodyKind_HTTP_BODY_KIND_URL_ENCODED
	case mhttp.HttpBodyKindRaw:
		return apiv1.HttpBodyKind_HTTP_BODY_KIND_RAW
	default:
		return apiv1.HttpBodyKind_HTTP_BODY_KIND_UNSPECIFIED
	}
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

// fromAPIHttpBodyKind converts API HttpBodyKind to model HttpBodyKind
func fromAPIHttpBodyKind(kind apiv1.HttpBodyKind) mhttp.HttpBodyKind {
	switch kind {
	case apiv1.HttpBodyKind_HTTP_BODY_KIND_UNSPECIFIED:
		return mhttp.HttpBodyKindNone
	case apiv1.HttpBodyKind_HTTP_BODY_KIND_FORM_DATA:
		return mhttp.HttpBodyKindFormData
	case apiv1.HttpBodyKind_HTTP_BODY_KIND_URL_ENCODED:
		return mhttp.HttpBodyKindUrlEncoded
	case apiv1.HttpBodyKind_HTTP_BODY_KIND_RAW:
		return mhttp.HttpBodyKindRaw
	default:
		return mhttp.HttpBodyKindNone // Default to None if unspecified
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

// toAPIHttpBodyFormData converts model HttpBodyForm to API HttpBodyFormData
func toAPIHttpBodyFormData(form mhttpbodyform.HttpBodyForm) *apiv1.HttpBodyFormData {
	return &apiv1.HttpBodyFormData{
		HttpBodyFormDataId: form.ID.Bytes(),
		HttpId:             form.HttpID.Bytes(),
		Key:                form.Key,
		Value:              form.Value,
		Enabled:            form.Enabled,
		Description:        form.Description,
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

// toAPIHttpVersion converts model HttpVersion to API HttpVersion
func toAPIHttpVersion(version dbmodels.HttpVersion) *apiv1.HttpVersion {
	return &apiv1.HttpVersion{
		HttpVersionId: version.ID.Bytes(),
		HttpId:        version.HttpID.Bytes(),
		Name:          version.VersionName,
		Description:   version.VersionDescription,
		CreatedAt:     version.CreatedAt,
	}
}

// toAPIHttpResponse converts DB HttpResponse to API HttpResponse
func toAPIHttpResponse(response dbmodels.HttpResponse) *apiv1.HttpResponse {
	var body string
	if utf8.Valid(response.Body) {
		body = string(response.Body)
	} else {
		body = fmt.Sprintf("[Binary data: %d bytes]", len(response.Body))
	}

	return &apiv1.HttpResponse{
		HttpResponseId: response.ID.Bytes(),
		HttpId:         response.HttpID.Bytes(),
		Status:         int32(response.Status.(int32)),
		Body:           body,
		Time:           timestamppb.New(response.Time),
		Duration:       int32(response.Duration.(int32)),
		Size:           int32(response.Size.(int32)),
	}
}

// toAPIHttpResponseHeader converts DB HttpResponseHeader to API HttpResponseHeader
func toAPIHttpResponseHeader(header dbmodels.HttpResponseHeader) *apiv1.HttpResponseHeader {
	return &apiv1.HttpResponseHeader{
		HttpResponseHeaderId: header.ID.Bytes(),
		HttpResponseId:       header.ResponseID.Bytes(),
		Key:                  header.Key,
		Value:                header.Value,
	}
}

// toAPIHttpResponseAssert converts DB HttpResponseAssert to API HttpResponseAssert
func toAPIHttpResponseAssert(assert dbmodels.HttpResponseAssert) *apiv1.HttpResponseAssert {
	return &apiv1.HttpResponseAssert{
		HttpResponseAssertId: assert.ID,
		HttpResponseId:       assert.ResponseID,
		Value:                assert.Value,
		Success:              assert.Success,
	}
}

// toAPIHttpBodyRaw converts DB HttpBodyRaw to API HttpBodyRaw
func toAPIHttpBodyRaw(httpID []byte, data string) *apiv1.HttpBodyRaw {
	return &apiv1.HttpBodyRaw{
		// HttpId: httpID,
		Data:   data,
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

// publishInsertEvent publishes an insert event for real-time sync
func (h *HttpServiceRPC) publishInsertEvent(http mhttp.HTTP) {
	h.stream.Publish(HttpTopic{WorkspaceID: http.WorkspaceID}, HttpEvent{
		Type: eventTypeInsert,
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

// publishVersionInsertEvent publishes an insert event for real-time sync
func (h *HttpServiceRPC) publishVersionInsertEvent(version dbmodels.HttpVersion, workspaceID idwrap.IDWrap) {
	h.httpVersionStream.Publish(HttpVersionTopic{WorkspaceID: workspaceID}, HttpVersionEvent{
		Type:        eventTypeInsert,
		HttpVersion: toAPIHttpVersion(version),
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

// getHttpVersionsByHttpID retrieves all versions for a specific HTTP entry
func (h *HttpServiceRPC) getHttpVersionsByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]dbmodels.HttpVersion, error) {
	rows, err := h.DB.QueryContext(ctx, `
		SELECT id, http_id, version_name, version_description, is_active, created_at, created_by
		FROM http_version
		WHERE http_id = ?
		ORDER BY created_at DESC
	`, httpID.Bytes())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []dbmodels.HttpVersion
	for rows.Next() {
		var version dbmodels.HttpVersion
		var createdByBytes []byte
		err := rows.Scan(
			&version.ID,
			&version.HttpID,
			&version.VersionName,
			&version.VersionDescription,
			&version.IsActive,
			&version.CreatedAt,
			&createdByBytes,
		)
		if err != nil {
			return nil, err
		}

		if len(createdByBytes) > 0 {
			createdByID, err := idwrap.NewFromBytes(createdByBytes)
			if err != nil {
				return nil, err
			}
			version.CreatedBy = &createdByID
		}

		versions = append(versions, version)
	}

	return versions, rows.Err()
}

// httpSyncResponseFrom converts HttpEvent to HttpSync response
func httpSyncResponseFrom(event HttpEvent) *apiv1.HttpSyncResponse {
	var value *apiv1.HttpSync_ValueUnion

	switch event.Type {
	case eventTypeInsert:
		name := event.Http.GetName()
		method := event.Http.GetMethod()
		url := event.Http.GetUrl()
		bodyKind := event.Http.GetBodyKind()
		lastRunAt := event.Http.GetLastRunAt()
		value = &apiv1.HttpSync_ValueUnion{
			Kind: apiv1.HttpSync_ValueUnion_KIND_INSERT,
			Insert: &apiv1.HttpSyncInsert{
				HttpId:    event.Http.GetHttpId(),
				Name:      name,
				Method:    method,
				Url:       url,
				BodyKind:  bodyKind,
				LastRunAt: lastRunAt,
			},
		}
		case eventTypeUpdate:
			name := event.Http.GetName()
			method := event.Http.GetMethod()
			url := event.Http.GetUrl()
			bodyKind := event.Http.GetBodyKind()
			lastRunAt := event.Http.GetLastRunAt()
	
			var lastRunAtUnion *apiv1.HttpSyncUpdate_LastRunAtUnion
		if lastRunAt != nil {
			lastRunAtUnion = &apiv1.HttpSyncUpdate_LastRunAtUnion{
				Kind:      apiv1.HttpSyncUpdate_LastRunAtUnion_KIND_TIMESTAMP,
				Timestamp: lastRunAt,
			}
		}

		value = &apiv1.HttpSync_ValueUnion{
			Kind: apiv1.HttpSync_ValueUnion_KIND_UPDATE,
			Update: &apiv1.HttpSyncUpdate{
				HttpId:    event.Http.GetHttpId(),
				Name:      &name,
				Method:    &method,
				Url:       &url,
				BodyKind:  &bodyKind,
				LastRunAt: lastRunAtUnion,
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
	case eventTypeInsert:
		key := event.HttpHeader.GetKey()
		value_ := event.HttpHeader.GetValue()
		enabled := event.HttpHeader.GetEnabled()
		description := event.HttpHeader.GetDescription()
		order := event.HttpHeader.GetOrder()
		value = &apiv1.HttpHeaderSync_ValueUnion{
			Kind: apiv1.HttpHeaderSync_ValueUnion_KIND_INSERT,
			Insert: &apiv1.HttpHeaderSyncInsert{
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
	case eventTypeInsert:
		key := event.HttpSearchParam.GetKey()
		value_ := event.HttpSearchParam.GetValue()
		enabled := event.HttpSearchParam.GetEnabled()
		description := event.HttpSearchParam.GetDescription()
		order := event.HttpSearchParam.GetOrder()
		value = &apiv1.HttpSearchParamSync_ValueUnion{
			Kind: apiv1.HttpSearchParamSync_ValueUnion_KIND_INSERT,
			Insert: &apiv1.HttpSearchParamSyncInsert{
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
	case eventTypeInsert:
		value_ := event.HttpAssert.GetValue()
		value = &apiv1.HttpAssertSync_ValueUnion{
			Kind: apiv1.HttpAssertSync_ValueUnion_KIND_INSERT,
			Insert: &apiv1.HttpAssertSyncInsert{
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

// httpVersionSyncResponseFrom converts HttpVersionEvent to HttpVersionSync response
func httpVersionSyncResponseFrom(event HttpVersionEvent) *apiv1.HttpVersionSyncResponse {
	var value *apiv1.HttpVersionSync_ValueUnion

	switch event.Type {
	case eventTypeInsert:
		value = &apiv1.HttpVersionSync_ValueUnion{
			Kind: apiv1.HttpVersionSync_ValueUnion_KIND_INSERT,
			Insert: &apiv1.HttpVersionSyncInsert{
				HttpVersionId: event.HttpVersion.GetHttpVersionId(),
				HttpId:        event.HttpVersion.GetHttpId(),
				Name:          event.HttpVersion.GetName(),
				Description:   event.HttpVersion.GetDescription(),
				CreatedAt:     event.HttpVersion.GetCreatedAt(),
			},
		}
	case eventTypeUpdate:
		value = &apiv1.HttpVersionSync_ValueUnion{
			Kind: apiv1.HttpVersionSync_ValueUnion_KIND_UPDATE,
			Update: &apiv1.HttpVersionSyncUpdate{
				HttpVersionId: event.HttpVersion.GetHttpVersionId(),
			},
		}
	case eventTypeDelete:
		value = &apiv1.HttpVersionSync_ValueUnion{
			Kind: apiv1.HttpVersionSync_ValueUnion_KIND_DELETE,
			Delete: &apiv1.HttpVersionSyncDelete{
				HttpVersionId: event.HttpVersion.GetHttpVersionId(),
			},
		}
	}

	return &apiv1.HttpVersionSyncResponse{
		Items: []*apiv1.HttpVersionSync{
			{
				Value: value,
			},
		},
	}
}

// httpResponseSyncResponseFrom converts HttpResponseEvent to HttpResponseSync response
func httpResponseSyncResponseFrom(event HttpResponseEvent) *apiv1.HttpResponseSyncResponse {
	var value *apiv1.HttpResponseSync_ValueUnion

	switch event.Type {
	case eventTypeInsert:
		status := event.HttpResponse.GetStatus()
		body := event.HttpResponse.GetBody()
		time := event.HttpResponse.GetTime()
		duration := event.HttpResponse.GetDuration()
		size := event.HttpResponse.GetSize()
		value = &apiv1.HttpResponseSync_ValueUnion{
			Kind: apiv1.HttpResponseSync_ValueUnion_KIND_INSERT,
			Insert: &apiv1.HttpResponseSyncInsert{
				HttpResponseId: event.HttpResponse.GetHttpResponseId(),
				HttpId:         event.HttpResponse.GetHttpId(),
				Status:         status,
				Body:           body,
				Time:           time,
				Duration:       duration,
				Size:           size,
			},
		}
	case eventTypeUpdate:
		status := event.HttpResponse.GetStatus()
		body := event.HttpResponse.GetBody()
		time := event.HttpResponse.GetTime()
		duration := event.HttpResponse.GetDuration()
		size := event.HttpResponse.GetSize()
		value = &apiv1.HttpResponseSync_ValueUnion{
			Kind: apiv1.HttpResponseSync_ValueUnion_KIND_UPDATE,
			Update: &apiv1.HttpResponseSyncUpdate{
				HttpResponseId: event.HttpResponse.GetHttpResponseId(),
				Status:         &status,
				Body:           &body,
				Time:           time,
				Duration:       &duration,
				Size:           &size,
			},
		}
	case eventTypeDelete:
		value = &apiv1.HttpResponseSync_ValueUnion{
			Kind: apiv1.HttpResponseSync_ValueUnion_KIND_DELETE,
			Delete: &apiv1.HttpResponseSyncDelete{
				HttpResponseId: event.HttpResponse.GetHttpResponseId(),
			},
		}
	}

	return &apiv1.HttpResponseSyncResponse{
		Items: []*apiv1.HttpResponseSync{
			{
				Value: value,
			},
		},
	}
}

// httpResponseHeaderSyncResponseFrom converts HttpResponseHeaderEvent to HttpResponseHeaderSync response
func httpResponseHeaderSyncResponseFrom(event HttpResponseHeaderEvent) *apiv1.HttpResponseHeaderSyncResponse {
	var value *apiv1.HttpResponseHeaderSync_ValueUnion

	switch event.Type {
	case eventTypeInsert:
		key := event.HttpResponseHeader.GetKey()
		value_ := event.HttpResponseHeader.GetValue()
		value = &apiv1.HttpResponseHeaderSync_ValueUnion{
			Kind: apiv1.HttpResponseHeaderSync_ValueUnion_KIND_INSERT,
			Insert: &apiv1.HttpResponseHeaderSyncInsert{
				HttpResponseHeaderId: event.HttpResponseHeader.GetHttpResponseHeaderId(),
				HttpResponseId:       event.HttpResponseHeader.GetHttpResponseId(),
				Key:                  key,
				Value:                value_,
			},
		}
	case eventTypeUpdate:
		key := event.HttpResponseHeader.GetKey()
		value_ := event.HttpResponseHeader.GetValue()
		value = &apiv1.HttpResponseHeaderSync_ValueUnion{
			Kind: apiv1.HttpResponseHeaderSync_ValueUnion_KIND_UPDATE,
			Update: &apiv1.HttpResponseHeaderSyncUpdate{
				HttpResponseHeaderId: event.HttpResponseHeader.GetHttpResponseHeaderId(),
				Key:                  &key,
				Value:                &value_,
			},
		}
	case eventTypeDelete:
		value = &apiv1.HttpResponseHeaderSync_ValueUnion{
			Kind: apiv1.HttpResponseHeaderSync_ValueUnion_KIND_DELETE,
			Delete: &apiv1.HttpResponseHeaderSyncDelete{
				HttpResponseHeaderId: event.HttpResponseHeader.GetHttpResponseHeaderId(),
			},
		}
	}

	return &apiv1.HttpResponseHeaderSyncResponse{
		Items: []*apiv1.HttpResponseHeaderSync{
			{
				Value: value,
			},
		},
	}
}

// httpResponseAssertSyncResponseFrom converts HttpResponseAssertEvent to HttpResponseAssertSync response
func httpResponseAssertSyncResponseFrom(event HttpResponseAssertEvent) *apiv1.HttpResponseAssertSyncResponse {
	var value *apiv1.HttpResponseAssertSync_ValueUnion

	switch event.Type {
	case eventTypeInsert:
		value_ := event.HttpResponseAssert.GetValue()
		success := event.HttpResponseAssert.GetSuccess()
		value = &apiv1.HttpResponseAssertSync_ValueUnion{
			Kind: apiv1.HttpResponseAssertSync_ValueUnion_KIND_INSERT,
			Insert: &apiv1.HttpResponseAssertSyncInsert{
				HttpResponseAssertId: event.HttpResponseAssert.GetHttpResponseAssertId(),
				HttpResponseId:       event.HttpResponseAssert.GetHttpResponseId(),
				Value:                value_,
				Success:              success,
			},
		}
	case eventTypeUpdate:
		value_ := event.HttpResponseAssert.GetValue()
		success := event.HttpResponseAssert.GetSuccess()
		value = &apiv1.HttpResponseAssertSync_ValueUnion{
			Kind: apiv1.HttpResponseAssertSync_ValueUnion_KIND_UPDATE,
			Update: &apiv1.HttpResponseAssertSyncUpdate{
				HttpResponseAssertId: event.HttpResponseAssert.GetHttpResponseAssertId(),
				Value:                &value_,
				Success:              &success,
			},
		}
	case eventTypeDelete:
		value = &apiv1.HttpResponseAssertSync_ValueUnion{
			Kind: apiv1.HttpResponseAssertSync_ValueUnion_KIND_DELETE,
			Delete: &apiv1.HttpResponseAssertSyncDelete{
				HttpResponseAssertId: event.HttpResponseAssert.GetHttpResponseAssertId(),
			},
		}
	}

	return &apiv1.HttpResponseAssertSyncResponse{
		Items: []*apiv1.HttpResponseAssertSync{
			{
				Value: value,
			},
		},
	}
}

// httpBodyRawSyncResponseFrom converts HttpBodyRawEvent to HttpBodyRawSync response
func httpBodyRawSyncResponseFrom(event HttpBodyRawEvent) *apiv1.HttpBodyRawSyncResponse {
	var value *apiv1.HttpBodyRawSync_ValueUnion

	switch event.Type {
	case eventTypeInsert:
		data := event.HttpBodyRaw.GetData()
		value = &apiv1.HttpBodyRawSync_ValueUnion{
			Kind: apiv1.HttpBodyRawSync_ValueUnion_KIND_INSERT,
			Insert: &apiv1.HttpBodyRawSyncInsert{
				HttpId: event.HttpBodyRaw.GetHttpId(),
				Data:   data,
			},
		}
	case eventTypeUpdate:
		data := event.HttpBodyRaw.GetData()
		value = &apiv1.HttpBodyRawSync_ValueUnion{
			Kind: apiv1.HttpBodyRawSync_ValueUnion_KIND_UPDATE,
			Update: &apiv1.HttpBodyRawSyncUpdate{
				HttpId: event.HttpBodyRaw.GetHttpId(),
				Data:   &data,
			},
		}
	case eventTypeDelete:
		value = &apiv1.HttpBodyRawSync_ValueUnion{
			Kind: apiv1.HttpBodyRawSync_ValueUnion_KIND_DELETE,
			Delete: &apiv1.HttpBodyRawSyncDelete{
				HttpId: event.HttpBodyRaw.GetHttpId(),
			},
		}
	}

	return &apiv1.HttpBodyRawSyncResponse{
		Items: []*apiv1.HttpBodyRawSync{
			{
				Value: value,
			},
		},
	}
}

// httpDeltaSyncResponseFrom converts HttpEvent to HttpDeltaSync response
func httpDeltaSyncResponseFrom(event HttpEvent, http mhttp.HTTP) *apiv1.HttpDeltaSyncResponse {
	var value *apiv1.HttpDeltaSync_ValueUnion

	switch event.Type {
	case eventTypeInsert:
		delta := &apiv1.HttpDeltaSyncInsert{
			DeltaHttpId: http.ID.Bytes(),
		}
		if http.ParentHttpID != nil {
			delta.HttpId = http.ParentHttpID.Bytes()
		}
		if http.DeltaName != nil {
			delta.Name = http.DeltaName
		}
		if http.DeltaMethod != nil {
			method := toAPIHttpMethod(*http.DeltaMethod)
			delta.Method = &method
		}
		if http.DeltaUrl != nil {
			delta.Url = http.DeltaUrl
		}
		// Note: BodyKind delta not implemented yet
		value = &apiv1.HttpDeltaSync_ValueUnion{
			Kind:   apiv1.HttpDeltaSync_ValueUnion_KIND_INSERT,
			Insert: delta,
		}
	case eventTypeUpdate:
		delta := &apiv1.HttpDeltaSyncUpdate{
			DeltaHttpId: http.ID.Bytes(),
		}
		if http.ParentHttpID != nil {
			delta.HttpId = http.ParentHttpID.Bytes()
		}
		if http.DeltaName != nil {
			nameStr := *http.DeltaName
			delta.Name = &apiv1.HttpDeltaSyncUpdate_NameUnion{
				Kind:    315301840, // KIND_STRING
				String_: &nameStr,
			}
		}
		if http.DeltaMethod != nil {
			method := toAPIHttpMethod(*http.DeltaMethod)
			delta.Method = &apiv1.HttpDeltaSyncUpdate_MethodUnion{
				Kind:       470142787, // KIND_HTTP_METHOD
				HttpMethod: &method,
			}
		}
		if http.DeltaUrl != nil {
			urlStr := *http.DeltaUrl
			delta.Url = &apiv1.HttpDeltaSyncUpdate_UrlUnion{
				Kind:    315301840, // KIND_STRING
				String_: &urlStr,
			}
		}
		// Note: BodyKind delta not implemented yet
		value = &apiv1.HttpDeltaSync_ValueUnion{
			Kind:   apiv1.HttpDeltaSync_ValueUnion_KIND_UPDATE,
			Update: delta,
		}
	case eventTypeDelete:
		value = &apiv1.HttpDeltaSync_ValueUnion{
			Kind: apiv1.HttpDeltaSync_ValueUnion_KIND_DELETE,
			Delete: &apiv1.HttpDeltaSyncDelete{
				DeltaHttpId: http.ID.Bytes(),
			},
		}
	}

	return &apiv1.HttpDeltaSyncResponse{
		Items: []*apiv1.HttpDeltaSync{
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
					Type: eventTypeInsert,
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

	return eventstream.StreamToClient(
		ctx,
		h.stream,
		snapshot,
		filter,
		httpSyncResponseFrom,
		send,
	)
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
	// Load all components
	headers, err := h.httpHeaderService.GetByHttpIDOrdered(ctx, httpEntry.ID)
	if err != nil {
		headers = []mhttpheader.HttpHeader{}
	}

	queries, err := h.httpSearchParamService.GetByHttpIDOrdered(ctx, httpEntry.ID)
	if err != nil {
		queries = []mhttpsearchparam.HttpSearchParam{}
	}

	rawBody, err := h.bodyService.GetByHttpID(ctx, httpEntry.ID)
	if err != nil && !errors.Is(err, shttp.ErrNoHttpBodyRawFound) {
		rawBody = nil
	}

	formBody, err := h.httpBodyFormService.GetHttpBodyFormsByHttpID(ctx, httpEntry.ID)
	if err != nil {
		formBody = []mhttpbodyform.HttpBodyForm{}
	}

	urlEncodedBody, err := h.httpBodyUrlEncodedService.GetHttpBodyUrlEncodedByHttpID(ctx, httpEntry.ID)
	if err != nil {
		urlEncodedBody = []mhttpbodyurlencoded.HttpBodyUrlEncoded{}
	}

	// Build variable context from previous HTTP responses in the workspace
	varMap, err := h.buildWorkspaceVarMap(ctx, httpEntry.WorkspaceID)
	if err != nil {
		// Continue with empty varMap rather than failing
		varMap = varsystem.VarMap{}
	}

	// Convert to mhttp types for request preparation
	mHeaders := make([]mhttp.HTTPHeader, len(headers))
	for i, v := range headers {
		mHeaders[i] = mhttp.HTTPHeader{
			HeaderKey:   v.Key,
			HeaderValue: v.Value,
			Enabled:     v.Enabled,
		}
	}

	mQueries := make([]mhttp.HTTPSearchParam, len(queries))
	for i, v := range queries {
		mQueries[i] = mhttp.HTTPSearchParam{
			ParamKey:   v.Key,
			ParamValue: v.Value,
			Enabled:    v.Enabled,
		}
	}

	mFormBody := make([]mhttp.HTTPBodyForm, len(formBody))
	for i, v := range formBody {
		mFormBody[i] = mhttp.HTTPBodyForm{
			FormKey:   v.Key,
			FormValue: v.Value,
			Enabled:   v.Enabled,
		}
	}

	mUrlEncodedBody := make([]mhttp.HTTPBodyUrlencoded, len(urlEncodedBody))
	for i, v := range urlEncodedBody {
		mUrlEncodedBody[i] = mhttp.HTTPBodyUrlencoded{
			UrlencodedKey:   v.Key,
			UrlencodedValue: v.Value,
			Enabled:         v.Enabled,
		}
	}

	// Prepare the HTTP request using request package
	res, err := request.PrepareHTTPRequestWithTracking(
		*httpEntry,
		mHeaders,
		mQueries,
		rawBody,
		mFormBody,
		mUrlEncodedBody,
		varMap,
	)
	if err != nil {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("failed to prepare request: %w", err))
	}
	httpReq := res.Request

	// Create HTTP client with timeout
	client := httpclient.New()

	// Start timing the HTTP request
	startTime := time.Now()
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
	duration := time.Since(startTime).Milliseconds()
	responseID, err := h.storeHttpResponse(ctx, httpEntry, httpResp, startTime, duration)
	if err != nil {
		// Continue with assertion evaluation even if response storage fails
		responseID = idwrap.IDWrap{} // Use empty ID as fallback
	}

	// Load and evaluate assertions with comprehensive error handling
	if err := h.evaluateAndStoreAssertions(ctx, httpEntry.ID, responseID, httpResp); err != nil {
		// Log detailed error but don't fail the request
		log.Printf("Failed to evaluate assertions for HTTP %s (response %s): %v",
			httpEntry.ID.String(), responseID.String(), err)
	}

	// Extract variables from HTTP response for downstream usage
	if err := h.extractResponseVariables(ctx, httpEntry.WorkspaceID, httpEntry.Name, &httpResp); err != nil {
		// Log error but don't fail the request
		log.Printf("Failed to extract response variables: %v", err)
	}

	return nil
}

// buildWorkspaceVarMap creates a variable map from workspace environments
func (h *HttpServiceRPC) buildWorkspaceVarMap(ctx context.Context, workspaceID idwrap.IDWrap) (varsystem.VarMap, error) {
	// Get workspace to find global environment
	workspace, err := h.ws.Get(ctx, workspaceID)
	if err != nil {
		return varsystem.VarMap{}, fmt.Errorf("failed to get workspace: %w", err)
	}

	// Get global environment variables
	var globalVars []mvar.Var
	if workspace.GlobalEnv != (idwrap.IDWrap{}) {
		globalVars, err = h.vs.GetVariableByEnvID(ctx, workspace.GlobalEnv)
		if err != nil && !errors.Is(err, svar.ErrNoVarFound) {
			return varsystem.VarMap{}, fmt.Errorf("failed to get global environment variables: %w", err)
		}
	}

	// Create variable map by merging global environment variables
	varMap := make(map[string]any)

	// Add global environment variables
	for _, envVar := range globalVars {
		if envVar.IsEnabled() {
			varMap[envVar.VarKey] = envVar.Value
		}
	}

	// Convert to varsystem.VarMap
	result := varsystem.NewVarMapFromAnyMap(varMap)

	return result, nil
}

// applyVariableSubstitution applies variable substitution to HTTP request components
func (h *HttpServiceRPC) applyVariableSubstitution(ctx context.Context, httpEntry *mhttp.HTTP,
	headers []interface{}, queries []interface{}, body []byte,
	varMap varsystem.VarMap) (*mhttp.HTTP, []interface{}, []interface{}, []byte, error) {

	// Create a tracking wrapper around the varMap to track variable usage
	tracker := varsystem.NewVarMapTracker(varMap)

	// Apply variable substitution to URL
	if varsystem.CheckStringHasAnyVarKey(httpEntry.Url) {
		resolvedURL, err := tracker.ReplaceVars(httpEntry.Url)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("failed to resolve variables in URL: %w", err)
		}
		httpEntry.Url = resolvedURL
	}

	// Apply variable substitution to headers
	for i, item := range headers {
		if header, ok := item.(mhttpheader.HttpHeader); ok && header.Enabled {
			// Substitute key
			if varsystem.CheckStringHasAnyVarKey(header.Key) {
				resolvedKey, err := tracker.ReplaceVars(header.Key)
				if err != nil {
					return nil, nil, nil, nil, fmt.Errorf("failed to resolve variables in header key '%s': %w", header.Key, err)
				}
				header.Key = resolvedKey
			}
			// Substitute value
			if varsystem.CheckStringHasAnyVarKey(header.Value) {
				resolvedValue, err := tracker.ReplaceVars(header.Value)
				if err != nil {
					return nil, nil, nil, nil, fmt.Errorf("failed to resolve variables in header value '%s': %w", header.Value, err)
				}
				header.Value = resolvedValue
			}
			headers[i] = header
		}
	}

	// Apply variable substitution to queries
	for i, item := range queries {
		if query, ok := item.(mhttpsearchparam.HttpSearchParam); ok && query.Enabled {
			// Substitute key
			if varsystem.CheckStringHasAnyVarKey(query.Key) {
				resolvedKey, err := tracker.ReplaceVars(query.Key)
				if err != nil {
					return nil, nil, nil, nil, fmt.Errorf("failed to resolve variables in query key '%s': %w", query.Key, err)
				}
				query.Key = resolvedKey
			}
			// Substitute value
			if varsystem.CheckStringHasAnyVarKey(query.Value) {
				resolvedValue, err := tracker.ReplaceVars(query.Value)
				if err != nil {
					return nil, nil, nil, nil, fmt.Errorf("failed to resolve variables in query value '%s': %w", query.Value, err)
				}
				query.Value = resolvedValue
			}
			queries[i] = query
		}
	}

	// Apply variable substitution to body
	if len(body) > 0 {
		bodyStr := string(body)
		if varsystem.CheckStringHasAnyVarKey(bodyStr) {
			resolvedBody, err := tracker.ReplaceVars(bodyStr)
			if err != nil {
				return nil, nil, nil, nil, fmt.Errorf("failed to resolve variables in request body: %w", err)
			}
			body = []byte(resolvedBody)
		}
	}

	return httpEntry, headers, queries, body, nil
}

// extractResponseVariables extracts variables from HTTP response for downstream usage
func (h *HttpServiceRPC) extractResponseVariables(ctx context.Context, workspaceID idwrap.IDWrap, httpName string, httpResp *httpclient.Response) error {
	// Convert HTTP response to variable format similar to nrequest pattern
	respVar := httpclient.ConvertResponseToVar(*httpResp)

	// Create response map following the nrequest pattern
	// TODO: Use responseMap when variable storage is implemented
	_ = map[string]any{
		"status":  float64(respVar.StatusCode),
		"body":    respVar.Body,
		"headers": cloneStringMapToAny(respVar.Headers),
	}

	// Store the response variables for future HTTP requests
	// TODO: Implement variable storage mechanism
	// This could store variables in:
	// 1. A dedicated HTTP response variable table
	// 2. Workspace-scoped variable storage
	// 3. In-memory cache for the session

	return nil
}

// cloneStringMapToAny converts a map[string]string to map[string]any
// This follows the pattern from nrequest.go
func cloneStringMapToAny(src map[string]string) map[string]any {
	if len(src) == 0 {
		return map[string]any{}
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
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

func (h *HttpServiceRPC) HttpInsert(ctx context.Context, req *connect.Request[apiv1.HttpInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP entry must be provided"))
	}

	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Step 1: Do ALL reads OUTSIDE transaction - get user's workspaces
	workspaces, err := h.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if len(workspaces) == 0 {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("user has no workspaces"))
	}

	// Step 2: Check permissions OUTSIDE transaction
	defaultWorkspaceID := workspaces[0].ID
	if err := h.checkWorkspaceWriteAccess(ctx, defaultWorkspaceID); err != nil {
		return nil, err
	}

	// Step 3: Process request data OUTSIDE transaction
	var httpModels []*mhttp.HTTP
	for _, item := range req.Msg.Items {
		if len(item.HttpId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_id is required"))
		}

		httpID, err := idwrap.NewFromBytes(item.HttpId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Create the HTTP entry model
		httpModel := &mhttp.HTTP{
			ID:          httpID,
			WorkspaceID: defaultWorkspaceID,
			Name:        item.Name,
			Url:         item.Url,
			Method:      fromAPIHttpMethod(item.Method),
			Description: "", // Description field not available in API yet
			BodyKind:    fromAPIHttpBodyKind(item.BodyKind),
		}

		httpModels = append(httpModels, httpModel)
	}

	// Step 4: Minimal write transaction for fast inserts only
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	hsService := h.hs.TX(tx)

	var createdHTTPs []mhttp.HTTP

	// Fast writes inside minimal transaction
	for _, httpModel := range httpModels {
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
		h.publishInsertEvent(http)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpUpdate(ctx context.Context, req *connect.Request[apiv1.HttpUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP entry must be provided"))
	}

	// Step 1: Process request data and get HTTP IDs OUTSIDE transaction
	var updateData []struct {
		httpID    idwrap.IDWrap
		name      *string
		url       *string
		method    *string
		bodyKind  *mhttp.HttpBodyKind
		httpModel *mhttp.HTTP
	}

	for _, item := range req.Msg.Items {
		if len(item.HttpId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_id is required"))
		}

		httpID, err := idwrap.NewFromBytes(item.HttpId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		var name *string
		var url *string
		var method *string
		var bodyKind *mhttp.HttpBodyKind

		// Process optional fields
		if item.Name != nil {
			name = item.Name
		}
		if item.Url != nil {
			url = item.Url
		}
		if item.Method != nil {
			m := fromAPIHttpMethod(*item.Method)
			method = &m
		}
		if item.BodyKind != nil {
			bk := fromAPIHttpBodyKind(*item.BodyKind)
			bodyKind = &bk
		}

		updateData = append(updateData, struct {
			httpID    idwrap.IDWrap
			name      *string
			url       *string
			method    *string
			bodyKind  *mhttp.HttpBodyKind
			httpModel *mhttp.HTTP
		}{httpID: httpID, name: name, url: url, method: method, bodyKind: bodyKind})
	}

	// Step 2: Get existing HTTP entries and check permissions OUTSIDE transaction
	for i := range updateData {
		existingHttp, err := h.hs.Get(ctx, updateData[i].httpID)
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

		// Store the existing model for later update
		updateData[i].httpModel = existingHttp
	}

	// Step 3: Apply updates to models OUTSIDE transaction
	for i := range updateData {
		if updateData[i].name != nil {
			updateData[i].httpModel.Name = *updateData[i].name
		}
		if updateData[i].url != nil {
			updateData[i].httpModel.Url = *updateData[i].url
		}
		if updateData[i].method != nil {
			updateData[i].httpModel.Method = *updateData[i].method
		}
		if updateData[i].bodyKind != nil {
			updateData[i].httpModel.BodyKind = *updateData[i].bodyKind
		}
	}

	// Step 4: Minimal write transaction for fast updates only
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	hsService := h.hs.TX(tx)

	var updatedHTTPs []mhttp.HTTP
	var newVersions []dbmodels.HttpVersion

	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Fast updates inside minimal transaction
	for _, data := range updateData {
		if err := hsService.Update(ctx, data.httpModel); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		updatedHTTPs = append(updatedHTTPs, *data.httpModel)

		// Create a new version for this update
		// Use Nano to ensure uniqueness during rapid updates
		versionName := fmt.Sprintf("v%d", time.Now().UnixNano())
		versionDesc := "Auto-saved version"
		
		version, err := hsService.CreateHttpVersion(ctx, data.httpID, userID, versionName, versionDesc)
		if err != nil {
			// Log error but don't fail the update? 
			// Strict mode: fail the update if version creation fails
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		newVersions = append(newVersions, *version)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish update events for real-time sync after successful commit
	for _, http := range updatedHTTPs {
		h.publishUpdateEvent(http)
	}

	// Publish version insert events
	for _, version := range newVersions {
		workspaceID := idwrap.IDWrap{} // Need workspace ID, get from http model or lookup
		// Efficient lookup: we have updatedHTTPs which correspond to newVersions by index
		// Find corresponding HTTP to get workspaceID
		for _, http := range updatedHTTPs {
			if http.ID == version.HttpID {
				workspaceID = http.WorkspaceID
				break
			}
		}
		if workspaceID != (idwrap.IDWrap{}) {
			h.publishVersionInsertEvent(version, workspaceID)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpDelete(ctx context.Context, req *connect.Request[apiv1.HttpDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP entry must be provided"))
	}

	// Step 1: Process request data and get HTTP IDs OUTSIDE transaction
	var deleteData []struct {
		httpID       idwrap.IDWrap
		existingHttp *mhttp.HTTP
		workspaceID  idwrap.IDWrap
	}

	for _, item := range req.Msg.Items {
		if len(item.HttpId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_id is required"))
		}

		httpID, err := idwrap.NewFromBytes(item.HttpId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		deleteData = append(deleteData, struct {
			httpID       idwrap.IDWrap
			existingHttp *mhttp.HTTP
			workspaceID  idwrap.IDWrap
		}{httpID: httpID})
	}

	// Step 2: Get existing HTTP entries and check permissions OUTSIDE transaction
	for i := range deleteData {
		existingHttp, err := h.hs.Get(ctx, deleteData[i].httpID)
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

		// Store the existing model and workspace ID for later deletion
		deleteData[i].existingHttp = existingHttp
		deleteData[i].workspaceID = existingHttp.WorkspaceID
	}

	// Step 3: Minimal write transaction for fast deletes only
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	hsService := h.hs.TX(tx)

	var deletedIDs []idwrap.IDWrap
	var deletedWorkspaceIDs []idwrap.IDWrap

	// Fast deletes inside minimal transaction
	for _, data := range deleteData {
		// Perform cascade delete - the database schema should handle foreign key constraints
		// This includes: http_search_param, http_header, http_body_form, http_body_urlencoded,
		// http_body_raw, http_assert, http_response, etc.
		if err := hsService.Delete(ctx, data.httpID); err != nil {
			// Handle foreign key constraint violations gracefully
			if isForeignKeyConstraintError(err) {
				return nil, connect.NewError(connect.CodeFailedPrecondition,
					errors.New("cannot delete HTTP entry with dependent records"))
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		deletedIDs = append(deletedIDs, data.httpID)
		deletedWorkspaceIDs = append(deletedWorkspaceIDs, data.workspaceID)
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
				// HttpId: http.ID.Bytes(),
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
			DeltaMethod: func() *string {
				if item.Method != nil {
					methodStr := fromAPIHttpMethod(*item.Method)
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
			case 183079996: // KIND_UNSET
				existingDelta.DeltaName = nil
			case 315301840: // KIND_STRING
				nameStr := item.Name.GetString_()
				existingDelta.DeltaName = &nameStr
			}
		}
		if item.Method != nil {
			switch item.Method.GetKind() {
			case 183079996: // KIND_UNSET
				existingDelta.DeltaMethod = nil
			case 470142787: // KIND_HTTP_METHOD
				method := item.Method.GetHttpMethod()
				existingDelta.DeltaMethod = httpMethodToString(&method)
			}
		}
		if item.Url != nil {
			switch item.Url.GetKind() {
			case 183079996: // KIND_UNSET
				existingDelta.DeltaUrl = nil
			case 315301840: // KIND_STRING
				urlStr := item.Url.GetString_()
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
			Http: toAPIHttp(delta),
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
			Http: toAPIHttp(delta),
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

	// Snapshot provider for initial state
	snapshot := func(ctx context.Context) ([]eventstream.Event[HttpTopic, HttpEvent], error) {
		// Get all HTTP entries for user
		httpList, err := h.listUserHttp(ctx)
		if err != nil {
			return nil, err
		}

		events := make([]eventstream.Event[HttpTopic, HttpEvent], 0)
		for _, http := range httpList {
			if !http.IsDelta {
				continue // Only include delta records
			}
			workspaceSet.Store(http.WorkspaceID.String(), struct{}{})
			events = append(events, eventstream.Event[HttpTopic, HttpEvent]{
				Topic: HttpTopic{WorkspaceID: http.WorkspaceID},
				Payload: HttpEvent{
					Type: eventTypeInsert,
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
		snapshot,
		filter,
		converter,
		send,
	)
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

	// Update LastRunAt, create version, and publish events
	now := time.Now().Unix()
	httpEntry.LastRunAt = &now

	// Use minimal transaction for update and version creation
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to begin transaction: %w", err))
	}
	defer tx.Rollback()

	hsService := h.hs.TX(tx)

	if err := hsService.Update(ctx, httpEntry); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update LastRunAt: %w", err))
	}

	// Create a new version for this run
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	versionName := fmt.Sprintf("v%d", time.Now().UnixNano())
	versionDesc := "Auto-saved version (Run)"
	
	version, err := hsService.CreateHttpVersion(ctx, httpEntry.ID, userID, versionName, versionDesc)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create version on run: %w", err))
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to commit transaction: %w", err))
	}

	h.publishUpdateEvent(*httpEntry)
	h.publishVersionInsertEvent(*version, httpEntry.WorkspaceID)

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpDuplicate(ctx context.Context, req *connect.Request[apiv1.HttpDuplicateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.HttpId) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_id is required"))
	}

	httpID, err := idwrap.NewFromBytes(req.Msg.HttpId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Get HTTP entry to check workspace permissions and retrieve source data
	httpEntry, err := h.hs.Get(ctx, httpID)
	if err != nil {
		if errors.Is(err, shttp.ErrNoHTTPFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Check read access to source (any role in workspace)
	if err := h.checkWorkspaceReadAccess(ctx, httpEntry.WorkspaceID); err != nil {
		return nil, err
	}

	// Check write access to workspace for creating new entries (Admin or Owner role required)
	if err := h.checkWorkspaceWriteAccess(ctx, httpEntry.WorkspaceID); err != nil {
		return nil, err
	}

	// Step 1: Gather all data OUTSIDE transaction to avoid "Read after Write" deadlocks
	headers, err := h.httpHeaderService.GetByHttpID(ctx, httpID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	searchParams, err := h.httpSearchParamService.GetByHttpID(ctx, httpID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	bodyForms, err := h.httpBodyFormService.GetHttpBodyFormsByHttpID(ctx, httpID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	bodyUrlEncoded, err := h.httpBodyUrlEncodedService.GetHttpBodyUrlEncodedByHttpID(ctx, httpID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	asserts, err := h.httpAssertService.GetHttpAssertsByHttpID(ctx, httpID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Start transaction for consistent duplication
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	// Create transaction-scoped services
	hsService := h.hs.TX(tx)
	httpHeaderService := h.httpHeaderService.TX(tx)
	httpSearchParamService := h.httpSearchParamService.TX(tx)
	httpBodyFormService := h.httpBodyFormService.TX(tx)
	httpBodyUrlEncodedService := h.httpBodyUrlEncodedService.TX(tx)
	httpAssertService := h.httpAssertService.TX(tx)

	// Create new HTTP entry with duplicated name
	newHttpID := idwrap.NewNow()
	duplicateName := fmt.Sprintf("Copy of %s", httpEntry.Name)
	duplicateHttp := &mhttp.HTTP{
		ID:           newHttpID,
		WorkspaceID:  httpEntry.WorkspaceID,
		FolderID:     httpEntry.FolderID,
		Name:         duplicateName,
		Url:          httpEntry.Url,
		Method:       httpEntry.Method,
		Description:  httpEntry.Description,
		ParentHttpID: httpEntry.ParentHttpID,
		// Clear delta fields for the duplicate
		IsDelta:          false,
		DeltaName:        nil,
		DeltaUrl:         nil,
		DeltaMethod:      nil,
		DeltaDescription: nil,
	}

	if err := hsService.Create(ctx, duplicateHttp); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Duplicate headers
	for _, header := range headers {
		newHeaderID := idwrap.NewNow()
		headerModel := &mhttpheader.HttpHeader{
			ID:          newHeaderID,
			HttpID:      newHttpID,
			Key:         header.Key,
			Value:       header.Value,
			Enabled:     header.Enabled,
			Description: header.Description,
		}
		if err := httpHeaderService.Create(ctx, headerModel); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	// Duplicate search params
	for _, param := range searchParams {
		newParamID := idwrap.NewNow()
		paramModel := &mhttpsearchparam.HttpSearchParam{
			ID:          newParamID,
			HttpID:      newHttpID,
			Key:         param.Key,
			Value:       param.Value,
			Enabled:     param.Enabled,
			Description: param.Description,
			Order:       param.Order,
		}
		if err := httpSearchParamService.Create(ctx, paramModel); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	// Duplicate body form entries
	for _, bodyForm := range bodyForms {
		newBodyFormID := idwrap.NewNow()
		bodyFormModel := &mhttpbodyform.HttpBodyForm{
			ID:                   newBodyFormID,
			HttpID:               newHttpID,
			Key:                  bodyForm.Key,
			Value:                bodyForm.Value,
			Enabled:              bodyForm.Enabled,
			Description:          bodyForm.Description,
			Order:                float32(bodyForm.Order),
			ParentHttpBodyFormID: bodyForm.ParentHttpBodyFormID, // Assuming direct copy is fine or handle recursive logic if needed
		}
		if err := httpBodyFormService.CreateHttpBodyForm(ctx, bodyFormModel); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	// Duplicate body URL encoded entries
	for _, bodyUrlEnc := range bodyUrlEncoded {
		newBodyUrlEncodedID := idwrap.NewNow()
		bodyUrlEncodedModel := &mhttpbodyurlencoded.HttpBodyUrlEncoded{
			ID:                         newBodyUrlEncodedID,
			HttpID:                     newHttpID,
			Key:                        bodyUrlEnc.Key,
			Value:                      bodyUrlEnc.Value,
			Enabled:                    bodyUrlEnc.Enabled,
			Description:                bodyUrlEnc.Description,
			Order:                      float32(bodyUrlEnc.Order),
			ParentHttpBodyUrlEncodedID: bodyUrlEnc.ParentHttpBodyUrlEncodedID, // Assuming direct copy is fine
		}
		if err := httpBodyUrlEncodedService.CreateHttpBodyUrlEncoded(ctx, bodyUrlEncodedModel); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	// Duplicate assertions
	for _, assert := range asserts {
		newAssertID := idwrap.NewNow()
		assertModel := &mhttpassert.HttpAssert{
			ID:          newAssertID,
			HttpID:      newHttpID,
			Key:         "", // HttpAssert doesn't use Key field
			Value:       assert.Value,
			Enabled:     true, // Assertions are always active
			Description: "",   // No description available in DB
			Order:       0,    // No order available in DB
		}
		if err := httpAssertService.CreateHttpAssert(ctx, assertModel); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	// Handle raw body if needed (check if source has raw body)
	// Note: This would depend on the raw body structure and service availability
	// The raw body might need to be handled separately based on your schema

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpVersionCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpVersionCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Get user's workspaces
	workspaces, err := h.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var allVersions []*apiv1.HttpVersion
	for _, workspace := range workspaces {
		// Get HTTP entries for this workspace
		httpList, err := h.hs.GetByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Get versions for each HTTP entry
		for _, http := range httpList {
			versions, err := h.getHttpVersionsByHttpID(ctx, http.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			// Convert to API format
			for _, version := range versions {
				apiVersion := toAPIHttpVersion(version)
				allVersions = append(allVersions, apiVersion)
			}
		}
	}

	return connect.NewResponse(&apiv1.HttpVersionCollectionResponse{Items: allVersions}), nil
}

func (h *HttpServiceRPC) HttpVersionSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpVersionSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return h.streamHttpVersionSync(ctx, userID, stream.Send)
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
				if errors.Is(err, shttpsearchparam.ErrNoHttpSearchParamFound) {
					continue
				}
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

func (h *HttpServiceRPC) HttpSearchParamInsert(ctx context.Context, req *connect.Request[apiv1.HttpSearchParamInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP search param must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var insertData []struct {
		paramModel *mhttpsearchparam.HttpSearchParam
	}

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

		// Verify the HTTP entry exists and user has access - use pool service
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

		// Create the param model
		paramModel := &mhttpsearchparam.HttpSearchParam{
			ID:          paramID,
			HttpID:      httpID,
			Key:         item.Key,
			Value:       item.Value,
			Enabled:     item.Enabled,
			Description: item.Description,
			Order:       float64(item.Order),
		}

		insertData = append(insertData, struct {
			paramModel *mhttpsearchparam.HttpSearchParam
		}{
			paramModel: paramModel,
		})
	}

	// Step 2: Execute inserts in transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	httpSearchParamService := h.httpSearchParamService.TX(tx)
	var createdParams []mhttpsearchparam.HttpSearchParam

	for _, data := range insertData {
		if err := httpSearchParamService.Create(ctx, data.paramModel); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		createdParams = append(createdParams, *data.paramModel)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish create events for real-time sync
	for _, param := range createdParams {
		// Get workspace ID for the HTTP entry (we can reuse pool read here as it's after commit)
		httpEntry, err := h.hs.Get(ctx, param.HttpID)
		if err != nil {
			// Log error but continue - event publishing shouldn't fail the operation
			continue
		}
		h.httpSearchParamStream.Publish(HttpSearchParamTopic{WorkspaceID: httpEntry.WorkspaceID}, HttpSearchParamEvent{
			Type:            eventTypeInsert,
			HttpSearchParam: toAPIHttpSearchParam(param),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpSearchParamUpdate(ctx context.Context, req *connect.Request[apiv1.HttpSearchParamUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP search param must be provided"))
	}

	// Step 1: Pre-process and check permissions OUTSIDE transaction
	var updateData []struct {
		paramID       idwrap.IDWrap
		existingParam *mhttpsearchparam.HttpSearchParam
		item          *apiv1.HttpSearchParamUpdate
	}

	for _, item := range req.Msg.Items {
		if len(item.HttpSearchParamId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_search_param_id is required"))
		}

		paramID, err := idwrap.NewFromBytes(item.HttpSearchParamId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing param - use pool service
		existingParam, err := h.httpSearchParamService.GetHttpSearchParam(ctx, paramID)
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

		updateData = append(updateData, struct {
			paramID       idwrap.IDWrap
			existingParam *mhttpsearchparam.HttpSearchParam
			item          *apiv1.HttpSearchParamUpdate
		}{
			paramID:       paramID,
			existingParam: existingParam,
			item:          item,
		})
	}

	// Step 2: Prepare updates (in memory)
	for _, data := range updateData {
		item := data.item
		existingParam := data.existingParam

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
	}

	// Step 3: Execute updates in transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	httpSearchParamService := h.httpSearchParamService.TX(tx)
	var updatedParams []mhttpsearchparam.HttpSearchParam

	for _, data := range updateData {
		if err := httpSearchParamService.Update(ctx, data.existingParam); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		updatedParams = append(updatedParams, *data.existingParam)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish update events for real-time sync
	for _, param := range updatedParams {
		// Get workspace ID for the HTTP entry
		httpEntry, err := h.hs.Get(ctx, param.HttpID)
		if err != nil {
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

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var deleteData []struct {
		paramID       idwrap.IDWrap
		existingParam *mhttpsearchparam.HttpSearchParam
		workspaceID   idwrap.IDWrap
	}

	for _, item := range req.Msg.Items {
		if len(item.HttpSearchParamId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_search_param_id is required"))
		}

		paramID, err := idwrap.NewFromBytes(item.HttpSearchParamId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing param - use pool service
		existingParam, err := h.httpSearchParamService.GetHttpSearchParam(ctx, paramID)
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

		deleteData = append(deleteData, struct {
			paramID       idwrap.IDWrap
			existingParam *mhttpsearchparam.HttpSearchParam
			workspaceID   idwrap.IDWrap
		}{
			paramID:       paramID,
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
	var deletedParams []mhttpsearchparam.HttpSearchParam
	var deletedWorkspaceIDs []idwrap.IDWrap

	for _, data := range deleteData {
		// Delete the param
		if err := httpSearchParamService.Delete(ctx, data.paramID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		deletedParams = append(deletedParams, *data.existingParam)
		deletedWorkspaceIDs = append(deletedWorkspaceIDs, data.workspaceID)
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
						Type:            eventTypeInsert,
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
						Type:       eventTypeInsert,
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

// streamHttpVersionSync streams HTTP version events to the client
func (h *HttpServiceRPC) streamHttpVersionSync(ctx context.Context, userID idwrap.IDWrap, send func(*apiv1.HttpVersionSyncResponse) error) error {
	var workspaceSet sync.Map

	// Snapshot provider for initial state
	snapshot := func(ctx context.Context) ([]eventstream.Event[HttpVersionTopic, HttpVersionEvent], error) {
		// Get all HTTP entries for user
		httpList, err := h.listUserHttp(ctx)
		if err != nil {
			return nil, err
		}

		events := make([]eventstream.Event[HttpVersionTopic, HttpVersionEvent], 0)
		for _, http := range httpList {
			workspaceSet.Store(http.WorkspaceID.String(), struct{}{})

			// Get versions for this HTTP entry
			versions, err := h.getHttpVersionsByHttpID(ctx, http.ID)
			if err != nil {
				return nil, err
			}

			for _, version := range versions {
				events = append(events, eventstream.Event[HttpVersionTopic, HttpVersionEvent]{
					Topic: HttpVersionTopic{WorkspaceID: http.WorkspaceID},
					Payload: HttpVersionEvent{
						Type:        eventTypeInsert,
						HttpVersion: toAPIHttpVersion(version),
					},
				})
			}
		}
		return events, nil
	}

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
		snapshot,
		filter,
		httpVersionSyncResponseFrom,
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
		httpList, err := h.hs.GetByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Get params for each HTTP entry
		for _, http := range httpList {
			params, err := h.httpSearchParamService.GetByHttpIDOrdered(ctx, http.ID)
			if err != nil {
				if errors.Is(err, shttpsearchparam.ErrNoHttpSearchParamFound) {
					continue
				}
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			// Convert to delta format
			for _, param := range params {
				delta := &apiv1.HttpSearchParamDelta{
					DeltaHttpSearchParamId: param.ID.Bytes(),
					HttpSearchParamId:      param.ID.Bytes(),
					// HttpId:                 param.HttpID.Bytes(),
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
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP search param delta must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var updateData []struct {
		deltaID       idwrap.IDWrap
		existingParam *mhttpsearchparam.HttpSearchParam
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
		existingParam, err := h.httpSearchParamService.GetHttpSearchParam(ctx, deltaID)
		if err != nil {
			if errors.Is(err, shttpsearchparam.ErrNoHttpSearchParamFound) {
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
			existingParam *mhttpsearchparam.HttpSearchParam
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
			case 183079996: // KIND_UNSET
				deltaKey = nil
			case 315301840: // KIND_STRING
				keyStr := item.Key.GetString_()
				deltaKey = &keyStr
			}
		}
		if item.Value != nil {
			switch item.Value.GetKind() {
			case 183079996: // KIND_UNSET
				deltaValue = nil
			case 315301840: // KIND_STRING
				valueStr := item.Value.GetString_()
				deltaValue = &valueStr
			}
		}
		if item.Enabled != nil {
			switch item.Enabled.GetKind() {
			case 183079996: // KIND_UNSET
				deltaEnabled = nil
			case 477045804: // KIND_BOOL
				enabledBool := item.Enabled.GetBool()
				deltaEnabled = &enabledBool
			}
		}
		if item.Description != nil {
			switch item.Description.GetKind() {
			case 183079996: // KIND_UNSET
				deltaDescription = nil
			case 315301840: // KIND_STRING
				descStr := item.Description.GetString_()
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
	var updatedParams []mhttpsearchparam.HttpSearchParam

	for _, update := range preparedUpdates {
		if err := httpSearchParamService.UpdateHttpSearchParamDelta(ctx, update.deltaID, update.deltaKey, update.deltaValue, update.deltaEnabled, update.deltaDescription, nil); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Get updated param for event publishing (must get from TX service to see changes)
		updatedParam, err := httpSearchParamService.GetHttpSearchParam(ctx, update.deltaID)
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
			HttpSearchParam: toAPIHttpSearchParam(param),
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
		existingParam *mhttpsearchparam.HttpSearchParam
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
		existingParam, err := h.httpSearchParamService.GetHttpSearchParam(ctx, deltaID)
		if err != nil {
			if errors.Is(err, shttpsearchparam.ErrNoHttpSearchParamFound) {
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
			existingParam *mhttpsearchparam.HttpSearchParam
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
	var deletedParams []mhttpsearchparam.HttpSearchParam
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
			HttpSearchParam: toAPIHttpSearchParam(param),
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

func (h *HttpServiceRPC) HttpAssertInsert(ctx context.Context, req *connect.Request[apiv1.HttpAssertInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP assert must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var insertData []struct {
		assertModel *mhttpassert.HttpAssert
	}

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

		// Verify the HTTP entry exists and user has access - use pool service
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

		// Create the assert model
		assertModel := &mhttpassert.HttpAssert{
			ID:          assertID,
			HttpID:      httpID,
			Key:         "", // HttpAssert doesn't use Key field
			Value:       item.Value,
			Enabled:     true, // Assertions are always active
			Description: "",   // No description in API
			Order:       0,    // No order in API
		}

		insertData = append(insertData, struct {
			assertModel *mhttpassert.HttpAssert
		}{
			assertModel: assertModel,
		})
	}

	// Step 2: Execute inserts in transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	httpAssertService := h.httpAssertService.TX(tx)
	var createdAsserts []mhttpassert.HttpAssert

	for _, data := range insertData {
		if err := httpAssertService.CreateHttpAssert(ctx, data.assertModel); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		createdAsserts = append(createdAsserts, *data.assertModel)
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
			Type:       eventTypeInsert,
			HttpAssert: toAPIHttpAssert(assert),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpAssertUpdate(ctx context.Context, req *connect.Request[apiv1.HttpAssertUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP assert must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var updateData []struct {
		existingAssert *mhttpassert.HttpAssert
		item           *apiv1.HttpAssertUpdate
	}

	for _, item := range req.Msg.Items {
		if len(item.HttpAssertId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_assert_id is required"))
		}

		assertID, err := idwrap.NewFromBytes(item.HttpAssertId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing assert - use pool service
		existingAssert, err := h.httpAssertService.GetHttpAssert(ctx, assertID)
		if err != nil {
			if errors.Is(err, shttpassert.ErrNoHttpAssertFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Verify the HTTP entry exists and user has access - use pool service
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

		updateData = append(updateData, struct {
			existingAssert *mhttpassert.HttpAssert
			item           *apiv1.HttpAssertUpdate
		}{
			existingAssert: existingAssert,
			item:           item,
		})
	}

	// Step 2: Prepare updates (in memory)
	for _, data := range updateData {
		item := data.item
		existingAssert := data.existingAssert

		if item.Value != nil {
			existingAssert.Value = *item.Value
		}
	}

	// Step 3: Execute updates in transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	httpAssertService := h.httpAssertService.TX(tx)
	var updatedAsserts []mhttpassert.HttpAssert

	for _, data := range updateData {
		if err := httpAssertService.UpdateHttpAssert(ctx, data.existingAssert); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		updatedAsserts = append(updatedAsserts, *data.existingAssert)
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

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var deleteData []struct {
		assertID       idwrap.IDWrap
		existingAssert *mhttpassert.HttpAssert
		workspaceID    idwrap.IDWrap
	}

	for _, item := range req.Msg.Items {
		if len(item.HttpAssertId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_assert_id is required"))
		}

		assertID, err := idwrap.NewFromBytes(item.HttpAssertId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing assert - use pool service
		existingAssert, err := h.httpAssertService.GetHttpAssert(ctx, assertID)
		if err != nil {
			if errors.Is(err, shttpassert.ErrNoHttpAssertFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Verify the HTTP entry exists and user has access - use pool service
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

		deleteData = append(deleteData, struct {
			assertID       idwrap.IDWrap
			existingAssert *mhttpassert.HttpAssert
			workspaceID    idwrap.IDWrap
		}{
			assertID:       assertID,
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
		if err := httpAssertService.DeleteHttpAssert(ctx, data.assertID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		deletedAsserts = append(deletedAsserts, *data.existingAssert)
		deletedWorkspaceIDs = append(deletedWorkspaceIDs, data.workspaceID)
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
					// HttpId:            assert.HttpID.Bytes(),
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
	// Stub implementation - delta updates are handled via HttpAssertDeltaCreate
	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpAssertDeltaDelete(ctx context.Context, req *connect.Request[apiv1.HttpAssertDeltaDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	// Stub implementation - delta deletion is not supported
	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpAssertDeltaSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpAssertDeltaSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return h.streamHttpAssertDeltaSync(ctx, userID, stream.Send)
}

func (h *HttpServiceRPC) HttpResponseCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpResponseCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Get user's workspaces
	workspaces, err := h.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var allResponses []*apiv1.HttpResponse
	for _, workspace := range workspaces {
		// Get HTTP entries for this workspace
		httpList, err := h.hs.GetByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Get responses for each HTTP entry
		for _, http := range httpList {
			responses, err := h.DB.QueryContext(ctx, `
				SELECT id, http_id, status, body, time, duration, size, created_at
				FROM http_response
				WHERE http_id = ?
				ORDER BY time DESC
			`, http.ID.Bytes())
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			for responses.Next() {
				var response dbmodels.HttpResponse
				var status int32
				var duration int32
				var size int32
				err := responses.Scan(
					&response.ID,
					&response.HttpID,
					&status,
					&response.Body,
					&response.Time,
					&duration,
					&size,
					&response.CreatedAt,
				)
				if err != nil {
					responses.Close()
					return nil, connect.NewError(connect.CodeInternal, err)
				}
				response.Status = status
				response.Duration = duration
				response.Size = size

				apiResponse := toAPIHttpResponse(response)
				allResponses = append(allResponses, apiResponse)
			}

			if err := responses.Err(); err != nil {
				responses.Close()
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			responses.Close()
		}
	}

	return connect.NewResponse(&apiv1.HttpResponseCollectionResponse{Items: allResponses}), nil
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

	// Snapshot provider for initial state
	snapshot := func(ctx context.Context) ([]eventstream.Event[HttpResponseTopic, HttpResponseEvent], error) {
		// Get all HTTP entries for user
		httpList, err := h.listUserHttp(ctx)
		if err != nil {
			return nil, err
		}

		events := make([]eventstream.Event[HttpResponseTopic, HttpResponseEvent], 0)
		for _, http := range httpList {
			workspaceSet.Store(http.WorkspaceID.String(), struct{}{})

			// Get responses for this HTTP entry
			responses, err := h.DB.QueryContext(ctx, `
				SELECT id, http_id, status, body, time, duration, size, created_at
				FROM http_response
				WHERE http_id = ?
				ORDER BY time DESC
			`, http.ID.Bytes())
			if err != nil {
				return nil, err
			}

			for responses.Next() {
				var response dbmodels.HttpResponse
				var status int32
				var duration int32
				var size int32
				err := responses.Scan(
					&response.ID,
					&response.HttpID,
					&status,
					&response.Body,
					&response.Time,
					&duration,
					&size,
					&response.CreatedAt,
				)
				if err != nil {
					responses.Close()
					return nil, err
				}
				response.Status = status
				response.Duration = duration
				response.Size = size

				events = append(events, eventstream.Event[HttpResponseTopic, HttpResponseEvent]{
					Topic: HttpResponseTopic{WorkspaceID: http.WorkspaceID},
					Payload: HttpResponseEvent{
						Type:         eventTypeInsert,
						HttpResponse: toAPIHttpResponse(response),
					},
				})
			}

			if err := responses.Err(); err != nil {
				responses.Close()
				return nil, err
			}
			responses.Close()
		}
		return events, nil
	}

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

	// Subscribe to events with snapshot
	events, err := h.httpResponseStream.Subscribe(ctx, filter, eventstream.WithSnapshot(snapshot))
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
			resp := httpResponseSyncResponseFrom(evt.Payload)
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

func (h *HttpServiceRPC) HttpResponseHeaderCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpResponseHeaderCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Get user's workspaces
	workspaces, err := h.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var allHeaders []*apiv1.HttpResponseHeader
	for _, workspace := range workspaces {
		// Get HTTP entries for this workspace
		httpList, err := h.hs.GetByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Get response headers for each HTTP entry
		for _, http := range httpList {
			headers, err := h.DB.QueryContext(ctx, `
				SELECT hrh.id, hrh.response_id, hrh.key, hrh.value, hrh.created_at
				FROM http_response_header hrh
				JOIN http_response hr ON hrh.response_id = hr.id
				WHERE hr.http_id = ?
				ORDER BY hrh.created_at DESC
			`, http.ID.Bytes())
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			for headers.Next() {
				var header dbmodels.HttpResponseHeader
				err := headers.Scan(
					&header.ID,
					&header.ResponseID,
					&header.Key,
					&header.Value,
					&header.CreatedAt,
				)
				if err != nil {
					headers.Close()
					return nil, connect.NewError(connect.CodeInternal, err)
				}

				apiHeader := toAPIHttpResponseHeader(header)
				allHeaders = append(allHeaders, apiHeader)
			}

			if err := headers.Err(); err != nil {
				headers.Close()
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			headers.Close()
		}
	}

	return connect.NewResponse(&apiv1.HttpResponseHeaderCollectionResponse{Items: allHeaders}), nil
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

	// Snapshot provider for initial state
	snapshot := func(ctx context.Context) ([]eventstream.Event[HttpResponseHeaderTopic, HttpResponseHeaderEvent], error) {
		// Get all HTTP entries for user
		httpList, err := h.listUserHttp(ctx)
		if err != nil {
			return nil, err
		}

		events := make([]eventstream.Event[HttpResponseHeaderTopic, HttpResponseHeaderEvent], 0)
		for _, http := range httpList {
			workspaceSet.Store(http.WorkspaceID.String(), struct{}{})

			// Get response headers for this HTTP entry
			headers, err := h.DB.QueryContext(ctx, `
				SELECT hrh.id, hrh.response_id, hrh.key, hrh.value, hrh.created_at
				FROM http_response_header hrh
				JOIN http_response hr ON hrh.response_id = hr.id
				WHERE hr.http_id = ?
				ORDER BY hrh.created_at DESC
			`, http.ID.Bytes())
			if err != nil {
				return nil, err
			}

			for headers.Next() {
				var header dbmodels.HttpResponseHeader
				err := headers.Scan(
					&header.ID,
					&header.ResponseID,
					&header.Key,
					&header.Value,
					&header.CreatedAt,
				)
				if err != nil {
					headers.Close()
					return nil, err
				}

				events = append(events, eventstream.Event[HttpResponseHeaderTopic, HttpResponseHeaderEvent]{
					Topic: HttpResponseHeaderTopic{WorkspaceID: http.WorkspaceID},
					Payload: HttpResponseHeaderEvent{
						Type:               eventTypeInsert,
						HttpResponseHeader: toAPIHttpResponseHeader(header),
					},
				})
			}

			if err := headers.Err(); err != nil {
				headers.Close()
				return nil, err
			}
			headers.Close()
		}
		return events, nil
	}

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

	// Subscribe to events with snapshot
	events, err := h.httpResponseHeaderStream.Subscribe(ctx, filter, eventstream.WithSnapshot(snapshot))
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
			resp := httpResponseHeaderSyncResponseFrom(evt.Payload)
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

func (h *HttpServiceRPC) HttpResponseAssertCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpResponseAssertCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Get user's workspaces
	workspaces, err := h.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var allAsserts []*apiv1.HttpResponseAssert
	for _, workspace := range workspaces {
		// Get HTTP entries for this workspace
		httpList, err := h.hs.GetByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Get response asserts for each HTTP entry
		for _, http := range httpList {
			asserts, err := h.DB.QueryContext(ctx, `
				SELECT hra.id, hra.response_id, hra.value, hra.success, hra.created_at
				FROM http_response_assert hra
				JOIN http_response hr ON hra.response_id = hr.id
				WHERE hr.http_id = ?
				ORDER BY hra.created_at DESC
			`, http.ID.Bytes())
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			for asserts.Next() {
				var assert dbmodels.HttpResponseAssert
				err := asserts.Scan(
					&assert.ID,
					&assert.ResponseID,
					&assert.Value,
					&assert.Success,
					&assert.CreatedAt,
				)
				if err != nil {
					asserts.Close()
					return nil, connect.NewError(connect.CodeInternal, err)
				}

				apiAssert := toAPIHttpResponseAssert(assert)
				allAsserts = append(allAsserts, apiAssert)
			}

			if err := asserts.Err(); err != nil {
				asserts.Close()
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			asserts.Close()
		}
	}

	return connect.NewResponse(&apiv1.HttpResponseAssertCollectionResponse{Items: allAsserts}), nil
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

	// Snapshot provider for initial state
	snapshot := func(ctx context.Context) ([]eventstream.Event[HttpResponseAssertTopic, HttpResponseAssertEvent], error) {
		// Get all HTTP entries for user
		httpList, err := h.listUserHttp(ctx)
		if err != nil {
			return nil, err
		}

		events := make([]eventstream.Event[HttpResponseAssertTopic, HttpResponseAssertEvent], 0)
		for _, http := range httpList {
			workspaceSet.Store(http.WorkspaceID.String(), struct{}{})

			// Get response asserts for this HTTP entry
			asserts, err := h.DB.QueryContext(ctx, `
				SELECT hra.id, hra.response_id, hra.value, hra.success, hra.created_at
				FROM http_response_assert hra
				JOIN http_response hr ON hra.response_id = hr.id
				WHERE hr.http_id = ?
				ORDER BY hra.created_at DESC
			`, http.ID.Bytes())
			if err != nil {
				return nil, err
			}

			for asserts.Next() {
				var assert dbmodels.HttpResponseAssert
				err := asserts.Scan(
					&assert.ID,
					&assert.ResponseID,
					&assert.Value,
					&assert.Success,
					&assert.CreatedAt,
				)
				if err != nil {
					asserts.Close()
					return nil, err
				}

				events = append(events, eventstream.Event[HttpResponseAssertTopic, HttpResponseAssertEvent]{
					Topic: HttpResponseAssertTopic{WorkspaceID: http.WorkspaceID},
					Payload: HttpResponseAssertEvent{
						Type:               eventTypeInsert,
						HttpResponseAssert: toAPIHttpResponseAssert(assert),
					},
				})
			}

			if err := asserts.Err(); err != nil {
				asserts.Close()
				return nil, err
			}
			asserts.Close()
		}
		return events, nil
	}

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

	// Subscribe to events with snapshot
	events, err := h.httpResponseAssertStream.Subscribe(ctx, filter, eventstream.WithSnapshot(snapshot))
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
			resp := httpResponseAssertSyncResponseFrom(evt.Payload)
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

func (h *HttpServiceRPC) HttpHeaderInsert(ctx context.Context, req *connect.Request[apiv1.HttpHeaderInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP header must be provided"))
	}

	// Step 1: Process request data and perform all reads/checks OUTSIDE transaction
	var insertData []struct {
		headerID    idwrap.IDWrap
		httpID      idwrap.IDWrap
		key         string
		value       string
		enabled     bool
		description string
		order       float64
		workspaceID idwrap.IDWrap
	}

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

		// Verify the HTTP entry exists and user has access - use pool service
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

		insertData = append(insertData, struct {
			headerID    idwrap.IDWrap
			httpID      idwrap.IDWrap
			key         string
			value       string
			enabled     bool
			description string
			order       float64
			workspaceID idwrap.IDWrap
		}{
			headerID:    headerID,
			httpID:      httpID,
			key:         item.Key,
			value:       item.Value,
			enabled:     item.Enabled,
			description: item.Description,
			order:       float64(item.Order),
			workspaceID: httpEntry.WorkspaceID,
		})
	}

	// Step 2: Minimal write transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	httpHeaderService := h.httpHeaderService.TX(tx)

	var createdHeaders []mhttpheader.HttpHeader

	for _, data := range insertData {
		// Create the header
		headerModel := &mhttpheader.HttpHeader{
			ID:          data.headerID,
			HttpID:      data.httpID,
			Key:         data.key,
			Value:       data.value,
			Enabled:     data.enabled,
			Description: data.description,
			Order:       float32(data.order),
		}

		if err := httpHeaderService.Create(ctx, headerModel); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		createdHeaders = append(createdHeaders, *headerModel)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Step 3: Publish create events for real-time sync
	for i, header := range createdHeaders {
		h.httpHeaderStream.Publish(HttpHeaderTopic{WorkspaceID: insertData[i].workspaceID}, HttpHeaderEvent{
			Type:       eventTypeInsert,
			HttpHeader: toAPIHttpHeader(header),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpHeaderUpdate(ctx context.Context, req *connect.Request[apiv1.HttpHeaderUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP header must be provided"))
	}

	// Step 1: Process request data and perform all reads/checks OUTSIDE transaction
	var updateData []struct {
		existingHeader mhttpheader.HttpHeader
		key            *string
		value          *string
		enabled        *bool
		description    *string
		order          *float32
		workspaceID    idwrap.IDWrap
	}

	for _, item := range req.Msg.Items {
		if len(item.HttpHeaderId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_header_id is required"))
		}

		headerID, err := idwrap.NewFromBytes(item.HttpHeaderId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing header - use pool service
		existingHeader, err := h.httpHeaderService.GetByID(ctx, headerID)
		if err != nil {
			if errors.Is(err, shttpheader.ErrNoHttpHeaderFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
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
			existingHeader mhttpheader.HttpHeader
			key            *string
			value          *string
			enabled        *bool
			description    *string
			order          *float32
			workspaceID    idwrap.IDWrap
		}{
			existingHeader: existingHeader,
			key:            item.Key,
			value:          item.Value,
			enabled:        item.Enabled,
			description:    item.Description,
			order:          item.Order,
			workspaceID:    httpEntry.WorkspaceID,
		})
	}

	// Step 2: Minimal write transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	httpHeaderService := h.httpHeaderService.TX(tx)

	var updatedHeaders []mhttpheader.HttpHeader

	for _, data := range updateData {
		header := data.existingHeader

		// Update fields if provided
		if data.key != nil {
			header.Key = *data.key
		}
		if data.value != nil {
			header.Value = *data.value
		}
		if data.enabled != nil {
			header.Enabled = *data.enabled
		}
		if data.description != nil {
			header.Description = *data.description
		}
		if data.order != nil {
			header.Order = *data.order
		}

		if err := httpHeaderService.Update(ctx, &header); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		updatedHeaders = append(updatedHeaders, header)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Step 3: Publish update events for real-time sync
	for i, header := range updatedHeaders {
		h.httpHeaderStream.Publish(HttpHeaderTopic{WorkspaceID: updateData[i].workspaceID}, HttpHeaderEvent{
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

	// Step 1: Process request data and perform all reads/checks OUTSIDE transaction
	var deleteData []struct {
		headerID    idwrap.IDWrap
		workspaceID idwrap.IDWrap
	}

	for _, item := range req.Msg.Items {
		if len(item.HttpHeaderId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_header_id is required"))
		}

		headerID, err := idwrap.NewFromBytes(item.HttpHeaderId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing header - use pool service
		existingHeader, err := h.httpHeaderService.GetByID(ctx, headerID)
		if err != nil {
			if errors.Is(err, shttpheader.ErrNoHttpHeaderFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
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
			headerID    idwrap.IDWrap
			workspaceID idwrap.IDWrap
		}{
			headerID:    headerID,
			workspaceID: httpEntry.WorkspaceID,
		})
	}

	// Step 2: Minimal write transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	httpHeaderService := h.httpHeaderService.TX(tx)

	var deletedHeaders []idwrap.IDWrap

	for _, data := range deleteData {
		if err := httpHeaderService.Delete(ctx, data.headerID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		deletedHeaders = append(deletedHeaders, data.headerID)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Step 3: Publish delete events for real-time sync
	for i, headerID := range deletedHeaders {
		h.httpHeaderStream.Publish(HttpHeaderTopic{WorkspaceID: deleteData[i].workspaceID}, HttpHeaderEvent{
			Type: eventTypeDelete,
			HttpHeader: &apiv1.HttpHeader{
				HttpHeaderId: headerID.Bytes(),
			},
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
						Type:       eventTypeInsert,
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
					// HttpId:            header.HttpID.Bytes(),
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
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP header delta must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var updateData []struct {
		deltaID        idwrap.IDWrap
		existingHeader mhttpheader.HttpHeader
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
			if errors.Is(err, shttpheader.ErrNoHttpHeaderFound) {
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
			existingHeader mhttpheader.HttpHeader
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
			case 183079996: // KIND_UNSET
				deltaKey = nil
			case 315301840: // KIND_STRING
				keyStr := item.Key.GetString_()
				deltaKey = &keyStr
			}
		}
		if item.Value != nil {
			switch item.Value.GetKind() {
			case 183079996: // KIND_UNSET
				deltaValue = nil
			case 315301840: // KIND_STRING
				valueStr := item.Value.GetString_()
				deltaValue = &valueStr
			}
		}
		if item.Enabled != nil {
			switch item.Enabled.GetKind() {
			case 183079996: // KIND_UNSET
				deltaEnabled = nil
			case 477045804: // KIND_BOOL
				enabledBool := item.Enabled.GetBool()
				deltaEnabled = &enabledBool
			}
		}
		if item.Description != nil {
			switch item.Description.GetKind() {
			case 183079996: // KIND_UNSET
				deltaDescription = nil
			case 315301840: // KIND_STRING
				descStr := item.Description.GetString_()
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
	var updatedHeaders []mhttpheader.HttpHeader

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
			HttpHeader: toAPIHttpHeader(header),
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
		existingHeader mhttpheader.HttpHeader
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
			if errors.Is(err, shttpheader.ErrNoHttpHeaderFound) {
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
			existingHeader mhttpheader.HttpHeader
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
	var deletedHeaders []mhttpheader.HttpHeader
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
			HttpHeader: toAPIHttpHeader(header),
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

func (h *HttpServiceRPC) HttpBodyFormDataCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpBodyFormDataCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Get user's workspaces
	workspaces, err := h.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var allBodyForms []*apiv1.HttpBodyFormData
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
				apiBodyForm := toAPIHttpBodyFormData(bodyForm)
				allBodyForms = append(allBodyForms, apiBodyForm)
			}
		}
	}

	return connect.NewResponse(&apiv1.HttpBodyFormDataCollectionResponse{Items: allBodyForms}), nil
}

func (h *HttpServiceRPC) HttpBodyFormDataInsert(ctx context.Context, req *connect.Request[apiv1.HttpBodyFormDataInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP body form must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var insertData []struct {
		bodyFormModel *mhttpbodyform.HttpBodyForm
	}

	for _, item := range req.Msg.Items {
		if len(item.HttpBodyFormDataId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_body_form_data_id is required"))
		}
		if len(item.HttpId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_id is required"))
		}

		bodyFormID, err := idwrap.NewFromBytes(item.HttpBodyFormDataId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		httpID, err := idwrap.NewFromBytes(item.HttpId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Verify the HTTP entry exists and user has access - use pool service
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

		// Create the body form model
		bodyFormModel := &mhttpbodyform.HttpBodyForm{
			ID:          bodyFormID,
			HttpID:      httpID,
			Key:         item.Key,
			Value:       item.Value,
			Enabled:     item.Enabled,
			Description: item.Description,
			Order:       item.Order,
		}

		insertData = append(insertData, struct {
			bodyFormModel *mhttpbodyform.HttpBodyForm
		}{
			bodyFormModel: bodyFormModel,
		})
	}

	// Step 2: Execute inserts in transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	httpBodyFormService := h.httpBodyFormService.TX(tx)

	var createdBodyForms []mhttpbodyform.HttpBodyForm

	for _, data := range insertData {
		if err := httpBodyFormService.CreateHttpBodyForm(ctx, data.bodyFormModel); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		createdBodyForms = append(createdBodyForms, *data.bodyFormModel)
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
			Type:         eventTypeInsert,
			HttpBodyForm: toAPIHttpBodyFormData(bodyForm),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyFormDataUpdate(ctx context.Context, req *connect.Request[apiv1.HttpBodyFormDataUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP body form must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var updateData []struct {
		existingBodyForm *mhttpbodyform.HttpBodyForm
		item             *apiv1.HttpBodyFormDataUpdate
	}

	for _, item := range req.Msg.Items {
		if len(item.HttpBodyFormDataId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_body_form_data_id is required"))
		}

		bodyFormID, err := idwrap.NewFromBytes(item.HttpBodyFormDataId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing body form - use pool service
		existingBodyForm, err := h.httpBodyFormService.GetHttpBodyForm(ctx, bodyFormID)
		if err != nil {
			if errors.Is(err, shttpbodyform.ErrNoHttpBodyFormFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Verify the HTTP entry exists and user has access - use pool service
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

		updateData = append(updateData, struct {
			existingBodyForm *mhttpbodyform.HttpBodyForm
			item             *apiv1.HttpBodyFormDataUpdate
		}{
			existingBodyForm: existingBodyForm,
			item:             item,
		})
	}

	// Step 2: Prepare updates (in memory)
	for _, data := range updateData {
		item := data.item
		existingBodyForm := data.existingBodyForm

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
	}

	// Step 3: Execute updates in transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	httpBodyFormService := h.httpBodyFormService.TX(tx)
	var updatedBodyForms []mhttpbodyform.HttpBodyForm

	for _, data := range updateData {
		if err := httpBodyFormService.UpdateHttpBodyForm(ctx, data.existingBodyForm); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		updatedBodyForms = append(updatedBodyForms, *data.existingBodyForm)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish update events for real-time sync
	for _, bodyForm := range updatedBodyForms {
		// Get workspace ID for the HTTP entry
		httpEntry, err := h.hs.Get(ctx, bodyForm.HttpID)
		if err != nil {
			continue
		}
		h.httpBodyFormStream.Publish(HttpBodyFormTopic{WorkspaceID: httpEntry.WorkspaceID}, HttpBodyFormEvent{
			Type:         eventTypeUpdate,
			HttpBodyForm: toAPIHttpBodyFormData(bodyForm),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyFormDataDelete(ctx context.Context, req *connect.Request[apiv1.HttpBodyFormDataDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP body form must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var deleteData []struct {
		bodyFormID       idwrap.IDWrap
		existingBodyForm *mhttpbodyform.HttpBodyForm
		workspaceID      idwrap.IDWrap
	}

	for _, item := range req.Msg.Items {
		if len(item.HttpBodyFormDataId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_body_form_data_id is required"))
		}

		bodyFormID, err := idwrap.NewFromBytes(item.HttpBodyFormDataId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing body form - use pool service
		existingBodyForm, err := h.httpBodyFormService.GetHttpBodyForm(ctx, bodyFormID)
		if err != nil {
			if errors.Is(err, shttpbodyform.ErrNoHttpBodyFormFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Verify the HTTP entry exists and user has access - use pool service
		httpEntry, err := h.hs.Get(ctx, existingBodyForm.HttpID)
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

		deleteData = append(deleteData, struct {
			bodyFormID       idwrap.IDWrap
			existingBodyForm *mhttpbodyform.HttpBodyForm
			workspaceID      idwrap.IDWrap
		}{
			bodyFormID:       bodyFormID,
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
	var deletedBodyForms []mhttpbodyform.HttpBodyForm
	var deletedWorkspaceIDs []idwrap.IDWrap

	for _, data := range deleteData {
		if err := httpBodyFormService.DeleteHttpBodyForm(ctx, data.bodyFormID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		deletedBodyForms = append(deletedBodyForms, *data.existingBodyForm)
		deletedWorkspaceIDs = append(deletedWorkspaceIDs, data.workspaceID)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish delete events for real-time sync
	for i, bodyForm := range deletedBodyForms {
		h.httpBodyFormStream.Publish(HttpBodyFormTopic{WorkspaceID: deletedWorkspaceIDs[i]}, HttpBodyFormEvent{
			Type:         eventTypeDelete,
			HttpBodyForm: toAPIHttpBodyFormData(bodyForm),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyFormDataSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpBodyFormDataSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return h.streamHttpBodyFormSync(ctx, userID, stream.Send)
}

func httpBodyFormDataSyncResponseFrom(event HttpBodyFormEvent) *apiv1.HttpBodyFormDataSyncResponse {
	var value *apiv1.HttpBodyFormDataSync_ValueUnion

	switch event.Type {
	case eventTypeInsert:
		key := event.HttpBodyForm.GetKey()
		value_ := event.HttpBodyForm.GetValue()
		enabled := event.HttpBodyForm.GetEnabled()
		description := event.HttpBodyForm.GetDescription()
		order := event.HttpBodyForm.GetOrder()
		value = &apiv1.HttpBodyFormDataSync_ValueUnion{
			Kind: apiv1.HttpBodyFormDataSync_ValueUnion_KIND_INSERT,
			Insert: &apiv1.HttpBodyFormDataSyncInsert{
				HttpBodyFormDataId: event.HttpBodyForm.GetHttpBodyFormDataId(),
				HttpId:             event.HttpBodyForm.GetHttpId(),
				Key:                key,
				Value:              value_,
				Enabled:            enabled,
				Description:        description,
				Order:              order,
			},
		}
	case eventTypeUpdate:
		value = &apiv1.HttpBodyFormDataSync_ValueUnion{
			Kind: apiv1.HttpBodyFormDataSync_ValueUnion_KIND_UPDATE,
			Update: &apiv1.HttpBodyFormDataSyncUpdate{
				HttpBodyFormDataId: event.HttpBodyForm.GetHttpBodyFormDataId(),
			},
		}
	case eventTypeDelete:
		value = &apiv1.HttpBodyFormDataSync_ValueUnion{
			Kind: apiv1.HttpBodyFormDataSync_ValueUnion_KIND_DELETE,
			Delete: &apiv1.HttpBodyFormDataSyncDelete{
				HttpBodyFormDataId: event.HttpBodyForm.GetHttpBodyFormDataId(),
			},
		}
	}

	return &apiv1.HttpBodyFormDataSyncResponse{
		Items: []*apiv1.HttpBodyFormDataSync{
			{
				Value: value,
			},
		},
	}
}

// streamHttpSearchParamDeltaSync streams HTTP search param delta events to the client
func (h *HttpServiceRPC) streamHttpSearchParamDeltaSync(ctx context.Context, userID idwrap.IDWrap, send func(*apiv1.HttpSearchParamDeltaSyncResponse) error) error {
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
				if !param.IsDelta {
					continue // Only include delta records
				}
				events = append(events, eventstream.Event[HttpSearchParamTopic, HttpSearchParamEvent]{
					Topic: HttpSearchParamTopic{WorkspaceID: http.WorkspaceID},
					Payload: HttpSearchParamEvent{
						Type:            eventTypeInsert,
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
			// Get the full param record for delta sync response
			paramID, err := idwrap.NewFromBytes(evt.Payload.HttpSearchParam.GetHttpSearchParamId())
			if err != nil {
				continue // Skip if can't parse ID
			}
			paramRecord, err := h.httpSearchParamService.GetHttpSearchParam(ctx, paramID)
			if err != nil {
				continue // Skip if can't get the record
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
func (h *HttpServiceRPC) streamHttpHeaderDeltaSync(ctx context.Context, userID idwrap.IDWrap, send func(*apiv1.HttpHeaderDeltaSyncResponse) error) error {
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
				if !header.IsDelta {
					continue // Only include delta records
				}
				events = append(events, eventstream.Event[HttpHeaderTopic, HttpHeaderEvent]{
					Topic: HttpHeaderTopic{WorkspaceID: http.WorkspaceID},
					Payload: HttpHeaderEvent{
						Type:       eventTypeInsert,
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
			// Get the full header record for delta sync response
			headerID, err := idwrap.NewFromBytes(evt.Payload.HttpHeader.GetHttpHeaderId())
			if err != nil {
				continue // Skip if can't parse ID
			}
			headerRecord, err := h.httpHeaderService.GetByID(ctx, headerID)
			if err != nil {
				continue // Skip if can't get the record
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
func (h *HttpServiceRPC) streamHttpBodyFormDeltaSync(ctx context.Context, userID idwrap.IDWrap, send func(*apiv1.HttpBodyFormDataDeltaSyncResponse) error) error {
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
				if !bodyForm.IsDelta {
					continue // Only include delta records
				}
				events = append(events, eventstream.Event[HttpBodyFormTopic, HttpBodyFormEvent]{
					Topic: HttpBodyFormTopic{WorkspaceID: http.WorkspaceID},
					Payload: HttpBodyFormEvent{
						Type:         eventTypeInsert,
						HttpBodyForm: toAPIHttpBodyFormData(bodyForm),
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
			// Get the full body form record for delta sync response
			bodyFormID, err := idwrap.NewFromBytes(evt.Payload.HttpBodyForm.GetHttpBodyFormDataId())
			if err != nil {
				continue // Skip if can't parse ID
			}
			bodyFormRecord, err := h.httpBodyFormService.GetHttpBodyForm(ctx, bodyFormID)
			if err != nil {
				continue // Skip if can't get the record
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
func (h *HttpServiceRPC) streamHttpAssertDeltaSync(ctx context.Context, userID idwrap.IDWrap, send func(*apiv1.HttpAssertDeltaSyncResponse) error) error {
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
				if !assert.IsDelta {
					continue // Only include delta records
				}
				events = append(events, eventstream.Event[HttpAssertTopic, HttpAssertEvent]{
					Topic: HttpAssertTopic{WorkspaceID: http.WorkspaceID},
					Payload: HttpAssertEvent{
						Type:       eventTypeInsert,
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
			// Get the full assert record for delta sync response
			assertID, err := idwrap.NewFromBytes(evt.Payload.HttpAssert.GetHttpAssertId())
			if err != nil {
				continue // Skip if can't parse ID
			}
			assertRecord, err := h.httpAssertService.GetHttpAssert(ctx, assertID)
			if err != nil {
				continue // Skip if can't get the record
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
func (h *HttpServiceRPC) streamHttpBodyFormSync(ctx context.Context, userID idwrap.IDWrap, send func(*apiv1.HttpBodyFormDataSyncResponse) error) error {
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
						Type:         eventTypeInsert,
						HttpBodyForm: toAPIHttpBodyFormData(bodyForm),
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
			resp := httpBodyFormDataSyncResponseFrom(evt.Payload)
			if err := send(resp); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

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
				delta := &apiv1.HttpBodyFormDataDelta{
					DeltaHttpBodyFormDataId: bodyForm.ID.Bytes(),
					HttpBodyFormDataId:      bodyForm.ID.Bytes(),
					// HttpId:                  bodyForm.HttpID.Bytes(),
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

func (h *HttpServiceRPC) HttpBodyFormDataDeltaUpdate(ctx context.Context, req *connect.Request[apiv1.HttpBodyFormDataDeltaUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	// Stub implementation - delta updates are handled via HttpBodyFormDeltaCreate
	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyFormDataDeltaDelete(ctx context.Context, req *connect.Request[apiv1.HttpBodyFormDataDeltaDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	// Stub implementation - delta deletion is not supported
	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyFormDataDeltaSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpBodyFormDataDeltaSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return h.streamHttpBodyFormDeltaSync(ctx, userID, stream.Send)
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

func (h *HttpServiceRPC) HttpBodyUrlEncodedInsert(ctx context.Context, req *connect.Request[apiv1.HttpBodyUrlEncodedInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP body URL encoded must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var insertData []struct {
		bodyUrlEncodedModel *mhttpbodyurlencoded.HttpBodyUrlEncoded
	}

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

		// Verify the HTTP entry exists and user has access - use pool service
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

		// Create the body URL encoded model
		bodyUrlEncodedModel := &mhttpbodyurlencoded.HttpBodyUrlEncoded{
			ID:          bodyUrlEncodedID,
			HttpID:      httpID,
			Key:         item.Key,
			Value:       item.Value,
			Enabled:     item.Enabled,
			Description: item.Description,
			Order:       item.Order,
		}

		insertData = append(insertData, struct {
			bodyUrlEncodedModel *mhttpbodyurlencoded.HttpBodyUrlEncoded
		}{
			bodyUrlEncodedModel: bodyUrlEncodedModel,
		})
	}

	// Step 2: Execute inserts in transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	httpBodyUrlEncodedService := h.httpBodyUrlEncodedService.TX(tx)
	var createdBodyUrlEncodeds []mhttpbodyurlencoded.HttpBodyUrlEncoded

	for _, data := range insertData {
		if err := httpBodyUrlEncodedService.CreateHttpBodyUrlEncoded(ctx, data.bodyUrlEncodedModel); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		createdBodyUrlEncodeds = append(createdBodyUrlEncodeds, *data.bodyUrlEncodedModel)
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
			Type:               eventTypeInsert,
			HttpBodyUrlEncoded: toAPIHttpBodyUrlEncoded(bodyUrlEncoded),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyUrlEncodedUpdate(ctx context.Context, req *connect.Request[apiv1.HttpBodyUrlEncodedUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP body URL encoded must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var updateData []struct {
		existingBodyUrlEncoded *mhttpbodyurlencoded.HttpBodyUrlEncoded
		item                   *apiv1.HttpBodyUrlEncodedUpdate
	}

	for _, item := range req.Msg.Items {
		if len(item.HttpBodyUrlEncodedId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_body_url_encoded_id is required"))
		}

		bodyUrlEncodedID, err := idwrap.NewFromBytes(item.HttpBodyUrlEncodedId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing body URL encoded - use pool service
		existingBodyUrlEncoded, err := h.httpBodyUrlEncodedService.GetHttpBodyUrlEncoded(ctx, bodyUrlEncodedID)
		if err != nil {
			if errors.Is(err, shttpbodyurlencoded.ErrNoHttpBodyUrlEncodedFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Verify the HTTP entry exists and user has access - use pool service
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

		updateData = append(updateData, struct {
			existingBodyUrlEncoded *mhttpbodyurlencoded.HttpBodyUrlEncoded
			item                   *apiv1.HttpBodyUrlEncodedUpdate
		}{
			existingBodyUrlEncoded: existingBodyUrlEncoded,
			item:                   item,
		})
	}

	// Step 2: Prepare updates (in memory)
	for _, data := range updateData {
		item := data.item
		existingBodyUrlEncoded := data.existingBodyUrlEncoded

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
	}

	// Step 3: Execute updates in transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	httpBodyUrlEncodedService := h.httpBodyUrlEncodedService.TX(tx)
	var updatedBodyUrlEncodeds []mhttpbodyurlencoded.HttpBodyUrlEncoded

	for _, data := range updateData {
		if err := httpBodyUrlEncodedService.UpdateHttpBodyUrlEncoded(ctx, data.existingBodyUrlEncoded); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		updatedBodyUrlEncodeds = append(updatedBodyUrlEncodeds, *data.existingBodyUrlEncoded)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish update events for real-time sync
	for _, bodyUrlEncoded := range updatedBodyUrlEncodeds {
		// Get workspace ID for the HTTP entry
		httpEntry, err := h.hs.Get(ctx, bodyUrlEncoded.HttpID)
		if err != nil {
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

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var deleteData []struct {
		bodyUrlEncodedID       idwrap.IDWrap
		existingBodyUrlEncoded *mhttpbodyurlencoded.HttpBodyUrlEncoded
		workspaceID            idwrap.IDWrap
	}

	for _, item := range req.Msg.Items {
		if len(item.HttpBodyUrlEncodedId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_body_url_encoded_id is required"))
		}

		bodyUrlEncodedID, err := idwrap.NewFromBytes(item.HttpBodyUrlEncodedId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing body URL encoded - use pool service
		existingBodyUrlEncoded, err := h.httpBodyUrlEncodedService.GetHttpBodyUrlEncoded(ctx, bodyUrlEncodedID)
		if err != nil {
			if errors.Is(err, shttpbodyurlencoded.ErrNoHttpBodyUrlEncodedFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Verify the HTTP entry exists and user has access - use pool service
		httpEntry, err := h.hs.Get(ctx, existingBodyUrlEncoded.HttpID)
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

		deleteData = append(deleteData, struct {
			bodyUrlEncodedID       idwrap.IDWrap
			existingBodyUrlEncoded *mhttpbodyurlencoded.HttpBodyUrlEncoded
			workspaceID            idwrap.IDWrap
		}{
			bodyUrlEncodedID:       bodyUrlEncodedID,
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
	var deletedBodyUrlEncodeds []mhttpbodyurlencoded.HttpBodyUrlEncoded
	var deletedWorkspaceIDs []idwrap.IDWrap

	for _, data := range deleteData {
		if err := httpBodyUrlEncodedService.DeleteHttpBodyUrlEncoded(ctx, data.bodyUrlEncodedID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		deletedBodyUrlEncodeds = append(deletedBodyUrlEncodeds, *data.existingBodyUrlEncoded)
		deletedWorkspaceIDs = append(deletedWorkspaceIDs, data.workspaceID)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish delete events for real-time sync
	for i, bodyUrlEncoded := range deletedBodyUrlEncodeds {
		h.httpBodyUrlEncodedStream.Publish(HttpBodyUrlEncodedTopic{WorkspaceID: deletedWorkspaceIDs[i]}, HttpBodyUrlEncodedEvent{
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
	case eventTypeInsert:
		key := event.HttpBodyUrlEncoded.GetKey()
		value_ := event.HttpBodyUrlEncoded.GetValue()
		enabled := event.HttpBodyUrlEncoded.GetEnabled()
		description := event.HttpBodyUrlEncoded.GetDescription()
		order := event.HttpBodyUrlEncoded.GetOrder()
		value = &apiv1.HttpBodyUrlEncodedSync_ValueUnion{
			Kind: apiv1.HttpBodyUrlEncodedSync_ValueUnion_KIND_INSERT,
			Insert: &apiv1.HttpBodyUrlEncodedSyncInsert{
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

// httpSearchParamDeltaSyncResponseFrom converts HttpSearchParamEvent and param record to HttpSearchParamDeltaSync response
func httpSearchParamDeltaSyncResponseFrom(event HttpSearchParamEvent, param mhttpsearchparam.HttpSearchParam) *apiv1.HttpSearchParamDeltaSyncResponse {
	var value *apiv1.HttpSearchParamDeltaSync_ValueUnion

	switch event.Type {
	case eventTypeInsert:
		delta := &apiv1.HttpSearchParamDeltaSyncInsert{
			DeltaHttpSearchParamId: param.ID.Bytes(),
		}
		if param.ParentHttpSearchParamID != nil {
			delta.HttpSearchParamId = param.ParentHttpSearchParamID.Bytes()
		}
		delta.HttpId = param.HttpID.Bytes()
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
		value = &apiv1.HttpSearchParamDeltaSync_ValueUnion{
			Kind:   apiv1.HttpSearchParamDeltaSync_ValueUnion_KIND_INSERT,
			Insert: delta,
		}
	case eventTypeUpdate:
		delta := &apiv1.HttpSearchParamDeltaSyncUpdate{
			DeltaHttpSearchParamId: param.ID.Bytes(),
		}
		if param.ParentHttpSearchParamID != nil {
			delta.HttpSearchParamId = param.ParentHttpSearchParamID.Bytes()
		}
		delta.HttpId = param.HttpID.Bytes()
		if param.DeltaKey != nil {
			keyStr := *param.DeltaKey
			delta.Key = &apiv1.HttpSearchParamDeltaSyncUpdate_KeyUnion{
				Kind:    315301840, // KIND_STRING
				String_: &keyStr,
			}
		}
		if param.DeltaValue != nil {
			valueStr := *param.DeltaValue
			delta.Value = &apiv1.HttpSearchParamDeltaSyncUpdate_ValueUnion{
				Kind:    315301840, // KIND_STRING
				String_: &valueStr,
			}
		}
		if param.DeltaEnabled != nil {
			enabledBool := *param.DeltaEnabled
			delta.Enabled = &apiv1.HttpSearchParamDeltaSyncUpdate_EnabledUnion{
				Kind: 477045804, // KIND_BOOL
				Bool: &enabledBool,
			}
		}
		if param.DeltaDescription != nil {
			descStr := *param.DeltaDescription
			delta.Description = &apiv1.HttpSearchParamDeltaSyncUpdate_DescriptionUnion{
				Kind:    315301840, // KIND_STRING
				String_: &descStr,
			}
		}
		if param.DeltaOrder != nil {
			orderFloat := float32(*param.DeltaOrder)
			delta.Order = &apiv1.HttpSearchParamDeltaSyncUpdate_OrderUnion{
				Kind:  182966389, // KIND_FLOAT
				Float: &orderFloat,
			}
		}
		value = &apiv1.HttpSearchParamDeltaSync_ValueUnion{
			Kind:   apiv1.HttpSearchParamDeltaSync_ValueUnion_KIND_UPDATE,
			Update: delta,
		}
	case eventTypeDelete:
		value = &apiv1.HttpSearchParamDeltaSync_ValueUnion{
			Kind: apiv1.HttpSearchParamDeltaSync_ValueUnion_KIND_DELETE,
			Delete: &apiv1.HttpSearchParamDeltaSyncDelete{
				DeltaHttpSearchParamId: param.ID.Bytes(),
			},
		}
	}

	return &apiv1.HttpSearchParamDeltaSyncResponse{
		Items: []*apiv1.HttpSearchParamDeltaSync{
			{
				Value: value,
			},
		},
	}
}

// httpHeaderDeltaSyncResponseFrom converts HttpHeaderEvent and header record to HttpHeaderDeltaSync response
func httpHeaderDeltaSyncResponseFrom(event HttpHeaderEvent, header mhttpheader.HttpHeader) *apiv1.HttpHeaderDeltaSyncResponse {
	var value *apiv1.HttpHeaderDeltaSync_ValueUnion

	switch event.Type {
	case eventTypeInsert:
		delta := &apiv1.HttpHeaderDeltaSyncInsert{
			DeltaHttpHeaderId: header.ID.Bytes(),
		}
		if header.ParentHttpHeaderID != nil {
			delta.HttpHeaderId = header.ParentHttpHeaderID.Bytes()
		}
		delta.HttpId = header.HttpID.Bytes()
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
		value = &apiv1.HttpHeaderDeltaSync_ValueUnion{
			Kind:   apiv1.HttpHeaderDeltaSync_ValueUnion_KIND_INSERT,
			Insert: delta,
		}
	case eventTypeUpdate:
		delta := &apiv1.HttpHeaderDeltaSyncUpdate{
			DeltaHttpHeaderId: header.ID.Bytes(),
		}
		if header.ParentHttpHeaderID != nil {
			delta.HttpHeaderId = header.ParentHttpHeaderID.Bytes()
		}
		delta.HttpId = header.HttpID.Bytes()
		if header.DeltaKey != nil {
			keyStr := *header.DeltaKey
			delta.Key = &apiv1.HttpHeaderDeltaSyncUpdate_KeyUnion{
				Kind:    315301840, // KIND_STRING
				String_: &keyStr,
			}
		}
		if header.DeltaValue != nil {
			valueStr := *header.DeltaValue
			delta.Value = &apiv1.HttpHeaderDeltaSyncUpdate_ValueUnion{
				Kind:    315301840, // KIND_STRING
				String_: &valueStr,
			}
		}
		if header.DeltaEnabled != nil {
			enabledBool := *header.DeltaEnabled
			delta.Enabled = &apiv1.HttpHeaderDeltaSyncUpdate_EnabledUnion{
				Kind: 477045804, // KIND_BOOL
				Bool: &enabledBool,
			}
		}
		if header.DeltaDescription != nil {
			descStr := *header.DeltaDescription
			delta.Description = &apiv1.HttpHeaderDeltaSyncUpdate_DescriptionUnion{
				Kind:    315301840, // KIND_STRING
				String_: &descStr,
			}
		}
		if header.DeltaOrder != nil {
			orderFloat := *header.DeltaOrder
			delta.Order = &apiv1.HttpHeaderDeltaSyncUpdate_OrderUnion{
				Kind:  182966389, // KIND_FLOAT
				Float: &orderFloat,
			}
		}
		value = &apiv1.HttpHeaderDeltaSync_ValueUnion{
			Kind:   apiv1.HttpHeaderDeltaSync_ValueUnion_KIND_UPDATE,
			Update: delta,
		}
	case eventTypeDelete:
		value = &apiv1.HttpHeaderDeltaSync_ValueUnion{
			Kind: apiv1.HttpHeaderDeltaSync_ValueUnion_KIND_DELETE,
			Delete: &apiv1.HttpHeaderDeltaSyncDelete{
				DeltaHttpHeaderId: header.ID.Bytes(),
			},
		}
	}

	return &apiv1.HttpHeaderDeltaSyncResponse{
		Items: []*apiv1.HttpHeaderDeltaSync{
			{
				Value: value,
			},
		},
	}
}

// httpBodyFormDeltaSyncResponseFrom converts HttpBodyFormEvent and form record to HttpBodyFormDeltaSync response
func httpBodyFormDataDeltaSyncResponseFrom(event HttpBodyFormEvent, form mhttpbodyform.HttpBodyForm) *apiv1.HttpBodyFormDataDeltaSyncResponse {
	var value *apiv1.HttpBodyFormDataDeltaSync_ValueUnion

	switch event.Type {
	case eventTypeInsert:
		delta := &apiv1.HttpBodyFormDataDeltaSyncInsert{
			DeltaHttpBodyFormDataId: form.ID.Bytes(),
		}
		if form.ParentHttpBodyFormID != nil {
			delta.HttpBodyFormDataId = form.ParentHttpBodyFormID.Bytes()
		}
		delta.HttpId = form.HttpID.Bytes()
		if form.DeltaKey != nil {
			delta.Key = form.DeltaKey
		}
		if form.DeltaValue != nil {
			delta.Value = form.DeltaValue
		}
		if form.DeltaEnabled != nil {
			delta.Enabled = form.DeltaEnabled
		}
		if form.DeltaDescription != nil {
			delta.Description = form.DeltaDescription
		}
		if form.DeltaOrder != nil {
			delta.Order = form.DeltaOrder
		}
		value = &apiv1.HttpBodyFormDataDeltaSync_ValueUnion{
			Kind:   apiv1.HttpBodyFormDataDeltaSync_ValueUnion_KIND_INSERT,
			Insert: delta,
		}
	case eventTypeUpdate:
		delta := &apiv1.HttpBodyFormDataDeltaSyncUpdate{
			DeltaHttpBodyFormDataId: form.ID.Bytes(),
		}
		if form.ParentHttpBodyFormID != nil {
			delta.HttpBodyFormDataId = form.ParentHttpBodyFormID.Bytes()
		}
		delta.HttpId = form.HttpID.Bytes()
		if form.DeltaKey != nil {
			keyStr := *form.DeltaKey
			delta.Key = &apiv1.HttpBodyFormDataDeltaSyncUpdate_KeyUnion{
				Kind:    315301840, // KIND_STRING
				String_: &keyStr,
			}
		}
		if form.DeltaValue != nil {
			valueStr := *form.DeltaValue
			delta.Value = &apiv1.HttpBodyFormDataDeltaSyncUpdate_ValueUnion{
				Kind:    315301840, // KIND_STRING
				String_: &valueStr,
			}
		}
		if form.DeltaEnabled != nil {
			enabledBool := *form.DeltaEnabled
			delta.Enabled = &apiv1.HttpBodyFormDataDeltaSyncUpdate_EnabledUnion{
				Kind: 477045804, // KIND_BOOL
				Bool: &enabledBool,
			}
		}
		if form.DeltaDescription != nil {
			descStr := *form.DeltaDescription
			delta.Description = &apiv1.HttpBodyFormDataDeltaSyncUpdate_DescriptionUnion{
				Kind:    315301840, // KIND_STRING
				String_: &descStr,
			}
		}
		if form.DeltaOrder != nil {
			orderFloat := *form.DeltaOrder
			delta.Order = &apiv1.HttpBodyFormDataDeltaSyncUpdate_OrderUnion{
				Kind:  182966389, // KIND_FLOAT
				Float: &orderFloat,
			}
		}
		value = &apiv1.HttpBodyFormDataDeltaSync_ValueUnion{
			Kind:   apiv1.HttpBodyFormDataDeltaSync_ValueUnion_KIND_UPDATE,
			Update: delta,
		}
	case eventTypeDelete:
		value = &apiv1.HttpBodyFormDataDeltaSync_ValueUnion{
			Kind: apiv1.HttpBodyFormDataDeltaSync_ValueUnion_KIND_DELETE,
			Delete: &apiv1.HttpBodyFormDataDeltaSyncDelete{
				DeltaHttpBodyFormDataId: form.ID.Bytes(),
			},
		}
	}

	return &apiv1.HttpBodyFormDataDeltaSyncResponse{
		Items: []*apiv1.HttpBodyFormDataDeltaSync{
			{
				Value: value,
			},
		},
	}
}

// httpAssertDeltaSyncResponseFrom converts HttpAssertEvent and assert record to HttpAssertDeltaSync response
func httpAssertDeltaSyncResponseFrom(event HttpAssertEvent, assert mhttpassert.HttpAssert) *apiv1.HttpAssertDeltaSyncResponse {
	var value *apiv1.HttpAssertDeltaSync_ValueUnion

	switch event.Type {
	case eventTypeInsert:
		delta := &apiv1.HttpAssertDeltaSyncInsert{
			DeltaHttpAssertId: assert.ID.Bytes(),
		}
		if assert.ParentHttpAssertID != nil {
			delta.HttpAssertId = assert.ParentHttpAssertID.Bytes()
		}
		delta.HttpId = assert.HttpID.Bytes()
		if assert.DeltaValue != nil {
			delta.Value = assert.DeltaValue
		}
		value = &apiv1.HttpAssertDeltaSync_ValueUnion{
			Kind:   apiv1.HttpAssertDeltaSync_ValueUnion_KIND_INSERT,
			Insert: delta,
		}
	case eventTypeUpdate:
		delta := &apiv1.HttpAssertDeltaSyncUpdate{
			DeltaHttpAssertId: assert.ID.Bytes(),
		}
		if assert.ParentHttpAssertID != nil {
			delta.HttpAssertId = assert.ParentHttpAssertID.Bytes()
		}
		delta.HttpId = assert.HttpID.Bytes()
		if assert.DeltaValue != nil {
			valueStr := *assert.DeltaValue
			delta.Value = &apiv1.HttpAssertDeltaSyncUpdate_ValueUnion{
				Kind:    315301840, // KIND_STRING
				String_: &valueStr,
			}
		}
		value = &apiv1.HttpAssertDeltaSync_ValueUnion{
			Kind:   apiv1.HttpAssertDeltaSync_ValueUnion_KIND_UPDATE,
			Update: delta,
		}
	case eventTypeDelete:
		value = &apiv1.HttpAssertDeltaSync_ValueUnion{
			Kind: apiv1.HttpAssertDeltaSync_ValueUnion_KIND_DELETE,
			Delete: &apiv1.HttpAssertDeltaSyncDelete{
				DeltaHttpAssertId: assert.ID.Bytes(),
			},
		}
	}

	return &apiv1.HttpAssertDeltaSyncResponse{
		Items: []*apiv1.HttpAssertDeltaSync{
			{
				Value: value,
			},
		},
	}
}

// httpBodyUrlEncodedDeltaSyncResponseFrom converts HttpBodyUrlEncodedEvent and body record to HttpBodyUrlEncodedDeltaSync response
func httpBodyUrlEncodedDeltaSyncResponseFrom(event HttpBodyUrlEncodedEvent, body mhttpbodyurlencoded.HttpBodyUrlEncoded) *apiv1.HttpBodyUrlEncodedDeltaSyncResponse {
	var value *apiv1.HttpBodyUrlEncodedDeltaSync_ValueUnion

	switch event.Type {
	case eventTypeInsert:
		delta := &apiv1.HttpBodyUrlEncodedDeltaSyncInsert{
			DeltaHttpBodyUrlEncodedId: body.ID.Bytes(),
		}
		if body.ParentHttpBodyUrlEncodedID != nil {
			delta.HttpBodyUrlEncodedId = body.ParentHttpBodyUrlEncodedID.Bytes()
		}
		delta.HttpId = body.HttpID.Bytes()
		if body.DeltaKey != nil {
			delta.Key = body.DeltaKey
		}
		if body.DeltaValue != nil {
			delta.Value = body.DeltaValue
		}
		if body.DeltaEnabled != nil {
			delta.Enabled = body.DeltaEnabled
		}
		if body.DeltaDescription != nil {
			delta.Description = body.DeltaDescription
		}
		if body.DeltaOrder != nil {
			delta.Order = body.DeltaOrder
		}
		value = &apiv1.HttpBodyUrlEncodedDeltaSync_ValueUnion{
			Kind:   apiv1.HttpBodyUrlEncodedDeltaSync_ValueUnion_KIND_INSERT,
			Insert: delta,
		}
	case eventTypeUpdate:
		delta := &apiv1.HttpBodyUrlEncodedDeltaSyncUpdate{
			DeltaHttpBodyUrlEncodedId: body.ID.Bytes(),
		}
		if body.ParentHttpBodyUrlEncodedID != nil {
			delta.HttpBodyUrlEncodedId = body.ParentHttpBodyUrlEncodedID.Bytes()
		}
		delta.HttpId = body.HttpID.Bytes()
		if body.DeltaKey != nil {
			keyStr := *body.DeltaKey
			delta.Key = &apiv1.HttpBodyUrlEncodedDeltaSyncUpdate_KeyUnion{
				Kind:    315301840, // KIND_STRING
				String_: &keyStr,
			}
		}
		if body.DeltaValue != nil {
			valueStr := *body.DeltaValue
			delta.Value = &apiv1.HttpBodyUrlEncodedDeltaSyncUpdate_ValueUnion{
				Kind:    315301840, // KIND_STRING
				String_: &valueStr,
			}
		}
		if body.DeltaEnabled != nil {
			enabledBool := *body.DeltaEnabled
			delta.Enabled = &apiv1.HttpBodyUrlEncodedDeltaSyncUpdate_EnabledUnion{
				Kind: 477045804, // KIND_BOOL
				Bool: &enabledBool,
			}
		}
		if body.DeltaDescription != nil {
			descStr := *body.DeltaDescription
			delta.Description = &apiv1.HttpBodyUrlEncodedDeltaSyncUpdate_DescriptionUnion{
				Kind:    315301840, // KIND_STRING
				String_: &descStr,
			}
		}
		if body.DeltaOrder != nil {
			orderFloat := *body.DeltaOrder
			delta.Order = &apiv1.HttpBodyUrlEncodedDeltaSyncUpdate_OrderUnion{
				Kind:  182966389, // KIND_FLOAT
				Float: &orderFloat,
			}
		}
		value = &apiv1.HttpBodyUrlEncodedDeltaSync_ValueUnion{
			Kind:   apiv1.HttpBodyUrlEncodedDeltaSync_ValueUnion_KIND_UPDATE,
			Update: delta,
		}
	case eventTypeDelete:
		value = &apiv1.HttpBodyUrlEncodedDeltaSync_ValueUnion{
			Kind: apiv1.HttpBodyUrlEncodedDeltaSync_ValueUnion_KIND_DELETE,
			Delete: &apiv1.HttpBodyUrlEncodedDeltaSyncDelete{
				DeltaHttpBodyUrlEncodedId: body.ID.Bytes(),
			},
		}
	}

	return &apiv1.HttpBodyUrlEncodedDeltaSyncResponse{
		Items: []*apiv1.HttpBodyUrlEncodedDeltaSync{
			{
				Value: value,
			},
		},
	}
}

// streamHttpBodyUrlEncodedDeltaSync streams HTTP body URL encoded delta events to the client
func (h *HttpServiceRPC) streamHttpBodyUrlEncodedDeltaSync(ctx context.Context, userID idwrap.IDWrap, send func(*apiv1.HttpBodyUrlEncodedDeltaSyncResponse) error) error {
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
				if !bodyUrlEncoded.IsDelta {
					continue // Only include delta records
				}
				events = append(events, eventstream.Event[HttpBodyUrlEncodedTopic, HttpBodyUrlEncodedEvent]{
					Topic: HttpBodyUrlEncodedTopic{WorkspaceID: http.WorkspaceID},
					Payload: HttpBodyUrlEncodedEvent{
						Type:               eventTypeInsert,
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
			// Get the full body URL encoded record for delta sync response
			bodyID, err := idwrap.NewFromBytes(evt.Payload.HttpBodyUrlEncoded.GetHttpBodyUrlEncodedId())
			if err != nil {
				continue // Skip if can't parse ID
			}
			bodyRecord, err := h.httpBodyUrlEncodedService.GetHttpBodyUrlEncoded(ctx, bodyID)
			if err != nil {
				continue // Skip if can't get the record
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
						Type:               eventTypeInsert,
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
					// HttpId:                    bodyUrlEncoded.HttpID.Bytes(),
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
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return h.streamHttpBodyUrlEncodedDeltaSync(ctx, userID, stream.Send)
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
			body, err := h.bodyService.GetByHttpID(ctx, http.ID)
			if err != nil && !errors.Is(err, shttp.ErrNoHttpBodyRawFound) {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			if body != nil {
				bodyRaw := &apiv1.HttpBodyRaw{
					// HttpId: http.ID.Bytes(),
					Data:   string(body.RawData), // Convert []byte to string
				}
				allBodies = append(allBodies, bodyRaw)
			}
		}
	}

	return connect.NewResponse(&apiv1.HttpBodyRawCollectionResponse{
		Items: allBodies,
	}), nil
}

func (h *HttpServiceRPC) HttpBodyRawInsert(ctx context.Context, req *connect.Request[apiv1.HttpBodyRawInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP body raw must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var insertData []struct {
		httpID      idwrap.IDWrap
		data        []byte
		contentType string
	}

	for _, item := range req.Msg.Items {
		if len(item.HttpId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_id is required"))
		}

		httpID, err := idwrap.NewFromBytes(item.HttpId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Verify the HTTP entry exists and user has access - use pool service
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

		// Determine content type based on content
		contentType := "text/plain"
		if json.Valid([]byte(item.Data)) {
			contentType = "application/json"
		}

		insertData = append(insertData, struct {
			httpID      idwrap.IDWrap
			data        []byte
			contentType string
		}{
			httpID:      httpID,
			data:        []byte(item.Data),
			contentType: contentType,
		})
	}

	// Step 2: Execute inserts in transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	bodyRawService := h.bodyService.TX(tx)

	for _, data := range insertData {
		// Create the body raw using the new service
		_, err = bodyRawService.Create(ctx, data.httpID, data.data, data.contentType)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpBodyRawUpdate(ctx context.Context, req *connect.Request[apiv1.HttpBodyRawUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP body raw must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var updateData []struct {
		existingBodyID idwrap.IDWrap
		data           []byte
		contentType    string
	}

	for _, item := range req.Msg.Items {
		if len(item.HttpId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_id is required"))
		}

		httpID, err := idwrap.NewFromBytes(item.HttpId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Verify the HTTP entry exists and user has access - use pool service
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

		// Get existing body raw to get its ID - use pool service
		existingBodyRaw, err := h.bodyService.GetByHttpID(ctx, httpID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHttpBodyRawFound) {
				return nil, connect.NewError(connect.CodeNotFound, errors.New("raw body not found for this HTTP entry"))
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Prepare update data if provided
		if item.Data != nil {
			// Determine content type based on new content
			contentType := "text/plain"
			if json.Valid([]byte(*item.Data)) {
				contentType = "application/json"
			}

			updateData = append(updateData, struct {
				existingBodyID idwrap.IDWrap
				data           []byte
				contentType    string
			}{
				existingBodyID: existingBodyRaw.ID,
				data:           []byte(*item.Data),
				contentType:    contentType,
			})
		}
	}

	// Step 2: Execute updates in transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	bodyRawService := h.bodyService.TX(tx)

	for _, data := range updateData {
		// Update using the new service
		_, err := bodyRawService.Update(ctx, data.existingBodyID, data.data, data.contentType)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
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

	// Snapshot provider for initial state
	snapshot := func(ctx context.Context) ([]eventstream.Event[HttpBodyRawTopic, HttpBodyRawEvent], error) {
		// Get all HTTP entries for user
		httpList, err := h.listUserHttp(ctx)
		if err != nil {
			return nil, err
		}

		events := make([]eventstream.Event[HttpBodyRawTopic, HttpBodyRawEvent], 0)
		for _, http := range httpList {
			workspaceSet.Store(http.WorkspaceID.String(), struct{}{})

			// Get body raw for this HTTP entry
			rows, err := h.DB.QueryContext(ctx, `
				SELECT http_id, data
				FROM http_body_raw
				WHERE http_id = ?
			`, http.ID.Bytes())
			if err != nil {
				return nil, err
			}

			for rows.Next() {
				var httpID []byte
				var data string
				err := rows.Scan(
					&httpID,
					&data,
				)
				if err != nil {
					rows.Close()
					return nil, err
				}

				events = append(events, eventstream.Event[HttpBodyRawTopic, HttpBodyRawEvent]{
					Topic: HttpBodyRawTopic{WorkspaceID: http.WorkspaceID},
					Payload: HttpBodyRawEvent{
						Type:        eventTypeInsert,
						HttpBodyRaw: toAPIHttpBodyRaw(httpID, data),
					},
				})
			}

			if err := rows.Err(); err != nil {
				rows.Close()
				return nil, err
			}
			rows.Close()
		}
		return events, nil
	}

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

	// Subscribe to events with snapshot
	events, err := h.httpBodyRawStream.Subscribe(ctx, filter, eventstream.WithSnapshot(snapshot))
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
			resp := httpBodyRawSyncResponseFrom(evt.Payload)
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



// storeHttpResponse handles HTTP response storage and publishes sync events
func (h *HttpServiceRPC) storeHttpResponse(ctx context.Context, httpEntry *mhttp.HTTP, resp httpclient.Response, requestTime time.Time, duration int64) (idwrap.IDWrap, error) {
	responseID := idwrap.NewNow()
	nowUnix := time.Now().Unix()

	httpResponse := dbmodels.HttpResponse{
		ID:        responseID,
		HttpID:    httpEntry.ID,
		Status:    int32(resp.StatusCode),
		Body:      resp.Body,
		Time:      requestTime,
		Duration:  int32(duration),
		Size:      int32(len(resp.Body)),
		CreatedAt: nowUnix,
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return idwrap.IDWrap{}, err
	}
	committed := false
	defer func() {
		if !committed {
			if rbErr := tx.Rollback(); rbErr != nil {
				log.Printf("Failed to rollback http response transaction: %v", rbErr)
			}
		}
	}()

	if err := h.httpResponseService.TX(tx).Create(ctx, httpResponse); err != nil {
		return idwrap.IDWrap{}, err
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO http_response_header (id, response_id, key, value, created_at)
		VALUES (?, ?, ?, ?, ?)
	`)
	if err != nil {
		return idwrap.IDWrap{}, err
	}
	defer stmt.Close()

	headerEvents := make([]HttpResponseHeaderEvent, 0, len(resp.Headers))
	for _, header := range resp.Headers {
		if header.HeaderKey == "" {
			continue
		}
		headerID := idwrap.NewNow()
		if _, err := stmt.ExecContext(ctx,
			headerID.Bytes(),
			responseID.Bytes(),
			header.HeaderKey,
			header.Value,
			nowUnix,
		); err != nil {
			return idwrap.IDWrap{}, err
		}
		headerEvents = append(headerEvents, HttpResponseHeaderEvent{
			Type: eventTypeInsert,
			HttpResponseHeader: toAPIHttpResponseHeader(dbmodels.HttpResponseHeader{
				ID:         headerID,
				ResponseID: responseID,
				Key:        header.HeaderKey,
				Value:      header.Value,
				CreatedAt:  nowUnix,
			}),
		})
	}

	if err := tx.Commit(); err != nil {
		return idwrap.IDWrap{}, err
	}
	committed = true

	topic := HttpResponseTopic{WorkspaceID: httpEntry.WorkspaceID}
	h.httpResponseStream.Publish(topic, HttpResponseEvent{
		Type:         eventTypeInsert,
		HttpResponse: toAPIHttpResponse(httpResponse),
	})

	headerTopic := HttpResponseHeaderTopic{WorkspaceID: httpEntry.WorkspaceID}
	for _, evt := range headerEvents {
		h.httpResponseHeaderStream.Publish(headerTopic, evt)
	}

	return responseID, nil
}

// evaluateAndStoreAssertions loads assertions for an HTTP entry, evaluates them against the response, and stores the results
// AssertionResult represents the result of an assertion evaluation
type AssertionResult struct {
	AssertionID idwrap.IDWrap
	Expression  string
	Success     bool
	Error       error
	EvaluatedAt time.Time
}

func (h *HttpServiceRPC) evaluateAndStoreAssertions(ctx context.Context, httpID idwrap.IDWrap, responseID idwrap.IDWrap, resp httpclient.Response) error {
	// Load assertions for this HTTP entry
	asserts, err := h.httpAssertService.GetHttpAssertsByHttpID(ctx, httpID)
	if err != nil {
		return fmt.Errorf("failed to load assertions for HTTP %s: %w", httpID.String(), err)
	}

	if len(asserts) == 0 {
		// No assertions to evaluate
		return nil
	}

	// Filter enabled assertions and log statistics
	enabledAsserts := make([]mhttpassert.HttpAssert, 0, len(asserts))
	for _, assert := range asserts {
		if assert.Enabled {
			enabledAsserts = append(enabledAsserts, assert)
		}
	}

	if len(enabledAsserts) == 0 {
		// No enabled assertions to evaluate
		return nil
	}

	// Create evaluation context with response data (shared across all assertions)
	evalContext := h.createAssertionEvalContext(resp)

	// Evaluate assertions in parallel and collect results
	results := h.evaluateAssertionsParallel(ctx, enabledAsserts, evalContext)

	// Store assertion results in batch with enhanced error handling
	if err := h.storeAssertionResultsBatch(ctx, httpID, responseID, results); err != nil {
		return fmt.Errorf("failed to store assertion results for HTTP %s: %w", httpID.String(), err)
	}

	return nil
}

// evaluateAssertionsParallel evaluates multiple assertions in parallel with timeout and error handling
func (h *HttpServiceRPC) evaluateAssertionsParallel(ctx context.Context, asserts []mhttpassert.HttpAssert, evalContext map[string]any) []AssertionResult {
	results := make([]AssertionResult, len(asserts))
	resultChan := make(chan AssertionResult, len(asserts))

	// Use a WaitGroup to wait for all goroutines to complete
	var wg sync.WaitGroup

	// Create a context with timeout for assertion evaluation (30 seconds per assertion batch)
	evalCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Evaluate each assertion in a separate goroutine
	for i, assert := range asserts {
		wg.Add(1)
		go func(idx int, assertion mhttpassert.HttpAssert) {
			defer wg.Done()
			startTime := time.Now()
			result := AssertionResult{
				AssertionID: assertion.ID,
				EvaluatedAt: startTime,
			}

			// Recover from panics in assertion evaluation
			defer func() {
				if r := recover(); r != nil {
					result.Error = fmt.Errorf("panic during assertion evaluation: %v", r)
					result.Success = false
					resultChan <- result
				}
			}()

			// Construct the expression to evaluate
			expression := assertion.Value
			if assertion.Key != "" {
				// If Key is provided, construct expression based on key type
				expression = h.constructAssertionExpression(assertion.Key, assertion.Value)
			}
			result.Expression = expression

			// Evaluate the assertion expression with context
			success, err := h.evaluateAssertion(evalCtx, expression, evalContext)
			if err != nil {
				// Check for context timeout
				if evalCtx.Err() == context.DeadlineExceeded {
					result.Error = fmt.Errorf("assertion evaluation timed out: %w", err)
				} else {
					result.Error = fmt.Errorf("evaluation failed: %w", err)
				}
				result.Success = false
			} else {
				result.Success = success
			}

			// Add evaluation duration for monitoring
			duration := time.Since(startTime)
			if duration > 5*time.Second {
				log.Printf("Slow assertion evaluation for %s: took %v", assertion.ID.String(), duration)
			}

			resultChan <- result
		}(i, assert)
	}

	// Close the result channel when all goroutines complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results preserving order with timeout
	collectCtx, collectCancel := context.WithTimeout(ctx, 35*time.Second)
	defer collectCancel()

	collectedCount := 0
	for {
		select {
		case result, ok := <-resultChan:
			if !ok {
				// Channel closed, all results collected
				goto done
			}
			// Find the original index for this result
			for j, assert := range asserts {
				if assert.ID == result.AssertionID {
					results[j] = result
					collectedCount++
					break
				}
			}

		case <-collectCtx.Done():
			// Collection timeout - fill missing results with timeout error
			log.Printf("Assertion result collection timed out after 35 seconds")
			for j, assert := range asserts {
				if results[j].AssertionID.String() == "" {
					results[j] = AssertionResult{
						AssertionID: assert.ID,
						Expression:  assert.Value,
						Success:     false,
						Error:       fmt.Errorf("collection timeout"),
						EvaluatedAt: time.Now(),
					}
				}
			}
			goto done

		case <-evalCtx.Done():
			// Evaluation context cancelled
			log.Printf("Assertion evaluation context cancelled: %v", evalCtx.Err())
			for j, assert := range asserts {
				if results[j].AssertionID.String() == "" {
					results[j] = AssertionResult{
						AssertionID: assert.ID,
						Expression:  assert.Value,
						Success:     false,
						Error:       fmt.Errorf("evaluation cancelled: %w", evalCtx.Err()),
						EvaluatedAt: time.Now(),
					}
				}
			}
			goto done
		}
	}

done:
	if collectedCount != len(asserts) {
		log.Printf("Only collected %d out of %d assertion results", collectedCount, len(asserts))
	}

	return results
}

// storeAssertionResultsBatch stores multiple assertion results in a single database transaction
func (h *HttpServiceRPC) storeAssertionResultsBatch(ctx context.Context, httpID idwrap.IDWrap, responseID idwrap.IDWrap, results []AssertionResult) error {
	if len(results) == 0 {
		return nil
	}

	// Start transaction for batch insertion
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			// Rollback on error
			if rbErr := tx.Rollback(); rbErr != nil {
				log.Printf("Failed to rollback transaction: %v", rbErr)
			}
		}
	}()

	// Prepare batch insert statement matching existing database schema
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO http_response_assert (id, response_id, value, success, created_at)
		VALUES (?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	// Insert all results in batch
	now := time.Now().Unix()
	var events []HttpResponseAssertEvent

	for _, result := range results {
		var value string
		var success bool

		if result.Error != nil {
			// Store error information in the value field
			value = fmt.Sprintf("ERROR: %s", result.Error.Error())
			success = false
		} else {
			// Store successful assertion result
			value = result.Expression
			success = result.Success
		}

		assertID := idwrap.NewNow()
		_, err := stmt.ExecContext(ctx,
			assertID.Bytes(),
			responseID.Bytes(),
			value,
			success,
			now,
		)
		if err != nil {
			return fmt.Errorf("failed to insert assertion result for %s: %w", result.AssertionID.String(), err)
		}

		events = append(events, HttpResponseAssertEvent{
			Type: eventTypeInsert,
			HttpResponseAssert: toAPIHttpResponseAssert(dbmodels.HttpResponseAssert{
				ID:         assertID.Bytes(),
				ResponseID: responseID.Bytes(),
				Value:      value,
				Success:    success,
				CreatedAt:  now,
			}),
		})
	}

	log.Printf("Stored %d assertion results for HTTP %s (response %s)",
		len(results), httpID.String(), responseID.String())

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Publish events
	workspaceID, err := h.hs.GetWorkspaceID(ctx, httpID)
	if err == nil {
		topic := HttpResponseAssertTopic{WorkspaceID: workspaceID}
		for _, evt := range events {
			h.httpResponseAssertStream.Publish(topic, evt)
		}
	} else {
		log.Printf("Failed to get workspace ID for publishing assertion events: %v", err)
	}

	return nil
}

// createAssertionEvalContext creates the evaluation context with response data and dynamic variables
func (h *HttpServiceRPC) createAssertionEvalContext(resp httpclient.Response) map[string]any {
	// Parse response body as JSON if possible, providing multiple formats
	var body any
	var bodyMap map[string]any
	bodyString := string(resp.Body)

	if json.Valid(resp.Body) {
		if err := json.Unmarshal(resp.Body, &body); err != nil {
			// If JSON parsing fails, use as string
			body = bodyString
		} else {
			// Also try to parse as map for easier access
			if mapBody, ok := body.(map[string]any); ok {
				bodyMap = mapBody
			}
		}
	} else {
		body = bodyString
	}

	// Convert headers to map with both original and lowercase keys
	headers := make(map[string]string)
	headersLower := make(map[string]string)
	contentType := ""
	contentLength := "0"

	for _, header := range resp.Headers {
		lowerKey := strings.ToLower(header.HeaderKey)
		headers[header.HeaderKey] = header.Value
		headersLower[lowerKey] = header.Value

		// Extract commonly used headers
		switch lowerKey {
		case "content-type":
			contentType = header.Value
		case "content-length":
			contentLength = header.Value
		}
	}

	// Extract JSON path helpers
	jsonPathHelpers := h.createJSONPathHelpers(bodyMap)

	// Create comprehensive evaluation context
	context := map[string]any{
		// Main response object
		"response": map[string]any{
			"status":         resp.StatusCode,
			"status_text":    h.getStatusText(resp.StatusCode),
			"body":           body,
			"body_string":    bodyString,
			"body_size":      len(resp.Body),
			"headers":        headers,
			"headers_lower":  headersLower,
			"content_type":   contentType,
			"content_length": contentLength,
		},

		// Direct access variables
		"status":         resp.StatusCode,
		"status_code":    resp.StatusCode,
		"status_text":    h.getStatusText(resp.StatusCode),
		"body":           body,
		"body_string":    bodyString,
		"body_size":      len(resp.Body),
		"headers":        headers,
		"headers_lower":  headersLower,
		"content_type":   contentType,
		"content_length": contentLength,

		// Convenience variables
		"success":      resp.StatusCode >= 200 && resp.StatusCode < 300,
		"client_error": resp.StatusCode >= 400 && resp.StatusCode < 500,
		"server_error": resp.StatusCode >= 500 && resp.StatusCode < 600,
		"is_json":      strings.HasPrefix(contentType, "application/json"),
		"is_html":      strings.HasPrefix(contentType, "text/html"),
		"is_text":      strings.HasPrefix(contentType, "text/"),
		"has_body":     len(resp.Body) > 0,

		// JSON path helpers
		"json": jsonPathHelpers,
	}

	return context
}

// createJSONPathHelpers creates helper functions for JSON path navigation
func (h *HttpServiceRPC) createJSONPathHelpers(bodyMap map[string]any) map[string]any {
	helpers := make(map[string]any)

	if bodyMap == nil {
		return helpers
	}

	// Helper function to get nested value by path
	getPath := func(path string) any {
		parts := strings.Split(path, ".")
		current := bodyMap

		for _, part := range parts {
			if next, ok := current[part]; ok {
				if nextMap, ok := next.(map[string]any); ok {
					current = nextMap
				} else {
					return next
				}
			} else {
				return nil
			}
		}

		return current
	}

	// Helper function to check if path exists
	hasPath := func(path string) bool {
		return getPath(path) != nil
	}

	// Helper function to get string value
	getString := func(path string) string {
		if val := getPath(path); val != nil {
			if str, ok := val.(string); ok {
				return str
			}
			return fmt.Sprintf("%v", val)
		}
		return ""
	}

	// Helper function to get numeric value
	getNumber := func(path string) float64 {
		if val := getPath(path); val != nil {
			if num, ok := val.(float64); ok {
				return num
			}
			if num, ok := val.(int); ok {
				return float64(num)
			}
		}
		return 0
	}

	helpers["path"] = getPath
	helpers["has"] = hasPath
	helpers["string"] = getString
	helpers["number"] = getNumber

	return helpers
}

// getStatusText returns the standard HTTP status text for a status code
func (h *HttpServiceRPC) getStatusText(statusCode int) string {
	switch {
	case statusCode >= 100 && statusCode < 200:
		switch statusCode {
		case 100:
			return "Continue"
		case 101:
			return "Switching Protocols"
		case 102:
			return "Processing"
		case 103:
			return "Early Hints"
		default:
			return "Informational"
		}
	case statusCode >= 200 && statusCode < 300:
		switch statusCode {
		case 200:
			return "OK"
		case 201:
			return "Created"
		case 202:
			return "Accepted"
		case 204:
			return "No Content"
		case 206:
			return "Partial Content"
		default:
			return "Success"
		}
	case statusCode >= 300 && statusCode < 400:
		switch statusCode {
		case 300:
			return "Multiple Choices"
		case 301:
			return "Moved Permanently"
		case 302:
			return "Found"
		case 304:
			return "Not Modified"
		case 307:
			return "Temporary Redirect"
		case 308:
			return "Permanent Redirect"
		default:
			return "Redirection"
		}
	case statusCode >= 400 && statusCode < 500:
		switch statusCode {
		case 400:
			return "Bad Request"
		case 401:
			return "Unauthorized"
		case 403:
			return "Forbidden"
		case 404:
			return "Not Found"
		case 405:
			return "Method Not Allowed"
		case 408:
			return "Request Timeout"
		case 409:
			return "Conflict"
		case 422:
			return "Unprocessable Entity"
		case 429:
			return "Too Many Requests"
		default:
			return "Client Error"
		}
	case statusCode >= 500 && statusCode < 600:
		switch statusCode {
		case 500:
			return "Internal Server Error"
		case 501:
			return "Not Implemented"
		case 502:
			return "Bad Gateway"
		case 503:
			return "Service Unavailable"
		case 504:
			return "Gateway Timeout"
		default:
			return "Server Error"
		}
	default:
		return "Unknown"
	}
}

// constructAssertionExpression constructs an expression from key and value
func (h *HttpServiceRPC) constructAssertionExpression(key, value string) string {
	switch key {
	case "status_code":
		return fmt.Sprintf("response.status == %s", value)
	case "response_time":
		return fmt.Sprintf("response_time %s", value)
	case "content_type":
		return fmt.Sprintf("response.headers['content-type'] == '%s'", value)
	case "body":
		return fmt.Sprintf("response.body == %s", value)
	default:
		// For unknown keys, assume it's a direct expression
		return value
	}
}

// evaluateAssertion evaluates an assertion expression against the provided context
func (h *HttpServiceRPC) evaluateAssertion(ctx context.Context, expressionStr string, context map[string]any) (bool, error) {
	env := expression.NewEnv(context)
	return expression.ExpressionEvaluteAsBool(ctx, env, expressionStr)
}

// storeAssertionResult stores the result of an assertion evaluation
func (h *HttpServiceRPC) storeAssertionResult(ctx context.Context, httpID idwrap.IDWrap, assertionValue string, success bool) error {
	_, err := h.DB.ExecContext(ctx, `
		INSERT INTO http_response_assert (id, http_id, value, success, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, idwrap.NewNow().Bytes(), httpID.Bytes(), assertionValue, success, time.Now().Unix())

	if err != nil {
		return fmt.Errorf("failed to insert assertion result: %w", err)
	}

	return nil
}
