//nolint:revive // exported
package topenapiv2

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mfile"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"

	"gopkg.in/yaml.v3"
)

// OpenAPIResolved contains all resolved HTTP requests and associated data from an OpenAPI/Swagger spec
type OpenAPIResolved struct {
	HTTPRequests []mhttp.HTTP
	Headers      []mhttp.HTTPHeader
	SearchParams []mhttp.HTTPSearchParam
	BodyRaw      []mhttp.HTTPBodyRaw
	Asserts      []mhttp.HTTPAssert
	Files        []mfile.File
	Flow         mflow.Flow
	Nodes        []mflow.Node
	RequestNodes []mflow.NodeRequest
	Edges        []mflow.Edge
}

// ConvertOptions defines configuration for OpenAPI spec conversion
type ConvertOptions struct {
	WorkspaceID idwrap.IDWrap
	FolderID    *idwrap.IDWrap
}

// ConvertOpenAPI converts OpenAPI/Swagger spec data (JSON or YAML) to HTTP models.
// Supports both Swagger 2.0 and OpenAPI 3.x formats.
func ConvertOpenAPI(data []byte, opts ConvertOptions) (*OpenAPIResolved, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty spec data")
	}

	spec, err := parseSpec(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OpenAPI spec: %w", err)
	}

	return convertSpec(spec, opts)
}

// --- Spec Parsing ---

// spec is a normalized internal representation for both Swagger 2.0 and OpenAPI 3.x
type spec struct {
	Title   string
	BaseURL string
	Paths   map[string]pathItem
}

// pathItem maps HTTP methods to operations
type pathItem struct {
	Operations map[string]operation // key: HTTP method (GET, POST, etc.)
}

// operation represents a single API operation
type operation struct {
	Summary     string
	OperationID string
	Parameters  []parameter
	RequestBody *requestBody
	Responses   map[string]response
}

// parameter represents an API parameter
type parameter struct {
	Name     string
	In       string // query, header, path, cookie
	Required bool
	Schema   *schemaObj
	Example  string
}

// requestBody represents the request body
type requestBody struct {
	ContentType string
	Example     string
	Schema      *schemaObj
}

// response represents an API response
type response struct {
	Description string
}

// schemaObj is a minimal schema representation to extract example values
type schemaObj struct {
	Type       string
	Example    interface{}
	Properties map[string]*schemaObj
}

// parseSpec parses raw data (JSON or YAML) into our normalized spec.
func parseSpec(data []byte) (*spec, error) {
	// Try JSON first, then YAML
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return nil, fmt.Errorf("data is neither valid JSON nor valid YAML: %w", err)
		}
	}

	if v, ok := raw["swagger"]; ok {
		if s, ok := v.(string); ok && strings.HasPrefix(s, "2") {
			return parseSwagger2(raw)
		}
	}

	if v, ok := raw["openapi"]; ok {
		if s, ok := v.(string); ok && strings.HasPrefix(s, "3") {
			return parseOpenAPI3(raw)
		}
	}

	return nil, fmt.Errorf("unrecognized spec format: missing 'swagger' or 'openapi' field")
}

// parseSwagger2 parses a Swagger 2.0 spec.
func parseSwagger2(raw map[string]interface{}) (*spec, error) {
	s := &spec{
		Paths: make(map[string]pathItem),
	}

	// Extract title
	if info, ok := raw["info"].(map[string]interface{}); ok {
		if title, ok := info["title"].(string); ok {
			s.Title = title
		}
	}

	// Build base URL from host, basePath, schemes
	scheme := "https"
	if schemes, ok := raw["schemes"].([]interface{}); ok && len(schemes) > 0 {
		if first, ok := schemes[0].(string); ok {
			scheme = first
		}
	}
	host, _ := raw["host"].(string)
	basePath, _ := raw["basePath"].(string)
	if host != "" {
		s.BaseURL = scheme + "://" + host + basePath
	}

	// Parse paths
	paths, ok := raw["paths"].(map[string]interface{})
	if !ok {
		return s, nil
	}

	for pathStr, pathData := range paths {
		pathMap, ok := pathData.(map[string]interface{})
		if !ok {
			continue
		}
		pi := pathItem{Operations: make(map[string]operation)}

		// Collect path-level parameters (shared by all operations on this path)
		var pathParams []parameter
		if pathParamsRaw, ok := pathMap["parameters"].([]interface{}); ok {
			pathParams = parseParameters(pathParamsRaw)
		}

		for method, opData := range pathMap {
			method = strings.ToUpper(method)
			if !isHTTPMethod(method) {
				continue
			}
			opMap, ok := opData.(map[string]interface{})
			if !ok {
				continue
			}
			op := parseSwagger2Operation(opMap, pathParams)
			pi.Operations[method] = op
		}
		s.Paths[pathStr] = pi
	}

	return s, nil
}

