package apicondition_test

import (
	"devtools-nodes/pkg/httpclient/httpmockclient"
	"devtools-nodes/pkg/model/mnode"
	"devtools-nodes/pkg/model/mnodemaster"
	"devtools-nodes/pkg/nodes/api"
	"devtools-nodes/pkg/nodes/condition"
	"devtools-nodes/pkg/nodes/merged/apicondition"
	"net/http"
	"testing"
)

func TestConditionRestStatus200(t *testing.T) {
	successNodeID := "node1"
	failNodeID := "node2"

	apiconditionData := &apicondition.ConditionDataRestStatus{
		ApiData: &api.RestApiData{
			Method:  "GET",
			Url:     "http://localhost:8080",
			Headers: map[string]string{},
			Body:    []byte{},
		},
		ConditionData: &condition.ConditionDataRestStatus{
			StatusCodeExits: map[string]string{
				"200": successNodeID,
				"404": failNodeID,
			},
		},
	}

	nodes := map[string]mnode.Node{
		successNodeID: {},
		failNodeID:    {},
	}

	mn := mnodemaster.NodeMaster{
		HttpClient: &httpmockclient.MockHttpClient{
			ReturnResponse: &http.Response{
				StatusCode: http.StatusOK,
			},
		},
		Vars: map[string]interface{}{
			api.VarResponseKey: &http.Response{
				StatusCode: http.StatusOK,
			},
		},
		Nodes: nodes,

		CurrentNode: &mnode.Node{
			Data: apiconditionData,
		},
	}
	err := apicondition.ApiConditionRestStatus(&mn)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if mn.NextNodeID != successNodeID {
		t.Errorf("unexpected next node id: %s", mn.NextNodeID)
	}
}
