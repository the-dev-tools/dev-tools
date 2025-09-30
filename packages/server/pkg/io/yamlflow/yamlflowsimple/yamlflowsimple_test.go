package yamlflowsimple

import (
	"context"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/http/request"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/ioworkspace"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/model/mbodyform"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mbodyurl"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/model/mvar"
	"the-dev-tools/server/pkg/model/mworkspace"
)

const roundTripYAML = `workspace_name: Round Trip Workspace
requests:
  - name: get-user
    method: GET
    url: https://example.dev/users
    assertions:
      - expression: response.status == 200
        enabled: true
flows:
  - name: sample
    steps:
      - request:
          name: Fetch User
          use_request: get-user
`

const bodyTypesYAML = `workspace_name: Body Types
requests:
  - name: form-template
    method: POST
    url: https://example.dev/upload
    body:
      type: form-data
      items:
        - name: file
          value: '{{ #file:./payload.bin }}'
          description: upload
flows:
  - name: sample
    steps:
      - request:
          name: UploadStep
          use_request: form-template
      - request:
          name: SearchStep
          method: POST
          url: https://example.dev/search
          body:
            type: x-www-form-urlencoded
            items:
              - name: q
                value: search
              - name: page
                value: '2'
                enabled: false
`

func TestYamlFlowRoundTripWithAssertions(t *testing.T) {
	workspace, err := ImportYamlFlowYAML([]byte(roundTripYAML))
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}

	if !workspaceHasAssertion(workspace, "response.status == 200") {
		t.Fatalf("imported workspace missing expected assertion")
	}

	exported, err := ExportYamlFlowYAML(context.Background(), workspace, nil)
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}

	var exportedDoc map[string]any
	if err := yaml.Unmarshal(exported, &exportedDoc); err != nil {
		t.Fatalf("failed to unmarshal exported yaml: %v", err)
	}

	assertions := collectAllRequestAssertions(exportedDoc)
	if len(assertions) == 0 {
		t.Fatalf("exported yaml missing assertions:\n%s", string(exported))
	}

	reimported, err := ImportYamlFlowYAML(exported)
	if err != nil {
		t.Fatalf("re-import failed: %v", err)
	}

	if !workspaceHasAssertion(reimported, "response.status == 200") {
		t.Fatalf("re-imported workspace missing expected assertion")
	}
}

func TestExportYamlFlowYAMLEnvironments(t *testing.T) {
	workspace, err := ImportYamlFlowYAML([]byte(roundTripYAML))
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}

	workspaceID := workspace.Workspace.ID
	activeEnvID := idwrap.NewNow()
	globalEnvID := idwrap.NewNow()
	now := time.Now()

	workspace.Workspace.ActiveEnv = activeEnvID
	workspace.Workspace.GlobalEnv = globalEnvID
	workspace.Environments = []menv.Env{
		{
			ID:          globalEnvID,
			WorkspaceID: workspaceID,
			Type:        menv.EnvGlobal,
			Name:        "global",
			Description: "Global scope",
			Updated:     now,
		},
		{
			ID:          activeEnvID,
			WorkspaceID: workspaceID,
			Type:        menv.EnvNormal,
			Name:        "dev",
			Description: "Development",
			Updated:     now,
		},
	}
	workspace.Variables = []mvar.Var{
		{
			ID:      idwrap.NewNow(),
			EnvID:   activeEnvID,
			VarKey:  "api_base",
			Value:   "https://dev.example.local",
			Enabled: true,
		},
		{
			ID:          idwrap.NewNow(),
			EnvID:       globalEnvID,
			VarKey:      "auth_token",
			Value:       "token-123",
			Enabled:     false,
			Description: "Temporarily disabled",
		},
	}

	exported, err := ExportYamlFlowYAML(context.Background(), workspace, nil)
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}

	var yamlDoc YamlFlowFormat
	if err := yaml.Unmarshal(exported, &yamlDoc); err != nil {
		t.Fatalf("unmarshal exported yaml failed: %v", err)
	}

	if yamlDoc.ActiveEnvironment != "dev" {
		t.Fatalf("expected active environment 'dev', got %q", yamlDoc.ActiveEnvironment)
	}
	if yamlDoc.GlobalEnvironment != "global" {
		t.Fatalf("expected global environment 'global', got %q", yamlDoc.GlobalEnvironment)
	}
	if len(yamlDoc.Environments) != 2 {
		t.Fatalf("expected 2 environments in export, got %d", len(yamlDoc.Environments))
	}

	imported, err := ImportYamlFlowYAML(exported)
	if err != nil {
		t.Fatalf("re-import failed: %v", err)
	}

	if len(imported.Environments) != 2 {
		t.Fatalf("expected 2 environments after import, got %d", len(imported.Environments))
	}

	nameByID := make(map[idwrap.IDWrap]string)
	for _, env := range imported.Environments {
		nameByID[env.ID] = env.Name
	}

	activeName := nameByID[imported.Workspace.ActiveEnv]
	if activeName != "dev" {
		t.Fatalf("expected active environment name 'dev', got %q", activeName)
	}

	globalName := nameByID[imported.Workspace.GlobalEnv]
	if globalName != "global" {
		t.Fatalf("expected global environment name 'global', got %q", globalName)
	}

	if len(imported.Variables) != 2 {
		t.Fatalf("expected 2 environment variables, got %d", len(imported.Variables))
	}

	var authVar mvar.Var
	var apiVar mvar.Var
	for _, v := range imported.Variables {
		switch v.VarKey {
		case "auth_token":
			authVar = v
		case "api_base":
			apiVar = v
		}
	}

	if authVar.VarKey != "auth_token" {
		t.Fatalf("expected auth_token variable to be present")
	}
	if authVar.Enabled {
		t.Fatalf("expected auth_token variable to be disabled after re-import")
	}
	if authVar.Description != "Temporarily disabled" {
		t.Fatalf("expected auth_token description to round-trip, got %q", authVar.Description)
	}
	if apiVar.VarKey != "api_base" || !apiVar.Enabled {
		t.Fatalf("expected api_base variable to remain enabled after re-import")
	}
}