// parseOpenAPI3 parses an OpenAPI 3.x spec.
func parseOpenAPI3(raw map[string]interface{}) (*spec, error) {
	s := &spec{
		Paths: make(map[string]pathItem),
	}

	// Extract title
	if info, ok := raw["info"].(map[string]interface{}); ok {
		if title, ok := info["title"].(string); ok {
			s.Title = title
		}
	}

	// Extract base URL from servers
	if servers, ok := raw["servers"].([]interface{}); ok && len(servers) > 0 {
		if server, ok := servers[0].(map[string]interface{}); ok {
			if serverURL, ok := server["url"].(string); ok {
				s.BaseURL = strings.TrimRight(serverURL, "/")
			}
		}
	}

	// Parse paths
	paths, ok := raw["paths"].(map[string]interface{})
	if !ok {
		return s, nil
	}

	for pathStr, pathData := range paths {
		pathMap, ok := pathData.(map[string]interface{})
		if !ok {
			continue
		}
		pi := pathItem{Operations: make(map[string]operation)}

		// Collect path-level parameters
		var pathParams []parameter
		if pathParamsRaw, ok := pathMap["parameters"].([]interface{}); ok {
			pathParams = parseParameters(pathParamsRaw)
		}

		for method, opData := range pathMap {
			method = strings.ToUpper(method)
			if !isHTTPMethod(method) {
				continue
			}
			opMap, ok := opData.(map[string]interface{})
			if !ok {
				continue
			}
			op := parseOpenAPI3Operation(opMap, pathParams)
			pi.Operations[method] = op
		}
		s.Paths[pathStr] = pi
	}

	return s, nil
}

// parseSwagger2Operation parses a Swagger 2.0 operation.
func parseSwagger2Operation(opMap map[string]interface{}, pathParams []parameter) operation {
	op := operation{
		Responses: make(map[string]response),
	}

	if summary, ok := opMap["summary"].(string); ok {
		op.Summary = summary
	}
	if opID, ok := opMap["operationId"].(string); ok {
		op.OperationID = opID
	}

	// Start with path-level parameters
	op.Parameters = append(op.Parameters, pathParams...)

	// Parse operation-level parameters (override path-level)
	if paramsRaw, ok := opMap["parameters"].([]interface{}); ok {
		opParams := parseParameters(paramsRaw)
		op.Parameters = mergeParameters(op.Parameters, opParams)
	}

	// In Swagger 2.0, body params are defined as parameters with "in": "body"
	for i, p := range op.Parameters {
		if p.In == "body" {
			op.RequestBody = &requestBody{
				// TODO: Should read the operation-level or spec-level "consumes" field
				// instead of hardcoding application/json. Works for most APIs but technically
				// incorrect for XML/form-data specs.
				ContentType: "application/json",
				Example:     p.Example,
				Schema:      p.Schema,
			}
			// Remove body param from parameters list
			op.Parameters = append(op.Parameters[:i], op.Parameters[i+1:]...)
			break
		}
	}

	// Parse responses
	if responses, ok := opMap["responses"].(map[string]interface{}); ok {
		for code, respData := range responses {
			if respMap, ok := respData.(map[string]interface{}); ok {
				desc, _ := respMap["description"].(string)
				op.Responses[code] = response{Description: desc}
			}
		}
	}

	return op
}

