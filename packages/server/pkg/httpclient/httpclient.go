//nolint:revive // exported
package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/idwrap"
	"time"

	"golang.org/x/net/html/charset"
)

type HttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

const TimeoutRequest = 60 * time.Second

func New() HttpClient {
	return &http.Client{
		Timeout: TimeoutRequest,
	}
}

type Query struct {
	QueryKey string
	Value    string
}

type Header struct {
	HeaderKey string
	Value     string
}

type Request struct {
	Method  string
	URL     string
	Queries []Query
	Headers []Header
	Body    []byte
}

type Response struct {
	StatusCode int      `json:"statusCode"`
	Body       []byte   `json:"body"`
	Headers    []Header `json:"headers"`
}

type ResponseVar struct {
	StatusCode int               `json:"status"`
	Body       any               `json:"body"`
	Headers    map[string]string `json:"headers"`
	Duration   int32             `json:"duration"`
}

func ConvertResponseToVar(r Response) ResponseVar {
	headersMaps := make(map[string]string)
	for _, header := range r.Headers {
		headersMaps[header.HeaderKey] = header.Value
	}

	// check if body seems like json; if so decode it into a map[string]interface{}, otherwise use a string.
	var body any
	if json.Valid(r.Body) {
		var jsonBody any
		decoder := json.NewDecoder(bytes.NewReader(r.Body))
		decoder.UseNumber()
		if err := decoder.Decode(&jsonBody); err == nil {
			body = jsonBody
		} else {
			body = string(r.Body)
		}
	} else {
		body = string(r.Body)
	}

	return ResponseVar{
		StatusCode: r.StatusCode,
		Body:       body,
		Headers:    headersMaps,
	}
}

func SendRequest(client HttpClient, req *Request) (*http.Response, error) {
	return SendRequestWithContext(context.Background(), client, req)
}

func SendRequestWithContext(ctx context.Context, client HttpClient, req *Request) (*http.Response, error) {
	reqRaw, err := http.NewRequestWithContext(ctx, req.Method, req.URL, bytes.NewReader(req.Body))
	if err != nil {
		return nil, err
	}

	qNew := ConvertQueriesToUrl(req.Queries, reqRaw.URL.Query())
	reqRaw.URL.RawQuery = qNew.Encode()
	reqRaw.Header = ConvertHeadersToHttp(req.Headers)
	return client.Do(reqRaw)
}

func SendRequestAndConvert(client HttpClient, req *Request, exampleID idwrap.IDWrap) (Response, error) {
	resp, err := SendRequest(client, req)
	if err != nil {
		return Response{}, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, err
	}

	encoding := strings.ToLower(resp.Header.Get("Content-Encoding"))
	if encoding != "" {
		body, err = compress.DecompressWithContentEncodeStr(body, encoding)
		if err != nil {
			return Response{}, err
		}
	}

	// Convert body to UTF-8 if content-type specifies a charset
	contentType := resp.Header.Get("Content-Type")
	if contentType != "" {
		reader, err := charset.NewReader(bytes.NewReader(body), contentType)
		if err == nil {
			body, err = io.ReadAll(reader)
			if err != nil {
				return Response{}, err
			}
		}
	}

	err = resp.Body.Close()
	if err != nil {
		return Response{}, err
	}
	return Response{
		StatusCode: resp.StatusCode,
		Body:       body,
		Headers:    ConvertHttpHeaderToHeaders(resp.Header),
	}, nil
}

func SendRequestAndConvertWithContext(ctx context.Context, client HttpClient, req *Request, exampleID idwrap.IDWrap) (Response, error) {
	resp, err := SendRequestWithContext(ctx, client, req)
	if err != nil {
		return Response{}, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, err
	}

	encoding := strings.ToLower(resp.Header.Get("Content-Encoding"))
	if encoding != "" {
		body, err = compress.DecompressWithContentEncodeStr(body, encoding)
		if err != nil {
			return Response{}, err
		}
	}

	// Convert body to UTF-8 if content-type specifies a charset
	contentType := resp.Header.Get("Content-Type")
	if contentType != "" {
		reader, err := charset.NewReader(bytes.NewReader(body), contentType)
		if err == nil {
			body, err = io.ReadAll(reader)
			if err != nil {
				return Response{}, err
			}
		}
	}

	err = resp.Body.Close()
	if err != nil {
		return Response{}, err
	}
	return Response{
		StatusCode: resp.StatusCode,
		Body:       body,
		Headers:    ConvertHttpHeaderToHeaders(resp.Header),
	}, nil
}

func ConvertHttpHeaderToHeaders(headers http.Header) []Header {
	result := make([]Header, 0, len(headers))
	for key, values := range headers {
		for _, value := range values {
			result = append(result, Header{
				HeaderKey: key,
				Value:     value,
			})
		}
	}
	return result
}

func ConvertHeadersToHttp(headers []Header) http.Header {
	result := make(http.Header)
	for _, header := range headers {
		result.Add(header.HeaderKey, header.Value)
	}
	return result
}

func ConvertQueriesToUrl(queries []Query, url url.Values) url.Values {
	for _, query := range queries {
		url.Add(query.QueryKey, query.Value)
	}
	return url
}
