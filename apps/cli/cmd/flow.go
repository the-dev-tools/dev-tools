package cmd

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/db/pkg/tursomem"
	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/node/nfor"
	"the-dev-tools/server/pkg/flow/node/nforeach"
	"the-dev-tools/server/pkg/flow/node/nif"
	"the-dev-tools/server/pkg/flow/node/njs"
	"the-dev-tools/server/pkg/flow/node/nnoop"
	"the-dev-tools/server/pkg/flow/node/nrequest"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/flow/runner/flowlocalrunner"
	"the-dev-tools/server/pkg/http/request"
	"the-dev-tools/server/pkg/httpclient"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/ioworkspace"
	"the-dev-tools/server/pkg/logconsole"
	"the-dev-tools/server/pkg/model/mexampleresp"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"the-dev-tools/server/pkg/model/mnnode/mnforeach"
	"the-dev-tools/server/pkg/model/mnnode/mnif"
	"the-dev-tools/server/pkg/model/mnnode/mnjs"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/service/flow/sedge"
	"the-dev-tools/server/pkg/service/sassert"
	"the-dev-tools/server/pkg/service/sassertres"
	"the-dev-tools/server/pkg/service/sbodyform"
	"the-dev-tools/server/pkg/service/sbodyraw"
	"the-dev-tools/server/pkg/service/sbodyurl"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/sexampleheader"
	"the-dev-tools/server/pkg/service/sexamplequery"
	"the-dev-tools/server/pkg/service/sexampleresp"
	"the-dev-tools/server/pkg/service/sexamplerespheader"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/service/sitemfolder"
	"the-dev-tools/server/pkg/service/snode"
	"the-dev-tools/server/pkg/service/snodefor"
	"the-dev-tools/server/pkg/service/snodeforeach"
	"the-dev-tools/server/pkg/service/snodeif"
	"the-dev-tools/server/pkg/service/snodejs"
	"the-dev-tools/server/pkg/service/snodenoop"
	"the-dev-tools/server/pkg/service/snoderequest"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/spec/dist/buf/go/nodejs_executor/v1/nodejs_executorv1connect"
	"time"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
)

type FLowServiceLocal struct {
	DB *sql.DB
	ws sworkspace.WorkspaceService

	// flow
	fs  sflow.FlowService
	fes sedge.EdgeService

	// request
	ias sitemapi.ItemApiService
	es  sitemapiexample.ItemApiExampleService
	qs  sexamplequery.ExampleQueryService
	hs  sexampleheader.HeaderService

	// body
	brs  sbodyraw.BodyRawService
	bfs  sbodyform.BodyFormService
	bues sbodyurl.BodyURLEncodedService

	// response
	ers  sexampleresp.ExampleRespService
	erhs sexamplerespheader.ExampleRespHeaderService
	as   sassert.AssertService
	ars  sassertres.AssertResultService

	// sub nodes
	ns   snode.NodeService
	rns  snoderequest.NodeRequestService
	fns  snodefor.NodeForService
	fens snodeforeach.NodeForEachService
	sns  snodenoop.NodeNoopService
	ins  snodeif.NodeIfService
	jsns snodejs.NodeJSService

	logChanMap logconsole.LogChanMap
}

func init() {
	rootCmd.AddCommand(flowCmd)
	flowCmd.AddCommand(flowRunCmd)
}

