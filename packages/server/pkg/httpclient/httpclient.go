//nolint:revive // exported
package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/compress"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"golang.org/x/net/html/charset"
)

type HttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

const TimeoutRequest = 60 * time.Second

// maxRedirects is how many redirects a request follows before failing, as net/http's default.
const maxRedirects = 10

type noRedirectsKey struct{}

// WithoutRedirects marks a request context so a client from New returns a 30x response as is
// instead of following it (the yamlflow request option follow_redirects: false). Other
// requests on the same client still follow redirects.
func WithoutRedirects(ctx context.Context) context.Context {
	return context.WithValue(ctx, noRedirectsKey{}, true)
}

func checkRedirect(req *http.Request, via []*http.Request) error {
	if noFollow, _ := req.Context().Value(noRedirectsKey{}).(bool); noFollow {
		return http.ErrUseLastResponse
	}
	if len(via) >= maxRedirects {
		return fmt.Errorf("stopped after %d redirects", maxRedirects)
	}
	return nil
}

func New() *http.Client {
	return &http.Client{
		Timeout:       TimeoutRequest,
		Jar:           newRecordingJar(),
		CheckRedirect: checkRedirect,
	}
}

// finalURL returns the URL a response came from (after any redirects) and how many
// redirects led there. Empty and zero for a response without its request (mock clients).
func finalURL(resp *http.Response) (string, int) {
	if resp.Request == nil || resp.Request.URL == nil {
		return "", 0
	}
	redirects := 0
	for r := resp.Request.Response; r != nil && r.Request != nil; r = r.Request.Response {
		redirects++
	}
	return resp.Request.URL.String(), redirects
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
	// URL is the URL the response came from, after any redirects; Redirects is how many
	// redirects were followed to get there.
	URL       string `json:"url,omitempty"`
	Redirects int    `json:"redirects,omitempty"`
	// Stream is set when the response was read as a stream (see SendRequestAndConvertStream).
	Stream *StreamResult `json:"stream,omitempty"`
}

type ResponseVar struct {
	StatusCode int               `json:"status"`
	Body       any               `json:"body"`
	Headers    map[string]string `json:"headers"`
	Duration   int32             `json:"duration"`
	// URL is the final URL after redirects (response.url); Redirects how many were followed
	// (response.redirects).
	URL       string `json:"url"`
	Redirects int    `json:"redirects"`
	// Stream is the streamed response's result; StreamFields exposes it to expressions.
	Stream *StreamResult `json:"-"`
}

// StreamFields are the response fields a streamed response adds: text, events, event_count,
// ttft_ms (when a token arrived) and usage (when the stream reported it). Nil when the
// response was not streamed.
func (r ResponseVar) StreamFields() map[string]any {
	s := r.Stream
	if s == nil {
		return nil
	}
	events := make([]any, 0, len(s.Events))
	for _, e := range s.Events {
		ev := map[string]any{"data": e.Data}
		if e.Event != "" {
			ev["event"] = e.Event
		}
		if e.ID != "" {
			ev["id"] = e.ID
		}
		events = append(events, ev)
	}
	out := map[string]any{"text": s.Text, "events": events, "event_count": s.EventCount}
	if s.TTFT != nil {
		out["ttft_ms"] = float64(s.TTFT.Microseconds()) / 1000
	}
	if s.Usage != nil {
		out["usage"] = s.Usage
	}
	return out
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
		URL:        r.URL,
		Redirects:  r.Redirects,
		Stream:     r.Stream,
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
	respURL, redirects := finalURL(resp)
	return Response{
		StatusCode: resp.StatusCode,
		Body:       body,
		Headers:    ConvertHttpHeaderToHeaders(resp.Header),
		URL:        respURL,
		Redirects:  redirects,
	}, nil
}

// SendRequestAndConvertWithContext sends a request and converts the response. A
// text/event-stream response is read as a raw SSE stream (see SendRequestAndConvertStream).
func SendRequestAndConvertWithContext(ctx context.Context, client HttpClient, req *Request, exampleID idwrap.IDWrap) (Response, error) {
	return SendRequestAndConvertStream(ctx, client, req, exampleID, nil)
}

// convertResponse reads a whole response body, decompressed and converted to UTF-8.
func convertResponse(resp *http.Response) (Response, error) {
	defer func() { _ = resp.Body.Close() }()
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

	respURL, redirects := finalURL(resp)
	return Response{
		StatusCode: resp.StatusCode,
		Body:       body,
		Headers:    ConvertHttpHeaderToHeaders(resp.Header),
		URL:        respURL,
		Redirects:  redirects,
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
