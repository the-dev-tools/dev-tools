package main

import (
	"context"
	"dev-tools-mail/pkg/emailclient"
	"dev-tools-mail/pkg/emailinvite"
	"dev-tools-nodes/pkg/model/medge"
	"dev-tools-nodes/pkg/resolver"
	authv1 "dev-tools-services/gen/auth/v1"
	"dev-tools-services/gen/auth/v1/authv1connect"
	collectionv1 "dev-tools-services/gen/collection/v1"
	"dev-tools-services/gen/collection/v1/collectionv1connect"
	nodedatav1 "dev-tools-services/gen/nodedata/v1"
	nodemasterv1 "dev-tools-services/gen/nodemaster/v1"
	"dev-tools-services/gen/nodemaster/v1/nodemasterv1connect"
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
	NodeFunc          = "node"
	AuthFunc          = "auth"
	PostmanImportFunc = "import"
	CollectionFunc    = "collection"
	RunApiFunc        = "runapi"
	EmailFunc         = "email"
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
	case PostmanImportFunc:
		PostManCollection()
	case CollectionFunc:
		GetCollection()
	case RunApiFunc:
		RunApi()
	case EmailFunc:
		RunEmail()
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
		SlaveHttpEndpoint: "h2c://dev-tools-slavenode.flycast",
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

func PostManCollection() {
	addr := flag.String("addr", "", "address of the node master service")

	flag.Parse()

	ctx := context.Background()

	/*
		createReqRaw := &collectionv1.CreateCollectionRequest{
			Name: "test",
		}
	*/

	// createReq := connect.NewRequest(createReqRaw)

	httpClient := httplb.NewClient(httplb.WithDefaultTimeout(time.Hour))
	client := collectionv1connect.NewCollectionServiceClient(httpClient, *addr)

	/*
		createResp, err := client.CreateCollection(ctx, createReq)
		if err != nil {
			log.Fatalf("service returns error: %v", err)
		}
	*/

	data, err := os.ReadFile("/home/electwix/dev/work/devtools-go-mono/dev-tools-cli/postman.json")
	if err != nil {
		log.Fatalf("failed to open file: %v", err)
	}

	req := &connect.Request[collectionv1.ImportPostmanRequest]{
		Msg: &collectionv1.ImportPostmanRequest{
			Name: "test",
			Data: data,
		},
	}

	resp, err := client.ImportPostman(ctx, req)
	if err != nil {
		log.Fatalf("service returns error: %v", err)
	}

	fmt.Println("Create Response: ", resp.Msg.Id)
}

func GetCollection() {
	addr := flag.String("addr", "", "address of the node master service")
	flag.Parse()

	ctx := context.Background()

	httpClient := httplb.NewClient(httplb.WithDefaultTimeout(time.Hour))
	client := collectionv1connect.NewCollectionServiceClient(httpClient, *addr)

	reqList := connect.NewRequest(&collectionv1.ListCollectionsRequest{})
	respList, err := client.ListCollections(ctx, reqList)
	if err != nil {
		log.Fatalf("service returns error: %v", err)
	}

	fmt.Println("List Response: ", respList.Msg)

	req := connect.NewRequest(&collectionv1.GetCollectionRequest{
		Id: "01J4M6YJVZEM4KMF9JZQG3XRGP",
	})

	resp, err := client.GetCollection(ctx, req)
	if err != nil {
		log.Fatalf("service returns error: %v", err)
	}

	fmt.Println("Got Response: ", resp.Msg.Id)
	for _, item := range resp.Msg.Items {
		RecursivePrint(item)
	}
}

// recursive print
func RecursivePrint(item *collectionv1.Item) {
	// Get The childs
	api := item.GetApiCall()
	if api != nil {
		fmt.Println(api.ParentId)
		fmt.Println(api.Meta.Id, api.Meta.Name, api.Data.Url)
	}
	folder := item.GetFolder()
	if folder != nil {
		fmt.Println("Folder: ", folder.Meta.Name)
		for _, item := range folder.Items {
			RecursivePrint(item)
		}
	}
	// Get Data

	data := item.GetData()
	if data == nil {
		return
	}
}

func RunApi() {
	addr := flag.String("addr", "", "address of the node master service")
	flag.Parse()

	ctx := context.Background()

	httpClient := httplb.NewClient(httplb.WithDefaultTimeout(time.Hour))
	client := collectionv1connect.NewCollectionServiceClient(httpClient, *addr)

	reqRun := connect.NewRequest(&collectionv1.RunApiCallRequest{
		Id: "01J4M6YJXY9NYH7KA5FQKDTPFV",
	})

	resp, err := client.RunApiCall(ctx, reqRun)
	if err != nil {
		log.Fatalf("service returns error: %v", err)
	}

	fmt.Println("Run Response: ", resp.Msg)
}

func RunEmail() {
	AWS_ACCESS_KEY := os.Getenv("AWS_ACCESS_KEY")
	if AWS_ACCESS_KEY == "" {
		log.Fatalf("AWS_ACCESS_KEY is empty")
	}
	AWS_SECRET_KEY := os.Getenv("AWS_SECRET_KEY")
	if AWS_SECRET_KEY == "" {
		log.Fatalf("AWS_SECRET_KEY is empty")
	}

	client, err := emailclient.NewClient(AWS_ACCESS_KEY, AWS_SECRET_KEY, "")
	if err != nil {
		log.Fatalf("failed to create email client: %v", err)
	}

	err = emailinvite.SendEmailInvite(context.Background(), *client, "ege@dev.tools", "http://localhost:8080")
	if err != nil {
		log.Fatalf("failed to send email: %v", err)
	}
}
