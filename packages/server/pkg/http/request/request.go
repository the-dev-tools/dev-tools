package request

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/errmap"
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
	"unicode/utf8"

	"connectrpc.com/connect"
)

type RequestResponseVar struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
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
	return RequestResponseVar{
		Method:  r.Method,
		URL:     r.URL,
		Headers: headersMaps,
		Queries: queriesMaps,
		Body:    string(r.Body),
	}
}

const logBodyLimit = 2048

func sanitizeHeadersForLog(headers []mexampleheader.Header) []map[string]string {
	if len(headers) == 0 {
		return nil
	}
	result := make([]map[string]string, 0, len(headers))
	for _, header := range headers {
		value := header.Value
		if strings.EqualFold(header.HeaderKey, "Authorization") {
			value = "[REDACTED]"
		}
		result = append(result, map[string]string{
			"key":   header.HeaderKey,
			"value": value,
		})
	}
	return result
}

func formatQueriesForLog(queries []mexamplequery.Query) []map[string]string {
	if len(queries) == 0 {
		return nil
	}
	result := make([]map[string]string, 0, len(queries))
	for _, query := range queries {
		result = append(result, map[string]string{
			"key":   query.QueryKey,
			"value": query.Value,
		})
	}
	return result
}

func formatBodyForLog(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	if !utf8.Valid(body) {
		encoded := base64.StdEncoding.EncodeToString(body)
		if len(encoded) > logBodyLimit {
			return "[base64]" + encoded[:logBodyLimit] + "...(truncated)"
		}
		return "[base64]" + encoded
	}
	text := string(body)
	if len(text) > logBodyLimit {
		return text[:logBodyLimit] + "...(truncated)"
	}
	return text
}

func LogPreparedRequest(ctx context.Context, logger *slog.Logger, executionID, nodeID idwrap.IDWrap, nodeName string, prepared *httpclient.Request) {
	if logger == nil || prepared == nil {
		return
	}
	logger.InfoContext(ctx, "Dispatching HTTP request",
		"execution_id", executionID.String(),
		"node_id", nodeID.String(),
		"node_name", nodeName,
		"method", prepared.Method,
		"url", prepared.URL,
		"queries", formatQueriesForLog(prepared.Queries),
		"headers", sanitizeHeadersForLog(prepared.Headers),
		"body", formatBodyForLog(prepared.Body),
	)
}

