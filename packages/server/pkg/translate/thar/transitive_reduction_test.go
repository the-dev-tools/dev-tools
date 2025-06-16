package thar

import (
	"testing"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"github.com/stretchr/testify/require"
)

func TestPerformTransitiveReduction(t *testing.T) {
	tests := []struct {
		name          string
		createEdges   func() ([]edge.Edge, idwrap.IDWrap, idwrap.IDWrap, idwrap.IDWrap, idwrap.IDWrap)
		expectedCount int
		checkEdges    func(t *testing.T, edges []edge.Edge, nodeA, nodeB, nodeC idwrap.IDWrap)
	}{
		{
			name: "Simple transitive case A→B→C with redundant A→C",
			createEdges: func() ([]edge.Edge, idwrap.IDWrap, idwrap.IDWrap, idwrap.IDWrap, idwrap.IDWrap) {
				flowID := idwrap.NewNow()
				nodeA := idwrap.NewNow()
				nodeB := idwrap.NewNow()
				nodeC := idwrap.NewNow()
				
				edges := []edge.Edge{
					{ID: idwrap.NewNow(), FlowID: flowID, SourceID: nodeA, TargetID: nodeB},
					{ID: idwrap.NewNow(), FlowID: flowID, SourceID: nodeB, TargetID: nodeC},
					{ID: idwrap.NewNow(), FlowID: flowID, SourceID: nodeA, TargetID: nodeC}, // Redundant
				}
				
				return edges, flowID, nodeA, nodeB, nodeC
			},
			expectedCount: 2,
			checkEdges: func(t *testing.T, edges []edge.Edge, nodeA, nodeB, nodeC idwrap.IDWrap) {
				// Should have A→B and B→C, but not A→C
				hasAtoB := false
				hasBtoC := false
				hasAtoC := false
				
				for _, e := range edges {
					if e.SourceID == nodeA && e.TargetID == nodeB {
						hasAtoB = true
					}
					if e.SourceID == nodeB && e.TargetID == nodeC {
						hasBtoC = true
					}
					if e.SourceID == nodeA && e.TargetID == nodeC {
						hasAtoC = true
					}
				}
				
				require.True(t, hasAtoB, "Expected edge A→B")
				require.True(t, hasBtoC, "Expected edge B→C")
				require.False(t, hasAtoC, "Should not have redundant edge A→C")
			},
		},
		{
			name: "Diamond pattern with redundant edges",
			createEdges: func() ([]edge.Edge, idwrap.IDWrap, idwrap.IDWrap, idwrap.IDWrap, idwrap.IDWrap) {
				flowID := idwrap.NewNow()
				nodeA := idwrap.NewNow()
				nodeB := idwrap.NewNow()
				nodeC := idwrap.NewNow()
				nodeD := idwrap.NewNow()
				
				edges := []edge.Edge{
					{ID: idwrap.NewNow(), FlowID: flowID, SourceID: nodeA, TargetID: nodeB},
					{ID: idwrap.NewNow(), FlowID: flowID, SourceID: nodeA, TargetID: nodeC},
					{ID: idwrap.NewNow(), FlowID: flowID, SourceID: nodeB, TargetID: nodeD},
					{ID: idwrap.NewNow(), FlowID: flowID, SourceID: nodeC, TargetID: nodeD},
					{ID: idwrap.NewNow(), FlowID: flowID, SourceID: nodeA, TargetID: nodeD}, // Redundant
				}
				
				return edges, flowID, nodeA, nodeB, nodeD
			},
			expectedCount: 4,
			checkEdges: func(t *testing.T, edges []edge.Edge, nodeA, nodeB, nodeD idwrap.IDWrap) {
				// Should not have direct A→D edge as it's reachable through B and C
				hasAtoD := false
				
				for _, e := range edges {
					if e.SourceID == nodeA && e.TargetID == nodeD {
						hasAtoD = true
					}
				}
				
				require.False(t, hasAtoD, "Should not have redundant edge A→D")
			},
		},
		{
			name: "No redundant edges",
			createEdges: func() ([]edge.Edge, idwrap.IDWrap, idwrap.IDWrap, idwrap.IDWrap, idwrap.IDWrap) {
				flowID := idwrap.NewNow()
				nodeA := idwrap.NewNow()
				nodeB := idwrap.NewNow()
				nodeC := idwrap.NewNow()
				
				edges := []edge.Edge{
					{ID: idwrap.NewNow(), FlowID: flowID, SourceID: nodeA, TargetID: nodeB},
					{ID: idwrap.NewNow(), FlowID: flowID, SourceID: nodeA, TargetID: nodeC},
				}
				
				return edges, flowID, nodeA, nodeB, nodeC
			},
			expectedCount: 2,
			checkEdges: func(t *testing.T, edges []edge.Edge, nodeA, nodeB, nodeC idwrap.IDWrap) {
				// All edges should remain
				require.Equal(t, 2, len(edges))
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edges, flowID, nodeA, nodeB, nodeC := tt.createEdges()
			
			result := &HarResvoled{
				Flow: mflow.Flow{ID: flowID},
				Edges: edges,
			}
			
			err := performTransitiveReduction(result)
			require.NoError(t, err)
			
			require.Equal(t, tt.expectedCount, len(result.Edges), "Wrong number of edges after reduction")
			tt.checkEdges(t, result.Edges, nodeA, nodeB, nodeC)
		})
	}
}

