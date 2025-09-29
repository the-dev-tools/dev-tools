package yamlflowsimple

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"strings"
	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/model/mbodyform"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mbodyurl"
	"the-dev-tools/server/pkg/model/mcollection"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mexamplequery"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mflowvariable"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"the-dev-tools/server/pkg/model/mnnode/mnforeach"
	"the-dev-tools/server/pkg/model/mnnode/mnif"
	"the-dev-tools/server/pkg/model/mnnode/mnjs"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/model/mvar"
	"the-dev-tools/server/pkg/varsystem"
)

// ========================================
// Constants
// ========================================

const (
	// Field name constants
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

	// Node type constants
	stepTypeRequest = "request"
	stepTypeIf      = "if"
	stepTypeFor     = "for"
	stepTypeForEach = "for_each"
	stepTypeJS      = "js"
)

const (
	fieldAssertionExpression = "expression"
	fieldAssertionEnabled    = "enabled"
)

// ========================================
// Error Types
// ========================================

// YamlFlowError provides structured error information
type YamlFlowError struct {
	Message string
	Field   string
	Value   interface{}
}

func (e YamlFlowError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s: field '%s' with value '%v'", e.Message, e.Field, e.Value)
	}
	return e.Message
}

func newYamlFlowError(message, field string, value interface{}) error {
	return YamlFlowError{
		Message: message,
		Field:   field,
		Value:   value,
	}
}

// ========================================
// Configuration Types
// ========================================

// NameValue represents a name-value pair for headers, queries, etc.
type NameValue struct {
	Name  string
	Value string
}

// RequestConfig holds configuration for a request
type RequestConfig struct {
	Name        string
	Method      string
	URL         string
	Headers     []NameValue
	QueryParams []NameValue
	Body        interface{}
}

// requestContext holds all the IDs and data needed for request processing
type requestContext struct {
	nodeID           idwrap.IDWrap
	endpointID       idwrap.IDWrap
	deltaEndpointID  idwrap.IDWrap
	exampleID        idwrap.IDWrap
	defaultExampleID idwrap.IDWrap
	deltaExampleID   idwrap.IDWrap
}

type assertionConfig struct {
	Expression string
	Enabled    bool
}

// ========================================
// Main Entry Points
// ========================================

// ConvertSimplifiedYAML converts simplified YAML to all the entities needed for import
func ConvertSimplifiedYAML(data []byte, collectionID, workspaceID idwrap.IDWrap) (SimplifiedYAMLResolved, error) {
	result := SimplifiedYAMLResolved{}

	// Parse the YAML
	yamlflowData, err := Parse(data)
	if err != nil {
		return result, err
	}

	// Create collection
	collection := mcollection.Collection{
		ID:          collectionID,
		Name:        "YamlFlow Collection",
		WorkspaceID: workspaceID,
	}
	result.Collections = append(result.Collections, collection)

	// Convert flow data
	flow := yamlflowData.Flow
	flow.WorkspaceID = workspaceID
	result.Flows = append(result.Flows, flow)

	// Copy all flow nodes
	result.FlowNodes = yamlflowData.Nodes

	// Copy all edges
	result.FlowEdges = yamlflowData.Edges

	// Convert variables to flow variables
	for _, v := range yamlflowData.Variables {
		flowVar := mflowvariable.FlowVariable{
			ID:      idwrap.NewNow(),
			FlowID:  flow.ID,
			Name:    v.VarKey,
			Value:   v.Value,
			Enabled: true,
		}
		result.FlowVariables = append(result.FlowVariables, flowVar)
	}

	// Copy node implementations
	result.FlowRequestNodes = yamlflowData.RequestNodes
	result.FlowConditionNodes = yamlflowData.ConditionNodes
	result.FlowNoopNodes = yamlflowData.NoopNodes
	result.FlowForNodes = yamlflowData.ForNodes
	result.FlowForEachNodes = yamlflowData.ForEachNodes
	result.FlowJSNodes = yamlflowData.JSNodes

	// Process endpoints and examples
	for _, endpoint := range yamlflowData.Endpoints {
		// Set collection ID
		endpoint.CollectionID = collectionID
		result.Endpoints = append(result.Endpoints, endpoint)
	}

	// Process examples
	for _, example := range yamlflowData.Examples {
		// Set collection ID
		example.CollectionID = collectionID
		result.Examples = append(result.Examples, example)
	}

	// Copy headers, queries, and bodies
	result.Headers = yamlflowData.Headers
	result.Queries = yamlflowData.Queries
	result.RawBodies = yamlflowData.RawBodies
	result.FormBodies = yamlflowData.FormBodies
	result.UrlBodies = yamlflowData.UrlBodies
	result.Asserts = yamlflowData.Asserts

	// Set Prev/Next for endpoints
	for i := range result.Endpoints {
		if i > 0 {
			prevID := &result.Endpoints[i-1].ID
			result.Endpoints[i].Prev = prevID
		}
		if i < len(result.Endpoints)-1 {
			nextID := &result.Endpoints[i+1].ID
			result.Endpoints[i].Next = nextID
		}
	}

	// Set Prev/Next for examples
	for i := range result.Examples {
		if i > 0 {
			prevID := &result.Examples[i-1].ID
			result.Examples[i].Prev = prevID
		}
		if i < len(result.Examples)-1 {
			nextID := &result.Examples[i+1].ID
			result.Examples[i].Next = nextID
		}
	}

	return result, nil
}

