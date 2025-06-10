package response

import (
	"context"
	"fmt"
	"strings"
	"the-dev-tools/server/pkg/expression"
	"the-dev-tools/server/pkg/http/request"
	"the-dev-tools/server/pkg/httpclient"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/model/massertres"
	"the-dev-tools/server/pkg/model/mexampleresp"
	"the-dev-tools/server/pkg/model/mexamplerespheader"
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

type AssertCouple struct {
	Assert    massert.Assert
	AssertRes massertres.AssertResult
}

func ResponseCreate(ctx context.Context, r request.RequestResponse, exampleResp mexampleresp.ExampleResp, lastResonseHeaders []mexamplerespheader.ExampleRespHeader, assertions []massert.Assert, varMap varsystem.VarMap) (*ResponseCreateOutput, error) {
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
	envMap := map[string]any{
		"response": map[string]any{
			"status":   responseVar.StatusCode,
			"body":     responseVar.Body,
			"headers":  responseVar.Headers,
			"duration": responseVar.Duration,
		},
	}
	mergedVarMap := varsystem.MergeVarMap(varMap, varsystem.NewVarMapFromAnyMap(envMap))
	exprEnv := expression.NewEnv(envMap)

	for _, assertion := range assertions {
		if assertion.Enable {
			// Use NormalizeExpression if {{ }} wrapper is found
			expr := assertion.Condition.Comparisons.Expression
			var err error
			if strings.Contains(expr, "{{") && strings.Contains(expr, "}}") {
				fmt.Println("expr", expr)
				fmt.Println("varMap", varMap)
				expr, err = expression.NormalizeExpression(ctx, expr, mergedVarMap)
				if err != nil {
					return nil, err
				}
			}

			ok, err := expression.ExpressionEvaluteAsBool(ctx, exprEnv, expr)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
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
