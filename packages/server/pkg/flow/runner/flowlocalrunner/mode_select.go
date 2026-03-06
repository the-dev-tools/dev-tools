package flowlocalrunner

import (
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

func selectExecutionMode(nodeMap map[idwrap.IDWrap]node.FlowNode, edgesMap mflow.EdgesMap) ExecutionMode {
	nodeCount := len(nodeMap)
	if nodeCount == 0 {
		return ExecutionModeSingle
	}

	const smallFlowThreshold = 6

	simpleStructure := true
	incomingNonLoop := make(map[idwrap.IDWrap]int)

	for sourceID, handles := range edgesMap {
		nonLoopTargets := 0
		for handle, targetIDs := range handles {
			if len(targetIDs) == 0 {
				continue
			}
			if handle == mflow.HandleLoop {
				if len(targetIDs) > 1 {
					simpleStructure = false
				}
				continue
			}

			nonLoopTargets += len(targetIDs)
			if len(targetIDs) > 1 {
				simpleStructure = false
			}
			for _, targetID := range targetIDs {
				incomingNonLoop[targetID]++
			}
		}
		if nonLoopTargets > 1 {
			simpleStructure = false
		}
		if _, ok := handles[mflow.HandleLoop]; ok && nonLoopTargets > 0 {
			// Loop node with additional branch work beyond the loop/then path
			if nonLoopTargets > 1 {
				simpleStructure = false
			}
		}

		if _, exists := nodeMap[sourceID]; !exists {
			// Node present in edges but missing from map; treat as complex and bail out
			simpleStructure = false
		}
	}

	for targetID, count := range incomingNonLoop {
		if count > 1 {
			simpleStructure = false
			break
		}
		if _, exists := nodeMap[targetID]; !exists {
			simpleStructure = false
		}
	}

	if nodeCount <= smallFlowThreshold && simpleStructure {
		return ExecutionModeSingle
	}

	return ExecutionModeMulti
}
