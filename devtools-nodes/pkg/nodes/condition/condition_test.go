package condition_test

import (
	"net/http"
	"testing"

	"github.com/DevToolsGit/devtools-nodes/pkg/model/mnode"
	"github.com/DevToolsGit/devtools-nodes/pkg/model/mnodemaster"
	"github.com/DevToolsGit/devtools-nodes/pkg/nodes/api"
	"github.com/DevToolsGit/devtools-nodes/pkg/nodes/condition"
)

func TestConditionRestStatus200(t *testing.T) {
	successNodeID := "node1"
	failNodeID := "node2"

	mn := mnodemaster.NodeMaster{
		Vars: map[string]interface{}{
			api.VarResponseKey: &http.Response{
				StatusCode: http.StatusOK,
			},
		},
		CurrentNode: &mnode.Node{
			Data: &condition.ConditionDataRestStatus{
				StatusCodeExits: map[string]string{
					"200": successNodeID,
					"404": failNodeID,
				},
			},
		},
	}
	err := condition.ConditionRestStatus(&mn)
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
			api.VarResponseKey: &http.Response{
				StatusCode: http.StatusNotFound,
			},
		},
		CurrentNode: &mnode.Node{
			Data: &condition.ConditionDataRestStatus{
				StatusCodeExits: map[string]string{
					"200": successNodeID,
					"404": failNodeID,
				},
			},
		},
	}
	err := condition.ConditionRestStatus(&mn)
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
			api.VarResponseKey: &http.Response{
				StatusCode: http.StatusNotFound,
			},
		},
		CurrentNode: &mnode.Node{
			Data: &condition.ConditionDataRestStatus{
				StatusCodeExits: map[string]string{
					"200": successNodeID,
					"4**": failNodeID,
				},
			},
		},
	}
	err := condition.ConditionRestStatus(&mn)
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
			api.VarResponseKey: &http.Response{
				StatusCode: http.StatusNotFound,
			},
		},
		CurrentNode: &mnode.Node{
			Data: &condition.ConditionDataRestStatus{
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
	err := condition.ConditionRestStatus(&mn)
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
			api.VarResponseKey: &http.Response{
				StatusCode: http.StatusNotFound,
			},
		},
		CurrentNode: &mnode.Node{
			Data: &condition.ConditionDataJsonMatch{
				Data: jsonByteArr,
				Path: "name.first",
				MatchExits: map[string]string{
					EgeNodeExpected:   TomasNode,
					TomasNodeExpected: EgeNode,
				},
			},
		},
	}

	err := condition.ConditionJsonMatch(&mn)
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
			api.VarResponseKey: &http.Response{
				StatusCode: http.StatusNotFound,
			},
		},
		CurrentNode: &mnode.Node{
			Data: &condition.ConditionDataJsonMatch{
				Data: jsonByteArr,
				Path: "name.first",
				MatchExits: map[string]string{
					EgeNodeExpected:   EgeNode,
					TomasNodeExpected: TomasNode,
				},
			},
		},
	}

	err := condition.ConditionJsonMatch(&mn)
	if err == nil {
		t.Errorf("expected error")
	}

	if mn.NextNodeID != "" {
		t.Errorf("unexpected next node id: %s", mn.NextNodeID)
	}
}
