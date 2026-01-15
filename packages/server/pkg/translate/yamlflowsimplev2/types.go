//nolint:revive // exported
package yamlflowsimplev2

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/compress"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

// YamlFlowFormatV2 represents the modern YAML structure for simplified workflows
type YamlFlowFormatV2 struct {
	WorkspaceName     string                      `yaml:"workspace_name"`
	ActiveEnvironment string                      `yaml:"active_environment,omitempty"`
	GlobalEnvironment string                      `yaml:"global_environment,omitempty"`
	Run               []YamlRunEntryV2            `yaml:"run,omitempty"`
	RequestTemplates  map[string]YamlRequestDefV2 `yaml:"request_templates,omitempty"`
	Requests          []YamlRequestDefV2          `yaml:"requests,omitempty"`
	Flows             []YamlFlowFlowV2            `yaml:"flows"`
	Environments      []YamlEnvironmentV2         `yaml:"environments,omitempty"`
}

// YamlRunEntryV2 represents an entry in the run list
type YamlRunEntryV2 struct {
	Flow      string        `yaml:"flow"`
	DependsOn StringOrSlice `yaml:"depends_on,omitempty"`
}

// YamlRequestDefV2 represents a request definition (template or standalone)
type YamlRequestDefV2 struct {
	Name        string            `yaml:"name,omitempty"`
	Method      string            `yaml:"method,omitempty"`
	URL         string            `yaml:"url,omitempty"`
	Headers     HeaderMapOrSlice  `yaml:"headers,omitempty"`
	QueryParams HeaderMapOrSlice  `yaml:"query_params,omitempty"`
	Body        *YamlBodyUnion    `yaml:"body,omitempty"`
	Assertions  AssertionsOrSlice `yaml:"assertions,omitempty"`
	Description string            `yaml:"description,omitempty"`
}

// YamlFlowFlowV2 represents a flow in the modern YAML format
type YamlFlowFlowV2 struct {
	Name      string                 `yaml:"name"`
	Variables []YamlFlowVariableV2   `yaml:"variables,omitempty"`
	Steps     []YamlStepWrapper      `yaml:"steps,omitempty"`
	Timeout   *int                   `yaml:"timeout,omitempty"`  // Flow timeout in seconds
	Metadata  map[string]interface{} `yaml:"metadata,omitempty"` // Additional flow metadata
}

// YamlStepWrapper handles the polymorphic step list
// A step is a map with a single key that identifies the type
type YamlStepWrapper struct {
	Request     *YamlStepRequest `yaml:"request,omitempty"`
	If          *YamlStepIf      `yaml:"if,omitempty"`
	For         *YamlStepFor     `yaml:"for,omitempty"`
	ForEach     *YamlStepForEach `yaml:"for_each,omitempty"`
	JS          *YamlStepJS      `yaml:"js,omitempty"`
	AI          *YamlStepAI      `yaml:"ai,omitempty"`
	ManualStart *YamlStepCommon  `yaml:"manual_start,omitempty"`
}

// Common fields for all step types
type YamlStepCommon struct {
	Name      string        `yaml:"name"`
	DependsOn StringOrSlice `yaml:"depends_on,omitempty"`
}

type YamlStepRequest struct {
	YamlStepCommon `yaml:",inline"`
	UseRequest     string            `yaml:"use_request,omitempty"`
	Method         string            `yaml:"method,omitempty"`
	URL            string            `yaml:"url,omitempty"`
	Headers        HeaderMapOrSlice  `yaml:"headers,omitempty"`
	QueryParams    HeaderMapOrSlice  `yaml:"query_params,omitempty"`
	Body           *YamlBodyUnion    `yaml:"body,omitempty"`
	Assertions     AssertionsOrSlice `yaml:"assertions,omitempty"`
}

type YamlStepIf struct {
	YamlStepCommon `yaml:",inline"`
	Condition      string `yaml:"condition"`
	Then           string `yaml:"then,omitempty"`
	Else           string `yaml:"else,omitempty"`
}

type YamlStepFor struct {
	YamlStepCommon `yaml:",inline"`
	IterCount      string `yaml:"iter_count"` // Expression or number
	Loop           string `yaml:"loop,omitempty"`
}

