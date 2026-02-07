//nolint:revive // exported
package rhttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"

	devtoolsdb "github.com/the-dev-tools/dev-tools/packages/db"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rlog"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/converter"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/expression"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/http/request"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/httpclient"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/patch"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/menv"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/senv"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/shttp"
	apiv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/http/v1"
	logv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/log/v1"
)

func (h *HttpServiceRPC) executeHTTPRequest(ctx context.Context, httpEntry *mhttp.HTTP) error {
	var resolvedHTTP mhttp.HTTP
	var mHeaders []mhttp.HTTPHeader
	var mQueries []mhttp.HTTPSearchParam
	var rawBody *mhttp.HTTPBodyRaw
	var mFormBody []mhttp.HTTPBodyForm
	var mUrlEncodedBody []mhttp.HTTPBodyUrlencoded

	// Check if this is a delta request and resolve it using the resolver
	if httpEntry.IsDelta && httpEntry.ParentHttpID != nil {
		// Use the resolver to merge base + delta
		resolved, err := h.resolver.Resolve(ctx, *httpEntry.ParentHttpID, &httpEntry.ID)
		if err != nil {
			return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to resolve delta request: %w", err))
		}

		resolvedHTTP = resolved.Resolved
		mHeaders = resolved.ResolvedHeaders
		mQueries = resolved.ResolvedQueries
		mFormBody = resolved.ResolvedFormBody
		mUrlEncodedBody = resolved.ResolvedUrlEncodedBody
		rawBody = &resolved.ResolvedRawBody

		// Use workspace ID from original entry (base might have different workspace)
		resolvedHTTP.WorkspaceID = httpEntry.WorkspaceID
	} else {
		// Non-delta request: load components directly
		resolvedHTTP = *httpEntry

		headers, err := h.httpHeaderService.GetByHttpIDOrdered(ctx, httpEntry.ID)
		if err != nil {
			headers = []mhttp.HTTPHeader{}
		}

		queries, err := h.httpSearchParamService.GetByHttpIDOrdered(ctx, httpEntry.ID)
		if err != nil {
			queries = []mhttp.HTTPSearchParam{}
		}

		rawBodyFetched, err := h.bodyService.GetByHttpID(ctx, httpEntry.ID)
		if err != nil && !errors.Is(err, shttp.ErrNoHttpBodyRawFound) {
			rawBodyFetched = nil
		}
		rawBody = rawBodyFetched

		formBody, err := h.httpBodyFormService.GetByHttpID(ctx, httpEntry.ID)
		if err != nil {
			formBody = []mhttp.HTTPBodyForm{}
		}

		urlEncodedBody, err := h.httpBodyUrlEncodedService.GetByHttpID(ctx, httpEntry.ID)
		if err != nil {
			urlEncodedBody = []mhttp.HTTPBodyUrlencoded{}
		}

		// Convert to mhttp types for request preparation
		mHeaders = make([]mhttp.HTTPHeader, len(headers))
		for i, v := range headers {
			mHeaders[i] = mhttp.HTTPHeader{
				Key:     v.Key,
				Value:   v.Value,
				Enabled: v.Enabled,
			}
		}

		mQueries = make([]mhttp.HTTPSearchParam, len(queries))
		for i, v := range queries {
			mQueries[i] = mhttp.HTTPSearchParam{
				Key:     v.Key,
				Value:   v.Value,
				Enabled: v.Enabled,
			}
		}

		mFormBody = make([]mhttp.HTTPBodyForm, len(formBody))
		for i, v := range formBody {
			mFormBody[i] = mhttp.HTTPBodyForm{
				Key:     v.Key,
				Value:   v.Value,
				Enabled: v.Enabled,
			}
		}

		mUrlEncodedBody = make([]mhttp.HTTPBodyUrlencoded, len(urlEncodedBody))
		for i, v := range urlEncodedBody {
			mUrlEncodedBody[i] = mhttp.HTTPBodyUrlencoded{
				Key:     v.Key,
				Value:   v.Value,
				Enabled: v.Enabled,
			}
		}
	}

	// Build variable context from previous HTTP responses in the workspace
	varMap, err := h.buildWorkspaceVarMap(ctx, httpEntry.WorkspaceID)
	if err != nil {
		// Continue with empty varMap rather than failing
		varMap = make(map[string]any)
	}

	// Prepare the HTTP request using request package
	res, err := request.PrepareHTTPRequestWithTracking(
		resolvedHTTP,
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
		var netErr net.Error
		if errors.As(err, &netErr) {
			if netErr.Timeout() {
				return connect.NewError(connect.CodeDeadlineExceeded, fmt.Errorf("request timeout: %w", err))
			}
			// Note: Temporary() is deprecated since Go 1.18 - treating temporary network errors as unavailable without checking
			return connect.NewError(connect.CodeUnavailable, fmt.Errorf("network error: %w", err))
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
		slog.WarnContext(ctx, "Failed to evaluate assertions",
			"http_id", httpEntry.ID.String(),
			"response_id", responseID.String(),
			"error", err)
	}

	return nil
}

// buildWorkspaceVarMap creates a variable map from workspace environments.
// Environment variables are stored as flat keys for direct access.
// Access via {{ apiKey }} or {{ varName }}.
func (h *HttpServiceRPC) buildWorkspaceVarMap(ctx context.Context, workspaceID idwrap.IDWrap) (map[string]any, error) {
	// Get workspace to find global environment
	workspace, err := h.ws.Get(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workspace: %w", err)
	}

	// Get global environment variables
	var globalVars []menv.Variable
	if workspace.GlobalEnv != (idwrap.IDWrap{}) {
		globalVars, err = h.vs.GetVariableByEnvID(ctx, workspace.GlobalEnv)
		if err != nil && !errors.Is(err, senv.ErrNoVarFound) {
			return nil, fmt.Errorf("failed to get global environment variables: %w", err)
		}
	}

	// Create environment variables map
	envVars := make(map[string]any)
	for _, envVar := range globalVars {
		if envVar.IsEnabled() {
			envVars[envVar.VarKey] = envVar.Value
		}
	}

	// Spread env vars directly into varMap
	varMap := make(map[string]any)
	for k, v := range envVars {
		varMap[k] = v
	}

	return varMap, nil
}

// extractResponseVariables logic was removed as variable storage is handled by rflow
// and rhttp is stateless regarding variable persistence from responses.

func (h *HttpServiceRPC) HttpRun(ctx context.Context, req *connect.Request[apiv1.HttpRunRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.HttpId) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_id is required"))
	}

	httpID, err := idwrap.NewFromBytes(req.Msg.HttpId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Get HTTP entry to check workspace permissions
	httpEntry, err := h.httpReader.Get(ctx, httpID)
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

	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Execute HTTP request with proper error handling
	if err := h.executeHTTPRequest(ctx, httpEntry); err != nil {
		h.logExecution(userID, httpEntry, err)

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
	defer devtoolsdb.TxnRollback(tx)

	hsWriter := shttp.NewWriter(tx)

	if err := hsWriter.Update(ctx, httpEntry); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update LastRunAt: %w", err))
	}

	// Create a new version for this run
	versionName := fmt.Sprintf("v%d", time.Now().UnixNano())
	versionDesc := "Auto-saved version (Run)"

	version, err := hsWriter.CreateHttpVersion(ctx, httpEntry.ID, userID, versionName, versionDesc)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create version on run: %w", err))
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to commit transaction: %w", err))
	}

	h.publishUpdateEvent(*httpEntry, patch.HTTPDeltaPatch{})
	h.publishVersionInsertEvent(*version, httpEntry.WorkspaceID)
	h.logExecution(userID, httpEntry, nil)

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// storeHttpResponse handles HTTP response storage and publishes sync events
func (h *HttpServiceRPC) storeHttpResponse(ctx context.Context, httpEntry *mhttp.HTTP, resp httpclient.Response, requestTime time.Time, duration int64) (idwrap.IDWrap, error) {
	responseID := idwrap.NewNow()
	nowUnix := time.Now().Unix()

	httpResponse := mhttp.HTTPResponse{
		ID:        responseID,
		HttpID:    httpEntry.ID,
		Status:    int32(resp.StatusCode), // nolint:gosec // G115
		Body:      resp.Body,
		Time:      requestTime.Unix(),
		Duration:  int32(duration),       // nolint:gosec // G115
		Size:      int32(len(resp.Body)), // nolint:gosec // G115
		CreatedAt: nowUnix,
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return idwrap.IDWrap{}, err
	}
	defer devtoolsdb.TxnRollback(tx)

	responseWriter := shttp.NewHttpResponseWriter(tx)

	if err := responseWriter.Create(ctx, httpResponse); err != nil {
		return idwrap.IDWrap{}, err
	}

	headerEvents := make([]HttpResponseHeaderEvent, 0, len(resp.Headers))
	for _, header := range resp.Headers {
		if header.HeaderKey == "" {
			continue
		}
		headerID := idwrap.NewNow()
		responseHeader := mhttp.HTTPResponseHeader{
			ID:          headerID,
			ResponseID:  responseID,
			HeaderKey:   header.HeaderKey,
			HeaderValue: header.Value,
			CreatedAt:   nowUnix,
		}

		if err := responseWriter.CreateHeader(ctx, responseHeader); err != nil {
			return idwrap.IDWrap{}, err
		}
		headerEvents = append(headerEvents, HttpResponseHeaderEvent{
			Type:               eventTypeInsert,
			HttpResponseHeader: converter.ToAPIHttpResponseHeader(responseHeader),
		})
	}

	if err := tx.Commit(); err != nil {
		return idwrap.IDWrap{}, err
	}

	if h.streamers.HttpResponse != nil {
		topic := HttpResponseTopic{WorkspaceID: httpEntry.WorkspaceID}
		h.streamers.HttpResponse.Publish(topic, HttpResponseEvent{
			Type:         eventTypeInsert,
			HttpResponse: converter.ToAPIHttpResponse(httpResponse),
		})
	}

	if h.streamers.HttpResponseHeader != nil {
		headerTopic := HttpResponseHeaderTopic{WorkspaceID: httpEntry.WorkspaceID}
		for _, evt := range headerEvents {
			h.streamers.HttpResponseHeader.Publish(headerTopic, evt)
		}
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
	asserts, err := h.httpAssertService.GetByHttpID(ctx, httpID)
	if err != nil {
		return fmt.Errorf("failed to load assertions for HTTP %s: %w", httpID.String(), err)
	}

	if len(asserts) == 0 {
		// No assertions to evaluate
		return nil
	}

	// Filter enabled assertions and log statistics
	enabledAsserts := make([]mhttp.HTTPAssert, 0, len(asserts))
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
func (h *HttpServiceRPC) evaluateAssertionsParallel(ctx context.Context, asserts []mhttp.HTTPAssert, evalContext map[string]any) []AssertionResult {
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
		go func(idx int, assertion mhttp.HTTPAssert) {
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

			// Use the assertion value directly as the expression
			expression := assertion.Value
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
				slog.WarnContext(ctx, "Slow assertion evaluation",
					"assertion_id", assertion.ID.String(),
					"duration", duration)
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
			slog.WarnContext(ctx, "Assertion result collection timed out after 35 seconds")
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
			slog.WarnContext(ctx, "Assertion evaluation context cancelled", "error", evalCtx.Err())
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
		slog.WarnContext(ctx, "Incomplete assertion result collection",
			"collected", collectedCount,
			"total", len(asserts))
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
	defer devtoolsdb.TxnRollback(tx)

	responseWriter := shttp.NewHttpResponseWriter(tx)

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
		assert := mhttp.HTTPResponseAssert{
			ID:         assertID,
			ResponseID: responseID,
			Value:      value,
			Success:    success,
			CreatedAt:  now,
		}

		if err := responseWriter.CreateAssert(ctx, assert); err != nil {
			return fmt.Errorf("failed to insert assertion result for %s: %w", result.AssertionID.String(), err)
		}

		events = append(events, HttpResponseAssertEvent{
			Type:               eventTypeInsert,
			HttpResponseAssert: converter.ToAPIHttpResponseAssert(assert),
		})
	}

	slog.InfoContext(ctx, "Stored assertion results",
		"count", len(results),
		"http_id", httpID.String(),
		"response_id", responseID.String())

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Publish events
	workspaceID, err := h.httpReader.GetWorkspaceID(ctx, httpID)
	if err == nil {
		if h.streamers.HttpResponseAssert != nil {
			topic := HttpResponseAssertTopic{WorkspaceID: workspaceID}
			for _, evt := range events {
				h.streamers.HttpResponseAssert.Publish(topic, evt)
			}
		}
	} else {
		slog.WarnContext(ctx, "Failed to get workspace ID for publishing assertion events", "error", err)
	}

	return nil
}

// createAssertionEvalContext creates the evaluation context with response data and dynamic variables
func (h *HttpServiceRPC) createAssertionEvalContext(resp httpclient.Response) map[string]any {
	// Parse response body as JSON if possible, providing multiple formats
	var body any
	var bodyMap map[string]any
	bodyString := string(resp.Body)

	if err := json.Unmarshal(resp.Body, &body); err != nil {
		// If JSON parsing fails, use as string
		body = bodyString
	} else {
		// Also try to parse as map for easier access
		if mapBody, ok := body.(map[string]any); ok {
			bodyMap = mapBody
		}
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

// evaluateAssertion evaluates an assertion expression against the provided context
func (h *HttpServiceRPC) evaluateAssertion(ctx context.Context, expressionStr string, context map[string]any) (bool, error) {
	env := expression.NewEnv(context)
	return expression.ExpressionEvaluteAsBool(ctx, env, expressionStr)
}

func (h *HttpServiceRPC) logExecution(userID idwrap.IDWrap, httpEntry *mhttp.HTTP, err error) {
	if h.streamers.Log == nil {
		return
	}

	status := "Success"
	level := logv1.LogLevel_LOG_LEVEL_WARNING // default info/warning
	errMsg := ""

	if err != nil {
		status = "Failed"
		level = logv1.LogLevel_LOG_LEVEL_ERROR
		errMsg = err.Error()
	}

	msg := fmt.Sprintf("HTTP %s: %s", httpEntry.Name, status)

	val, _ := structpb.NewValue(map[string]any{
		"http_id": httpEntry.ID.String(),
		"name":    httpEntry.Name,
		"status":  status,
		"error":   errMsg,
	})

	h.streamers.Log.Publish(rlog.LogTopic{UserID: userID}, rlog.LogEvent{
		Type: rlog.EventTypeInsert,
		Log: &logv1.Log{
			LogId: idwrap.NewNow().Bytes(),
			Name:  msg,
			Level: level,
			Value: val,
		},
	})
}
