package main

import (
	"context"
	"devtools-nodes/pkg/model/medge"
	"devtools-nodes/pkg/resolver"
	nodedatav1 "devtools-services/gen/nodedata/v1"
	nodemasterv1 "devtools-services/gen/nodemaster/v1"
	"devtools-services/gen/nodemaster/v1/nodemasterv1connect"
	"flag"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"connectrpc.com/connect"
	"github.com/bufbuild/httplb"
	"google.golang.org/protobuf/types/known/anypb"
)

func main() {
	addr := flag.String("addr", "http://localhost:8080", "address of the node master service")
	times := flag.Int("times", 1, "number of times to run the node master")
	thread := flag.Int("thread", 10, "number of times to run the node master")

	flag.Parse()

	if addr == nil {
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
		Url:         "https://api.keepitdev.com",
		Method:      "GET",
		QueryParams: map[string]string{"param1": "value1"},
		Headers:     map[string]string{"header1": "value1"},
		Body:        []byte("body"),
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

func GetHttpClient() *http.Client {
	return http.DefaultClient
}
