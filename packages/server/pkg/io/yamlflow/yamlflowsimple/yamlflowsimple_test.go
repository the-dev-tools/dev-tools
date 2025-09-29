package yamlflowsimple

import (
	"context"
	"testing"

	"gopkg.in/yaml.v3"

	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/ioworkspace"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
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
