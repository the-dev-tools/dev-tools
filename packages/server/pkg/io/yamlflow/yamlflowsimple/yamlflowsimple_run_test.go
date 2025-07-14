package yamlflowsimple

import (
	"testing"
)

func TestParseRunField(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
		check   func(t *testing.T, data *YamlFlowData)
	}{
		{
			name: "Simple run field with single flow",
			yaml: `
workspace_name: Test Workspace
run:
  - flow: FlowA
flows:
  - name: FlowA
    steps:
      - request:
          name: Request1
          url: http://example.com
`,
			wantErr: false,
			check: func(t *testing.T, data *YamlFlowData) {
				if data.Flow.Name != "FlowA" {
					t.Errorf("Expected flow name 'FlowA', got '%s'", data.Flow.Name)
				}
			},
		},
		{
			name: "Run field with flow dependencies",
			yaml: `
workspace_name: Test Workspace
run:
  - flow: FlowA
  - flow: FlowB
    depends_on: FlowA
  - flow: FlowC
    depends_on:
      - RequestA
      - FlowB
flows:
  - name: FlowA
    steps:
      - request:
          name: RequestA
          url: http://example.com/a
  - name: FlowB
    steps:
      - request:
          name: RequestB
          url: http://example.com/b
  - name: FlowC
    steps:
      - request:
          name: RequestC
          url: http://example.com/c
`,
			wantErr: false,
			check: func(t *testing.T, data *YamlFlowData) {
				// Should process the first flow in run list
				if data.Flow.Name != "FlowA" {
					t.Errorf("Expected flow name 'FlowA', got '%s'", data.Flow.Name)
				}
			},
		},
		{
			name: "Run field with single string dependency",
			yaml: `
workspace_name: Test Workspace
run:
  - flow: FlowA
  - flow: FlowB
    depends_on: FlowA
flows:
  - name: FlowA
    steps:
      - request:
          name: RequestA
          url: http://example.com/a
  - name: FlowB
    steps:
      - request:
          name: RequestB
          url: http://example.com/b
`,
			wantErr: false,
			check: func(t *testing.T, data *YamlFlowData) {
				if data.Flow.Name != "FlowA" {
					t.Errorf("Expected flow name 'FlowA', got '%s'", data.Flow.Name)
				}
			},
		},
		{
			name: "Run field with non-existent flow",
			yaml: `
workspace_name: Test Workspace
run:
  - flow: NonExistentFlow
flows:
  - name: FlowA
    steps:
      - request:
          name: RequestA
          url: http://example.com
`,
			wantErr: true,
		},
		{
			name: "No run field - backward compatibility",
			yaml: `
workspace_name: Test Workspace
flows:
  - name: FlowA
    steps:
      - request:
          name: RequestA
          url: http://example.com
`,
			wantErr: false,
			check: func(t *testing.T, data *YamlFlowData) {
				// Should default to first flow
				if data.Flow.Name != "FlowA" {
					t.Errorf("Expected flow name 'FlowA', got '%s'", data.Flow.Name)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := Parse([]byte(tt.yaml))
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.check != nil {
				tt.check(t, data)
			}
		})
	}
}

func TestParseRunFieldStructure(t *testing.T) {
	runArray := []map[string]any{
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
	}

	entries, err := parseRunField(runArray)
	if err != nil {
		t.Fatalf("parseRunField() error = %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("Expected 3 entries, got %d", len(entries))
	}

	// Check first entry
	if entries[0].Flow != "FlowA" || len(entries[0].DependsOn) != 0 {
		t.Errorf("Entry 0: expected FlowA with no dependencies, got %+v", entries[0])
	}

	// Check second entry
	if entries[1].Flow != "FlowB" || len(entries[1].DependsOn) != 1 || entries[1].DependsOn[0] != "FlowA" {
		t.Errorf("Entry 1: expected FlowB depending on FlowA, got %+v", entries[1])
	}

	// Check third entry
	if entries[2].Flow != "FlowC" || len(entries[2].DependsOn) != 2 {
		t.Errorf("Entry 2: expected FlowC with 2 dependencies, got %+v", entries[2])
	}
	if entries[2].DependsOn[0] != "RequestA" || entries[2].DependsOn[1] != "FlowB" {
		t.Errorf("Entry 2: expected dependencies [RequestA, FlowB], got %v", entries[2].DependsOn)
	}
}
