package topenapiv2

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mfile"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
)

// --- Real-world Swagger 2.0 (Petstore) ---

func TestRealWorld_PetstoreSwagger2(t *testing.T) {
	path := filepath.Join("..", "..", "..", "test", "openapi", "petstore_swagger2.json")
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	opts := ConvertOptions{
		WorkspaceID: idwrap.NewNow(),
	}

	resolved, err := ConvertOpenAPI(data, opts)
	require.NoError(t, err)

	// Petstore has 14 operations across all paths
	require.Equal(t, 14, len(resolved.HTTPRequests), "Should import all 14 operations")

	// Flow
	require.NotEmpty(t, resolved.Flow.ID, "Should generate a Flow ID")
	require.Equal(t, "Petstore API", resolved.Flow.Name, "Flow should use spec title")

	// Nodes: 1 start + 14 request
	require.Equal(t, 15, len(resolved.Nodes), "Should have 15 nodes (1 start + 14 request)")
	require.Equal(t, 14, len(resolved.RequestNodes), "Should have 14 request node metadata entries")
	require.Equal(t, 14, len(resolved.Edges), "Should have 14 edges")

	t.Logf("Imported Petstore Swagger 2.0:")
	t.Logf("  - Requests: %d", len(resolved.HTTPRequests))
	t.Logf("  - Flow Nodes: %d", len(resolved.Nodes))
	t.Logf("  - Flow Edges: %d", len(resolved.Edges))
	t.Logf("  - Files/Folders: %d", len(resolved.Files))
	t.Logf("  - Headers: %d", len(resolved.Headers))
	t.Logf("  - Query Params: %d", len(resolved.SearchParams))
	t.Logf("  - Body Raw: %d", len(resolved.BodyRaw))

	// Verify each request has a valid URL with the base URL prefix
	for _, req := range resolved.HTTPRequests {
		require.True(t, strings.HasPrefix(req.Url, "https://petstore.swagger.io/v2"),
			"URL should start with base URL, got: %s", req.Url)
		require.NotEmpty(t, req.Method, "Method should not be empty for %s", req.Name)
		require.NotEmpty(t, req.Name, "Name should not be empty")
	}
}

func TestRealWorld_PetstoreSwagger2_Methods(t *testing.T) {
	path := filepath.Join("..", "..", "..", "test", "openapi", "petstore_swagger2.json")
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	opts := ConvertOptions{WorkspaceID: idwrap.NewNow()}
	resolved, err := ConvertOpenAPI(data, opts)
	require.NoError(t, err)

	methodCounts := map[string]int{}
	for _, req := range resolved.HTTPRequests {
		methodCounts[req.Method]++
	}

	// Petstore: 6 GET, 3 POST, 2 PUT, 3 DELETE
	require.Equal(t, 6, methodCounts["GET"], "Should have 6 GET operations")
	require.Equal(t, 3, methodCounts["POST"], "Should have 3 POST operations")
	require.Equal(t, 2, methodCounts["PUT"], "Should have 2 PUT operations")
	require.Equal(t, 3, methodCounts["DELETE"], "Should have 3 DELETE operations")
}

func TestRealWorld_PetstoreSwagger2_PathParams(t *testing.T) {
	path := filepath.Join("..", "..", "..", "test", "openapi", "petstore_swagger2.json")
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	opts := ConvertOptions{WorkspaceID: idwrap.NewNow()}
	resolved, err := ConvertOpenAPI(data, opts)
	require.NoError(t, err)

	// Find GET /pet/{petId} - should have path param replaced with example value
	var getPet *mhttp.HTTP
	for i := range resolved.HTTPRequests {
		if resolved.HTTPRequests[i].Name == "Find pet by ID" {
			getPet = &resolved.HTTPRequests[i]
			break
		}
	}
	require.NotNil(t, getPet, "Should find 'Find pet by ID' request")
	require.Equal(t, "https://petstore.swagger.io/v2/pet/42", getPet.Url,
		"Path param {petId} should be replaced with example value 42")

	// Find GET /user/{username} - should have path param replaced with example
	var getUser *mhttp.HTTP
	for i := range resolved.HTTPRequests {
		if resolved.HTTPRequests[i].Name == "Get user by user name" {
			getUser = &resolved.HTTPRequests[i]
			break
		}
	}
	require.NotNil(t, getUser, "Should find 'Get user by user name' request")
	require.Equal(t, "https://petstore.swagger.io/v2/user/johndoe", getUser.Url,
		"Path param {username} should be replaced with example value 'johndoe'")
}

