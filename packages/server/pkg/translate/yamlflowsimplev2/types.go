package yamlflowsimplev2

import (
	"fmt"

	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mflowvariable"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"the-dev-tools/server/pkg/model/mnnode/mnforeach"
	"the-dev-tools/server/pkg/model/mnnode/mnif"
	"the-dev-tools/server/pkg/model/mnnode/mnjs"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
)

// YamlFlowFormatV2 represents the modern YAML structure for simplified workflows
type YamlFlowFormatV2 struct {
	WorkspaceName     string                    `yaml:"workspace_name"`
	ActiveEnvironment string                    `yaml:"active_environment,omitempty"`
	GlobalEnvironment string                    `yaml:"global_environment,omitempty"`
	Run               []map[string]any          `yaml:"run,omitempty"`
	RequestTemplates  map[string]map[string]any `yaml:"request_templates,omitempty"`
	Requests          []map[string]any          `yaml:"requests,omitempty"`
	Flows             []YamlFlowFlowV2         `yaml:"flows"`
	Environments      []YamlEnvironmentV2      `yaml:"environments,omitempty"`
}

// YamlFlowFlowV2 represents a flow in the modern YAML format
type YamlFlowFlowV2 struct {
	Name      string                 `yaml:"name"`
	Variables []YamlFlowVariableV2   `yaml:"variables,omitempty"`
	Steps     []map[string]any       `yaml:"steps,omitempty"`
	Timeout   *int                   `yaml:"timeout,omitempty"`   // Flow timeout in seconds
	Metadata  map[string]interface{} `yaml:"metadata,omitempty"` // Additional flow metadata
}

// YamlFlowVariableV2 represents a flow variable in the modern YAML format
type YamlFlowVariableV2 struct {
	Name        string `yaml:"name"`
	Value       string `yaml:"value"`
	Description string `yaml:"description,omitempty"`
	Secret      bool   `yaml:"secret,omitempty"` // Whether the variable contains sensitive data
}

// YamlEnvironmentV2 represents an environment in the modern YAML format
type YamlEnvironmentV2 struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description,omitempty"`
	Variables   map[string]string `yaml:"variables"`
}

// SimplifiedYAMLResolvedV2 contains all entities parsed from simplified YAML using modern models
type SimplifiedYAMLResolvedV2 struct {
	// Modern HTTP models (direct to workspace)
	HTTPRequests []mhttp.HTTP

	// Associated HTTP data structures
	SearchParams   []mhttp.HTTPSearchParam
	Headers        []mhttp.HTTPHeader
	BodyForms      []mhttp.HTTPBodyForm
	BodyUrlencoded []mhttp.HTTPBodyUrlencoded
	BodyRaw        []*mhttp.HTTPBodyRaw

	// File organization for workspace
	Files []mfile.File

	// Flow structures (unchanged but with direct workspace integration)
	Flows              []mflow.Flow
	FlowNodes          []mnnode.MNode
	FlowEdges          []edge.Edge
	FlowVariables      []mflowvariable.FlowVariable
	FlowRequestNodes   []mnrequest.MNRequest
	FlowConditionNodes []mnif.MNIF
	FlowNoopNodes      []mnnoop.NoopNode
	FlowForNodes       []mnfor.MNFor
	FlowForEachNodes   []mnforeach.MNForEach
	FlowJSNodes        []mnjs.MNJS
}

// ConvertOptionsV2 contains options for modern YAML conversion
type ConvertOptionsV2 struct {
	WorkspaceID    idwrap.IDWrap
	FolderID       *idwrap.IDWrap
	ParentHttpID   *idwrap.IDWrap
	IsDelta        bool
	DeltaName      *string
	CollectionName string

	// Additional modern options
	EnableCompression bool                 // Whether to compress large bodies
	CompressionType   compress.CompressType // Compression type to use
	GenerateFiles     bool          // Whether to generate file records
	FileOrder         int           // Starting order for files
}

// YamlFlowDataV2 contains the intermediate data structure during YAML parsing
type YamlFlowDataV2 struct {
	Flow     mflow.Flow
	Nodes    []mnnode.MNode
	Edges    []edge.Edge
	Variables []YamlVariableV2

	// HTTP request data
	HTTPRequests []YamlHTTPRequestV2

	// Flow node implementations
	NoopNodes      []mnnoop.NoopNode
	RequestNodes   []mnrequest.MNRequest
	ConditionNodes []mnif.MNIF
	ForNodes       []mnfor.MNFor
	ForEachNodes   []mnforeach.MNForEach
	JSNodes        []mnjs.MNJS
}

// YamlVariableV2 represents a variable during parsing
type YamlVariableV2 struct {
	VarKey string
	Value  string
}

// YamlHTTPRequestV2 represents a simplified HTTP request during parsing
type YamlHTTPRequestV2 struct {
	Name        string
	Method      string
	URL         string
	Headers     []YamlNameValuePairV2
	QueryParams []YamlNameValuePairV2
	Body        *YamlBodyV2
	Assertions  []YamlAssertionV2
	Description string
}

