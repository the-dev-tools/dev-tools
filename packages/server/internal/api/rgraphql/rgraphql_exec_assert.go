//nolint:revive // exported
package rgraphql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	devtoolsdb "github.com/the-dev-tools/dev-tools/packages/db"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/expression"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mgraphql"
)

// AssertionResult represents the result of evaluating a single assertion
type AssertionResult struct {
	AssertionID idwrap.IDWrap
	Expression  string
	Success     bool
	Error       error
	EvaluatedAt time.Time
}

// GraphQLResponseData wraps the response for assertion evaluation
type GraphQLResponseData struct {
	StatusCode int
	Body       []byte
	Headers    map[string]string
}

// evaluateAndStoreAssertions evaluates assertions and stores them within a transaction, returning the created assertions
// This is used by GraphQLRun to evaluate assertions before commit so they can be cloned into snapshots
func (s *GraphQLServiceRPC) evaluateAndStoreAssertions(ctx context.Context, tx *sql.Tx, graphqlID idwrap.IDWrap, responseID idwrap.IDWrap, workspaceID idwrap.IDWrap, resp GraphQLResponseData, asserts []mgraphql.GraphQLAssert) ([]mgraphql.GraphQLResponseAssert, error) {
	if len(asserts) == 0 {
		return []mgraphql.GraphQLResponseAssert{}, nil
	}

	enabledAsserts := make([]mgraphql.GraphQLAssert, 0, len(asserts))
	for _, assert := range asserts {
		if assert.IsEnabled() {
			enabledAsserts = append(enabledAsserts, assert)
		}
	}

	if len(enabledAsserts) == 0 {
		return []mgraphql.GraphQLResponseAssert{}, nil
	}

	evalContext := s.createAssertionEvalContext(resp)
	results := s.evaluateAssertionsParallel(ctx, enabledAsserts, evalContext)

	// Store results within the provided transaction
	responseAsserts, err := s.storeAssertionResultsInTx(ctx, tx, responseID, results)
	if err != nil {
		return nil, fmt.Errorf("failed to store assertion results for GraphQL %s: %w", graphqlID.String(), err)
	}

	return responseAsserts, nil
}

// evaluateResolvedAssertions evaluates pre-resolved assertions against the response and stores the results
// This is the original function used for standalone assertion evaluation (kept for compatibility)
func (s *GraphQLServiceRPC) evaluateResolvedAssertions(ctx context.Context, graphqlID idwrap.IDWrap, responseID idwrap.IDWrap, workspaceID idwrap.IDWrap, resp GraphQLResponseData, asserts []mgraphql.GraphQLAssert) error {
	if len(asserts) == 0 {
		return nil
	}

	enabledAsserts := make([]mgraphql.GraphQLAssert, 0, len(asserts))
	for _, assert := range asserts {
		if assert.IsEnabled() {
			enabledAsserts = append(enabledAsserts, assert)
		}
	}

	if len(enabledAsserts) == 0 {
		return nil
	}

	evalContext := s.createAssertionEvalContext(resp)
	results := s.evaluateAssertionsParallel(ctx, enabledAsserts, evalContext)

	if err := s.storeAssertionResultsBatch(ctx, graphqlID, responseID, workspaceID, results); err != nil {
		return fmt.Errorf("failed to store assertion results for GraphQL %s: %w", graphqlID.String(), err)
	}

	return nil
}

