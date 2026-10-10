package runner

import (
	"context"
	"fmt"
	"strings"
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
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/ioworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

// Cleanup is a flow's built cleanup: block, ready to run after the flow's
// normal steps. A nil *Cleanup (the flow has no cleanup block) runs nothing.
type Cleanup struct {
	flowID  idwrap.IDWrap
	steps   []ioworkspace.FlowCleanupStep
	nodes   map[idwrap.IDWrap]node.FlowNode
	timeout time.Duration
}

// CleanupBuildDeps are the per-run resources cleanup nodes share with the
// flow's normal nodes.
type CleanupBuildDeps struct {
	Timeout     time.Duration
	HTTPClient  httpclient.HttpClient
	RespChan    chan nrequest.NodeRequestSideResp
	GQLRespChan chan ngraphql.NodeGraphQLSideResp
}

// BuildCleanup builds the node implementations of flow's cleanup block. It
// returns nil when the flow has none.
func BuildCleanup(ctx context.Context, flow mflow.Flow, services RunnerServices, deps CleanupBuildDeps) (*Cleanup, error) {
	block, ok := services.Cleanups[flow.ID]
	if !ok || len(block.Steps) == 0 {
		return nil, nil
	}

	nodes, err := services.NodeService.GetNodesByFlowID(ctx, block.CleanupFlowID)
	if err != nil {
		return nil, fmt.Errorf("get cleanup nodes: %w", err)
	}

	// BuildNodes insists on an entry node. Cleanup flows have none (their
	// steps never run as a graph), so hand it a throwaway one; runStep starts
	// each step directly and never uses it.
	nodes = append(nodes, mflow.Node{
		ID:       idwrap.NewNow(),
		FlowID:   block.CleanupFlowID,
		Name:     "cleanup",
		NodeKind: mflow.NODE_KIND_MANUAL_START,
	})

	cleanupFlow := flow
	cleanupFlow.ID = block.CleanupFlowID
	nodeMap, _, err := services.Builder.BuildNodes(
		ctx,
		cleanupFlow,
		nodes,
		deps.Timeout,
		deps.HTTPClient,
		deps.RespChan,
		deps.GQLRespChan,
		services.JSClient,
	)
	if err != nil {
		return nil, fmt.Errorf("build cleanup steps: %w", err)
	}

	stepNodes := make(map[idwrap.IDWrap]node.FlowNode, len(block.Steps))
	for _, step := range block.Steps {
		n, ok := nodeMap[step.NodeID]
		if !ok {
			return nil, fmt.Errorf("cleanup step '%s' was not imported", step.Name)
		}
		stepNodes[step.NodeID] = n
	}
	ApplyStreams(stepNodes, services.Streams)

	return &Cleanup{
		flowID:  block.CleanupFlowID,
		steps:   block.Steps,
		nodes:   stepNodes,
		timeout: deps.Timeout,
	}, nil
}

// Size is the number of cleanup steps.
func (c *Cleanup) Size() int {
	if c == nil {
		return 0
	}
	return len(c.steps)
}

// DisplayNames are the cleanup step names as reporters show them.
func (c *Cleanup) DisplayNames() []string {
	if c == nil {
		return nil
	}
	names := make([]string, 0, len(c.steps))
	for _, step := range c.steps {
		names = append(names, reporter.DisplayStepName(step.Name, true))
	}
	return names
}

// Nodes returns the cleanup step nodes by ID.
func (c *Cleanup) Nodes() map[idwrap.IDWrap]node.FlowNode {
	if c == nil {
		return nil
	}
	return c.nodes
}

// Run executes the cleanup steps one at a time, in order, against the
// flow's final variable map (which it extends with the cleanup steps' own
// outputs). Every step is attempted: a failure does not stop the steps after
// it. A step is skipped instead when a step it reads produced no output, or
// a cleanup step it depends on did not succeed.
//
// It returns one result per step and, if any step failed, an error naming the
// first failure. Cleanup stops early only when ctx is cancelled (Ctrl-C).
func (c *Cleanup) Run(ctx context.Context, varMap map[string]any, emit func(reporter.NodeStatusEvent)) ([]model.NodeRunResult, error) {
	if c.Size() == 0 {
		return nil, nil
	}

	results := make([]model.NodeRunResult, 0, len(c.steps))
	succeeded := make(map[string]bool, len(c.steps))
	var firstErr error

	for _, step := range c.steps {
		if ctx.Err() != nil {
			break
		}

		if reason := c.skipReason(step, varMap, succeeded); reason != "" {
			emit(reporter.NodeStatusEvent{
				Status:     runner.FlowNodeStatus{NodeID: step.NodeID, Name: step.Name, State: mflow.NODE_STATE_CANCELED},
				Cleanup:    true,
				SkipReason: reason,
			})
			results = append(results, model.NodeRunResult{
				NodeID:     step.NodeID.String(),
				Name:       step.Name,
				State:      model.NodeStateSkipped,
				Cleanup:    true,
				SkipReason: reason,
			})
			continue
		}

		stepResults, err := c.runStep(ctx, step, varMap, emit)
		results = append(results, stepResults...)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("cleanup step '%s' failed: %w", step.Name, err)
			}
			continue
		}
		succeeded[step.Name] = true
	}

	return results, firstErr
}

func (c *Cleanup) skipReason(step ioworkspace.FlowCleanupStep, varMap map[string]any, succeeded map[string]bool) string {
	for _, dep := range step.DependsOn {
		if !succeeded[dep] {
			return fmt.Sprintf("depends on cleanup step '%s', which did not succeed", dep)
		}
	}

	var missing []string
	for _, ref := range step.References {
		if _, ok := varMap[ref]; !ok {
			missing = append(missing, ref)
		}
	}
	if len(missing) > 0 {
		return fmt.Sprintf("'%s' produced no output (it never ran)", strings.Join(missing, "', '"))
	}
	return ""
}

// runStep runs a single cleanup node through the flow engine, so it gets the
// same timeout, variable writes and status events as a normal step.
func (c *Cleanup) runStep(ctx context.Context, step ioworkspace.FlowCleanupStep, varMap map[string]any, emit func(reporter.NodeStatusEvent)) ([]model.NodeRunResult, error) {
	nodeMap := map[idwrap.IDWrap]node.FlowNode{step.NodeID: c.nodes[step.NodeID]}
	inst := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), c.flowID, []idwrap.IDWrap{step.NodeID}, nodeMap, mflow.EdgesMap{}, c.timeout, nil)

	// One node emits a handful of statuses; the buffers only need to outlast
	// a synchronous Run, which closes both channels when it returns.
	statusChan := make(chan runner.FlowNodeStatus, 64)
	flowStatusChan := make(chan runner.FlowStatus, 8)
	runErr := inst.Run(ctx, statusChan, flowStatusChan, varMap)

	var results []model.NodeRunResult
	var nodeErr error
	for status := range statusChan {
		emit(reporter.NodeStatusEvent{Status: status, Cleanup: true})
		if status.State == mflow.NODE_STATE_RUNNING {
			continue
		}
		nodeResult := buildNodeRunResult(status)
		nodeResult.Cleanup = true
		results = append(results, nodeResult)
		if status.State != mflow.NODE_STATE_SUCCESS && nodeErr == nil {
			nodeErr = status.Error
			if nodeErr == nil {
				nodeErr = fmt.Errorf("finished with state %s", mflow.StringNodeState(status.State))
			}
		}
	}
	for range flowStatusChan {
		// Drain; the outcome is runErr and the node statuses above.
	}

	if nodeErr != nil {
		return results, nodeErr
	}
	return results, runErr
}
