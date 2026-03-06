package runner

import (
	"sync"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

// FlowGraph is an immutable representation of a flow's DAG topology.
// Constructed once before execution begins, then shared across all components.
// It holds no node implementations (no node.FlowNode) to avoid import cycles.
type FlowGraph struct {
	Edges          mflow.EdgesMap
	StartNodeIDs   []idwrap.IDWrap
	Predecessors   map[idwrap.IDWrap][]idwrap.IDWrap
	ConvergeCounts map[idwrap.IDWrap]uint32
}

// NewFlowGraph constructs an immutable graph from edges and start nodes.
// It precomputes predecessor maps and convergence counts.
func NewFlowGraph(edges mflow.EdgesMap, startNodeIDs []idwrap.IDWrap) *FlowGraph {
	predecessors := BuildPredecessorMap(edges)
	convergeCounts := make(map[idwrap.IDWrap]uint32)
	for nodeID, preds := range predecessors {
		if count := uint32(len(preds)); count > 1 { //nolint:gosec // G115
			convergeCounts[nodeID] = count
		}
	}
	return &FlowGraph{
		Edges:          edges,
		StartNodeIDs:   startNodeIDs,
		Predecessors:   predecessors,
		ConvergeCounts: convergeCounts,
	}
}

// NewFlowGraphFromPredecessors constructs a FlowGraph when the predecessor map
// is already computed (e.g., inside RunNodeSync where loop nodes pre-build it).
func NewFlowGraphFromPredecessors(
	edges mflow.EdgesMap,
	startNodeID idwrap.IDWrap,
	predecessors map[idwrap.IDWrap][]idwrap.IDWrap,
) *FlowGraph {
	convergeCounts := make(map[idwrap.IDWrap]uint32)
	for nodeID, preds := range predecessors {
		if count := uint32(len(preds)); count > 1 { //nolint:gosec // G115
			convergeCounts[nodeID] = count
		}
	}
	return &FlowGraph{
		Edges:          edges,
		StartNodeIDs:   []idwrap.IDWrap{startNodeID},
		Predecessors:   predecessors,
		ConvergeCounts: convergeCounts,
	}
}

// NewConvergenceTracker creates a fresh mutable tracker for one execution.
func (g *FlowGraph) NewConvergenceTracker() *ConvergenceTracker {
	pending := make(map[idwrap.IDWrap]uint32, len(g.ConvergeCounts))
	for nodeID, count := range g.ConvergeCounts {
		pending[nodeID] = count
	}
	return &ConvergenceTracker{pending: pending}
}

// NewConvergenceTrackerFromPending creates a tracker from a pre-built pending
// map (e.g. from FlowNodeRequest.PendingAtmoicMap). It copies the map to avoid
// aliasing the original.
func NewConvergenceTrackerFromPending(pending map[idwrap.IDWrap]uint32) *ConvergenceTracker {
	clone := make(map[idwrap.IDWrap]uint32, len(pending))
	for k, v := range pending {
		clone[k] = v
	}
	return &ConvergenceTracker{pending: clone}
}

// BuildPredecessorMap computes which nodes precede each node in the graph.
func BuildPredecessorMap(edges mflow.EdgesMap) map[idwrap.IDWrap][]idwrap.IDWrap {
	predecessors := make(map[idwrap.IDWrap][]idwrap.IDWrap, len(edges))
	for sourceID, handles := range edges {
		for _, targets := range handles {
			for _, targetID := range targets {
				predecessors[targetID] = append(predecessors[targetID], sourceID)
			}
		}
	}
	return predecessors
}

// ConvergenceTracker tracks how many predecessors have completed for
// convergence (join) nodes. It is the mutable counterpart to FlowGraph's
// immutable ConvergeCounts.
type ConvergenceTracker struct {
	mu      sync.Mutex
	pending map[idwrap.IDWrap]uint32
}

// Arrive records that one predecessor of nodeID has completed.
// Returns true when all predecessors have arrived (the node is ready).
func (ct *ConvergenceTracker) Arrive(nodeID idwrap.IDWrap) bool {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	remaining, ok := ct.pending[nodeID]
	if !ok {
		return true // not a convergence point
	}
	if remaining <= 1 {
		delete(ct.pending, nodeID)
		return true
	}
	ct.pending[nodeID] = remaining - 1
	return false
}

// Clone creates an independent copy for loop iteration isolation.
func (ct *ConvergenceTracker) Clone() *ConvergenceTracker {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	clone := make(map[idwrap.IDWrap]uint32, len(ct.pending))
	for k, v := range ct.pending {
		clone[k] = v
	}
	return &ConvergenceTracker{pending: clone}
}
