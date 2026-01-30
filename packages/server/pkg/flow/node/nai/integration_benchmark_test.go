//go:build ai_integration

package nai

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/naiprovider"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

// getProviderAndModel detects which provider/model is being used from environment
func getProviderAndModel() (provider, model string) {
	if os.Getenv("OPENAI_API_KEY") != "" {
		provider = "openai"
		model = os.Getenv("OPENAI_MODEL")
		if model == "" {
			model = "gpt-4o-mini" // default
		}
	} else if os.Getenv("ANTHROPIC_API_KEY") != "" {
		provider = "anthropic"
		model = os.Getenv("ANTHROPIC_MODEL")
		if model == "" {
			model = "claude-3-sonnet"
		}
	} else if os.Getenv("GEMINI_API_KEY") != "" {
		provider = "google"
		model = "gemini-pro"
	} else {
		provider = "unknown"
		model = "unknown"
	}
	return
}

// =============================================================================
// COMPREHENSIVE BENCHMARK TEST
// Runs all POCs across all scenarios multiple times and produces scoring report
// =============================================================================

// TestPOC_Benchmark_Full runs a comprehensive benchmark of all POCs
// This is the main entry point for comparing POC effectiveness.
//
// Run with:
//
//	RUN_AI_INTEGRATION_TESTS=true OPENAI_API_KEY=sk-... \
//	  go test -tags ai_integration -v -run TestPOC_Benchmark_Full \
//	  ./packages/server/pkg/flow/node/nai
func TestPOC_Benchmark_Full(t *testing.T) {
	llm := SetupGenericIntegrationTest(t)
	suite := NewPOCBenchmarkSuite()

	// Configuration
	runsPerTest := 2 // Number of runs per POC/scenario combination
	scenarios := []string{"Simple", "Medium", "Complex"}

	t.Log("Starting POC Benchmark...")
	t.Logf("Configuration: %d runs per test, scenarios: %v", runsPerTest, scenarios)
	t.Log("")

	// Run benchmarks for each scenario
	for _, scenario := range scenarios {
		t.Logf("=== Scenario: %s ===", scenario)

		for run := 1; run <= runsPerTest; run++ {
			t.Logf("  Run %d/%d", run, runsPerTest)

			// POC 1: Unified Introspection
			if m := runPOC1(t, llm, scenario); m != nil {
				suite.AddResult(*m)
			}

			// POC 2: User Description
			if m := runPOC2(t, llm, scenario); m != nil {
				suite.AddResult(*m)
			}

			// POC 3: Discovery Tool
			if m := runPOC3(t, llm, scenario); m != nil {
				suite.AddResult(*m)
			}

			// POC 4: Typed AI Parameters
			if m := runPOC4(t, llm, scenario); m != nil {
				suite.AddResult(*m)
			}

			// POC 5: Auto-Chaining
			if m := runPOC5(t, llm, scenario); m != nil {
				suite.AddResult(*m)
			}

			// POC 6: Few-Shot Examples
			if m := runPOC6(t, llm, scenario); m != nil {
				suite.AddResult(*m)
			}

			// POC 7: ReAct Pattern
			if m := runPOC7(t, llm, scenario); m != nil {
				suite.AddResult(*m)
			}
		}
	}

	// Print results
	suite.PrintRanking(t)
	suite.PrintComparisonMatrix(t)

	// Save to file
	provider, model := getProviderAndModel()
	if err := suite.SaveToFile(t, provider, model, scenarios, runsPerTest); err != nil {
		t.Logf("Warning: Failed to save results to file: %v", err)
	}
}

