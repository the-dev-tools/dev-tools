package response

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"the-dev-tools/server/pkg/expression"
	"the-dev-tools/server/pkg/http/request"
	"the-dev-tools/server/pkg/httpclient"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/varsystem"
	"the-dev-tools/server/pkg/zstdcompress"

	"connectrpc.com/connect"
)

type ResponseCreateOutput struct {
	BodyRaw     []byte
	HTTPResponse mhttp.HTTPResponse

	AssertCouples []AssertCouple

	// new headers
	CreateHeaders, UpdateHeaders []mhttp.HTTPResponseHeader
	DeleteHeaderIds              []idwrap.IDWrap
}

type ResponseCreateHTTPOutput struct {
	HTTPResponse    mhttp.HTTPResponse
	ResponseHeaders []mhttp.HTTPResponseHeader
	ResponseAsserts []mhttp.HTTPResponseAssert
}

type AssertCouple struct {
	Assert    mhttp.HTTPAssert
	AssertRes mhttp.HTTPResponseAssert
}

func ResponseCreateHTTP(
	ctx context.Context,
	r request.RequestResponse,
	httpID idwrap.IDWrap,
	assertions []mhttp.HTTPAssert,
	varMap varsystem.VarMap,
	flowVars map[string]any,
) (*ResponseCreateHTTPOutput, error) {
	respHttp := r.HttpResp
	lapse := r.LapTime

	responseID := idwrap.NewNow()
	now := time.Now().Unix()

	httpResponse := mhttp.HTTPResponse{
		ID:        responseID,
		HttpID:    httpID,
		Status:    int32(respHttp.StatusCode), // nolint:gosec // G115
		Body:      respHttp.Body,
		Time:      now,
		Duration:  int32(lapse.Milliseconds()), // nolint:gosec // G115
		Size:      int32(len(respHttp.Body)),   // nolint:gosec // G115
		CreatedAt: now,
	}

	responseHeaders := make([]mhttp.HTTPResponseHeader, 0, len(respHttp.Headers))
	for _, h := range respHttp.Headers {
		responseHeaders = append(responseHeaders, mhttp.HTTPResponseHeader{
			ID:          idwrap.NewNow(),
			ResponseID:  responseID,
			HeaderKey:   h.HeaderKey,
			HeaderValue: h.Value,
			CreatedAt:   now,
		})
	}

	responseAsserts := make([]mhttp.HTTPResponseAssert, 0)
	responseVar := httpclient.ConvertResponseToVar(respHttp)
	responseBinding := map[string]any{
		"status":   responseVar.StatusCode,
		"body":     responseVar.Body,
		"headers":  responseVar.Headers,
		"duration": responseVar.Duration,
	}
	responseEnv := map[string]any{"response": responseBinding}
	mergedVarMap := varsystem.MergeVarMap(varMap, varsystem.NewVarMapFromAnyMap(responseEnv))
	evalEnvMap := buildAssertionEnv(flowVars, responseBinding)
	exprEnv := expression.NewEnv(evalEnvMap)

	normalizedExprCache := make(map[string]string)

	for _, assertion := range assertions {
		if assertion.Enabled {
			expr := assertion.AssertValue // Using Value as expression? Old model used Condition.Comparisons.Expression.
			// mhttp.HTTPAssert has AssertKey and AssertValue.
			// If it's an expression assertion, maybe Key is empty or Value is the expression?
			// Or Key is description?
			// "AssertKey string", "AssertValue string".
			// "Condition" is missing.
			// Spec says: `model HttpAssert { ... value: string; }`.
			// `key` is likely the name/description or key if it's a key-value assertion.
			// Assuming `AssertValue` is the expression to evaluate.
			
			// Check if we need normalization
			if strings.Contains(expr, "{{") && strings.Contains(expr, "}}") {
				if cached, ok := normalizedExprCache[expr]; ok {
					expr = cached
				} else {
					normalized, err := expression.NormalizeExpression(ctx, expr, mergedVarMap)
					if err != nil {
						return nil, err
					}
					normalizedExprCache[expr] = normalized
					expr = normalized
				}
			}

			ok, err := expression.ExpressionEvaluteAsBool(ctx, exprEnv, expr)
			if err != nil {
				annotatedErr := annotateUnknownNameError(err, evalEnvMap)
				return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("expression %q failed: %w", expr, annotatedErr))
			}

			responseAsserts = append(responseAsserts, mhttp.HTTPResponseAssert{
				ID:         idwrap.NewNow(),
				ResponseID: responseID,
				Value:      expr,
				Success:    ok,
				CreatedAt:  now,
			})
		}
	}

	return &ResponseCreateHTTPOutput{
		HTTPResponse:    httpResponse,
		ResponseHeaders: responseHeaders,
		ResponseAsserts: responseAsserts,
	}, nil
}

