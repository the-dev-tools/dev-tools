package nodeloop

/*

import (
	"the-dev-tools/nodes/pkg/convert"
	"the-dev-tools/nodes/pkg/model/medge"
	"the-dev-tools/nodes/pkg/model/mnodedata"
	"the-dev-tools/nodes/pkg/model/mnodemaster"
	"the-dev-tools/nodes/pkg/model/mstatus"
	"the-dev-tools/nodes/pkg/nodemaster"
	nodemasterv1 "the-dev-tools/services/gen/nodemaster/v1"
	nodeslavev1 "the-dev-tools/services/gen/nodeslave/v1"
	"the-dev-tools/services/gen/nodeslave/v1/nodeslavev1connect"
	"errors"
	"log"
	"sync"

	"connectrpc.com/connect"
	"github.com/bufbuild/httplb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func ForLoop(nm *mnodemaster.NodeMaster) error {
	data, ok := nm.CurrentNode.Data.(*mnodedata.NodeLoopData)
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
			nodemaster.SetNextNode(nm, data.LoopStartNode)
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

	nodemaster.SetNextNode(nm, nextNode)
	data.Count--
	return nil
}

func ForRemoteLoop(nm *mnodemaster.NodeMaster) error {
	data, ok := nm.CurrentNode.Data.(*mnodedata.NodeLoopRemoteData)
	if !ok {
		return mnodemaster.ErrInvalidDataType
	}

	if data.Count == 0 {
		nm.NextNodeID = nm.CurrentNode.Edges.OutNodes[medge.DefaultSuccessEdge]
		return nil
	}

	nodes := make(map[string]*nodemasterv1.Node)
	for _, node := range nm.Nodes {
		castedData, err := convert.ConvertStructToMsg(node.Data)
		if err != nil {
			return err
		}

		nodes[node.ID] = &nodemasterv1.Node{
			Id:      node.ID,
			Type:    node.Type,
			Data:    castedData,
			OwnerId: node.OwnerID,
			GroupId: node.GroupID,
			Edges: &nodemasterv1.Edges{
				OutNodes: node.Edges.OutNodes,
				InNodes:  node.Edges.InNodes,
			},
		}
	}

	VarAnyPb, err := convert.ConvertVarsToAny(nm.Vars)
	if err != nil {
		return err
	}

	multiRunReq := nodeslavev1.NodeSlaveServiceRunMultiRequest{
		StartNodeId: data.LoopStartNode,
		StopNodeId:  "",
		Nodes:       nodes,
		Vars:        VarAnyPb,
	}

	upstream := data.SlaveHttpEndpoint

	wg := sync.WaitGroup{}
	errChan := make(chan error, data.Count)

	perRound := data.Count / data.MachinesAmount
	for i := uint64(0); i < data.MachinesAmount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			httpClient := httplb.NewClient()
			if httpClient == nil {
				errChan <- errors.New("failed to create http client")
			}
			defer httpClient.Close()

			req := connect.NewRequest(&multiRunReq)
			if req == nil {
				errChan <- errors.New("failed to create request")
			}

			client := nodeslavev1connect.NewNodeSlaveServiceClient(httpClient, upstream)
			if client == nil {
				errChan <- errors.New("failed to create client")
			}
			for j := uint64(0); j < perRound; j++ {

				stream, err := client.RunMulti(nm.Ctx, req)
				if err != nil {
					errChan <- err
				}

				defer stream.Close()

				for stream.Receive() {

					msg := stream.Msg()
					if msg == nil {
						errChan <- errors.New("stream.Msg() is nil")
						continue
					}

					if msg.NodeStatus == nil {
						panic(msg.NodeStatus)
					}

					if msg.NodeStatus.Data == nil {
						errChan <- errors.New("stream.Msg().NodeStatus.Data is nil")
						panic(msg.NodeStatus.Data)
					}

					nodeDataRaw, err := anypb.UnmarshalNew(msg.NodeStatus.Data, proto.UnmarshalOptions{})
					if err != nil {
						errChan <- errors.New("stream.Msg() is nil")
					}

					nodeStatusData, err := convert.ConvertMsgToNodeStatus(nodeDataRaw)
					if err != nil {
						errChan <- err
						log.Fatalf("Error: %v", err)
					}

					nodeStatus := mstatus.NodeStatus{
						Type: msg.NodeStatus.Type,
						Data: nodeStatusData,
					}

					nm.StateChan <- nodeStatus
					// nodeIDPase := stream.Msg().NodeId
				}
			}
		}()
	}

	wg.Wait()

	select {
	case err := <-errChan:
		return err
	default:
	}

	return nil
}
*/