// TestPOC_Benchmark_Quick runs a quick benchmark with fewer iterations
func TestPOC_Benchmark_Quick(t *testing.T) {
	llm := SetupGenericIntegrationTest(t)
	suite := NewPOCBenchmarkSuite()

	t.Log("Running Quick POC Benchmark (Simple scenario only, 1 run each)...")

	// Only run Simple scenario once for quick feedback
	pocs := []struct {
		name string
		run  func(*testing.T, llms.Model, string) *POCMetrics
	}{
		{"POC1", runPOC1},
		{"POC2", runPOC2},
		{"POC4", runPOC4},
		{"POC5", runPOC5},
		{"POC6", runPOC6},
		{"POC7", runPOC7},
	}

	for _, poc := range pocs {
		if m := poc.run(t, llm, "Simple"); m != nil {
			suite.AddResult(*m)
			logMetrics(t, *m)
		}
	}

	suite.PrintRanking(t)

	// Save to file
	provider, model := getProviderAndModel()
	if err := suite.SaveToFile(t, provider, model, []string{"Simple"}, 1); err != nil {
		t.Logf("Warning: Failed to save results to file: %v", err)
	}
}

// =============================================================================
// POC RUNNER FUNCTIONS
// Each returns metrics from a single run
// =============================================================================

func runPOC1(t *testing.T, llm llms.Model, scenario string) *POCMetrics {
	ctx := context.Background()
	var metrics POCMetrics
	metrics.POCName = "POC1-Introspect"
	metrics.Scenario = scenario

	switch scenario {
	case "Simple":
		s := setupSimpleScenario(t, false)
		prompt := `Fetch information about user ID 5 and tell me their name and email.`
		m := runAINode(ctx, t, llm, s.AINodeID, s.NodeMap, s.EdgeMap, prompt, 10)
		metrics.Success = m.Success
		metrics.ToolCalls = s.GetUserNode.executionCounter
		metrics.Duration = m.Duration
		metrics.ErrorMessage = m.ErrorMessage

	case "Medium":
		s := setupMediumScenario(t, false)
		prompt := `Get comments for the first post of user ID 1. Tell me what the comment says.`
		m := runAINode(ctx, t, llm, s.AINodeID, s.NodeMap, s.EdgeMap, prompt, 15)
		metrics.Success = m.Success
		metrics.ToolCalls = s.GetUserNode.executionCounter + s.GetPostsNode.executionCounter + s.GetCommentsNode.executionCounter
		metrics.Duration = m.Duration
		metrics.ErrorMessage = m.ErrorMessage

	case "Complex":
		s := setupComplexScenario(t, false)
		prompt := `Fetch data from "/api/v1/data" with apiKey "secret123", transform it, and validate that the total is at least 200.`
		m := runAINode(ctx, t, llm, s.AINodeID, s.NodeMap, s.EdgeMap, prompt, 20)
		metrics.Success = m.Success
		metrics.ToolCalls = s.FetchDataNode.executionCounter + s.TransformNode.executionCounter + s.ValidateNode.executionCounter
		metrics.Duration = m.Duration
		metrics.ErrorMessage = m.ErrorMessage
	}

	return &metrics
}

func runPOC2(t *testing.T, llm llms.Model, scenario string) *POCMetrics {
	ctx := context.Background()
	var metrics POCMetrics
	metrics.POCName = "POC2-UserDesc"
	metrics.Scenario = scenario

	switch scenario {
	case "Simple":
		s := setupSimpleScenario(t, true) // Use custom descriptions
		prompt := `Fetch information about user ID 5 and tell me their name and email.`
		m := runAINode(ctx, t, llm, s.AINodeID, s.NodeMap, s.EdgeMap, prompt, 10)
		metrics.Success = m.Success
		metrics.ToolCalls = s.GetUserNode.executionCounter
		metrics.Duration = m.Duration
		metrics.ErrorMessage = m.ErrorMessage

	case "Medium":
		s := setupMediumScenario(t, true)
		prompt := `Get comments for the first post of user ID 1. Tell me what the comment says.`
		m := runAINode(ctx, t, llm, s.AINodeID, s.NodeMap, s.EdgeMap, prompt, 15)
		metrics.Success = m.Success
		metrics.ToolCalls = s.GetUserNode.executionCounter + s.GetPostsNode.executionCounter + s.GetCommentsNode.executionCounter
		metrics.Duration = m.Duration
		metrics.ErrorMessage = m.ErrorMessage

	case "Complex":
		s := setupComplexScenario(t, true)
		prompt := `Fetch data from "/api/v1/data" with apiKey "secret123", transform it, and validate that the total is at least 200.`
		m := runAINode(ctx, t, llm, s.AINodeID, s.NodeMap, s.EdgeMap, prompt, 20)
		metrics.Success = m.Success
		metrics.ToolCalls = s.FetchDataNode.executionCounter + s.TransformNode.executionCounter + s.ValidateNode.executionCounter
		metrics.Duration = m.Duration
		metrics.ErrorMessage = m.ErrorMessage
	}

	return &metrics
}

