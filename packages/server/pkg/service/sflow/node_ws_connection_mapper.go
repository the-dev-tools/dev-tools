package sflow

import (
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

func ConvertToDBNodeWsConnection(n mflow.NodeWsConnection) (gen.FlowNodeWsConnection, bool) {
	if n.WebSocketID == nil || isZeroID(*n.WebSocketID) {
		return gen.FlowNodeWsConnection{}, false
	}

	return gen.FlowNodeWsConnection{
		FlowNodeID:  n.FlowNodeID,
		WebsocketID: n.WebSocketID,
	}, true
}

func ConvertToModelNodeWsConnection(n gen.FlowNodeWsConnection) *mflow.NodeWsConnection {
	return &mflow.NodeWsConnection{
		FlowNodeID:  n.FlowNodeID,
		WebSocketID: n.WebsocketID,
	}
}
