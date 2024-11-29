package nodemaster_test

import (
	"context"
	"net/http"
	"testing"
	"the-dev-tools/nodes/pkg/httpclient/httpmockclient"
	"the-dev-tools/nodes/pkg/model/medge"
	"the-dev-tools/nodes/pkg/model/mnode"
	"the-dev-tools/nodes/pkg/model/mnodemaster"
	"the-dev-tools/nodes/pkg/model/mstatus"
	"the-dev-tools/nodes/pkg/nodemaster"
)

func MockResolver(nodeType string) (func(*mnodemaster.NodeMaster) error, error) {
	resolvedNodeFunc := func(nm *mnodemaster.NodeMaster) error {
		nm.NextNodeID = nm.CurrentNode.Edges.OutNodes["success"]
		return nil
	}
	return resolvedNodeFunc, nil
}

func TestNodeMasterRun(t *testing.T) {
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

	stateChan := make(chan mstatus.NodeStatus)
	mockHttpClient := httpmockclient.NewMockHttpClient(&http.Response{})
	nm, err := nodemaster.NewNodeMaster("start", nodes, MockResolver, nodemaster.ExecuteNode, stateChan, mockHttpClient)
	if err != nil {
		t.Errorf("Error creating NodeMaster: %v", err)
	}

	err = nodemaster.Run(nm, context.Background())
	if err != nil {
		t.Errorf("Error running NodeMaster: %v", err)
	}
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