// Parse parses the yamlflow YAML and returns YamlFlowData
func Parse(data []byte) (*YamlFlowData, error) {
	var yamlflow YamlFlowFormat
	var rawYamlFlow map[string]any

	// First unmarshal to a generic map to handle step types properly
	if err := yaml.Unmarshal(data, &rawYamlFlow); err != nil {
		return nil, fmt.Errorf("failed to unmarshal yamlflow format: %w", err)
	}

	// Then unmarshal to structured format
	if err := yaml.Unmarshal(data, &yamlflow); err != nil {
		return nil, fmt.Errorf("failed to unmarshal yamlflow format: %w", err)
	}

	if yamlflow.WorkspaceName == "" {
		return nil, newYamlFlowError("workspace_name is required", "workspace_name", nil)
	}

	// Parse run field if present
	var runEntries []RunEntry
	if len(yamlflow.Run) > 0 {
		parsedEntries, err := parseRunField(yamlflow.Run)
		if err != nil {
			return nil, fmt.Errorf("failed to parse run field: %w", err)
		}
		runEntries = parsedEntries
	}

	// Parse request templates (support both old and new format)
	var templates map[string]*requestTemplate
	if yamlflow.RequestTemplates != nil {
		templates = parseRequestTemplates(yamlflow.RequestTemplates)
	} else if yamlflow.Requests != nil {
		templates = parseRequests(yamlflow.Requests)
	} else {
		templates = make(map[string]*requestTemplate)
	}

	// Initialize yamlflow data
	yamlflowData := &YamlFlowData{
		Nodes:          make([]mnnode.MNode, 0),
		Edges:          make([]edge.Edge, 0),
		Variables:      make([]mvar.Var, 0),
		NoopNodes:      make([]mnnoop.NoopNode, 0),
		RequestNodes:   make([]mnrequest.MNRequest, 0),
		ConditionNodes: make([]mnif.MNIF, 0),
		ForNodes:       make([]mnfor.MNFor, 0),
		ForEachNodes:   make([]mnforeach.MNForEach, 0),
		JSNodes:        make([]mnjs.MNJS, 0),
		Endpoints:      make([]mitemapi.ItemApi, 0),
		Examples:       make([]mitemapiexample.ItemApiExample, 0),
		Headers:        make([]mexampleheader.Header, 0),
		Queries:        make([]mexamplequery.Query, 0),
		RawBodies:      make([]mbodyraw.ExampleBodyRaw, 0),
		FormBodies:     make([]mbodyform.BodyForm, 0),
		UrlBodies:      make([]mbodyurl.BodyURLEncoded, 0),
		Asserts:        make([]massert.Assert, 0),
	}

	// Determine which flow to process
	var flowToProcess YamlFlowFlow
	var flowName string

	if len(runEntries) > 0 {
		// If run field is present, use the first flow specified there
		flowName = runEntries[0].Flow
		found := false
		for _, f := range yamlflow.Flows {
			if f.Name == flowName {
				flowToProcess = f
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("flow '%s' specified in run field not found", flowName)
		}
	} else {
		// Otherwise, process first flow (backward compatibility)
		if len(yamlflow.Flows) == 0 {
			return nil, newYamlFlowError("at least one flow is required", "flows", nil)
		}
		flowToProcess = yamlflow.Flows[0]
		flowName = flowToProcess.Name
	}

	flowID := idwrap.NewNow()

	yamlflowData.Flow = mflow.Flow{
		ID:   flowID,
		Name: flowName,
	}

	// Process flow variables
	// Note: You can set a "timeout" variable to control flow execution timeout (in seconds)
	// Default is 60 seconds if not specified. Example: - name: timeout, value: "300"
	for _, v := range flowToProcess.Variables {
		yamlflowData.Variables = append(yamlflowData.Variables, mvar.Var{
			VarKey: v.Name,
			Value:  v.Value,
		})
	}

	// Create variable map for resolution
	varMap := varsystem.NewVarMap(yamlflowData.Variables)

	// Create start node
	startNodeID := idwrap.NewNow()
	startNode := mnnode.MNode{
		ID:       startNodeID,
		FlowID:   flowID,
		Name:     "Start",
		NodeKind: mnnode.NODE_KIND_NO_OP,
	}
	yamlflowData.Nodes = append(yamlflowData.Nodes, startNode)

	noopNode := mnnoop.NoopNode{
		FlowNodeID: startNodeID,
		Type:       mnnoop.NODE_NO_OP_KIND_START,
	}
	yamlflowData.NoopNodes = append(yamlflowData.NoopNodes, noopNode)

	// Get raw steps
	rawFlows, ok := rawYamlFlow[fieldFlows].([]any)
	if !ok || len(rawFlows) == 0 {
		return nil, fmt.Errorf("invalid flows format")
	}

	var rawSteps []map[string]any
	for _, rf := range rawFlows {
		rfMap, ok := rf.(map[string]any)
		if !ok {
			continue
		}
		if name, ok := rfMap[fieldName].(string); ok && name == flowName {
			if steps, ok := rfMap[fieldSteps].([]any); ok {
				for _, step := range steps {
					if stepMap, ok := step.(map[string]any); ok && len(stepMap) == 1 {
						rawSteps = append(rawSteps, stepMap)
					}
				}
			}
			break
		}
	}

	// Process steps
	nodeInfoMap := make(map[string]*nodeInfo)
	nodeList := make([]*nodeInfo, 0)

	for i, rawStep := range rawSteps {
		for stepType, stepData := range rawStep {
			dataMap, ok := stepData.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("invalid step data format")
			}

			nodeName, ok := dataMap[fieldName].(string)
			if !ok || nodeName == "" {
				return nil, newYamlFlowError(fmt.Sprintf("missing required '%s' field", fieldName), "", nil)
			}

			nodeID := idwrap.NewNow()
			info := &nodeInfo{
				id:    nodeID,
				name:  nodeName,
				index: i,
			}

			// Get dependencies
			if deps, ok := dataMap[fieldDependsOn].([]any); ok {
				for _, dep := range deps {
					if depStr, ok := dep.(string); ok && depStr != "" {
						info.dependsOn = append(info.dependsOn, depStr)
					}
				}
			}

			nodeInfoMap[nodeName] = info
			nodeList = append(nodeList, info)

			// Process step based on type
			switch stepType {
			case stepTypeRequest:
				if err := processRequestStepForNode(nodeName, nodeID, flowID, dataMap, templates, varMap, yamlflowData); err != nil {
					return nil, err
				}
			case stepTypeIf:
				if err := processIfStepForNode(nodeName, nodeID, flowID, dataMap, yamlflowData); err != nil {
					return nil, err
				}
			case stepTypeFor:
				if err := processForStepForNode(nodeName, nodeID, flowID, dataMap, yamlflowData); err != nil {
					return nil, err
				}
			case stepTypeForEach:
				if err := processForEachStepForNode(nodeName, nodeID, flowID, dataMap, yamlflowData); err != nil {
					return nil, err
				}
			case stepTypeJS:
				if err := processJSStepForNode(nodeName, nodeID, flowID, dataMap, yamlflowData); err != nil {
					return nil, err
				}
			default:
				return nil, newYamlFlowError("unknown step type", "stepType", stepType)
			}
		}
	}

	// Create edges
	createEdgesForFlow(flowID, startNodeID, nodeInfoMap, nodeList, rawSteps, yamlflowData)

	// Handle run dependencies if present
	if len(runEntries) > 0 {
		processRunDependencies(runEntries, flowName, nodeInfoMap, yamlflowData)
	}

	// Position nodes
	if err := positionNodes(yamlflowData); err != nil {
		return nil, err
	}

	return yamlflowData, nil
}

