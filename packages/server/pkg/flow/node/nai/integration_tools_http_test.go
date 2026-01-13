//go:build ai_integration

package nai

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

// mockHttpNode simulates an HTTP Request Node
// In a real flow, this would be an instance of nhttp.NodeHTTP
type mockHttpNode struct {
	ID   idwrap.IDWrap
	Name string
}

func (n *mockHttpNode) GetID() idwrap.IDWrap {
	return n.ID
}

func (n *mockHttpNode) GetName() string {
	return n.Name
}

func (n *mockHttpNode) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	// Simulate HTTP Request to https://api.example.com/users/1
	// We mimic the structure of a real HTTP node output
	response := map[string]interface{}{
		"status": 200,
		"body": map[string]interface{}{
			"id":       1,
			"username": "jdoe_dev",
			"email":    "jdoe@devtools.local",
			"role":     "admin",
		},
		"headers": map[string]string{
			"Content-Type": "application/json",
		},
	}
	
	req.VarMap[n.Name] = map[string]interface{}{
		"response": response,
	}
	
	return node.FlowNodeResult{
		NextNodeID: []idwrap.IDWrap{},
	}
}

func (n *mockHttpNode) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	resultChan <- n.RunSync(ctx, req)
}

func TestNodeAI_LiveHTTPTool(t *testing.T) {
	// Test: AI uses an "HTTP Request" node (simulated) to fetch user data and analyze it.
	llm := SetupGenericIntegrationTest(t)
	ctx := context.Background()

	// 1. Setup Nodes
	aiNodeID := idwrap.NewNow()
	httpNodeID := idwrap.NewNow()
	
	httpNodeName := "FetchUserProfile"
	httpNode := &mockHttpNode{
		ID:   httpNodeID,
		Name: httpNodeName,
	}

	// 2. Configure AI Node
	// Task: Call tool -> Read JSON output -> Summarize user
	prompt := fmt.Sprintf(`
		1. Call the tool '%s' to get user profile data.
		2. Extract the username and role from the response.
		3. Tell me if this user is an admin.
	`, httpNodeName)
	
	n := New(aiNodeID, "AI_AGENT", mflow.AiModelGpt52Pro, "", idwrap.IDWrap{}, prompt, 0, nil)
	n.LLM = llm

	// 3. Connect Nodes
	edgeMap := mflow.EdgesMap{
		aiNodeID: {
			mflow.HandleAiTools: []idwrap.IDWrap{httpNodeID},
		},
	}
	
	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		aiNodeID:   n,
		httpNodeID: httpNode,
	}

	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
		EdgeSourceMap: edgeMap,
		NodeMap:       nodeMap,
	}

	// 4. Run
	t.Logf("Running AI with HTTP Tool: %s", httpNodeName)
	res := n.RunSync(ctx, req)
	assert.NoError(t, res.Err)

	// 5. Verify
	// Check tool execution
	toolOutput, ok := req.VarMap[httpNodeName]
	assert.True(t, ok, "HTTP node should have executed")
	t.Logf("HTTP Tool Output: %v", toolOutput)

	// Check AI analysis
	val, err := node.ReadNodeVar(req, "AI_AGENT", "text")
	assert.NoError(t, err)
	t.Logf("AI Response: %v", val)
	
	// Assertions on the AI's understanding
	assert.Contains(t, val, "jdoe_dev", "AI should mention the username")
	assert.Contains(t, val, "admin", "AI should identify the role")
}
