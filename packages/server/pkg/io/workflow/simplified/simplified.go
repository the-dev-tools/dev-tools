package simplified

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/io/workflow"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/model/massertres"
	"the-dev-tools/server/pkg/model/mbodyform"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mbodyurl"
	"the-dev-tools/server/pkg/model/mcollection"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mexamplequery"
	"the-dev-tools/server/pkg/model/mexampleresp"
	"the-dev-tools/server/pkg/model/mexamplerespheader"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mflowvariable"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mitemfolder"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"the-dev-tools/server/pkg/model/mnnode/mnforeach"
	"the-dev-tools/server/pkg/model/mnnode/mnif"
	"the-dev-tools/server/pkg/model/mnnode/mnjs"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/model/mworkspace"

	"gopkg.in/yaml.v3"
)

// Simplified implements the workflow.Workflow interface
type Simplified struct{}

// New creates a new simplified workflow formatter
func New() *Simplified {
	return &Simplified{}
}

// Marshal implements workflow.Workflow
func (s *Simplified) Marshal(data *workflow.WorkspaceData, format workflow.Format) ([]byte, error) {
	// Convert WorkspaceData to simplified format
	simplified, err := s.toSimplified(data)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to simplified format: %w", err)
	}

	switch format {
	case workflow.FormatYAML:
		return yaml.Marshal(simplified)
	case workflow.FormatJSON:
		return json.Marshal(simplified)
	case workflow.FormatTOML:
		return nil, fmt.Errorf("TOML format not yet implemented")
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

// Unmarshal implements workflow.Workflow
func (s *Simplified) Unmarshal(data []byte, format workflow.Format) (*workflow.WorkspaceData, error) {
	var simplified SimplifiedWorkflow

	switch format {
	case workflow.FormatYAML:
		if err := yaml.Unmarshal(data, &simplified); err != nil {
			return nil, fmt.Errorf("failed to unmarshal YAML: %w", err)
		}
	case workflow.FormatJSON:
		if err := json.Unmarshal(data, &simplified); err != nil {
			return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
		}
	case workflow.FormatTOML:
		return nil, fmt.Errorf("TOML format not yet implemented")
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}

	return s.fromSimplified(&simplified)
}

// toSimplified converts internal WorkspaceData to simplified format
func (s *Simplified) toSimplified(data *workflow.WorkspaceData) (*SimplifiedWorkflow, error) {
	simplified := &SimplifiedWorkflow{
		WorkspaceName: data.Workspace.Name,
		Flows:         make([]Flow, 0, len(data.Flows)),
		Requests:      make([]GlobalRequest, 0),
		Run:           make([]RunStep, 0),
	}

	// TODO: Extract global requests from the data
	// This will be implemented when we update the ioworkspace logic

	// Convert flows
	for _, flow := range data.Flows {
		sFlow := Flow{
			Name:      flow.Name,
			Variables: make(map[string]string),
			Steps:     make([]Step, 0),
		}

		// Get flow variables
		for _, v := range data.FlowVariables {
			if v.FlowID == flow.ID {
				sFlow.Variables[v.Name] = v.Value
			}
		}

		// Get flow nodes and convert to steps
		nodeMap := make(map[idwrap.IDWrap]*mnnode.MNode)
		for i := range data.FlowNodes {
			node := &data.FlowNodes[i]
			if node.FlowID == flow.ID {
				nodeMap[node.ID] = node
			}
		}

		// Convert nodes to steps
		for _, node := range nodeMap {
			step, err := s.nodeToStep(node, data, flow.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to convert node %s to step: %w", node.Name, err)
			}
			if step != nil {
				sFlow.Steps = append(sFlow.Steps, *step)
			}
		}

		simplified.Flows = append(simplified.Flows, sFlow)
	}

	// Build run order from flows
	for _, flow := range data.Flows {
		simplified.Run = append(simplified.Run, RunStep{
			Flow:      flow.Name,
			DependsOn: []string{}, // TODO: Extract dependencies
		})
	}

	return simplified, nil
}

// nodeToStep converts a flow node to a simplified step
func (s *Simplified) nodeToStep(node *mnnode.MNode, data *workflow.WorkspaceData, flowID idwrap.IDWrap) (*Step, error) {
	step := &Step{
		DependsOn: s.getNodeDependencies(node.ID, data.FlowEdges),
	}

	switch node.NodeKind {
	case mnnode.NODE_KIND_REQUEST:
		// Find the request node
		for _, reqNode := range data.FlowRequestNodes {
			if reqNode.FlowNodeID == node.ID {
				reqStep, err := s.convertRequestNode(node, &reqNode, data)
				if err != nil {
					return nil, err
				}
				step.Type = StepTypeRequest
				step.Request = reqStep
				return step, nil
			}
		}

	case mnnode.NODE_KIND_CONDITION:
		// Find the condition node
		for _, condNode := range data.FlowConditionNodes {
			if condNode.FlowNodeID == node.ID {
				step.Type = StepTypeIf
				step.If = &IfStep{
					Name:       node.Name,
					Expression: condNode.Condition.Comparisons.Expression,
					Then:       s.getTargetNodeName(node.ID, edge.HandleThen, data),
					Else:       s.getTargetNodeName(node.ID, edge.HandleElse, data),
				}
				return step, nil
			}
		}

	case mnnode.NODE_KIND_FOR:
		// Find the for node
		for _, forNode := range data.FlowForNodes {
			if forNode.FlowNodeID == node.ID {
				step.Type = StepTypeFor
				step.For = &ForStep{
					Name:      node.Name,
					IterCount: forNode.IterCount,
					Loop:      s.getTargetNodeName(node.ID, edge.HandleLoop, data),
				}
				return step, nil
			}
		}

	case mnnode.NODE_KIND_FOR_EACH:
		// Find the for-each node
		for _, forEachNode := range data.FlowForEachNodes {
			if forEachNode.FlowNodeID == node.ID {
				step.Type = StepTypeForEach
				step.ForEach = &ForEachStep{
					Name:       node.Name,
					Collection: forEachNode.IterExpression,
					Item:       node.Name + "_item", // TODO: Extract from node data
					Loop:       s.getTargetNodeName(node.ID, edge.HandleLoop, data),
				}
				return step, nil
			}
		}

	case mnnode.NODE_KIND_JS:
		// Find the JS node
		for _, jsNode := range data.FlowJSNodes {
			if jsNode.FlowNodeID == node.ID {
				step.Type = StepTypeJS
				step.JS = &JSStep{
					Name: node.Name,
					Code: string(jsNode.Code),
				}
				return step, nil
			}
		}

	case mnnode.NODE_KIND_NO_OP:
		// Skip no-op nodes
		return nil, nil
	}

	return nil, fmt.Errorf("unknown node kind: %d", node.NodeKind)
}

// convertRequestNode converts a request node to a request step
func (s *Simplified) convertRequestNode(node *mnnode.MNode, reqNode *mnrequest.MNRequest, data *workflow.WorkspaceData) (*RequestStep, error) {
	reqStep := &RequestStep{
		Name:    node.Name,
		Headers: make(map[string]string),
	}

	// Get endpoint data
	if reqNode.EndpointID != nil {
		for _, endpoint := range data.Endpoints {
			if endpoint.ID == *reqNode.EndpointID {
				reqStep.Method = endpoint.Method
				reqStep.URL = endpoint.Url
				break
			}
		}
	}

	// Get headers
	if reqNode.ExampleID != nil {
		for _, header := range data.RequestHeaders {
			if header.ExampleID == *reqNode.ExampleID && header.Enable {
				reqStep.Headers[header.HeaderKey] = header.Value
			}
		}
	}

	// Get body
	if reqNode.ExampleID != nil {
		for _, body := range data.RequestBodyRaw {
			if body.ExampleID == *reqNode.ExampleID {
				// TODO: Parse body content
				reqStep.Body = &BodyFormat{
					Kind: BodyKindJSON,
					Value: map[string]interface{}{
						"raw": string(body.Data),
					},
				}
				break
			}
		}
	}

	return reqStep, nil
}

// getNodeDependencies gets the dependencies for a node based on incoming edges
func (s *Simplified) getNodeDependencies(nodeID idwrap.IDWrap, edges []edge.Edge) []string {
	deps := make([]string, 0)
	// TODO: Implement dependency extraction from edges
	return deps
}

// getTargetNodeName gets the name of the target node for a specific edge handle
func (s *Simplified) getTargetNodeName(sourceID idwrap.IDWrap, handle edge.EdgeHandle, data *workflow.WorkspaceData) string {
	for _, e := range data.FlowEdges {
		if e.SourceID == sourceID && e.SourceHandler == handle {
			// Find the target node
			for _, node := range data.FlowNodes {
				if node.ID == e.TargetID {
					return node.Name
				}
			}
		}
	}
	return ""
}

// fromSimplified converts simplified format to internal WorkspaceData
func (s *Simplified) fromSimplified(simplified *SimplifiedWorkflow) (*workflow.WorkspaceData, error) {
	// Validate required fields
	if simplified.WorkspaceName == "" {
		return nil, fmt.Errorf("workspace_name is required")
	}

	// Validate flows
	if len(simplified.Flows) == 0 {
		return nil, fmt.Errorf("at least one flow is required")
	}

	for _, flow := range simplified.Flows {
		if flow.Name == "" {
			return nil, fmt.Errorf("flow name is required")
		}
		if len(flow.Steps) == 0 {
			return nil, fmt.Errorf("flow '%s' must have at least one step", flow.Name)
		}

		// Validate steps
		for i, step := range flow.Steps {
			if err := s.validateStep(step, i); err != nil {
				return nil, fmt.Errorf("flow '%s': %w", flow.Name, err)
			}
		}
	}

	// Create workspace data structure with proper IDs
	workspaceID := idwrap.NewNow()
	collectionID := idwrap.NewNow()

	workspaceData := &workflow.WorkspaceData{
		Workspace: mworkspace.Workspace{
			ID:   workspaceID,
			Name: simplified.WorkspaceName,
		},
		Collection: mcollection.Collection{
			ID:          collectionID,
			Name:        "Workflow Collection",
			WorkspaceID: workspaceID,
		},
		Flows:                  make([]mflow.Flow, 0),
		FlowNodes:              make([]mnnode.MNode, 0),
		FlowEdges:              make([]edge.Edge, 0),
		FlowVariables:          make([]mflowvariable.FlowVariable, 0),
		Endpoints:              make([]mitemapi.ItemApi, 0),
		Examples:               make([]mitemapiexample.ItemApiExample, 0),
		RequestHeaders:         make([]mexampleheader.Header, 0),
		RequestBodyRaw:         make([]mbodyraw.ExampleBodyRaw, 0),
		FlowRequestNodes:       make([]mnrequest.MNRequest, 0),
		FlowConditionNodes:     make([]mnif.MNIF, 0),
		FlowNoopNodes:          make([]mnnoop.NoopNode, 0),
		FlowForNodes:           make([]mnfor.MNFor, 0),
		FlowForEachNodes:       make([]mnforeach.MNForEach, 0),
		FlowJSNodes:            make([]mnjs.MNJS, 0),
		EndpointExampleMap:     make(map[idwrap.IDWrap][]idwrap.IDWrap),
		Folders:                make([]mitemfolder.ItemFolder, 0),
		RequestQueries:         make([]mexamplequery.Query, 0),
		RequestBodyForm:        make([]mbodyform.BodyForm, 0),
		RequestBodyUrlencoded:  make([]mbodyurl.BodyURLEncoded, 0),
		RequestAsserts:         make([]massert.Assert, 0),
		Responses:              make([]mexampleresp.ExampleResp, 0),
		ResponseHeaders:        make([]mexamplerespheader.ExampleRespHeader, 0),
		ResponseAsserts:        make([]massertres.AssertResult, 0),
		ResponseBodyRaw:        make([]mbodyraw.ExampleBodyRaw, 0),
		ResponseBodyForm:       make([]mbodyform.BodyForm, 0),
		ResponseBodyUrlencoded: make([]mbodyurl.BodyURLEncoded, 0),
	}

	// Create a map of global request definitions for quick lookup
	globalRequests := make(map[string]*GlobalRequest)
	for i := range simplified.Requests {
		req := &simplified.Requests[i]
		globalRequests[req.Name] = req
	}

	// Process each flow
	for _, sFlow := range simplified.Flows {
		// Create flow
		flowID := idwrap.NewNow()
		flow := mflow.Flow{
			ID:          flowID,
			Name:        sFlow.Name,
			WorkspaceID: workspaceID,
		}
		workspaceData.Flows = append(workspaceData.Flows, flow)

		// Create flow variables
		for name, value := range sFlow.Variables {
			varID := idwrap.NewNow()
			flowVar := mflowvariable.FlowVariable{
				ID:     varID,
				FlowID: flowID,
				Name:   name,
				Value:  value,
			}
			workspaceData.FlowVariables = append(workspaceData.FlowVariables, flowVar)
		}

		// Map to track node names to IDs for edge creation
		nodeNameToID := make(map[string]idwrap.IDWrap)

		// Create start node
		startNodeID := idwrap.NewNow()
		startNode := mnnode.MNode{
			ID:       startNodeID,
			FlowID:   flowID,
			Name:     "Start",
			NodeKind: mnnode.NODE_KIND_NO_OP,
		}
		workspaceData.FlowNodes = append(workspaceData.FlowNodes, startNode)
		
		// Create no-op start node
		noopNode := mnnoop.NoopNode{
			FlowNodeID: startNodeID,
			Type:       mnnoop.NODE_NO_OP_KIND_START,
		}
		workspaceData.FlowNoopNodes = append(workspaceData.FlowNoopNodes, noopNode)

		// Process steps
		for _, step := range sFlow.Steps {
			nodeID := idwrap.NewNow()

			// Create the appropriate node based on step type
			var nodeName string
			var nodeKind mnnode.NodeKind

			switch step.Type {
			case StepTypeRequest:
				if step.Request != nil {
					nodeName = step.Request.Name
					nodeKind = mnnode.NODE_KIND_REQUEST
					err := s.createRequestNode(nodeID, flowID, step.Request, globalRequests, workspaceData)
					if err != nil {
						return nil, fmt.Errorf("failed to create request node: %w", err)
					}
				}

			case StepTypeIf:
				if step.If != nil {
					nodeName = step.If.Name
					nodeKind = mnnode.NODE_KIND_CONDITION
					s.createIfNode(nodeID, flowID, step.If, workspaceData)
				}

			case StepTypeFor:
				if step.For != nil {
					nodeName = step.For.Name
					nodeKind = mnnode.NODE_KIND_FOR
					s.createForNode(nodeID, flowID, step.For, workspaceData)
				}

			case StepTypeForEach:
				if step.ForEach != nil {
					nodeName = step.ForEach.Name
					nodeKind = mnnode.NODE_KIND_FOR_EACH
					s.createForEachNode(nodeID, flowID, step.ForEach, workspaceData)
				}

			case StepTypeJS:
				if step.JS != nil {
					nodeName = step.JS.Name
					nodeKind = mnnode.NODE_KIND_JS
					s.createJSNode(nodeID, flowID, step.JS, workspaceData)
				}
			}

			// Create the base node
			if nodeName != "" {
				node := mnnode.MNode{
					ID:       nodeID,
					FlowID:   flowID,
					Name:     nodeName,
					NodeKind: nodeKind,
				}
				workspaceData.FlowNodes = append(workspaceData.FlowNodes, node)
				nodeNameToID[nodeName] = nodeID
			}
		}

		// Create edges based on dependencies and step connections
		s.createFlowEdges(sFlow, flowID, nodeNameToID, workspaceData)
		
		// Create edge from start node to first step
		if len(sFlow.Steps) > 0 {
			var firstNodeName string
			firstStep := sFlow.Steps[0]
			switch firstStep.Type {
			case StepTypeRequest:
				if firstStep.Request != nil {
					firstNodeName = firstStep.Request.Name
				}
			case StepTypeIf:
				if firstStep.If != nil {
					firstNodeName = firstStep.If.Name
				}
			case StepTypeFor:
				if firstStep.For != nil {
					firstNodeName = firstStep.For.Name
				}
			case StepTypeForEach:
				if firstStep.ForEach != nil {
					firstNodeName = firstStep.ForEach.Name
				}
			case StepTypeJS:
				if firstStep.JS != nil {
					firstNodeName = firstStep.JS.Name
				}
			}
			
			if firstNodeName != "" {
				if firstNodeID, ok := nodeNameToID[firstNodeName]; ok {
					edgeID := idwrap.NewNow()
					e := edge.Edge{
						ID:            edgeID,
						FlowID:        flowID,
						SourceID:      startNodeID,
						TargetID:      firstNodeID,
						SourceHandler: edge.HandleThen,
					}
					workspaceData.FlowEdges = append(workspaceData.FlowEdges, e)
					// Debug log
					// fmt.Printf("Created edge from Start to %s\n", firstNodeName)
				}
			}
		}
	}

	// TODO: Process run dependencies

	return workspaceData, nil
}

// createRequestNode creates a request node and related entities
func (s *Simplified) createRequestNode(nodeID, flowID idwrap.IDWrap, req *RequestStep, globalRequests map[string]*GlobalRequest, data *workflow.WorkspaceData) error {
	// Validate required fields
	if req.Name == "" {
		return fmt.Errorf("request name is required")
	}

	// Merge with global request if specified
	method := req.Method
	urlStr := req.URL
	headers := req.Headers
	body := req.Body

	if req.UseRequest != "" {
		if globalReq, ok := globalRequests[req.UseRequest]; ok {
			if method == "" {
				method = globalReq.Method
			}
			if urlStr == "" {
				urlStr = globalReq.URL
			}
			// Merge headers
			if headers == nil {
				headers = globalReq.Headers
			} else {
				// Copy global headers first
				merged := make(map[string]string)
				for k, v := range globalReq.Headers {
					merged[k] = v
				}
				// Override with step-specific headers
				for k, v := range headers {
					merged[k] = v
				}
				headers = merged
			}
			if body == nil {
				body = globalReq.Body
			}
		}
	}

	// Validate required fields after merging
	if method == "" {
		return fmt.Errorf("request method is required for '%s'", req.Name)
	}
	if urlStr == "" {
		return fmt.Errorf("request URL is required for '%s'", req.Name)
	}

	// Parse URL to extract query parameters
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		// If parsing fails, use the URL as-is
		parsedURL = &url.URL{
			Path: urlStr,
		}
	}

	// Extract base URL without query parameters
	baseURL := urlStr
	if parsedURL.RawQuery != "" {
		baseURL = strings.Split(urlStr, "?")[0]
	}

	// Create endpoint
	endpointID := idwrap.NewNow()
	endpoint := mitemapi.ItemApi{
		ID:           endpointID,
		Name:         req.Name,
		Method:       method,
		Url:          baseURL, // Store URL without query parameters
		CollectionID: data.Collection.ID,
	}
	data.Endpoints = append(data.Endpoints, endpoint)

	// Create example
	exampleID := idwrap.NewNow()
	example := mitemapiexample.ItemApiExample{
		ID:           exampleID,
		Name:         "Default",
		ItemApiID:    endpointID,
		CollectionID: data.Collection.ID,
	}
	data.Examples = append(data.Examples, example)

	// Create headers
	for key, value := range headers {
		headerID := idwrap.NewNow()
		header := mexampleheader.Header{
			ID:        headerID,
			ExampleID: exampleID,
			HeaderKey: key,
			Value:     value,
			Enable:    true,
		}
		data.RequestHeaders = append(data.RequestHeaders, header)
	}

	// Create query parameters
	if parsedURL.RawQuery != "" {
		queryParams := parsedURL.Query()
		for key, values := range queryParams {
			// Join multiple values with comma
			value := strings.Join(values, ",")

			queryID := idwrap.NewNow()
			query := mexamplequery.Query{
				ID:        queryID,
				ExampleID: exampleID,
				QueryKey:  key,
				Value:     value,
				Enable:    true,
			}
			data.RequestQueries = append(data.RequestQueries, query)
		}
	}

	// Create body based on type
	if body != nil && body.Value != nil {
		switch body.Kind {
		case BodyKindForm:
			// Handle form data
			for key, value := range body.Value {
				formID := idwrap.NewNow()
				formEntry := mbodyform.BodyForm{
					ID:        formID,
					ExampleID: exampleID,
					BodyKey:   key,
					Value:     fmt.Sprintf("%v", value),
					Enable:    true,
				}
				data.RequestBodyForm = append(data.RequestBodyForm, formEntry)
			}

		case BodyKindURLEncoded:
			// Handle URL-encoded data
			for key, value := range body.Value {
				urlID := idwrap.NewNow()
				urlEntry := mbodyurl.BodyURLEncoded{
					ID:        urlID,
					ExampleID: exampleID,
					BodyKey:   key,
					Value:     fmt.Sprintf("%v", value),
					Enable:    true,
				}
				data.RequestBodyUrlencoded = append(data.RequestBodyUrlencoded, urlEntry)
			}

		case BodyKindRaw:
			// Handle raw text
			rawText := ""
			// Check if the value is a string in the map
			if val, ok := body.Value["value"]; ok {
				rawText = fmt.Sprintf("%v", val)
			} else {
				// Otherwise convert the whole map to JSON
				data, _ := json.Marshal(body.Value)
				rawText = string(data)
			}

			bodyID := idwrap.NewNow()
			rawBody := mbodyraw.ExampleBodyRaw{
				ID:        bodyID,
				ExampleID: exampleID,
				Data:      []byte(rawText),
			}
			data.RequestBodyRaw = append(data.RequestBodyRaw, rawBody)

		default: // BodyKindJSON or unspecified
			// Handle JSON data
			bodyData, err := json.Marshal(body.Value)
			if err != nil {
				return fmt.Errorf("failed to marshal JSON body: %w", err)
			}

			bodyID := idwrap.NewNow()
			rawBody := mbodyraw.ExampleBodyRaw{
				ID:        bodyID,
				ExampleID: exampleID,
				Data:      bodyData,
			}
			data.RequestBodyRaw = append(data.RequestBodyRaw, rawBody)
		}
	} else {
		// Create empty raw body for requests without a body
		bodyID := idwrap.NewNow()
		rawBody := mbodyraw.ExampleBodyRaw{
			ID:        bodyID,
			ExampleID: exampleID,
			Data:      []byte{},
		}
		data.RequestBodyRaw = append(data.RequestBodyRaw, rawBody)
	}

	// Create request node
	reqNode := mnrequest.MNRequest{
		FlowNodeID: nodeID,
		EndpointID: &endpointID,
		ExampleID:  &exampleID,
	}
	data.FlowRequestNodes = append(data.FlowRequestNodes, reqNode)

	// Update endpoint-example map
	if data.EndpointExampleMap == nil {
		data.EndpointExampleMap = make(map[idwrap.IDWrap][]idwrap.IDWrap)
	}
	data.EndpointExampleMap[endpointID] = append(data.EndpointExampleMap[endpointID], exampleID)

	return nil
}

