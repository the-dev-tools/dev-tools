package runner_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/nfor"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/nstart"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner/flowlocalrunner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// trackingNode is a simple node that records when it runs
type trackingNode struct {
	id    idwrap.IDWrap
	name  string
	mu    *sync.Mutex
	log   *[]string
	delay time.Duration
}

func newTrackingNode(name string, mu *sync.Mutex, log *[]string, delay time.Duration) *trackingNode {
	return &trackingNode{
		id:    idwrap.NewNow(),
		name:  name,
		mu:    mu,
		log:   log,
		delay: delay,
	}
}

func (n *trackingNode) GetID() idwrap.IDWrap { return n.id }
func (n *trackingNode) GetName() string      { return n.name }

func (n *trackingNode) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	fmt.Printf("Node %s running\n", n.name)
	if n.delay > 0 {
		time.Sleep(n.delay)
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	*n.log = append(*n.log, n.name)

	nextID := mflow.GetNextNodeID(req.EdgeSourceMap, n.id, mflow.HandleThen)
	return node.FlowNodeResult{NextNodeID: nextID}
}

func (n *trackingNode) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	resultChan <- n.RunSync(ctx, req)
}

func TestLoopExecutionOrder(t *testing.T) {
	// Setup shared log
	var executionLog []string
	var mu sync.Mutex

	// Create nodes
	startNode := nstart.New(idwrap.NewNow(), "Start")

	// Loop node: 3 iterations
	loopNode := nfor.New(idwrap.NewNow(), "Loop", 3, 10*time.Second, mflow.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)

	// Child nodes inside the loop
	// We add a small delay to node A to simulate work and potential race conditions
	nodeA := newTrackingNode("Node A", &mu, &executionLog, 10*time.Millisecond)
	nodeB := newTrackingNode("Node B", &mu, &executionLog, 0)

	// Build edges
	// Start -> Loop
	// Loop (Loop handle) -> Node A
	// Node A -> Node B
	edges := []mflow.Edge{
		mflow.NewEdge(idwrap.NewNow(), startNode.GetID(), loopNode.GetID(), mflow.HandleUnspecified),
		mflow.NewEdge(idwrap.NewNow(), loopNode.GetID(), nodeA.GetID(), mflow.HandleLoop),
		mflow.NewEdge(idwrap.NewNow(), nodeA.GetID(), nodeB.GetID(), mflow.HandleThen),
	}
	edgeMap := mflow.NewEdgesMap(edges)

	// Setup node registry
	nodeRegistry := map[idwrap.IDWrap]node.FlowNode{
		startNode.GetID(): startNode,
		loopNode.GetID():  loopNode,
		nodeA.GetID():     nodeA,
		nodeB.GetID():     nodeB,
	}

	// Capture log events
	var logEvents []runner.FlowNodeStatus
	var logMu sync.Mutex

	// Setup variable system
	varSystem := make(map[string]any)

	// Execution context
	ctx := context.Background()
	req := &node.FlowNodeRequest{
		VarMap:        varSystem,
		ReadWriteLock: &sync.RWMutex{},
		NodeMap:       nodeRegistry,
		EdgeSourceMap: edgeMap,
		Timeout:       30 * time.Second,
		LogPushFunc: func(status runner.FlowNodeStatus) {
			logMu.Lock()
			defer logMu.Unlock()
			// Only capture completion events (SUCCESS/FAILURE) to verify completion order
			if status.State == mflow.NODE_STATE_SUCCESS || status.State == mflow.NODE_STATE_FAILURE {
				logEvents = append(logEvents, status)
			}
		},
	}

	// Calculate predecessors
	predecessors := flowlocalrunner.BuildPredecessorMap(edgeMap)

	// Run the flow starting from Start node
	err := flowlocalrunner.RunNodeASync(ctx, startNode.GetID(), req, req.LogPushFunc, predecessors)
	require.NoError(t, err)

	// Verify actual execution order
	expectedExec := []string{"Node A", "Node B", "Node A", "Node B", "Node A", "Node B"}

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, expectedExec, executionLog, "Physical execution order mismatch")

	// Verify Log Event Order
	logMu.Lock()
	defer logMu.Unlock()

	var eventNames []string
	for _, e := range logEvents {
		// Filter out Loop events themselves, just check child nodes
		if e.Name == "Node A" || e.Name == "Node B" {
			eventNames = append(eventNames, e.Name)
		}
	}

	assert.Equal(t, expectedExec, eventNames, "Log event emission order mismatch")
	if !assert.ObjectsAreEqual(expectedExec, eventNames) {
		fmt.Printf("Expected Events: %v\nActual Events:   %v\n", expectedExec, eventNames)
	}
}
