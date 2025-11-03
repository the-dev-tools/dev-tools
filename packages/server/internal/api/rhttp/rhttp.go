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
	"the-dev-tools/server/pkg/model/mworkspaceuser"
	"the-dev-tools/server/pkg/service/sbodyraw"
	"the-dev-tools/server/pkg/service/sexampleheader"
	"the-dev-tools/server/pkg/service/sexamplequery"
	"the-dev-tools/server/pkg/service/sexampleresp"
	"the-dev-tools/server/pkg/service/shttp"
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

	stream eventstream.SyncStreamer[HttpTopic, HttpEvent]
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
	stream eventstream.SyncStreamer[HttpTopic, HttpEvent],
) HttpServiceRPC {
	return HttpServiceRPC{
		DB:            db,
		hs:            hs,
		us:            us,
		ws:            ws,
		wus:           wus,
		headerService: headerService,
		queryService:  queryService,
		bodyService:   bodyService,
		respService:   respService,
		stream:        stream,
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
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpSearchParamCreate(ctx context.Context, req *connect.Request[apiv1.HttpSearchParamCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpSearchParamUpdate(ctx context.Context, req *connect.Request[apiv1.HttpSearchParamUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpSearchParamDelete(ctx context.Context, req *connect.Request[apiv1.HttpSearchParamDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpSearchParamSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpSearchParamSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpSearchParamDeltaCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpSearchParamDeltaCollectionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpSearchParamDeltaCreate(ctx context.Context, req *connect.Request[apiv1.HttpSearchParamDeltaCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpSearchParamDeltaUpdate(ctx context.Context, req *connect.Request[apiv1.HttpSearchParamDeltaUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpSearchParamDeltaDelete(ctx context.Context, req *connect.Request[apiv1.HttpSearchParamDeltaDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpSearchParamDeltaSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpSearchParamDeltaSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpAssertCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpAssertCollectionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpAssertCreate(ctx context.Context, req *connect.Request[apiv1.HttpAssertCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpAssertUpdate(ctx context.Context, req *connect.Request[apiv1.HttpAssertUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpAssertDelete(ctx context.Context, req *connect.Request[apiv1.HttpAssertDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpAssertSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpAssertSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpAssertDeltaCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpAssertDeltaCollectionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpAssertDeltaCreate(ctx context.Context, req *connect.Request[apiv1.HttpAssertDeltaCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpAssertDeltaUpdate(ctx context.Context, req *connect.Request[apiv1.HttpAssertDeltaUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpAssertDeltaDelete(ctx context.Context, req *connect.Request[apiv1.HttpAssertDeltaDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpAssertDeltaSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpAssertDeltaSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
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
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpHeaderCreate(ctx context.Context, req *connect.Request[apiv1.HttpHeaderCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpHeaderUpdate(ctx context.Context, req *connect.Request[apiv1.HttpHeaderUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpHeaderDelete(ctx context.Context, req *connect.Request[apiv1.HttpHeaderDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpHeaderSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpHeaderSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpHeaderDeltaCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpHeaderDeltaCollectionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpHeaderDeltaCreate(ctx context.Context, req *connect.Request[apiv1.HttpHeaderDeltaCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpHeaderDeltaUpdate(ctx context.Context, req *connect.Request[apiv1.HttpHeaderDeltaUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpHeaderDeltaDelete(ctx context.Context, req *connect.Request[apiv1.HttpHeaderDeltaDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpHeaderDeltaSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpHeaderDeltaSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpBodyFormCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpBodyFormCollectionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpBodyFormCreate(ctx context.Context, req *connect.Request[apiv1.HttpBodyFormCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpBodyFormUpdate(ctx context.Context, req *connect.Request[apiv1.HttpBodyFormUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpBodyFormDelete(ctx context.Context, req *connect.Request[apiv1.HttpBodyFormDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpBodyFormSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpBodyFormSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpBodyFormDeltaCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpBodyFormDeltaCollectionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpBodyFormDeltaCreate(ctx context.Context, req *connect.Request[apiv1.HttpBodyFormDeltaCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpBodyFormDeltaUpdate(ctx context.Context, req *connect.Request[apiv1.HttpBodyFormDeltaUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpBodyFormDeltaDelete(ctx context.Context, req *connect.Request[apiv1.HttpBodyFormDeltaDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpBodyFormDeltaSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpBodyFormDeltaSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpBodyUrlEncodedCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpBodyUrlEncodedCollectionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpBodyUrlEncodedCreate(ctx context.Context, req *connect.Request[apiv1.HttpBodyUrlEncodedCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpBodyUrlEncodedUpdate(ctx context.Context, req *connect.Request[apiv1.HttpBodyUrlEncodedUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpBodyUrlEncodedDelete(ctx context.Context, req *connect.Request[apiv1.HttpBodyUrlEncodedDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpBodyUrlEncodedSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpBodyUrlEncodedSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpBodyUrlEncodedDeltaCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpBodyUrlEncodedDeltaCollectionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpBodyUrlEncodedDeltaCreate(ctx context.Context, req *connect.Request[apiv1.HttpBodyUrlEncodedDeltaCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpBodyUrlEncodedDeltaUpdate(ctx context.Context, req *connect.Request[apiv1.HttpBodyUrlEncodedDeltaUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpBodyUrlEncodedDeltaDelete(ctx context.Context, req *connect.Request[apiv1.HttpBodyUrlEncodedDeltaDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *HttpServiceRPC) HttpBodyUrlEncodedDeltaSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.HttpBodyUrlEncodedDeltaSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
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
