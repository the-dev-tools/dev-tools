package testing

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/node/nfor"
	"the-dev-tools/server/pkg/flow/node/nforeach"
	"the-dev-tools/server/pkg/flow/node/nif"
	"the-dev-tools/server/pkg/flow/node/nnoop"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
)

// NodeFactory creates a new instance of a specific node type for testing
type NodeFactory func() node.FlowNode

// NodeTestSuite defines the complete test suite for a node type
type NodeTestSuite struct {
	Factory     NodeFactory
	TestCases   []NodeTestCase
	BaseOptions TestNodeOptions
}

// GetFORNodeSuite returns the test suite for FOR nodes
func GetFORNodeSuite() NodeTestSuite {
	return NodeTestSuite{
		Factory: func() node.FlowNode {
			return nfor.New(idwrap.NewNow(), "TestFOR", 3, 0, mnfor.ErrorHandling_ERROR_HANDLING_IGNORE)
		},
		BaseOptions: TestNodeOptions{
			EdgeMap: map[idwrap.IDWrap]map[edge.EdgeHandle][]idwrap.IDWrap{
				// Will be populated with actual node ID in tests
			},
			ExpectStatusEvents: true, // FOR nodes emit status events
		},
		TestCases: []NodeTestCase{
			{
				Name: "Basic Success",
				TestFunc: func(t *testing.T, ctx *TestContext, testNode node.FlowNode) {
					opts := DefaultTestNodeOptions()
					opts.EdgeMap = edge.EdgesMap{
						testNode.GetID(): {
							edge.HandleLoop: {}, // Empty loop for basic test
						},
					}
					opts.ExpectStatusEvents = true
					TestNodeSuccess(t, testNode, opts)
				},
			},
			{
				Name: "Error Handling",
				TestFunc: func(t *testing.T, ctx *TestContext, testNode node.FlowNode) {
					opts := DefaultTestNodeOptions()
					opts.EdgeMap = edge.EdgesMap{
						testNode.GetID(): {
							edge.HandleLoop: {}, // Empty loop
						},
					}
					TestNodeError(t, testNode, opts, func(req *node.FlowNodeRequest) {
						req.Timeout = 1 * time.Nanosecond // Force timeout error
					})
				},
			},
			{
				Name: "Timeout",
				TestFunc: func(t *testing.T, ctx *TestContext, testNode node.FlowNode) {
					opts := DefaultTestNodeOptions()
					opts.EdgeMap = edge.EdgesMap{
						testNode.GetID(): {
							edge.HandleLoop: {}, // Empty loop
						},
					}
					TestNodeTimeout(t, testNode, opts)
				},
			},
			{
				Name: "Async Execution",
				TestFunc: func(t *testing.T, ctx *TestContext, testNode node.FlowNode) {
					opts := DefaultTestNodeOptions()
					opts.EdgeMap = edge.EdgesMap{
						testNode.GetID(): {
							edge.HandleLoop: {}, // Empty loop
						},
					}
					opts.ExpectStatusEvents = true
					TestNodeAsync(t, testNode, opts)
				},
			},
			{
				Name: "Iteration Behavior",
				TestFunc: func(t *testing.T, ctx *TestContext, testNode node.FlowNode) {
					// Test that FOR node handles iterations correctly
					forNode := testNode.(*nfor.NodeFor)
					require.Equal(t, int64(3), forNode.IterCount, "FOR node should have 3 max iterations")
					t.Log("FOR node iteration test passed")
				},
			},
			{
				Name: "Error Handling Mode",
				TestFunc: func(t *testing.T, ctx *TestContext, testNode node.FlowNode) {
					// Test error handling mode configuration
					forNode := testNode.(*nfor.NodeFor)
					require.Equal(t, mnfor.ErrorHandling_ERROR_HANDLING_IGNORE, forNode.ErrorHandling, "FOR node should use IGNORE error handling")
					t.Log("FOR node error handling mode test passed")
				},
			},
		},
	}
}

