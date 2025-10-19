package testing

import (
	"testing"

	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/node/nfor"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
)

// TestAllNodes runs all test configurations for all node types using idiomatic Go patterns
func TestAllNodes(t *testing.T) {
	// Create FOR node creator to avoid circular imports
	forCreator := func() node.FlowNode {
		return nfor.New(idwrap.NewNow(), "TestFOR", 3, 0, mnfor.ErrorHandling_ERROR_HANDLING_IGNORE)
	}

	nodeTests := AllNodeTestsWithFOR(forCreator)

	for nodeName, tests := range nodeTests {
		t.Run(nodeName, func(t *testing.T) {
			t.Logf("Running tests for node type: %s", nodeName)

			// Create a fresh node instance for this test run
			testNode := tests.CreateNode()
			if testNode == nil {
				t.Fatalf("Node creator returned nil for %s", nodeName)
			}

			// Run all test cases for this node
			RunNodeTests(t, testNode, tests.TestCases)
		})
	}
}

// TestFORNode runs specific tests for FOR nodes
func TestFORNode(t *testing.T) {
	forCreator := func() node.FlowNode {
		return nfor.New(idwrap.NewNow(), "TestFOR", 3, 0, mnfor.ErrorHandling_ERROR_HANDLING_IGNORE)
	}
	tests := FORNodeTests(forCreator)
	testNode := tests.CreateNode()
	RunNodeTests(t, testNode, tests.TestCases)
}

// TestFOREACHNode runs specific tests for FOREACH nodes
func TestFOREACHNode(t *testing.T) {
	tests := FOREACHNodeTests()
	testNode := tests.CreateNode()
	RunNodeTests(t, testNode, tests.TestCases)
}

// TestIFNode runs specific tests for IF nodes
func TestIFNode(t *testing.T) {
	tests := IFNodeTests()
	testNode := tests.CreateNode()
	RunNodeTests(t, testNode, tests.TestCases)
}

// TestNOOPNode runs specific tests for NOOP nodes
func TestNOOPNode(t *testing.T) {
	tests := NOOPNodeTests()
	testNode := tests.CreateNode()
	RunNodeTests(t, testNode, tests.TestCases)
}

// TestGenericNodeExecutionLegacy maintains compatibility with the old test framework
// This function preserves the original test behavior while using the new idiomatic approach
func TestGenericNodeExecution(t *testing.T) {
	t.Log("Running generic tests for all registered node types using idiomatic approach...")
	TestAllNodes(t)
}

// TestFORNodeGeneric maintains compatibility with the old FOR node test
func TestFORNodeGeneric(t *testing.T) {
	t.Log("Testing FOR node through idiomatic framework...")
	TestFORNode(t)
}

// TestNOOPNodeGeneric maintains compatibility with the old NOOP node test
func TestNOOPNodeGeneric(t *testing.T) {
	t.Log("Testing NOOP node through idiomatic framework...")
	TestNOOPNode(t)
}
