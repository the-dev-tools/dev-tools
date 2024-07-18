package nodemaster_test

import (
	"context"
	"devtools-nodes/pkg/model/medge"
	"devtools-nodes/pkg/model/mnode"
	"devtools-nodes/pkg/model/mnodemaster"
	"devtools-nodes/pkg/nodemaster"
	"testing"

	"github.com/google/uuid"
)

func MockResolver(nodeType string) (func(*mnodemaster.NodeMaster) error, error) {
	resolvedNodeFunc := func(nm *mnodemaster.NodeMaster) error {
		nm.NextNodeID = nm.CurrentNode.Edges.OutNodes["success"]
		return nil
	}
	return resolvedNodeFunc, nil
}

func TestNodeMasterRun(t *testing.T) {
	uuid, err := uuid.NewV7()
	if err != nil {
		t.Errorf("Error generating UUID: %v", err)
	}

	nodes := map[string]mnode.Node{
		"start": {ID: "start", Type: "start", Data: nil, Edges: medge.Edges{
			OutNodes: map[string]string{"success": "middle"},
		}},
		"middle": {ID: "middle", Type: "middle", Data: nil, Edges: medge.Edges{
			OutNodes: map[string]string{"success": "end"},
		}},
		"end": {ID: "end", Type: "end", Data: nil, Edges: medge.Edges{
			OutNodes: map[string]string{"success": ""},
		}},
	}

	nm := &mnodemaster.NodeMaster{
		ID:          uuid.String(),
		StartNodeID: "start",
		Nodes:       nodes,
		Vars:        map[string]interface{}{},
		CurrentNode: nil,
		NextNodeID:  "",
		Resolver:    MockResolver,
	}

	nodemaster.Run(nm, context.Background())
}

func TestNodeMasterSetAndGetVar(t *testing.T) {
	nm := &mnodemaster.NodeMaster{
		ID:   "nodeMasterID",
		Vars: map[string]interface{}{},
	}

	testKey := "key"
	testValue := "value"

	nodemaster.SetVar(nm, "key", "value", "triggerID")

	returnVal, err := nodemaster.GetVar(nm, testKey)
	if err != nil {
		t.Errorf("Error getting var: %v", err)
	}

	if returnVal != testValue {
		t.Errorf("Expected %v, got %v", testValue, returnVal)
	}
}
