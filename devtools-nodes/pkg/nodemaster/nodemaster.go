package nodemaster

import (
	"errors"

	"github.com/DevToolsGit/devtools-nodes/pkg/model/mnode"
	"github.com/DevToolsGit/devtools-nodes/pkg/model/mnodemaster"
	"github.com/DevToolsGit/devtools-nodes/pkg/resolver"
)

var ErrNodeNotFound = errors.New("node not found")

func Run(nm *mnodemaster.NodeMaster) error {
	startNode, ok := nm.Nodes[nm.StartNodeID]
	nm.CurrentNode = &startNode
	if !ok {
		return ErrNodeNotFound
	}

	for {
		err := ExecuteNext(nm)
		if err != nil {
			return err
		}

		if nm.NextNodeID == "" {
			// done with the execution
			break
		}

		nextNode, err := GetNodeByID(nm, nm.NextNodeID)
		if err != nil {
			return err
		}

		nm.CurrentNode = nextNode
	}

	return nil
}

func GetNodeByID(nm *mnodemaster.NodeMaster, nodeID string) (*mnode.Node, error) {
	node, ok := nm.Nodes[nodeID]
	if !ok {
		return nil, ErrNodeNotFound
	}
	return &node, nil
}

var ErrInvalidDataType = errors.New("invalid data type")

func ExecuteNext(nm *mnodemaster.NodeMaster) error {
	nodeType := nm.CurrentNode.Type

	nodeFunc, err := resolver.ResolveNodeFunc(nodeType)
	if err != nil {
		return err
	}

	err = nodeFunc(nm)
	if err != nil {
		return errors.New("nodeFunc failed")
	}

	return nil
}
