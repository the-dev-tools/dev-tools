package main

import (
	"context"
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

	data := &nodedatav1.ApiCallData{
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
	}

	nm := &nodemasterv1.NodeMasterServiceRunRequest{
		Id:          "123",
		StartNodeId: "Node1",
		Nodes:       map[string]*nodemasterv1.Node{"Node1": &node},
		Vars:        map[string]*anypb.Any{"var1": anyData},
	}

	req := connect.NewRequest(nm)
	resp, err := client.Run(context.Background(), req)
	if err != nil {
		log.Fatalf("failed to run node master: %v", err)
	}

	fmt.Println(resp)
}
