package workflowsimple_test

import (
	"strings"
	"testing"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/io/workflow/workflowsimple"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"

	"github.com/stretchr/testify/require"
)

func TestParseSimpleWorkflow(t *testing.T) {
	yamlData := `
workspace_name: Test Workspace

flows:
  - name: SimpleFlow
    variables:
      - name: base_url
        value: "https://api.example.com"
      - name: token
        value: "bearer123"
    steps:
      - request:
          name: GetUser
          url: "{{base_url}}/users/1"
          method: GET
          headers:
            Authorization: "Bearer {{token}}"
`

	data, err := workflowsimple.Parse([]byte(yamlData))
	require.NoError(t, err)

	// Verify flow
	require.Equal(t, "SimpleFlow", data.Flow.Name)

	// Verify variables
	require.Len(t, data.Variables, 2)
	varMap := make(map[string]string)
	for _, v := range data.Variables {
		varMap[v.VarKey] = v.Value
	}
	require.Equal(t, "https://api.example.com", varMap["base_url"])
	require.Equal(t, "bearer123", varMap["token"])

	// Verify nodes
	require.Len(t, data.Nodes, 2) // Start + GetUser
	requestNodes := 0
	startNodes := 0
	for _, node := range data.Nodes {
		switch node.NodeKind {
		case mnnode.NODE_KIND_REQUEST:
			requestNodes++
			require.Equal(t, "GetUser", node.Name)
		case mnnode.NODE_KIND_NO_OP:
			startNodes++
			require.Equal(t, "Start", node.Name)
		}
	}
	require.Equal(t, 1, requestNodes)
	require.Equal(t, 1, startNodes)

	// Verify endpoints and examples
	require.Len(t, data.Endpoints, 2) // base + delta
	require.Len(t, data.Examples, 3) // base + default + delta

	// Verify base endpoint
	baseEndpoint := data.Endpoints[0]
	require.Equal(t, "GetUser", baseEndpoint.Name)
	require.Equal(t, "{{base_url}}/users/1", baseEndpoint.Url)
	require.Equal(t, "GET", baseEndpoint.Method)
	require.Nil(t, baseEndpoint.DeltaParentID)
	require.False(t, baseEndpoint.Hidden)

	// Verify delta endpoint
	deltaEndpoint := data.Endpoints[1]
	require.Equal(t, "GetUser (delta)", deltaEndpoint.Name)
	require.Equal(t, "{{base_url}}/users/1", deltaEndpoint.Url) // keeps variables
	require.Equal(t, "GET", deltaEndpoint.Method)
	require.NotNil(t, deltaEndpoint.DeltaParentID)
	require.Equal(t, baseEndpoint.ID, *deltaEndpoint.DeltaParentID)
	require.True(t, deltaEndpoint.Hidden)

	// Verify headers
	require.Greater(t, len(data.Headers), 0)
	
	// Check for resolved header values in delta
	foundResolvedHeader := false
	for _, h := range data.Headers {
		if h.HeaderKey == "Authorization" && h.Value == "Bearer bearer123" {
			foundResolvedHeader = true
			break
		}
	}
	require.True(t, foundResolvedHeader, "Should have resolved authorization header")

	// Verify edges
	require.Len(t, data.Edges, 1)
	require.Equal(t, edge.HandleUnspecified, data.Edges[0].SourceHandler)
}

func TestParseRequestTemplates(t *testing.T) {
	yamlData := `
workspace_name: Test Templates

request_templates:
  api_base:
    method: POST
    url: "{{base_url}}/api/v1"
    headers:
      Content-Type: application/json
    body:
      body_json:
        version: 1

flows:
  - name: TemplateFlow
    variables:
      - name: base_url
        value: "https://example.com"
    steps:
      - request:
          name: UseTemplate
          use_request: api_base
          headers:
            X-Custom: custom-value
`

	data, err := workflowsimple.Parse([]byte(yamlData))
	require.NoError(t, err)

	// Find the request endpoint
	var endpoint *mitemapi.ItemApi
	for i := range data.Endpoints {
		if data.Endpoints[i].Name == "UseTemplate" {
			endpoint = &data.Endpoints[i]
			break
		}
	}
	require.NotNil(t, endpoint)

	// Verify template values were used
	require.Equal(t, "POST", endpoint.Method)
	require.Equal(t, "{{base_url}}/api/v1", endpoint.Url)

	// Verify headers include both template and override
	headerMap := make(map[string]int)
	for _, h := range data.Headers {
		if h.ExampleID == data.Examples[0].ID {
			headerMap[h.HeaderKey]++
		}
	}
	require.Equal(t, 1, headerMap["Content-Type"])
	require.Equal(t, 1, headerMap["X-Custom"])

	// Verify body was included
	require.Greater(t, len(data.RawBodies), 0)
}

