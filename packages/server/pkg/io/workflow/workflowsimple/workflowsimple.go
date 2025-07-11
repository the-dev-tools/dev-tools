package workflowsimple

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mbodyraw"
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

// ========================================
// Error Types
// ========================================

// WorkflowError provides structured error information
type WorkflowError struct {
	Message string
	Field   string
	Value   interface{}
}

func (e WorkflowError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s: field '%s' with value '%v'", e.Message, e.Field, e.Value)
	}
	return e.Message
}

func newWorkflowError(message, field string, value interface{}) error {
	return WorkflowError{
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

// ========================================
// Main Entry Points
// ========================================

// ConvertSimplifiedYAML converts simplified YAML to all the entities needed for import
func ConvertSimplifiedYAML(data []byte, collectionID, workspaceID idwrap.IDWrap) (SimplifiedYAMLResolved, error) {
	result := SimplifiedYAMLResolved{}

	// Parse the YAML
	workflowData, err := Parse(data)
	if err != nil {
		return result, err
	}

	// Create collection
	collection := mcollection.Collection{
		ID:          collectionID,
		Name:        "Workflow Collection",
		WorkspaceID: workspaceID,
	}
	result.Collections = append(result.Collections, collection)

	// Convert flow data
	flow := workflowData.Flow
	flow.WorkspaceID = workspaceID
	result.Flows = append(result.Flows, flow)

	// Copy all flow nodes
	result.FlowNodes = workflowData.Nodes

	// Copy all edges
	result.FlowEdges = workflowData.Edges

	// Convert variables to flow variables
	for _, v := range workflowData.Variables {
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
	result.FlowRequestNodes = workflowData.RequestNodes
	result.FlowConditionNodes = workflowData.ConditionNodes
	result.FlowNoopNodes = workflowData.NoopNodes
	result.FlowForNodes = workflowData.ForNodes
	result.FlowForEachNodes = workflowData.ForEachNodes
	result.FlowJSNodes = workflowData.JSNodes

	// Process endpoints and examples
	for _, endpoint := range workflowData.Endpoints {
		// Set collection ID
		endpoint.CollectionID = collectionID
		result.Endpoints = append(result.Endpoints, endpoint)
	}

	// Process examples
	for _, example := range workflowData.Examples {
		// Set collection ID
		example.CollectionID = collectionID
		result.Examples = append(result.Examples, example)
	}

	// Copy headers, queries, and bodies
	result.Headers = workflowData.Headers
	result.Queries = workflowData.Queries
	result.RawBodies = workflowData.RawBodies

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

// Parse parses the workflow YAML and returns WorkflowData
func Parse(data []byte) (*WorkflowData, error) {
	var workflow WorkflowFormat
	var rawWorkflow map[string]any

	// First unmarshal to a generic map to handle step types properly
	if err := yaml.Unmarshal(data, &rawWorkflow); err != nil {
		return nil, fmt.Errorf("failed to unmarshal workflow format: %w", err)
	}

	// Then unmarshal to structured format
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		return nil, fmt.Errorf("failed to unmarshal workflow format: %w", err)
	}

	if workflow.WorkspaceName == "" {
		return nil, newWorkflowError("workspace_name is required", "workspace_name", nil)
	}

	// Parse run field if present
	var runEntries []RunEntry
	if len(workflow.Run) > 0 {
		parsedEntries, err := parseRunField(workflow.Run)
		if err != nil {
			return nil, fmt.Errorf("failed to parse run field: %w", err)
		}
		runEntries = parsedEntries
	}

	// Parse request templates (support both old and new format)
	var templates map[string]*requestTemplate
	if workflow.RequestTemplates != nil {
		templates = parseRequestTemplates(workflow.RequestTemplates)
	} else if workflow.Requests != nil {
		templates = parseRequests(workflow.Requests)
	} else {
		templates = make(map[string]*requestTemplate)
	}

	// Initialize workflow data
	workflowData := &WorkflowData{
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
	}

	// Determine which flow to process
	var flowToProcess WorkflowFlow
	var flowName string

	if len(runEntries) > 0 {
		// If run field is present, use the first flow specified there
		flowName = runEntries[0].Flow
		found := false
		for _, f := range workflow.Flows {
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
		if len(workflow.Flows) == 0 {
			return nil, newWorkflowError("at least one flow is required", "flows", nil)
		}
		flowToProcess = workflow.Flows[0]
		flowName = flowToProcess.Name
	}

	flowID := idwrap.NewNow()

	workflowData.Flow = mflow.Flow{
		ID:   flowID,
		Name: flowName,
	}

	// Process flow variables
	// Note: You can set a "timeout" variable to control flow execution timeout (in seconds)
	// Default is 60 seconds if not specified. Example: - name: timeout, value: "300"
	for _, v := range flowToProcess.Variables {
		workflowData.Variables = append(workflowData.Variables, mvar.Var{
			VarKey: v.Name,
			Value:  v.Value,
		})
	}

	// Create variable map for resolution
	varMap := varsystem.NewVarMap(workflowData.Variables)

	// Create start node
	startNodeID := idwrap.NewNow()
	startNode := mnnode.MNode{
		ID:       startNodeID,
		FlowID:   flowID,
		Name:     "Start",
		NodeKind: mnnode.NODE_KIND_NO_OP,
	}
	workflowData.Nodes = append(workflowData.Nodes, startNode)

	noopNode := mnnoop.NoopNode{
		FlowNodeID: startNodeID,
		Type:       mnnoop.NODE_NO_OP_KIND_START,
	}
	workflowData.NoopNodes = append(workflowData.NoopNodes, noopNode)

	// Get raw steps
	rawFlows, ok := rawWorkflow[fieldFlows].([]any)
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
				return nil, newWorkflowError("step missing required field", fieldName, nodeName)
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
				if err := processRequestStepForNode(nodeName, nodeID, flowID, dataMap, templates, varMap, workflowData); err != nil {
					return nil, err
				}
			case stepTypeIf:
				if err := processIfStepForNode(nodeName, nodeID, flowID, dataMap, workflowData); err != nil {
					return nil, err
				}
			case stepTypeFor:
				if err := processForStepForNode(nodeName, nodeID, flowID, dataMap, workflowData); err != nil {
					return nil, err
				}
			case stepTypeForEach:
				if err := processForEachStepForNode(nodeName, nodeID, flowID, dataMap, workflowData); err != nil {
					return nil, err
				}
			case stepTypeJS:
				if err := processJSStepForNode(nodeName, nodeID, flowID, dataMap, workflowData); err != nil {
					return nil, err
				}
			default:
				return nil, newWorkflowError("unknown step type", "stepType", stepType)
			}
		}
	}

	// Create edges
	createEdgesForFlow(flowID, startNodeID, nodeInfoMap, nodeList, rawSteps, workflowData)

	// Handle run dependencies if present
	if len(runEntries) > 0 {
		processRunDependencies(runEntries, flowName, nodeInfoMap, workflowData)
	}

	// Position nodes
	if err := positionNodes(workflowData); err != nil {
		return nil, err
	}

	return workflowData, nil
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
		rt.body = body
	}

	return rt
}

