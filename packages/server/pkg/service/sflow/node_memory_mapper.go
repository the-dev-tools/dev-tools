package sflow

import (
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

func ConvertDBToNodeMemory(nf gen.FlowNodeMemory) *mflow.NodeMemory {
	nodeID, _ := idwrap.NewFromBytes(nf.FlowNodeID)
	return &mflow.NodeMemory{
		FlowNodeID: nodeID,
		MemoryType: mflow.AiMemoryType(nf.MemoryType),
		WindowSize: nf.WindowSize,
	}
}

func ConvertNodeMemoryToDB(mn mflow.NodeMemory) gen.FlowNodeMemory {
	return gen.FlowNodeMemory{
		FlowNodeID: mn.FlowNodeID.Bytes(),
		MemoryType: int8(mn.MemoryType),
		WindowSize: mn.WindowSize,
	}
}
