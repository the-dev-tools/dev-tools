package sortenabled_test

import (
	"reflect"
	"testing"
	"the-dev-tools/server/pkg/sort/sortenabled"
)

type MockEnabled struct {
	enabled bool
	value   int
}

func (m MockEnabled) IsEnabled() bool {
	return m.enabled
}

func TestSortEnabled(t *testing.T) {
	tests := []struct {
		name     string
		input    []MockEnabled
		state    bool
		expected []MockEnabled
	}{
		{
			name: "Sort with enabled true",
			input: []MockEnabled{
				{enabled: false, value: 1},
				{enabled: true, value: 2},
				{enabled: false, value: 3},
				{enabled: true, value: 4},
			},
			state: true,
			expected: []MockEnabled{
				{enabled: true, value: 2},
				{enabled: true, value: 4},
			},
		},
		{
			name: "Sort with enabled false",
			input: []MockEnabled{
				{enabled: false, value: 1},
				{enabled: true, value: 2},
				{enabled: false, value: 3},
				{enabled: true, value: 4},
			},
			state: false,
			expected: []MockEnabled{
				{enabled: false, value: 1},
				{enabled: false, value: 3},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputCopy := make([]MockEnabled, len(tt.input))
			copy(inputCopy, tt.input)

			sortenabled.GetAllWithState(&inputCopy, tt.state)

			if !reflect.DeepEqual(inputCopy, tt.expected) {
				t.Errorf("SortEnabled() = %v, want %v", inputCopy, tt.expected)
			}
		})
	}
}