func TestEnsureProperDependencyOrderingWithTransitiveReduction(t *testing.T) {
	// Test that ensureProperDependencyOrdering properly calls transitive reduction
	flowID := idwrap.NewNow()
	startID := idwrap.NewNow()
	nodeA := idwrap.NewNow()
	nodeB := idwrap.NewNow()
	nodeC := idwrap.NewNow()
	
	result := &HarResvoled{
		Flow: mflow.Flow{ID: flowID},
		Nodes: []mnnode.MNode{
			{ID: startID, Name: "Start", NodeKind: mnnode.NODE_KIND_NO_OP},
			{ID: nodeA, Name: "A", NodeKind: mnnode.NODE_KIND_REQUEST},
			{ID: nodeB, Name: "B", NodeKind: mnnode.NODE_KIND_REQUEST},
			{ID: nodeC, Name: "C", NodeKind: mnnode.NODE_KIND_REQUEST},
		},
		NoopNodes: []mnnoop.NoopNode{
			{FlowNodeID: startID, Type: mnnoop.NODE_NO_OP_KIND_START},
		},
		Edges: []edge.Edge{
			// A→B→C with redundant A→C
			{ID: idwrap.NewNow(), FlowID: flowID, SourceID: nodeA, TargetID: nodeB},
			{ID: idwrap.NewNow(), FlowID: flowID, SourceID: nodeB, TargetID: nodeC},
			{ID: idwrap.NewNow(), FlowID: flowID, SourceID: nodeA, TargetID: nodeC}, // Redundant
		},
	}
	
	err := ensureProperDependencyOrdering(result, startID, flowID)
	require.NoError(t, err)
	
	// Should have:
	// 1. Removed the redundant A→C edge
	// 2. Added start→A edge (since A has no incoming dependencies after reduction)
	
	// Count edges by type
	startToA := 0
	aToB := 0
	bToC := 0
	aToC := 0
	
	for _, e := range result.Edges {
		if e.SourceID == startID && e.TargetID == nodeA {
			startToA++
		}
		if e.SourceID == nodeA && e.TargetID == nodeB {
			aToB++
		}
		if e.SourceID == nodeB && e.TargetID == nodeC {
			bToC++
		}
		if e.SourceID == nodeA && e.TargetID == nodeC {
			aToC++
		}
	}
	
	require.Equal(t, 1, startToA, "Should have start→A edge")
	require.Equal(t, 1, aToB, "Should have A→B edge")
	require.Equal(t, 1, bToC, "Should have B→C edge")
	require.Equal(t, 0, aToC, "Should not have redundant A→C edge")
	require.Equal(t, 3, len(result.Edges), "Should have exactly 3 edges")
}