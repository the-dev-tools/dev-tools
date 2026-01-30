//go:build ai_integration

package nai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/naiprovider"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/varsystem"
)

// =============================================================================
// METRICS TRACKING
// =============================================================================

// POCMetrics tracks execution metrics for a PoC test
type POCMetrics struct {
	POCName       string
	Scenario      string
	Success       bool
	ToolCalls     int
	ErrorMessage  string
	Duration      time.Duration
	DiscoverCalls int // For POC #3 - how many discover_tools calls
}

// POCResult aggregates results for comparison
type POCResult struct {
	POC1 POCMetrics
	POC2 POCMetrics
	POC3 POCMetrics
}

// logMetrics logs the metrics for a test
func logMetrics(t *testing.T, m POCMetrics) {
	status := "✓"
	if !m.Success {
		status = "✗"
	}
	t.Logf("%s [%s] %s: %s (tool calls: %d, duration: %v)",
		status, m.POCName, m.Scenario, m.ErrorMessage, m.ToolCalls, m.Duration)
}

// =============================================================================
// MOCK NODES WITH VARIABLE INTROSPECTION AND DESCRIPTIONS
// =============================================================================

// describableMockNode implements both VariableIntrospector and DescribableNode
// This is used for all 3 PoCs to test different description approaches
type describableMockNode struct {
	ID               idwrap.IDWrap
	Name             string
	Description      string   // User-defined description for PoC #2
	RequiredVars     []string // Variables this node requires as input
	OutputVars       []string // Variables this node outputs
	RunFunc          func(req *node.FlowNodeRequest) (any, error)
	executionCounter int // Track how many times this node was executed
}

func (n *describableMockNode) GetID() idwrap.IDWrap { return n.ID }
func (n *describableMockNode) GetName() string      { return n.Name }

// GetDescription implements DescribableNode for PoC #2
func (n *describableMockNode) GetDescription() string {
	return n.Description
}

// GetRequiredVariables implements VariableIntrospector for PoC #1
func (n *describableMockNode) GetRequiredVariables() []string {
	return n.RequiredVars
}

// GetOutputVariables implements VariableIntrospector for PoC #1
func (n *describableMockNode) GetOutputVariables() []string {
	return n.OutputVars
}

func (n *describableMockNode) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	n.executionCounter++
	output, err := n.RunFunc(req)
	if err != nil {
		return node.FlowNodeResult{Err: err}
	}
	req.VarMap[n.Name] = output
	return node.FlowNodeResult{NextNodeID: []idwrap.IDWrap{}}
}

func (n *describableMockNode) RunAsync(ctx context.Context, req *node.FlowNodeRequest, res chan node.FlowNodeResult) {
	res <- n.RunSync(ctx, req)
}

// =============================================================================
// SCENARIO BUILDERS - Creates test scenarios with varying complexity
// =============================================================================

// simpleScenario creates a single HTTP node scenario
// AI must: Set userId variable, call GetUser, get user data
type simpleScenario struct {
	GetUserNode *describableMockNode
	NodeMap     map[idwrap.IDWrap]node.FlowNode
	EdgeMap     mflow.EdgesMap
	AINodeID    idwrap.IDWrap
	ProviderID  idwrap.IDWrap
}

func setupSimpleScenario(t *testing.T, useCustomDescription bool) *simpleScenario {
	aiNodeID := idwrap.NewNow()
	providerID := idwrap.NewNow()
	getUserID := idwrap.NewNow()

	// Create GetUser node - simulates HTTP call to jsonplaceholder API
	getUserNode := &describableMockNode{
		ID:           getUserID,
		Name:         "GetUser",
		RequiredVars: []string{"ai_1.userId"}, // AI must set this before calling
		OutputVars:   []string{"response.status", "response.body.id", "response.body.name", "response.body.email"},
		RunFunc: func(req *node.FlowNodeRequest) (any, error) {
			// Check if userId was set by AI
			vm := varsystem.NewVarMapFromAnyMap(req.VarMap)
			userIDStr, err := vm.ReplaceVars("{{ai_1.userId}}")
			if err != nil {
				return nil, fmt.Errorf("userId not set: %w", err)
			}

			// Simulate API response based on userId
			userID := userIDStr
			return map[string]any{
				"response": map[string]any{
					"status": 200,
					"body": map[string]any{
						"id":       userID,
						"name":     "John Doe",
						"email":    "john.doe@example.com",
						"username": "johndoe",
					},
				},
			}, nil
		},
	}

	if useCustomDescription {
		getUserNode.Description = `Fetches a user from the API by their ID.

BEFORE CALLING: You MUST set the variable 'ai_1.userId' to the user ID you want to fetch.
Example: Use set_variable with key="ai_1.userId" and value="1"

AFTER CALLING: User data is available at:
- GetUser.response.body.id - The user's ID
- GetUser.response.body.name - The user's full name
- GetUser.response.body.email - The user's email address
- GetUser.response.body.username - The user's username`
	}

	providerNode := CreateTestAiProviderNode(providerID)

	edgeMap := mflow.EdgesMap{
		aiNodeID: {
			mflow.HandleAiProvider: []idwrap.IDWrap{providerID},
			mflow.HandleAiTools:    []idwrap.IDWrap{getUserID},
		},
	}

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		providerID: providerNode,
		getUserID:  getUserNode,
	}

	return &simpleScenario{
		GetUserNode: getUserNode,
		NodeMap:     nodeMap,
		EdgeMap:     edgeMap,
		AINodeID:    aiNodeID,
		ProviderID:  providerID,
	}
}

// mediumScenario creates a 3-node chain scenario
// AI must: GetUser -> GetPosts (using userId) -> GetComments (using postId)
type mediumScenario struct {
	GetUserNode     *describableMockNode
	GetPostsNode    *describableMockNode
	GetCommentsNode *describableMockNode
	NodeMap         map[idwrap.IDWrap]node.FlowNode
	EdgeMap         mflow.EdgesMap
	AINodeID        idwrap.IDWrap
	ProviderID      idwrap.IDWrap
}

func setupMediumScenario(t *testing.T, useCustomDescription bool) *mediumScenario {
	aiNodeID := idwrap.NewNow()
	providerID := idwrap.NewNow()
	getUserID := idwrap.NewNow()
	getPostsID := idwrap.NewNow()
	getCommentsID := idwrap.NewNow()

	// Node 1: GetUser
	getUserNode := &describableMockNode{
		ID:           getUserID,
		Name:         "GetUser",
		RequiredVars: []string{"ai_1.userId"},
		OutputVars:   []string{"response.body.id", "response.body.name"},
		RunFunc: func(req *node.FlowNodeRequest) (any, error) {
			vm := varsystem.NewVarMapFromAnyMap(req.VarMap)
			userIDStr, err := vm.ReplaceVars("{{ai_1.userId}}")
			if err != nil {
				return nil, fmt.Errorf("userId not set: %w", err)
			}
			return map[string]any{
				"response": map[string]any{
					"status": 200,
					"body": map[string]any{
						"id":   userIDStr,
						"name": "Alice Smith",
					},
				},
			}, nil
		},
	}

	// Node 2: GetPosts - requires userId from GetUser
	getPostsNode := &describableMockNode{
		ID:           getPostsID,
		Name:         "GetPosts",
		RequiredVars: []string{"GetUser.response.body.id"},
		OutputVars:   []string{"response.body[0].id", "response.body[0].title"},
		RunFunc: func(req *node.FlowNodeRequest) (any, error) {
			// Check that GetUser was called first
			userData, ok := req.VarMap["GetUser"]
			if !ok {
				return nil, fmt.Errorf("GetUser must be called first")
			}
			userMap := userData.(map[string]any)
			resp := userMap["response"].(map[string]any)
			body := resp["body"].(map[string]any)
			userID := body["id"]

			return map[string]any{
				"response": map[string]any{
					"status": 200,
					"body": []map[string]any{
						{"id": "101", "title": "First Post", "userId": userID},
						{"id": "102", "title": "Second Post", "userId": userID},
					},
				},
			}, nil
		},
	}

	// Node 3: GetComments - requires postId from GetPosts
	getCommentsNode := &describableMockNode{
		ID:           getCommentsID,
		Name:         "GetComments",
		RequiredVars: []string{"ai_1.postId"},
		OutputVars:   []string{"response.body[0].body", "response.body[0].email"},
		RunFunc: func(req *node.FlowNodeRequest) (any, error) {
			vm := varsystem.NewVarMapFromAnyMap(req.VarMap)
			postIDStr, err := vm.ReplaceVars("{{ai_1.postId}}")
			if err != nil {
				return nil, fmt.Errorf("postId not set: %w", err)
			}

			return map[string]any{
				"response": map[string]any{
					"status": 200,
					"body": []map[string]any{
						{"id": "1001", "postId": postIDStr, "body": "Great post!", "email": "commenter@example.com"},
					},
				},
			}, nil
		},
	}

	if useCustomDescription {
		getUserNode.Description = `Fetches user details by ID.
REQUIRED: Set 'ai_1.userId' before calling.
OUTPUT: User data at GetUser.response.body (id, name)`

		getPostsNode.Description = `Fetches posts for a user.
REQUIRED: Call GetUser first - uses GetUser.response.body.id automatically.
OUTPUT: Array of posts at GetPosts.response.body (each has id, title, userId)`

		getCommentsNode.Description = `Fetches comments for a post.
REQUIRED: Set 'ai_1.postId' to the post ID (e.g., from GetPosts.response.body[0].id).
OUTPUT: Array of comments at GetComments.response.body (each has id, body, email)`
	}

	providerNode := CreateTestAiProviderNode(providerID)

	edgeMap := mflow.EdgesMap{
		aiNodeID: {
			mflow.HandleAiProvider: []idwrap.IDWrap{providerID},
			mflow.HandleAiTools:    []idwrap.IDWrap{getUserID, getPostsID, getCommentsID},
		},
	}

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		providerID:    providerNode,
		getUserID:     getUserNode,
		getPostsID:    getPostsNode,
		getCommentsID: getCommentsNode,
	}

	return &mediumScenario{
		GetUserNode:     getUserNode,
		GetPostsNode:    getPostsNode,
		GetCommentsNode: getCommentsNode,
		NodeMap:         nodeMap,
		EdgeMap:         edgeMap,
		AINodeID:        aiNodeID,
		ProviderID:      providerID,
	}
}