func TestParseControlFlowNodes(t *testing.T) {
	yamlData := `
workspace_name: Control Flow Test

flows:
  - name: ControlFlow
    steps:
      - request:
          name: InitialRequest
          url: "https://api.example.com/data"
          method: GET
      
      - if:
          name: CheckStatus
          condition: "InitialRequest.response.status == 200"
          then: ProcessSuccess
          else: HandleError
      
      - request:
          name: ProcessSuccess
          url: "https://api.example.com/success"
          method: POST
      
      - js:
          name: HandleError
          code: |
            console.error("Request failed");
            return { error: true };
      
      - for:
          name: ProcessItems
          iter_count: 3
          loop: ProcessItem
          depends_on:
            - ProcessSuccess
      
      - request:
          name: ProcessItem
          url: "https://api.example.com/item"
          method: GET
      
      - for_each:
          name: ProcessArray
          items: "response.items"
          loop: ProcessArrayItem
          depends_on:
            - ProcessItems
      
      - js:
          name: ProcessArrayItem
          code: "return item;"
`

	data, err := workflowsimple.Parse([]byte(yamlData))
	require.NoError(t, err)

	// Verify node counts
	nodeTypes := make(map[mnnode.NodeKind]int)
	nodeNames := make(map[string]idwrap.IDWrap)
	for _, node := range data.Nodes {
		nodeTypes[node.NodeKind]++
		nodeNames[node.Name] = node.ID
	}

	require.Equal(t, 3, nodeTypes[mnnode.NODE_KIND_REQUEST])
	require.Equal(t, 1, nodeTypes[mnnode.NODE_KIND_CONDITION])
	require.Equal(t, 2, nodeTypes[mnnode.NODE_KIND_JS])
	require.Equal(t, 1, nodeTypes[mnnode.NODE_KIND_FOR])
	require.Equal(t, 1, nodeTypes[mnnode.NODE_KIND_FOR_EACH])
	require.Greater(t, nodeTypes[mnnode.NODE_KIND_NO_OP], 0) // At least Start node

	// Verify condition node
	require.Len(t, data.ConditionNodes, 1)
	condNode := data.ConditionNodes[0]
	require.Equal(t, "InitialRequest.response.status == 200", condNode.Condition.Comparisons.Expression)

	// Verify for node
	require.Len(t, data.ForNodes, 2) // for and for_each
	forNode := data.ForNodes[0]
	require.Equal(t, int64(3), forNode.IterCount)

	// Verify JS nodes
	require.Len(t, data.JSNodes, 2)

	// Verify edges for control flow
	edgeMap := make(map[edge.EdgeHandle]int)
	for _, e := range data.Edges {
		edgeMap[e.SourceHandler]++
	}
	require.Greater(t, edgeMap[edge.HandleThen], 0)
	require.Greater(t, edgeMap[edge.HandleElse], 0)
	require.Greater(t, edgeMap[edge.HandleLoop], 0)
}

func TestParseDependencies(t *testing.T) {
	yamlData := `
workspace_name: Dependency Test

flows:
  - name: DependencyFlow
    steps:
      - request:
          name: Step1
          url: "https://api.example.com/1"
          method: GET
      
      - request:
          name: Step2
          url: "https://api.example.com/2"
          method: GET
      
      - request:
          name: Step3
          url: "https://api.example.com/3"
          method: GET
          depends_on:
            - Step1
            - Step2
      
      - request:
          name: Step4
          url: "https://api.example.com/4"
          method: GET
          depends_on:
            - Step3
`

	data, err := workflowsimple.Parse([]byte(yamlData))
	require.NoError(t, err)

	// Build node name to ID map
	nodeMap := make(map[string]idwrap.IDWrap)
	for _, node := range data.Nodes {
		nodeMap[node.Name] = node.ID
	}

	// Build edge map
	edges := make(map[string][]string)
	for _, e := range data.Edges {
		sourceName := ""
		targetName := ""
		for name, id := range nodeMap {
			if id == e.SourceID {
				sourceName = name
			}
			if id == e.TargetID {
				targetName = name
			}
		}
		edges[sourceName] = append(edges[sourceName], targetName)
	}

	// Verify dependencies
	require.Contains(t, edges["Step1"], "Step3")
	require.Contains(t, edges["Step2"], "Step3")
	require.Contains(t, edges["Step3"], "Step4")

	// Verify implicit sequential edge
	require.Contains(t, edges["Step1"], "Step2")

	// Verify start node connections
	require.Contains(t, edges["Start"], "Step1")
}

