package yamlflowsimplev2

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/ioworkspace"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mflowvariable"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"the-dev-tools/server/pkg/model/mnnode/mnforeach"
	"the-dev-tools/server/pkg/model/mnnode/mnif"
	"the-dev-tools/server/pkg/model/mnnode/mnjs"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/varsystem"
)

// Constants for field names and step types
const (
	fieldName        = "name"
	fieldValue       = "value"
	fieldMethod      = "method"
	fieldURL         = "url"
	fieldHeaders     = "headers"
	fieldQueryParams = "query_params"
	fieldBody        = "body"
	fieldAssertions  = "assertions"
	fieldCondition   = "condition"
	fieldIterCount   = "iter_count"
	fieldItems       = "items"
	fieldCode        = "code"
	fieldDependsOn   = "depends_on"
	fieldUseRequest  = "use_request"
	fieldThen        = "then"
	fieldElse        = "else"
	fieldLoop        = "loop"
	fieldSteps       = "steps"
	fieldFlows       = "flows"
	fieldRun         = "run"
	fieldFlow        = "flow"
	fieldDescription = "description"

	stepTypeRequest = "request"
	stepTypeIf      = "if"
	stepTypeFor     = "for"
	stepTypeForEach = "for_each"
	stepTypeJS      = "js"
)

// ConvertSimplifiedYAML converts simplified YAML to modern HTTP and flow models
func ConvertSimplifiedYAML(data []byte, opts ConvertOptionsV2) (*ioworkspace.WorkspaceBundle, error) {
	// Validate options
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid options: %w", err)
	}

	// Parse YAML to get structured data and raw data
	yamlFormat, rawData, err := parseYAMLData(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Validate YAML structure
	if err := yamlFormat.Validate(); err != nil {
		return nil, fmt.Errorf("invalid YAML structure: %w", err)
	}

	// Parse run field if present
	runEntries, err := parseRunField(yamlFormat.Run)
	if err != nil {
		return nil, fmt.Errorf("failed to parse run field: %w", err)
	}

	// Parse request templates
	templates, err := parseRequestTemplates(yamlFormat.RequestTemplates, yamlFormat.Requests)
	if err != nil {
		return nil, fmt.Errorf("failed to parse request templates: %w", err)
	}

	// Initialize resolved data structure with workspace metadata
	result := &ioworkspace.WorkspaceBundle{
		Workspace: mworkspace.Workspace{
			ID:   opts.WorkspaceID,
			Name: yamlFormat.WorkspaceName,
		},
	}

	// Process flows and generate HTTP requests
	for _, flowEntry := range yamlFormat.Flows {
		flowData, err := processFlow(flowEntry, runEntries, templates, rawData, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to process flow '%s': %w", flowEntry.Name, err)
		}

		// Merge flow data into result
		mergeFlowData(result, flowData, opts)
	}

	return result, nil
}

// parseYAMLData parses YAML data into both structured and raw formats
func parseYAMLData(data []byte) (*YamlFlowFormatV2, map[string]any, error) {
	var yamlFormat YamlFlowFormatV2
	var rawData map[string]any

	// First unmarshal to raw map for proper step type handling
	if err := yaml.Unmarshal(data, &rawData); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal raw YAML: %w", err)
	}

	// Then unmarshal to structured format
	if err := yaml.Unmarshal(data, &yamlFormat); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal structured YAML: %w", err)
	}

	return &yamlFormat, rawData, nil
}