func workspaceHasAssertion(workspace *ioworkspace.WorkspaceData, expression string) bool {
	for _, a := range workspace.ExampleAsserts {
		if a.Condition.Comparisons.Expression == expression {
			return true
		}
	}
	return false
}

func collectAllRequestAssertions(doc map[string]any) []map[string]any {
	requestsAny, ok := doc["requests"].([]any)
	if !ok {
		return nil
	}

	var result []map[string]any

	for _, reqAny := range requestsAny {
		req, ok := reqAny.(map[string]any)
		if !ok {
			continue
		}
		if assertions, ok := req["assertions"].([]any); ok {
			for _, assertionAny := range assertions {
				if assertionMap, ok := assertionAny.(map[string]any); ok {
					result = append(result, assertionMap)
				}
			}
		}
	}
	return result
}

func TestBuildRequestDefinitionsDedupsAssertions(t *testing.T) {
	flowID := idwrap.NewNow()
	nodeID := idwrap.NewNow()
	endpointID := idwrap.NewNow()
	deltaEndpointID := idwrap.NewNow()
	exampleID := idwrap.NewNow()
	deltaExampleID := idwrap.NewNow()

	workspace := &ioworkspace.WorkspaceData{
		Workspace: mworkspace.Workspace{ID: idwrap.NewNow(), Name: "workspace"},
		Flows: []mflow.Flow{
			{ID: flowID, Name: "flow"},
		},
		FlowNodes: []mnnode.MNode{
			{ID: nodeID, FlowID: flowID, Name: "request_1", NodeKind: mnnode.NODE_KIND_REQUEST},
		},
		FlowRequestNodes: []mnrequest.MNRequest{
			{
				FlowNodeID:       nodeID,
				EndpointID:       &endpointID,
				ExampleID:        &exampleID,
				DeltaEndpointID:  &deltaEndpointID,
				DeltaExampleID:   &deltaExampleID,
				HasRequestConfig: true,
			},
		},
		Endpoints: []mitemapi.ItemApi{
			{ID: endpointID, Name: "request_1", Method: "GET", Url: "https://example.dev"},
			{ID: deltaEndpointID, Name: "request_1 (delta)", Method: "GET", Url: "https://example.dev", DeltaParentID: &endpointID},
		},
		Examples: []mitemapiexample.ItemApiExample{
			{ID: exampleID, ItemApiID: endpointID, Name: "request_1"},
			{ID: deltaExampleID, ItemApiID: deltaEndpointID, Name: "request_1 (delta)", VersionParentID: &exampleID},
		},
		ExampleAsserts: []massert.Assert{
			{ID: idwrap.NewNow(), ExampleID: exampleID, Condition: mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "response.status == 201"}}, Enable: true},
			{ID: idwrap.NewNow(), ExampleID: exampleID, Condition: mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "response.status == 201"}}, Enable: true},
			{ID: idwrap.NewNow(), ExampleID: deltaExampleID, Condition: mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "response.status == 201"}}, Enable: true},
		},
	}

	requests, err := buildRequestDefinitions(context.Background(), workspace, nil)
	if err != nil {
		t.Fatalf("buildRequestDefinitions failed: %v", err)
	}
	req, ok := requests["request_1"]
	if !ok {
		t.Fatalf("expected request_1 in definitions")
	}

	assertionsAny, ok := req["assertions"].([]map[string]any)
	if !ok {
		t.Fatalf("expected assertions slice in request definition")
	}

	if len(assertionsAny) != 1 {
		t.Fatalf("expected 1 assertion after dedup, got %d", len(assertionsAny))
	}

	if expr, _ := assertionsAny[0]["expression"].(string); expr != "response.status == 201" {
		t.Fatalf("unexpected assertion expression %q", expr)
	}
}