func TestRealWorld_PetstoreSwagger2_QueryParams(t *testing.T) {
	path := filepath.Join("..", "..", "..", "test", "openapi", "petstore_swagger2.json")
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	opts := ConvertOptions{WorkspaceID: idwrap.NewNow()}
	resolved, err := ConvertOpenAPI(data, opts)
	require.NoError(t, err)

	// Find GET /pet/findByStatus - should have 'status' query param
	var findByStatus *mhttp.HTTP
	for i := range resolved.HTTPRequests {
		if resolved.HTTPRequests[i].Name == "Finds Pets by status" {
			findByStatus = &resolved.HTTPRequests[i]
			break
		}
	}
	require.NotNil(t, findByStatus, "Should find 'Finds Pets by status' request")

	var statusParam *mhttp.HTTPSearchParam
	for i := range resolved.SearchParams {
		if resolved.SearchParams[i].HttpID == findByStatus.ID && resolved.SearchParams[i].Key == "status" {
			statusParam = &resolved.SearchParams[i]
			break
		}
	}
	require.NotNil(t, statusParam, "Should find 'status' query param")
	require.Equal(t, "available", statusParam.Value, "Should have example value 'available'")
	require.True(t, statusParam.Enabled, "Required param should be enabled")

	// Find GET /user/login - should have username + password query params
	var loginUser *mhttp.HTTP
	for i := range resolved.HTTPRequests {
		if resolved.HTTPRequests[i].Name == "Logs user into the system" {
			loginUser = &resolved.HTTPRequests[i]
			break
		}
	}
	require.NotNil(t, loginUser, "Should find 'Logs user into the system' request")

	loginParams := map[string]string{}
	for _, sp := range resolved.SearchParams {
		if sp.HttpID == loginUser.ID {
			loginParams[sp.Key] = sp.Value
		}
	}
	require.Equal(t, "johndoe", loginParams["username"])
	require.Equal(t, "pass123", loginParams["password"])
}

func TestRealWorld_PetstoreSwagger2_Headers(t *testing.T) {
	path := filepath.Join("..", "..", "..", "test", "openapi", "petstore_swagger2.json")
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	opts := ConvertOptions{WorkspaceID: idwrap.NewNow()}
	resolved, err := ConvertOpenAPI(data, opts)
	require.NoError(t, err)

	// Find DELETE /pet/{petId} - should have api_key header
	var deletePet *mhttp.HTTP
	for i := range resolved.HTTPRequests {
		if resolved.HTTPRequests[i].Name == "Deletes a pet" {
			deletePet = &resolved.HTTPRequests[i]
			break
		}
	}
	require.NotNil(t, deletePet, "Should find 'Deletes a pet' request")

	var apiKeyHeader *mhttp.HTTPHeader
	for i := range resolved.Headers {
		if resolved.Headers[i].HttpID == deletePet.ID && resolved.Headers[i].Key == "api_key" {
			apiKeyHeader = &resolved.Headers[i]
			break
		}
	}
	require.NotNil(t, apiKeyHeader, "Should find api_key header")
	require.Equal(t, "special-key", apiKeyHeader.Value)

	// Find GET /store/inventory - should have Authorization header
	var getInventory *mhttp.HTTP
	for i := range resolved.HTTPRequests {
		if resolved.HTTPRequests[i].Name == "Returns pet inventories by status" {
			getInventory = &resolved.HTTPRequests[i]
			break
		}
	}
	require.NotNil(t, getInventory, "Should find 'Returns pet inventories by status' request")

	var authHeader *mhttp.HTTPHeader
	for i := range resolved.Headers {
		if resolved.Headers[i].HttpID == getInventory.ID && resolved.Headers[i].Key == "Authorization" {
			authHeader = &resolved.Headers[i]
			break
		}
	}
	require.NotNil(t, authHeader, "Should find Authorization header")
	require.Equal(t, "Bearer abc123", authHeader.Value)
}