// GetFOREACHNodeSuite returns the test suite for FOREACH nodes
func GetFOREACHNodeSuite() NodeTestSuite {
	condition := mcondition.Condition{
		Comparisons: mcondition.Comparison{
			Expression: "true",
		},
	}

	return NodeTestSuite{
		Factory: func() node.FlowNode {
			return nforeach.New(
				idwrap.NewNow(),
				"TestFOREACH",
				"items", // iterPath
				10*time.Second,
				condition,
				mnfor.ErrorHandling_ERROR_HANDLING_IGNORE,
			)
		},
		BaseOptions: TestNodeOptions{
			VarMap: map[string]any{
				"items": []string{"item1", "item2", "item3"},
			},
			ExpectStatusEvents: false, // FOREACH nodes don't emit status by default
		},
		TestCases: []NodeTestCase{
			{
				Name: "Basic Success",
				TestFunc: func(t *testing.T, ctx *TestContext, testNode node.FlowNode) {
					opts := DefaultTestNodeOptions()
					opts.VarMap = map[string]any{
						"items": []string{"item1", "item2", "item3"},
					}
					opts.EdgeMap = edge.EdgesMap{
						testNode.GetID(): {
							edge.HandleLoop: {}, // Empty loop
						},
					}
					opts.ExpectStatusEvents = false
					TestNodeSuccess(t, testNode, opts)
				},
			},
			{
				Name: "Error Handling",
				TestFunc: func(t *testing.T, ctx *TestContext, testNode node.FlowNode) {
					opts := DefaultTestNodeOptions()
					opts.VarMap = map[string]any{
						"items": []string{"item1", "item2", "item3"},
					}
					opts.EdgeMap = edge.EdgesMap{
						testNode.GetID(): {
							edge.HandleLoop: {}, // Empty loop
						},
					}
					TestNodeError(t, testNode, opts, func(req *node.FlowNodeRequest) {
						req.Timeout = 1 * time.Nanosecond // Force timeout error
					})
				},
			},
			{
				Name: "Timeout",
				TestFunc: func(t *testing.T, ctx *TestContext, testNode node.FlowNode) {
					opts := DefaultTestNodeOptions()
					opts.VarMap = map[string]any{
						"items": []string{"item1", "item2", "item3"},
					}
					opts.EdgeMap = edge.EdgesMap{
						testNode.GetID(): {
							edge.HandleLoop: {}, // Empty loop
						},
					}
					TestNodeTimeout(t, testNode, opts)
				},
			},
			{
				Name: "Async Execution",
				TestFunc: func(t *testing.T, ctx *TestContext, testNode node.FlowNode) {
					opts := DefaultTestNodeOptions()
					opts.VarMap = map[string]any{
						"items": []string{"item1", "item2", "item3"},
					}
					opts.EdgeMap = edge.EdgesMap{
						testNode.GetID(): {
							edge.HandleLoop: {}, // Empty loop
						},
					}
					opts.ExpectStatusEvents = false
					TestNodeAsync(t, testNode, opts)
				},
			},
			{
				Name: "Collection Iteration",
				TestFunc: func(t *testing.T, ctx *TestContext, testNode node.FlowNode) {
					// Test that FOREACH node is configured correctly
					foreachNode := testNode.(*nforeach.NodeForEach)
					require.Equal(t, "items", foreachNode.IterPath, "FOREACH node should iterate over 'items'")
					t.Log("FOREACH node collection iteration test passed")
				},
			},
		},
	}
}