// ========================================
// Parsing Helper Functions
// ========================================

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
				// Single dependency
				entry.DependsOn = []string{v}
			case []any:
				// Multiple dependencies
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

// parseRequestTemplates parses request templates into a map
func parseRequestTemplates(templates map[string]map[string]any) map[string]*requestTemplate {
	result := make(map[string]*requestTemplate)
	for name, tmpl := range templates {
		result[name] = parseRequestDataFromMap(tmpl)
	}
	return result
}

// parseRequests parses the new requests format into templates
func parseRequests(requests []map[string]any) map[string]*requestTemplate {
	result := make(map[string]*requestTemplate)

	for _, req := range requests {
		// Get the request name
		name, ok := req[fieldName].(string)
		if !ok || name == "" {
			continue
		}

		result[name] = parseRequestDataFromMap(req)
	}

	return result
}

// parseRequestDataFromMap parses a single request template from data
func parseRequestDataFromMap(data map[string]any) *requestTemplate {
	rt := &requestTemplate{}
	if method, ok := data[fieldMethod].(string); ok && method != "" {
		rt.method = method
	}
	if url, ok := data[fieldURL].(string); ok && url != "" {
		rt.url = url
	}

	// Parse headers - support both map and array formats
	if headers, ok := data[fieldHeaders].(map[string]any); ok {
		// Map format: {"X-Header": "value"}
		for k, v := range headers {
			if str, ok := v.(string); ok {
				rt.headers = append(rt.headers, map[string]string{
					fieldName:  k,
					fieldValue: str,
				})
			}
		}
	} else if headers, ok := data[fieldHeaders].([]any); ok {
		// Array format (for parseRequests compatibility)
		for _, h := range headers {
			if hMap, ok := h.(map[string]any); ok {
				strMap := make(map[string]string)
				for k, v := range hMap {
					if str, ok := v.(string); ok {
						strMap[k] = str
					}
				}
				rt.headers = append(rt.headers, strMap)
			} else if hMap, ok := h.(map[string]string); ok {
				rt.headers = append(rt.headers, hMap)
			}
		}
	}

	// Parse query params - support both map and array formats
	if queryParams, ok := data[fieldQueryParams].(map[string]any); ok {
		// Map format: {"param": "value"}
		for k, v := range queryParams {
			if str, ok := v.(string); ok {
				rt.queryParams = append(rt.queryParams, map[string]string{
					fieldName:  k,
					fieldValue: str,
				})
			}
		}
	} else if queryParams, ok := data[fieldQueryParams].([]any); ok {
		// Array format (for parseRequests compatibility)
		for _, q := range queryParams {
			if qMap, ok := q.(map[string]any); ok {
				strMap := make(map[string]string)
				for k, v := range qMap {
					if str, ok := v.(string); ok {
						strMap[k] = str
					}
				}
				rt.queryParams = append(rt.queryParams, strMap)
			} else if qMap, ok := q.(map[string]string); ok {
				rt.queryParams = append(rt.queryParams, qMap)
			}
		}
	}

	if body, ok := data[fieldBody].(map[string]any); ok {
		if def, err := parseBodyDefinition(body); err == nil {
			rt.body = def
		}
	}

	if assertions, ok := data[fieldAssertions]; ok {
		rt.assertions = parseAssertionsFromAny(assertions)
	}

	return rt
}

func parseBodyDefinition(body map[string]any) (*bodyDefinition, error) {
	if body == nil {
		return nil, nil
	}
	typRaw, _ := body["type"].(string)
	typ := strings.TrimSpace(strings.ToLower(typRaw))
	switch typ {
	case "form-data":
		items := parseBodyItems(body["items"])
		if len(items) == 0 {
			return &bodyDefinition{kind: bodyKindRaw, raw: cloneBodyMap(body, "type")}, nil
		}
		return &bodyDefinition{kind: bodyKindForm, formItems: items}, nil
	case "x-www-form-urlencoded":
		items := parseBodyItems(body["items"])
		if len(items) == 0 {
			return &bodyDefinition{kind: bodyKindRaw, raw: cloneBodyMap(body, "type")}, nil
		}
		return &bodyDefinition{kind: bodyKindUrlEncoded, urlItems: items}, nil
	case "", "json":
		return &bodyDefinition{kind: bodyKindRaw, raw: cloneBodyMap(body, "type")}, nil
	default:
		return &bodyDefinition{kind: bodyKindRaw, raw: cloneBodyMap(body, "type")}, nil
	}
}

