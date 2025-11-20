package cmd

import (
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"the-dev-tools/server/pkg/model/mnnode/mnforeach"
	"the-dev-tools/server/pkg/model/mnnode/mnif"
	"the-dev-tools/server/pkg/model/mnnode/mnjs"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
)

type DefaultConfig struct {
	// unified files (replaces collections)
	Files []mfile.File

	// HTTP requests and related data (unified mhttp models)
	HTTPRequests      []mhttp.HTTP
	HTTPHeaders       []mhttp.HTTPHeader
	HTTPSearchParams  []mhttp.HTTPSearchParam
	HTTPAsserts       []mhttp.HTTPAssert
	HTTPBodyForms     []mhttp.HTTPBodyForm
	HTTPBodyUrlencoded []mhttp.HTTPBodyUrlencoded
	HTTPBodyRaws      []mhttp.HTTPBodyRaw
	HTTPResponses     []mhttp.HTTPResponse
	HTTPResponseHeaders []mhttp.HTTPResponseHeader
	HTTPResponseAsserts []mhttp.HTTPResponseAssert

	// flows (kept as-is - no unified model available yet)
	Flows []mflow.Flow

	// Root nodes (kept as-is)
	FlowNodes []mnnode.MNode

	// Sub nodes (kept as-is)
	FlowRequestNodes   []mnrequest.MNRequest
	FlowConditionNodes []mnif.MNIF
	FlowNoopNodes      []mnnoop.NoopNode
	FlowForNodes       []mnfor.MNFor
	FlowForEachNodes   []mnforeach.MNForEach
	FlowJSNodes        []mnjs.MNJS
}
