package httpclient

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mexamplequery"
	"the-dev-tools/server/pkg/model/mexamplerespheader"
	"time"
)

type HttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

const TimeoutRequest = 5 * time.Second

func New() HttpClient {
	return &http.Client{
		Timeout: TimeoutRequest,
	}
}

type Request struct {
	Method  string
	URL     string
	Queries []mexamplequery.Query
	Headers []mexampleheader.Header
	Body    []byte
}

type Response struct {
	StatusCode int                                    `json:"statusCode"`
	Body       []byte                                 `json:"body"`
	Headers    []mexamplerespheader.ExampleRespHeader `json:"headers"`
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
	var body interface{}
	if json.Valid(r.Body) {
		var jsonBody map[string]interface{}
		// If unmarshaling works, use the decoded JSON.
		if err := json.Unmarshal(r.Body, &jsonBody); err == nil {
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
	reqRaw, err := http.NewRequest(req.Method, req.URL, bytes.NewReader(req.Body))
	if err != nil {
		return nil, err
	}
	qNew := ConvertModelToQuery(req.Queries, reqRaw.URL.Query())
	reqRaw.URL.RawQuery = qNew.Encode()
	reqRaw.Header = ConvertModelToHeader(req.Headers)
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

	err = resp.Body.Close()
	if err != nil {
		return Response{}, err
	}
	return Response{
		StatusCode: resp.StatusCode,
		Body:       body,
		Headers:    ConvertHeaderToModel(resp.Header, exampleID),
	}, nil
}

func ConvertHeaderToModel(headers http.Header, exampleID idwrap.IDWrap) []mexamplerespheader.ExampleRespHeader {
	result := make([]mexamplerespheader.ExampleRespHeader, 0, len(headers))
	for key, values := range headers {
		for _, value := range values {
			result = append(result, mexamplerespheader.ExampleRespHeader{
				ExampleRespID: exampleID,
				HeaderKey:     key,
				Value:         value,
			})
		}
	}
	return result
}

func ConvertQueryToModel(query map[string][]string, exampleID idwrap.IDWrap) []mexamplequery.Query {
	var result []mexamplequery.Query
	for key, values := range query {
		for _, value := range values {
			result = append(result, mexamplequery.Query{
				ExampleID: exampleID,
				QueryKey:  key,
				Value:     value,
			})
		}
	}
	return result
}

func ConvertModelToHeader(headers []mexampleheader.Header) http.Header {
	result := make(http.Header)
	for _, header := range headers {
		result.Add(header.HeaderKey, header.Value)
	}
	return result
}

func ConvertModelToQuery(queries []mexamplequery.Query, url url.Values) url.Values {
	for _, query := range queries {
		url.Add(query.QueryKey, query.Value)
	}
	return url
}