func runPOC3(t *testing.T, llm llms.Model, scenario string) *POCMetrics {
	ctx := context.Background()
	var metrics POCMetrics
	metrics.POCName = "POC3-Discovery"
	metrics.Scenario = scenario

	switch scenario {
	case "Simple":
		s := setupSimpleScenario(t, false)
		prompt := `You have access to discover_tools to learn about available tools.
First discover what tools are available, then fetch information about user ID 5.`
		aiNode := New(s.AINodeID, "ai_1", prompt, 10, nil)
		s.NodeMap[s.ProviderID].(*naiprovider.NodeAiProvider).LLM = llm
		aiNode.EnableDiscoveryTool = true
		s.NodeMap[s.AINodeID] = aiNode

		req := &node.FlowNodeRequest{
			VarMap:        make(map[string]any),
			ReadWriteLock: &sync.RWMutex{},
			EdgeSourceMap: s.EdgeMap,
			NodeMap:       s.NodeMap,
		}

		start := time.Now()
		res := aiNode.RunSync(ctx, req)
		metrics.Duration = time.Since(start)
		metrics.Success = res.Err == nil
		metrics.ToolCalls = s.GetUserNode.executionCounter
		metrics.DiscoverCalls = aiNode.DiscoverToolCalls
		if res.Err != nil {
			metrics.ErrorMessage = res.Err.Error()
		}

	case "Medium":
		s := setupMediumScenario(t, false)
		prompt := `You have access to discover_tools. Use it to learn about tools, then get comments for the first post of user ID 1.`
		aiNode := New(s.AINodeID, "ai_1", prompt, 15, nil)
		s.NodeMap[s.ProviderID].(*naiprovider.NodeAiProvider).LLM = llm
		aiNode.EnableDiscoveryTool = true
		s.NodeMap[s.AINodeID] = aiNode

		req := &node.FlowNodeRequest{
			VarMap:        make(map[string]any),
			ReadWriteLock: &sync.RWMutex{},
			EdgeSourceMap: s.EdgeMap,
			NodeMap:       s.NodeMap,
		}

		start := time.Now()
		res := aiNode.RunSync(ctx, req)
		metrics.Duration = time.Since(start)
		metrics.Success = res.Err == nil
		metrics.ToolCalls = s.GetUserNode.executionCounter + s.GetPostsNode.executionCounter + s.GetCommentsNode.executionCounter
		metrics.DiscoverCalls = aiNode.DiscoverToolCalls
		if res.Err != nil {
			metrics.ErrorMessage = res.Err.Error()
		}

	case "Complex":
		s := setupComplexScenario(t, false)
		prompt := `Use discover_tools first, then: Fetch data from "/api/v1/data" with apiKey "secret123", transform it, and validate total >= 200.`
		aiNode := New(s.AINodeID, "ai_1", prompt, 20, nil)
		s.NodeMap[s.ProviderID].(*naiprovider.NodeAiProvider).LLM = llm
		aiNode.EnableDiscoveryTool = true
		s.NodeMap[s.AINodeID] = aiNode

		req := &node.FlowNodeRequest{
			VarMap:        make(map[string]any),
			ReadWriteLock: &sync.RWMutex{},
			EdgeSourceMap: s.EdgeMap,
			NodeMap:       s.NodeMap,
		}

		start := time.Now()
		res := aiNode.RunSync(ctx, req)
		metrics.Duration = time.Since(start)
		metrics.Success = res.Err == nil
		metrics.ToolCalls = s.FetchDataNode.executionCounter + s.TransformNode.executionCounter + s.ValidateNode.executionCounter
		metrics.DiscoverCalls = aiNode.DiscoverToolCalls
		if res.Err != nil {
			metrics.ErrorMessage = res.Err.Error()
		}
	}

	return &metrics
}

