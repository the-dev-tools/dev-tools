//nolint:revive // exported
package response

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/expression"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mgraphql"
)

type ResponseCreateGraphQLOutput struct {
	GraphQLResponse mgraphql.GraphQLResponse
	ResponseHeaders []mgraphql.GraphQLResponseHeader
	ResponseAsserts []mgraphql.GraphQLResponseAssert
}

func ResponseCreateGraphQL(
	ctx context.Context,
	respBody []byte,
	statusCode int,
	duration time.Duration,
	headers []mgraphql.GraphQLResponseHeader,
	graphqlID idwrap.IDWrap,
	assertions []mgraphql.GraphQLAssert,
	flowVars map[string]any,
) (*ResponseCreateGraphQLOutput, error) {
	responseID := idwrap.NewNow()
	now := time.Now().Unix()

	// Create response model
	graphqlResponse := mgraphql.GraphQLResponse{
		ID:        responseID,
		GraphQLID: graphqlID,
		Status:    int32(statusCode),
		Body:      respBody,
		Time:      now,
		Duration:  int32(duration.Milliseconds()),
		Size:      int32(len(respBody)),
		CreatedAt: now,
	}

	// Set response ID on headers
	responseHeaders := make([]mgraphql.GraphQLResponseHeader, len(headers))
	for i, h := range headers {
		responseHeaders[i] = h
		responseHeaders[i].ResponseID = responseID
		responseHeaders[i].CreatedAt = now
	}

	// Parse response body as JSON (similar to HTTP)
	var respBodyParsed any
	if err := json.Unmarshal(respBody, &respBodyParsed); err != nil {
		respBodyParsed = string(respBody)
	}

	// Build response variable (similar to HTTP's ConvertResponseToVar)
	responseVar := map[string]any{
		"status":   float64(statusCode),
		"body":     respBodyParsed,
		"headers":  convertHeadersToMap(headers),
		"duration": float64(duration.Milliseconds()),
	}

	// Build unified environment with flowVars and response binding
	// For GraphQL, also extract "data" and "errors" fields to top level for easier access
	evalEnvMap := buildAssertionEnv(flowVars, responseVar, respBodyParsed)
	env := expression.NewUnifiedEnv(evalEnvMap)

	responseAsserts := make([]mgraphql.GraphQLResponseAssert, 0)

	// Evaluate assertions (SAME pattern as HTTP)
	for _, assertion := range assertions {
		if assertion.Enabled {
			expr := assertion.Value

			// Skip assertions with empty expressions
			if strings.TrimSpace(expr) == "" {
				continue
			}

			// If expression contains {{ }}, interpolate first
			evaluatedExpr := expr
			if expression.HasVars(expr) {
				interpolated, err := env.Interpolate(expr)
				if err != nil {
					return nil, err
				}
				evaluatedExpr = interpolated
			}

			// Evaluate as boolean expression
			ok, err := env.EvalBool(ctx, evaluatedExpr)
			if err != nil {
				annotatedErr := annotateUnknownNameError(err, evalEnvMap)
				return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("expression %q failed: %w", evaluatedExpr, annotatedErr))
			}

			responseAsserts = append(responseAsserts, mgraphql.GraphQLResponseAssert{
				ID:         idwrap.NewNow(),
				ResponseID: responseID,
				Value:      evaluatedExpr,
				Success:    ok,
				CreatedAt:  now,
			})
		}
	}

	return &ResponseCreateGraphQLOutput{
		GraphQLResponse: graphqlResponse,
		ResponseHeaders: responseHeaders,
		ResponseAsserts: responseAsserts,
	}, nil
}

func buildAssertionEnv(flowVars map[string]any, responseBinding map[string]any, respBodyParsed any) map[string]any {
	env := make(map[string]any)

	// Add flow variables first
	for k, v := range flowVars {
		env[k] = v
	}

	// Add response binding for backward compatibility
	env["response"] = responseBinding

	// Extract GraphQL-specific fields from response body (matching GraphQL tab behavior)
	var data any
	var errors any
	if bodyMap, ok := respBodyParsed.(map[string]any); ok {
		if d, hasData := bodyMap["data"]; hasData {
			data = d
		}
		if e, hasErrors := bodyMap["errors"]; hasErrors {
			errors = e
		}
	}

	// Add GraphQL-specific fields at top level for easier access (matching GraphQL tab behavior)
	// This allows assertions like: data.users[0].id == "1"
	env["data"] = data
	env["errors"] = errors

	return env
}

func convertHeadersToMap(headers []mgraphql.GraphQLResponseHeader) map[string]any {
	headersMap := make(map[string]any)
	for _, h := range headers {
		if existing, ok := headersMap[h.HeaderKey]; ok {
			// Multiple values for same key - convert to array
			if arr, isArr := existing.([]any); isArr {
				headersMap[h.HeaderKey] = append(arr, h.HeaderValue)
			} else {
				headersMap[h.HeaderKey] = []any{existing, h.HeaderValue}
			}
		} else {
			headersMap[h.HeaderKey] = h.HeaderValue
		}
	}
	return headersMap
}

func annotateUnknownNameError(err error, env map[string]any) error {
	if err == nil {
		return nil
	}
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "unknown name") {
		keys := collectEnvKeys(env)
		if len(keys) > 0 {
			return fmt.Errorf("%w (available variables: %s)", err, strings.Join(keys, ", "))
		}
	}
	return err
}

func collectEnvKeys(env map[string]any) []string {
	if len(env) == 0 {
		return nil
	}
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