// YamlNameValuePairV2 represents a name-value pair for headers, queries, etc.
type YamlNameValuePairV2 struct {
	Name        string
	Value       string
	Description string
	Enabled     bool
}

// YamlBodyV2 represents request body data in various formats
type YamlBodyV2 struct {
	Type    string                 // "raw", "form-data", "urlencoded", "json"
	Raw     string                 // Raw body content
	JSON    map[string]interface{} // JSON body as map
	Form    []YamlNameValuePairV2  // Form data
	UrlEncoded []YamlNameValuePairV2 // URL encoded data
}

// YamlAssertionV2 represents an assertion for HTTP requests
type YamlAssertionV2 struct {
	Expression string
	Enabled    bool
}

// HTTPMapping tracks the relationship between legacy example IDs and modern HTTP IDs
type HTTPMapping struct {
	LegacyExampleID idwrap.IDWrap
	ModernHTTPID    idwrap.IDWrap
}

// ParseResult contains the result of YAML parsing with mappings for conversion
type ParseResult struct {
	Data    *YamlFlowDataV2
	Mappings []HTTPMapping
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

// NewYamlFlowErrorV2 creates a new YAML flow error
func NewYamlFlowErrorV2(message, field string, value interface{}) error {
	return YamlFlowErrorV2{
		Message: message,
		Field:   field,
		Value:   value,
	}
}

// NewYamlFlowErrorWithLineV2 creates a new YAML flow error with line number
func NewYamlFlowErrorWithLineV2(message, field string, value interface{}, line int) error {
	return YamlFlowErrorV2{
		Message: message,
		Field:   field,
		Value:   value,
		Line:    line,
	}
}

// Validation functions

// Validate validates the ConvertOptionsV2
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

// Validate validates the YamlFlowFormatV2
func (yf YamlFlowFormatV2) Validate() error {
	if yf.WorkspaceName == "" {
		return NewYamlFlowErrorV2("workspace_name is required", "workspace_name", nil)
	}

	if len(yf.Flows) == 0 {
		return NewYamlFlowErrorV2("at least one flow is required", "flows", nil)
	}

	// Validate each flow
	for i, flow := range yf.Flows {
		if flow.Name == "" {
			return NewYamlFlowErrorWithLineV2("flow name is required", "name", nil, i)
		}

		// Check for duplicate flow names
		for j := i + 1; j < len(yf.Flows); j++ {
			if yf.Flows[j].Name == flow.Name {
				return NewYamlFlowErrorWithLineV2("duplicate flow name", "name", flow.Name, i)
			}
		}
	}

	return nil
}

// Validate validates a YamlFlowFlowV2
func (yf YamlFlowFlowV2) Validate() error {
	if yf.Name == "" {
		return NewYamlFlowErrorV2("flow name is required", "name", nil)
	}

	// Validate variable names
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

// GetDefaultOptions returns default options for conversion
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

// Helper functions for working with the data structures

// GetHTTPByID finds an HTTP request by ID in the resolved data
func (syr *SimplifiedYAMLResolvedV2) GetHTTPByID(id idwrap.IDWrap) *mhttp.HTTP {
	for _, http := range syr.HTTPRequests {
		if http.ID.Compare(id) == 0 {
			return &http
		}
	}
	return nil
}

// GetFileByContentID finds a file by its ContentID in the resolved data
func (syr *SimplifiedYAMLResolvedV2) GetFileByContentID(contentID idwrap.IDWrap) *mfile.File {
	for _, file := range syr.Files {
		if file.ContentID != nil && file.ContentID.Compare(contentID) == 0 {
			return &file
		}
	}
	return nil
}

// GetFlowByName finds a flow by name in the resolved data
func (syr *SimplifiedYAMLResolvedV2) GetFlowByName(name string) *mflow.Flow {
	for _, flow := range syr.Flows {
		if flow.Name == name {
			return &flow
		}
	}
	return nil
}

// CountEntities returns a summary of entities in the resolved data
func (syr *SimplifiedYAMLResolvedV2) CountEntities() map[string]int {
	return map[string]int{
		"http_requests":      len(syr.HTTPRequests),
		"search_params":      len(syr.SearchParams),
		"headers":           len(syr.Headers),
		"body_forms":        len(syr.BodyForms),
		"body_urlencoded":   len(syr.BodyUrlencoded),
		"body_raw":          len(syr.BodyRaw),
		"files":             len(syr.Files),
		"flows":             len(syr.Flows),
		"flow_nodes":        len(syr.FlowNodes),
		"flow_edges":        len(syr.FlowEdges),
		"flow_variables":    len(syr.FlowVariables),
		"flow_request_nodes": len(syr.FlowRequestNodes),
		"flow_condition_nodes": len(syr.FlowConditionNodes),
		"flow_noop_nodes":   len(syr.FlowNoopNodes),
		"flow_for_nodes":    len(syr.FlowForNodes),
		"flow_foreach_nodes": len(syr.FlowForEachNodes),
		"flow_js_nodes":     len(syr.FlowJSNodes),
	}
}