func runPOC4(t *testing.T, llm llms.Model, scenario string) *POCMetrics {
	ctx := context.Background()
	var metrics POCMetrics
	metrics.POCName = "POC4-TypedParam"
	metrics.Scenario = scenario

	switch scenario {
	case "Simple":
		s := setupSimpleScenarioPOC4(t)
		prompt := `Fetch information about user ID 5 and tell me their name and email.`
		m := runAINode(ctx, t, llm, s.AINodeID, s.NodeMap, s.EdgeMap, prompt, 10)
		metrics.Success = m.Success
		metrics.ToolCalls = s.GetUserNode.executionCounter
		metrics.Duration = m.Duration
		metrics.ErrorMessage = m.ErrorMessage

	case "Medium":
		s := setupMediumScenarioPOC4(t)
		prompt := `Get comments for the first post of user ID 1. Tell me what the comment says.`
		m := runAINode(ctx, t, llm, s.AINodeID, s.NodeMap, s.EdgeMap, prompt, 15)
		metrics.Success = m.Success
		metrics.ToolCalls = s.GetUserNode.executionCounter + s.GetPostsNode.executionCounter + s.GetCommentsNode.executionCounter
		metrics.Duration = m.Duration
		metrics.ErrorMessage = m.ErrorMessage

	case "Complex":
		s := setupComplexScenarioPOC4(t)
		prompt := `Fetch data from "/api/v1/data" with apiKey "secret123", transform it, and validate that the total is at least 200.`
		m := runAINode(ctx, t, llm, s.AINodeID, s.NodeMap, s.EdgeMap, prompt, 20)
		metrics.Success = m.Success
		metrics.ToolCalls = s.FetchDataNode.executionCounter + s.TransformNode.executionCounter + s.ValidateNode.executionCounter
		metrics.Duration = m.Duration
		metrics.ErrorMessage = m.ErrorMessage
	}

	return &metrics
}

func runPOC5(t *testing.T, llm llms.Model, scenario string) *POCMetrics {
	ctx := context.Background()
	var metrics POCMetrics
	metrics.POCName = "POC5-AutoChain"
	metrics.Scenario = scenario

	switch scenario {
	case "Simple":
		s := setupSimpleScenarioPOC5(t)
		prompt := `Fetch information about user ID 5 and tell me their name and email.`
		m := runAINode(ctx, t, llm, s.AINodeID, s.NodeMap, s.EdgeMap, prompt, 10)
		metrics.Success = m.Success
		metrics.ToolCalls = s.GetUserNode.executionCounter
		metrics.Duration = m.Duration
		metrics.ErrorMessage = m.ErrorMessage

	case "Medium":
		s := setupMediumScenarioPOC5(t)
		prompt := `Get comments for the first post of user ID 1. Tell me what the comment says.`
		m := runAINode(ctx, t, llm, s.AINodeID, s.NodeMap, s.EdgeMap, prompt, 15)
		metrics.Success = m.Success
		metrics.ToolCalls = s.GetUserNode.executionCounter + s.GetPostsNode.executionCounter + s.GetCommentsNode.executionCounter
		metrics.Duration = m.Duration
		metrics.ErrorMessage = m.ErrorMessage

	case "Complex":
		s := setupComplexScenarioPOC5(t)
		prompt := `Fetch data from "/api/v1/data" with apiKey "secret123", transform it, and validate that the total is at least 200.`
		m := runAINode(ctx, t, llm, s.AINodeID, s.NodeMap, s.EdgeMap, prompt, 20)
		metrics.Success = m.Success
		metrics.ToolCalls = s.FetchDataNode.executionCounter + s.TransformNode.executionCounter + s.ValidateNode.executionCounter
		metrics.Duration = m.Duration
		metrics.ErrorMessage = m.ErrorMessage
	}

	return &metrics
}

