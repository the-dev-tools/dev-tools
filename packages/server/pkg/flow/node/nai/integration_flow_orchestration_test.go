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

// functionalMockNode runs a custom function logic
type functionalMockNode struct {
	ID      idwrap.IDWrap
	Name    string
	RunFunc func(req *node.FlowNodeRequest) (any, error)
}

func (n *functionalMockNode) GetID() idwrap.IDWrap { return n.ID }
func (n *functionalMockNode) GetName() string      { return n.Name }
func (n *functionalMockNode) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	output, err := n.RunFunc(req)
	if err != nil {
		return node.FlowNodeResult{Err: err}
	}
	// Write output to VarMap so AI can see it
	req.VarMap[n.Name] = output
	return node.FlowNodeResult{NextNodeID: []idwrap.IDWrap{}}
}
func (n *functionalMockNode) RunAsync(ctx context.Context, req *node.FlowNodeRequest, res chan node.FlowNodeResult) {
	res <- n.RunSync(ctx, req)
}

func TestNodeAI_LiveMultiStepFlow(t *testing.T) {
	// Scenario:
	// 1. Fetch User -> Output: {username: "alice"}
	// 2. Login -> Input: Reads "FetchUser.username" -> Output: {token: "secret-token"}
	// 3. GetData -> Input: Reads "Login.token" -> Output: {data: "MISSION_COMPLETED"}
	llm := SetupGenericIntegrationTest(t)
	ctx := context.Background()

	// 1. Create Nodes
	aiNodeID := idwrap.NewNow()
	nodeUserID := idwrap.NewNow()
	nodeLoginID := idwrap.NewNow()
	nodeDataID := idwrap.NewNow()

	// -- Node 1: Fetch User --
	fetchUserNode := &functionalMockNode{
		ID:   nodeUserID,
		Name: "FetchUser",
		RunFunc: func(req *node.FlowNodeRequest) (any, error) {
			return map[string]string{"username": "alice"}, nil
		},
	}

	// -- Node 2: Login --
	loginNode := &functionalMockNode{
		ID:   nodeLoginID,
		Name: "Login",
		RunFunc: func(req *node.FlowNodeRequest) (any, error) {
			// Check if previous step data exists
			userOutput, ok := req.VarMap["FetchUser"].(map[string]string)
			if !ok || userOutput["username"] != "alice" {
				return nil, fmt.Errorf("login failed: missing or invalid user from FetchUser")
			}
			return map[string]string{"token": "xyz-secret-token"}, nil
		},
	}

	// -- Node 3: Fetch Data --
	fetchDataNode := &functionalMockNode{
		ID:   nodeDataID,
		Name: "FetchData",
		RunFunc: func(req *node.FlowNodeRequest) (any, error) {
			// Check if token exists
			loginOutput, ok := req.VarMap["Login"].(map[string]string)
			if !ok || loginOutput["token"] != "xyz-secret-token" {
				return nil, fmt.Errorf("unauthorized: missing token")
			}
			return map[string]string{"secret": "MISSION_COMPLETED"}, nil
		},
	}

	// 2. Configure AI
	// We prompt it to orchestrate the flow.
	prompt := `
		You are an orchestration agent.
		1. Run 'FetchUser' to find the target user.
		2. Run 'Login' (it will automatically read the user from step 1).
		3. Run 'FetchData' (it will automatically read the token from step 2).
		4. Return the final secret.
	`
	
	// 4. Create AI Node
	aiNode := New(aiNodeID, "AI_ORCHESTRATOR", mflow.AiModelCustom, modelName, credID, prompt, 0, factory)
	n.LLM = llm

	// 3. Connect Edges
	edgeMap := mflow.EdgesMap{
		aiNodeID: {
			mflow.HandleAiTools: []idwrap.IDWrap{nodeUserID, nodeLoginID, nodeDataID},
		},
	}
	
	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		aiNodeID:    n,
		nodeUserID:  fetchUserNode,
		nodeLoginID: loginNode,
		nodeDataID:  fetchDataNode,
	}

	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
		EdgeSourceMap: edgeMap,
		NodeMap:       nodeMap,
	}

	// 4. Run
	t.Logf("Running Multi-Step AI Flow...")
	res := n.RunSync(ctx, req)
	assert.NoError(t, res.Err)

	// 5. Verify
	val, err := node.ReadNodeVar(req, "AI_ORCHESTRATOR", "text")
	assert.NoError(t, err)
	t.Logf("Final AI Response: %v", val)
	
	assert.Contains(t, val, "MISSION_COMPLETED", "AI should retrieve the final secret")
}
