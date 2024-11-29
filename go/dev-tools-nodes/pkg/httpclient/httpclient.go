package httpclient

import (
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mexampleheader"
	"dev-tools-backend/pkg/model/mexamplequery"
	"dev-tools-backend/pkg/model/mexamplerespheader"
	"io"
	"net/http"
	"net/url"
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
	StatusCode int
	Body       []byte
	Headers    []mexamplerespheader.ExampleRespHeader
}

func SendRequest(client HttpClient, req Request) (*http.Response, error) {
	reqRaw, err := http.NewRequest(req.Method, req.URL, nil)
	if err != nil {
		return nil, err
	}
	qNew := ConvertModelToQuery(req.Queries, reqRaw.URL.Query())
	reqRaw.URL.RawQuery = qNew.Encode()
	reqRaw.Header = ConvertModelToHeader(req.Headers)
	return client.Do(reqRaw)
}

func SendRequestAndConvert(client HttpClient, req Request, exampleID idwrap.IDWrap) (Response, error) {
	resp, err := SendRequest(client, req)
	if err != nil {
		return Response{}, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, err
	}

	defer resp.Body.Close()
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