// parseOpenAPI3Operation parses an OpenAPI 3.x operation.
func parseOpenAPI3Operation(opMap map[string]interface{}, pathParams []parameter) operation {
	op := operation{
		Responses: make(map[string]response),
	}

	if summary, ok := opMap["summary"].(string); ok {
		op.Summary = summary
	}
	if opID, ok := opMap["operationId"].(string); ok {
		op.OperationID = opID
	}

	// Start with path-level parameters
	op.Parameters = append(op.Parameters, pathParams...)

	// Parse operation-level parameters
	if paramsRaw, ok := opMap["parameters"].([]interface{}); ok {
		opParams := parseParameters(paramsRaw)
		op.Parameters = mergeParameters(op.Parameters, opParams)
	}

	// Parse requestBody (OpenAPI 3.x)
	if rbRaw, ok := opMap["requestBody"].(map[string]interface{}); ok {
		op.RequestBody = parseRequestBody(rbRaw)
	}

	// Parse responses
	if responses, ok := opMap["responses"].(map[string]interface{}); ok {
		for code, respData := range responses {
			if respMap, ok := respData.(map[string]interface{}); ok {
				desc, _ := respMap["description"].(string)
				op.Responses[code] = response{Description: desc}
			}
		}
	}

	return op
}

// parseParameters parses a list of parameter objects.
func parseParameters(paramsRaw []interface{}) []parameter {
	var params []parameter
	for _, pRaw := range paramsRaw {
		pMap, ok := pRaw.(map[string]interface{})
		if !ok {
			continue
		}
		p := parameter{}
		p.Name, _ = pMap["name"].(string)
		p.In, _ = pMap["in"].(string)
		p.Required, _ = pMap["required"].(bool)

		// Extract example value
		if example, ok := pMap["example"]; ok {
			p.Example = fmt.Sprintf("%v", example)
		}

		// Parse schema for example values
		if schemaRaw, ok := pMap["schema"].(map[string]interface{}); ok {
			p.Schema = parseSchema(schemaRaw)
			if p.Example == "" && p.Schema.Example != nil {
				p.Example = fmt.Sprintf("%v", p.Schema.Example)
			}
		}

		params = append(params, p)
	}
	return params
}

// mergeParameters merges path-level and operation-level parameters.
// Operation-level parameters override path-level parameters with the same name+in.
func mergeParameters(pathParams, opParams []parameter) []parameter {
	merged := make(map[string]parameter)
	for _, p := range pathParams {
		merged[p.In+":"+p.Name] = p
	}
	for _, p := range opParams {
		merged[p.In+":"+p.Name] = p
	}
	result := make([]parameter, 0, len(merged))
	for _, k := range sortedKeys(merged) {
		result = append(result, merged[k])
	}
	return result
}

// parseRequestBody parses an OpenAPI 3.x requestBody.
func parseRequestBody(rbMap map[string]interface{}) *requestBody {
	rb := &requestBody{}

	content, ok := rbMap["content"].(map[string]interface{})
	if !ok {
		return rb
	}

	// Prefer application/json, fall back to first content type (sorted for deterministic selection)
	contentTypes := sortedKeys(content)
	if len(contentTypes) == 0 {
		return rb
	}

	// First pass: look for application/json
	if ctData, ok := content["application/json"]; ok {
		applyContentType(rb, "application/json", ctData)
		return rb
	}

	// Fallback: use first in sorted order
	applyContentType(rb, contentTypes[0], content[contentTypes[0]])
	return rb
}

// applyContentType sets the content type, schema, and example on the request body.
func applyContentType(rb *requestBody, ct string, ctData interface{}) {
	rb.ContentType = ct
	if ctMap, ok := ctData.(map[string]interface{}); ok {
		if schemaRaw, ok := ctMap["schema"].(map[string]interface{}); ok {
			rb.Schema = parseSchema(schemaRaw)
		}
		if example, ok := ctMap["example"]; ok {
			if exBytes, err := json.Marshal(example); err == nil {
				rb.Example = string(exBytes)
			}
		}
	}
}

// parseSchema parses a minimal schema object.
// TODO: Does not resolve $ref references. Real-world specs use $ref extensively
// (e.g. "$ref": "#/definitions/Pet"), so imported schemas with $ref will have
// missing data. This is a known V1 limitation — a follow-up should resolve
// references from the spec's definitions/components.
func parseSchema(raw map[string]interface{}) *schemaObj {
	s := &schemaObj{}
	s.Type, _ = raw["type"].(string)
	s.Example = raw["example"]
	if props, ok := raw["properties"].(map[string]interface{}); ok {
		s.Properties = make(map[string]*schemaObj)
		for key, val := range props {
			if propMap, ok := val.(map[string]interface{}); ok {
				s.Properties[key] = parseSchema(propMap)
			}
		}
	}
	return s
}

// --- Conversion to HTTP Models ---