func TestBuildRequestDefinitionsIncludesFormBody(t *testing.T) {
	flowID := idwrap.NewNow()
	nodeID := idwrap.NewNow()
	endpointID := idwrap.NewNow()
	deltaEndpointID := idwrap.NewNow()
	exampleID := idwrap.NewNow()
	deltaExampleID := idwrap.NewNow()
	baseFormID := idwrap.NewNow()

	workspace := &ioworkspace.WorkspaceData{
		Workspace: mworkspace.Workspace{ID: idwrap.NewNow(), Name: "workspace"},
		Flows:     []mflow.Flow{{ID: flowID, Name: "flow"}},
		FlowNodes: []mnnode.MNode{{ID: nodeID, FlowID: flowID, Name: "request_form", NodeKind: mnnode.NODE_KIND_REQUEST}},
		FlowRequestNodes: []mnrequest.MNRequest{{
			FlowNodeID:       nodeID,
			EndpointID:       &endpointID,
			ExampleID:        &exampleID,
			DeltaEndpointID:  &deltaEndpointID,
			DeltaExampleID:   &deltaExampleID,
			HasRequestConfig: true,
		}},
		Endpoints: []mitemapi.ItemApi{
			{ID: endpointID, Name: "request_form", Method: "POST", Url: "https://example.dev"},
			{ID: deltaEndpointID, Name: "request_form (delta)", Method: "POST", Url: "https://example.dev", DeltaParentID: &endpointID},
		},
		Examples: []mitemapiexample.ItemApiExample{
			{ID: exampleID, ItemApiID: endpointID, Name: "request_form", BodyType: mitemapiexample.BodyTypeForm},
			{ID: deltaExampleID, ItemApiID: deltaEndpointID, Name: "request_form (delta)", BodyType: mitemapiexample.BodyTypeForm, VersionParentID: &exampleID},
		},
		FormBodies: []mbodyform.BodyForm{
			{ID: baseFormID, ExampleID: exampleID, BodyKey: "file", Value: "{{ #file:./payload.bin }}", Enable: true, Description: "upload"},
		},
	}

	requests, err := buildRequestDefinitions(context.Background(), workspace, nil)
	if err != nil {
		t.Fatalf("buildRequestDefinitions failed: %v", err)
	}

	req, ok := requests["request_form"]
	if !ok {
		t.Fatalf("expected request_form in definitions")
	}

	body, ok := req["body"].(map[string]any)
	if !ok {
		t.Fatalf("expected map body entry, got %T", req["body"])
	}
	if bodyType, _ := body["type"].(string); bodyType != "form-data" {
		t.Fatalf("expected form-data type, got %q", bodyType)
	}

	itemsAny, ok := body["items"].([]map[string]any)
	if !ok {
		t.Fatalf("expected slice items, got %T", body["items"])
	}
	if len(itemsAny) != 1 {
		t.Fatalf("expected 1 form item, got %d", len(itemsAny))
	}

	item := itemsAny[0]
	if name, _ := item["name"].(string); name != "file" {
		t.Fatalf("unexpected form name %q", name)
	}
	if value, _ := item["value"].(string); value != "{{ #file:./payload.bin }}" {
		t.Fatalf("unexpected form value %q", value)
	}
	if enabled, _ := item["enabled"].(bool); !enabled {
		t.Fatalf("expected enabled true for first item")
	}
	if desc, _ := item["description"].(string); desc != "upload" {
		t.Fatalf("unexpected description %q", desc)
	}
}

func TestBuildRequestDefinitionsPrefersFormBodyOverRaw(t *testing.T) {
	flowID := idwrap.NewNow()
	nodeID := idwrap.NewNow()
	endpointID := idwrap.NewNow()
	deltaEndpointID := idwrap.NewNow()
	exampleID := idwrap.NewNow()
	deltaExampleID := idwrap.NewNow()

	workspace := &ioworkspace.WorkspaceData{
		Workspace: mworkspace.Workspace{ID: idwrap.NewNow(), Name: "workspace"},
		Flows:     []mflow.Flow{{ID: flowID, Name: "flow"}},
		FlowNodes: []mnnode.MNode{{ID: nodeID, FlowID: flowID, Name: "request_form", NodeKind: mnnode.NODE_KIND_REQUEST}},
		FlowRequestNodes: []mnrequest.MNRequest{{
			FlowNodeID:       nodeID,
			EndpointID:       &endpointID,
			ExampleID:        &exampleID,
			DeltaEndpointID:  &deltaEndpointID,
			DeltaExampleID:   &deltaExampleID,
			HasRequestConfig: true,
		}},
		Endpoints: []mitemapi.ItemApi{
			{ID: endpointID, Name: "request_form", Method: "POST", Url: "https://example.dev"},
			{ID: deltaEndpointID, Name: "request_form (delta)", Method: "POST", Url: "https://example.dev", DeltaParentID: &endpointID},
		},
		Examples: []mitemapiexample.ItemApiExample{
			{ID: exampleID, ItemApiID: endpointID, Name: "request_form", BodyType: mitemapiexample.BodyTypeRaw},
			{ID: deltaExampleID, ItemApiID: deltaEndpointID, Name: "request_form (delta)", BodyType: mitemapiexample.BodyTypeForm, VersionParentID: &exampleID},
		},
		Rawbodies: []mbodyraw.ExampleBodyRaw{{
			ID:        idwrap.NewNow(),
			ExampleID: exampleID,
			Data:      []byte(`{"legacy":true}`),
		}},
		FormBodies: []mbodyform.BodyForm{
			{ID: idwrap.NewNow(), ExampleID: deltaExampleID, BodyKey: "file", Value: "{{ #file:./payload.bin }}", Enable: true},
		},
	}

	requests, err := buildRequestDefinitions(context.Background(), workspace, nil)
	if err != nil {
		t.Fatalf("buildRequestDefinitions failed: %v", err)
	}

	req, ok := requests["request_form"]
	if !ok {
		t.Fatalf("expected request_form in definitions")
	}

	body, ok := req["body"].(map[string]any)
	if !ok {
		t.Fatalf("expected map body entry, got %T", req["body"])
	}
	if bodyType, _ := body["type"].(string); bodyType != "form-data" {
		t.Fatalf("expected form-data type, got %q", bodyType)
	}
}

