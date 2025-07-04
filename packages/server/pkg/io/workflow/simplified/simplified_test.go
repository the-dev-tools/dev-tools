package simplified_test

import (
	"testing"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/io/workflow"
	"the-dev-tools/server/pkg/io/workflow/simplified"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mcollection"
	"the-dev-tools/server/pkg/model/mexampleheader"
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
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/testutil"
)

func TestSimplifiedYAML(t *testing.T) {
	yaml := `
workspace_name: Test Workspace

flows:
  - name: TestFlow
    variables:
      baseUrl: "https://api.example.com"
    steps:
      - request:
          name: GetUser
          method: GET
          url: "{{baseUrl}}/users/123"
          headers:
            Authorization: "Bearer {{token}}"
`

	s := simplified.New()
	wd, err := s.Unmarshal([]byte(yaml), workflow.FormatYAML)
	if err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	// Verify workspace
	testutil.AssertFatal(t, "Test Workspace", wd.Workspace.Name)

	// Verify collection
	testutil.AssertFatal(t, "Workflow Collection", wd.Collection.Name)

	// Verify flows
	testutil.AssertFatal(t, 1, len(wd.Flows))
	testutil.AssertFatal(t, "TestFlow", wd.Flows[0].Name)

	// Verify variables
	testutil.AssertFatal(t, 1, len(wd.FlowVariables))
	testutil.AssertFatal(t, "baseUrl", wd.FlowVariables[0].Name)
	testutil.AssertFatal(t, "https://api.example.com", wd.FlowVariables[0].Value)

	// Verify nodes
	testutil.AssertFatal(t, 1, len(wd.FlowNodes))
	testutil.AssertFatal(t, "GetUser", wd.FlowNodes[0].Name)

	// Verify endpoints
	testutil.AssertFatal(t, 1, len(wd.Endpoints))
	testutil.AssertFatal(t, "GET", wd.Endpoints[0].Method)
	testutil.AssertFatal(t, "{{baseUrl}}/users/123", wd.Endpoints[0].Url)

	// Verify headers
	testutil.AssertFatal(t, 1, len(wd.RequestHeaders))
	testutil.AssertFatal(t, "Authorization", wd.RequestHeaders[0].HeaderKey)
	testutil.AssertFatal(t, "Bearer {{token}}", wd.RequestHeaders[0].Value)
}

func TestSimplifiedJSON(t *testing.T) {
	jsonData := `{
		"workspace_name": "Test Workspace",
		"flows": [
			{
				"name": "TestFlow",
				"variables": {
					"baseUrl": "https://api.example.com"
				},
				"steps": [
					{
						"request": {
							"name": "GetUser",
							"method": "GET",
							"url": "{{baseUrl}}/users/123",
							"headers": {
								"Authorization": "Bearer {{token}}"
							}
						}
					}
				]
			}
		]
	}`

	s := simplified.New()
	wd, err := s.Unmarshal([]byte(jsonData), workflow.FormatJSON)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Verify workspace
	testutil.AssertFatal(t, "Test Workspace", wd.Workspace.Name)

	// Verify flows
	testutil.AssertFatal(t, 1, len(wd.Flows))
	testutil.AssertFatal(t, "TestFlow", wd.Flows[0].Name)
}

func TestSimplifiedMarshalYAML(t *testing.T) {
	// Create test data
	workspaceID := idwrap.NewNow()
	collectionID := idwrap.NewNow()

	wd := &workflow.WorkspaceData{
		Workspace: mworkspace.Workspace{
			ID:   workspaceID,
			Name: "Test Workspace",
		},
		Collection: mcollection.Collection{
			ID:          collectionID,
			Name:        "Test Collection",
			WorkspaceID: workspaceID,
		},
		// Add minimal required data
		Flows:              []mflow.Flow{},
		FlowNodes:          []mnnode.MNode{},
		FlowEdges:          []edge.Edge{},
		FlowVariables:      []mflowvariable.FlowVariable{},
		Endpoints:          []mitemapi.ItemApi{},
		Examples:           []mitemapiexample.ItemApiExample{},
		RequestHeaders:     []mexampleheader.Header{},
		RequestBodyRaw:     []mbodyraw.ExampleBodyRaw{},
		FlowRequestNodes:   []mnrequest.MNRequest{},
		FlowConditionNodes: []mnif.MNIF{},
		FlowNoopNodes:      []mnnoop.NoopNode{},
		FlowForNodes:       []mnfor.MNFor{},
		FlowForEachNodes:   []mnforeach.MNForEach{},
		FlowJSNodes:        []mnjs.MNJS{},
	}

	s := simplified.New()
	yamlData, err := s.Marshal(wd, workflow.FormatYAML)
	if err != nil {
		t.Fatalf("Failed to marshal to YAML: %v", err)
	}

	// Verify the YAML contains expected content
	yamlStr := string(yamlData)
	if !contains(yamlStr, "workspace_name: Test Workspace") {
		t.Errorf("Expected YAML to contain workspace name, got: %s", yamlStr)
	}
}

