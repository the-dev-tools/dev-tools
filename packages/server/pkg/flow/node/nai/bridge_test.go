package nai

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/mocknode"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

func TestNodeToolBridge(t *testing.T) {
	ctx := context.Background()
	req := &node.FlowNodeRequest{
		VarMap: make(map[string]any),
	}

	runCount := 0
	mn := &mocknode.MockNode{
		ID: idwrap.NewNow(),
		OnRun: func() {
			runCount++
		},
	}

	nt := NewNodeTool(mn, req)
	lcTool := nt.AsLangChainTool()

	assert.Equal(t, "mock", lcTool.Function.Name)

	res, err := nt.Execute(ctx, "{}")
	assert.NoError(t, err)
	assert.Contains(t, res, "No output captured")
	assert.Equal(t, 1, runCount)
}

func TestNodeToolBridgeWithOutput(t *testing.T) {
	ctx := context.Background()
	req := &node.FlowNodeRequest{
		VarMap: make(map[string]any),
	}

	mn := &mocknode.MockNode{
		ID: idwrap.NewNow(),
		OnRun: func() {
			// Simulate node writing output to VarMap (like HTTP node does)
			req.VarMap["mock"] = map[string]any{
				"response": map[string]any{
					"status": float64(200),
					"body": map[string]any{
						"message": "Hello from API",
						"users":   []string{"alice", "bob"},
					},
				},
			}
		},
	}

	nt := NewNodeTool(mn, req)
	res, err := nt.Execute(ctx, "{}")

	assert.NoError(t, err)
	assert.Contains(t, res, `"status":200`)
	assert.Contains(t, res, `"message":"Hello from API"`)
	assert.Contains(t, res, `"users"`)
}