var flowCmd = &cobra.Command{
	Use:   "flow",
	Short: "Workspace Flow Controls",
	Long:  `Workspace Flow Controls`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var flowRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run Flow",
	Long:  `Running Flow`,
	RunE: func(cmd *cobra.Command, args []string) error {

		ctx := cmd.Context()

		fileData, err := os.ReadFile(workspaceFilePath)
		if err != nil {
			return err
		}

		workspaceData, err := ioworkspace.UnmarshalWorkspace(fileData)
		if err != nil {
			return err
		}

		db, _, err := tursomem.NewTursoLocal(ctx)
		if err != nil {
			return err
		}

		queries, err := gen.Prepare(ctx, db)
		if err != nil {
			return err
		}

		workspaceService := sworkspace.New(queries)
		collectionService := scollection.New(queries)
		folderService := sitemfolder.New(queries)
		endpointService := sitemapi.New(queries)
		exampleService := sitemapiexample.New(queries)
		exampleHeaderService := sexampleheader.New(queries)
		exampleQueryService := sexamplequery.New(queries)
		exampleAssertService := sassert.New(queries)
		rawBodyService := sbodyraw.New(queries)
		formBodyService := sbodyform.New(queries)
		urlBodyService := sbodyurl.New(queries)
		responseService := sexampleresp.New(queries)
		responseHeaderService := sexamplerespheader.New(queries)
		responseAssertService := sassertres.New(queries)
		flowService := sflow.New(queries)
		flowNodeService := snode.New(queries)
		flowRequestService := snoderequest.New(queries)
		flowConditionService := snodeif.New(queries)
		flowNoopService := snodenoop.New(queries)
		flowEdgeService := sedge.New(queries)
		flowForService := snodefor.New(queries)
		flowForEachService := snodeforeach.New(queries)
		flowJSService := snodejs.New(queries)
		flowEdges := sedge.New(queries)

		ioWorkspaceService := ioworkspace.NewIOWorkspaceService(
			db,
			workspaceService,
			collectionService,
			folderService,
			endpointService,
			exampleService,
			exampleHeaderService,
			exampleQueryService,
			exampleAssertService,
			rawBodyService,
			formBodyService,
			urlBodyService,
			responseService,
			responseHeaderService,
			responseAssertService,
			flowService,
			flowNodeService,
			flowEdgeService,
			flowRequestService,
			*flowConditionService,
			flowNoopService,
			flowForService,
			flowForEachService,
			flowJSService,
		)

		logMap := logconsole.NewLogChanMap()

		flowServiceLocal := FLowServiceLocal{
			DB:         db,
			ws:         workspaceService,
			fs:         flowService,
			fes:        flowEdges,
			ias:        endpointService,
			es:         exampleService,
			qs:         exampleQueryService,
			hs:         exampleHeaderService,
			brs:        rawBodyService,
			bfs:        formBodyService,
			bues:       urlBodyService,
			ers:        responseService,
			erhs:       responseHeaderService,
			as:         exampleAssertService,
			ars:        responseAssertService,
			ns:         flowNodeService,
			rns:        flowRequestService,
			fns:        flowForService,
			fens:       flowForEachService,
			sns:        flowNoopService,
			ins:        *flowConditionService,
			jsns:       flowJSService,
			logChanMap: logMap,
		}

		// TODO: move to const
		workspaceID := workspaceData.Workspace.ID
		c := flowServiceLocal

		err = ioWorkspaceService.ImportWorkspace(ctx, *workspaceData)
		if err != nil {
			return err
		}

		if len(args) < 1 {
			return errors.New("args should be more then 0")
		}

		nameOrID := args[0]

		var flowPtr *mflow.Flow
		// check if id
		id, err := idwrap.NewText(nameOrID)
		if err != nil {
			// seems like not id try find name
			flows, err := c.fs.GetFlowsByWorkspaceID(ctx, workspaceID)
			if err != nil {
				return err
			}
			for _, flow := range flows {
				if nameOrID == flow.Name {
					flowPtr = &flow
				}
			}

			if flowPtr == nil {
				return fmt.Errorf("%s didn't match any flow names", nameOrID)
			}

		} else {
			flow, err := c.fs.GetFlow(ctx, id)
			if err != nil {
				return err
			}
			flowPtr = &flow
		}

		log.Println("found flow", flowPtr.Name)
		return flowRun(ctx, flowPtr, c)
	},
}

func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%.2fµs", float64(d.Nanoseconds())/1000)
	} else if d < time.Second {
		return fmt.Sprintf("%.2fms", float64(d.Nanoseconds())/1000000)
	} else if d < time.Minute {
		return fmt.Sprintf("%.2fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.2fm", d.Minutes())
	}
	return fmt.Sprintf("%.2fh", d.Hours())
}

