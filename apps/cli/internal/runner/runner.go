package runner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/the-dev-tools/dev-tools/apps/cli/internal/model"
	"github.com/the-dev-tools/dev-tools/apps/cli/internal/reporter"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/ngraphql"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/nrequest"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner/flowlocalrunner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/httpclient"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/private/node_js_executor/v1/node_js_executorv1connect"

	// Service interfaces
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/flowbuilder"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"

	"connectrpc.com/connect"
)

type RunnerServices struct {
	NodeService         sflow.NodeService
	EdgeService         sflow.EdgeService
	FlowVariableService sflow.FlowVariableService
	Builder             *flowbuilder.Builder
	JSClient            node_js_executorv1connect.NodeJsExecutorServiceClient
}

// RunMultipleFlows executes multiple flows based on the run field configuration.
// Flows run sequentially in dependency order (a topological sort of the
// run: block, ties broken by original list order) rather than run: block
// declaration order. A flow whose dependency failed, or was itself skipped,
// is reported as skipped instead of attempted; flows with no such gate still
// run even if an unrelated earlier flow failed.
func RunMultipleFlows(ctx context.Context, fileData []byte, allFlows []mflow.Flow, services RunnerServices, logger *slog.Logger, reporters *reporter.ReporterGroup) error {
	entries, err := parseRunEntries(fileData)
	if err != nil {
		return err
	}

	// Create flow map for easy lookup, and fail fast if the run: block names
	// a flow that does not exist, before running anything.
	flowMap := make(map[string]*mflow.Flow, len(allFlows))
	for i := range allFlows {
		flowMap[allFlows[i].Name] = &allFlows[i]
	}
	for _, entry := range entries {
		if _, exists := flowMap[entry.flowName]; !exists {
			return fmt.Errorf("flow '%s' not found in workflow", entry.flowName)
		}
	}

	// Unknown depends_on names are dropped with a warning rather than
	// aborting the run: they were silently ignored before the run: block
	// gained a topological sort, and shipped example files rely on that.
	// Cycles and duplicate flow names are still hard errors.
	sorted, warnings, err := topoSortRunEntries(entries)
	for _, w := range warnings {
		fmt.Fprintln(os.Stderr, w)
		logger.Warn(w)
	}
	if err != nil {
		return err
	}

	// Track execution results
	executionResults := make(map[string]model.FlowRunResult, len(sorted))
	statusByFlow := make(map[string]string, len(sorted))
	consoleEnabled := reporters != nil && reporters.HasConsole()

	// Execute flows in dependency order
	if consoleEnabled {
		fmt.Println("\n=== Multi-Flow Execution Starting ===")
		fmt.Printf("Flows to execute: %d\n", len(sorted))
	}

	overallStartTime := time.Now()

	for i, entry := range sorted {
		if failedDep, gated := firstUnsuccessfulDependency(entry, statusByFlow); gated {
			reason := fmt.Sprintf("dependency %q failed", failedDep)
			result := model.FlowRunResult{
				FlowName: entry.flowName,
				Status:   "skipped",
				Error:    reason,
			}
			executionResults[entry.flowName] = result
			statusByFlow[entry.flowName] = result.Status

			logger.Warn("flow skipped", "flow", entry.flowName, "reason", reason)
			if reporters != nil {
				reporters.HandleFlowResult(result)
			}
			if consoleEnabled {
				fmt.Printf("\n[%d/%d] Skipping flow: %s (%s)\n", i+1, len(sorted), entry.flowName, reason)
			}
			continue
		}

		flow := flowMap[entry.flowName]

		if consoleEnabled {
			fmt.Printf("\n[%d/%d] Executing flow: %s\n", i+1, len(sorted), entry.flowName)
			if len(entry.dependsOn) > 0 {
				fmt.Printf("   Dependencies: %v\n", entry.dependsOn)
			}
		}

		result, err := RunFlow(ctx, flow, services, reporters)
		executionResults[entry.flowName] = result
		statusByFlow[entry.flowName] = result.Status

		if err != nil {
			if consoleEnabled {
				fmt.Printf("   ❌ Flow failed: %v\n", err)
			}
			logger.Error("flow execution failed", "flow", entry.flowName, "error", err)
		} else if consoleEnabled {
			fmt.Printf("   ✅ Flow completed successfully (Duration: %s)\n", reporter.FormatDuration(result.Duration))
		}
	}

	if consoleEnabled {
		overallDuration := time.Since(overallStartTime)
		fmt.Println("\n=== Multi-Flow Execution Summary ===")
		fmt.Printf("Total duration: %s\n", reporter.FormatDuration(overallDuration))
		fmt.Println("\nFlow Results:")

		successCount := 0
		for _, entry := range sorted {
			result := executionResults[entry.flowName]
			status := "❌ Failed"
			switch {
			case strings.EqualFold(result.Status, "success"):
				status = "✅ Success"
				successCount++
			case strings.EqualFold(result.Status, "skipped"):
				status = "⏭️  Skipped"
			}
			fmt.Printf("  %-20s %s (Duration: %s)\n", entry.flowName, status, reporter.FormatDuration(result.Duration))
		}

		fmt.Printf("\nFlows completed: %d/%d\n", successCount, len(sorted))
	}

	var problems []string
	for _, entry := range sorted {
		result := executionResults[entry.flowName]
		if strings.EqualFold(result.Status, "success") {
			continue
		}
		detail := result.Error
		if detail == "" {
			detail = "no result recorded"
		}
		problems = append(problems, fmt.Sprintf("%s: %s (%s)", entry.flowName, detail, result.Status))
	}
	if len(problems) > 0 {
		return fmt.Errorf("multi-flow execution failed: %s", strings.Join(problems, "; "))
	}

	return nil
}

