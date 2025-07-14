package cmd

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
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
	yamlflowsimple "the-dev-tools/server/pkg/io/yamlflow/yamlflowsimple"
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
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/sexampleheader"
	"the-dev-tools/server/pkg/service/sexamplequery"
	"the-dev-tools/server/pkg/service/sexampleresp"
	"the-dev-tools/server/pkg/service/sexamplerespheader"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sflowvariable"
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
	"the-dev-tools/server/pkg/service/svar"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/spec/dist/buf/go/nodejs_executor/v1/nodejs_executorv1connect"
	"time"

	"the-dev-tools/cli/embeded/embededJS"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type FlowServiceLocal struct {
	DB *sql.DB
	ws sworkspace.WorkspaceService

	// flow
	fs  sflow.FlowService
	fes sedge.EdgeService
	fvs sflowvariable.FlowVariableService

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
	// Add yamlflowRunCmd directly to flowCmd since we only have one run command now
	flowCmd.AddCommand(yamlflowRunCmd)
}

var flowCmd = &cobra.Command{
	Use:   "flow",
	Short: "Flow Controls",
	Long:  `Flow Controls`,
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}



var yamlflowRunCmd = &cobra.Command{
	Use:   "run [yamlflow-file] [flow-name]",
	Short: "Run flow from yamlflow file",
	Long:  `Running Flow from a yamlflow format file. If flow-name is not provided, executes all flows from the 'run' field in order.`,
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		// TODO: move into context
		var logLevel slog.Level
		logLevelStr := os.Getenv("LOG_LEVEL")
		switch logLevelStr {
		case "DEBUG":
			logLevel = slog.LevelDebug
		case "INFO":
			logLevel = slog.LevelInfo
		case "WARNING":
			logLevel = slog.LevelWarn
		case "ERROR":
			logLevel = slog.LevelError
		default:
			logLevel = slog.LevelError
		}

		loggerHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: logLevel,
		})

		logger := slog.New(loggerHandler)

		yamlflowFilePath := args[0]
		var flowName string
		var runMultiple bool

		fileData, err := os.ReadFile(yamlflowFilePath)
		if err != nil {
			return err
		}

		// Check if flow name was provided as argument
		if len(args) > 1 {
			flowName = args[1]
			runMultiple = false
		} else {
			// Check for run field to execute multiple flows
			var rawYAML map[string]interface{}
			if err := yaml.Unmarshal(fileData, &rawYAML); err == nil {
				if runField, ok := rawYAML["run"].([]interface{}); ok && len(runField) > 0 {
					// Execute all flows in run field
					runMultiple = true
					log.Println("Executing flows based on run field configuration")
				}
			}

			if !runMultiple {
				return fmt.Errorf("no flow name provided and no run field found in workflow file")
			}
		}

		// Parse workflow YAML to workspace data
		var workspaceData *ioworkspace.WorkspaceData
		if runMultiple {
			// Use multi-flow import when running all flows
			workspaceData, err = yamlflowsimple.ImportYamlFlowYAMLMultiFlow(fileData)
			if err != nil {
				// Log the error from simplified format for debugging
				log.Printf("yamlflowsimple.ImportYamlFlowYAMLMultiFlow failed: %v", err)
				
				// Fall back to standard format
				workspaceData, err = ioworkspace.UnmarshalWorkflowYAML(fileData)
				if err != nil {
					return fmt.Errorf("failed to parse workflow: %w", err)
				}
			}
		} else {
			// For single flow, we still need to import all flows to find the requested one
			workspaceData, err = yamlflowsimple.ImportYamlFlowYAMLMultiFlow(fileData)
			if err != nil {
				// Log the error from simplified format for debugging
				log.Printf("yamlflowsimple.ImportYamlFlowYAMLMultiFlow failed: %v", err)
				
				// Fall back to standard format
				workspaceData, err = ioworkspace.UnmarshalWorkflowYAML(fileData)
				if err != nil {
					return fmt.Errorf("failed to parse workflow: %w", err)
				}
			}
		}

		err = workspaceData.VerifyIds()
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

		collectionService := scollection.New(queries, logger)
		workspaceService := sworkspace.New(queries)
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
		flowVariableService := sflowvariable.New(queries)
		flowForService := snodefor.New(queries)
		flowForEachService := snodeforeach.New(queries)
		flowJSService := snodejs.New(queries)
		flowEdges := sedge.New(queries)
		envService := senv.New(queries)
		varService := svar.New(queries)

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
			flowVariableService,
			flowRequestService,
			*flowConditionService,
			flowNoopService,
			flowForService,
			flowForEachService,
			flowJSService,
			envService,
			varService,
		)

		logMap := logconsole.NewLogChanMap()

		flowServiceLocal := FlowServiceLocal{
			DB:         db,
			ws:         workspaceService,
			fs:         flowService,
			fes:        flowEdges,
			fvs:        flowVariableService,
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

		// Import the workspace data
		err = ioWorkspaceService.ImportWorkspace(ctx, *workspaceData)
		if err != nil {
			return err
		}

		// Find the flow by name
		workspaceID := workspaceData.Workspace.ID
		c := flowServiceLocal

		flows, err := c.fs.GetFlowsByWorkspaceID(ctx, workspaceID)
		if err != nil {
			return err
		}

		if runMultiple {
			// Execute multiple flows based on run field
			return runMultipleFlows(ctx, fileData, flows, c, logger)
		} else {
			// Execute single flow (existing behavior)
			var flowPtr *mflow.Flow
			for _, flow := range flows {
				if flowName == flow.Name {
					flowPtr = &flow
					break
				}
			}

			if flowPtr == nil {
				return fmt.Errorf("flow '%s' not found in the workflow file", flowName)
			}

			log.Println("found flow", flowPtr.Name)
			err = flowRun(ctx, flowPtr, c)

			if err != nil {
				logger.Error(err.Error())
			}
			return err
		}
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

// flowExecutionResult holds the result of a flow execution
type flowExecutionResult struct {
	flowName     string
	success      bool
	duration     time.Duration
	nodeResults  map[string]interface{} // Store node results by name
	variables    map[string]interface{} // Variables from the flow
	err          error
}

// runMultipleFlows executes multiple flows based on the run field configuration
func runMultipleFlows(ctx context.Context, fileData []byte, allFlows []mflow.Flow, c FlowServiceLocal, logger *slog.Logger) error {
	// Parse the run field to get flow order and dependencies
	var rawYAML map[string]interface{}
	if err := yaml.Unmarshal(fileData, &rawYAML); err != nil {
		return fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	runField, ok := rawYAML["run"].([]interface{})
	if !ok || len(runField) == 0 {
		return fmt.Errorf("no run field found in workflow")
	}

	// Parse run entries
	type runEntry struct {
		flowName  string
		dependsOn []string
	}

	var runEntries []runEntry
	for _, entry := range runField {
		entryMap, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}

		flowName, ok := entryMap["flow"].(string)
		if !ok || flowName == "" {
			continue
		}

		re := runEntry{flowName: flowName}

		// Parse dependencies
		if deps, ok := entryMap["depends_on"]; ok {
			switch v := deps.(type) {
			case string:
				re.dependsOn = []string{v}
			case []interface{}:
				for _, dep := range v {
					if depStr, ok := dep.(string); ok {
						re.dependsOn = append(re.dependsOn, depStr)
					}
				}
			}
		}

		runEntries = append(runEntries, re)
	}

	// Create flow map for easy lookup
	flowMap := make(map[string]*mflow.Flow)
	for i := range allFlows {
		flowMap[allFlows[i].Name] = &allFlows[i]
	}

	// Track execution results
	executionResults := make(map[string]*flowExecutionResult)
	sharedVariables := make(map[string]interface{})
	_ = sharedVariables // TODO: Implement variable sharing between flows

	// Execute flows in order
	fmt.Println("\n=== Multi-Flow Execution Starting ===")
	fmt.Printf("Flows to execute: %d\n", len(runEntries))

	overallStartTime := time.Now()

	for i, entry := range runEntries {
		flow, exists := flowMap[entry.flowName]
		if !exists {
			return fmt.Errorf("flow '%s' not found in workflow", entry.flowName)
		}

		// Check dependencies
		for _, dep := range entry.dependsOn {
			// Check if dependency is a flow
			if depResult, ok := executionResults[dep]; ok {
				if !depResult.success {
					return fmt.Errorf("flow '%s' depends on '%s' which failed", entry.flowName, dep)
				}
			}
			// Note: We could also check for node dependencies here in the future
		}

		fmt.Printf("\n[%d/%d] Executing flow: %s\n", i+1, len(runEntries), entry.flowName)
		if len(entry.dependsOn) > 0 {
			fmt.Printf("   Dependencies: %v\n", entry.dependsOn)
		}

		// Execute the flow
		startTime := time.Now()
		err := flowRun(ctx, flow, c)
		duration := time.Since(startTime)

		result := &flowExecutionResult{
			flowName:    entry.flowName,
			success:     err == nil,
			duration:    duration,
			nodeResults: make(map[string]interface{}), // TODO: Capture actual results
			variables:   make(map[string]interface{}), // TODO: Capture flow variables
			err:         err,
		}

		executionResults[entry.flowName] = result

		if err != nil {
			fmt.Printf("   ❌ Flow failed: %v\n", err)
			logger.Error("flow execution failed", "flow", entry.flowName, "error", err)
			// Continue to show summary even if a flow fails
		} else {
			fmt.Printf("   ✅ Flow completed successfully (Duration: %s)\n", formatDuration(duration))
		}
	}

	// Display summary
	overallDuration := time.Since(overallStartTime)
	fmt.Println("\n=== Multi-Flow Execution Summary ===")
	fmt.Printf("Total duration: %s\n", formatDuration(overallDuration))
	fmt.Println("\nFlow Results:")

	successCount := 0
	for _, entry := range runEntries {
		result := executionResults[entry.flowName]
		status := "✅ Success"
		if !result.success {
			status = "❌ Failed"
		} else {
			successCount++
		}
		fmt.Printf("  %-20s %s (Duration: %s)\n", result.flowName, status, formatDuration(result.duration))
	}

	fmt.Printf("\nFlows completed: %d/%d\n", successCount, len(runEntries))

	// Return error if any flow failed
	for _, result := range executionResults {
		if !result.success && result.err != nil {
			return fmt.Errorf("multi-flow execution failed: one or more flows failed")
		}
	}

	return nil
}

func flowRun(ctx context.Context, flowPtr *mflow.Flow, c FlowServiceLocal) error {
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

	flowVars, err := c.fvs.GetFlowVariablesByFlowID(ctx, latestFlowID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, errors.New("get edges"))
	}

	flowVarsMap := make(map[string]any, len(flowVars))
	for _, flowVar := range flowVars {
		if flowVar.Enabled {
			flowVarsMap[flowVar.Name] = flowVar.Value
		}
	}

	// Create temporary request to safely read timeout variable
	tempReq := &node.FlowNodeRequest{
		VarMap:        flowVarsMap,
		ReadWriteLock: &sync.RWMutex{},
	}

	// Set default timeout to 60 seconds, check for timeout variable override
	nodeTimeout := time.Second * 60
	if timeoutVar, err := node.ReadVarRaw(tempReq, "timeout"); err == nil {
		if timeoutSeconds, ok := timeoutVar.(float64); ok && timeoutSeconds > 0 {
			nodeTimeout = time.Duration(timeoutSeconds) * time.Second
		} else if timeoutSecondsInt, ok := timeoutVar.(int); ok && timeoutSecondsInt > 0 {
			nodeTimeout = time.Duration(timeoutSecondsInt) * time.Second
		}
	}

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
		flowNodeMap[forNode.FlowNodeID] = nfor.New(forNode.FlowNodeID, name, forNode.IterCount, nodeTimeout, forNode.ErrorHandling)
	}

	requestNodeRespChan := make(chan nrequest.NodeRequestSideResp, len(requestNodes)*100)
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

		exampleResp, err := c.ers.GetExampleRespByExampleIDLatest(ctx, example.ID)
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

			if requestNode.DeltaEndpointID != nil {
				deltaEndpoint, err := c.ias.GetItemApi(ctx, *requestNode.DeltaEndpointID)
				if err != nil {
					return connect.NewError(connect.CodeInternal, err)
				}
				endpoint.Url = deltaEndpoint.Url
				endpoint.Method = deltaEndpoint.Method
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
		comp := ifNode.Condition
		name := nodeNameMap[ifNode.FlowNodeID]
		flowNodeMap[ifNode.FlowNodeID] = nif.New(ifNode.FlowNodeID, name, comp)
	}

	for _, noopNode := range noopNodes {
		name := nodeNameMap[noopNode.FlowNodeID]
		flowNodeMap[noopNode.FlowNodeID] = nnoop.New(noopNode.FlowNodeID, name)
	}

	for _, forEachNode := range forEachNodes {
		name := nodeNameMap[forEachNode.FlowNodeID]
		flowNodeMap[forEachNode.FlowNodeID] = nforeach.New(forEachNode.FlowNodeID, name, forEachNode.IterExpression, nodeTimeout,
			forEachNode.Condition, forEachNode.ErrorHandling)
	}

	var clientPtr *nodejs_executorv1connect.NodeJSExecutorServiceClient
	if len(jsNodes) > 0 {
		// Attempt to start the NodeJS worker if node is available
		nodePath, err := exec.LookPath("node")
		if err != nil {
			slog.Warn("node binary not found in PATH, assuming worker-js is already running or not needed")
		} else {
			slog.Info("node binary found", "path", nodePath)
			cmd := exec.CommandContext(ctx, nodePath, "--experimental-vm-modules", "--disable-warning=ExperimentalWarning", "-")

			cmd.Stdin = strings.NewReader(embededJS.WorkerJS) // Pipe the embedded script content
			// TODO: Optionally pipe stdout/stderr of the node process
			// cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err := cmd.Start()
			if err != nil {
				slog.Error("failed to start worker-js", "error", err)
			} else {
				defer func() {
					_ = cmd.Cancel()
				}()
				slog.Info("worker-js process started", "pid", cmd.Process.Pid)
				go func() {
					err := cmd.Wait()
					if err != nil {
						slog.Error("worker-js process exited with error", "error", err)
					} else {
						slog.Info("worker-js process exited successfully")
					}
				}()
			}
		}

		client := nodejs_executorv1connect.NewNodeJSExecutorServiceClient(httpclient.New(), "http://localhost:9090")
		clientPtr = &client
	}

	for _, jsNode := range jsNodes {
		if jsNode.CodeCompressType != compress.CompressTypeNone {
			jsNode.Code, err = compress.Decompress(jsNode.Code, jsNode.CodeCompressType)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
		}

		name := nodeNameMap[jsNode.FlowNodeID]

		flowNodeMap[jsNode.FlowNodeID] = njs.New(jsNode.FlowNodeID, name, string(jsNode.Code), *clientPtr)
	}

	// Use the same timeout for the flow runner
	runnerInst := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), latestFlowID, startNodeID, flowNodeMap, edgeMap, nodeTimeout)

	flowNodeStatusChan := make(chan runner.FlowNodeStatus, 1000)
	flowStatusChan := make(chan runner.FlowStatus, 10)

	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var successCount int
	totalNodes := len(flowNodeMap)

	flowTitle := flowPtr.Name

	// Calculate max step name length
	maxStepNameLen := len("Step") // Default to header length
	for _, name := range nodeNameMap {
		if len(name) > maxStepNameLen {
			maxStepNameLen = len(name)
		}
	}

	tableWidth := 2 + 20 + 3 + maxStepNameLen + 3 + 8 + 3 + 11 + 2 // Total = maxStepNameLen + 52

	topBottomBorder := strings.Repeat("=", tableWidth)
	separatorBorder := strings.Repeat("─", tableWidth)
	tableRowFmt := fmt.Sprintf("| %%-20s | %%-%ds | %%-8s | %%-11s |\n", maxStepNameLen)

	// Format Flow title line to fit within the table width
	displayTitleContent := fmt.Sprintf(" Flow: %s", flowTitle) // Use original flowTitle
	maxContentWidthInTitle := tableWidth - 2                   // Available space between '|' and '|'

	if len(displayTitleContent) > maxContentWidthInTitle {
		if maxContentWidthInTitle > 3 { // Check if space for "..."
			displayTitleContent = displayTitleContent[:maxContentWidthInTitle-3] + "..."
		} else if maxContentWidthInTitle >= 0 { // Only truncate if non-negative space
			displayTitleContent = displayTitleContent[:maxContentWidthInTitle]
		} else {
			displayTitleContent = "" // Not enough space for anything
		}
	}

	paddingLength := maxContentWidthInTitle - len(displayTitleContent)
	if paddingLength < 0 {
		paddingLength = 0
	}

	fmt.Println(topBottomBorder)
	fmt.Printf("|%s%s|", displayTitleContent, strings.Repeat(" ", paddingLength))
	fmt.Println() // Ensure newline before separator
	fmt.Println(separatorBorder)
	fmt.Printf(tableRowFmt, "Timestamp", "Step", "Duration", "Status") // tableRowFmt includes a newline
	fmt.Println(separatorBorder)

	done := make(chan error, 1)
	go func() {
		defer close(done)
		nodeStatusFunc := func(flowNodeStatus runner.FlowNodeStatus) {
			name := flowNodeStatus.Name
			stateStr := mnnode.StringNodeStateWithIcons(flowNodeStatus.State)

			if flowNodeStatus.State != mnnode.NODE_STATE_RUNNING {
				fmt.Printf(tableRowFmt, // tableRowFmt includes a newline
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
	flowRunErr := runnerInst.Run(ctx, flowNodeStatusChan, flowStatusChan, flowVarsMap)

	// wait for the flow to finish
	flowErr := <-done

	flowTimeLapse := time.Since(flowTime)

	close(requestNodeRespChan)

	fmt.Println(topBottomBorder) // Use dynamic border
	fmt.Printf("Flow Duration: %v | Steps: %d/%d Successful\n", flowTimeLapse, successCount, totalNodes)

	if flowErr != nil {
		return flowErr
	}

	if flowRunErr != nil {
		return flowRunErr
	}

	return nil
}
