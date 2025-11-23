package harv2

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	"the-dev-tools/server/pkg/depfinder"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
)

// HAR Translator v2 - Modern implementation using mhttp.HTTP and mfile.File
// Replaces collection-based architecture with workspace-based organization

// HAR represents the structure of a HAR (HTTP Archive) file
type HAR struct {
	Log Log `json:"log"`
}

// Log contains the entries of a HAR file
type Log struct {
	Entries []Entry `json:"entries"`
}

// Entry represents a single HTTP request/response pair
type Entry struct {
	StartedDateTime time.Time `json:"startedDateTime"`
	ResourceType    string    `json:"_resourceType"`
	Request         Request   `json:"request"`
	Response        Response  `json:"response"`
}

// Request represents the HTTP request information
type Request struct {
	Method      string    `json:"method"`
	URL         string    `json:"url"`
	HTTPVersion string    `json:"httpVersion"`
	Headers     []Header  `json:"headers"`
	PostData    *PostData `json:"postData,omitempty"`
	QueryString []Query   `json:"queryString"`
}

// Response represents the HTTP response information
type Response struct {
	Status      int      `json:"status"`
	StatusText  string   `json:"statusText"`
	HTTPVersion string   `json:"httpVersion"`
	Headers     []Header `json:"headers"`
	Content     Content  `json:"content"`
}

// Header represents an HTTP header
type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Query represents a URL query parameter
type Query struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// PostData represents the request body data
type PostData struct {
	MimeType string  `json:"mimeType"`
	Text     string  `json:"text"`
	Params   []Param `json:"params,omitempty"`
}

// Param represents a form parameter
type Param struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Content represents the response body content
type Content struct {
	Size     int    `json:"size"`
	MimeType string `json:"mimeType"`
	Text     string `json:"text"`
}

// Constants for processing behavior
const (
	RawBodyCheck                 = "application/json"
	FormBodyCheck                = "multipart/form-data"
	UrlEncodedBodyCheck          = "application/x-www-form-urlencoded"
	TimestampSequencingThreshold = 50 * time.Millisecond // Connect requests within 50ms for better sequencing
)

// HarResolved represents the complete translated result using modern models
type HarResolved struct {
	// HTTP Requests (modern mhttp.HTTP model)
	HTTPRequests []mhttp.HTTP `json:"http_requests"`

	// Child Entities
	HTTPHeaders        []mhttp.HTTPHeader        `json:"http_headers"`
	HTTPSearchParams   []mhttp.HTTPSearchParam   `json:"http_search_params"`
	HTTPBodyForms      []mhttp.HTTPBodyForm      `json:"http_body_forms"`
	HTTPBodyUrlEncoded []mhttp.HTTPBodyUrlencoded `json:"http_body_urlencoded"`
	HTTPBodyRaws       []mhttp.HTTPBodyRaw       `json:"http_body_raws"`

	// File System (modern mfile.File model)
	Files []mfile.File `json:"files"`

	// Flow Items (preserving existing flow generation)
	Flow         mflow.Flow          `json:"flow"`
	Nodes        []mnnode.MNode      `json:"nodes"`
	RequestNodes []mnrequest.MNRequest `json:"request_nodes"`
	Edges        []edge.Edge         `json:"edges"`
}

// Helper functions for request processing
func requiresSequentialOrdering(method string) bool {
	return strings.EqualFold(method, "DELETE")
}

func isMutationMethod(method string) bool {
	switch strings.ToUpper(method) {
	case "POST", "PUT", "PATCH", "DELETE":
		return true
	default:
		return false
	}
}

func edgeExists(edges []edge.Edge, source, target idwrap.IDWrap) bool {
	for _, e := range edges {
		if e.SourceID == source && e.TargetID == target {
			return true
		}
	}
	return false
}

