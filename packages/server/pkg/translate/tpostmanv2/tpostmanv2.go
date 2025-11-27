package tpostmanv2

import (
	"encoding/json"
	"fmt"
	"time"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mhttp"
)

// PostmanResolved contains all resolved HTTP requests and associated data from a Postman collection
type PostmanResolved struct {
	// Primary HTTP requests extracted from the collection
	HTTPRequests []mhttp.HTTP

	// Associated data structures for each HTTP request
	SearchParams   []mhttp.HTTPSearchParam
	Headers        []mhttp.HTTPHeader
	BodyForms      []mhttp.HTTPBodyForm
	BodyUrlencoded []mhttp.HTTPBodyUrlencoded
	BodyRaw        []*mhttp.HTTPBodyRaw

	// File system integration for workspace organization
	Files []mfile.File
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
}

// PostmanItem represents an item in a Postman collection (can be folder or request)
type PostmanItem struct {
	Name     string            `json:"name"`
	Item     []PostmanItem     `json:"item,omitempty"`
	Request  *PostmanRequest   `json:"request,omitempty"`
	Response []PostmanResponse `json:"response,omitempty"`
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

	if err := processItems(collection.Item, idwrap.IDWrap{}, opts, resolved); err != nil {
		return nil, fmt.Errorf("failed to process collection items: %w", err)
	}

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

	// Build collection items from HTTP requests
	for _, httpReq := range resolved.HTTPRequests {
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

// processItems recursively processes Postman collection items and extracts HTTP requests
func processItems(items []PostmanItem, parentFolderID idwrap.IDWrap, opts ConvertOptions, resolved *PostmanResolved) error {
	for _, item := range items {
		if item.Request == nil {
			// This is a folder, process its children
			folderID := idwrap.NewNow()
			if err := processItems(item.Item, folderID, opts, resolved); err != nil {
				return err
			}
		} else {
			// This is an HTTP request, convert it
			httpReq, associatedData, err := convertPostmanRequestToHTTP(item, opts)
			if err != nil {
				return fmt.Errorf("failed to convert request %q: %w", item.Name, err)
			}

			resolved.HTTPRequests = append(resolved.HTTPRequests, *httpReq)
			resolved.Headers = append(resolved.Headers, associatedData.Headers...)
			resolved.SearchParams = append(resolved.SearchParams, associatedData.SearchParams...)
			resolved.BodyForms = append(resolved.BodyForms, associatedData.BodyForms...)
			resolved.BodyUrlencoded = append(resolved.BodyUrlencoded, associatedData.BodyUrlencoded...)
			if associatedData.BodyRaw != nil {
				resolved.BodyRaw = append(resolved.BodyRaw, associatedData.BodyRaw)
			}

			// Create file record for this HTTP request
			file := createFileRecord(*httpReq, parentFolderID, opts)
			resolved.Files = append(resolved.Files, file)
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

// convertPostmanRequestToHTTP converts a Postman request to modern HTTP models
func convertPostmanRequestToHTTP(item PostmanItem, opts ConvertOptions) (*mhttp.HTTP, *HTTPAssociatedData, error) {
	httpID := idwrap.NewNow()
	now := time.Now().UnixMilli()

	// Extract URL and search parameters
	baseURL, searchParams := convertPostmanURLToSearchParams(item.Request.URL, httpID)

	// Convert headers with authentication
	headers := convertPostmanHeadersToHTTPHeaders(item.Request.Header, item.Request.Auth, httpID)

	// Convert request body
	var bodyRaw *mhttp.HTTPBodyRaw
	var bodyForms []mhttp.HTTPBodyForm
	var bodyUrlencoded []mhttp.HTTPBodyUrlencoded

	if item.Request.Body != nil {
		bodyRaw, bodyForms, bodyUrlencoded = convertPostmanBodyToHTTPModels(item.Request.Body, httpID)
	}

	// Create the main HTTP request
	httpReq := &mhttp.HTTP{
		ID:           httpID,
		WorkspaceID:  opts.WorkspaceID,
		FolderID:     opts.FolderID,
		Name:         item.Name,
		Url:          baseURL,
		Method:       item.Request.Method,
		Description:  item.Request.Description,
		ParentHttpID: opts.ParentHttpID,
		IsDelta:      opts.IsDelta,
		DeltaName:    opts.DeltaName,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	// If method is not specified, default to GET
	if httpReq.Method == "" {
		httpReq.Method = "GET"
	}

	associatedData := &HTTPAssociatedData{
		Headers:        headers,
		SearchParams:   searchParams,
		BodyForms:      bodyForms,
		BodyUrlencoded: bodyUrlencoded,
		BodyRaw:        bodyRaw,
	}

	return httpReq, associatedData, nil
}

// createFileRecord creates a file record for an HTTP request
func createFileRecord(httpReq mhttp.HTTP, parentID idwrap.IDWrap, opts ConvertOptions) mfile.File {
	filename := httpReq.Name
	if filename == "" {
		filename = "untitled_request"
	}

	var pid *idwrap.IDWrap
	if parentID.Compare(idwrap.IDWrap{}) != 0 {
		pid = &parentID
	}

	return mfile.File{
		ID:          idwrap.NewNow(),
		WorkspaceID: opts.WorkspaceID,
		ParentID:    pid,
		ContentID:   &httpReq.ID,
		ContentType: mfile.ContentTypeHTTP,
		Name:        filename,
		Order:       0, // Will be set by caller
		UpdatedAt:   time.Now(),
	}
}