func parseBodyItems(raw any) []bodyItem {
	items := make([]bodyItem, 0)
	appendItem := func(m map[string]any) {
		if item, ok := bodyItemFromMap(m); ok {
			items = append(items, item)
		}
	}
	switch v := raw.(type) {
	case []any:
		for _, entry := range v {
			if m, ok := entry.(map[string]any); ok {
				appendItem(m)
			}
		}
	case []map[string]any:
		for _, m := range v {
			appendItem(m)
		}
	}
	return items
}

func bodyItemFromMap(raw map[string]any) (bodyItem, bool) {
	name, _ := raw["name"].(string)
	name = strings.TrimSpace(name)
	if name == "" {
		return bodyItem{}, false
	}
	valueStr := ""
	if val, ok := raw["value"].(string); ok {
		valueStr = val
	} else if val, exists := raw["value"]; exists && val != nil {
		valueStr = fmt.Sprint(val)
	}
	item := bodyItem{
		Name:    name,
		Value:   valueStr,
		Enabled: true,
	}
	if desc, ok := raw["description"].(string); ok {
		item.Description = desc
	}
	if enabledRaw, ok := raw["enabled"]; ok {
		item.Enabled = parseEnabledFlag(enabledRaw)
	}
	return item, true
}

func cloneBodyMap(body map[string]any, skipKeys ...string) map[string]any {
	if len(body) == 0 {
		return nil
	}
	skip := make(map[string]struct{}, len(skipKeys))
	for _, key := range skipKeys {
		skip[key] = struct{}{}
	}
	result := make(map[string]any, len(body))
	for k, v := range body {
		if _, ignore := skip[k]; ignore {
			continue
		}
		result[k] = v
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func parseEnabledFlag(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		s := strings.TrimSpace(strings.ToLower(v))
		return s != "false" && s != "0" && s != "no" && s != "off"
	case float64:
		return v != 0
	case int:
		return v != 0
	case int64:
		return v != 0
	default:
		return true
	}
}

func selectBodyDefinitions(usingTemplate bool, templateDef, stepDef *bodyDefinition) (*bodyDefinition, *bodyDefinition) {
	base := templateDef
	if base == nil && !usingTemplate {
		base = stepDef
	}
	delta := stepDef
	if delta == nil {
		delta = base
	}
	return base, delta
}

func bodyDefinitionToExampleType(def *bodyDefinition) mitemapiexample.BodyType {
	if def == nil {
		return mitemapiexample.BodyTypeRaw
	}
	switch def.kind {
	case bodyKindForm:
		return mitemapiexample.BodyTypeForm
	case bodyKindUrlEncoded:
		return mitemapiexample.BodyTypeUrlencoded
	default:
		return mitemapiexample.BodyTypeRaw
	}
}

func applyBodyDefinitions(ctx *requestContext, baseDef, deltaDef *bodyDefinition, data *YamlFlowData) error {
	if baseDef == nil {
		baseDef = &bodyDefinition{kind: bodyKindRaw}
	}
	if deltaDef == nil {
		deltaDef = baseDef
	}
	if err := setBodyForExample(ctx.exampleID, baseDef, data); err != nil {
		return err
	}
	if err := setBodyForExample(ctx.defaultExampleID, baseDef, data); err != nil {
		return err
	}
	if err := setBodyForExample(ctx.deltaExampleID, deltaDef, data); err != nil {
		return err
	}
	return nil
}

func setBodyForExample(exampleID idwrap.IDWrap, def *bodyDefinition, data *YamlFlowData) error {
	kind := bodyKindRaw
	if def != nil {
		kind = def.kind
	}
	switch kind {
	case bodyKindForm:
		formMap := make(map[string]any, len(def.formItems))
		for _, item := range def.formItems {
			data.FormBodies = append(data.FormBodies, mbodyform.BodyForm{
				ID:          idwrap.NewNow(),
				ExampleID:   exampleID,
				BodyKey:     item.Name,
				Value:       item.Value,
				Description: item.Description,
				Enable:      item.Enabled,
			})
			if _, exists := formMap[item.Name]; !exists {
				formMap[item.Name] = item.Value
			}
		}
		bodyBytes, err := marshalBodyMap(formMap)
		if err != nil {
			return err
		}
		addRawBody(exampleID, bodyBytes, data)
	case bodyKindUrlEncoded:
		urlMap := make(map[string]any, len(def.urlItems))
		for _, item := range def.urlItems {
			data.UrlBodies = append(data.UrlBodies, mbodyurl.BodyURLEncoded{
				ID:          idwrap.NewNow(),
				ExampleID:   exampleID,
				BodyKey:     item.Name,
				Value:       item.Value,
				Description: item.Description,
				Enable:      item.Enabled,
			})
			if _, exists := urlMap[item.Name]; !exists {
				urlMap[item.Name] = item.Value
			}
		}
		bodyBytes, err := marshalBodyMap(urlMap)
		if err != nil {
			return err
		}
		addRawBody(exampleID, bodyBytes, data)
	default:
		bodyBytes, err := marshalBodyMap(nil)
		if def != nil {
			bodyBytes, err = marshalBodyMap(def.raw)
		}
		if err != nil {
			return err
		}
		addRawBody(exampleID, bodyBytes, data)
	}
	return nil
}

func marshalBodyMap(raw map[string]any) ([]byte, error) {
	if len(raw) == 0 {
		return []byte("{}"), nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return []byte("{}"), nil
	}
	return data, nil
}

func addRawBody(exampleID idwrap.IDWrap, bodyData []byte, data *YamlFlowData) {
	if bodyData == nil {
		bodyData = []byte("{}")
	}
	visualMode := mbodyraw.VisualizeModeJSON
	if !json.Valid(bodyData) {
		visualMode = mbodyraw.VisualizeModeText
	}
	data.RawBodies = append(data.RawBodies, mbodyraw.ExampleBodyRaw{
		ID:            idwrap.NewNow(),
		ExampleID:     exampleID,
		Data:          bodyData,
		CompressType:  compress.CompressTypeNone,
		VisualizeMode: visualMode,
	})
}

// ========================================
// Node Processing Functions
// ========================================

// addNodeWithName adds a flow node with the given name
func addNodeWithName(nodeName string, nodeID, flowID idwrap.IDWrap, kind mnnode.NodeKind, data *YamlFlowData) {
	data.Nodes = append(data.Nodes, mnnode.MNode{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     nodeName,
		NodeKind: kind,
	})
}

// processRequestStepForNode processes a request step for a given node
func processRequestStepForNode(nodeName string, nodeID, flowID idwrap.IDWrap, stepData map[string]any, templates map[string]*requestTemplate, varMap varsystem.VarMap, data *YamlFlowData) error {
	// Initialize request configuration
	method, url := "GET", ""
	var templateHeaders, templateQueries, stepHeaderOverrides, stepQueryOverrides []map[string]string
	var usingTemplate bool
	var templateAssertions, stepAssertions []assertionConfig
	var templateBodyDef *bodyDefinition
	var stepBodyDef *bodyDefinition

	// Check if using template
	if useRequest, ok := stepData[fieldUseRequest].(string); ok && useRequest != "" {
		if tmpl, exists := templates[useRequest]; exists {
			usingTemplate = true
			templateHeaders = tmpl.headers
			templateQueries = tmpl.queryParams
			templateBodyDef = tmpl.body
			templateAssertions = tmpl.assertions
			if tmpl.method != "" {
				method = tmpl.method
			}
			if tmpl.url != "" {
				url = tmpl.url
			}
		} else {
			return newYamlFlowError(fmt.Sprintf("request step '%s' references unknown template '%s'", nodeName, useRequest), fieldUseRequest, useRequest)
		}
	}

	// Override with step-specific data
	if m, ok := stepData[fieldMethod].(string); ok && m != "" {
		method = m
	}
	if u, ok := stepData[fieldURL].(string); ok && u != "" {
		url = u
	}
	// URL is required either from template or step definition
	if url == "" {
		return newYamlFlowError(fmt.Sprintf("request step '%s' missing required url", nodeName), fieldURL, nil)
	}

	// Parse step overrides
	if h, ok := stepData[fieldHeaders].(map[string]any); ok {
		for k, v := range h {
			if vs, ok := v.(string); ok {
				stepHeaderOverrides = append(stepHeaderOverrides, map[string]string{fieldName: k, fieldValue: vs})
			}
		}
	}
	if q, ok := stepData[fieldQueryParams].(map[string]any); ok {
		for k, v := range q {
			if vs, ok := v.(string); ok {
				stepQueryOverrides = append(stepQueryOverrides, map[string]string{fieldName: k, fieldValue: vs})
			}
		}
	}
	if b, ok := stepData[fieldBody].(map[string]any); ok {
		def, err := parseBodyDefinition(b)
		if err != nil {
			return fmt.Errorf("request step '%s' has invalid body: %w", nodeName, err)
		}
		stepBodyDef = def
	}
	if assertions, ok := stepData[fieldAssertions]; ok {
		stepAssertions = parseAssertionsFromAny(assertions)
	}

	baseDef, deltaDef := selectBodyDefinitions(usingTemplate, templateBodyDef, stepBodyDef)
	baseBodyType := bodyDefinitionToExampleType(baseDef)
	deltaBodyType := bodyDefinitionToExampleType(deltaDef)

	// Create all request entities
	ctx := createRequestEntitiesForNode(nodeName, nodeID, flowID, url, method, baseBodyType, deltaBodyType, data)

	// Process headers
	processNameValuePairsForExamples(
		ctx.exampleID, ctx.defaultExampleID, ctx.deltaExampleID,
		templateHeaders, stepHeaderOverrides,
		usingTemplate,
		varMap,
		func(name, value string, id, exampleID idwrap.IDWrap, deltaParentID *idwrap.IDWrap) interface{} {
			return mexampleheader.Header{
				ID:            id,
				ExampleID:     exampleID,
				HeaderKey:     name,
				Value:         value,
				DeltaParentID: deltaParentID,
				Enable:        true,
			}
		},
		func(item interface{}) {
			data.Headers = append(data.Headers, item.(mexampleheader.Header))
		},
		data,
	)

	// Process query parameters
	processNameValuePairsForExamples(
		ctx.exampleID, ctx.defaultExampleID, ctx.deltaExampleID,
		templateQueries, stepQueryOverrides,
		usingTemplate,
		varMap,
		func(name, value string, id, exampleID idwrap.IDWrap, deltaParentID *idwrap.IDWrap) interface{} {
			return mexamplequery.Query{
				ID:            id,
				ExampleID:     exampleID,
				QueryKey:      name,
				Value:         value,
				DeltaParentID: deltaParentID,
				Enable:        true,
			}
		},
		func(item interface{}) {
			data.Queries = append(data.Queries, item.(mexamplequery.Query))
		},
		data,
	)

	if err := applyBodyDefinitions(ctx, baseDef, deltaDef, data); err != nil {
		return err
	}

	processAssertionsForExamples(ctx, templateAssertions, stepAssertions, usingTemplate, data)

	return nil
}

// processIfStepForNode processes an if step for a given node
func processIfStepForNode(nodeName string, nodeID, flowID idwrap.IDWrap, stepData map[string]any, data *YamlFlowData) error {
	addNodeWithName(nodeName, nodeID, flowID, mnnode.NODE_KIND_CONDITION, data)

	condition, ok := stepData[fieldCondition].(string)
	if !ok || condition == "" {
		return newYamlFlowError(fmt.Sprintf("if step '%s' missing required condition", nodeName), fieldCondition, nil)
	}

	data.ConditionNodes = append(data.ConditionNodes, mnif.MNIF{
		FlowNodeID: nodeID,
		Condition: mcondition.Condition{
			Comparisons: mcondition.Comparison{Expression: condition},
		},
	})
	return nil
}

// processForStepForNode processes a for step for a given node
func processForStepForNode(nodeName string, nodeID, flowID idwrap.IDWrap, stepData map[string]any, data *YamlFlowData) error {
	addNodeWithName(nodeName, nodeID, flowID, mnnode.NODE_KIND_FOR, data)

	iterCount := 1 // Default to 1 if not specified
	if val, ok := stepData[fieldIterCount]; ok {
		if i, ok := val.(int); ok {
			iterCount = i
		} else if f, ok := val.(float64); ok {
			iterCount = int(f)
		}
	}

	data.ForNodes = append(data.ForNodes, mnfor.MNFor{
		FlowNodeID: nodeID,
		IterCount:  int64(iterCount),
	})
	return nil
}

// processForEachStepForNode processes a for_each step for a given node
func processForEachStepForNode(nodeName string, nodeID, flowID idwrap.IDWrap, stepData map[string]any, data *YamlFlowData) error {
	addNodeWithName(nodeName, nodeID, flowID, mnnode.NODE_KIND_FOR_EACH, data)

	items, ok := stepData[fieldItems].(string)
	if !ok || items == "" {
		return newYamlFlowError(fmt.Sprintf("for_each step '%s' missing required items", nodeName), fieldItems, nil)
	}

	data.ForEachNodes = append(data.ForEachNodes, mnforeach.MNForEach{
		FlowNodeID:     nodeID,
		IterExpression: items,
	})
	return nil
}

// processJSStepForNode processes a JavaScript step for a given node
func processJSStepForNode(nodeName string, nodeID, flowID idwrap.IDWrap, stepData map[string]any, data *YamlFlowData) error {
	addNodeWithName(nodeName, nodeID, flowID, mnnode.NODE_KIND_JS, data)

	code, ok := stepData[fieldCode].(string)
	if !ok || code == "" {
		return newYamlFlowError(fmt.Sprintf("js step '%s' missing required code", nodeName), fieldCode, nil)
	}

	data.JSNodes = append(data.JSNodes, mnjs.MNJS{
		FlowNodeID: nodeID,
		Code:       []byte(code),
	})
	return nil
}

// ========================================
// Request Helper Functions
// ========================================

// createRequestEntitiesForNode creates all the entities needed for a request node
func createRequestEntitiesForNode(nodeName string, nodeID, flowID idwrap.IDWrap, url, method string, baseBodyType, deltaBodyType mitemapiexample.BodyType, data *YamlFlowData) *requestContext {
	ctx := &requestContext{
		nodeID:           nodeID,
		endpointID:       idwrap.NewNow(),
		deltaEndpointID:  idwrap.NewNow(),
		exampleID:        idwrap.NewNow(),
		defaultExampleID: idwrap.NewNow(),
		deltaExampleID:   idwrap.NewNow(),
	}

	if baseBodyType == mitemapiexample.BodyTypeNone {
		baseBodyType = mitemapiexample.BodyTypeRaw
	}
	if deltaBodyType == mitemapiexample.BodyTypeNone {
		deltaBodyType = baseBodyType
	}

	// Add node
	data.Nodes = append(data.Nodes, mnnode.MNode{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     nodeName,
		NodeKind: mnnode.NODE_KIND_REQUEST,
	})

	// Add endpoints
	data.Endpoints = append(data.Endpoints,
		mitemapi.ItemApi{ID: ctx.endpointID, Name: nodeName, Url: url, Method: method},
		mitemapi.ItemApi{
			ID:            ctx.deltaEndpointID,
			Name:          fmt.Sprintf("%s (delta)", nodeName),
			Url:           url,
			Method:        method,
			DeltaParentID: &ctx.endpointID,
			Hidden:        true,
		},
	)

	// Add examples
	data.Examples = append(data.Examples,
		mitemapiexample.ItemApiExample{
			ID:        ctx.exampleID,
			Name:      nodeName,
			ItemApiID: ctx.endpointID,
			IsDefault: true,
			BodyType:  baseBodyType,
		},
		mitemapiexample.ItemApiExample{
			ID:        ctx.defaultExampleID,
			Name:      fmt.Sprintf("%s (default)", nodeName),
			ItemApiID: ctx.endpointID,
			IsDefault: false,
			BodyType:  baseBodyType,
		},
		mitemapiexample.ItemApiExample{
			ID:              ctx.deltaExampleID,
			Name:            fmt.Sprintf("%s (delta)", nodeName),
			ItemApiID:       ctx.deltaEndpointID,
			IsDefault:       true,
			VersionParentID: &ctx.defaultExampleID,
			BodyType:        deltaBodyType,
		},
	)

	// Add request node
	data.RequestNodes = append(data.RequestNodes, mnrequest.MNRequest{
		FlowNodeID:       nodeID,
		EndpointID:       &ctx.endpointID,
		ExampleID:        &ctx.exampleID,
		DeltaEndpointID:  &ctx.deltaEndpointID,
		DeltaExampleID:   &ctx.deltaExampleID,
		HasRequestConfig: true,
	})

	return ctx
}

func parseAssertionsFromAny(value interface{}) []assertionConfig {
	result := make([]assertionConfig, 0)
	appendConfig := func(raw map[string]any) {
		if cfg, ok := convertAssertionMap(raw); ok {
			result = append(result, cfg)
		}
	}

	switch v := value.(type) {
	case []any:
		for _, item := range v {
			if raw, ok := item.(map[string]any); ok {
				appendConfig(raw)
			} else if rawStr, ok := item.(map[string]string); ok {
				rawAny := make(map[string]any, len(rawStr))
				for k, val := range rawStr {
					rawAny[k] = val
				}
				appendConfig(rawAny)
			}
		}
	case []map[string]any:
		for _, item := range v {
			appendConfig(item)
		}
	case []map[string]string:
		for _, item := range v {
			raw := make(map[string]any, len(item))
			for k, val := range item {
				raw[k] = val
			}
			appendConfig(raw)
		}
	}

	return result
}

func convertAssertionMap(raw map[string]any) (assertionConfig, bool) {
	expr, ok := raw[fieldAssertionExpression].(string)
	if !ok || expr == "" {
		return assertionConfig{}, false
	}
	enabled := true
	if enabledRaw, exists := raw[fieldAssertionEnabled]; exists {
		switch v := enabledRaw.(type) {
		case bool:
			enabled = v
		case string:
			if v == "false" || v == "0" {
				enabled = false
			}
		case int:
			enabled = v != 0
		case int64:
			enabled = v != 0
		}
	}

	return assertionConfig{Expression: expr, Enabled: enabled}, true
}

func processAssertionsForExamples(ctx *requestContext, templateAssertions, stepAssertions []assertionConfig, usingTemplate bool, data *YamlFlowData) {
	var baseConfigs []assertionConfig
	var deltaConfigs []assertionConfig

	if usingTemplate {
		if len(templateAssertions) > 0 {
			baseConfigs = append(baseConfigs, templateAssertions...)
		}
		if len(stepAssertions) > 0 {
			deltaConfigs = append(deltaConfigs, stepAssertions...)
		}
	} else {
		if len(stepAssertions) > 0 {
			baseConfigs = append(baseConfigs, stepAssertions...)
		} else if len(templateAssertions) > 0 {
			baseConfigs = append(baseConfigs, templateAssertions...)
		}
	}

	if len(baseConfigs) == 0 && len(deltaConfigs) == 0 {
		return
	}

	if len(baseConfigs) == 0 {
		baseConfigs = append(baseConfigs, deltaConfigs...)
	}

	// Ensure base configs can cover delta entries for parent mapping
	if len(deltaConfigs) > len(baseConfigs) {
		baseConfigs = append(baseConfigs, deltaConfigs[len(baseConfigs):]...)
	}

	baseAsserts := buildAssertionsForExample(ctx.exampleID, baseConfigs, nil)
	defaultAsserts := buildAssertionsForExample(ctx.defaultExampleID, baseConfigs, nil)
	data.Asserts = append(data.Asserts, baseAsserts...)
	data.Asserts = append(data.Asserts, defaultAsserts...)

	if len(baseAsserts) == 0 {
		return
	}

	if !usingTemplate {
		deltaConfigs = baseConfigs
	} else if len(deltaConfigs) == 0 {
		deltaConfigs = baseConfigs
	}

	if len(deltaConfigs) == 0 {
		return
	}

	deltaAsserts := buildAssertionsForExample(ctx.deltaExampleID, deltaConfigs, baseAsserts)
	data.Asserts = append(data.Asserts, deltaAsserts...)
}

func buildAssertionsForExample(exampleID idwrap.IDWrap, configs []assertionConfig, parentRefs []massert.Assert) []massert.Assert {
	if len(configs) == 0 {
		return nil
	}

	asserts := make([]massert.Assert, len(configs))
	for i, cfg := range configs {
		assert := massert.Assert{
			ID:        idwrap.NewNow(),
			ExampleID: exampleID,
			Condition: mcondition.Condition{
				Comparisons: mcondition.Comparison{Expression: cfg.Expression},
			},
			Enable: cfg.Enabled,
		}
		if parentRefs != nil && i < len(parentRefs) {
			parentID := parentRefs[i].ID
			assert.DeltaParentID = &parentID
		}
		asserts[i] = assert
	}

	linkAssertionList(asserts)
	return asserts
}

func linkAssertionList(asserts []massert.Assert) {
	for i := range asserts {
		if i > 0 {
			prevID := asserts[i-1].ID
			asserts[i].Prev = &prevID
		} else {
			asserts[i].Prev = nil
		}

		if i < len(asserts)-1 {
			nextID := asserts[i+1].ID
			asserts[i].Next = &nextID
		} else {
			asserts[i].Next = nil
		}
	}
}

// ========================================
// Name-Value Processing Functions
// ========================================

// convertToNameValueMap converts various formats to a simple name-value map
func convertToNameValueMap(pairs []map[string]string) map[string]string {
	result := make(map[string]string)
	for _, p := range pairs {
		if name, ok := p[fieldName]; ok {
			if value, ok := p[fieldValue]; ok {
				result[name] = value
			}
		} else {
			// Direct map format
			for k, v := range p {
				if k != fieldName && k != fieldValue {
					result[k] = v
				}
			}
		}
	}
	return result
}

// processNameValuePairs processes headers or query parameters generically
func processNameValuePairsForExamples(
	exampleID, defaultExampleID, deltaExampleID idwrap.IDWrap,
	templatePairs, overridePairs []map[string]string,
	usingTemplate bool,
	varMap varsystem.VarMap,
	createFunc func(name, value string, id, exampleID idwrap.IDWrap, deltaParentID *idwrap.IDWrap) interface{},
	appendFunc func(interface{}),
	data *YamlFlowData,
) {
	getID := func(item interface{}) idwrap.IDWrap {
		switch v := item.(type) {
		case mexampleheader.Header:
			return v.ID
		case mexamplequery.Query:
			return v.ID
		default:
			return idwrap.IDWrap{}
		}
	}

	if usingTemplate {
		templateMap := convertToNameValueMap(templatePairs)
		overrideMap := convertToNameValueMap(overridePairs)
		processedNames := make(map[string]bool)

		// Process template items
		for name, templateValue := range templateMap {
			processedNames[name] = true

			// Base item
			baseItem := createFunc(name, templateValue, idwrap.NewNow(), exampleID, nil)
			appendFunc(baseItem)
			baseID := getID(baseItem)

			// Default item with resolved value
			resolvedValue, _ := varMap.ReplaceVars(templateValue)
			defaultItem := createFunc(name, resolvedValue, idwrap.NewNow(), defaultExampleID, nil)
			appendFunc(defaultItem)
			defaultID := getID(defaultItem)
			if baseID == (idwrap.IDWrap{}) {
				// Fallback to default ID if we couldn't detect the base ID type
				baseID = defaultID
			}

			// Check if overridden
			if overrideValue, isOverridden := overrideMap[name]; isOverridden {
				deltaItem := createFunc(name, overrideValue, idwrap.NewNow(), deltaExampleID, &baseID)
				appendFunc(deltaItem)
			}
		}

		// Process override-only items
		for name, overrideValue := range overrideMap {
			if !processedNames[name] {
				// Base item
				baseItem := createFunc(name, overrideValue, idwrap.NewNow(), exampleID, nil)
				appendFunc(baseItem)
				baseID := getID(baseItem)

				// Default item
				resolvedValue, _ := varMap.ReplaceVars(overrideValue)
				defaultItem := createFunc(name, resolvedValue, idwrap.NewNow(), defaultExampleID, nil)
				appendFunc(defaultItem)
				defaultID := getID(defaultItem)
				if baseID == (idwrap.IDWrap{}) {
					baseID = defaultID
				}

				// Delta item
				deltaItem := createFunc(name, overrideValue, idwrap.NewNow(), deltaExampleID, &baseID)
				appendFunc(deltaItem)
			}
		}
	} else {
		// No template - process directly
		directMap := convertToNameValueMap(overridePairs)
		for name, value := range directMap {
			// Base item
			baseItem := createFunc(name, value, idwrap.NewNow(), exampleID, nil)
			appendFunc(baseItem)
			baseID := getID(baseItem)

			// Default item
			resolvedValue, _ := varMap.ReplaceVars(value)
			defaultItem := createFunc(name, resolvedValue, idwrap.NewNow(), defaultExampleID, nil)
			appendFunc(defaultItem)
			defaultID := getID(defaultItem)
			if baseID == (idwrap.IDWrap{}) {
				baseID = defaultID
			}

			// Delta item if has variables
			if varsystem.CheckStringHasAnyVarKey(value) {
				deltaItem := createFunc(name, value, idwrap.NewNow(), deltaExampleID, &baseID)
				appendFunc(deltaItem)
			}
		}
	}
}

// ========================================
// Edge Creation Functions
// ========================================

// createEdgesForFlow creates edges based on dependencies and sequential order
func createEdgesForFlow(flowID, startNodeID idwrap.IDWrap, nodeInfoMap map[string]*nodeInfo, nodeList []*nodeInfo, rawSteps []map[string]any, data *YamlFlowData) {
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
				data.Edges = append(data.Edges, edge)
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
			data.Edges = append(data.Edges, edge)
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
			data.Edges = append(data.Edges, edge)
		}
	}

	// Create edges for control flow nodes
	// We need to parse the original step data to get then/else/loop targets
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
						data.Edges = append(data.Edges, edge)
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
						data.Edges = append(data.Edges, edge)
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
						data.Edges = append(data.Edges, edge)
						hasIncoming[targetID.id] = true
					}
				}
			}
		}
	}
}

