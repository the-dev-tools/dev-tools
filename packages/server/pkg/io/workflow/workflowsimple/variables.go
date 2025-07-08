package workflowsimple

import (
	"regexp"
	"strings"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/model/mvar"
)

// VariableInfo holds information about a variable reference found in the YAML
type VariableInfo struct {
	Name       string
	IsEnvVar   bool
	Value      string // For flow variables defined in YAML
	HasValue   bool   // Whether this variable has a defined value
}

// ExtractVariableReferences extracts all variable references from workflow data
func ExtractVariableReferences(workflowData *WorkflowData) map[string]*VariableInfo {
	variables := make(map[string]*VariableInfo)
	
	// First, add all flow variables that are defined in the YAML
	for _, v := range workflowData.Variables {
		variables[v.VarKey] = &VariableInfo{
			Name:     v.VarKey,
			IsEnvVar: false,
			Value:    v.Value,
			HasValue: true,
		}
	}
	
	// Regular expression to find variable references
	varPattern := regexp.MustCompile(`\{\{\s*([^}]+)\s*\}\}`)
	
	// Helper function to extract variables from a string
	extractFromString := func(s string) {
		matches := varPattern.FindAllStringSubmatch(s, -1)
		for _, match := range matches {
			if len(match) > 1 {
				varName := strings.TrimSpace(match[1])
				
				// Check if it's an environment variable
				isEnv := strings.HasPrefix(varName, menv.EnvVariablePrefix)
				if isEnv {
					varName = strings.TrimPrefix(varName, menv.EnvVariablePrefix)
				}
				
				// Only add if not already defined as a flow variable
				if _, exists := variables[varName]; !exists {
					variables[varName] = &VariableInfo{
						Name:     varName,
						IsEnvVar: isEnv,
						HasValue: false,
					}
				}
			}
		}
	}
	
	// Extract from headers
	for _, header := range workflowData.Headers {
		extractFromString(header.HeaderKey)
		extractFromString(header.Value)
	}
	
	// Extract from queries
	for _, query := range workflowData.Queries {
		extractFromString(query.QueryKey)
		extractFromString(query.Value)
	}
	
	// Extract from raw bodies
	for _, body := range workflowData.RawBodies {
		extractFromString(string(body.Data))
	}
	
	// Extract from endpoints
	for _, endpoint := range workflowData.Endpoints {
		extractFromString(endpoint.Url)
		extractFromString(endpoint.Name)
	}
	
	// Extract from examples
	for _, example := range workflowData.Examples {
		extractFromString(example.Name)
	}
	
	// Extract from JS nodes
	for _, jsNode := range workflowData.JSNodes {
		extractFromString(string(jsNode.Code))
	}
	
	// Extract from condition nodes
	for _, condNode := range workflowData.ConditionNodes {
		extractFromString(condNode.Condition.Comparisons.Expression)
	}
	
	return variables
}

// SeparateVariablesByType separates variables into flow and environment variables
func SeparateVariablesByType(variables map[string]*VariableInfo) (flowVars []mvar.Var, envVars []mvar.Var) {
	for _, varInfo := range variables {
		v := mvar.Var{
			VarKey: varInfo.Name,
			Value:  varInfo.Value,
		}
		
		if varInfo.IsEnvVar {
			envVars = append(envVars, v)
		} else if varInfo.HasValue {
			// Only add flow variables that have defined values
			flowVars = append(flowVars, v)
		}
	}
	
	return flowVars, envVars
}

// CheckStringHasEnvVar checks if a string contains environment variable references
func CheckStringHasEnvVar(s string) bool {
	return strings.Contains(s, "{{") && strings.Contains(s, menv.EnvVariablePrefix)
}