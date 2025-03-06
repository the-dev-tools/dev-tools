package request

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/url"
	"strings"
	"the-dev-tools/backend/pkg/compress"
	"the-dev-tools/backend/pkg/httpclient"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mbodyform"
	"the-dev-tools/backend/pkg/model/mbodyraw"
	"the-dev-tools/backend/pkg/model/mbodyurl"
	"the-dev-tools/backend/pkg/model/mexampleheader"
	"the-dev-tools/backend/pkg/model/mexamplequery"
	"the-dev-tools/backend/pkg/model/mitemapi"
	"the-dev-tools/backend/pkg/model/mitemapiexample"
	"the-dev-tools/backend/pkg/varsystem"
	"time"

	"connectrpc.com/connect"
)

type RequestResponse struct {
	HttpResp httpclient.Response
	LapTime  time.Duration
}

func PrepareRequest(endpoint mitemapi.ItemApi, example mitemapiexample.ItemApiExample, queries []mexamplequery.Query, headers []mexampleheader.Header,
	rawBody mbodyraw.ExampleBodyRaw, formBody []mbodyform.BodyForm, urlBody []mbodyurl.BodyURLEncoded, varMap varsystem.VarMap, client httpclient.HttpClient,
) (*RequestResponse, error) {
	var err error
	if varsystem.CheckStringHasAnyVarKey(endpoint.Url) {
		endpoint.Url, err = varMap.ReplaceVars(endpoint.Url)
		if err != nil {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
	}

	compressType := compress.CompressTypeNone
	if varMap != nil {
		for i, query := range queries {
			if varsystem.CheckIsVar(query.Value) {
				key := varsystem.GetVarKeyFromRaw(query.Value)
				if val, ok := varMap.Get(key); ok {
					queries[i].Value = val.Value
				} else {
					return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("%s named error not found", key))
				}
			}
		}
	}

	for i, header := range headers {
		if header.HeaderKey == "Content-Encoding" {
			switch strings.ToLower(header.Value) {
			case "gzip":
				compressType = compress.CompressTypeGzip
			case "zstd":
				compressType = compress.CompressTypeZstd
			case "deflate", "br", "identity":
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("%s not supported", header.Value))
			default:
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid compression type %s", header.Value))
			}
		}

		if varMap != nil {
			if varsystem.CheckIsVar(header.Value) {
				key := varsystem.GetVarKeyFromRaw(header.Value)
				if val, ok := varMap.Get(key); !ok {
					return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("%s named error not found", key))
				} else {
					headers[i].Value = val.Value
				}
			}
		}
	}

	bodyBytes := &bytes.Buffer{}
	switch example.BodyType {
	case mitemapiexample.BodyTypeRaw:
		bodyBytes.Write(rawBody.Data)
	case mitemapiexample.BodyTypeForm:
		writer := multipart.NewWriter(bodyBytes)
		for _, v := range formBody {
			if err := writer.WriteField(v.BodyKey, v.Value); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		}
	case mitemapiexample.BodyTypeUrlencoded:
		urlVal := url.Values{}
		for _, url := range urlBody {
			urlVal.Add(url.BodyKey, url.Value)
		}
		endpoint.Url += urlVal.Encode()
	}

	if compressType != compress.CompressTypeNone {
		compressedData, err := compress.Compress(bodyBytes.Bytes(), compressType)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		bodyBytes = bytes.NewBuffer(compressedData)
	}

	httpReq := httpclient.Request{
		Method:  endpoint.Method,
		URL:     endpoint.Url,
		Headers: headers,
		Queries: queries,
		Body:    bodyBytes.Bytes(),
	}

	now := time.Now()
	respHttp, err := httpclient.SendRequestAndConvert(client, httpReq, example.ID)
	lapse := time.Since(now)
	if err != nil {
		return nil, connect.NewError(connect.CodeAborted, err)
	}

	return &RequestResponse{HttpResp: respHttp, LapTime: lapse}, nil
}