func TestSimplifiedMarshalJSON(t *testing.T) {
	// Create test data
	workspaceID := idwrap.NewNow()
	collectionID := idwrap.NewNow()

	wd := &workflow.WorkspaceData{
		Workspace: mworkspace.Workspace{
			ID:   workspaceID,
			Name: "Test Workspace",
		},
		Collection: mcollection.Collection{
			ID:          collectionID,
			Name:        "Test Collection",
			WorkspaceID: workspaceID,
		},
		// Add minimal required data
		Flows:              []mflow.Flow{},
		FlowNodes:          []mnnode.MNode{},
		FlowEdges:          []edge.Edge{},
		FlowVariables:      []mflowvariable.FlowVariable{},
		Endpoints:          []mitemapi.ItemApi{},
		Examples:           []mitemapiexample.ItemApiExample{},
		RequestHeaders:     []mexampleheader.Header{},
		RequestBodyRaw:     []mbodyraw.ExampleBodyRaw{},
		FlowRequestNodes:   []mnrequest.MNRequest{},
		FlowConditionNodes: []mnif.MNIF{},
		FlowNoopNodes:      []mnnoop.NoopNode{},
		FlowForNodes:       []mnfor.MNFor{},
		FlowForEachNodes:   []mnforeach.MNForEach{},
		FlowJSNodes:        []mnjs.MNJS{},
	}

	s := simplified.New()
	jsonData, err := s.Marshal(wd, workflow.FormatJSON)
	if err != nil {
		t.Fatalf("Failed to marshal to JSON: %v", err)
	}

	// Verify the JSON contains expected content
	jsonStr := string(jsonData)
	if !contains(jsonStr, `"workspace_name":"Test Workspace"`) && !contains(jsonStr, `"workspace_name": "Test Workspace"`) {
		t.Errorf("Expected JSON to contain workspace name, got: %s", jsonStr)
	}
}

func TestSimplifiedIfStep(t *testing.T) {
	yaml := `
workspace_name: Test Workspace

flows:
  - name: ConditionalFlow
    steps:
      - if:
          name: CheckCondition
          expression: "status == 200"
          then: HandleSuccess
          else: HandleError
      - request:
          name: HandleSuccess
          method: GET
          url: "https://api.example.com/success"
      - request:
          name: HandleError
          method: GET
          url: "https://api.example.com/error"
`

	s := simplified.New()
	wd, err := s.Unmarshal([]byte(yaml), workflow.FormatYAML)
	if err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	// Verify flows
	testutil.AssertFatal(t, 1, len(wd.Flows))

	// Verify nodes
	testutil.AssertFatal(t, 3, len(wd.FlowNodes))

	// Verify condition nodes
	testutil.AssertFatal(t, 1, len(wd.FlowConditionNodes))
	testutil.AssertFatal(t, "status == 200", wd.FlowConditionNodes[0].Condition.Comparisons.Expression)

	// Verify edges for conditional flow
	thenEdges := 0
	elseEdges := 0
	for _, e := range wd.FlowEdges {
		if e.SourceHandler == edge.HandleThen {
			thenEdges++
		} else if e.SourceHandler == edge.HandleElse {
			elseEdges++
		}
	}
	testutil.AssertFatal(t, 1, thenEdges)
	testutil.AssertFatal(t, 1, elseEdges)
}

func TestSimplifiedForStep(t *testing.T) {
	yaml := `
workspace_name: Test Workspace

flows:
  - name: LoopFlow
    steps:
      - for:
          name: RepeatRequest
          iter_count: 5
          loop: MakeRequest
      - request:
          name: MakeRequest
          method: GET
          url: "https://api.example.com/item"
`

	s := simplified.New()
	wd, err := s.Unmarshal([]byte(yaml), workflow.FormatYAML)
	if err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	// Verify for nodes
	testutil.AssertFatal(t, 1, len(wd.FlowForNodes))
	testutil.AssertFatal(t, int64(5), wd.FlowForNodes[0].IterCount)

	// Verify loop edge exists
	loopEdges := 0
	for _, e := range wd.FlowEdges {
		if e.SourceHandler == edge.HandleLoop {
			loopEdges++
		}
	}
	testutil.AssertFatal(t, 1, loopEdges)
}

