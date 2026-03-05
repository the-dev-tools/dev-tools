package sflow

import (
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

func ConvertDBToNodeWait(nw gen.FlowNodeWait) *mflow.NodeWait {
	return &mflow.NodeWait{
		FlowNodeID: nw.FlowNodeID,
		DurationMs: nw.DurationMs,
	}
}

func ConvertNodeWaitToDB(mn mflow.NodeWait) gen.FlowNodeWait {
	return gen.FlowNodeWait{
		FlowNodeID: mn.FlowNodeID,
		DurationMs: mn.DurationMs,
	}
}