// complexScenario creates a mixed node type scenario
// AI must use: HTTP node, JS transformation node, and conditional logic
type complexScenario struct {
	FetchDataNode     *describableMockNode
	TransformNode     *describableMockNode // Simulates JS node
	ValidateNode      *describableMockNode // Simulates conditional check
	NodeMap           map[idwrap.IDWrap]node.FlowNode
	EdgeMap           mflow.EdgesMap
	AINodeID          idwrap.IDWrap
	ProviderID        idwrap.IDWrap
}

func setupComplexScenario(t *testing.T, useCustomDescription bool) *complexScenario {
	aiNodeID := idwrap.NewNow()
	providerID := idwrap.NewNow()
	fetchDataID := idwrap.NewNow()
	transformID := idwrap.NewNow()
	validateID := idwrap.NewNow()

	// Node 1: FetchData (HTTP-like)
	fetchDataNode := &describableMockNode{
		ID:           fetchDataID,
		Name:         "FetchData",
		RequiredVars: []string{"ai_1.endpoint", "ai_1.apiKey"},
		OutputVars:   []string{"response.body.items", "response.body.total"},
		RunFunc: func(req *node.FlowNodeRequest) (any, error) {
			vm := varsystem.NewVarMapFromAnyMap(req.VarMap)
			endpoint, err := vm.ReplaceVars("{{ai_1.endpoint}}")
			if err != nil {
				return nil, fmt.Errorf("endpoint not set: %w", err)
			}
			apiKey, err := vm.ReplaceVars("{{ai_1.apiKey}}")
			if err != nil {
				return nil, fmt.Errorf("apiKey not set: %w", err)
			}

			// Validate inputs
			if !strings.Contains(endpoint, "api") {
				return nil, fmt.Errorf("invalid endpoint: %s", endpoint)
			}
			if len(apiKey) < 5 {
				return nil, fmt.Errorf("invalid apiKey")
			}

			return map[string]any{
				"response": map[string]any{
					"status": 200,
					"body": map[string]any{
						"items": []map[string]any{
							{"id": "1", "value": 100, "active": true},
							{"id": "2", "value": 200, "active": false},
							{"id": "3", "value": 150, "active": true},
						},
						"total": 3,
					},
				},
			}, nil
		},
	}

	// Node 2: Transform (JS-like) - filters active items and sums values
	transformNode := &describableMockNode{
		ID:           transformID,
		Name:         "TransformData",
		RequiredVars: []string{"FetchData.response.body.items"},
		OutputVars:   []string{"result.activeItems", "result.totalValue", "result.count"},
		RunFunc: func(req *node.FlowNodeRequest) (any, error) {
			fetchData, ok := req.VarMap["FetchData"]
			if !ok {
				return nil, fmt.Errorf("FetchData must be called first")
			}

			fetchMap := fetchData.(map[string]any)
			resp := fetchMap["response"].(map[string]any)
			body := resp["body"].(map[string]any)
			items := body["items"].([]map[string]any)

			// Filter and transform
			var activeItems []map[string]any
			totalValue := 0
			for _, item := range items {
				if active, ok := item["active"].(bool); ok && active {
					activeItems = append(activeItems, item)
					if val, ok := item["value"].(int); ok {
						totalValue += val
					}
				}
			}

			return map[string]any{
				"result": map[string]any{
					"activeItems": activeItems,
					"totalValue":  totalValue,
					"count":       len(activeItems),
				},
			}, nil
		},
	}

	// Node 3: Validate - checks if total meets threshold
	validateNode := &describableMockNode{
		ID:           validateID,
		Name:         "ValidateResult",
		RequiredVars: []string{"TransformData.result.totalValue", "ai_1.threshold"},
		OutputVars:   []string{"result.valid", "result.message"},
		RunFunc: func(req *node.FlowNodeRequest) (any, error) {
			transformData, ok := req.VarMap["TransformData"]
			if !ok {
				return nil, fmt.Errorf("TransformData must be called first")
			}

			vm := varsystem.NewVarMapFromAnyMap(req.VarMap)
			thresholdStr, err := vm.ReplaceVars("{{ai_1.threshold}}")
			if err != nil {
				return nil, fmt.Errorf("threshold not set: %w", err)
			}

			transformMap := transformData.(map[string]any)
			result := transformMap["result"].(map[string]any)
			totalValue := result["totalValue"].(int)

			var threshold int
			fmt.Sscanf(thresholdStr, "%d", &threshold)

			valid := totalValue >= threshold
			message := "VALIDATION_PASSED"
			if !valid {
				message = "VALIDATION_FAILED"
			}

			return map[string]any{
				"result": map[string]any{
					"valid":      valid,
					"message":    message,
					"totalValue": totalValue,
					"threshold":  threshold,
				},
			}, nil
		},
	}

	if useCustomDescription {
		fetchDataNode.Description = `HTTP request to fetch data from an API.
REQUIRED INPUTS (set via set_variable):
- ai_1.endpoint: API endpoint URL (must contain 'api')
- ai_1.apiKey: API authentication key (min 5 chars)
OUTPUT: FetchData.response.body.items (array), FetchData.response.body.total (count)`

		transformNode.Description = `JavaScript transformation node that processes data.
REQUIRED: Call FetchData first - uses FetchData.response.body.items.
BEHAVIOR: Filters to active items only and calculates sum of values.
OUTPUT: TransformData.result.activeItems, TransformData.result.totalValue, TransformData.result.count`

		validateNode.Description = `Validation node that checks if total meets a threshold.
REQUIRED INPUTS:
- Call TransformData first (uses TransformData.result.totalValue)
- Set ai_1.threshold to your minimum acceptable value
OUTPUT: ValidateResult.result.valid (bool), ValidateResult.result.message (VALIDATION_PASSED or VALIDATION_FAILED)`
	}

	providerNode := CreateTestAiProviderNode(providerID)

	edgeMap := mflow.EdgesMap{
		aiNodeID: {
			mflow.HandleAiProvider: []idwrap.IDWrap{providerID},
			mflow.HandleAiTools:    []idwrap.IDWrap{fetchDataID, transformID, validateID},
		},
	}

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		providerID:  providerNode,
		fetchDataID: fetchDataNode,
		transformID: transformNode,
		validateID:  validateNode,
	}

	return &complexScenario{
		FetchDataNode: fetchDataNode,
		TransformNode: transformNode,
		ValidateNode:  validateNode,
		NodeMap:       nodeMap,
		EdgeMap:       edgeMap,
		AINodeID:      aiNodeID,
		ProviderID:    providerID,
	}
}

// =============================================================================
// POC #1: UNIFIED INTROSPECTION TESTS
// Uses auto-generated descriptions from VariableIntrospector
// =============================================================================

