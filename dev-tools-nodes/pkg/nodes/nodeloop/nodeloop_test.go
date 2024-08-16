package nodeloop_test

import (
	"dev-tools-nodes/pkg/model/medge"
	"dev-tools-nodes/pkg/model/mnode"
	"dev-tools-nodes/pkg/model/mnodedata"
	"dev-tools-nodes/pkg/model/mnodemaster"
	"dev-tools-nodes/pkg/nodes/nodeloop"
	"testing"
)

func TestLoopNode(t *testing.T) {
	NodeID1 := "node1"
	NodeID2 := "node2"
	NodeID3 := "node3"

	loopCount := 10
	nodeLength := 3
	totalResolve := 0
	expectedResolve := loopCount * nodeLength

	loopNode := &mnode.Node{
		Data: &mnodedata.NodeLoopData{
			Count:         loopCount,
			LoopStartNode: "node1",
		},
	}

	mockResolverFunc := func(string) (func(*mnodemaster.NodeMaster) error, error) {
		resolvedNodeFunc := func(nm *mnodemaster.NodeMaster) error {
			totalResolve++
			nodeID, ok := nm.CurrentNode.Edges.OutNodes["success"]
			if !ok {
				nm.NextNodeID = ""
				return nil
			}
			nm.NextNodeID = nodeID
			return nil
		}
		return resolvedNodeFunc, nil
	}

	nm := &mnodemaster.NodeMaster{
		CurrentNode: loopNode,
		Resolver:    mockResolverFunc,
	}

	nodes := map[string]mnode.Node{
		NodeID1: {
			ID: NodeID1,
			Edges: medge.Edges{
				OutNodes: map[string]string{
					"success": NodeID2,
				},
			},
		},
		NodeID2: {
			ID: NodeID2,
			Edges: medge.Edges{
				OutNodes: map[string]string{
					"success": NodeID3,
				},
			},
		},
		NodeID3: {
			ID:    NodeID3,
			Edges: medge.Edges{},
		},
	}

	nm.Nodes = nodes

	err := nodeloop.ForLoop(nm)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if totalResolve != expectedResolve {
		t.Errorf("unexpected totalResolve: %d", totalResolve)
	}
}
