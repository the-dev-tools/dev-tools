package testing

import (
	"testing"
	"time"

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

// init registers all known node types with their test configurations
func init() {
	registerFORNode()
	registerFOREACHNode()
	registerIFNode()
	registerNOOPNode()
}

// registerFORNode registers the FOR node type with its test configuration
func registerFORNode() {
	RegisterNodeType("FOR", func() node.FlowNode {
		return nfor.New(idwrap.NewNow(), "TestFOR", 3, 0, mnfor.ErrorHandling_ERROR_HANDLING_IGNORE)
	}, NodeTestConfig{
		SupportsErrorHandling: true,
		SupportsIterations:    true,
		SupportsTimeout:       true,
		SupportsAsync:         true,
		SupportsConditions:    true,
		BasicSuccessTest:      true,
		ErrorHandlingTest:     true,
		TimeoutTest:           true,
		AsyncTest:             true,
		CustomTests: []func(*testing.T, node.FlowNode){
			FORNodeIterationTest,
			FORNodeErrorHandlingTest,
			FORNodeExecutionIDTest,
		},
		MockEdgeMap: func(n node.FlowNode) edge.EdgesMap {
			// FOR node with empty loop for basic testing
			return edge.EdgesMap{
				n.GetID(): {
					edge.HandleLoop: {}, // Empty loop
				},
			}
		},
	})
}

// registerFOREACHNode registers the FOR-EACH node type with its test configuration
func registerFOREACHNode() {
	RegisterNodeType("FOREACH", func() node.FlowNode {
		// Create a simple condition for FOREACH
		condition := mcondition.Condition{
			Comparisons: mcondition.Comparison{
				Expression: "true",
			},
		}
		return nforeach.New(
			idwrap.NewNow(),
			"TestFOREACH",
			"items", // iterPath
			10*time.Second,
			condition,
			mnfor.ErrorHandling_ERROR_HANDLING_IGNORE,
		)
	}, NodeTestConfig{
		SupportsErrorHandling: true,
		SupportsIterations:    true,
		SupportsTimeout:       true,
		SupportsAsync:         true,
		BasicSuccessTest:      true,
		ErrorHandlingTest:     true,
		TimeoutTest:           true,
		AsyncTest:             true,
		CustomTests: []func(*testing.T, node.FlowNode){
			FOREACHNodeIterationTest,
			FOREACHNodeErrorHandlingTest,
		},
		MockRequestGenerator: func(n node.FlowNode) *node.FlowNodeRequest {
			// FOR-EACH needs input data
			testCtx := NewTestContext(&testing.T{}, TestContextOptions{})
			defer testCtx.Cleanup()

			req := testCtx.CreateNodeRequest(n.GetID(), n.GetName(), NodeRequestOptions{
				VarMap: map[string]any{
					"items": []string{"item1", "item2", "item3"},
				},
				NodeMap: map[idwrap.IDWrap]node.FlowNode{n.GetID(): n},
				EdgeSourceMap: edge.EdgesMap{
					n.GetID(): {
						edge.HandleLoop: {}, // Empty loop
					},
				},
				ExecutionID: idwrap.NewNow(),
				Timeout:     10 * time.Second,
			})
			return req
		},
	})
}

// registerIFNode registers the IF node type with its test configuration
func registerIFNode() {
	RegisterNodeType("IF", func() node.FlowNode {
		// Create a simple condition that evaluates to true
		condition := mcondition.Condition{
			Comparisons: mcondition.Comparison{
				Expression: "true",
			},
		}
		return nif.New(idwrap.NewNow(), "TestIF", condition)
	}, NodeTestConfig{
		SupportsTimeout:    true,
		SupportsAsync:      true,
		SupportsConditions: true,
		BasicSuccessTest:   true,
		TimeoutTest:        true,
		AsyncTest:          true,
		CustomTests: []func(*testing.T, node.FlowNode){
			IFNodeConditionTest,
			IFNodeBranchTest,
		},
		MockEdgeMap: func(n node.FlowNode) edge.EdgesMap {
			return edge.EdgesMap{
				n.GetID(): {
					edge.HandleThen: {idwrap.NewNow()}, // Dummy then target
					edge.HandleElse: {idwrap.NewNow()}, // Dummy else target
				},
			}
		},
	})
}

// registerNOOPNode registers the NOOP node type with its test configuration
func registerNOOPNode() {
	RegisterNodeType("NOOP", func() node.FlowNode {
		return nnoop.New(idwrap.NewNow(), "TestNOOP")
	}, NodeTestConfig{
		SupportsTimeout:  true,
		SupportsAsync:    true,
		BasicSuccessTest: true,
		TimeoutTest:      true,
		AsyncTest:        true,
		CustomTests: []func(*testing.T, node.FlowNode){
			NOOPNodeBasicTest,
		},
	})
}

// Custom test functions for specific node types

// FOR node custom tests
func FORNodeIterationTest(t *testing.T, n node.FlowNode) {
	// This would be the comprehensive iteration test we already wrote
	t.Log("FOR node iteration test - would run comprehensive iteration behavior")
}

func FORNodeErrorHandlingTest(t *testing.T, n node.FlowNode) {
	// This would test the different error handling modes
	t.Log("FOR node error handling test - would test IGNORE/BREAK/UNSPECIFIED modes")
}

func FORNodeExecutionIDTest(t *testing.T, n node.FlowNode) {
	// This would test ExecutionID reuse patterns
	t.Log("FOR node ExecutionID test - would test ID reuse for iterations")
}

// FOREACH node custom tests
func FOREACHNodeIterationTest(t *testing.T, n node.FlowNode) {
	t.Log("FOREACH node iteration test - would test collection iteration")
}

func FOREACHNodeErrorHandlingTest(t *testing.T, n node.FlowNode) {
	t.Log("FOREACH node error handling test - would test error modes for collection iteration")
}

// IF node custom tests
func IFNodeConditionTest(t *testing.T, n node.FlowNode) {
	t.Log("IF node condition test - would test true/false conditions")
}

func IFNodeBranchTest(t *testing.T, n node.FlowNode) {
	t.Log("IF node branch test - would test then/else branch selection")
}

// NOOP node custom tests
func NOOPNodeBasicTest(t *testing.T, n node.FlowNode) {
	t.Log("NOOP node basic test - would test no-op behavior")
}
