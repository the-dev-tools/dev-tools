//nolint:revive // exported
package request

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/errmap"
	"the-dev-tools/server/pkg/httpclient"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/varsystem"
	"time"
	"unicode/utf8"

	"connectrpc.com/connect"
)

const (
	HeaderContentEncoding = "Content-Encoding"
	HeaderContentType     = "Content-Type"
	EncodingGzip          = "gzip"
	EncodingZstd          = "zstd"
	EncodingDeflate       = "deflate"
	EncodingIdentity      = "identity"
	EncodingBr            = "br"
	MimeOctetStream       = "application/octet-stream"
	MimeJSON              = "application/json"
	MimeXML               = "application/xml"
	MimeTextPlain         = "text/plain"
	MimeTextHTML          = "text/html"
	MimeFormUrlEncoded    = "application/x-www-form-urlencoded"
)

// PrepareHTTPRequestResult holds the result of preparing a request with tracked variable usage
type PrepareHTTPRequestResult struct {
	Request  *httpclient.Request
	ReadVars map[string]string // Variables that were read during request preparation
}

// PrepareHTTPRequestWithTracking prepares a request using mhttp models and tracks variable usage
func PrepareHTTPRequestWithTracking(
	httpReq mhttp.HTTP,
	headers []mhttp.HTTPHeader,
	params []mhttp.HTTPSearchParam,
	rawBody *mhttp.HTTPBodyRaw,
	formBody []mhttp.HTTPBodyForm,
	urlBody []mhttp.HTTPBodyUrlencoded,
	varMap varsystem.VarMap,
) (*PrepareHTTPRequestResult, error) {
	// Create a tracking wrapper around the varMap
	tracker := varsystem.NewVarMapTracker(varMap)

	var err error
	if varsystem.CheckStringHasAnyVarKey(httpReq.Url) {
		httpReq.Url, err = tracker.ReplaceVars(httpReq.Url)
		if err != nil {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
	}

	// Filter enabled items
	activeHeaders := make([]mhttp.HTTPHeader, 0, len(headers))
	for _, h := range headers {
		if h.Enabled {
			activeHeaders = append(activeHeaders, h)
		}
	}

	activeParams := make([]mhttp.HTTPSearchParam, 0, len(params))
	for _, p := range params {
		if p.Enabled {
			activeParams = append(activeParams, p)
		}
	}

	activeFormBody := make([]mhttp.HTTPBodyForm, 0, len(formBody))
	for _, f := range formBody {
		if f.Enabled {
			activeFormBody = append(activeFormBody, f)
		}
	}

	activeUrlBody := make([]mhttp.HTTPBodyUrlencoded, 0, len(urlBody))
	for _, u := range urlBody {
		if u.Enabled {
			activeUrlBody = append(activeUrlBody, u)
		}
	}

	// Process Query Params
	clientQueries := make([]httpclient.Query, len(activeParams))
	for i, param := range activeParams {
		key := param.Key
		if varsystem.CheckStringHasAnyVarKey(key) {
			key, err = tracker.ReplaceVars(key)
			if err != nil {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
		}

		val := param.Value
		if varsystem.CheckStringHasAnyVarKey(val) {
			val, err = tracker.ReplaceVars(val)
			if err != nil {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
		}
		clientQueries[i] = httpclient.Query{QueryKey: key, Value: val}
	}

	// Process Headers
	compressType := compress.CompressTypeNone
	clientHeaders := make([]httpclient.Header, len(activeHeaders))
	for i, header := range activeHeaders {
		if header.Key == HeaderContentEncoding {
			switch strings.ToLower(header.Value) {
			case EncodingGzip:
				compressType = compress.CompressTypeGzip
			case EncodingZstd:
				compressType = compress.CompressTypeZstd
			case EncodingBr:
				compressType = compress.CompressTypeBr
			case EncodingDeflate, EncodingIdentity:
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("%s not supported", header.Value))
			default:
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid compression type %s", header.Value))
			}
		}

		key := header.Key
		if varsystem.CheckStringHasAnyVarKey(key) {
			key, err = tracker.ReplaceVars(key)
			if err != nil {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
		}

		val := header.Value
		if varsystem.CheckStringHasAnyVarKey(val) {
			val, err = tracker.ReplaceVars(val)
			if err != nil {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
		}
		clientHeaders[i] = httpclient.Header{HeaderKey: key, Value: val}
	}

	bodyBytes := &bytes.Buffer{}

	switch httpReq.BodyKind {
	case mhttp.HttpBodyKindRaw:
		if rawBody != nil && len(rawBody.RawData) > 0 {
			data := rawBody.RawData
			if rawBody.CompressionType != int8(compress.CompressTypeNone) {
				data, err = compress.Decompress(data, compress.CompressType(rawBody.CompressionType))
				if err != nil {
					return nil, err
				}
			}
			bodyStr := string(data)
			bodyStr, err = tracker.ReplaceVars(bodyStr)
			if err != nil {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			_, err = bodyBytes.WriteString(bodyStr)
			if err != nil {
				return nil, err
			}

			// Auto-detect Content-Type if not already set in headers
			if !hasContentTypeHeader(clientHeaders) {
				if detectedType := detectContentType([]byte(bodyStr)); detectedType != "" {
					clientHeaders = append(clientHeaders, httpclient.Header{
						HeaderKey: HeaderContentType,
						Value:     detectedType,
					})
				}
			}
		}
	case mhttp.HttpBodyKindFormData:
		writer := multipart.NewWriter(bodyBytes)

		// Add Content-Type header with multipart boundary
		contentTypeHeader := httpclient.Header{
			HeaderKey: "Content-Type",
			Value:     writer.FormDataContentType(),
		}
		clientHeaders = append(clientHeaders, contentTypeHeader)

		for _, v := range activeFormBody {
			actualBodyKey := v.Key
			if varsystem.CheckStringHasAnyVarKey(v.Key) {
				actualBodyKey, err = tracker.ReplaceVars(v.Key)
				if err != nil {
					return nil, connect.NewError(connect.CodeNotFound, err)
				}
			}

			// First check if this value contains file references (before variable replacement)
			filePathsToUpload := []string{}
			potentialFileRefs := strings.Split(v.Value, ",")
			allAreFileReferences := true

		Loop1:
			for _, ref := range potentialFileRefs {
				trimmedRef := strings.TrimSpace(ref)
				// Check if this is a variable containing a file reference
				switch {
				case varsystem.CheckIsVar(trimmedRef):
					key := strings.TrimSpace(varsystem.GetVarKeyFromRaw(trimmedRef))
					if varsystem.IsFileReference(key) {
						// This is {{#file:path}} format
						filePathsToUpload = append(filePathsToUpload, varsystem.GetIsFileReferencePath(key))
						// Track the file reference read
						fileKey := strings.TrimSpace(key)
						tracker.ReadVars[fileKey], _ = varsystem.ReadFileContentAsString(fileKey)
					} else {
						// This is a regular variable, try to resolve it
						if val, ok := tracker.Get(key); ok {
							if varsystem.IsFileReference(val.Value) {
								fileKey := strings.TrimSpace(val.Value)
								filePathsToUpload = append(filePathsToUpload, varsystem.GetIsFileReferencePath(fileKey))
							} else {
								allAreFileReferences = false
								break Loop1
							}
						} else {
							allAreFileReferences = false
							break Loop1
						}
					}
				case varsystem.IsFileReference(trimmedRef):
					// This is direct #file:path format
					filePathsToUpload = append(filePathsToUpload, varsystem.GetIsFileReferencePath(trimmedRef))
					// Track the file reference read
					fileKey := strings.TrimSpace(trimmedRef)
					tracker.ReadVars[fileKey], _ = varsystem.ReadFileContentAsString(fileKey)
				default:
					allAreFileReferences = false
					break Loop1
				}
			}

			resolvedValue := v.Value
			if !allAreFileReferences && varsystem.CheckStringHasAnyVarKey(v.Value) {
				// Only replace variables if this is not a file reference
				resolvedValue, err = tracker.ReplaceVars(v.Value)
				if err != nil {
					return nil, connect.NewError(connect.CodeNotFound, err)
				}
			}

			if allAreFileReferences && len(filePathsToUpload) > 0 {
				// This is a file upload (single or multiple)
				for _, filePath := range filePathsToUpload {
					fileContentBytes, err := os.ReadFile(filepath.Clean(filePath))
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
						mimeType = MimeOctetStream
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
	case mhttp.HttpBodyKindUrlEncoded:
		urlVal := url.Values{}
		for _, u := range activeUrlBody {
			bodyKey := u.Key
			if varsystem.CheckStringHasAnyVarKey(bodyKey) {
				bodyKey, err = tracker.ReplaceVars(bodyKey)
				if err != nil {
					return nil, connect.NewError(connect.CodeNotFound, err)
				}
			}
			bodyValue := u.Value
			if varsystem.CheckStringHasAnyVarKey(bodyValue) {
				bodyValue, err = tracker.ReplaceVars(bodyValue)
				if err != nil {
					return nil, connect.NewError(connect.CodeNotFound, err)
				}
			}

			urlVal.Add(bodyKey, bodyValue)
		}
		encodedData := urlVal.Encode()
		_, err = bodyBytes.WriteString(encodedData)
		if err != nil {
			return nil, err
		}

		// Add Content-Type if not present
		hasContentType := false
		for _, h := range clientHeaders {
			if strings.EqualFold(h.HeaderKey, "Content-Type") {
				hasContentType = true
				break
			}
		}
		if !hasContentType {
			clientHeaders = append(clientHeaders, httpclient.Header{
				HeaderKey: HeaderContentType,
				Value:     MimeFormUrlEncoded,
			})
		}
	}

	if compressType != compress.CompressTypeNone {
		compressedData, err := compress.Compress(bodyBytes.Bytes(), compressType)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		bodyBytes = bytes.NewBuffer(compressedData)
	}

	httpReqObj := &httpclient.Request{
		Method:  httpReq.Method,
		URL:     httpReq.Url,
		Headers: clientHeaders,
		Queries: clientQueries,
		Body:    bodyBytes.Bytes(),
	}

	return &PrepareHTTPRequestResult{
		Request:  httpReqObj,
		ReadVars: tracker.GetReadVars(),
	}, nil
}

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

func sanitizeHeadersForLog(headers []httpclient.Header) []map[string]string {
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

func formatQueriesForLog(queries []httpclient.Query) []map[string]string {
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

func validateHeadersForHTTP(headers []mhttp.HTTPHeader) error {
	for _, header := range headers {
		if header.Key == "" && header.Value == "" {
			continue
		}
		if hasInvalidHeaderCharacters(header.Key, false) {
			return fmt.Errorf("header %q can only contain visible ASCII characters", header.Key)
		}
		if hasInvalidHeaderCharacters(header.Value, true) {
			return fmt.Errorf("header %q cannot include line breaks or other control characters; trim file contents or encode them before use", header.Key)
		}
	}
	return nil
}

func hasInvalidHeaderCharacters(input string, allowTab bool) bool {
	for i := 0; i < len(input); i++ {
		b := input[i]
		switch b {
		case '\r', '\n':
			return true
		case '\t':
			if allowTab {
				continue
			}
			return true
		}
		if b < 0x20 || b == 0x7f {
			return true
		}
	}
	return false
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

// detectContentType attempts to automatically detect the content type of raw body data.
// Returns the detected content type string (e.g., "application/json", "text/xml").
// If detection fails or data is empty, returns empty string to indicate no auto-detection.
func detectContentType(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	// Trim leading whitespace to find the first meaningful character
	trimmed := bytes.TrimLeft(data, " \t\n\r")
	if len(trimmed) == 0 {
		return MimeTextPlain
	}

	firstChar := trimmed[0]

	// Check for JSON: starts with { or [
	if firstChar == '{' || firstChar == '[' {
		// Validate it's actually JSON by attempting a partial parse
		var js any
		if json.Unmarshal(data, &js) == nil {
			return MimeJSON
		}
	}

	// Check for XML: starts with <?xml or <! or just <tag>
	if firstChar == '<' {
		lower := strings.ToLower(string(trimmed))
		if strings.HasPrefix(lower, "<?xml") {
			return MimeXML
		}
		if strings.HasPrefix(lower, "<!doctype html") || strings.HasPrefix(lower, "<html") {
			return MimeTextHTML
		}
		// Generic XML detection: starts with < followed by valid tag characters
		if len(trimmed) > 1 && ((trimmed[1] >= 'a' && trimmed[1] <= 'z') || (trimmed[1] >= 'A' && trimmed[1] <= 'Z') || trimmed[1] == '!' || trimmed[1] == '?') {
			return MimeXML
		}
	}

	// Check if it's valid UTF-8 text
	if utf8.Valid(data) {
		return MimeTextPlain
	}

	// Binary data
	return MimeOctetStream
}

// hasContentTypeHeader checks if a Content-Type header is already present
func hasContentTypeHeader(headers []httpclient.Header) bool {
	for _, h := range headers {
		if strings.EqualFold(h.HeaderKey, HeaderContentType) {
			return true
		}
	}
	return false
}

// PrepareRequestResult holds the result of preparing a request with tracked variable usage
type PrepareRequestResult struct {
	Request  *httpclient.Request
	ReadVars map[string]string // Variables that were read during request preparation
}

func PrepareRequest(endpoint mhttp.HTTP, example mhttp.HTTP, queries []mhttp.HTTPSearchParam, headers []mhttp.HTTPHeader,
	rawBody mhttp.HTTPBodyRaw, formBody []mhttp.HTTPBodyForm, urlBody []mhttp.HTTPBodyUrlencoded, varMap varsystem.VarMap,
) (*httpclient.Request, error) {
	var err error
	if varsystem.CheckStringHasAnyVarKey(endpoint.Url) {
		endpoint.Url, err = varMap.ReplaceVars(endpoint.Url)
		if err != nil {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
	}

	// get only enabled
	// Filter enabled items manually since mhttp models don't implement IsEnabled()
	activeHeaders := make([]mhttp.HTTPHeader, 0, len(headers))
	for _, h := range headers {
		if h.Enabled {
			activeHeaders = append(activeHeaders, h)
		}
	}
	headers = activeHeaders

	activeQueries := make([]mhttp.HTTPSearchParam, 0, len(queries))
	for _, q := range queries {
		if q.Enabled {
			activeQueries = append(activeQueries, q)
		}
	}
	queries = activeQueries

	activeFormBody := make([]mhttp.HTTPBodyForm, 0, len(formBody))
	for _, f := range formBody {
		if f.Enabled {
			activeFormBody = append(activeFormBody, f)
		}
	}
	formBody = activeFormBody

	activeUrlBody := make([]mhttp.HTTPBodyUrlencoded, 0, len(urlBody))
	for _, u := range urlBody {
		if u.Enabled {
			activeUrlBody = append(activeUrlBody, u)
		}
	}
	urlBody = activeUrlBody

	clientQueries := make([]httpclient.Query, len(queries))
	if varMap != nil {
		for i, query := range queries {
			if varsystem.CheckIsVar(query.Key) {
				key := varsystem.GetVarKeyFromRaw(query.Key)
				if val, ok := varMap.Get(key); ok {
					query.Key = val.Value
				} else {
					return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("%s named variable not found", key))
				}
			}

			if varsystem.CheckIsVar(query.Value) {
				key := varsystem.GetVarKeyFromRaw(query.Value)
				if val, ok := varMap.Get(key); ok {
					query.Value = val.Value
				} else {
					return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("%s named variable not found", key))
				}
			}
			clientQueries[i] = httpclient.Query{QueryKey: query.Key, Value: query.Value}
		}
	} else {
		for i, query := range queries {
			clientQueries[i] = httpclient.Query{QueryKey: query.Key, Value: query.Value}
		}
	}

	compressType := compress.CompressTypeNone
	clientHeaders := make([]httpclient.Header, len(headers))
	for i, header := range headers {
		if header.Key == "Content-Encoding" {
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
			if varsystem.CheckIsVar(header.Key) {
				key := varsystem.GetVarKeyFromRaw(header.Key)
				if val, ok := varMap.Get(key); ok {
					header.Key = val.Value
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
				header.Value = replacedValue
			}
		}
		clientHeaders[i] = httpclient.Header{HeaderKey: header.Key, Value: header.Value}
	}

	bodyBytes := &bytes.Buffer{}
	switch example.BodyKind {
	case mhttp.HttpBodyKindRaw:
		if len(rawBody.RawData) > 0 {
			if rawBody.CompressionType != int8(compress.CompressTypeNone) {
				rawBody.RawData, err = compress.Decompress(rawBody.RawData, compress.CompressType(rawBody.CompressionType))
				if err != nil {
					return nil, err
				}
			}
			bodyStr := string(rawBody.RawData)
			bodyStr, err = varMap.ReplaceVars(bodyStr)
			if err != nil {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			rawBody.RawData = []byte(bodyStr)

			// Auto-detect Content-Type if not already set in headers
			if !hasContentTypeHeader(clientHeaders) {
				if detectedType := detectContentType(rawBody.RawData); detectedType != "" {
					clientHeaders = append(clientHeaders, httpclient.Header{
						HeaderKey: "Content-Type",
						Value:     detectedType,
					})
				}
			}
		}
		_, err = bodyBytes.Write(rawBody.RawData)
		if err != nil {
			return nil, err
		}
	case mhttp.HttpBodyKindFormData:
		writer := multipart.NewWriter(bodyBytes)

		// Add Content-Type header with multipart boundary
		contentTypeHeader := httpclient.Header{
			HeaderKey: "Content-Type",
			Value:     writer.FormDataContentType(),
		}
		clientHeaders = append(clientHeaders, contentTypeHeader)

		for _, v := range formBody {
			actualBodyKey := v.Key
			if varsystem.CheckIsVar(v.Key) {
				key := varsystem.GetVarKeyFromRaw(v.Key)
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

		Loop2:
			for _, ref := range potentialFileRefs {
				trimmedRef := strings.TrimSpace(ref)
				// Check if this is a variable containing a file reference
				switch {
				case varsystem.CheckIsVar(trimmedRef):
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
								break Loop2
							}
						} else {
							allAreFileReferences = false
							break Loop2
						}
					}
				case varsystem.IsFileReference(trimmedRef):
					// This is direct #file:path format
					filePathsToUpload = append(filePathsToUpload, varsystem.GetIsFileReferencePath(trimmedRef))
				default:
					allAreFileReferences = false
					break Loop2
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
					fileContentBytes, err := os.ReadFile(filepath.Clean(filePath))
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
	case mhttp.HttpBodyKindUrlEncoded:
		urlVal := url.Values{}
		for _, url := range urlBody {
			if varsystem.CheckIsVar(url.Key) {
				key := varsystem.GetVarKeyFromRaw(url.Value)
				if val, ok := varMap.Get(key); ok {
					url.Key = val.Value
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

			urlVal.Add(url.Key, url.Value)
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

	if err := validateHeadersForHTTP(headers); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	httpReq := &httpclient.Request{
		Method:  endpoint.Method,
		URL:     endpoint.Url,
		Headers: clientHeaders,
		Queries: clientQueries,
		Body:    bodyBytes.Bytes(),
	}

	return httpReq, nil
}

// PrepareRequestWithTracking prepares a request and tracks which variables are read
func PrepareRequestWithTracking(endpoint mhttp.HTTP, example mhttp.HTTP, queries []mhttp.HTTPSearchParam, headers []mhttp.HTTPHeader,
	rawBody mhttp.HTTPBodyRaw, formBody []mhttp.HTTPBodyForm, urlBody []mhttp.HTTPBodyUrlencoded, varMap varsystem.VarMap,
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
	// Filter enabled items manually since mhttp models don't implement IsEnabled()
	activeHeaders := make([]mhttp.HTTPHeader, 0, len(headers))
	for _, h := range headers {
		if h.Enabled {
			activeHeaders = append(activeHeaders, h)
		}
	}
	headers = activeHeaders

	activeQueries := make([]mhttp.HTTPSearchParam, 0, len(queries))
	for _, q := range queries {
		if q.Enabled {
			activeQueries = append(activeQueries, q)
		}
	}
	queries = activeQueries

	activeFormBody := make([]mhttp.HTTPBodyForm, 0, len(formBody))
	for _, f := range formBody {
		if f.Enabled {
			activeFormBody = append(activeFormBody, f)
		}
	}
	formBody = activeFormBody

	activeUrlBody := make([]mhttp.HTTPBodyUrlencoded, 0, len(urlBody))
	for _, u := range urlBody {
		if u.Enabled {
			activeUrlBody = append(activeUrlBody, u)
		}
	}
	urlBody = activeUrlBody

	clientQueries := make([]httpclient.Query, len(queries))
	if varMap != nil {
		for i, query := range queries {
			if varsystem.CheckStringHasAnyVarKey(query.Key) {
				resolvedKey, err := tracker.ReplaceVars(query.Key)
				if err != nil {
					return nil, connect.NewError(connect.CodeNotFound, err)
				}
				query.Key = resolvedKey
			}

			if varsystem.CheckStringHasAnyVarKey(query.Value) {
				resolvedValue, err := tracker.ReplaceVars(query.Value)
				if err != nil {
					return nil, connect.NewError(connect.CodeNotFound, err)
				}
				query.Value = resolvedValue
			}
			clientQueries[i] = httpclient.Query{QueryKey: query.Key, Value: query.Value}
		}
	} else {
		for i, query := range queries {
			clientQueries[i] = httpclient.Query{QueryKey: query.Key, Value: query.Value}
		}
	}

	compressType := compress.CompressTypeNone
	clientHeaders := make([]httpclient.Header, len(headers))
	for i, header := range headers {
		if header.Key == "Content-Encoding" {
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
			if varsystem.CheckStringHasAnyVarKey(header.Key) {
				resolvedKey, err := tracker.ReplaceVars(header.Key)
				if err != nil {
					return nil, connect.NewError(connect.CodeNotFound, err)
				}
				header.Key = resolvedKey
			}

			if varsystem.CheckStringHasAnyVarKey(header.Value) {
				// Use tracking wrapper's ReplaceVars for any string containing variables
				replacedValue, err := tracker.ReplaceVars(header.Value)
				if err != nil {
					return nil, connect.NewError(connect.CodeNotFound, err)
				}
				header.Value = replacedValue
			}
		}
		clientHeaders[i] = httpclient.Header{HeaderKey: header.Key, Value: header.Value}
	}

	bodyBytes := &bytes.Buffer{}
	switch example.BodyKind {
	case mhttp.HttpBodyKindRaw:
		if len(rawBody.RawData) > 0 {
			if rawBody.CompressionType != int8(compress.CompressTypeNone) {
				rawBody.RawData, err = compress.Decompress(rawBody.RawData, compress.CompressType(rawBody.CompressionType))
				if err != nil {
					return nil, err
				}
			}
			bodyStr := string(rawBody.RawData)
			bodyStr, err = tracker.ReplaceVars(bodyStr)
			if err != nil {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			rawBody.RawData = []byte(bodyStr)

			// Auto-detect Content-Type if not already set in headers
			if !hasContentTypeHeader(clientHeaders) {
				if detectedType := detectContentType(rawBody.RawData); detectedType != "" {
					clientHeaders = append(clientHeaders, httpclient.Header{
						HeaderKey: "Content-Type",
						Value:     detectedType,
					})
				}
			}
		}
		_, err = bodyBytes.Write(rawBody.RawData)
		if err != nil {
			return nil, err
		}
	case mhttp.HttpBodyKindFormData:
		writer := multipart.NewWriter(bodyBytes)

		// Add Content-Type header with multipart boundary
		contentTypeHeader := httpclient.Header{
			HeaderKey: "Content-Type",
			Value:     writer.FormDataContentType(),
		}
		clientHeaders = append(clientHeaders, contentTypeHeader)

		for _, v := range formBody {
			actualBodyKey := v.Key
			if varsystem.CheckStringHasAnyVarKey(v.Key) {
				resolvedKey, err := tracker.ReplaceVars(v.Key)
				if err != nil {
					return nil, connect.NewError(connect.CodeNotFound, err)
				}
				actualBodyKey = resolvedKey
			}

			// First check if this value contains file references (before variable replacement)
			filePathsToUpload := []string{}
			potentialFileRefs := strings.Split(v.Value, ",")
			allAreFileReferences := true

		Loop3:
			for _, ref := range potentialFileRefs {
				trimmedRef := strings.TrimSpace(ref)
				// Check if this is a variable containing a file reference
				switch {
				case varsystem.CheckIsVar(trimmedRef):
					key := strings.TrimSpace(varsystem.GetVarKeyFromRaw(trimmedRef))
					if varsystem.IsFileReference(key) {
						// This is {{#file:path}} format
						filePathsToUpload = append(filePathsToUpload, varsystem.GetIsFileReferencePath(key))
						// Track the file reference read
						fileKey := strings.TrimSpace(key)
						tracker.ReadVars[fileKey], _ = varsystem.ReadFileContentAsString(fileKey)
					} else {
						// This is a regular variable, try to resolve it
						if val, ok := tracker.Get(key); ok {
							if varsystem.IsFileReference(val.Value) {
								fileKey := strings.TrimSpace(val.Value)
								filePathsToUpload = append(filePathsToUpload, varsystem.GetIsFileReferencePath(fileKey))
							} else {
								allAreFileReferences = false
								break Loop3
							}
						} else {
							allAreFileReferences = false
							break Loop3
						}
					}
				case varsystem.IsFileReference(trimmedRef):
					// This is direct #file:path format
					filePathsToUpload = append(filePathsToUpload, varsystem.GetIsFileReferencePath(trimmedRef))
					// Track the file reference read
					fileKey := strings.TrimSpace(trimmedRef)
					tracker.ReadVars[fileKey], _ = varsystem.ReadFileContentAsString(fileKey)
				default:
					allAreFileReferences = false
					break Loop3
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
					fileContentBytes, err := os.ReadFile(filepath.Clean(filePath))
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
	case mhttp.HttpBodyKindUrlEncoded:
		urlVal := url.Values{}
		for _, url := range urlBody {
			bodyKey := url.Key
			if varsystem.CheckStringHasAnyVarKey(bodyKey) {
				resolvedKey, err := tracker.ReplaceVars(bodyKey)
				if err != nil {
					return nil, connect.NewError(connect.CodeNotFound, err)
				}
				bodyKey = resolvedKey
			}
			bodyValue := url.Value
			if varsystem.CheckStringHasAnyVarKey(bodyValue) {
				resolvedValue, err := tracker.ReplaceVars(bodyValue)
				if err != nil {
					return nil, connect.NewError(connect.CodeNotFound, err)
				}
				bodyValue = resolvedValue
			}

			urlVal.Add(bodyKey, bodyValue)
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

	if err := validateHeadersForHTTP(headers); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	httpReq := &httpclient.Request{
		Method:  endpoint.Method,
		URL:     endpoint.Url,
		Headers: clientHeaders,
		Queries: clientQueries,
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
	Base, Delta               mhttp.HTTP
	BaseQueries, DeltaQueries []mhttp.HTTPSearchParam
	BaseHeaders, DeltaHeaders []mhttp.HTTPHeader

	// Bodies
	BaseRawBody, DeltaRawBody               mhttp.HTTPBodyRaw
	BaseFormBody, DeltaFormBody             []mhttp.HTTPBodyForm
	BaseUrlEncodedBody, DeltaUrlEncodedBody []mhttp.HTTPBodyUrlencoded
	BaseAsserts, DeltaAsserts               []mhttp.HTTPAssert
}

type MergeExamplesOutput struct {
	Merged              mhttp.HTTP
	MergeQueries        []mhttp.HTTPSearchParam
	MergeHeaders        []mhttp.HTTPHeader
	MergeRawBody        mhttp.HTTPBodyRaw
	MergeFormBody       []mhttp.HTTPBodyForm
	MergeUrlEncodedBody []mhttp.HTTPBodyUrlencoded
	MergeAsserts        []mhttp.HTTPAssert
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
		output.Merged.BodyKind = input.Base.BodyKind
	}

	// Query
	queryMap := make(map[idwrap.IDWrap]mhttp.HTTPSearchParam, len(input.BaseQueries))
	for _, q := range input.BaseQueries {
		queryMap[q.ID] = q
	}

	// Create a map for matching base queries by key name (for legacy delta queries)
	baseQueryByKey := make(map[string]mhttp.HTTPSearchParam)
	for _, q := range input.BaseQueries {
		baseQueryByKey[q.Key] = q
	}

	for _, q := range input.DeltaQueries {
		// Handle delta queries with parent relationships
		if q.ParentHttpSearchParamID != nil {
			queryMap[*q.ParentHttpSearchParamID] = q
		} else {
			// For delta queries without parent ID, try to find matching base query by key name
			if baseQuery, exists := baseQueryByKey[q.Key]; exists {
				queryMap[baseQuery.ID] = q
			} else {
				// If no matching base query found, add as new query
				queryMap[q.ID] = q
			}
		}
	}

	output.MergeQueries = make([]mhttp.HTTPSearchParam, 0, len(queryMap))
	for _, q := range queryMap {
		output.MergeQueries = append(output.MergeQueries, q)
	}

	// Header
	headerMap := make(map[idwrap.IDWrap]mhttp.HTTPHeader, len(input.BaseHeaders))
	for _, h := range input.BaseHeaders {
		headerMap[h.ID] = h
	}

	// Create a map for matching base headers by key name (for legacy delta headers)
	baseHeaderByKey := make(map[string]mhttp.HTTPHeader)
	for _, h := range input.BaseHeaders {
		baseHeaderByKey[h.Key] = h
	}

	for _, h := range input.DeltaHeaders {
		// Handle delta headers with parent relationships
		if h.ParentHttpHeaderID != nil {
			headerMap[*h.ParentHttpHeaderID] = h
		} else {
			// For delta headers without parent ID, try to find matching base header by key name
			if baseHeader, exists := baseHeaderByKey[h.Key]; exists {
				headerMap[baseHeader.ID] = h
			} else {
				// If no matching base header found, add as new header
				headerMap[h.ID] = h
			}
		}
	}

	output.MergeHeaders = make([]mhttp.HTTPHeader, 0, len(headerMap))
	for _, h := range headerMap {
		output.MergeHeaders = append(output.MergeHeaders, h)
	}

	// Raw Body
	if len(input.DeltaRawBody.RawData) > 0 {
		output.MergeRawBody = input.DeltaRawBody
	} else {
		output.MergeRawBody = input.BaseRawBody
	}

	// Form Body
	formMap := make(map[idwrap.IDWrap]mhttp.HTTPBodyForm, len(input.BaseFormBody))
	for _, f := range input.BaseFormBody {
		formMap[f.ID] = f
	}

	for _, f := range input.DeltaFormBody {
		formMap[f.ID] = f
	}

	output.MergeFormBody = make([]mhttp.HTTPBodyForm, 0, len(formMap))
	for _, f := range formMap {
		output.MergeFormBody = append(output.MergeFormBody, f)
	}

	// Url Encoded Body
	urlEncodedMap := make(map[idwrap.IDWrap]mhttp.HTTPBodyUrlencoded, len(input.BaseUrlEncodedBody))
	for _, f := range input.BaseUrlEncodedBody {
		urlEncodedMap[f.ID] = f
	}

	for _, f := range input.DeltaUrlEncodedBody {
		urlEncodedMap[f.ID] = f
	}

	output.MergeUrlEncodedBody = make([]mhttp.HTTPBodyUrlencoded, 0, len(urlEncodedMap))
	for _, f := range urlEncodedMap {
		output.MergeUrlEncodedBody = append(output.MergeUrlEncodedBody, f)
	}

	output.MergeAsserts = mergeAsserts(input.BaseAsserts, input.DeltaAsserts)

	return output
}

func mergeAsserts(baseAsserts, deltaAsserts []mhttp.HTTPAssert) []mhttp.HTTPAssert {
	orderedBase := orderAsserts(baseAsserts)
	if len(deltaAsserts) == 0 {
		return orderedBase
	}

	orderedDelta := orderAsserts(deltaAsserts)
	baseMap := make(map[idwrap.IDWrap]mhttp.HTTPAssert, len(orderedBase))
	baseOrder := make([]idwrap.IDWrap, 0, len(orderedBase))

	for _, assert := range orderedBase {
		baseMap[assert.ID] = assert
		baseOrder = append(baseOrder, assert.ID)
	}

	additions := make([]mhttp.HTTPAssert, 0)
	for _, deltaAssert := range orderedDelta {
		if deltaAssert.ParentHttpAssertID != nil {
			if _, exists := baseMap[*deltaAssert.ParentHttpAssertID]; exists {
				baseMap[*deltaAssert.ParentHttpAssertID] = deltaAssert
				continue
			}
		}
		additions = append(additions, deltaAssert)
	}

	merged := make([]mhttp.HTTPAssert, 0, len(baseMap)+len(additions))
	for _, id := range baseOrder {
		if assert, exists := baseMap[id]; exists {
			merged = append(merged, assert)
		}
	}

	if len(additions) > 0 {
		merged = append(merged, orderAsserts(additions)...)
	}

	return merged
}

func orderAsserts(asserts []mhttp.HTTPAssert) []mhttp.HTTPAssert {
	if len(asserts) <= 1 {
		return append([]mhttp.HTTPAssert(nil), asserts...)
	}

	// Create a copy and sort by Order field
	ordered := make([]mhttp.HTTPAssert, len(asserts))
	copy(ordered, asserts)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].Order < ordered[j].Order
	})

	return ordered
}
