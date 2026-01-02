package eventsync

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTopologicalSort_NoDependencies(t *testing.T) {
	deps := map[EventKind][]EventKind{
		"a": {},
		"b": {},
		"c": {},
	}

	order, err := TopologicalSort(deps)
	require.NoError(t, err)
	require.Len(t, order, 3)
	// Should be alphabetically sorted since all are roots
	require.Equal(t, []EventKind{"a", "b", "c"}, order)
}

func TestTopologicalSort_LinearChain(t *testing.T) {
	deps := map[EventKind][]EventKind{
		"a": {},
		"b": {"a"},
		"c": {"b"},
	}

	order, err := TopologicalSort(deps)
	require.NoError(t, err)
	require.Equal(t, []EventKind{"a", "b", "c"}, order)
}

func TestTopologicalSort_Diamond(t *testing.T) {
	// Diamond pattern:
	//     a
	//    / \
	//   b   c
	//    \ /
	//     d
	deps := map[EventKind][]EventKind{
		"a": {},
		"b": {"a"},
		"c": {"a"},
		"d": {"b", "c"},
	}

	order, err := TopologicalSort(deps)
	require.NoError(t, err)

	// a must come first
	require.Equal(t, EventKind("a"), order[0])

	// b and c must come before d
	aIdx := indexOf(order, "a")
	bIdx := indexOf(order, "b")
	cIdx := indexOf(order, "c")
	dIdx := indexOf(order, "d")

	require.Less(t, aIdx, bIdx)
	require.Less(t, aIdx, cIdx)
	require.Less(t, bIdx, dIdx)
	require.Less(t, cIdx, dIdx)
}

func TestTopologicalSort_DetectsCycle(t *testing.T) {
	deps := map[EventKind][]EventKind{
		"a": {"c"}, // a depends on c
		"b": {"a"}, // b depends on a
		"c": {"b"}, // c depends on b (cycle!)
	}

	_, err := TopologicalSort(deps)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cycle")
}

func TestValidateDependencies_NoError(t *testing.T) {
	// The static Dependencies map should be valid
	err := ValidateDependencies()
	require.NoError(t, err)
}

func TestGetEventOrder_ReturnsValidOrder(t *testing.T) {
	order := GetEventOrder()
	require.NotEmpty(t, order)

	// Verify Flow comes before FlowFile and Node
	flowIdx := indexOf(order, KindFlow)
	flowFileIdx := indexOf(order, KindFlowFile)
	nodeIdx := indexOf(order, KindNode)

	require.NotEqual(t, -1, flowIdx, "Flow should be in order")
	require.NotEqual(t, -1, flowFileIdx, "FlowFile should be in order")
	require.NotEqual(t, -1, nodeIdx, "Node should be in order")

	require.Less(t, flowIdx, flowFileIdx, "Flow should come before FlowFile")
	require.Less(t, flowIdx, nodeIdx, "Flow should come before Node")

	// Verify Node comes before Edge
	edgeIdx := indexOf(order, KindEdge)
	require.Less(t, nodeIdx, edgeIdx, "Node should come before Edge")

	// Verify HTTP comes before its children
	httpIdx := indexOf(order, KindHTTP)
	httpHeaderIdx := indexOf(order, KindHTTPHeader)
	require.Less(t, httpIdx, httpHeaderIdx, "HTTP should come before HTTPHeader")
}

func TestGetEventPriority(t *testing.T) {
	// Flow should have lower priority (earlier) than FlowFile
	flowPri := GetEventPriority(KindFlow)
	flowFilePri := GetEventPriority(KindFlowFile)
	require.Less(t, flowPri, flowFilePri)

	// Unknown kind should return -1
	unknownPri := GetEventPriority("unknown_kind")
	require.Equal(t, -1, unknownPri)
}

func TestSortEventKinds(t *testing.T) {
	kinds := []EventKind{KindHTTPHeader, KindFlow, KindNode, KindFlowFile}
	SortEventKinds(kinds)

	// Verify sorted order matches dependency order
	for i := 0; i < len(kinds)-1; i++ {
		priI := GetEventPriority(kinds[i])
		priJ := GetEventPriority(kinds[i+1])
		require.LessOrEqual(t, priI, priJ, "kinds should be sorted by priority")
	}
}

func indexOf(slice []EventKind, item EventKind) int {
	for i, v := range slice {
		if v == item {
			return i
		}
	}
	return -1
}
