package rreference

import "github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"

// nodeDefaultSchemas maps each node kind to its default variable schema.
// Used when no execution data exists for a node.
// To add a new node kind: add one entry to this map.
var nodeDefaultSchemas = map[mflow.NodeKind]map[string]any{
	mflow.NODE_KIND_FOR: {
		"index": 0,
	},
	mflow.NODE_KIND_FOR_EACH: {
		"item": nil,
		"key":  0,
	},
	mflow.NODE_KIND_REQUEST: {
		"request":  defaultHTTPRequestSchema(),
		"response": defaultHTTPResponseSchema(),
	},
	mflow.NODE_KIND_GRAPHQL: {
		"request": map[string]any{
			"url":       "string",
			"query":     "string",
			"variables": map[string]any{},
			"headers":   map[string]string{},
		},
		"response": defaultHTTPResponseSchema(),
	},
	mflow.NODE_KIND_WS_CONNECTION: {
		"url":       "string",
		"connected": false,
		"cookies":   map[string]string{},
		"message":   "string",
		"index":     0,
		"type":      "string",
	},
	mflow.NODE_KIND_WS_SEND: {
		"type":           "string",
		"message":        "string",
		"connectionNode": "string",
		"cookies":        map[string]string{},
	},
	mflow.NODE_KIND_AI: {
		"text":          "",
		"total_metrics": map[string]any{},
		"iteration":     0,
	},
	mflow.NODE_KIND_JS: {},
	mflow.NODE_KIND_CONDITION: {
		"condition": "",
		"result":    false,
	},
	mflow.NODE_KIND_AI_PROVIDER: {
		"text":       "",
		"tool_calls": []any{},
		"metrics":    map[string]any{},
	},
	mflow.NODE_KIND_RUN_SUB_FLOW: {},
	// NODE_KIND_SUB_FLOW_RETURN: intentionally absent — terminal node, no output.
	// NODE_KIND_SUB_FLOW_TRIGGER: handled separately (requires DB lookup for params).
}

// nodeDefaultSchema returns the default schema for a node kind.
// Returns (nil, false) for kinds without a schema (SUB_FLOW_RETURN, unknown kinds).
// SUB_FLOW_TRIGGER is handled separately because it requires a DB lookup.
func nodeDefaultSchema(kind mflow.NodeKind) (map[string]any, bool) {
	schema, ok := nodeDefaultSchemas[kind]
	return schema, ok
}

func defaultHTTPResponseSchema() map[string]any {
	return map[string]any{
		"status":   200,
		"body":     map[string]any{},
		"headers":  map[string]string{},
		"duration": 0,
	}
}

func defaultHTTPRequestSchema() map[string]any {
	return map[string]any{
		"headers": map[string]string{},
		"queries": map[string]string{},
		"body":    "string",
	}
}

func defaultGraphQLResponseSchema() map[string]any {
	return map[string]any{
		"status":   200,
		"body":     map[string]any{},
		"data":     map[string]any{},
		"errors":   nil,
		"headers":  map[string]string{},
		"duration": 0,
	}
}
