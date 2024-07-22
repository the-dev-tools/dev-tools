package loop

import (
	"devtools-nodes/pkg/convert"
	"devtools-nodes/pkg/model/medge"
	"devtools-nodes/pkg/model/mnodedata"
	"devtools-nodes/pkg/model/mnodemaster"
	"devtools-nodes/pkg/nodemaster"
	nodemasterv1 "devtools-services/gen/nodemaster/v1"
	nodeslavev1 "devtools-services/gen/nodeslave/v1"
	"devtools-services/gen/nodeslave/v1/nodeslavev1connect"
	"errors"
	"sync"

	"connectrpc.com/connect"
	"github.com/bufbuild/httplb"
)

func ForLoop(nm *mnodemaster.NodeMaster) error {
	data, ok := nm.CurrentNode.Data.(*mnodedata.LoopData)
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
	data, ok := nm.CurrentNode.Data.(*mnodedata.LoopRemoteData)
	if !ok {
		return mnodemaster.ErrInvalidDataType
	}

	if data.Count == 0 {
		nm.NextNodeID = nm.CurrentNode.Edges.OutNodes[medge.DefaultSuccessEdge]
		return nil
	}

	nodes := make(map[string]*nodemasterv1.Node)

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
					}
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
