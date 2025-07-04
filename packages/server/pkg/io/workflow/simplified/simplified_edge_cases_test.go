package simplified_test

import (
	"testing"
	"the-dev-tools/server/pkg/io/workflow"
	"the-dev-tools/server/pkg/io/workflow/simplified"
	"the-dev-tools/server/pkg/testutil"
)

func TestSimplifiedEdgeCases(t *testing.T) {
	t.Run("Query Parameters", func(t *testing.T) {
		yaml := `
workspace_name: Test Workspace

flows:
  - name: QueryTest
    steps:
      - request:
          name: SearchUsers
          method: GET
          url: "https://api.example.com/users?page=1&limit=10&filter={{searchTerm}}"
`
		s := simplified.New()
		wd, err := s.Unmarshal([]byte(yaml), workflow.FormatYAML)
		if err != nil {
			t.Fatalf("Failed to unmarshal YAML: %v", err)
		}

		// Verify URL is stored without query parameters
		testutil.AssertFatal(t, 1, len(wd.Endpoints))
		testutil.AssertFatal(t, "https://api.example.com/users", wd.Endpoints[0].Url)

		// Verify query parameters are stored separately
		testutil.AssertFatal(t, 3, len(wd.RequestQueries))

		// Check each query parameter
		queryMap := make(map[string]string)
		for _, q := range wd.RequestQueries {
			queryMap[q.QueryKey] = q.Value
		}

		testutil.AssertFatal(t, "1", queryMap["page"])
		testutil.AssertFatal(t, "10", queryMap["limit"])
		testutil.AssertFatal(t, "{{searchTerm}}", queryMap["filter"])
	})

	t.Run("Form Body Type", func(t *testing.T) {
		yaml := `
workspace_name: Test Workspace

flows:
  - name: FormTest
    steps:
      - request:
          name: SubmitForm
          method: POST
          url: "https://api.example.com/form"
          body:
            kind: form
            value:
              username: testuser
              password: testpass
              remember: "true"
`
		s := simplified.New()
		wd, err := s.Unmarshal([]byte(yaml), workflow.FormatYAML)
		if err != nil {
			t.Fatalf("Failed to unmarshal YAML: %v", err)
		}

		// Verify form body was created
		testutil.AssertFatal(t, 0, len(wd.RequestBodyRaw))
		testutil.AssertFatal(t, 3, len(wd.RequestBodyForm))

		// Check form fields
		formMap := make(map[string]string)
		for _, f := range wd.RequestBodyForm {
			formMap[f.BodyKey] = f.Value
		}

		testutil.AssertFatal(t, "testuser", formMap["username"])
		testutil.AssertFatal(t, "testpass", formMap["password"])
		testutil.AssertFatal(t, "true", formMap["remember"])
	})

	t.Run("URL Encoded Body Type", func(t *testing.T) {
		yaml := `
workspace_name: Test Workspace

flows:
  - name: URLEncodedTest
    steps:
      - request:
          name: OAuth
          method: POST
          url: "https://api.example.com/oauth/token"
          body:
            kind: url
            value:
              grant_type: password
              username: user@example.com
              password: secret123
`
		s := simplified.New()
		wd, err := s.Unmarshal([]byte(yaml), workflow.FormatYAML)
		if err != nil {
			t.Fatalf("Failed to unmarshal YAML: %v", err)
		}

		// Verify URL-encoded body was created
		testutil.AssertFatal(t, 0, len(wd.RequestBodyRaw))
		testutil.AssertFatal(t, 3, len(wd.RequestBodyUrlencoded))

		// Check URL-encoded fields
		urlMap := make(map[string]string)
		for _, u := range wd.RequestBodyUrlencoded {
			urlMap[u.BodyKey] = u.Value
		}

		testutil.AssertFatal(t, "password", urlMap["grant_type"])
		testutil.AssertFatal(t, "user@example.com", urlMap["username"])
		testutil.AssertFatal(t, "secret123", urlMap["password"])
	})

	t.Run("Raw Text Body", func(t *testing.T) {
		yaml := `
workspace_name: Test Workspace

flows:
  - name: RawBodyTest
    steps:
      - request:
          name: SendRawText
          method: POST
          url: "https://api.example.com/raw"
          body:
            kind: raw
            value: "This is raw text content"
`
		s := simplified.New()
		wd, err := s.Unmarshal([]byte(yaml), workflow.FormatYAML)
		if err != nil {
			t.Fatalf("Failed to unmarshal YAML: %v", err)
		}

		// Verify body was created
		testutil.AssertFatal(t, 1, len(wd.RequestBodyRaw))
	})

	t.Run("Multiple Requests Same Endpoint Pattern", func(t *testing.T) {
		yaml := `
workspace_name: Test Workspace

flows:
  - name: MultiRequestTest
    steps:
      - request:
          name: GetUser1
          method: GET
          url: "https://api.example.com/users/1"
          headers:
            X-Request-ID: "req-1"
      - request:
          name: GetUser2
          method: GET
          url: "https://api.example.com/users/2"
          headers:
            X-Request-ID: "req-2"
`
		s := simplified.New()
		wd, err := s.Unmarshal([]byte(yaml), workflow.FormatYAML)
		if err != nil {
			t.Fatalf("Failed to unmarshal YAML: %v", err)
		}

		// Each request should create its own endpoint and example
		testutil.AssertFatal(t, 2, len(wd.Endpoints))
		testutil.AssertFatal(t, 2, len(wd.Examples))
		testutil.AssertFatal(t, 2, len(wd.RequestHeaders))

		// Verify each has unique IDs
		if wd.Endpoints[0].ID == wd.Endpoints[1].ID {
			t.Error("Expected different endpoint IDs for each request")
		}
		if wd.Examples[0].ID == wd.Examples[1].ID {
			t.Error("Expected different example IDs for each request")
		}
	})

	t.Run("Complex Nested Control Flow", func(t *testing.T) {
		yaml := `
workspace_name: Test Workspace

flows:
  - name: NestedFlow
    steps:
      - for:
          name: ProcessBatch
          iter_count: 3
          loop: CheckItem
      - if:
          name: CheckItem
          expression: "item.valid == true"
          then: ProcessValid
          else: ProcessInvalid
      - request:
          name: ProcessValid
          method: POST
          url: "https://api.example.com/process/valid"
      - request:
          name: ProcessInvalid
          method: POST
          url: "https://api.example.com/process/invalid"
`
		s := simplified.New()
		wd, err := s.Unmarshal([]byte(yaml), workflow.FormatYAML)
		if err != nil {
			t.Fatalf("Failed to unmarshal YAML: %v", err)
		}

		// Verify all nodes were created
		testutil.AssertFatal(t, 4, len(wd.FlowNodes))
		testutil.AssertFatal(t, 1, len(wd.FlowForNodes))
		testutil.AssertFatal(t, 1, len(wd.FlowConditionNodes))
		testutil.AssertFatal(t, 2, len(wd.FlowRequestNodes))
	})

	t.Run("ForEach with Collection", func(t *testing.T) {
		yaml := `
workspace_name: Test Workspace

flows:
  - name: ForEachTest
    steps:
      - for_each:
          name: ProcessItems
          collection: "response.items"
          item: currentItem
          loop: ProcessItem
      - request:
          name: ProcessItem
          method: POST
          url: "https://api.example.com/process/{{currentItem.id}}"
          body:
            data: "{{currentItem}}"
`
		s := simplified.New()
		wd, err := s.Unmarshal([]byte(yaml), workflow.FormatYAML)
		if err != nil {
			t.Fatalf("Failed to unmarshal YAML: %v", err)
		}

		// Verify for-each node
		testutil.AssertFatal(t, 1, len(wd.FlowForEachNodes))
		testutil.AssertFatal(t, "response.items", wd.FlowForEachNodes[0].IterExpression)
	})

	t.Run("Empty Body Should Not Create Body Entity", func(t *testing.T) {
		yaml := `
workspace_name: Test Workspace

flows:
  - name: NoBodyTest
    steps:
      - request:
          name: GetRequest
          method: GET
          url: "https://api.example.com/data"
`
		s := simplified.New()
		wd, err := s.Unmarshal([]byte(yaml), workflow.FormatYAML)
		if err != nil {
			t.Fatalf("Failed to unmarshal YAML: %v", err)
		}

		// Should not create body for GET request without body
		testutil.AssertFatal(t, 0, len(wd.RequestBodyRaw))
	})

	t.Run("Dependencies Between Steps", func(t *testing.T) {
		yaml := `
workspace_name: Test Workspace

flows:
  - name: DependencyTest
    steps:
      - request:
          name: Step1
          method: GET
          url: "https://api.example.com/step1"
      - request:
          name: Step2
          method: GET
          url: "https://api.example.com/step2"
          depends_on: [Step1]
      - request:
          name: Step3
          method: GET
          url: "https://api.example.com/step3"
          depends_on: [Step1, Step2]
`
		s := simplified.New()
		wd, err := s.Unmarshal([]byte(yaml), workflow.FormatYAML)
		if err != nil {
			t.Fatalf("Failed to unmarshal YAML: %v", err)
		}

		// Verify nodes were created
		testutil.AssertFatal(t, 3, len(wd.FlowNodes))

		// TODO: Dependencies should create appropriate edges
	})

	t.Run("Missing Required Fields", func(t *testing.T) {
		testCases := []struct {
			name      string
			yaml      string
			wantError string
		}{
			{
				name: "Missing workspace name",
				yaml: `
flows:
  - name: TestFlow
    steps:
      - request:
          name: Test
          method: GET
          url: "https://api.example.com"
`,
				wantError: "workspace_name is required",
			},
			{
				name: "Missing request method",
				yaml: `
workspace_name: Test
flows:
  - name: TestFlow
    steps:
      - request:
          name: Test
          url: "https://api.example.com"
`,
				wantError: "request method is required",
			},
			{
				name: "Missing request URL",
				yaml: `
workspace_name: Test
flows:
  - name: TestFlow
    steps:
      - request:
          name: Test
          method: GET
`,
				wantError: "request URL is required",
			},
			{
				name: "Missing flow name",
				yaml: `
workspace_name: Test
flows:
  - steps:
      - request:
          name: Test
          method: GET
          url: "https://api.example.com"
`,
				wantError: "flow name is required",
			},
			{
				name: "Empty flow steps",
				yaml: `
workspace_name: Test
flows:
  - name: TestFlow
    steps: []
`,
				wantError: "must have at least one step",
			},
			{
				name: "Missing if expression",
				yaml: `
workspace_name: Test
flows:
  - name: TestFlow
    steps:
      - if:
          name: Check
          then: Success
`,
				wantError: "if expression is required",
			},
			{
				name: "Invalid for iter_count",
				yaml: `
workspace_name: Test
flows:
  - name: TestFlow
    steps:
      - for:
          name: Loop
          iter_count: -1
          loop: DoSomething
`,
				wantError: "for iter_count must be positive",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				s := simplified.New()
				_, err := s.Unmarshal([]byte(tc.yaml), workflow.FormatYAML)
				if err == nil {
					t.Fatalf("Expected error containing '%s', but got no error", tc.wantError)
				}
				if !contains(err.Error(), tc.wantError) {
					t.Fatalf("Expected error containing '%s', got: %v", tc.wantError, err)
				}
			})
		}
	})
}