func TestPOC1_Simple_UnifiedIntrospection(t *testing.T) {
	llm := SetupGenericIntegrationTest(t)
	ctx := context.Background()
	scenario := setupSimpleScenario(t, false) // No custom description

	prompt := `You are a helpful assistant with access to tools.

Task: Fetch information about user ID 5.

Instructions:
1. First, set the userId variable to 5
2. Then call the GetUser tool
3. Tell me the user's name and email`

	aiNode := New(scenario.AINodeID, "ai_1", prompt, 10, nil)
	scenario.NodeMap[scenario.ProviderID].(*naiprovider.NodeAiProvider).LLM = llm
	scenario.NodeMap[scenario.AINodeID] = aiNode

	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
		EdgeSourceMap: scenario.EdgeMap,
		NodeMap:       scenario.NodeMap,
	}

	start := time.Now()
	res := aiNode.RunSync(ctx, req)
	duration := time.Since(start)

	metrics := POCMetrics{
		POCName:   "POC1-Introspection",
		Scenario:  "Simple",
		Success:   res.Err == nil,
		Duration:  duration,
		ToolCalls: scenario.GetUserNode.executionCounter,
	}

	if res.Err != nil {
		metrics.ErrorMessage = res.Err.Error()
	} else {
		// Verify results
		val, err := node.ReadNodeVar(req, "ai_1", "text")
		require.NoError(t, err)
		response := val.(string)
		t.Logf("AI Response: %s", response)

		// Check if user data was retrieved
		if !strings.Contains(strings.ToLower(response), "john") && !strings.Contains(strings.ToLower(response), "doe") {
			metrics.Success = false
			metrics.ErrorMessage = "AI did not retrieve user data correctly"
		}
	}

	logMetrics(t, metrics)
	assert.True(t, metrics.Success, "POC1 Simple should succeed")
}

func TestPOC1_Medium_UnifiedIntrospection(t *testing.T) {
	llm := SetupGenericIntegrationTest(t)
	ctx := context.Background()
	scenario := setupMediumScenario(t, false)

	prompt := `You are an API orchestration agent.

Task: Get comments for the first post of user ID 1.

You need to:
1. Set userId to 1 and get the user
2. Get the user's posts
3. Use the first post's ID to get comments
4. Tell me what the comment says`

	aiNode := New(scenario.AINodeID, "ai_1", prompt, 15, nil)
	scenario.NodeMap[scenario.ProviderID].(*naiprovider.NodeAiProvider).LLM = llm
	scenario.NodeMap[scenario.AINodeID] = aiNode

	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
		EdgeSourceMap: scenario.EdgeMap,
		NodeMap:       scenario.NodeMap,
	}

	start := time.Now()
	res := aiNode.RunSync(ctx, req)
	duration := time.Since(start)

	totalToolCalls := scenario.GetUserNode.executionCounter +
		scenario.GetPostsNode.executionCounter +
		scenario.GetCommentsNode.executionCounter

	metrics := POCMetrics{
		POCName:   "POC1-Introspection",
		Scenario:  "Medium",
		Success:   res.Err == nil,
		Duration:  duration,
		ToolCalls: totalToolCalls,
	}

	if res.Err != nil {
		metrics.ErrorMessage = res.Err.Error()
	} else {
		val, err := node.ReadNodeVar(req, "ai_1", "text")
		require.NoError(t, err)
		response := val.(string)
		t.Logf("AI Response: %s", response)

		if !strings.Contains(strings.ToLower(response), "great post") {
			metrics.Success = false
			metrics.ErrorMessage = "AI did not retrieve comment correctly"
		}
	}

	logMetrics(t, metrics)
	assert.True(t, metrics.Success, "POC1 Medium should succeed")
}

func TestPOC1_Complex_UnifiedIntrospection(t *testing.T) {
	llm := SetupGenericIntegrationTest(t)
	ctx := context.Background()
	scenario := setupComplexScenario(t, false)

	prompt := `You are a data pipeline orchestration agent.

Task: Fetch data, transform it, and validate the result.

Steps:
1. Set endpoint to "/api/v1/data" and apiKey to "secret123"
2. Call FetchData to get items
3. Call TransformData to filter active items and sum values
4. Set threshold to 200 and call ValidateResult
5. Tell me if validation passed and what the total value was`

	aiNode := New(scenario.AINodeID, "ai_1", prompt, 20, nil)
	scenario.NodeMap[scenario.ProviderID].(*naiprovider.NodeAiProvider).LLM = llm
	scenario.NodeMap[scenario.AINodeID] = aiNode

	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
		EdgeSourceMap: scenario.EdgeMap,
		NodeMap:       scenario.NodeMap,
	}

	start := time.Now()
	res := aiNode.RunSync(ctx, req)
	duration := time.Since(start)

	totalToolCalls := scenario.FetchDataNode.executionCounter +
		scenario.TransformNode.executionCounter +
		scenario.ValidateNode.executionCounter

	metrics := POCMetrics{
		POCName:   "POC1-Introspection",
		Scenario:  "Complex",
		Success:   res.Err == nil,
		Duration:  duration,
		ToolCalls: totalToolCalls,
	}

	if res.Err != nil {
		metrics.ErrorMessage = res.Err.Error()
	} else {
		val, err := node.ReadNodeVar(req, "ai_1", "text")
		require.NoError(t, err)
		response := val.(string)
		t.Logf("AI Response: %s", response)

		if !strings.Contains(strings.ToUpper(response), "PASSED") && !strings.Contains(response, "250") {
			metrics.Success = false
			metrics.ErrorMessage = "AI did not complete pipeline correctly"
		}
	}

	logMetrics(t, metrics)
	assert.True(t, metrics.Success, "POC1 Complex should succeed")
}

// =============================================================================
// POC #2: USER-DEFINED DESCRIPTION TESTS
// Uses custom Description field on nodes
// =============================================================================

func TestPOC2_Simple_UserDescription(t *testing.T) {
	llm := SetupGenericIntegrationTest(t)
	ctx := context.Background()
	scenario := setupSimpleScenario(t, true) // WITH custom description

	prompt := `You are a helpful assistant with access to tools.
Task: Fetch information about user ID 5 and tell me their name and email.`

	aiNode := New(scenario.AINodeID, "ai_1", prompt, 10, nil)
	scenario.NodeMap[scenario.ProviderID].(*naiprovider.NodeAiProvider).LLM = llm
	scenario.NodeMap[scenario.AINodeID] = aiNode

	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
		EdgeSourceMap: scenario.EdgeMap,
		NodeMap:       scenario.NodeMap,
	}

	start := time.Now()
	res := aiNode.RunSync(ctx, req)
	duration := time.Since(start)

	metrics := POCMetrics{
		POCName:   "POC2-UserDescription",
		Scenario:  "Simple",
		Success:   res.Err == nil,
		Duration:  duration,
		ToolCalls: scenario.GetUserNode.executionCounter,
	}

	if res.Err != nil {
		metrics.ErrorMessage = res.Err.Error()
	} else {
		val, err := node.ReadNodeVar(req, "ai_1", "text")
		require.NoError(t, err)
		response := val.(string)
		t.Logf("AI Response: %s", response)

		if !strings.Contains(strings.ToLower(response), "john") {
			metrics.Success = false
			metrics.ErrorMessage = "AI did not retrieve user data correctly"
		}
	}

	logMetrics(t, metrics)
	assert.True(t, metrics.Success, "POC2 Simple should succeed")
}

func TestPOC2_Medium_UserDescription(t *testing.T) {
	llm := SetupGenericIntegrationTest(t)
	ctx := context.Background()
	scenario := setupMediumScenario(t, true)

	prompt := `You are an API orchestration agent.
Task: Get comments for the first post of user ID 1. Tell me what the comment says.`

	aiNode := New(scenario.AINodeID, "ai_1", prompt, 15, nil)
	scenario.NodeMap[scenario.ProviderID].(*naiprovider.NodeAiProvider).LLM = llm
	scenario.NodeMap[scenario.AINodeID] = aiNode

	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
		EdgeSourceMap: scenario.EdgeMap,
		NodeMap:       scenario.NodeMap,
	}

	start := time.Now()
	res := aiNode.RunSync(ctx, req)
	duration := time.Since(start)

	totalToolCalls := scenario.GetUserNode.executionCounter +
		scenario.GetPostsNode.executionCounter +
		scenario.GetCommentsNode.executionCounter

	metrics := POCMetrics{
		POCName:   "POC2-UserDescription",
		Scenario:  "Medium",
		Success:   res.Err == nil,
		Duration:  duration,
		ToolCalls: totalToolCalls,
	}

	if res.Err != nil {
		metrics.ErrorMessage = res.Err.Error()
	} else {
		val, err := node.ReadNodeVar(req, "ai_1", "text")
		require.NoError(t, err)
		response := val.(string)
		t.Logf("AI Response: %s", response)

		if !strings.Contains(strings.ToLower(response), "great post") {
			metrics.Success = false
			metrics.ErrorMessage = "AI did not retrieve comment correctly"
		}
	}

	logMetrics(t, metrics)
	assert.True(t, metrics.Success, "POC2 Medium should succeed")
}

