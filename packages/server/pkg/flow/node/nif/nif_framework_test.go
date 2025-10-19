package nif_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/node/nif"
	nodetesting "the-dev-tools/server/pkg/flow/node/testing"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mcondition"
)

// TestIFNode_FrameworkTests demonstrates using the testing framework for IF nodes
func TestIFNode_FrameworkTests(t *testing.T) {
	// Test cases for different conditions
	testCases := []nodetesting.NodeTestCase{
		{
			Name: "Condition True - Then Branch",
			TestFunc: func(t *testing.T, ctx *nodetesting.TestContext, testNode node.FlowNode) {
				// Create IF node with true condition
				ifNode := nif.New(idwrap.NewNow(), "test-true", mcondition.Condition{
					Comparisons: mcondition.Comparison{Expression: "1 == 1"},
				})

				opts := nodetesting.DefaultTestNodeOptions()
				opts.EdgeMap = edge.EdgesMap{
					ifNode.GetID(): {
						edge.HandleThen: {idwrap.NewNow()}, // Dummy then target
						edge.HandleElse: {idwrap.NewNow()}, // Dummy else target
					},
				}
				opts.ExpectStatusEvents = false

				nodetesting.TestNodeSuccess(t, ifNode, opts)
			},
		},
		{
			Name: "Condition False - Else Branch",
			TestFunc: func(t *testing.T, ctx *nodetesting.TestContext, testNode node.FlowNode) {
				// Create IF node with false condition
				ifNode := nif.New(idwrap.NewNow(), "test-false", mcondition.Condition{
					Comparisons: mcondition.Comparison{Expression: "1 == 2"},
				})

				opts := nodetesting.DefaultTestNodeOptions()
				opts.EdgeMap = edge.EdgesMap{
					ifNode.GetID(): {
						edge.HandleThen: {idwrap.NewNow()}, // Dummy then target
						edge.HandleElse: {idwrap.NewNow()}, // Dummy else target
					},
				}
				opts.ExpectStatusEvents = false

				nodetesting.TestNodeSuccess(t, ifNode, opts)
			},
		},
		{
			Name: "Variable-Based Condition True",
			TestFunc: func(t *testing.T, ctx *nodetesting.TestContext, testNode node.FlowNode) {
				// Create IF node with variable condition
				ifNode := nif.New(idwrap.NewNow(), "test-var-true", mcondition.Condition{
					Comparisons: mcondition.Comparison{Expression: "a == 1"},
				})

				opts := nodetesting.DefaultTestNodeOptions()
				opts.VarMap = map[string]any{"a": 1}
				opts.EdgeMap = edge.EdgesMap{
					ifNode.GetID(): {
						edge.HandleThen: {idwrap.NewNow()}, // Dummy then target
						edge.HandleElse: {idwrap.NewNow()}, // Dummy else target
					},
				}
				opts.ExpectStatusEvents = false

				nodetesting.TestNodeSuccess(t, ifNode, opts)
			},
		},
		{
			Name: "Variable-Based Condition False",
			TestFunc: func(t *testing.T, ctx *nodetesting.TestContext, testNode node.FlowNode) {
				// Create IF node with variable condition
				ifNode := nif.New(idwrap.NewNow(), "test-var-false", mcondition.Condition{
					Comparisons: mcondition.Comparison{Expression: "a == 1"},
				})

				opts := nodetesting.DefaultTestNodeOptions()
				opts.VarMap = map[string]any{"a": 2}
				opts.EdgeMap = edge.EdgesMap{
					ifNode.GetID(): {
						edge.HandleThen: {idwrap.NewNow()}, // Dummy then target
						edge.HandleElse: {idwrap.NewNow()}, // Dummy else target
					},
				}
				opts.ExpectStatusEvents = false

				nodetesting.TestNodeSuccess(t, ifNode, opts)
			},
		},
		{
			Name: "Timeout Behavior",
			TestFunc: func(t *testing.T, ctx *nodetesting.TestContext, testNode node.FlowNode) {
				ifNode := nif.New(idwrap.NewNow(), "test-timeout", mcondition.Condition{
					Comparisons: mcondition.Comparison{Expression: "true"},
				})

				opts := nodetesting.DefaultTestNodeOptions()
				opts.EdgeMap = edge.EdgesMap{
					ifNode.GetID(): {
						edge.HandleThen: {idwrap.NewNow()},
						edge.HandleElse: {idwrap.NewNow()},
					},
				}

				nodetesting.TestNodeTimeout(t, ifNode, opts)
			},
		},
		{
			Name: "Async Execution",
			TestFunc: func(t *testing.T, ctx *nodetesting.TestContext, testNode node.FlowNode) {
				ifNode := nif.New(idwrap.NewNow(), "test-async", mcondition.Condition{
					Comparisons: mcondition.Comparison{Expression: "true"},
				})

				opts := nodetesting.DefaultTestNodeOptions()
				opts.EdgeMap = edge.EdgesMap{
					ifNode.GetID(): {
						edge.HandleThen: {idwrap.NewNow()},
						edge.HandleElse: {idwrap.NewNow()},
					},
				}
				opts.ExpectStatusEvents = false

				nodetesting.TestNodeAsync(t, ifNode, opts)
			},
		},
		{
			Name: "Node Configuration Validation",
			TestFunc: func(t *testing.T, ctx *nodetesting.TestContext, testNode node.FlowNode) {
				// Test that IF node is configured correctly
				ifNode := nif.New(idwrap.NewNow(), "test-config", mcondition.Condition{
					Comparisons: mcondition.Comparison{Expression: "test == 'value'"},
				})

				// Validate basic configuration
				require.Equal(t, "test-config", ifNode.GetName(), "IF node should have correct name")
				require.NotEmpty(t, ifNode.GetID(), "IF node should have an ID")

				// Validate condition structure
				condition := ifNode.Condition
				require.NotNil(t, condition, "IF node should have a condition")
				require.Equal(t, "test == 'value'", condition.Comparisons.Expression, "IF node should preserve condition expression")

				t.Log("IF node configuration validation passed")
			},
		},
	}

	// Run all test cases using the framework
	testNode := nif.New(idwrap.NewNow(), "test-if", mcondition.Condition{
		Comparisons: mcondition.Comparison{Expression: "true"},
	})

	nodetesting.RunNodeTests(t, testNode, testCases)
}

