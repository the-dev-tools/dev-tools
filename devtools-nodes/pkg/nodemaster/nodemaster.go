package nodemaster

import (
	"context"
	"devtools-nodes/pkg/httpclient"
	"devtools-nodes/pkg/model/mnode"
	"devtools-nodes/pkg/model/mnodemaster"
	"devtools-nodes/pkg/model/mstatus"
	"errors"
	"log"

	"github.com/google/uuid"
)

var ErrNodeNotFound = errors.New("node not found")

func NewNodeMaster(startNodeID string, nodes map[string]mnode.Node, resolver mnodemaster.Resolver, executeNodeFunc mnodemaster.ExcuteNodeFunc, stateChan chan mstatus.StatusUpdateData, httpClient httpclient.HttpClient) (*mnodemaster.NodeMaster, error) {
	uuid, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}

	return &mnodemaster.NodeMaster{
		ID:              uuid.String(),
		StartNodeID:     startNodeID,
		Nodes:           nodes,
		Vars:            map[string]interface{}{},
		Resolver:        resolver,
		ExecuteNodeFunc: executeNodeFunc,
		Logger:          log.Default(),
		StateChan:       stateChan,
		HttpClient:      httpClient,
		CurrentNode:     nil,
		NextNodeID:      "",
	}, nil
}

func Run(nm *mnodemaster.NodeMaster, ctx context.Context) error {
	startNode, ok := nm.Nodes[nm.StartNodeID]
	nm.CurrentNode = &startNode
	if !ok {
		return ErrNodeNotFound
	}

	for {
		nm.Logger.Printf("Executing node %s", nm.CurrentNode.ID)
		err := nm.ExecuteNodeFunc(ctx, nm, nm.Resolver)
		if err != nil {
			return err
		}

		nm.Logger.Printf("Node %s execution completed", nm.CurrentNode.ID)
		if nm.NextNodeID == "" {
			nm.Logger.Printf("Next node is empty")
			// done with the execution
			break
		}

		nm.Logger.Printf("Next node %s", nm.NextNodeID)

		nextNode, err := GetNodeByID(nm, nm.NextNodeID)
		if err != nil {
			return err
		}

		nm.CurrentNode = nextNode
	}

	nm.Logger.Printf("NodeMaster %s Execution completed", nm.ID)

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

func ExecuteNode(ctx context.Context, nm *mnodemaster.NodeMaster, resolver mnodemaster.Resolver) error {
	if nm.CurrentNode == nil {
		return errors.New("current node is nil")
	}
	nodeType := nm.CurrentNode.Type

	nodeFunc, err := resolver(nodeType)
	if err != nil {
		return err
	}

	err = nodeFunc(nm)
	if err != nil {
		return errors.New("nodeFunc failed: " + err.Error())
	}

	return nil
}

func SetNextNode(nm *mnodemaster.NodeMaster, nodeID string) {
	nm.NextNodeID = nodeID

	if nm.StateChan != nil {
		nm.StateChan <- mstatus.StatusUpdateData{
			Type:      mstatus.StatusTypeNextNode,
			Data:      mstatus.StatusDataNextNode{NodeID: nodeID},
			TriggerBy: nm.CurrentNode.ID,
		}
	}
}

func GetVar(nm *mnodemaster.NodeMaster, key string) (interface{}, error) {
	val, ok := nm.Vars[key]
	if !ok {
		return nil, ErrInvalidDataType
	}
	return val, nil
}

func SetVar(nm *mnodemaster.NodeMaster, key string, val interface{}, triggerBy string) {
	nm.Vars[key] = val

	if nm.StateChan != nil {
		nm.StateChan <- mstatus.StatusUpdateData{
			Type:      mstatus.StatusTypeSetVar,
			Data:      mstatus.StatusDataSetVar{Key: key, Val: val},
			TriggerBy: triggerBy,
		}
	}
}
