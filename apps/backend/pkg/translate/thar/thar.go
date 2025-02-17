package thar

import (
	"encoding/json"
	"errors"
	"log"
	"sort"
	"strings"
	"the-dev-tools/backend/pkg/compress"
	"the-dev-tools/backend/pkg/flow/edge"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mbodyform"
	"the-dev-tools/backend/pkg/model/mbodyraw"
	"the-dev-tools/backend/pkg/model/mbodyurl"
	"the-dev-tools/backend/pkg/model/mexampleheader"
	"the-dev-tools/backend/pkg/model/mexamplequery"
	"the-dev-tools/backend/pkg/model/mflow"
	"the-dev-tools/backend/pkg/model/mitemapi"
	"the-dev-tools/backend/pkg/model/mitemapiexample"
	"the-dev-tools/backend/pkg/model/mnnode"
	"the-dev-tools/backend/pkg/model/mnnode/mnnoop"
	"the-dev-tools/backend/pkg/model/mnnode/mnrequest"
	"time"
)

type HarResvoled struct {
	// Collection Items
	Apis             []mitemapi.ItemApi
	Examples         []mitemapiexample.ItemApiExample
	Queries          []mexamplequery.Query
	Headers          []mexampleheader.Header
	RawBodies        []mbodyraw.ExampleBodyRaw
	FormBodies       []mbodyform.BodyForm
	UrlEncodedBodies []mbodyurl.BodyURLEncoded

	// Flow Items
	Flow         mflow.Flow
	Nodes        []mnnode.MNode
	RequestNodes []mnrequest.MNRequest
	Edges        []edge.Edge
	NoopNodes    []mnnoop.NoopNode
}

type HAR struct {
	Log Log `json:"log"`
}

type Log struct {
	Entries []Entry `json:"entries"`
}

type Entry struct {
	StartedDateTime time.Time `json:"startedDateTime"`
	Request         Request   `json:"request"`
	Response        Response  `json:"response"`
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
	MimeType string  `json:"mimeType"`
	Text     string  `json:"text"`
	Params   []Param `json:"params,omitempty"`
}

type Param struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Content struct {
	Size     int    `json:"size"`
	MimeType string `json:"mimeType"`
	Text     string `json:"text"`
}

const (
	RawBodyCheck        = "application/json"
	FormBodyCheck       = "multipart/form-data"
	UrlEncodedBodyCheck = "application/x-www-form-urlencoded"
)

func ConvertRaw(data []byte) (*HAR, error) {
	var harFile HAR
	err := json.Unmarshal(data, &harFile)
	if err != nil {
		return nil, err
	}
	return &harFile, nil
}

