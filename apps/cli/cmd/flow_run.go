package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/node/nrequest"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/flow/runner/flowlocalrunner"
	"the-dev-tools/server/pkg/httpclient"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/spec/dist/buf/go/api/node_js_executor/v1/node_js_executorv1connect"

	"connectrpc.com/connect"
	"gopkg.in/yaml.v3"
)

// runMultipleFlows executes multiple flows based on the run field configuration
func runMultipleFlows(ctx context.Context, fileData []byte, allFlows []mflow.Flow, c FlowServiceLocal, logger *slog.Logger, reporters *ReporterGroup, jsClient node_js_executorv1connect.NodeJsExecutorServiceClient) error {
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

		result, err := flowRun(ctx, flow, c, reporters, jsClient)
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

func flowRun(ctx context.Context, flowPtr *mflow.Flow, c FlowServiceLocal, reporters *ReporterGroup, jsClient node_js_executorv1connect.NodeJsExecutorServiceClient) (FlowRunResult, error) {
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

	// Build flow variables using flowbuilder
	// Note: BuildVariables takes workspaceID, not flowID, to fetch environment variables
	flowVarsMap, err := c.builder.BuildVariables(ctx, flowPtr.WorkspaceID, flowVars)
	if err != nil {
		return markFailure(connect.NewError(connect.CodeInternal, fmt.Errorf("build variables: %w", err)))
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

	// Initialize resources for request nodes
	httpClient := httpclient.New()
	// Estimate buffer size: nodes * 100 is a safe upper bound for most CLI runs
	requestBufferSize := len(nodes) * 100
	requestRespChan := make(chan nrequest.NodeRequestSideResp, requestBufferSize)

	// Start a goroutine to consume request responses
	go func() {
		for resp := range requestRespChan {
			if resp.Done != nil {
				close(resp.Done)
			}
		}
	}()
	defer close(requestRespChan)

	// Build flow node map using flowbuilder
	flowNodeMap, startNodeID, err := c.builder.BuildNodes(
		ctx,
		*flowPtr,
		nodes,
		nodeTimeout,
		httpClient,
		requestRespChan,
		jsClient,
	)
	if err != nil {
		return markFailure(err)
	}

	// Use the same timeout for the flow runner
	runnerInst := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), latestFlowID, startNodeID, flowNodeMap, edgeMap, nodeTimeout, nil)

	// Use a large buffer for CLI to avoid blocking
	flowNodeStatusChan := make(chan runner.FlowNodeStatus, 10000)
	flowStatusChan := make(chan runner.FlowStatus, 100)

	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	nodeNames := make([]string, 0, len(flowNodeMap))
	for _, node := range flowNodeMap {
		nodeNames = append(nodeNames, node.GetName())
	}

	if reporters != nil {
		reporters.HandleFlowStart(FlowStartInfo{
			FlowID:     result.FlowID,
			FlowName:   flowPtr.Name,
			TotalNodes: len(flowNodeMap),
			NodeNames:  nodeNames,
		})
	}

	// Start the runner
	go func() {
		if err := runnerInst.Run(subCtx, flowNodeStatusChan, flowStatusChan, flowVarsMap); err != nil {
			slog.Error("flow runner failed", "error", err)
		}
	}()

	// Collect results
	nodeResults := make([]NodeRunResult, 0)
	var finalStatus runner.FlowStatus

	// Wait for completion
	for {
		select {
		case nodeStatus := <-flowNodeStatusChan:
			if reporters != nil {
				reporters.HandleNodeStatus(NodeStatusEvent{
					FlowID:   result.FlowID,
					FlowName: flowPtr.Name,
					Status:   nodeStatus,
				})
			}
			if nodeStatus.State != mnnode.NODE_STATE_RUNNING {
				nodeResults = append(nodeResults, buildNodeRunResult(nodeStatus))
			}

		case flowStatus := <-flowStatusChan:
			finalStatus = flowStatus
			if runner.IsFlowStatusDone(flowStatus) {
				goto Done
			}

		case <-ctx.Done():
			return markFailure(ctx.Err())
		}
	}

Done:
	result.Duration = time.Since(result.Started)
	result.Nodes = nodeResults

	if finalStatus == runner.FlowStatusSuccess {
		result.Status = "success"
	} else {
		result.Status = "failed"
		// Try to find the error from the nodes
		for _, nr := range nodeResults {
			if nr.Error != "" {
				result.Error = nr.Error
				break
			}
		}
		if result.Error == "" {
			result.Error = fmt.Sprintf("Flow finished with status: %s", runner.FlowStatusString(finalStatus))
		}
	}

	if reporters != nil {
		reporters.HandleFlowResult(result)
	}

	if finalStatus != runner.FlowStatusSuccess {
		return result, errors.New(result.Error)
	}

	return result, nil
}