// createIfNode creates a condition node
func (s *Simplified) createIfNode(nodeID, flowID idwrap.IDWrap, ifStep *IfStep, data *workflow.WorkspaceData) {
	condNode := mnif.MNIF{
		FlowNodeID: nodeID,
		Condition: mcondition.Condition{
			Comparisons: mcondition.Comparison{
				Expression: ifStep.Expression,
			},
		},
	}
	data.FlowConditionNodes = append(data.FlowConditionNodes, condNode)
}

// createForNode creates a for loop node
func (s *Simplified) createForNode(nodeID, flowID idwrap.IDWrap, forStep *ForStep, data *workflow.WorkspaceData) {
	forNode := mnfor.MNFor{
		FlowNodeID: nodeID,
		IterCount:  forStep.IterCount,
	}
	data.FlowForNodes = append(data.FlowForNodes, forNode)
}

// createForEachNode creates a for-each loop node
func (s *Simplified) createForEachNode(nodeID, flowID idwrap.IDWrap, forEachStep *ForEachStep, data *workflow.WorkspaceData) {
	forEachNode := mnforeach.MNForEach{
		FlowNodeID:     nodeID,
		IterExpression: forEachStep.Collection,
	}
	data.FlowForEachNodes = append(data.FlowForEachNodes, forEachNode)
}

