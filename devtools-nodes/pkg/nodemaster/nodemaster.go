package nodemaster

import (
	"devtools-nodes/pkg/httpclient"
	"devtools-nodes/pkg/model/mnode"
	"devtools-nodes/pkg/model/mnodemaster"
	"devtools-nodes/pkg/model/mresolver"
	"devtools-nodes/pkg/model/mstatus"
	"errors"

	"github.com/google/uuid"
)

var ErrNodeNotFound = errors.New("node not found")

func NewNodeMaster(startNodeID string, nodes map[string]mnode.Node, resolver mresolver.Resolver, stateChan chan mstatus.StatusUpdateData, httpClient httpclient.HttpClient) (*mnodemaster.NodeMaster, error) {
	uuid, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}

	return &mnodemaster.NodeMaster{
		ID:          uuid.String(),
		StartNodeID: startNodeID,
		Nodes:       nodes,
		Vars:        map[string]interface{}{},
		Resolver:    resolver,
		StateChan:   stateChan,
		HttpClient:  httpClient,
		CurrentNode: nil,
		NextNodeID:  "",
	}, nil
}

func Run(nm *mnodemaster.NodeMaster) error {
	startNode, ok := nm.Nodes[nm.StartNodeID]
	nm.CurrentNode = &startNode
	if !ok {
		return ErrNodeNotFound
	}

	for {
		err := ExecuteNode(nm, nm.Resolver)
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

func ExecuteNode(nm *mnodemaster.NodeMaster, resolver mresolver.Resolver) error {
	nodeType := nm.CurrentNode.Type

	nodeFunc, err := nm.Resolver(nodeType)
	if err != nil {
		return err
	}

	err = nodeFunc(nm)
	if err != nil {
		return errors.New("nodeFunc failed")
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