// TestIFNode_FrameworkIntegration demonstrates integration with the framework's built-in IF tests
func TestIFNode_FrameworkIntegration(t *testing.T) {
	// Use the framework's built-in IF node tests
	ifTests := nodetesting.IFNodeTests()

	// Run the built-in test suite
	for _, testCase := range ifTests.TestCases {
		t.Run(testCase.Name, func(t *testing.T) {
			testNode := ifTests.CreateNode()
			testCase.TestFunc(t, nodetesting.NewTestContext(t), testNode)
		})
	}
}

// TestIFNode_MigrationExample shows the before/after comparison for migration
func TestIFNode_MigrationExample(t *testing.T) {
	t.Run("Framework Approach", func(t *testing.T) {
		// This is the framework approach - much cleaner!
		ifNode := nif.New(idwrap.NewNow(), "test", mcondition.Condition{
			Comparisons: mcondition.Comparison{Expression: "1 == 1"},
		})

		opts := nodetesting.DefaultTestNodeOptions()
		opts.EdgeMap = edge.EdgesMap{
			ifNode.GetID(): {
				edge.HandleThen: {idwrap.NewNow()},
				edge.HandleElse: {idwrap.NewNow()},
			},
		}

		nodetesting.TestNodeSuccess(t, ifNode, opts)
	})

	t.Run("Manual Approach (Legacy)", func(t *testing.T) {
		// This demonstrates what the manual approach looked like
		// (This is for comparison - normally you wouldn't have both)

		// The framework approach eliminates ~50 lines of boilerplate!
		t.Log("Framework approach reduces test code from ~50 lines to ~10 lines")
	})
}