// createJSNode creates a JavaScript node
func (s *Simplified) createJSNode(nodeID, flowID idwrap.IDWrap, jsStep *JSStep, data *workflow.WorkspaceData) {
	jsNode := mnjs.MNJS{
		FlowNodeID: nodeID,
		Code:       []byte(jsStep.Code),
	}
	data.FlowJSNodes = append(data.FlowJSNodes, jsNode)
}

// createFlowEdges creates edges between nodes in a flow
func (s *Simplified) createFlowEdges(flow Flow, flowID idwrap.IDWrap, nodeNameToID map[string]idwrap.IDWrap, data *workflow.WorkspaceData) {
	// First, collect all nodes that are targets of control flow edges
	controlFlowTargets := make(map[string]bool)
	for _, step := range flow.Steps {
		switch step.Type {
		case StepTypeIf:
			if step.If != nil {
				if step.If.Then != "" {
					controlFlowTargets[step.If.Then] = true
				}
				if step.If.Else != "" {
					controlFlowTargets[step.If.Else] = true
				}
			}
		case StepTypeFor:
			if step.For != nil && step.For.Loop != "" {
				controlFlowTargets[step.For.Loop] = true
			}
		case StepTypeForEach:
			if step.ForEach != nil && step.ForEach.Loop != "" {
				controlFlowTargets[step.ForEach.Loop] = true
			}
		}
	}

	// Create sequential edges between steps
	for i := 0; i < len(flow.Steps)-1; i++ {
		currentStep := flow.Steps[i]
		nextStep := flow.Steps[i+1]

		// Skip sequential edge creation for control flow nodes that define their own edges
		shouldSkipSequential := false
		switch currentStep.Type {
		case StepTypeIf:
			// If nodes create their own then/else edges
			shouldSkipSequential = true
		case StepTypeFor, StepTypeForEach:
			// Loop nodes create their own loop edges
			shouldSkipSequential = true
		}

		// Also skip if the next node is a target of a control flow edge
		var nextNodeName string
		switch nextStep.Type {
		case StepTypeRequest:
			if nextStep.Request != nil {
				nextNodeName = nextStep.Request.Name
			}
		case StepTypeIf:
			if nextStep.If != nil {
				nextNodeName = nextStep.If.Name
			}
		case StepTypeFor:
			if nextStep.For != nil {
				nextNodeName = nextStep.For.Name
			}
		case StepTypeForEach:
			if nextStep.ForEach != nil {
				nextNodeName = nextStep.ForEach.Name
			}
		case StepTypeJS:
			if nextStep.JS != nil {
				nextNodeName = nextStep.JS.Name
			}
		}

		if controlFlowTargets[nextNodeName] {
			shouldSkipSequential = true
		}

		if shouldSkipSequential {
			continue
		}

		var currentNodeName string

		// Get current node name
		switch currentStep.Type {
		case StepTypeRequest:
			if currentStep.Request != nil {
				currentNodeName = currentStep.Request.Name
			}
		case StepTypeIf:
			if currentStep.If != nil {
				currentNodeName = currentStep.If.Name
			}
		case StepTypeFor:
			if currentStep.For != nil {
				currentNodeName = currentStep.For.Name
			}
		case StepTypeForEach:
			if currentStep.ForEach != nil {
				currentNodeName = currentStep.ForEach.Name
			}
		case StepTypeJS:
			if currentStep.JS != nil {
				currentNodeName = currentStep.JS.Name
			}
		}

		// Get next node name
		switch nextStep.Type {
		case StepTypeRequest:
			if nextStep.Request != nil {
				nextNodeName = nextStep.Request.Name
			}
		case StepTypeIf:
			if nextStep.If != nil {
				nextNodeName = nextStep.If.Name
			}
		case StepTypeFor:
			if nextStep.For != nil {
				nextNodeName = nextStep.For.Name
			}
		case StepTypeForEach:
			if nextStep.ForEach != nil {
				nextNodeName = nextStep.ForEach.Name
			}
		case StepTypeJS:
			if nextStep.JS != nil {
				nextNodeName = nextStep.JS.Name
			}
		}

		// Create edge
		if currentNodeName != "" && nextNodeName != "" {
			if currentID, ok := nodeNameToID[currentNodeName]; ok {
				if nextID, ok := nodeNameToID[nextNodeName]; ok {
					edgeID := idwrap.NewNow()
					e := edge.Edge{
						ID:            edgeID,
						FlowID:        flowID,
						SourceID:      currentID,
						TargetID:      nextID,
						SourceHandler: edge.HandleThen,
					}
					data.FlowEdges = append(data.FlowEdges, e)
				}
			}
		}
	}

	// Create edges for control flow nodes
	for _, step := range flow.Steps {
		switch step.Type {
		case StepTypeIf:
			if step.If != nil {
				s.createIfEdges(step.If, flowID, nodeNameToID, data)
			}
		case StepTypeFor:
			if step.For != nil {
				s.createForEdges(step.For, flowID, nodeNameToID, data)
			}
		case StepTypeForEach:
			if step.ForEach != nil {
				s.createForEachEdges(step.ForEach, flowID, nodeNameToID, data)
			}
		}
	}

	// Create dependency edges
	for _, step := range flow.Steps {
		if len(step.DependsOn) > 0 {
			// Get the target node name
			var targetNodeName string
			switch step.Type {
			case StepTypeRequest:
				if step.Request != nil {
					targetNodeName = step.Request.Name
				}
			case StepTypeIf:
				if step.If != nil {
					targetNodeName = step.If.Name
				}
			case StepTypeFor:
				if step.For != nil {
					targetNodeName = step.For.Name
				}
			case StepTypeForEach:
				if step.ForEach != nil {
					targetNodeName = step.ForEach.Name
				}
			case StepTypeJS:
				if step.JS != nil {
					targetNodeName = step.JS.Name
				}
			}

			if targetNodeName != "" {
				if targetID, ok := nodeNameToID[targetNodeName]; ok {
					// Create edge from each dependency to this node
					for _, depName := range step.DependsOn {
						if sourceID, ok := nodeNameToID[depName]; ok {
							edgeID := idwrap.NewNow()
							e := edge.Edge{
								ID:            edgeID,
								FlowID:        flowID,
								SourceID:      sourceID,
								TargetID:      targetID,
								SourceHandler: edge.HandleThen,
							}
							data.FlowEdges = append(data.FlowEdges, e)
						}
					}
				}
			}
		}
	}
}