func TestExportYamlFlowYAMLIncludesFormType(t *testing.T) {
	flowID := idwrap.NewNow()
	nodeID := idwrap.NewNow()
	endpointID := idwrap.NewNow()
	deltaEndpointID := idwrap.NewNow()
	exampleID := idwrap.NewNow()
	deltaExampleID := idwrap.NewNow()

	workspace := &ioworkspace.WorkspaceData{
		Workspace: mworkspace.Workspace{ID: idwrap.NewNow(), Name: "workspace"},
		Flows:     []mflow.Flow{{ID: flowID, Name: "flow"}},
		FlowNodes: []mnnode.MNode{{ID: nodeID, FlowID: flowID, Name: "node", NodeKind: mnnode.NODE_KIND_REQUEST}},
		FlowRequestNodes: []mnrequest.MNRequest{{
			FlowNodeID:       nodeID,
			EndpointID:       &endpointID,
			ExampleID:        &exampleID,
			DeltaEndpointID:  &deltaEndpointID,
			DeltaExampleID:   &deltaExampleID,
			HasRequestConfig: true,
		}},
		Endpoints: []mitemapi.ItemApi{
			{ID: endpointID, Name: "node", Method: "POST", Url: "https://example.dev"},
			{ID: deltaEndpointID, Name: "node (delta)", Method: "POST", Url: "https://example.dev", DeltaParentID: &endpointID},
		},
		Examples: []mitemapiexample.ItemApiExample{
			{ID: exampleID, ItemApiID: endpointID, Name: "node", BodyType: mitemapiexample.BodyTypeForm},
			{ID: deltaExampleID, ItemApiID: deltaEndpointID, Name: "node (delta)", BodyType: mitemapiexample.BodyTypeForm, VersionParentID: &exampleID},
		},
		FormBodies: []mbodyform.BodyForm{
			{ID: idwrap.NewNow(), ExampleID: exampleID, BodyKey: "file", Value: "{{ #file:./payload.bin }}", Enable: true},
		},
	}

	output, err := ExportYamlFlowYAML(context.Background(), workspace, nil)
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}

	if !strings.Contains(string(output), "type: form-data") {
		t.Fatalf("expected exported yaml to contain form-data type:\n%s", string(output))
	}
}

func TestEncodeMergedBodyFormFallbackToRaw(t *testing.T) {
	mergeOutput := request.MergeExamplesOutput{
		Merged:       mitemapiexample.ItemApiExample{BodyType: mitemapiexample.BodyTypeForm},
		MergeRawBody: mbodyraw.ExampleBodyRaw{Data: []byte(`{"email":"admin@example.com","password":"admin123"}`)},
	}

	body, ok := encodeMergedBody(mergeOutput)
	if !ok {
		t.Fatalf("expected body encoding to succeed")
	}
	bodyMap, ok := body.(map[string]any)
	if !ok {
		t.Fatalf("expected map body entry, got %T", body)
	}
	if bodyType, _ := bodyMap["type"].(string); bodyType != "form-data" {
		t.Fatalf("expected form-data fallback, got %q", bodyType)
	}
	items, _ := bodyMap["items"].([]map[string]any)
	if len(items) != 2 {
		t.Fatalf("expected fallback to create 2 items, got %d", len(items))
	}
}