func TestPOC2_Complex_UserDescription(t *testing.T) {
	llm := SetupGenericIntegrationTest(t)
	ctx := context.Background()
	scenario := setupComplexScenario(t, true)

	prompt := `You are a data pipeline orchestration agent.
Task: Fetch data from "/api/v1/data" with apiKey "secret123", transform it, validate with threshold 200.
Tell me if validation passed and the total value.`

	aiNode := New(scenario.AINodeID, "ai_1", prompt, 20, nil)
	scenario.NodeMap[scenario.ProviderID].(*naiprovider.NodeAiProvider).LLM = llm
	scenario.NodeMap[scenario.AINodeID] = aiNode

	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
		EdgeSourceMap: scenario.EdgeMap,
		NodeMap:       scenario.NodeMap,
	}

	start := time.Now()
	res := aiNode.RunSync(ctx, req)
	duration := time.Since(start)

	totalToolCalls := scenario.FetchDataNode.executionCounter +
		scenario.TransformNode.executionCounter +
		scenario.ValidateNode.executionCounter

	metrics := POCMetrics{
		POCName:   "POC2-UserDescription",
		Scenario:  "Complex",
		Success:   res.Err == nil,
		Duration:  duration,
		ToolCalls: totalToolCalls,
	}

	if res.Err != nil {
		metrics.ErrorMessage = res.Err.Error()
	} else {
		val, err := node.ReadNodeVar(req, "ai_1", "text")
		require.NoError(t, err)
		response := val.(string)
		t.Logf("AI Response: %s", response)

		if !strings.Contains(strings.ToUpper(response), "PASSED") && !strings.Contains(response, "250") {
			metrics.Success = false
			metrics.ErrorMessage = "AI did not complete pipeline correctly"
		}
	}

	logMetrics(t, metrics)
	assert.True(t, metrics.Success, "POC2 Complex should succeed")
}

// =============================================================================
// POC #3: DISCOVERY TOOL TESTS
// AI can call discover_tools to learn about available tools dynamically
// =============================================================================

func TestPOC3_Simple_DiscoveryTool(t *testing.T) {
	llm := SetupGenericIntegrationTest(t)
	ctx := context.Background()
	scenario := setupSimpleScenario(t, true) // Use descriptions for discovery

	// For POC3, we tell the AI about discover_tools but start minimal
	prompt := `You are a helpful assistant with tools available.

Task: Fetch information about user ID 5 and tell me their name and email.

You have a 'discover_tools' function that lists available tools and how to use them.
Consider using it first to understand what tools are available.`

	aiNode := New(scenario.AINodeID, "ai_1", prompt, 12, nil)
	scenario.NodeMap[scenario.ProviderID].(*naiprovider.NodeAiProvider).LLM = llm
	aiNode.EnableDiscoveryTool = true // Enable discover_tools for this test
	scenario.NodeMap[scenario.AINodeID] = aiNode

	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
		EdgeSourceMap: scenario.EdgeMap,
		NodeMap:       scenario.NodeMap,
	}

	start := time.Now()
	res := aiNode.RunSync(ctx, req)
	duration := time.Since(start)

	metrics := POCMetrics{
		POCName:       "POC3-DiscoveryTool",
		Scenario:      "Simple",
		Success:       res.Err == nil,
		Duration:      duration,
		ToolCalls:     scenario.GetUserNode.executionCounter,
		DiscoverCalls: aiNode.DiscoverToolCalls,
	}

	if res.Err != nil {
		metrics.ErrorMessage = res.Err.Error()
	} else {
		val, err := node.ReadNodeVar(req, "ai_1", "text")
		require.NoError(t, err)
		response := val.(string)
		t.Logf("AI Response: %s", response)
		t.Logf("Discovery tool called %d times", aiNode.DiscoverToolCalls)

		if !strings.Contains(strings.ToLower(response), "john") {
			metrics.Success = false
			metrics.ErrorMessage = "AI did not retrieve user data correctly"
		}
	}

	logMetrics(t, metrics)
	assert.True(t, metrics.Success, "POC3 Simple should succeed")
}

func TestPOC3_Medium_DiscoveryTool(t *testing.T) {
	llm := SetupGenericIntegrationTest(t)
	ctx := context.Background()
	scenario := setupMediumScenario(t, true)

	prompt := `You are an API orchestration agent.

Task: Get comments for the first post of user ID 1. Tell me what the comment says.

You have a 'discover_tools' function to learn about available tools.`

	aiNode := New(scenario.AINodeID, "ai_1", prompt, 18, nil)
	scenario.NodeMap[scenario.ProviderID].(*naiprovider.NodeAiProvider).LLM = llm
	aiNode.EnableDiscoveryTool = true
	scenario.NodeMap[scenario.AINodeID] = aiNode

	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
		EdgeSourceMap: scenario.EdgeMap,
		NodeMap:       scenario.NodeMap,
	}

	start := time.Now()
	res := aiNode.RunSync(ctx, req)
	duration := time.Since(start)

	totalToolCalls := scenario.GetUserNode.executionCounter +
		scenario.GetPostsNode.executionCounter +
		scenario.GetCommentsNode.executionCounter

	metrics := POCMetrics{
		POCName:       "POC3-DiscoveryTool",
		Scenario:      "Medium",
		Success:       res.Err == nil,
		Duration:      duration,
		ToolCalls:     totalToolCalls,
		DiscoverCalls: aiNode.DiscoverToolCalls,
	}

	if res.Err != nil {
		metrics.ErrorMessage = res.Err.Error()
	} else {
		val, err := node.ReadNodeVar(req, "ai_1", "text")
		require.NoError(t, err)
		response := val.(string)
		t.Logf("AI Response: %s", response)
		t.Logf("Discovery tool called %d times", aiNode.DiscoverToolCalls)

		if !strings.Contains(strings.ToLower(response), "great post") {
			metrics.Success = false
			metrics.ErrorMessage = "AI did not retrieve comment correctly"
		}
	}

	logMetrics(t, metrics)
	assert.True(t, metrics.Success, "POC3 Medium should succeed")
}

func TestPOC3_Complex_DiscoveryTool(t *testing.T) {
	llm := SetupGenericIntegrationTest(t)
	ctx := context.Background()
	scenario := setupComplexScenario(t, true)

	prompt := `You are a data pipeline orchestration agent.

Task: Fetch data from "/api/v1/data" with apiKey "secret123", transform it, validate with threshold 200.
Tell me if validation passed and the total value.

You have a 'discover_tools' function to learn about available tools and their requirements.`

	aiNode := New(scenario.AINodeID, "ai_1", prompt, 25, nil)
	scenario.NodeMap[scenario.ProviderID].(*naiprovider.NodeAiProvider).LLM = llm
	aiNode.EnableDiscoveryTool = true
	scenario.NodeMap[scenario.AINodeID] = aiNode

	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
		EdgeSourceMap: scenario.EdgeMap,
		NodeMap:       scenario.NodeMap,
	}

	start := time.Now()
	res := aiNode.RunSync(ctx, req)
	duration := time.Since(start)

	totalToolCalls := scenario.FetchDataNode.executionCounter +
		scenario.TransformNode.executionCounter +
		scenario.ValidateNode.executionCounter

	metrics := POCMetrics{
		POCName:       "POC3-DiscoveryTool",
		Scenario:      "Complex",
		Success:       res.Err == nil,
		Duration:      duration,
		ToolCalls:     totalToolCalls,
		DiscoverCalls: aiNode.DiscoverToolCalls,
	}

	if res.Err != nil {
		metrics.ErrorMessage = res.Err.Error()
	} else {
		val, err := node.ReadNodeVar(req, "ai_1", "text")
		require.NoError(t, err)
		response := val.(string)
		t.Logf("AI Response: %s", response)
		t.Logf("Discovery tool called %d times", aiNode.DiscoverToolCalls)

		if !strings.Contains(strings.ToUpper(response), "PASSED") && !strings.Contains(response, "250") {
			metrics.Success = false
			metrics.ErrorMessage = "AI did not complete pipeline correctly"
		}
	}

	logMetrics(t, metrics)
	assert.True(t, metrics.Success, "POC3 Complex should succeed")
}

// =============================================================================
// POC #4: AI PARAM TYPED SYNTAX TESTS
// Uses {{ ai('name', 'description', 'type') }} syntax for rich type info
// =============================================================================

// aiParamMockNode implements AIParamProvider for POC #4
// Uses the {{ ai('name', 'desc', 'type') }} syntax for typed parameters
type aiParamMockNode struct {
	ID               idwrap.IDWrap
	Name             string
	AIParams         []AIParam // Typed AI parameters
	OutputVars       []string
	RunFunc          func(req *node.FlowNodeRequest) (any, error)
	executionCounter int
}

func (n *aiParamMockNode) GetID() idwrap.IDWrap { return n.ID }
func (n *aiParamMockNode) GetName() string      { return n.Name }

// GetAIParams implements AIParamProvider for POC #4
func (n *aiParamMockNode) GetAIParams() []AIParam {
	return n.AIParams
}

// GetRequiredVariables implements VariableIntrospector
func (n *aiParamMockNode) GetRequiredVariables() []string {
	return ExtractAIParamNames("ai_1", n.AIParams)
}

