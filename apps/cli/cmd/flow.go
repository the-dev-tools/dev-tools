package cmd

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"
	"sync"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/db/pkg/sqlitemem"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/node/nfor"
	"the-dev-tools/server/pkg/flow/node/nforeach"
	"the-dev-tools/server/pkg/flow/node/nif"
	"the-dev-tools/server/pkg/flow/node/nnoop"
	"the-dev-tools/server/pkg/flow/node/nrequest"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/flow/runner/flowlocalrunner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/ioworkspace"
	yamlflowsimplev2 "the-dev-tools/server/pkg/translate/yamlflowsimplev2"
	"the-dev-tools/server/pkg/logconsole"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"the-dev-tools/server/pkg/model/mnnode/mnforeach"
	"the-dev-tools/server/pkg/model/mnnode/mnif"
	"the-dev-tools/server/pkg/model/mnnode/mnjs"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/httpclient"
	"the-dev-tools/server/pkg/service/flow/sedge"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sflowvariable"
	// "the-dev-tools/server/pkg/service/sitemapi" // Removed - using v2 packages
	// "the-dev-tools/server/pkg/service/sitemapiexample" // Removed - using v2 packages
	// "the-dev-tools/server/pkg/service/sitemfolder" // Removed - using v2 packages
	"the-dev-tools/server/pkg/service/snode"
	"the-dev-tools/server/pkg/service/snodefor"
	"the-dev-tools/server/pkg/service/snodeforeach"
	"the-dev-tools/server/pkg/service/snodeif"
	"the-dev-tools/server/pkg/service/snodejs"
	"the-dev-tools/server/pkg/service/snodenoop"
	"the-dev-tools/server/pkg/service/snoderequest"
	"the-dev-tools/server/pkg/service/sworkspace"
	// "the-dev-tools/spec/dist/buf/go/node_js_executor/v1/node_js_executorv1connect" // Commented out - using local execution
	// V2 service imports
	"the-dev-tools/server/pkg/service/shttp"
	// "the-dev-tools/server/pkg/service/sfile"
	"time"

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

	// sub nodes
	ns   snode.NodeService
	rns  snoderequest.NodeRequestService
	fns  snodefor.NodeForService
	fens snodeforeach.NodeForEachService
	sns  snodenoop.NodeNoopService
	ins  snodeif.NodeIfService
	jsns snodejs.NodeJSService

	// V2 services
	hs     shttp.HTTPService
	hh     shttp.HttpHeaderService
	hsp    *shttp.HttpSearchParamService
	hbf    *shttp.HttpBodyFormService
	hbu    *shttp.HttpBodyUrlEncodedService
	hbr    *shttp.HttpBodyRawService
	has    *shttp.HttpAssertService
	logger *slog.Logger

	logChanMap logconsole.LogChanMap
}