func TestDeltaEndpointCreation(t *testing.T) {
	yaml := `
workspace_name: Test Workspace

requests:
  - name: base_request
    method: GET
    url: "https://api.example.com/{{endpoint}}"
    headers:
      Authorization: "Bearer {{token}}"

flows:
  - name: TestFlow
    variables:
      endpoint: users
      token: test-token
    steps:
      - request:
          name: Request1
          use_request: base_request
          headers:
            X-Custom: "value1"
      - request:
          name: Request2
          use_request: base_request
          headers:
            X-Custom: "value2"
      - request:
          name: Request3
          use_request: base_request
          url: "https://api.example.com/posts"
          method: POST
          body:
            data: test
`

	s := simplified.New()
	wd, err := s.Unmarshal([]byte(yaml), workflow.FormatYAML)
	if err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	// Each request should create its own endpoint and example (deltas)
	testutil.AssertFatal(t, 3, len(wd.Endpoints))
	testutil.AssertFatal(t, 3, len(wd.Examples))
	testutil.AssertFatal(t, 3, len(wd.FlowRequestNodes))

	// Verify each endpoint has unique ID
	endpointIDs := make(map[string]bool)
	for _, ep := range wd.Endpoints {
		if endpointIDs[ep.ID.String()] {
			t.Error("Found duplicate endpoint ID")
		}
		endpointIDs[ep.ID.String()] = true
	}

	// Verify each example has unique ID
	exampleIDs := make(map[string]bool)
	for _, ex := range wd.Examples {
		if exampleIDs[ex.ID.String()] {
			t.Error("Found duplicate example ID")
		}
		exampleIDs[ex.ID.String()] = true
	}

	// Verify method override
	foundPostMethod := false
	for _, ep := range wd.Endpoints {
		if ep.Name == "Request3" && ep.Method == "POST" {
			foundPostMethod = true
		}
	}
	if !foundPostMethod {
		t.Error("Expected Request3 to have POST method override")
	}

	// Verify headers are created correctly
	headersByExample := make(map[string][]string)
	for _, h := range wd.RequestHeaders {
		headersByExample[h.ExampleID.String()] = append(headersByExample[h.ExampleID.String()], h.HeaderKey)
	}

	// Check headers for each example (removing unused code)

	// Verify all examples have Authorization header
	for _, headers := range headersByExample {
		hasAuth := false
		for _, h := range headers {
			if h == "Authorization" {
				hasAuth = true
				break
			}
		}
		if !hasAuth {
			t.Error("Expected Authorization header in all examples")
		}
	}

	// Verify first two requests have X-Custom header
	customCount := 0
	for _, headers := range headersByExample {
		for _, h := range headers {
			if h == "X-Custom" {
				customCount++
				break
			}
		}
	}
	if customCount < 2 {
		t.Errorf("Expected X-Custom header in at least 2 examples, got %d", customCount)
	}

	// Verify body is only created for Request3
	testutil.AssertFatal(t, 1, len(wd.RequestBodyRaw))
}

