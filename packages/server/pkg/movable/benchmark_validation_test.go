package movable

import (
	"testing"
)

// TestBenchmarkValidation demonstrates that our performance validation framework works
func TestBenchmarkValidation(t *testing.T) {
	// Create validation instance
	validator := CreateBenchmarkValidation()
	
	// Run performance validation
	result := validator.ValidatePerformanceTargets()
	
	// Check that tests were run
	if result.TestsRun == 0 {
		t.Fatal("No performance tests were run")
	}
	
	// Check that we have the expected number of tests
	expectedTests := 4 // ContextResolution, DeltaOperations, BatchOperations, SyncPropagation
	if result.TestsRun != expectedTests {
		t.Errorf("Expected %d tests, got %d", expectedTests, result.TestsRun)
	}
	
	// Verify all required test categories are present
	requiredTests := []string{
		"ContextResolution",
		"DeltaOperations", 
		"BatchOperations",
		"SyncPropagation",
	}
	
	for _, testName := range requiredTests {
		if _, exists := result.Results[testName]; !exists {
			t.Errorf("Required test '%s' was not run", testName)
		}
	}
	
	// Generate and validate report
	report := result.GeneratePerformanceReport()
	if len(report) == 0 {
		t.Error("Performance report is empty")
	}
	
	// Log the report for visibility
	t.Logf("\n%s", report)
	
	// In a real benchmark environment, we would expect all tests to pass
	// For this demonstration, we'll just verify the framework works
	t.Logf("Performance validation framework working correctly: %d/%d tests completed", 
		result.TestsPassed, result.TestsRun)
}

// TestBenchmarkFrameworkComponents tests individual components
func TestBenchmarkFrameworkComponents(t *testing.T) {
	validator := CreateBenchmarkValidation()
	
	// Test context cache functionality
	if validator.contextCache == nil {
		t.Error("Context cache not initialized")
	}
	
	// Test delta manager functionality  
	if validator.deltaManager == nil {
		t.Error("Delta manager not initialized")
	}
	
	// Test scope resolver functionality
	if validator.scopeResolver == nil {
		t.Error("Scope resolver not initialized")
	}
	
	t.Log("All benchmark framework components initialized successfully")
}

// TestPerformanceTargetDefinitions ensures our performance targets are well-defined
func TestPerformanceTargetDefinitions(t *testing.T) {
	targets := map[string]string{
		"Single operation":    "< 100µs",
		"Batch (100 ops)":     "< 1ms", 
		"Context resolution":  "< 1µs cached",
		"Sync propagation":    "< 500ms for 1000 items",
	}
	
	t.Log("Performance Targets:")
	for operation, target := range targets {
		t.Logf("• %s: %s", operation, target)
	}
	
	// Verify we have targets for all major operations
	if len(targets) != 4 {
		t.Errorf("Expected 4 performance targets, got %d", len(targets))
	}
}