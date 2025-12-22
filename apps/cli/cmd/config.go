package cmd

import (
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mhttp"
)

type DefaultConfig struct {
	// unified files (replaces collections)
	Files []mfile.File

	// HTTP requests and related data (unified mhttp models)
	HTTPRequests        []mhttp.HTTP
	HTTPHeaders         []mhttp.HTTPHeader
	HTTPSearchParams    []mhttp.HTTPSearchParam
	HTTPAsserts         []mhttp.HTTPAssert
	HTTPBodyForms       []mhttp.HTTPBodyForm
	HTTPBodyUrlencoded  []mhttp.HTTPBodyUrlencoded
	HTTPBodyRaws        []mhttp.HTTPBodyRaw
	HTTPResponses       []mhttp.HTTPResponse
	HTTPResponseHeaders []mhttp.HTTPResponseHeader
	HTTPResponseAsserts []mhttp.HTTPResponseAssert

	// flows (kept as-is - no unified model available yet)
	Flows []mflow.Flow

	// Root nodes (kept as-is)
	FlowNodes []mflow.Node

	// Sub nodes (kept as-is)
	FlowRequestNodes   []mflow.NodeRequest
	FlowConditionNodes []mflow.NodeIf
	FlowForNodes       []mflow.NodeFor
	FlowForEachNodes   []mflow.NodeForEach
	FlowJSNodes        []mflow.NodeJS
}