func TestNodePositioning(t *testing.T) {
	yamlData := `
workspace_name: Position Test

flows:
  - name: PositionFlow
    steps:
      - request:
          name: A
          url: "https://api.example.com/a"
          method: GET
      
      - request:
          name: B
          url: "https://api.example.com/b"
          method: GET
          depends_on:
            - A
      
      - request:
          name: C
          url: "https://api.example.com/c"
          method: GET
          depends_on:
            - A
      
      - request:
          name: D
          url: "https://api.example.com/d"
          method: GET
          depends_on:
            - B
            - C
`

	data, err := workflowsimple.Parse([]byte(yamlData))
	require.NoError(t, err)

	// Build node map
	nodePositions := make(map[string]struct{ X, Y float64 })
	for _, node := range data.Nodes {
		nodePositions[node.Name] = struct{ X, Y float64 }{
			X: node.PositionX,
			Y: node.PositionY,
		}
	}

	// Verify Y positions (levels)
	require.Equal(t, nodePositions["Start"].Y, float64(0))
	require.Equal(t, nodePositions["A"].Y, float64(300))
	require.Equal(t, nodePositions["B"].Y, float64(600))
	require.Equal(t, nodePositions["C"].Y, float64(600))
	require.Equal(t, nodePositions["D"].Y, float64(900))

	// Verify X positions (B and C should be spaced)
	require.NotEqual(t, nodePositions["B"].X, nodePositions["C"].X)
}

func TestVariableResolution(t *testing.T) {
	yamlData := `
workspace_name: Variable Test

flows:
  - name: VarFlow
    variables:
      - name: host
        value: "api.example.com"
      - name: version
        value: "v2"
      - name: auth
        value: "token123"
    steps:
      - request:
          name: TestVars
          url: "https://{{host}}/{{version}}/users"
          method: GET
          headers:
            Authorization: "Bearer {{auth}}"
          query_params:
            api_version: "{{version}}"
`

	data, err := workflowsimple.Parse([]byte(yamlData))
	require.NoError(t, err)

	// Find delta endpoint (should keep variables)
	var deltaEndpoint *mitemapi.ItemApi
	for i := range data.Endpoints {
		if strings.Contains(data.Endpoints[i].Name, "delta") {
			deltaEndpoint = &data.Endpoints[i]
			break
		}
	}
	require.NotNil(t, deltaEndpoint)
	require.Equal(t, "https://{{host}}/{{version}}/users", deltaEndpoint.Url)

	// Check resolved header in delta
	foundResolvedAuth := false
	for _, h := range data.Headers {
		if h.HeaderKey == "Authorization" && h.Value == "Bearer token123" {
			foundResolvedAuth = true
			break
		}
	}
	require.True(t, foundResolvedAuth)

	// Check resolved query param
	foundResolvedQuery := false
	for _, q := range data.Queries {
		if q.QueryKey == "api_version" && q.Value == "v2" {
			foundResolvedQuery = true
			break
		}
	}
	require.True(t, foundResolvedQuery)
}