// GetOutputVariables implements VariableIntrospector
func (n *aiParamMockNode) GetOutputVariables() []string {
	return n.OutputVars
}

func (n *aiParamMockNode) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	n.executionCounter++
	output, err := n.RunFunc(req)
	if err != nil {
		return node.FlowNodeResult{Err: err}
	}
	req.VarMap[n.Name] = output
	return node.FlowNodeResult{NextNodeID: []idwrap.IDWrap{}}
}

func (n *aiParamMockNode) RunAsync(ctx context.Context, req *node.FlowNodeRequest, res chan node.FlowNodeResult) {
	res <- n.RunSync(ctx, req)
}

// simpleScenarioPOC4 uses AI param syntax
type simpleScenarioPOC4 struct {
	GetUserNode *aiParamMockNode
	NodeMap     map[idwrap.IDWrap]node.FlowNode
	EdgeMap     mflow.EdgesMap
	AINodeID    idwrap.IDWrap
	ProviderID  idwrap.IDWrap
}

func setupSimpleScenarioPOC4(t *testing.T) *simpleScenarioPOC4 {
	aiNodeID := idwrap.NewNow()
	providerID := idwrap.NewNow()
	getUserID := idwrap.NewNow()

	// Node with typed AI params: {{ ai('userId', 'The user ID to fetch', 'number') }}
	getUserNode := &aiParamMockNode{
		ID:   getUserID,
		Name: "GetUser",
		AIParams: []AIParam{
			{Name: "userId", Description: "The user ID to fetch from the API", Type: AIParamTypeNumber, Required: true},
		},
		OutputVars: []string{"response.body.id", "response.body.name", "response.body.email"},
		RunFunc: func(req *node.FlowNodeRequest) (any, error) {
			vm := varsystem.NewVarMapFromAnyMap(req.VarMap)
			userIDStr, err := vm.ReplaceVars("{{ai_1.userId}}")
			if err != nil {
				return nil, fmt.Errorf("userId not set: %w", err)
			}
			return map[string]any{
				"response": map[string]any{
					"status": 200,
					"body": map[string]any{
						"id":    userIDStr,
						"name":  "John Doe",
						"email": "john.doe@example.com",
					},
				},
			}, nil
		},
	}

	providerNode := CreateTestAiProviderNode(providerID)

	edgeMap := mflow.EdgesMap{
		aiNodeID: {
			mflow.HandleAiProvider: []idwrap.IDWrap{providerID},
			mflow.HandleAiTools:    []idwrap.IDWrap{getUserID},
		},
	}

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		providerID: providerNode,
		getUserID:  getUserNode,
	}

	return &simpleScenarioPOC4{
		GetUserNode: getUserNode,
		NodeMap:     nodeMap,
		EdgeMap:     edgeMap,
		AINodeID:    aiNodeID,
		ProviderID:  providerID,
	}
}

// mediumScenarioPOC4 uses AI param syntax for chained operations
type mediumScenarioPOC4 struct {
	GetUserNode     *aiParamMockNode
	GetPostsNode    *aiParamMockNode
	GetCommentsNode *aiParamMockNode
	NodeMap         map[idwrap.IDWrap]node.FlowNode
	EdgeMap         mflow.EdgesMap
	AINodeID        idwrap.IDWrap
	ProviderID      idwrap.IDWrap
}

func setupMediumScenarioPOC4(t *testing.T) *mediumScenarioPOC4 {
	aiNodeID := idwrap.NewNow()
	providerID := idwrap.NewNow()
	getUserID := idwrap.NewNow()
	getPostsID := idwrap.NewNow()
	getCommentsID := idwrap.NewNow()

	getUserNode := &aiParamMockNode{
		ID:   getUserID,
		Name: "GetUser",
		AIParams: []AIParam{
			{Name: "userId", Description: "The user ID to fetch", Type: AIParamTypeNumber, Required: true},
		},
		OutputVars: []string{"response.body.id", "response.body.name"},
		RunFunc: func(req *node.FlowNodeRequest) (any, error) {
			vm := varsystem.NewVarMapFromAnyMap(req.VarMap)
			userIDStr, err := vm.ReplaceVars("{{ai_1.userId}}")
			if err != nil {
				return nil, fmt.Errorf("userId not set: %w", err)
			}
			return map[string]any{
				"response": map[string]any{
					"status": 200,
					"body":   map[string]any{"id": userIDStr, "name": "Alice Smith"},
				},
			}, nil
		},
	}

	getPostsNode := &aiParamMockNode{
		ID:         getPostsID,
		Name:       "GetPosts",
		AIParams:   []AIParam{}, // No AI params - reads from GetUser output
		OutputVars: []string{"response.body[0].id", "response.body[0].title"},
		RunFunc: func(req *node.FlowNodeRequest) (any, error) {
			userData, ok := req.VarMap["GetUser"]
			if !ok {
				return nil, fmt.Errorf("GetUser must be called first")
			}
			userMap := userData.(map[string]any)
			resp := userMap["response"].(map[string]any)
			body := resp["body"].(map[string]any)
			userID := body["id"]
			return map[string]any{
				"response": map[string]any{
					"status": 200,
					"body": []map[string]any{
						{"id": "101", "title": "First Post", "userId": userID},
					},
				},
			}, nil
		},
	}

	getCommentsNode := &aiParamMockNode{
		ID:   getCommentsID,
		Name: "GetComments",
		AIParams: []AIParam{
			{Name: "postId", Description: "The post ID to get comments for (use value from GetPosts.response.body[0].id)", Type: AIParamTypeString, Required: true},
		},
		OutputVars: []string{"response.body[0].body", "response.body[0].email"},
		RunFunc: func(req *node.FlowNodeRequest) (any, error) {
			vm := varsystem.NewVarMapFromAnyMap(req.VarMap)
			postIDStr, err := vm.ReplaceVars("{{ai_1.postId}}")
			if err != nil {
				return nil, fmt.Errorf("postId not set: %w", err)
			}
			return map[string]any{
				"response": map[string]any{
					"status": 200,
					"body": []map[string]any{
						{"id": "1001", "postId": postIDStr, "body": "Great post!", "email": "commenter@example.com"},
					},
				},
			}, nil
		},
	}

	providerNode := CreateTestAiProviderNode(providerID)

	edgeMap := mflow.EdgesMap{
		aiNodeID: {
			mflow.HandleAiProvider: []idwrap.IDWrap{providerID},
			mflow.HandleAiTools:    []idwrap.IDWrap{getUserID, getPostsID, getCommentsID},
		},
	}

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		providerID:    providerNode,
		getUserID:     getUserNode,
		getPostsID:    getPostsNode,
		getCommentsID: getCommentsNode,
	}

	return &mediumScenarioPOC4{
		GetUserNode:     getUserNode,
		GetPostsNode:    getPostsNode,
		GetCommentsNode: getCommentsNode,
		NodeMap:         nodeMap,
		EdgeMap:         edgeMap,
		AINodeID:        aiNodeID,
		ProviderID:      providerID,
	}
}

// complexScenarioPOC4 uses AI param syntax with multiple typed params
type complexScenarioPOC4 struct {
	FetchDataNode *aiParamMockNode
	TransformNode *aiParamMockNode
	ValidateNode  *aiParamMockNode
	NodeMap       map[idwrap.IDWrap]node.FlowNode
	EdgeMap       mflow.EdgesMap
	AINodeID      idwrap.IDWrap
	ProviderID    idwrap.IDWrap
}

