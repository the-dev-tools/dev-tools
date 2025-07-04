package simplified

import "encoding/json"

// SimplifiedWorkflow represents the user-friendly format
type SimplifiedWorkflow struct {
	WorkspaceName string          `yaml:"workspace_name" json:"workspace_name" toml:"workspace_name"`
	Run           []RunStep       `yaml:"run,omitempty" json:"run,omitempty" toml:"run,omitempty"`
	Requests      []GlobalRequest `yaml:"requests,omitempty" json:"requests,omitempty" toml:"requests,omitempty"`
	Flows         []Flow          `yaml:"flows" json:"flows" toml:"flows"`
}

// RunStep represents a flow execution step in the run section
type RunStep struct {
	Flow      string   `yaml:"flow" json:"flow" toml:"flow"`
	DependsOn []string `yaml:"depends_on,omitempty" json:"depends_on,omitempty" toml:"depends_on,omitempty"`
}

// Flow represents a workflow flow
type Flow struct {
	Name      string            `yaml:"name" json:"name" toml:"name"`
	Variables map[string]string `yaml:"variables,omitempty" json:"variables,omitempty" toml:"variables,omitempty"`
	Steps     []Step            `yaml:"steps" json:"steps" toml:"steps"`
}

// GlobalRequest defines a reusable request template
type GlobalRequest struct {
	Name    string            `yaml:"name" json:"name" toml:"name"`
	Method  string            `yaml:"method" json:"method" toml:"method"`
	URL     string            `yaml:"url" json:"url" toml:"url"`
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty" toml:"headers,omitempty"`
	Body    *BodyFormat       `yaml:"body,omitempty" json:"body,omitempty" toml:"body,omitempty"`
}

// Step represents a generic workflow step
type Step struct {
	Type      StepType `yaml:"-" json:"-" toml:"-"`
	DependsOn []string `yaml:"depends_on,omitempty" json:"depends_on,omitempty" toml:"depends_on,omitempty"`

	// Request step fields
	Request *RequestStep `yaml:"request,omitempty" json:"request,omitempty" toml:"request,omitempty"`

	// If step fields
	If *IfStep `yaml:"if,omitempty" json:"if,omitempty" toml:"if,omitempty"`

	// For step fields
	For *ForStep `yaml:"for,omitempty" json:"for,omitempty" toml:"for,omitempty"`

	// ForEach step fields
	ForEach *ForEachStep `yaml:"for_each,omitempty" json:"for_each,omitempty" toml:"for_each,omitempty"`

	// JS step fields
	JS *JSStep `yaml:"js,omitempty" json:"js,omitempty" toml:"js,omitempty"`
}

// UnmarshalYAML implements custom unmarshaling to properly handle step types
func (s *Step) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Define a raw type that matches our structure
	type rawStep struct {
		DependsOn []string     `yaml:"depends_on,omitempty"`
		Request   *RequestStep `yaml:"request,omitempty"`
		If        *IfStep      `yaml:"if,omitempty"`
		For       *ForStep     `yaml:"for,omitempty"`
		ForEach   *ForEachStep `yaml:"for_each,omitempty"`
		JS        *JSStep      `yaml:"js,omitempty"`
	}

	var raw rawStep
	if err := unmarshal(&raw); err != nil {
		return err
	}

	// Set the fields and determine the type
	s.DependsOn = raw.DependsOn

	if raw.Request != nil {
		s.Type = StepTypeRequest
		s.Request = raw.Request
	} else if raw.If != nil {
		s.Type = StepTypeIf
		s.If = raw.If
	} else if raw.For != nil {
		s.Type = StepTypeFor
		s.For = raw.For
	} else if raw.ForEach != nil {
		s.Type = StepTypeForEach
		s.ForEach = raw.ForEach
	} else if raw.JS != nil {
		s.Type = StepTypeJS
		s.JS = raw.JS
	}

	return nil
}

// StepType represents the type of workflow step
type StepType int

const (
	StepTypeUnspecified StepType = iota
	StepTypeRequest
	StepTypeIf
	StepTypeFor
	StepTypeForEach
	StepTypeJS
)

