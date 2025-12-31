//nolint:revive // exported
package yamlflowsimplev2

import (
	"fmt"
	"sort"

	"the-dev-tools/server/pkg/flowgraph"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/ioworkspace"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mhttp"

	"gopkg.in/yaml.v3"
)

// MarshalSimplifiedYAML converts resolved data structures back to the simplified YAML format
func MarshalSimplifiedYAML(data *ioworkspace.WorkspaceBundle) ([]byte, error) {
	if data == nil {
		return nil, fmt.Errorf("input data is nil")
	}

	// Build maps for efficient lookup
	nodeMap := make(map[idwrap.IDWrap]mflow.Node)
	for _, n := range data.FlowNodes {
		nodeMap[n.ID] = n
	}

	// HTTP Maps
	httpMap := make(map[idwrap.IDWrap]mhttp.HTTP)
	for _, h := range data.HTTPRequests {
		httpMap[h.ID] = h
	}

	// HTTP Related Data Maps
	headersMap := make(map[idwrap.IDWrap][]mhttp.HTTPHeader)
	for _, h := range data.HTTPHeaders {
		headersMap[h.HttpID] = append(headersMap[h.HttpID], h)
	}

	paramsMap := make(map[idwrap.IDWrap][]mhttp.HTTPSearchParam)
	for _, p := range data.HTTPSearchParams {
		paramsMap[p.HttpID] = append(paramsMap[p.HttpID], p)
	}

	bodyRawMap := make(map[idwrap.IDWrap]mhttp.HTTPBodyRaw)
	for _, b := range data.HTTPBodyRaw {
		bodyRawMap[b.HttpID] = b
	}

	bodyFormMap := make(map[idwrap.IDWrap][]mhttp.HTTPBodyForm)
	for _, f := range data.HTTPBodyForms {
		bodyFormMap[f.HttpID] = append(bodyFormMap[f.HttpID], f)
	}

	bodyUrlMap := make(map[idwrap.IDWrap][]mhttp.HTTPBodyUrlencoded)
	for _, u := range data.HTTPBodyUrlencoded {
		bodyUrlMap[u.HttpID] = append(bodyUrlMap[u.HttpID], u)
	}

	assertsMap := make(map[idwrap.IDWrap][]mhttp.HTTPAssert)
	for _, a := range data.HTTPAsserts {
		assertsMap[a.HttpID] = append(assertsMap[a.HttpID], a)
	}

	// Build lookup context for delta merging
	deltaCtx := &deltaLookupContext{
		httpMap:     httpMap,
		headersMap:  headersMap,
		paramsMap:   paramsMap,
		bodyRawMap:  bodyRawMap,
		bodyFormMap: bodyFormMap,
		bodyUrlMap:  bodyUrlMap,
		assertsMap:  assertsMap,
	}

	// Node Specific Maps
	reqNodeMap := make(map[idwrap.IDWrap]mflow.NodeRequest)
	for _, n := range data.FlowRequestNodes {
		reqNodeMap[n.FlowNodeID] = n
	}

	ifNodeMap := make(map[idwrap.IDWrap]mflow.NodeIf)
	for _, n := range data.FlowConditionNodes {
		ifNodeMap[n.FlowNodeID] = n
	}

	forNodeMap := make(map[idwrap.IDWrap]mflow.NodeFor)
	for _, n := range data.FlowForNodes {
		forNodeMap[n.FlowNodeID] = n
	}

	forEachNodeMap := make(map[idwrap.IDWrap]mflow.NodeForEach)
	for _, n := range data.FlowForEachNodes {
		forEachNodeMap[n.FlowNodeID] = n
	}

	jsNodeMap := make(map[idwrap.IDWrap]mflow.NodeJS)
	for _, n := range data.FlowJSNodes {
		jsNodeMap[n.FlowNodeID] = n
	}

	// Edges Map (Source -> []Edge)
	edgesBySource := make(map[idwrap.IDWrap][]mflow.Edge)
	edgesByTarget := make(map[idwrap.IDWrap][]mflow.Edge)
	for _, e := range data.FlowEdges {
		edgesBySource[e.SourceID] = append(edgesBySource[e.SourceID], e)
		edgesByTarget[e.TargetID] = append(edgesByTarget[e.TargetID], e)
	}

	// 1. Construct the root YAML structure using the workspace name from the bundle
	wsName := data.Workspace.Name
	if wsName == "" {
		wsName = "Exported Workspace"
	}

	yamlFormat := YamlFlowFormatV2{
		WorkspaceName: wsName,
		Flows:         make([]YamlFlowFlowV2, 0),
	}

	// 2. Build top-level requests section from HTTP requests
	httpIDToRequestName := make(map[idwrap.IDWrap]string)
	requestNameUsed := make(map[string]bool)
	httpIDToDeltaID := make(map[idwrap.IDWrap]idwrap.IDWrap)

	// First pass: collect all HTTP requests used in flows and create unique names
	for _, flow := range data.Flows {
		for _, n := range data.FlowNodes {
			if n.FlowID != flow.ID || n.NodeKind != mflow.NODE_KIND_REQUEST {
				continue
			}
			reqNode, ok := reqNodeMap[n.ID]
			if !ok || reqNode.HttpID == nil {
				continue
			}
			httpReq, ok := httpMap[*reqNode.HttpID]
			if !ok {
				continue
			}

			if _, exists := httpIDToRequestName[httpReq.ID]; exists {
				continue
			}

			reqName := httpReq.Name
			if reqName == "" {
				reqName = "Request"
			}

			baseName := reqName
			counter := 1
			for requestNameUsed[reqName] {
				reqName = fmt.Sprintf("%s_%d", baseName, counter)
				counter++
			}
			requestNameUsed[reqName] = true
			httpIDToRequestName[httpReq.ID] = reqName

			if reqNode.DeltaHttpID != nil {
				httpIDToDeltaID[httpReq.ID] = *reqNode.DeltaHttpID
			}
		}
	}

	// Second pass: build the requests section
	var requests []YamlRequestDefV2
	var httpIDs []idwrap.IDWrap
	for httpID := range httpIDToRequestName {
		httpIDs = append(httpIDs, httpID)
	}
	sort.Slice(httpIDs, func(i, j int) bool {
		return httpIDToRequestName[httpIDs[i]] < httpIDToRequestName[httpIDs[j]]
	})

	for _, httpID := range httpIDs {
		reqName := httpIDToRequestName[httpID]
		httpReq := httpMap[httpID]

		var deltaHttpID *idwrap.IDWrap
		if did, hasDelta := httpIDToDeltaID[httpID]; hasDelta {
			deltaHttpID = &did
		}

		reqDef := buildRequestDefWithDelta(reqName, httpReq, deltaHttpID, deltaCtx)
		requests = append(requests, reqDef)
	}

	if len(requests) > 0 {
		yamlFormat.Requests = requests
	}

	// 3. Process each Flow
	flowNameUsed := make(map[string]bool)
	for _, flow := range data.Flows {
		flowName := flow.Name
		if flowName == "" {
			flowName = "Flow"
		}

		baseName := flowName
		counter := 1
		for flowNameUsed[flowName] {
			flowName = fmt.Sprintf("%s_%d", baseName, counter)
			counter++
		}
		flowNameUsed[flowName] = true

		flowYaml := YamlFlowFlowV2{
			Name:      flowName,
			Variables: make([]YamlFlowVariableV2, 0),
			Steps:     make([]YamlStepWrapper, 0),
		}

		for _, fv := range data.FlowVariables {
			if fv.FlowID == flow.ID {
				flowYaml.Variables = append(flowYaml.Variables, YamlFlowVariableV2{
					Name:  fv.Name,
					Value: fv.Value,
				})
			}
		}

		var flowNodes []mflow.Node
		var flowEdges []mflow.Edge
		var startNodeID idwrap.IDWrap
		for _, n := range data.FlowNodes {
			if n.FlowID == flow.ID {
				flowNodes = append(flowNodes, n)
				if n.NodeKind == mflow.NODE_KIND_MANUAL_START {
					startNodeID = n.ID
				}
			}
		}
		for _, e := range data.FlowEdges {
			if e.FlowID == flow.ID {
				flowEdges = append(flowEdges, e)
			}
		}

		orderedNodes := flowgraph.LinearizeNodes(startNodeID, flowNodes, flowEdges)

		for _, node := range orderedNodes {
			var stepWrapper YamlStepWrapper

			// Implicit deps
			var explicitDeps []string
			incoming := edgesByTarget[node.ID]
			for _, e := range incoming {
				sourceNode, ok := nodeMap[e.SourceID]
				if !ok {
					continue
				}

				depStr := sourceNode.Name
				switch e.SourceHandler {
				case mflow.HandleThen:
					depStr += ".then"
				case mflow.HandleElse:
					depStr += ".else"
				case mflow.HandleLoop:
					depStr += ".loop"
				case mflow.HandleUnspecified:
					// Do nothing, just the name
				default:
					// Unknown handler, default to name
				}

				explicitDeps = append(explicitDeps, depStr)
			}
			sort.Strings(explicitDeps)

			// Common struct logic
			common := YamlStepCommon{
				Name:      node.Name,
				DependsOn: StringOrSlice(explicitDeps),
			}

			switch node.NodeKind {
			case mflow.NODE_KIND_REQUEST:
				reqNode, ok := reqNodeMap[node.ID]
				if !ok || reqNode.HttpID == nil {
					continue
				}
				httpReq, ok := httpMap[*reqNode.HttpID]
				if !ok {
					continue
				}

				reqStep := &YamlStepRequest{
					YamlStepCommon: common,
				}

				if reqName, exists := httpIDToRequestName[httpReq.ID]; exists {
					reqStep.UseRequest = reqName
				} else {
					reqStep.Method = httpReq.Method
					reqStep.URL = httpReq.Url
				}
				stepWrapper.Request = reqStep

			case mflow.NODE_KIND_CONDITION:
				ifNode, ok := ifNodeMap[node.ID]
				if !ok {
					continue
				}
				ifStep := &YamlStepIf{
					YamlStepCommon: common,
					Condition:      ifNode.Condition.Comparisons.Expression,
				}
				// Removed legacy then/else fields
				stepWrapper.If = ifStep

			case mflow.NODE_KIND_FOR:
				forNode, ok := forNodeMap[node.ID]
				if !ok {
					continue
				}
				forStep := &YamlStepFor{
					YamlStepCommon: common,
					IterCount:      fmt.Sprintf("%d", forNode.IterCount),
				}
				// Removed legacy loop field
				stepWrapper.For = forStep

			case mflow.NODE_KIND_FOR_EACH:
				forEachNode, ok := forEachNodeMap[node.ID]
				if !ok {
					continue
				}
				forEachStep := &YamlStepForEach{
					YamlStepCommon: common,
					Items:          forEachNode.IterExpression,
				}
				// Removed legacy loop field
				stepWrapper.ForEach = forEachStep

			case mflow.NODE_KIND_JS:
				jsNode, ok := jsNodeMap[node.ID]
				if !ok {
					continue
				}
				jsStep := &YamlStepJS{
					YamlStepCommon: common,
					Code:           string(jsNode.Code),
				}
				stepWrapper.JS = jsStep

			case mflow.NODE_KIND_MANUAL_START:
				if node.ID == startNodeID {
					stepWrapper.ManualStart = &common
				} else {
					continue
				}
			}

			// Add to flow
			// Because stepWrapper has pointer fields, "empty" fields are nil
			// Checking if any field is set (simplified check, assume one set if we got here)
			isValid := stepWrapper.Request != nil || stepWrapper.If != nil || stepWrapper.For != nil || stepWrapper.ForEach != nil || stepWrapper.JS != nil || stepWrapper.ManualStart != nil
			if isValid {
				flowYaml.Steps = append(flowYaml.Steps, stepWrapper)
			}
		}

		yamlFormat.Flows = append(yamlFormat.Flows, flowYaml)
	}

	// 4. Export Environments
	if len(data.Environments) > 0 {
		envMap := make(map[idwrap.IDWrap]*YamlEnvironmentV2)
		for _, env := range data.Environments {
			envMap[env.ID] = &YamlEnvironmentV2{
				Name:      env.Name,
				Variables: make(map[string]string),
			}
		}
		for _, v := range data.EnvironmentVars {
			if env, ok := envMap[v.EnvID]; ok {
				env.Variables[v.VarKey] = v.Value
			}
		}
		for _, env := range data.Environments {
			if yamlEnv, ok := envMap[env.ID]; ok {
				yamlFormat.Environments = append(yamlFormat.Environments, *yamlEnv)
			}
		}
	}

	// 5. Generate default Run configuration
	if len(yamlFormat.Flows) > 0 {
		yamlFormat.Run = make([]YamlRunEntryV2, 0, len(yamlFormat.Flows))
		for _, flow := range yamlFormat.Flows {
			yamlFormat.Run = append(yamlFormat.Run, YamlRunEntryV2{
				Flow: flow.Name,
			})
		}
	}

	return yaml.Marshal(yamlFormat)
}

