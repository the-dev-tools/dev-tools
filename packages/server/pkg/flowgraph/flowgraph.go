// Package flowgraph provides graph layout algorithms for flow nodes.
// It supports both horizontal (left-to-right) and vertical (top-to-bottom)
// layouts using BFS-based level assignment.
package flowgraph

import (
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

// LayoutOrientation defines the primary direction of flow.
type LayoutOrientation int

const (
	// LayoutHorizontal: nodes flow left-to-right (X increases with depth).
	// Used by HAR import (harv2).
	LayoutHorizontal LayoutOrientation = iota

	// LayoutVertical: nodes flow top-to-bottom (Y increases with depth).
	// Used by YAML import (ioworkspace).
	LayoutVertical
)

// LayoutConfig configures the layout algorithm.
type LayoutConfig struct {
	// Orientation controls whether depth maps to X (horizontal) or Y (vertical).
	Orientation LayoutOrientation

	// SpacingPrimary is spacing along the primary axis (direction of flow).
	SpacingPrimary float64

	// SpacingSecondary is spacing perpendicular to flow (for parallel nodes).
	SpacingSecondary float64

	// StartX is the starting X position.
	StartX float64

	// StartY is the starting Y position.
	StartY float64
}

// Position holds X and Y coordinates.
type Position struct {
	X float64
	Y float64
}

// LayoutResult contains the computed positions for each node.
type LayoutResult struct {
	// Positions maps node IDs to their computed positions.
	Positions map[idwrap.IDWrap]Position

	// Levels maps node IDs to their depth level (0 = start node).
	Levels map[idwrap.IDWrap]int

	// MaxLevel is the deepest level in the graph.
	MaxLevel int
}

// DefaultHorizontalConfig returns the default configuration for horizontal layout.
// This matches the harv2 layout: nodes flow left-to-right with vertical stacking.
func DefaultHorizontalConfig() LayoutConfig {
	return LayoutConfig{
		Orientation:      LayoutHorizontal,
		SpacingPrimary:   300, // X spacing between levels
		SpacingSecondary: 150, // Y spacing between parallel nodes
		StartX:           0,
		StartY:           0,
	}
}

// DefaultVerticalConfig returns the default configuration for vertical layout.
// This matches the ioworkspace layout: nodes flow top-to-bottom with horizontal stacking.
func DefaultVerticalConfig() LayoutConfig {
	return LayoutConfig{
		Orientation:      LayoutVertical,
		SpacingPrimary:   300, // Y spacing between levels
		SpacingSecondary: 400, // X spacing between parallel nodes
		StartX:           0,
		StartY:           0,
	}
}

// BuildOutgoingAdjacency builds a map of node ID -> list of target node IDs.
func BuildOutgoingAdjacency(edges []mflow.Edge) map[idwrap.IDWrap][]idwrap.IDWrap {
	adj := make(map[idwrap.IDWrap][]idwrap.IDWrap)
	for _, e := range edges {
		adj[e.SourceID] = append(adj[e.SourceID], e.TargetID)
	}
	return adj
}

// BuildIncomingAdjacency builds a map of node ID -> list of source node IDs.
func BuildIncomingAdjacency(edges []mflow.Edge) map[idwrap.IDWrap][]idwrap.IDWrap {
	adj := make(map[idwrap.IDWrap][]idwrap.IDWrap)
	for _, e := range edges {
		adj[e.TargetID] = append(adj[e.TargetID], e.SourceID)
	}
	return adj
}

// FindStartNode finds the start node (NODE_KIND_MANUAL_START) in a node slice.
func FindStartNode(nodes []mflow.Node) (*mflow.Node, bool) {
	for i := range nodes {
		if nodes[i].NodeKind == mflow.NODE_KIND_MANUAL_START {
			return &nodes[i], true
		}
	}
	return nil, false
}

// EdgeExists checks if an edge exists between source and target.
func EdgeExists(edges []mflow.Edge, source, target idwrap.IDWrap) bool {
	for _, e := range edges {
		if e.SourceID == source && e.TargetID == target {
			return true
		}
	}
	return false
}

// BuildNodeMap creates a map of node ID -> node pointer for quick lookup.
func BuildNodeMap(nodes []mflow.Node) map[idwrap.IDWrap]*mflow.Node {
	nodeMap := make(map[idwrap.IDWrap]*mflow.Node)
	for i := range nodes {
		nodeMap[nodes[i].ID] = &nodes[i]
	}
	return nodeMap
}