func SendRequest(req httpclient.Request, exampleID idwrap.IDWrap) (*RequestResponse, error) {
	now := time.Now()
	respHttp, err := httpclient.SendRequestAndConvert(httpclient.New(), req, exampleID)
	lapse := time.Since(now)
	if err != nil {
		return nil, connect.NewError(connect.CodeAborted, err)
	}

	return &RequestResponse{HttpResp: respHttp, LapTime: lapse}, nil
}

type MergeExamplesInput struct {
	Base, Delta               mitemapiexample.ItemApiExample
	BaseQueries, DeltaQueries []mexamplequery.Query
	BaseHeaders, DeltaHeaders []mexampleheader.Header

	// Bodies
	BaseRawBody, DeltaRawBody               mbodyraw.ExampleBodyRaw
	BaseFormBody, DeltaFormBody             []mbodyform.BodyForm
	BaseUrlEncodedBody, DeltaUrlEncodedBody []mbodyurl.BodyURLEncoded
}

type MergeExamplesOutput struct {
	Merged              mitemapiexample.ItemApiExample
	MergeQueries        []mexamplequery.Query
	MergeHeaders        []mexampleheader.Header
	MergeRawBody        mbodyraw.ExampleBodyRaw
	MergeFormBody       []mbodyform.BodyForm
	MergeUrlEncodedBody []mbodyurl.BodyURLEncoded
}

// Function will merge two examples
// but ID will be the same as the base example
func MergeExamples(input MergeExamplesInput) MergeExamplesOutput {
	output := MergeExamplesOutput{}
	if input.Base.ID == input.Delta.ID {
		output.Merged = input.Base
	} else {
		output.Merged = input.Delta
		output.Merged.ID = input.Base.ID
	}

	// Query
	queryMap := make(map[idwrap.IDWrap]mexamplequery.Query, len(input.BaseQueries))
	for _, q := range input.BaseQueries {
		queryMap[q.ID] = q
	}
	for _, q := range input.DeltaQueries {
		queryMap[q.ID] = q
	}

	output.MergeQueries = make([]mexamplequery.Query, 0, len(queryMap))
	for _, q := range queryMap {
		output.MergeQueries = append(output.MergeQueries, q)
	}

	// Header
	headerMap := make(map[idwrap.IDWrap]mexampleheader.Header, len(input.BaseHeaders))
	for _, h := range input.BaseHeaders {
		headerMap[h.ID] = h
	}

	for _, h := range input.DeltaHeaders {
		headerMap[h.ID] = h
	}

	output.MergeHeaders = make([]mexampleheader.Header, 0, len(headerMap))
	for _, h := range headerMap {
		output.MergeHeaders = append(output.MergeHeaders, h)
	}

	// Raw Body
	if len(input.DeltaRawBody.Data) > 0 {
		output.MergeRawBody = input.DeltaRawBody
	} else {
		output.MergeRawBody = input.BaseRawBody
	}

	// Form Body
	formMap := make(map[idwrap.IDWrap]mbodyform.BodyForm, len(input.BaseFormBody))
	for _, f := range input.BaseFormBody {
		formMap[f.ID] = f
	}

	for _, f := range input.DeltaFormBody {
		formMap[f.ID] = f
	}

	output.MergeFormBody = make([]mbodyform.BodyForm, 0, len(formMap))
	for _, f := range formMap {
		output.MergeFormBody = append(output.MergeFormBody, f)
	}

	// Url Encoded Body
	urlEncodedMap := make(map[idwrap.IDWrap]mbodyurl.BodyURLEncoded, len(input.BaseUrlEncodedBody))
	for _, f := range input.BaseUrlEncodedBody {
		urlEncodedMap[f.ID] = f
	}

	for _, f := range input.DeltaUrlEncodedBody {
		urlEncodedMap[f.ID] = f
	}

	output.MergeUrlEncodedBody = make([]mbodyurl.BodyURLEncoded, 0, len(urlEncodedMap))
	for _, f := range urlEncodedMap {
		output.MergeUrlEncodedBody = append(output.MergeUrlEncodedBody, f)
	}

	return output
}
