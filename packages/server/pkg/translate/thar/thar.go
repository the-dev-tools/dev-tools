package thar

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/depfinder"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/model/mbodyform"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mbodyurl"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mexamplequery"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mitemfolder"
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
	Folders          []mitemfolder.ItemFolder
	Asserts          []massert.Assert

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
	RawBodyCheck                = "application/json"
	FormBodyCheck               = "multipart/form-data"
	UrlEncodedBodyCheck         = "application/x-www-form-urlencoded"
	TimestampSequencingThreshold = 50 * time.Millisecond // Connect requests within 50ms for better sequencing
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

func ConvertParamToFormBodiesWithTemplating(params []Param, exampleId idwrap.IDWrap, depFinder *depfinder.DepFinder) []mbodyform.BodyForm {
	result := make([]mbodyform.BodyForm, len(params))
	for i, param := range params {
		val := param.Value
		// Try to replace tokens in form values
		if newVal, found, _ := (*depFinder).ReplaceWithPaths(val); found {
			val = newVal.(string)
		}
		result[i] = mbodyform.BodyForm{
			ID:        idwrap.NewNow(),
			BodyKey:   param.Name,
			Value:     val,
			Enable:    true,
			ExampleID: exampleId,
		}
	}
	return result
}

func ConvertParamToFormBodiesWithDeltaParent(params []Param, deltaExampleID idwrap.IDWrap, baseBodies []mbodyform.BodyForm, depFinder *depfinder.DepFinder) []mbodyform.BodyForm {
	var result []mbodyform.BodyForm

	// Create a map of base bodies by their key for quick lookup
	baseBodyMap := make(map[string]mbodyform.BodyForm)
	for _, baseBody := range baseBodies {
		baseBodyMap[baseBody.BodyKey] = baseBody
	}

	for _, param := range params {
		val := param.Value
		// Try to replace tokens in form values
		if newVal, found, _ := (*depFinder).ReplaceWithPaths(val); found {
			val = newVal.(string)
		}

		// Find the corresponding base body
		var deltaParentID *idwrap.IDWrap
		if baseBody, exists := baseBodyMap[param.Name]; exists {
			deltaParentID = &baseBody.ID
		}

		result = append(result, mbodyform.BodyForm{
			ID:            idwrap.NewNow(),
			BodyKey:       param.Name,
			Value:         val,
			Enable:        true,
			ExampleID:     deltaExampleID,
				DeltaParentID: deltaParentID,
		})
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

func ConvertParamToUrlBodiesWithTemplating(params []Param, exampleId idwrap.IDWrap, depFinder *depfinder.DepFinder) []mbodyurl.BodyURLEncoded {
	result := make([]mbodyurl.BodyURLEncoded, len(params))
	for i, param := range params {
		val := param.Value
		// Try to replace tokens in URL-encoded values
		if newVal, found, _ := (*depFinder).ReplaceWithPaths(val); found {
			val = newVal.(string)
		}
		result[i] = mbodyurl.BodyURLEncoded{
			ID:        idwrap.NewNow(),
			BodyKey:   param.Name,
			Value:     val,
			Enable:    true,
			ExampleID: exampleId,
		}
	}
	return result
}

func ConvertParamToUrlBodiesWithDeltaParent(params []Param, deltaExampleID idwrap.IDWrap, baseBodies []mbodyurl.BodyURLEncoded, depFinder *depfinder.DepFinder) []mbodyurl.BodyURLEncoded {
	var result []mbodyurl.BodyURLEncoded

	// Create a map of base bodies by their key for quick lookup
	baseBodyMap := make(map[string]mbodyurl.BodyURLEncoded)
	for _, baseBody := range baseBodies {
		baseBodyMap[baseBody.BodyKey] = baseBody
	}

	for _, param := range params {
		val := param.Value
		// Try to replace tokens in URL-encoded values
		if newVal, found, _ := (*depFinder).ReplaceWithPaths(val); found {
			val = newVal.(string)
		}

		// Find the corresponding base body
		var deltaParentID *idwrap.IDWrap
		if baseBody, exists := baseBodyMap[param.Name]; exists {
			deltaParentID = &baseBody.ID
		}

		result = append(result, mbodyurl.BodyURLEncoded{
			ID:            idwrap.NewNow(),
			BodyKey:       param.Name,
			Value:         val,
			Enable:        true,
			ExampleID:     deltaExampleID,
				DeltaParentID: deltaParentID,
		})
	}
	return result
}

// createFolderHierarchy creates a folder hierarchy based on URL structure
// Returns the leaf folder ID and all folders to be created
func createFolderHierarchy(requestURL string, collectionID idwrap.IDWrap, existingFolders map[string]idwrap.IDWrap) (idwrap.IDWrap, []mitemfolder.ItemFolder, error) {
	parsedURL, err := url.Parse(requestURL)
	if err != nil {
		return idwrap.IDWrap{}, nil, err
	}

	// Extract domain and path segments
	domain := parsedURL.Host
	if domain == "" {
		domain = "unknown"
	}

	pathSegments := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	if len(pathSegments) == 1 && pathSegments[0] == "" {
		pathSegments = []string{} // Empty path
	}

	// Create folder hierarchy: domain -> path segments (excluding the last one which becomes the API name)
	var folders []mitemfolder.ItemFolder
	var lastFolderID idwrap.IDWrap

	// Create domain folder
	domainKey := domain
	if folderID, exists := existingFolders[domainKey]; exists {
		lastFolderID = folderID
	} else {
		lastFolderID = idwrap.NewNow()
		domainFolder := mitemfolder.ItemFolder{
			ID:           lastFolderID,
			Name:         domain,
			CollectionID: collectionID,
			ParentID:     nil,
		}
		folders = append(folders, domainFolder)
		existingFolders[domainKey] = lastFolderID
	}

	// Create path segment folders based on URL structure
	// For URLs like /api/categories/16, we want:
	// - api folder (created)
	// - categories folder (parent: api)
	// - API name: 16 (placed in categories folder)
	// For URLs like /api/categories, we want:
	// - api folder (created)
	// - categories folder (parent: api)
	// - API name: categories (placed in categories folder)

	if len(pathSegments) > 1 {
		// Create folders for all path segments except the last one
		for i := 0; i < len(pathSegments)-1; i++ {
			segment := pathSegments[i]
			if segment == "" {
				continue
			}

			// Create key for this folder path - use full path to ensure uniqueness
			folderPath := domain + "/" + strings.Join(pathSegments[:i+1], "/")
			if folderID, exists := existingFolders[folderPath]; exists {
				lastFolderID = folderID
			} else {
				parentFolderID := lastFolderID // Current parent
				newFolderID := idwrap.NewNow()

				folder := mitemfolder.ItemFolder{
					ID:           newFolderID,
					Name:         segment,
					CollectionID: collectionID,
					ParentID:     &parentFolderID,
				}
				folders = append(folders, folder)
				existingFolders[folderPath] = newFolderID
				lastFolderID = newFolderID
			}
		}

		// For the last path segment, decide if it should be a folder or API name
		lastSegment := pathSegments[len(pathSegments)-1]
		if !isLikelyID(lastSegment) && !isAPIEndpoint(lastSegment) {
			// If it's not an ID or API endpoint, create a folder for it too
			folderPath := domain + "/" + strings.Join(pathSegments, "/")
			if folderID, exists := existingFolders[folderPath]; exists {
				lastFolderID = folderID
			} else {
				parentFolderID := lastFolderID // Current parent
				newFolderID := idwrap.NewNow()

				folder := mitemfolder.ItemFolder{
					ID:           newFolderID,
					Name:         lastSegment,
					CollectionID: collectionID,
					ParentID:     &parentFolderID,
				}
				folders = append(folders, folder)
				existingFolders[folderPath] = newFolderID
				lastFolderID = newFolderID
			}
		}
	}

	return lastFolderID, folders, nil
}

// isLikelyID checks if a string looks like an ID (numeric or UUID-like)
func isLikelyID(segment string) bool {
	if segment == "" {
		return false
	}

	// Check if it's all numeric
	allNumeric := true
	for _, r := range segment {
		if r < '0' || r > '9' {
			allNumeric = false
			break
		}
	}
	if allNumeric && len(segment) > 0 {
		return true
	}

	// Check if it looks like a UUID (contains hyphens and alphanumeric)
	if strings.Contains(segment, "-") && len(segment) >= 8 {
		return true
	}

	// Check if it's a very long alphanumeric string (likely an ID)
	if len(segment) > 15 {
		alphaNumeric := true
		for _, r := range segment {
			if (r < '0' || r > '9') && (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
				alphaNumeric = false
				break
			}
		}
		if alphaNumeric {
			return true
		}
	}

	return false
}

// isAPIEndpoint checks if a segment is likely an API endpoint (action) rather than a resource
func isAPIEndpoint(segment string) bool {
	// Common API action words that shouldn't be folders
	apiActions := []string{
		"login", "logout", "register", "signin", "signout", "signup",
		"create", "update", "delete", "list", "get", "post", "put", "patch",
		"search", "filter", "sort", "export", "import", "download", "upload",
		"activate", "deactivate", "enable", "disable", "approve", "reject",
		"send", "receive", "process", "validate", "verify", "confirm",
		"reset", "refresh", "sync", "backup", "restore", "health", "status",
		"profile", "settings", "preferences", "account", "dashboard",
		"overview", "summary", "details", "info", "metadata",
	}

	segmentLower := strings.ToLower(segment)
	for _, action := range apiActions {
		if segmentLower == action {
			return true
		}
	}

	return false
}

// getAPINameFromURL extracts the API name from the URL with method awareness
func getAPINameFromURL(requestURL string, method string) string {
	parsedURL, err := url.Parse(requestURL)
	if err != nil {
		return requestURL // Fallback to full URL
	}

	pathSegments := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	if len(pathSegments) > 0 && pathSegments[len(pathSegments)-1] != "" {
		lastSegment := pathSegments[len(pathSegments)-1]

		// For DELETE operations with an ID as the last segment (e.g., /tags/uuid),
		// use the resource name (second-to-last segment) as the base name
		if method == "DELETE" && len(pathSegments) > 1 && isLikelyID(lastSegment) {
			resourceName := pathSegments[len(pathSegments)-2]
			return resourceName
		}

		// If the last segment is an ID and we have a meaningful second-to-last segment,
		// use the ID as the name
		if len(pathSegments) > 1 && isLikelyID(lastSegment) {
			return lastSegment
		}

		// For collection endpoints like /api/categories, use the last segment
		return lastSegment
	}

	// If no path segments or empty last segment, use domain or full URL
	if parsedURL.Host != "" {
		return parsedURL.Host
	}
	return requestURL
}

// convertHARInternal is the internal implementation that accepts existing folders map
func convertHARInternal(har *HAR, collectionID, workspaceID idwrap.IDWrap, depFinder *depfinder.DepFinder, existingFoldersMap map[string]idwrap.IDWrap) (HarResvoled, error) {
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

	// Map to track existing folders by their path to avoid duplicates
	existingFolders := existingFoldersMap
	if existingFolders == nil {
		existingFolders = make(map[string]idwrap.IDWrap)
	}

	// Track previous node for timestamp-based sequencing
	var previousNodeID *idwrap.IDWrap
	var previousTimestamp *time.Time

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

		// Create folder hierarchy for this URL
		leafFolderID, newFolders, err := createFolderHierarchy(originalURL, collectionID, existingFolders)
		if err != nil {
			return result, fmt.Errorf("failed to create folder hierarchy for URL %s: %w", originalURL, err)
		}

		// Add new folders to result
		result.Folders = append(result.Folders, newFolders...)

		// Extract API name from URL with method awareness
		apiName := getAPINameFromURL(originalURL, entry.Request.Method)

		// Create Endpoint/api for each entry
		apiID := idwrap.NewNow()
		api := &mitemapi.ItemApi{
			ID:           apiID,
			Name:         apiName,      // Use extracted API name
			Url:          templatedURL, // Use templated URL for the actual endpoint
			Method:       entry.Request.Method,
			CollectionID: collectionID,
			FolderID:     &leafFolderID, // Place API in the appropriate folder
		}
		result.Apis = append(result.Apis, *api)

		// Create Delta Endpoint/api for delta functionality
		deltaApiID := idwrap.NewNow()
		deltaApi := &mitemapi.ItemApi{
			ID:            deltaApiID,
			Name:          fmt.Sprintf("%s (Delta)", apiName),
			Url:           templatedURL, // Use templated URL for the delta endpoint
			Method:        entry.Request.Method,
			CollectionID:  collectionID,
			FolderID:      &leafFolderID, // Place API in the appropriate folder
			DeltaParentID: &apiID,        // Reference the parent API
		}
		result.Apis = append(result.Apis, *deltaApi)

		// Create an example for this entry.
		exampleID := idwrap.NewNow()
		example := mitemapiexample.ItemApiExample{
			ID:           exampleID,
			CollectionID: collectionID,
			Name:         apiName,
			BodyType:     mitemapiexample.BodyTypeRaw,
			ItemApiID:    apiID,
		}

		// If first occurrence, create a default example as well.
		defaultExampleID := idwrap.NewNow()
		exampleDefault := mitemapiexample.ItemApiExample{
			ID:           defaultExampleID,
			CollectionID: collectionID,
			Name:         apiName,
			BodyType:     mitemapiexample.BodyTypeRaw,
			IsDefault:    true,
			ItemApiID:    apiID,
		}
		deltaExampleID := idwrap.NewNow()
		deltaExample := mitemapiexample.ItemApiExample{
			ID:              deltaExampleID,
			Name:            fmt.Sprintf("%s (Delta)", apiName),
			CollectionID:    collectionID,
			ItemApiID:       apiID,
			VersionParentID: &defaultExampleID,
		}
		
		// Only add a flow node once per unique API.
		flowNodeID := idwrap.NewNow()
		request := mnrequest.MNRequest{
			FlowNodeID:      flowNodeID,
			EndpointID:      &api.ID,
			ExampleID:       &exampleID,
			DeltaExampleID:  &deltaExampleID,
			DeltaEndpointID: &deltaApiID,
		}
		result.RequestNodes = append(result.RequestNodes, request)

		var connected bool

		// Check for timestamp-based sequencing to preserve some HAR ordering
		// This creates edges between consecutive requests that are close in time,
		// maintaining parallelism for requests further apart while ensuring
		// sequential execution for rapid consecutive requests
		currentTimestamp := entry.StartedDateTime
		if previousNodeID != nil && previousTimestamp != nil {
			timeDiff := currentTimestamp.Sub(*previousTimestamp)
			if timeDiff >= 0 && timeDiff <= TimestampSequencingThreshold {
				// Connect to previous node if within threshold
				result.Edges = append(result.Edges, edge.Edge{
					ID:            idwrap.NewNow(),
					FlowID:        flowID,
					SourceID:      *previousNodeID,
					TargetID:      flowNodeID,
					SourceHandler: edge.HandleUnspecified,
				})
				connected = true
			}
		}

		// Update previous node tracking
		previousNodeID = &flowNodeID
		previousTimestamp = &currentTimestamp

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

		// Process headers for dependency tracking but don't modify original values yet
		originalHeaders := make([]Header, len(entry.Request.Headers))
		copy(originalHeaders, entry.Request.Headers)

		deltaHeaders := make([]Header, len(entry.Request.Headers))
		copy(deltaHeaders, entry.Request.Headers)
		
		// Track which headers have dependencies so we only create delta versions for those
		headersWithDependencies := make(map[int]bool)

		for i, header := range deltaHeaders {
			// Special handling for Authorization headers with Bearer tokens
			if strings.EqualFold(header.Name, "Authorization") && strings.HasPrefix(header.Value, "Bearer ") {
				token := strings.TrimPrefix(header.Value, "Bearer ")
				couple, err := (*depFinder).FindVar(token)
				if err == nil {
					deltaHeaders[i].Value = fmt.Sprintf("Bearer {{ %s }}", couple.Path)
					headersWithDependencies[i] = true
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
			deltaHeaders[i].Value = couple.Path
			headersWithDependencies[i] = true

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

		// Use original headers for both default and normal examples
		headers := extractHeaders(originalHeaders, exampleID)
		headersDefault := extractHeaders(originalHeaders, defaultExampleID)
		result.Headers = append(result.Headers, headers...)
		result.Headers = append(result.Headers, headersDefault...)

		// Process queries - original for default, templated for delta
		originalQueries := make([]Query, len(entry.Request.QueryString))
		deltaQueries := make([]Query, len(entry.Request.QueryString))
		
		// Track which queries have dependencies so we only create delta versions for those
		queriesWithDependencies := make(map[int]bool)

		for i, query := range entry.Request.QueryString {
			// Keep original values for default
			originalQueries[i] = Query{Name: query.Name, Value: query.Value}

			// Replace tokens in query values for delta
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
					// Check if the processed JSON actually changed
					if val != query.Value {
						queriesWithDependencies[i] = true
					}
				}
			}
			if !replaced {
				if newVal, found, _ := (*depFinder).ReplaceWithPaths(val); found {
					val = newVal.(string)
					queriesWithDependencies[i] = true
				}
			}
			deltaQueries[i] = Query{Name: query.Name, Value: val}
		}

		queriesApi := extractQueryParams(originalQueries, exampleID)
		queriesDefaultApi := extractQueryParams(originalQueries, defaultExampleID)
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

		// Declare variables for form bodies and URL-encoded bodies at higher scope
		var formBodies []mbodyform.BodyForm
		var urlEncodedBodies []mbodyurl.BodyURLEncoded
		var templatedBodyBytes []byte // Store templated JSON for delta examples

		if entry.Request.PostData != nil {
			postData := entry.Request.PostData
			if strings.Contains(postData.MimeType, FormBodyCheck) {
				// Use original values for both normal and default examples
				formBodies = ConvertParamToFormBodies(postData.Params, exampleID)
				result.FormBodies = append(result.FormBodies, formBodies...)
				formBodiesDefault := ConvertParamToFormBodies(postData.Params, defaultExampleID)
				result.FormBodies = append(result.FormBodies, formBodiesDefault...)

				example.BodyType = mitemapiexample.BodyTypeForm
			} else if strings.Contains(postData.MimeType, UrlEncodedBodyCheck) {
				// Use original values for both normal and default examples
				urlEncodedBodies = ConvertParamToUrlBodies(postData.Params, exampleID)
				result.UrlEncodedBodies = append(result.UrlEncodedBodies, urlEncodedBodies...)
				urlEncodedBodiesDefault := ConvertParamToUrlBodies(postData.Params, defaultExampleID)
				result.UrlEncodedBodies = append(result.UrlEncodedBodies, urlEncodedBodiesDefault...)

				example.BodyType = mitemapiexample.BodyTypeUrlencoded

			} else {
				// For JSON and other raw bodies, use original data without templating
				// JSON bodies should never be templated according to requirements
				bodyBytes := []byte(postData.Text)

				// Still check for dependencies to create edges, but don't modify the body
				if json.Valid(bodyBytes) {
					resultDep := depFinder.TemplateJSON(bodyBytes)
					if resultDep.Err != nil {
						// Error templating JSON
					} else {
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
							// Store templated JSON for delta examples
							templatedBodyBytes = resultDep.NewJson
						}
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
			}
		}

		// Don't immediately connect to start node - we'll handle this after all nodes are processed
		// to ensure proper dependency ordering
		if !connected {
			posX = float64(slotIndex * slotSize)
			posY = 100
			nodePosMap[flowID] = mpos{x: posX, y: posY}
			slotIndex++
		}

		if len(entry.Response.Content.Text) != 0 {
			repsonseBodyBytes := []byte(entry.Response.Content.Text)
			if json.Valid(repsonseBodyBytes) {
				path := fmt.Sprintf("%s.%s.%s", requestName, "response", "body")
				nodeID := flowNodeID
				couple := depfinder.VarCouple{Path: path, NodeID: nodeID}
				// Ignore error from AddJsonBytes as it's not critical for the conversion
				_ = depFinder.AddJsonBytes(repsonseBodyBytes, couple)
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
		
		// Use templated body for delta if it was created
		if templatedBodyBytes != nil {
			deltaBody.Data = templatedBodyBytes
			// Handle compression for templated delta body
			if len(deltaBody.Data) > 1024 {
				compressedData, err := compress.Compress(deltaBody.Data, compress.CompressTypeZstd)
				if err == nil && len(compressedData) < len(deltaBody.Data) {
					deltaBody.Data = compressedData
					deltaBody.CompressType = compress.CompressTypeZstd
				}
			}
		}
		
		result.RawBodies = append(result.RawBodies, deltaBody)

		// Create delta headers, queries, form bodies, and URL-encoded bodies
		// ONLY Delta examples use templated values for dependencies
		// Filter deltaHeaders to only include those with dependencies
		var deltaHeadersWithDeps []Header
		for i, header := range deltaHeaders {
			if headersWithDependencies[i] {
				deltaHeadersWithDeps = append(deltaHeadersWithDeps, header)
			}
		}
		
		headersDelta := extractHeadersWithDeltaParent(deltaHeadersWithDeps, deltaExampleID, headers)
		result.Headers = append(result.Headers, headersDelta...)

		// Filter deltaQueries to only include those with dependencies
		var deltaQueriesWithDeps []Query
		for i, query := range deltaQueries {
			if queriesWithDependencies[i] {
				deltaQueriesWithDeps = append(deltaQueriesWithDeps, query)
			}
		}
		
		queriesDelta := extractQueryParamsWithDeltaParent(deltaQueriesWithDeps, deltaExampleID, queriesApi)
		result.Queries = append(result.Queries, queriesDelta...)

		// Add delta form bodies and URL-encoded bodies if they exist (with templating and proper DeltaParentID)
		if entry.Request.PostData != nil {
			postData := entry.Request.PostData
			if strings.Contains(postData.MimeType, FormBodyCheck) {
				formBodiesDelta := ConvertParamToFormBodiesWithDeltaParent(postData.Params, deltaExampleID, formBodies, depFinder)
				result.FormBodies = append(result.FormBodies, formBodiesDelta...)
			} else if strings.Contains(postData.MimeType, UrlEncodedBodyCheck) {
				urlEncodedBodiesDelta := ConvertParamToUrlBodiesWithDeltaParent(postData.Params, deltaExampleID, urlEncodedBodies, depFinder)
				result.UrlEncodedBodies = append(result.UrlEncodedBodies, urlEncodedBodiesDelta...)
			}
		}

		result.Examples = append(result.Examples, example)
		exampleDefault.BodyType = example.BodyType
		result.Examples = append(result.Examples, exampleDefault)
		result.Examples = append(result.Examples, deltaExample)

		// Create status code assertions for all examples
		if entry.Response.Status > 0 {
			// Create assertion for the normal example
			assertNormal := createStatusCodeAssertion(exampleID, entry.Response.Status)
			result.Asserts = append(result.Asserts, assertNormal)

			// Create assertion for the default example
			assertDefault := createStatusCodeAssertion(defaultExampleID, entry.Response.Status)
			result.Asserts = append(result.Asserts, assertDefault)

			// Create delta assertion for the delta example
			// The delta assertion references the default assertion as its parent
			assertDelta := createStatusCodeAssertionWithDeltaParent(deltaExampleID, &assertDefault.ID, entry.Response.Status)
			result.Asserts = append(result.Asserts, assertDelta)
		}
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

	// Set Prev/Next for assertions to maintain ordering
	for i := range result.Asserts {
		if i > 0 {
			prevAssert := &result.Asserts[i-1]
			result.Asserts[i].Prev = &prevAssert.ID
		}
		if i < len(result.Asserts)-1 {
			nextAssert := &result.Asserts[i+1]
			result.Asserts[i].Next = &nextAssert.ID
		}
	}

	// After all entries are processed, connect nodes without dependencies to the start node
	// and ensure proper dependency ordering
	err := ensureProperDependencyOrdering(&result, startNodeID, flowID)
	if err != nil {
		return result, err
	}

	err = ReorganizeNodePositions(&result)
	if err != nil {
		return result, err
	}

	return result, nil
}

// ensureProperDependencyOrdering ensures that:
// 1. Nodes without incoming dependencies are connected to the start node
// 2. Dependency chains are properly ordered
// 3. Redundant transitive edges are removed
func ensureProperDependencyOrdering(result *HarResvoled, startNodeID idwrap.IDWrap, flowID idwrap.IDWrap) error {
	// First, perform transitive reduction to remove redundant edges
	err := performTransitiveReduction(result)
	if err != nil {
		return err
	}

	// Build a map of which nodes have incoming dependencies
	hasIncomingDependencies := make(map[idwrap.IDWrap]bool)

	for _, edge := range result.Edges {
		// Skip edges from the start node (these will be added by this function)
		if edge.SourceID != startNodeID {
			hasIncomingDependencies[edge.TargetID] = true
		}
	}

	// Find all request nodes that don't have incoming dependencies
	// and connect them to the start node
	nodeMap := make(map[idwrap.IDWrap]*mnnode.MNode)
	for i := range result.Nodes {
		nodeMap[result.Nodes[i].ID] = &result.Nodes[i]
	}

	for _, node := range result.Nodes {
		// Skip the start node itself
		if node.ID == startNodeID {
			continue
		}

		// If this request node has no incoming dependencies, connect it to start
		if !hasIncomingDependencies[node.ID] {
			result.Edges = append(result.Edges, edge.Edge{
				ID:            idwrap.NewNow(),
				FlowID:        flowID,
				SourceID:      startNodeID,
				TargetID:      node.ID,
				SourceHandler: edge.HandleUnspecified,
			})
		}
	}

	return nil
}

// performTransitiveReduction removes redundant edges from the dependency graph.
// If there's a path from A to C through B (A→B→C), then a direct edge A→C is redundant.
func performTransitiveReduction(result *HarResvoled) error {
	// Build adjacency list for the graph
	adjacencyList := make(map[idwrap.IDWrap]map[idwrap.IDWrap]bool)
	for _, edge := range result.Edges {
		if adjacencyList[edge.SourceID] == nil {
			adjacencyList[edge.SourceID] = make(map[idwrap.IDWrap]bool)
		}
		adjacencyList[edge.SourceID][edge.TargetID] = true
	}

	// For each node, compute all nodes reachable through paths of length > 1
	for source := range adjacencyList {
		// Find all nodes reachable from source through intermediate nodes
		reachableThroughPaths := make(map[idwrap.IDWrap]bool)
		
		// Check all direct neighbors
		for intermediate := range adjacencyList[source] {
			// From each direct neighbor, find what nodes are reachable
			if intermediateNeighbors, exists := adjacencyList[intermediate]; exists {
				for target := range intermediateNeighbors {
					// Mark that we can reach 'target' from 'source' through 'intermediate'
					reachableThroughPaths[target] = true
				}
			}
		}

		// Remove direct edges that are redundant (reachable through other paths)
		for target := range reachableThroughPaths {
			if adjacencyList[source][target] {
				// This edge is redundant, mark it for removal
				delete(adjacencyList[source], target)
			}
		}
	}

	// Rebuild the edges list without redundant edges
	var newEdges []edge.Edge
	for _, e := range result.Edges {
		if adjacencyList[e.SourceID] != nil && adjacencyList[e.SourceID][e.TargetID] {
			newEdges = append(newEdges, e)
		}
	}

	// Update the result with the reduced set of edges
	result.Edges = newEdges
	return nil
}

// ConvertHARWithDepFinder allows injecting a custom depFinder (for testing)
func ConvertHARWithDepFinder(har *HAR, collectionID, workspaceID idwrap.IDWrap, depFinder *depfinder.DepFinder) (HarResvoled, error) {
	return convertHARInternal(har, collectionID, workspaceID, depFinder, nil)
}

// ConvertHAR uses a new depFinder (for production)
func ConvertHAR(har *HAR, collectionID, workspaceID idwrap.IDWrap) (HarResvoled, error) {
	return ConvertHARWithDepFinder(har, collectionID, workspaceID, nil)
}

// ConvertHARWithExistingData allows passing pre-loaded folders and APIs for optimization
func ConvertHARWithExistingData(har *HAR, collectionID, workspaceID idwrap.IDWrap, existingFolders []mitemfolder.ItemFolder) (HarResvoled, error) {
	// Build folder map from existing folders
	folderMap := make(map[string]idwrap.IDWrap)
	
	// First, create a map by ID for quick lookups
	folderByID := make(map[idwrap.IDWrap]*mitemfolder.ItemFolder)
	for i := range existingFolders {
		folderByID[existingFolders[i].ID] = &existingFolders[i]
	}
	
	// Now build the path map
	for i := range existingFolders {
		folder := &existingFolders[i]
		path := buildFolderPath(folder, folderByID)
		folderMap[path] = folder.ID
		
		// Also add just the name for root folders
		if folder.ParentID == nil {
			folderMap[folder.Name] = folder.ID
		}
	}
	
	// Use existing ConvertHARWithDepFinder but inject folder map
	depFinder := depfinder.NewDepFinder()
	result, err := convertHARInternal(har, collectionID, workspaceID, &depFinder, folderMap)
	return result, err
}

// buildFolderPath reconstructs the full path for a folder
func buildFolderPath(folder *mitemfolder.ItemFolder, folderByID map[idwrap.IDWrap]*mitemfolder.ItemFolder) string {
	if folder.ParentID == nil {
		return folder.Name
	}
	
	parent, exists := folderByID[*folder.ParentID]
	if !exists {
		return folder.Name
	}
	
	parentPath := buildFolderPath(parent, folderByID)
	return parentPath + "/" + folder.Name
}

// ConvertHARWithDepFinderAndFolders is for future use
func ConvertHARWithDepFinderAndFolders(har *HAR, collectionID, workspaceID idwrap.IDWrap, depFinder *depfinder.DepFinder, preloadedFolders map[string]idwrap.IDWrap) (HarResvoled, error) {
	// For now, just use the existing function
	// TODO: Implement folder preloading optimization
	return ConvertHARWithDepFinder(har, collectionID, workspaceID, depFinder)
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

func extractHeadersWithDeltaParent(headers []Header, deltaExampleID idwrap.IDWrap, baseHeaders []mexampleheader.Header) []mexampleheader.Header {
	var result []mexampleheader.Header

	// Create a map of base headers by their key for quick lookup
	baseHeaderMap := make(map[string]mexampleheader.Header)
	for _, baseHeader := range baseHeaders {
		baseHeaderMap[baseHeader.HeaderKey] = baseHeader
	}

	for _, header := range headers {
		if len(header.Name) > 0 {
			// don't support pseudo-header atm
			if header.Name[0] == ':' {
				continue
			}

			// Find the corresponding base header with matching key
			var deltaParentID *idwrap.IDWrap
			if baseHeader, exists := baseHeaderMap[header.Name]; exists {
				deltaParentID = &baseHeader.ID
			}

			h := mexampleheader.Header{
				ID:            idwrap.NewNow(),
				ExampleID:     deltaExampleID,
				HeaderKey:     header.Name,
				Value:         header.Value,
				Enable:        true,
				DeltaParentID: deltaParentID,
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

func extractQueryParamsWithDeltaParent(queries []Query, deltaExampleID idwrap.IDWrap, baseQueries []mexamplequery.Query) []mexamplequery.Query {
	var result []mexamplequery.Query

	// Create a map of base queries by their key for quick lookup
	baseQueryMap := make(map[string]mexamplequery.Query)
	for _, baseQuery := range baseQueries {
		baseQueryMap[baseQuery.QueryKey] = baseQuery
	}

	for _, query := range queries {
		// Find the corresponding base query
		var deltaParentID *idwrap.IDWrap
		if baseQuery, exists := baseQueryMap[query.Name]; exists {
			deltaParentID = &baseQuery.ID
		}

		q := mexamplequery.Query{
			ID:            idwrap.NewNow(),
			ExampleID:     deltaExampleID,
			QueryKey:      query.Name,
			Value:         query.Value,
			Enable:        true,
				DeltaParentID: deltaParentID,
		}
		result = append(result, q)
	}
	return result
}

// ReorganizeNodePositions positions flow nodes using a level-based layout.
// Parallel nodes are positioned at the same Y level, sequential nodes at deeper levels.
func ReorganizeNodePositions(result *HarResvoled) error {
	const (
		nodeSpacingX = 400 // Horizontal spacing between parallel nodes
		nodeSpacingY = 300 // Vertical spacing between levels
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

	// Build adjacency lists from edges
	outgoingEdges := make(map[idwrap.IDWrap][]idwrap.IDWrap)
	incomingEdges := make(map[idwrap.IDWrap][]idwrap.IDWrap)
	for _, e := range result.Edges {
		outgoingEdges[e.SourceID] = append(outgoingEdges[e.SourceID], e.TargetID)
		incomingEdges[e.TargetID] = append(incomingEdges[e.TargetID], e.SourceID)
	}

	// Calculate dependency levels using BFS
	nodeLevels := make(map[idwrap.IDWrap]int)
	levelNodes := make(map[int][]idwrap.IDWrap) // level -> nodes at that level

	// BFS to assign levels
	queue := []idwrap.IDWrap{startNode.ID}
	nodeLevels[startNode.ID] = 0
	levelNodes[0] = []idwrap.IDWrap{startNode.ID}

	for len(queue) > 0 {
		currentNodeID := queue[0]
		queue = queue[1:]

		// Process all children
		for _, childID := range outgoingEdges[currentNodeID] {
			// Calculate the maximum level of all parents + 1
			maxParentLevel := -1
			for _, parentID := range incomingEdges[childID] {
				if parentLevel, exists := nodeLevels[parentID]; exists {
					if parentLevel > maxParentLevel {
						maxParentLevel = parentLevel
					}
				}
			}

			childLevel := maxParentLevel + 1
			
			// Only update if this is a new node or we found a deeper level
			if existingLevel, exists := nodeLevels[childID]; !exists || childLevel > existingLevel {
				// Remove from old level if it existed
				if exists {
					oldLevelNodes := levelNodes[existingLevel]
					for i, nodeID := range oldLevelNodes {
						if nodeID == childID {
							levelNodes[existingLevel] = append(oldLevelNodes[:i], oldLevelNodes[i+1:]...)
							break
						}
					}
				}

				// Add to new level
				nodeLevels[childID] = childLevel
				levelNodes[childLevel] = append(levelNodes[childLevel], childID)
				queue = append(queue, childID)
			}
		}
	}

	// Position nodes level by level
	for level := 0; level <= len(levelNodes)-1; level++ {
		nodes := levelNodes[level]
		if len(nodes) == 0 {
			continue
		}

		// Calculate Y position for this level
		yPos := float64(startY + level*nodeSpacingY)

		// Calculate starting X position to center the nodes at this level
		totalWidth := float64((len(nodes) - 1) * nodeSpacingX)
		startXForLevel := float64(startX) - totalWidth/2

		// Position each node in this level
		for i, nodeID := range nodes {
			if node := nodeMap[nodeID]; node != nil {
				node.PositionX = startXForLevel + float64(i*nodeSpacingX)
				node.PositionY = yPos
			}
		}
	}

	return nil
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

// createStatusCodeAssertion creates an assertion for checking the response status code
func createStatusCodeAssertion(exampleID idwrap.IDWrap, statusCode int) massert.Assert {
	// Create the condition expression for status code check
	// The expression uses JSONPath-like syntax to check response.status
	expression := fmt.Sprintf("response.status == %d", statusCode)
	
	return massert.Assert{
		ID:        idwrap.NewNow(),
		ExampleID: exampleID,
		Condition: mcondition.Condition{
			Comparisons: mcondition.Comparison{
				Expression: expression,
			},
		},
		Enable: true,
		// For HAR imports, we don't set Prev/Next as assertions don't have ordering in this context
		Prev: nil,
		Next: nil,
	}
}

// createStatusCodeAssertionWithDeltaParent creates a delta assertion for status code check
func createStatusCodeAssertionWithDeltaParent(deltaExampleID idwrap.IDWrap, deltaParentID *idwrap.IDWrap, statusCode int) massert.Assert {
	expression := fmt.Sprintf("response.status == %d", statusCode)
	
	return massert.Assert{
		ID:            idwrap.NewNow(),
		ExampleID:     deltaExampleID,
		DeltaParentID: deltaParentID,
		Condition: mcondition.Condition{
			Comparisons: mcondition.Comparison{
				Expression: expression,
			},
		},
		Enable: true,
		Prev:   nil,
		Next:   nil,
	}
}
