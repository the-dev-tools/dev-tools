package thar

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/depfinder"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mbodyform"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mbodyurl"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mexamplequery"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
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
	ResourceType    string    `json:"_resourceType"`
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
		// check if json field not found
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

// ConvertHARWithDepFinder allows injecting a custom depFinder (for testing)
func ConvertHARWithDepFinder(har *HAR, collectionID, workspaceID idwrap.IDWrap, depFinder *depfinder.DepFinder) (HarResvoled, error) {
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
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        har.Log.Entries[0].Request.URL,
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

	type mpos struct {
		x float64
		y float64
	}

	if depFinder == nil {
		newFinder := depfinder.NewDepFinder()
		depFinder = &newFinder
	}
	nodePosMap := make(map[idwrap.IDWrap]mpos)

	slotIndex := 0
	const slotSize = 400

	// Process each entry in the HAR file
	for i, entry := range har.Log.Entries {
		// Only process XHR requests.
		if !IsXHRRequest(entry) {
			continue
		}

		requestName := fmt.Sprintf("request_%d", i)

		// Check for UUIDs in the URL path and replace them with templated variables
		originalURL := entry.Request.URL
		templatedURL, urlHasTemplates, urlCouples := (*depFinder).ReplaceURLPathParams(originalURL)

		// Update the entry URL if templates were found
		if urlHasTemplates {
			entry.Request.URL = templatedURL
		}

		// Create Endpoint/api for each entry
		apiID := idwrap.NewNow()
		api := &mitemapi.ItemApi{
			ID:           apiID,
			Name:         originalURL,  // Use original URL for display name
			Url:          templatedURL, // Use templated URL for the actual endpoint
			Method:       entry.Request.Method,
			CollectionID: collectionID,
		}
		result.Apis = append(result.Apis, *api)

		// Create an example for this entry.
		exampleID := idwrap.NewNow()
		example := mitemapiexample.ItemApiExample{
			ID:           exampleID,
			CollectionID: collectionID,
			Name:         entry.Request.URL,
			BodyType:     mitemapiexample.BodyTypeRaw,
			ItemApiID:    apiID,
		}

		// If first occurrence, create a default example as well.
		defaultExampleID := idwrap.NewNow()
		exampleDefault := mitemapiexample.ItemApiExample{
			ID:           defaultExampleID,
			CollectionID: collectionID,
			Name:         entry.Request.URL,
			BodyType:     mitemapiexample.BodyTypeRaw,
			IsDefault:    true,
			ItemApiID:    apiID,
		}
		deltaExampleID := idwrap.NewNow()
		deltaExample := mitemapiexample.ItemApiExample{
			ID:           deltaExampleID,
			Name:         fmt.Sprintf("%s (Delta)", entry.Request.URL),
			CollectionID: collectionID,
			ItemApiID:    apiID,
		}
		// Only add a flow node once per unique API.
		flowNodeID := idwrap.NewNow()
		request := mnrequest.MNRequest{
			FlowNodeID:     flowNodeID,
			EndpointID:     &api.ID,
			ExampleID:      &exampleID,
			DeltaExampleID: &deltaExampleID,
		}
		result.RequestNodes = append(result.RequestNodes, request)

		var connected bool

		// Add edges for URL path parameter dependencies
		for _, couple := range urlCouples {
			result.Edges = append(result.Edges, edge.Edge{
				ID:            idwrap.NewNow(),
				FlowID:        flowID,
				SourceID:      couple.NodeID,
				TargetID:      flowNodeID,
				SourceHandler: edge.HandleUnspecified,
			})
			connected = true
		}

		for i, header := range entry.Request.Headers {
			// Special handling for Authorization headers with Bearer tokens
			if strings.EqualFold(header.Name, "Authorization") && strings.HasPrefix(header.Value, "Bearer ") {
				token := strings.TrimPrefix(header.Value, "Bearer ")
				couple, err := (*depFinder).FindVar(token)
				if err == nil {
					entry.Request.Headers[i].Value = fmt.Sprintf("Bearer {{ %s }}", couple.Path)
					result.Edges = append(result.Edges, edge.Edge{
						ID:            idwrap.NewNow(),
						FlowID:        flowID,
						SourceID:      couple.NodeID,
						TargetID:      flowNodeID,
						SourceHandler: edge.HandleUnspecified,
					})
					connected = true
					continue
				}
			}

			// Regular header processing
			couple, err := (*depFinder).FindVar(header.Value)
			if err != nil {
				if err == depfinder.ErrNotFound {
					continue
				}
				return result, err
			}
			entry.Request.Headers[i].Value = couple.Path

			result.Edges = append(result.Edges, edge.Edge{
				ID:            idwrap.NewNow(),
				FlowID:        flowID,
				SourceID:      couple.NodeID,
				TargetID:      flowNodeID,
				SourceHandler: edge.HandleUnspecified,
			})
			connected = true
		}

		/*
			for _, header := range entry.Response.Headers {
				path := fmt.Sprintf("{{ %s.%s.%s.%s }}", requestName, "response", "headers", http.CanonicalHeaderKey(header.Name))
				depFinder.AddVar(header.Value, depfinder.VarCouple{Path: path, NodeID: flowNodeID})
			}
		*/

		node := mnnode.MNode{
			ID:        flowNodeID,
			FlowID:    flowID,
			Name:      requestName,
			NodeKind:  mnnode.NODE_KIND_REQUEST,
			PositionX: posX,
			PositionY: posY,
		}
		result.Nodes = append(result.Nodes, node)

		headers := extractHeaders(entry.Request.Headers, exampleID)
		headersDefault := extractHeaders(entry.Request.Headers, defaultExampleID)
		result.Headers = append(result.Headers, headers...)
		result.Headers = append(result.Headers, headersDefault...)

		queries := make([]Query, len(entry.Request.QueryString))
		for i, query := range entry.Request.QueryString {
			// Replace tokens in query values
			val := query.Value
			var replaced bool
			// If the value is valid JSON, parse and template it
			var jsonObj interface{}
			if err := json.Unmarshal([]byte(val), &jsonObj); err == nil {
				// Recursively process JSON structure
				processedObj := processJSONForTokens(jsonObj, *depFinder)
				if marshaled, err := json.Marshal(processedObj); err == nil {
					val = string(marshaled)
					replaced = true
				}
			}
			if !replaced {
				if newVal, found, _ := (*depFinder).ReplaceWithPaths(val); found {
					val = newVal.(string)
				}
			}
			queries[i] = Query{Name: query.Name, Value: val}
		}
		queriesApi := extractQueryParams(queries, exampleID)
		queriesDefaultApi := extractQueryParams(queries, defaultExampleID)
		result.Queries = append(result.Queries, queriesApi...)
		result.Queries = append(result.Queries, queriesDefaultApi...)

		// Handle the request body.
		rawBody := mbodyraw.ExampleBodyRaw{
			ID:            idwrap.NewNow(),
			ExampleID:     exampleID,
			Data:          []byte(""),
			CompressType:  compress.CompressTypeNone,
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
				bodyBytes := []byte(postData.Text)
				if json.Valid(bodyBytes) {
					resultDep := depFinder.TemplateJSON(bodyBytes)
					if resultDep.Err != nil {
						fmt.Println("Error 4: ", resultDep.Err, postData.Text)
					} else {
						fmt.Println("find any: ", resultDep.FindAny)
						if resultDep.FindAny {
							connected = true
							for _, couple := range resultDep.Couples {
								result.Edges = append(result.Edges, edge.Edge{
									ID:            idwrap.NewNow(),
									FlowID:        flowID,
									SourceID:      couple.NodeID,
									TargetID:      flowNodeID,
									SourceHandler: edge.HandleUnspecified,
								})
							}
							bodyBytes = resultDep.NewJson
						}
					}
					rawBody.Data = bodyBytes
					example.BodyType = mitemapiexample.BodyTypeRaw
					if len(rawBody.Data) > 1024 {
						compressedData, err := compress.Compress(rawBody.Data, compress.CompressTypeZstd)
						if err != nil {
							return result, err
						}
						if len(compressedData) < len(rawBody.Data) {
							rawBody.Data = compressedData
							rawBody.CompressType = compress.CompressTypeZstd
						}
					}
				} else {
					// For non-JSON bodies, try to replace tokens in the string
					val := postData.Text
					var replaced bool
					var jsonObj any
					if err := json.Unmarshal([]byte(val), &jsonObj); err == nil {
						// Recursively process JSON structure
						processedObj := processJSONForTokens(jsonObj, *depFinder)
						if marshaled, err := json.Marshal(processedObj); err == nil {
							val = string(marshaled)
							replaced = true
						}
					}
					if !replaced {
						if newVal, found, _ := (*depFinder).ReplaceWithPaths(val); found {
							val = newVal.(string)
						}
					}
					rawBody.Data = []byte(val)
					example.BodyType = mitemapiexample.BodyTypeRaw
					if len(rawBody.Data) > 1024 {
						compressedData, err := compress.Compress(rawBody.Data, compress.CompressTypeZstd)
						if err != nil {
							return result, err
						}
						if len(compressedData) < len(rawBody.Data) {
							rawBody.Data = compressedData
							rawBody.CompressType = compress.CompressTypeZstd
						}
					}
				}
			}
		}

		if !connected {
			posX = float64(slotIndex * slotSize)
			posY = 100
			nodePosMap[flowID] = mpos{x: posX, y: posY}
			slotIndex++
			result.Edges = append(result.Edges, edge.Edge{
				ID:            idwrap.NewNow(),
				FlowID:        flowID,
				SourceID:      startNodeID,
				TargetID:      flowNodeID,
				SourceHandler: edge.HandleUnspecified,
			})
		}

		if len(entry.Response.Content.Text) != 0 {
			repsonseBodyBytes := []byte(entry.Response.Content.Text)
			if json.Valid(repsonseBodyBytes) {
				path := fmt.Sprintf("%s.%s.%s", requestName, "response", "body")
				nodeID := flowNodeID
				couple := depfinder.VarCouple{Path: path, NodeID: nodeID}
				err := depFinder.AddJsonBytes(repsonseBodyBytes, couple)
				if err != nil {
					fmt.Println(err)
				}
			}
		}

		result.RawBodies = append(result.RawBodies, rawBody)
		rawBodyDefault := rawBody
		rawBodyDefault.ID = idwrap.NewNow()
		rawBodyDefault.ExampleID = defaultExampleID
		result.RawBodies = append(result.RawBodies, rawBodyDefault)

		deltaBody := rawBodyDefault
		deltaBody.ID = idwrap.NewNow()
		deltaBody.ExampleID = deltaExampleID
		result.RawBodies = append(result.RawBodies, deltaBody)

		result.Examples = append(result.Examples, example)
		exampleDefault.BodyType = example.BodyType
		result.Examples = append(result.Examples, exampleDefault)
		result.Examples = append(result.Examples, deltaExample)
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

	err := ReorganizeNodePositions(&result)
	if err != nil {
		return result, err
	}

	return result, nil
}