// quoteEscaper is used to escape quotes in MIME headers.
var quoteEscaper = strings.NewReplacer("\\", "\\\\", "\"", "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}

// PrepareRequestResult holds the result of preparing a request with tracked variable usage
type PrepareRequestResult struct {
	Request  *httpclient.Request
	ReadVars map[string]string // Variables that were read during request preparation
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

			if varsystem.CheckStringHasAnyVarKey(header.Value) {
				// Use varsystem's ReplaceVars for any string containing variables
				replacedValue, err := varMap.ReplaceVars(header.Value)
				if err != nil {
					return nil, connect.NewError(connect.CodeNotFound, err)
				}
				headers[i].Value = replacedValue
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

		// Add Content-Type header with multipart boundary
		contentTypeHeader := mexampleheader.Header{
			HeaderKey: "Content-Type",
			Value:     writer.FormDataContentType(),
			Enable:    true,
		}
		headers = append(headers, contentTypeHeader)

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

			// First check if this value contains file references (before variable replacement)
			filePathsToUpload := []string{}
			potentialFileRefs := strings.Split(v.Value, ",")
			allAreFileReferences := true

			for _, ref := range potentialFileRefs {
				trimmedRef := strings.TrimSpace(ref)
				// Check if this is a variable containing a file reference
				if varsystem.CheckIsVar(trimmedRef) {
					key := varsystem.GetVarKeyFromRaw(trimmedRef)
					if varsystem.IsFileReference(key) {
						// This is {{#file:path}} format
						filePathsToUpload = append(filePathsToUpload, varsystem.GetIsFileReferencePath(key))
					} else {
						// This is a regular variable, try to resolve it
						if val, ok := varMap.Get(key); ok {
							if varsystem.IsFileReference(val.Value) {
								filePathsToUpload = append(filePathsToUpload, varsystem.GetIsFileReferencePath(val.Value))
							} else {
								allAreFileReferences = false
								break
							}
						} else {
							allAreFileReferences = false
							break
						}
					}
				} else if varsystem.IsFileReference(trimmedRef) {
					// This is direct #file:path format
					filePathsToUpload = append(filePathsToUpload, varsystem.GetIsFileReferencePath(trimmedRef))
				} else {
					allAreFileReferences = false
					break
				}
			}

			resolvedValue := v.Value
			if !allAreFileReferences && varsystem.CheckStringHasAnyVarKey(v.Value) {
				// Only replace variables if this is not a file reference
				var err error
				resolvedValue, err = varMap.ReplaceVars(v.Value)
				if err != nil {
					return nil, connect.NewError(connect.CodeNotFound, err)
				}
			}

			if allAreFileReferences && len(filePathsToUpload) > 0 {
				// This is a file upload (single or multiple)
				for _, filePath := range filePathsToUpload {
					fileContentBytes, err := os.ReadFile(filePath)
					if err != nil {
						return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to read file %s: %w", filePath, err))
					}

					fileName := filepath.Base(filePath)

					h := make(textproto.MIMEHeader)
					h.Set("Content-Disposition",
						fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
							escapeQuotes(actualBodyKey), escapeQuotes(fileName)))

					mimeType := mime.TypeByExtension(filepath.Ext(fileName))
					if mimeType == "" {
						mimeType = "application/octet-stream"
					}
					h.Set("Content-Type", mimeType)

					partWriter, err := writer.CreatePart(h)
					if err != nil {
						return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create form part: %w", err))
					}

					if _, err = partWriter.Write(fileContentBytes); err != nil {
						return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to write file content: %w", err))
					}
				}
			} else {
				// This is a regular text field
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

// PrepareRequestWithTracking prepares a request and tracks which variables are read
func PrepareRequestWithTracking(endpoint mitemapi.ItemApi, example mitemapiexample.ItemApiExample, queries []mexamplequery.Query, headers []mexampleheader.Header,
	rawBody mbodyraw.ExampleBodyRaw, formBody []mbodyform.BodyForm, urlBody []mbodyurl.BodyURLEncoded, varMap varsystem.VarMap,
) (*PrepareRequestResult, error) {
	// Create a tracking wrapper around the varMap
	tracker := varsystem.NewVarMapTracker(varMap)

	var err error
	if varsystem.CheckStringHasAnyVarKey(endpoint.Url) {
		endpoint.Url, err = tracker.ReplaceVars(endpoint.Url)
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
				if val, ok := tracker.Get(key); ok {
					queries[i].QueryKey = val.Value
				} else {
					return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("%s named variable not found", key))
				}
			}

			if varsystem.CheckIsVar(query.Value) {
				key := varsystem.GetVarKeyFromRaw(query.Value)
				if val, ok := tracker.Get(key); ok {
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
				if val, ok := tracker.Get(key); ok {
					headers[i].HeaderKey = val.Value
				} else {
					return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("%s named variable not found", key))
				}
			}

			if varsystem.CheckStringHasAnyVarKey(header.Value) {
				// Use tracking wrapper's ReplaceVars for any string containing variables
				replacedValue, err := tracker.ReplaceVars(header.Value)
				if err != nil {
					return nil, connect.NewError(connect.CodeNotFound, err)
				}
				headers[i].Value = replacedValue
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
			bodyStr, err = tracker.ReplaceVars(bodyStr)
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

		// Add Content-Type header with multipart boundary
		contentTypeHeader := mexampleheader.Header{
			HeaderKey: "Content-Type",
			Value:     writer.FormDataContentType(),
			Enable:    true,
		}
		headers = append(headers, contentTypeHeader)

		for _, v := range formBody {
			actualBodyKey := v.BodyKey
			if varsystem.CheckIsVar(v.BodyKey) {
				key := varsystem.GetVarKeyFromRaw(v.BodyKey)
				if val, ok := tracker.Get(key); ok {
					actualBodyKey = val.Value
				} else {
					return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("%s named error not found", key))
				}
			}

			// First check if this value contains file references (before variable replacement)
			filePathsToUpload := []string{}
			potentialFileRefs := strings.Split(v.Value, ",")
			allAreFileReferences := true

			for _, ref := range potentialFileRefs {
				trimmedRef := strings.TrimSpace(ref)
				// Check if this is a variable containing a file reference
				if varsystem.CheckIsVar(trimmedRef) {
					key := varsystem.GetVarKeyFromRaw(trimmedRef)
					if varsystem.IsFileReference(key) {
						// This is {{#file:path}} format
						filePathsToUpload = append(filePathsToUpload, varsystem.GetIsFileReferencePath(key))
						// Track the file reference read
						tracker.ReadVars[key], _ = varsystem.ReadFileContentAsString(key)
					} else {
						// This is a regular variable, try to resolve it
						if val, ok := tracker.Get(key); ok {
							if varsystem.IsFileReference(val.Value) {
								filePathsToUpload = append(filePathsToUpload, varsystem.GetIsFileReferencePath(val.Value))
							} else {
								allAreFileReferences = false
								break
							}
						} else {
							allAreFileReferences = false
							break
						}
					}
				} else if varsystem.IsFileReference(trimmedRef) {
					// This is direct #file:path format
					filePathsToUpload = append(filePathsToUpload, varsystem.GetIsFileReferencePath(trimmedRef))
					// Track the file reference read
					tracker.ReadVars[trimmedRef], _ = varsystem.ReadFileContentAsString(trimmedRef)
				} else {
					allAreFileReferences = false
					break
				}
			}

			resolvedValue := v.Value
			if !allAreFileReferences && varsystem.CheckStringHasAnyVarKey(v.Value) {
				// Only replace variables if this is not a file reference
				var err error
				resolvedValue, err = tracker.ReplaceVars(v.Value)
				if err != nil {
					return nil, connect.NewError(connect.CodeNotFound, err)
				}
			}

			if allAreFileReferences && len(filePathsToUpload) > 0 {
				// This is a file upload (single or multiple)
				for _, filePath := range filePathsToUpload {
					fileContentBytes, err := os.ReadFile(filePath)
					if err != nil {
						return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to read file %s: %w", filePath, err))
					}

					fileName := filepath.Base(filePath)

					h := make(textproto.MIMEHeader)
					h.Set("Content-Disposition",
						fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
							escapeQuotes(actualBodyKey), escapeQuotes(fileName)))

					mimeType := mime.TypeByExtension(filepath.Ext(fileName))
					if mimeType == "" {
						mimeType = "application/octet-stream"
					}
					h.Set("Content-Type", mimeType)

					partWriter, err := writer.CreatePart(h)
					if err != nil {
						return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create form part: %w", err))
					}

					if _, err = partWriter.Write(fileContentBytes); err != nil {
						return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to write file content: %w", err))
					}
				}
			} else {
				// This is a regular text field
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
				if val, ok := tracker.Get(key); ok {
					url.BodyKey = val.Value
				} else {
					return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("%s named error not found", key))
				}
			}
			if varsystem.CheckIsVar(url.Value) {
				key := varsystem.GetVarKeyFromRaw(url.Value)
				if val, ok := tracker.Get(key); ok {
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

	return &PrepareRequestResult{
		Request:  httpReq,
		ReadVars: tracker.GetReadVars(),
	}, nil
}

func SendRequest(req *httpclient.Request, exampleID idwrap.IDWrap, client httpclient.HttpClient) (*RequestResponse, error) {
	now := time.Now()
	respHttp, err := httpclient.SendRequestAndConvert(client, req, exampleID)
	lapse := time.Since(now)
	if err != nil {
		return nil, errmap.MapRequestError(req.Method, req.URL, err)
	}

	return &RequestResponse{HttpResp: respHttp, LapTime: lapse}, nil
}

func SendRequestWithContext(ctx context.Context, req *httpclient.Request, exampleID idwrap.IDWrap, client httpclient.HttpClient) (*RequestResponse, error) {
	now := time.Now()
	respHttp, err := httpclient.SendRequestAndConvertWithContext(ctx, client, req, exampleID)
	lapse := time.Since(now)
	if err != nil {
		// Preserve context cancellation/timeout classification and annotate with request data
		return nil, errmap.MapRequestError(req.Method, req.URL, err)
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

	// Create a map for matching base queries by key name (for legacy delta queries)
	baseQueryByKey := make(map[string]mexamplequery.Query)
	for _, q := range input.BaseQueries {
		baseQueryByKey[q.QueryKey] = q
	}

	for _, q := range input.DeltaQueries {
		// Handle legacy delta queries that don't have DeltaParentID set
		if q.DeltaParentID != nil {
			queryMap[*q.DeltaParentID] = q
		} else {
			// For legacy delta queries without parent ID, try to find matching base query by key name
			if baseQuery, exists := baseQueryByKey[q.QueryKey]; exists {
				queryMap[baseQuery.ID] = q
			} else {
				// If no matching base query found, add as new query
				queryMap[q.ID] = q
			}
		}
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

	// Create a map for matching base headers by key name (for legacy delta headers)
	baseHeaderByKey := make(map[string]mexampleheader.Header)
	for _, h := range input.BaseHeaders {
		baseHeaderByKey[h.HeaderKey] = h
	}

	for _, h := range input.DeltaHeaders {
		// Handle legacy delta headers that don't have DeltaParentID set
		if h.DeltaParentID != nil {
			headerMap[*h.DeltaParentID] = h
		} else {
			// For legacy delta headers without parent ID, try to find matching base header by key name
			if baseHeader, exists := baseHeaderByKey[h.HeaderKey]; exists {
				headerMap[baseHeader.ID] = h
			} else {
				// If no matching base header found, add as new header
				headerMap[h.ID] = h
			}
		}
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
