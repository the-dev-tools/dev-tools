package sflow

import (
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

func ConvertToDBNodeWsSend(n mflow.NodeWsSend) gen.FlowNodeWsSend {
	return gen.FlowNodeWsSend{
		FlowNodeID:           n.FlowNodeID,
		WsConnectionNodeName: n.WsConnectionNodeName,
		Message:              n.Message,
	}
}

func ConvertToModelNodeWsSend(n gen.FlowNodeWsSend) *mflow.NodeWsSend {
	return &mflow.NodeWsSend{
		FlowNodeID:           n.FlowNodeID,
		WsConnectionNodeName: n.WsConnectionNodeName,
		Message:              n.Message,
	}
}