type deltaLookupContext struct {
	httpMap     map[idwrap.IDWrap]mhttp.HTTP
	headersMap  map[idwrap.IDWrap][]mhttp.HTTPHeader
	paramsMap   map[idwrap.IDWrap][]mhttp.HTTPSearchParam
	bodyRawMap  map[idwrap.IDWrap]mhttp.HTTPBodyRaw
	bodyFormMap map[idwrap.IDWrap][]mhttp.HTTPBodyForm
	bodyUrlMap  map[idwrap.IDWrap][]mhttp.HTTPBodyUrlencoded
	assertsMap  map[idwrap.IDWrap][]mhttp.HTTPAssert
}

func buildRequestDefWithDelta(reqName string, baseHttp mhttp.HTTP, deltaHttpID *idwrap.IDWrap, ctx *deltaLookupContext) YamlRequestDefV2 {
	method := baseHttp.Method
	url := baseHttp.Url

	if deltaHttpID != nil {
		if deltaHttp, ok := ctx.httpMap[*deltaHttpID]; ok {
			if deltaHttp.DeltaUrl != nil && *deltaHttp.DeltaUrl != "" {
				url = *deltaHttp.DeltaUrl
			}
			if deltaHttp.DeltaMethod != nil && *deltaHttp.DeltaMethod != "" {
				method = *deltaHttp.DeltaMethod
			}
		}
	}

	reqDef := YamlRequestDefV2{
		Name:   reqName,
		Method: method,
		URL:    url,
	}

	reqDef.Headers = mergeHeaders(baseHttp.ID, deltaHttpID, ctx)
	reqDef.QueryParams = mergeQueryParams(baseHttp.ID, deltaHttpID, ctx)
	reqDef.Body = mergeBody(baseHttp.ID, deltaHttpID, ctx)
	reqDef.Assertions = mergeAssertions(baseHttp.ID, deltaHttpID, ctx)

	return reqDef
}

