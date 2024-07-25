package nodecondition_test

import (
	"devtools-nodes/pkg/model/mnode"
	"devtools-nodes/pkg/model/mnodedata"
	"devtools-nodes/pkg/model/mnodemaster"
	"devtools-nodes/pkg/nodes/nodeapi"
	"devtools-nodes/pkg/nodes/nodecondition"
	"net/http"
	"testing"
)

func TestConditionRestStatus200(t *testing.T) {
	successNodeID := "node1"
	failNodeID := "node2"

	mn := mnodemaster.NodeMaster{
		Vars: map[string]interface{}{
			nodeapi.VarResponseKey: &http.Response{
				StatusCode: http.StatusOK,
			},
		},
		CurrentNode: &mnode.Node{
			Data: &mnodedata.NodeConditionRestStatusData{
				StatusCodeExits: map[string]string{
					"200": successNodeID,
					"404": failNodeID,
				},
			},
		},
	}
	err := nodecondition.ConditionRestStatus(&mn)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if mn.NextNodeID != successNodeID {
		t.Errorf("unexpected next node id: %s", mn.NextNodeID)
	}
}

func TestConditionRestStatus404(t *testing.T) {
	successNodeID := "node1"
	failNodeID := "node2"

	mn := mnodemaster.NodeMaster{
		Vars: map[string]interface{}{
			nodeapi.VarResponseKey: &http.Response{
				StatusCode: http.StatusNotFound,
			},
		},
		CurrentNode: &mnode.Node{
			Data: &mnodedata.NodeConditionRestStatusData{
				StatusCodeExits: map[string]string{
					"200": successNodeID,
					"404": failNodeID,
				},
			},
		},
	}
	err := nodecondition.ConditionRestStatus(&mn)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if mn.NextNodeID != failNodeID {
		t.Errorf("unexpected next node id: %s", mn.NextNodeID)
	}
}

func TestConditionRestStatusMatching(t *testing.T) {
	successNodeID := "node1"
	failNodeID := "node2"

	mn := mnodemaster.NodeMaster{
		Vars: map[string]interface{}{
			nodeapi.VarResponseKey: &http.Response{
				StatusCode: http.StatusNotFound,
			},
		},
		CurrentNode: &mnode.Node{
			Data: &mnodedata.NodeConditionRestStatusData{
				StatusCodeExits: map[string]string{
					"200": successNodeID,
					"4**": failNodeID,
				},
			},
		},
	}
	err := nodecondition.ConditionRestStatus(&mn)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if mn.NextNodeID != failNodeID {
		t.Errorf("unexpected next node id: %s", mn.NextNodeID)
	}
}

func TestConditionRestStatusMatchingMulti(t *testing.T) {
	testSuccessNodeID := "node1"
	testFailNodeID := "node2"

	mn := mnodemaster.NodeMaster{
		Vars: map[string]interface{}{
			nodeapi.VarResponseKey: &http.Response{
				StatusCode: http.StatusNotFound,
			},
		},
		CurrentNode: &mnode.Node{
			Data: &mnodedata.NodeConditionRestStatusData{
				StatusCodeExits: map[string]string{
					"2**": testFailNodeID,
					"300": testSuccessNodeID,
					"4**": testSuccessNodeID,
					"500": testFailNodeID,
					"6**": testFailNodeID,
				},
			},
		},
	}
	err := nodecondition.ConditionRestStatus(&mn)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if mn.NextNodeID != testSuccessNodeID {
		t.Errorf("unexpected next node id: %s", mn.NextNodeID)
	}
}

func TestConditionJsonMatch(t *testing.T) {
	jsonStr := `{"name":{"first":"Ege","last":"Tuzun"},"age":22}`
	jsonByteArr := []byte(jsonStr)

	EgeNode := "node1"
	TomasNode := "node2"

	EgeNodeExpected := "Ege"
	TomasNodeExpected := "Tomas"

	mn := mnodemaster.NodeMaster{
		Vars: map[string]interface{}{
			nodeapi.VarResponseKey: &http.Response{
				StatusCode: http.StatusNotFound,
			},
		},
		CurrentNode: &mnode.Node{
			Data: &mnodedata.NodeConditionJsonMatchData{
				Data: jsonByteArr,
				Path: "name.first",
				MatchExits: map[string]string{
					EgeNodeExpected:   TomasNode,
					TomasNodeExpected: EgeNode,
				},
			},
		},
	}

	err := nodecondition.ConditionJsonMatch(&mn)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConditionJsonMatchNoRoute(t *testing.T) {
	jsonStr := `{"name":{"first":"Hello","last":"World"},"age":22}`
	jsonByteArr := []byte(jsonStr)
	EgeNode := "node1"
	TomasNode := "node2"
	EgeNodeExpected := "Ege"
	TomasNodeExpected := "Tomas"
	mn := mnodemaster.NodeMaster{
		Vars: map[string]interface{}{
			nodeapi.VarResponseKey: &http.Response{
				StatusCode: http.StatusNotFound,
			},
		},
		CurrentNode: &mnode.Node{
			Data: &mnodedata.NodeConditionJsonMatchData{
				Data: jsonByteArr,
				Path: "name.first",
				MatchExits: map[string]string{
					EgeNodeExpected:   EgeNode,
					TomasNodeExpected: TomasNode,
				},
			},
		},
	}

	err := nodecondition.ConditionJsonMatch(&mn)
	if err == nil {
		t.Errorf("expected error")
	}

	if mn.NextNodeID != "" {
		t.Errorf("unexpected next node id: %s", mn.NextNodeID)
	}
}

func TestConditionExpression(t *testing.T) {
	successNodeID := "node1"
	failNodeID := "node2"

	successNodeKey := mnodemaster.EdgeSuccess
	failNodeKey := mnodemaster.EdgeFailure

	mn := mnodemaster.NodeMaster{
		Vars: map[string]interface{}{
			"test": 1,
		},
		CurrentNode: &mnode.Node{
			Data: &mnodedata.NodeConditionExpressionData{
				Expression: "test == 1",
				MatchExits: map[string]string{
					successNodeKey: successNodeID,
					failNodeKey:    failNodeID,
				},
			},
		},
	}
	err := nodecondition.ConditionExpression(&mn)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if mn.NextNodeID != "node1" {
		t.Errorf("unexpected next node id: %s", mn.NextNodeID)
	}
}

func TestConditionExpressionExit(t *testing.T) {
	successNodeID := "node1"
	failNodeID := "node2"

	mn := mnodemaster.NodeMaster{
		Vars: map[string]interface{}{
			"test": 1,
		},
		CurrentNode: &mnode.Node{
			Data: &mnodedata.NodeConditionExpressionData{
				Expression: "10 * 10",
				MatchExits: map[string]string{
					"100": successNodeID,
					"10":  failNodeID,
				},
			},
		},
	}
	err := nodecondition.ConditionExpression(&mn)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if mn.NextNodeID != "node1" {
		t.Errorf("unexpected next node id: %s", mn.NextNodeID)
	}
}
