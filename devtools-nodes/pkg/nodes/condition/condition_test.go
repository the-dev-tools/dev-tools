package condition_test

import (
	"net/http"
	"testing"

	"github.com/DevToolsGit/devtools-nodes/pkg/model/mnode"
	"github.com/DevToolsGit/devtools-nodes/pkg/model/mnodemaster"
	"github.com/DevToolsGit/devtools-nodes/pkg/nodes/api"
	"github.com/DevToolsGit/devtools-nodes/pkg/nodes/condition"
)

func TestConditionRestStatus(t *testing.T) {
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
	if mn.NextNodeID != "node1" {
		t.Errorf("unexpected next node id: %s", mn.NextNodeID)
	}
}
