package topenapiv2

import (
	"testing"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
)

func TestConvertOpenAPI_Swagger2(t *testing.T) {
	swaggerJSON := []byte(`{
		"swagger": "2.0",
		"info": {"title": "Pet Store", "version": "1.0.0"},
		"host": "petstore.swagger.io",
		"basePath": "/v2",
		"schemes": ["https"],
		"paths": {
			"/pets": {
				"get": {
					"summary": "List all pets",
					"operationId": "listPets",
					"parameters": [
						{"name": "limit", "in": "query", "type": "integer", "required": false, "example": 10}
					],
					"responses": {
						"200": {"description": "A list of pets"}
					}
				},
				"post": {
					"summary": "Create a pet",
					"operationId": "createPet",
					"parameters": [
						{
							"name": "body",
							"in": "body",
							"schema": {
								"type": "object",
								"properties": {
									"name": {"type": "string", "example": "doggie"},
									"status": {"type": "string", "example": "available"}
								}
							}
						}
					],
					"responses": {
						"201": {"description": "Pet created"}
					}
				}
			},
			"/pets/{petId}": {
				"get": {
					"summary": "Get a pet by ID",
					"operationId": "getPet",
					"parameters": [
						{"name": "petId", "in": "path", "type": "integer", "required": true, "example": 123}
					],
					"responses": {
						"200": {"description": "A pet"}
					}
				}
			}
		}
	}`)

	opts := ConvertOptions{
		WorkspaceID: idwrap.NewNow(),
	}

	resolved, err := ConvertOpenAPI(swaggerJSON, opts)
	if err != nil {
		t.Fatalf("ConvertOpenAPI() error = %v", err)
	}

	// Should have 3 HTTP requests (GET /pets, POST /pets, GET /pets/{petId})
	if len(resolved.HTTPRequests) != 3 {
		t.Errorf("expected 3 HTTP requests, got %d", len(resolved.HTTPRequests))
	}

	// Verify flow was created
	if resolved.Flow.Name != "Pet Store" {
		t.Errorf("expected flow name 'Pet Store', got %q", resolved.Flow.Name)
	}

	// Verify base URL
	for _, req := range resolved.HTTPRequests {
		if req.Url == "" {
			t.Errorf("HTTP request %q has empty URL", req.Name)
		}
		if req.Method == "" {
			t.Errorf("HTTP request %q has empty method", req.Name)
		}
	}

	// Find the GET /pets request and verify query params
	var getReq *mhttp.HTTP
	for i := range resolved.HTTPRequests {
		if resolved.HTTPRequests[i].Method == "GET" && resolved.HTTPRequests[i].Name == "List all pets" {
			getReq = &resolved.HTTPRequests[i]
			break
		}
	}
	if getReq == nil {
		t.Fatal("could not find GET /pets request")
	}

	// Verify query parameters
	var foundLimit bool
	for _, sp := range resolved.SearchParams {
		if sp.HttpID == getReq.ID && sp.Key == "limit" {
			foundLimit = true
			if sp.Value != "10" {
				t.Errorf("expected limit value '10', got %q", sp.Value)
			}
		}
	}
	if !foundLimit {
		t.Error("expected to find 'limit' query parameter")
	}

	// Find POST /pets and verify body
	var postReq *mhttp.HTTP
	for i := range resolved.HTTPRequests {
		if resolved.HTTPRequests[i].Method == "POST" {
			postReq = &resolved.HTTPRequests[i]
			break
		}
	}
	if postReq == nil {
		t.Fatal("could not find POST /pets request")
	}
	if postReq.BodyKind != mhttp.HttpBodyKindRaw {
		t.Errorf("expected body kind Raw, got %v", postReq.BodyKind)
	}

	// Verify body raw exists for POST
	var foundBody bool
	for _, br := range resolved.BodyRaw {
		if br.HttpID == postReq.ID {
			foundBody = true
			if len(br.RawData) == 0 {
				t.Error("expected non-empty body raw data")
			}
		}
	}
	if !foundBody {
		t.Error("expected to find body raw for POST request")
	}

	// Verify path parameter replacement
	var getPetReq *mhttp.HTTP
	for i := range resolved.HTTPRequests {
		if resolved.HTTPRequests[i].Name == "Get a pet by ID" {
			getPetReq = &resolved.HTTPRequests[i]
			break
		}
	}
	if getPetReq == nil {
		t.Fatal("could not find GET /pets/{petId} request")
	}
	if getPetReq.Url != "https://petstore.swagger.io/v2/pets/123" {
		t.Errorf("expected URL with petId replaced, got %q", getPetReq.Url)
	}

	// Verify nodes and edges
	if len(resolved.Nodes) != 4 { // 1 start + 3 request nodes
		t.Errorf("expected 4 nodes, got %d", len(resolved.Nodes))
	}
	if len(resolved.Edges) != 3 {
		t.Errorf("expected 3 edges, got %d", len(resolved.Edges))
	}

	// Verify files
	if len(resolved.Files) == 0 {
		t.Error("expected files to be created")
	}
}