type YamlStepForEach struct {
	YamlStepCommon `yaml:",inline"`
	Items          string `yaml:"items"` // Expression
	Loop           string `yaml:"loop,omitempty"`
}

type YamlStepJS struct {
	YamlStepCommon `yaml:",inline"`
	Code           string `yaml:"code"`
}

type YamlStepAI struct {
	YamlStepCommon `yaml:",inline"`
	Prompt         string `yaml:"prompt"`                   // The prompt template
	MaxIterations  int    `yaml:"max_iterations,omitempty"` // Max agent iterations (default 5)
}

// YamlFlowVariableV2 represents a flow variable
type YamlFlowVariableV2 struct {
	Name        string `yaml:"name"`
	Value       string `yaml:"value"`
	Description string `yaml:"description,omitempty"`
	Secret      bool   `yaml:"secret,omitempty"` // Whether the variable contains sensitive data
}

// YamlEnvironmentV2 represents an environment
type YamlEnvironmentV2 struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description,omitempty"`
	Variables   map[string]string `yaml:"variables"`
}

// --- Custom Marshaler/Unmarshaler Types ---

// StringOrSlice handles either a single string or a list of strings
type StringOrSlice []string

func (s *StringOrSlice) UnmarshalYAML(value *yaml.Node) error {
	var single string
	if err := value.Decode(&single); err == nil {
		*s = []string{single}
		return nil
	}
	var list []string
	if err := value.Decode(&list); err == nil {
		*s = list
		return nil
	}
	return fmt.Errorf("expected string or list of strings")
}

func (s StringOrSlice) MarshalYAML() (interface{}, error) {
	if len(s) == 1 {
		return s[0], nil
	}
	return []string(s), nil
}

// HeaderMapOrSlice handles "headers" and "query_params"
type HeaderMapOrSlice []YamlNameValuePairV2

func (h *HeaderMapOrSlice) UnmarshalYAML(value *yaml.Node) error {
	// Try slice of objects
	var list []YamlNameValuePairV2
	if err := value.Decode(&list); err == nil {
		*h = list
		return nil
	}
	// Try map
	var m map[string]string
	if err := value.Decode(&m); err == nil {
		var res []YamlNameValuePairV2
		for k, v := range m {
			res = append(res, YamlNameValuePairV2{
				Name:    k,
				Value:   v,
				Enabled: true,
			})
		}
		*h = res
		return nil
	}
	return fmt.Errorf("expected map or list of objects for headers/params")
}

func (h HeaderMapOrSlice) MarshalYAML() (interface{}, error) {
	// Simplify to map if possible (all enabled, no descriptions)
	canSimplify := true
	m := make(map[string]string)
	for _, item := range h {
		if !item.Enabled || item.Description != "" {
			canSimplify = false
			break
		}
		m[item.Name] = item.Value
	}
	if canSimplify && len(h) > 0 {
		return m, nil
	}
	if len(h) == 0 {
		return nil, nil // Omit if empty
	}
	return []YamlNameValuePairV2(h), nil
}

// YamlBodyUnion handles flexible body parsing
type YamlBodyUnion struct {
	Type        string                 `yaml:"type"`
	Raw         string                 `yaml:"raw,omitempty"`
	JSON        map[string]interface{} `yaml:"json,omitempty"`
	Form        HeaderMapOrSlice       `yaml:"form_data,omitempty"`
	UrlEncoded  HeaderMapOrSlice       `yaml:"urlencoded,omitempty"`
	Compression string                 `yaml:"compression,omitempty"`
}

func (b *YamlBodyUnion) UnmarshalYAML(value *yaml.Node) error {
	// 1. Check if simple string (raw)
	var raw string
	if err := value.Decode(&raw); err == nil {
		b.Type = BodyTypeRaw
		b.Raw = raw
		return nil
	}

	// 2. Try to decode as map first to catch flat fields
	var m map[string]interface{}
	if err := value.Decode(&m); err == nil {
		// Check if it's a structured body definition (has 'type')
		if _, ok := m["type"].(string); ok {
			// Structured - decode as struct
			type alias YamlBodyUnion
			var obj alias
			if err := value.Decode(&obj); err == nil {
				*b = YamlBodyUnion(obj)
				return nil
			}
		}

		// Not structured or 'type' missing - treat entire map as JSON body
		b.Type = BodyTypeJSON
		b.JSON = m
		return nil
	}

	return fmt.Errorf("invalid body format")
}