func runPOC6(t *testing.T, llm llms.Model, scenario string) *POCMetrics {
	ctx := context.Background()
	var metrics POCMetrics
	metrics.POCName = "POC6-FewShot"
	metrics.Scenario = scenario

	switch scenario {
	case "Simple":
		s := setupSimpleScenario(t, false)
		examples := []FewShotExample{{
			Description: "Fetching user data",
			Steps: []string{
				"set_variable(key=\"ai_1.userId\", value=\"42\")",
				"Call GetUser",
				"get_variable(key=\"GetUser.response.body.name\")",
			},
		}}
		prompt := GenerateFewShotPrompt(examples) + "\n\nTASK: Fetch user ID 5 and tell me their name and email."
		m := runAINode(ctx, t, llm, s.AINodeID, s.NodeMap, s.EdgeMap, prompt, 10)
		metrics.Success = m.Success
		metrics.ToolCalls = s.GetUserNode.executionCounter
		metrics.Duration = m.Duration
		metrics.ErrorMessage = m.ErrorMessage

	case "Medium":
		s := setupMediumScenario(t, false)
		examples := []FewShotExample{{
			Description: "Chaining calls",
			Steps: []string{
				"set_variable(key=\"ai_1.userId\", value=\"1\")",
				"Call GetUser",
				"set_variable(key=\"ai_1.userId\", value=<from GetUser>)",
				"Call GetPosts",
				"set_variable(key=\"ai_1.postId\", value=<first post ID>)",
				"Call GetComments",
			},
		}}
		prompt := GenerateFewShotPrompt(examples) + "\n\nTASK: Get comments for the first post of user ID 1."
		m := runAINode(ctx, t, llm, s.AINodeID, s.NodeMap, s.EdgeMap, prompt, 15)
		metrics.Success = m.Success
		metrics.ToolCalls = s.GetUserNode.executionCounter + s.GetPostsNode.executionCounter + s.GetCommentsNode.executionCounter
		metrics.Duration = m.Duration
		metrics.ErrorMessage = m.ErrorMessage

	case "Complex":
		s := setupComplexScenario(t, false)
		examples := []FewShotExample{{
			Description: "Data pipeline",
			Steps: []string{
				"set_variable(key=\"ai_1.endpoint\", value=\"/api/v1/data\")",
				"set_variable(key=\"ai_1.apiKey\", value=\"key\")",
				"Call FetchData",
				"Call TransformData",
				"set_variable(key=\"ai_1.threshold\", value=200)",
				"Call ValidateResult",
			},
		}}
		prompt := GenerateFewShotPrompt(examples) + "\n\nTASK: Fetch from \"/api/v1/data\" with apiKey \"secret123\", transform, validate >= 200."
		m := runAINode(ctx, t, llm, s.AINodeID, s.NodeMap, s.EdgeMap, prompt, 20)
		metrics.Success = m.Success
		metrics.ToolCalls = s.FetchDataNode.executionCounter + s.TransformNode.executionCounter + s.ValidateNode.executionCounter
		metrics.Duration = m.Duration
		metrics.ErrorMessage = m.ErrorMessage
	}

	return &metrics
}