func TestConvertOpenAPI_OpenAPI3(t *testing.T) {
	openAPI3JSON := []byte(`{
		"openapi": "3.0.0",
		"info": {"title": "User API", "version": "1.0.0"},
		"servers": [{"url": "https://api.example.com/v1"}],
		"paths": {
			"/users": {
				"get": {
					"summary": "List users",
					"parameters": [
						{"name": "Authorization", "in": "header", "required": true, "example": "Bearer token123"},
						{"name": "page", "in": "query", "required": false, "example": 1}
					],
					"responses": {
						"200": {"description": "OK"}
					}
				},
				"post": {
					"summary": "Create user",
					"requestBody": {
						"content": {
							"application/json": {
								"schema": {
									"type": "object",
									"properties": {
										"name": {"type": "string", "example": "John"},
										"email": {"type": "string", "example": "john@example.com"}
									}
								}
							}
						}
					},
					"responses": {
						"201": {"description": "Created"}
					}
				}
			}
		}
	}`)

	opts := ConvertOptions{
		WorkspaceID: idwrap.NewNow(),
	}

	resolved, err := ConvertOpenAPI(openAPI3JSON, opts)
	if err != nil {
		t.Fatalf("ConvertOpenAPI() error = %v", err)
	}

	if len(resolved.HTTPRequests) != 2 {
		t.Errorf("expected 2 HTTP requests, got %d", len(resolved.HTTPRequests))
	}

	if resolved.Flow.Name != "User API" {
		t.Errorf("expected flow name 'User API', got %q", resolved.Flow.Name)
	}

	// Verify GET /users has Authorization header
	var getUsersReq *mhttp.HTTP
	for i := range resolved.HTTPRequests {
		if resolved.HTTPRequests[i].Method == "GET" {
			getUsersReq = &resolved.HTTPRequests[i]
			break
		}
	}
	if getUsersReq == nil {
		t.Fatal("could not find GET /users request")
	}

	var foundAuth bool
	for _, h := range resolved.Headers {
		if h.HttpID == getUsersReq.ID && h.Key == "Authorization" {
			foundAuth = true
			if h.Value != "Bearer token123" {
				t.Errorf("expected Authorization value 'Bearer token123', got %q", h.Value)
			}
		}
	}
	if !foundAuth {
		t.Error("expected to find Authorization header")
	}

	// Verify POST /users has Content-Type header
	var postReq *mhttp.HTTP
	for i := range resolved.HTTPRequests {
		if resolved.HTTPRequests[i].Method == "POST" {
			postReq = &resolved.HTTPRequests[i]
			break
		}
	}
	if postReq == nil {
		t.Fatal("could not find POST /users request")
	}

	var foundContentType bool
	for _, h := range resolved.Headers {
		if h.HttpID == postReq.ID && h.Key == "Content-Type" {
			foundContentType = true
			if h.Value != "application/json" {
				t.Errorf("expected Content-Type 'application/json', got %q", h.Value)
			}
		}
	}
	if !foundContentType {
		t.Error("expected to find Content-Type header for POST request")
	}
}

func TestConvertOpenAPI_YAML(t *testing.T) {
	yamlSpec := []byte(`
openapi: "3.0.0"
info:
  title: YAML API
  version: "1.0"
servers:
  - url: https://yaml-api.example.com
paths:
  /items:
    get:
      summary: List items
      responses:
        "200":
          description: Success
`)

	opts := ConvertOptions{
		WorkspaceID: idwrap.NewNow(),
	}

	resolved, err := ConvertOpenAPI(yamlSpec, opts)
	if err != nil {
		t.Fatalf("ConvertOpenAPI() error = %v", err)
	}

	if len(resolved.HTTPRequests) != 1 {
		t.Errorf("expected 1 HTTP request, got %d", len(resolved.HTTPRequests))
	}

	if resolved.HTTPRequests[0].Url != "https://yaml-api.example.com/items" {
		t.Errorf("unexpected URL: %q", resolved.HTTPRequests[0].Url)
	}
}

