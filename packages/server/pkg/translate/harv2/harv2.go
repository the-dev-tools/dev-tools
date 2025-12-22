//nolint:revive // exported
package harv2

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	"the-dev-tools/server/pkg/depfinder" //nolint:gocritic // imports grouping

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/service/shttp"
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
	HTTPHeaders        []mhttp.HTTPHeader         `json:"http_headers"`
	HTTPSearchParams   []mhttp.HTTPSearchParam    `json:"http_search_params"`
	HTTPBodyForms      []mhttp.HTTPBodyForm       `json:"http_body_forms"`
	HTTPBodyUrlEncoded []mhttp.HTTPBodyUrlencoded `json:"http_body_urlencoded"`
	HTTPBodyRaws       []mhttp.HTTPBodyRaw        `json:"http_body_raws"`
	HTTPAsserts        []mhttp.HTTPAssert         `json:"http_asserts"`

	// File System (modern mfile.File model)
	Files []mfile.File `json:"files"`

	// Flow Items (preserving existing flow generation)
	Flow         mflow.Flow          `json:"flow"`
	Nodes        []mflow.Node        `json:"nodes"`
	RequestNodes []mflow.NodeRequest `json:"request_nodes"`
	Edges        []mflow.Edge        `json:"edges"`
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

func edgeExists(edges []mflow.Edge, source, target idwrap.IDWrap) bool {
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

// ConvertHARWithService converts HAR with overwrite detection using HTTP service
// This is the enhanced function that prevents duplicates and creates proper deltas
func ConvertHARWithService(ctx context.Context, har *HAR, workspaceID idwrap.IDWrap, httpService *shttp.HTTPService) (*HarResolved, error) {
	return ConvertHARWithDepFinderAndService(ctx, har, workspaceID, nil, httpService)
}

// ConvertHARWithDepFinderAndService converts HAR with dependency finding and overwrite detection
func ConvertHARWithDepFinderAndService(ctx context.Context, har *HAR, workspaceID idwrap.IDWrap, depFinder *depfinder.DepFinder, httpService *shttp.HTTPService) (*HarResolved, error) {
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

	// Initialize DepFinder if nil
	if depFinder == nil {
		df := depfinder.NewDepFinder()
		depFinder = &df
	}

	// Process entries and build the graph
	result, err := processEntriesWithService(ctx, entries, workspaceID, depFinder, httpService)
	if err != nil {
		return nil, fmt.Errorf("failed to process entries: %w", err)
	}

	// Create file for the flow
	if !mfile.IDIsZero(result.Flow.ID) {
		flowFile := mfile.File{
			ID:          result.Flow.ID,
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

	// Initialize DepFinder if nil
	if depFinder == nil {
		df := depfinder.NewDepFinder()
		depFinder = &df
	}

	// Process entries and build the graph
	result, err := processEntries(entries, workspaceID, depFinder)
	if err != nil {
		return nil, fmt.Errorf("failed to process entries: %w", err)
	}

	// Create file for the flow
	if !mfile.IDIsZero(result.Flow.ID) {
		flowFile := mfile.File{
			ID:          result.Flow.ID,
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

// processEntries converts HAR entries to entities and builds the dependency graph
func processEntries(entries []Entry, workspaceID idwrap.IDWrap, depFinder *depfinder.DepFinder) (*HarResolved, error) {
	result := &HarResolved{
		HTTPRequests:       make([]mhttp.HTTP, 0, len(entries)),
		HTTPHeaders:        make([]mhttp.HTTPHeader, 0),
		HTTPSearchParams:   make([]mhttp.HTTPSearchParam, 0),
		HTTPBodyForms:      make([]mhttp.HTTPBodyForm, 0),
		HTTPBodyUrlEncoded: make([]mhttp.HTTPBodyUrlencoded, 0),
		HTTPBodyRaws:       make([]mhttp.HTTPBodyRaw, 0),
		HTTPAsserts:        make([]mhttp.HTTPAssert, 0),
		Nodes:              make([]mflow.Node, 0),
		RequestNodes:       make([]mflow.NodeRequest, 0),
		Edges:              make([]mflow.Edge, 0),
	}

	// Create Flow
	flowID := idwrap.NewNow()
	result.Flow = mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Imported HAR Flow",
		Duration:    0,
	}

	// 1. Create Start Node
	startNodeID := idwrap.NewNow()
	startNode := mflow.Node{
		ID:        startNodeID,
		FlowID:    flowID,
		Name:      "Start",
		NodeKind:  mflow.NODE_KIND_MANUAL_START,
		PositionX: 0,
		PositionY: 0,
	}
	result.Nodes = append(result.Nodes, startNode)

	// Tracking variables for dependency rules
	var previousNodeID *idwrap.IDWrap
	var previousTimestamp *time.Time
	var lastMutationNodeID *idwrap.IDWrap

	fileMap := make(map[string]mfile.File)
	folderMap := make(map[string]idwrap.IDWrap)
	folderFileMap := make(map[string]mfile.File)

	// Layout parameters
	const nodeSpacingX = 300
	currentX := float64(nodeSpacingX) // Start after Start node

	// Global counter for node naming
	nodeCounter := 0

	for i, entry := range entries {
		nodeCounter++

		// 1. Create Raw (Base) Request - No DepFinder
		baseReq, baseHeaders, baseParams, baseBodyForms, baseBodyUrlEncoded, baseBodyRaws, _, err := createHTTPRequestFromEntryWithDeps(entry, workspaceID, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create base request for entry %d: %w", i, err)
		}

		// 2. Create Templated (Delta) Request - With DepFinder
		templatedReq, templatedHeaders, templatedParams, templatedBodyForms, templatedBodyUrlEncoded, templatedBodyRaws, deps, err := createHTTPRequestFromEntryWithDeps(entry, workspaceID, depFinder)
		if err != nil {
			return nil, fmt.Errorf("failed to create templated request for entry %d: %w", i, err)
		}

		// Create Node
		nodeID := idwrap.NewNow()
		node := mflow.Node{
			ID:        nodeID,
			FlowID:    flowID,
			Name:      fmt.Sprintf("request_%d", nodeCounter),
			NodeKind:  mflow.NODE_KIND_REQUEST,
			PositionX: currentX,
			PositionY: 0, // Will be refined later if we implement sophisticated layout, currently horizontal
		}
		currentX += nodeSpacingX

		// 3. Create Delta Request Object (links to Base)
		deltaReq := createDeltaVersion(*baseReq)

		// Apply templated values to Delta Request
		if templatedReq.Url != baseReq.Url {
			deltaReq.Url = templatedReq.Url
			deltaReq.DeltaUrl = &templatedReq.Url
		}
		// We could also check Method/BodyKind but typically URL/Body are the dependency targets.

		// 4. Calculate Delta Components
		deltaHeaders := CreateDeltaHeaders(baseHeaders, templatedHeaders, deltaReq.ID)
		deltaParams := CreateDeltaSearchParams(baseParams, templatedParams, deltaReq.ID)
		deltaBodyForms := CreateDeltaBodyForms(baseBodyForms, templatedBodyForms, deltaReq.ID)
		deltaBodyUrlEncoded := CreateDeltaBodyUrlEncoded(baseBodyUrlEncoded, templatedBodyUrlEncoded, deltaReq.ID)

		// Raw body is singular, handle separately
		var baseRaw, templatedRaw *mhttp.HTTPBodyRaw
		if len(baseBodyRaws) > 0 {
			baseRaw = &baseBodyRaws[0]
		}
		if len(templatedBodyRaws) > 0 {
			templatedRaw = &templatedBodyRaws[0]
		}
		deltaRaw := CreateDeltaBodyRaw(baseRaw, templatedRaw, deltaReq.ID)

		// Create Request Node config
		reqNode := mflow.NodeRequest{
			FlowNodeID:  nodeID,
			HttpID:      &baseReq.ID,
			DeltaHttpID: &deltaReq.ID,
		}

		// 5. Add to Result
		result.Nodes = append(result.Nodes, node)
		result.RequestNodes = append(result.RequestNodes, reqNode)
		result.HTTPRequests = append(result.HTTPRequests, *baseReq, deltaReq)

		// Add Base Components
		result.HTTPHeaders = append(result.HTTPHeaders, baseHeaders...)
		result.HTTPSearchParams = append(result.HTTPSearchParams, baseParams...)
		result.HTTPBodyForms = append(result.HTTPBodyForms, baseBodyForms...)
		result.HTTPBodyUrlEncoded = append(result.HTTPBodyUrlEncoded, baseBodyUrlEncoded...)
		result.HTTPBodyRaws = append(result.HTTPBodyRaws, baseBodyRaws...)

		// Add Delta Components
		result.HTTPHeaders = append(result.HTTPHeaders, deltaHeaders...)
		result.HTTPSearchParams = append(result.HTTPSearchParams, deltaParams...)
		result.HTTPBodyForms = append(result.HTTPBodyForms, deltaBodyForms...)
		result.HTTPBodyUrlEncoded = append(result.HTTPBodyUrlEncoded, deltaBodyUrlEncoded...)
		if deltaRaw != nil {
			result.HTTPBodyRaws = append(result.HTTPBodyRaws, *deltaRaw)
		}

		// Create assertion for response status code if HAR has a valid response status
		if entry.Response.Status > 0 {
			baseAssert, deltaAssert := createStatusAssertions(baseReq.ID, deltaReq.ID, entry.Response.Status, i)
			result.HTTPAsserts = append(result.HTTPAsserts, baseAssert, deltaAssert)
		}

		// File System
		file, _, err := createFileStructure(baseReq, workspaceID, folderMap, folderFileMap)
		if err != nil {
			return nil, fmt.Errorf("failed to create file structure for entry %d: %w", i, err)
		}
		fileMap[baseReq.ID.String()] = *file

		// Create File for Delta
		deltaFile := mfile.File{
			ID:          deltaReq.ID,
			WorkspaceID: workspaceID,
			ParentID:    &file.ID,
			ContentID:   &deltaReq.ID,
			ContentType: mfile.ContentTypeHTTPDelta,
			Name:        deltaReq.Name,
			Order:       file.Order,
			UpdatedAt:   time.Now(),
		}
		fileMap[deltaReq.ID.String()] = deltaFile

		// --- Dependency Logic ---

		// 1. Data Dependency (Edges from DepFinder)
		for _, couple := range deps {
			if !edgeExists(result.Edges, couple.NodeID, nodeID) {
				addEdge(result, flowID, couple.NodeID, nodeID)
			}
		}

		// 2. Timestamp Sequencing
		currentTimestamp := entry.StartedDateTime
		if previousNodeID != nil && previousTimestamp != nil {
			timeDiff := currentTimestamp.Sub(*previousTimestamp)
			if timeDiff >= 0 && timeDiff <= TimestampSequencingThreshold {
				addEdge(result, flowID, *previousNodeID, nodeID)
			}
		}

		// 3. Mutation Chain
		if isMutationMethod(baseReq.Method) {
			if lastMutationNodeID != nil && *lastMutationNodeID != nodeID {
				// Avoid duplicate edges if already connected via timestamp
				if !edgeExists(result.Edges, *lastMutationNodeID, nodeID) {
					addEdge(result, flowID, *lastMutationNodeID, nodeID)
				}
			}
			lastMutationID := nodeID
			lastMutationNodeID = &lastMutationID
		} else if requiresSequentialOrdering(baseReq.Method) && previousNodeID != nil {
			// Strict ordering for DELETE etc if not caught by mutation chain
			if !edgeExists(result.Edges, *previousNodeID, nodeID) {
				addEdge(result, flowID, *previousNodeID, nodeID)
			}
		}

		// Update tracking
		previousNodeID = &nodeID
		previousTimestamp = &currentTimestamp

		// --- End Dependency Logic ---

		// Add response to DepFinder for future requests
		if entry.Response.Content.Text != "" {
			// Try to parse as JSON
			if strings.Contains(entry.Response.Content.MimeType, "json") ||
				strings.HasPrefix(strings.TrimSpace(entry.Response.Content.Text), "{") {
				path := fmt.Sprintf("%s.%s.%s", node.Name, "response", "body")
				couple := depfinder.VarCouple{Path: path, NodeID: nodeID}
				_ = depFinder.AddJsonBytes([]byte(entry.Response.Content.Text), couple)
			}
		}
		// Add headers to DepFinder? (Optional, can add if needed)
	}

	// Add folder files to result
	for _, folderFile := range folderFileMap {
		result.Files = append(result.Files, folderFile)
	}

	// Sort files
	sortedFiles := make([]mfile.File, 0, len(fileMap))
	for _, file := range fileMap {
		sortedFiles = append(sortedFiles, file)
	}
	sort.Slice(sortedFiles, func(i, j int) bool {
		return sortedFiles[i].Order < sortedFiles[j].Order
	})
	result.Files = append(result.Files, sortedFiles...)

	// 4. Rooting (Connect orphans to Start) & Cleanup
	if err := finalizeGraph(result, startNodeID, flowID); err != nil {
		return nil, err
	}

	// 5. Reorganize Positions
	if err := ReorganizeNodePositions(result); err != nil {
		return nil, err
	}

	return result, nil
}

// processEntriesWithService converts HAR entries to entities with overwrite detection
func processEntriesWithService(ctx context.Context, entries []Entry, workspaceID idwrap.IDWrap, depFinder *depfinder.DepFinder, httpService *shttp.HTTPService) (*HarResolved, error) { //nolint:gocritic // hugeParam
	result := &HarResolved{
		HTTPRequests:       make([]mhttp.HTTP, 0, len(entries)),
		HTTPHeaders:        make([]mhttp.HTTPHeader, 0),
		HTTPSearchParams:   make([]mhttp.HTTPSearchParam, 0),
		HTTPBodyForms:      make([]mhttp.HTTPBodyForm, 0),
		HTTPBodyUrlEncoded: make([]mhttp.HTTPBodyUrlencoded, 0),
		HTTPBodyRaws:       make([]mhttp.HTTPBodyRaw, 0),
		HTTPAsserts:        make([]mhttp.HTTPAssert, 0),
		Nodes:              make([]mflow.Node, 0),
		RequestNodes:       make([]mflow.NodeRequest, 0),
		Edges:              make([]mflow.Edge, 0),
	}

	// Create Flow
	flowID := idwrap.NewNow()
	result.Flow = mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Imported HAR Flow",
		Duration:    0,
	}

	// 1. Create Start Node
	startNodeID := idwrap.NewNow()
	startNode := mflow.Node{
		ID:        startNodeID,
		FlowID:    flowID,
		Name:      "Start",
		NodeKind:  mflow.NODE_KIND_MANUAL_START,
		PositionX: 0,
		PositionY: 0,
	}
	result.Nodes = append(result.Nodes, startNode)

	// Tracking variables for dependency rules
	var previousNodeID *idwrap.IDWrap
	var previousTimestamp *time.Time
	var lastMutationNodeID *idwrap.IDWrap

	fileMap := make(map[string]mfile.File)
	folderMap := make(map[string]idwrap.IDWrap)
	folderFileMap := make(map[string]mfile.File)

	// Layout parameters
	const nodeSpacingX = 300
	currentX := float64(nodeSpacingX) // Start after Start node

	// Global counter for node naming
	nodeCounter := 0

	for i, entry := range entries {
		nodeCounter++

		// 1. Create Raw (Base) Request - No DepFinder
		baseReqRaw, baseHeadersRaw, baseParamsRaw, baseBodyFormsRaw, baseBodyUrlEncodedRaw, baseBodyRawsRaw, _, err := createHTTPRequestFromEntryWithDeps(entry, workspaceID, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create raw request for entry %d: %w", i, err)
		}

		// 2. Create Templated (Delta) Request - With DepFinder
		templatedReq, templatedHeaders, templatedParams, templatedBodyForms, templatedBodyUrlEncoded, templatedBodyRaws, deps, err := createHTTPRequestFromEntryWithDeps(entry, workspaceID, depFinder)
		if err != nil {
			return nil, fmt.Errorf("failed to create templated request for entry %d: %w", i, err)
		}

		// Check for existing request for overwrite detection
		var existingRequest *mhttp.HTTP
		var existingHeaders []mhttp.HTTPHeader
		var existingParams []mhttp.HTTPSearchParam
		var existingBodyForms []mhttp.HTTPBodyForm
		var existingBodyUrlEncoded []mhttp.HTTPBodyUrlencoded
		var existingBodyRaws []mhttp.HTTPBodyRaw

		if httpService != nil {
			existing, err := httpService.FindByURLAndMethod(ctx, workspaceID, baseReqRaw.Url, baseReqRaw.Method)
			if err == nil {
				existingRequest = existing

				// Load existing child entities to ensure accurate delta comparison (Index-based lookups)
				if h, err := httpService.GetHeadersByHttpID(ctx, existing.ID); err == nil {
					existingHeaders = h
				}
				if p, err := httpService.GetSearchParamsByHttpID(ctx, existing.ID); err == nil {
					existingParams = p
				}
				if f, err := httpService.GetBodyFormsByHttpID(ctx, existing.ID); err == nil {
					existingBodyForms = f
				}
				if u, err := httpService.GetBodyUrlEncodedByHttpID(ctx, existing.ID); err == nil {
					existingBodyUrlEncoded = u
				}
				if r, err := httpService.GetBodyRawByHttpID(ctx, existing.ID); err == nil && r != nil {
					existingBodyRaws = []mhttp.HTTPBodyRaw{*r}
				}
			}
		}

		var baseRequest *mhttp.HTTP
		var deltaReq *mhttp.HTTP

		// Child entities to use for Delta comparison (Base vs Templated)
		var comparisonBaseHeaders []mhttp.HTTPHeader
		var comparisonBaseParams []mhttp.HTTPSearchParam
		var comparisonBaseBodyForms []mhttp.HTTPBodyForm
		var comparisonBaseBodyUrlEncoded []mhttp.HTTPBodyUrlencoded
		var comparisonBaseBodyRaws []mhttp.HTTPBodyRaw

		if existingRequest != nil {
			// Use existing request as base, create delta for new request
			baseRequest = existingRequest
			comparisonBaseHeaders = existingHeaders
			comparisonBaseParams = existingParams
			comparisonBaseBodyForms = existingBodyForms
			comparisonBaseBodyUrlEncoded = existingBodyUrlEncoded
			comparisonBaseBodyRaws = existingBodyRaws

			// Check for existing Delta to overwrite
			var existingDeltaID idwrap.IDWrap
			if httpService != nil {
				deltas, err := httpService.GetDeltasByParentID(ctx, existingRequest.ID)
				if err == nil && len(deltas) > 0 {
					// For now, we just pick the first delta found as the "default" overwrite target.
					// Future improvement: could match by name "Import Delta" or similar if we support multiple deltas.
					existingDeltaID = deltas[0].ID
				}
			}

			if existingDeltaID.Compare(idwrap.IDWrap{}) == 0 {
				existingDeltaID = idwrap.NewNow()
			}

			// Create delta with only the fields that actually differ from the existing base
			deltaReq = &mhttp.HTTP{
				ID:           existingDeltaID,
				WorkspaceID:  workspaceID,
				ParentHttpID: &existingRequest.ID,
				Name:         templatedReq.Name + " (Delta)",
				Url:          templatedReq.Url,
				Method:       templatedReq.Method,
				Description:  templatedReq.Description + " [Import Delta]",
				BodyKind:     templatedReq.BodyKind,
				IsDelta:      true,
				CreatedAt:    templatedReq.CreatedAt,
				UpdatedAt:    templatedReq.UpdatedAt,
			}

			// Only set Delta* fields when there's an actual difference from the base
			if templatedReq.Url != existingRequest.Url {
				deltaReq.DeltaUrl = &templatedReq.Url
			}
			if templatedReq.Method != existingRequest.Method {
				deltaReq.DeltaMethod = &templatedReq.Method
			}
			if templatedReq.Name != existingRequest.Name {
				deltaReq.DeltaName = &templatedReq.Name
			}
			if templatedReq.Description != existingRequest.Description {
				deltaReq.DeltaDescription = &templatedReq.Description
			}
			if templatedReq.BodyKind != existingRequest.BodyKind {
				deltaReq.DeltaBodyKind = &templatedReq.BodyKind
			}
		} else {
			// No existing request, use new request as base
			baseRequest = baseReqRaw
			comparisonBaseHeaders = baseHeadersRaw
			comparisonBaseParams = baseParamsRaw
			comparisonBaseBodyForms = baseBodyFormsRaw
			comparisonBaseBodyUrlEncoded = baseBodyUrlEncodedRaw
			comparisonBaseBodyRaws = baseBodyRawsRaw

			// Create standard delta version
			baseDelta := createDeltaVersion(*baseRequest)
			deltaReq = &baseDelta

			// Apply templated values to Delta Request
			if templatedReq.Url != baseRequest.Url {
				deltaReq.Url = templatedReq.Url
				deltaReq.DeltaUrl = &templatedReq.Url
			}
		}

		// Add both base and delta requests to result
		if existingRequest == nil {
			result.HTTPRequests = append(result.HTTPRequests, *baseRequest)
			// Add Base Children
			result.HTTPHeaders = append(result.HTTPHeaders, baseHeadersRaw...)
			result.HTTPSearchParams = append(result.HTTPSearchParams, baseParamsRaw...)
			result.HTTPBodyForms = append(result.HTTPBodyForms, baseBodyFormsRaw...)
			result.HTTPBodyUrlEncoded = append(result.HTTPBodyUrlEncoded, baseBodyUrlEncodedRaw...)
			result.HTTPBodyRaws = append(result.HTTPBodyRaws, baseBodyRawsRaw...)
		}
		result.HTTPRequests = append(result.HTTPRequests, *deltaReq)

		// Create delta child entities (comparing Base vs Templated)
		deltaHeaders := CreateDeltaHeaders(comparisonBaseHeaders, templatedHeaders, deltaReq.ID)
		result.HTTPHeaders = append(result.HTTPHeaders, deltaHeaders...)

		deltaParams := CreateDeltaSearchParams(comparisonBaseParams, templatedParams, deltaReq.ID)
		result.HTTPSearchParams = append(result.HTTPSearchParams, deltaParams...)

		deltaForms := CreateDeltaBodyForms(comparisonBaseBodyForms, templatedBodyForms, deltaReq.ID)
		result.HTTPBodyForms = append(result.HTTPBodyForms, deltaForms...)

		deltaEncoded := CreateDeltaBodyUrlEncoded(comparisonBaseBodyUrlEncoded, templatedBodyUrlEncoded, deltaReq.ID)
		result.HTTPBodyUrlEncoded = append(result.HTTPBodyUrlEncoded, deltaEncoded...)

		var baseRaw *mhttp.HTTPBodyRaw
		if len(comparisonBaseBodyRaws) > 0 {
			baseRaw = &comparisonBaseBodyRaws[0]
		}
		var templatedRaw *mhttp.HTTPBodyRaw
		if len(templatedBodyRaws) > 0 {
			templatedRaw = &templatedBodyRaws[0]
		}
		deltaRaw := CreateDeltaBodyRaw(baseRaw, templatedRaw, deltaReq.ID)
		if deltaRaw != nil {
			result.HTTPBodyRaws = append(result.HTTPBodyRaws, *deltaRaw)
		}

		// Create assertion for response status code if HAR has a valid response status
		if entry.Response.Status > 0 {
			baseAssert, deltaAssert := createStatusAssertions(baseRequest.ID, deltaReq.ID, entry.Response.Status, i)
			result.HTTPAsserts = append(result.HTTPAsserts, baseAssert, deltaAssert)
		}

		// Create Node
		nodeID := idwrap.NewNow()
		node := mflow.Node{
			ID:        nodeID,
			FlowID:    flowID,
			Name:      fmt.Sprintf("request_%d", nodeCounter),
			NodeKind:  mflow.NODE_KIND_REQUEST,
			PositionX: currentX,
			PositionY: 0,
		}
		currentX += nodeSpacingX

		// Create Request Node config
		reqNode := mflow.NodeRequest{
			FlowNodeID:  nodeID,
			HttpID:      &baseRequest.ID,
			DeltaHttpID: &deltaReq.ID,
		}

		// Append to result
		result.Nodes = append(result.Nodes, node)
		result.RequestNodes = append(result.RequestNodes, reqNode)

		// File System
		file, _, err := createFileStructure(baseRequest, workspaceID, folderMap, folderFileMap)
		if err != nil {
			return nil, fmt.Errorf("failed to create file structure for entry %d: %w", i, err)
		}
		fileMap[baseRequest.ID.String()] = *file

		// Create File for Delta
		deltaFile := mfile.File{
			ID:          deltaReq.ID,
			WorkspaceID: workspaceID,
			ParentID:    &file.ID,
			ContentID:   &deltaReq.ID,
			ContentType: mfile.ContentTypeHTTPDelta,
			Name:        deltaReq.Name,
			Order:       file.Order,
			UpdatedAt:   time.Now(),
		}
		fileMap[deltaReq.ID.String()] = deltaFile

		// --- Dependency Logic (same as original) ---

		// 1. Data Dependency (Edges from DepFinder)
		for _, couple := range deps {
			if !edgeExists(result.Edges, couple.NodeID, nodeID) {
				addEdge(result, flowID, couple.NodeID, nodeID)
			}
		}

		// Timestamp Sequencing
		currentTimestamp := entry.StartedDateTime
		if previousNodeID != nil && previousTimestamp != nil {
			timeDiff := currentTimestamp.Sub(*previousTimestamp)
			if timeDiff >= 0 && timeDiff <= TimestampSequencingThreshold {
				addEdge(result, flowID, *previousNodeID, nodeID)
			}
		}

		// Mutation Chain
		if isMutationMethod(baseRequest.Method) {
			if lastMutationNodeID != nil && *lastMutationNodeID != nodeID {
				if !edgeExists(result.Edges, *lastMutationNodeID, nodeID) {
					addEdge(result, flowID, *lastMutationNodeID, nodeID)
				}
			}
			lastMutationID := nodeID
			lastMutationNodeID = &lastMutationID
		} else if requiresSequentialOrdering(baseRequest.Method) && previousNodeID != nil {
			if !edgeExists(result.Edges, *previousNodeID, nodeID) {
				addEdge(result, flowID, *previousNodeID, nodeID)
			}
		}

		// Update tracking
		previousNodeID = &nodeID
		previousTimestamp = &currentTimestamp

		// Add response to DepFinder for future requests
		if entry.Response.Content.Text != "" {
			if strings.Contains(entry.Response.Content.MimeType, "json") ||
				strings.HasPrefix(strings.TrimSpace(entry.Response.Content.Text), "{") {
				path := fmt.Sprintf("%s.%s.%s", node.Name, "response", "body")
				couple := depfinder.VarCouple{Path: path, NodeID: nodeID}
				_ = depFinder.AddJsonBytes([]byte(entry.Response.Content.Text), couple)
			}
		}
	}

	// Add folder files to result
	for _, folderFile := range folderFileMap {
		result.Files = append(result.Files, folderFile)
	}

	// Sort files
	sortedFiles := make([]mfile.File, 0, len(fileMap))
	for _, file := range fileMap {
		sortedFiles = append(sortedFiles, file)
	}
	sort.Slice(sortedFiles, func(i, j int) bool {
		return sortedFiles[i].Order < sortedFiles[j].Order
	})
	result.Files = append(result.Files, sortedFiles...)

	// Rooting and positioning
	if err := finalizeGraph(result, startNodeID, flowID); err != nil {
		return nil, err
	}

	if err := ReorganizeNodePositions(result); err != nil {
		return nil, err
	}

	return result, nil
}

func addEdge(result *HarResolved, flowID, sourceID, targetID idwrap.IDWrap) {
	result.Edges = append(result.Edges, mflow.Edge{
		ID:            idwrap.NewNow(),
		FlowID:        flowID,
		SourceID:      sourceID,
		TargetID:      targetID,
		SourceHandler: mflow.HandleUnspecified,
	})
}

// finalizeGraph connects orphans to start node and performs cleanup
func finalizeGraph(result *HarResolved, startNodeID idwrap.IDWrap, flowID idwrap.IDWrap) error {
	// 1. Transitive Reduction
	result.Edges = applyTransitiveReduction(result.Edges)

	// 2. Rooting (Connect orphans to Start)
	hasIncoming := make(map[idwrap.IDWrap]bool)
	for _, e := range result.Edges {
		hasIncoming[e.TargetID] = true
	}

	for _, node := range result.Nodes {
		if node.ID == startNodeID {
			continue
		}
		if !hasIncoming[node.ID] {
			addEdge(result, flowID, startNodeID, node.ID)
		}
	}

	return nil
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
		ID:          httpReq.ID,
		WorkspaceID: workspaceID,
		ParentID:    &folderID,
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
	allSegments := make([]string, 0, len(hostParts)+len(cleanSegments))
	allSegments = append(allSegments, hostParts...)
	allSegments = append(allSegments, cleanSegments...)
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
		ParentID:    parentID,
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

// ReorganizeNodePositions positions flow nodes using a level-based horizontal layout.
// Sequential nodes flow left-to-right, parallel nodes are stacked vertically.
func ReorganizeNodePositions(result *HarResolved) error {
	const (
		nodeSpacingX = 200 // Horizontal spacing between sequential levels
		nodeSpacingY = 150 // Vertical spacing between parallel nodes
		startX       = 0   // Starting X position
		startY       = 0   // Starting Y position
	)

	// Map for quick node lookup
	nodeMap := make(map[idwrap.IDWrap]*mflow.Node)
	for i := range result.Nodes {
		nodeMap[result.Nodes[i].ID] = &result.Nodes[i]
	}

	// Find start node
	var startNode *mflow.Node
	for i := range result.Nodes {
		if result.Nodes[i].NodeKind == mflow.NODE_KIND_MANUAL_START {
			startNode = &result.Nodes[i]
			break
		}
	}
	if startNode == nil {
		return fmt.Errorf("start node not found")
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

	// Safety counter to prevent infinite loops on cyclic graphs
	processedCount := 0
	maxProcessed := len(result.Nodes) * len(result.Nodes)
	if maxProcessed < 10000 {
		maxProcessed = 10000
	}

	for len(queue) > 0 {
		// Check for potential infinite loop
		if processedCount > maxProcessed {
			break
		}
		processedCount++

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

	// Position nodes level by level (horizontal flow: left-to-right)
	for level := 0; level <= len(levelNodes)-1; level++ {
		nodes := levelNodes[level]
		if len(nodes) == 0 {
			continue
		}

		// Calculate X position for this level (sequential nodes go right)
		xPos := float64(startX + level*nodeSpacingX)

		// Calculate starting Y position to center the nodes at this level
		totalHeight := float64((len(nodes) - 1) * nodeSpacingY)
		startYForLevel := float64(startY) - totalHeight/2

		// Position each node in this level
		for i, nodeID := range nodes {
			if node := nodeMap[nodeID]; node != nil {
				node.PositionX = xPos
				node.PositionY = startYForLevel + float64(i*nodeSpacingY)
			}
		}
	}

	return nil
}

// applyTransitiveReduction removes redundant edges from the graph
func applyTransitiveReduction(edges []mflow.Edge) []mflow.Edge {
	if len(edges) == 0 {
		return edges
	}

	// Performance optimization: Skip reduction for large graphs to avoid O(E^2) complexity
	// 2000 edges is roughly where performance starts to degrade noticeably (>1s)
	if len(edges) > 2000 {
		return edges
	}

	// Build adjacency map
	adjMap := make(map[idwrap.IDWrap][]idwrap.IDWrap)
	for _, edge := range edges {
		adjMap[edge.SourceID] = append(adjMap[edge.SourceID], edge.TargetID)
	}

	// For each edge, check if there's an alternative path
	var reducedEdges []mflow.Edge
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
			visited[neighbor] = true
		}
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current == target {
			return true // Found alternative path
		}

		for _, neighbor := range adjMap[current] {
			if !visited[neighbor] {
				visited[neighbor] = true
				queue = append(queue, neighbor)
			}
		}
	}

	return false
}

// createStatusAssertions creates base and delta assertions for HTTP response status code
func createStatusAssertions(baseHttpID, deltaHttpID idwrap.IDWrap, statusCode int, entryIndex int) (mhttp.HTTPAssert, mhttp.HTTPAssert) {
	now := time.Now().Unix()
	// Format the assertion expression as "response.status == XXX" where XXX is the status code
	assertExpr := fmt.Sprintf("response.status == %d", statusCode)

	baseAssertID := idwrap.NewNow()
	baseAssert := mhttp.HTTPAssert{
		ID:           baseAssertID,
		HttpID:       baseHttpID,
		Value:        assertExpr,
		Enabled:      true,
		Description:  fmt.Sprintf("Verify response status is %d (from HAR import)", statusCode),
		DisplayOrder: float32(entryIndex),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	deltaAssert := mhttp.HTTPAssert{
		ID:                 idwrap.NewNow(),
		HttpID:             deltaHttpID,
		Value:              assertExpr,
		Enabled:            true,
		Description:        fmt.Sprintf("Verify response status is %d (from HAR import)", statusCode),
		DisplayOrder:       float32(entryIndex),
		ParentHttpAssertID: &baseAssertID,
		IsDelta:            true,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	return baseAssert, deltaAssert
}