func ConvertParamToFormBodies(params []Param, exampleId idwrap.IDWrap) []mbodyform.BodyForm {
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

func ConvertParamToUrlBodies(params []Param, exampleId idwrap.IDWrap) []mbodyurl.BodyURLEncoded {
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
func ConvertHAR(har *HAR, collectionID, workspaceID, rootFlowID idwrap.IDWrap) (HarResvoled, error) {
	result := HarResvoled{}

	if len(har.Log.Entries) == 0 {
		return result, errors.New("HAR file is empty")
	}

	// sort by started time
	sort.Slice(har.Log.Entries, func(i, j int) bool {
		return har.Log.Entries[i].StartedDateTime.Before(har.Log.Entries[j].StartedDateTime)
	})

	flowID := idwrap.NewNow()
	result.Flow = mflow.Flow{
		ID:         flowID,
		FlowRootID: rootFlowID,
		Name:       har.Log.Entries[0].Request.URL,
	}

	var posX, posY float64

	startNodeID := idwrap.NewNow()
	startNode := mnnode.MNode{
		ID:        startNodeID,
		FlowID:    flowID,
		Name:      "Start",
		NodeKind:  mnnode.NODE_KIND_NO_OP,
		PositionX: posX,
		PositionY: posY,
	}
	result.Nodes = append(result.Nodes, startNode)

	startNodeNoop := mnnoop.NoopNode{
		FlowNodeID: startNodeID,
		Type:       mnnoop.NODE_NO_OP_KIND_START,
	}
	result.NoopNodes = append(result.NoopNodes, startNodeNoop)

	// Use a map to merge equivalent XHR entries.
	apiMap := make(map[string]*mitemapi.ItemApi)

	// Process each entry in the HAR file
	for _, entry := range har.Log.Entries {
		// Only process XHR requests.
		if !isXHRRequest(entry) {
			continue
		}

		// Build a key based on method + URL.
		key := entry.Request.Method + " " + entry.Request.URL

		var api *mitemapi.ItemApi

		// Check if the API endpoint already exists.
		if _, ok := apiMap[key]; ok {
			continue
		}

		// Create Endpoint/api for first occurrence.
		apiID := idwrap.NewNow()
		api = &mitemapi.ItemApi{
			ID:           apiID,
			Name:         key,
			Url:          entry.Request.URL,
			Method:       entry.Request.Method,
			CollectionID: collectionID,
		}
		apiMap[key] = api
		result.Apis = append(result.Apis, *api)

		// Create an example for this entry.
		exampleID := idwrap.NewNow()
		example := mitemapiexample.ItemApiExample{
			Name:         key,
			BodyType:     mitemapiexample.BodyTypeRaw,
			CollectionID: collectionID,
			ItemApiID:    api.ID,
			ID:           exampleID,
		}

		// If first occurrence, create a default example as well.
		defaultExampleID := idwrap.NewNow()
		exampleDefault := mitemapiexample.ItemApiExample{
			Name:         key,
			BodyType:     mitemapiexample.BodyTypeRaw,
			IsDefault:    true,
			CollectionID: collectionID,
			ItemApiID:    api.ID,
			ID:           defaultExampleID,
		}
		// Only add a flow node once per unique API.
		posY += 200
		flowNodeID := idwrap.NewNow()
		node := mnnode.MNode{
			ID:        flowNodeID,
			FlowID:    flowID,
			Name:      key,
			NodeKind:  mnnode.NODE_KIND_REQUEST,
			PositionX: posX,
			PositionY: posY,
		}
		result.Nodes = append(result.Nodes, node)

		request := mnrequest.MNRequest{
			FlowNodeID: flowNodeID,
			EndpointID: &api.ID,
			ExampleID:  &exampleID,
		}
		result.RequestNodes = append(result.RequestNodes, request)

		headers := extractHeaders(entry.Request.Headers, exampleID)
		headersDefault := extractHeaders(entry.Request.Headers, defaultExampleID)
		result.Headers = append(result.Headers, headers...)
		result.Headers = append(result.Headers, headersDefault...)

		queries := extractQueryParams(entry.Request.QueryString, exampleID)
		queriesDefault := extractQueryParams(entry.Request.QueryString, defaultExampleID)
		result.Queries = append(result.Queries, queries...)
		result.Queries = append(result.Queries, queriesDefault...)

		// Handle the request body.
		rawBody := mbodyraw.ExampleBodyRaw{
			ID:            idwrap.NewNow(),
			ExampleID:     exampleID,
			Data:          []byte(""),
			CompressType:  mbodyraw.CompressTypeNone,
			VisualizeMode: mbodyraw.VisualizeModeText,
		}

		if entry.Request.PostData != nil {
			postData := entry.Request.PostData
			if strings.Contains(postData.MimeType, FormBodyCheck) {
				formBodies := ConvertParamToFormBodies(postData.Params, exampleID)
				result.FormBodies = append(result.FormBodies, formBodies...)
				formBodiesDefault := ConvertParamToFormBodies(postData.Params, defaultExampleID)
				result.FormBodies = append(result.FormBodies, formBodiesDefault...)

				example.BodyType = mitemapiexample.BodyTypeForm
			} else if strings.Contains(postData.MimeType, UrlEncodedBodyCheck) {
				urlEncodedBodies := ConvertParamToUrlBodies(postData.Params, exampleID)
				result.UrlEncodedBodies = append(result.UrlEncodedBodies, urlEncodedBodies...)
				urlEncodedBodiesDefault := ConvertParamToUrlBodies(postData.Params, defaultExampleID)
				result.UrlEncodedBodies = append(result.UrlEncodedBodies, urlEncodedBodiesDefault...)

				example.BodyType = mitemapiexample.BodyTypeUrlencoded

			} else {

				rawBody.Data = []byte(postData.Text)
				example.BodyType = mitemapiexample.BodyTypeRaw
				if len(rawBody.Data) > 1024 {
					compressedData, err := compress.Compress(rawBody.Data, compress.CompressTypeZstd)
					if err != nil {
						return result, err
					}
					if len(compressedData) < len(rawBody.Data) {
						rawBody.Data = compressedData
						rawBody.CompressType = mbodyraw.CompressTypeZstd
					}
				}

			}
		}
		result.RawBodies = append(result.RawBodies, rawBody)
		rawBodyDefault := rawBody
		rawBodyDefault.ID = idwrap.NewNow()
		rawBodyDefault.ExampleID = defaultExampleID
		result.RawBodies = append(result.RawBodies, rawBodyDefault)

		result.Examples = append(result.Examples, example)
		exampleDefault.BodyType = example.BodyType
		result.Examples = append(result.Examples, exampleDefault)
	}

	for i := range result.Apis {
		if i > 0 {
			prevApi := &result.Apis[i-1]
			result.Apis[i].Prev = &prevApi.ID
		}
		if i < len(result.Apis)-1 {
			nextApi := &result.Apis[i+1]
			result.Apis[i].Next = &nextApi.ID
		}
	}

	// Verification loop
	for i := range result.Apis {
		current := &result.Apis[i]
		if i > 0 {
			// Verify prev link
			if current.Prev == nil || *current.Prev != result.Apis[i-1].ID {
				log.Fatalf("invalid prev link at index %d", i)
			}
		}
		if i < len(result.Apis)-1 {
			// Verify next link
			if current.Next == nil || *current.Next != result.Apis[i+1].ID {
				log.Fatalf("invalid next link at index %d", i)
			}
		}
	}

	for i := range result.Examples {
		if i > 0 {
			prevExample := &result.Examples[i-1]
			result.Examples[i].Prev = &prevExample.ID
		}
		if i < len(result.Examples)-1 {
			nextExample := &result.Examples[i+1]
			result.Examples[i].Next = &nextExample.ID
		}
	}

	// Create sequential edges for the flow nodes.
	for i, node := range result.Nodes {
		if i+1 > len(result.Nodes)-1 {
			break
		}
		currentEdge := edge.Edge{
			ID:            idwrap.NewNow(),
			FlowID:        flowID,
			SourceID:      node.ID,
			TargetID:      result.Nodes[i+1].ID,
			SourceHandler: edge.HandleUnspecified,
		}
		result.Edges = append(result.Edges, currentEdge)
	}

	return result, nil
}

// Helper: returns true if the HAR entry is for an XHR request.
func isXHRRequest(entry Entry) bool {
	// Check the X-Requested-With header – common for XHR.
	for _, header := range entry.Request.Headers {
		if strings.EqualFold(header.Name, "X-Requested-With") &&
			strings.EqualFold(header.Value, "XMLHttpRequest") {
			return true
		}
	}
	// Also check the Content-Type header for typical XHR MIME types.
	for _, header := range entry.Request.Headers {
		if strings.EqualFold(header.Name, "Content-Type") {
			if strings.Contains(header.Value, "application/json") ||
				strings.Contains(header.Value, "application/xml") ||
				strings.Contains(header.Value, "text/plain") {
				return true
			}
		}
	}
	return false
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
