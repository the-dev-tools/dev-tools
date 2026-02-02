//go:build ai_integration

package nai

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/naiprovider"
)

// =============================================================================
// POC #6: FEW-SHOT EXAMPLES
// Provides example tool call sequences in the prompt to guide AI
// =============================================================================

// FewShotExample represents an example tool usage for the AI
type FewShotExample struct {
	Description string   // What the example demonstrates
	Steps       []string // Step-by-step tool calls
}

// GenerateFewShotPrompt creates a prompt section with examples
func GenerateFewShotPrompt(examples []FewShotExample) string {
	var sb strings.Builder
	sb.WriteString("\n## EXAMPLES\n")
	sb.WriteString("Here are examples of how to use the tools correctly:\n\n")

	for i, ex := range examples {
		sb.WriteString(fmt.Sprintf("### Example %d: %s\n", i+1, ex.Description))
		for j, step := range ex.Steps {
			sb.WriteString(fmt.Sprintf("%d. %s\n", j+1, step))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func TestPOC6_Simple_FewShotExamples(t *testing.T) {
	llm := SetupGenericIntegrationTest(t)
	ctx := context.Background()
	scenario := setupSimpleScenario(t, false)

	// Few-shot examples showing the correct pattern
	examples := []FewShotExample{
		{
			Description: "Fetching user data",
			Steps: []string{
				"Call set_variable with key=\"ai_1.userId\" and value=\"42\"",
				"Call GetUser tool (no parameters needed, it reads from ai_1.userId)",
				"Call get_variable with key=\"GetUser.response.body.name\" to get the result",
			},
		},
	}

	prompt := fmt.Sprintf(`You are a helpful assistant with access to tools.
%s
## YOUR TASK
Fetch information about user ID 5 and tell me their name and email.`, GenerateFewShotPrompt(examples))

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
		POCName:   "POC6-FewShot",
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
	assert.True(t, metrics.Success, "POC6 Simple should succeed")
}

func TestPOC6_Medium_FewShotExamples(t *testing.T) {
	llm := SetupGenericIntegrationTest(t)
	ctx := context.Background()
	scenario := setupMediumScenario(t, false)

	examples := []FewShotExample{
		{
			Description: "Chaining API calls (User → Posts → Comments)",
			Steps: []string{
				"set_variable(key=\"ai_1.userId\", value=\"1\")",
				"Call GetUser → get user data at GetUser.response.body",
				"set_variable(key=\"ai_1.userId\", value=<userId from GetUser>)",
				"Call GetPosts → get posts at GetPosts.response.body.posts",
				"set_variable(key=\"ai_1.postId\", value=<first post ID>)",
				"Call GetComments → get comments at GetComments.response.body.comments",
			},
		},
	}

	prompt := fmt.Sprintf(`You are an API orchestration agent.
%s
## YOUR TASK
Get comments for the first post of user ID 1. Tell me what the comment says.`, GenerateFewShotPrompt(examples))

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
		POCName:   "POC6-FewShot",
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
	assert.True(t, metrics.Success, "POC6 Medium should succeed")
}

func TestPOC6_Complex_FewShotExamples(t *testing.T) {
	llm := SetupGenericIntegrationTest(t)
	ctx := context.Background()
	scenario := setupComplexScenario(t, false)

	examples := []FewShotExample{
		{
			Description: "Data pipeline (Fetch → Transform → Validate)",
			Steps: []string{
				"set_variable(key=\"ai_1.endpoint\", value=\"/api/v1/data\")",
				"set_variable(key=\"ai_1.apiKey\", value=\"your-api-key\")",
				"Call FetchData → data at FetchData.response.body.items",
				"Call TransformData → results at TransformData.result.totalValue",
				"set_variable(key=\"ai_1.threshold\", value=200)",
				"Call ValidateResult → check ValidateResult.result.valid",
			},
		},
	}

	prompt := fmt.Sprintf(`You are a data pipeline orchestration agent.
%s
## YOUR TASK
Fetch data from "/api/v1/data" with apiKey "secret123", transform it, and validate that the total is at least 200.
Tell me if validation passed and what the total value was.`, GenerateFewShotPrompt(examples))

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
		POCName:   "POC6-FewShot",
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
	assert.True(t, metrics.Success, "POC6 Complex should succeed")
}

// =============================================================================
// POC #7: REACT PATTERN
// Forces AI to reason before each action: Thought → Action → Observation
// =============================================================================

// GenerateReActPrompt creates a ReAct-style prompt
func GenerateReActPrompt(task string, tools []string) string {
	var sb strings.Builder

	sb.WriteString(`You are an AI assistant that follows the ReAct pattern.
For every step, you MUST:
1. THOUGHT: Explain what you need to do next and why
2. ACTION: Call exactly one tool
3. OBSERVATION: Analyze the result before proceeding

Available tools: `)
	sb.WriteString(strings.Join(tools, ", "))
	sb.WriteString("\n\n")

	sb.WriteString(`## FORMAT
Always respond in this exact format for each step:

THOUGHT: [Your reasoning about what to do next]
ACTION: [Tool call]
OBSERVATION: [What you learned from the result]

Then repeat until task is complete, ending with:
FINAL ANSWER: [Your complete response to the user]

`)
	sb.WriteString("## TASK\n")
	sb.WriteString(task)

	return sb.String()
}

func TestPOC7_Simple_ReActPattern(t *testing.T) {
	llm := SetupGenericIntegrationTest(t)
	ctx := context.Background()
	scenario := setupSimpleScenario(t, false)

	tools := []string{"get_variable", "set_variable", "GetUser"}
	prompt := GenerateReActPrompt(
		"Fetch information about user ID 5. Tell me the user's name and email.",
		tools,
	)

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
		POCName:   "POC7-ReAct",
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
	assert.True(t, metrics.Success, "POC7 Simple should succeed")
}

func TestPOC7_Medium_ReActPattern(t *testing.T) {
	llm := SetupGenericIntegrationTest(t)
	ctx := context.Background()
	scenario := setupMediumScenario(t, false)

	tools := []string{"get_variable", "set_variable", "GetUser", "GetPosts", "GetComments"}
	prompt := GenerateReActPrompt(
		"Get comments for the first post of user ID 1. Tell me what the comment says.",
		tools,
	)

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
		POCName:   "POC7-ReAct",
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
	assert.True(t, metrics.Success, "POC7 Medium should succeed")
}

func TestPOC7_Complex_ReActPattern(t *testing.T) {
	llm := SetupGenericIntegrationTest(t)
	ctx := context.Background()
	scenario := setupComplexScenario(t, false)

	tools := []string{"get_variable", "set_variable", "FetchData", "TransformData", "ValidateResult"}
	prompt := GenerateReActPrompt(
		`Fetch data from "/api/v1/data" with apiKey "secret123", transform it, and validate that the total is at least 200. Tell me if validation passed and what the total value was.`,
		tools,
	)

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
		POCName:   "POC7-ReAct",
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
	assert.True(t, metrics.Success, "POC7 Complex should succeed")
}

// =============================================================================
// POC #8: DEPENDENCY GRAPH
// Explicitly shows tool execution order as a graph
// =============================================================================

// DependencyNode represents a node in the tool dependency graph
type DependencyNode struct {
	Tool        string
	DependsOn   []string
	Provides    []string
	Description string
}

// GenerateDependencyGraphPrompt creates a prompt with explicit dependency information
func GenerateDependencyGraphPrompt(nodes []DependencyNode) string {
	var sb strings.Builder
	sb.WriteString("\n## TOOL DEPENDENCY GRAPH\n")
	sb.WriteString("Execute tools in this order (arrows show data flow):\n\n")
	sb.WriteString("```\n")

	for i, n := range nodes {
		if i > 0 {
			sb.WriteString("    │\n")
			sb.WriteString("    ▼\n")
		}
		sb.WriteString(fmt.Sprintf("[%s]\n", n.Tool))
		if len(n.DependsOn) > 0 {
			sb.WriteString(fmt.Sprintf("  Requires: %s\n", strings.Join(n.DependsOn, ", ")))
		}
		if len(n.Provides) > 0 {
			sb.WriteString(fmt.Sprintf("  Provides: %s\n", strings.Join(n.Provides, ", ")))
		}
	}

	sb.WriteString("```\n")
	return sb.String()
}

func TestPOC8_Complex_DependencyGraph(t *testing.T) {
	llm := SetupGenericIntegrationTest(t)
	ctx := context.Background()
	scenario := setupComplexScenario(t, false)

	depGraph := []DependencyNode{
		{
			Tool:      "FetchData",
			DependsOn: []string{"ai_1.endpoint", "ai_1.apiKey"},
			Provides:  []string{"FetchData.response.body.items"},
		},
		{
			Tool:      "TransformData",
			DependsOn: []string{"FetchData.response.body.items"},
			Provides:  []string{"TransformData.result.totalValue"},
		},
		{
			Tool:      "ValidateResult",
			DependsOn: []string{"TransformData.result.totalValue", "ai_1.threshold"},
			Provides:  []string{"ValidateResult.result.valid", "ValidateResult.result.message"},
		},
	}

	prompt := fmt.Sprintf(`You are a data pipeline agent.
%s
## TASK
1. Set endpoint="/api/v1/data" and apiKey="secret123"
2. Follow the dependency graph above
3. Set threshold=200 before ValidateResult
4. Tell me if validation passed and the total value.`, GenerateDependencyGraphPrompt(depGraph))

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
		POCName:   "POC8-DepGraph",
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
	assert.True(t, metrics.Success, "POC8 Complex should succeed")
}

// =============================================================================
// MEDIUM SCENARIO WITH AI PARAMS FOR POC6/7/8
// =============================================================================

// aiParamMediumMockNode implements AIParamProvider for POC comparisons
type aiParamMediumMockNode struct {
	describableMockNode
	aiParams []AIParam
}

func (n *aiParamMediumMockNode) GetAIParams() []AIParam {
	return n.aiParams
}

func setupMediumScenarioWithAIParams(t *testing.T) *mediumScenario {
	base := setupMediumScenario(t, false)

	// Wrap nodes with AI params
	getUserNode := &aiParamMediumMockNode{
		describableMockNode: *base.GetUserNode,
		aiParams: []AIParam{
			{Name: "userId", Description: "User ID to fetch", Type: "number"},
		},
	}

	getPostsNode := &aiParamMediumMockNode{
		describableMockNode: *base.GetPostsNode,
		aiParams: []AIParam{
			{Name: "userId", Description: "User ID for posts", Type: "number", SourceHint: "GetUser.response.body.id"},
		},
	}

	getCommentsNode := &aiParamMediumMockNode{
		describableMockNode: *base.GetCommentsNode,
		aiParams: []AIParam{
			{Name: "postId", Description: "Post ID for comments", Type: "number", SourceHint: "GetPosts.response.body.posts[0].id"},
		},
	}

	base.NodeMap[getUserNode.ID] = getUserNode
	base.NodeMap[getPostsNode.ID] = getPostsNode
	base.NodeMap[getCommentsNode.ID] = getCommentsNode
	base.GetUserNode = &getUserNode.describableMockNode
	base.GetPostsNode = &getPostsNode.describableMockNode
	base.GetCommentsNode = &getCommentsNode.describableMockNode

	return base
}
