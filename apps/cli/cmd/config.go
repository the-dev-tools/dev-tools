package cmd

import (
	"the-dev-tools/backend/pkg/model/massert"
	"the-dev-tools/backend/pkg/model/mbodyform"
	"the-dev-tools/backend/pkg/model/mbodyraw"
	"the-dev-tools/backend/pkg/model/mbodyurl"
	"the-dev-tools/backend/pkg/model/mcollection"
	"the-dev-tools/backend/pkg/model/mexampleheader"
	"the-dev-tools/backend/pkg/model/mexamplequery"
	"the-dev-tools/backend/pkg/model/mexampleresp"
	"the-dev-tools/backend/pkg/model/mexamplerespheader"
	"the-dev-tools/backend/pkg/model/mflow"
	"the-dev-tools/backend/pkg/model/mitemapi"
	"the-dev-tools/backend/pkg/model/mitemapiexample"
	"the-dev-tools/backend/pkg/model/mitemfolder"
	"the-dev-tools/backend/pkg/model/mnnode"
	"the-dev-tools/backend/pkg/model/mnnode/mnfor"
	"the-dev-tools/backend/pkg/model/mnnode/mnforeach"
	"the-dev-tools/backend/pkg/model/mnnode/mnif"
	"the-dev-tools/backend/pkg/model/mnnode/mnjs"
	"the-dev-tools/backend/pkg/model/mnnode/mnnoop"
	"the-dev-tools/backend/pkg/model/mnnode/mnrequest"
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
