//nolint:revive // exported
package tpostmanv2

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"

	"the-dev-tools/server/pkg/depfinder"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mhttp"
)

// PostmanResolved contains all resolved HTTP requests and associated data from a Postman collection
type PostmanResolved struct {
	// Primary HTTP requests extracted from the collection (both Base and Delta)
	HTTPRequests []mhttp.HTTP

	// Associated data structures for each HTTP request
	SearchParams   []mhttp.HTTPSearchParam
	Headers        []mhttp.HTTPHeader
	BodyForms      []mhttp.HTTPBodyForm
	BodyUrlencoded []mhttp.HTTPBodyUrlencoded
	BodyRaw        []mhttp.HTTPBodyRaw
	Asserts        []mhttp.HTTPAssert

	// File system integration for workspace organization
	Files []mfile.File

	// Collection-level variables
	Variables []PostmanVariable

	// Flow integration (aligning with harv2)
	Flow         mflow.Flow
	Nodes        []mflow.Node
	RequestNodes []mflow.NodeRequest
	Edges        []mflow.Edge
}

// PostmanVariable represents a variable in a Postman collection
type PostmanVariable struct {
	Key   string
	Value string
}

// ConvertOptions defines configuration for Postman collection conversion
type ConvertOptions struct {
	WorkspaceID    idwrap.IDWrap  // Target workspace for all generated content
	FolderID       *idwrap.IDWrap // Optional parent folder for organization
	ParentHttpID   *idwrap.IDWrap // For delta system parent relationships
	IsDelta        bool           // Whether this represents a delta variation
	DeltaName      *string        // Optional name for delta variation
	CollectionName string         // Name used for file/folder generation
}

// PostmanCollection represents a simplified Postman collection structure for parsing
type PostmanCollection struct {
	Info struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Schema      string `json:"schema"`
	} `json:"info"`
	Item     []PostmanItem `json:"item"`
	Variable []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	} `json:"variable"`
	Auth *PostmanAuth `json:"auth,omitempty"`
}

// PostmanItem represents an item in a Postman collection (can be folder or request)
type PostmanItem struct {
	Name     string            `json:"name"`
	Item     []PostmanItem     `json:"item,omitempty"`
	Request  *PostmanRequest   `json:"request,omitempty"`
	Response []PostmanResponse `json:"response,omitempty"`
	Auth     *PostmanAuth      `json:"auth,omitempty"`
}

// PostmanRequest represents an HTTP request in Postman format
type PostmanRequest struct {
	Method      string          `json:"method"`
	Header      []PostmanHeader `json:"header"`
	Body        *PostmanBody    `json:"body,omitempty"`
	URL         PostmanURL      `json:"url"`
	Description string          `json:"description"`
	Auth        *PostmanAuth    `json:"auth,omitempty"`
}

// PostmanAuth represents authentication configuration for requests
type PostmanAuth struct {
	Type   string             `json:"type"`
	APIKey []PostmanAuthParam `json:"apikey,omitempty"`
	Basic  []PostmanAuthParam `json:"basic,omitempty"`
	Bearer []PostmanAuthParam `json:"bearer,omitempty"`
}

// PostmanAuthParam represents authentication parameters
type PostmanAuthParam struct {
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

// PostmanHeader represents a header in Postman format
type PostmanHeader struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Description string `json:"description"`
	Disabled    bool   `json:"disabled"`
}

// PostmanBody represents request body in Postman format
type PostmanBody struct {
	Mode       string              `json:"mode"`
	Raw        string              `json:"raw,omitempty"`
	FormData   []PostmanFormData   `json:"formdata,omitempty"`
	URLEncoded []PostmanURLEncoded `json:"urlencoded,omitempty"`
}

// PostmanFormData represents form data in Postman format
type PostmanFormData struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Description string `json:"description"`
	Disabled    bool   `json:"disabled"`
	Type        string `json:"type"`
}

// PostmanURLEncoded represents URL-encoded data in Postman format
type PostmanURLEncoded struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Description string `json:"description"`
	Disabled    bool   `json:"disabled"`
}

// PostmanURL represents a URL in Postman format
type PostmanURL struct {
	Raw      string              `json:"raw"`
	Protocol string              `json:"protocol,omitempty"`
	Host     []string            `json:"host,omitempty"`
	Port     string              `json:"port,omitempty"`
	Path     []string            `json:"path,omitempty"`
	Query    []PostmanQueryParam `json:"query,omitempty"`
	Hash     string              `json:"hash,omitempty"`
}

// PostmanQueryParam represents a query parameter in Postman format
type PostmanQueryParam struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Description string `json:"description"`
	Disabled    bool   `json:"disabled"`
}