func setupComplexScenarioPOC4(t *testing.T) *complexScenarioPOC4 {
	aiNodeID := idwrap.NewNow()
	providerID := idwrap.NewNow()
	fetchDataID := idwrap.NewNow()
	transformID := idwrap.NewNow()
	validateID := idwrap.NewNow()

	fetchDataNode := &aiParamMockNode{
		ID:   fetchDataID,
		Name: "FetchData",
		AIParams: []AIParam{
			{Name: "endpoint", Description: "API endpoint URL (must contain 'api')", Type: AIParamTypeString, Required: true},
			{Name: "apiKey", Description: "API authentication key (minimum 5 characters)", Type: AIParamTypeString, Required: true},
		},
		OutputVars: []string{"response.body.items", "response.body.total"},
		RunFunc: func(req *node.FlowNodeRequest) (any, error) {
			vm := varsystem.NewVarMapFromAnyMap(req.VarMap)
			endpoint, err := vm.ReplaceVars("{{ai_1.endpoint}}")
			if err != nil {
				return nil, fmt.Errorf("endpoint not set: %w", err)
			}
			apiKey, err := vm.ReplaceVars("{{ai_1.apiKey}}")
			if err != nil {
				return nil, fmt.Errorf("apiKey not set: %w", err)
			}
			if !strings.Contains(endpoint, "api") {
				return nil, fmt.Errorf("invalid endpoint: %s", endpoint)
			}
			if len(apiKey) < 5 {
				return nil, fmt.Errorf("invalid apiKey")
			}
			return map[string]any{
				"response": map[string]any{
					"status": 200,
					"body": map[string]any{
						"items": []map[string]any{
							{"id": "1", "value": 100, "active": true},
							{"id": "2", "value": 200, "active": false},
							{"id": "3", "value": 150, "active": true},
						},
						"total": 3,
					},
				},
			}, nil
		},
	}

	transformNode := &aiParamMockNode{
		ID:         transformID,
		Name:       "TransformData",
		AIParams:   []AIParam{}, // Reads from FetchData output
		OutputVars: []string{"result.activeItems", "result.totalValue", "result.count"},
		RunFunc: func(req *node.FlowNodeRequest) (any, error) {
			fetchData, ok := req.VarMap["FetchData"]
			if !ok {
				return nil, fmt.Errorf("FetchData must be called first")
			}
			fetchMap := fetchData.(map[string]any)
			resp := fetchMap["response"].(map[string]any)
			body := resp["body"].(map[string]any)
			items := body["items"].([]map[string]any)

			var activeItems []map[string]any
			totalValue := 0
			for _, item := range items {
				if active, ok := item["active"].(bool); ok && active {
					activeItems = append(activeItems, item)
					if val, ok := item["value"].(int); ok {
						totalValue += val
					}
				}
			}
			return map[string]any{
				"result": map[string]any{
					"activeItems": activeItems,
					"totalValue":  totalValue,
					"count":       len(activeItems),
				},
			}, nil
		},
	}

	validateNode := &aiParamMockNode{
		ID:   validateID,
		Name: "ValidateResult",
		AIParams: []AIParam{
			{Name: "threshold", Description: "Minimum acceptable total value", Type: AIParamTypeNumber, Required: true},
		},
		OutputVars: []string{"result.valid", "result.message"},
		RunFunc: func(req *node.FlowNodeRequest) (any, error) {
			transformData, ok := req.VarMap["TransformData"]
			if !ok {
				return nil, fmt.Errorf("TransformData must be called first")
			}
			vm := varsystem.NewVarMapFromAnyMap(req.VarMap)
			thresholdStr, err := vm.ReplaceVars("{{ai_1.threshold}}")
			if err != nil {
				return nil, fmt.Errorf("threshold not set: %w", err)
			}
			transformMap := transformData.(map[string]any)
			result := transformMap["result"].(map[string]any)
			totalValue := result["totalValue"].(int)

			var threshold int
			fmt.Sscanf(thresholdStr, "%d", &threshold)
			valid := totalValue >= threshold
			message := "VALIDATION_PASSED"
			if !valid {
				message = "VALIDATION_FAILED"
			}
			return map[string]any{
				"result": map[string]any{
					"valid":      valid,
					"message":    message,
					"totalValue": totalValue,
					"threshold":  threshold,
				},
			}, nil
		},
	}

	providerNode := CreateTestAiProviderNode(providerID)

	edgeMap := mflow.EdgesMap{
		aiNodeID: {
			mflow.HandleAiProvider: []idwrap.IDWrap{providerID},
			mflow.HandleAiTools:    []idwrap.IDWrap{fetchDataID, transformID, validateID},
		},
	}

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		providerID:  providerNode,
		fetchDataID: fetchDataNode,
		transformID: transformNode,
		validateID:  validateNode,
	}

	return &complexScenarioPOC4{
		FetchDataNode: fetchDataNode,
		TransformNode: transformNode,
		ValidateNode:  validateNode,
		NodeMap:       nodeMap,
		EdgeMap:       edgeMap,
		AINodeID:      aiNodeID,
		ProviderID:    providerID,
	}
}

func TestPOC4_Simple_AIParamSyntax(t *testing.T) {
	llm := SetupGenericIntegrationTest(t)
	ctx := context.Background()
	scenario := setupSimpleScenarioPOC4(t)

	// Minimal prompt - AI should understand from typed params
	prompt := `Task: Fetch information about user ID 5 and tell me their name and email.`

	aiNode := New(scenario.AINodeID, "ai_1", prompt, 10, nil)
	scenario.NodeMap[scenario.ProviderID].(*naiprovider.NodeAiProvider).LLM = llm
	scenario.NodeMap[scenario.AINodeID] = aiNode

	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
		EdgeSourceMap: scenario.EdgeMap,
		NodeMap:       scenario.NodeMap,
	}

	start := time.Now()
	res := aiNode.RunSync(ctx, req)
	duration := time.Since(start)

	metrics := POCMetrics{
		POCName:   "POC4-AIParamSyntax",
		Scenario:  "Simple",
		Success:   res.Err == nil,
		Duration:  duration,
		ToolCalls: scenario.GetUserNode.executionCounter,
	}

	if res.Err != nil {
		metrics.ErrorMessage = res.Err.Error()
	} else {
		val, err := node.ReadNodeVar(req, "ai_1", "text")
		require.NoError(t, err)
		response := val.(string)
		t.Logf("AI Response: %s", response)

		if !strings.Contains(strings.ToLower(response), "john") {
			metrics.Success = false
			metrics.ErrorMessage = "AI did not retrieve user data correctly"
		}
	}

	logMetrics(t, metrics)
	assert.True(t, metrics.Success, "POC4 Simple should succeed")
}

func TestPOC4_Medium_AIParamSyntax(t *testing.T) {
	llm := SetupGenericIntegrationTest(t)
	ctx := context.Background()
	scenario := setupMediumScenarioPOC4(t)

	prompt := `Task: Get comments for the first post of user ID 1. Tell me what the comment says.`

	aiNode := New(scenario.AINodeID, "ai_1", prompt, 15, nil)
	scenario.NodeMap[scenario.ProviderID].(*naiprovider.NodeAiProvider).LLM = llm
	scenario.NodeMap[scenario.AINodeID] = aiNode

	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
		EdgeSourceMap: scenario.EdgeMap,
		NodeMap:       scenario.NodeMap,
	}

	start := time.Now()
	res := aiNode.RunSync(ctx, req)
	duration := time.Since(start)

	totalToolCalls := scenario.GetUserNode.executionCounter +
		scenario.GetPostsNode.executionCounter +
		scenario.GetCommentsNode.executionCounter

	metrics := POCMetrics{
		POCName:   "POC4-AIParamSyntax",
		Scenario:  "Medium",
		Success:   res.Err == nil,
		Duration:  duration,
		ToolCalls: totalToolCalls,
	}

	if res.Err != nil {
		metrics.ErrorMessage = res.Err.Error()
	} else {
		val, err := node.ReadNodeVar(req, "ai_1", "text")
		require.NoError(t, err)
		response := val.(string)
		t.Logf("AI Response: %s", response)

		if !strings.Contains(strings.ToLower(response), "great post") {
			metrics.Success = false
			metrics.ErrorMessage = "AI did not retrieve comment correctly"
		}
	}

	logMetrics(t, metrics)
	assert.True(t, metrics.Success, "POC4 Medium should succeed")
}

func TestPOC4_Complex_AIParamSyntax(t *testing.T) {
	llm := SetupGenericIntegrationTest(t)
	ctx := context.Background()
	scenario := setupComplexScenarioPOC4(t)

	prompt := `Task: Fetch data from "/api/v1/data" with apiKey "secret123", transform it, validate with threshold 200.
Tell me if validation passed and the total value.`

	aiNode := New(scenario.AINodeID, "ai_1", prompt, 20, nil)
	scenario.NodeMap[scenario.ProviderID].(*naiprovider.NodeAiProvider).LLM = llm
	scenario.NodeMap[scenario.AINodeID] = aiNode

	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
		EdgeSourceMap: scenario.EdgeMap,
		NodeMap:       scenario.NodeMap,
	}

	start := time.Now()
	res := aiNode.RunSync(ctx, req)
	duration := time.Since(start)

	totalToolCalls := scenario.FetchDataNode.executionCounter +
		scenario.TransformNode.executionCounter +
		scenario.ValidateNode.executionCounter

	metrics := POCMetrics{
		POCName:   "POC4-AIParamSyntax",
		Scenario:  "Complex",
		Success:   res.Err == nil,
		Duration:  duration,
		ToolCalls: totalToolCalls,
	}

	if res.Err != nil {
		metrics.ErrorMessage = res.Err.Error()
	} else {
		val, err := node.ReadNodeVar(req, "ai_1", "text")
		require.NoError(t, err)
		response := val.(string)
		t.Logf("AI Response: %s", response)

		if !strings.Contains(strings.ToUpper(response), "PASSED") && !strings.Contains(response, "250") {
			metrics.Success = false
			metrics.ErrorMessage = "AI did not complete pipeline correctly"
		}
	}

	logMetrics(t, metrics)
	assert.True(t, metrics.Success, "POC4 Complex should succeed")
}