func TestBuildRequestDefinitionsIncludesUrlEncodedBody(t *testing.T) {
	flowID := idwrap.NewNow()
	nodeID := idwrap.NewNow()
	endpointID := idwrap.NewNow()
	deltaEndpointID := idwrap.NewNow()
	exampleID := idwrap.NewNow()
	deltaExampleID := idwrap.NewNow()

	workspace := &ioworkspace.WorkspaceData{
		Workspace: mworkspace.Workspace{ID: idwrap.NewNow(), Name: "workspace"},
		Flows:     []mflow.Flow{{ID: flowID, Name: "flow"}},
		FlowNodes: []mnnode.MNode{{ID: nodeID, FlowID: flowID, Name: "request_urlencoded", NodeKind: mnnode.NODE_KIND_REQUEST}},
		FlowRequestNodes: []mnrequest.MNRequest{{
			FlowNodeID:       nodeID,
			EndpointID:       &endpointID,
			ExampleID:        &exampleID,
			DeltaEndpointID:  &deltaEndpointID,
			DeltaExampleID:   &deltaExampleID,
			HasRequestConfig: true,
		}},
		Endpoints: []mitemapi.ItemApi{
			{ID: endpointID, Name: "request_urlencoded", Method: "POST", Url: "https://example.dev"},
			{ID: deltaEndpointID, Name: "request_urlencoded (delta)", Method: "POST", Url: "https://example.dev", DeltaParentID: &endpointID},
		},
		Examples: []mitemapiexample.ItemApiExample{
			{ID: exampleID, ItemApiID: endpointID, Name: "request_urlencoded", BodyType: mitemapiexample.BodyTypeUrlencoded},
			{ID: deltaExampleID, ItemApiID: deltaEndpointID, Name: "request_urlencoded (delta)", BodyType: mitemapiexample.BodyTypeUrlencoded, VersionParentID: &exampleID},
		},
		UrlBodies: []mbodyurl.BodyURLEncoded{
			{ID: idwrap.NewNow(), ExampleID: exampleID, BodyKey: "search", Value: "foo", Enable: true},
			{ID: idwrap.NewNow(), ExampleID: deltaExampleID, BodyKey: "page", Value: "2", Enable: true},
		},
	}

	requests, err := buildRequestDefinitions(context.Background(), workspace, nil)
	if err != nil {
		t.Fatalf("buildRequestDefinitions failed: %v", err)
	}

	req, ok := requests["request_urlencoded"]
	if !ok {
		t.Fatalf("expected request_urlencoded in definitions")
	}

	body, ok := req["body"].(map[string]any)
	if !ok {
		t.Fatalf("expected map body entry, got %T", req["body"])
	}
	if bodyType, _ := body["type"].(string); bodyType != "x-www-form-urlencoded" {
		t.Fatalf("expected x-www-form-urlencoded type, got %q", bodyType)
	}

	itemsAny, ok := body["items"].([]map[string]any)
	if !ok {
		t.Fatalf("expected slice items, got %T", body["items"])
	}
	if len(itemsAny) != 2 {
		t.Fatalf("expected 2 urlencoded items, got %d", len(itemsAny))
	}

	params := make(map[string]map[string]any, len(itemsAny))
	for _, item := range itemsAny {
		if name, _ := item["name"].(string); name != "" {
			params[name] = item
		}
	}

	first, ok := params["search"]
	if !ok {
		t.Fatalf("missing search param")
	}
	if value, _ := first["value"].(string); value != "foo" {
		t.Fatalf("unexpected search value %q", value)
	}
	if enabled, _ := first["enabled"].(bool); !enabled {
		t.Fatalf("expected enabled true for search param")
	}

	if _, ok := params["page"]; !ok {
		t.Fatalf("missing page param")
	}
}

func TestConvertRequestNodeCleanAddsFormBodyOverride(t *testing.T) {
	flowID := idwrap.NewNow()
	nodeID := idwrap.NewNow()
	endpointID := idwrap.NewNow()
	deltaEndpointID := idwrap.NewNow()
	exampleID := idwrap.NewNow()
	deltaExampleID := idwrap.NewNow()

	workspace := &ioworkspace.WorkspaceData{
		Workspace: mworkspace.Workspace{ID: idwrap.NewNow(), Name: "workspace"},
		Flows:     []mflow.Flow{{ID: flowID, Name: "flow"}},
		FlowNodes: []mnnode.MNode{{ID: nodeID, FlowID: flowID, Name: "node", NodeKind: mnnode.NODE_KIND_REQUEST}},
		FlowRequestNodes: []mnrequest.MNRequest{{
			FlowNodeID:       nodeID,
			EndpointID:       &endpointID,
			ExampleID:        &exampleID,
			DeltaEndpointID:  &deltaEndpointID,
			DeltaExampleID:   &deltaExampleID,
			HasRequestConfig: true,
		}},
		Endpoints: []mitemapi.ItemApi{
			{ID: endpointID, Name: "node", Method: "POST", Url: "https://example.dev"},
			{ID: deltaEndpointID, Name: "node (delta)", Method: "POST", Url: "https://example.dev", DeltaParentID: &endpointID},
		},
		Examples: []mitemapiexample.ItemApiExample{
			{ID: exampleID, ItemApiID: endpointID, Name: "node", BodyType: mitemapiexample.BodyTypeForm},
			{ID: deltaExampleID, ItemApiID: deltaEndpointID, Name: "node (delta)", BodyType: mitemapiexample.BodyTypeForm, VersionParentID: &exampleID},
		},
		FormBodies: []mbodyform.BodyForm{
			{ID: idwrap.NewNow(), ExampleID: deltaExampleID, BodyKey: "file", Value: "{{ #file:./data.bin }}", Enable: true},
		},
	}

	node := workspace.FlowNodes[0]
	incoming := map[idwrap.IDWrap][]edge.Edge{}
	outgoing := map[idwrap.IDWrap][]edge.Edge{}
	nodeMap := map[idwrap.IDWrap]mnnode.MNode{node.ID: node}
	nodeToRequest := map[string]string{node.Name: node.Name}

	step, err := convertRequestNodeClean(context.Background(), node, incoming, outgoing, idwrap.IDWrap{}, nodeMap, workspace, nodeToRequest, nil)
	if err != nil {
		t.Fatalf("convertRequestNodeClean returned error: %v", err)
	}

	requestStep, ok := step["request"].(map[string]any)
	if !ok {
		t.Fatalf("expected request step, got %T", step["request"])
	}

	body, ok := requestStep["body"].(map[string]any)
	if !ok {
		t.Fatalf("expected map body entry, got %T", requestStep["body"])
	}
	if bodyType, _ := body["type"].(string); bodyType != "form-data" {
		t.Fatalf("expected form-data type, got %q", bodyType)
	}

	itemsAny, ok := body["items"].([]map[string]any)
	if !ok || len(itemsAny) != 1 {
		t.Fatalf("expected single form item slice, got %#v", body["items"])
	}
	if name, _ := itemsAny[0]["name"].(string); name != "file" {
		t.Fatalf("unexpected form item name %q", name)
	}
	if value, _ := itemsAny[0]["value"].(string); value != "{{ #file:./data.bin }}" {
		t.Fatalf("unexpected form item value %q", value)
	}
}