// evaluateAssertionsParallel evaluates multiple assertions in parallel with timeout and error handling
func (s *GraphQLServiceRPC) evaluateAssertionsParallel(ctx context.Context, asserts []mgraphql.GraphQLAssert, evalContext map[string]any) []AssertionResult {
	results := make([]AssertionResult, len(asserts))
	resultChan := make(chan AssertionResult, len(asserts))

	var wg sync.WaitGroup

	// Create a context with timeout for assertion evaluation (30 seconds per assertion batch)
	evalCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Evaluate each assertion in a separate goroutine
	for i, assert := range asserts {
		wg.Add(1)
		go func(idx int, assertion mgraphql.GraphQLAssert) {
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
			success, err := s.evaluateAssertion(evalCtx, expression, evalContext)
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

// storeAssertionResultsInTx stores assertion results within an existing transaction and returns the created assertions
func (s *GraphQLServiceRPC) storeAssertionResultsInTx(ctx context.Context, tx *sql.Tx, responseID idwrap.IDWrap, results []AssertionResult) ([]mgraphql.GraphQLResponseAssert, error) {
	if len(results) == 0 {
		return []mgraphql.GraphQLResponseAssert{}, nil
	}

	txResponseService := s.responseService.TX(tx)
	now := time.Now().Unix()
	responseAsserts := make([]mgraphql.GraphQLResponseAssert, 0, len(results))

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
		assert := mgraphql.GraphQLResponseAssert{
			ID:         assertID,
			ResponseID: responseID,
			Value:      value,
			Success:    success,
			CreatedAt:  now,
		}

		if err := txResponseService.CreateAssert(ctx, assert); err != nil {
			return nil, fmt.Errorf("failed to insert assertion result for %s: %w", result.AssertionID.String(), err)
		}

		responseAsserts = append(responseAsserts, assert)
	}

	slog.InfoContext(ctx, "Stored assertion results in transaction",
		"count", len(results),
		"response_id", responseID.String())

	return responseAsserts, nil
}

// storeAssertionResultsBatch stores multiple assertion results in a single database transaction
func (s *GraphQLServiceRPC) storeAssertionResultsBatch(ctx context.Context, graphqlID idwrap.IDWrap, responseID idwrap.IDWrap, workspaceID idwrap.IDWrap, results []AssertionResult) error {
	if len(results) == 0 {
		return nil
	}

	// Start transaction for batch insertion
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer devtoolsdb.TxnRollback(tx)

	txResponseService := s.responseService.TX(tx)

	// Insert all results in batch
	now := time.Now().Unix()
	var events []GraphQLResponseAssertEvent

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
		assert := mgraphql.GraphQLResponseAssert{
			ID:         assertID,
			ResponseID: responseID,
			Value:      value,
			Success:    success,
			CreatedAt:  now,
		}

		if err := txResponseService.CreateAssert(ctx, assert); err != nil {
			return fmt.Errorf("failed to insert assertion result for %s: %w", result.AssertionID.String(), err)
		}

		events = append(events, GraphQLResponseAssertEvent{
			Type:                  eventTypeInsert,
			GraphQLResponseAssert: ToAPIGraphQLResponseAssert(assert),
		})
	}

	slog.InfoContext(ctx, "Stored assertion results",
		"count", len(results),
		"graphql_id", graphqlID.String(),
		"response_id", responseID.String())

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Publish events
	if s.streamers.GraphQLResponseAssert != nil {
		topic := GraphQLResponseAssertTopic{WorkspaceID: workspaceID}
		for _, evt := range events {
			s.streamers.GraphQLResponseAssert.Publish(topic, evt)
		}
	}

	return nil
}

// createAssertionEvalContext creates the evaluation context with response data
func (s *GraphQLServiceRPC) createAssertionEvalContext(resp GraphQLResponseData) map[string]any {
	// Parse response body as JSON if possible
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

	// Convert headers to map
	headers := make(map[string]string)
	headersLower := make(map[string]string)
	contentType := ""

	for key, value := range resp.Headers {
		lowerKey := strings.ToLower(key)
		headers[key] = value
		headersLower[lowerKey] = value

		if lowerKey == "content-type" {
			contentType = value
		}
	}

	// Extract GraphQL-specific fields from response
	var data any
	var errors any
	if bodyMap != nil {
		if d, ok := bodyMap["data"]; ok {
			data = d
		}
		if e, ok := bodyMap["errors"]; ok {
			errors = e
		}
	}

	// Extract JSON path helpers (for full body navigation)
	jsonPathHelpers := s.createJSONPathHelpers(bodyMap)

	// Extract JSON path helpers for data field specifically
	var dataMap map[string]any
	if data != nil {
		if dm, ok := data.(map[string]any); ok {
			dataMap = dm
		}
	}
	dataPathHelpers := s.createJSONPathHelpers(dataMap)

	// Create comprehensive evaluation context
	context := map[string]any{
		// Main response object
		"response": map[string]any{
			"status":  resp.StatusCode,
			"body":    body,
			"headers": headers,
			"data":    data,
			"errors":  errors,
		},

		// Direct access to commonly used fields
		"status":       resp.StatusCode,
		"body":         body,
		"body_string":  bodyString,
		"headers":      headers,
		"content_type": contentType,

		// GraphQL-specific fields (top-level for convenience)
		"data":   data,
		"errors": errors,

		// Convenience variables
		"success":      resp.StatusCode >= 200 && resp.StatusCode < 300,
		"client_error": resp.StatusCode >= 400 && resp.StatusCode < 500,
		"server_error": resp.StatusCode >= 500 && resp.StatusCode < 600,
		"is_json":      strings.HasPrefix(contentType, "application/json"),
		"has_body":     len(resp.Body) > 0,
		"has_data":     data != nil,
		"has_errors":   errors != nil,

		// JSON path helpers (for full body)
		"json": jsonPathHelpers,
		// JSON path helpers specifically for data field
		"dataJson": dataPathHelpers,
	}

	return context
}

// createJSONPathHelpers creates helper functions for JSON path navigation
func (s *GraphQLServiceRPC) createJSONPathHelpers(bodyMap map[string]any) map[string]any {
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

	// Helper to check if path exists
	hasPath := func(path string) bool {
		return getPath(path) != nil
	}

	// Helper to get string value
	getString := func(path string) string {
		val := getPath(path)
		if val == nil {
			return ""
		}
		if str, ok := val.(string); ok {
			return str
		}
		return fmt.Sprintf("%v", val)
	}

	// Helper to get numeric value
	getNumber := func(path string) float64 {
		val := getPath(path)
		if val == nil {
			return 0
		}
		switch num := val.(type) {
		case float64:
			return num
		case int:
			return float64(num)
		case int64:
			return float64(num)
		default:
			if str, ok := val.(string); ok {
				var f float64
				fmt.Sscanf(str, "%f", &f)
				return f
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
func (s *GraphQLServiceRPC) evaluateAssertion(ctx context.Context, expressionStr string, context map[string]any) (bool, error) {
	env := expression.NewEnv(context)
	return expression.ExpressionEvaluteAsBool(ctx, env, expressionStr)
}