// =============================================================================
// POC #5: AUTO-CHAINING WITH SOURCE HINTS
// Uses {{ ai('name', 'desc', 'type', 'source') }} for automatic data flow
// =============================================================================

// simpleScenarioPOC5 uses auto-chaining (same as POC4 for simple case)
type simpleScenarioPOC5 struct {
	GetUserNode *aiParamMockNode
	NodeMap     map[idwrap.IDWrap]node.FlowNode
	EdgeMap     mflow.EdgesMap
	AINodeID    idwrap.IDWrap
	ProviderID  idwrap.IDWrap
}

func setupSimpleScenarioPOC5(t *testing.T) *simpleScenarioPOC5 {
	aiNodeID := idwrap.NewNow()
	providerID := idwrap.NewNow()
	getUserID := idwrap.NewNow()

	// Simple scenario - no chaining needed, same as POC4
	getUserNode := &aiParamMockNode{
		ID:   getUserID,
		Name: "GetUser",
		AIParams: []AIParam{
			{Name: "userId", Description: "The user ID to fetch", Type: AIParamTypeNumber, Required: true},
		},
		OutputVars: []string{"response.body.id", "response.body.name", "response.body.email"},
		RunFunc: func(req *node.FlowNodeRequest) (any, error) {
			vm := varsystem.NewVarMapFromAnyMap(req.VarMap)
			userIDStr, err := vm.ReplaceVars("{{ai_1.userId}}")
			if err != nil {
				return nil, fmt.Errorf("userId not set: %w", err)
			}
			return map[string]any{
				"response": map[string]any{
					"status": 200,
					"body": map[string]any{
						"id":    userIDStr,
						"name":  "John Doe",
						"email": "john.doe@example.com",
					},
				},
			}, nil
		},
	}

	providerNode := CreateTestAiProviderNode(providerID)

	edgeMap := mflow.EdgesMap{
		aiNodeID: {
			mflow.HandleAiProvider: []idwrap.IDWrap{providerID},
			mflow.HandleAiTools:    []idwrap.IDWrap{getUserID},
		},
	}

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		providerID: providerNode,
		getUserID:  getUserNode,
	}

	return &simpleScenarioPOC5{
		GetUserNode: getUserNode,
		NodeMap:     nodeMap,
		EdgeMap:     edgeMap,
		AINodeID:    aiNodeID,
		ProviderID:  providerID,
	}
}

// mediumScenarioPOC5 uses auto-chaining with source hints
type mediumScenarioPOC5 struct {
	GetUserNode     *aiParamMockNode
	GetPostsNode    *aiParamMockNode
	GetCommentsNode *aiParamMockNode
	NodeMap         map[idwrap.IDWrap]node.FlowNode
	EdgeMap         mflow.EdgesMap
	AINodeID        idwrap.IDWrap
	ProviderID      idwrap.IDWrap
}

func setupMediumScenarioPOC5(t *testing.T) *mediumScenarioPOC5 {
	aiNodeID := idwrap.NewNow()
	providerID := idwrap.NewNow()
	getUserID := idwrap.NewNow()
	getPostsID := idwrap.NewNow()
	getCommentsID := idwrap.NewNow()

	// Node 1: GetUser - takes userId, outputs user data
	getUserNode := &aiParamMockNode{
		ID:   getUserID,
		Name: "GetUser",
		AIParams: []AIParam{
			{Name: "userId", Description: "The user ID to fetch", Type: AIParamTypeNumber, Required: true},
		},
		OutputVars: []string{"response.body.id", "response.body.name"},
		RunFunc: func(req *node.FlowNodeRequest) (any, error) {
			vm := varsystem.NewVarMapFromAnyMap(req.VarMap)
			userIDStr, err := vm.ReplaceVars("{{ai_1.userId}}")
			if err != nil {
				return nil, fmt.Errorf("userId not set: %w", err)
			}
			return map[string]any{
				"response": map[string]any{
					"status": 200,
					"body":   map[string]any{"id": userIDStr, "name": "Alice Smith"},
				},
			}, nil
		},
	}

	// Node 2: GetPosts - NO input params, reads from GetUser automatically
	getPostsNode := &aiParamMockNode{
		ID:         getPostsID,
		Name:       "GetPosts",
		AIParams:   []AIParam{}, // No params - auto-reads from previous
		OutputVars: []string{"response.body[0].id", "response.body[0].title"},
		RunFunc: func(req *node.FlowNodeRequest) (any, error) {
			userData, ok := req.VarMap["GetUser"]
			if !ok {
				return nil, fmt.Errorf("GetUser must be called first")
			}
			userMap := userData.(map[string]any)
			resp := userMap["response"].(map[string]any)
			body := resp["body"].(map[string]any)
			userID := body["id"]
			return map[string]any{
				"response": map[string]any{
					"status": 200,
					"body": []map[string]any{
						{"id": "101", "title": "First Post", "userId": userID},
					},
				},
			}, nil
		},
	}

	// Node 3: GetComments - WITH SOURCE HINT for auto-chaining!
	// The AI will see exactly where to get postId from
	getCommentsNode := &aiParamMockNode{
		ID:   getCommentsID,
		Name: "GetComments",
		AIParams: []AIParam{
			{
				Name:        "postId",
				Description: "The post ID to get comments for",
				Type:        AIParamTypeString,
				Required:    true,
				SourceHint:  "GetPosts.response.body[0].id", // <-- AUTO-CHAIN HINT!
			},
		},
		OutputVars: []string{"response.body[0].body", "response.body[0].email"},
		RunFunc: func(req *node.FlowNodeRequest) (any, error) {
			vm := varsystem.NewVarMapFromAnyMap(req.VarMap)
			postIDStr, err := vm.ReplaceVars("{{ai_1.postId}}")
			if err != nil {
				return nil, fmt.Errorf("postId not set: %w", err)
			}
			return map[string]any{
				"response": map[string]any{
					"status": 200,
					"body": []map[string]any{
						{"id": "1001", "postId": postIDStr, "body": "Great post!", "email": "commenter@example.com"},
					},
				},
			}, nil
		},
	}

	providerNode := CreateTestAiProviderNode(providerID)

	edgeMap := mflow.EdgesMap{
		aiNodeID: {
			mflow.HandleAiProvider: []idwrap.IDWrap{providerID},
			mflow.HandleAiTools:    []idwrap.IDWrap{getUserID, getPostsID, getCommentsID},
		},
	}

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		providerID:    providerNode,
		getUserID:     getUserNode,
		getPostsID:    getPostsNode,
		getCommentsID: getCommentsNode,
	}

	return &mediumScenarioPOC5{
		GetUserNode:     getUserNode,
		GetPostsNode:    getPostsNode,
		GetCommentsNode: getCommentsNode,
		NodeMap:         nodeMap,
		EdgeMap:         edgeMap,
		AINodeID:        aiNodeID,
		ProviderID:      providerID,
	}
}

func TestPOC5_Medium_AutoChaining(t *testing.T) {
	llm := SetupGenericIntegrationTest(t)
	ctx := context.Background()
	scenario := setupMediumScenarioPOC5(t)

	// Very minimal prompt - AI should understand the chain from descriptions
	prompt := `Get comments for the first post of user ID 1. Tell me what the comment says.`

	aiNode := New(scenario.AINodeID, "ai_1", prompt, 15, nil)
	scenario.NodeMap[scenario.ProviderID].(*naiprovider.NodeAiProvider).LLM = llm
	scenario.NodeMap[scenario.AINodeID] = aiNode

	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
		EdgeSourceMap: scenario.EdgeMap,
		NodeMap:       scenario.NodeMap,
	}

	start := time.Now()
	res := aiNode.RunSync(ctx, req)
	duration := time.Since(start)

	totalToolCalls := scenario.GetUserNode.executionCounter +
		scenario.GetPostsNode.executionCounter +
		scenario.GetCommentsNode.executionCounter

	metrics := POCMetrics{
		POCName:   "POC5-AutoChaining",
		Scenario:  "Medium",
		Success:   res.Err == nil,
		Duration:  duration,
		ToolCalls: totalToolCalls,
	}

	if res.Err != nil {
		metrics.ErrorMessage = res.Err.Error()
	} else {
		val, err := node.ReadNodeVar(req, "ai_1", "text")
		require.NoError(t, err)
		response := val.(string)
		t.Logf("AI Response: %s", response)

		if !strings.Contains(strings.ToLower(response), "great post") {
			metrics.Success = false
			metrics.ErrorMessage = "AI did not retrieve comment correctly"
		}
	}

	logMetrics(t, metrics)
	assert.True(t, metrics.Success, "POC5 Medium (Auto-Chaining) should succeed")
}

// complexScenarioPOC5 demonstrates full auto-chaining with multiple source hints
type complexScenarioPOC5 struct {
	FetchDataNode *aiParamMockNode
	TransformNode *aiParamMockNode
	ValidateNode  *aiParamMockNode
	NodeMap       map[idwrap.IDWrap]node.FlowNode
	EdgeMap       mflow.EdgesMap
	AINodeID      idwrap.IDWrap
	ProviderID    idwrap.IDWrap
}