func TestConvertRequestNodeCleanAddsUrlEncodedOverride(t *testing.T) {
	flowID := idwrap.NewNow()
	nodeID := idwrap.NewNow()
	endpointID := idwrap.NewNow()
	deltaEndpointID := idwrap.NewNow()
	exampleID := idwrap.NewNow()
	deltaExampleID := idwrap.NewNow()

	workspace := &ioworkspace.WorkspaceData{
		Workspace: mworkspace.Workspace{ID: idwrap.NewNow(), Name: "workspace"},
		Flows:     []mflow.Flow{{ID: flowID, Name: "flow"}},
		FlowNodes: []mnnode.MNode{{ID: nodeID, FlowID: flowID, Name: "node", NodeKind: mnnode.NODE_KIND_REQUEST}},
		FlowRequestNodes: []mnrequest.MNRequest{{
			FlowNodeID:       nodeID,
			EndpointID:       &endpointID,
			ExampleID:        &exampleID,
			DeltaEndpointID:  &deltaEndpointID,
			DeltaExampleID:   &deltaExampleID,
			HasRequestConfig: true,
		}},
		Endpoints: []mitemapi.ItemApi{
			{ID: endpointID, Name: "node", Method: "POST", Url: "https://example.dev"},
			{ID: deltaEndpointID, Name: "node (delta)", Method: "POST", Url: "https://example.dev", DeltaParentID: &endpointID},
		},
		Examples: []mitemapiexample.ItemApiExample{
			{ID: exampleID, ItemApiID: endpointID, Name: "node", BodyType: mitemapiexample.BodyTypeUrlencoded},
			{ID: deltaExampleID, ItemApiID: deltaEndpointID, Name: "node (delta)", BodyType: mitemapiexample.BodyTypeUrlencoded, VersionParentID: &exampleID},
		},
		UrlBodies: []mbodyurl.BodyURLEncoded{
			{ID: idwrap.NewNow(), ExampleID: deltaExampleID, BodyKey: "search", Value: "query", Enable: true},
		},
	}

	node := workspace.FlowNodes[0]
	incoming := map[idwrap.IDWrap][]edge.Edge{}
	outgoing := map[idwrap.IDWrap][]edge.Edge{}
	nodeMap := map[idwrap.IDWrap]mnnode.MNode{node.ID: node}
	nodeToRequest := map[string]string{node.Name: node.Name}

	step, err := convertRequestNodeClean(context.Background(), node, incoming, outgoing, idwrap.IDWrap{}, nodeMap, workspace, nodeToRequest, nil)
	if err != nil {
		t.Fatalf("convertRequestNodeClean returned error: %v", err)
	}

	requestStep, ok := step["request"].(map[string]any)
	if !ok {
		t.Fatalf("expected request step, got %T", step["request"])
	}

	body, ok := requestStep["body"].(map[string]any)
	if !ok {
		t.Fatalf("expected map body entry, got %T", requestStep["body"])
	}
	if bodyType, _ := body["type"].(string); bodyType != "x-www-form-urlencoded" {
		t.Fatalf("expected x-www-form-urlencoded type, got %q", bodyType)
	}

	itemsAny, ok := body["items"].([]map[string]any)
	if !ok || len(itemsAny) != 1 {
		t.Fatalf("expected single urlencoded item slice, got %#v", body["items"])
	}
	if name, _ := itemsAny[0]["name"].(string); name != "search" {
		t.Fatalf("unexpected urlencoded item name %q", name)
	}
	if value, _ := itemsAny[0]["value"].(string); value != "query" {
		t.Fatalf("unexpected urlencoded item value %q", value)
	}
}

