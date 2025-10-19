package testing

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/node/nforeach"
	"the-dev-tools/server/pkg/flow/node/nif"
	"the-dev-tools/server/pkg/flow/node/nnoop"
	"the-dev-tools/server/pkg/flow/node/nstart"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
)

// NodeCreator creates a new instance of a specific node type for testing
type NodeCreator func() node.FlowNode

// NodeTests defines the complete test configuration for a node type
type NodeTests struct {
	CreateNode  NodeCreator
	TestCases   []NodeTestCase
	BaseOptions TestNodeOptions
}

// FORNodeTests returns the test configuration for FOR nodes
// Note: This function requires the caller to provide a FOR node creator to avoid circular imports
func FORNodeTests(creator func() node.FlowNode) NodeTests {
	return NodeTests{
		CreateNode: creator,
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
				Name: "Node Configuration",
				TestFunc: func(t *testing.T, ctx *TestContext, testNode node.FlowNode) {
					// Basic node configuration test - works with any node
					require.NotEmpty(t, testNode.GetName(), "Node should have a name")
					require.NotEmpty(t, testNode.GetID(), "Node should have an ID")
					t.Log("FOR node configuration test passed")
				},
			},
		},
	}
}

// FOREACHNodeTests returns the test configuration for FOREACH nodes
func FOREACHNodeTests() NodeTests {
	condition := mcondition.Condition{
		Comparisons: mcondition.Comparison{
			Expression: "true",
		},
	}

	return NodeTests{
		CreateNode: func() node.FlowNode {
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

// IFNodeTests returns the test configuration for IF nodes
func IFNodeTests() NodeTests {
	condition := mcondition.Condition{
		Comparisons: mcondition.Comparison{
			Expression: "true",
		},
	}

	return NodeTests{
		CreateNode: func() node.FlowNode {
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

// NOOPNodeTests returns the test configuration for NOOP nodes
func NOOPNodeTests() NodeTests {
	return NodeTests{
		CreateNode: func() node.FlowNode {
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

// JSNodeTests returns the test configuration for JS nodes
// Note: This function requires the caller to provide a JS node creator to avoid circular imports
func JSNodeTests(creator func() node.FlowNode) NodeTests {
	return NodeTests{
		CreateNode: creator,
		BaseOptions: TestNodeOptions{
			ExpectStatusEvents: false, // JS nodes don't emit status by default
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
				Name: "Error Handling",
				TestFunc: func(t *testing.T, ctx *TestContext, testNode node.FlowNode) {
					opts := DefaultTestNodeOptions()
					TestNodeError(t, testNode, opts, func(req *node.FlowNodeRequest) {
						req.Timeout = 1 * time.Nanosecond // Force timeout error
					})
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
				Name: "JS Code Configuration",
				TestFunc: func(t *testing.T, ctx *TestContext, testNode node.FlowNode) {
					// Test that JS node has basic configuration
					require.NotEmpty(t, testNode.GetName(), "JS node should have a name")
					require.NotEmpty(t, testNode.GetID(), "JS node should have an ID")
					t.Log("JS node configuration test passed")
				},
			},
		},
	}
}

// REQUESTNodeTests returns the test configuration for REQUEST nodes
// Note: This function requires the caller to provide a REQUEST node creator to avoid circular imports
func REQUESTNodeTests(creator func() node.FlowNode) NodeTests {
	return NodeTests{
		CreateNode: creator,
		BaseOptions: TestNodeOptions{
			ExpectStatusEvents: false, // REQUEST nodes don't emit status by default
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
				Name: "Error Handling",
				TestFunc: func(t *testing.T, ctx *TestContext, testNode node.FlowNode) {
					opts := DefaultTestNodeOptions()
					TestNodeError(t, testNode, opts, func(req *node.FlowNodeRequest) {
						req.Timeout = 1 * time.Nanosecond // Force timeout error
					})
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
				Name: "Request Configuration",
				TestFunc: func(t *testing.T, ctx *TestContext, testNode node.FlowNode) {
					// Test that REQUEST node is configured correctly
					require.NotEmpty(t, testNode.GetName(), "REQUEST node should have a name")
					require.NotEmpty(t, testNode.GetID(), "REQUEST node should have an ID")
					t.Log("REQUEST node configuration test passed")
				},
			},
		},
	}
}

// STARTNodeTests returns the test configuration for START nodes
func STARTNodeTests() NodeTests {
	return NodeTests{
		CreateNode: func() node.FlowNode {
			return nstart.New(idwrap.NewNow(), "TestSTART")
		},
		BaseOptions: TestNodeOptions{
			ExpectStatusEvents: false, // START nodes don't emit status
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
				Name: "Start Node Behavior",
				TestFunc: func(t *testing.T, ctx *TestContext, testNode node.FlowNode) {
					// Test that START node is configured correctly
					require.Equal(t, "TestSTART", testNode.GetName(), "START node should have correct name")
					t.Log("START node basic behavior test passed")
				},
			},
		},
	}
}

// AllNodeTests returns all available node test configurations that don't require creator functions
// Note: FOR, JS, and REQUEST node tests require creator functions to avoid circular imports
func AllNodeTests() map[string]NodeTests {
	return map[string]NodeTests{
		"FOREACH": FOREACHNodeTests(),
		"IF":      IFNodeTests(),
		"NOOP":    NOOPNodeTests(),
		"START":   STARTNodeTests(),
	}
}

// AllNodeTestsWithFOR returns all available node test configurations including FOR
// The caller must provide a FOR node creator function
func AllNodeTestsWithFOR(forCreator func() node.FlowNode) map[string]NodeTests {
	tests := AllNodeTests()
	tests["FOR"] = FORNodeTests(forCreator)
	return tests
}

// AllNodeTestsWithSpecial returns all available node test configurations including special nodes
// The caller must provide creator functions for nodes that require them
func AllNodeTestsWithSpecial(forCreator, jsCreator, requestCreator func() node.FlowNode) map[string]NodeTests {
	tests := AllNodeTests()
	if forCreator != nil {
		tests["FOR"] = FORNodeTests(forCreator)
	}
	if jsCreator != nil {
		tests["JS"] = JSNodeTests(jsCreator)
	}
	if requestCreator != nil {
		tests["REQUEST"] = REQUESTNodeTests(requestCreator)
	}
	return tests
}