func setupComplexScenarioPOC5(t *testing.T) *complexScenarioPOC5 {
	aiNodeID := idwrap.NewNow()
	providerID := idwrap.NewNow()
	fetchDataID := idwrap.NewNow()
	transformID := idwrap.NewNow()
	validateID := idwrap.NewNow()

	fetchDataNode := &aiParamMockNode{
		ID:   fetchDataID,
		Name: "FetchData",
		AIParams: []AIParam{
			{Name: "endpoint", Description: "API endpoint URL (must contain 'api')", Type: AIParamTypeString, Required: true},
			{Name: "apiKey", Description: "API authentication key (min 5 chars)", Type: AIParamTypeString, Required: true},
		},
		OutputVars: []string{"response.body.items", "response.body.total"},
		RunFunc: func(req *node.FlowNodeRequest) (any, error) {
			vm := varsystem.NewVarMapFromAnyMap(req.VarMap)
			endpoint, err := vm.ReplaceVars("{{ai_1.endpoint}}")
			if err != nil {
				return nil, fmt.Errorf("endpoint not set: %w", err)
			}
			apiKey, err := vm.ReplaceVars("{{ai_1.apiKey}}")
			if err != nil {
				return nil, fmt.Errorf("apiKey not set: %w", err)
			}
			if !strings.Contains(endpoint, "api") {
				return nil, fmt.Errorf("invalid endpoint: %s", endpoint)
			}
			if len(apiKey) < 5 {
				return nil, fmt.Errorf("invalid apiKey")
			}
			return map[string]any{
				"response": map[string]any{
					"status": 200,
					"body": map[string]any{
						"items": []map[string]any{
							{"id": "1", "value": 100, "active": true},
							{"id": "2", "value": 200, "active": false},
							{"id": "3", "value": 150, "active": true},
						},
						"total": 3,
					},
				},
			}, nil
		},
	}

	// TransformData - no inputs, automatically uses FetchData output
	transformNode := &aiParamMockNode{
		ID:         transformID,
		Name:       "TransformData",
		AIParams:   []AIParam{}, // Auto-reads from FetchData
		OutputVars: []string{"result.activeItems", "result.totalValue", "result.count"},
		RunFunc: func(req *node.FlowNodeRequest) (any, error) {
			fetchData, ok := req.VarMap["FetchData"]
			if !ok {
				return nil, fmt.Errorf("FetchData must be called first")
			}
			fetchMap := fetchData.(map[string]any)
			resp := fetchMap["response"].(map[string]any)
			body := resp["body"].(map[string]any)
			items := body["items"].([]map[string]any)

			var activeItems []map[string]any
			totalValue := 0
			for _, item := range items {
				if active, ok := item["active"].(bool); ok && active {
					activeItems = append(activeItems, item)
					if val, ok := item["value"].(int); ok {
						totalValue += val
					}
				}
			}
			return map[string]any{
				"result": map[string]any{
					"activeItems": activeItems,
					"totalValue":  totalValue,
					"count":       len(activeItems),
				},
			}, nil
		},
	}

	// ValidateResult - with SOURCE HINT for totalValue!
	validateNode := &aiParamMockNode{
		ID:   validateID,
		Name: "ValidateResult",
		AIParams: []AIParam{
			{
				Name:        "threshold",
				Description: "Minimum acceptable total value",
				Type:        AIParamTypeNumber,
				Required:    true,
				// No source hint - user provides this
			},
			{
				Name:        "actualValue",
				Description: "The actual total value to validate",
				Type:        AIParamTypeNumber,
				Required:    true,
				SourceHint:  "TransformData.result.totalValue", // <-- AUTO-CHAIN!
			},
		},
		OutputVars: []string{"result.valid", "result.message"},
		RunFunc: func(req *node.FlowNodeRequest) (any, error) {
			vm := varsystem.NewVarMapFromAnyMap(req.VarMap)
			thresholdStr, err := vm.ReplaceVars("{{ai_1.threshold}}")
			if err != nil {
				return nil, fmt.Errorf("threshold not set: %w", err)
			}
			actualStr, err := vm.ReplaceVars("{{ai_1.actualValue}}")
			if err != nil {
				return nil, fmt.Errorf("actualValue not set: %w", err)
			}

			var threshold, actual int
			fmt.Sscanf(thresholdStr, "%d", &threshold)
			fmt.Sscanf(actualStr, "%d", &actual)

			valid := actual >= threshold
			message := "VALIDATION_PASSED"
			if !valid {
				message = "VALIDATION_FAILED"
			}
			return map[string]any{
				"result": map[string]any{
					"valid":      valid,
					"message":    message,
					"totalValue": actual,
					"threshold":  threshold,
				},
			}, nil
		},
	}

	providerNode := CreateTestAiProviderNode(providerID)

	edgeMap := mflow.EdgesMap{
		aiNodeID: {
			mflow.HandleAiProvider: []idwrap.IDWrap{providerID},
			mflow.HandleAiTools:    []idwrap.IDWrap{fetchDataID, transformID, validateID},
		},
	}

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		providerID:  providerNode,
		fetchDataID: fetchDataNode,
		transformID: transformNode,
		validateID:  validateNode,
	}

	return &complexScenarioPOC5{
		FetchDataNode: fetchDataNode,
		TransformNode: transformNode,
		ValidateNode:  validateNode,
		NodeMap:       nodeMap,
		EdgeMap:       edgeMap,
		AINodeID:      aiNodeID,
		ProviderID:    providerID,
	}
}

func TestPOC5_Complex_AutoChaining(t *testing.T) {
	llm := SetupGenericIntegrationTest(t)
	ctx := context.Background()
	scenario := setupComplexScenarioPOC5(t)

	// Minimal prompt - AI should follow the chain hints
	prompt := `Fetch data from "/api/v1/data" with apiKey "secret123", transform it, and validate that the total is at least 200. Tell me if it passed.`

	aiNode := New(scenario.AINodeID, "ai_1", prompt, 20, nil)
	scenario.NodeMap[scenario.ProviderID].(*naiprovider.NodeAiProvider).LLM = llm
	scenario.NodeMap[scenario.AINodeID] = aiNode

	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
		EdgeSourceMap: scenario.EdgeMap,
		NodeMap:       scenario.NodeMap,
	}

	start := time.Now()
	res := aiNode.RunSync(ctx, req)
	duration := time.Since(start)

	totalToolCalls := scenario.FetchDataNode.executionCounter +
		scenario.TransformNode.executionCounter +
		scenario.ValidateNode.executionCounter

	metrics := POCMetrics{
		POCName:   "POC5-AutoChaining",
		Scenario:  "Complex",
		Success:   res.Err == nil,
		Duration:  duration,
		ToolCalls: totalToolCalls,
	}

	if res.Err != nil {
		metrics.ErrorMessage = res.Err.Error()
	} else {
		val, err := node.ReadNodeVar(req, "ai_1", "text")
		require.NoError(t, err)
		response := val.(string)
		t.Logf("AI Response: %s", response)

		if !strings.Contains(strings.ToUpper(response), "PASSED") && !strings.Contains(response, "250") {
			metrics.Success = false
			metrics.ErrorMessage = "AI did not complete pipeline correctly"
		}
	}

	logMetrics(t, metrics)
	assert.True(t, metrics.Success, "POC5 Complex (Auto-Chaining) should succeed")
}

// =============================================================================
// COMPARISON TEST - Runs all PoCs and compares results
// =============================================================================

func TestPOC_Comparison_AllScenarios(t *testing.T) {
	llm := SetupGenericIntegrationTest(t)
	_ = llm // Will be used when running all tests

	t.Log("=== POC Comparison Test Results ===")
	t.Log("Run individual POC tests to see detailed comparison")
	t.Log("")
	t.Log("POC #1: Unified Introspection - Auto-generated from VariableIntrospector")
	t.Log("POC #2: User Description - Custom description field on nodes")
	t.Log("POC #3: Discovery Tool - On-demand discover_tools function")
	t.Log("POC #4: AI Param Syntax - {{ ai('name', 'desc', 'type') }} typed parameters")
	t.Log("POC #5: Auto-Chaining - {{ ai('name', 'desc', 'type', 'source') }} with chain hints")
	t.Log("")
	t.Log("Expected metrics to compare:")
	t.Log("- Success rate across scenarios")
	t.Log("- Number of tool calls (fewer = more efficient)")
	t.Log("- Discovery calls for POC #3")
	t.Log("- Execution duration")
}

// =============================================================================
// HELPER: JSON output for programmatic comparison
// =============================================================================

func metricsToJSON(m POCMetrics) string {
	data, _ := json.MarshalIndent(m, "", "  ")
	return string(data)
}