func mergeHeaders(baseHttpID idwrap.IDWrap, deltaHttpID *idwrap.IDWrap, ctx *deltaLookupContext) HeaderMapOrSlice {
	var result []YamlNameValuePairV2
	baseHeaders := ctx.headersMap[baseHttpID]

	deltaByParentID := make(map[idwrap.IDWrap]mhttp.HTTPHeader)
	if deltaHttpID != nil {
		for _, dh := range ctx.headersMap[*deltaHttpID] {
			if dh.ParentHttpHeaderID != nil {
				deltaByParentID[*dh.ParentHttpHeaderID] = dh
			}
		}
	}

	for _, h := range baseHeaders {
		key := h.Key
		value := h.Value
		enabled := h.Enabled
		description := h.Description

		if delta, hasDelta := deltaByParentID[h.ID]; hasDelta {
			if delta.DeltaKey != nil {
				key = *delta.DeltaKey
			}
			if delta.DeltaValue != nil {
				value = *delta.DeltaValue
			}
			if delta.DeltaEnabled != nil {
				enabled = *delta.DeltaEnabled
			}
			if delta.DeltaDescription != nil {
				description = *delta.DeltaDescription
			}
		}

		result = append(result, YamlNameValuePairV2{
			Name: key, Value: value, Enabled: enabled, Description: description,
		})
	}

	if deltaHttpID != nil {
		for _, dh := range ctx.headersMap[*deltaHttpID] {
			if dh.ParentHttpHeaderID == nil {
				result = append(result, YamlNameValuePairV2{
					Name: dh.Key, Value: dh.Value, Enabled: dh.Enabled, Description: dh.Description,
				})
			}
		}
	}

	if len(result) == 0 {
		return nil
	}
	return HeaderMapOrSlice(result)
}

