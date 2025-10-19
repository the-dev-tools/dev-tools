package testing

import (
	"testing"
)

// TestGenericNodeExecution runs the generic test framework for all registered node types
func TestGenericNodeExecution(t *testing.T) {
	t.Log("Running generic tests for all registered node types...")
	GenerateTestsForAllNodes(t)
}

// TestFORNodeGeneric specifically tests the FOR node through the generic framework
func TestFORNodeGeneric(t *testing.T) {
	t.Log("Testing FOR node through generic framework...")

	registry := GetNodeTypeRegistry()
	constructor, exists := registry.constructors["FOR"]
	if !exists {
		t.Fatal("FOR node not registered")
	}

	config := registry.configs["FOR"]
	testNode := constructor()

	t.Run("FOR Basic Success", func(t *testing.T) {
		GenericBasicSuccessTest(t, testNode, config)
	})

	t.Run("FOR Error Handling", func(t *testing.T) {
		GenericErrorHandlingTest(t, testNode, config)
	})

	t.Run("FOR Timeout", func(t *testing.T) {
		GenericTimeoutTest(t, testNode, config)
	})

	t.Run("FOR Async", func(t *testing.T) {
		GenericAsyncTest(t, testNode, config)
	})
}

// TestNOOPNodeGeneric specifically tests the NOOP node through the generic framework
func TestNOOPNodeGeneric(t *testing.T) {
	t.Log("Testing NOOP node through generic framework...")

	registry := GetNodeTypeRegistry()
	constructor, exists := registry.constructors["NOOP"]
	if !exists {
		t.Fatal("NOOP node not registered")
	}

	config := registry.configs["NOOP"]
	testNode := constructor()

	t.Run("NOOP Basic Success", func(t *testing.T) {
		GenericBasicSuccessTest(t, testNode, config)
	})

	t.Run("NOOP Async", func(t *testing.T) {
		GenericAsyncTest(t, testNode, config)
	})
}
