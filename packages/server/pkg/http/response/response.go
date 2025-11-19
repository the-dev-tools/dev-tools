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
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/model/massertres"
	"the-dev-tools/server/pkg/model/mexampleresp"
	"the-dev-tools/server/pkg/model/mexamplerespheader"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/varsystem"
	"the-dev-tools/server/pkg/zstdcompress"

	"connectrpc.com/connect"
)

type ResponseCreateOutput struct {
	BodyRaw     []byte
	ExampleResp mexampleresp.ExampleResp

	AssertCouples []AssertCouple

	// new headers
	CreateHeaders, UpdateHeaders []mexamplerespheader.ExampleRespHeader
	DeleteHeaderIds              []idwrap.IDWrap
}

type ResponseCreateHTTPOutput struct {
	HTTPResponse    mhttp.HTTPResponse
	ResponseHeaders []mhttp.HTTPResponseHeader
	ResponseAsserts []mhttp.HTTPResponseAssert
}

type AssertCouple struct {
	Assert    massert.Assert
	AssertRes massertres.AssertResult
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
		Status:    int32(respHttp.StatusCode),
		Body:      respHttp.Body,
		Time:      now,
		Duration:  int32(lapse.Milliseconds()),
		Size:      int32(len(respHttp.Body)),
		CreatedAt: now,
	}

	responseHeaders := make([]mhttp.HTTPResponseHeader, 0, len(respHttp.Headers))
	for _, h := range respHttp.Headers {
		responseHeaders = append(responseHeaders, mhttp.HTTPResponseHeader{
			ID:          idwrap.NewNow(),
			HttpID:      httpID,
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
				ID:        idwrap.NewNow(),
				HttpID:    httpID,
				Value:     expr,
				Success:   ok,
				CreatedAt: now,
			})
		}
	}

	return &ResponseCreateHTTPOutput{
		HTTPResponse:    httpResponse,
		ResponseHeaders: responseHeaders,
		ResponseAsserts: responseAsserts,
	}, nil
}

func ResponseCreate(ctx context.Context, r request.RequestResponse, exampleResp mexampleresp.ExampleResp, lastResonseHeaders []mexamplerespheader.ExampleRespHeader, assertions []massert.Assert, varMap varsystem.VarMap, flowVars map[string]any) (*ResponseCreateOutput, error) {
	ResponseCreateOutput := ResponseCreateOutput{}
	respHttp := r.HttpResp
	lapse := r.LapTime
	ResponseCreateOutput.BodyRaw = respHttp.Body
	bodyData := respHttp.Body

	exampleResp.BodyCompressType = mexampleresp.BodyCompressTypeNone

	if len(bodyData) > 1024 {
		bodyDataTemp := zstdcompress.Compress(bodyData)
		if len(bodyDataTemp) < len(bodyData) {
			exampleResp.BodyCompressType = mexampleresp.BodyCompressTypeZstd
			bodyData = bodyDataTemp
		}
	}

	exampleResp.Body = bodyData
	exampleResp.Duration = int32(lapse.Milliseconds())
	exampleResp.Status = uint16(respHttp.StatusCode)

	ResponseCreateOutput.ExampleResp = exampleResp

	taskCreateHeaders := make([]mexamplerespheader.ExampleRespHeader, 0)
	taskUpdateHeaders := make([]mexamplerespheader.ExampleRespHeader, 0)
	taskDeleteHeaders := make([]idwrap.IDWrap, 0)

	// Create a map for quick lookup of current headers by key
	headerMap := make(map[string]mexamplerespheader.ExampleRespHeader, len(lastResonseHeaders))
	headerProcessed := make(map[string]struct{}, len(lastResonseHeaders))

	for _, header := range lastResonseHeaders {
		headerMap[header.HeaderKey] = header
	}

	for _, respHeader := range respHttp.Headers {
		dbHeader, found := headerMap[respHeader.HeaderKey]
		headerProcessed[respHeader.HeaderKey] = struct{}{}

		if found {
			// Update existing header if values differ
			if dbHeader.Value != respHeader.Value {
				dbHeader.Value = respHeader.Value
				taskUpdateHeaders = append(taskUpdateHeaders, dbHeader)
			}
		} else {
			// Create new header if not found
			taskCreateHeaders = append(taskCreateHeaders, mexamplerespheader.ExampleRespHeader{
				ID:            idwrap.NewNow(),
				ExampleRespID: exampleResp.ID,
				HeaderKey:     respHeader.HeaderKey,
				Value:         respHeader.Value,
			})
		}
	}

	for _, header := range lastResonseHeaders {
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
		if assertion.Enable {
			// Use NormalizeExpression if {{ }} wrapper is found
			expr := assertion.Condition.Comparisons.Expression
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
			res := massertres.AssertResult{
				ID:         idwrap.NewNow(),
				ResponseID: exampleResp.ID,
				AssertID:   assertion.ID,
				Result:     ok,
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