func mergeQueryParams(baseHttpID idwrap.IDWrap, deltaHttpID *idwrap.IDWrap, ctx *deltaLookupContext) HeaderMapOrSlice {
	var result []YamlNameValuePairV2
	baseParams := ctx.paramsMap[baseHttpID]

	deltaByParentID := make(map[idwrap.IDWrap]mhttp.HTTPSearchParam)
	if deltaHttpID != nil {
		for _, dp := range ctx.paramsMap[*deltaHttpID] {
			if dp.ParentHttpSearchParamID != nil {
				deltaByParentID[*dp.ParentHttpSearchParamID] = dp
			}
		}
	}

	for _, p := range baseParams {
		key := p.Key
		value := p.Value
		enabled := p.Enabled
		description := p.Description

		if delta, hasDelta := deltaByParentID[p.ID]; hasDelta {
			if delta.DeltaKey != nil {
				key = *delta.DeltaKey
			}
			if delta.DeltaValue != nil {
				value = *delta.DeltaValue
			}
			if delta.DeltaEnabled != nil {
				enabled = *delta.DeltaEnabled
			}
			if delta.DeltaDescription != nil {
				description = *delta.DeltaDescription
			}
		}

		result = append(result, YamlNameValuePairV2{
			Name: key, Value: value, Enabled: enabled, Description: description,
		})
	}

	if deltaHttpID != nil {
		for _, dp := range ctx.paramsMap[*deltaHttpID] {
			if dp.ParentHttpSearchParamID == nil {
				result = append(result, YamlNameValuePairV2{
					Name: dp.Key, Value: dp.Value, Enabled: dp.Enabled, Description: dp.Description,
				})
			}
		}
	}

	if len(result) == 0 {
		return nil
	}
	return HeaderMapOrSlice(result)
}