func TestRealWorld_PetstoreSwagger2_Bodies(t *testing.T) {
	path := filepath.Join("..", "..", "..", "test", "openapi", "petstore_swagger2.json")
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	opts := ConvertOptions{WorkspaceID: idwrap.NewNow()}
	resolved, err := ConvertOpenAPI(data, opts)
	require.NoError(t, err)

	// Find POST /pet - should have body with example JSON
	var addPet *mhttp.HTTP
	for i := range resolved.HTTPRequests {
		if resolved.HTTPRequests[i].Name == "Add a new pet to the store" {
			addPet = &resolved.HTTPRequests[i]
			break
		}
	}
	require.NotNil(t, addPet, "Should find 'Add a new pet to the store' request")
	require.Equal(t, mhttp.HttpBodyKindRaw, addPet.BodyKind, "POST should have raw body kind")

	var addPetBody *mhttp.HTTPBodyRaw
	for i := range resolved.BodyRaw {
		if resolved.BodyRaw[i].HttpID == addPet.ID {
			addPetBody = &resolved.BodyRaw[i]
			break
		}
	}
	require.NotNil(t, addPetBody, "Should find raw body for 'Add a new pet to the store'")
	require.NotEmpty(t, addPetBody.RawData, "Body should not be empty")

	// Body should be valid JSON with expected example fields
	var bodyMap map[string]interface{}
	err = json.Unmarshal(addPetBody.RawData, &bodyMap)
	require.NoError(t, err, "Body should be valid JSON")
	require.Equal(t, "doggie", bodyMap["name"], "Body should contain example name")
	require.Equal(t, "available", bodyMap["status"], "Body should contain example status")

	// Content-Type header should be present
	var ctHeader *mhttp.HTTPHeader
	for i := range resolved.Headers {
		if resolved.Headers[i].HttpID == addPet.ID && resolved.Headers[i].Key == "Content-Type" {
			ctHeader = &resolved.Headers[i]
			break
		}
	}
	require.NotNil(t, ctHeader, "POST requests should have Content-Type header")
	require.Equal(t, "application/json", ctHeader.Value)
}

// --- Real-world OpenAPI 3.0 (Stripe-like) ---

func TestRealWorld_StripeOpenAPI3(t *testing.T) {
	path := filepath.Join("..", "..", "..", "test", "openapi", "stripe_openapi3.json")
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	opts := ConvertOptions{
		WorkspaceID: idwrap.NewNow(),
	}

	resolved, err := ConvertOpenAPI(data, opts)
	require.NoError(t, err)

	// Stripe-like API has 9 operations
	require.Equal(t, 9, len(resolved.HTTPRequests), "Should import all 9 operations")

	require.Equal(t, "Stripe-like Payment API", resolved.Flow.Name)

	// Nodes: 1 start + 9 request
	require.Equal(t, 10, len(resolved.Nodes), "Should have 10 nodes (1 start + 9 request)")
	require.Equal(t, 9, len(resolved.RequestNodes))
	require.Equal(t, 9, len(resolved.Edges))

	t.Logf("Imported Stripe-like OpenAPI 3.0:")
	t.Logf("  - Requests: %d", len(resolved.HTTPRequests))
	t.Logf("  - Flow Nodes: %d", len(resolved.Nodes))
	t.Logf("  - Files/Folders: %d", len(resolved.Files))
	t.Logf("  - Headers: %d", len(resolved.Headers))
	t.Logf("  - Query Params: %d", len(resolved.SearchParams))
	t.Logf("  - Body Raw: %d", len(resolved.BodyRaw))

	// All URLs should use the first server (production)
	for _, req := range resolved.HTTPRequests {
		require.True(t, strings.HasPrefix(req.Url, "https://api.stripe-example.com/v1"),
			"URL should use first server URL, got: %s", req.Url)
	}
}

