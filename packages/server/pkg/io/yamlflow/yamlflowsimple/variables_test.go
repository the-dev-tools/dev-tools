package yamlflowsimple

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mexamplequery"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mnnode/mnif"
	"the-dev-tools/server/pkg/model/mnnode/mnjs"
	"the-dev-tools/server/pkg/model/mvar"
)

func TestExtractVariableReferences(t *testing.T) {
	tests := []struct {
		name         string
		workflowData *YamlFlowData
		expected     map[string]*VariableInfo
	}{
		{
			name: "Extract flow variables from YAML definition",
			workflowData: &YamlFlowData{
				Variables: []mvar.Var{
					{VarKey: "api_key", Value: "test123"},
					{VarKey: "base_url", Value: "https://api.example.com"},
				},
			},
			expected: map[string]*VariableInfo{
				"api_key": {
					Name:     "api_key",
					IsEnvVar: false,
					Value:    "test123",
					HasValue: true,
				},
				"base_url": {
					Name:     "base_url",
					IsEnvVar: false,
					Value:    "https://api.example.com",
					HasValue: true,
				},
			},
		},
		{
			name: "Extract environment variables from headers",
			workflowData: &YamlFlowData{
				Headers: []mexampleheader.Header{
					{HeaderKey: "Authorization", Value: "Bearer {{ env.API_TOKEN }}"},
					{HeaderKey: "X-Custom", Value: "{{ env.CUSTOM_VALUE }}"},
				},
			},
			expected: map[string]*VariableInfo{
				"API_TOKEN": {
					Name:     "API_TOKEN",
					IsEnvVar: true,
					HasValue: false,
				},
				"CUSTOM_VALUE": {
					Name:     "CUSTOM_VALUE",
					IsEnvVar: true,
					HasValue: false,
				},
			},
		},
		{
			name: "Extract mixed variables from different sources",
			workflowData: &YamlFlowData{
				Variables: []mvar.Var{
					{VarKey: "flow_var", Value: "flow_value"},
				},
				Headers: []mexampleheader.Header{
					{HeaderKey: "X-Flow", Value: "{{ flow_var }}"},
					{HeaderKey: "X-Env", Value: "{{ env.ENV_VAR }}"},
				},
				Queries: []mexamplequery.Query{
					{QueryKey: "param", Value: "{{ another_var }}"},
					{QueryKey: "env_param", Value: "{{ env.QUERY_VAR }}"},
				},
				RawBodies: []mbodyraw.ExampleBodyRaw{
					{Data: []byte(`{"key": "{{ body_var }}", "env": "{{ env.BODY_ENV }}"}`)},
				},
			},
			expected: map[string]*VariableInfo{
				"flow_var": {
					Name:     "flow_var",
					IsEnvVar: false,
					Value:    "flow_value",
					HasValue: true,
				},
				"ENV_VAR": {
					Name:     "ENV_VAR",
					IsEnvVar: true,
					HasValue: false,
				},
				"another_var": {
					Name:     "another_var",
					IsEnvVar: false,
					HasValue: false,
				},
				"QUERY_VAR": {
					Name:     "QUERY_VAR",
					IsEnvVar: true,
					HasValue: false,
				},
				"body_var": {
					Name:     "body_var",
					IsEnvVar: false,
					HasValue: false,
				},
				"BODY_ENV": {
					Name:     "BODY_ENV",
					IsEnvVar: true,
					HasValue: false,
				},
			},
		},
		{
			name: "Extract from endpoints and examples",
			workflowData: &YamlFlowData{
				Endpoints: []mitemapi.ItemApi{
					{Url: "/api/{{ version }}/users", Name: "{{ env.API_BASE }}"},
				},
				Examples: []mitemapiexample.ItemApiExample{
					{Name: "Test {{ test_name }}"},
				},
			},
			expected: map[string]*VariableInfo{
				"version": {
					Name:     "version",
					IsEnvVar: false,
					HasValue: false,
				},
				"API_BASE": {
					Name:     "API_BASE",
					IsEnvVar: true,
					HasValue: false,
				},
				"test_name": {
					Name:     "test_name",
					IsEnvVar: false,
					HasValue: false,
				},
			},
		},
		{
			name: "Extract from JS and condition nodes",
			workflowData: &YamlFlowData{
				JSNodes: []mnjs.MNJS{
					{Code: []byte(`console.log("{{ env.LOG_LEVEL }}"); var x = "{{ js_var }}";`)},
				},
				ConditionNodes: []mnif.MNIF{
					{
						Condition: mcondition.Condition{
							Comparisons: mcondition.Comparison{
								Expression: "{{ env.CONDITION_VAR }}",
							},
						},
					},
				},
			},
			expected: map[string]*VariableInfo{
				"LOG_LEVEL": {
					Name:     "LOG_LEVEL",
					IsEnvVar: true,
					HasValue: false,
				},
				"js_var": {
					Name:     "js_var",
					IsEnvVar: false,
					HasValue: false,
				},
				"CONDITION_VAR": {
					Name:     "CONDITION_VAR",
					IsEnvVar: true,
					HasValue: false,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractVariableReferences(tt.workflowData)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSeparateVariablesByType(t *testing.T) {
	variables := map[string]*VariableInfo{
		"flow_var1": {
			Name:     "flow_var1",
			IsEnvVar: false,
			Value:    "value1",
			HasValue: true,
		},
		"flow_var2": {
			Name:     "flow_var2",
			IsEnvVar: false,
			Value:    "value2",
			HasValue: true,
		},
		"env_var1": {
			Name:     "env_var1",
			IsEnvVar: true,
			HasValue: false,
		},
		"env_var2": {
			Name:     "env_var2",
			IsEnvVar: true,
			Value:    "env_value",
			HasValue: true,
		},
		"undefined_flow_var": {
			Name:     "undefined_flow_var",
			IsEnvVar: false,
			HasValue: false,
		},
	}

	flowVars, envVars := SeparateVariablesByType(variables)

	// Check flow variables (only those with values)
	assert.Len(t, flowVars, 2)
	flowVarMap := make(map[string]string)
	for _, v := range flowVars {
		flowVarMap[v.VarKey] = v.Value
	}
	assert.Equal(t, "value1", flowVarMap["flow_var1"])
	assert.Equal(t, "value2", flowVarMap["flow_var2"])

	// Check environment variables
	assert.Len(t, envVars, 2)
	envVarMap := make(map[string]string)
	for _, v := range envVars {
		envVarMap[v.VarKey] = v.Value
	}
	assert.Contains(t, envVarMap, "env_var1")
	assert.Equal(t, "env_value", envVarMap["env_var2"])
}

func TestCheckStringHasEnvVar(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"{{ env.API_KEY }}", true},
		{"Bearer {{ env.TOKEN }}", true},
		{"{{ flow_var }}", false},
		{"No variables here", false},
		{"Multiple {{ env.VAR1 }} and {{ env.VAR2 }}", true},
		{"Mixed {{ flow }} and {{ env.ENV }}", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := CheckStringHasEnvVar(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestImportYamlFlowYAMLWithVariables(t *testing.T) {
	yamlData := []byte(`
workspace_name: Test Workspace
flows:
  - name: Test Flow
    variables:
      - name: api_key
        value: "test-key-123"
      - name: base_url
        value: "https://api.test.com"
    steps:
      - request:
          name: Test Request
          method: GET
          url: "{{ base_url }}/users"
          headers:
            Authorization: "Bearer {{ api_key }}"
            X-Env-Header: "{{ env.ENV_TOKEN }}"
          query_params:
            env_param: "{{ env.QUERY_PARAM }}"
`)

	result, err := ImportYamlFlowYAML(yamlData)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Check that a default environment was created
	assert.Len(t, result.Environments, 1)
	assert.Equal(t, "Default Environment", result.Environments[0].Name)
	assert.Equal(t, menv.EnvNormal, result.Environments[0].Type)

	// Check environment variables were created
	assert.Len(t, result.Variables, 2) // ENV_TOKEN and QUERY_PARAM
	envVarMap := make(map[string]bool)
	for _, v := range result.Variables {
		envVarMap[v.VarKey] = true
		assert.Equal(t, result.Environments[0].ID, v.EnvID)
		assert.True(t, v.Enabled)
	}
	assert.True(t, envVarMap["ENV_TOKEN"])
	assert.True(t, envVarMap["QUERY_PARAM"])

	// Check flow variables were created
	assert.Len(t, result.FlowVariables, 2) // api_key and base_url
	flowVarMap := make(map[string]string)
	for _, v := range result.FlowVariables {
		flowVarMap[v.Name] = v.Value
		assert.Equal(t, result.Flows[0].ID, v.FlowID)
		assert.True(t, v.Enabled)
	}
	assert.Equal(t, "test-key-123", flowVarMap["api_key"])
	assert.Equal(t, "https://api.test.com", flowVarMap["base_url"])
}