func RunFlow(ctx context.Context, flowPtr *mflow.Flow, services RunnerServices, reporters *reporter.ReporterGroup) (model.FlowRunResult, error) {
	result := model.FlowRunResult{
		FlowID:   flowPtr.ID.String(),
		FlowName: flowPtr.Name,
		Started:  time.Now(),
	}

	markFailure := func(err error) (model.FlowRunResult, error) {
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

	nodes, err := services.NodeService.GetNodesByFlowID(ctx, latestFlowID)
	if err != nil {
		return markFailure(connect.NewError(connect.CodeInternal, errors.New("get nodes")))
	}

	edges, err := services.EdgeService.GetEdgesByFlowID(ctx, latestFlowID)
	if err != nil {
		return markFailure(connect.NewError(connect.CodeInternal, errors.New("get edges")))
	}
	edgeMap := mflow.NewEdgesMap(edges)

	flowVars, err := services.FlowVariableService.GetFlowVariablesByFlowID(ctx, latestFlowID)
	if err != nil {
		return markFailure(connect.NewError(connect.CodeInternal, errors.New("get edges")))
	}

	// Build flow variables using flowbuilder
	// Note: BuildVariables takes workspaceID, not flowID, to fetch environment variables
	flowVarsMap, err := services.Builder.BuildVariables(ctx, flowPtr.WorkspaceID, flowVars)
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

	// Initialize GraphQL response channel
	gqlRespChan := make(chan ngraphql.NodeGraphQLSideResp, requestBufferSize)
	go func() {
		for resp := range gqlRespChan {
			if resp.Done != nil {
				close(resp.Done)
			}
		}
	}()
	defer close(gqlRespChan)

	// Build flow node map using flowbuilder
	flowNodeMap, startNodeIDs, err := services.Builder.BuildNodes(
		ctx,
		*flowPtr,
		nodes,
		nodeTimeout,
		httpClient,
		requestRespChan,
		gqlRespChan,
		services.JSClient,
	)
	if err != nil {
		return markFailure(err)
	}

	// Use the same timeout for the flow runner
	runnerInst := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), latestFlowID, startNodeIDs, flowNodeMap, edgeMap, nodeTimeout, nil)

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
		reporters.HandleFlowStart(reporter.FlowStartInfo{
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
	nodeResults := make([]model.NodeRunResult, 0)
	var finalStatus runner.FlowStatus

	// Wait for completion
	for {
		select {
		case nodeStatus, ok := <-flowNodeStatusChan:
			if !ok {
				flowNodeStatusChan = nil
				continue
			}
			if reporters != nil {
				reporters.HandleNodeStatus(reporter.NodeStatusEvent{
					FlowID:   result.FlowID,
					FlowName: flowPtr.Name,
					Status:   nodeStatus,
				})
			}
			if nodeStatus.State != mflow.NODE_STATE_RUNNING {
				// Hack: Fix for unintended file system artifacts (like .git folder) being picked up as nodes
				// This usually happens when implicit file scanning interacts with the flow execution
				if nodeStatus.Name == ".git" || strings.HasPrefix(nodeStatus.Name, ".git/") || strings.HasPrefix(nodeStatus.Name, ".git\\") {
					continue
				}
				nodeResults = append(nodeResults, buildNodeRunResult(nodeStatus))
			}

		case flowStatus, ok := <-flowStatusChan:
			if !ok {
				flowStatusChan = nil
				continue
			}
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

func buildNodeRunResult(status runner.FlowNodeStatus) model.NodeRunResult {
	nodeResult := model.NodeRunResult{
		NodeID:      status.NodeID.String(),
		ExecutionID: status.ExecutionID.String(),
		Name:        status.Name,
		State:       mflow.StringNodeState(status.State),
		Duration:    status.RunDuration,
	}

	if status.Error != nil {
		nodeResult.Error = status.Error.Error()
	}

	if status.IterationContext != nil {
		ctx := &model.IterationContextResult{
			IterationPath:  append([]int(nil), status.IterationContext.IterationPath...),
			ExecutionIndex: status.IterationContext.ExecutionIndex,
		}
		if len(status.IterationContext.ParentNodes) > 0 {
			parents := make([]string, 0, len(status.IterationContext.ParentNodes))
			for _, parent := range status.IterationContext.ParentNodes {
				parents = append(parents, parent.String())
			}
			ctx.ParentNodes = parents
		}
		nodeResult.IterationContext = ctx
	}

	// Capture output data if present
	nodeResult.OutputData = status.OutputData

	return nodeResult
}
