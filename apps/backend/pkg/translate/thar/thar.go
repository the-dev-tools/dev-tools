package thar

import (
	"encoding/json"
	"strings"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mbodyform"
	"the-dev-tools/backend/pkg/model/mbodyraw"
	"the-dev-tools/backend/pkg/model/mbodyurl"
	"the-dev-tools/backend/pkg/model/mexampleheader"
	"the-dev-tools/backend/pkg/model/mexamplequery"
	"the-dev-tools/backend/pkg/model/mitemapi"
	"the-dev-tools/backend/pkg/model/mitemapiexample"
)

type HarResvoled struct {
	Apis             []mitemapi.ItemApi
	Examples         []mitemapiexample.ItemApiExample
	Queries          []mexamplequery.Query
	Headers          []mexampleheader.Header
	RawBodies        []mbodyraw.ExampleBodyRaw
	FormBodies       []mbodyform.BodyForm
	UrlEncodedBodies []mbodyurl.BodyURLEncoded
}

type HAR struct {
	Log Log `json:"log"`
}

type Log struct {
	Entries []Entry `json:"entries"`
}

type Entry struct {
	Request  Request  `json:"request"`
	Response Response `json:"response"`
}

type Request struct {
	Method      string    `json:"method"`
	URL         string    `json:"url"`
	HTTPVersion string    `json:"httpVersion"`
	Headers     []Header  `json:"headers"`
	PostData    *PostData `json:"postData,omitempty"`
	QueryString []Query   `json:"queryString"`
}

type Response struct {
	Status      int      `json:"status"`
	StatusText  string   `json:"statusText"`
	HTTPVersion string   `json:"httpVersion"`
	Headers     []Header `json:"headers"`
	Content     Content  `json:"content"`
}

type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Query struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type PostData struct {
	MimeType string   `json:"mimeType"`
	Text     string   `json:"text"`
	Params   []Parmas `json:"params,omitempty"`
}

type Parmas struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Content struct {
	Size     int    `json:"size"`
	MimeType string `json:"mimeType"`
	Text     string `json:"text"`
}

func ConvertRaw(data []byte) (*HAR, error) {
	var harFile HAR
	err := json.Unmarshal(data, &harFile)
	if err != nil {
		return nil, err
	}
	return &harFile, nil
}

func ConvertParamToFormBodies(params []Parmas, exampleId idwrap.IDWrap) []mbodyform.BodyForm {
	result := make([]mbodyform.BodyForm, len(params))
	for i, param := range params {
		result[i] = mbodyform.BodyForm{
			ID:        idwrap.NewNow(),
			BodyKey:   param.Name,
			Value:     param.Value,
			Enable:    true,
			ExampleID: exampleId,
		}
	}
	return result
}

func ConvertParamToUrlBodies(params []Parmas, exampleId idwrap.IDWrap) []mbodyurl.BodyURLEncoded {
	result := make([]mbodyurl.BodyURLEncoded, len(params))
	for i, param := range params {
		result[i] = mbodyurl.BodyURLEncoded{
			ID:        idwrap.NewNow(),
			BodyKey:   param.Name,
			Value:     param.Value,
			Enable:    true,
			ExampleID: exampleId,
		}
	}
	return result
}