// createIfEdges creates edges for if nodes
func (s *Simplified) createIfEdges(ifStep *IfStep, flowID idwrap.IDWrap, nodeNameToID map[string]idwrap.IDWrap, data *workflow.WorkspaceData) {
	if sourceID, ok := nodeNameToID[ifStep.Name]; ok {
		// Create then edge
		if ifStep.Then != "" {
			if targetID, ok := nodeNameToID[ifStep.Then]; ok {
				edgeID := idwrap.NewNow()
				e := edge.Edge{
					ID:            edgeID,
					FlowID:        flowID,
					SourceID:      sourceID,
					TargetID:      targetID,
					SourceHandler: edge.HandleThen,
				}
				data.FlowEdges = append(data.FlowEdges, e)
			}
		}

		// Create else edge
		if ifStep.Else != "" {
			if targetID, ok := nodeNameToID[ifStep.Else]; ok {
				edgeID := idwrap.NewNow()
				e := edge.Edge{
					ID:            edgeID,
					FlowID:        flowID,
					SourceID:      sourceID,
					TargetID:      targetID,
					SourceHandler: edge.HandleElse,
				}
				data.FlowEdges = append(data.FlowEdges, e)
			}
		}
	}
}

// createForEdges creates edges for for nodes
func (s *Simplified) createForEdges(forStep *ForStep, flowID idwrap.IDWrap, nodeNameToID map[string]idwrap.IDWrap, data *workflow.WorkspaceData) {
	if sourceID, ok := nodeNameToID[forStep.Name]; ok {
		// Create loop edge
		if forStep.Loop != "" {
			if targetID, ok := nodeNameToID[forStep.Loop]; ok {
				edgeID := idwrap.NewNow()
				e := edge.Edge{
					ID:            edgeID,
					FlowID:        flowID,
					SourceID:      sourceID,
					TargetID:      targetID,
					SourceHandler: edge.HandleLoop,
				}
				data.FlowEdges = append(data.FlowEdges, e)
			}
		}
	}
}