// convertSpec converts a parsed spec into resolved HTTP models.
func convertSpec(s *spec, opts ConvertOptions) (*OpenAPIResolved, error) {
	resolved := &OpenAPIResolved{}

	// Create flow
	flowID := idwrap.NewNow()
	flowName := s.Title
	if flowName == "" {
		flowName = "Imported OpenAPI Spec"
	}
	resolved.Flow = mflow.Flow{
		ID:          flowID,
		WorkspaceID: opts.WorkspaceID,
		Name:        flowName,
	}

	// Create start node
	startNodeID := idwrap.NewNow()
	resolved.Nodes = append(resolved.Nodes, mflow.Node{
		ID:        startNodeID,
		FlowID:    flowID,
		Name:      "Start",
		NodeKind:  mflow.NODE_KIND_MANUAL_START,
		PositionX: 0,
		PositionY: 0,
	})

	previousNodeID := startNodeID

	// Sort paths for deterministic output
	sortedPaths := sortedKeys(s.Paths)

	for _, pathStr := range sortedPaths {
		pi := s.Paths[pathStr]

		// Sort methods for deterministic output
		sortedMethods := sortedKeys(pi.Operations)

		for _, method := range sortedMethods {
			op := pi.Operations[method]

			httpReq, headers, searchParams, bodyRaw, assert := convertOperation(
				method, pathStr, s.BaseURL, op, opts,
			)

			// Create flow node
			nodeID := idwrap.NewNow()
			node := mflow.Node{
				ID:        nodeID,
				FlowID:    flowID,
				Name:      fmt.Sprintf("http_%d", len(resolved.RequestNodes)+1),
				NodeKind:  mflow.NODE_KIND_REQUEST,
				PositionX: float64(len(resolved.RequestNodes)+1) * 300,
				PositionY: 0,
			}

			reqNode := mflow.NodeRequest{
				FlowNodeID: nodeID,
				HttpID:     &httpReq.ID,
			}

			// Edge from previous node
			resolved.Edges = append(resolved.Edges, mflow.Edge{
				ID:            idwrap.NewNow(),
				FlowID:        flowID,
				SourceID:      previousNodeID,
				TargetID:      nodeID,
				SourceHandler: mflow.HandleUnspecified,
			})
			previousNodeID = nodeID

			// Create file record
			file := createFileRecord(httpReq, opts)

			// Collect entities
			resolved.HTTPRequests = append(resolved.HTTPRequests, httpReq)
			resolved.Headers = append(resolved.Headers, headers...)
			resolved.SearchParams = append(resolved.SearchParams, searchParams...)
			if bodyRaw != nil {
				resolved.BodyRaw = append(resolved.BodyRaw, *bodyRaw)
			}
			if assert != nil {
				resolved.Asserts = append(resolved.Asserts, *assert)
			}
			resolved.Files = append(resolved.Files, file)
			resolved.Nodes = append(resolved.Nodes, node)
			resolved.RequestNodes = append(resolved.RequestNodes, reqNode)
		}
	}

	// Create folder structure from URLs
	folderFiles := buildFolderStructure(resolved.HTTPRequests, resolved.Files, opts)
	resolved.Files = append(resolved.Files, folderFiles...)

	// Create flow file entry
	flowFile := mfile.File{
		ID:          resolved.Flow.ID,
		WorkspaceID: opts.WorkspaceID,
		ContentID:   &resolved.Flow.ID,
		ContentType: mfile.ContentTypeFlow,
		Name:        resolved.Flow.Name,
		Order:       -1,
		UpdatedAt:   time.Now(),
	}
	resolved.Files = append(resolved.Files, flowFile)

	return resolved, nil
}