// ConvertRaw parses raw HAR data into our HAR structure
func ConvertRaw(data []byte) (*HAR, error) {
	var harFile HAR
	err := json.Unmarshal(data, &harFile)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HAR data: %w", err)
	}
	return &harFile, nil
}

// ConvertHAR is the main entry point for HAR conversion using modern architecture
// This is the primary function that replaces the legacy ConvertHAR from thar package
func ConvertHAR(har *HAR, workspaceID idwrap.IDWrap) (*HarResolved, error) {
	return ConvertHARWithDepFinder(har, workspaceID, nil)
}

// ConvertHARWithDepFinder converts HAR with dependency finding support
func ConvertHARWithDepFinder(har *HAR, workspaceID idwrap.IDWrap, depFinder *depfinder.DepFinder) (*HarResolved, error) {
	if har == nil {
		return nil, fmt.Errorf("HAR input cannot be nil")
	}

	if len(har.Log.Entries) == 0 {
		return &HarResolved{}, nil
	}

	// Sort entries by started date for proper sequencing
	entries := make([]Entry, len(har.Log.Entries))
	copy(entries, har.Log.Entries)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].StartedDateTime.Before(entries[j].StartedDateTime)
	})

	// Process entries and create HTTP requests and file structure
	result, err := processEntriesToHTTP(entries, workspaceID, depFinder)
	if err != nil {
		return nil, fmt.Errorf("failed to process entries: %w", err)
	}

	// Generate flow nodes and edges
	if err := generateFlowGraph(result, entries, result.HTTPRequests); err != nil {
		return nil, fmt.Errorf("failed to generate flow graph: %w", err)
	}

	// Create file for the flow
	if !mfile.IDIsZero(result.Flow.ID) {
		flowFile := mfile.File{
			ID:          idwrap.NewNow(),
			WorkspaceID: workspaceID,
			ContentID:   &result.Flow.ID,
			ContentType: mfile.ContentTypeFlow,
			Name:        result.Flow.Name,
			Order:       -1, // Put flow at top/special order
			UpdatedAt:   time.Now(),
		}
		result.Files = append(result.Files, flowFile)
	}

	return result, nil
}

// processEntriesToHTTP converts HAR entries to mhttp.HTTP entities and file structure
func processEntriesToHTTP(entries []Entry, workspaceID idwrap.IDWrap, depFinder *depfinder.DepFinder) (*HarResolved, error) {
	result := &HarResolved{
		HTTPRequests:       make([]mhttp.HTTP, 0, len(entries)),
		HTTPHeaders:        make([]mhttp.HTTPHeader, 0),
		HTTPSearchParams:   make([]mhttp.HTTPSearchParam, 0),
		HTTPBodyForms:      make([]mhttp.HTTPBodyForm, 0),
		HTTPBodyUrlEncoded: make([]mhttp.HTTPBodyUrlencoded, 0),
		HTTPBodyRaws:       make([]mhttp.HTTPBodyRaw, 0),
	}

	fileMap := make(map[string]mfile.File)
	folderMap := make(map[string]idwrap.IDWrap)
	folderFileMap := make(map[string]mfile.File)

	for i, entry := range entries {
		// Create HTTP request and child entities
		httpReq, headers, params, bodyForms, bodyUrlEncoded, bodyRaws, err := createHTTPRequestFromEntry(entry, workspaceID)
		if err != nil {
			return nil, fmt.Errorf("failed to create HTTP request for entry %d: %w", i, err)
		}

		// Append entities to result
		result.HTTPRequests = append(result.HTTPRequests, *httpReq)
		result.HTTPHeaders = append(result.HTTPHeaders, headers...)
		result.HTTPSearchParams = append(result.HTTPSearchParams, params...)
		result.HTTPBodyForms = append(result.HTTPBodyForms, bodyForms...)
		result.HTTPBodyUrlEncoded = append(result.HTTPBodyUrlEncoded, bodyUrlEncoded...)
		result.HTTPBodyRaws = append(result.HTTPBodyRaws, bodyRaws...)

		// Create delta version (always create both original and delta)
		deltaReq := createDeltaVersion(*httpReq)
		result.HTTPRequests = append(result.HTTPRequests, deltaReq)

		// Create file system structure
		file, _, err := createFileStructure(httpReq, workspaceID, folderMap, folderFileMap)
		if err != nil {
			return nil, fmt.Errorf("failed to create file structure for entry %d: %w", i, err)
		}

		fileMap[httpReq.ID.String()] = *file
	}

	// Add folder files to result
	for _, folderFile := range folderFileMap {
		result.Files = append(result.Files, folderFile)
	}

	// Sort files for consistent ordering
	sortedFiles := make([]mfile.File, 0, len(fileMap))
	for _, file := range fileMap {
		sortedFiles = append(sortedFiles, file)
	}
	sort.Slice(sortedFiles, func(i, j int) bool {
		return sortedFiles[i].Order < sortedFiles[j].Order
	})

	result.Files = append(result.Files, sortedFiles...)
	return result, nil
}