// PostmanResponse represents a response in Postman format
type PostmanResponse struct {
	Name        string             `json:"name"`
	OriginalReq PostmanOriginalReq `json:"originalRequest"`
	Status      string             `json:"status"`
	Code        int                `json:"code"`
	Headers     []PostmanHeader    `json:"header"`
	Body        string             `json:"body"`
	Cookie      []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"cookie"`
}

// PostmanOriginalReq represents the original request for a response
type PostmanOriginalReq struct {
	Method string          `json:"method"`
	URL    PostmanURL      `json:"url"`
	Header []PostmanHeader `json:"header"`
	Body   *PostmanBody    `json:"body"`
}

// ConvertPostmanCollection converts Postman collection JSON data to modern HTTP models
func ConvertPostmanCollection(data []byte, opts ConvertOptions) (*PostmanResolved, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty collection data")
	}

	var collection PostmanCollection
	if err := json.Unmarshal(data, &collection); err != nil {
		return nil, fmt.Errorf("failed to parse Postman collection: %w", err)
	}

	resolved := &PostmanResolved{}

	// Import collection variables
	for _, v := range collection.Variable {
		resolved.Variables = append(resolved.Variables, PostmanVariable{
			Key:   v.Key,
			Value: v.Value,
		})
	}

	// Initialize Flow (aligning with harv2)
	flowID := idwrap.NewNow()
	resolved.Flow = mflow.Flow{
		ID:          flowID,
		WorkspaceID: opts.WorkspaceID,
		Name:        collection.Info.Name,
		Duration:    0,
	}
	if resolved.Flow.Name == "" {
		resolved.Flow.Name = "Imported Postman Collection"
	}

	// Create Start Node
	startNodeID := idwrap.NewNow()
	startNode := mflow.Node{
		ID:        startNodeID,
		FlowID:    flowID,
		Name:      "Start",
		NodeKind:  mflow.NODE_KIND_MANUAL_START,
		PositionX: 0,
		PositionY: 0,
	}
	resolved.Nodes = append(resolved.Nodes, startNode)

	// Initialize DepFinder for automatic dependency discovery
	df := depfinder.NewDepFinder()

	// Initialize folder context for URL-based folder creation
	fc := newFolderContext(opts.WorkspaceID)

	// Track the previous node for sequential linking
	var previousNodeID *idwrap.IDWrap = &startNodeID

	if err := processItems(collection.Item, idwrap.IDWrap{}, collection.Auth, previousNodeID, &df, fc, opts, resolved); err != nil {
		return nil, fmt.Errorf("failed to process collection items: %w", err)
	}

	// Append URL-based folder files to result
	fc.appendFolderFiles(resolved)

	// Create Flow file entry (aligning with harv2)
	if !mfile.IDIsZero(resolved.Flow.ID) {
		flowFile := mfile.File{
			ID:          resolved.Flow.ID,
			WorkspaceID: opts.WorkspaceID,
			ContentID:   &resolved.Flow.ID,
			ContentType: mfile.ContentTypeFlow,
			Name:        resolved.Flow.Name,
			Order:       -1, // Put flow at top/special order
			UpdatedAt:   time.Now(),
		}
		resolved.Files = append(resolved.Files, flowFile)
	}

	// Extract template variables from URLs/headers/body and add placeholders
	extractTemplateVariables(collection, resolved)

	return resolved, nil
}

// ParsePostmanCollection parses Postman collection JSON into a structured collection object
func ParsePostmanCollection(data []byte) (*PostmanCollection, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty collection data")
	}

	var collection PostmanCollection
	if err := json.Unmarshal(data, &collection); err != nil {
		return nil, fmt.Errorf("failed to parse Postman collection: %w", err)
	}

	return &collection, nil
}

// ConvertToFiles extracts only the file records from a Postman collection conversion
func ConvertToFiles(data []byte, opts ConvertOptions) ([]mfile.File, error) {
	resolved, err := ConvertPostmanCollection(data, opts)
	if err != nil {
		return nil, err
	}

	return resolved.Files, nil
}

// ConvertToHTTPRequests extracts only the HTTP requests from a Postman collection conversion
func ConvertToHTTPRequests(data []byte, opts ConvertOptions) ([]mhttp.HTTP, error) {
	resolved, err := ConvertPostmanCollection(data, opts)
	if err != nil {
		return nil, err
	}

	return resolved.HTTPRequests, nil
}

// createFileRecord creates a file record for an HTTP request
// Uses httpReq.ID as the file ID so frontend can match file to HTTP content
func createFileRecord(httpReq mhttp.HTTP, opts ConvertOptions) mfile.File {
	filename := httpReq.Name
	if filename == "" {
		filename = "untitled_request"
	}

	return mfile.File{
		ID:          httpReq.ID, // Same ID as HTTP request (like HAR does)
		WorkspaceID: opts.WorkspaceID,
		ParentID:    httpReq.FolderID,
		ContentID:   &httpReq.ID,
		ContentType: mfile.ContentTypeHTTP,
		Name:        filename,
		Order:       0, // Will be set by caller
		UpdatedAt:   time.Now(),
	}
}

// BuildPostmanCollection creates Postman collection JSON from resolved HTTP data
func BuildPostmanCollection(resolved *PostmanResolved) ([]byte, error) {
	collection := PostmanCollection{
		Info: struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Schema      string `json:"schema"`
		}{
			Name:        "Generated Collection",
			Description: "Generated from HTTP requests",
			Schema:      "https://schema.getpostman.com/json/collection/v2.1.0/collection.json",
		},
		Item: make([]PostmanItem, 0, len(resolved.HTTPRequests)),
	}

	// Build collection items from HTTP requests (filtering for base requests only for simple export)
	for _, httpReq := range resolved.HTTPRequests {
		if httpReq.IsDelta {
			continue
		}

		searchParams := extractSearchParamsForHTTP(httpReq.ID, resolved.SearchParams)
		item := PostmanItem{
			Name: httpReq.Name,
			Request: &PostmanRequest{
				Method:      httpReq.Method,
				URL:         buildPostmanURL(httpReq.Url, searchParams),
				Description: httpReq.Description,
				Header:      extractHeadersForHTTP(httpReq.ID, resolved.Headers),
			},
		}

		// Add request body if present
		if bodyRaw := extractBodyRawForHTTP(httpReq.ID, resolved.BodyRaw); bodyRaw != nil {
			item.Request.Body = &PostmanBody{
				Mode: "raw",
				Raw:  string(bodyRaw.RawData),
			}
		}

		collection.Item = append(collection.Item, item)
	}

	return json.MarshalIndent(collection, "", "  ")
}

// extractBodyRawForHTTP finds the raw body associated with a specific HTTP request
func extractBodyRawForHTTP(httpID idwrap.IDWrap, bodyRaws []mhttp.HTTPBodyRaw) *mhttp.HTTPBodyRaw {
	for i := range bodyRaws {
		if bodyRaws[i].HttpID.Compare(httpID) == 0 {
			return &bodyRaws[i]
		}
	}
	return nil
}

// processItems recursively processes Postman collection items and extracts HTTP requests
func processItems(items []PostmanItem, parentFolderID idwrap.IDWrap, inheritedAuth *PostmanAuth, previousNodeID *idwrap.IDWrap, df *depfinder.DepFinder, fc *folderContext, opts ConvertOptions, resolved *PostmanResolved) error {
	for _, item := range items {
		// Use inherited auth if item doesn't have its own
		effectiveAuth := item.Auth
		if effectiveAuth == nil {
			effectiveAuth = inheritedAuth
		}

		if item.Request == nil && (len(item.Item) > 0 || item.Item != nil) {
			// This is a folder, process its children
			folderID := idwrap.NewNow()

			// Create folder record with empty name fallback
			folderName := item.Name
			if folderName == "" {
				folderName = "Unnamed Folder"
			}
			folderFile := mfile.File{
				ID:          folderID,
				WorkspaceID: opts.WorkspaceID,
				ContentType: mfile.ContentTypeFolder,
				Name:        folderName,
				UpdatedAt:   time.Now(),
			}
			if parentFolderID.Compare(idwrap.IDWrap{}) != 0 {
				folderFile.ParentID = &parentFolderID
			} else if opts.FolderID != nil {
				folderFile.ParentID = opts.FolderID
			}
			resolved.Files = append(resolved.Files, folderFile)

			if err := processItems(item.Item, folderID, effectiveAuth, previousNodeID, df, fc, opts, resolved); err != nil {
				return err
			}
		} else if item.Request != nil {
			// This is an HTTP request, convert it using the Base + Delta system (aligning with harv2)

			// 1. Create Base Request (Literal)
			baseReq, baseHeaders, baseParams, baseBodyForms, baseBodyUrlEncoded, baseBodyRaw, _, err := convertPostmanRequestToHTTPModels(item, effectiveAuth, nil, opts)
			if err != nil {
				return fmt.Errorf("failed to convert request %q: %w", item.Name, err)
			}

			// 2. Create Templated (Delta) Request (With DepFinder)
			templatedReq, templatedHeaders, templatedParams, templatedBodyForms, templatedBodyUrlEncoded, templatedBodyRaw, deps, err := convertPostmanRequestToHTTPModels(item, effectiveAuth, df, opts)
			if err != nil {
				return fmt.Errorf("failed to convert templated request %q: %w", item.Name, err)
			}

			// Set the folder ID for both
			// If inside a Postman folder, use that folder
			// Otherwise, create URL-based folder hierarchy (like HAR does)
			if parentFolderID.Compare(idwrap.IDWrap{}) != 0 {
				baseReq.FolderID = &parentFolderID
			} else if opts.FolderID != nil {
				baseReq.FolderID = opts.FolderID
			} else if baseReq.Url != "" && fc != nil {
				// Root-level request: create URL-based folder structure
				urlFolderID, err := fc.getOrCreateURLFolder(baseReq.Url)
				if err == nil && urlFolderID.Compare(idwrap.IDWrap{}) != 0 {
					baseReq.FolderID = &urlFolderID
				}
			}

			// 3. Create Delta Request Object
			deltaReq := createDeltaVersion(*baseReq)
			deltaReq.FolderID = baseReq.FolderID

			// Apply templated values to Delta Request
			if templatedReq.Url != baseReq.Url {
				deltaReq.Url = templatedReq.Url
				deltaReq.DeltaUrl = &templatedReq.Url
			}

			// 4. Calculate Delta Components
			deltaHeaders := createDeltaHeaders(baseHeaders, templatedHeaders, deltaReq.ID)
			deltaParams := createDeltaSearchParams(baseParams, templatedParams, deltaReq.ID)
			deltaBodyForms := createDeltaBodyForms(baseBodyForms, templatedBodyForms, deltaReq.ID)
			deltaBodyUrlEncoded := createDeltaBodyUrlEncoded(baseBodyUrlEncoded, templatedBodyUrlEncoded, deltaReq.ID)
			deltaRaw := createDeltaBodyRaw(baseBodyRaw, templatedBodyRaw, deltaReq.ID)

			// 5. Create Node and Request Node data
			nodeID := idwrap.NewNow()
			node := mflow.Node{
				ID:        nodeID,
				FlowID:    resolved.Flow.ID,
				Name:      fmt.Sprintf("http_%d", len(resolved.RequestNodes)+1),
				NodeKind:  mflow.NODE_KIND_REQUEST,
				PositionX: float64(len(resolved.RequestNodes)+1) * 300,
				PositionY: 0,
			}

			reqNode := mflow.NodeRequest{
				FlowNodeID:  nodeID,
				HttpID:      &baseReq.ID,
				DeltaHttpID: &deltaReq.ID,
			}

			// 6. Create Edge (Sequential)
			if previousNodeID != nil {
				resolved.Edges = append(resolved.Edges, mflow.Edge{
					ID:            idwrap.NewNow(),
					FlowID:        resolved.Flow.ID,
					SourceID:      *previousNodeID,
					TargetID:      nodeID,
					SourceHandler: mflow.HandleUnspecified,
				})
			}

			// 7. Add Data Dependencies (from DepFinder)
			for _, couple := range deps {
				// Avoid duplicate edges
				exists := false
				for _, e := range resolved.Edges {
					if e.SourceID == couple.NodeID && e.TargetID == nodeID {
						exists = true
						break
					}
				}
				if !exists {
					resolved.Edges = append(resolved.Edges, mflow.Edge{
						ID:            idwrap.NewNow(),
						FlowID:        resolved.Flow.ID,
						SourceID:      couple.NodeID,
						TargetID:      nodeID,
						SourceHandler: mflow.HandleUnspecified,
					})
				}
			}

			// 8. Create Status Assertion from Examples
			if len(item.Response) > 0 && item.Response[0].Code > 0 {
				baseAssert, deltaAssert := createStatusAssertions(baseReq.ID, deltaReq.ID, item.Response[0].Code, len(resolved.Asserts))
				resolved.Asserts = append(resolved.Asserts, baseAssert, deltaAssert)
			}

			// 9. Collect all entities
			resolved.HTTPRequests = append(resolved.HTTPRequests, *baseReq, deltaReq)
			resolved.Headers = append(resolved.Headers, baseHeaders...)
			resolved.Headers = append(resolved.Headers, deltaHeaders...)
			resolved.SearchParams = append(resolved.SearchParams, baseParams...)
			resolved.SearchParams = append(resolved.SearchParams, deltaParams...)
			resolved.BodyForms = append(resolved.BodyForms, baseBodyForms...)
			resolved.BodyForms = append(resolved.BodyForms, deltaBodyForms...)
			resolved.BodyUrlencoded = append(resolved.BodyUrlencoded, baseBodyUrlEncoded...)
			resolved.BodyUrlencoded = append(resolved.BodyUrlencoded, deltaBodyUrlEncoded...)
			if baseBodyRaw != nil {
				resolved.BodyRaw = append(resolved.BodyRaw, *baseBodyRaw)
			}
			if deltaRaw != nil {
				resolved.BodyRaw = append(resolved.BodyRaw, *deltaRaw)
			}

			resolved.Nodes = append(resolved.Nodes, node)
			resolved.RequestNodes = append(resolved.RequestNodes, reqNode)

			// Update previous node for next iteration
			*previousNodeID = nodeID

			// Create file record for this HTTP request
			file := createFileRecord(*baseReq, opts)
			resolved.Files = append(resolved.Files, file)

			// Create File for Delta (aligning with harv2)
			deltaName := fmt.Sprintf("%s (Delta)", baseReq.Name)
			deltaFile := mfile.File{
				ID:          deltaReq.ID,
				WorkspaceID: opts.WorkspaceID,
				ParentID:    &file.ID,
				ContentID:   &deltaReq.ID,
				ContentType: mfile.ContentTypeHTTPDelta,
				Name:        deltaName,
				Order:       file.Order,
				UpdatedAt:   time.Now(),
			}
			resolved.Files = append(resolved.Files, deltaFile)

			// 10. Feed Examples into DepFinder for future requests
			for _, resp := range item.Response {
				if resp.Body != "" {
					path := fmt.Sprintf("%s.response.body", node.Name)
					_ = df.AddJsonBytes([]byte(resp.Body), depfinder.VarCouple{Path: path, NodeID: nodeID})
				}
			}
		}
	}
	return nil
}

// HTTPAssociatedData contains all data associated with an HTTP request
type HTTPAssociatedData struct {
	Headers        []mhttp.HTTPHeader
	SearchParams   []mhttp.HTTPSearchParam
	BodyForms      []mhttp.HTTPBodyForm
	BodyUrlencoded []mhttp.HTTPBodyUrlencoded
	BodyRaw        *mhttp.HTTPBodyRaw
}

// convertPostmanRequestToHTTPModels converts a Postman request to modern HTTP models with optional dependency finding
func convertPostmanRequestToHTTPModels(item PostmanItem, inheritedAuth *PostmanAuth, df *depfinder.DepFinder, opts ConvertOptions) (
	*mhttp.HTTP,
	[]mhttp.HTTPHeader,
	[]mhttp.HTTPSearchParam,
	[]mhttp.HTTPBodyForm,
	[]mhttp.HTTPBodyUrlencoded,
	*mhttp.HTTPBodyRaw,
	[]depfinder.VarCouple,
	error,
) {
	httpID := idwrap.NewNow()
	now := time.Now().UnixMilli()

	var allCouples []depfinder.VarCouple

	// Extract URL and search parameters
	baseURL, searchParams := convertPostmanURLToSearchParams(item.Request.URL, httpID)

	if df != nil {
		// Check URL for dependencies
		newURL, found, couples := df.ReplaceURLPathParams(baseURL)
		if found {
			baseURL = newURL
			allCouples = append(allCouples, couples...)
		}
	}

	// Determine authentication: request level auth > inherited auth
	effectiveAuth := item.Request.Auth
	if effectiveAuth == nil {
		effectiveAuth = inheritedAuth
	}

	// Convert headers with authentication
	headers := convertPostmanHeadersToHTTPHeaders(item.Request.Header, effectiveAuth, httpID)

	if df != nil {
		// Check headers for dependencies
		for i := range headers {
			if newVal, found, couples := df.ReplaceWithPathsSubstring(headers[i].Value); found {
				if strVal, ok := newVal.(string); ok {
					headers[i].Value = strVal
					allCouples = append(allCouples, couples...)
				}
			}
		}
		// Check search params for dependencies
		for i := range searchParams {
			if newVal, found, couples := df.ReplaceWithPaths(searchParams[i].Value); found {
				if strVal, ok := newVal.(string); ok {
					searchParams[i].Value = strVal
					allCouples = append(allCouples, couples...)
				}
			}
		}
	}

	// Convert request body
	var bodyRaw *mhttp.HTTPBodyRaw
	var bodyForms []mhttp.HTTPBodyForm
	var bodyUrlencoded []mhttp.HTTPBodyUrlencoded

	if item.Request.Body != nil {
		bodyRaw, bodyForms, bodyUrlencoded = convertPostmanBodyToHTTPModels(item.Request.Body, httpID)

		if df != nil {
			// Check body forms for dependencies
			for i := range bodyForms {
				if newVal, found, couples := df.ReplaceWithPaths(bodyForms[i].Value); found {
					if strVal, ok := newVal.(string); ok {
						bodyForms[i].Value = strVal
						allCouples = append(allCouples, couples...)
					}
				}
			}
			// Check body urlencoded for dependencies
			for i := range bodyUrlencoded {
				if newVal, found, couples := df.ReplaceWithPaths(bodyUrlencoded[i].Value); found {
					if strVal, ok := newVal.(string); ok {
						bodyUrlencoded[i].Value = strVal
						allCouples = append(allCouples, couples...)
					}
				}
			}
			// Check raw body for dependencies
			if bodyRaw != nil {
				res := df.TemplateJSON(bodyRaw.RawData)
				if res.Err == nil {
					bodyRaw.RawData = res.NewJson
					allCouples = append(allCouples, res.Couples...)
				}
			}
		}
	}

	// Determine body kind
	bodyKind := mhttp.HttpBodyKindNone
	if item.Request.Body != nil {
		switch item.Request.Body.Mode {
		case "raw":
			bodyKind = mhttp.HttpBodyKindRaw
		case "formdata":
			bodyKind = mhttp.HttpBodyKindFormData
		case "urlencoded":
			bodyKind = mhttp.HttpBodyKindUrlEncoded
		}
	}

	// Create the main HTTP request
	httpReq := &mhttp.HTTP{
		ID:           httpID,
		WorkspaceID:  opts.WorkspaceID,
		Name:         item.Name,
		Url:          baseURL,
		Method:       item.Request.Method,
		Description:  item.Request.Description,
		ParentHttpID: opts.ParentHttpID,
		IsDelta:      false,
		BodyKind:     bodyKind,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	// If method is not specified, default to GET
	if httpReq.Method == "" {
		httpReq.Method = "GET"
	}

	return httpReq, headers, searchParams, bodyForms, bodyUrlencoded, bodyRaw, allCouples, nil
}

// createDeltaVersion creates a delta version of an HTTP request (aligning with harv2)
func createDeltaVersion(original mhttp.HTTP) mhttp.HTTP {
	delta := mhttp.HTTP{
		ID:           idwrap.NewNow(),
		WorkspaceID:  original.WorkspaceID,
		ParentHttpID: &original.ID,
		Name:         original.Name + " (Delta)",
		Url:          original.Url,
		Method:       original.Method,
		Description:  original.Description + " [Delta Version]",
		IsDelta:      true,
		CreatedAt:    original.CreatedAt + 1,
		UpdatedAt:    original.UpdatedAt + 1,
	}

	return delta
}

// createStatusAssertions creates base and delta assertions for HTTP response status code
func createStatusAssertions(baseHttpID, deltaHttpID idwrap.IDWrap, statusCode int, assertCount int) (mhttp.HTTPAssert, mhttp.HTTPAssert) {
	now := time.Now().Unix()
	assertExpr := fmt.Sprintf("response.status == %d", statusCode)

	baseAssertID := idwrap.NewNow()
	baseAssert := mhttp.HTTPAssert{
		ID:           baseAssertID,
		HttpID:       baseHttpID,
		Value:        assertExpr,
		Enabled:      true,
		Description:  fmt.Sprintf("Verify response status is %d (from Postman import)", statusCode),
		DisplayOrder: float32(assertCount),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	deltaAssert := mhttp.HTTPAssert{
		ID:                 idwrap.NewNow(),
		HttpID:             deltaHttpID,
		Value:              assertExpr,
		Enabled:            true,
		Description:        fmt.Sprintf("Verify response status is %d (from Postman import)", statusCode),
		DisplayOrder:       float32(assertCount),
		ParentHttpAssertID: &baseAssertID,
		IsDelta:            true,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	return baseAssert, deltaAssert
}

// createDeltaHeaders creates delta headers when templated headers differ from base request
func createDeltaHeaders(originalHeaders []mhttp.HTTPHeader, newHeaders []mhttp.HTTPHeader, deltaHttpID idwrap.IDWrap) []mhttp.HTTPHeader {
	var deltaHeaders []mhttp.HTTPHeader
	originalMap := make(map[string]mhttp.HTTPHeader)
	for _, header := range originalHeaders {
		originalMap[header.Key] = header
	}
	for _, newHeader := range newHeaders {
		original, exists := originalMap[newHeader.Key]
		if !exists || original.Value != newHeader.Value {
			deltaKey := newHeader.Key
			deltaValue := newHeader.Value
			deltaEnabled := true
			var parentHeaderID *idwrap.IDWrap
			if exists {
				parentHeaderID = &original.ID
			}
			deltaHeaders = append(deltaHeaders, mhttp.HTTPHeader{
				ID:                 idwrap.NewNow(),
				HttpID:             deltaHttpID,
				Key:                deltaKey,
				Value:              deltaValue,
				Enabled:            true,
				ParentHttpHeaderID: parentHeaderID,
				IsDelta:            true,
				DeltaKey:           &deltaKey,
				DeltaValue:         &deltaValue,
				DeltaEnabled:       &deltaEnabled,
				CreatedAt:          newHeader.CreatedAt + 1,
				UpdatedAt:          newHeader.UpdatedAt + 1,
			})
		}
	}
	return deltaHeaders
}

// createDeltaSearchParams creates delta search params when templated params differ from base request
func createDeltaSearchParams(originalParams []mhttp.HTTPSearchParam, newParams []mhttp.HTTPSearchParam, deltaHttpID idwrap.IDWrap) []mhttp.HTTPSearchParam {
	var deltaParams []mhttp.HTTPSearchParam
	originalMap := make(map[string]mhttp.HTTPSearchParam)
	for _, param := range originalParams {
		originalMap[param.Key] = param
	}
	for _, newParam := range newParams {
		original, exists := originalMap[newParam.Key]
		if !exists || original.Value != newParam.Value {
			deltaKey := newParam.Key
			deltaValue := newParam.Value
			deltaEnabled := true
			var parentSearchParamID *idwrap.IDWrap
			if exists {
				parentSearchParamID = &original.ID
			}
			deltaParams = append(deltaParams, mhttp.HTTPSearchParam{
				ID:                      idwrap.NewNow(),
				HttpID:                  deltaHttpID,
				Key:                     deltaKey,
				Value:                   deltaValue,
				Enabled:                 true,
				ParentHttpSearchParamID: parentSearchParamID,
				IsDelta:                 true,
				DeltaKey:                &deltaKey,
				DeltaValue:              &deltaValue,
				DeltaEnabled:            &deltaEnabled,
				CreatedAt:               newParam.CreatedAt + 1,
				UpdatedAt:               newParam.UpdatedAt + 1,
			})
		}
	}
	return deltaParams
}

// createDeltaBodyForms creates delta body forms when templated forms differ from base request
func createDeltaBodyForms(originalForms []mhttp.HTTPBodyForm, newForms []mhttp.HTTPBodyForm, deltaHttpID idwrap.IDWrap) []mhttp.HTTPBodyForm {
	var deltaForms []mhttp.HTTPBodyForm
	originalMap := make(map[string]mhttp.HTTPBodyForm)
	for _, form := range originalForms {
		originalMap[form.Key] = form
	}
	for _, newForm := range newForms {
		original, exists := originalMap[newForm.Key]
		if !exists || original.Value != newForm.Value {
			deltaKey := newForm.Key
			deltaValue := newForm.Value
			deltaEnabled := true
			var parentBodyFormID *idwrap.IDWrap
			if exists {
				parentBodyFormID = &original.ID
			}
			deltaForms = append(deltaForms, mhttp.HTTPBodyForm{
				ID:                   idwrap.NewNow(),
				HttpID:               deltaHttpID,
				Key:                  deltaKey,
				Value:                deltaValue,
				Enabled:              true,
				ParentHttpBodyFormID: parentBodyFormID,
				IsDelta:              true,
				DeltaKey:             &deltaKey,
				DeltaValue:           &deltaValue,
				DeltaEnabled:         &deltaEnabled,
				CreatedAt:            newForm.CreatedAt + 1,
				UpdatedAt:            newForm.UpdatedAt + 1,
			})
		}
	}
	return deltaForms
}

// createDeltaBodyUrlEncoded creates delta URL-encoded body when templated body differs from base request
func createDeltaBodyUrlEncoded(originalEncoded []mhttp.HTTPBodyUrlencoded, newEncoded []mhttp.HTTPBodyUrlencoded, deltaHttpID idwrap.IDWrap) []mhttp.HTTPBodyUrlencoded {
	var deltaEncoded []mhttp.HTTPBodyUrlencoded
	originalMap := make(map[string]mhttp.HTTPBodyUrlencoded)
	for _, encoded := range originalEncoded {
		originalMap[encoded.Key] = encoded
	}
	for _, newEncoded := range newEncoded {
		original, exists := originalMap[newEncoded.Key]
		if !exists || original.Value != newEncoded.Value {
			deltaKey := newEncoded.Key
			deltaValue := newEncoded.Value
			deltaEnabled := true
			var parentBodyUrlencodedID *idwrap.IDWrap
			if exists {
				parentBodyUrlencodedID = &original.ID
			}
			deltaEncoded = append(deltaEncoded, mhttp.HTTPBodyUrlencoded{
				ID:                         idwrap.NewNow(),
				HttpID:                     deltaHttpID,
				Key:                        deltaKey,
				Value:                      deltaValue,
				Enabled:                    true,
				ParentHttpBodyUrlEncodedID: parentBodyUrlencodedID,
				IsDelta:                    true,
				DeltaKey:                   &deltaKey,
				DeltaValue:                 &deltaValue,
				DeltaEnabled:               &deltaEnabled,
				CreatedAt:                  newEncoded.CreatedAt + 1,
				UpdatedAt:                  newEncoded.UpdatedAt + 1,
			})
		}
	}
	return deltaEncoded
}

// createDeltaBodyRaw creates delta raw body when templated body differs from base request
func createDeltaBodyRaw(originalRaw *mhttp.HTTPBodyRaw, newRaw *mhttp.HTTPBodyRaw, deltaHttpID idwrap.IDWrap) *mhttp.HTTPBodyRaw {
	if newRaw == nil {
		return nil
	}
	if originalRaw == nil {
		return &mhttp.HTTPBodyRaw{
			ID:              idwrap.NewNow(),
			HttpID:          deltaHttpID,
			RawData:         newRaw.RawData,
			CompressionType: newRaw.CompressionType,
			CreatedAt:       newRaw.CreatedAt,
			UpdatedAt:       newRaw.UpdatedAt,
		}
	}
	if string(originalRaw.RawData) == string(newRaw.RawData) && originalRaw.CompressionType == newRaw.CompressionType {
		return nil
	}
	deltaRawData := newRaw.RawData
	deltaCompressionType := newRaw.CompressionType
	return &mhttp.HTTPBodyRaw{
		ID:                   idwrap.NewNow(),
		HttpID:               deltaHttpID,
		RawData:              newRaw.RawData,
		CompressionType:      newRaw.CompressionType,
		ParentBodyRawID:      &originalRaw.ID,
		IsDelta:              true,
		DeltaRawData:         deltaRawData,
		DeltaCompressionType: &deltaCompressionType,
		CreatedAt:            newRaw.CreatedAt + 1,
		UpdatedAt:            newRaw.UpdatedAt + 1,
	}
}

// folderContext tracks URL-based folders during Postman import
type folderContext struct {
	folderMap     map[string]idwrap.IDWrap // path -> folder ID
	folderFileMap map[string]mfile.File   // path -> folder file
	workspaceID   idwrap.IDWrap
}

// newFolderContext creates a new folder tracking context
func newFolderContext(workspaceID idwrap.IDWrap) *folderContext {
	return &folderContext{
		folderMap:     make(map[string]idwrap.IDWrap),
		folderFileMap: make(map[string]mfile.File),
		workspaceID:   workspaceID,
	}
}

// appendFolderFiles adds all created folder files to the resolved files
func (fc *folderContext) appendFolderFiles(resolved *PostmanResolved) {
	for _, folderFile := range fc.folderFileMap {
		resolved.Files = append(resolved.Files, folderFile)
	}
}

// getOrCreateURLFolder creates or retrieves folder ID for a URL-based path
func (fc *folderContext) getOrCreateURLFolder(urlStr string) (idwrap.IDWrap, error) {
	folderPath := buildFolderPathFromURL(urlStr)
	if folderPath == "" || folderPath == "/" {
		return idwrap.IDWrap{}, nil
	}

	return fc.getOrCreateFolder(folderPath)
}

// getOrCreateFolder creates or retrieves folder ID for a given path
func (fc *folderContext) getOrCreateFolder(folderPath string) (idwrap.IDWrap, error) {
	if existingID, exists := fc.folderMap[folderPath]; exists {
		return existingID, nil
	}

	// Create parent folders if needed first
	var parentID *idwrap.IDWrap
	parentPath := path.Dir(folderPath)
	if parentPath != "/" && parentPath != "." && parentPath != "" {
		pid, err := fc.getOrCreateFolder(parentPath)
		if err != nil {
			return idwrap.IDWrap{}, err
		}
		parentID = &pid
	}

	// Create new folder file
	folderID := idwrap.NewNow()
	folderName := path.Base(folderPath)
	if folderName == "" || folderName == "." || folderName == "/" {
		folderName = "imported"
	}

	folderFile := mfile.File{
		ID:          folderID,
		WorkspaceID: fc.workspaceID,
		ParentID:    parentID,
		ContentID:   nil, // Folders don't have content
		ContentType: mfile.ContentTypeFolder,
		Name:        folderName,
		Order:       0,
		UpdatedAt:   time.Now(),
	}

	fc.folderMap[folderPath] = folderID
	fc.folderFileMap[folderPath] = folderFile

	return folderID, nil
}

// buildFolderPathFromURL creates a hierarchical folder path from a URL
func buildFolderPathFromURL(urlStr string) string {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}

	// Normalize hostname: api.example.com -> com/example/api
	// Filter out empty parts to avoid double slashes in path
	var hostParts []string
	hostname := parsedURL.Hostname()
	if hostname != "" {
		parts := strings.Split(hostname, ".")
		for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
			parts[i], parts[j] = parts[j], parts[i]
		}
		for _, part := range parts {
			if part != "" {
				hostParts = append(hostParts, part)
			}
		}
	}

	// Clean up path segments
	pathSegments := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	var cleanSegments []string
	for _, segment := range pathSegments {
		if segment != "" && !isNumericSegment(segment) {
			cleanSegments = append(cleanSegments, sanitizeFileName(segment))
		}
	}

	// Combine host and path
	allSegments := make([]string, 0, len(hostParts)+len(cleanSegments))
	allSegments = append(allSegments, hostParts...)
	allSegments = append(allSegments, cleanSegments...)

	if len(allSegments) == 0 {
		return ""
	}
	return "/" + strings.Join(allSegments, "/")
}

// sanitizeFileName cleans up a string to be used as a filename
func sanitizeFileName(name string) string {
	replacer := strings.NewReplacer(
		" ", "_",
		"?", "",
		"#", "",
		"&", "_and_",
		"=", "_eq_",
		"<", "_lt_",
		">", "_gt_",
		"*", "_star_",
		"\"", "",
		"'", "",
		"/", "_",
		"\\", "_",
	)

	result := replacer.Replace(name)
	if result == "" {
		return "unnamed"
	}
	return result
}

// isNumericSegment checks if a URL path segment is purely numeric (like IDs)
func isNumericSegment(segment string) bool {
	if segment == "" {
		return false
	}
	for _, c := range segment {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// templateVarRegex matches Postman-style template variables like {{variableName}}
var templateVarRegex = regexp.MustCompile(`\{\{([^}]+)\}\}`)

// extractTemplateVariables finds all {{variable}} patterns in the collection
// and adds placeholder variables for any that aren't already defined
func extractTemplateVariables(collection PostmanCollection, resolved *PostmanResolved) {
	// Build set of existing variable keys
	existingVars := make(map[string]bool)
	for _, v := range resolved.Variables {
		existingVars[v.Key] = true
	}

	// Collect all unique template variables
	foundVars := make(map[string]bool)
	extractVarsFromItems(collection.Item, foundVars)

	// Add placeholder variables for any that aren't defined
	for varName := range foundVars {
		if !existingVars[varName] {
			resolved.Variables = append(resolved.Variables, PostmanVariable{
				Key:   varName,
				Value: "https://dev.tools/",
			})
		}
	}
}

// extractVarsFromItems recursively extracts template variables from collection items
func extractVarsFromItems(items []PostmanItem, foundVars map[string]bool) {
	for _, item := range items {
		// Recurse into folders
		if len(item.Item) > 0 {
			extractVarsFromItems(item.Item, foundVars)
		}

		// Extract from request
		if item.Request != nil {
			// URL
			extractVarsFromString(item.Request.URL.Raw, foundVars)
			for _, host := range item.Request.URL.Host {
				extractVarsFromString(host, foundVars)
			}
			for _, pathPart := range item.Request.URL.Path {
				extractVarsFromString(pathPart, foundVars)
			}
			for _, query := range item.Request.URL.Query {
				extractVarsFromString(query.Key, foundVars)
				extractVarsFromString(query.Value, foundVars)
			}

			// Headers
			for _, header := range item.Request.Header {
				extractVarsFromString(header.Key, foundVars)
				extractVarsFromString(header.Value, foundVars)
			}

			// Body
			if item.Request.Body != nil {
				extractVarsFromString(item.Request.Body.Raw, foundVars)
				for _, form := range item.Request.Body.FormData {
					extractVarsFromString(form.Key, foundVars)
					extractVarsFromString(form.Value, foundVars)
				}
				for _, encoded := range item.Request.Body.URLEncoded {
					extractVarsFromString(encoded.Key, foundVars)
					extractVarsFromString(encoded.Value, foundVars)
				}
			}

			// Auth
			if item.Request.Auth != nil {
				extractVarsFromAuth(item.Request.Auth, foundVars)
			}
		}

		// Auth at item level
		if item.Auth != nil {
			extractVarsFromAuth(item.Auth, foundVars)
		}
	}
}

// extractVarsFromAuth extracts template variables from auth configuration
func extractVarsFromAuth(auth *PostmanAuth, foundVars map[string]bool) {
	for _, param := range auth.APIKey {
		extractVarsFromString(param.Value, foundVars)
	}
	for _, param := range auth.Basic {
		extractVarsFromString(param.Value, foundVars)
	}
	for _, param := range auth.Bearer {
		extractVarsFromString(param.Value, foundVars)
	}
}

// extractVarsFromString finds all {{variable}} patterns in a string
func extractVarsFromString(s string, foundVars map[string]bool) {
	matches := templateVarRegex.FindAllStringSubmatch(s, -1)
	for _, match := range matches {
		if len(match) > 1 {
			varName := strings.TrimSpace(match[1])
			// Skip dynamic response references like http_1.response.body.token
			if !strings.Contains(varName, ".") {
				foundVars[varName] = true
			}
		}
	}
}