func TestRealWorld_StripeOpenAPI3_PathLevelParams(t *testing.T) {
	path := filepath.Join("..", "..", "..", "test", "openapi", "stripe_openapi3.json")
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	opts := ConvertOptions{WorkspaceID: idwrap.NewNow()}
	resolved, err := ConvertOpenAPI(data, opts)
	require.NoError(t, err)

	// /customers/{customerId} has path-level param shared by GET, POST, DELETE
	var retrieveCustomer *mhttp.HTTP
	for i := range resolved.HTTPRequests {
		if resolved.HTTPRequests[i].Name == "Retrieve a customer" {
			retrieveCustomer = &resolved.HTTPRequests[i]
			break
		}
	}
	require.NotNil(t, retrieveCustomer, "Should find 'Retrieve a customer' request")
	require.Equal(t, "https://api.stripe-example.com/v1/customers/cus_abc123", retrieveCustomer.Url,
		"Path-level param {customerId} should be resolved to example value")

	// DELETE /customers/{customerId} should also resolve the path param
	var deleteCustomer *mhttp.HTTP
	for i := range resolved.HTTPRequests {
		if resolved.HTTPRequests[i].Name == "Delete a customer" {
			deleteCustomer = &resolved.HTTPRequests[i]
			break
		}
	}
	require.NotNil(t, deleteCustomer, "Should find 'Delete a customer' request")
	require.Equal(t, "https://api.stripe-example.com/v1/customers/cus_abc123", deleteCustomer.Url)
}

func TestRealWorld_StripeOpenAPI3_RequestBodies(t *testing.T) {
	path := filepath.Join("..", "..", "..", "test", "openapi", "stripe_openapi3.json")
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	opts := ConvertOptions{WorkspaceID: idwrap.NewNow()}
	resolved, err := ConvertOpenAPI(data, opts)
	require.NoError(t, err)

	// POST /customers should have body with customer fields
	var createCustomer *mhttp.HTTP
	for i := range resolved.HTTPRequests {
		if resolved.HTTPRequests[i].Name == "Create a customer" {
			createCustomer = &resolved.HTTPRequests[i]
			break
		}
	}
	require.NotNil(t, createCustomer, "Should find 'Create a customer' request")
	require.Equal(t, mhttp.HttpBodyKindRaw, createCustomer.BodyKind)

	var body *mhttp.HTTPBodyRaw
	for i := range resolved.BodyRaw {
		if resolved.BodyRaw[i].HttpID == createCustomer.ID {
			body = &resolved.BodyRaw[i]
			break
		}
	}
	require.NotNil(t, body, "Should have body for 'Create a customer'")

	var bodyMap map[string]interface{}
	err = json.Unmarshal(body.RawData, &bodyMap)
	require.NoError(t, err, "Body should be valid JSON")
	require.Equal(t, "jenny@example.com", bodyMap["email"])
	require.Equal(t, "Jenny Rosen", bodyMap["name"])

	// POST /charges should have body with charge fields
	var createCharge *mhttp.HTTP
	for i := range resolved.HTTPRequests {
		if resolved.HTTPRequests[i].Name == "Create a charge" {
			createCharge = &resolved.HTTPRequests[i]
			break
		}
	}
	require.NotNil(t, createCharge, "Should find 'Create a charge' request")

	var chargeBody *mhttp.HTTPBodyRaw
	for i := range resolved.BodyRaw {
		if resolved.BodyRaw[i].HttpID == createCharge.ID {
			chargeBody = &resolved.BodyRaw[i]
			break
		}
	}
	require.NotNil(t, chargeBody, "Should have body for 'Create a charge'")

	var chargeMap map[string]interface{}
	err = json.Unmarshal(chargeBody.RawData, &chargeMap)
	require.NoError(t, err)
	require.Equal(t, "usd", chargeMap["currency"])
	require.Equal(t, "cus_abc123", chargeMap["customer"])
}