func ResponseCreate(ctx context.Context, r request.RequestResponse, httpResponse mhttp.HTTPResponse, lastResponseHeaders []mhttp.HTTPResponseHeader, assertions []mhttp.HTTPAssert, varMap varsystem.VarMap, flowVars map[string]any) (*ResponseCreateOutput, error) {
	ResponseCreateOutput := ResponseCreateOutput{}
	respHttp := r.HttpResp
	lapse := r.LapTime
	ResponseCreateOutput.BodyRaw = respHttp.Body
	bodyData := respHttp.Body

	// Note: mhttp.HTTPResponse doesn't have compression field, handle compression at raw level if needed

	if len(bodyData) > 1024 {
		bodyDataTemp := zstdcompress.Compress(bodyData)
		if len(bodyDataTemp) < len(bodyData) {
			// Store compressed data in body
			bodyData = bodyDataTemp
		}
	}

	// Update httpResponse with actual response data
	httpResponse.Body = bodyData
	httpResponse.Duration = int32(lapse.Milliseconds()) // nolint:gosec // G115
	httpResponse.Status = int32(respHttp.StatusCode)    // nolint:gosec // G115
	httpResponse.Size = int32(len(bodyData))            // nolint:gosec // G115
	httpResponse.CreatedAt = time.Now().Unix()

	ResponseCreateOutput.HTTPResponse = httpResponse

	taskCreateHeaders := make([]mhttp.HTTPResponseHeader, 0)
	taskUpdateHeaders := make([]mhttp.HTTPResponseHeader, 0)
	taskDeleteHeaders := make([]idwrap.IDWrap, 0)

	// Create a map for quick lookup of current headers by key
	headerMap := make(map[string]mhttp.HTTPResponseHeader, len(lastResponseHeaders))
	headerProcessed := make(map[string]struct{}, len(lastResponseHeaders))

	for _, header := range lastResponseHeaders {
		headerMap[header.HeaderKey] = header
	}

	for _, respHeader := range respHttp.Headers {
		dbHeader, found := headerMap[respHeader.HeaderKey]
		headerProcessed[respHeader.HeaderKey] = struct{}{}

		if found {
			// Update existing header if values differ
			if dbHeader.HeaderValue != respHeader.Value {
				dbHeader.HeaderValue = respHeader.Value
				taskUpdateHeaders = append(taskUpdateHeaders, dbHeader)
			}
		} else {
			// Create new header if not found
			taskCreateHeaders = append(taskCreateHeaders, mhttp.HTTPResponseHeader{
				ID:          idwrap.NewNow(),
				ResponseID:  httpResponse.ID,
				HeaderKey:   respHeader.HeaderKey,
				HeaderValue: respHeader.Value,
				CreatedAt:   time.Now().Unix(),
			})
		}
	}

	for _, header := range lastResponseHeaders {
		_, ok := headerProcessed[header.HeaderKey]
		if !ok {
			taskDeleteHeaders = append(taskDeleteHeaders, header.ID)
		}
	}

	ResponseCreateOutput.CreateHeaders = taskCreateHeaders
	ResponseCreateOutput.UpdateHeaders = taskUpdateHeaders
	ResponseCreateOutput.DeleteHeaderIds = taskDeleteHeaders

	var resultArr []AssertCouple
	// TODO: move to proper package
	responseVar := httpclient.ConvertResponseToVar(respHttp)

	// Create environment manually to ensure proper structure
	responseBinding := map[string]any{
		"status":   responseVar.StatusCode,
		"body":     responseVar.Body,
		"headers":  responseVar.Headers,
		"duration": responseVar.Duration,
	}
	responseEnv := map[string]any{"response": responseBinding}
	mergedVarMap := varsystem.MergeVarMap(varMap, varsystem.NewVarMapFromAnyMap(responseEnv))
	evalEnvMap := buildAssertionEnv(flowVars, responseBinding)
	exprEnv := expression.NewEnv(evalEnvMap)

	normalizedExprCache := make(map[string]string)
	for _, assertion := range assertions {
		if assertion.Enabled {
			// Use NormalizeExpression if {{ }} wrapper is found
			expr := assertion.AssertValue
			if strings.Contains(expr, "{{") && strings.Contains(expr, "}}") {
				if cached, ok := normalizedExprCache[expr]; ok {
					expr = cached
				} else {
					normalized, err := expression.NormalizeExpression(ctx, expr, mergedVarMap)
					if err != nil {
						return nil, err
					}
					normalizedExprCache[expr] = normalized
					expr = normalized
				}
			}

			ok, err := expression.ExpressionEvaluteAsBool(ctx, exprEnv, expr)
			if err != nil {
				annotatedErr := annotateUnknownNameError(err, evalEnvMap)
				return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("expression %q failed: %w", expr, annotatedErr))
			}
			res := mhttp.HTTPResponseAssert{
				ID:         idwrap.NewNow(),
				ResponseID: httpResponse.ID,
				Value:      expr,
				Success:    ok,
				CreatedAt:  time.Now().Unix(),
			}

			resultArr = append(resultArr, AssertCouple{
				Assert:    assertion,
				AssertRes: res,
			})

		}
	}

	ResponseCreateOutput.AssertCouples = resultArr

	return &ResponseCreateOutput, nil
}

func buildAssertionEnv(flowVars map[string]any, responseBinding map[string]any) map[string]any {
	envSize := 1
	if len(flowVars) > 0 {
		envSize += len(flowVars)
	}
	env := make(map[string]any, envSize)
	for k, v := range flowVars {
		env[k] = v
	}
	env["response"] = responseBinding
	return env
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
