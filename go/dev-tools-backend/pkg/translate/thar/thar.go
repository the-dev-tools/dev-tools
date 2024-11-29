package thar

import (
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mbodyform"
	"dev-tools-backend/pkg/model/mbodyraw"
	"dev-tools-backend/pkg/model/mbodyurl"
	"dev-tools-backend/pkg/model/mitemapi"
	"dev-tools-backend/pkg/model/mitemapiexample"
	"encoding/json"
	"strings"
)

type HarResvoled struct {
	apis             []mitemapi.ItemApi
	examples         []mitemapiexample.ItemApiExample
	rawBodies        []mbodyraw.ExampleBodyRaw
	formBodies       []mbodyform.BodyForm
	urlEncodedBodies []mbodyurl.BodyURLEncoded
}

type HAR struct {
	Log struct {
		Entries []Entry `json:"entries"`
	} `json:"log"`
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
			BodyKey:   param.Name,
			Value:     param.Value,
			Enable:    true,
			ExampleID: exampleId,
		}
	}
	return result
}

func ConvertHAR(har *HAR) (HarResvoled, error) {
	result := HarResvoled{}

	// Process each entry in the HAR file
	for _, entry := range har.Log.Entries {
		api := mitemapi.ItemApi{
			ID:     idwrap.NewNow(),
			Url:    entry.Request.URL,
			Method: entry.Request.Method,
		}
		result.apis = append(result.apis, api)

		example := mitemapiexample.ItemApiExample{
			ItemApiID: api.ID,
			Name:      entry.Request.Method + " " + entry.Request.URL,
			BodyType:  mitemapiexample.BodyTypeRaw,
		}
		result.examples = append(result.examples, example)

		if entry.Request.PostData != nil {
			postData := entry.Request.PostData
			switch {
			case strings.Contains(postData.MimeType, "application/json"):
				rawBody := mbodyraw.ExampleBodyRaw{
					Data:          []byte(entry.Request.PostData.Text),
					VisualizeMode: mbodyraw.VisualizeModeJSON,
				}
				result.rawBodies = append(result.rawBodies, rawBody)
			case strings.Contains(postData.MimeType, "multipart/form-data"):
				formBodies := ConvertParamToFormBodies(postData.Params, example.ID)
				result.formBodies = append(result.formBodies, formBodies...)
			case strings.Contains(postData.MimeType, "application/x-www-form-urlencoded"):
				urlEncodedBodies := ConvertParamToUrlBodies(postData.Params, example.ID)
				result.urlEncodedBodies = append(result.urlEncodedBodies, urlEncodedBodies...)
			}
		}
	}

	return result, nil
}

func extractHeaders(headers []Header) map[string]string {
	result := make(map[string]string)
	for _, header := range headers {
		result[header.Name] = header.Value
	}
	return result
}

func extractQueryParams(queries []Query) map[string]string {
	result := make(map[string]string)
	for _, query := range queries {
		result[query.Name] = query.Value
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