func runPOC7(t *testing.T, llm llms.Model, scenario string) *POCMetrics {
	ctx := context.Background()
	var metrics POCMetrics
	metrics.POCName = "POC7-ReAct"
	metrics.Scenario = scenario

	switch scenario {
	case "Simple":
		s := setupSimpleScenario(t, false)
		tools := []string{"get_variable", "set_variable", "GetUser"}
		prompt := GenerateReActPrompt("Fetch user ID 5 and tell me their name and email.", tools)
		m := runAINode(ctx, t, llm, s.AINodeID, s.NodeMap, s.EdgeMap, prompt, 10)
		metrics.Success = m.Success
		metrics.ToolCalls = s.GetUserNode.executionCounter
		metrics.Duration = m.Duration
		metrics.ErrorMessage = m.ErrorMessage

	case "Medium":
		s := setupMediumScenario(t, false)
		tools := []string{"get_variable", "set_variable", "GetUser", "GetPosts", "GetComments"}
		prompt := GenerateReActPrompt("Get comments for the first post of user ID 1.", tools)
		m := runAINode(ctx, t, llm, s.AINodeID, s.NodeMap, s.EdgeMap, prompt, 15)
		metrics.Success = m.Success
		metrics.ToolCalls = s.GetUserNode.executionCounter + s.GetPostsNode.executionCounter + s.GetCommentsNode.executionCounter
		metrics.Duration = m.Duration
		metrics.ErrorMessage = m.ErrorMessage

	case "Complex":
		s := setupComplexScenario(t, false)
		tools := []string{"get_variable", "set_variable", "FetchData", "TransformData", "ValidateResult"}
		prompt := GenerateReActPrompt("Fetch from \"/api/v1/data\" with apiKey \"secret123\", transform, validate >= 200.", tools)
		m := runAINode(ctx, t, llm, s.AINodeID, s.NodeMap, s.EdgeMap, prompt, 20)
		metrics.Success = m.Success
		metrics.ToolCalls = s.FetchDataNode.executionCounter + s.TransformNode.executionCounter + s.ValidateNode.executionCounter
		metrics.Duration = m.Duration
		metrics.ErrorMessage = m.ErrorMessage
	}

	return &metrics
}

// =============================================================================
// HELPER: Run AI Node and capture metrics
// =============================================================================

type runResult struct {
	Success      bool
	Duration     time.Duration
	ErrorMessage string
	Response     string
}

func runAINode(ctx context.Context, t *testing.T, llm llms.Model, aiNodeID idwrap.IDWrap,
	nodeMap map[idwrap.IDWrap]node.FlowNode, edgeMap mflow.EdgesMap,
	prompt string, maxIterations int32) runResult {

	aiNode := New(aiNodeID, "ai_1", prompt, maxIterations, nil)
	// Get provider ID from edge map and set LLM
	if providerIDs, ok := edgeMap[aiNodeID][mflow.HandleAiProvider]; ok && len(providerIDs) > 0 {
		nodeMap[providerIDs[0]].(*naiprovider.NodeAiProvider).LLM = llm
	}
	nodeMap[aiNodeID] = aiNode

	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
		EdgeSourceMap: edgeMap,
		NodeMap:       nodeMap,
	}

	start := time.Now()
	res := aiNode.RunSync(ctx, req)
	duration := time.Since(start)

	result := runResult{
		Success:  res.Err == nil,
		Duration: duration,
	}

	if res.Err != nil {
		result.ErrorMessage = res.Err.Error()
	} else {
		val, err := node.ReadNodeVar(req, "ai_1", "text")
		if err == nil {
			result.Response = val.(string)
			// Basic validation
			if !validateResponse(result.Response) {
				result.Success = false
				result.ErrorMessage = "Response validation failed"
			}
		}
	}

	return result
}

func validateResponse(response string) bool {
	lower := strings.ToLower(response)
	// Check for common success indicators
	hasContent := len(response) > 20
	notError := !strings.Contains(lower, "error") || strings.Contains(lower, "no error")
	return hasContent && notError
}
