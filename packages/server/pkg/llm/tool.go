package llm

// Tool represents a tool that can be used by an LLM.
type Tool struct {
	Type     string
	Function *FunctionDef
}

// FunctionDef represents a function definition for a tool.
type FunctionDef struct {
	Name        string
	Description string
	Parameters  map[string]any
}