// RequestStep defines HTTP request configuration
type RequestStep struct {
	Name       string            `yaml:"name" json:"name" toml:"name"`
	UseRequest string            `yaml:"use_request,omitempty" json:"use_request,omitempty" toml:"use_request,omitempty"`
	Method     string            `yaml:"method,omitempty" json:"method,omitempty" toml:"method,omitempty"`
	URL        string            `yaml:"url,omitempty" json:"url,omitempty" toml:"url,omitempty"`
	Headers    map[string]string `yaml:"headers,omitempty" json:"headers,omitempty" toml:"headers,omitempty"`
	Body       *BodyFormat       `yaml:"body,omitempty" json:"body,omitempty" toml:"body,omitempty"`
}

// IfStep defines conditional branching
type IfStep struct {
	Name       string `yaml:"name" json:"name" toml:"name"`
	Expression string `yaml:"expression" json:"expression" toml:"expression"`
	Then       string `yaml:"then,omitempty" json:"then,omitempty" toml:"then,omitempty"`
	Else       string `yaml:"else,omitempty" json:"else,omitempty" toml:"else,omitempty"`
}

// ForStep defines loop iteration
type ForStep struct {
	Name      string `yaml:"name" json:"name" toml:"name"`
	IterCount int64  `yaml:"iter_count" json:"iter_count" toml:"iter_count"`
	Loop      string `yaml:"loop,omitempty" json:"loop,omitempty" toml:"loop,omitempty"`
}

// ForEachStep defines iteration over a collection
type ForEachStep struct {
	Name       string `yaml:"name" json:"name" toml:"name"`
	Collection string `yaml:"collection" json:"collection" toml:"collection"`
	Item       string `yaml:"item,omitempty" json:"item,omitempty" toml:"item,omitempty"`
	Loop       string `yaml:"loop,omitempty" json:"loop,omitempty" toml:"loop,omitempty"`
}

// JSStep defines JavaScript execution
type JSStep struct {
	Name string `yaml:"name" json:"name" toml:"name"`
	Code string `yaml:"code" json:"code" toml:"code"`
}

// BodyFormat represents the body format with kind and value
type BodyFormat struct {
	Kind  BodyKind               `yaml:"kind,omitempty" json:"kind,omitempty" toml:"kind,omitempty"`
	Value map[string]interface{} `yaml:"value,omitempty" json:"value,omitempty" toml:"value,omitempty"`
}

// BodyKind represents the type of body content
type BodyKind string

const (
	BodyKindJSON        BodyKind = "json"
	BodyKindForm        BodyKind = "form"
	BodyKindURLEncoded  BodyKind = "url"
	BodyKindRaw         BodyKind = "raw"
	BodyKindUnspecified BodyKind = ""
)

// UnmarshalYAML implements custom unmarshaling to support both simple and explicit formats
func (b *BodyFormat) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// First try to unmarshal as the explicit format
	type explicitFormat struct {
		Kind  string                 `yaml:"kind"`
		Value map[string]interface{} `yaml:"value"`
	}

	var explicit explicitFormat
	if err := unmarshal(&explicit); err == nil && explicit.Kind != "" {
		b.Kind = BodyKind(explicit.Kind)
		b.Value = explicit.Value
		return nil
	}

	// If that fails, try to unmarshal as a direct value (simple format)
	var value interface{}
	if err := unmarshal(&value); err != nil {
		return err
	}

	// Convert to map if possible
	if mapValue, ok := value.(map[string]interface{}); ok {
		b.Kind = BodyKindJSON
		b.Value = mapValue
	} else {
		// Handle other types by wrapping in a map
		b.Kind = BodyKindJSON
		b.Value = map[string]interface{}{"value": value}
	}
	return nil
}

// UnmarshalJSON implements custom unmarshaling for JSON
func (b *BodyFormat) UnmarshalJSON(data []byte) error {
	// First try to unmarshal as the explicit format
	type explicitFormat struct {
		Kind  string                 `json:"kind"`
		Value map[string]interface{} `json:"value"`
	}

	var explicit explicitFormat
	if err := json.Unmarshal(data, &explicit); err == nil && explicit.Kind != "" {
		b.Kind = BodyKind(explicit.Kind)
		b.Value = explicit.Value
		return nil
	}

	// If that fails, try to unmarshal as a direct value (simple format)
	var value map[string]interface{}
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}

	// Default to JSON kind for simple format
	b.Kind = BodyKindJSON
	b.Value = value
	return nil
}
