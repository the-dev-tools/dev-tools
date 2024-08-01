package main

import (
	"context"
	"devtools-backend/pkg/model/mcollection"
	"devtools-nodes/pkg/model/medge"
	"devtools-nodes/pkg/resolver"
	authv1 "devtools-services/gen/auth/v1"
	"devtools-services/gen/auth/v1/authv1connect"
	collectionv1 "devtools-services/gen/collection/v1"
	"devtools-services/gen/collection/v1/collectionv1connect"
	nodedatav1 "devtools-services/gen/nodedata/v1"
	nodemasterv1 "devtools-services/gen/nodemaster/v1"
	"devtools-services/gen/nodemaster/v1/nodemasterv1connect"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"connectrpc.com/connect"
	"github.com/bufbuild/httplb"
	"google.golang.org/protobuf/types/known/anypb"
)

const (
	NodeFunc       = "node"
	AuthFunc       = "auth"
	CollectionFunc = "collection"
)

func main() {
	fmt.Println("Starting cli")

	lastArg := len(os.Args) - 1
	cmd := os.Args[lastArg]

	switch cmd {
	case NodeFunc:
		NodeFuncHandler()
	case AuthFunc:
		AuthFuncHandler()
	case CollectionFunc:
		Collection()
	default:
		fmt.Println("Invalid function", cmd)
	}
}

func GetHttpClient() *http.Client {
	return http.DefaultClient
}

func AuthFuncHandler() {
	addr := flag.String("addr", "http://localhost:8080", "address of the node master service")
	token := flag.String("token", "", "token for the request")
	flag.Parse()

	fmt.Println("Address: ", *addr)
	fmt.Println("Token: ", *token)

	if *token == "" {
		log.Fatalf("failed to get token")
	}

	reqRaw := &authv1.AuthServiceDIDRequest{
		DidToken: *token,
	}

	req := connect.NewRequest(reqRaw)

	httpClient := httplb.NewClient(httplb.WithDefaultTimeout(time.Hour))
	client := authv1connect.NewAuthServiceClient(httpClient, *addr)
	ctx := context.Background()
	resp, err := client.DID(ctx, req)
	if err != nil {
		log.Fatalf("service returns error: %v", err)
	}

	if resp == nil {
		log.Fatalf("failed to get response")
	}

	fmt.Println("Response: ", resp)
	fmt.Println("AccessToken: ", resp.Msg.AccessToken)
	fmt.Println("RefreshToken: ", resp.Msg.RefreshToken)
}

