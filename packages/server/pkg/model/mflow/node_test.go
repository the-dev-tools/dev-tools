package mflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStringNodeState(t *testing.T) {
	tests := []struct {
		name     string
		state    NodeState
		expected string
	}{
		{
			name:     "UNSPECIFIED state",
			state:    NODE_STATE_UNSPECIFIED,
			expected: "Unspecified",
		},
		{
			name:     "RUNNING state",
			state:    NODE_STATE_RUNNING,
			expected: "Running",
		},
		{
			name:     "SUCCESS state",
			state:    NODE_STATE_SUCCESS,
			expected: "Success",
		},
		{
			name:     "FAILURE state",
			state:    NODE_STATE_FAILURE,
			expected: "Failure",
		},
		{
			name:     "CANCELED state",
			state:    NODE_STATE_CANCELED,
			expected: "Canceled",
		},
		{
			name:     "Invalid state -1",
			state:    NodeState(-1),
			expected: "Unknown",
		},
		{
			name:     "Invalid state -100",
			state:    NodeState(-100),
			expected: "Unknown",
		},
		{
			name:     "Invalid state 5",
			state:    NodeState(5),
			expected: "Unknown",
		},
		{
			name:     "Invalid state 127",
			state:    NodeState(127),
			expected: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StringNodeState(tt.state)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestStringNodeStateWithIcons(t *testing.T) {
	tests := []struct {
		name     string
		state    NodeState
		expected string
	}{
		{
			name:     "UNSPECIFIED state",
			state:    NODE_STATE_UNSPECIFIED,
			expected: "🔄 Starting",
		},
		{
			name:     "RUNNING state",
			state:    NODE_STATE_RUNNING,
			expected: "⏳ Running",
		},
		{
			name:     "SUCCESS state",
			state:    NODE_STATE_SUCCESS,
			expected: "✅ Success",
		},
		{
			name:     "FAILURE state",
			state:    NODE_STATE_FAILURE,
			expected: "❌ Failed",
		},
		{
			name:     "CANCELED state",
			state:    NODE_STATE_CANCELED,
			expected: "Canceled",
		},
		{
			name:     "Invalid state -1",
			state:    NodeState(-1),
			expected: "❓ Unknown",
		},
		{
			name:     "Invalid state 5",
			state:    NodeState(5),
			expected: "❓ Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StringNodeStateWithIcons(tt.state)
			assert.Equal(t, tt.expected, result)
		})
	}
}