// parseRunField parses the run field into RunEntry structs
func parseRunField(runArray []map[string]any) ([]RunEntry, error) {
	var entries []RunEntry

	for _, itemMap := range runArray {
		flowName, ok := itemMap[fieldFlow].(string)
		if !ok || flowName == "" {
			return nil, fmt.Errorf("each run entry must have a 'flow' field")
		}

		entry := RunEntry{
			Flow: flowName,
		}

		// Parse depends_on field
		if deps, ok := itemMap[fieldDependsOn]; ok {
			switch v := deps.(type) {
			case string:
				entry.DependsOn = []string{v}
			case []any:
				for _, dep := range v {
					if depStr, ok := dep.(string); ok {
						entry.DependsOn = append(entry.DependsOn, depStr)
					}
				}
			default:
				return nil, fmt.Errorf("depends_on must be a string or array of strings")
			}
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// RunEntry represents an entry in the run field
type RunEntry struct {
	Flow      string
	DependsOn []string
}

// parseRequestTemplates parses request templates into a map
func parseRequestTemplates(templates map[string]map[string]any, requests []YamlRequestDefV2) (map[string]*YamlHTTPRequestV2, error) {
	result := make(map[string]*YamlHTTPRequestV2)

	// Parse templates
	for name, tmpl := range templates {
		httpReq, err := parseHTTPRequestData(tmpl)
		if err != nil {
			return nil, fmt.Errorf("failed to parse template '%s': %w", name, err)
		}
		result[name] = httpReq
	}

	// Parse direct requests from YamlRequestDefV2 structs
	for _, req := range requests {
		if req.Name == "" {
			continue
		}

		if _, exists := result[req.Name]; exists {
			return nil, fmt.Errorf("duplicate request name: %s", req.Name)
		}

		httpReq := &YamlHTTPRequestV2{
			Name:   req.Name,
			Method: req.Method,
			URL:    req.URL,
		}

		// Convert headers
		if len(req.Headers) > 0 {
			for k, v := range req.Headers {
				httpReq.Headers = append(httpReq.Headers, YamlNameValuePairV2{
					Name:    k,
					Value:   v,
					Enabled: true,
				})
			}
		}

		// Convert query params
		if len(req.QueryParams) > 0 {
			for k, v := range req.QueryParams {
				httpReq.QueryParams = append(httpReq.QueryParams, YamlNameValuePairV2{
					Name:    k,
					Value:   v,
					Enabled: true,
				})
			}
		}

		// Convert body - handle the map[string]any format
		if req.Body != nil {
			bodyMap, ok := req.Body.(map[string]any)
			if ok {
				body, err := parseBodyData(bodyMap)
				if err != nil {
					return nil, fmt.Errorf("failed to parse body for request '%s': %w", req.Name, err)
				}
				httpReq.Body = body
			}
		}

		result[req.Name] = httpReq
	}

	return result, nil
}

// parseHTTPRequestData parses HTTP request data from a map
func parseHTTPRequestData(data map[string]any) (*YamlHTTPRequestV2, error) {
	httpReq := &YamlHTTPRequestV2{}

	if name, ok := data[fieldName].(string); ok {
		httpReq.Name = name
	}
	if method, ok := data[fieldMethod].(string); ok {
		httpReq.Method = method
	}
	if url, ok := data[fieldURL].(string); ok {
		httpReq.URL = url
	}
	if description, ok := data[fieldDescription].(string); ok {
		httpReq.Description = description
	}

	// Parse headers (handle both array and map formats)
	if headers, ok := data[fieldHeaders].([]any); ok {
		// Array format: [{"name": "key", "value": "value"}]
		for _, h := range headers {
			if headerMap, ok := h.(map[string]any); ok {
				header := parseNameValuePair(headerMap)
				httpReq.Headers = append(httpReq.Headers, header)
			}
		}
	} else if headers, ok := data[fieldHeaders].(map[string]any); ok {
		// Map format: {"key": "value"}
		for key, value := range headers {
			if valueStr, ok := value.(string); ok {
				header := YamlNameValuePairV2{
					Name:    key,
					Value:   valueStr,
					Enabled: true,
				}
				httpReq.Headers = append(httpReq.Headers, header)
			}
		}
	}

	// Parse query parameters (handle both array and map formats)
	if queryParams, ok := data[fieldQueryParams].([]any); ok {
		// Array format: [{"name": "key", "value": "value"}]
		for _, q := range queryParams {
			if queryMap, ok := q.(map[string]any); ok {
				param := parseNameValuePair(queryMap)
				httpReq.QueryParams = append(httpReq.QueryParams, param)
			}
		}
	} else if queryParams, ok := data[fieldQueryParams].(map[string]any); ok {
		// Map format: {"key": "value"}
		for key, value := range queryParams {
			if valueStr, ok := value.(string); ok {
				param := YamlNameValuePairV2{
					Name:    key,
					Value:   valueStr,
					Enabled: true,
				}
				httpReq.QueryParams = append(httpReq.QueryParams, param)
			}
		}
	}

	// Parse body
	if bodyData, ok := data[fieldBody].(map[string]any); ok {
		body, err := parseBodyData(bodyData)
		if err != nil {
			return nil, fmt.Errorf("failed to parse body: %w", err)
		}
		httpReq.Body = body
	}

	// Parse assertions
	if assertions, ok := data[fieldAssertions]; ok {
		asserts, err := parseAssertions(assertions)
		if err != nil {
			return nil, fmt.Errorf("failed to parse assertions: %w", err)
		}
		httpReq.Assertions = asserts
	}

	return httpReq, nil
}

// parseNameValuePair parses a name-value pair from a map
func parseNameValuePair(data map[string]any) YamlNameValuePairV2 {
	pair := YamlNameValuePairV2{
		Enabled: true,
	}

	if name, ok := data[fieldName].(string); ok {
		pair.Name = name
	}
	if value, ok := data[fieldValue].(string); ok {
		pair.Value = value
	}
	if description, ok := data[fieldDescription].(string); ok {
		pair.Description = description
	}
	if enabled, ok := data["enabled"].(bool); ok {
		pair.Enabled = enabled
	}

	return pair
}

// parseBodyData parses body data from a map
func parseBodyData(data map[string]any) (*YamlBodyV2, error) {
	body := &YamlBodyV2{}

	if bodyType, ok := data["type"].(string); ok {
		body.Type = strings.ToLower(bodyType)
	} else {
		body.Type = "raw" // Default to raw
	}

	if raw, ok := data["raw"].(string); ok {
		body.Raw = raw
	}

	if jsonData, ok := data["json"].(map[string]any); ok {
		body.JSON = jsonData
		body.Type = "json"
	}

	if formData, ok := data["form_data"].([]any); ok {
		for _, f := range formData {
			if formMap, ok := f.(map[string]any); ok {
				formPair := parseNameValuePair(formMap)
				body.Form = append(body.Form, formPair)
			}
		}
		if len(body.Form) > 0 {
			body.Type = "form-data"
		}
	}

	if urlData, ok := data["urlencoded"].([]any); ok {
		for _, u := range urlData {
			if urlMap, ok := u.(map[string]any); ok {
				urlPair := parseNameValuePair(urlMap)
				body.UrlEncoded = append(body.UrlEncoded, urlPair)
			}
		}
		if len(body.UrlEncoded) > 0 {
			body.Type = "urlencoded"
		}
	}

	return body, nil
}

// parseAssertions parses assertions from any format
func parseAssertions(data any) ([]YamlAssertionV2, error) {
	var assertions []YamlAssertionV2

	switch v := data.(type) {
	case []any:
		for _, item := range v {
			if assertMap, ok := item.(map[string]any); ok {
				assertion := parseAssertion(assertMap)
				assertions = append(assertions, assertion)
			}
		}
	case map[string]any:
		assertion := parseAssertion(v)
		assertions = append(assertions, assertion)
	}

	return assertions, nil
}

// parseAssertion parses a single assertion from a map
func parseAssertion(data map[string]any) YamlAssertionV2 {
	assertion := YamlAssertionV2{
		Enabled: true,
	}

	if expression, ok := data["expression"].(string); ok {
		assertion.Expression = expression
	}
	if enabled, ok := data["enabled"].(bool); ok {
		assertion.Enabled = enabled
	}

	return assertion
}

// processFlow processes a single flow and returns the generated data
func processFlow(flowEntry YamlFlowFlowV2, runEntries []RunEntry, templates map[string]*YamlHTTPRequestV2, rawData map[string]any, opts ConvertOptionsV2) (*ioworkspace.WorkspaceBundle, error) {
	result := &ioworkspace.WorkspaceBundle{}

	// Create flow entity
	flowID := idwrap.NewNow()
	flow := mflow.Flow{
		ID:          flowID,
		Name:        flowEntry.Name,
		WorkspaceID: opts.WorkspaceID,
	}
	if flowEntry.Timeout != nil {
		// Note: Store timeout as a flow variable for now, as the mflow.Flow model doesn't have a timeout field
		// This can be used by the flow execution engine
	}
	result.Flows = append(result.Flows, flow)

	// Create folder for the flow if generating files
	if opts.GenerateFiles {
		folderID := idwrap.NewNow()
		folderFile := mfile.File{
			ID:          folderID,
			WorkspaceID: opts.WorkspaceID,
			ParentID:    opts.FolderID, // Nested under parent folder if provided
			ContentID:   &folderID,
			ContentType: mfile.ContentTypeFolder,
			Name:        flowEntry.Name,
			Order:       float64(opts.FileOrder),
			UpdatedAt:   time.Now(),
		}
		result.Files = append(result.Files, folderFile)
		// Update opts to use this folder as parent for HTTP files
		opts.FolderID = &folderID
	}

	// Process flow variables
	varMap, err := processFlowVariables(flowEntry, flowID, result)
	if err != nil {
		return nil, fmt.Errorf("failed to process flow variables: %w", err)
	}

	// Get raw steps for this flow
	rawSteps, err := getRawStepsForFlow(flowEntry.Name, rawData)
	if err != nil {
		return nil, fmt.Errorf("failed to get raw steps: %w", err)
	}

	// Create start node
	startNodeID := createStartNode(flowID, result)

	// Process steps
	nodeInfoMap, nodeList, err := processSteps(flowEntry, rawSteps, templates, varMap, flowID, startNodeID, opts, result)
	if err != nil {
		return nil, fmt.Errorf("failed to process steps: %w", err)
	}

	// Create edges
	if err := createEdges(flowID, startNodeID, nodeInfoMap, nodeList, rawSteps, result); err != nil {
		return nil, fmt.Errorf("failed to create edges: %w", err)
	}

	return result, nil
}

// processFlowVariables processes flow variables and returns a variable map
func processFlowVariables(flowEntry YamlFlowFlowV2, flowID idwrap.IDWrap, result *ioworkspace.WorkspaceBundle) (varsystem.VarMap, error) {

	// Create flow variable entities
	for _, variable := range flowEntry.Variables {
		flowVar := mflowvariable.FlowVariable{
			ID:      idwrap.NewNow(),
			FlowID:  flowID,
			Name:    variable.Name,
			Value:   variable.Value,
			Enabled: true,
		}
		result.FlowVariables = append(result.FlowVariables, flowVar)
	}

	return varsystem.NewVarMap(nil), nil
}

// getRawStepsForFlow extracts raw steps for a specific flow
func getRawStepsForFlow(flowName string, rawData map[string]any) ([]map[string]any, error) {
	rawFlows, ok := rawData[fieldFlows].([]any)
	if !ok {
		return nil, fmt.Errorf("flows field not found or invalid")
	}

	for _, rf := range rawFlows {
		rfMap, ok := rf.(map[string]any)
		if !ok {
			continue
		}
		if name, ok := rfMap[fieldName].(string); ok && name == flowName {
			if steps, ok := rfMap[fieldSteps].([]any); ok {
				var rawSteps []map[string]any
				for _, step := range steps {
					if stepMap, ok := step.(map[string]any); ok && len(stepMap) == 1 {
						rawSteps = append(rawSteps, stepMap)
					}
				}
				return rawSteps, nil
			}
		}
	}

	return nil, fmt.Errorf("flow '%s' not found", flowName)
}

// nodeInfo tracks information about a flow node
type nodeInfo struct {
	id         idwrap.IDWrap
	name       string
	index      int
	dependsOn  []string
	httpReq    *mhttp.HTTP
	associated *HTTPAssociatedData
}

// createStartNode creates the start node for a flow
func createStartNode(flowID idwrap.IDWrap, result *ioworkspace.WorkspaceBundle) idwrap.IDWrap {
	startNodeID := idwrap.NewNow()

	startNode := mnnode.MNode{
		ID:       startNodeID,
		FlowID:   flowID,
		Name:     "Start",
		NodeKind: mnnode.NODE_KIND_NO_OP,
	}
	result.FlowNodes = append(result.FlowNodes, startNode)

	noopNode := mnnoop.NoopNode{
		FlowNodeID: startNodeID,
		Type:       mnnoop.NODE_NO_OP_KIND_START,
	}
	result.FlowNoopNodes = append(result.FlowNoopNodes, noopNode)

	return startNodeID
}

// processSteps processes all steps in a flow
func processSteps(flowEntry YamlFlowFlowV2, rawSteps []map[string]any, templates map[string]*YamlHTTPRequestV2, varMap varsystem.VarMap, flowID, startNodeID idwrap.IDWrap, opts ConvertOptionsV2, result *ioworkspace.WorkspaceBundle) (map[string]*nodeInfo, []*nodeInfo, error) {
	nodeInfoMap := make(map[string]*nodeInfo)
	nodeList := make([]*nodeInfo, 0)

	for i, rawStep := range rawSteps {
		for stepType, stepData := range rawStep {
			dataMap, ok := stepData.(map[string]any)
			if !ok {
				return nil, nil, fmt.Errorf("invalid step data format")
			}

			nodeName, ok := dataMap[fieldName].(string)
			if !ok || nodeName == "" {
				return nil, nil, NewYamlFlowErrorV2(fmt.Sprintf("missing required '%s' field", fieldName), "", nil)
			}

			nodeID := idwrap.NewNow()
			info := &nodeInfo{
				id:    nodeID,
				name:  nodeName,
				index: i,
			}

			// Extract dependencies
			if deps, ok := dataMap[fieldDependsOn].([]any); ok {
				for _, dep := range deps {
					if depStr, ok := dep.(string); ok && depStr != "" {
						info.dependsOn = append(info.dependsOn, depStr)
					}
				}
			}

			// Process step based on type
			switch stepType {
			case stepTypeRequest:
				httpReq, associated, err := processRequestStep(nodeName, nodeID, flowID, dataMap, templates, varMap, opts)
				if err != nil {
					return nil, nil, err
				}
				info.httpReq = httpReq
				info.associated = associated
				result.HTTPRequests = append(result.HTTPRequests, *httpReq)
				if associated != nil {
					mergeAssociatedData(result, associated)
				}

				// Create file if requested
				if opts.GenerateFiles {
					file := createFileForHTTP(*httpReq, opts)
					result.Files = append(result.Files, file)
				}

			case stepTypeIf:
				if err := processIfStep(nodeName, nodeID, flowID, dataMap, result); err != nil {
					return nil, nil, err
				}
			case stepTypeFor:
				if err := processForStep(nodeName, nodeID, flowID, dataMap, result); err != nil {
					return nil, nil, err
				}
			case stepTypeForEach:
				if err := processForEachStep(nodeName, nodeID, flowID, dataMap, result); err != nil {
					return nil, nil, err
				}
			case stepTypeJS:
				if err := processJSStep(nodeName, nodeID, flowID, dataMap, result); err != nil {
					return nil, nil, err
				}
			default:
				return nil, nil, NewYamlFlowErrorV2("unknown step type", "stepType", stepType)
			}

			nodeInfoMap[nodeName] = info
			nodeList = append(nodeList, info)
		}
	}

	return nodeInfoMap, nodeList, nil
}

// processRequestStep processes a request step
func processRequestStep(nodeName string, nodeID, flowID idwrap.IDWrap, stepData map[string]any, templates map[string]*YamlHTTPRequestV2, varMap varsystem.VarMap, opts ConvertOptionsV2) (*mhttp.HTTP, *HTTPAssociatedData, error) {
	// Initialize with defaults
	method, url := "GET", ""
	var templateData *YamlHTTPRequestV2
	var stepOverrides *YamlHTTPRequestV2
	var usingTemplate bool

	// Check if using template
	if useRequest, ok := stepData[fieldUseRequest].(string); ok && useRequest != "" {
		if tmpl, exists := templates[useRequest]; exists {
			usingTemplate = true
			templateData = tmpl
			method = tmpl.Method
			url = tmpl.URL
		} else {
			return nil, nil, NewYamlFlowErrorV2(fmt.Sprintf("request step '%s' references unknown template '%s'", nodeName, useRequest), fieldUseRequest, useRequest)
		}
	}

	// Override with step-specific data
	if m, ok := stepData[fieldMethod].(string); ok && m != "" {
		method = m
	}
	if u, ok := stepData[fieldURL].(string); ok && u != "" {
		url = u
	}

	// URL is required
	if url == "" {
		return nil, nil, NewYamlFlowErrorV2(fmt.Sprintf("request step '%s' missing required url", nodeName), fieldURL, nil)
	}

	// Parse step overrides
	stepReq, err := parseHTTPRequestData(stepData)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse step data: %w", err)
	}
	stepOverrides = stepReq

	// Merge template and step data
	finalReq := mergeHTTPRequestData(templateData, stepOverrides, usingTemplate)

	// Create HTTP entity
	httpID := idwrap.NewNow()
	now := time.Now().UnixMilli()

	httpReq := &mhttp.HTTP{
		ID:           httpID,
		WorkspaceID:  opts.WorkspaceID,
		FolderID:     opts.FolderID,
		Name:         nodeName,
		Url:          url,
		Method:       method,
		Description:  finalReq.Description,
		ParentHttpID: opts.ParentHttpID,
		IsDelta:      opts.IsDelta,
		DeltaName:    opts.DeltaName,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	// Create request node
	requestNode := mnrequest.MNRequest{
		FlowNodeID:       nodeID,
		HttpID:           &httpID, // Direct reference to HTTP ID
		HasRequestConfig: true,
	}

	// Create flow node
	flowNode := mnnode.MNode{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     nodeName,
		NodeKind: mnnode.NODE_KIND_REQUEST,
	}

	// Generate associated data (headers, query params, body)
	associated := &HTTPAssociatedData{
		Headers:      convertToHTTPHeaders(finalReq.Headers, httpID),
		SearchParams: convertToHTTPSearchParams(finalReq.QueryParams, httpID),
		FlowNode:     &flowNode,
		RequestNode:  &requestNode,
	}

	// Process body
	if finalReq.Body != nil {
		bodyRaw, bodyForms, bodyUrlencoded, bodyKind := convertBodyData(finalReq.Body, httpID, opts)
		associated.BodyRaw = bodyRaw
		associated.BodyForms = bodyForms
		associated.BodyUrlencoded = bodyUrlencoded
		httpReq.BodyKind = bodyKind
	}

	return httpReq, associated, nil
}

// HTTPAssociatedData contains data associated with an HTTP request
type HTTPAssociatedData struct {
	Headers        []mhttp.HTTPHeader
	SearchParams   []mhttp.HTTPSearchParam
	BodyForms      []mhttp.HTTPBodyForm
	BodyUrlencoded []mhttp.HTTPBodyUrlencoded
	BodyRaw        *mhttp.HTTPBodyRaw
	FlowNode       *mnnode.MNode
	RequestNode    *mnrequest.MNRequest
}

// mergeHTTPRequestData merges template and step HTTP request data
func mergeHTTPRequestData(template, step *YamlHTTPRequestV2, usingTemplate bool) *YamlHTTPRequestV2 {
	result := &YamlHTTPRequestV2{
		Name:        step.Name,
		Method:      step.Method,
		URL:         step.URL,
		Description: step.Description,
		Headers:     step.Headers,
		QueryParams: step.QueryParams,
		Body:        step.Body,
		Assertions:  step.Assertions,
	}

	if template != nil {
		if result.Name == "" {
			result.Name = template.Name
		}
		if result.Method == "" {
			result.Method = template.Method
		}
		if result.URL == "" {
			result.URL = template.URL
		}
		if result.Description == "" {
			result.Description = template.Description
		}
		if len(result.Headers) == 0 {
			result.Headers = template.Headers
		}
		if len(result.QueryParams) == 0 {
			result.QueryParams = template.QueryParams
		}
		if result.Body == nil {
			result.Body = template.Body
		}
		if len(result.Assertions) == 0 {
			result.Assertions = template.Assertions
		}
	}

	return result
}

// convertToHTTPHeaders converts YAML headers to HTTP header models
func convertToHTTPHeaders(headers []YamlNameValuePairV2, httpID idwrap.IDWrap) []mhttp.HTTPHeader {
	var httpHeaders []mhttp.HTTPHeader

	for _, header := range headers {
		if !header.Enabled {
			continue
		}

		httpHeader := mhttp.HTTPHeader{
			ID:          idwrap.NewNow(),
			HttpID:      httpID,
			Key:         header.Name,
			Value:       header.Value,
			Description: header.Description,
			Enabled:     true,
			CreatedAt:   time.Now().UnixMilli(),
			UpdatedAt:   time.Now().UnixMilli(),
		}
		httpHeaders = append(httpHeaders, httpHeader)
	}

	return httpHeaders
}

// convertToHTTPSearchParams converts YAML query params to HTTP search param models
func convertToHTTPSearchParams(params []YamlNameValuePairV2, httpID idwrap.IDWrap) []mhttp.HTTPSearchParam {
	var searchParams []mhttp.HTTPSearchParam

	for _, param := range params {
		if !param.Enabled {
			continue
		}

		searchParam := mhttp.HTTPSearchParam{
			ID:          idwrap.NewNow(),
			HttpID:      httpID,
			Key:         param.Name,
			Value:       param.Value,
			Description: param.Description,
			Enabled:     true,
			CreatedAt:   time.Now().UnixMilli(),
			UpdatedAt:   time.Now().UnixMilli(),
		}
		searchParams = append(searchParams, searchParam)
	}

	return searchParams
}

// convertBodyData converts YAML body data to HTTP body models
func convertBodyData(body *YamlBodyV2, httpID idwrap.IDWrap, opts ConvertOptionsV2) (*mhttp.HTTPBodyRaw, []mhttp.HTTPBodyForm, []mhttp.HTTPBodyUrlencoded, mhttp.HttpBodyKind) {
	switch body.Type {
	case "form-data":
		return nil, convertToBodyForms(body.Form, httpID), nil, mhttp.HttpBodyKindFormData
	case "urlencoded":
		return nil, nil, convertToBodyUrlencoded(body.UrlEncoded, httpID), mhttp.HttpBodyKindUrlEncoded
	default:
		// Default to raw body (handles JSON and raw text)
		return convertToBodyRaw(body, httpID, opts), nil, nil, mhttp.HttpBodyKindRaw
	}
}

// convertToBodyRaw converts YAML body to HTTP raw body model
func convertToBodyRaw(body *YamlBodyV2, httpID idwrap.IDWrap, opts ConvertOptionsV2) *mhttp.HTTPBodyRaw {
	var rawData []byte
	var contentType string

	if body.Type == "json" && body.JSON != nil {
		jsonData, err := json.Marshal(body.JSON)
		if err == nil {
			rawData = jsonData
			contentType = "application/json"
		}
	}

	if len(rawData) == 0 {
		// Use raw text
		rawData = []byte(body.Raw)
		contentType = "text/plain"
	}

	// Apply compression if enabled
	var compressionType compress.CompressType
	if opts.EnableCompression && len(rawData) > 1024 { // Only compress if larger than 1KB
		compressed, err := compress.Compress(rawData, opts.CompressionType)
		if err == nil {
			rawData = compressed
			compressionType = opts.CompressionType
		}
	} else {
		compressionType = compress.CompressTypeNone
	}

	return &mhttp.HTTPBodyRaw{
		ID:              idwrap.NewNow(),
		HttpID:          httpID,
		RawData:         rawData,
		ContentType:     contentType,
		CompressionType: int8(compressionType),
		CreatedAt:       time.Now().UnixMilli(),
		UpdatedAt:       time.Now().UnixMilli(),
	}
}

// convertToBodyForms converts YAML form data to HTTP body form models
func convertToBodyForms(formData []YamlNameValuePairV2, httpID idwrap.IDWrap) []mhttp.HTTPBodyForm {
	var bodyForms []mhttp.HTTPBodyForm

	for _, form := range formData {
		if !form.Enabled {
			continue
		}

		bodyForm := mhttp.HTTPBodyForm{
			ID:          idwrap.NewNow(),
			HttpID:      httpID,
			Key:         form.Name,
			Value:       form.Value,
			Description: form.Description,
			Enabled:     true,
			CreatedAt:   time.Now().UnixMilli(),
			UpdatedAt:   time.Now().UnixMilli(),
		}
		bodyForms = append(bodyForms, bodyForm)
	}

	return bodyForms
}

// convertToBodyUrlencoded converts YAML URL encoded data to HTTP body URL encoded models
func convertToBodyUrlencoded(urlData []YamlNameValuePairV2, httpID idwrap.IDWrap) []mhttp.HTTPBodyUrlencoded {
	var bodyUrlencoded []mhttp.HTTPBodyUrlencoded

	for _, url := range urlData {
		if !url.Enabled {
			continue
		}

		bodyUrl := mhttp.HTTPBodyUrlencoded{
			ID:          idwrap.NewNow(),
			HttpID:      httpID,
			Key:         url.Name,
			Value:       url.Value,
			Description: url.Description,
			Enabled:     true,
			CreatedAt:   time.Now().UnixMilli(),
			UpdatedAt:   time.Now().UnixMilli(),
		}
		bodyUrlencoded = append(bodyUrlencoded, bodyUrl)
	}

	return bodyUrlencoded
}

// createFileForHTTP creates a file record for an HTTP request
func createFileForHTTP(httpReq mhttp.HTTP, opts ConvertOptionsV2) mfile.File {
	filename := httpReq.Name
	if filename == "" {
		filename = "untitled_request"
	}

	// Use httpReq.ID as the file ID so frontend can look up HTTP by fileId
	// This matches the pattern used in HAR import
	return mfile.File{
		ID:          httpReq.ID,
		WorkspaceID: opts.WorkspaceID,
		ParentID:    opts.FolderID,
		ContentID:   &httpReq.ID,
		ContentType: mfile.ContentTypeHTTP,
		Name:        filename,
		Order:       float64(opts.FileOrder),
		UpdatedAt:   time.Now(),
	}
}

// processIfStep processes an if step
func processIfStep(nodeName string, nodeID, flowID idwrap.IDWrap, stepData map[string]any, result *ioworkspace.WorkspaceBundle) error {
	condition, ok := stepData[fieldCondition].(string)
	if !ok || condition == "" {
		return NewYamlFlowErrorV2(fmt.Sprintf("if step '%s' missing required condition", nodeName), fieldCondition, nil)
	}

	// Create flow node
	flowNode := mnnode.MNode{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     nodeName,
		NodeKind: mnnode.NODE_KIND_CONDITION,
	}
	result.FlowNodes = append(result.FlowNodes, flowNode)

	// Create condition node
	conditionNode := mnif.MNIF{
		FlowNodeID: nodeID,
		Condition: mcondition.Condition{
			Comparisons: mcondition.Comparison{Expression: condition},
		},
	}
	result.FlowConditionNodes = append(result.FlowConditionNodes, conditionNode)

	return nil
}

// processForStep processes a for step
func processForStep(nodeName string, nodeID, flowID idwrap.IDWrap, stepData map[string]any, result *ioworkspace.WorkspaceBundle) error {
	iterCount := 1 // Default to 1 if not specified
	if val, ok := stepData[fieldIterCount]; ok {
		switch v := val.(type) {
		case int:
			iterCount = v
		case float64:
			iterCount = int(v)
		}
	}

	// Create flow node
	flowNode := mnnode.MNode{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     nodeName,
		NodeKind: mnnode.NODE_KIND_FOR,
	}
	result.FlowNodes = append(result.FlowNodes, flowNode)

	// Create for node
	forNode := mnfor.MNFor{
		FlowNodeID: nodeID,
		IterCount:  int64(iterCount),
	}
	result.FlowForNodes = append(result.FlowForNodes, forNode)

	return nil
}

// processForEachStep processes a for_each step
func processForEachStep(nodeName string, nodeID, flowID idwrap.IDWrap, stepData map[string]any, result *ioworkspace.WorkspaceBundle) error {
	items, ok := stepData[fieldItems].(string)
	if !ok || items == "" {
		return NewYamlFlowErrorV2(fmt.Sprintf("for_each step '%s' missing required items", nodeName), fieldItems, nil)
	}

	// Create flow node
	flowNode := mnnode.MNode{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     nodeName,
		NodeKind: mnnode.NODE_KIND_FOR_EACH,
	}
	result.FlowNodes = append(result.FlowNodes, flowNode)

	// Create for each node
	forEachNode := mnforeach.MNForEach{
		FlowNodeID:     nodeID,
		IterExpression: items,
	}
	result.FlowForEachNodes = append(result.FlowForEachNodes, forEachNode)

	return nil
}

// processJSStep processes a JavaScript step
func processJSStep(nodeName string, nodeID, flowID idwrap.IDWrap, stepData map[string]any, result *ioworkspace.WorkspaceBundle) error {
	code, ok := stepData[fieldCode].(string)
	if !ok || code == "" {
		return NewYamlFlowErrorV2(fmt.Sprintf("js step '%s' missing required code", nodeName), fieldCode, nil)
	}

	// Trim trailing whitespace and newlines from code
	code = strings.TrimRight(code, " \t\r\n")

	// Create flow node
	flowNode := mnnode.MNode{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     nodeName,
		NodeKind: mnnode.NODE_KIND_JS,
	}
	result.FlowNodes = append(result.FlowNodes, flowNode)

	// Create JS node
	jsNode := mnjs.MNJS{
		FlowNodeID: nodeID,
		Code:       []byte(code),
	}
	result.FlowJSNodes = append(result.FlowJSNodes, jsNode)

	return nil
}

// mergeFlowData merges flow data into the main result
func mergeFlowData(result, flowData *ioworkspace.WorkspaceBundle, opts ConvertOptionsV2) {
	result.Flows = append(result.Flows, flowData.Flows...)
	result.FlowNodes = append(result.FlowNodes, flowData.FlowNodes...)
	result.FlowEdges = append(result.FlowEdges, flowData.FlowEdges...)
	result.FlowVariables = append(result.FlowVariables, flowData.FlowVariables...)
	result.FlowRequestNodes = append(result.FlowRequestNodes, flowData.FlowRequestNodes...)
	result.FlowConditionNodes = append(result.FlowConditionNodes, flowData.FlowConditionNodes...)
	result.FlowNoopNodes = append(result.FlowNoopNodes, flowData.FlowNoopNodes...)
	result.FlowForNodes = append(result.FlowForNodes, flowData.FlowForNodes...)
	result.FlowForEachNodes = append(result.FlowForEachNodes, flowData.FlowForEachNodes...)
	result.FlowJSNodes = append(result.FlowJSNodes, flowData.FlowJSNodes...)

	// Merge HTTP-related data
	result.HTTPRequests = append(result.HTTPRequests, flowData.HTTPRequests...)
	result.HTTPHeaders = append(result.HTTPHeaders, flowData.HTTPHeaders...)
	result.HTTPSearchParams = append(result.HTTPSearchParams, flowData.HTTPSearchParams...)
	result.HTTPBodyForms = append(result.HTTPBodyForms, flowData.HTTPBodyForms...)
	result.HTTPBodyUrlencoded = append(result.HTTPBodyUrlencoded, flowData.HTTPBodyUrlencoded...)
	result.HTTPBodyRaw = append(result.HTTPBodyRaw, flowData.HTTPBodyRaw...)
	result.Files = append(result.Files, flowData.Files...)
}

// mergeAssociatedData merges associated HTTP data into the result
func mergeAssociatedData(result *ioworkspace.WorkspaceBundle, associated *HTTPAssociatedData) {
	if associated != nil {
		result.HTTPHeaders = append(result.HTTPHeaders, associated.Headers...)
		result.HTTPSearchParams = append(result.HTTPSearchParams, associated.SearchParams...)
		result.HTTPBodyForms = append(result.HTTPBodyForms, associated.BodyForms...)
		result.HTTPBodyUrlencoded = append(result.HTTPBodyUrlencoded, associated.BodyUrlencoded...)
		if associated.BodyRaw != nil {
			result.HTTPBodyRaw = append(result.HTTPBodyRaw, *associated.BodyRaw)
		}
		if associated.FlowNode != nil {
			result.FlowNodes = append(result.FlowNodes, *associated.FlowNode)
		}
		if associated.RequestNode != nil {
			result.FlowRequestNodes = append(result.FlowRequestNodes, *associated.RequestNode)
		}
	}
}

// createEdges creates flow edges based on dependencies and control flow
func createEdges(flowID, startNodeID idwrap.IDWrap, nodeInfoMap map[string]*nodeInfo, nodeList []*nodeInfo, rawSteps []map[string]any, result *ioworkspace.WorkspaceBundle) error {
	// Track which nodes have incoming edges
	hasIncoming := make(map[idwrap.IDWrap]bool)

	// Create edges for explicit dependencies
	for _, info := range nodeList {
		for _, depName := range info.dependsOn {
			if depInfo, exists := nodeInfoMap[depName]; exists {
				edge := edge.Edge{
					ID:            idwrap.NewNow(),
					FlowID:        flowID,
					SourceID:      depInfo.id,
					TargetID:      info.id,
					SourceHandler: edge.HandleUnspecified,
				}
				result.FlowEdges = append(result.FlowEdges, edge)
				hasIncoming[info.id] = true
			}
		}
	}

	// Create edges for sequential steps (implicit dependencies)
	for i := 0; i < len(nodeList)-1; i++ {
		current := nodeList[i]
		next := nodeList[i+1]

		// Only create sequential edge if next node doesn't have explicit dependencies
		if len(next.dependsOn) == 0 && !hasIncoming[next.id] {
			edge := edge.Edge{
				ID:            idwrap.NewNow(),
				FlowID:        flowID,
				SourceID:      current.id,
				TargetID:      next.id,
				SourceHandler: edge.HandleUnspecified,
			}
			result.FlowEdges = append(result.FlowEdges, edge)
			hasIncoming[next.id] = true
		}
	}

	// Connect nodes without incoming edges to start node
	for _, info := range nodeList {
		if !hasIncoming[info.id] {
			edge := edge.Edge{
				ID:            idwrap.NewNow(),
				FlowID:        flowID,
				SourceID:      startNodeID,
				TargetID:      info.id,
				SourceHandler: edge.HandleUnspecified,
			}
			result.FlowEdges = append(result.FlowEdges, edge)
		}
	}

	// Create edges for control flow nodes
	for _, rawStep := range rawSteps {
		for stepType, stepData := range rawStep {
			dataMap, _ := stepData.(map[string]any)
			nodeName, _ := dataMap[fieldName].(string)
			nodeID := nodeInfoMap[nodeName].id

			switch stepType {
			case stepTypeIf:
				// Create edges for then/else branches
				if thenTarget, ok := dataMap[fieldThen].(string); ok && thenTarget != "" {
					if targetID, exists := nodeInfoMap[thenTarget]; exists {
						edge := edge.Edge{
							ID:            idwrap.NewNow(),
							FlowID:        flowID,
							SourceID:      nodeID,
							TargetID:      targetID.id,
							SourceHandler: edge.HandleThen,
						}
						result.FlowEdges = append(result.FlowEdges, edge)
						hasIncoming[targetID.id] = true
					}
				}

				if elseTarget, ok := dataMap[fieldElse].(string); ok && elseTarget != "" {
					if targetID, exists := nodeInfoMap[elseTarget]; exists {
						edge := edge.Edge{
							ID:            idwrap.NewNow(),
							FlowID:        flowID,
							SourceID:      nodeID,
							TargetID:      targetID.id,
							SourceHandler: edge.HandleElse,
						}
						result.FlowEdges = append(result.FlowEdges, edge)
						hasIncoming[targetID.id] = true
					}
				}

			case stepTypeFor, stepTypeForEach:
				// Create edge for loop body
				if loopTarget, ok := dataMap[fieldLoop].(string); ok && loopTarget != "" {
					if targetID, exists := nodeInfoMap[loopTarget]; exists {
						edge := edge.Edge{
							ID:            idwrap.NewNow(),
							FlowID:        flowID,
							SourceID:      nodeID,
							TargetID:      targetID.id,
							SourceHandler: edge.HandleLoop,
						}
						result.FlowEdges = append(result.FlowEdges, edge)
						hasIncoming[targetID.id] = true
					}
				}
			}
		}
	}

	return nil
}
