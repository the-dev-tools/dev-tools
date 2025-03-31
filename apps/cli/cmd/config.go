package cmd

import (
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/model/mbodyform"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mbodyurl"
	"the-dev-tools/server/pkg/model/mcollection"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mexamplequery"
	"the-dev-tools/server/pkg/model/mexampleresp"
	"the-dev-tools/server/pkg/model/mexamplerespheader"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mitemfolder"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"the-dev-tools/server/pkg/model/mnnode/mnforeach"
	"the-dev-tools/server/pkg/model/mnnode/mnif"
	"the-dev-tools/server/pkg/model/mnnode/mnjs"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
)

type DefaultConfig struct {
	// collections
	Collections []mcollection.Collection
	Folders     []mitemfolder.ItemFolder
	Endpoints   []mitemapi.ItemApi
	Examples    []mitemapiexample.ItemApiExample

	// example sub items

	ExampleHeaders []mexampleheader.Header
	ExampleQueries []mexamplequery.Query
	ExampleAsserts []massert.Assert

	// body
	Rawbodies  []mbodyraw.ExampleBodyRaw
	FormBodies []mbodyform.BodyForm
	UrlBodies  []mbodyurl.BodyURLEncoded

	// response
	ExampleResponses       []mexampleresp.ExampleResp
	ExampleResponseHeaders []mexamplerespheader.ExampleRespHeader
	ExampleResponseAsserts []massert.Assert

	// flows
	Flows []mflow.Flow

	// Root nodes
	FlowNodes []mnnode.MNode

	// Sub nodes
	FlowRequestNodes   []mnrequest.MNRequest
	FlowConditionNodes []mnif.MNIF
	FlowNoopNodes      []mnnoop.NoopNode
	FlowForNodes       []mnfor.MNFor
	FlowForEachNodes   []mnforeach.MNForEach
	FlowJSNodes        []mnjs.MNJS
}