// ========================================
// Node Processing Functions
// ========================================

// addNodeWithName adds a flow node with the given name
func addNodeWithName(nodeName string, nodeID, flowID idwrap.IDWrap, kind mnnode.NodeKind, data *WorkflowData) {
	data.Nodes = append(data.Nodes, mnnode.MNode{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     nodeName,
		NodeKind: kind,
	})
}

// processRequestStepForNode processes a request step for a given node
func processRequestStepForNode(nodeName string, nodeID, flowID idwrap.IDWrap, stepData map[string]any, templates map[string]*requestTemplate, varMap varsystem.VarMap, data *WorkflowData) error {
	// Initialize request configuration
	method, url := "GET", ""
	var templateHeaders, templateQueries, stepHeaderOverrides, stepQueryOverrides []map[string]string
	var templateBody, stepBodyOverride map[string]any
	var usingTemplate bool

	// Check if using template
	if useRequest, ok := stepData[fieldUseRequest].(string); ok && useRequest != "" {
		if tmpl, exists := templates[useRequest]; exists {
			usingTemplate = true
			templateHeaders = tmpl.headers
			templateQueries = tmpl.queryParams
			templateBody = tmpl.body
			if tmpl.method != "" {
				method = tmpl.method
			}
			if tmpl.url != "" {
				url = tmpl.url
			}
		} else {
			return newWorkflowError(fmt.Sprintf("request step '%s' references unknown template '%s'", nodeName, useRequest), fieldUseRequest, useRequest)
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
		return newWorkflowError(fmt.Sprintf("request step '%s' missing required url", nodeName), fieldURL, nil)
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
		stepBodyOverride = b
	}

	// Create all request entities
	ctx := createRequestEntitiesForNode(nodeName, nodeID, flowID, url, method, data)

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

	// Process body
	var bodyData []byte
	if usingTemplate && templateBody != nil {
		// Use template body for base
		bodyData, _ = json.Marshal(templateBody)
		addBodyToExamples(ctx, bodyData, data)

		// If there's an override, update delta only
		if stepBodyOverride != nil {
			overrideData, err := json.Marshal(stepBodyOverride)
			if err != nil {
				return fmt.Errorf("failed to marshal body: %w", err)
			}
			// Update only the delta body
			for i := range data.RawBodies {
				if data.RawBodies[i].ExampleID == ctx.deltaExampleID {
					data.RawBodies[i].Data = overrideData
					break
				}
			}
		}
	} else if stepBodyOverride != nil {
		// No template, use step body for all
		bodyData, _ = json.Marshal(stepBodyOverride)
		addBodyToExamples(ctx, bodyData, data)
	} else {
		// No body at all
		addBodyToExamples(ctx, nil, data)
	}

	return nil
}

// processIfStepForNode processes an if step for a given node
func processIfStepForNode(nodeName string, nodeID, flowID idwrap.IDWrap, stepData map[string]any, data *WorkflowData) error {
	addNodeWithName(nodeName, nodeID, flowID, mnnode.NODE_KIND_CONDITION, data)

	condition, ok := stepData[fieldCondition].(string)
	if !ok || condition == "" {
		return newWorkflowError(fmt.Sprintf("if step '%s' missing required condition", nodeName), fieldCondition, nil)
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
func processForStepForNode(nodeName string, nodeID, flowID idwrap.IDWrap, stepData map[string]any, data *WorkflowData) error {
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
func processForEachStepForNode(nodeName string, nodeID, flowID idwrap.IDWrap, stepData map[string]any, data *WorkflowData) error {
	addNodeWithName(nodeName, nodeID, flowID, mnnode.NODE_KIND_FOR_EACH, data)

	items, ok := stepData[fieldItems].(string)
	if !ok || items == "" {
		return newWorkflowError(fmt.Sprintf("for_each step '%s' missing required items", nodeName), fieldItems, nil)
	}

	data.ForEachNodes = append(data.ForEachNodes, mnforeach.MNForEach{
		FlowNodeID:     nodeID,
		IterExpression: items,
	})
	return nil
}

// processJSStepForNode processes a JavaScript step for a given node
func processJSStepForNode(nodeName string, nodeID, flowID idwrap.IDWrap, stepData map[string]any, data *WorkflowData) error {
	addNodeWithName(nodeName, nodeID, flowID, mnnode.NODE_KIND_JS, data)

	code, ok := stepData[fieldCode].(string)
	if !ok || code == "" {
		return newWorkflowError(fmt.Sprintf("js step '%s' missing required code", nodeName), fieldCode, nil)
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
func createRequestEntitiesForNode(nodeName string, nodeID, flowID idwrap.IDWrap, url, method string, data *WorkflowData) *requestContext {
	ctx := &requestContext{
		nodeID:           nodeID,
		endpointID:       idwrap.NewNow(),
		deltaEndpointID:  idwrap.NewNow(),
		exampleID:        idwrap.NewNow(),
		defaultExampleID: idwrap.NewNow(),
		deltaExampleID:   idwrap.NewNow(),
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
			BodyType:  mitemapiexample.BodyTypeRaw,
		},
		mitemapiexample.ItemApiExample{
			ID:        ctx.defaultExampleID,
			Name:      fmt.Sprintf("%s (default)", nodeName),
			ItemApiID: ctx.endpointID,
			IsDefault: false,
			BodyType:  mitemapiexample.BodyTypeRaw,
		},
		mitemapiexample.ItemApiExample{
			ID:              ctx.deltaExampleID,
			Name:            fmt.Sprintf("%s (delta)", nodeName),
			ItemApiID:       ctx.deltaEndpointID,
			IsDefault:       true,
			VersionParentID: &ctx.defaultExampleID,
			BodyType:        mitemapiexample.BodyTypeRaw,
		},
	)

	// Add request node
	data.RequestNodes = append(data.RequestNodes, mnrequest.MNRequest{
		FlowNodeID:      nodeID,
		EndpointID:      &ctx.endpointID,
		ExampleID:       &ctx.exampleID,
		DeltaEndpointID: &ctx.deltaEndpointID,
		DeltaExampleID:  &ctx.deltaExampleID,
	})

	return ctx
}

// addBodyToExamples adds body data for all three examples
func addBodyToExamples(ctx *requestContext, bodyData []byte, data *WorkflowData) {
	if bodyData == nil {
		bodyData = []byte("{}")
	}

	visualMode := mbodyraw.VisualizeModeJSON
	if !json.Valid(bodyData) {
		visualMode = mbodyraw.VisualizeModeText
	}

	for _, exampleID := range []idwrap.IDWrap{ctx.exampleID, ctx.defaultExampleID, ctx.deltaExampleID} {
		data.RawBodies = append(data.RawBodies, mbodyraw.ExampleBodyRaw{
			ID:            idwrap.NewNow(),
			ExampleID:     exampleID,
			Data:          bodyData,
			CompressType:  compress.CompressTypeNone,
			VisualizeMode: visualMode,
		})
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
	data *WorkflowData,
) {
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

			// Default item with resolved value
			resolvedValue, _ := varMap.ReplaceVars(templateValue)
			defaultItem := createFunc(name, resolvedValue, idwrap.NewNow(), defaultExampleID, nil)
			appendFunc(defaultItem)

			// Check if overridden
			if overrideValue, isOverridden := overrideMap[name]; isOverridden {
				// Need to get ID from the actual type
				var defaultID idwrap.IDWrap
				switch v := defaultItem.(type) {
				case mexampleheader.Header:
					defaultID = v.ID
				case mexamplequery.Query:
					defaultID = v.ID
				}
				deltaItem := createFunc(name, overrideValue, idwrap.NewNow(), deltaExampleID, &defaultID)
				appendFunc(deltaItem)
			}
		}

		// Process override-only items
		for name, overrideValue := range overrideMap {
			if !processedNames[name] {
				// Base item
				baseItem := createFunc(name, overrideValue, idwrap.NewNow(), exampleID, nil)
				appendFunc(baseItem)

				// Default item
				resolvedValue, _ := varMap.ReplaceVars(overrideValue)
				defaultItem := createFunc(name, resolvedValue, idwrap.NewNow(), defaultExampleID, nil)
				appendFunc(defaultItem)

				// Delta item
				var defaultID idwrap.IDWrap
				switch v := defaultItem.(type) {
				case mexampleheader.Header:
					defaultID = v.ID
				case mexamplequery.Query:
					defaultID = v.ID
				}
				deltaItem := createFunc(name, overrideValue, idwrap.NewNow(), deltaExampleID, &defaultID)
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

			// Default item
			resolvedValue, _ := varMap.ReplaceVars(value)
			defaultItem := createFunc(name, resolvedValue, idwrap.NewNow(), defaultExampleID, nil)
			appendFunc(defaultItem)

			// Delta item if has variables
			if varsystem.CheckStringHasAnyVarKey(value) {
				var defaultID idwrap.IDWrap
				switch v := defaultItem.(type) {
				case mexampleheader.Header:
					defaultID = v.ID
				case mexamplequery.Query:
					defaultID = v.ID
				}
				deltaItem := createFunc(name, value, idwrap.NewNow(), deltaExampleID, &defaultID)
				appendFunc(deltaItem)
			}
		}
	}
}

// ========================================
// Edge Creation Functions
// ========================================

// createEdgesForFlow creates edges based on dependencies and sequential order
func createEdgesForFlow(flowID, startNodeID idwrap.IDWrap, nodeInfoMap map[string]*nodeInfo, nodeList []*nodeInfo, rawSteps []map[string]any, data *WorkflowData) {
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
func processRunDependencies(runEntries []RunEntry, currentFlowName string, nodeInfoMap map[string]*nodeInfo, data *WorkflowData) {
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