func TestRealWorld_StripeOpenAPI3_MultipleHeaders(t *testing.T) {
	path := filepath.Join("..", "..", "..", "test", "openapi", "stripe_openapi3.json")
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	opts := ConvertOptions{WorkspaceID: idwrap.NewNow()}
	resolved, err := ConvertOpenAPI(data, opts)
	require.NoError(t, err)

	// POST /customers should have Authorization + Idempotency-Key + Content-Type
	var createCustomer *mhttp.HTTP
	for i := range resolved.HTTPRequests {
		if resolved.HTTPRequests[i].Name == "Create a customer" {
			createCustomer = &resolved.HTTPRequests[i]
			break
		}
	}
	require.NotNil(t, createCustomer)

	headerMap := map[string]string{}
	for _, h := range resolved.Headers {
		if h.HttpID == createCustomer.ID {
			headerMap[h.Key] = h.Value
		}
	}
	require.Equal(t, "Bearer sk_test_123456", headerMap["Authorization"])
	require.Equal(t, "unique-key-123", headerMap["Idempotency-Key"])
	require.Equal(t, "application/json", headerMap["Content-Type"])
}

// --- Structural integrity tests ---

func TestRealWorld_FlowStructureIntegrity(t *testing.T) {
	path := filepath.Join("..", "..", "..", "test", "openapi", "stripe_openapi3.json")
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	opts := ConvertOptions{WorkspaceID: idwrap.NewNow()}
	resolved, err := ConvertOpenAPI(data, opts)
	require.NoError(t, err)

	flowID := resolved.Flow.ID

	// Every node should belong to the flow
	for _, node := range resolved.Nodes {
		require.Equal(t, flowID, node.FlowID, "Node %q should belong to flow", node.Name)
	}

	// Every edge should belong to the flow and reference valid nodes
	nodeIDs := map[idwrap.IDWrap]bool{}
	for _, node := range resolved.Nodes {
		nodeIDs[node.ID] = true
	}
	for i, edge := range resolved.Edges {
		require.Equal(t, flowID, edge.FlowID, "Edge %d should belong to flow", i)
		require.True(t, nodeIDs[edge.SourceID], "Edge %d source should be a valid node", i)
		require.True(t, nodeIDs[edge.TargetID], "Edge %d target should be a valid node", i)
	}

	// Start node should exist
	var startNode *mflow.Node
	for i := range resolved.Nodes {
		if resolved.Nodes[i].NodeKind == mflow.NODE_KIND_MANUAL_START {
			startNode = &resolved.Nodes[i]
			break
		}
	}
	require.NotNil(t, startNode, "Should have a start node")

	// First edge should come from the start node
	require.Equal(t, startNode.ID, resolved.Edges[0].SourceID, "First edge should originate from start node")

	// Each request node should reference a valid HTTP request
	for _, rn := range resolved.RequestNodes {
		require.NotNil(t, rn.HttpID, "Request node should have an HttpID")
		found := false
		for _, req := range resolved.HTTPRequests {
			if req.ID == *rn.HttpID {
				found = true
				break
			}
		}
		require.True(t, found, "Request node's HttpID should match an HTTP request")
	}
}

func TestRealWorld_FileStructureIntegrity(t *testing.T) {
	path := filepath.Join("..", "..", "..", "test", "openapi", "petstore_swagger2.json")
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	opts := ConvertOptions{WorkspaceID: idwrap.NewNow()}
	resolved, err := ConvertOpenAPI(data, opts)
	require.NoError(t, err)

	// Every HTTP request should have a corresponding file
	httpFileCount := 0
	flowFileCount := 0
	folderCount := 0
	for _, f := range resolved.Files {
		switch f.ContentType {
		case mfile.ContentTypeHTTP:
			httpFileCount++
		case mfile.ContentTypeFlow:
			flowFileCount++
		case mfile.ContentTypeFolder:
			folderCount++
		}
		require.Equal(t, opts.WorkspaceID, f.WorkspaceID, "File should belong to workspace")
	}

	require.Equal(t, len(resolved.HTTPRequests), httpFileCount,
		"Each HTTP request should have a file entry")
	require.Equal(t, 1, flowFileCount, "Should have exactly 1 flow file")
	require.Greater(t, folderCount, 0, "Should have at least 1 folder")

	t.Logf("Files: %d HTTP, %d flow, %d folders", httpFileCount, flowFileCount, folderCount)
}