func init() {
	rootCmd.AddCommand(flowCmd)
	// Add yamlflowRunCmd directly to flowCmd since we only have one run command now
	flowCmd.AddCommand(yamlflowRunCmd)
	yamlflowRunCmd.Flags().StringSliceVar(&reportFormats, "report", []string{"console"}, "Report outputs to produce (format[:path]). Supported formats: console, json, junit.")
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

		// Parse workflow YAML using v2 packages
		// Create a workspace ID for the import
		workspaceID := idwrap.NewNow()

		// Convert YAML using v2 converter
		resolved, err := yamlflowsimplev2.ConvertSimplifiedYAML(fileData, yamlflowsimplev2.ConvertOptionsV2{
			WorkspaceID: workspaceID,
		})
		if err != nil {
			return fmt.Errorf("failed to convert YAML using v2: %w", err)
		}

		db, _, err := sqlitemem.NewSQLiteMem(ctx)
		if err != nil {
			return err
		}

		queries, err := gen.Prepare(ctx, db)
		if err != nil {
			return err
		}

		// Initialize services
		workspaceService := sworkspace.New(queries)
		flowService := sflow.New(queries)
		flowNodeService := snode.New(queries)
		flowRequestService := snoderequest.New(queries)
		flowConditionService := snodeif.New(queries)
		flowNoopService := snodenoop.New(queries)
		flowVariableService := sflowvariable.New(queries)
		flowForService := snodefor.New(queries)
		flowForEachService := snodeforeach.New(queries)
		flowJSService := snodejs.New(queries)
		flowEdges := sedge.New(queries)

		// V2 services
		httpService := shttp.New(queries, logger)
		httpHeaderService := shttp.NewHttpHeaderService(queries)
		httpSearchParamService := shttp.NewHttpSearchParamService(queries)
		httpBodyFormService := shttp.NewHttpBodyFormService(queries)
		httpBodyUrlEncodedService := shttp.NewHttpBodyUrlEncodedService(queries)
		httpBodyRawService := shttp.NewHttpBodyRawService(queries)
		httpAssertService := shttp.NewHttpAssertService(queries)

		logMap := logconsole.NewLogChanMap()

		flowServiceLocal := FlowServiceLocal{
			DB:   db,
			ws:   workspaceService,
			fs:   flowService,
			fes:  flowEdges,
			fvs:  flowVariableService,
			ns:   flowNodeService,
			rns:  flowRequestService,
			fns:  flowForService,
			fens: flowForEachService,
			sns:  flowNoopService,
			ins:  *flowConditionService,
			jsns: flowJSService,
			// V2 services
			hs:     httpService,
			hh:     httpHeaderService,
			hsp:    httpSearchParamService,
			hbf:    httpBodyFormService,
			hbu:    httpBodyUrlEncodedService,
			hbr:    httpBodyRawService,
			has:    httpAssertService,
			logger: logger,

			logChanMap: logMap,
		}

		// Import all entities from the resolved bundle
		log.Printf("Importing workspace bundle: %d flows, %d nodes", len(resolved.Flows), len(resolved.FlowNodes))

		// Create IOWorkspaceService
		ioService := ioworkspace.New(queries, logger)

		// Start transaction for import
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}

		// Import options
		importOpts := ioworkspace.GetDefaultImportOptions(workspaceID)
		importOpts.PreserveIDs = true // Preserve IDs generated by the converter

		if _, err := ioService.Import(ctx, tx, resolved, importOpts); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to import workspace bundle: %w", err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit transaction: %w", err)
		}

		// Find the flow by name - use the workspaceID we created earlier
		c := flowServiceLocal

		flows, err := c.fs.GetFlowsByWorkspaceID(ctx, workspaceID)
		if err != nil {
			return err
		}

		specs, err := parseReportSpecs(reportFormats)
		if err != nil {
			return err
		}
		reporters, err := newReporterGroup(specs)
		if err != nil {
			return err
		}

		var runErr error
		if runMultiple {
			// Execute multiple flows based on run field
			runErr = runMultipleFlows(ctx, fileData, flows, c, logger, reporters)
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
			_, runErr = flowRun(ctx, flowPtr, c, reporters)

			if runErr != nil {
				logger.Error(runErr.Error())
			}
		}

		flushErr := reporters.Flush()
		if runErr != nil {
			return runErr
		}
		return flushErr
	},
}

var reportFormats []string

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

// runMultipleFlows executes multiple flows based on the run field configuration
func runMultipleFlows(ctx context.Context, fileData []byte, allFlows []mflow.Flow, c FlowServiceLocal, logger *slog.Logger, reporters *ReporterGroup) error {
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
	executionResults := make(map[string]FlowRunResult)
	consoleEnabled := reporters != nil && reporters.HasConsole()
	sharedVariables := make(map[string]interface{})
	_ = sharedVariables // TODO: Implement variable sharing between flows

	// Execute flows in order
	if consoleEnabled {
		fmt.Println("\n=== Multi-Flow Execution Starting ===")
		fmt.Printf("Flows to execute: %d\n", len(runEntries))
	}

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
				if !strings.EqualFold(depResult.Status, "success") {
					return fmt.Errorf("flow '%s' depends on '%s' which failed", entry.flowName, dep)
				}
			}
			// Note: We could also check for node dependencies here in the future
		}

		if consoleEnabled {
			fmt.Printf("\n[%d/%d] Executing flow: %s\n", i+1, len(runEntries), entry.flowName)
			if len(entry.dependsOn) > 0 {
				fmt.Printf("   Dependencies: %v\n", entry.dependsOn)
			}
		}

		result, err := flowRun(ctx, flow, c, reporters)
		executionResults[entry.flowName] = result

		if err != nil {
			if consoleEnabled {
				fmt.Printf("   ❌ Flow failed: %v\n", err)
			}
			logger.Error("flow execution failed", "flow", entry.flowName, "error", err)
		} else if consoleEnabled {
			fmt.Printf("   ✅ Flow completed successfully (Duration: %s)\n", formatDuration(result.Duration))
		}
	}

	if consoleEnabled {
		overallDuration := time.Since(overallStartTime)
		fmt.Println("\n=== Multi-Flow Execution Summary ===")
		fmt.Printf("Total duration: %s\n", formatDuration(overallDuration))
		fmt.Println("\nFlow Results:")

		successCount := 0
		for _, entry := range runEntries {
			result := executionResults[entry.flowName]
			status := "✅ Success"
			if !strings.EqualFold(result.Status, "success") {
				status = "❌ Failed"
			} else {
				successCount++
			}
			fmt.Printf("  %-20s %s (Duration: %s)\n", result.FlowName, status, formatDuration(result.Duration))
		}

		fmt.Printf("\nFlows completed: %d/%d\n", successCount, len(runEntries))
	}

	for _, result := range executionResults {
		if !strings.EqualFold(result.Status, "success") {
			if result.Error != "" {
				return fmt.Errorf("multi-flow execution failed: %s", result.Error)
			}
			return fmt.Errorf("multi-flow execution failed: one or more flows failed")
		}
	}

	return nil
}

