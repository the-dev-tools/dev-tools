package loop

import (
	"context"
	"devtools-nodes/pkg/convert"
	"devtools-nodes/pkg/model/medge"
	"devtools-nodes/pkg/model/mnodemaster"
	"devtools-nodes/pkg/nodemaster"
	"devtools-platform/pkg/client/flyclient"
	"devtools-platform/pkg/machine/flymachine"
	nodemasterv1 "devtools-services/gen/nodemaster/v1"
	"devtools-services/gen/nodeslave/v1/nodeslavev1connect"
	"errors"
	"fmt"
	"net/http"
	"os"

	nodev1 "devtools-services/gen/nodeslave/v1"

	"connectrpc.com/connect"
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
		err := nodemaster.ExecuteNode(nm.Ctx, nm, nm.Resolver)
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

	nextNode, ok := nm.CurrentNode.Edges.OutNodes[medge.DefaultSuccessEdge]
	if !ok {
		nm.NextNodeID = ""
	}

	nm.NextNodeID = nextNode
	data.Count--
	return nil
}

type LoopRemoteData struct {
	Count          uint64
	LoopStartNode  string
	MachinesAmount uint64
}

func ForRemoteLoop(nm *mnodemaster.NodeMaster) error {
	data, ok := nm.CurrentNode.Data.(*LoopRemoteData)
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
	token := os.Getenv("FLY_TOKEN")

	client := flyclient.New(token, "devtools-nodes", false)
	if client != nil {
		return errors.New("client is nil")
	}

	flyMachines := make([]*flymachine.FlyMachine, data.MachinesAmount)
	for i := uint64(0); i < data.MachinesAmount; i++ {
		flyMachines[i] = &flymachine.FlyMachine{
			ID:   "id",
			Name: "name",
			Config: flymachine.FlyMachineCreateConfig{
				Image: "image",
			},
		}
	}

	machines, err := client.CreateMachines(flyMachines)
	if err != nil {
		return err
	}

	var connectClients []nodeslavev1connect.NodeSlaveServiceClient
	for _, machine := range machines {
		baseURL := fmt.Sprintf("http://%s:%d", machine.GetIP(), machine.GetInternalPort())

		connectClient := nodeslavev1connect.NewNodeSlaveServiceClient(http.DefaultClient, baseURL)
		connectClients = append(connectClients, connectClient)
	}

	for _, connectClient := range connectClients {
		for {
			enyData, err := convert.ConvertStructToMsg(nm.CurrentNode.Data)
			if err != nil {
				return err
			}

			nodeRemote := &nodev1.NodeSlaveServiceRunRequest{
				Node: &nodemasterv1.Node{
					Id:      nm.CurrentNode.ID,
					Type:    nm.CurrentNode.Type,
					Data:    enyData,
					OwnerId: nm.CurrentNode.OwnerID,
					GroupId: nm.CurrentNode.GroupID,
					Edges: &nodemasterv1.Edges{
						OutNodes: nm.CurrentNode.Edges.OutNodes,
						InNodes:  nm.CurrentNode.Edges.InNodes,
					},
				},
			}

			res, err := connectClient.Run(context.TODO(), connect.NewRequest(nodeRemote))
			if err != nil {
				return err
			}

			nextNodeID := res.Msg.NextNodeId

			if nm.NextNodeID == "" {
				break
			}

			loopCurrentNode, err = nodemaster.GetNodeByID(nm, nextNodeID)
			if err != nil {
				return err
			}
			nm.CurrentNode = loopCurrentNode
		}
	}

	return nil
}
