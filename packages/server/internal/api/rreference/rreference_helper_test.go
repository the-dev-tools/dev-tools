package rreference

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIsIterationExecution tests the helper function
func TestIsIterationExecution(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Iteration 0", "Iteration 0", true},
		{"Iteration 1", "Iteration 1", true},
		{"Iteration 100", "Iteration 100", true},
		{"Error Summary", "Error Summary", true},
		{"Main node name", "ForEachNode", false},
		{"Regular node", "MyRequestNode", false},
		{"Partial iteration", "Iter", false},
		{"Contains iteration", "MyIteration 0", false},
		{"Empty string", "", false},
		{"Node with iteration path", "OuterLoop iteration 1, InnerLoop iteration 2", false},
		{"Just Iteration", "Iteration", false},
		{"Iteration with extra text", "Iteration 0 extra", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isIterationExecution(tt.input)
			assert.Equal(t, tt.expected, result, "For input: %s", tt.input)
		})
	}
}
