package eventsync

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

func TestComputeNodeOrder_EmptyNodes(t *testing.T) {
	order := ComputeNodeOrder(nil, nil)
	require.Nil(t, order)
}

func TestComputeNodeOrder_SingleNode(t *testing.T) {
	nodes := []mflow.Node{
		{ID: idwrap.NewNow(), NodeKind: mflow.NODE_KIND_MANUAL_START},
	}

	order := ComputeNodeOrder(nodes, nil)
	require.Len(t, order, 1)
	require.Equal(t, nodes[0].ID, order[0])
}

func TestComputeNodeOrder_LinearChain(t *testing.T) {
	// Start -> Request1 -> Request2
	startID := idwrap.NewNow()
	req1ID := idwrap.NewNow()
	req2ID := idwrap.NewNow()

	nodes := []mflow.Node{
		{ID: req2ID, NodeKind: mflow.NODE_KIND_REQUEST}, // Out of order
		{ID: startID, NodeKind: mflow.NODE_KIND_MANUAL_START},
		{ID: req1ID, NodeKind: mflow.NODE_KIND_REQUEST},
	}

	edges := []mflow.Edge{
		{SourceID: startID, TargetID: req1ID},
		{SourceID: req1ID, TargetID: req2ID},
	}

	order := ComputeNodeOrder(nodes, edges)
	require.Len(t, order, 3)

	// Start should be first (level 0)
	require.Equal(t, startID, order[0])
	// Request1 should be second (level 1)
	require.Equal(t, req1ID, order[1])
	// Request2 should be third (level 2)
	require.Equal(t, req2ID, order[2])
}

func TestComputeNodeOrder_ContainerBeforeChildren(t *testing.T) {
	// Start -> ForEach -> Request (inside ForEach)
	startID := idwrap.NewNow()
	forEachID := idwrap.NewNow()
	requestID := idwrap.NewNow()

	nodes := []mflow.Node{
		{ID: requestID, NodeKind: mflow.NODE_KIND_REQUEST},
		{ID: forEachID, NodeKind: mflow.NODE_KIND_FOR_EACH},
		{ID: startID, NodeKind: mflow.NODE_KIND_MANUAL_START},
	}

	edges := []mflow.Edge{
		{SourceID: startID, TargetID: forEachID},
		{SourceID: forEachID, TargetID: requestID},
	}

	order := ComputeNodeOrder(nodes, edges)
	require.Len(t, order, 3)

	// Verify order: Start -> ForEach -> Request
	require.Equal(t, startID, order[0])
	require.Equal(t, forEachID, order[1])
	require.Equal(t, requestID, order[2])
}

func TestComputeNodeOrder_ParallelBranches(t *testing.T) {
	// Start -> [Request1, Request2] (parallel)
	startID := idwrap.NewNow()
	req1ID := idwrap.NewNow()
	req2ID := idwrap.NewNow()

	nodes := []mflow.Node{
		{ID: startID, NodeKind: mflow.NODE_KIND_MANUAL_START},
		{ID: req1ID, NodeKind: mflow.NODE_KIND_REQUEST},
		{ID: req2ID, NodeKind: mflow.NODE_KIND_REQUEST},
	}

	edges := []mflow.Edge{
		{SourceID: startID, TargetID: req1ID},
		{SourceID: startID, TargetID: req2ID},
	}

	order := ComputeNodeOrder(nodes, edges)
	require.Len(t, order, 3)

	// Start should be first
	require.Equal(t, startID, order[0])

	// Both requests should be after start (same level, sorted by ID for determinism)
	startIdx := 0
	var req1Idx, req2Idx int
	for i, id := range order {
		if id == req1ID {
			req1Idx = i
		}
		if id == req2ID {
			req2Idx = i
		}
	}
	require.Greater(t, req1Idx, startIdx)
	require.Greater(t, req2Idx, startIdx)
}

func TestComputeNodeOrder_OrphanNodes(t *testing.T) {
	// Start -> Request1, Orphan (not connected)
	startID := idwrap.NewNow()
	req1ID := idwrap.NewNow()
	orphanID := idwrap.NewNow()

	nodes := []mflow.Node{
		{ID: orphanID, NodeKind: mflow.NODE_KIND_REQUEST}, // Not connected
		{ID: startID, NodeKind: mflow.NODE_KIND_MANUAL_START},
		{ID: req1ID, NodeKind: mflow.NODE_KIND_REQUEST},
	}

	edges := []mflow.Edge{
		{SourceID: startID, TargetID: req1ID},
		// orphanID not connected
	}

	order := ComputeNodeOrder(nodes, edges)
	require.Len(t, order, 3)

	// Verify connected nodes come before orphans
	startIdx := indexOfID(order, startID)
	req1Idx := indexOfID(order, req1ID)
	orphanIdx := indexOfID(order, orphanID)

	require.Less(t, startIdx, req1Idx)
	require.Less(t, req1Idx, orphanIdx, "Orphan nodes should come after connected nodes")
}

func TestSortNodesByOrder(t *testing.T) {
	startID := idwrap.NewNow()
	req1ID := idwrap.NewNow()
	req2ID := idwrap.NewNow()

	// Nodes in wrong order
	nodes := []mflow.Node{
		{ID: req2ID, NodeKind: mflow.NODE_KIND_REQUEST},
		{ID: req1ID, NodeKind: mflow.NODE_KIND_REQUEST},
		{ID: startID, NodeKind: mflow.NODE_KIND_MANUAL_START},
	}

	edges := []mflow.Edge{
		{SourceID: startID, TargetID: req1ID},
		{SourceID: req1ID, TargetID: req2ID},
	}

	SortNodesByOrder(nodes, edges)

	// Nodes should now be in correct order
	require.Equal(t, startID, nodes[0].ID)
	require.Equal(t, req1ID, nodes[1].ID)
	require.Equal(t, req2ID, nodes[2].ID)
}

func TestGetNodeKindPriority(t *testing.T) {
	// ManualStart should have lowest priority (highest precedence)
	startPri := GetNodeKindPriority(mflow.NODE_KIND_MANUAL_START)
	forEachPri := GetNodeKindPriority(mflow.NODE_KIND_FOR_EACH)
	requestPri := GetNodeKindPriority(mflow.NODE_KIND_REQUEST)

	require.Less(t, startPri, forEachPri)
	require.Less(t, forEachPri, requestPri)
}

func indexOfID(slice []idwrap.IDWrap, item idwrap.IDWrap) int {
	for i, v := range slice {
		if v.Compare(item) == 0 {
			return i
		}
	}
	return -1
}