func TestRealWorld_NoDuplicateIDs(t *testing.T) {
	path := filepath.Join("..", "..", "..", "test", "openapi", "stripe_openapi3.json")
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	opts := ConvertOptions{WorkspaceID: idwrap.NewNow()}
	resolved, err := ConvertOpenAPI(data, opts)
	require.NoError(t, err)

	allIDs := map[idwrap.IDWrap]string{}

	for _, req := range resolved.HTTPRequests {
		key := "http:" + req.Name
		existing, dup := allIDs[req.ID]
		require.False(t, dup, "Duplicate HTTP ID: %s and %s", existing, key)
		allIDs[req.ID] = key
	}

	for _, node := range resolved.Nodes {
		key := "node:" + node.Name
		existing, dup := allIDs[node.ID]
		require.False(t, dup, "Duplicate node ID: %s and %s", existing, key)
		allIDs[node.ID] = key
	}

	for i, edge := range resolved.Edges {
		key := "edge:" + string(rune(i))
		existing, dup := allIDs[edge.ID]
		require.False(t, dup, "Duplicate edge ID: %s and %s", existing, key)
		allIDs[edge.ID] = key
	}
}

func TestRealWorld_AllWorkspaceIDsConsistent(t *testing.T) {
	path := filepath.Join("..", "..", "..", "test", "openapi", "petstore_swagger2.json")
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	wsID := idwrap.NewNow()
	opts := ConvertOptions{WorkspaceID: wsID}
	resolved, err := ConvertOpenAPI(data, opts)
	require.NoError(t, err)

	require.Equal(t, wsID, resolved.Flow.WorkspaceID)
	for _, req := range resolved.HTTPRequests {
		require.Equal(t, wsID, req.WorkspaceID, "HTTP request %q should have correct workspace", req.Name)
	}
	for _, f := range resolved.Files {
		require.Equal(t, wsID, f.WorkspaceID, "File %q should have correct workspace", f.Name)
	}
}

// --- GET-only vs POST-body distinction ---

func TestRealWorld_GETRequestsHaveNoBodies(t *testing.T) {
	path := filepath.Join("..", "..", "..", "test", "openapi", "stripe_openapi3.json")
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	opts := ConvertOptions{WorkspaceID: idwrap.NewNow()}
	resolved, err := ConvertOpenAPI(data, opts)
	require.NoError(t, err)

	bodyHTTPIDs := map[idwrap.IDWrap]bool{}
	for _, br := range resolved.BodyRaw {
		bodyHTTPIDs[br.HttpID] = true
	}

	for _, req := range resolved.HTTPRequests {
		if req.Method == "GET" || req.Method == "DELETE" {
			require.Equal(t, mhttp.HttpBodyKindNone, req.BodyKind,
				"%s %s should have no body kind", req.Method, req.Name)
			require.False(t, bodyHTTPIDs[req.ID],
				"%s %s should have no raw body data", req.Method, req.Name)
		}
	}
}

func TestRealWorld_POSTRequestsHaveBodies(t *testing.T) {
	path := filepath.Join("..", "..", "..", "test", "openapi", "stripe_openapi3.json")
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	opts := ConvertOptions{WorkspaceID: idwrap.NewNow()}
	resolved, err := ConvertOpenAPI(data, opts)
	require.NoError(t, err)

	bodyHTTPIDs := map[idwrap.IDWrap]bool{}
	for _, br := range resolved.BodyRaw {
		bodyHTTPIDs[br.HttpID] = true
	}

	for _, req := range resolved.HTTPRequests {
		if req.Method == "POST" {
			// POST requests in the Stripe spec all have requestBody
			require.Equal(t, mhttp.HttpBodyKindRaw, req.BodyKind,
				"POST %s should have raw body kind", req.Name)
			require.True(t, bodyHTTPIDs[req.ID],
				"POST %s should have raw body data", req.Name)
		}
	}
}
