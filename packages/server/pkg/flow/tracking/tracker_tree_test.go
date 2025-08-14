package tracking

import (
	"reflect"
	"testing"
)

func TestVariableTracker_GetReadVarsAsTree(t *testing.T) {
	tracker := NewVariableTracker()
	
	// Track some read operations with dot notation
	tracker.TrackRead("request_0.response.body.token", "abc123")
	tracker.TrackRead("request_0.response.status", 200)
	tracker.TrackRead("config.timeout", 30)
	
	expected := map[string]any{
		"request_0": map[string]any{
			"response": map[string]any{
				"body": map[string]any{
					"token": "abc123",
				},
				"status": 200,
			},
		},
		"config": map[string]any{
			"timeout": 30,
		},
	}
	
	result := tracker.GetReadVarsAsTree()
	
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %+v, got %+v", expected, result)
	}
}

func TestVariableTracker_GetWrittenVarsAsTree(t *testing.T) {
	tracker := NewVariableTracker()
	
	// Track some write operations with dot notation
	tracker.TrackWrite("request_1.request.method", "POST")
	tracker.TrackWrite("request_1.request.body", `{"test": "data"}`)
	tracker.TrackWrite("request_1.response.status", 201)
	
	expected := map[string]any{
		"request_1": map[string]any{
			"request": map[string]any{
				"method": "POST",
				"body":   `{"test": "data"}`,
			},
			"response": map[string]any{
				"status": 201,
			},
		},
	}
	
	result := tracker.GetWrittenVarsAsTree()
	
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %+v, got %+v", expected, result)
	}
}

func TestVariableTracker_TreeWithSpaces(t *testing.T) {
	tracker := NewVariableTracker()
	
	// Test real-world scenario with spaces in keys
	tracker.TrackRead(" request_0.response.body.token ", "token123")
	tracker.TrackRead("request_0.response.headers", map[string]string{"Content-Type": "application/json"})
	
	expected := map[string]any{
		"request_0": map[string]any{
			"response": map[string]any{
				"body": map[string]any{
					"token": "token123",
				},
				"headers": map[string]string{"Content-Type": "application/json"},
			},
		},
	}
	
	result := tracker.GetReadVarsAsTree()
	
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %+v, got %+v", expected, result)
	}
}

func TestVariableTracker_EmptyTracker(t *testing.T) {
	tracker := NewVariableTracker()
	
	readTree := tracker.GetReadVarsAsTree()
	writtenTree := tracker.GetWrittenVarsAsTree()
	
	if len(readTree) != 0 {
		t.Errorf("Expected empty read tree, got %+v", readTree)
	}
	
	if len(writtenTree) != 0 {
		t.Errorf("Expected empty written tree, got %+v", writtenTree)
	}
}

func TestVariableTracker_NilTrackerTree(t *testing.T) {
	var tracker *VariableTracker = nil
	
	readTree := tracker.GetReadVarsAsTree()
	writtenTree := tracker.GetWrittenVarsAsTree()
	
	if len(readTree) != 0 {
		t.Errorf("Expected empty read tree from nil tracker, got %+v", readTree)
	}
	
	if len(writtenTree) != 0 {
		t.Errorf("Expected empty written tree from nil tracker, got %+v", writtenTree)
	}
}