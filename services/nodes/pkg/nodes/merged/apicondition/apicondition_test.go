package apicondition_test

import (
	"net/http"
	"testing"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mexampleheader"
	"the-dev-tools/backend/pkg/model/mexamplequery"
	"the-dev-tools/nodes/pkg/httpclient/httpmockclient"
	"the-dev-tools/nodes/pkg/model/mnode"
	"the-dev-tools/nodes/pkg/model/mnodedata"
	"the-dev-tools/nodes/pkg/model/mnodemaster"
	"the-dev-tools/nodes/pkg/nodes/merged/apicondition"
	api "the-dev-tools/nodes/pkg/nodes/nodeapi"
)

func TestConditionRestStatus200(t *testing.T) {
	successNodeID := "node1"
	failNodeID := "node2"

	apiconditionData := &apicondition.ConditionDataRestStatus{
		ApiData: &mnodedata.NodeApiRestData{
			Method: "GET",
			Url:    "http://localhost:8080",
			Headers: []mexampleheader.Header{
				{ID: idwrap.NewNow(), HeaderKey: "Content-Type", Value: "application/json"},
			},
			Query: []mexamplequery.Query{
				{ID: idwrap.NewNow(), QueryKey: "key", Value: "value"},
			},
			Body: []byte{},
		},
		ConditionData: &mnodedata.NodeConditionRestStatusData{
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
