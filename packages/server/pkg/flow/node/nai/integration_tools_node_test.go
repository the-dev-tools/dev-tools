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

// simpleMockNode implements node.FlowNode for testing purposes
type simpleMockNode struct {
	ID   idwrap.IDWrap
	Name string
}

func (n *simpleMockNode) GetID() idwrap.IDWrap {
	return n.ID
}

func (n *simpleMockNode) GetName() string {
	return n.Name
}

func (n *simpleMockNode) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	// Simulate doing something useful
	// Write a result to the var map so we know it ran
	req.VarMap[n.Name] = map[string]interface{}{
		"output": "Mock Tool Execution Successful!",
	}
	return node.FlowNodeResult{
		NextNodeID: []idwrap.IDWrap{}, // Terminal node
	}
}

func (n *simpleMockNode) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	resultChan <- n.RunSync(ctx, req)
}

func TestNodeAI_LiveNodesAsTools(t *testing.T) {
	// This test verifies that the AI can call a connected Node as a tool.
	llm := SetupGenericIntegrationTest(t)
	ctx := context.Background()

	// 1. Create Nodes
	aiNodeID := idwrap.NewNow()
	providerNodeID := idwrap.NewNow()
	toolNodeID := idwrap.NewNow()

	toolNodeName := "ExecuteSecretOperation"
	mockToolNode := &simpleMockNode{
		ID:   toolNodeID,
		Name: toolNodeName,
	}

	// 2. Configure AI Node
	// Prompt explicitly asks to use the connected tool
	prompt := fmt.Sprintf("Please run the tool named '%s' and tell me what the result was.", toolNodeName)

	// Note: We don't need the factory here because we inject the LLM directly
	aiNode := New(aiNodeID, "AI_AGENT", prompt, 5, nil)

	// Create AI Provider node and set the LLM
	providerNode := CreateTestAiProviderNode(providerNodeID)
	providerNode.LLM = llm

	// 3. Setup Request with Connection
	// Connect AI Node -> Provider via HandleAiProvider, Tool Node via HandleAiTools
	edgeMap := mflow.EdgesMap{
		aiNodeID: {
			mflow.HandleAiProvider: []idwrap.IDWrap{providerNodeID},
			mflow.HandleAiTools:    []idwrap.IDWrap{toolNodeID},
		},
	}

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		aiNodeID:       aiNode,
		providerNodeID: providerNode,
		toolNodeID:     mockToolNode,
	}

	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
		EdgeSourceMap: edgeMap,
		NodeMap:       nodeMap,
	}

	// 4. Run
	t.Logf("Running AI Node with attached tool: %s", toolNodeName)
	res := aiNode.RunSync(ctx, req)
	assert.NoError(t, res.Err)

	// 5. Verification

	// Check if the mock tool actually ran (it writes to var map)
	toolOutput, ok := req.VarMap[toolNodeName]
	assert.True(t, ok, "Tool node should have written to VarMap")
	t.Logf("Tool Output (Direct): %v", toolOutput)

	// Check AI's final response
	val, err := node.ReadNodeVar(req, "AI_AGENT", "text")
	assert.NoError(t, err)
	t.Logf("AI Response: %v", val)

	// The AI should mention the success or the specific output text
	assert.Contains(t, val, "Successful")
}