func (b YamlBodyUnion) MarshalYAML() (interface{}, error) {
	if b.Type == BodyTypeRaw && b.JSON == nil && len(b.Form) == 0 && len(b.UrlEncoded) == 0 {
		return b.Raw, nil
	}
	type alias YamlBodyUnion
	return alias(b), nil
}

// AssertionsOrSlice handles assertions
type AssertionsOrSlice []YamlAssertionV2

func (a *AssertionsOrSlice) UnmarshalYAML(value *yaml.Node) error {
	var listStr []string
	if err := value.Decode(&listStr); err == nil {
		var res []YamlAssertionV2
		for _, s := range listStr {
			res = append(res, YamlAssertionV2{Expression: s, Enabled: true})
		}
		*a = res
		return nil
	}
	var listObj []YamlAssertionV2
	if err := value.Decode(&listObj); err == nil {
		*a = listObj
		return nil
	}
	return fmt.Errorf("invalid assertions format")
}

func (a AssertionsOrSlice) MarshalYAML() (interface{}, error) {
	canSimplify := true
	var simple []string
	for _, item := range a {
		if !item.Enabled {
			canSimplify = false
			break
		}
		simple = append(simple, item.Expression)
	}
	if canSimplify && len(a) > 0 {
		return simple, nil
	}
	if len(a) == 0 {
		return nil, nil
	}
	return []YamlAssertionV2(a), nil
}

type YamlNameValuePairV2 struct {
	Name        string `yaml:"name"`
	Value       string `yaml:"value"`
	Description string `yaml:"description,omitempty"`
	Enabled     bool   `yaml:"enabled"`
}

func (p *YamlNameValuePairV2) UnmarshalYAML(value *yaml.Node) error {
	type alias YamlNameValuePairV2
	aux := &alias{Enabled: true}
	if err := value.Decode(aux); err != nil {
		return err
	}
	*p = YamlNameValuePairV2(*aux)
	return nil
}

type YamlAssertionV2 struct {
	Expression string `yaml:"expression"`
	Enabled    bool   `yaml:"enabled"`
}

func (p *YamlAssertionV2) UnmarshalYAML(value *yaml.Node) error {
	type alias YamlAssertionV2
	aux := &alias{Enabled: true}
	if err := value.Decode(aux); err != nil {
		return err
	}
	*p = YamlAssertionV2(*aux)
	return nil
}

// ConvertOptionsV2 contains options for modern YAML conversion
type ConvertOptionsV2 struct {
	WorkspaceID    idwrap.IDWrap
	FolderID       *idwrap.IDWrap
	ParentHttpID   *idwrap.IDWrap
	IsDelta        bool
	DeltaName      *string
	CollectionName string

	EnableCompression bool
	CompressionType   compress.CompressType
	GenerateFiles     bool
	FileOrder         int

	// CredentialMap maps credential names to their IDs for AI node resolution.
	// If nil, credential_id in YAML must be a valid ID string.
	CredentialMap map[string]idwrap.IDWrap
}

// YamlFlowDataV2 contains the intermediate data structure during YAML parsing
type YamlFlowDataV2 struct {
	Flow      mflow.Flow
	Nodes     []mflow.Node
	Edges     []mflow.Edge
	Variables []YamlVariableV2

	// HTTP request data
	HTTPRequests []YamlHTTPRequestV2

	// Flow node implementations
	RequestNodes   []mflow.NodeRequest
	ConditionNodes []mflow.NodeIf
	ForNodes       []mflow.NodeFor
	ForEachNodes   []mflow.NodeForEach
	JSNodes        []mflow.NodeJS
	AINodes        []mflow.NodeAI
}

// YamlVariableV2 represents a variable during parsing
type YamlVariableV2 struct {
	VarKey string
	Value  string
}