func TestImportYamlFlowWithTypedBodies(t *testing.T) {
	workspace, err := ImportYamlFlowYAML([]byte(bodyTypesYAML))
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}

	findNodeName := func(id idwrap.IDWrap) string {
		for _, node := range workspace.FlowNodes {
			if node.ID == id {
				return node.Name
			}
		}
		return ""
	}

	findExample := func(id idwrap.IDWrap) *mitemapiexample.ItemApiExample {
		for i := range workspace.Examples {
			if workspace.Examples[i].ID == id {
				return &workspace.Examples[i]
			}
		}
		return nil
	}

	var uploadExampleID, uploadDeltaID idwrap.IDWrap
	var searchExampleID, searchDeltaID idwrap.IDWrap
	for _, reqNode := range workspace.FlowRequestNodes {
		name := findNodeName(reqNode.FlowNodeID)
		switch name {
		case "UploadStep":
			if reqNode.ExampleID != nil {
				uploadExampleID = *reqNode.ExampleID
			}
			if reqNode.DeltaExampleID != nil {
				uploadDeltaID = *reqNode.DeltaExampleID
			}
		case "SearchStep":
			if reqNode.ExampleID != nil {
				searchExampleID = *reqNode.ExampleID
			}
			if reqNode.DeltaExampleID != nil {
				searchDeltaID = *reqNode.DeltaExampleID
			}
		}
	}

	if uploadExampleID == (idwrap.IDWrap{}) || uploadDeltaID == (idwrap.IDWrap{}) {
		t.Fatalf("missing upload example identifiers")
	}
	if searchExampleID == (idwrap.IDWrap{}) || searchDeltaID == (idwrap.IDWrap{}) {
		t.Fatalf("missing search example identifiers")
	}

	uploadExample := findExample(uploadExampleID)
	uploadDelta := findExample(uploadDeltaID)
	if uploadExample == nil || uploadDelta == nil {
		t.Fatalf("missing upload examples")
	}
	if uploadExample.BodyType != mitemapiexample.BodyTypeForm || uploadDelta.BodyType != mitemapiexample.BodyTypeForm {
		t.Fatalf("expected upload examples to use form body type")
	}

	searchExample := findExample(searchExampleID)
	searchDelta := findExample(searchDeltaID)
	if searchExample == nil || searchDelta == nil {
		t.Fatalf("missing search examples")
	}
	if searchExample.BodyType != mitemapiexample.BodyTypeUrlencoded || searchDelta.BodyType != mitemapiexample.BodyTypeUrlencoded {
		t.Fatalf("expected search examples to use urlencoded body type")
	}

	var foundForm bool
	for _, form := range workspace.FormBodies {
		if form.ExampleID == uploadExampleID && form.Value == "{{ #file:./payload.bin }}" {
			foundForm = true
			break
		}
	}
	if !foundForm {
		t.Fatalf("expected form body entry for upload example")
	}

	var searchParams int
	for _, param := range workspace.UrlBodies {
		if param.ExampleID == searchExampleID {
			searchParams++
		}
	}
	if searchParams == 0 {
		t.Fatalf("expected urlencoded parameters for search example")
	}

	var rawFormFallback bool
	for _, raw := range workspace.Rawbodies {
		if raw.ExampleID == uploadExampleID || raw.ExampleID == searchExampleID {
			rawFormFallback = true
		}
	}
	if !rawFormFallback {
		t.Fatalf("expected raw fallback body for typed example")
	}

	exported, err := ExportYamlFlowYAML(context.Background(), workspace, nil)
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}

	var doc map[string]any
	if err := yaml.Unmarshal(exported, &doc); err != nil {
		t.Fatalf("failed to unmarshal exported yaml: %v", err)
	}

	requestsAny, ok := doc["requests"].([]any)
	if !ok {
		t.Fatalf("expected requests array in export")
	}
	var formRequest map[string]any
	for _, reqAny := range requestsAny {
		if reqMap, ok := reqAny.(map[string]any); ok {
			if name, _ := reqMap["name"].(string); name == "UploadStep" {
				formRequest = reqMap
				break
			}
		}
	}
	if formRequest == nil {
		t.Fatalf("UploadStep request definition not found in export")
	}
	body, ok := formRequest["body"].(map[string]any)
	if !ok {
		t.Fatalf("expected structured body for form-template request")
	}
	if bodyType, _ := body["type"].(string); bodyType != "form-data" {
		t.Fatalf("expected form-data type in exported request, got %q", bodyType)
	}

	flowsAny, ok := doc["flows"].([]any)
	if !ok || len(flowsAny) == 0 {
		t.Fatalf("exported yaml missing flows section")
	}
	flowMap, ok := flowsAny[0].(map[string]any)
	if !ok {
		t.Fatalf("expected flow map in export")
	}
	stepsAny, ok := flowMap["steps"].([]any)
	if !ok {
		t.Fatalf("expected steps array in export flow")
	}
	var searchStep map[string]any
	for _, stepAny := range stepsAny {
		if stepMap, ok := stepAny.(map[string]any); ok {
			if req, ok := stepMap["request"].(map[string]any); ok {
				if name, _ := req["name"].(string); name == "SearchStep" {
					searchStep = req
					break
				}
			}
		}
	}
	if searchStep == nil {
		t.Fatalf("search step not found in export")
	}
	stepBody, ok := searchStep["body"].(map[string]any)
	if !ok {
		t.Fatalf("expected body on search step override")
	}
	if bodyType, _ := stepBody["type"].(string); bodyType != "x-www-form-urlencoded" {
		t.Fatalf("expected urlencoded type in search step, got %q", bodyType)
	}

	reimported, err := ImportYamlFlowYAML(exported)
	if err != nil {
		t.Fatalf("re-import failed: %v", err)
	}
	var reimportFoundForm bool
	for _, form := range reimported.FormBodies {
		if form.Value == "{{ #file:./payload.bin }}" {
			reimportFoundForm = true
			break
		}
	}
	if !reimportFoundForm {
		t.Fatalf("expected form body to round-trip through export/import")
	}
	var reimportFoundParam bool
	for _, param := range reimported.UrlBodies {
		if param.BodyKey == "q" && param.Value == "search" {
			reimportFoundParam = true
			break
		}
	}
	if !reimportFoundParam {
		t.Fatalf("expected urlencoded param to round-trip through export/import")
	}
}

