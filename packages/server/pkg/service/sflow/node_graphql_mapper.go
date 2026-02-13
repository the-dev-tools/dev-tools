package sflow

import (
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

func ConvertToDBNodeGraphQL(ng mflow.NodeGraphQL) (gen.FlowNodeGraphql, bool) {
	if ng.GraphQLID == nil || isZeroID(*ng.GraphQLID) {
		return gen.FlowNodeGraphql{}, false
	}

	return gen.FlowNodeGraphql{
		FlowNodeID: ng.FlowNodeID,
		GraphqlID:  *ng.GraphQLID,
	}, true
}

func ConvertToModelNodeGraphQL(ng gen.FlowNodeGraphql) *mflow.NodeGraphQL {
	graphqlID := ng.GraphqlID

	return &mflow.NodeGraphQL{
		FlowNodeID: ng.FlowNodeID,
		GraphQLID:  &graphqlID,
	}
}