func TestDependencyEdges(t *testing.T) {
	yaml := `
workspace_name: Test Workspace

flows:
  - name: DependencyFlow
    steps:
      - request:
          name: Step1
          method: GET
          url: "https://api.example.com/step1"
      - request:
          name: Step2
          method: GET
          url: "https://api.example.com/step2"
        depends_on: [Step1]
      - request:
          name: Step3
          method: GET
          url: "https://api.example.com/step3"
        depends_on: [Step1, Step2]
      - request:
          name: Step4
          method: GET
          url: "https://api.example.com/step4"
        depends_on: [Step3]
`

	s := simplified.New()
	wd, err := s.Unmarshal([]byte(yaml), workflow.FormatYAML)
	if err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	// Create node name to ID map
	nodeMap := make(map[string]string)
	for _, node := range wd.FlowNodes {
		nodeMap[node.Name] = node.ID.String()
	}

	// Verify dependency edges were created
	// Should have: Step1->Step2, Step1->Step3, Step2->Step3, Step3->Step4
	expectedEdges := []struct {
		from string
		to   string
	}{
		{"Step1", "Step2"},
		{"Step1", "Step3"},
		{"Step2", "Step3"},
		{"Step3", "Step4"},
	}

	for _, exp := range expectedEdges {
		found := false
		for _, edge := range wd.FlowEdges {
			if edge.SourceID.String() == nodeMap[exp.from] &&
				edge.TargetID.String() == nodeMap[exp.to] {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected edge from %s to %s not found", exp.from, exp.to)
		}
	}
}
