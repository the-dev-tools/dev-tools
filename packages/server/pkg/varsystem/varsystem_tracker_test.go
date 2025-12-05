package varsystem_test

import (
	"os"
	"testing"

	"the-dev-tools/server/pkg/model/mvar"
	"the-dev-tools/server/pkg/varsystem"

	"github.com/stretchr/testify/require")

func TestVarMapTracker_Get(t *testing.T) {
	// Create a base VarMap
	vars := []mvar.Var{
		{VarKey: "token", Value: "abc123"},
		{VarKey: "baseUrl", Value: "https://api.example.com"},
	}
	varMap := varsystem.NewVarMap(vars)

	// Create tracker
	tracker := varsystem.NewVarMapTracker(varMap)

	// Test getting a variable
	val, ok := tracker.Get("token")
	if !ok {
		t.Fatalf("Expected to find variable 'token', but it was not found")
	}
	if val.Value != "abc123" {
		t.Errorf("Expected token value 'abc123', got '%s'", val.Value)
	}

	// Check that the variable was tracked
	readVars := tracker.GetReadVars()
	if len(readVars) != 1 {
		t.Errorf("Expected 1 tracked read, got %d", len(readVars))
	}
	if readVars["token"] != "abc123" {
		t.Errorf("Expected tracked token value 'abc123', got '%s'", readVars["token"])
	}

	// Test getting another variable
	val2, ok2 := tracker.Get("baseUrl")
	if !ok2 {
		t.Fatalf("Expected to find variable 'baseUrl', but it was not found")
	}
	if val2.Value != "https://api.example.com" {
		t.Errorf("Expected baseUrl value 'https://api.example.com', got '%s'", val2.Value)
	}

	// Check that both variables are tracked
	readVars = tracker.GetReadVars()
	if len(readVars) != 2 {
		t.Errorf("Expected 2 tracked reads, got %d", len(readVars))
	}
}

func TestVarMapTracker_ReplaceVars(t *testing.T) {
	// Create a base VarMap
	vars := []mvar.Var{
		{VarKey: "token", Value: "abc123"},
		{VarKey: "baseUrl", Value: "https://api.example.com"},
		{VarKey: "version", Value: "v1"},
	}
	varMap := varsystem.NewVarMap(vars)

	// Create tracker
	tracker := varsystem.NewVarMapTracker(varMap)

	// Test replacing variables in a URL
	input := "{{baseUrl}}/{{version}}/users?token={{token}}"
	result, err := tracker.ReplaceVars(input)
	require.NoError(t, err, "Unexpected error")

	expected := "https://api.example.com/v1/users?token=abc123"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}

	// Check that all variables were tracked
	readVars := tracker.GetReadVars()
	if len(readVars) != 3 {
		t.Errorf("Expected 3 tracked reads, got %d", len(readVars))
	}

	expectedVars := map[string]string{
		"token":   "abc123",
		"baseUrl": "https://api.example.com",
		"version": "v1",
	}

	for key, expectedValue := range expectedVars {
		if readVars[key] != expectedValue {
			t.Errorf("Expected tracked %s value '%s', got '%s'", key, expectedValue, readVars[key])
		}
	}
}

func TestVarMapTracker_ReplaceVars_SingleVariable(t *testing.T) {
	// Create a base VarMap
	vars := []mvar.Var{
		{VarKey: "message", Value: "Hello World"},
	}
	varMap := varsystem.NewVarMap(vars)

	// Create tracker
	tracker := varsystem.NewVarMapTracker(varMap)

	// Test replacing a single variable
	input := "{{message}}"
	result, err := tracker.ReplaceVars(input)
	require.NoError(t, err, "Unexpected error")

	if result != "Hello World" {
		t.Errorf("Expected 'Hello World', got '%s'", result)
	}

	// Check tracking
	readVars := tracker.GetReadVars()
	if len(readVars) != 1 {
		t.Errorf("Expected 1 tracked read, got %d", len(readVars))
	}
	if readVars["message"] != "Hello World" {
		t.Errorf("Expected tracked message value 'Hello World', got '%s'", readVars["message"])
	}
}

func TestVarMapTracker_ReplaceVars_MissingVariable(t *testing.T) {
	// Create an empty VarMap
	vars := []mvar.Var{}
	varMap := varsystem.NewVarMap(vars)

	// Create tracker
	tracker := varsystem.NewVarMapTracker(varMap)

	// Test replacing a missing variable
	input := "{{missing}}"
	_, err := tracker.ReplaceVars(input)
	if err == nil {
		t.Fatalf("Expected error for missing variable, but got nil")
	}

	// Check that no variables were tracked
	readVars := tracker.GetReadVars()
	if len(readVars) != 0 {
		t.Errorf("Expected 0 tracked reads, got %d", len(readVars))
	}
}

func TestVarMapTracker_ReplaceVars_NoVariables(t *testing.T) {
	// Create a base VarMap
	vars := []mvar.Var{
		{VarKey: "token", Value: "abc123"},
	}
	varMap := varsystem.NewVarMap(vars)

	// Create tracker
	tracker := varsystem.NewVarMapTracker(varMap)

	// Test string with no variables
	input := "https://api.example.com/users"
	result, err := tracker.ReplaceVars(input)
	require.NoError(t, err, "Unexpected error")

	if result != input {
		t.Errorf("Expected '%s', got '%s'", input, result)
	}

	// Check that no variables were tracked
	readVars := tracker.GetReadVars()
	if len(readVars) != 0 {
		t.Errorf("Expected 0 tracked reads, got %d", len(readVars))
	}
}

func TestVarMapTracker_GetReadVars_IsolatedCopy(t *testing.T) {
	// Create a base VarMap
	vars := []mvar.Var{
		{VarKey: "token", Value: "abc123"},
	}
	varMap := varsystem.NewVarMap(vars)

	// Create tracker
	tracker := varsystem.NewVarMapTracker(varMap)

	// Track a variable
	tracker.Get("token")

	// Get read vars
	readVars1 := tracker.GetReadVars()
	readVars2 := tracker.GetReadVars()

	// Modify one copy
	readVars1["token"] = "modified"

	// Check that the other copy is unaffected
	if readVars2["token"] != "abc123" {
		t.Errorf("Expected unmodified copy to have value 'abc123', got '%s'", readVars2["token"])
	}

	// Check that the tracker's internal state is unaffected
	readVars3 := tracker.GetReadVars()
	if readVars3["token"] != "abc123" {
		t.Errorf("Expected tracker internal state to be unaffected, got '%s'", readVars3["token"])
	}
}

func TestVarMapTracker_ReplaceVars_EnvReference(t *testing.T) {
	const envKey = "VARSYSTEM_TRACKER_ENV"
	const envValue = "tracker-env-value"
	prevValue, had := os.LookupEnv(envKey)
	if err := os.Setenv(envKey, envValue); err != nil {
		t.Fatalf("failed to set env: %v", err)
	}
	defer func() {
		if had {
			_ = os.Setenv(envKey, prevValue)
		} else {
			_ = os.Unsetenv(envKey)
		}
	}()

	vars := []mvar.Var{
		{VarKey: "token", Value: "#env:" + envKey},
	}
	tracker := varsystem.NewVarMapTracker(varsystem.NewVarMap(vars))

	result, err := tracker.ReplaceVars("Bearer {{token}}")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result != "Bearer "+envValue {
		t.Fatalf("expected %q, got %q", "Bearer "+envValue, result)
	}

	readVars := tracker.GetReadVars()
	if readVars["token"] != envValue {
		t.Fatalf("expected tracked value %q, got %q", envValue, readVars["token"])
	}
}