func TestErrorCases(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want string
	}{
		{
			name: "missing workspace name",
			yaml: `
flows:
  - name: TestFlow
    steps:
      - request:
          name: Test
          url: "https://example.com"
`,
			want: "workspace_name is required",
		},
		{
			name: "no flows",
			yaml: `
workspace_name: Test
`,
			want: "at least one flow is required",
		},
		{
			name: "missing step name",
			yaml: `
workspace_name: Test
flows:
  - name: TestFlow
    steps:
      - request:
          url: "https://example.com"
`,
			want: "missing required 'name' field",
		},
		{
			name: "missing request url",
			yaml: `
workspace_name: Test
flows:
  - name: TestFlow
    steps:
      - request:
          name: Test
          method: GET
`,
			want: "missing required url",
		},
		{
			name: "missing if condition",
			yaml: `
workspace_name: Test
flows:
  - name: TestFlow
    steps:
      - if:
          name: Test
          then: Something
`,
			want: "missing required condition",
		},
		{
			name: "missing for iter_count",
			yaml: `
workspace_name: Test
flows:
  - name: TestFlow
    steps:
      - for:
          name: Test
          loop: Something
`,
			want: "missing required iter_count",
		},
		{
			name: "unknown step type",
			yaml: `
workspace_name: Test
flows:
  - name: TestFlow
    steps:
      - unknown:
          name: Test
`,
			want: "unknown step type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := workflowsimple.Parse([]byte(tt.yaml))
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestComplexWorkflow(t *testing.T) {
	yamlData := `
workspace_name: Complex E-commerce Workflow

request_templates:
  auth_headers:
    headers:
      Authorization: "Bearer {{auth_token}}"
      X-API-Version: "{{api_version}}"

flows:
  - name: OrderProcessingFlow
    variables:
      - name: base_url
        value: "https://api.shop.com"
      - name: auth_token
        value: "secret123"
      - name: api_version
        value: "2.0"
    steps:
      - request:
          name: GetOrders
          use_request: auth_headers
          url: "{{base_url}}/orders"
          method: GET
          query_params:
            status: pending
            limit: "10"

      - if:
          name: CheckOrdersExist
          condition: "GetOrders.response.body.count > 0"
          then: ProcessOrders
          else: NoOrdersFound

      - for_each:
          name: ProcessOrders
          items: "GetOrders.response.body.orders"
          loop: ProcessSingleOrder

      - request:
          name: ProcessSingleOrder
          url: "{{base_url}}/orders/{{item.id}}/process"
          method: POST
          body:
            body_json:
              action: "validate"
              timestamp: "{{now}}"

      - js:
          name: NoOrdersFound
          code: |
            console.log("No pending orders found");
            return { processed: 0 };

      - request:
          name: SendSummary
          url: "{{base_url}}/reports/daily"
          method: POST
          depends_on:
            - ProcessOrders
            - NoOrdersFound
          body:
            body_json:
              date: "{{today}}"
              processed_count: "{{ProcessOrders.result.count || 0}}"
`

	data, err := workflowsimple.Parse([]byte(yamlData))
	require.NoError(t, err)

	// Verify all components were created
	require.Equal(t, "OrderProcessingFlow", data.Flow.Name)
	require.Len(t, data.Variables, 3)

	// Count node types
	nodeTypes := make(map[mnnode.NodeKind]int)
	for _, node := range data.Nodes {
		nodeTypes[node.NodeKind]++
	}

	require.Equal(t, 3, nodeTypes[mnnode.NODE_KIND_REQUEST])
	require.Equal(t, 1, nodeTypes[mnnode.NODE_KIND_CONDITION])
	require.Equal(t, 1, nodeTypes[mnnode.NODE_KIND_FOR_EACH])
	require.Equal(t, 1, nodeTypes[mnnode.NODE_KIND_JS])

	// Verify template application
	authHeaderCount := 0
	for _, h := range data.Headers {
		if h.HeaderKey == "Authorization" || h.HeaderKey == "X-API-Version" {
			authHeaderCount++
		}
	}
	require.Greater(t, authHeaderCount, 0, "Should have auth headers from template")

	// Verify complex dependencies
	edgeCount := len(data.Edges)
	require.Greater(t, edgeCount, 5, "Should have multiple edges for complex flow")

	// Verify delta examples exist
	deltaExamples := 0
	for _, ex := range data.Examples {
		if strings.Contains(ex.Name, "delta") {
			deltaExamples++
		}
	}
	require.Greater(t, deltaExamples, 0, "Should have delta examples")
}

func TestStartNodeConnections(t *testing.T) {
	yamlData := `
workspace_name: Start Node Test

flows:
  - name: TestFlow
    steps:
      - request:
          name: Independent1
          url: "https://api.example.com/1"
          method: GET
      
      - request:
          name: Independent2
          url: "https://api.example.com/2"
          method: GET
      
      - request:
          name: Dependent
          url: "https://api.example.com/3"
          method: GET
          depends_on:
            - Independent1
`

	data, err := workflowsimple.Parse([]byte(yamlData))
	require.NoError(t, err)

	// Find start node
	var startNodeID idwrap.IDWrap
	for _, node := range data.Nodes {
		if node.Name == "Start" {
			startNodeID = node.ID
			break
		}
	}
	require.NotEqual(t, idwrap.IDWrap{}, startNodeID)

	// Find noop node
	var startNoopFound bool
	for _, noop := range data.NoopNodes {
		if noop.FlowNodeID == startNodeID && noop.Type == mnnoop.NODE_NO_OP_KIND_START {
			startNoopFound = true
			break
		}
	}
	require.True(t, startNoopFound, "Start noop node should exist")

	// Count edges from start node
	edgesFromStart := 0
	for _, e := range data.Edges {
		if e.SourceID == startNodeID {
			edgesFromStart++
		}
	}
	require.Equal(t, 1, edgesFromStart, "Start should connect to first independent node")
}