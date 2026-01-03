// Package eventsync provides dependency-based event ordering for real-time sync.
// It ensures events are published in the correct order for frontend rendering
// (TanStack DB requires entities to exist before things that reference them).
package eventsync

// EventKind represents a type of sync event.
// The ordering of events is determined by the Dependencies map.
type EventKind string

const (
	// Core entities (roots)
	KindFlow        EventKind = "flow"
	KindEnvironment EventKind = "environment"
	KindFolder      EventKind = "folder"

	// Flow-related (depend on Flow)
	KindFlowFile EventKind = "flow_file"
	KindNode     EventKind = "node"

	// Graph structure (depend on Node)
	KindEdge EventKind = "edge"

	// HTTP entities (depend on Node for request nodes)
	KindHTTP EventKind = "http"

	// HTTP children (depend on HTTP)
	KindHTTPFile       EventKind = "http_file"
	KindHTTPHeader     EventKind = "http_header"
	KindHTTPParam      EventKind = "http_param"
	KindHTTPBodyForm   EventKind = "http_body_form"
	KindHTTPBodyURL    EventKind = "http_body_url"
	KindHTTPBodyRaw    EventKind = "http_body_raw"
	KindHTTPAssert     EventKind = "http_assert"

	// Environment children (depend on Environment)
	KindEnvVariable EventKind = "env_variable"
)

// Dependencies defines what each event kind depends on.
// This is the single source of truth for event ordering.
// The topological sort uses this to compute the correct publish order.
//
// Frontend TanStack DB requires:
// - Flow must exist before FlowFile (FlowFile.contentId references Flow.id)
// - Flow must exist before Node (Node.flowId references Flow.id)
// - Node must exist before Edge (Edge.sourceId/targetId reference Node.id)
// - HTTP must exist before its children (headers, params, etc.)
var Dependencies = map[EventKind][]EventKind{
	// Roots - no dependencies
	KindFlow:        {},
	KindEnvironment: {},

	// Flow children - depend on Flow existing
	KindFlowFile: {KindFlow},
	KindFolder:   {KindFlowFile}, // Folders come after flow files for consistent ordering
	KindNode:     {KindFlow},

	// Graph edges - depend on nodes existing
	KindEdge: {KindNode},

	// HTTP - depends on Node (request nodes reference HTTP)
	KindHTTP: {KindNode},

	// HTTP children - depend on HTTP existing
	KindHTTPFile:     {KindHTTP},
	KindHTTPHeader:   {KindHTTP},
	KindHTTPParam:    {KindHTTP},
	KindHTTPBodyForm: {KindHTTP},
	KindHTTPBodyURL:  {KindHTTP},
	KindHTTPBodyRaw:  {KindHTTP},
	KindHTTPAssert:   {KindHTTP},

	// Environment children
	KindEnvVariable: {KindEnvironment},
}