// convertOperation converts a single API operation to HTTP models.
func convertOperation(
	method, pathStr, baseURL string,
	op operation,
	opts ConvertOptions,
) (mhttp.HTTP, []mhttp.HTTPHeader, []mhttp.HTTPSearchParam, *mhttp.HTTPBodyRaw, *mhttp.HTTPAssert) {
	httpID := idwrap.NewNow()
	now := time.Now().UnixMilli()

	// Build URL with path parameter placeholders
	fullURL := baseURL + pathStr
	for _, p := range op.Parameters {
		if p.In == "path" {
			// Replace {param} with example value or placeholder
			value := p.Example
			if value == "" {
				value = "{{" + p.Name + "}}"
			}
			fullURL = strings.ReplaceAll(fullURL, "{"+p.Name+"}", value)
		}
	}

	// Build request name
	name := op.Summary
	if name == "" {
		name = op.OperationID
	}
	if name == "" {
		name = method + " " + pathStr
	}

	// Determine body kind
	bodyKind := mhttp.HttpBodyKindNone
	if op.RequestBody != nil {
		bodyKind = mhttp.HttpBodyKindRaw
	}

	httpReq := mhttp.HTTP{
		ID:          httpID,
		WorkspaceID: opts.WorkspaceID,
		FolderID:    opts.FolderID,
		Name:        name,
		Url:         fullURL,
		Method:      method,
		Description: op.Summary,
		BodyKind:    bodyKind,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Convert headers
	var headers []mhttp.HTTPHeader
	headerOrder := 0
	for _, p := range op.Parameters {
		if p.In == "header" {
			headers = append(headers, mhttp.HTTPHeader{
				ID:           idwrap.NewNow(),
				HttpID:       httpID,
				Key:          p.Name,
				Value:        p.Example,
				Enabled:      true,
				DisplayOrder: float32(headerOrder),
				CreatedAt:    now,
				UpdatedAt:    now,
			})
			headerOrder++
		}
	}

	// Add Content-Type header if there's a request body
	if op.RequestBody != nil && op.RequestBody.ContentType != "" {
		headers = append(headers, mhttp.HTTPHeader{
			ID:           idwrap.NewNow(),
			HttpID:       httpID,
			Key:          "Content-Type",
			Value:        op.RequestBody.ContentType,
			Enabled:      true,
			DisplayOrder: float32(headerOrder),
			CreatedAt:    now,
			UpdatedAt:    now,
		})
	}

	// Convert query parameters
	var searchParams []mhttp.HTTPSearchParam
	paramOrder := 0
	for _, p := range op.Parameters {
		if p.In == "query" {
			searchParams = append(searchParams, mhttp.HTTPSearchParam{
				ID:           idwrap.NewNow(),
				HttpID:       httpID,
				Key:          p.Name,
				Value:        p.Example,
				Enabled:      p.Required,
				DisplayOrder: float64(paramOrder),
				CreatedAt:    now,
				UpdatedAt:    now,
			})
			paramOrder++
		}
	}

	// Convert request body
	var bodyRaw *mhttp.HTTPBodyRaw
	if op.RequestBody != nil {
		bodyContent := op.RequestBody.Example
		if bodyContent == "" && op.RequestBody.Schema != nil {
			bodyContent = generateExampleJSON(op.RequestBody.Schema)
		}
		if bodyContent != "" {
			bodyRaw = &mhttp.HTTPBodyRaw{
				ID:        idwrap.NewNow(),
				HttpID:    httpID,
				RawData:   []byte(bodyContent),
				CreatedAt: now,
				UpdatedAt: now,
			}
		}
	}

	// Create status code assertion from first success response
	// Sort response codes for deterministic selection
	var assert *mhttp.HTTPAssert
	responseCodes := sortedKeys(op.Responses)
	for _, code := range responseCodes {
		if strings.HasPrefix(code, "2") {
			statusCode := 200
			if _, err := fmt.Sscanf(code, "%d", &statusCode); err == nil {
				assert = &mhttp.HTTPAssert{
					ID:           idwrap.NewNow(),
					HttpID:       httpID,
					Value:        fmt.Sprintf("response.status == %d", statusCode),
					Enabled:      true,
					Description:  fmt.Sprintf("Verify response status is %d (from OpenAPI import)", statusCode),
					DisplayOrder: 0,
					CreatedAt:    now,
					UpdatedAt:    now,
				}
			}
			break
		}
	}

	return httpReq, headers, searchParams, bodyRaw, assert
}

// --- Helper Functions ---

// generateExampleJSON generates a minimal example JSON from a schema.
func generateExampleJSON(s *schemaObj) string {
	if s == nil {
		return ""
	}
	if s.Example != nil {
		if b, err := json.MarshalIndent(s.Example, "", "  "); err == nil {
			return string(b)
		}
	}
	if len(s.Properties) > 0 {
		obj := make(map[string]interface{})
		for key, prop := range s.Properties {
			if prop.Example != nil {
				obj[key] = prop.Example
			} else {
				obj[key] = exampleForType(prop.Type)
			}
		}
		if b, err := json.MarshalIndent(obj, "", "  "); err == nil {
			return string(b)
		}
	}
	return ""
}

// exampleForType returns a placeholder value for a JSON schema type.
func exampleForType(t string) interface{} {
	switch t {
	case "string":
		return "string"
	case "integer", "number":
		return 0
	case "boolean":
		return false
	case "array":
		return []interface{}{}
	case "object":
		return map[string]interface{}{}
	default:
		return nil
	}
}

// createFileRecord creates a file record for an HTTP request.
func createFileRecord(httpReq mhttp.HTTP, opts ConvertOptions) mfile.File {
	filename := httpReq.Name
	if filename == "" {
		filename = "untitled_request"
	}
	return mfile.File{
		ID:          httpReq.ID,
		WorkspaceID: opts.WorkspaceID,
		ParentID:    httpReq.FolderID,
		ContentID:   &httpReq.ID,
		ContentType: mfile.ContentTypeHTTP,
		Name:        filename,
		Order:       0,
		UpdatedAt:   time.Now(),
	}
}

// buildFolderStructure creates URL-based folder structure similar to Postman and HAR imports.
func buildFolderStructure(httpReqs []mhttp.HTTP, existingFiles []mfile.File, opts ConvertOptions) []mfile.File {
	folderMap := make(map[string]idwrap.IDWrap)
	folderFiles := make(map[string]mfile.File)

	for i := range httpReqs {
		req := &httpReqs[i]
		if req.FolderID != nil {
			continue // Already has a folder
		}

		folderPath := buildFolderPathFromURL(req.Url)
		if folderPath == "" || folderPath == "/" {
			continue
		}

		folderID := getOrCreateFolder(folderPath, opts.WorkspaceID, folderMap, folderFiles)
		if folderID.Compare(idwrap.IDWrap{}) != 0 {
			req.FolderID = &folderID
			// Also update the corresponding file's parent
			for j := range existingFiles {
				if existingFiles[j].ID == req.ID {
					existingFiles[j].ParentID = &folderID
					break
				}
			}
		}
	}

	result := make([]mfile.File, 0, len(folderFiles))
	for _, k := range sortedKeys(folderFiles) {
		result = append(result, folderFiles[k])
	}
	return result
}

// getOrCreateFolder creates or retrieves a folder ID for a given path.
func getOrCreateFolder(folderPath string, workspaceID idwrap.IDWrap, folderMap map[string]idwrap.IDWrap, folderFiles map[string]mfile.File) idwrap.IDWrap {
	if existingID, exists := folderMap[folderPath]; exists {
		return existingID
	}

	// Create parent folders first
	var parentID *idwrap.IDWrap
	parentPath := path.Dir(folderPath)
	if parentPath != "/" && parentPath != "." && parentPath != "" {
		pid := getOrCreateFolder(parentPath, workspaceID, folderMap, folderFiles)
		parentID = &pid
	}

	folderID := idwrap.NewNow()
	folderName := path.Base(folderPath)
	if folderName == "" || folderName == "." || folderName == "/" {
		folderName = "imported"
	}

	folderFiles[folderPath] = mfile.File{
		ID:          folderID,
		WorkspaceID: workspaceID,
		ParentID:    parentID,
		ContentType: mfile.ContentTypeFolder,
		Name:        folderName,
		Order:       0,
		UpdatedAt:   time.Now(),
	}
	folderMap[folderPath] = folderID
	return folderID
}

// buildFolderPathFromURL creates a hierarchical folder path from a URL.
func buildFolderPathFromURL(urlStr string) string {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}

	hostname := parsedURL.Hostname()
	if hostname == "" {
		return ""
	}

	// Reverse hostname: api.example.com -> com/example/api
	hostParts := strings.Split(hostname, ".")
	for i, j := 0, len(hostParts)-1; i < j; i, j = i+1, j-1 {
		hostParts[i], hostParts[j] = hostParts[j], hostParts[i]
	}

	var allSegments []string
	for _, part := range hostParts {
		if part != "" {
			allSegments = append(allSegments, part)
		}
	}

	if len(allSegments) == 0 {
		return ""
	}
	return "/" + strings.Join(allSegments, "/")
}

// isHTTPMethod checks if a string is a valid HTTP method.
func isHTTPMethod(s string) bool {
	switch s {
	case "GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS", "TRACE":
		return true
	}
	return false
}

// sortedKeys returns sorted keys from a map.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
