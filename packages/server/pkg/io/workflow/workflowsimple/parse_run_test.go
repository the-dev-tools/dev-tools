package workflowsimple

import (
	"reflect"
	"testing"
)

func TestParseRunFieldUnit(t *testing.T) {
	tests := []struct {
		name     string
		runArray []map[string]any
		want     []RunEntry
		wantErr  bool
	}{
		{
			name: "Single flow without dependencies",
			runArray: []map[string]any{
				{
					"flow": "FlowA",
				},
			},
			want: []RunEntry{
				{Flow: "FlowA", DependsOn: nil},
			},
			wantErr: false,
		},
		{
			name: "Flow with single string dependency",
			runArray: []map[string]any{
				{
					"flow":       "FlowB",
					"depends_on": "FlowA",
				},
			},
			want: []RunEntry{
				{Flow: "FlowB", DependsOn: []string{"FlowA"}},
			},
			wantErr: false,
		},
		{
			name: "Flow with multiple dependencies",
			runArray: []map[string]any{
				{
					"flow": "FlowC",
					"depends_on": []any{
						"RequestA",
						"FlowB",
					},
				},
			},
			want: []RunEntry{
				{Flow: "FlowC", DependsOn: []string{"RequestA", "FlowB"}},
			},
			wantErr: false,
		},
		{
			name: "Multiple flows with mixed dependencies",
			runArray: []map[string]any{
				{
					"flow": "FlowA",
				},
				{
					"flow":       "FlowB",
					"depends_on": "FlowA",
				},
				{
					"flow": "FlowC",
					"depends_on": []any{
						"RequestA",
						"FlowB",
					},
				},
			},
			want: []RunEntry{
				{Flow: "FlowA", DependsOn: nil},
				{Flow: "FlowB", DependsOn: []string{"FlowA"}},
				{Flow: "FlowC", DependsOn: []string{"RequestA", "FlowB"}},
			},
			wantErr: false,
		},
		{
			name: "Invalid entry - missing flow field",
			runArray: []map[string]any{
				{
					"depends_on": "FlowA",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Invalid depends_on type",
			runArray: []map[string]any{
				{
					"flow":       "FlowB",
					"depends_on": 123,
				},
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRunField(tt.runArray)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseRunField() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("parseRunField() = %+v, want %+v", got, tt.want)
				}
			}
		})
	}
}