// processRunDependencies handles dependencies specified in the run field
func processRunDependencies(runEntries []RunEntry, currentFlowName string, nodeInfoMap map[string]*nodeInfo, data *YamlFlowData) {
	// Find the current flow's run entry
	var currentEntry *RunEntry
	for i := range runEntries {
		if runEntries[i].Flow == currentFlowName {
			currentEntry = &runEntries[i]
			break
		}
	}

	if currentEntry == nil || len(currentEntry.DependsOn) == 0 {
		// No dependencies to process
		return
	}

	// For the current implementation, we'll create implicit dependencies
	// by making the first node of the current flow depend on the last nodes
	// of the dependency flows/requests

	// Find the first node in the current flow (excluding Start node)
	var firstNode *nodeInfo
	for _, node := range nodeInfoMap {
		if firstNode == nil || node.index < firstNode.index {
			firstNode = node
		}
	}

	if firstNode == nil {
		return
	}

	// Process each dependency
	for _, dep := range currentEntry.DependsOn {
		// Check if dependency is a node in the current flow
		if depNode, exists := nodeInfoMap[dep]; exists {
			// Add this dependency to the first node's dependsOn list
			firstNode.dependsOn = append(firstNode.dependsOn, depNode.name)
		}
		// Note: Cross-flow dependencies would require more complex handling
		// which is not currently supported by the single-flow architecture
	}
}
