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

type HARFile struct {
	Log struct {
		Entries []Entry `json:"entries"`
	} `json:"log"`
}

type Entry struct {
	Request struct {
		Method      string    `json:"method"`
		URL         string    `json:"url"`
		HTTPVersion string    `json:"httpVersion"`
		Headers     []Header  `json:"headers"`
		PostData    *PostData `json:"postData,omitempty"`
		QueryString []Query   `json:"queryString"`
	} `json:"request"`
	Response struct {
		Status      int      `json:"status"`
		StatusText  string   `json:"statusText"`
		HTTPVersion string   `json:"httpVersion"`
		Headers     []Header `json:"headers"`
		Content     Content  `json:"content"`
	} `json:"response"`
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
	MimeType string `json:"mimeType"`
	Text     string `json:"text"`
	Params   []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"params,omitempty"`
}

type Content struct {
	Size     int    `json:"size"`
	MimeType string `json:"mimeType"`
	Text     string `json:"text"`
}

func ConvertHAR(data []byte) (HarResvoled, error) {
	result := HarResvoled{}

	var harFile HARFile
	if err := json.Unmarshal(data, &harFile); err != nil {
		return result, err
	}

	// Process each entry in the HAR file
	for _, entry := range harFile.Log.Entries {
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
				formBodies := make([]mbodyform.BodyForm, 0)
				for _, param := range postData.Params {
					formBodies = append(formBodies, mbodyform.BodyForm{
						BodyKey:   param.Name,
						Value:     param.Value,
						Enable:    true,
						ExampleID: example.ID,
					})
				}
				result.formBodies = append(result.formBodies, formBodies...)
			case strings.Contains(postData.MimeType, "application/x-www-form-urlencoded"):
				urlEncodedBodies := make([]mbodyurl.BodyURLEncoded, 0)
				for _, param := range postData.Params {
					urlEncodedBodies = append(urlEncodedBodies, mbodyurl.BodyURLEncoded{
						BodyKey:   param.Name,
						Value:     param.Value,
						Enable:    true,
						ExampleID: example.ID,
					})
				}
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