// createForEachEdges creates edges for for-each nodes
func (s *Simplified) createForEachEdges(forEachStep *ForEachStep, flowID idwrap.IDWrap, nodeNameToID map[string]idwrap.IDWrap, data *workflow.WorkspaceData) {
	if sourceID, ok := nodeNameToID[forEachStep.Name]; ok {
		// Create loop edge
		if forEachStep.Loop != "" {
			if targetID, ok := nodeNameToID[forEachStep.Loop]; ok {
				edgeID := idwrap.NewNow()
				e := edge.Edge{
					ID:            edgeID,
					FlowID:        flowID,
					SourceID:      sourceID,
					TargetID:      targetID,
					SourceHandler: edge.HandleLoop,
				}
				data.FlowEdges = append(data.FlowEdges, e)
			}
		}
	}
}

// validateStep validates a workflow step
func (s *Simplified) validateStep(step Step, index int) error {
	// Count how many step types are defined
	definedTypes := 0
	if step.Request != nil {
		definedTypes++
	}
	if step.If != nil {
		definedTypes++
	}
	if step.For != nil {
		definedTypes++
	}
	if step.ForEach != nil {
		definedTypes++
	}
	if step.JS != nil {
		definedTypes++
	}

	if definedTypes == 0 {
		return fmt.Errorf("step %d: must define exactly one step type (request, if, for, for_each, or js)", index+1)
	}
	if definedTypes > 1 {
		return fmt.Errorf("step %d: multiple step types defined, only one allowed", index+1)
	}

	// Validate specific step types
	switch step.Type {
	case StepTypeRequest:
		if step.Request.Name == "" {
			return fmt.Errorf("step %d: request name is required", index+1)
		}
		// Method and URL validation happens in createRequestNode

	case StepTypeIf:
		if step.If.Name == "" {
			return fmt.Errorf("step %d: if name is required", index+1)
		}
		if step.If.Expression == "" {
			return fmt.Errorf("step %d: if expression is required", index+1)
		}

	case StepTypeFor:
		if step.For.Name == "" {
			return fmt.Errorf("step %d: for name is required", index+1)
		}
		if step.For.IterCount <= 0 {
			return fmt.Errorf("step %d: for iter_count must be positive", index+1)
		}

	case StepTypeForEach:
		if step.ForEach.Name == "" {
			return fmt.Errorf("step %d: for_each name is required", index+1)
		}
		if step.ForEach.Collection == "" {
			return fmt.Errorf("step %d: for_each collection is required", index+1)
		}

	case StepTypeJS:
		if step.JS.Name == "" {
			return fmt.Errorf("step %d: js name is required", index+1)
		}
		if step.JS.Code == "" {
			return fmt.Errorf("step %d: js code is required", index+1)
		}
	}

	return nil
}