func mergeBody(baseHttpID idwrap.IDWrap, deltaHttpID *idwrap.IDWrap, ctx *deltaLookupContext) *YamlBodyUnion {
	// Forms
	if forms := mergeBodyForms(baseHttpID, deltaHttpID, ctx); len(forms) > 0 {
		return &YamlBodyUnion{
			Type: "form-data",
			Form: HeaderMapOrSlice(forms),
		}
	}

	// UrlEncoded
	if urlencoded := mergeBodyUrlencoded(baseHttpID, deltaHttpID, ctx); len(urlencoded) > 0 {
		return &YamlBodyUnion{
			Type:       "urlencoded",
			UrlEncoded: HeaderMapOrSlice(urlencoded),
		}
	}

	// Raw
	return mergeBodyRaw(baseHttpID, deltaHttpID, ctx)
}

func mergeBodyForms(baseHttpID idwrap.IDWrap, deltaHttpID *idwrap.IDWrap, ctx *deltaLookupContext) []YamlNameValuePairV2 {
	baseForms := ctx.bodyFormMap[baseHttpID]
	var result []YamlNameValuePairV2

	// if base empty, check delta new
	if len(baseForms) == 0 {
		if deltaHttpID != nil {
			deltaForms := ctx.bodyFormMap[*deltaHttpID]
			for _, f := range deltaForms {
				result = append(result, YamlNameValuePairV2{Name: f.Key, Value: f.Value, Enabled: f.Enabled, Description: f.Description})
			}
		}
		return result
	}

	deltaByParentID := make(map[idwrap.IDWrap]mhttp.HTTPBodyForm)
	if deltaHttpID != nil {
		for _, df := range ctx.bodyFormMap[*deltaHttpID] {
			if df.ParentHttpBodyFormID != nil {
				deltaByParentID[*df.ParentHttpBodyFormID] = df
			}
		}
	}

	for _, f := range baseForms {
		key := f.Key
		value := f.Value
		enabled := f.Enabled
		description := f.Description

		if delta, hasDelta := deltaByParentID[f.ID]; hasDelta {
			if delta.DeltaKey != nil {
				key = *delta.DeltaKey
			}
			if delta.DeltaValue != nil {
				value = *delta.DeltaValue
			}
			if delta.DeltaEnabled != nil {
				enabled = *delta.DeltaEnabled
			}
			if delta.DeltaDescription != nil {
				description = *delta.DeltaDescription
			}
		}

		result = append(result, YamlNameValuePairV2{Name: key, Value: value, Enabled: enabled, Description: description})
	}

	if deltaHttpID != nil {
		for _, df := range ctx.bodyFormMap[*deltaHttpID] {
			if df.ParentHttpBodyFormID == nil {
				result = append(result, YamlNameValuePairV2{Name: df.Key, Value: df.Value, Enabled: df.Enabled, Description: df.Description})
			}
		}
	}

	return result
}