// TODO: refactor this function to make it more readable
func ConvertHAR(har *HAR, collectionID idwrap.IDWrap) (HarResvoled, error) {
	result := HarResvoled{}

	// Process each entry in the HAR file
	for _, entry := range har.Log.Entries {
		// creating Endpoint/api
		api := mitemapi.ItemApi{
			ID:           idwrap.NewNow(),
			Name:         entry.Request.Method + " " + entry.Request.URL,
			Url:          entry.Request.URL,
			Method:       entry.Request.Method,
			CollectionID: collectionID,
		}

		result.Apis = append(result.Apis, api)

		// Default Example
		defaultExampleID := idwrap.NewNow()
		exampleDefault := mitemapiexample.ItemApiExample{
			Name:      entry.Request.Method + " " + entry.Request.URL,
			BodyType:  mitemapiexample.BodyTypeNone,
			IsDefault: true,

			CollectionID: collectionID,
			ItemApiID:    api.ID,
			ID:           defaultExampleID,
		}

		// Creating example
		exampleID := idwrap.NewNow()
		example := mitemapiexample.ItemApiExample{
			Name:     entry.Request.Method + " " + entry.Request.URL,
			BodyType: mitemapiexample.BodyTypeRaw,

			CollectionID: collectionID,
			ItemApiID:    api.ID,
			ID:           exampleID,
		}

		// Get headers
		headers := extractHeaders(entry.Request.Headers, exampleID)
		result.Headers = append(result.Headers, headers...)

		// Get queries
		queries := extractQueryParams(entry.Request.QueryString, exampleID)
		result.Queries = append(result.Queries, queries...)

		rawBody := mbodyraw.ExampleBodyRaw{
			ID:            idwrap.NewNow(),
			ExampleID:     exampleID,
			Data:          []byte(""),
			CompressType:  mbodyraw.CompressTypeNone,
			VisualizeMode: mbodyraw.VisualizeModeText,
		}

		if entry.Request.PostData != nil {
			postData := entry.Request.PostData
			switch {
			case strings.Contains(postData.MimeType, "application/json"):
				rawBody.Data = []byte(postData.Text)
				example.BodyType = mitemapiexample.BodyTypeRaw
			case strings.Contains(postData.MimeType, "multipart/form-data"):
				formBodies := ConvertParamToFormBodies(postData.Params, exampleID)
				result.FormBodies = append(result.FormBodies, formBodies...)
				formBodiesDefault := ConvertParamToFormBodies(postData.Params, defaultExampleID)
				result.FormBodies = append(result.FormBodies, formBodiesDefault...)

				example.BodyType = mitemapiexample.BodyTypeUrlencoded
			case strings.Contains(postData.MimeType, "application/x-www-form-urlencoded"):
				urlEncodedBodies := ConvertParamToUrlBodies(postData.Params, exampleID)
				result.UrlEncodedBodies = append(result.UrlEncodedBodies, urlEncodedBodies...)
				urlEncodedBodiesDefault := ConvertParamToUrlBodies(postData.Params, defaultExampleID)
				result.UrlEncodedBodies = append(result.UrlEncodedBodies, urlEncodedBodiesDefault...)

				example.BodyType = mitemapiexample.BodyTypeForm
			}
		}
		result.RawBodies = append(result.RawBodies, rawBody)
		rawBodyDefault := rawBody
		rawBodyDefault.ID = idwrap.NewNow()
		rawBodyDefault.ExampleID = defaultExampleID
		result.RawBodies = append(result.RawBodies, rawBodyDefault)

		result.Examples = append(result.Examples, example)
		result.Examples = append(result.Examples, exampleDefault)
	}

	// create prev and next fiels for each api and example
	var prevApi *mitemapi.ItemApi
	var prevExample *mitemapiexample.ItemApiExample
	var nextApi *mitemapi.ItemApi
	var nextExample *mitemapiexample.ItemApiExample
	for i := range result.Apis {
		if i > 0 {
			prevApi = &result.Apis[i-1]
			result.Apis[i].Prev = &prevApi.ID
		}
		if i < len(result.Apis)-1 {
			nextApi = &result.Apis[i+1]
			result.Apis[i].Next = &nextApi.ID
		}
	}

	for i := range result.Examples {
		if i > 0 {
			prevExample = &result.Examples[i-1]
			result.Examples[i].Prev = &prevExample.ID
		}
		if i < len(result.Examples)-1 {
			nextExample = &result.Examples[i+1]
			result.Examples[i].Next = &nextExample.ID
		}
	}

	return result, nil
}

func extractHeaders(headers []Header, exampleID idwrap.IDWrap) []mexampleheader.Header {
	var result []mexampleheader.Header
	for _, header := range headers {
		h := mexampleheader.Header{
			ID:        idwrap.NewNow(),
			ExampleID: exampleID,
			HeaderKey: header.Name,
			Value:     header.Value,
			Enable:    true,
		}
		result = append(result, h)
	}
	return result
}

func extractQueryParams(queries []Query, exampleID idwrap.IDWrap) []mexamplequery.Query {
	var result []mexamplequery.Query
	for _, query := range queries {
		q := mexamplequery.Query{
			ID:        idwrap.NewNow(),
			ExampleID: exampleID,
			QueryKey:  query.Name,
			Value:     query.Value,
			Enable:    true,
		}
		result = append(result, q)
	}
	return result
}

func hasHeader(headers []Header, name string) bool {
	for _, header := range headers {
		if strings.EqualFold(header.Name, name) {
			return true
		}
	}
	return false
}
