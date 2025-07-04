package workflow

import (
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/model/massertres"
	"the-dev-tools/server/pkg/model/mbodyform"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mbodyurl"
	"the-dev-tools/server/pkg/model/mcollection"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mexamplequery"
	"the-dev-tools/server/pkg/model/mexampleresp"
	"the-dev-tools/server/pkg/model/mexamplerespheader"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mflowvariable"
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
	"the-dev-tools/server/pkg/model/mworkspace"
)

// Workflow represents the core interface for workflow operations
type Workflow interface {
	// Marshal converts WorkspaceData to bytes in the specific format
	Marshal(data *WorkspaceData, format Format) ([]byte, error)

	// Unmarshal converts bytes to WorkspaceData
	Unmarshal(data []byte, format Format) (*WorkspaceData, error)
}

// WorkspaceData represents the internal data structure
type WorkspaceData struct {
	Workspace  mworkspace.Workspace
	Collection mcollection.Collection
	Folders    []mitemfolder.ItemFolder
	Endpoints  []mitemapi.ItemApi
	Examples   []mitemapiexample.ItemApiExample

	FlowNodes          []mnnode.MNode
	FlowEdges          []edge.Edge
	FlowVariables      []mflowvariable.FlowVariable
	FlowRequestNodes   []mnrequest.MNRequest
	FlowConditionNodes []mnif.MNIF
	FlowNoopNodes      []mnnoop.NoopNode
	FlowForNodes       []mnfor.MNFor
	FlowForEachNodes   []mnforeach.MNForEach
	FlowJSNodes        []mnjs.MNJS
	Flows              []mflow.Flow

	RequestQueries        []mexamplequery.Query
	RequestHeaders        []mexampleheader.Header
	RequestBodyRaw        []mbodyraw.ExampleBodyRaw
	RequestBodyForm       []mbodyform.BodyForm
	RequestBodyUrlencoded []mbodyurl.BodyURLEncoded
	RequestAsserts        []massert.Assert

	Responses              []mexampleresp.ExampleResp
	ResponseHeaders        []mexamplerespheader.ExampleRespHeader
	ResponseAsserts        []massertres.AssertResult
	ResponseBodyRaw        []mbodyraw.ExampleBodyRaw
	ResponseBodyForm       []mbodyform.BodyForm
	ResponseBodyUrlencoded []mbodyurl.BodyURLEncoded

	// Map EndpointID to a list of its ExampleIDs
	EndpointExampleMap map[idwrap.IDWrap][]idwrap.IDWrap
}