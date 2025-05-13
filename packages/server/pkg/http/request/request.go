package request

import (
	"bytes"
	"fmt"
	"mime"
	"mime/multipart"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/httpclient"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mbodyform"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mbodyurl"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mexamplequery"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/sort/sortenabled"
	"the-dev-tools/server/pkg/varsystem"
	"time"

	"connectrpc.com/connect"
)

type RequestResponseVar struct {
	Headers map[string]string `json:"headers"`
	Queries map[string]string `json:"queries"`
	Body    string            `json:"body"`
}

type RequestResponse struct {
	HttpResp httpclient.Response
	LapTime  time.Duration
}

func ConvertRequestToVar(r *httpclient.Request) RequestResponseVar {
	headersMaps := make(map[string]string, len(r.Headers))
	queriesMaps := make(map[string]string, len(r.Queries))
	for _, header := range r.Headers {
		headersMaps[header.HeaderKey] = header.Value
	}

	for _, query := range r.Queries {
		queriesMaps[query.QueryKey] = query.Value
	}
	return RequestResponseVar{Headers: headersMaps, Queries: queriesMaps, Body: string(r.Body)}
}

// quoteEscaper is used to escape quotes in MIME headers.
var quoteEscaper = strings.NewReplacer("\\", "\\\\", "\"", "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}

func PrepareRequest(endpoint mitemapi.ItemApi, example mitemapiexample.ItemApiExample, queries []mexamplequery.Query, headers []mexampleheader.Header,
	rawBody mbodyraw.ExampleBodyRaw, formBody []mbodyform.BodyForm, urlBody []mbodyurl.BodyURLEncoded, varMap varsystem.VarMap,
) (*httpclient.Request, error) {
	var err error
	if varsystem.CheckStringHasAnyVarKey(endpoint.Url) {
		endpoint.Url, err = varMap.ReplaceVars(endpoint.Url)
		if err != nil {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
	}

	// get only enabled
	sortenabled.GetAllWithState(&headers, true)
	sortenabled.GetAllWithState(&queries, true)
	sortenabled.GetAllWithState(&formBody, true)
	sortenabled.GetAllWithState(&urlBody, true)

	compressType := compress.CompressTypeNone
	if varMap != nil {
		for i, query := range queries {
			if varsystem.CheckIsVar(query.QueryKey) {
				key := varsystem.GetVarKeyFromRaw(query.QueryKey)
				if val, ok := varMap.Get(key); ok {
					queries[i].QueryKey = val.Value
				} else {
					return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("%s named variable not found", key))
				}
			}

			if varsystem.CheckIsVar(query.Value) {
				key := varsystem.GetVarKeyFromRaw(query.Value)
				if val, ok := varMap.Get(key); ok {
					queries[i].Value = val.Value
				} else {
					return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("%s named variable not found", key))
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
			case "br":
				compressType = compress.CompressTypeBr
			case "deflate", "identity":
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("%s not supported", header.Value))
			default:
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid compression type %s", header.Value))
			}
		}

		if varMap != nil {
			if varsystem.CheckIsVar(header.HeaderKey) {
				key := varsystem.GetVarKeyFromRaw(header.HeaderKey)
				if val, ok := varMap.Get(key); ok {
					headers[i].HeaderKey = val.Value
				} else {
					return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("%s named variable not found", key))
				}
			}

			if varsystem.CheckIsVar(header.Value) {
				key := varsystem.GetVarKeyFromRaw(header.Value)
				if val, ok := varMap.Get(key); ok {
					headers[i].Value = val.Value
				} else {
					return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("%s named variablot found", key))
				}
			}
		}
	}

	bodyBytes := &bytes.Buffer{}
	switch example.BodyType {
	case mitemapiexample.BodyTypeRaw:
		if len(rawBody.Data) > 0 {
			if rawBody.CompressType != compress.CompressTypeNone {
				rawBody.Data, err = compress.Decompress(rawBody.Data, rawBody.CompressType)
				if err != nil {
					return nil, err
				}
			}
			bodyStr := string(rawBody.Data)
			bodyStr, err = varMap.ReplaceVars(bodyStr)
			if err != nil {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			rawBody.Data = []byte(bodyStr)
		}
		_, err = bodyBytes.Write(rawBody.Data)
		if err != nil {
			return nil, err
		}
	case mitemapiexample.BodyTypeForm:
		writer := multipart.NewWriter(bodyBytes)
		for _, v := range formBody {
			actualBodyKey := v.BodyKey
			if varsystem.CheckIsVar(v.BodyKey) {
				key := varsystem.GetVarKeyFromRaw(v.BodyKey)
				if val, ok := varMap.Get(key); ok {
					actualBodyKey = val.Value
				} else {
					return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("%s named error not found", key))
				}
			}

			resolvedValue := v.Value
			isFileUpload := false
			actualFilePathForUpload := ""

			if varsystem.CheckIsVar(v.Value) {
				variableKey := varsystem.GetVarKeyFromRaw(v.Value)
				if varsystem.IsFileReference(variableKey) { // Direct file reference: {{#file:path.txt}}
					isFileUpload = true
					actualFilePathForUpload = varsystem.GetIsFileReferencePath(variableKey)
				} else { // Regular variable: {{my_var}}
					if val, ok := varMap.Get(variableKey); ok {
						if strings.HasPrefix(val.Value, "#file:") { // Variable contains file reference
							isFileUpload = true
							actualFilePathForUpload = strings.TrimPrefix(val.Value, "#file:")
						} else { // Variable contains regular text
							resolvedValue = val.Value
						}
					} else {
						return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("%s named error not found", variableKey))
					}
				}
			}

			if isFileUpload {
				fileContentBytes, err := os.ReadFile(actualFilePathForUpload)
				if err != nil {
					return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to read file %s: %w", actualFilePathForUpload, err))
				}

				fileName := filepath.Base(actualFilePathForUpload)

				h := make(textproto.MIMEHeader)
				h.Set("Content-Disposition",
					fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
						escapeQuotes(actualBodyKey), escapeQuotes(fileName)))

				// Determine MIME type based on file extension
				mimeType := mime.TypeByExtension(filepath.Ext(fileName))
				if mimeType == "" {
					mimeType = "application/octet-stream" // Default if type can't be determined
				}
				h.Set("Content-Type", mimeType)

				partWriter, err := writer.CreatePart(h)
				if err != nil {
					return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create form part: %w", err))
				}

				if _, err = partWriter.Write(fileContentBytes); err != nil {
					return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to write file content: %w", err))
				}
			} else {
				if err := writer.WriteField(actualBodyKey, resolvedValue); err != nil {
					return nil, connect.NewError(connect.CodeInternal, err)
				}
			}
		}
		if err := writer.Close(); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to close multipart writer: %w", err))
		}
	case mitemapiexample.BodyTypeUrlencoded:
		urlVal := url.Values{}
		for _, url := range urlBody {
			if varsystem.CheckIsVar(url.BodyKey) {
				key := varsystem.GetVarKeyFromRaw(url.Value)
				if val, ok := varMap.Get(key); ok {
					url.BodyKey = val.Value
				} else {
					return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("%s named error not found", key))
				}
			}
			if varsystem.CheckIsVar(url.Value) {
				key := varsystem.GetVarKeyFromRaw(url.Value)
				if val, ok := varMap.Get(key); ok {
					url.Value = val.Value
				} else {
					return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("%s named error not found", key))
				}
			}

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

	httpReq := &httpclient.Request{
		Method:  endpoint.Method,
		URL:     endpoint.Url,
		Headers: headers,
		Queries: queries,
		Body:    bodyBytes.Bytes(),
	}

	return httpReq, nil
}

func SendRequest(req *httpclient.Request, exampleID idwrap.IDWrap, client httpclient.HttpClient) (*RequestResponse, error) {
	now := time.Now()
	respHttp, err := httpclient.SendRequestAndConvert(client, req, exampleID)
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
		// INFO: seems like FE update base example insteed of delta for bodytype
		output.Merged.BodyType = input.Base.BodyType
	}

	// Query
	queryMap := make(map[idwrap.IDWrap]mexamplequery.Query, len(input.BaseQueries))
	for _, q := range input.BaseQueries {
		queryMap[q.ID] = q
	}
	for _, q := range input.DeltaQueries {
		queryMap[*q.DeltaParentID] = q
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
		headerMap[*h.DeltaParentID] = h
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
