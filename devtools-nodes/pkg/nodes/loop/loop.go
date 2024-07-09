package loop

import (
	"github.com/DevToolsGit/devtools-nodes/pkg/model/medge"
	"github.com/DevToolsGit/devtools-nodes/pkg/model/mnodemaster"
	"github.com/DevToolsGit/devtools-nodes/pkg/nodemaster"
)

type LoopData struct {
	Count         int
	LoopStartNode string
}

func ForLoop(nm *mnodemaster.NodeMaster) error {
	data, ok := nm.CurrentNode.Data.(*LoopData)
	if !ok {
		return mnodemaster.ErrInvalidDataType
	}

	if data.Count == 0 {
		nm.NextNodeID = nm.CurrentNode.Edges.OutNodes[medge.DefaultSuccessEdge]
		return nil
	}

	loopCurrentNode, err := nodemaster.GetNodeByID(nm, data.LoopStartNode)
	if err != nil {
		return err
	}
	nm.CurrentNode = loopCurrentNode

	currentLoopAmount := 0

	for {
		err := nodemaster.ExecuteNode(nm, nm.Resolver)
		if err != nil {
			return err
		}

		if nm.NextNodeID == "" {
			currentLoopAmount++
			if currentLoopAmount == data.Count {
				break
			}
			nm.NextNodeID = data.LoopStartNode
		}

		loopCurrentNode, err = nodemaster.GetNodeByID(nm, nm.NextNodeID)
		if err != nil {
			return err
		}
		nm.CurrentNode = loopCurrentNode

	}

	nm.NextNodeID = data.LoopStartNode
	data.Count--
	return nil
}
