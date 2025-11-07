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
	"the-dev-tools/server/pkg/model/mitemfolder"
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

	// File System (modern mfile.File model)
	Files []mfile.File `json:"files"`

	// Flow Items (preserving existing flow generation)
	Flow         mflow.Flow         `json:"flow"`
	Nodes        []mnnode.MNode     `json:"nodes"`
	RequestNodes []mnrequest.MNRequest `json:"request_nodes"`
	Edges        []edge.Edge        `json:"edges"`
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

	result := &HarResolved{}

	// Sort entries by started date for proper sequencing
	entries := make([]Entry, len(har.Log.Entries))
	copy(entries, har.Log.Entries)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].StartedDateTime.Before(entries[j].StartedDateTime)
	})

	// Process entries and create HTTP requests and file structure
	httpReqs, files, err := processEntriesToHTTP(entries, workspaceID, depFinder)
	if err != nil {
		return nil, fmt.Errorf("failed to process entries: %w", err)
	}

	result.HTTPRequests = httpReqs
	result.Files = files

	// Generate flow nodes and edges
	if err := generateFlowGraph(result, entries, httpReqs); err != nil {
		return nil, fmt.Errorf("failed to generate flow graph: %w", err)
	}

	return result, nil
}

// processEntriesToHTTP converts HAR entries to mhttp.HTTP entities and file structure
func processEntriesToHTTP(entries []Entry, workspaceID idwrap.IDWrap, depFinder *depfinder.DepFinder) ([]mhttp.HTTP, []mfile.File, error) {
	httpRequests := make([]mhttp.HTTP, 0, len(entries))
	fileMap := make(map[string]mfile.File)
	folderMap := make(map[string]idwrap.IDWrap)

	for i, entry := range entries {
		// Create HTTP request
		httpReq, err := createHTTPRequestFromEntry(entry, workspaceID)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create HTTP request for entry %d: %w", i, err)
		}

		// Create delta version (always create both original and delta)
		deltaReq := createDeltaVersion(*httpReq)

		httpRequests = append(httpRequests, *httpReq, deltaReq)

		// Create file system structure
		file, _, err := createFileStructure(httpReq, workspaceID, folderMap)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create file structure for entry %d: %w", i, err)
		}

		fileMap[httpReq.ID.String()] = *file
	}

	// Sort files for consistent ordering
	sortedFiles := make([]mfile.File, 0, len(fileMap))
	for _, file := range fileMap {
		sortedFiles = append(sortedFiles, file)
	}
	sort.Slice(sortedFiles, func(i, j int) bool {
		return sortedFiles[i].Order < sortedFiles[j].Order
	})

	return httpRequests, sortedFiles, nil
}

// createHTTPRequestFromEntry creates an mhttp.HTTP entity from a HAR entry
func createHTTPRequestFromEntry(entry Entry, workspaceID idwrap.IDWrap) (*mhttp.HTTP, error) {
	parsedURL, err := url.Parse(entry.Request.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL %s: %w", entry.Request.URL, err)
	}

	// Generate a descriptive name from the URL
	name := generateRequestName(entry.Request.Method, parsedURL)

	httpReq := &mhttp.HTTP{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		Name:        name,
		Url:         entry.Request.URL,
		Method:      entry.Request.Method,
		Description: fmt.Sprintf("Imported from HAR - %s %s", entry.Request.Method, entry.Request.URL),
		IsDelta:     false,
		CreatedAt:   entry.StartedDateTime.UnixMilli(),
		UpdatedAt:   entry.StartedDateTime.UnixMilli(),
	}

	return httpReq, nil
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
func createFileStructure(httpReq *mhttp.HTTP, workspaceID idwrap.IDWrap, folderMap map[string]idwrap.IDWrap) (*mfile.File, string, error) {
	parsedURL, err := url.Parse(httpReq.Url)
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse URL %s: %w", httpReq.Url, err)
	}

	// Build folder path from URL structure
	folderPath := buildFolderPathFromURL(parsedURL)
	folderID, err := getOrCreateFolder(folderPath, workspaceID, folderMap)
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
		ContentKind: mfile.ContentKindAPI, // This maps to the old item_api
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
func getOrCreateFolder(folderPath string, workspaceID idwrap.IDWrap, folderMap map[string]idwrap.IDWrap) (idwrap.IDWrap, error) {
	if existingID, exists := folderMap[folderPath]; exists {
		return existingID, nil
	}

	// Create new folder - simplified structure using collection-based folder model
	collectionID := idwrap.NewNow() // Use a temporary collection ID
	folder := &mitemfolder.ItemFolder{
		ID:           idwrap.NewNow(),
		CollectionID: collectionID,
		Name:         path.Base(folderPath),
	}

	// TODO: In a real implementation, we would need to persist this folder
	// For now, we'll just use the ID in our map
	folderMap[folderPath] = folder.ID

	// Create parent folders if needed
	parentPath := path.Dir(folderPath)
	if parentPath != "/" && parentPath != "." {
		parentID, err := getOrCreateFolder(parentPath, workspaceID, folderMap)
		if err != nil {
			return idwrap.IDWrap{}, err
		}
		folder.ParentID = &parentID
	}

	return folder.ID, nil
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