// createHTTPRequestFromEntry creates an mhttp.HTTP entity and children from a HAR entry
func createHTTPRequestFromEntry(entry Entry, workspaceID idwrap.IDWrap) (
	*mhttp.HTTP,
	[]mhttp.HTTPHeader,
	[]mhttp.HTTPSearchParam,
	[]mhttp.HTTPBodyForm,
	[]mhttp.HTTPBodyUrlencoded,
	[]mhttp.HTTPBodyRaw,
	error,
) {
	parsedURL, err := url.Parse(entry.Request.URL)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to parse URL %s: %w", entry.Request.URL, err)
	}

	// Determine body kind
	bodyKind := mhttp.HttpBodyKindNone
	if entry.Request.PostData != nil {
		mimeType := strings.ToLower(entry.Request.PostData.MimeType)
		if strings.Contains(mimeType, FormBodyCheck) {
			bodyKind = mhttp.HttpBodyKindFormData
		} else if strings.Contains(mimeType, UrlEncodedBodyCheck) {
			bodyKind = mhttp.HttpBodyKindUrlEncoded
		} else {
			bodyKind = mhttp.HttpBodyKindRaw
		}
	}

	// Generate a descriptive name from the URL
	name := generateRequestName(entry.Request.Method, parsedURL)
	now := entry.StartedDateTime.UnixMilli()
	httpID := idwrap.NewNow()

	httpReq := &mhttp.HTTP{
		ID:          httpID,
		WorkspaceID: workspaceID,
		Name:        name,
		Url:         entry.Request.URL,
		Method:      entry.Request.Method,
		Description: fmt.Sprintf("Imported from HAR - %s %s", entry.Request.Method, entry.Request.URL),
		BodyKind:    bodyKind,
		IsDelta:     false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Extract headers
	headers := make([]mhttp.HTTPHeader, 0, len(entry.Request.Headers))
	for _, h := range entry.Request.Headers {
		headers = append(headers, mhttp.HTTPHeader{
			ID:          idwrap.NewNow(),
			HttpID:      httpID,
			HeaderKey:   h.Name,
			HeaderValue: h.Value,
			Enabled:     true,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
	}

	// Extract Query Parameters
	params := make([]mhttp.HTTPSearchParam, 0, len(entry.Request.QueryString))
	for _, q := range entry.Request.QueryString {
		params = append(params, mhttp.HTTPSearchParam{
			ID:         idwrap.NewNow(),
			HttpID:     httpID,
			ParamKey:   q.Name,
			ParamValue: q.Value,
			Enabled:    true,
			CreatedAt:  now,
			UpdatedAt:  now,
		})
	}

	// Extract Body
	var bodyForms []mhttp.HTTPBodyForm
	var bodyUrlEncoded []mhttp.HTTPBodyUrlencoded
	var bodyRaws []mhttp.HTTPBodyRaw

	if entry.Request.PostData != nil {
		if bodyKind == mhttp.HttpBodyKindFormData {
			for _, p := range entry.Request.PostData.Params {
				bodyForms = append(bodyForms, mhttp.HTTPBodyForm{
					ID:        idwrap.NewNow(),
					HttpID:    httpID,
					FormKey:   p.Name,
					FormValue: p.Value,
					Enabled:   true,
					CreatedAt: now,
					UpdatedAt: now,
				})
			}
		} else if bodyKind == mhttp.HttpBodyKindUrlEncoded {
			for _, p := range entry.Request.PostData.Params {
				bodyUrlEncoded = append(bodyUrlEncoded, mhttp.HTTPBodyUrlencoded{
					ID:              idwrap.NewNow(),
					HttpID:          httpID,
					UrlencodedKey:   p.Name,
					UrlencodedValue: p.Value,
					Enabled:         true,
					CreatedAt:       now,
					UpdatedAt:       now,
				})
			}
		} else if bodyKind == mhttp.HttpBodyKindRaw {
			bodyRaws = append(bodyRaws, mhttp.HTTPBodyRaw{
				ID:              idwrap.NewNow(),
				HttpID:          httpID,
				RawData:         []byte(entry.Request.PostData.Text),
				ContentType:     entry.Request.PostData.MimeType,
				CompressionType: 0, // Default to no compression
				CreatedAt:       now,
				UpdatedAt:       now,
			})
		}
	}

	return httpReq, headers, params, bodyForms, bodyUrlEncoded, bodyRaws, nil
}

// createDeltaVersion creates a delta version of an HTTP request
func createDeltaVersion(original mhttp.HTTP) mhttp.HTTP {
	deltaName := original.Name + " (Delta)"
	deltaURL := original.Url
	deltaMethod := original.Method
	deltaDesc := original.Description + " [Delta Version]"

	delta := mhttp.HTTP{
		ID:               idwrap.NewNow(),
		WorkspaceID:      original.WorkspaceID,
		ParentHttpID:     &original.ID,
		Name:             deltaName,
		Url:              original.Url,
		Method:           original.Method,
		Description:      deltaDesc,
		IsDelta:          true,
		DeltaName:        &deltaName,
		DeltaUrl:         &deltaURL,
		DeltaMethod:      &deltaMethod,
		DeltaDescription: &deltaDesc,
		CreatedAt:        original.CreatedAt + 1, // Ensure slightly later timestamp
		UpdatedAt:        original.UpdatedAt + 1,
	}

	return delta
}

// generateRequestName creates a descriptive name from HTTP method and URL
func generateRequestName(method string, parsedURL *url.URL) string {
	// Extract meaningful path segments
	pathSegments := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")

	// Take last 2-3 meaningful segments
	var meaningfulSegments []string
	for i := len(pathSegments) - 1; i >= 0 && len(meaningfulSegments) < 3; i-- {
		segment := pathSegments[i]
		if segment != "" && !strings.HasPrefix(segment, "{") && !isNumericSegment(segment) {
			meaningfulSegments = append([]string{segment}, meaningfulSegments...)
		}
	}

	// Include hostname if no meaningful path segments
	if len(meaningfulSegments) == 0 {
		host := strings.Replace(parsedURL.Hostname(), "www.", "", 1)
		host = strings.Replace(host, ".", " ", -1)
		return fmt.Sprintf("%s %s", method, strings.Title(host))
	}

	// Build final name
	pathName := strings.Join(meaningfulSegments, " ")
	return fmt.Sprintf("%s %s", method, strings.Title(strings.Replace(pathName, "-", " ", -1)))
}

// isNumericSegment checks if a URL segment is purely numeric (likely an ID)
func isNumericSegment(segment string) bool {
	for _, r := range segment {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(segment) > 0
}

// createFileStructure creates the file system hierarchy for an HTTP request
func createFileStructure(httpReq *mhttp.HTTP, workspaceID idwrap.IDWrap, folderMap map[string]idwrap.IDWrap, folderFileMap map[string]mfile.File) (*mfile.File, string, error) {
	parsedURL, err := url.Parse(httpReq.Url)
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse URL %s: %w", httpReq.Url, err)
	}

	// Build folder path from URL structure
	folderPath := buildFolderPathFromURL(parsedURL)
	folderID, err := getOrCreateFolder(folderPath, workspaceID, folderMap, folderFileMap)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create folder structure: %w", err)
	}

	// Create the file representing this HTTP request
	fileName := fmt.Sprintf("%s.request", sanitizeFileName(httpReq.Name))

	file := &mfile.File{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		FolderID:    &folderID,
		ContentID:   &httpReq.ID,
		ContentType: mfile.ContentTypeHTTP, // This maps to the old item_api
		Name:        fileName,
		Order:       float64(httpReq.CreatedAt),
	}

	return file, folderPath, nil
}

// buildFolderPathFromURL creates a hierarchical folder path from a URL
func buildFolderPathFromURL(parsedURL *url.URL) string {
	// Normalize hostname: api.example.com -> com/example/api
	hostParts := strings.Split(parsedURL.Hostname(), ".")
	for i, j := 0, len(hostParts)-1; i < j; i, j = i+1, j-1 {
		hostParts[i], hostParts[j] = hostParts[j], hostParts[i]
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
	allSegments := append(hostParts, cleanSegments...)
	return "/" + strings.Join(allSegments, "/")
}

// getOrCreateFolder creates or retrieves folder ID for a given path
func getOrCreateFolder(folderPath string, workspaceID idwrap.IDWrap, folderMap map[string]idwrap.IDWrap, folderFileMap map[string]mfile.File) (idwrap.IDWrap, error) {
	if existingID, exists := folderMap[folderPath]; exists {
		return existingID, nil
	}

	// Create parent folders if needed first to get parent ID
	var parentID *idwrap.IDWrap
	parentPath := path.Dir(folderPath)
	if parentPath != "/" && parentPath != "." {
		pid, err := getOrCreateFolder(parentPath, workspaceID, folderMap, folderFileMap)
		if err != nil {
			return idwrap.IDWrap{}, err
		}
		parentID = &pid
	}

	// Create new folder file
	folderID := idwrap.NewNow()
	folderFile := mfile.File{
		ID:          folderID,
		WorkspaceID: workspaceID,
		FolderID:    parentID,
		ContentID:   nil, // Folders don't have separate content objects in this model
		ContentType: mfile.ContentTypeFolder,
		Name:        path.Base(folderPath),
		Order:       0,
		UpdatedAt:   time.Now(),
	}

	folderMap[folderPath] = folderID
	folderFileMap[folderPath] = folderFile

	return folderID, nil
}

// sanitizeFileName cleans up a string to be used as a filename
func sanitizeFileName(name string) string {
	// Replace problematic characters with underscores
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

	return replacer.Replace(name)
}

// generateFlowGraph creates the flow visualization graph from HTTP requests
func generateFlowGraph(result *HarResolved, entries []Entry, httpRequests []mhttp.HTTP) error {
	if len(httpRequests) == 0 {
		return nil
	}

	// Create flow
	flowID := idwrap.NewNow()
	result.Flow = mflow.Flow{
		ID:          flowID,
		WorkspaceID: httpRequests[0].WorkspaceID,
		Name:        "Imported HAR Flow",
		Duration:    0, // Will be calculated if needed
	}

	// Create request nodes and edges
	if err := createRequestNodesAndEdges(result, entries, httpRequests); err != nil {
		return fmt.Errorf("failed to create request nodes: %w", err)
	}

	return nil
}

// createRequestNodesAndEdges creates flow nodes and dependency edges
func createRequestNodesAndEdges(result *HarResolved, entries []Entry, httpRequests []mhttp.HTTP) error {
	// Create request nodes
	requestNodes := make([]mnrequest.MNRequest, 0, len(httpRequests))

	// Create both nodes and requestNodes arrays
	nodes := make([]mnnode.MNode, 0, len(httpRequests))

	for _, httpReq := range httpRequests {
		if httpReq.IsDelta {
			continue // Skip delta requests for flow visualization
		}

		// Create MNode for visualization
		node := mnnode.MNode{
			ID:        idwrap.NewNow(),
			FlowID:    result.Flow.ID,
			Name:      httpReq.Name,
			NodeKind:  mnnode.NODE_KIND_REQUEST,
			PositionX: 0, // Will be positioned by layout algorithm
			PositionY: 0, // Will be positioned by layout algorithm
		}

		// Create MNRequest node data
		reqNode := mnrequest.MNRequest{
			FlowNodeID: node.ID,
			HttpID:     httpReq.ID,
		}

		nodes = append(nodes, node)
		requestNodes = append(requestNodes, reqNode)
	}

	result.RequestNodes = requestNodes
	result.Nodes = nodes

	// Create edges based on dependencies and timing
	edges := make([]edge.Edge, 0)

	// Simple sequence edges based on timestamp
	for i := 1; i < len(nodes); i++ {
		prevEntry := entries[i-1]
		currEntry := entries[i]

		// Create edge if within threshold or if dependency exists
		timeDiff := currEntry.StartedDateTime.Sub(prevEntry.StartedDateTime)
		if timeDiff <= TimestampSequencingThreshold || hasDependency(prevEntry, currEntry) {
			edge := edge.Edge{
				ID:        idwrap.NewNow(),
				FlowID:    result.Flow.ID,
				SourceID:  nodes[i-1].ID,
				TargetID:  nodes[i].ID,
			}
			edges = append(edges, edge)
		}
	}

	// Apply transitive reduction to remove redundant edges
	edges = applyTransitiveReduction(edges)
	result.Edges = edges

	return nil
}

// hasDependency checks if there's a dependency between two HAR entries
func hasDependency(prev, curr Entry) bool {
	prevURL, _ := url.Parse(prev.Request.URL)
	currURL, _ := url.Parse(curr.Request.URL)

	// Check if current URL uses data from previous response
	// This is a simplified implementation - the full version would be more sophisticated
	if strings.Contains(currURL.String(), prevURL.Host) {
		return true
	}

	return false
}

// applyTransitiveReduction removes redundant edges from the graph
func applyTransitiveReduction(edges []edge.Edge) []edge.Edge {
	if len(edges) == 0 {
		return edges
	}

	// Build adjacency map
	adjMap := make(map[idwrap.IDWrap][]idwrap.IDWrap)
	for _, edge := range edges {
		adjMap[edge.SourceID] = append(adjMap[edge.SourceID], edge.TargetID)
	}

	// For each edge, check if there's an alternative path
	var reducedEdges []edge.Edge
	for _, edge := range edges {
		if !hasAlternativePath(adjMap, edge.SourceID, edge.TargetID, edge.TargetID) {
			reducedEdges = append(reducedEdges, edge)
		}
	}

	return reducedEdges
}

// hasAlternativePath checks if there's a path from source to target that doesn't use the direct edge
func hasAlternativePath(adjMap map[idwrap.IDWrap][]idwrap.IDWrap, source, target, avoidTarget idwrap.IDWrap) bool {
	visited := make(map[idwrap.IDWrap]bool)
	var queue []idwrap.IDWrap

	// Start from source, explore all neighbors except the direct target
	for _, neighbor := range adjMap[source] {
		if neighbor != avoidTarget {
			queue = append(queue, neighbor)
		}
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current == target {
			return true // Found alternative path
		}

		if visited[current] {
			continue
		}
		visited[current] = true

		for _, neighbor := range adjMap[current] {
			if !visited[neighbor] {
				queue = append(queue, neighbor)
			}
		}
	}

	return false
}