func mergeBodyUrlencoded(baseHttpID idwrap.IDWrap, deltaHttpID *idwrap.IDWrap, ctx *deltaLookupContext) []YamlNameValuePairV2 {
	baseUrls := ctx.bodyUrlMap[baseHttpID]
	var result []YamlNameValuePairV2

	if len(baseUrls) == 0 {
		if deltaHttpID != nil {
			deltaUrls := ctx.bodyUrlMap[*deltaHttpID]
			for _, u := range deltaUrls {
				result = append(result, YamlNameValuePairV2{Name: u.Key, Value: u.Value, Enabled: u.Enabled, Description: u.Description})
			}
		}
		return result
	}

	deltaByParentID := make(map[idwrap.IDWrap]mhttp.HTTPBodyUrlencoded)
	if deltaHttpID != nil {
		for _, du := range ctx.bodyUrlMap[*deltaHttpID] {
			if du.ParentHttpBodyUrlEncodedID != nil {
				deltaByParentID[*du.ParentHttpBodyUrlEncodedID] = du
			}
		}
	}

	for _, u := range baseUrls {
		key := u.Key
		value := u.Value
		enabled := u.Enabled
		description := u.Description

		if delta, hasDelta := deltaByParentID[u.ID]; hasDelta {
			if delta.DeltaKey != nil {
				key = *delta.DeltaKey
			}
			if delta.DeltaValue != nil {
				value = *delta.DeltaValue
			}
			if delta.DeltaEnabled != nil {
				enabled = *delta.DeltaEnabled
			}
			if delta.DeltaDescription != nil {
				description = *delta.DeltaDescription
			}
		}
		result = append(result, YamlNameValuePairV2{Name: key, Value: value, Enabled: enabled, Description: description})
	}

	if deltaHttpID != nil {
		for _, du := range ctx.bodyUrlMap[*deltaHttpID] {
			if du.ParentHttpBodyUrlEncodedID == nil {
				result = append(result, YamlNameValuePairV2{Name: du.Key, Value: du.Value, Enabled: du.Enabled, Description: du.Description})
			}
		}
	}
	return result
}

func mergeBodyRaw(baseHttpID idwrap.IDWrap, deltaHttpID *idwrap.IDWrap, ctx *deltaLookupContext) *YamlBodyUnion {
	// Delta raw body fully overwrites the base body
	// When delta exists with DeltaRawData, use ONLY the delta (no merging with base)
	// Always output as raw type to preserve template variables like {{ request_5.response.body.id }}

	if deltaHttpID != nil {
		deltaRaw, ok := ctx.bodyRawMap[*deltaHttpID]
		if ok && len(deltaRaw.DeltaRawData) > 0 {
			// Delta fully overwrites - use only delta data as raw
			return &YamlBodyUnion{
				Type: "raw",
				Raw:  string(deltaRaw.DeltaRawData),
			}
		}
	}

	// No delta override - use base body
	baseRaw, ok := ctx.bodyRawMap[baseHttpID]
	if !ok || len(baseRaw.RawData) == 0 {
		return nil
	}

	return &YamlBodyUnion{
		Type: "raw",
		Raw:  string(baseRaw.RawData),
	}
}

func mergeAssertions(baseHttpID idwrap.IDWrap, deltaHttpID *idwrap.IDWrap, ctx *deltaLookupContext) AssertionsOrSlice {
	var result []YamlAssertionV2
	baseAsserts := ctx.assertsMap[baseHttpID]

	deltaByParentID := make(map[idwrap.IDWrap]mhttp.HTTPAssert)
	if deltaHttpID != nil {
		for _, da := range ctx.assertsMap[*deltaHttpID] {
			if da.ParentHttpAssertID != nil {
				deltaByParentID[*da.ParentHttpAssertID] = da
			}
		}
	}

	for _, a := range baseAsserts {
		val := a.Value
		enabled := a.Enabled

		if delta, hasDelta := deltaByParentID[a.ID]; hasDelta {
			if delta.DeltaValue != nil {
				val = *delta.DeltaValue
			}
			if delta.DeltaEnabled != nil {
				enabled = *delta.DeltaEnabled
			}
		}

		result = append(result, YamlAssertionV2{Expression: val, Enabled: enabled})
	}

	if deltaHttpID != nil {
		for _, da := range ctx.assertsMap[*deltaHttpID] {
			if da.ParentHttpAssertID == nil {
				result = append(result, YamlAssertionV2{Expression: da.Value, Enabled: da.Enabled})
			}
		}
	}

	if len(result) == 0 {
		return nil
	}
	return AssertionsOrSlice(result)
}