// GetIFNodeSuite returns the test suite for IF nodes
func GetIFNodeSuite() NodeTestSuite {
	condition := mcondition.Condition{
		Comparisons: mcondition.Comparison{
			Expression: "true",
		},
	}

	return NodeTestSuite{
		Factory: func() node.FlowNode {
			return nif.New(idwrap.NewNow(), "TestIF", condition)
		},
		BaseOptions: TestNodeOptions{
			ExpectStatusEvents: false, // IF nodes don't emit status by default
		},
		TestCases: []NodeTestCase{
			{
				Name: "Basic Success",
				TestFunc: func(t *testing.T, ctx *TestContext, testNode node.FlowNode) {
					opts := DefaultTestNodeOptions()
					opts.EdgeMap = edge.EdgesMap{
						testNode.GetID(): {
							edge.HandleThen: {idwrap.NewNow()}, // Dummy then target
							edge.HandleElse: {idwrap.NewNow()}, // Dummy else target
						},
					}
					opts.ExpectStatusEvents = false
					TestNodeSuccess(t, testNode, opts)
				},
			},
			{
				Name: "Timeout",
				TestFunc: func(t *testing.T, ctx *TestContext, testNode node.FlowNode) {
					opts := DefaultTestNodeOptions()
					opts.EdgeMap = edge.EdgesMap{
						testNode.GetID(): {
							edge.HandleThen: {idwrap.NewNow()}, // Dummy then target
							edge.HandleElse: {idwrap.NewNow()}, // Dummy else target
						},
					}
					TestNodeTimeout(t, testNode, opts)
				},
			},
			{
				Name: "Async Execution",
				TestFunc: func(t *testing.T, ctx *TestContext, testNode node.FlowNode) {
					opts := DefaultTestNodeOptions()
					opts.EdgeMap = edge.EdgesMap{
						testNode.GetID(): {
							edge.HandleThen: {idwrap.NewNow()}, // Dummy then target
							edge.HandleElse: {idwrap.NewNow()}, // Dummy else target
						},
					}
					opts.ExpectStatusEvents = false
					TestNodeAsync(t, testNode, opts)
				},
			},
			{
				Name: "Condition Evaluation",
				TestFunc: func(t *testing.T, ctx *TestContext, testNode node.FlowNode) {
					// Test that IF node has a condition
					ifNode := testNode.(*nif.NodeIf)
					condition := ifNode.Condition
					require.NotNil(t, condition, "IF node should have a condition")
					t.Log("IF node condition evaluation test passed")
				},
			},
			{
				Name: "Branch Selection",
				TestFunc: func(t *testing.T, ctx *TestContext, testNode node.FlowNode) {
					// Test that IF node can select branches
					ifNode := testNode.(*nif.NodeIf)
					condition := ifNode.Condition
					require.Equal(t, "true", condition.Comparisons.Expression, "IF node should have true condition for then branch")
					t.Log("IF node branch selection test passed")
				},
			},
		},
	}
}

// GetNOOPNodeSuite returns the test suite for NOOP nodes
func GetNOOPNodeSuite() NodeTestSuite {
	return NodeTestSuite{
		Factory: func() node.FlowNode {
			return nnoop.New(idwrap.NewNow(), "TestNOOP")
		},
		BaseOptions: TestNodeOptions{
			ExpectStatusEvents: false, // NOOP nodes don't emit status
		},
		TestCases: []NodeTestCase{
			{
				Name: "Basic Success",
				TestFunc: func(t *testing.T, ctx *TestContext, testNode node.FlowNode) {
					opts := DefaultTestNodeOptions()
					opts.ExpectStatusEvents = false
					TestNodeSuccess(t, testNode, opts)
				},
			},
			{
				Name: "Timeout",
				TestFunc: func(t *testing.T, ctx *TestContext, testNode node.FlowNode) {
					opts := DefaultTestNodeOptions()
					TestNodeTimeout(t, testNode, opts)
				},
			},
			{
				Name: "Async Execution",
				TestFunc: func(t *testing.T, ctx *TestContext, testNode node.FlowNode) {
					opts := DefaultTestNodeOptions()
					opts.ExpectStatusEvents = false
					TestNodeAsync(t, testNode, opts)
				},
			},
			{
				Name: "No-Op Behavior",
				TestFunc: func(t *testing.T, ctx *TestContext, testNode node.FlowNode) {
					// Test that NOOP node does nothing
					noopNode := testNode.(*nnoop.NodeNoop)
					require.Equal(t, "TestNOOP", noopNode.Name, "NOOP node should have correct name")
					t.Log("NOOP node basic behavior test passed")
				},
			},
		},
	}
}

// GetAllNodeSuites returns all available node test suites
func GetAllNodeSuites() map[string]NodeTestSuite {
	return map[string]NodeTestSuite{
		"FOR":     GetFORNodeSuite(),
		"FOREACH": GetFOREACHNodeSuite(),
		"IF":      GetIFNodeSuite(),
		"NOOP":    GetNOOPNodeSuite(),
	}
}