// ConvertHAR uses a new depFinder (for production)
func ConvertHAR(har *HAR, collectionID, workspaceID idwrap.IDWrap) (HarResvoled, error) {
	return ConvertHARWithDepFinder(har, collectionID, workspaceID, nil)
}

// Helper: returns true if the HAR entry is for an XHR request.
func IsXHRRequest(entry Entry) bool {
	// Check if the entry has _resourceType set to xhr
	if entry.ResourceType == "xhr" {
		return true
	}

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
		if len(header.Name) > 0 {
			// don't support pseudo-header atm
			if header.Name[0] == ':' {
				continue
			}
			h := mexampleheader.Header{
				ID:        idwrap.NewNow(),
				ExampleID: exampleID,
				HeaderKey: header.Name,
				Value:     header.Value,
				Enable:    true,
			}
			result = append(result, h)
		}
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

// ReorganizeNodePositions positions flow nodes using a grid system to prevent overlaps.
// Each node is assigned to a unique grid cell, guaranteeing no overlaps.
func ReorganizeNodePositions(result *HarResvoled) error {
	const (
		gridCellSize = 400 // Size of each grid cell
		startX       = 0   // Starting X position
		startY       = 0   // Starting Y position
	)

	// Map for quick node lookup
	nodeMap := make(map[idwrap.IDWrap]*mnnode.MNode)
	for i := range result.Nodes {
		nodeMap[result.Nodes[i].ID] = &result.Nodes[i]
	}

	// Find start node
	var startNode *mnnode.MNode
	for i := range result.NoopNodes {
		if result.NoopNodes[i].Type == mnnoop.NODE_NO_OP_KIND_START {
			startNode = nodeMap[result.NoopNodes[i].FlowNodeID]
			break
		}
	}
	if startNode == nil {
		return errors.New("start node not found")
	}

	// Build an adjacency list from edges
	outgoingEdges := make(map[idwrap.IDWrap][]idwrap.IDWrap)
	incomingEdges := make(map[idwrap.IDWrap][]idwrap.IDWrap)
	for _, e := range result.Edges {
		outgoingEdges[e.SourceID] = append(outgoingEdges[e.SourceID], e.TargetID)
		incomingEdges[e.TargetID] = append(incomingEdges[e.TargetID], e.SourceID)
	}

	// Grid tracking system
	occupiedGrid := make(map[string]bool)

	// Position start node at origin
	startNode.PositionX = startX
	startNode.PositionY = startY
	occupiedGrid["0,0"] = true

	// Use topological ordering to position nodes
	positioned := make(map[idwrap.IDWrap]bool)
	positionQueue := []idwrap.IDWrap{startNode.ID}
	positioned[startNode.ID] = true

	for len(positionQueue) > 0 {
		nodeID := positionQueue[0]
		positionQueue = positionQueue[1:]

		// Add all children of this node to the queue
		for _, childID := range outgoingEdges[nodeID] {
			if positioned[childID] {
				continue // Already positioned
			}

			childNode := nodeMap[childID]
			if childNode == nil {
				continue
			}

			// Find a grid position for this child
			parentNode := nodeMap[nodeID]
			childX, childY := findNextAvailableGridPosition(parentNode.PositionX, parentNode.PositionY, gridCellSize, occupiedGrid)

			childNode.PositionX = childX
			childNode.PositionY = childY

			// Mark position as occupied
			gridX := int(childX / gridCellSize)
			gridY := int(childY / gridCellSize)
			gridKey := fmt.Sprintf("%d,%d", gridX, gridY)
			occupiedGrid[gridKey] = true

			positioned[childID] = true
			positionQueue = append(positionQueue, childID)
		}
	}

	return nil
}

// findNextAvailableGridPosition finds the next available grid position near the parent
func findNextAvailableGridPosition(parentX, parentY float64, gridCellSize int, occupiedGrid map[string]bool) (float64, float64) {
	// Start searching from positions near the parent
	baseGridX := int(parentX / float64(gridCellSize))
	baseGridY := int(parentY / float64(gridCellSize))

	// First, try positions directly below the parent (preferred for tree structure)
	for yOffset := 1; yOffset <= 10; yOffset++ {
		for xOffset := 0; xOffset <= yOffset; xOffset++ {
			// Try positions: directly below, then slightly to the sides
			positions := []struct{ x, y int }{
				{baseGridX, baseGridY + yOffset},           // directly below
				{baseGridX + xOffset, baseGridY + yOffset}, // below and to the right
				{baseGridX - xOffset, baseGridY + yOffset}, // below and to the left
			}

			for _, pos := range positions {
				if xOffset == 0 && pos.x != baseGridX {
					continue // skip duplicate direct below position
				}

				gridKey := fmt.Sprintf("%d,%d", pos.x, pos.y)
				if !occupiedGrid[gridKey] {
					return float64(pos.x * gridCellSize), float64(pos.y * gridCellSize)
				}
			}
		}
	}

	// If no position found below, search in expanding rings around the parent position
	for radius := 1; radius <= 20; radius++ {
		for x := baseGridX - radius; x <= baseGridX+radius; x++ {
			for y := baseGridY - radius; y <= baseGridY+radius; y++ {
				// Only check the perimeter of the current radius
				if x != baseGridX-radius && x != baseGridX+radius && y != baseGridY-radius && y != baseGridY+radius {
					continue
				}

				gridKey := fmt.Sprintf("%d,%d", x, y)
				if !occupiedGrid[gridKey] {
					return float64(x * gridCellSize), float64(y * gridCellSize)
				}
			}
		}
	}

	// Fallback: use a position based on the number of occupied positions
	fallbackX := float64(len(occupiedGrid) * gridCellSize)
	fallbackY := parentY + float64(gridCellSize)
	return fallbackX, fallbackY
}

func processJSONForTokens(obj interface{}, depFinder depfinder.DepFinder) interface{} {
	switch v := obj.(type) {
	case map[string]interface{}:
		// Process map values recursively
		for key, val := range v {
			v[key] = processJSONForTokens(val, depFinder)
		}
		return v
	case []interface{}:
		// Process array elements recursively
		for i, val := range v {
			v[i] = processJSONForTokens(val, depFinder)
		}
		return v
	case string:
		// Try to replace tokens in string values
		if newVal, found, _ := depFinder.ReplaceWithPaths(v); found {
			return newVal
		}
		return v
	default:
		return v
	}
}