func flowRun(ctx context.Context, flowPtr *mflow.Flow, c FlowServiceLocal, reporters *ReporterGroup) (FlowRunResult, error) {
	result := FlowRunResult{
		FlowID:   flowPtr.ID.String(),
		FlowName: flowPtr.Name,
		Started:  time.Now(),
	}

	markFailure := func(err error) (FlowRunResult, error) {
		if err != nil {
			result.Error = err.Error()
		}
		result.Status = "failed"
		result.Duration = time.Since(result.Started)
		if reporters != nil {
			reporters.HandleFlowResult(result)
		}
		return result, err
	}

	latestFlowID := flowPtr.ID

	nodes, err := c.ns.GetNodesByFlowID(ctx, latestFlowID)
	if err != nil {
		return markFailure(connect.NewError(connect.CodeInternal, errors.New("get nodes")))
	}

	edges, err := c.fes.GetEdgesByFlowID(ctx, latestFlowID)
	if err != nil {
		return markFailure(connect.NewError(connect.CodeInternal, errors.New("get edges")))
	}
	edgeMap := edge.NewEdgesMap(edges)

	flowVars, err := c.fvs.GetFlowVariablesByFlowID(ctx, latestFlowID)
	if err != nil {
		return markFailure(connect.NewError(connect.CodeInternal, errors.New("get edges")))
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
				return markFailure(connect.NewError(connect.CodeInternal, fmt.Errorf("get node request: %w", err)))
			}
			requestNodes = append(requestNodes, *rn)
		case mnnode.NODE_KIND_FOR:
			fn, err := c.fns.GetNodeFor(ctx, node.ID)
			if err != nil {
				return markFailure(connect.NewError(connect.CodeInternal, fmt.Errorf("get node for: %w", err)))
			}
			forNodes = append(forNodes, *fn)
		case mnnode.NODE_KIND_FOR_EACH:
			fen, err := c.fens.GetNodeForEach(ctx, node.ID)
			if err != nil {
				return markFailure(connect.NewError(connect.CodeInternal, fmt.Errorf("get node for each: %w", err)))
			}
			forEachNodes = append(forEachNodes, *fen)
		case mnnode.NODE_KIND_NO_OP:
			sn, err := c.sns.GetNodeNoop(ctx, node.ID)
			if err != nil {
				return markFailure(connect.NewError(connect.CodeInternal, fmt.Errorf("get node start: %w", err)))
			}
			noopNodes = append(noopNodes, *sn)
		case mnnode.NODE_KIND_CONDITION:
			in, err := c.ins.GetNodeIf(ctx, node.ID)
			if err != nil {
				return markFailure(connect.NewError(connect.CodeInternal, errors.New("get node if")))
			}
			ifNodes = append(ifNodes, *in)
		case mnnode.NODE_KIND_JS:
			jsn, err := c.jsns.GetNodeJS(ctx, node.ID)
			if err != nil {
				return markFailure(connect.NewError(connect.CodeInternal, fmt.Errorf("get node js: %w", err)))
			}
			jsNodes = append(jsNodes, jsn)
		default:
			return markFailure(connect.NewError(connect.CodeInternal, errors.New("not supported node")))
		}
	}

	var foundStartNode bool
	for _, node := range noopNodes {
		if node.Type == mnnoop.NODE_NO_OP_KIND_START {
			if foundStartNode {
				return markFailure(connect.NewError(connect.CodeInternal, errors.New("multiple start nodes")))
			}
			foundStartNode = true
			startNodeID = node.FlowNodeID
		}
	}
	if !foundStartNode {
		return markFailure(connect.NewError(connect.CodeInternal, errors.New("no start node")))
	}

	flowNodeMap := make(map[idwrap.IDWrap]node.FlowNode, 0)
	for _, forNode := range forNodes {
		name := nodeNameMap[forNode.FlowNodeID]
		flowNodeMap[forNode.FlowNodeID] = nfor.New(forNode.FlowNodeID, name, forNode.IterCount, nodeTimeout, forNode.ErrorHandling)
	}

	// Calculate buffer size for request responses based on flow complexity
	requestBufferSize := len(requestNodes) * 100
	if forNodeCount := len(forNodes); forNodeCount > 0 {
		// For flows with iterations, we need larger buffers
		var maxIterations int64
		for _, fn := range forNodes {
			if fn.IterCount > maxIterations {
				maxIterations = fn.IterCount
			}
		}
		// Estimate requests per iteration
		estimatedRequests := int(maxIterations) * len(requestNodes) * 2
		if estimatedRequests > requestBufferSize {
			requestBufferSize = estimatedRequests
		}
	}
	httpClient := httpclient.New()
	requestNodeRespChan := make(chan nrequest.NodeRequestSideResp, requestBufferSize)

	// Start a goroutine to consume request responses and signal completion
	// This is necessary because nrequest nodes block waiting for Done to be closed
	var respDrain sync.WaitGroup
	respDrain.Add(1)
	go func() {
		defer respDrain.Done()
		for resp := range requestNodeRespChan {
			// Signal that we've processed the response
			if resp.Done != nil {
				close(resp.Done)
			}
		}
	}()

	for _, requestNode := range requestNodes {
		if requestNode.HttpID == nil {
			return markFailure(connect.NewError(connect.CodeInternal, fmt.Errorf("request node %s has no http id", requestNode.FlowNodeID)))
		}

		httpRecord, err := c.hs.Get(ctx, *requestNode.HttpID)
		if err != nil {
			return markFailure(connect.NewError(connect.CodeInternal, fmt.Errorf("load http %s: %w", requestNode.HttpID.String(), err)))
		}

		headers, err := c.hh.GetByHttpID(ctx, *requestNode.HttpID)
		if err != nil {
			return markFailure(connect.NewError(connect.CodeInternal, fmt.Errorf("load http headers: %w", err)))
		}

		queries, err := c.hsp.GetByHttpID(ctx, *requestNode.HttpID)
		if err != nil {
			return markFailure(connect.NewError(connect.CodeInternal, fmt.Errorf("load http queries: %w", err)))
		}

		forms, err := c.hbf.GetByHttpID(ctx, *requestNode.HttpID)
		if err != nil {
			return markFailure(connect.NewError(connect.CodeInternal, fmt.Errorf("load http body forms: %w", err)))
		}

		urlEncoded, err := c.hbu.GetByHttpID(ctx, *requestNode.HttpID)
		if err != nil {
			return markFailure(connect.NewError(connect.CodeInternal, fmt.Errorf("load http body urlencoded: %w", err)))
		}
		urlEncodedVals := urlEncoded

		rawBody, err := c.hbr.GetByHttpID(ctx, *requestNode.HttpID)
		if err != nil && !errors.Is(err, shttp.ErrNoHttpBodyRawFound) && !errors.Is(err, sql.ErrNoRows) {
			return markFailure(connect.NewError(connect.CodeInternal, fmt.Errorf("load http body raw: %w", err)))
		}

		asserts, err := c.has.GetByHttpID(ctx, *requestNode.HttpID)
		if err != nil {
			return markFailure(connect.NewError(connect.CodeInternal, fmt.Errorf("load http asserts: %w", err)))
		}

		name := nodeNameMap[requestNode.FlowNodeID]
		flowNodeMap[requestNode.FlowNodeID] = nrequest.New(
			requestNode.FlowNodeID,
			name,
			*httpRecord,
			headers,
			queries,
			rawBody,
			forms,
			urlEncodedVals,
			asserts,
			httpClient,
			requestNodeRespChan,
			c.logger,
		)
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

	// Node.js executor code commented out - using local execution instead
	// var clientPtr *node_js_executorv1connect.NodeJsExecutorServiceClient
	if len(jsNodes) > 0 {
		// TODO: Implement local JS execution or skip JS nodes for now
		log.Printf("Warning: %d JS nodes found but Node.js executor is disabled", len(jsNodes))
		/*
		// Original Node.js executor code - disabled for v2 migration
		if nodePath, err := exec.LookPath("node"); err != nil {
			slog.Warn("node binary not found in PATH, assuming worker-js is already running or not needed")
		} else {
			// ... Node.js startup code would go here
		}
		*/
	}

	// TODO: Implement JS node handling without external executor
	for _, jsNode := range jsNodes {
		log.Printf("Skipping JS node '%s' - executor disabled", nodeNameMap[jsNode.FlowNodeID])
		// Create a no-op node instead of JS node for now
		jsNodeID := jsNode.FlowNodeID
		jsNodeName := nodeNameMap[jsNodeID]
		flowNodeMap[jsNodeID] = nnoop.New(jsNodeID, jsNodeName)
	}

	// Use the same timeout for the flow runner
	runnerInst := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), latestFlowID, startNodeID, flowNodeMap, edgeMap, nodeTimeout, nil)

	// Calculate buffer size based on expected load
	// For large iteration counts, we need bigger buffers to prevent blocking
	bufferSize := 10000
	if forNodeCount := len(forNodes); forNodeCount > 0 {
		// Estimate based on for node iterations
		var maxIterations int64
		for _, fn := range forNodes {
			if fn.IterCount > maxIterations {
				maxIterations = fn.IterCount
			}
		}
		// Buffer should handle at least all iterations * nodes
		estimatedSize := int(maxIterations) * len(flowNodeMap) * 2
		if estimatedSize > bufferSize {
			bufferSize = estimatedSize
		}
	}

	flowNodeStatusChan := make(chan runner.FlowNodeStatus, bufferSize)
	flowStatusChan := make(chan runner.FlowStatus, 100)

	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	nodeNames := make([]string, 0, len(nodeNameMap))
	for _, name := range nodeNameMap {
		nodeNames = append(nodeNames, name)
	}

	if reporters != nil {
		reporters.HandleFlowStart(FlowStartInfo{
			FlowID:     result.FlowID,
			FlowName:   flowPtr.Name,
			TotalNodes: len(flowNodeMap),
			NodeNames:  nodeNames,
		})
	}

	nodeResults := make([]NodeRunResult, 0, len(flowNodeMap))
	done := make(chan error, 1)
	go func() {
		defer close(done)

		nodeStatusFunc := func(flowNodeStatus runner.FlowNodeStatus) {
			if reporters != nil {
				reporters.HandleNodeStatus(NodeStatusEvent{
					FlowID:   result.FlowID,
					FlowName: flowPtr.Name,
					Status:   flowNodeStatus,
				})
			}

			if flowNodeStatus.State != mnnode.NODE_STATE_RUNNING {
				nodeResults = append(nodeResults, buildNodeRunResult(flowNodeStatus))
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
				if runner.IsFlowStatusDone(flowStatus) {
					for flowNodeStatus := range flowNodeStatusChan {
						nodeStatusFunc(flowNodeStatus)
					}
					done <- nil
					return
				}
			}
		}
	}()

	result.Started = time.Now()
	flowRunErr := runnerInst.Run(ctx, flowNodeStatusChan, flowStatusChan, flowVarsMap)

	// wait for the flow to finish
	flowErr := <-done

	result.Duration = time.Since(result.Started)
	result.Nodes = nodeResults

	close(requestNodeRespChan)
	respDrain.Wait()

	var finalErr error
	if flowErr != nil {
		finalErr = flowErr
	} else if flowRunErr != nil {
		finalErr = flowRunErr
	}

	switch {
	case finalErr == nil:
		result.Status = "success"
	case errors.Is(finalErr, context.DeadlineExceeded):
		result.Status = "timeout"
		result.Error = finalErr.Error()
	case errors.Is(finalErr, context.Canceled):
		result.Status = "canceled"
		result.Error = finalErr.Error()
	default:
		result.Status = "failed"
		if finalErr != nil {
			result.Error = finalErr.Error()
		}
	}

	if reporters != nil {
		reporters.HandleFlowResult(result)
	}

	return result, finalErr
}