func TestExportYamlFlowYAMLSkipsOrphanNodes(t *testing.T) {
	flowID := idwrap.NewNow()
	startID := idwrap.NewNow()
	connectedID := idwrap.NewNow()
	orphanID := idwrap.NewNow()
	endpointConnectedID := idwrap.NewNow()
	exampleConnectedID := idwrap.NewNow()
	endpointOrphanID := idwrap.NewNow()
	exampleOrphanID := idwrap.NewNow()

	workspace := &ioworkspace.WorkspaceData{
		Workspace: mworkspace.Workspace{ID: idwrap.NewNow(), Name: "workspace"},
		Flows: []mflow.Flow{
			{ID: flowID, Name: "flow"},
		},
		FlowNodes: []mnnode.MNode{
			{ID: startID, FlowID: flowID, Name: "Start", NodeKind: mnnode.NODE_KIND_NO_OP},
			{ID: connectedID, FlowID: flowID, Name: "connected", NodeKind: mnnode.NODE_KIND_REQUEST},
			{ID: orphanID, FlowID: flowID, Name: "orphan", NodeKind: mnnode.NODE_KIND_REQUEST},
		},
		FlowNoopNodes: []mnnoop.NoopNode{
			{FlowNodeID: startID, Type: mnnoop.NODE_NO_OP_KIND_START},
		},
		FlowRequestNodes: []mnrequest.MNRequest{
			{FlowNodeID: connectedID, EndpointID: &endpointConnectedID, ExampleID: &exampleConnectedID},
			{FlowNodeID: orphanID, EndpointID: &endpointOrphanID, ExampleID: &exampleOrphanID},
		},
		FlowEdges: []edge.Edge{
			{FlowID: flowID, SourceID: startID, TargetID: connectedID},
		},
		Endpoints: []mitemapi.ItemApi{
			{ID: endpointConnectedID, Name: "connected", Method: "GET", Url: "https://example.dev/connected"},
			{ID: endpointOrphanID, Name: "orphan", Method: "GET", Url: "https://example.dev/orphan"},
		},
		Examples: []mitemapiexample.ItemApiExample{
			{ID: exampleConnectedID, ItemApiID: endpointConnectedID, Name: "connected"},
			{ID: exampleOrphanID, ItemApiID: endpointOrphanID, Name: "orphan"},
		},
	}

	data, err := ExportYamlFlowYAML(context.Background(), workspace, nil)
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}

	var exported map[string]any
	if err := yaml.Unmarshal(data, &exported); err != nil {
		t.Fatalf("failed to unmarshal export: %v", err)
	}

	flowsAny, ok := exported["flows"].([]any)
	if !ok || len(flowsAny) != 1 {
		t.Fatalf("expected a single exported flow, got %v", exported["flows"])
	}

	flowMap, ok := flowsAny[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected flow entry type %T", flowsAny[0])
	}

	stepsAny, ok := flowMap["steps"].([]any)
	if !ok {
		t.Fatalf("expected steps in exported flow, got %v", flowMap["steps"])
	}

	var names []string
	for _, stepAny := range stepsAny {
		stepMap, ok := stepAny.(map[string]any)
		if !ok {
			continue
		}
		requestData, ok := stepMap["request"].(map[string]any)
		if !ok {
			continue
		}
		name, _ := requestData["name"].(string)
		names = append(names, name)
	}

	if len(names) != 1 {
		t.Fatalf("expected exactly one exported step, got %v", names)
	}
	if names[0] != "connected" {
		t.Fatalf("expected step for connected node, got %q", names[0])
	}
	for _, name := range names {
		if name == "orphan" {
			t.Fatalf("orphan node should not be exported: %v", names)
		}
	}

	requestsAny, ok := exported["requests"].([]any)
	if !ok {
		t.Fatalf("expected requests in export, got %T", exported["requests"])
	}
	var requestNames []string
	for _, reqAny := range requestsAny {
		reqMap, ok := reqAny.(map[string]any)
		if !ok {
			continue
		}
		name, _ := reqMap["name"].(string)
		requestNames = append(requestNames, name)
	}
	if len(requestNames) != 1 || requestNames[0] != "connected" {
		t.Fatalf("expected only connected request definition, got %v", requestNames)
	}
}
