package testing

import (
	"testing"
)

// TestAllNodes runs all test suites for all node types using the new idiomatic approach
func TestAllNodes(t *testing.T) {
	nodeSuites := GetAllNodeSuites()

	for nodeName, suite := range nodeSuites {
		t.Run(nodeName, func(t *testing.T) {
			t.Logf("Running tests for node type: %s", nodeName)

			// Create a fresh node instance for this test run
			testNode := suite.Factory()
			if testNode == nil {
				t.Fatalf("Node factory returned nil for %s", nodeName)
			}

			// Run all test cases for this node
			RunNodeTests(t, testNode, suite.TestCases)
		})
	}
}

// TestFORNode runs specific tests for FOR nodes
func TestFORNode(t *testing.T) {
	suite := GetFORNodeSuite()
	testNode := suite.Factory()
	RunNodeTests(t, testNode, suite.TestCases)
}

// TestFOREACHNode runs specific tests for FOREACH nodes
func TestFOREACHNode(t *testing.T) {
	suite := GetFOREACHNodeSuite()
	testNode := suite.Factory()
	RunNodeTests(t, testNode, suite.TestCases)
}

// TestIFNode runs specific tests for IF nodes
func TestIFNode(t *testing.T) {
	suite := GetIFNodeSuite()
	testNode := suite.Factory()
	RunNodeTests(t, testNode, suite.TestCases)
}

// TestNOOPNode runs specific tests for NOOP nodes
func TestNOOPNode(t *testing.T) {
	suite := GetNOOPNodeSuite()
	testNode := suite.Factory()
	RunNodeTests(t, testNode, suite.TestCases)
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