func TestConvertOpenAPI_EmptyData(t *testing.T) {
	opts := ConvertOptions{WorkspaceID: idwrap.NewNow()}
	_, err := ConvertOpenAPI([]byte{}, opts)
	if err == nil {
		t.Error("expected error for empty data")
	}
}

func TestConvertOpenAPI_InvalidData(t *testing.T) {
	opts := ConvertOptions{WorkspaceID: idwrap.NewNow()}
	_, err := ConvertOpenAPI([]byte("not json or yaml"), opts)
	if err == nil {
		t.Error("expected error for invalid data")
	}
}

func TestConvertOpenAPI_NoPathsReturnsEmptyRequests(t *testing.T) {
	data := []byte(`{"openapi": "3.0.0", "info": {"title": "Empty"}}`)
	opts := ConvertOptions{WorkspaceID: idwrap.NewNow()}
	resolved, err := ConvertOpenAPI(data, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resolved.HTTPRequests) != 0 {
		t.Errorf("expected 0 HTTP requests, got %d", len(resolved.HTTPRequests))
	}
}

func TestParseSpec_Swagger2(t *testing.T) {
	data := []byte(`{"swagger": "2.0", "info": {"title": "Test"}, "host": "api.test.com", "basePath": "/v1", "schemes": ["https"]}`)
	s, err := parseSpec(data)
	if err != nil {
		t.Fatalf("parseSpec() error = %v", err)
	}
	if s.Title != "Test" {
		t.Errorf("expected title 'Test', got %q", s.Title)
	}
	if s.BaseURL != "https://api.test.com/v1" {
		t.Errorf("expected base URL 'https://api.test.com/v1', got %q", s.BaseURL)
	}
}

func TestParseSpec_OpenAPI3(t *testing.T) {
	data := []byte(`{"openapi": "3.0.0", "info": {"title": "Test3"}, "servers": [{"url": "https://api3.test.com"}]}`)
	s, err := parseSpec(data)
	if err != nil {
		t.Fatalf("parseSpec() error = %v", err)
	}
	if s.Title != "Test3" {
		t.Errorf("expected title 'Test3', got %q", s.Title)
	}
	if s.BaseURL != "https://api3.test.com" {
		t.Errorf("expected base URL 'https://api3.test.com', got %q", s.BaseURL)
	}
}

func TestIsHTTPMethod(t *testing.T) {
	tests := []struct {
		method string
		want   bool
	}{
		{"GET", true},
		{"POST", true},
		{"PUT", true},
		{"DELETE", true},
		{"PATCH", true},
		{"HEAD", true},
		{"OPTIONS", true},
		{"parameters", false},
		{"summary", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isHTTPMethod(tt.method); got != tt.want {
			t.Errorf("isHTTPMethod(%q) = %v, want %v", tt.method, got, tt.want)
		}
	}
}

func TestGenerateExampleJSON(t *testing.T) {
	schema := &schemaObj{
		Type: "object",
		Properties: map[string]*schemaObj{
			"name":  {Type: "string", Example: "John"},
			"age":   {Type: "integer"},
			"email": {Type: "string"},
		},
	}

	result := generateExampleJSON(schema)
	if result == "" {
		t.Error("expected non-empty example JSON")
	}
}

func TestBuildFolderPathFromURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://api.example.com/v1/users", "/com/example/api"},
		{"https://localhost:8080/api", "/localhost"},
		{"", ""},
		{"not-a-url", ""},
	}
	for _, tt := range tests {
		got := buildFolderPathFromURL(tt.url)
		if got != tt.want {
			t.Errorf("buildFolderPathFromURL(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestConvertOpenAPI_PathLevelParameters(t *testing.T) {
	spec := []byte(`{
		"openapi": "3.0.0",
		"info": {"title": "Test"},
		"servers": [{"url": "https://api.test.com"}],
		"paths": {
			"/items/{itemId}": {
				"parameters": [
					{"name": "itemId", "in": "path", "required": true, "example": "abc123"}
				],
				"get": {
					"summary": "Get item",
					"responses": {"200": {"description": "OK"}}
				},
				"put": {
					"summary": "Update item",
					"responses": {"200": {"description": "OK"}}
				}
			}
		}
	}`)

	opts := ConvertOptions{WorkspaceID: idwrap.NewNow()}
	resolved, err := ConvertOpenAPI(spec, opts)
	if err != nil {
		t.Fatalf("ConvertOpenAPI() error = %v", err)
	}

	// Both GET and PUT should have the path param resolved
	for _, req := range resolved.HTTPRequests {
		if req.Url != "https://api.test.com/items/abc123" {
			t.Errorf("expected URL with itemId replaced, got %q for %s", req.Url, req.Method)
		}
	}
}