func NodeFuncHandler() {
	addr := flag.String("addr", "", "address of the node master service")
	times := flag.Int("times", 1, "number of times to run the node master")
	thread := flag.Int("thread", 10, "number of times to run the node master")

	flag.Parse()

	if *addr == "" {
		log.Fatalf("failed to get address")
	}

	fmt.Println("Address: ", *addr)
	fmt.Println("Times: ", *times)
	fmt.Println("Thread: ", *thread)

	loopData := &nodedatav1.NodeForRemote{
		Count:             25,
		LoopStartNode:     "node1",
		MachineEmount:     15,
		SlaveHttpEndpoint: "h2c://devtools-slavenode.flycast",
	}

	loopDataMsg, err := anypb.New(loopData)
	if err != nil {
		log.Fatalf("failed to create anypb: %v", err)
	}

	nodeForRemote := nodemasterv1.Node{
		Id:      "nodeLooper",
		Type:    resolver.NodeTypeLoopRemote,
		OwnerId: "someid",
		Data:    loopDataMsg,
		GroupId: "someid",
		Edges:   &nodemasterv1.Edges{},
	}

	apiCallData := &nodedatav1.NodeApiCallData{
		Url:         "https://8bde-81-214-83-129.ngrok-free.app/",
		Method:      "POST",
		QueryParams: map[string]string{"param1": "value1"},
		Headers:     map[string]string{"header1": "value1"},
		Body:        []byte("start_stop=true"),
	}

	apiCallDataMsg, err := anypb.New(apiCallData)
	if err != nil {
		log.Fatalf("failed to create anypb: %v", err)
	}

	node := nodemasterv1.Node{
		Id:      "node1",
		Type:    resolver.ApiCallRest,
		OwnerId: "someid",
		Data:    apiCallDataMsg,
		GroupId: "someid",
		Edges: &nodemasterv1.Edges{
			OutNodes: map[string]string{medge.DefaultSuccessEdge: "node2"},
		},
	}

	node2 := nodemasterv1.Node{
		Id:      "node2",
		Type:    resolver.ApiCallRest,
		OwnerId: "someid",
		Data:    apiCallDataMsg,
		GroupId: "someid",
		Edges:   &nodemasterv1.Edges{},
	}

	nm := &nodemasterv1.NodeMasterServiceRunRequest{
		Id:          "123",
		StartNodeId: nodeForRemote.Id,
		Nodes:       map[string]*nodemasterv1.Node{nodeForRemote.Id: &nodeForRemote, node.Id: &node, node2.Id: &node2},
		Vars:        map[string]*anypb.Any{"var1": apiCallDataMsg},
	}

	start := time.Now()

	var ops atomic.Uint64
	var execute atomic.Uint64

	wg := sync.WaitGroup{}
	for i := 0; i < *thread; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < *times; i++ {
				fmt.Println("sent request", i)
				requestTime := time.Now()

				httpClient := httplb.NewClient(httplb.WithDefaultTimeout(time.Hour))
				client := nodemasterv1connect.NewNodeMasterServiceClient(httpClient, *addr)
				req := connect.NewRequest(nm)
				stream, err := client.Run(context.Background(), req)
				if err != nil {
					log.Fatalf("failed to run node master: %v", err)
				}
				defer stream.Close()
				for stream.Receive() {
					ops.Add(1)
					msg := stream.Msg()
					fmt.Println("Response: ", msg)

				}
				if err := stream.Err(); err != nil {
					take := time.Since(start)
					requestTake := time.Since(requestTime)
					fmt.Println("Time taken: ", take)
					fmt.Println("Request Time taken: ", requestTake)
					log.Fatalf("failed to receive stream: %v", stream.Err())
				}
				execute.Add(1)
			}
		}()
	}
	wg.Wait()

	fmt.Println("Ops: ", ops.Load())
	fmt.Println("Execute: ", execute.Load())

	take := time.Since(start)
	fmt.Println("Time taken: ", take)

	fmt.Println("Done")
}

func Collection() {
	addr := flag.String("addr", "", "address of the node master service")

	flag.Parse()

	ctx := context.Background()

	createReqRaw := &collectionv1.CreateRequest{
		Name: "test",
	}

	createReq := connect.NewRequest(createReqRaw)

	httpClient := httplb.NewClient(httplb.WithDefaultTimeout(time.Hour))
	client := collectionv1connect.NewCollectionServiceClient(httpClient, *addr)
	createResp, err := client.Create(ctx, createReq)
	if err != nil {
		log.Fatalf("service returns error: %v", err)
	}

	loadReqRaw := &collectionv1.LoadRequest{
		Id: createResp.Msg.Id,
	}

	var parentID string = "parent"

	// Add node to collection
	createNodeReqRaw := &collectionv1.CollectionNode{
		Id:           "node1",
		CollectionId: createResp.Msg.Id,
		Name:         "test",
		Type:         mcollection.CollectionNodeTypeRequest,
		ParentId:     &parentID,
		Data: &nodedatav1.NodeApiCallData{
			Method:      "GET",
			Url:         "https://google.com/",
			Headers:     map[string]string{"header1": "value1"},
			Body:        []byte("start_stop=true"),
			QueryParams: map[string]string{"param1": "value1"},
		},
	}

	saveReqRaw := &collectionv1.SaveRequest{
		Id:    createResp.Msg.Id,
		Name:  "test",
		Nodes: []*collectionv1.CollectionNode{createNodeReqRaw},
	}

	saveReq := connect.NewRequest(saveReqRaw)

	_, err = client.Save(ctx, saveReq)
	if err != nil {
		log.Fatalf("service returns error: %v", err)
	}

	loadReq := connect.NewRequest(loadReqRaw)
	loadResp, err := client.Load(ctx, loadReq)
	if err != nil {
		log.Fatalf("service returns error: %v", err)
	}

	fmt.Println("Response: ", loadResp)
}