func flowRun(ctx context.Context, flowPtr *mflow.Flow, c FLowServiceLocal) error {
	latestFlowID := flowPtr.ID

	nodes, err := c.ns.GetNodesByFlowID(ctx, latestFlowID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, errors.New("get nodes"))
	}

	edges, err := c.fes.GetEdgesByFlowID(ctx, latestFlowID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, errors.New("get edges"))
	}
	edgeMap := edge.NewEdgesMap(edges)

	var requestNodes []mnrequest.MNRequest
	var forNodes []mnfor.MNFor
	var forEachNodes []mnforeach.MNForEach
	var ifNodes []mnif.MNIF
	var noopNodes []mnnoop.NoopNode
	var jsNodes []mnjs.MNJS
	var startNodeID idwrap.IDWrap

	nodeNameMap := make(map[idwrap.IDWrap]string, len(nodes))

	for _, node := range nodes {
		nodeNameMap[node.ID] = node.Name

		switch node.NodeKind {
		case mnnode.NODE_KIND_REQUEST:
			rn, err := c.rns.GetNodeRequest(ctx, node.ID)
			if err != nil {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("get node request: %w", err))
			}
			requestNodes = append(requestNodes, *rn)
		case mnnode.NODE_KIND_FOR:
			fn, err := c.fns.GetNodeFor(ctx, node.ID)
			if err != nil {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("get node for: %w", err))
			}
			forNodes = append(forNodes, *fn)
		case mnnode.NODE_KIND_FOR_EACH:
			fen, err := c.fens.GetNodeForEach(ctx, node.ID)
			if err != nil {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("get node for each: %w", err))
			}
			forEachNodes = append(forEachNodes, *fen)
		case mnnode.NODE_KIND_NO_OP:
			sn, err := c.sns.GetNodeNoop(ctx, node.ID)
			if err != nil {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("get node start: %w", err))
			}
			noopNodes = append(noopNodes, *sn)
		case mnnode.NODE_KIND_CONDITION:
			in, err := c.ins.GetNodeIf(ctx, node.ID)
			if err != nil {
				return connect.NewError(connect.CodeInternal, errors.New("get node if"))
			}
			ifNodes = append(ifNodes, *in)
		case mnnode.NODE_KIND_JS:
			jsn, err := c.jsns.GetNodeJS(ctx, node.ID)
			if err != nil {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("get node js: %w", err))
			}
			jsNodes = append(jsNodes, jsn)
		default:
			return connect.NewError(connect.CodeInternal, errors.New("not supported node"))
		}
	}

	var foundStartNode bool
	for _, node := range noopNodes {
		if node.Type == mnnoop.NODE_NO_OP_KIND_START {
			if foundStartNode {
				return connect.NewError(connect.CodeInternal, errors.New("multiple start nodes"))
			}
			foundStartNode = true
			startNodeID = node.FlowNodeID
		}
	}
	if !foundStartNode {
		return connect.NewError(connect.CodeInternal, errors.New("no start node"))
	}

	flowNodeMap := make(map[idwrap.IDWrap]node.FlowNode, 0)
	for _, forNode := range forNodes {
		name := nodeNameMap[forNode.FlowNodeID]
		flowNodeMap[forNode.FlowNodeID] = nfor.New(forNode.FlowNodeID, name, forNode.IterCount, time.Second)
	}

	requestNodeRespChan := make(chan nrequest.NodeRequestSideResp, len(requestNodes))
	for _, requestNode := range requestNodes {

		// Base Request
		if requestNode.EndpointID == nil || requestNode.ExampleID == nil {
			return connect.NewError(connect.CodeInternal, fmt.Errorf("endpoint or example not found for %s", requestNode.FlowNodeID))
		}
		endpoint, err := c.ias.GetItemApi(ctx, *requestNode.EndpointID)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}

		example, err := c.es.GetApiExample(ctx, *requestNode.ExampleID)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}

		if example.ItemApiID != endpoint.ID {
			return connect.NewError(connect.CodeInternal, errors.New("example and endpoint not match"))
		}
		headers, err := c.hs.GetHeaderByExampleID(ctx, example.ID)
		if err != nil {
			return connect.NewError(connect.CodeInternal, errors.New("get headers"))
		}
		queries, err := c.qs.GetExampleQueriesByExampleID(ctx, example.ID)
		if err != nil {
			return connect.NewError(connect.CodeInternal, errors.New("get queries"))
		}

		rawBody, err := c.brs.GetBodyRawByExampleID(ctx, example.ID)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}

		formBody, err := c.bfs.GetBodyFormsByExampleID(ctx, example.ID)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}

		urlBody, err := c.bues.GetBodyURLEncodedByExampleID(ctx, example.ID)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}

		exampleResp, err := c.ers.GetExampleRespByExampleID(ctx, example.ID)
		if err != nil {
			if err == sexampleresp.ErrNoRespFound {
				exampleResp = &mexampleresp.ExampleResp{
					ID:        idwrap.NewNow(),
					ExampleID: example.ID,
				}
				err = c.ers.CreateExampleResp(ctx, *exampleResp)
				if err != nil {
					return connect.NewError(connect.CodeInternal, errors.New("create example resp"))
				}
			} else {
				return connect.NewError(connect.CodeInternal, err)
			}
		}

		exampleRespHeader, err := c.erhs.GetHeaderByRespID(ctx, exampleResp.ID)
		if err != nil {
			return connect.NewError(connect.CodeInternal, errors.New("get example resp header"))
		}

		asserts, err := c.as.GetAssertByExampleID(ctx, example.ID)
		if err != nil && err != sassert.ErrNoAssertFound {
			return connect.NewError(connect.CodeInternal, err)
		}

		// Delta Request
		if requestNode.DeltaExampleID != nil {
			deltaExample, err := c.es.GetApiExample(ctx, *requestNode.DeltaExampleID)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}

			deltaHeaders, err := c.hs.GetHeaderByExampleID(ctx, deltaExample.ID)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}

			deltaQueries, err := c.qs.GetExampleQueriesByExampleID(ctx, deltaExample.ID)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}

			rawBodyDelta, err := c.brs.GetBodyRawByExampleID(ctx, deltaExample.ID)
			if err != nil {
				return connect.NewError(connect.CodeInternal, errors.New("delta raw body not found"))
			}

			formBodyDelta, err := c.bfs.GetBodyFormsByExampleID(ctx, deltaExample.ID)
			if err != nil {
				return connect.NewError(connect.CodeInternal, errors.New("delta form body not found"))
			}

			urlBodyDelta, err := c.bues.GetBodyURLEncodedByExampleID(ctx, deltaExample.ID)
			if err != nil {
				return connect.NewError(connect.CodeInternal, errors.New("delta url body not found"))
			}

			mergeExamplesInput := request.MergeExamplesInput{
				Base:  *example,
				Delta: *deltaExample,

				BaseQueries:  queries,
				DeltaQueries: deltaQueries,

				BaseHeaders:  headers,
				DeltaHeaders: deltaHeaders,

				BaseRawBody:  *rawBody,
				DeltaRawBody: *rawBodyDelta,

				BaseFormBody:  formBody,
				DeltaFormBody: formBodyDelta,

				BaseUrlEncodedBody:  urlBody,
				DeltaUrlEncodedBody: urlBodyDelta,
			}

			mergeExampleOutput := request.MergeExamples(mergeExamplesInput)
			example = &mergeExampleOutput.Merged

			headers = mergeExampleOutput.MergeHeaders
			queries = mergeExampleOutput.MergeQueries

			rawBody = &mergeExampleOutput.MergeRawBody
			formBody = mergeExampleOutput.MergeFormBody
			urlBody = mergeExampleOutput.MergeUrlEncodedBody
		}

		httpClient := httpclient.New()

		name := nodeNameMap[requestNode.FlowNodeID]

		flowNodeMap[requestNode.FlowNodeID] = nrequest.New(requestNode.FlowNodeID, name, *endpoint, *example, queries, headers, *rawBody, formBody, urlBody,
			*exampleResp, exampleRespHeader, asserts, httpClient, requestNodeRespChan)
	}

	for _, ifNode := range ifNodes {
		comp := ifNode.Condition.Comparisons
		name := nodeNameMap[ifNode.FlowNodeID]
		flowNodeMap[ifNode.FlowNodeID] = nif.New(ifNode.FlowNodeID, name, comp.Kind, comp.Path, comp.Value)
	}

	for _, noopNode := range noopNodes {
		name := nodeNameMap[noopNode.FlowNodeID]
		flowNodeMap[noopNode.FlowNodeID] = nnoop.New(noopNode.FlowNodeID, name)
	}

	for _, forEachNode := range forEachNodes {
		name := nodeNameMap[forEachNode.FlowNodeID]
		flowNodeMap[forEachNode.FlowNodeID] = nforeach.New(forEachNode.FlowNodeID, name, forEachNode.IterPath, time.Second,
			forEachNode.Condition, forEachNode.ErrorHandling)
	}

	var clientPtr *nodejs_executorv1connect.NodeJSExecutorServiceClient
	for i, jsNode := range jsNodes {
		if i == 0 {
			client := nodejs_executorv1connect.NewNodeJSExecutorServiceClient(httpclient.New(), "http://localhost:9090")
			clientPtr = &client
		}

		if jsNode.CodeCompressType != compress.CompressTypeNone {
			jsNode.Code, err = compress.Decompress(jsNode.Code, jsNode.CodeCompressType)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
		}

		name := nodeNameMap[jsNode.FlowNodeID]

		flowNodeMap[jsNode.FlowNodeID] = njs.New(jsNode.FlowNodeID, name, string(jsNode.Code), *clientPtr)
	}

	// TODO: get timeout from flow config
	runnerInst := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), latestFlowID, startNodeID, flowNodeMap, edgeMap, time.Second*10)

	flowNodeStatusChan := make(chan runner.FlowNodeStatus, 1000)
	flowStatusChan := make(chan runner.FlowStatus, 10)

	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var successCount int
	totalNodes := len(flowNodeMap)

	flowTitle := flowPtr.Name
	const maxTitleLenght = 64
	if len(flowTitle) > maxTitleLenght {
		flowTitle = flowTitle[:maxTitleLenght]
		flowTitle += "..."
	}

	fmt.Println("=================================================================")
	fmt.Printf("| Flow: %s\n", flowTitle)
	fmt.Println("─────────────────────────────────────────────────────────────────")
	fmt.Println("| Timestamp           | Step           | Duration | Status      |")
	fmt.Println("─────────────────────────────────────────────────────────────────")

	done := make(chan error, 1)
	go func() {
		defer close(done)
		nodeStatusFunc := func(flowNodeStatus runner.FlowNodeStatus) {
			//id := flowNodeStatus.NodeID
			name := flowNodeStatus.Name
			//idStr := id.String()
			stateStr := mnnode.StringNodeStateWithIcons(flowNodeStatus.State)

			if flowNodeStatus.State != mnnode.NODE_STATE_RUNNING {

				fmt.Printf("| %-20s | %-14s | %-8s | %-10s |\n",
					time.Now().Format("2006-01-02 15:04:05"),
					name,
					formatDuration(flowNodeStatus.RunDuration),
					stateStr)

				if flowNodeStatus.State == mnnode.NODE_STATE_SUCCESS {
					successCount++
				}
			}
		}

		for {
			select {
			case <-subCtx.Done():
				close(flowNodeStatusChan)
				close(flowStatusChan)
				done <- errors.New("context done")
				return
			case flowNodeStatus, ok := <-flowNodeStatusChan:
				if !ok {
					return
				}
				nodeStatusFunc(flowNodeStatus)
			case flowStatus, ok := <-flowStatusChan:
				if !ok {
					return
				}
				if len(flowNodeStatusChan) > 0 {
					for flowNodeStatus := range flowNodeStatusChan {
						nodeStatusFunc(flowNodeStatus)
					}
				}
				if runner.IsFlowStatusDone(flowStatus) {
					done <- nil
					return
				}
			}
		}
	}()

	flowTime := time.Now()
	flowRunErr := runnerInst.Run(ctx, flowNodeStatusChan, flowStatusChan, nil)

	// wait for the flow to finish
	flowErr := <-done

	flowTimeLapse := time.Since(flowTime)

	close(requestNodeRespChan)

	fmt.Println("=================================================================")
	fmt.Printf("Flow Duration: %v | Steps: %d/%d Successful\n", flowTimeLapse, successCount, totalNodes)

	if flowErr != nil {
		return err
	}

	if flowRunErr != nil {
		return err
	}

	return nil
}
