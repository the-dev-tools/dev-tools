package main

import (
	"context"
	"devtools-nodes/pkg/model/medge"
	"devtools-nodes/pkg/resolver"
	nodedatav1 "devtools-services/gen/nodedata/v1"
	nodemasterv1 "devtools-services/gen/nodemaster/v1"
	"devtools-services/gen/nodemaster/v1/nodemasterv1connect"
	"fmt"
	"log"
	"net/http"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/anypb"
)

func main() {
	client := nodemasterv1connect.NewNodeMasterServiceClient(http.DefaultClient, "http://localhost:8080")

	data := &nodedatav1.NodeApiCallData{
		Url:         "http://google.com",
		Method:      "GET",
		QueryParams: map[string]string{"param1": "value1"},
		Headers:     map[string]string{"header1": "value1"},
		Body:        []byte("body"),
	}

	anyData, err := anypb.New(data)
	if err != nil {
		log.Fatalf("failed to create anypb: %v", err)
	}

	node := nodemasterv1.Node{
		Id:      "node1",
		Type:    resolver.ApiCallRest,
		OwnerId: "someid",
		Data:    anyData,
		GroupId: "someid",
		Edges: &nodemasterv1.Edges{
			OutNodes: map[string]string{medge.DefaultSuccessEdge: "node2"},
		},
	}

	node2 := nodemasterv1.Node{
		Id:      "node2",
		Type:    resolver.ApiCallRest,
		OwnerId: "someid",
		Data:    anyData,
		GroupId: "someid",
		Edges:   &nodemasterv1.Edges{},
	}

	nm := &nodemasterv1.NodeMasterServiceRunRequest{
		Id:          "123",
		StartNodeId: node.Id,
		Nodes:       map[string]*nodemasterv1.Node{node.Id: &node, node2.Id: &node2},
		Vars:        map[string]*anypb.Any{"var1": anyData},
	}

	req := connect.NewRequest(nm)
	resp, err := client.Run(context.Background(), req)
	if err != nil {
		log.Fatalf("failed to run node master: %v", err)
	}

	fmt.Println(resp)
}