// YamlHTTPRequestV2 represents a simplified HTTP request during parsing
// NOTE: We keep this for internal use in converter/exporter logic,
// but it essentially mirrors YamlRequestDefV2 now.
type YamlHTTPRequestV2 struct {
	Name        string
	Method      string
	URL         string
	Headers     []YamlNameValuePairV2
	QueryParams []YamlNameValuePairV2
	Body        *YamlBodyUnion
	Assertions  []YamlAssertionV2
	Description string
}

// Error types for better error handling
type YamlFlowErrorV2 struct {
	Message string
	Field   string
	Value   interface{}
	Line    int // Optional line number for debugging
}

func (e YamlFlowErrorV2) Error() string {
	if e.Field != "" {
		if e.Line > 0 {
			return fmt.Sprintf("line %d: %s: field '%s' with value '%v'", e.Line, e.Message, e.Field, e.Value)
		}
		return fmt.Sprintf("%s: field '%s' with value '%v'", e.Message, e.Field, e.Value)
	}
	if e.Line > 0 {
		return fmt.Sprintf("line %d: %s", e.Line, e.Message)
	}
	return e.Message
}

func NewYamlFlowErrorV2(message, field string, value interface{}) error {
	return YamlFlowErrorV2{
		Message: message,
		Field:   field,
		Value:   value,
	}
}

func NewYamlFlowErrorWithLineV2(message, field string, value interface{}, line int) error {
	return YamlFlowErrorV2{
		Message: message,
		Field:   field,
		Value:   value,
		Line:    line,
	}
}

// Validation functions

func (opts ConvertOptionsV2) Validate() error {
	if opts.WorkspaceID.Compare(idwrap.IDWrap{}) == 0 {
		return NewYamlFlowErrorV2("workspace ID is required", "workspace_id", opts.WorkspaceID)
	}

	if opts.IsDelta && opts.DeltaName == nil {
		return NewYamlFlowErrorV2("delta name is required when IsDelta is true", "delta_name", nil)
	}

	if opts.CompressionType != compress.CompressTypeNone &&
		opts.CompressionType != compress.CompressTypeGzip {
		return NewYamlFlowErrorV2("invalid compression type", "compression_type", opts.CompressionType)
	}

	return nil
}

func (yf YamlFlowFormatV2) Validate() error {
	if yf.WorkspaceName == "" {
		return NewYamlFlowErrorV2("workspace_name is required", "workspace_name", nil)
	}

	if len(yf.Flows) == 0 {
		return NewYamlFlowErrorV2("at least one flow is required", "flows", nil)
	}

	for i, flow := range yf.Flows {
		if flow.Name == "" {
			return NewYamlFlowErrorWithLineV2("flow name is required", "name", nil, i)
		}

		for j := i + 1; j < len(yf.Flows); j++ {
			if yf.Flows[j].Name == flow.Name {
				return NewYamlFlowErrorWithLineV2("duplicate flow name", "name", flow.Name, i)
			}
		}
	}

	return nil
}

func (yf YamlFlowFlowV2) Validate() error {
	if yf.Name == "" {
		return NewYamlFlowErrorV2("flow name is required", "name", nil)
	}

	varNames := make(map[string]bool)
	for i, variable := range yf.Variables {
		if variable.Name == "" {
			return NewYamlFlowErrorV2("variable name is required", "variables["+string(rune(i))+"].name", nil)
		}

		if varNames[variable.Name] {
			return NewYamlFlowErrorV2("duplicate variable name", "variables["+string(rune(i))+"].name", variable.Name)
		}
		varNames[variable.Name] = true
	}

	return nil
}

func GetDefaultOptions(workspaceID idwrap.IDWrap) ConvertOptionsV2 {
	return ConvertOptionsV2{
		WorkspaceID:       workspaceID,
		FolderID:          nil,
		ParentHttpID:      nil,
		IsDelta:           false,
		DeltaName:         nil,
		CollectionName:    "Imported Collection",
		EnableCompression: true,
		CompressionType:   compress.CompressTypeGzip,
		GenerateFiles:     true,
		FileOrder:         0,
	}
}