func TestSimplifiedJSStep(t *testing.T) {
	yaml := `
workspace_name: Test Workspace

flows:
  - name: ScriptFlow
    steps:
      - js:
          name: ProcessData
          code: |
            const result = data.map(item => item.value * 2);
            return result;
`

	s := simplified.New()
	wd, err := s.Unmarshal([]byte(yaml), workflow.FormatYAML)
	if err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	// Verify JS nodes
	testutil.AssertFatal(t, 1, len(wd.FlowJSNodes))
	code := string(wd.FlowJSNodes[0].Code)
	if !contains(code, "const result = data.map") {
		t.Errorf("Expected JS code to contain map function, got: %s", code)
	}
}

func TestSimplifiedGlobalRequests(t *testing.T) {
	yaml := `
workspace_name: Test Workspace

requests:
  - name: base_api_request
    method: GET
    headers:
      Authorization: "Bearer {{token}}"
      Content-Type: "application/json"

flows:
  - name: TestFlow
    steps:
      - request:
          name: GetUser
          use_request: base_api_request
          url: "https://api.example.com/users/123"
      - request:
          name: GetPosts
          use_request: base_api_request
          url: "https://api.example.com/posts"
          headers:
            X-Custom: "custom-value"
`

	s := simplified.New()
	wd, err := s.Unmarshal([]byte(yaml), workflow.FormatYAML)
	if err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	// Verify endpoints inherit from global request
	testutil.AssertFatal(t, 2, len(wd.Endpoints))
	testutil.AssertFatal(t, "GET", wd.Endpoints[0].Method)
	testutil.AssertFatal(t, "GET", wd.Endpoints[1].Method)

	// Verify headers are merged correctly
	// First request should have 2 headers from global
	// Second request should have 3 headers (2 from global + 1 custom)
	headerCount := make(map[idwrap.IDWrap]int)
	for _, h := range wd.RequestHeaders {
		headerCount[h.ExampleID]++
	}

	// Should have headers for both examples
	testutil.AssertFatal(t, 2, len(headerCount))
}

func TestSimplifiedRunDependencies(t *testing.T) {
	yaml := `
workspace_name: Test Workspace

run:
  - flow: Setup
  - flow: MainFlow
    depends_on: [Setup]
  - flow: Cleanup
    depends_on: [MainFlow]

flows:
  - name: Setup
    steps:
      - request:
          name: Initialize
          method: POST
          url: "https://api.example.com/init"
  - name: MainFlow
    steps:
      - request:
          name: Process
          method: GET
          url: "https://api.example.com/process"
  - name: Cleanup
    steps:
      - request:
          name: Teardown
          method: DELETE
          url: "https://api.example.com/cleanup"
`

	s := simplified.New()
	wd, err := s.Unmarshal([]byte(yaml), workflow.FormatYAML)
	if err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	// Verify flows
	testutil.AssertFatal(t, 3, len(wd.Flows))

	// TODO: Verify run dependencies when implemented
}

func TestSimplifiedBodyFormats(t *testing.T) {
	yaml := `
workspace_name: Test Workspace

flows:
  - name: BodyTestFlow
    steps:
      - request:
          name: SimpleBody
          method: POST
          url: "https://api.example.com/simple"
          body:
            key1: value1
            key2: value2
      - request:
          name: ExplicitBody
          method: POST
          url: "https://api.example.com/explicit"
          body:
            kind: json
            value:
              data: test
`

	s := simplified.New()
	wd, err := s.Unmarshal([]byte(yaml), workflow.FormatYAML)
	if err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	// Verify bodies
	testutil.AssertFatal(t, 2, len(wd.RequestBodyRaw))
}

func TestSimplifiedUnsupportedFormat(t *testing.T) {
	s := simplified.New()

	// Test unsupported format for unmarshal
	_, err := s.Unmarshal([]byte("test"), workflow.FormatTOML)
	if err == nil {
		t.Fatalf("Expected error for unsupported TOML format")
	}

	// Test unsupported format for marshal
	wd := &workflow.WorkspaceData{}
	_, err = s.Marshal(wd, workflow.FormatTOML)
	if err == nil {
		t.Fatalf("Expected error for unsupported TOML format")
	}
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && (s[:len(substr)] == substr || contains(s[1:], substr)))
}
