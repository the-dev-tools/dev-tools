package sflow

import (
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

func ConvertToDBNodeGraphQL(ng mflow.NodeGraphQL) (gen.FlowNodeGraphql, bool) {
	if ng.GraphQLID == nil || isZeroID(*ng.GraphQLID) {
		return gen.FlowNodeGraphql{}, false
	}

	dbNode := gen.FlowNodeGraphql{
		FlowNodeID: ng.FlowNodeID,
		GraphqlID:  *ng.GraphQLID,
	}

	if ng.DeltaGraphQLID != nil {
		dbNode.DeltaGraphqlID = ng.DeltaGraphQLID.Bytes()
	}

	return dbNode, true
}

func ConvertToModelNodeGraphQL(ng gen.FlowNodeGraphql) *mflow.NodeGraphQL {
	graphqlID := ng.GraphqlID
	modelNode := &mflow.NodeGraphQL{
		FlowNodeID: ng.FlowNodeID,
		GraphQLID:  &graphqlID,
	}

	if len(ng.DeltaGraphqlID) > 0 {
		deltaID, err := idwrap.NewFromBytes(ng.DeltaGraphqlID)
		if err == nil {
			modelNode.DeltaGraphQLID = &deltaID
		}
	}

	return modelNode
}
