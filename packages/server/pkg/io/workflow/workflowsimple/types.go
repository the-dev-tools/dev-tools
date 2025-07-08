package workflowsimple

import (
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mcollection"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mexamplequery"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mflowvariable"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"the-dev-tools/server/pkg/model/mnnode/mnforeach"
	"the-dev-tools/server/pkg/model/mnnode/mnif"
	"the-dev-tools/server/pkg/model/mnnode/mnjs"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/model/mvar"
)

// WorkflowFormat represents the top-level YAML structure
type WorkflowFormat struct {
	WorkspaceName    string                    `yaml:"workspace_name"`
	Run              map[string]any            `yaml:"run,omitempty"`
	RequestTemplates map[string]map[string]any `yaml:"request_templates,omitempty"` // Old format
	Requests         []map[string]any          `yaml:"requests,omitempty"`          // New format
	Flows            []WorkflowFlow            `yaml:"flows"`
}

// WorkflowFlow represents a single flow definition
type WorkflowFlow struct {
	Name      string     `yaml:"name"`
	Variables []Variable `yaml:"variables,omitempty"`
	Steps     []any      `yaml:"steps"`
}

// Variable represents a flow variable
// Special variables:
//   - "timeout": Controls flow execution timeout in seconds (default: 60)
//     Example: {name: "timeout", value: "300"} sets a 5-minute timeout
type Variable struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

// WorkflowStep is the base step with common fields
type WorkflowStep struct {
	DependsOn []string `yaml:"depends_on,omitempty"`
}

// RequestStep represents a request step
type RequestStep struct {
	WorkflowStep `yaml:",inline"`
	Name         string              `yaml:"name"`
	UseRequest   string              `yaml:"use_request,omitempty"`
	Method       string              `yaml:"method,omitempty"`
	Url          string              `yaml:"url,omitempty"`
	Headers      []map[string]string `yaml:"headers,omitempty"`
	QueryParams  []map[string]string `yaml:"query_params,omitempty"`
	Body         map[string]any      `yaml:"body,omitempty"`
}

// IfStep represents a conditional step
type IfStep struct {
	WorkflowStep `yaml:",inline"`
	Name         string `yaml:"name"`
	Condition    string `yaml:"condition"`
	Then         string `yaml:"then,omitempty"`
	Else         string `yaml:"else,omitempty"`
}

// ForStep represents a for loop step
type ForStep struct {
	WorkflowStep `yaml:",inline"`
	Name         string `yaml:"name"`
	IterCount    int    `yaml:"iter_count"`
	Loop         string `yaml:"loop,omitempty"`
}

// ForEachStep represents a for-each loop step
type ForEachStep struct {
	WorkflowStep `yaml:",inline"`
	Name         string `yaml:"name"`
	Items        string `yaml:"items"`
	Loop         string `yaml:"loop,omitempty"`
}

// JSStep represents a JavaScript execution step
type JSStep struct {
	WorkflowStep `yaml:",inline"`
	Name         string `yaml:"name"`
	Code         string `yaml:"code"`
}

// WorkflowData represents the parsed workflow data
type WorkflowData struct {
	// Flow items
	Flow           mflow.Flow
	Nodes          []mnnode.MNode
	Edges          []edge.Edge
	Variables      []mvar.Var
	NoopNodes      []mnnoop.NoopNode
	RequestNodes   []mnrequest.MNRequest
	ConditionNodes []mnif.MNIF
	ForNodes       []mnfor.MNFor
	ForEachNodes   []mnforeach.MNForEach
	JSNodes        []mnjs.MNJS

	// Collection items
	Endpoints []mitemapi.ItemApi
	Examples  []mitemapiexample.ItemApiExample
	Headers   []mexampleheader.Header
	Queries   []mexamplequery.Query
	RawBodies []mbodyraw.ExampleBodyRaw
}

// SimplifiedYAMLResolved contains all entities parsed from simplified YAML
type SimplifiedYAMLResolved struct {
	// Collection Items
	Collections []mcollection.Collection
	Endpoints   []mitemapi.ItemApi
	Examples    []mitemapiexample.ItemApiExample
	Headers     []mexampleheader.Header
	Queries     []mexamplequery.Query
	RawBodies   []mbodyraw.ExampleBodyRaw

	// Flow Items
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

// nodeInfo stores information about nodes during parsing
type nodeInfo struct {
	id        idwrap.IDWrap
	name      string
	index     int
	dependsOn []string
}

// requestTemplate stores parsed request template data
type requestTemplate struct {
	method      string
	url         string
	headers     []map[string]string
	queryParams []map[string]string
	body        map[string]any
}
